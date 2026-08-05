package archivev2

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func newEmptyStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	directory := t.TempDir()
	target, err := store.OpenSQLiteWithAssetStore(filepath.Join(directory, "restored.sqlite"), filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = target.Close() })
	if err := target.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return target
}

// TestRestoreRoundTripPreservesCanonicalState is the core P4 assertion: what
// a v2 archive promises is what a restore reconstructs, including resource
// bytes, relations, revision history and provenance.
func TestRestoreRoundTripPreservesCanonicalState(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	exported, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}

	target := newEmptyStore(t)
	summary, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Applied.Documents != exported.Counts.Documents ||
		summary.Applied.Revisions != exported.Counts.Revisions ||
		summary.Applied.Resources != exported.Counts.Resources ||
		summary.Applied.DocumentResources != exported.Counts.DocumentResources ||
		summary.Applied.DocumentTags != exported.Counts.DocumentTags ||
		summary.Applied.Provenance != exported.Counts.Provenance ||
		summary.Applied.SourceBundles != exported.Counts.SourceBundles {
		t.Fatalf("restore applied %+v but the archive carried %+v", summary.Applied, exported.Counts)
	}

	// Adopt preserves the archive's database universe and mints a new replica.
	if summary.DatabaseID != exported.DatabaseID {
		t.Fatalf("adopt did not preserve the database universe: %s vs %s", summary.DatabaseID, exported.DatabaseID)
	}
	if summary.ReplicaID == exported.SourceReplicaID {
		t.Fatal("a restored writable copy reused the source replica ID")
	}

	// Note content survives exactly.
	restored, err := target.GetDocument(ctx, fixture.publicDoc.ID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := fixture.store.GetDocument(ctx, fixture.publicDoc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Title != original.Title || restored.Body != original.Body {
		t.Fatal("restored note title or body differs from the original")
	}
	if restored.NotebookID != original.NotebookID {
		t.Fatal("restored note lost its notebook")
	}

	// Complete revision history survives.
	originalRevisions, err := fixture.store.ListDocumentRevisions(ctx, fixture.publicDoc.ID)
	if err != nil {
		t.Fatal(err)
	}
	restoredRevisions, err := target.ListDocumentRevisions(ctx, fixture.publicDoc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restoredRevisions) != len(originalRevisions) {
		t.Fatalf("restored %d revisions, original had %d", len(restoredRevisions), len(originalRevisions))
	}

	// The trashed note is still trashed rather than resurrected.
	if _, err := target.GetDocument(ctx, fixture.trashedDoc.ID); err == nil {
		t.Fatal("a trashed note came back as a current note")
	}

	// Search works against restored content.
	hits, err := target.Search(ctx, store.SearchRequest{Query: "child", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits.Hits) == 0 {
		t.Fatal("restored library is not searchable")
	}
}

// TestRestoreRoundTripPreservesResourceBytes covers the attachment path the
// recipe corpora cannot exercise: exact bytes, the logical resource row, and
// the document relation must all survive.
func TestRestoreRoundTripPreservesResourceBytes(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	target := newEmptyStore(t)
	if _, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt}); err != nil {
		t.Fatal(err)
	}

	originalResource, originalContent, err := fixture.store.OpenResourceContent(ctx, fixture.resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	originalBytes, err := io.ReadAll(originalContent)
	_ = originalContent.Close()
	if err != nil {
		t.Fatal(err)
	}
	restoredResource, restoredContent, err := target.OpenResourceContent(ctx, fixture.resource.ID)
	if err != nil {
		t.Fatalf("restored library lost the resource: %v", err)
	}
	restoredBytes, err := io.ReadAll(restoredContent)
	_ = restoredContent.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredBytes) != string(originalBytes) {
		t.Fatal("restored resource bytes differ from the original")
	}
	if restoredResource.SHA256 != originalResource.SHA256 || restoredResource.SizeBytes != originalResource.SizeBytes {
		t.Fatalf("restored resource identity differs: %+v vs %+v", restoredResource, originalResource)
	}
	if restoredResource.Filename != originalResource.Filename || restoredResource.MIMEType != originalResource.MIMEType {
		t.Fatal("restored resource metadata differs from the original")
	}

	references, err := target.ListDocumentResources(ctx, fixture.publicDoc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 1 || references[0].ResourceID != fixture.resource.ID || references[0].RelationType != "embedded" {
		t.Fatalf("restored resource relation differs: %+v", references)
	}
}

// TestRestoreRefusesUnverifiedArchives proves the ordering guarantee: an
// archive that fails verification must not write anything at all.
func TestRestoreRefusesUnverifiedArchives(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	// Corrupt one blob so verification fails.
	corruptFirstBlob(t, archive)

	target := newEmptyStore(t)
	if _, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt}); err == nil ||
		!strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("expected a pre-write verification refusal, got %v", err)
	}
	empty, err := target.LibraryIsEmpty(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !empty {
		t.Fatal("a failed restore left canonical rows behind")
	}
}

