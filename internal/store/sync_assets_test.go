package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/syncassets"
)

func generatedBytes(seed int64, size int) []byte {
	random := rand.New(rand.NewSource(seed))
	value := make([]byte, size)
	_, _ = random.Read(value)
	return value
}

// attachedFixture creates a note with an attachment on one replica and ships
// the operations — but not the bytes — to another.
func attachedFixture(t *testing.T, content []byte, mimeType string) (*SQLiteStore, *SQLiteStore, Resource, string) {
	t.Helper()
	ctx := context.Background()
	origin := newAdmissionReplica(t, "db_g8")
	receiver := newAdmissionReplica(t, "db_g8")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, receiver})

	document, err := origin.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_g8", Title: "With attachment", Body: "see the file\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	resource, err := origin.CreateResource(ctx, CreateResourceRequest{
		CollectionID: document.CollectionID, Filename: "attachment.bin",
		MIMEType: mimeType, Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := origin.AttachDocumentResource(ctx, AttachResourceRequest{
		DocumentID: document.ID, ResourceID: resource.ID, RelationType: "attachment",
	}); err != nil {
		t.Fatal(err)
	}
	shipAll(t, origin, receiver)
	return origin, receiver, resource, document.ID
}

func TestAttachmentMetadataArrivesBeforeItsBytes(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(1, 3*syncassets.ChunkBytes+512)
	origin, receiver, resource, documentID := attachedFixture(t, content, "application/octet-stream")

	// The note and its reference are usable immediately.
	if _, err := receiver.GetDocument(ctx, documentID); err != nil {
		t.Fatalf("the note did not arrive: %v", err)
	}
	mirrored, err := receiver.GetResource(ctx, resource.ID)
	if err != nil {
		t.Fatalf("the resource metadata did not arrive: %v", err)
	}
	if mirrored.SHA256 != resource.SHA256 || mirrored.SizeBytes != int64(len(content)) {
		t.Fatalf("resource metadata does not match the origin: %+v", mirrored)
	}
	refs := queryRows(t, receiver, `SELECT resource_id FROM document_resource_refs WHERE document_id = ?`, documentID)
	if len(refs) != 1 || refs[0]["resource_id"] != resource.ID {
		t.Fatalf("the note's reference to its attachment did not arrive: %+v", refs)
	}

	// The bytes are not here, and saying so is the point.
	availability, err := receiver.ResourceAvailabilityFor(ctx, resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if availability.Available {
		t.Fatal("the receiver claims bytes it never received")
	}
	if availability.SourceCount != 1 {
		t.Fatalf("the advertising peer was not recorded: %+v", availability)
	}
	if availability.TotalChunks != 4 {
		t.Fatalf("chunk count = %d, want 4", availability.TotalChunks)
	}
	if _, _, err := receiver.OpenResourceContent(ctx, resource.ID); !errors.Is(err, ErrResourceUnavailable) {
		t.Fatalf("opening unavailable content must not look like a missing resource: %v", err)
	}
	// And no placeholder bytes were written anywhere.
	assertNoStagedOrStoredBytes(t, receiver, resource.SHA256)

	// The origin, which has the bytes, is unaffected.
	if _, reader, err := origin.OpenResourceContent(ctx, resource.ID); err != nil {
		t.Fatalf("the origin lost its own attachment: %v", err)
	} else {
		_ = reader.Close()
	}
}

func assertNoStagedOrStoredBytes(t *testing.T, st *SQLiteStore, sha256Hex string) {
	t.Helper()
	stored := filepath.Join(st.assetRoot, "sha256", sha256Hex[0:2], sha256Hex[2:4], sha256Hex)
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Fatalf("a placeholder appeared in the content-addressed store: %v", err)
	}
	rows := queryRows(t, st, `SELECT storage_path, availability FROM blobs WHERE sha256 = ?`, sha256Hex)
	if len(rows) != 1 || rows[0]["availability"] != "unavailable" || rows[0]["storage_path"] != "" {
		t.Fatalf("blob row does not describe an unavailable object: %+v", rows)
	}
}

func TestMaterializationFetchesVerifiesAndInstalls(t *testing.T) {
	ctx := context.Background()
	for name, size := range map[string]int{
		"whole-object":   64 << 10,
		"chunked-object": 3*syncassets.ChunkBytes + 512,
		"exact-multiple": 2 * syncassets.ChunkBytes,
		"empty-object":   0,
	} {
		t.Run(name, func(t *testing.T) {
			content := generatedBytes(int64(size)+7, size)
			origin, receiver, resource, _ := attachedFixture(t, content, "application/octet-stream")

			if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
				t.Fatal(err)
			}
			report, err := receiver.MaterializeResources(ctx, NewLocalObjectProvider(origin), 8)
			if err != nil {
				t.Fatal(err)
			}
			if report.Materialized != 1 || report.Failed != 0 {
				t.Fatalf("report = %+v", report)
			}
			availability, err := receiver.ResourceAvailabilityFor(ctx, resource.ID)
			if err != nil || !availability.Available {
				t.Fatalf("availability = %+v err=%v", availability, err)
			}
			_, reader, err := receiver.OpenResourceContent(ctx, resource.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			var received bytes.Buffer
			if _, err := received.ReadFrom(reader); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(received.Bytes(), content) {
				t.Fatalf("materialized %d bytes, want %d", received.Len(), len(content))
			}
			// The staging area is left clean, not full of the object twice.
			if entries, err := os.ReadDir(filepath.Join(receiver.assetRoot, stagingDirectory)); err == nil && len(entries) != 0 {
				t.Fatalf("staging still holds %d entries", len(entries))
			}
		})
	}
}

// A source that hands over something other than what the protocol named must
// never get its bytes into canonical storage, however it lies.
func TestHostileSourcesAreRefused(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(9, 2*syncassets.ChunkBytes+64)

	cases := []struct {
		name       string
		provider   func(*SQLiteStore) ObjectProvider
		wantReason string
	}{
		{
			name:       "no-peer-has-the-bytes",
			provider:   func(*SQLiteStore) ObjectProvider { return failingProvider{} },
			wantReason: "no_source",
		},
		{
			name: "a-chunk-is-corrupted-in-flight",
			provider: func(origin *SQLiteStore) ObjectProvider {
				return &tamperingProvider{inner: NewLocalObjectProvider(origin), corrupts: true, corruptChunk: 1}
			},
			wantReason: "corrupt_chunk",
		},
		{
			name: "the-manifest-is-not-the-one-that-was-named",
			provider: func(origin *SQLiteStore) ObjectProvider {
				return &tamperingProvider{inner: NewLocalObjectProvider(origin), rewriteManifest: true}
			},
			wantReason: "manifest_digest_mismatch",
		},
		{
			name: "the-manifest-does-not-describe-the-object",
			provider: func(origin *SQLiteStore) ObjectProvider {
				return &tamperingProvider{inner: NewLocalObjectProvider(origin), invalidManifest: true}
			},
			wantReason: "invalid_manifest",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			origin, receiver, resource, _ := attachedFixture(t, content, "application/octet-stream")
			if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
				t.Fatal(err)
			}
			report, err := receiver.MaterializeResources(ctx, test.provider(origin), 8)
			if err != nil {
				t.Fatal(err)
			}
			if report.Materialized != 0 || report.Failed != 1 {
				t.Fatalf("report = %+v", report)
			}
			availability, err := receiver.ResourceAvailabilityFor(ctx, resource.ID)
			if err != nil {
				t.Fatal(err)
			}
			if availability.Available {
				t.Fatal("unverified bytes became available")
			}
			if availability.LastReason != test.wantReason {
				t.Fatalf("last reason = %q, want %q", availability.LastReason, test.wantReason)
			}
			assertNoStagedOrStoredBytes(t, receiver, resource.SHA256)
			// The metadata survives the failure: the reference is still there
			// and the object can be asked for again.
			if _, err := receiver.GetResource(ctx, resource.ID); err != nil {
				t.Fatalf("a failed fetch cost the replica its metadata: %v", err)
			}
		})
	}
}

