// Command g14e-tool exercises the production full-scale REST and directory
// snapshot paths without exposing private corpus details in its JSON report.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncrest"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

type keyMaterial struct {
	group   syncwire.GroupKey
	private ed25519.PrivateKey
}

func (k *keyMaterial) Current() (syncwire.GroupKey, error) { return k.group, nil }
func (k *keyMaterial) Lookup(string, uint32) (syncwire.GroupKey, error) {
	return k.group, nil
}
func (k *keyMaterial) Sign(message []byte) []byte { return ed25519.Sign(k.private, message) }
func (k *keyMaterial) SignerKeyID() string {
	return syncwire.SignerKeyID(k.private.Public().(ed25519.PublicKey))
}
func (k *keyMaterial) PublicSigningKey() ed25519.PublicKey {
	return k.private.Public().(ed25519.PublicKey)
}
func (k *keyMaterial) PublicKey(id string) (ed25519.PublicKey, bool) {
	if id != k.SignerKeyID() {
		return nil, false
	}
	return k.PublicSigningKey(), true
}

type report struct {
	SealedBytes            int64  `json:"sealed_bytes"`
	RESTDownloadCalls      int    `json:"rest_download_calls"`
	DirectoryPublishCalls  int    `json:"directory_publish_calls"`
	DirectoryDownloadCalls int    `json:"directory_download_calls"`
	RESTVerified           bool   `json:"rest_verified"`
	DirectoryVerified      bool   `json:"directory_verified"`
	RestoreStage           string `json:"restore_stage"`
	ReplicaRotated         bool   `json:"replica_rotated"`
	DerivedDocuments       int64  `json:"derived_documents"`
	PostSnapshotConverged  bool   `json:"post_snapshot_converged"`
}

func main() {
	if len(os.Args) != 5 || os.Args[1] != "catchup" {
		fatal("usage: g14e-tool catchup <host-db> <host-assets> <workspace>")
	}
	value, err := catchup(os.Args[2], os.Args[3], os.Args[4])
	if err != nil {
		fatal(err.Error())
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fatal(err.Error())
	}
}

