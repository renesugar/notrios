package synccarrier

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// replicaFixture is one real replica bound to one real carrier directory. The
// tests below use real SQLite stores rather than a model, because the thing
// under test is precisely the seam between a durable journal and an untrusted
// filesystem — and a model of either end would be a model of the part that
// already works.
type replicaFixture struct {
	name      string
	store     *store.SQLiteStore
	carrier   *Directory
	round     *Round
	signer    *syncwire.MemorySigner
	handshake syncstate.Handshake
}

// carrierFixture is a set of replicas of one database sharing one folder.
type carrierFixture struct {
	root     string
	keys     *syncwire.MemoryKeyRing
	verifier *syncwire.MemoryVerifier
	replicas []*replicaFixture
}

func newCarrierFixture(t *testing.T, count int, options Options) *carrierFixture {
	t.Helper()
	return newFixture(t, count, options, true)
}

// newUnpairedCarrierFixture enrolls the signing keys but configures no peer for
// admission, which is the state a replica is in after discovering a candidate
// and before a user decides to pair with it.
func newUnpairedCarrierFixture(t *testing.T, count int, options Options) *carrierFixture {
	t.Helper()
	return newFixture(t, count, options, false)
}

func newFixture(t *testing.T, count int, options Options, pair bool) *carrierFixture {
	t.Helper()
	ctx := context.Background()
	databaseID := "db_g11_carrier"
	// One group key shared by every replica of one database, and one signing
	// key per replica. That split is G0's: a signature says who wrote an
	// artifact, and the group key says who may read it.
	keys, err := syncwire.NewMemoryKeyRing("key_g11")
	if err != nil {
		t.Fatal(err)
	}
	fixture := &carrierFixture{root: t.TempDir(), keys: keys, verifier: syncwire.NewMemoryVerifier()}

	stores := make([]*store.SQLiteStore, 0, count)
	for index := 0; index < count; index++ {
		canonical, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
		if err != nil {
			t.Fatalf("open replica %d: %v", index, err)
		}
		t.Cleanup(func() { _ = canonical.Close() })
		if err := canonical.Bootstrap(ctx); err != nil {
			t.Fatalf("bootstrap replica %d: %v", index, err)
		}
		if _, err := canonical.AdoptDatabaseIdentity(ctx, databaseID); err != nil {
			t.Fatalf("adopt identity %d: %v", index, err)
		}
		if _, err := canonical.EnrollLocalJournal(ctx, "G11 carrier fixture"); err != nil {
			t.Fatalf("enroll journal %d: %v", index, err)
		}
		stores = append(stores, canonical)
	}

	handshakes := make([]syncstate.Handshake, count)
	for index, canonical := range stores {
		handshake, err := canonical.LocalSyncHandshake(ctx)
		if err != nil {
			t.Fatal(err)
		}
		handshakes[index] = handshake
	}
	if pair {
		for receiver, canonical := range stores {
			for source, handshake := range handshakes {
				if receiver == source {
					continue
				}
				if err := canonical.ConfigureSyncAdmissionPeer(ctx, handshake); err != nil {
					t.Fatalf("configure %d for %d: %v", receiver, source, err)
				}
			}
		}
	}

	for index, canonical := range stores {
		signer, err := syncwire.NewMemorySigner()
		if err != nil {
			t.Fatal(err)
		}
		fixture.verifier.Enroll(signer.Public())
		carrier, err := NewDirectory(fixture.root, currentKey(t, keys), databaseID, handshakes[index].ReplicaID)
		if err != nil {
			t.Fatal(err)
		}
		replica := &replicaFixture{
			name: string(rune('A' + index)), store: canonical, carrier: carrier,
			signer: signer, handshake: handshakes[index],
		}
		replica.round = NewRound(NewStoreReplica(canonical), carrier, keys, signer,
			fixture.verifier, store.NewLocalObjectProvider(canonical), options)
		fixture.replicas = append(fixture.replicas, replica)
	}
	return fixture
}

func currentKey(t *testing.T, keys *syncwire.MemoryKeyRing) syncwire.GroupKey {
	t.Helper()
	group, err := keys.Current()
	if err != nil {
		t.Fatal(err)
	}
	return group
}

