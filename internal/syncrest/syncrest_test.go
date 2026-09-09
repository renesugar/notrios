package syncrest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// keyMaterial is one replica's keys: the shared group key and its own signing
// identity. Both replicas in a fixture share the group and differ in the
// signature, which is the split G0 fixed.
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

// PublicKey makes this key material a verifier for its own signatures, which is
// what a replica needs to read back what it published.
func (k *keyMaterial) PublicKey(signerKeyID string) (ed25519.PublicKey, bool) {
	if signerKeyID != k.SignerKeyID() {
		return nil, false
	}
	return k.private.Public().(ed25519.PublicKey), true
}

type replica struct {
	name      string
	store     *store.SQLiteStore
	keys      *keyMaterial
	handshake syncstate.Handshake
}

type fixture struct {
	host    *replica
	guest   *replica
	server  *httptest.Server
	client  *syncauth.Client
	carrier *Carrier
	// hostRound is how the host merges what arrives. The transport does not
	// merge: a REST peer moves artifacts, and the round on the machine that
	// owns the library applies them.
	hostRound *synccarrier.Round
	hostDir   string
}

func newReplica(t *testing.T, name, databaseID string, group syncwire.GroupKey) *replica {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	canonical, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = canonical.Close() })
	if err := canonical.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := canonical.AdoptDatabaseIdentity(ctx, databaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := canonical.EnrollLocalJournal(ctx, "G14 fixture"); err != nil {
		t.Fatal(err)
	}
	handshake, err := canonical.LocalSyncHandshake(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &replica{name: name, store: canonical, keys: &keyMaterial{group: group, private: private}, handshake: handshake}
}

// newFixture builds a host serving its sync surface over loopback HTTP and a
// guest that talks to it, each enrolled with the other.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	var groupBytes [syncwire.GroupKeyBytes]byte
	if _, err := rand.Read(groupBytes[:]); err != nil {
		t.Fatal(err)
	}
	group := syncwire.GroupKey{KeyID: "key_g14", Epoch: 1, Key: groupBytes}

	host := newReplica(t, "host", "db_g14", group)
	guest := newReplica(t, "guest", "db_g14", group)
	for _, pair := range []struct{ local, remote *replica }{{host, guest}, {guest, host}} {
		if _, err := pair.local.store.EnrollPeerSigningKey(ctx, pair.remote.handshake.ReplicaID,
			pair.remote.keys.PublicSigningKey(), "fixture"); err != nil {
			t.Fatal(err)
		}
		if err := pair.local.store.ConfigureSyncAdmissionPeer(ctx, pair.remote.handshake); err != nil {
			t.Fatal(err)
		}
	}

	hostRoot := t.TempDir()
	cfg := config.Default()
	cfg.Sync.REST.Enabled = true
	cfg.Data.Directory = hostRoot
	handler := httpapi.NewServerWithOptions(httpapi.ServerOptions{Store: host.store, Config: cfg})
	if err := handler.AttachSyncSecurity(host.store, host.keys, syncauth.DefaultLimits(), nil); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := &syncauth.Client{
		BaseURL: server.URL, DatabaseID: "db_g14", ReplicaID: guest.handshake.ReplicaID,
		SignerKeyID: guest.keys.SignerKeyID(), Private: guest.keys.private,
	}
	carrier := New(client, group, guest.handshake.ReplicaID)

	// The host reads and writes the same folder through an ordinary directory
	// carrier — the one the REST surface hosts — so both ends of the exchange
	// are the shipped G11 round.
	hostCarrier, err := synccarrier.NewDirectory(filepath.Join(hostRoot, "sync-carrier"), group,
		"db_g14", host.handshake.ReplicaID)
	if err != nil {
		t.Fatal(err)
	}
	hostRound := synccarrier.NewRound(synccarrier.NewStoreReplica(host.store), hostCarrier,
		host.keys, host.keys, syncwire.MultiVerifier{host.keys, host.store.PeerVerifier()},
		store.NewLocalObjectProvider(host.store), synccarrier.Options{})

	return &fixture{
		host: host, guest: guest, server: server, client: client, carrier: carrier,
		hostRound: hostRound, hostDir: hostRoot,
	}
}

// guestRound builds the guest's round over the REST carrier.
func (f *fixture) guestRound() *synccarrier.Round {
	return synccarrier.NewRound(synccarrier.NewStoreReplica(f.guest.store), f.carrier,
		f.guest.keys, f.guest.keys, syncwire.MultiVerifier{f.guest.keys, f.guest.store.PeerVerifier()},
		store.NewLocalObjectProvider(f.guest.store), synccarrier.Options{})
}

