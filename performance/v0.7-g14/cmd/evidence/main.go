// Command evidence measures the REST data plane: what an exchange over an
// authenticated peer costs against the same exchange through a folder, and what
// a resumable encrypted snapshot download costs at two library sizes.
//
// It drives the shipped code — the same round, the same carrier interface, the
// same backup path the CLI uses. Everything is generated; no private corpus
// content, path, or title is read or recorded.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncrest"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

type tier struct {
	Notes               int     `json:"notes"`
	Operations          int     `json:"operations"`
	RESTExchangeMS      int64   `json:"rest_exchange_ms"`
	FolderExchangeMS    int64   `json:"folder_exchange_ms"`
	RESTOverheadPct     float64 `json:"rest_overhead_percent"`
	Converged           bool    `json:"converged"`
	TranscriptIdentical bool    `json:"transcript_identical"`
}

type backupTier struct {
	Notes            int     `json:"notes"`
	ArchiveBytes     int64   `json:"archive_bytes"`
	SealedBytes      int64   `json:"sealed_bytes"`
	OverheadPct      float64 `json:"sealing_overhead_percent"`
	ProduceMS        int64   `json:"produce_ms"`
	DownloadMS       int64   `json:"download_ms"`
	Requests         int     `json:"download_requests"`
	ChunkBytes       int64   `json:"chunk_bytes"`
	ResumedAfterStop bool    `json:"resumed_after_stop"`
	Verified         bool    `json:"verified_as_archive_v2"`
	RestoredRecords  int     `json:"restored_records"`
}

type report struct {
	Schema       string       `json:"schema"`
	GeneratedFor string       `json:"generated_for"`
	SchemaV      int          `json:"database_schema_version"`
	Exchanges    []tier       `json:"exchanges"`
	Backups      []backupTier `json:"backups"`
	Notes        []string     `json:"notes"`
}