// sync runs one round on every replica, in order, the given number of times.
// Two passes are the minimum for a fact to travel: the first publishes it, the
// second admits it and advertises the result.
func (f *carrierFixture) sync(t *testing.T, passes int) []Result {
	t.Helper()
	var results []Result
	for pass := 0; pass < passes; pass++ {
		for _, replica := range f.replicas {
			result, err := replica.round.Run(context.Background())
			if err != nil {
				t.Fatalf("%s round %d: %v", replica.name, pass, err)
			}
			results = append(results, result)
		}
	}
	return results
}

func (f *carrierFixture) createNote(t *testing.T, index int, id, title, body string) string {
	t.Helper()
	document, err := f.replicas[index].store.CreateDocument(context.Background(), store.CreateDocumentRequest{
		PreferredID: id, Title: title, Body: body,
	})
	if err != nil {
		t.Fatalf("create note on %s: %v", f.replicas[index].name, err)
	}
	return document.ID
}

func (f *carrierFixture) requireBody(t *testing.T, index int, documentID, want string) {
	t.Helper()
	document, err := f.replicas[index].store.GetDocument(context.Background(), documentID)
	if err != nil {
		t.Fatalf("replica %s does not hold %s: %v", f.replicas[index].name, documentID, err)
	}
	if document.Body != want {
		t.Fatalf("replica %s body = %q, want %q", f.replicas[index].name, document.Body, want)
	}
}

func TestTwoReplicasConvergeThroughANewDirectory(t *testing.T) {
	fixture := newCarrierFixture(t, 2, Options{})
	left := fixture.createNote(t, 0, "doc_left", "From the left", "left body\n")
	right := fixture.createNote(t, 1, "doc_right", "From the right", "right body\n")

	fixture.sync(t, 2)

	fixture.requireBody(t, 1, left, "left body\n")
	fixture.requireBody(t, 0, right, "right body\n")

	// The carrier holds artifacts, and none of them is a note anyone could
	// read. This is the property the whole layout exists for, so it is asserted
	// rather than assumed.
	assertNoPlaintextOnCarrier(t, fixture.root, "From the left", "From the right", "left body", "right body",
		fixture.replicas[0].handshake.ReplicaID, fixture.replicas[1].handshake.ReplicaID, "db_g11_carrier")
}

func TestThreeReplicasConvergeAndRelayThroughOneDirectory(t *testing.T) {
	fixture := newCarrierFixture(t, 3, Options{})
	first := fixture.createNote(t, 0, "doc_one", "One", "one\n")
	second := fixture.createNote(t, 1, "doc_two", "Two", "two\n")
	third := fixture.createNote(t, 2, "doc_three", "Three", "three\n")

	fixture.sync(t, 3)

	for index := range fixture.replicas {
		fixture.requireBody(t, index, first, "one\n")
		fixture.requireBody(t, index, second, "two\n")
		fixture.requireBody(t, index, third, "three\n")
	}
}

func TestDeletingTheDirectoryLosesNothingAndRepublishes(t *testing.T) {
	fixture := newCarrierFixture(t, 2, Options{})
	first := fixture.createNote(t, 0, "doc_before", "Before", "before\n")
	fixture.sync(t, 2)
	fixture.requireBody(t, 1, first, "before\n")

	// The carrier is disposable. Deleting all of it is not a recovery
	// procedure; it is a case the protocol has to treat as ordinary.
	if err := os.RemoveAll(filepath.Join(fixture.root, rootFolder)); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(fixture.root); err != nil || len(entries) != 0 {
		t.Fatalf("carrier root was not emptied: %v %d", err, len(entries))
	}

	second := fixture.createNote(t, 1, "doc_after", "After", "after\n")
	fixture.sync(t, 3)

	fixture.requireBody(t, 0, second, "after\n")
	fixture.requireBody(t, 1, first, "before\n")
	// And the note that predates the deletion is republished, so a third party
	// restoring only the carrier would still find it.
	if !carrierHoldsEnvelopes(t, fixture) {
		t.Fatal("the recreated carrier holds no envelopes at all")
	}
}