// TestRestoreIntentIsMandatory keeps the P2 rule that no restore happens by
// default and that adopt refuses a populated target.
func TestRestoreIntentIsMandatory(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	target := newEmptyStore(t)
	if _, err := Restore(ctx, target, archive, RestoreOptions{}); err == nil {
		t.Fatal("a restore ran without an explicit intent")
	}
	if _, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt}); err != nil {
		t.Fatal(err)
	}
	// The target is no longer empty, so adopt must refuse a second time.
	if _, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt}); err == nil {
		t.Fatal("adopt accepted a populated target")
	}
}

func corruptFirstBlob(t *testing.T, root string) {
	t.Helper()
	for _, entry := range readIndexEntries(t, root) {
		if entry.Kind != "blob" || entry.SizeBytes == 0 {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(entry.Location.Path))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw[0] ^= 1
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("fixture has no blob to corrupt")
}

// TestFullArchiveCarriesUnreferencedResources guards a defect the attachment
// corpus found: the source held 758 resources and the archive carried 757,
// because reachability starts from selected documents. That is right for a
// subset and wrong for a backup — an unreferenced resource inside its
// retention window is live state garbage collection has not collected.
func TestFullArchiveCarriesUnreferencedResources(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	orphan, err := fixture.store.CreateResource(ctx, store.CreateResourceRequest{
		PreferredID: "res_orphan", Filename: "orphan.bin", MIMEType: "application/octet-stream",
		Content: strings.NewReader("orphaned bytes no note references"),
	})
	if err != nil {
		t.Fatal(err)
	}

	full := filepath.Join(t.TempDir(), "full")
	report, err := Export(ctx, fixture.store, full, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Resources != 2 {
		t.Fatalf("a full archive carried %d resources; both the attached and the unreferenced one belong in a backup", report.Counts.Resources)
	}

	target := newEmptyStore(t)
	if _, err := Restore(ctx, target, full, RestoreOptions{Intent: RestoreAdopt}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.GetResource(ctx, orphan.ID); err != nil {
		t.Fatalf("restore lost the unreferenced resource: %v", err)
	}

	// A scoped subset still carries only what its notes reach.
	subset := filepath.Join(t.TempDir(), "subset")
	subsetOptions := exportOptions(TargetSubsetTransfer)
	subsetOptions.Selection = store.SelectionSpec{NotebookIDs: []string{fixture.public.ID}}
	subsetReport, err := Export(ctx, fixture.store, subset, subsetOptions)
	if err != nil {
		t.Fatal(err)
	}
	if subsetReport.Counts.Resources != 1 {
		t.Fatalf("a subset transfer carried %d resources; it should carry only reachable ones", subsetReport.Counts.Resources)
	}
}

// TestRestoredSourceBundlesStayOutOfTheBlobStore guards the second defect the
// attachment corpus found: restore registered every source bundle as an
// ordinary blob, taking that table from 731 rows to 112,060 and exposing
// preserved source bytes to a garbage collector the schema deliberately keeps
// away from them.
func TestRestoredSourceBundlesStayOutOfTheBlobStore(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	target := newEmptyStore(t)
	summary, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Applied.SourceBundles == 0 {
		t.Fatal("fixture carried no source bundle to check")
	}

	// The blob store must hold resource blobs and revision bodies only.
	sourceReport, err := fixture.store.ResourceReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	restoredReport, err := target.ResourceReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(restoredReport.UnreferencedBlobs) > len(sourceReport.UnreferencedBlobs)+1 {
		t.Fatalf("restore added unreferenced blobs: %d versus %d in the source",
			len(restoredReport.UnreferencedBlobs), len(sourceReport.UnreferencedBlobs))
	}

	// And a restored bundle must actually be readable at the path it records.
	item, content, err := target.OpenSourceBundleItem(ctx, "joplin",
		sha256Hex("/home/user/private/export"), "default",
		sha256Hex("/home/user/private/export/abc123.md"))
	if err != nil {
		t.Fatalf("restored source bundle is unreadable: %v", err)
	}
	raw, err := io.ReadAll(content)
	_ = content.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "Public\n\nid: abc123\n" {
		t.Fatal("restored source bundle bytes differ from the original")
	}
	if !strings.HasPrefix(item.StoragePath, "source-bundles/") {
		t.Fatalf("restored bundle stored outside its namespace: %q", item.StoragePath)
	}
}