func catchup(hostDB, hostAssets, workspace string) (report, error) {
	ctx := context.Background()
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return report{}, err
	}
	host, err := store.OpenSQLiteWithAssetStore(hostDB, hostAssets)
	if err != nil {
		return report{}, err
	}
	defer host.Close()
	if err := host.Bootstrap(ctx); err != nil {
		return report{}, err
	}
	if _, err := host.EnrollLocalJournal(ctx, "G14e full-scale host"); err != nil {
		return report{}, err
	}
	hostHandshake, err := host.LocalSyncHandshake(ctx)
	if err != nil {
		return report{}, err
	}

	group, err := newGroupKey()
	if err != nil {
		return report{}, err
	}
	hostKeys, err := newKeys(group)
	if err != nil {
		return report{}, err
	}
	requesterRoot := filepath.Join(workspace, "requester")
	requester, requesterHandshake, requesterKeys, err := newReplica(ctx, requesterRoot, hostHandshake.DatabaseID, group)
	if err != nil {
		return report{}, err
	}
	defer requester.Close()
	if err := pair(ctx, host, hostHandshake, hostKeys, requester, requesterHandshake, requesterKeys); err != nil {
		return report{}, err
	}
	if err := host.PermitSnapshotSource(ctx, requesterHandshake.ReplicaID, true); err != nil {
		return report{}, err
	}

	serverRoot := filepath.Join(workspace, "server")
	cfg := config.Default()
	cfg.Sync.REST.Enabled = true
	cfg.Data.Directory = serverRoot
	handler := httpapi.NewServerWithOptions(httpapi.ServerOptions{Store: host, Config: cfg})
	if err := handler.AttachSyncSecurity(host, hostKeys, syncauth.DefaultLimits(), nil); err != nil {
		return report{}, err
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &syncauth.Client{BaseURL: server.URL, DatabaseID: hostHandshake.DatabaseID,
		ReplicaID: requesterHandshake.ReplicaID, SignerKeyID: requesterKeys.SignerKeyID(),
		Private: requesterKeys.private}
	// Put the convergence fixture in the physical image, then edit its body
	// after the snapshot boundary. This proves incremental replay without
	// making a new metadata record the scale variable under test.
	seed, err := host.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Generated G14e convergence evidence", Body: "generated snapshot baseline\n",
	})
	if err != nil {
		return report{}, err
	}

	backup, err := syncrest.RequestBackup(ctx, client)
	if err != nil {
		return report{}, err
	}
	sealed := filepath.Join(workspace, "rest-download.nbk")
	restCalls, err := downloadREST(ctx, client, backup, sealed)
	if err != nil {
		return report{}, err
	}
	restOpen := filepath.Join(workspace, "rest-open")
	if err := os.MkdirAll(restOpen, 0o700); err != nil {
		return report{}, err
	}
	snapshotRoot, verified, err := syncrest.OpenBackup(backup, sealed, restOpen, requesterKeys,
		syncwire.MultiVerifier{requesterKeys, requester.PeerVerifier()})
	if err != nil || !verified.ReadyForInstall {
		return report{}, fmt.Errorf("REST snapshot did not verify: %w", err)
	}

	// Exercise the shared-directory carrier with the exact immutable sealed
	// bytes received over REST. Both publication and download stop at bounded
	// offsets and resume from durable prefixes.
	directoryRoot := filepath.Join(workspace, "directory-carrier")
	if err := os.MkdirAll(directoryRoot, 0o700); err != nil {
		return report{}, err
	}
	directory, err := synccarrier.NewDirectory(directoryRoot, group,
		hostHandshake.DatabaseID, requesterHandshake.ReplicaID)
	if err != nil {
		return report{}, err
	}
	name := backup.SealedSHA256
	publishCalls := 0
	for {
		_, complete, copyErr := directory.PublishSnapshotFile(ctx, name, sealed, backup.SealedSHA256, 16<<20)
		if copyErr != nil {
			return report{}, copyErr
		}
		publishCalls++
		if complete {
			break
		}
	}
	directoryCopy := filepath.Join(workspace, "directory-download.nbk")
	downloadCalls := 0
	for {
		_, complete, copyErr := directory.DownloadSnapshotFile(ctx, directory.Namespace(), name,
			backup.SealedSHA256, directoryCopy, backup.SealedBytes, 16<<20)
		if copyErr != nil {
			return report{}, copyErr
		}
		downloadCalls++
		if complete {
			break
		}
	}
	directoryOpen := filepath.Join(workspace, "directory-open")
	if err := os.MkdirAll(directoryOpen, 0o700); err != nil {
		return report{}, err
	}
	_, directoryVerified, err := syncrest.OpenBackup(backup, directoryCopy, directoryOpen, requesterKeys,
		syncwire.MultiVerifier{requesterKeys, requester.PeerVerifier()})
	if err != nil || !directoryVerified.ReadyForInstall {
		return report{}, fmt.Errorf("directory snapshot did not verify: %w", err)
	}

	// A blank target still receives an emergency image before replacement. It
	// is then activated with a fresh allocator and the authenticated vector.
	targetRoot := filepath.Join(workspace, "restored")
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return report{}, err
	}
	targetDB := filepath.Join(targetRoot, "notes.sqlite")
	targetAssets := filepath.Join(targetRoot, "assets")
	blank, err := store.OpenSQLiteWithAssetStore(targetDB, targetAssets)
	if err != nil {
		return report{}, err
	}
	if err := blank.Bootstrap(ctx); err != nil {
		blank.Close()
		return report{}, err
	}
	if err := blank.Close(); err != nil {
		return report{}, err
	}
	restoredReport, err := snapshotimage.Restore(ctx, snapshotRoot, snapshotimage.RestoreOptions{
		Intent: "adopt", TargetDatabase: targetDB, TargetAssetRoot: targetAssets,
		EmergencyDirectory: filepath.Join(workspace, "emergency"),
	})
	if err != nil {
		return report{}, err
	}
	restored, err := store.OpenSQLiteWithAssetStore(targetDB, targetAssets)
	if err != nil {
		return report{}, err
	}
	defer restored.Close()

	hostHandshake, err = host.LocalSyncHandshake(ctx)
	if err != nil {
		return report{}, err
	}
	converged, err := postSnapshotReplay(ctx, host, hostHandshake, hostKeys, restored, seed, server.URL, serverRoot, group)
	if err != nil {
		return report{}, err
	}
	return report{
		SealedBytes: backup.SealedBytes, RESTDownloadCalls: restCalls,
		DirectoryPublishCalls: publishCalls, DirectoryDownloadCalls: downloadCalls,
		RESTVerified: verified.ReadyForInstall, DirectoryVerified: directoryVerified.ReadyForInstall,
		RestoreStage: restoredReport.Stage, ReplicaRotated: restoredReport.OldReplicaID != restoredReport.NewReplicaID,
		DerivedDocuments: restoredReport.DerivedDocuments, PostSnapshotConverged: converged,
	}, nil
}

func downloadREST(ctx context.Context, client *syncauth.Client, backup syncrest.Backup, path string) (int, error) {
	calls := 0
	for {
		_, complete, err := syncrest.DownloadBackup(ctx, client, backup, path, 16<<20)
		if err != nil {
			return calls, err
		}
		calls++
		if complete {
			return calls, nil
		}
	}
}

func newGroupKey() (syncwire.GroupKey, error) {
	key := syncwire.GroupKey{KeyID: "key_g14e", Epoch: 1}
	_, err := rand.Read(key.Key[:])
	return key, err
}

