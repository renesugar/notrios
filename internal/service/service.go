// Package service wires configuration, the SQLite store, the HTTP API
// handler, and the optional Recoll search sidecar into one runnable unit.
// notriosd (headless) and notrios (built-in GUI) share this startup path.
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/profiles"
	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/recoll"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncjobs"
	"github.com/renesugar/notrios/internal/synckeys"
	"github.com/renesugar/notrios/internal/syncrest"
	"github.com/renesugar/notrios/internal/syncwire"
)

// Service is a started Notrios backend.
type Service struct {
	Config    config.Config
	Store     *store.SQLiteStore
	Handler   *httpapi.Server
	SyncJobs  *syncjobs.Manager
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
	workers   sync.WaitGroup
	syncKeys  syncSecretProvider
}

// New creates storage directories, opens and bootstraps the store, builds the
// HTTP handler, and starts the optional search sidecar.
func New(cfg config.Config) (*Service, error) {
	if err := profiles.ValidateStartup(context.Background(), cfg); err != nil {
		return nil, err
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		return nil, err
	}
	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		return nil, err
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		_ = st.Close()
		return nil, err
	}
	// Say so when the schema was upgraded. A migration rewrites the user's
	// library, and until H4b it happened with no backup and no notice at all --
	// the only sign was the absence of a complaint. The backup is named here
	// because a user who later needs it has to be able to find it.
	if report, migrated := st.LastMigration(); migrated {
		log.Printf("notriosd migrated this database from schema %d to %d; a verified copy of it as it was is in %s",
			report.FromVersion, report.ToVersion, report.BackupDir)
	}
	// A configured non-none target is the explicit enrollment boundary from
	// G4. Enrollment creates durable local journal state only; transport remains
	// unimplemented and changing an already enrolled profile back to none does
	// not discard history that peers may still need.
	if target := strings.TrimSpace(cfg.Sync.Target); target != "" && target != "none" {
		if _, err := st.EnrollLocalJournal(context.Background(), "profile sync target "+target); err != nil {
			_ = st.Close()
			return nil, err
		}
	}
	handler := httpapi.NewServerWithOptions(httpapi.ServerOptions{Store: st, Config: cfg})
	provider, providerErr := newSyncSecretStore(cfg, st)
	if providerErr == nil {
		handler.AttachSyncSecretStore(provider)
		// Said once at startup rather than on every sync: an installed profile
		// still holding its keys in the development file is a state the user
		// has to act on, and it is not visible anywhere they would otherwise
		// look.
		if file, ok := provider.(*fileSyncSecretStore); ok && file.advisory != "" {
			log.Printf("sync credentials: %s", file.advisory)
		}
	} else {
		// A nil interface rather than a nil pointer inside one: the checks
		// downstream compare against nil, and a typed nil would pass them and
		// then panic on the first call.
		provider = nil
		handler.SetSyncSecretUnavailable(providerErr.Error())
		log.Printf("local sync secret provider is unavailable: %v", providerErr)
	}
	if err := attachSyncSecurity(cfg, st, handler, provider); err != nil {
		_ = st.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	svc := &Service{Config: cfg, Store: st, Handler: handler, ctx: ctx, cancel: cancel, syncKeys: provider}
	if err := svc.startSyncJobs(); err != nil {
		cancel()
		_ = st.Close()
		return nil, err
	}
	if cfg.SearchSidecar.Enabled {
		svc.startSearchSidecar()
	}
	return svc, nil
}

