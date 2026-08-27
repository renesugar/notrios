package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/profiles"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/synccatchup"
	"github.com/renesugar/notrios/internal/syncstate"
)

const developmentSecretWarning = "Sync keys use an owner-only 0600 development file, not an operating-system keychain. Treat it like the password to this library."

// SyncLocalKeys is deliberately provider-neutral. Pairing can adopt a group
// key, but neither this interface nor any response can reveal one.
type SyncLocalKeys interface {
	SyncKeys
	AdoptGroupKey(keyID string, epoch uint32, key []byte) error
}

// SyncSecretStore is G16's injectable secret-store seam. v0.7 supplies the
// warned file implementation in service; v0.8 can replace it without changing
// pairing, HTTP, or React code.
type SyncSecretStore interface {
	ProviderName() string
	Warning() string
	Open() (SyncLocalKeys, error)
	Create() (SyncLocalKeys, error)
}

func (s *Server) AttachSyncSecretStore(provider SyncSecretStore) { s.syncSecrets = provider }

func (s *Server) syncUIRoutes() {
	s.mux.HandleFunc("GET /api/v1/sync-ui", s.handleSyncUIStatus)
	s.mux.HandleFunc("POST /api/v1/sync-ui/initialize", s.handleSyncUIInitialize)
	s.mux.HandleFunc("PUT /api/v1/sync-ui/configuration", s.handleSyncUIConfiguration)
	s.mux.HandleFunc("POST /api/v1/sync-ui/discover", s.handleSyncUIDiscover)
	s.mux.HandleFunc("POST /api/v1/sync-ui/invitations", s.handleSyncUIInvitation)
	s.mux.HandleFunc("POST /api/v1/sync-ui/pair", s.handleSyncUIPair)
	s.mux.HandleFunc("POST /api/v1/sync-ui/peers/{replica_id}/snapshot-permission", s.handleSyncUISnapshotPermission)
	s.mux.HandleFunc("POST /api/v1/sync-ui/peers/{replica_id}/retirement-preview", s.handleSyncUIRetirementPreview)
	s.mux.HandleFunc("POST /api/v1/sync-ui/peers/{replica_id}/retire", s.handleSyncUIRetirePeer)
	s.mux.HandleFunc("GET /api/v1/sync-ui/retention", s.handleSyncUIRetention)
	s.mux.HandleFunc("POST /api/v1/sync-ui/recovery", s.handleSyncUIRecovery)
	s.mux.HandleFunc("GET /api/v1/sync-ui/conflicts/{conflict_id}", s.handleSyncUIConflict)
	s.mux.HandleFunc("POST /api/v1/sync-ui/conflicts/{conflict_id}/resolve", s.handleSyncUIConflictResolve)
	s.mux.HandleFunc("POST /api/v1/sync-ui/resources/{resource_id}/intent", s.handleSyncUIResourceIntent)
	s.mux.HandleFunc("POST /api/v1/sync-ui/backups", s.handleSyncUIBackupCreate)
	s.mux.HandleFunc("POST /api/v1/sync-ui/backups/inspect", s.handleSyncUIBackupInspect)
}

func (s *Server) requireLocalSyncUI(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if isLoopbackRequest(r) {
		return true
	}
	// Wails sends in-process AssetServer requests without a socket address. It
	// is local only when the configured listener is itself loopback.
	if strings.TrimSpace(r.RemoteAddr) == "" {
		host, _, err := net.SplitHostPort(s.config.Server.ListenAddr)
		if err == nil && net.ParseIP(host).IsLoopback() {
			return true
		}
	}
	writeError(w, http.StatusForbidden, "loopback_only", "synchronization setup is available only from the local application")
	return false
}

func (s *Server) sqliteStore() (*store.SQLiteStore, bool) {
	st, ok := s.store.(*store.SQLiteStore)
	return st, ok && st != nil
}

type syncUIProfile struct {
	Name          string `json:"name"`
	ProfileID     string `json:"profile_id,omitempty"`
	DatabaseID    string `json:"database_id,omitempty"`
	PublicBaseURL string `json:"public_base_url,omitempty"`
	SyncTarget    string `json:"sync_target"`
	Active        bool   `json:"active"`
}