// Content whose type contradicts what was advertised is refused, but only when
// sniffing is actually conclusive about it.
func TestMIMEMismatchIsRefusedAndOrdinaryTypesAreNot(t *testing.T) {
	ctx := context.Background()
	pdf := append([]byte("%PDF-1.7\n"), generatedBytes(4, 4096)...)

	origin, receiver, resource, _ := attachedFixture(t, pdf, "image/png")
	if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
		t.Fatal(err)
	}
	report, err := receiver.MaterializeResources(ctx, NewLocalObjectProvider(origin), 8)
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed != 1 {
		t.Fatalf("a PDF advertised as a PNG was accepted: %+v", report)
	}
	availability, _ := receiver.ResourceAvailabilityFor(ctx, resource.ID)
	if availability.LastReason != "mime_mismatch" || availability.Available {
		t.Fatalf("availability = %+v", availability)
	}

	// An attachment whose type sniffing cannot identify must still transfer.
	opaque := generatedBytes(5, 8192)
	origin2, receiver2, resource2, _ := attachedFixture(t, opaque, "application/vnd.oasis.opendocument.text")
	if err := receiver2.PinResource(ctx, resource2.ID, true); err != nil {
		t.Fatal(err)
	}
	report, err = receiver2.MaterializeResources(ctx, NewLocalObjectProvider(origin2), 8)
	if err != nil || report.Materialized != 1 {
		t.Fatalf("an unrecognizable but honest attachment was refused: %+v %v", report, err)
	}
}