func newKeys(group syncwire.GroupKey) (*keyMaterial, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &keyMaterial{group: group, private: private}, nil
}

func newReplica(ctx context.Context, root, databaseID string, group syncwire.GroupKey) (*store.SQLiteStore, syncstate.Handshake, *keyMaterial, error) {
	canonical, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		return nil, syncstate.Handshake{}, nil, err
	}
	failed := true
	defer func() {
		if failed {
			canonical.Close()
		}
	}()
	if err := canonical.Bootstrap(ctx); err != nil {
		return nil, syncstate.Handshake{}, nil, err
	}
	if _, err := canonical.AdoptDatabaseIdentity(ctx, databaseID); err != nil {
		return nil, syncstate.Handshake{}, nil, err
	}
	if _, err := canonical.EnrollLocalJournal(ctx, "G14e requester"); err != nil {
		return nil, syncstate.Handshake{}, nil, err
	}
	handshake, err := canonical.LocalSyncHandshake(ctx)
	if err != nil {
		return nil, syncstate.Handshake{}, nil, err
	}
	keys, err := newKeys(group)
	if err != nil {
		return nil, syncstate.Handshake{}, nil, err
	}
	failed = false
	return canonical, handshake, keys, nil
}

func pair(ctx context.Context, left *store.SQLiteStore, leftHandshake syncstate.Handshake, leftKeys *keyMaterial,
	right *store.SQLiteStore, rightHandshake syncstate.Handshake, rightKeys *keyMaterial) error {
	for _, item := range []struct {
		local     *store.SQLiteStore
		remote    syncstate.Handshake
		publicKey ed25519.PublicKey
	}{
		{left, rightHandshake, rightKeys.PublicSigningKey()},
		{right, leftHandshake, leftKeys.PublicSigningKey()},
	} {
		if _, err := item.local.EnrollPeerSigningKey(ctx, item.remote.ReplicaID, item.publicKey, "G14e"); err != nil {
			return err
		}
		if err := item.local.ConfigureSyncAdmissionPeer(ctx, item.remote); err != nil {
			return err
		}
	}
	return nil
}

func postSnapshotReplay(ctx context.Context, host *store.SQLiteStore, hostHandshake syncstate.Handshake,
	hostKeys *keyMaterial, restored *store.SQLiteStore, seed store.Document, baseURL, serverRoot string, group syncwire.GroupKey) (bool, error) {
	if _, err := restored.EnrollLocalJournal(ctx, "G14e restored replica"); err != nil {
		return false, err
	}
	restoredHandshake, err := restored.LocalSyncHandshake(ctx)
	if err != nil {
		return false, err
	}
	restoredKeys, err := newKeys(group)
	if err != nil {
		return false, err
	}
	if err := pair(ctx, host, hostHandshake, hostKeys, restored, restoredHandshake, restoredKeys); err != nil {
		return false, err
	}
	document, err := host.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: seed.ID, Title: seed.Title, Body: "generated post-snapshot convergence evidence\n",
		BaseRevisionID: seed.CurrentRevisionID,
	})
	if err != nil {
		return false, err
	}
	client := &syncauth.Client{BaseURL: baseURL, DatabaseID: hostHandshake.DatabaseID,
		ReplicaID: restoredHandshake.ReplicaID, SignerKeyID: restoredKeys.SignerKeyID(), Private: restoredKeys.private}
	restCarrier := syncrest.New(client, group, restoredHandshake.ReplicaID)
	restoredRound := synccarrier.NewRound(synccarrier.NewStoreReplica(restored), restCarrier,
		restoredKeys, restoredKeys, syncwire.MultiVerifier{restoredKeys, restored.PeerVerifier()},
		store.NewLocalObjectProvider(restored), synccarrier.Options{})
	hostCarrier, err := synccarrier.NewDirectory(filepath.Join(serverRoot, "sync-carrier"), group,
		hostHandshake.DatabaseID, hostHandshake.ReplicaID)
	if err != nil {
		return false, err
	}
	// The service hosts the ordinary G11 directory carrier. The host publishes
	// locally and the restored peer consumes the identical artifacts through
	// the authenticated REST courier.
	hostRound := synccarrier.NewRound(synccarrier.NewStoreReplica(host), hostCarrier,
		hostKeys, hostKeys, syncwire.MultiVerifier{hostKeys, host.PeerVerifier()},
		store.NewLocalObjectProvider(host), synccarrier.Options{})
	for pass := 0; pass < 3; pass++ {
		if _, err := hostRound.Run(ctx); err != nil {
			return false, err
		}
		if _, err := restoredRound.Run(ctx); err != nil {
			return false, err
		}
	}
	got, err := restored.GetDocument(ctx, document.ID)
	return err == nil && got.Body == document.Body, err
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