func (s *Server) syncUIProfiles() []syncUIProfile {
	active := syncUIProfile{Name: s.config.Profile.Name, ProfileID: s.config.Profile.ID,
		PublicBaseURL: s.config.Server.PublicBaseURL, SyncTarget: normalizedSyncTarget(s.config.Sync.Target), Active: true}
	if identity, err := s.store.GetDatabaseIdentity(context.Background()); err == nil {
		active.DatabaseID = identity.DatabaseID
	}
	items := []syncUIProfile{}
	registryPath := strings.TrimSpace(s.config.Profile.RegistryPath)
	if registryPath != "" {
		if registry, err := profiles.Load(registryPath); err == nil {
			for _, profile := range registry.Profiles {
				item := syncUIProfile{Name: profile.Name, ProfileID: profile.ProfileID, DatabaseID: profile.DatabaseID,
					Active: profile.ProfileID != "" && profile.ProfileID == s.config.Profile.ID}
				if profile.ConfigPath != "" {
					if cfg, loadErr := config.Load(profile.ConfigPath); loadErr == nil {
						item.PublicBaseURL, item.SyncTarget = cfg.Server.PublicBaseURL, normalizedSyncTarget(cfg.Sync.Target)
					}
				}
				items = append(items, item)
			}
		}
	}
	if len(items) == 0 {
		if active.Name == "" {
			active.Name = "Default"
		}
		items = append(items, active)
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items
}

func (s *Server) handleSyncUIStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "synchronization setup requires the canonical SQLite store")
		return
	}
	ctx := r.Context()
	identity, err := st.GetDatabaseIdentity(ctx)
	if writeStoreError(w, err, "sync_ui_status_failed") {
		return
	}
	journal, _ := st.JournalStatus(ctx)
	vector, _ := st.SyncStateVector(ctx)
	peerStates, _ := st.ListSyncPeers(ctx)
	peerKeys, _ := st.ListPeerSigningKeys(ctx)
	retention, _ := st.PlanSyncRetention(ctx, s.syncUIRetentionRequest())
	retentionPeers := map[string]store.SyncPeerRetentionStatus{}
	for _, peer := range retention.Peers {
		retentionPeers[peer.ReplicaID] = peer
	}
	acknowledged := map[string]syncstate.Vector{}
	for _, peer := range peerStates {
		acknowledged[peer.Handshake.ReplicaID] = peer.Handshake.StateVector
	}
	peers := make([]map[string]any, 0, len(peerKeys))
	for _, key := range peerKeys {
		behind := int64(0)
		for replicaID, sequence := range vector {
			if sequence > acknowledged[key.ReplicaID][replicaID] {
				behind += sequence - acknowledged[key.ReplicaID][replicaID]
			}
		}
		state := key.Status
		retentionPeer := retentionPeers[key.ReplicaID]
		if retentionPeer.Status == "retired" {
			state = "retired"
		} else if retentionPeer.FullResyncRequired {
			state = "full_resync_required"
		} else if retentionPeer.BeyondHorizon {
			state = "retention_horizon"
		} else if retentionPeer.Warning {
			state = "retention_warning"
		}
		if state == "active" && behind > 0 {
			state = "behind"
		}
		peers = append(peers, map[string]any{
			"replica_id": key.ReplicaID, "status": state, "behind_operations": behind,
			"enrolled_at": key.EnrolledAt, "revoked_at": key.RevokedAt,
			"snapshot_permitted": st.SnapshotSources().PermittedSource(key.ReplicaID),
			"last_acknowledged":  retentionPeer.LastAcknowledged, "warning_at": retentionPeer.WarningAt,
			"horizon_at": retentionPeer.HorizonAt, "full_resync_required": retentionPeer.FullResyncRequired,
		})
	}
	jobs := s.syncUIJobs(ctx)
	conflicts, _ := st.ListSyncConflicts(ctx, 50)
	resources, resourceTruncated, _ := st.ListSyncResourceStatus(ctx, 50)
	repairs, repairTruncated, _ := st.ListSyncRepairEvents(ctx, 50)
	provider := map[string]any{"available": s.syncSecrets != nil, "configured": false,
		"name": "unavailable", "warning": developmentSecretWarning}
	if s.syncSecrets != nil {
		provider["name"] = s.syncSecrets.ProviderName()
		provider["warning"] = s.syncSecrets.Warning()
		if _, openErr := s.syncSecrets.Open(); openErr == nil {
			provider["configured"] = true
		}
	}
	status := "ready"
	if !journal.Enabled || normalizedSyncTarget(s.config.Sync.Target) == profiles.SyncNone {
		status = "setup"
	}
	for _, job := range jobs {
		if job.State == store.JobRunning || job.State == store.JobQueued {
			status = "syncing"
			break
		}
		if job.Error != "" && strings.Contains(strings.ToLower(job.Error), "offline") {
			status = "offline"
		}
	}
	if status == "ready" && len(conflicts.Conflicts) > 0 {
		status = "attention"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": status,
		"active_profile": map[string]any{"name": defaultString(s.config.Profile.Name, "Default"),
			"profile_id": s.config.Profile.ID, "database_id": identity.DatabaseID, "replica_id": identity.ReplicaID},
		"profiles": itemsOrEmpty(s.syncUIProfiles()),
		"configuration": map[string]any{"target": normalizedSyncTarget(s.config.Sync.Target),
			"directory": s.config.Sync.Directory, "rest_base_url": s.config.Sync.RESTBaseURL,
			"rest_inbound_enabled": s.config.Sync.REST.Enabled, "restart_required": false},
		"journal_enabled": journal.Enabled, "secret_store": provider, "peers": peers, "jobs": jobs,
		"conflicts": conflicts.Conflicts, "conflicts_truncated": conflicts.Truncated,
		"resources": resources, "resources_truncated": resourceTruncated,
		"repairs": repairs, "repairs_truncated": repairTruncated,
		"retention": map[string]any{"history_seconds": retention.HistorySeconds, "snapshot_id": retention.SnapshotID,
			"eligible_operations": retention.EligibleOperations, "eligible_tombstones": len(retention.Tombstones),
			"digest": retention.Digest, "repair": retention.Repair},
	})
}