func TestInterruptedTransferResumesFromVerifiedChunks(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(13, 4*syncassets.ChunkBytes+128)
	origin, receiver, resource, _ := attachedFixture(t, content, "application/octet-stream")
	if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
		t.Fatal(err)
	}

	// Fail after two chunks, as a dropped connection would.
	partial := &tamperingProvider{inner: NewLocalObjectProvider(origin), failAfterChunks: 2}
	report, err := receiver.MaterializeResources(ctx, partial, 8)
	if err != nil {
		t.Fatal(err)
	}
	if report.Materialized != 0 {
		t.Fatalf("a truncated transfer produced an object: %+v", report)
	}
	staged := queryRows(t, receiver, `SELECT ordinal FROM sync_blob_chunks WHERE blob_sha256 = ? AND fetched = 1 ORDER BY ordinal`, resource.SHA256)
	if len(staged) != 2 {
		t.Fatalf("expected two verified chunks to survive, got %d", len(staged))
	}

	// Resuming asks only for what is missing.
	counting := &countingProvider{inner: NewLocalObjectProvider(origin)}
	report, err = receiver.MaterializeResources(ctx, counting, 8)
	if err != nil || report.Materialized != 1 {
		t.Fatalf("resume failed: %+v %v", report, err)
	}
	if counting.chunks != 3 {
		t.Fatalf("resume fetched %d chunks, want the 3 that were missing", counting.chunks)
	}
	assertContentMatches(t, receiver, resource.ID, content)
}