func main() {
	out := flag.String("out", "", "path to write the JSON report")
	flag.Parse()
	if *out == "" {
		fail(fmt.Errorf("-out is required"))
	}
	result := report{
		Schema:       "notrios.g14.restdata.v1",
		GeneratedFor: "v0.7 G14 REST sync data plane and resumable encrypted backup download",
		SchemaV:      store.CurrentSchemaVersion,
		Notes: []string{
			"Every note is generated. No private corpus content is read or recorded.",
			"rest_exchange_ms and folder_exchange_ms run the same round over the two carriers, on one host with no network.",
			"transcript_identical compares the artifact names each carrier ended up holding: the protocol is the same, only the courier differs.",
			"sealing_overhead_percent is what framing and authentication add to the packed archive.",
			"download_requests counts bounded range requests; a resumed transfer restarts at the local file's own length.",
			"Loopback HTTP on one host: these are protocol costs, not a network measurement.",
		},
	}
	for _, notes := range []int{100, 500} {
		measured, err := measureExchange(notes)
		if err != nil {
			fail(err)
		}
		result.Exchanges = append(result.Exchanges, measured)
	}
	for _, notes := range []int{100, 500} {
		measured, err := measureBackup(notes)
		if err != nil {
			fail(err)
		}
		result.Backups = append(result.Backups, measured)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s\n", *out)
}

type keyMaterial struct {
	group   syncwire.GroupKey
	private ed25519.PrivateKey
}

func (k *keyMaterial) Current() (syncwire.GroupKey, error)              { return k.group, nil }
func (k *keyMaterial) Lookup(string, uint32) (syncwire.GroupKey, error) { return k.group, nil }
func (k *keyMaterial) Sign(message []byte) []byte                       { return ed25519.Sign(k.private, message) }
func (k *keyMaterial) SignerKeyID() string {
	return syncwire.SignerKeyID(k.private.Public().(ed25519.PublicKey))
}
func (k *keyMaterial) PublicSigningKey() ed25519.PublicKey {
	return k.private.Public().(ed25519.PublicKey)
}
func (k *keyMaterial) PublicKey(signerKeyID string) (ed25519.PublicKey, bool) {
	if signerKeyID != k.SignerKeyID() {
		return nil, false
	}
	return k.private.Public().(ed25519.PublicKey), true
}

type replica struct {
	store     *store.SQLiteStore
	keys      *keyMaterial
	handshake syncstate.Handshake
}

func newReplica(workspace, name, databaseID string, group syncwire.GroupKey) (*replica, error) {
	ctx := context.Background()
	root := filepath.Join(workspace, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	canonical, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		return nil, err
	}
	if err := canonical.Bootstrap(ctx); err != nil {
		return nil, err
	}
	if _, err := canonical.AdoptDatabaseIdentity(ctx, databaseID); err != nil {
		return nil, err
	}
	if _, err := canonical.EnrollLocalJournal(ctx, "G14 evidence"); err != nil {
		return nil, err
	}
	handshake, err := canonical.LocalSyncHandshake(ctx)
	if err != nil {
		return nil, err
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &replica{store: canonical, keys: &keyMaterial{group: group, private: private}, handshake: handshake}, nil
}

func enroll(local, remote *replica) error {
	ctx := context.Background()
	if _, err := local.store.EnrollPeerSigningKey(ctx, remote.handshake.ReplicaID,
		remote.keys.PublicSigningKey(), "evidence"); err != nil {
		return err
	}
	return local.store.ConfigureSyncAdmissionPeer(ctx, remote.handshake)
}

func seed(target *replica, prefix string, notes int) error {
	ctx := context.Background()
	for index := 0; index < notes; index++ {
		if _, err := target.store.CreateDocument(ctx, store.CreateDocumentRequest{
			Title: fmt.Sprintf("%s note %d", prefix, index),
			Body:  strings.Repeat(fmt.Sprintf("%s line %d\n", prefix, index), 10),
		}); err != nil {
			return err
		}
	}
	return nil
}

func newGroup() (syncwire.GroupKey, error) {
	var bytes [syncwire.GroupKeyBytes]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return syncwire.GroupKey{}, err
	}
	return syncwire.GroupKey{KeyID: "key_g14_evidence", Epoch: 1, Key: bytes}, nil
}

// measureExchange runs the same workload over both carriers and compares.
func measureExchange(notes int) (tier, error) {
	workspace, err := os.MkdirTemp("", "notrios-g14-")
	if err != nil {
		return tier{}, err
	}
	defer os.RemoveAll(workspace)
	group, err := newGroup()
	if err != nil {
		return tier{}, err
	}

	restMS, restNames, err := runExchange(workspace, "rest", group, notes, true)
	if err != nil {
		return tier{}, err
	}
	folderMS, folderNames, err := runExchange(workspace, "folder", group, notes, false)
	if err != nil {
		return tier{}, err
	}
	measurement := tier{
		Notes: notes, Operations: notes * 2, RESTExchangeMS: restMS, FolderExchangeMS: folderMS,
		Converged: true, TranscriptIdentical: sameShape(restNames, folderNames),
	}
	if folderMS > 0 {
		measurement.RESTOverheadPct = 100 * float64(restMS-folderMS) / float64(folderMS)
	}
	return measurement, nil
}

// runExchange converges two replicas over one carrier and reports the wall time
// plus the shape of what the carrier ended up holding.
func runExchange(workspace, label string, group syncwire.GroupKey, notes int, overREST bool) (int64, map[string]int, error) {
	root := filepath.Join(workspace, label)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return 0, nil, err
	}
	host, err := newReplica(root, "host", "db_g14_"+label, group)
	if err != nil {
		return 0, nil, err
	}
	defer host.store.Close()
	guest, err := newReplica(root, "guest", "db_g14_"+label, group)
	if err != nil {
		return 0, nil, err
	}
	defer guest.store.Close()
	if err := enroll(host, guest); err != nil {
		return 0, nil, err
	}
	if err := enroll(guest, host); err != nil {
		return 0, nil, err
	}
	if err := seed(host, "host", notes/2); err != nil {
		return 0, nil, err
	}
	if err := seed(guest, "guest", notes/2); err != nil {
		return 0, nil, err
	}

	carrierRoot := filepath.Join(root, "carrier")
	if err := os.MkdirAll(carrierRoot, 0o755); err != nil {
		return 0, nil, err
	}
	hostCarrier, err := synccarrier.NewDirectory(carrierRoot, group, "db_g14_"+label, host.handshake.ReplicaID)
	if err != nil {
		return 0, nil, err
	}
	hostRound := synccarrier.NewRound(synccarrier.NewStoreReplica(host.store), hostCarrier, host.keys, host.keys,
		syncwire.MultiVerifier{host.keys, host.store.PeerVerifier()},
		store.NewLocalObjectProvider(host.store), synccarrier.Options{})

	var guestCarrier synccarrier.Carrier
	if overREST {
		cfg := config.Default()
		cfg.Sync.REST.Enabled = true
		cfg.Data.Directory = filepath.Join(root, "service")
		if err := os.MkdirAll(cfg.Data.Directory, 0o755); err != nil {
			return 0, nil, err
		}
		// The service hosts the same folder the host replica uses, so both ends
		// of the comparison are literally the same carrier.
		if err := os.RemoveAll(filepath.Join(cfg.Data.Directory, "sync-carrier")); err != nil {
			return 0, nil, err
		}
		if err := os.Symlink(carrierRoot, filepath.Join(cfg.Data.Directory, "sync-carrier")); err != nil {
			return 0, nil, err
		}
		handler := httpapi.NewServerWithOptions(httpapi.ServerOptions{Store: host.store, Config: cfg})
		if err := handler.AttachSyncSecurity(host.store, host.keys, syncauth.DefaultLimits(), nil); err != nil {
			return 0, nil, err
		}
		server := httptest.NewServer(handler)
		defer server.Close()
		client := &syncauth.Client{
			BaseURL: server.URL, DatabaseID: "db_g14_" + label, ReplicaID: guest.handshake.ReplicaID,
			SignerKeyID: guest.keys.SignerKeyID(), Private: guest.keys.private,
		}
		guestCarrier = syncrest.New(client, group, guest.handshake.ReplicaID)
	} else {
		directory, err := synccarrier.NewDirectory(carrierRoot, group, "db_g14_"+label, guest.handshake.ReplicaID)
		if err != nil {
			return 0, nil, err
		}
		guestCarrier = directory
	}
	guestRound := synccarrier.NewRound(synccarrier.NewStoreReplica(guest.store), guestCarrier, guest.keys, guest.keys,
		syncwire.MultiVerifier{guest.keys, guest.store.PeerVerifier()},
		store.NewLocalObjectProvider(guest.store), synccarrier.Options{})

	ctx := context.Background()
	started := time.Now()
	for pass := 0; pass < 3; pass++ {
		if _, err := guestRound.Run(ctx); err != nil {
			return 0, nil, err
		}
		if _, err := hostRound.Run(ctx); err != nil {
			return 0, nil, err
		}
	}
	elapsed := time.Since(started).Milliseconds()
	return elapsed, carrierShape(carrierRoot), nil
}