// startSyncJobs wires the one explicitly configured target into G15's durable
// outbox. Missing development key material leaves control unavailable (and is
// reported) while preserving G4's journal boundary; `sync init` is the explicit
// act that makes the target runnable.
func (s *Service) startSyncJobs() error {
	targetType := strings.ToLower(strings.TrimSpace(s.Config.Sync.Target))
	if targetType == "" || targetType == "none" {
		return nil
	}
	status, err := s.Store.JournalStatus(s.ctx)
	if err != nil {
		return err
	}
	identity, err := s.Store.GetDatabaseIdentity(s.ctx)
	if err != nil {
		return err
	}
	if s.syncKeys == nil {
		log.Printf("sync target configured but the local secret provider is unavailable")
		return nil
	}
	keys, err := s.syncKeys.openKeyFile()
	if err != nil {
		log.Printf("sync target configured but durable jobs are unavailable until `notriosctl sync init` creates usable key material: %v", err)
		return nil
	}
	verifier := syncwire.MultiVerifier{keys, s.Store.PeerVerifier()}
	manager := syncjobs.New(s.Store)
	var targetID string
	var builder syncjobs.CarrierBuild

	switch targetType {
	case "directory":
		root := filepath.Clean(strings.TrimSpace(s.Config.Sync.Directory))
		if root == "." || root == "" {
			log.Printf("sync target directory establishes a journal boundary, but durable jobs are unavailable until sync.directory is configured")
			return nil
		}
		targetID = store.SyncTargetID("directory:" + root)
		builder = func(options synccarrier.Options, byteBudget int64) (*synccarrier.Round, *synccarrier.BudgetCarrier, store.ObjectProvider, error) {
			group, currentErr := keys.Current()
			if currentErr != nil {
				return nil, nil, nil, fmt.Errorf("load current sync group key: %w", currentErr)
			}
			carrier, buildErr := synccarrier.NewDirectory(root, group, identity.DatabaseID, status.ReplicaID)
			if buildErr != nil {
				return nil, nil, nil, buildErr
			}
			budget := synccarrier.NewBudgetCarrier(carrier, byteBudget)
			round := synccarrier.NewRound(synccarrier.NewStoreReplica(s.Store), budget, keys, keys, verifier,
				store.NewLocalObjectProvider(s.Store), options)
			return round, budget, synccarrier.NewProvider(budget, keys, verifier, syncwire.Limits{}), nil
		}
	case "rest":
		baseURL := strings.TrimRight(strings.TrimSpace(s.Config.Sync.RESTBaseURL), "/")
		if baseURL == "" {
			log.Printf("sync target rest establishes/serves a journal boundary, but outbound durable jobs are unavailable until sync.rest_base_url is configured")
			return nil
		}
		targetID = store.SyncTargetID("rest:" + baseURL)
		builder = func(options synccarrier.Options, byteBudget int64) (*synccarrier.Round, *synccarrier.BudgetCarrier, store.ObjectProvider, error) {
			group, currentErr := keys.Current()
			if currentErr != nil {
				return nil, nil, nil, fmt.Errorf("load current sync group key: %w", currentErr)
			}
			client := &syncauth.Client{BaseURL: baseURL, DatabaseID: identity.DatabaseID, ReplicaID: status.ReplicaID,
				SignerKeyID: keys.SignerKeyID(), Private: keys.PrivateSigningKey()}
			carrier := syncrest.New(client, group, status.ReplicaID)
			budget := synccarrier.NewBudgetCarrier(carrier, byteBudget)
			round := synccarrier.NewRound(synccarrier.NewStoreReplica(s.Store), budget, keys, keys, verifier,
				store.NewLocalObjectProvider(s.Store), options)
			return round, budget, synccarrier.NewProvider(budget, keys, verifier, syncwire.Limits{}), nil
		}
	default:
		return fmt.Errorf("sync target must be none, directory, or rest")
	}
	target := &syncjobs.CarrierTarget{Store: s.Store, Build: builder, MaterializeLimit: 16}
	if targetType == "rest" {
		target.Catchup = func(ctx context.Context, byteBudget int64, progress syncjobs.Progress) (map[string]any, error) {
			client := &syncauth.Client{BaseURL: strings.TrimRight(strings.TrimSpace(s.Config.Sync.RESTBaseURL), "/"),
				DatabaseID: identity.DatabaseID, ReplicaID: status.ReplicaID,
				SignerKeyID: keys.SignerKeyID(), Private: keys.PrivateSigningKey()}
			return s.runRESTCatchup(ctx, client, keys, byteBudget, progress)
		}
	}
	if err := manager.Register(targetID, target); err != nil {
		return err
	}
	s.SyncJobs = manager
	s.Handler.AttachSyncJobs(manager, targetID)
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		manager.Serve(s.ctx, "service-"+status.ReplicaID)
	}()
	log.Printf("durable sync job worker active for opaque target %s", targetID)
	return nil
}