func (s *Server) syncUIRetentionRequest() store.SyncRetentionRequest {
	return store.SyncRetentionRequest{HistoryFor: s.config.Retention.SyncHistoryDuration(),
		WarningBefore: s.config.Retention.SyncPeerWarningDuration()}
}

func (s *Server) handleSyncUIRetention(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "retention requires the canonical SQLite store")
		return
	}
	report, err := st.PlanSyncRetention(r.Context(), s.syncUIRetentionRequest())
	if writeStoreError(w, err, "sync_retention_failed") {
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleSyncUIRetirementPreview(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "peer retirement requires the canonical SQLite store")
		return
	}
	replicaID := strings.TrimSpace(r.PathValue("replica_id"))
	report, err := st.PlanSyncRetention(r.Context(), s.syncUIRetentionRequest())
	if writeStoreError(w, err, "peer_retirement_preview_failed") {
		return
	}
	for _, peer := range report.Peers {
		if peer.ReplicaID != replicaID {
			continue
		}
		writeJSON(w, http.StatusOK, map[string]any{"dry_run": true, "peer": peer,
			"confirmation": "retire-peer:" + replicaID,
			"consequences": []string{"The peer stops holding the retention watermark open.", "Its old credentials cannot re-enroll.", "That device must reset and pair as a new replica."}})
		return
	}
	writeError(w, http.StatusNotFound, "peer_not_found", "no such enrolled peer")
}