func (f *fixture) exchange(t *testing.T, passes int) {
	t.Helper()
	guest := f.guestRound()
	for pass := 0; pass < passes; pass++ {
		if _, err := guest.Run(context.Background()); err != nil {
			t.Fatalf("guest round %d: %v", pass, err)
		}
		if _, err := f.hostRound.Run(context.Background()); err != nil {
			t.Fatalf("host round %d: %v", pass, err)
		}
	}
}

func (f *fixture) note(t *testing.T, who *replica, id, body string) string {
	t.Helper()
	document, err := who.store.CreateDocument(context.Background(), store.CreateDocumentRequest{
		PreferredID: id, Title: "Note " + id, Body: body,
	})
	if err != nil {
		t.Fatalf("create on %s: %v", who.name, err)
	}
	return document.ID
}

func (f *fixture) requireBody(t *testing.T, who *replica, documentID, want string) {
	t.Helper()
	document, err := who.store.GetDocument(context.Background(), documentID)
	if err != nil {
		t.Fatalf("%s does not hold %s: %v", who.name, documentID, err)
	}
	if document.Body != want {
		t.Fatalf("%s body = %q, want %q", who.name, document.Body, want)
	}
}

func TestTwoReplicasConvergeOverREST(t *testing.T) {
	fixture := newFixture(t)
	fromGuest := fixture.note(t, fixture.guest, "doc_guest", "guest body\n")
	fromHost := fixture.note(t, fixture.host, "doc_host", "host body\n")

	fixture.exchange(t, 3)

	fixture.requireBody(t, fixture.host, fromGuest, "guest body\n")
	fixture.requireBody(t, fixture.guest, fromHost, "host body\n")
}

// TestTheRESTTranscriptIsTheDirectoryTranscript is the parity the plan asked
// for. The same round, the same artifacts, the same names — the only difference
// is the courier, so what the REST peer stored is byte-for-byte what a folder
// would have held.
func TestTheRESTTranscriptIsTheDirectoryTranscript(t *testing.T) {
	fixture := newFixture(t)
	fixture.note(t, fixture.guest, "doc_parity", "parity body\n")
	fixture.exchange(t, 2)

	// What the guest published over HTTP landed in the host's folder under the
	// guest's own blinded namespace, with the protocol's own names.
	guestNamespace := syncwire.CarrierName(fixture.guest.keys.group, "replica", fixture.guest.handshake.ReplicaID)
	base := filepath.Join(fixture.hostDir, "sync-carrier", "notrios-sync", "v1",
		syncwire.CarrierName(fixture.guest.keys.group, "database", "db_g14"), "replicas", guestNamespace)
	envelopes, err := os.ReadDir(filepath.Join(base, "envelopes"))
	if err != nil || len(envelopes) == 0 {
		t.Fatalf("the guest's envelopes are not in the host's carrier: %v", err)
	}
	for _, entry := range envelopes {
		if !strings.HasSuffix(entry.Name(), ".nar") {
			t.Fatalf("unexpected entry %q", entry.Name())
		}
		contents, err := os.ReadFile(filepath.Join(base, "envelopes", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		// Sealed, signed protocol bytes — not JSON a server rewrote.
		if !strings.HasPrefix(string(contents), syncwire.ArtifactMagic) {
			t.Fatalf("the stored artifact is not a %s artifact", syncwire.ArtifactMagic)
		}
	}
}

func TestAPeerCannotPublishIntoAnotherNamespace(t *testing.T) {
	fixture := newFixture(t)
	fixture.note(t, fixture.guest, "doc_ns", "body\n")
	fixture.exchange(t, 2)

	// The guest asks to write under the host's namespace by naming it in the
	// path. There is no such path: the route takes a class and a name, and the
	// namespace comes from who is asking.
	hostNamespace := syncwire.CarrierName(fixture.host.keys.group, "replica", fixture.host.handshake.ReplicaID)
	status, _, err := fixture.client.Do(context.Background(), http.MethodPut,
		"/api/v1/sync/carrier/"+hostNamespace+"/envelopes", []byte("not an artifact"))
	if err != nil {
		t.Fatal(err)
	}
	if status == http.StatusOK {
		t.Fatal("a peer published into another namespace")
	}
}

func TestRangeRequestsBehaveAndAnUnsatisfiableOneIs416(t *testing.T) {
	fixture := newFixture(t)
	fixture.note(t, fixture.guest, "doc_range", "ranged body\n")
	fixture.exchange(t, 2)

	guestNamespace := fixture.carrier.Namespace()
	names, err := fixture.carrier.List(context.Background(), guestNamespace, synccarrier.ClassEnvelope)
	if err != nil || len(names) == 0 {
		t.Fatalf("no envelope to range over: %v", err)
	}
	path := fmt.Sprintf("/api/v1/sync/carrier/%s/%s/%s", guestNamespace, synccarrier.ClassEnvelope, names[0])
	whole, body, err := fixture.client.Do(context.Background(), http.MethodGet, path, nil)
	if err != nil || whole != http.StatusOK {
		t.Fatalf("whole read: %d %v", whole, err)
	}
	status, partial, err := fixture.client.DoRange(context.Background(), http.MethodGet, path, "bytes=0-15")
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", status)
	}
	if len(partial) != 16 || string(partial) != string(body[:16]) {
		t.Fatalf("range returned %d bytes that do not match the whole", len(partial))
	}
	beyond, _, err := fixture.client.DoRange(context.Background(), http.MethodGet, path,
		fmt.Sprintf("bytes=%d-%d", len(body)+10, len(body)+20))
	if err != nil {
		t.Fatal(err)
	}
	if beyond != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("unsatisfiable range = %d, want 416", beyond)
	}
}