func TestATornOrTruncatedArtifactIsSkippedNotBelieved(t *testing.T) {
	fixture := newCarrierFixture(t, 2, Options{})
	documentID := fixture.createNote(t, 0, "doc_torn", "Torn", "intact body\n")
	// Only the sender runs: the receiver must meet the damaged artifacts first,
	// not a copy it already admitted.
	if _, err := fixture.replicas[0].round.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Truncate every envelope the way an interrupted copy would. A reader that
	// trusted the filename, the size, or the provider's atomicity would accept
	// these; this one must not.
	truncated := 0
	forEachArtifact(t, fixture.root, ClassEnvelope, func(path string) {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, contents[:len(contents)/2], 0o644); err != nil {
			t.Fatal(err)
		}
		truncated++
	})
	if truncated == 0 {
		t.Fatal("no envelope was published, so nothing was tested")
	}

	result, err := fixture.replicas[1].round.Run(context.Background())
	if err != nil {
		t.Fatalf("a torn artifact must not fail the round: %v", err)
	}
	if result.AdmittedOperations != 0 {
		t.Fatalf("admitted %d operations from truncated envelopes", result.AdmittedOperations)
	}
	if result.Skipped["unreadable_artifact"]+result.Skipped["malformed_artifact"]+result.Skipped["over_limit"] == 0 {
		t.Fatalf("truncation was not reported as a skip: %v", result.Skipped)
	}
	if _, err := fixture.replicas[1].store.GetDocument(context.Background(), documentID); err == nil {
		t.Fatal("a truncated envelope delivered a note")
	}

	// Recovery needs no intervention: the sender republishes on its next round
	// because the artifact's name is derived from what it covers.
	fixture.sync(t, 2)
	fixture.requireBody(t, 1, documentID, "intact body\n")
}

func TestAnArtifactSignedByAnUnenrolledKeyIsRefusedAndReportedAsACandidate(t *testing.T) {
	fixture := newCarrierFixture(t, 2, Options{})
	fixture.createNote(t, 0, "doc_stranger", "Stranger", "body\n")
	fixture.sync(t, 1)

	// A stranger who obtained the group key can write into the folder. The
	// signature is the control that stops it from being applied.
	fixture.verifier.Revoke(syncwire.SignerKeyID(fixture.replicas[0].signer.Public()))

	result, err := fixture.replicas[1].round.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.AdmittedOperations != 0 {
		t.Fatalf("admitted %d operations from an unenrolled signer", result.AdmittedOperations)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want exactly one", result.Candidates)
	}
	candidate := result.Candidates[0]
	if candidate.Reason != "unenrolled_signing_key" {
		t.Fatalf("candidate reason = %q", candidate.Reason)
	}
	// Everything about an unverifiable artifact except its key id is unknown,
	// and the candidate must not claim otherwise.
	if candidate.ReplicaID != "" || candidate.DatabaseID != "" {
		t.Fatalf("an unverifiable candidate claimed an identity: %+v", candidate)
	}
	if candidate.SignerKeyID == "" {
		t.Fatal("the candidate does not name the key that would have to be enrolled")
	}
}