// A staged chunk that was damaged while sitting on disk must not be mistaken
// for progress, or a resumed transfer would assemble a corrupt object.
func TestDamagedStagedChunkIsRefetched(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(17, 3*syncassets.ChunkBytes)
	origin, receiver, resource, _ := attachedFixture(t, content, "application/octet-stream")
	if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := receiver.MaterializeResources(ctx, &tamperingProvider{
		inner: NewLocalObjectProvider(origin), failAfterChunks: 2,
	}, 8); err != nil {
		t.Fatal(err)
	}
	damaged := filepath.Join(receiver.assetRoot, stagingDirectory, resource.SHA256, "0")
	if err := os.WriteFile(damaged, bytes.Repeat([]byte{0xff}, syncassets.ChunkBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	counting := &countingProvider{inner: NewLocalObjectProvider(origin)}
	report, err := receiver.MaterializeResources(ctx, counting, 8)
	if err != nil || report.Materialized != 1 {
		t.Fatalf("resume after damage failed: %+v %v", report, err)
	}
	if counting.chunks != 2 {
		t.Fatalf("refetched %d chunks; the damaged one and the missing one make 2", counting.chunks)
	}
	assertContentMatches(t, receiver, resource.ID, content)
}

func assertContentMatches(t *testing.T, st *SQLiteStore, resourceID string, want []byte) {
	t.Helper()
	_, reader, err := st.OpenResourceContent(context.Background(), resourceID)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var got bytes.Buffer
	if _, err := got.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("content mismatch: %d bytes, want %d", got.Len(), len(want))
	}
}

func TestPolicyDecidesWhatIsFetchedWithoutBeingAsked(t *testing.T) {
	ctx := context.Background()

	// A small attachment is eligible for automatic fetching under the default.
	small := generatedBytes(23, 32<<10)
	origin, receiver, resource, _ := attachedFixture(t, small, "application/octet-stream")
	availability, err := receiver.ResourceAvailabilityFor(ctx, resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !availability.Requested || availability.LastReason != "eager_small_object" {
		t.Fatalf("a small attachment was not requested automatically: %+v", availability)
	}
	report, err := receiver.MaterializeResources(ctx, NewLocalObjectProvider(origin), 8)
	if err != nil || report.Materialized != 1 {
		t.Fatalf("report = %+v err=%v", report, err)
	}

	// A large one waits, and says why.
	large := generatedBytes(29, 2*syncassets.ChunkBytes)
	originLarge, receiverLarge, largeResource, _ := attachedFixture(t, large, "application/octet-stream")
	availability, err = receiverLarge.ResourceAvailabilityFor(ctx, largeResource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if availability.Requested || availability.LastReason != "above_eager_threshold" {
		t.Fatalf("a large attachment was fetched without being asked for: %+v", availability)
	}
	if report, err := receiverLarge.MaterializeResources(ctx, NewLocalObjectProvider(originLarge), 8); err != nil || report.Considered != 0 {
		t.Fatalf("an unwanted object was fetched: %+v %v", report, err)
	}

	// Asking for it explicitly is enough; pinning is not required.
	if err := receiverLarge.RequestResource(ctx, largeResource.ID); err != nil {
		t.Fatal(err)
	}
	if report, err := receiverLarge.MaterializeResources(ctx, NewLocalObjectProvider(originLarge), 8); err != nil || report.Materialized != 1 {
		t.Fatalf("an explicitly requested object was not fetched: %+v %v", report, err)
	}
	assertContentMatches(t, receiverLarge, largeResource.ID, large)
}

func TestLazyPolicyFetchesNothingOnItsOwn(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(31, 16<<10)
	origin := newAdmissionReplica(t, "db_g8_lazy")
	receiver := newAdmissionReplica(t, "db_g8_lazy")
	receiver.SetMaterializationPolicy("lazy")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, receiver})

	document, err := origin.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_lazy", Title: "Lazy", Body: "x\n"})
	if err != nil {
		t.Fatal(err)
	}
	resource, err := origin.CreateResource(ctx, CreateResourceRequest{
		CollectionID: document.CollectionID, Filename: "small.bin",
		MIMEType: "application/octet-stream", Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	shipAll(t, origin, receiver)

	availability, err := receiver.ResourceAvailabilityFor(ctx, resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if availability.Requested || availability.LastReason != "lazy_policy" {
		t.Fatalf("a lazy replica fetched on its own: %+v", availability)
	}
	if report, err := receiver.MaterializeResources(ctx, NewLocalObjectProvider(origin), 8); err != nil || report.Considered != 0 {
		t.Fatalf("report = %+v err=%v", report, err)
	}
	// Pinning still works, which is what makes lazy a policy rather than a
	// refusal to synchronize attachments at all.
	if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
		t.Fatal(err)
	}
	if report, err := receiver.MaterializeResources(ctx, NewLocalObjectProvider(origin), 8); err != nil || report.Materialized != 1 {
		t.Fatalf("a pinned object was not fetched under the lazy policy: %+v %v", report, err)
	}
}

// The same bytes attached twice are one object. A replica that already holds
// them must not download them again just because a second resource names them.
func TestSharedBytesAreNotFetchedTwice(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(37, 48<<10)
	origin, receiver, first, _ := attachedFixture(t, content, "application/octet-stream")

	if err := receiver.PinResource(ctx, first.ID, true); err != nil {
		t.Fatal(err)
	}
	counting := &countingProvider{inner: NewLocalObjectProvider(origin)}
	if report, err := receiver.MaterializeResources(ctx, counting, 8); err != nil || report.Materialized != 1 {
		t.Fatalf("report = %+v err=%v", report, err)
	}
	fetchedOnce := counting.chunks

	second, err := origin.CreateResource(ctx, CreateResourceRequest{
		CollectionID: "default", Filename: "same-bytes-again.bin",
		MIMEType: "application/octet-stream", Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.SHA256 != first.SHA256 {
		t.Fatal("the fixture did not produce two resources over one object")
	}
	shipAll(t, origin, receiver)

	if report, err := receiver.MaterializeResources(ctx, counting, 8); err != nil || report.Considered != 0 {
		t.Fatalf("the second resource triggered a fetch: %+v %v", report, err)
	}
	if counting.chunks != fetchedOnce {
		t.Fatalf("bytes were fetched twice: %d then %d", fetchedOnce, counting.chunks)
	}
	availability, err := receiver.ResourceAvailabilityFor(ctx, second.ID)
	if err != nil || !availability.Available {
		t.Fatalf("the deduplicated resource is not available: %+v %v", availability, err)
	}
}

func TestMaterializationSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(41, 3*syncassets.ChunkBytes)
	directory := t.TempDir()
	originPath, receiverPath := filepath.Join(directory, "origin"), filepath.Join(directory, "receiver")

	origin := openDurableReplica(t, originPath, "db_g8_restart")
	receiver := openDurableReplica(t, receiverPath, "db_g8_restart")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, receiver})
	document, err := origin.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_restart", Title: "R", Body: "x\n"})
	if err != nil {
		t.Fatal(err)
	}
	resource, err := origin.CreateResource(ctx, CreateResourceRequest{
		CollectionID: document.CollectionID, Filename: "big.bin",
		MIMEType: "application/octet-stream", Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	shipAll(t, origin, receiver)
	if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := receiver.MaterializeResources(ctx, &tamperingProvider{
		inner: NewLocalObjectProvider(origin), failAfterChunks: 1,
	}, 8); err != nil {
		t.Fatal(err)
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openDurableReplica(t, receiverPath, "")
	availability, err := reopened.ResourceAvailabilityFor(ctx, resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if availability.Available || !availability.Pinned || availability.FetchedChunks != 1 {
		t.Fatalf("restart lost the transfer state: %+v", availability)
	}
	report, err := reopened.MaterializeResources(ctx, NewLocalObjectProvider(origin), 8)
	if err != nil || report.Materialized != 1 {
		t.Fatalf("resume after restart failed: %+v %v", report, err)
	}
	assertContentMatches(t, reopened, resource.ID, content)
}

func openDurableReplica(t *testing.T, directory, databaseID string) *SQLiteStore {
	t.Helper()
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Join(directory, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := OpenSQLiteWithAssetStore(filepath.Join(directory, "notes.sqlite"), filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if databaseID != "" {
		if _, err := st.AdoptDatabaseIdentity(ctx, databaseID); err != nil {
			t.Fatal(err)
		}
		if _, err := st.EnrollLocalJournal(ctx, "G8 fixture replica"); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

func TestMaterializationIsBoundedAndOrdered(t *testing.T) {
	ctx := context.Background()
	origin := newAdmissionReplica(t, "db_g8_bound")
	receiver := newAdmissionReplica(t, "db_g8_bound")
	receiver.SetMaterializationPolicy("lazy")
	configureAllAdmissionPeers(t, []*SQLiteStore{origin, receiver})

	var resources []Resource
	for index := 0; index < 6; index++ {
		resource, err := origin.CreateResource(ctx, CreateResourceRequest{
			CollectionID: "default", Filename: fmt.Sprintf("file-%d.bin", index),
			MIMEType: "application/octet-stream", Content: bytes.NewReader(generatedBytes(int64(index)+100, 4096+index)),
		})
		if err != nil {
			t.Fatal(err)
		}
		resources = append(resources, resource)
	}
	shipAll(t, origin, receiver)
	for _, resource := range resources {
		if err := receiver.RequestResource(ctx, resource.ID); err != nil {
			t.Fatal(err)
		}
	}
	// The limit is a bound on one pass, not a cap on the work overall.
	report, err := receiver.MaterializeResources(ctx, NewLocalObjectProvider(origin), 2)
	if err != nil {
		t.Fatal(err)
	}
	if report.Considered != 2 || report.Materialized != 2 {
		t.Fatalf("a bounded pass did more than it was allowed: %+v", report)
	}
	for pass := 0; pass < 3; pass++ {
		if _, err := receiver.MaterializeResources(ctx, NewLocalObjectProvider(origin), 2); err != nil {
			t.Fatal(err)
		}
	}
	for _, resource := range resources {
		availability, err := receiver.ResourceAvailabilityFor(ctx, resource.ID)
		if err != nil || !availability.Available {
			t.Fatalf("%s did not finish across bounded passes: %+v", resource.ID, availability)
		}
	}
}

// Materialization must never reach out to a remote-media URL: the only thing it
// talks to is the provider it was handed.
func TestMaterializationOnlyEverUsesItsProvider(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(53, 2*syncassets.ChunkBytes)
	origin, receiver, resource, _ := attachedFixture(t, content, "application/octet-stream")
	if err := receiver.PinResource(ctx, resource.ID, true); err != nil {
		t.Fatal(err)
	}
	recording := &countingProvider{inner: NewLocalObjectProvider(origin)}
	if _, err := receiver.MaterializeResources(ctx, recording, 8); err != nil {
		t.Fatal(err)
	}
	if recording.manifests != 1 || recording.chunks != 2 {
		t.Fatalf("provider calls = %d manifests, %d chunks", recording.manifests, recording.chunks)
	}
	for _, sha := range recording.requested {
		if sha != resource.SHA256 {
			t.Fatalf("the provider was asked for %q, which is not the object being fetched", sha)
		}
	}
	if _, err := receiver.MaterializeResources(ctx, nil, 8); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("materializing with no provider must be refused rather than improvised")
	}
}

type failingProvider struct{}

func (failingProvider) FetchManifest(context.Context, string) (syncassets.Manifest, error) {
	return syncassets.Manifest{}, errors.New("no peer currently holds this object")
}
func (failingProvider) FetchChunk(context.Context, string, int) ([]byte, error) {
	return nil, errors.New("no peer currently holds this object")
}

type countingProvider struct {
	inner     ObjectProvider
	mu        sync.Mutex
	manifests int
	chunks    int
	requested []string
}

func (p *countingProvider) FetchManifest(ctx context.Context, sha string) (syncassets.Manifest, error) {
	p.mu.Lock()
	p.manifests++
	p.requested = append(p.requested, sha)
	p.mu.Unlock()
	return p.inner.FetchManifest(ctx, sha)
}

func (p *countingProvider) FetchChunk(ctx context.Context, sha string, ordinal int) ([]byte, error) {
	p.mu.Lock()
	p.chunks++
	p.requested = append(p.requested, sha)
	p.mu.Unlock()
	return p.inner.FetchChunk(ctx, sha, ordinal)
}

type tamperingProvider struct {
	inner ObjectProvider
	// corruptChunk needs its own flag: ordinal zero is a real chunk, so a
	// bare int field would silently corrupt the first chunk of every provider
	// that only meant to drop the connection.
	corrupts        bool
	corruptChunk    int
	failAfterChunks int
	rewriteManifest bool
	invalidManifest bool
	served          int
}

func (p *tamperingProvider) FetchManifest(ctx context.Context, sha string) (syncassets.Manifest, error) {
	manifest, err := p.inner.FetchManifest(ctx, sha)
	if err != nil {
		return manifest, err
	}
	if p.invalidManifest {
		manifest.Chunks = manifest.Chunks[:len(manifest.Chunks)-1]
		return manifest, nil
	}
	if p.rewriteManifest {
		// Internally consistent, but describing chunks the sender chose. If a
		// receiver accepted this it would be verifying against the attacker's
		// hashes rather than the ones the protocol named.
		for index := range manifest.Chunks {
			manifest.Chunks[index].SHA256 = syncassets.SHA256Hex([]byte(fmt.Sprint("substitute", index)))
		}
	}
	return manifest, nil
}

func (p *tamperingProvider) FetchChunk(ctx context.Context, sha string, ordinal int) ([]byte, error) {
	if p.failAfterChunks > 0 && p.served >= p.failAfterChunks {
		return nil, errors.New("connection dropped")
	}
	content, err := p.inner.FetchChunk(ctx, sha, ordinal)
	if err != nil {
		return nil, err
	}
	p.served++
	if p.corrupts && p.corruptChunk == ordinal && len(content) > 0 {
		content = append([]byte(nil), content...)
		content[0] ^= 0xff
	}
	return content, nil
}

func TestSchemaV23BackfillsManifestsForExistingBlobs(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	path := filepath.Join(directory, "notes.sqlite")
	st, err := OpenSQLiteWithAssetStore(path, filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	content := generatedBytes(61, 2*syncassets.ChunkBytes+5)
	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		CollectionID: "default", Filename: "existing.bin",
		MIMEType: "application/octet-stream", Content: bytes.NewReader(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Rewind to a v22 database: the blob is local, nothing describes its shape.
	for _, statement := range []string{
		`DELETE FROM sync_blob_chunks`, `DELETE FROM sync_blob_manifests`, `PRAGMA user_version = 22`,
	} {
		if err := st.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLiteWithAssetStore(path, filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatalf("upgrade to v23: %v", err)
	}
	status, err := reopened.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema status: %+v err=%v", status, err)
	}
	manifests := queryRows(t, reopened, `SELECT chunk_count, byte_length, complete FROM sync_blob_manifests WHERE blob_sha256 = ?`, resource.SHA256)
	if len(manifests) != 1 || manifests[0]["chunk_count"] != "3" || manifests[0]["complete"] != "1" {
		t.Fatalf("manifest was not backfilled: %+v", manifests)
	}
	chunks := queryRows(t, reopened, `SELECT ordinal, fetched FROM sync_blob_chunks WHERE blob_sha256 = ? ORDER BY ordinal`, resource.SHA256)
	if len(chunks) != 3 {
		t.Fatalf("chunk rows = %+v", chunks)
	}
	for _, chunk := range chunks {
		if chunk["fetched"] != "1" {
			t.Fatalf("a local blob's chunks must be marked held: %+v", chunk)
		}
	}
	// An existing blob is local, and the upgrade must not suggest otherwise.
	blobs := queryRows(t, reopened, `SELECT availability FROM blobs WHERE sha256 = ?`, resource.SHA256)
	if len(blobs) != 1 || blobs[0]["availability"] != "local" {
		t.Fatalf("existing blob availability = %+v", blobs)
	}
}

func TestBlobRowsCannotClaimBytesTheyDoNotHave(t *testing.T) {
	ctx := context.Background()
	st := newAdmissionReplica(t, "db_g8_guard")
	cases := []string{
		`INSERT INTO blobs(sha256, storage_path, size_bytes, mime_type, availability)
		   VALUES('` + strings.Repeat("a", 64) + `', '', 10, 'text/plain', 'local')`,
		`INSERT INTO blobs(sha256, storage_path, size_bytes, mime_type, availability)
		   VALUES('` + strings.Repeat("b", 64) + `', 'sha256/aa/bb/cc', 10, 'text/plain', 'unavailable')`,
	}
	for _, statement := range cases {
		if err := st.Exec(ctx, statement); err == nil {
			t.Fatalf("a blob row that contradicts itself was accepted: %s", statement)
		}
	}
}

// An unmaterialized attachment must not be silently dropped from a full
// archive, and it must not make garbage collection look like corruption.
func TestUnavailableObjectsAreHandledByExportAndCollection(t *testing.T) {
	ctx := context.Background()
	content := generatedBytes(67, 96<<10)
	_, receiver, resource, _ := attachedFixture(t, content, "application/octet-stream")

	if _, _, err := receiver.OpenBlobContent(ctx, resource.SHA256); !errors.Is(err, ErrResourceUnavailable) {
		t.Fatalf("exporting an unmaterialized object must say so plainly: %v", err)
	}

	// Detaching leaves the resource unreferenced, so collection may remove it.
	if err := receiver.Exec(ctx, `DELETE FROM document_resource_refs`); err != nil {
		t.Fatal(err)
	}
	if err := receiver.Exec(ctx, `UPDATE resources SET unreferenced_at = created_at, unreferenced_reason = 'detached'`); err != nil {
		t.Fatal(err)
	}
	report, err := receiver.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{UnreferencedFor: 0}, Apply: true, Now: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "unsafe storage path") {
			t.Fatalf("collecting an unmaterialized object reported it as suspicious: %q", warning)
		}
	}
	// Every trace of the transfer goes with it, so a later admission of the
	// same bytes does not think it has already fetched them.
	for _, table := range []string{"sync_blob_chunks", "sync_blob_manifests", "sync_blob_sources", "sync_blob_materialization"} {
		rows := queryRows(t, receiver, `SELECT COUNT(*) AS total FROM `+table+` WHERE blob_sha256 = ?`, resource.SHA256)
		if rows[0]["total"] != "0" {
			t.Fatalf("%s still describes a collected object", table)
		}
	}
}