// handleSyncUIRetirePeer requires the GUI review flow to echo the exact peer
// identity before it records the signed, destructive retirement decision.
//
//notrios:doc user gui-peer-retirement-confirmation
//notrios:help gui sync-center
//notrios:claim gui-peer-retirement-check go:github.com/renesugar/notrios/internal/httpapi#TestSyncUIPeerRetirementRequiresPreviewAndExactConfirmation
func (s *Server) handleSyncUIRetirePeer(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok || s.syncSecrets == nil {
		writeError(w, http.StatusServiceUnavailable, "sync_keys_unavailable", "peer retirement requires the canonical store and local signing keys")
		return
	}
	replicaID := strings.TrimSpace(r.PathValue("replica_id"))
	var req struct {
		Confirmation string `json:"confirmation"`
		Reason       string `json:"reason"`
	}
	if !decodeBoundedJSON(w, r, 4096, &req) {
		return
	}
	if req.Confirmation != "retire-peer:"+replicaID {
		writeError(w, http.StatusBadRequest, "confirmation_required", "review this peer's retirement preview and confirm its exact identity")
		return
	}
	keys, err := s.syncSecrets.Open()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "sync_keys_unavailable", "local signing keys are unavailable")
		return
	}
	result, err := st.RetireSyncPeer(r.Context(), store.RetireSyncPeerRequest{ReplicaID: replicaID, Reason: req.Reason, Signer: keys})
	if writeStoreError(w, err, "peer_retirement_failed") {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func itemsOrEmpty(items []syncUIProfile) []syncUIProfile {
	if items == nil {
		return []syncUIProfile{}
	}
	return items
}

func (s *Server) syncUIJobs(ctx context.Context) []api.JobStatus {
	jobs := []store.Job{}
	for _, kind := range []string{store.JobKindSyncIncremental, store.JobKindSyncResourceFetch, store.JobKindSyncCatchup, store.JobKindSyncRestorePrep} {
		page, err := s.store.ListJobs(ctx, store.JobListRequest{Kind: kind, Limit: 10})
		if err == nil {
			jobs = append(jobs, page.Jobs...)
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	if len(jobs) > 20 {
		jobs = jobs[:20]
	}
	views := make([]api.JobStatus, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, toAPIJob(job))
	}
	return views
}

func (s *Server) handleSyncUIInitialize(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok || s.syncSecrets == nil {
		writeError(w, http.StatusServiceUnavailable, "sync_setup_unavailable", "local synchronization setup is not available")
		return
	}
	s.syncUIMu.Lock()
	defer s.syncUIMu.Unlock()
	keys, err := s.syncSecrets.Open()
	if err != nil {
		keys, err = s.syncSecrets.Create()
	}
	if err != nil || keys == nil {
		writeError(w, http.StatusInternalServerError, "secret_store_failed", "the local secret store could not be initialized")
		return
	}
	journal, err := st.EnrollLocalJournal(r.Context(), "local sync setup UI")
	if writeStoreError(w, err, "sync_enrollment_failed") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"initialized": true, "replica_id": journal.ReplicaID,
		"provider": s.syncSecrets.ProviderName(), "warning": s.syncSecrets.Warning(), "restart_required": true})
}

type syncUIConfigurationRequest struct {
	Target             string `json:"target"`
	Directory          string `json:"directory,omitempty"`
	RESTBaseURL        string `json:"rest_base_url,omitempty"`
	RESTInboundEnabled *bool  `json:"rest_inbound_enabled,omitempty"`
}