type fileSyncSecretStore struct {
	path string
	// advisory is set when an installed profile is still on this file only
	// because it already had key material here. It is carried into Warning so
	// that every surface showing the provider also shows what to do about it.
	advisory string
	mu       sync.Mutex
	keys     *synckeys.KeyFile
}

// resolveKeyFilePath is where key material lives, whichever store protects
// it. Both providers use it so that turning on the keychain never also moves
// the file.
func resolveKeyFilePath(cfg config.Config, st *store.SQLiteStore) (string, error) {
	path := strings.TrimSpace(cfg.Sync.REST.KeyFile)
	if path != "" {
		return path, nil
	}
	identity, err := st.GetDatabaseIdentity(context.Background())
	if err != nil {
		return "", err
	}
	return synckeys.DefaultPath(identity.DatabaseID)
}

func (p *fileSyncSecretStore) ProviderName() string { return "locked-file-development" }
func (p *fileSyncSecretStore) Warning() string {
	warning := "Sync keys use an owner-only 0600 development file, not an operating-system keychain. Treat it like the password to this library."
	if p.advisory != "" {
		warning += " " + p.advisory
	}
	return warning
}
func (p *fileSyncSecretStore) openKeyFile() (*synckeys.KeyFile, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.keys != nil {
		return p.keys, nil
	}
	keys, err := synckeys.Open(p.path)
	if err != nil {
		return nil, err
	}
	p.keys = keys
	return keys, nil
}

func (p *fileSyncSecretStore) Open() (httpapi.SyncLocalKeys, error) { return p.openKeyFile() }
func (p *fileSyncSecretStore) Create() (httpapi.SyncLocalKeys, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.keys != nil {
		return p.keys, nil
	}
	keys, err := synckeys.Create(p.path)
	if err != nil {
		return nil, err
	}
	p.keys = keys
	return keys, nil
}