func TestAnEnrolledKeyForAnUnpairedReplicaIsReportedNotApplied(t *testing.T) {
	ctx := context.Background()
	// The verifier knows both signing keys, so artifacts open; neither replica
	// is configured for admission, which is exactly the state discovery is for.
	fixture := newUnpairedCarrierFixture(t, 2, Options{})
	documentID := fixture.createNote(t, 0, "doc_unpaired", "Unpaired", "body\n")

	if _, err := fixture.replicas[0].round.Run(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.replicas[1].round.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.AdmittedOperations != 0 {
		t.Fatalf("admitted %d operations from an unpaired replica", result.AdmittedOperations)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Reason != "not_enrolled_for_admission" {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	if result.Candidates[0].ReplicaID != fixture.replicas[0].handshake.ReplicaID {
		t.Fatalf("candidate does not name the replica a user would pair with: %+v", result.Candidates[0])
	}
	if _, err := fixture.replicas[1].store.GetDocument(ctx, documentID); err == nil {
		t.Fatal("an unpaired peer's note was applied")
	}
}

func TestArtifactsPublishedIntoAnotherPeersNamespaceAreRefused(t *testing.T) {
	ctx := context.Background()
	fixture := newCarrierFixture(t, 2, Options{})
	fixture.createNote(t, 0, "doc_forged", "Forged", "body\n")
	if _, err := fixture.replicas[0].round.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// Move replica A's artifacts into a namespace that is not A's. Both peers
	// hold the group key, so this is something an enrolled peer can do; the
	// binding between a namespace and the identity inside its artifacts is what
	// refuses it.
	source := filepath.Join(fixture.replicas[0].carrier.Root(), replicasDir, fixture.replicas[0].carrier.Namespace())
	target := filepath.Join(fixture.replicas[0].carrier.Root(), replicasDir, strings.Repeat("ab", 16))
	if err := os.Rename(source, target); err != nil {
		t.Fatal(err)
	}

	result, err := fixture.replicas[1].round.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.AdmittedOperations != 0 {
		t.Fatalf("admitted %d operations from a foreign namespace", result.AdmittedOperations)
	}
	if result.Skipped["namespace_identity_mismatch"] == 0 {
		t.Fatalf("the namespace binding did not refuse the artifact: %v", result.Skipped)
	}
}

func TestUnknownEntriesReorderedListingsAndWrongCaseNamesAreIgnored(t *testing.T) {
	ctx := context.Background()
	fixture := newCarrierFixture(t, 2, Options{})
	documentID := fixture.createNote(t, 0, "doc_noise", "Noise", "body\n")
	if _, err := fixture.replicas[0].round.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// The kinds of entry a real shared folder accumulates: provider sidecars,
	// conflict copies, another tool's temporary file, a directory, and an
	// upper-case copy that a case-insensitive filesystem would fold onto a real
	// artifact's name.
	envelopes := filepath.Join(fixture.replicas[0].carrier.Root(), replicasDir,
		fixture.replicas[0].carrier.Namespace(), string(ClassEnvelope))
	var realName string
	forEachArtifact(t, fixture.root, ClassEnvelope, func(path string) { realName = filepath.Base(path) })
	if realName == "" {
		t.Fatal("no envelope to work with")
	}
	contents, err := os.ReadFile(filepath.Join(envelopes, realName))
	if err != nil {
		t.Fatal(err)
	}
	for _, noise := range []string{
		".DS_Store", "desktop.ini", "sync-conflict-2026.txt", "half-copied.tmp",
		strings.ToUpper(strings.TrimSuffix(realName, artifactExt)) + artifactExt,
		"deadbeef.nar", strings.Repeat("z", 64) + artifactExt,
	} {
		if err := os.WriteFile(filepath.Join(envelopes, noise), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(envelopes, strings.Repeat("cd", 32)+artifactExt), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := fixture.replicas[1].round.Run(ctx)
	if err != nil {
		t.Fatalf("noise in the carrier failed the round: %v", err)
	}
	// Exactly one envelope was real, and the duplicates under invalid names
	// were never opened — so the round did not even have to rely on admission
	// being idempotent.
	if result.AdmittedEnvelopes != 1 {
		t.Fatalf("admitted %d envelopes, want 1: %v", result.AdmittedEnvelopes, result.Skipped)
	}
	fixture.requireBody(t, 1, documentID, "body\n")
}

func TestPublishingIsIdempotentSoAQuietCarrierStopsGrowing(t *testing.T) {
	fixture := newCarrierFixture(t, 2, Options{})
	fixture.createNote(t, 0, "doc_quiet", "Quiet", "body\n")
	fixture.sync(t, 3)
	before := countArtifacts(t, fixture.root)

	// Ten more rounds with nothing new to say. Naming artifacts by their sealed
	// bytes would add a file per round forever, because every seal draws a
	// fresh salt; naming them by what they logically are does not.
	fixture.sync(t, 10)
	after := countArtifacts(t, fixture.root)
	if after != before {
		t.Fatalf("a quiet carrier grew from %d to %d artifacts", before, after)
	}
}

func TestCleanupNeverRemovesAnotherPeersFiles(t *testing.T) {
	ctx := context.Background()
	fixture := newCarrierFixture(t, 2, Options{Cleanup: true})
	fixture.createNote(t, 0, "doc_cleanup", "Cleanup", "body\n")
	fixture.sync(t, 3)

	foreign := filepath.Join(fixture.replicas[0].carrier.Root(), replicasDir,
		strings.Repeat("ef", 16), string(ClassEnvelope))
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	stranger := filepath.Join(foreign, strings.Repeat("12", 32)+artifactExt)
	if err := os.WriteFile(stranger, []byte("not ours to delete"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.replicas[0].round.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stranger); err != nil {
		t.Fatalf("cleanup removed a file in another namespace: %v", err)
	}
	// And the carrier API refuses it directly, not merely by not trying.
	if err := fixture.replicas[0].carrier.Remove(ctx, ClassEnvelope, "../../elsewhere"); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("Remove accepted a path outside the namespace: %v", err)
	}
}

func TestCorrectnessSurvivesNoCleanupAtAll(t *testing.T) {
	// G11's resolved decision requires this: cleanup is an optimization, and a
	// carrier nobody ever tidies must still converge.
	fixture := newCarrierFixture(t, 2, Options{Cleanup: false, MaxOperationsPerEnvelope: 1})
	var ids []string
	for index := 0; index < 6; index++ {
		ids = append(ids, fixture.createNote(t, index%2, "", "Note", "body\n"))
		fixture.sync(t, 1)
	}
	fixture.sync(t, 3)
	for _, documentID := range ids {
		fixture.requireBody(t, 0, documentID, "body\n")
		fixture.requireBody(t, 1, documentID, "body\n")
	}
}

func TestAnUnavailableCarrierIsReportedNotFatalToTheReplica(t *testing.T) {
	ctx := context.Background()
	fixture := newCarrierFixture(t, 2, Options{})
	documentID := fixture.createNote(t, 0, "doc_removable", "Removable", "body\n")
	fixture.sync(t, 2)
	fixture.requireBody(t, 1, documentID, "body\n")

	// A removable drive, unplugged: the root is gone and the replica is asked
	// to sync anyway.
	unplugged := filepath.Join(t.TempDir(), "no-such-mount")
	carrier, err := NewDirectory(unplugged, currentKey(t, fixture.keys), "db_g11_carrier",
		fixture.replicas[0].handshake.ReplicaID)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.Available() {
		t.Fatal("an absent mount point reported itself available")
	}
	round := NewRound(NewStoreReplica(fixture.replicas[0].store), carrier, fixture.keys,
		fixture.replicas[0].signer, fixture.verifier, nil, Options{})
	// The round refuses rather than creating the mount point. Creating it would
	// write the carrier onto the boot disk underneath an empty mount, where the
	// peer will never look for it and the space will never be noticed.
	if _, err := round.Run(ctx); !errors.Is(err, ErrCarrierUnavailable) {
		t.Fatalf("an unavailable carrier produced %v, want ErrCarrierUnavailable", err)
	}
	if _, err := os.Stat(unplugged); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the round created the absent mount point: %v", err)
	}
	// The library itself is untouched and still works.
	fixture.requireBody(t, 0, documentID, "body\n")
}

func forEachArtifact(t *testing.T, root string, class Class, visit func(path string)) {
	t.Helper()
	base := filepath.Join(root, rootFolder, layoutVersion)
	_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != string(class) || !strings.HasSuffix(path, artifactExt) {
			return nil
		}
		visit(path)
		return nil
	})
}

func countArtifacts(t *testing.T, root string) int {
	t.Helper()
	count := 0
	_ = filepath.Walk(filepath.Join(root, rootFolder), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, artifactExt) {
			count++
		}
		return nil
	})
	return count
}

func carrierHoldsEnvelopes(t *testing.T, fixture *carrierFixture) bool {
	t.Helper()
	found := false
	forEachArtifact(t, fixture.root, ClassEnvelope, func(string) { found = true })
	return found
}

// assertNoPlaintextOnCarrier reads every byte of the carrier, names included,
// and refuses to find any of the strings a carrier is not permitted to learn.
func assertNoPlaintextOnCarrier(t *testing.T, root string, forbidden ...string) {
	t.Helper()
	_ = filepath.Walk(filepath.Join(root, rootFolder), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		for _, secret := range forbidden {
			if strings.Contains(relative, secret) {
				t.Fatalf("carrier path %q exposes %q", relative, secret)
			}
		}
		if info.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, secret := range forbidden {
			if strings.Contains(string(contents), secret) {
				t.Fatalf("carrier file %q exposes %q in its bytes", relative, secret)
			}
		}
		return nil
	})
}