func (s *Server) handleSyncUIConfiguration(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	var req syncUIConfigurationRequest
	if !decodeBoundedJSON(w, r, 16<<10, &req) {
		return
	}
	req.Target = normalizedSyncTarget(req.Target)
	next := s.config
	next.Sync.Target, next.Sync.Directory, next.Sync.RESTBaseURL = req.Target, "", ""
	if req.RESTInboundEnabled != nil {
		next.Sync.REST.Enabled = *req.RESTInboundEnabled
	}
	switch req.Target {
	case profiles.SyncNone:
	case profiles.SyncDirectory:
		path := filepath.Clean(strings.TrimSpace(req.Directory))
		if !filepath.IsAbs(path) {
			writeError(w, http.StatusBadRequest, "validation_failed", "the shared directory must be an absolute path selected on this device")
			return
		}
		next.Sync.Directory = path
	case profiles.SyncREST:
		peerURL, err := validatePeerURL(req.RESTBaseURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		next.Sync.RESTBaseURL = peerURL
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", "sync target must be none, directory, or rest")
		return
	}
	if strings.TrimSpace(next.ConfigPath) == "" {
		writeError(w, http.StatusConflict, "profile_config_required", "this process was not started from a writable runtime profile")
		return
	}
	if err := config.WriteProfileFile(next.ConfigPath, next); err != nil {
		writeError(w, http.StatusInternalServerError, "configuration_failed", "the active profile configuration could not be updated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"target": req.Target, "restart_required": true,
		"message": "Restart this profile to activate the changed transport."})
}

func (s *Server) handleSyncUIDiscover(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	if s.syncJobs == nil || s.syncTargetID == "" {
		writeError(w, http.StatusServiceUnavailable, "sync_target_unavailable", "restart this profile with a configured sync target before discovery")
		return
	}
	result, err := s.syncJobs.Discover(r.Context(), s.syncTargetID)
	if writeStoreError(w, err, "sync_discovery_failed") {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type syncUIInvitationRequest struct {
	Label      string `json:"label,omitempty"`
	TTLMinutes int    `json:"ttl_minutes,omitempty"`
}

func (s *Server) handleSyncUIInvitation(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	if !s.SyncEnabled() || s.sync == nil {
		writeError(w, http.StatusServiceUnavailable, "pairing_unavailable", "enable the inbound REST sync surface and restart before inviting a peer")
		return
	}
	var req syncUIInvitationRequest
	if !decodeBoundedJSON(w, r, 8<<10, &req) {
		return
	}
	if req.TTLMinutes == 0 {
		req.TTLMinutes = 15
	}
	if req.TTLMinutes < 5 || req.TTLMinutes > 60 || len(req.Label) > 128 {
		writeError(w, http.StatusBadRequest, "validation_failed", "an invitation lasts 5–60 minutes and its label is at most 128 characters")
		return
	}
	secret, err := syncauth.NewPairingSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pairing_failed", "a pairing code could not be generated")
		return
	}
	invitation, err := s.sync.store.CreatePairingInvitation(r.Context(), secret, time.Duration(req.TTLMinutes)*time.Minute, req.Label)
	if writeStoreError(w, err, "pairing_failed") {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invitation_id": invitation.ID,
		"code": syncauth.FormatCode(secret), "expires_at": invitation.ExpiresAt, "single_use": true})
}

type syncUIPairRequest struct {
	BaseURL string `json:"base_url"`
	Code    string `json:"code"`
}

func (s *Server) handleSyncUIPair(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok || s.syncSecrets == nil {
		writeError(w, http.StatusServiceUnavailable, "pairing_unavailable", "initialize synchronization before pairing")
		return
	}
	var req syncUIPairRequest
	if !decodeBoundedJSON(w, r, 16<<10, &req) {
		return
	}
	peerURL, err := validatePeerURL(req.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	secret, err := syncauth.ParseCode(req.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "the pairing code is not complete")
		return
	}
	s.syncUIMu.Lock()
	defer s.syncUIMu.Unlock()
	keys, err := s.syncSecrets.Open()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "pairing_unavailable", "initialize the local secret store before pairing")
		return
	}
	journal, err := st.JournalStatus(r.Context())
	if err != nil || !journal.Enabled {
		writeError(w, http.StatusConflict, "pairing_unavailable", "initialize synchronization before pairing")
		return
	}
	identity, err := st.GetDatabaseIdentity(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pairing_failed", "the local library identity is unavailable")
		return
	}
	request := syncauth.PairingRequest{InvitationID: store.InvitationID(secret), ReplicaID: journal.ReplicaID,
		PublicKey: syncauth.EncodePublicKey(keys.PublicSigningKey()), SchemaVerion: store.CurrentSchemaVersion}
	request.Proof, err = syncauth.ProveRequest(secret, request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pairing_failed", "the pairing request could not be proved")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	status, body, err := syncauth.Pair(ctx, &http.Client{Timeout: 30 * time.Second}, peerURL, req.Code, request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "pairing_offline", "the peer could not be reached")
		return
	}
	if status != http.StatusOK {
		writeError(w, http.StatusBadGateway, "pairing_refused", "the peer refused that address or pairing code")
		return
	}
	var response syncauth.PairingResponse
	if err := json.Unmarshal(body, &response); err != nil || syncauth.VerifyResponseProof(secret, response) != nil {
		writeError(w, http.StatusBadGateway, "pairing_refused", "the peer's response did not prove possession of the code")
		return
	}
	if response.DatabaseID != identity.DatabaseID {
		writeError(w, http.StatusConflict, "wrong_database", "that peer belongs to a different library; restore/adopt that library before pairing")
		return
	}
	group, err := syncauth.UnwrapGroupKey(secret, response.WrappedGroup)
	if err != nil || keys.AdoptGroupKey(response.KeyID, response.Epoch, group) != nil {
		writeError(w, http.StatusConflict, "pairing_conflict", "the local secret store already belongs to a different synchronization group")
		return
	}
	public, err := syncauth.DecodePublicKey(response.PublicKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, "pairing_refused", "the peer returned an invalid signing identity")
		return
	}
	if _, err := st.EnrollPeerSigningKey(r.Context(), response.ReplicaID, public, "paired from local UI"); err != nil {
		writeStoreError(w, err, "pairing_conflict")
		return
	}
	schemaVersion := response.SchemaVersion
	if schemaVersion == 0 {
		schemaVersion = store.CurrentSchemaVersion
	}
	if err := st.ConfigureSyncAdmissionPeer(r.Context(), syncstate.NewHandshake(identity.DatabaseID,
		response.ReplicaID, schemaVersion, syncstate.Vector{response.ReplicaID: 0})); err != nil {
		writeStoreError(w, err, "pairing_conflict")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"paired_with": response.ReplicaID,
		"database_id": response.DatabaseID, "public_base_url": response.PublicBaseURL})
}

func validatePeerURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("the REST peer address must be an absolute URL without credentials, query, or fragment")
	}
	host := parsed.Hostname()
	loopback := host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return "", errors.New("REST peer addresses require HTTPS except on loopback")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (s *Server) handleSyncUISnapshotPermission(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "sync_setup_unavailable", "snapshot permission requires the canonical SQLite store")
		return
	}
	var req struct {
		Permitted bool `json:"permitted"`
	}
	if !decodeBoundedJSON(w, r, 4<<10, &req) {
		return
	}
	replicaID := r.PathValue("replica_id")
	if err := st.PermitSnapshotSource(r.Context(), replicaID, req.Permitted); err != nil {
		writeStoreError(w, err, "snapshot_permission_failed")
		return
	}
	// Read the effective policy back: a row never grants an unknown, retired,
	// or revoked replica permission to receive a complete library snapshot.
	effective := st.SnapshotSources().PermittedSource(replicaID)
	if req.Permitted && !effective {
		writeError(w, http.StatusConflict, "peer_not_active", "only an active enrolled peer can receive catch-up snapshots")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"replica_id": replicaID, "snapshot_permitted": effective})
}

func (s *Server) handleSyncUIRecovery(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	if s.syncJobs == nil || s.syncTargetID == "" {
		writeError(w, http.StatusServiceUnavailable, "sync_target_unavailable", "restart this profile with a runnable sync target before requesting catch-up")
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if !decodeBoundedJSON(w, r, 4<<10, &req) {
		return
	}
	if req.Action != "catchup" && req.Action != "reset" {
		writeError(w, http.StatusBadRequest, "validation_failed", "recovery action must be catchup or reset")
		return
	}
	job, err := s.syncJobs.Start(r.Context(), store.CreateSyncJobRequest{Kind: store.JobKindSyncCatchup,
		Actor: store.SyncJobActorREST, TargetID: s.syncTargetID, ByteBudget: store.MaxSyncJobByteBudget})
	if writeStoreError(w, err, "catchup_start_failed") {
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": toAPIJob(job.Job), "requested_action": req.Action,
		"destructive_review_required": req.Action == "reset"})
}

func (s *Server) handleSyncUIConflict(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	detail, err := s.store.GetSyncConflictDetail(r.Context(), r.PathValue("conflict_id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such synchronization conflict")
		return
	}
	if writeStoreError(w, err, "conflict_read_failed") {
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleSyncUIConflictResolve(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	var req struct {
		Title string `json:"title,omitempty"`
		Body  string `json:"body"`
	}
	if !decodeBoundedJSON(w, r, int64(syncbodyRequestLimit()), &req) {
		return
	}
	document, err := s.store.ResolveSyncConflict(r.Context(), store.ResolveSyncConflictRequest{
		ConflictID: r.PathValue("conflict_id"), Title: req.Title, Body: req.Body})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "that conflict is no longer current")
		return
	}
	if writeStoreError(w, err, "conflict_resolution_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIDocument(document))
}

func syncbodyRequestLimit() int { return (1 << 20) + (32 << 10) }

func (s *Server) handleSyncUIResourceIntent(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	var req struct {
		Pinned    bool `json:"pinned"`
		Requested bool `json:"requested"`
	}
	if !decodeBoundedJSON(w, r, 4<<10, &req) {
		return
	}
	if err := s.store.SetSyncResourceIntent(r.Context(), r.PathValue("resource_id"), req.Pinned, req.Requested); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no such resource")
			return
		}
		writeStoreError(w, err, "resource_intent_failed")
		return
	}
	var job any
	if req.Requested && s.syncJobs != nil && s.syncTargetID != "" {
		started, err := s.syncJobs.Start(r.Context(), store.CreateSyncJobRequest{Kind: store.JobKindSyncResourceFetch,
			Actor: store.SyncJobActorREST, TargetID: s.syncTargetID, ByteBudget: store.DefaultSyncJobByteBudget})
		if err == nil {
			job = toAPIJob(started.Job)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"resource_id": r.PathValue("resource_id"),
		"pinned": req.Pinned, "requested": req.Requested, "job": job})
}

type syncUIPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleSyncUIBackupCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	st, ok := s.sqliteStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "backup_unavailable", "password backup requires the canonical SQLite store")
		return
	}
	var req syncUIPasswordRequest
	if !decodeBoundedJSON(w, r, 8<<10, &req) {
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 4096 {
		writeError(w, http.StatusBadRequest, "validation_failed", "use a backup password between 8 and 4096 characters")
		return
	}
	base := filepath.Join(s.config.Data.Directory, "backup-staging")
	if err := os.MkdirAll(base, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "backup staging could not be prepared")
		return
	}
	workspace, err := os.MkdirTemp(base, "portable-")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "backup staging could not be prepared")
		return
	}
	defer os.RemoveAll(workspace)
	report, err := syncbackup.CreatePortable(r.Context(), st, st.AssetRoot(), workspace, req.Password)
	req.Password = ""
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "the verified password backup could not be created")
		return
	}
	file, err := os.Open(report.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "the completed backup could not be opened")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/vnd.notrios.password-backup")
	w.Header().Set("Content-Disposition", `attachment; filename="notrios-backup.npb"`)
	w.Header().Set("Content-Length", fmt.Sprint(report.Bytes))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, file)
}

func (s *Server) handleSyncUIBackupInspect(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocalSyncUI(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (17<<30)+(1<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "provide one bounded backup file and password")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	password := r.FormValue("password")
	file, _, err := r.FormFile("backup")
	if err != nil || password == "" || len(password) > 4096 {
		writeError(w, http.StatusBadRequest, "validation_failed", "provide the backup file and its password")
		return
	}
	defer file.Close()
	base := filepath.Join(s.config.Data.Directory, "restore-review")
	if err := os.MkdirAll(base, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "backup_review_failed", "restore review staging could not be prepared")
		return
	}
	workspace, err := os.MkdirTemp(base, "review-")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_review_failed", "restore review staging could not be prepared")
		return
	}
	defer os.RemoveAll(workspace)
	inputPath := filepath.Join(workspace, "uploaded.npb")
	output, err := os.OpenFile(inputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_, err = io.Copy(output, file)
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "backup_review_failed", "the uploaded backup could not be staged")
		return
	}
	reviewDir := filepath.Join(workspace, "opened")
	report, err := syncbackup.InspectPortable(r.Context(), inputPath, reviewDir, password)
	password = ""
	if errors.Is(err, synccatchup.ErrBadPassword) {
		writeError(w, http.StatusUnauthorized, "wrong_backup_password", "That password did not open this backup. Nothing was changed; retry or cancel.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "backup_invalid", "the backup did not pass authenticated snapshot verification")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true, "database_id": report.Verify.DatabaseID,
		"snapshot_id": report.Verify.SnapshotID, "schema_version": report.Verify.SchemaVersion,
		"objects": report.Verify.Objects, "database_bytes": report.Verify.DatabaseBytes,
		"external_bytes": report.Verify.ExternalBytes, "ready_for_destructive_review": true,
		"applied": false, "next": "Choose replace or adopt only after reviewing the active profile and emergency-backup requirement."})
}

func decodeBoundedJSON(w http.ResponseWriter, r *http.Request, limit int64, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "request must be bounded JSON with only documented fields")
		return false
	}
	return true
}

func normalizedSyncTarget(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case profiles.SyncDirectory:
		return profiles.SyncDirectory
	case profiles.SyncREST:
		return profiles.SyncREST
	default:
		return profiles.SyncNone
	}
}