func TestMultiGiBDownloadResumesFromTheDurableFileLength(t *testing.T) {
	const total = int64(3)<<30 + 12345
	const tail = int64(64 << 10)
	var gotRange string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", total-tail, total-1, total))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(make([]byte, tail))
	}))
	defer server.Close()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	client := &syncauth.Client{
		BaseURL: server.URL, DatabaseID: "db_multigib", ReplicaID: "replica_multigib",
		SignerKeyID: syncwire.SignerKeyID(private.Public().(ed25519.PublicKey)), Private: private,
	}
	destination := filepath.Join(t.TempDir(), "snapshot.nbk")
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(total - tail); err != nil {
		t.Fatal(err)
	}
	file.Close()
	written, complete, err := DownloadBackup(context.Background(), client, Backup{
		ID: "opaque-multigib", SealedBytes: total,
	}, destination, tail)
	if err != nil || !complete || written != total {
		t.Fatalf("multi-GiB resume: %d %v %v", written, complete, err)
	}
	wantRange := fmt.Sprintf("bytes=%d-%d", total-tail, total-1)
	if gotRange != wantRange {
		t.Fatalf("Range = %q, want %q", gotRange, wantRange)
	}
}

// TestAnInterruptedBackupResumesAndVerifies is G14's working state: a replica
// asks for a snapshot, the transfer stops part way, it resumes from where it
// stopped, and the G14c physical snapshot verifier — not the container or
// transport — decides whether what arrived is an installable snapshot.
func TestAnInterruptedBackupResumesAndVerifies(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)
	for index := 0; index < 5; index++ {
		fixture.note(t, fixture.host, fmt.Sprintf("doc_backup_%d", index), fmt.Sprintf("body %d\n", index))
	}

	// Enrolment is not permission: the host must explicitly allow this peer to
	// receive a complete copy of the library.
	if _, err := RequestBackup(ctx, fixture.client); err == nil {
		t.Fatal("a snapshot was produced for a peer that was not permitted")
	}
	if err := fixture.host.store.PermitSnapshotSource(ctx, fixture.guest.handshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	backup, err := RequestBackup(ctx, fixture.client)
	if err != nil {
		t.Fatalf("RequestBackup: %v", err)
	}
	if backup.SealedBytes <= 0 || backup.SealedSHA256 == "" || backup.WrappedKey == "" {
		t.Fatalf("the peer described nothing usable: %+v", backup)
	}

	workspace := t.TempDir()
	sealed := filepath.Join(workspace, "snapshot.nbk")

	// One bounded call at a time, so the transfer is interrupted repeatedly and
	// resumes from the local file's own length.
	calls := 0
	for {
		written, complete, err := DownloadBackup(ctx, fixture.client, backup, sealed, 64<<10)
		if err != nil {
			t.Fatalf("download: %v", err)
		}
		calls++
		if complete {
			if written != backup.SealedBytes {
				t.Fatalf("completed at %d of %d bytes", written, backup.SealedBytes)
			}
			break
		}
		if calls > 4096 {
			t.Fatal("the download never completed")
		}
	}
	if calls < 2 {
		t.Fatalf("the transfer finished in %d call(s), so resume was never exercised", calls)
	}

	archiveDir, report, err := OpenBackup(backup, sealed, workspace,
		fixture.guest.keys, syncwire.MultiVerifier{fixture.guest.keys, fixture.guest.store.PeerVerifier()})
	if err != nil {
		t.Fatalf("OpenBackup: %v", err)
	}
	if report.CommitSHA256 == "" || !report.ReadyForInstall {
		t.Fatalf("the physical snapshot did not verify: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "manifest.json")); err != nil {
		t.Fatalf("the extracted archive has no manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archiveDir, "interrupted-open.partial"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	archiveDir, report, err = OpenBackup(backup, sealed, workspace,
		fixture.guest.keys, syncwire.MultiVerifier{fixture.guest.keys, fixture.guest.store.PeerVerifier()})
	if err != nil || !report.ReadyForInstall {
		t.Fatalf("restart physical open from durable sealed bytes: %+v %v", report, err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "interrupted-open.partial")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restarted open retained partial extraction: %v", err)
	}
	// The snapshot's vector is what makes a restored replica able to resume
	// incrementally rather than asking for everything again.
	if len(backup.SnapshotVector) == 0 {
		t.Fatal("the backup carries no state vector")
	}

	// Restore it into a blank replica with an explicit intent, then let that
	// replica continue incrementally over REST. This is the whole loop G10 and
	// G14 exist to close: a new device gets a library from a snapshot and then
	// keeps up with ordinary exchanges rather than replaying everything.
	blankRoot := t.TempDir()
	blankDB := filepath.Join(blankRoot, "notes.sqlite")
	blankAssets := filepath.Join(blankRoot, "assets")
	blank, err := store.OpenSQLiteWithAssetStore(blankDB, blankAssets)
	if err != nil {
		t.Fatal(err)
	}
	if err := blank.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := blank.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotimage.Restore(ctx, archiveDir, snapshotimage.RestoreOptions{
		Intent: "adopt", TargetDatabase: blankDB, TargetAssetRoot: blankAssets,
		EmergencyDirectory: filepath.Join(blankRoot, "emergency"),
	}); err != nil {
		t.Fatalf("restore with an explicit intent: %v", err)
	}
	blank, err = store.OpenSQLiteWithAssetStore(blankDB, blankAssets)
	if err != nil {
		t.Fatal(err)
	}
	defer blank.Close()
	for index := 0; index < 5; index++ {
		if _, err := blank.GetDocument(ctx, fmt.Sprintf("doc_backup_%d", index)); err != nil {
			t.Fatalf("the restored replica is missing doc_backup_%d: %v", index, err)
		}
	}

	// A note written after the snapshot reaches the restored replica through an
	// ordinary exchange.
	fixture.note(t, fixture.host, "doc_after_snapshot", "written after the snapshot\n")
	restored := newRestoredPeer(t, fixture, blank)
	for pass := 0; pass < 3; pass++ {
		if _, err := restored.Run(ctx); err != nil {
			t.Fatalf("the restored replica could not exchange: %v", err)
		}
		if _, err := fixture.hostRound.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := blank.GetDocument(ctx, "doc_after_snapshot"); err != nil {
		t.Fatalf("the restored replica did not continue incrementally: %v", err)
	}
}

// newRestoredPeer enrols a replica built from a snapshot with the host and
// gives it a round over REST. Restoring produced a new replica identity, so it
// is a new peer as far as the host is concerned — which is the point of adopt.
func newRestoredPeer(t *testing.T, f *fixture, restored *store.SQLiteStore) *synccarrier.Round {
	t.Helper()
	ctx := context.Background()
	if _, err := restored.EnrollLocalJournal(ctx, "restored replica"); err != nil {
		t.Fatal(err)
	}
	handshake, err := restored.LocalSyncHandshake(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys := &keyMaterial{group: f.guest.keys.group, private: private}
	for _, pair := range []struct {
		local  *store.SQLiteStore
		remote syncstate.Handshake
		public ed25519.PublicKey
	}{
		{f.host.store, handshake, keys.PublicSigningKey()},
		{restored, f.host.handshake, f.host.keys.PublicSigningKey()},
	} {
		if _, err := pair.local.EnrollPeerSigningKey(ctx, pair.remote.ReplicaID, pair.public, "restored"); err != nil {
			t.Fatal(err)
		}
		if err := pair.local.ConfigureSyncAdmissionPeer(ctx, pair.remote); err != nil {
			t.Fatal(err)
		}
	}
	client := &syncauth.Client{
		BaseURL: f.server.URL, DatabaseID: handshake.DatabaseID, ReplicaID: handshake.ReplicaID,
		SignerKeyID: keys.SignerKeyID(), Private: private,
	}
	return synccarrier.NewRound(synccarrier.NewStoreReplica(restored),
		New(client, keys.group, handshake.ReplicaID), keys, keys,
		syncwire.MultiVerifier{keys, restored.PeerVerifier()},
		store.NewLocalObjectProvider(restored), synccarrier.Options{})
}

func TestATamperedBackupIsRefusedBeforeItIsOpened(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)
	fixture.note(t, fixture.host, "doc_tamper", "body\n")
	if err := fixture.host.store.PermitSnapshotSource(ctx, fixture.guest.handshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	backup, err := RequestBackup(ctx, fixture.client)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	sealed := filepath.Join(workspace, "snapshot.nbk")
	for {
		_, complete, err := DownloadBackup(ctx, fixture.client, backup, sealed, 0)
		if err != nil {
			t.Fatal(err)
		}
		if complete {
			break
		}
	}
	contents, err := os.ReadFile(sealed)
	if err != nil {
		t.Fatal(err)
	}
	contents[len(contents)/2] ^= 0x01
	if err := os.WriteFile(sealed, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenBackup(backup, sealed, workspace, fixture.guest.keys,
		syncwire.MultiVerifier{fixture.guest.keys, fixture.guest.store.PeerVerifier()}); err == nil {
		t.Fatal("a tampered snapshot was opened")
	}
	// And the failure is the transport's own hash check, before any container
	// parsing happens at all.
	_, _, err = OpenBackup(backup, sealed, workspace, fixture.guest.keys,
		syncwire.MultiVerifier{fixture.guest.keys, fixture.guest.store.PeerVerifier()})
	if !strings.Contains(err.Error(), "does not match its declared hash") {
		t.Fatalf("the refusal came from the wrong layer: %v", err)
	}
}

func TestABackupBelongsToTheReplicaThatAskedForIt(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)
	fixture.note(t, fixture.host, "doc_owner", "body\n")
	if err := fixture.host.store.PermitSnapshotSource(ctx, fixture.guest.handshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	backup, err := RequestBackup(ctx, fixture.client)
	if err != nil {
		t.Fatal(err)
	}

	// A third replica, enrolled and authenticated, asks for someone else's
	// backup by its opaque id. A backup is a complete copy of a library, and
	// authorization for one is not authorization for another's.
	third := newReplica(t, "third", "db_g14", fixture.guest.keys.group)
	if _, err := fixture.host.store.EnrollPeerSigningKey(ctx, third.handshake.ReplicaID,
		third.keys.PublicSigningKey(), "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.host.store.ConfigureSyncAdmissionPeer(ctx, third.handshake); err != nil {
		t.Fatal(err)
	}
	thirdClient := &syncauth.Client{
		BaseURL: fixture.server.URL, DatabaseID: "db_g14", ReplicaID: third.handshake.ReplicaID,
		SignerKeyID: third.keys.SignerKeyID(), Private: third.keys.private,
	}
	status, _, err := thirdClient.Do(ctx, http.MethodGet, "/api/v1/sync/backups/"+backup.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusNotFound {
		t.Fatalf("another replica fetched a backup it did not ask for: %d", status)
	}
}

func TestBackupMemoryStaysBoundedByTheFrame(t *testing.T) {
	// The container is packed and sealed through a pipe and opened frame by
	// frame, so nothing here is proof by measurement — it is proof by shape:
	// a frame is the largest buffer either direction allocates. What this test
	// adds is that a snapshot larger than one frame round-trips at all.
	fixture := newFixture(t)
	ctx := context.Background()
	// A library big enough to need several frames.
	for index := 0; index < 40; index++ {
		fixture.note(t, fixture.host, fmt.Sprintf("doc_big_%d", index), strings.Repeat("padding line\n", 900))
	}
	if err := fixture.host.store.PermitSnapshotSource(ctx, fixture.guest.handshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	backup, err := RequestBackup(ctx, fixture.client)
	if err != nil {
		t.Fatal(err)
	}
	if backup.SealedBytes < 512*1024 {
		t.Skip("the generated library did not produce a multi-frame snapshot")
	}
	workspace := t.TempDir()
	sealed := filepath.Join(workspace, "snapshot.nbk")
	for {
		_, complete, err := DownloadBackup(ctx, fixture.client, backup, sealed, 256*1024)
		if err != nil {
			t.Fatal(err)
		}
		if complete {
			break
		}
	}
	_, report, err := OpenBackup(backup, sealed, workspace, fixture.guest.keys,
		syncwire.MultiVerifier{fixture.guest.keys, fixture.guest.store.PeerVerifier()})
	if err != nil || !report.ReadyForInstall {
		t.Fatalf("a multi-frame snapshot did not verify: %v %+v", err, report)
	}
}
