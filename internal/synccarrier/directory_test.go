package synccarrier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncwire"
)

func testDirectory(t *testing.T, root string) *Directory {
	t.Helper()
	keys, err := syncwire.NewMemoryKeyRing("key_directory")
	if err != nil {
		t.Fatal(err)
	}
	group, err := keys.Current()
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := NewDirectory(root, group, "db_directory", "rep_directory")
	if err != nil {
		t.Fatal(err)
	}
	if err := carrier.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	return carrier
}

func TestAContentNamedArtifactIsVerifiedAgainstItsOwnName(t *testing.T) {
	ctx := context.Background()
	carrier := testDirectory(t, t.TempDir())
	name, err := carrier.Publish(ctx, ClassSnapshot, "", []byte("a catch-up message"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := carrier.Read(ctx, carrier.Namespace(), ClassSnapshot, name); err != nil ||
		string(got) != "a catch-up message" {
		t.Fatalf("read back %q: %v", got, err)
	}

	// Change one byte without changing the name — a bit flip in transit, or a
	// provider serving a stale cached copy under a name that has moved on.
	path := carrier.artifactPath(carrier.Namespace(), ClassSnapshot, name)
	if err := os.WriteFile(path, []byte("a catch-up messagX"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := carrier.Read(ctx, carrier.Namespace(), ClassSnapshot, name); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("a substituted artifact was accepted: %v", err)
	}
}

func TestPublishFallsBackWhenTheFilesystemWillNotRename(t *testing.T) {
	ctx := context.Background()
	carrier := testDirectory(t, t.TempDir())
	carrier.rename = func(string, string) error { return errors.New("rename is not supported here") }

	name, err := carrier.Publish(ctx, ClassSnapshot, "", []byte("published without rename"))
	if err != nil {
		t.Fatalf("a carrier without rename could not publish: %v", err)
	}
	if !carrier.NonAtomic() {
		t.Fatal("the fallback was used but not recorded")
	}
	got, err := carrier.Read(ctx, carrier.Namespace(), ClassSnapshot, name)
	if err != nil || string(got) != "published without rename" {
		t.Fatalf("read back %q: %v", got, err)
	}
	// Nothing is left behind in staging either way.
	staging, err := os.ReadDir(filepath.Join(carrier.namespacePath(carrier.Namespace()), stagingDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(staging) != 0 {
		t.Fatalf("staging holds %d leftover files", len(staging))
	}
}

func TestAnEntryThatDisappearsBetweenListingAndReadingIsASkip(t *testing.T) {
	ctx := context.Background()
	carrier := testDirectory(t, t.TempDir())
	name, err := carrier.Publish(ctx, ClassSnapshot, "", []byte("here and then gone"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := carrier.List(ctx, carrier.Namespace(), ClassSnapshot)
	if err != nil || len(names) != 1 {
		t.Fatalf("list = %v (%v)", names, err)
	}
	// A cloud folder's listing is a claim about the past. Between the listing
	// and the read another peer's cleanup, or the provider, may have removed
	// the file.
	if err := os.Remove(carrier.artifactPath(carrier.Namespace(), ClassSnapshot, name)); err != nil {
		t.Fatal(err)
	}
	if _, err := carrier.Read(ctx, carrier.Namespace(), ClassSnapshot, name); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("a vanished entry produced %v, want ErrUnreadable", err)
	}
}

func TestAnOversizedEntryIsRefusedBeforeItIsRead(t *testing.T) {
	ctx := context.Background()
	carrier := testDirectory(t, t.TempDir())
	name := strings.Repeat("ab", 16)
	path := carrier.artifactPath(carrier.Namespace(), ClassObject, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	oversized, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse: the point is the declared size, and allocating seventeen real
	// megabytes to prove a bound is checked before allocation would be funny.
	if err := oversized.Truncate(MaxArtifactBytes + 1); err != nil {
		t.Fatal(err)
	}
	oversized.Close()
	if _, err := carrier.Read(ctx, carrier.Namespace(), ClassObject, name); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("an oversized entry produced %v, want ErrTooLarge", err)
	}
}

func TestListingIsBoundedAndOrdered(t *testing.T) {
	ctx := context.Background()
	carrier := testDirectory(t, t.TempDir())
	for index := 0; index < 24; index++ {
		if _, err := carrier.Publish(ctx, ClassSnapshot, "", []byte(fmt.Sprintf("artifact %d", index))); err != nil {
			t.Fatal(err)
		}
	}
	first, err := carrier.List(ctx, carrier.Namespace(), ClassSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 24 {
		t.Fatalf("listed %d artifacts, want 24", len(first))
	}
	second, err := carrier.List(ctx, carrier.Namespace(), ClassSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	// The order is this package's, not the filesystem's: two listings of an
	// unchanged directory agree, and a provider that returns entries in
	// whatever order it likes cannot change what a round does.
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("listing order is not stable at %d: %q vs %q", index, first[index], second[index])
		}
		if index > 0 && first[index] <= first[index-1] {
			t.Fatalf("listing is not sorted at %d", index)
		}
	}
}

func TestTwoWritersOnOneCarrierDoNotInterfere(t *testing.T) {
	fixture := newCarrierFixture(t, 2, Options{})
	for index := 0; index < 4; index++ {
		fixture.createNote(t, 0, "", "Left", fmt.Sprintf("left %d\n", index))
		fixture.createNote(t, 1, "", "Right", fmt.Sprintf("right %d\n", index))
	}

	// Both replicas publish and scan at the same time, repeatedly. Each writes
	// only inside its own namespace, so there is no lock to take and nothing to
	// race for; this asserts that claim rather than assuming it.
	for pass := 0; pass < 4; pass++ {
		var group sync.WaitGroup
		errs := make([]error, len(fixture.replicas))
		for index, replica := range fixture.replicas {
			group.Add(1)
			go func(index int, replica *replicaFixture) {
				defer group.Done()
				_, errs[index] = replica.round.Run(context.Background())
			}(index, replica)
		}
		group.Wait()
		for index, err := range errs {
			if err != nil {
				t.Fatalf("concurrent round on %s: %v", fixture.replicas[index].name, err)
			}
		}
	}

	left, err := fixture.replicas[0].store.SyncStateVector(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	right, err := fixture.replicas[1].store.SyncStateVector(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(left) != fmt.Sprint(right) {
		t.Fatalf("concurrent writers diverged: %v vs %v", left, right)
	}
}

func TestACarrierCarriedOnRemovableMediaConvergesInTwoTrips(t *testing.T) {
	ctx := context.Background()
	// A drive is not a shared folder: only one replica can see it at a time,
	// and it arrives at the second replica under a different path.
	fixture := newCarrierFixture(t, 2, Options{})
	documentID := fixture.createNote(t, 0, "doc_usb", "Carried", "carried by hand\n")

	if _, err := fixture.replicas[0].round.Run(ctx); err != nil {
		t.Fatal(err)
	}

	carried := t.TempDir()
	if err := os.Rename(filepath.Join(fixture.root, rootFolder), filepath.Join(carried, rootFolder)); err != nil {
		t.Fatal(err)
	}
	mounted, err := NewDirectory(carried, currentKey(t, fixture.keys), "db_g11_carrier",
		fixture.replicas[1].handshake.ReplicaID)
	if err != nil {
		t.Fatal(err)
	}
	round := NewRound(NewStoreReplica(fixture.replicas[1].store), mounted, fixture.keys,
		fixture.replicas[1].signer, fixture.verifier, nil, Options{})
	if _, err := round.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// One trip out, and the note is there. The replica that has never touched
	// this drive still received exactly what it was missing, because the sender
	// published from what it remembers about its peer rather than waiting to
	// read a greeting off the drive.
	fixture.requireBody(t, 1, documentID, "carried by hand\n")
}

func TestAttachmentBytesTravelThroughTheCarrierOnRequest(t *testing.T) {
	ctx := context.Background()
	fixture := newCarrierFixture(t, 2, Options{})
	origin, receiver := fixture.replicas[0], fixture.replicas[1]

	document, err := origin.store.CreateDocument(ctx, store.CreateDocumentRequest{
		PreferredID: "doc_attached", Title: "With attachment", Body: "see the file\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := bytes.Repeat([]byte("notrios carrier payload\n"), 1024)
	resource, err := origin.store.CreateResource(ctx, store.CreateResourceRequest{
		CollectionID: document.CollectionID, Filename: "attachment.bin",
		MIMEType: "application/octet-stream", Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := origin.store.AttachDocumentResource(ctx, store.AttachResourceRequest{
		DocumentID: document.ID, ResourceID: resource.ID, RelationType: "attachment",
	}); err != nil {
		t.Fatal(err)
	}

	fixture.sync(t, 2)

	// The note and the resource metadata arrive first, and the bytes do not
	// come with them: G8's lazy materialization is unchanged by having a real
	// transport under it.
	if _, err := receiver.store.GetDocument(ctx, document.ID); err != nil {
		t.Fatalf("the note did not arrive: %v", err)
	}
	if _, _, err := receiver.store.OpenResourceContent(ctx, resource.ID); !errors.Is(err, store.ErrResourceUnavailable) {
		t.Fatalf("resource content before materialization: %v", err)
	}

	// The receiver asked for the bytes in the round above; the holder answers
	// on the next one, and only then can they be materialized.
	fixture.sync(t, 2)
	provider := NewProvider(receiver.carrier, fixture.keys, fixture.verifier, syncwire.Limits{})
	report, err := receiver.store.MaterializeResources(ctx, provider, 8)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if report.Materialized != 1 {
		t.Fatalf("materialized %d objects (%+v)", report.Materialized, report.Reasons)
	}
	_, reader, err := receiver.store.OpenResourceContent(ctx, resource.ID)
	if err != nil {
		t.Fatalf("the materialized resource is not readable: %v", err)
	}
	defer reader.Close()
	got := new(bytes.Buffer)
	if _, err := got.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), content) {
		t.Fatalf("materialized %d bytes, want %d", got.Len(), len(content))
	}
}

func TestAnObjectNobodyPublishesStaysUnavailableRatherThanFailing(t *testing.T) {
	ctx := context.Background()
	fixture := newCarrierFixture(t, 2, Options{})
	provider := NewProvider(fixture.replicas[1].carrier, fixture.keys, fixture.verifier, syncwire.Limits{})
	_, err := provider.FetchChunk(ctx, strings.Repeat("00", 32), 0)
	if !errors.Is(err, ErrObjectUnavailable) {
		t.Fatalf("fetching an unpublished object produced %v", err)
	}
	// The error names no full content hash, because a log a user pastes into a
	// bug report should not contain one.
	if strings.Contains(err.Error(), strings.Repeat("00", 32)) {
		t.Fatalf("the error printed a full content hash: %v", err)
	}
}