// runRESTCatchup downloads and fully verifies a peer-key-wrapped physical
// snapshot into the profile's private inbox. It never installs it: reset or
// replacement remains a separate explicit destructive review.
func (s *Service) runRESTCatchup(ctx context.Context, client *syncauth.Client, keys *synckeys.KeyFile,
	byteBudget int64, progress syncjobs.Progress) (map[string]any, error) {
	backup, err := syncrest.RequestBackup(ctx, client)
	if err != nil {
		return nil, err
	}
	if backup.SealedBytes > byteBudget {
		return nil, synccarrier.ErrByteBudget
	}
	root := filepath.Join(s.stateRoot(), "catchup-inbox", backup.ID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	sealedPath := filepath.Join(root, "snapshot.nbk")
	for {
		written, complete, err := syncrest.DownloadBackup(ctx, client, backup, sealedPath, int64(syncauth.MaxRangeResponseBytes))
		if err != nil {
			return nil, err
		}
		if err := progress("catchup_transfer", written, backup.SealedBytes, written,
			map[string]any{"last_phase": "catchup_transfer", "received_bytes": written}); err != nil {
			return nil, err
		}
		if complete {
			break
		}
	}
	snapshotDir, report, err := syncrest.OpenBackup(backup, sealedPath, root, keys,
		syncwire.MultiVerifier{keys, s.Store.PeerVerifier()})
	if err != nil {
		return nil, err
	}
	_ = os.Remove(sealedPath)
	if err := progress("catchup_verified", 1, 1, backup.SealedBytes,
		map[string]any{"last_phase": "catchup_verified", "objects": report.Objects}); err != nil {
		return nil, err
	}
	_ = snapshotDir // The path is intentionally kept out of job summaries and API responses.
	return map[string]any{"snapshot_id": report.SnapshotID, "database_id": report.DatabaseID,
		"objects": report.Objects, "sealed_bytes": backup.SealedBytes, "ready_for_review": true}, nil
}

// attachSyncSecurity enables the peer-authenticated sync surface, and refuses
// to start rather than expose it unsafely.
//
// The refusals are the point of this function. A sync surface on a non-loopback
// address without TLS would put a library's traffic on the network in the
// clear; a surface enabled without key material would answer pairing requests
// it cannot complete. Both are configuration mistakes that are invisible until
// something is already exposed, so they are startup failures with an
// explanation rather than warnings in a log nobody reads.
func attachSyncSecurity(cfg config.Config, st *store.SQLiteStore, handler *httpapi.Server, provider syncSecretProvider) error {
	if !cfg.Sync.REST.Enabled {
		return nil
	}
	if err := config.ValidateSyncTransport(cfg); err != nil {
		return err
	}
	identity, err := st.GetDatabaseIdentity(context.Background())
	if err != nil {
		return err
	}
	if provider == nil {
		return errors.New("sync rest is enabled but its secret provider is unavailable")
	}
	keys, err := provider.openKeyFile()
	if err != nil {
		return fmt.Errorf("sync rest is enabled but its key material is unusable: %w", err)
	}
	limits := syncauth.Limits{
		RequestsPerMinute: cfg.Sync.REST.RequestsPerMinute,
		Burst:             cfg.Sync.REST.Burst,
		FailuresPerMinute: cfg.Sync.REST.FailuresPerMinute,
	}
	if err := handler.AttachSyncSecurity(st, keys, limits, nil); err != nil {
		return err
	}
	log.Printf("sync REST surface enabled for database %s (tls=%t)", identity.DatabaseID,
		cfg.Sync.REST.TLSCertFile != "")
	return nil
}

// HTTPServer returns a configured *http.Server for the service handler.
func (s *Service) HTTPServer() *http.Server {
	return &http.Server{
		Addr:              s.Config.Server.ListenAddr,
		Handler:           restrictRemoteRequests(s.Handler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// ListenAndServe is the single serving path for every Notrios executable.
// Configuration validation and transport selection must not be split between
// binaries: accepting TLS material and then serving plaintext is worse than a
// startup refusal because the operator has no visible indication of the loss.
func (s *Service) ListenAndServe() error {
	server := s.HTTPServer()
	certificate, key := s.TLSFiles()
	return explainBindFailure(listenAndServe(server, certificate, key), s.Config.Server.ListenAddr)
}

// explainBindFailure turns "address already in use" into advice.
//
// Two Notrios instances on one machine is a supported arrangement -- a
// development checkout and an installed instance, or several profiles -- and
// they are kept apart by having different databases and different ports.
//
// H4a removed the commonest collision: a checkout now defaults to
// 127.0.0.1:8099 and an installed instance to 127.0.0.1:8080, so those two no
// longer fight. What remains is profiles, which all default to the installed
// address until given their own, and programs that are not Notrios at all --
// 8080 is a popular port.
//
// The advice must not name either default. Recommending 8099 to an installed
// instance, as this message did before H4a, moves it straight onto the port a
// checkout expects to own.
func explainBindFailure(err error, listenAddr string) error {
	if err == nil || !errors.Is(err, syscall.EADDRINUSE) {
		return err
	}
	listenAddr = strings.TrimSpace(listenAddr)
	if listenAddr == "" {
		listenAddr = "the configured address"
	}
	return fmt.Errorf("%w\n\n"+
		"%s is already in use. Another Notrios instance is the usual reason: every\n"+
		"profile defaults to the installed address until it is given its own. A\n"+
		"program that is not Notrios is the other -- 127.0.0.1:8080 is a popular port.\n\n"+
		"A development checkout and an installed instance no longer collide by\n"+
		"themselves: a checkout defaults to 127.0.0.1:8099 and an installed instance\n"+
		"to 127.0.0.1:8080.\n\n"+
		"Give this one a port that is neither, with -addr 127.0.0.1:8081, or set\n"+
		"server.listen_addr in its configuration. `notriosctl profile list` shows the\n"+
		"address each profile will bind.", err, listenAddr)
}

type servingHTTPServer interface {
	ListenAndServe() error
	ListenAndServeTLS(string, string) error
}

func listenAndServe(server servingHTTPServer, certificate, key string) error {
	certificate = strings.TrimSpace(certificate)
	key = strings.TrimSpace(key)
	if (certificate == "") != (key == "") {
		return errors.New("service TLS configuration requires both a certificate and key")
	}
	if certificate != "" {
		return server.ListenAndServeTLS(certificate, key)
	}
	return server.ListenAndServe()
}

// restrictRemoteRequests preserves the v0.7 boundary on a shared listener:
// only peer-authenticated sync routes may be reached from another machine.
// Ordinary REST, MCP, web, and local sync-administration routes remain usable
// through loopback even when the socket is bound to 0.0.0.0 for peer sync.
func restrictRemoteRequests(next http.Handler) http.Handler {
	remotePeer := http.NewServeMux()
	for _, pattern := range []string{
		"POST /api/v1/sync/pair",
		"GET /api/v1/sync/handshake",
		"GET /api/v1/sync/carrier/namespaces",
		"GET /api/v1/sync/carrier/{namespace}/{class}",
		"GET /api/v1/sync/carrier/{namespace}/{class}/{name}",
		"HEAD /api/v1/sync/carrier/{namespace}/{class}/{name}",
		"PUT /api/v1/sync/carrier/{class}/{name}",
		"DELETE /api/v1/sync/carrier/{class}/{name}",
		"POST /api/v1/sync/backups",
		"GET /api/v1/sync/backups/{backup_id}",
		"HEAD /api/v1/sync/backups/{backup_id}",
	} {
		remotePeer.Handle(pattern, next)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if remoteAddressIsLoopback(r.RemoteAddr) && requestHostIsLocal(r.Host) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/sync/") {
			// A finite method/path mux is the remote authority. An unknown
			// sync-looking path must not fall through the application's GET /
			// handler and receive the web UI.
			remotePeer.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"loopback_only","message":"ordinary Notrios APIs are available only on loopback"}}` + "\n"))
	})
}

func requestHostIsLocal(hostport string) bool {
	host := strings.TrimSpace(hostport)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else {
		// A Host header without a port is valid. A colon that failed parsing is
		// neither a hostname nor a safe unbracketed IPv6 representation.
		if strings.Contains(host, ":") {
			return false
		}
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") || strings.EqualFold(host, "wails.localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func remoteAddressIsLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

// TLSFiles returns the certificate and key the service should serve with, or
// two empty strings for plaintext.
func (s *Service) TLSFiles() (string, string) {
	return s.Config.Sync.REST.TLSCertFile, s.Config.Sync.REST.TLSKeyFile
}

// Close stops background work and the store.
func (s *Service) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.workers.Wait()
		s.closeErr = s.Store.Close()
	})
	return s.closeErr
}

// startSearchSidecar wires the optional Recoll sidecar: generated config, an
// initial exact reconciliation plus index pass, merged search results, and a
// background loop that drains bounded outbox batches, periodically reconciles,
// and reindexes. Any
// missing binary degrades to FTS5-only search without failing startup.
func (s *Service) startSearchSidecar() {
	cfg := s.Config
	sidecar := recoll.New(cfg.SearchSidecar.IndexDir, cfg.Data.ProjectionDir, cfg.SearchSidecar.Binary)
	s.Handler.AttachSidecarStatus(sidecar)
	if !sidecar.Available() {
		log.Printf("search sidecar enabled but %q/recollq not found; continuing with FTS5 only", cfg.SearchSidecar.Binary)
		return
	}
	if err := sidecar.EnsureConfig(); err != nil {
		sidecar.RecordError(err)
		log.Printf("search sidecar config generation failed; continuing with FTS5 only: %v", err)
		return
	}
	writer := projection.Writer{Dir: cfg.Data.ProjectionDir}
	if report, err := projection.Reconcile(s.ctx, s.Store, writer, 200); err != nil {
		sidecar.RecordError(err)
		log.Printf("projection reconciliation failed: %v", err)
	} else {
		sidecar.RecordReconciliation(report)
		log.Printf("startup projection reconciliation: canonical=%d missing=%d stale=%d orphaned=%d repaired=%d failed=%d",
			report.Canonical, report.Missing, report.Stale, report.Orphaned, report.Repaired, report.Failed)
	}
	if report, err := projection.DrainOutbox(s.ctx, s.Store, writer, 200, 20); err != nil {
		sidecar.RecordError(err)
		log.Printf("projection outbox sync failed: %v", err)
	} else {
		sidecar.RecordProjection(report)
		s.recordSidecarQueue(sidecar)
	}
	if err := sidecar.Index(s.ctx); err != nil {
		log.Printf("recollindex failed; continuing with FTS5 only: %v", err)
		return
	}
	sidecar.SetActive(true)
	s.Handler.AttachSidecar(sidecar)
	log.Printf("search sidecar active: recoll config %s indexing %s", cfg.SearchSidecar.IndexDir, cfg.Data.ProjectionDir)

	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		syncTicker := time.NewTicker(30 * time.Second)
		reconcileTicker := time.NewTicker(10 * time.Minute)
		defer syncTicker.Stop()
		defer reconcileTicker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-syncTicker.C:
				s.runSidecarPass(sidecar, writer, false)
			case <-reconcileTicker.C:
				s.runSidecarPass(sidecar, writer, true)
			}
		}
	}()
}

func (s *Service) runSidecarPass(sidecar *recoll.Sidecar, writer projection.Writer, reconcile bool) {
	changed := false
	report, err := projection.DrainOutbox(s.ctx, s.Store, writer, 200, 20)
	if err != nil {
		sidecar.RecordError(err)
		log.Printf("projection outbox sync failed: %v", err)
	} else {
		sidecar.RecordProjection(report)
		changed = report.Written > 0 || report.Removed > 0
		if report.Jobs > 0 {
			log.Printf("incremental %s", report)
		}
	}
	s.recordSidecarQueue(sidecar)
	if reconcile {
		reconcileReport, reconcileErr := projection.Reconcile(s.ctx, s.Store, writer, 200)
		if reconcileErr != nil {
			sidecar.RecordError(reconcileErr)
			log.Printf("projection reconciliation failed: %v", reconcileErr)
		} else {
			sidecar.RecordReconciliation(reconcileReport)
			changed = changed || reconcileReport.Repaired > 0
			log.Printf("projection reconciliation: canonical=%d missing=%d stale=%d orphaned=%d repaired=%d failed=%d",
				reconcileReport.Canonical, reconcileReport.Missing, reconcileReport.Stale,
				reconcileReport.Orphaned, reconcileReport.Repaired, reconcileReport.Failed)
		}
	}
	if changed {
		if err := sidecar.Index(s.ctx); err != nil {
			log.Printf("recollindex failed: %v", err)
		}
	}
}

func (s *Service) recordSidecarQueue(sidecar *recoll.Sidecar) {
	queue, err := s.Store.ProjectionQueueStatus(s.ctx)
	if err != nil {
		sidecar.RecordError(err)
		return
	}
	sidecar.RecordQueue(queue.Pending, queue.Failed)
}

// stateRoot is where the catch-up inbox lives: a downloaded peer snapshot is
// state, not the library, and it is backed up before a purge.
func (s *Service) stateRoot() string {
	for _, candidate := range []string{s.Config.Data.StateDir, s.Config.Data.Directory} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return "."
}
