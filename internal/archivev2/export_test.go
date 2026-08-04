package archivev2

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// fixedCreatedAt pins the only wall-clock manifest field so determinism
// assertions compare the whole archive rather than everything but the time.
var fixedCreatedAt = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

type exportFixture struct {
	store       *store.SQLiteStore
	public      store.Notebook
	child       store.Notebook
	publicDoc   store.Document
	childDoc    store.Document
	privateDoc  store.Document
	trashedDoc  store.Document
	resource    store.Resource
	sharedPairA store.Document
	sharedPairB store.Document
}

// newExportFixture builds a small but complete canonical database: nested
// notebooks, tags, resources, links across a selection boundary, provenance,
// an exact source bundle, a trashed note, and two notes with identical bodies.
func newExportFixture(t *testing.T) *exportFixture {
	t.Helper()
	ctx := context.Background()
	assets := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(t.TempDir(), "notes.sqlite"), assets)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	fixture := &exportFixture{store: st}
	if fixture.public, err = st.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: "nb_public", Name: "Public"}); err != nil {
		t.Fatal(err)
	}
	if fixture.child, err = st.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: "nb_child", ParentID: fixture.public.ID, Name: "Child"}); err != nil {
		t.Fatal(err)
	}
	if fixture.childDoc, err = st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_child", NotebookID: fixture.child.ID, Title: "Child", Body: "child body"}); err != nil {
		t.Fatal(err)
	}
	if fixture.privateDoc, err = st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_private", Title: "Private", Body: "private body"}); err != nil {
		t.Fatal(err)
	}

	resource, err := st.CreateResource(ctx, store.CreateResourceRequest{PreferredID: "res_image", Filename: "image.png", MIMEType: "image/png", Content: bytes.NewReader([]byte("\x89PNG\r\n\x1a\nfixture"))})
	if err != nil {
		t.Fatal(err)
	}
	fixture.resource = resource

	body := fmt.Sprintf("[child](%s) [private](%s) [missing](Missing Note) [away](https://example.com) ![image](%s)",
		fixture.childDoc.URI, fixture.privateDoc.URI, resource.URI)
	if fixture.publicDoc, err = st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_public", NotebookID: fixture.public.ID, Title: "Public", Body: body}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{DocumentID: fixture.publicDoc.ID, ResourceID: resource.ID, RelationType: "embedded"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDocumentTag(ctx, fixture.publicDoc.ID, "published"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDocumentTag(ctx, fixture.privateDoc.ID, "internal"); err != nil {
		t.Fatal(err)
	}
	// A second revision proves complete history travels with a full archive.
	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{ID: fixture.publicDoc.ID, Title: "Public", Body: body + "\n\nrevised", BaseRevisionID: fixture.publicDoc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{
		DocumentID: fixture.publicDoc.ID, SourceSystem: "joplin", ExternalID: "abc123",
		Author: "Author", AuthorID: "author-1", SourceURL: "https://example.com/post",
		MetadataJSON: `{"secret":"local-only"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutSourceBundleItem(ctx, store.PutSourceBundleItemRequest{
		SourceSystem: "joplin", SourceKey: "/home/user/private/export", CollectionID: "default",
		ItemKey: "/home/user/private/export/abc123.md", ItemType: "note", ExternalID: "abc123",
		RelativePath: "abc123.md", PropertyOrder: []string{"id", "title"},
		Content: bytes.NewReader([]byte("Public\n\nid: abc123\n")),
	}); err != nil {
		t.Fatal(err)
	}

	// Two notes with identical bodies must share exactly one body object.
	if fixture.sharedPairA, err = st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_shared_a", Title: "Shared A", Body: "identical"}); err != nil {
		t.Fatal(err)
	}
	if fixture.sharedPairB, err = st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_shared_b", Title: "Shared B", Body: "identical"}); err != nil {
		t.Fatal(err)
	}

	if fixture.trashedDoc, err = st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_trashed", Title: "Trashed", Body: "trashed body"}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: fixture.trashedDoc.ID, BaseRevisionID: fixture.trashedDoc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func exportOptions(target string) ExportOptions {
	return ExportOptions{Target: target, SnapshotID: "snap_fixture", CreatedAt: fixedCreatedAt}
}

func TestFullArchiveExportVerifiesAndCarriesCompleteHistory(t *testing.T) {
	fixture := newExportFixture(t)
	destination := filepath.Join(t.TempDir(), "archive")
	report, err := Export(context.Background(), fixture.store, destination, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}
	if !report.Verified || !report.FullBackup {
		t.Fatalf("expected a verified full backup: %+v", report)
	}
	if report.Counts.Documents != 6 {
		t.Fatalf("expected every note including the trashed one: %+v", report.Counts)
	}
	if report.Counts.Revisions != 8 {
		// Six creates plus one update plus the soft-delete revision.
		t.Fatalf("expected complete revision history, got %d", report.Counts.Revisions)
	}
	if report.Counts.Resources != 1 || report.Counts.DocumentResources != 1 || report.Counts.SourceBundles != 1 {
		t.Fatalf("unexpected attachment counts: %+v", report.Counts)
	}
	if report.Counts.SearchNotebooks != 2 || report.Counts.Tags != 2 {
		t.Fatalf("expected builtin search notebooks and both tags: %+v", report.Counts)
	}
	if report.ClearedLinkTargets != 0 {
		t.Fatalf("a full archive should not exclude any link target, got %d", report.ClearedLinkTargets)
	}
	verification, err := VerifyDirectory(destination, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if verification.CommitSHA256 != report.CommitSHA256 || verification.Counts != report.Counts {
		t.Fatalf("verification disagrees with the report: %+v vs %+v", verification, report)
	}

	records := decodeRecords(t, destination)
	// Identical bodies deduplicate to one object, and the trashed note keeps
	// its deletion timestamp.
	bodies := map[string]bool{}
	trashedSeen := false
	for _, record := range records[RecordRevision] {
		var revision RevisionRecord
		decode(t, record, &revision)
		bodies[revision.Body.SHA256] = true
	}
	for _, record := range records[RecordDocument] {
		var document DocumentRecord
		decode(t, record, &document)
		if document.ID == fixture.trashedDoc.ID {
			trashedSeen = true
			if document.DeletedAt == "" {
				t.Fatal("trashed note lost its deletion timestamp")
			}
		}
	}
	if !trashedSeen {
		t.Fatal("full archive omitted the trashed note")
	}
	sharedBodies := 0
	for _, record := range records[RecordRevision] {
		var revision RevisionRecord
		decode(t, record, &revision)
		if revision.DocumentID == fixture.sharedPairA.ID || revision.DocumentID == fixture.sharedPairB.ID {
			sharedBodies++
		}
	}
	if sharedBodies != 2 {
		t.Fatalf("expected two revisions with identical bodies, got %d", sharedBodies)
	}
	if report.DeduplicatedObjects < 1 {
		t.Fatalf("identical bodies were not deduplicated: %+v", report)
	}

	// Full archives preserve private source metadata and raw keys never leave
	// the store.
	var provenance ProvenanceRecord
	decode(t, records[RecordProvenance][0], &provenance)
	if !strings.Contains(string(provenance.MetadataJSON), "local-only") {
		t.Fatalf("full archive stripped private source metadata: %s", provenance.MetadataJSON)
	}
	var bundle SourceBundleRecord
	decode(t, records[RecordSourceBundle][0], &bundle)
	if bundle.RelativePath != "abc123.md" || len(bundle.SourceKeySHA256) != 64 {
		t.Fatalf("unexpected source bundle record: %+v", bundle)
	}
	raw, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"/home/user/private", assetRootMarker} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("manifest leaked a local path fragment %q", secret)
		}
	}
}

const assetRootMarker = "storage_path"

func TestSubsetExportScopesRecordsAndStripsPrivateMetadata(t *testing.T) {
	fixture := newExportFixture(t)
	destination := filepath.Join(t.TempDir(), "subset")
	options := exportOptions(TargetSubsetTransfer)
	options.Selection = store.SelectionSpec{NotebookIDs: []string{fixture.public.ID}}
	report, err := Export(context.Background(), fixture.store, destination, options)
	if err != nil {
		t.Fatal(err)
	}
	if report.FullBackup {
		t.Fatal("a notebook subset must not be reported as a full backup")
	}
	if !containsSubstring(report.Warnings, "not a complete database backup") {
		t.Fatalf("subset export must warn about scope: %v", report.Warnings)
	}
	if report.Counts.Documents != 2 {
		t.Fatalf("expected the public notebook subtree only: %+v", report.Counts)
	}
	if report.Counts.SearchNotebooks != 0 {
		t.Fatal("subset transfers must not carry whole-library search notebooks")
	}
	if report.Counts.Notebooks != 2 {
		t.Fatalf("expected the selected notebook plus its parent chain: %+v", report.Counts)
	}
	if report.Counts.Tags != 1 {
		t.Fatalf("expected only tags used by selected notes: %+v", report.Counts)
	}
	if _, err := VerifyDirectory(destination, DefaultLimits()); err != nil {
		t.Fatal(err)
	}

	records := decodeRecords(t, destination)
	var provenance ProvenanceRecord
	decode(t, records[RecordProvenance][0], &provenance)
	if string(provenance.MetadataJSON) != "{}" {
		t.Fatalf("subset transfer kept private source metadata: %s", provenance.MetadataJSON)
	}
	if provenance.SourceURL == "" {
		t.Fatal("subset transfer dropped permitted provenance identity")
	}
	cleared := 0
	for _, record := range records[RecordLink] {
		var link LinkRecord
		decode(t, record, &link)
		if link.ResolutionStatus == "target_excluded" {
			cleared++
			if link.TargetDocumentID != "" {
				t.Fatal("an excluded link target was still encoded")
			}
		}
	}
	if cleared != 1 || report.ClearedLinkTargets != 1 {
		t.Fatalf("expected exactly the link to the unselected private note, got %d/%d", cleared, report.ClearedLinkTargets)
	}
}

func TestExportIsDeterministicAndBindsThePlanDigest(t *testing.T) {
	fixture := newExportFixture(t)
	first := filepath.Join(t.TempDir(), "a")
	second := filepath.Join(t.TempDir(), "b")
	firstReport, err := Export(context.Background(), fixture.store, first, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}
	secondReport, err := Export(context.Background(), fixture.store, second, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}
	if firstReport.CommitSHA256 != secondReport.CommitSHA256 {
		t.Fatalf("identical state produced different commits: %s vs %s", firstReport.CommitSHA256, secondReport.CommitSHA256)
	}
	firstManifest, err := os.ReadFile(filepath.Join(first, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	secondManifest, err := os.ReadFile(filepath.Join(second, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstManifest, secondManifest) {
		t.Fatal("identical state produced different manifest bytes")
	}

	plan, err := fixture.store.PlanSelection(context.Background(), store.SelectionPlanRequest{Target: store.SelectionTargetFullArchive})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ManifestSHA256 != firstReport.SelectionManifestSHA256 {
		t.Fatalf("the archive did not bind the dry-run plan digest: %s vs %s", plan.ManifestSHA256, firstReport.SelectionManifestSHA256)
	}
}

func TestInterruptedExportLeavesNoApparentlyCompleteArchiveAndResumes(t *testing.T) {
	fixture := newExportFixture(t)
	destination := filepath.Join(t.TempDir(), "archive")

	// Simulate an interruption after objects exist but before the manifest is
	// published: run a real export, then drop the completion marker.
	if _, err := Export(context.Background(), fixture.store, destination, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(destination, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(destination, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "incomplete archive") {
		t.Fatalf("an object tree without a manifest must be incomplete, got %v", err)
	}
	// A stale object from an abandoned run must not survive into the archive.
	stale := filepath.Join(destination, "objects", "sha256", "00", strings.Repeat("0", 64))
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Export(context.Background(), fixture.store, destination, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}
	if report.ReusedObjects == 0 {
		t.Fatal("a resumed export rewrote every object instead of reusing published ones")
	}
	if report.BytesWritten >= report.Bytes {
		t.Fatalf("a resumed export should rewrite fewer bytes than it archives: %d of %d", report.BytesWritten, report.Bytes)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("the stale object was not pruned")
	}
	if _, err := VerifyDirectory(destination, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destination + stagingSuffix); !os.IsNotExist(err) {
		t.Fatal("the staging directory outlived the export")
	}
}

func TestExportRefusesUnsafeDestinationsAndUnsupportedModes(t *testing.T) {
	fixture := newExportFixture(t)
	ctx := context.Background()

	t.Run("existing complete archive", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "archive")
		if _, err := Export(ctx, fixture.store, destination, exportOptions(TargetFullArchive)); err != nil {
			t.Fatal(err)
		}
		if _, err := Export(ctx, fixture.store, destination, exportOptions(TargetFullArchive)); err == nil || !strings.Contains(err.Error(), "already holds a complete archive") {
			t.Fatalf("expected an overwrite refusal, got %v", err)
		}
		options := exportOptions(TargetFullArchive)
		options.Overwrite = true
		if _, err := Export(ctx, fixture.store, destination, options); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("foreign directory", func(t *testing.T) {
		destination := t.TempDir()
		if err := os.WriteFile(filepath.Join(destination, "notes.txt"), []byte("mine"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Export(ctx, fixture.store, destination, exportOptions(TargetFullArchive)); err == nil || !strings.Contains(err.Error(), "not an archive-v2 directory") {
			t.Fatalf("expected a foreign-directory refusal, got %v", err)
		}
	})

	t.Run("publication handoff", func(t *testing.T) {
		options := exportOptions(TargetPublicationHandoff)
		options.Selection = store.SelectionSpec{NotebookIDs: []string{fixture.public.ID}}
		if _, err := Export(ctx, fixture.store, filepath.Join(t.TempDir(), "publish"), options); err == nil || !strings.Contains(err.Error(), "publication profile slice") {
			t.Fatalf("expected publication handoff to be refused, got %v", err)
		}
	})

	t.Run("content rewriting policy", func(t *testing.T) {
		options := exportOptions(TargetSubsetTransfer)
		options.Selection = store.SelectionSpec{NotebookIDs: []string{fixture.public.ID}}
		options.Policy = store.PrivacyPolicy{LinkAction: "plain_text"}
		if _, err := Export(ctx, fixture.store, filepath.Join(t.TempDir(), "rewrite"), options); err == nil || !strings.Contains(err.Error(), "rewrites note content") {
			t.Fatalf("expected a link-rewriting refusal, got %v", err)
		}
	})

	t.Run("object budget", func(t *testing.T) {
		options := exportOptions(TargetFullArchive)
		options.Limits = DefaultLimits()
		options.Limits.MaxObjects = 2
		destination := filepath.Join(t.TempDir(), "bounded")
		if _, err := Export(ctx, fixture.store, destination, options); err == nil || !strings.Contains(err.Error(), "immutable objects") {
			t.Fatalf("expected an object-budget error, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(destination, "manifest.json")); !os.IsNotExist(err) {
			t.Fatal("a failed export published a manifest")
		}
	})

	t.Run("record chunking", func(t *testing.T) {
		options := exportOptions(TargetFullArchive)
		options.RecordsPerObject = 3
		destination := filepath.Join(t.TempDir(), "chunked")
		report, err := Export(ctx, fixture.store, destination, options)
		if err != nil {
			t.Fatal(err)
		}
		if report.RecordObjects < 2 {
			t.Fatalf("small chunks should produce several record objects: %+v", report)
		}
		if _, err := VerifyDirectory(destination, DefaultLimits()); err != nil {
			t.Fatal(err)
		}
	})
}

func containsSubstring(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func decodeRecords(t *testing.T, root string) map[string][]json.RawMessage {
	t.Helper()
	records := map[string][]json.RawMessage{}
	for _, entry := range readIndexEntries(t, root) {
		if entry.Kind != "records" {
			continue
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(entry.Location.Path)))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64<<10), 1<<20)
		for scanner.Scan() {
			var envelope RecordEnvelope
			if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			records[envelope.Type] = append(records[envelope.Type], envelope.Payload)
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return records
}

func decode(t *testing.T, raw json.RawMessage, target any) {
	t.Helper()
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}