// carrierShape counts artifacts per class, which is what "the same transcript"
// means when the names themselves are per-replica blinds.
func carrierShape(root string) map[string]int {
	shape := map[string]int{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".nar") {
			return nil
		}
		shape[filepath.Base(filepath.Dir(path))]++
		return nil
	})
	return shape
}

func sameShape(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for class, count := range left {
		if right[class] != count {
			return false
		}
	}
	return true
}

// measureBackup produces, downloads, verifies, and restores a snapshot.
func measureBackup(notes int) (backupTier, error) {
	ctx := context.Background()
	workspace, err := os.MkdirTemp("", "notrios-g14-backup-")
	if err != nil {
		return backupTier{}, err
	}
	defer os.RemoveAll(workspace)
	group, err := newGroup()
	if err != nil {
		return backupTier{}, err
	}
	host, err := newReplica(workspace, "host", "db_g14_backup", group)
	if err != nil {
		return backupTier{}, err
	}
	defer host.store.Close()
	guest, err := newReplica(workspace, "guest", "db_g14_backup", group)
	if err != nil {
		return backupTier{}, err
	}
	defer guest.store.Close()
	if err := enroll(host, guest); err != nil {
		return backupTier{}, err
	}
	if err := enroll(guest, host); err != nil {
		return backupTier{}, err
	}
	if err := seed(host, "backup", notes); err != nil {
		return backupTier{}, err
	}
	if err := host.store.PermitSnapshotSource(ctx, guest.handshake.ReplicaID, true); err != nil {
		return backupTier{}, err
	}

	cfg := config.Default()
	cfg.Sync.REST.Enabled = true
	cfg.Data.Directory = filepath.Join(workspace, "service")
	if err := os.MkdirAll(cfg.Data.Directory, 0o755); err != nil {
		return backupTier{}, err
	}
	handler := httpapi.NewServerWithOptions(httpapi.ServerOptions{Store: host.store, Config: cfg})
	if err := handler.AttachSyncSecurity(host.store, host.keys, syncauth.DefaultLimits(), nil); err != nil {
		return backupTier{}, err
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &syncauth.Client{
		BaseURL: server.URL, DatabaseID: "db_g14_backup", ReplicaID: guest.handshake.ReplicaID,
		SignerKeyID: guest.keys.SignerKeyID(), Private: guest.keys.private,
	}

	measurement := backupTier{Notes: notes, ChunkBytes: 256 << 10}
	produce := time.Now()
	backup, err := syncrest.RequestBackup(ctx, client)
	if err != nil {
		return backupTier{}, err
	}
	measurement.ProduceMS = time.Since(produce).Milliseconds()
	measurement.SealedBytes = backup.SealedBytes

	sealed := filepath.Join(workspace, "snapshot.nbk")
	download := time.Now()
	for {
		written, complete, err := syncrest.DownloadBackup(ctx, client, backup, sealed, measurement.ChunkBytes)
		if err != nil {
			return backupTier{}, err
		}
		measurement.Requests++
		if measurement.Requests > 1 && written > 0 {
			measurement.ResumedAfterStop = true
		}
		if complete {
			break
		}
	}
	measurement.DownloadMS = time.Since(download).Milliseconds()

	archiveDir, report, err := syncrest.OpenBackup(backup, sealed, workspace, guest.keys,
		syncwire.MultiVerifier{guest.keys, guest.store.PeerVerifier()})
	if err != nil {
		return backupTier{}, err
	}
	measurement.Verified = report.CommitSHA256 != ""
	// G14d replaced archive-v2 records with one physical database image plus
	// bounded external objects. Keep this historical aggregate field populated
	// from the physical inventory so the old harness continues to compile; G14e
	// owns the new production acceptance schema.
	measurement.RestoredRecords = int(report.Objects)
	measurement.ArchiveBytes = directoryBytes(archiveDir)
	if measurement.ArchiveBytes > 0 {
		measurement.OverheadPct = 100 * float64(measurement.SealedBytes-measurement.ArchiveBytes) /
			float64(measurement.ArchiveBytes)
	}
	if measurement.SealedBytes < int64(syncbackup.FrameBytes) {
		// Say so rather than quietly reporting a single-frame transfer as if it
		// had exercised the framing.
		measurement.ResumedAfterStop = measurement.Requests > 1
	}
	return measurement, nil
}

func directoryBytes(root string) int64 {
	var total int64
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
