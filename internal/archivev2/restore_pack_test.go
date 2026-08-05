package archivev2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// archivedObjects fingerprints what an archive carries, independently of how it
// stores it. Every record object's hash covers the record JSON inside it, and
// every blob object's hash covers its bytes, so two archives with equal sets
// carry equal content. Pack objects are containers rather than content and are
// excluded: they are exactly the thing the layout changes.
func archivedObjects(t *testing.T, root string) []string {
	t.Helper()
	hashes := []string{}
	for _, entry := range readIndexEntries(t, root) {
		if entry.Kind == "pack" {
			continue
		}
		hashes = append(hashes, entry.Kind+":"+entry.SHA256)
	}
	sort.Strings(hashes)
	return hashes
}

// TestPackedAndLooseArchivesRestoreToTheSameLibrary is the central packed-restore
// assertion: the object layout is a storage decision, so it must be invisible in
// the restored library. Both archives are restored, then re-exported loosely;
// equal object sets mean equal canonical state, not merely equal row counts.
func TestPackedAndLooseArchivesRestoreToTheSameLibrary(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	loose := filepath.Join(t.TempDir(), "loose")
	packed := filepath.Join(t.TempDir(), "packed")
	if _, err := Export(ctx, fixture.store, loose, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	packedReport, err := Export(ctx, fixture.store, packed, packedOptions())
	if err != nil {
		t.Fatal(err)
	}
	if packedReport.PackObjects == 0 {
		t.Fatalf("fixture did not produce packs: %+v", packedReport)
	}

	fromLoose, fromPacked := newEmptyStore(t), newEmptyStore(t)
	looseSummary, err := Restore(ctx, fromLoose, loose, RestoreOptions{Intent: RestoreAdopt})
	if err != nil {
		t.Fatal(err)
	}
	packedSummary, err := Restore(ctx, fromPacked, packed, RestoreOptions{Intent: RestoreAdopt})
	if err != nil {
		t.Fatalf("restoring a packed archive failed: %v", err)
	}
	if packedSummary.Applied != looseSummary.Applied {
		t.Fatalf("layout changed what was restored: %+v vs %+v", packedSummary.Applied, looseSummary.Applied)
	}
	if packedSummary.Blobs != looseSummary.Blobs || packedSummary.BlobBytes != looseSummary.BlobBytes {
		t.Fatalf("layout changed admitted blobs: %d/%d vs %d/%d",
			packedSummary.Blobs, packedSummary.BlobBytes, looseSummary.Blobs, looseSummary.BlobBytes)
	}

	// Re-export both restored libraries the same way and compare content.
	reLoose := filepath.Join(t.TempDir(), "re-loose")
	rePacked := filepath.Join(t.TempDir(), "re-packed")
	if _, err := Export(ctx, fromLoose, reLoose, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(ctx, fromPacked, rePacked, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	fromLooseObjects, fromPackedObjects := archivedObjects(t, reLoose), archivedObjects(t, rePacked)
	if strings.Join(fromPackedObjects, "\n") != strings.Join(fromLooseObjects, "\n") {
		t.Fatalf("packed restore produced different canonical content: %d objects vs %d",
			len(fromPackedObjects), len(fromLooseObjects))
	}
}

// TestPackedRestorePreservesResourceBytesAndSourceBundles reads the attachment
// and exact-source paths back out of packs. Resource bytes and bundle bytes
// reach the restored store as pack slices rather than whole files, so an
// off-by-one in slice bounds would corrupt them while every count still agreed.
func TestPackedRestorePreservesResourceBytesAndSourceBundles(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	packed := filepath.Join(t.TempDir(), "packed")
	if _, err := Export(ctx, fixture.store, packed, packedOptions()); err != nil {
		t.Fatal(err)
	}
	target := newEmptyStore(t)
	if _, err := Restore(ctx, target, packed, RestoreOptions{Intent: RestoreAdopt}); err != nil {
		t.Fatal(err)
	}

	originalResource, originalReader, err := fixture.store.OpenResourceContent(ctx, fixture.resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	originalBytes, err := io.ReadAll(originalReader)
	_ = originalReader.Close()
	if err != nil {
		t.Fatal(err)
	}
	restoredResource, restoredReader, err := target.OpenResourceContent(ctx, fixture.resource.ID)
	if err != nil {
		t.Fatalf("packed restore lost the resource: %v", err)
	}
	restoredBytes, err := io.ReadAll(restoredReader)
	_ = restoredReader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restoredBytes, originalBytes) {
		t.Fatal("resource bytes read out of a pack differ from the original")
	}
	if restoredResource.SHA256 != originalResource.SHA256 || restoredResource.SizeBytes != originalResource.SizeBytes {
		t.Fatalf("resource identity differs after a packed restore: %+v vs %+v", restoredResource, originalResource)
	}

	item, bundle, err := target.OpenSourceBundleItem(ctx, "joplin",
		sha256Hex("/home/user/private/export"), "default",
		sha256Hex("/home/user/private/export/abc123.md"))
	if err != nil {
		t.Fatalf("packed restore lost the exact source bundle: %v", err)
	}
	bundleBytes, err := io.ReadAll(bundle)
	_ = bundle.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(bundleBytes) != "Public\n\nid: abc123\n" {
		t.Fatalf("source bundle bytes read out of a pack differ: %q", string(bundleBytes))
	}
	if !strings.HasPrefix(item.StoragePath, "source-bundles/") {
		t.Fatalf("packed restore put a bundle outside its namespace: %s", item.StoragePath)
	}
}

// TestPackedRestoreRefusesCorruptedPacksBeforeWriting keeps the ordering
// guarantee under the packed layout: verification covers pack slices too, so a
// tampered pack must stop the restore before the first canonical write.
func TestPackedRestoreRefusesCorruptedPacksBeforeWriting(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	packed := filepath.Join(t.TempDir(), "packed")
	if _, err := Export(ctx, fixture.store, packed, packedOptions()); err != nil {
		t.Fatal(err)
	}
	var packPath string
	for _, entry := range readIndexEntries(t, packed) {
		if entry.Kind == "pack" {
			packPath = entry.Location.Path
			break
		}
	}
	if packPath == "" {
		t.Fatal("no pack object in the index")
	}
	full := filepath.Join(packed, filepath.FromSlash(packPath))
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 1
	if err := os.WriteFile(full, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	target := newEmptyStore(t)
	if _, err := Restore(ctx, target, packed, RestoreOptions{Intent: RestoreAdopt}); err == nil {
		t.Fatal("a corrupted packed archive was restored")
	}
	empty, err := target.LibraryIsEmpty(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !empty {
		t.Fatal("a refused packed restore still wrote canonical rows")
	}
}

// TestPackHandleCacheKeepsBorrowedPacksOpen pins the cache contract itself,
// independently of any fixture's geometry: eviction may close idle handles to
// stay under the bound, but never one a reader still holds.
func TestPackHandleCacheKeepsBorrowedPacksOpen(t *testing.T) {
	ctx := context.Background()
	packed := filepath.Join(t.TempDir(), "packed")
	if _, err := Export(ctx, newPackSpanningFixture(t, maxOpenPacks*4), packed, packedOptions()); err != nil {
		t.Fatal(err)
	}
	packs := []string{}
	for _, entry := range readIndexEntries(t, packed) {
		if entry.Kind == "pack" {
			packs = append(packs, entry.SHA256)
		}
	}
	if len(packs) <= maxOpenPacks {
		t.Fatalf("need more than %d packs to force eviction, got %d", maxOpenPacks, len(packs))
	}

	source := newPackSource(DefaultLimits())
	defer source.close()
	held, err := source.acquire(packed, packs[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, hash := range packs[1:] {
		handle, err := source.acquire(packed, hash)
		if err != nil {
			t.Fatal(err)
		}
		source.release(handle)
	}
	if len(source.open_) > maxOpenPacks {
		t.Fatalf("cache grew past its bound with only one borrowed handle: %d", len(source.open_))
	}
	if _, err := held.file.ReadAt(make([]byte, 1), 0); err != nil {
		t.Fatalf("a borrowed pack was evicted while still in use: %v", err)
	}

	// Once released it becomes an ordinary eviction candidate again.
	source.release(held)
	for _, hash := range packs[1:] {
		handle, err := source.acquire(packed, hash)
		if err != nil {
			t.Fatal(err)
		}
		source.release(handle)
	}
	if len(source.open_) > maxOpenPacks {
		t.Fatalf("released handles stopped being evictable: %d open", len(source.open_))
	}
}

// newPackSpanningFixture builds a library that forces restore to hold a reader
// into one pack while opening many others. Each note body is larger than the
// pack roll threshold, so bodies land in separate packs; the titles are long
// enough that one revision record object cannot be consumed in a single
// buffered read, so the reader over its pack stays live across those opens.
//
// A small fixture never reaches this state, which is why loose-only coverage
// said nothing about it.
func newPackSpanningFixture(t *testing.T, notes int) *store.SQLiteStore {
	t.Helper()
	ctx := context.Background()
	directory := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(directory, "notes.sqlite"), filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < notes; i++ {
		body := fmt.Sprintf("note-%03d ", i) + strings.Repeat(fmt.Sprintf("%03d-", i), 512)
		title := fmt.Sprintf("note-%03d ", i) + strings.Repeat("t", 4096)
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_%03d", i), Title: title, Body: body,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

// TestPackedRestoreSpansMorePacksThanTheHandleCache proves the pack handle
// cache cannot invalidate a reader that restore is still using. The cache is
// bounded on purpose — packs are large — so eviction is correct, but evicting a
// pack whose slice is still being read is not.
func TestPackedRestoreSpansMorePacksThanTheHandleCache(t *testing.T) {
	ctx := context.Background()
	source := newPackSpanningFixture(t, maxOpenPacks*4)
	packed := filepath.Join(t.TempDir(), "packed")
	report, err := Export(ctx, source, packed, packedOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.PackObjects <= maxOpenPacks {
		t.Fatalf("fixture produced %d packs, need more than the %d-handle cache", report.PackObjects, maxOpenPacks)
	}

	target := newEmptyStore(t)
	summary, err := Restore(ctx, target, packed, RestoreOptions{Intent: RestoreAdopt})
	if err != nil {
		t.Fatalf("restore failed while reading across more packs than the cache holds: %v", err)
	}
	if summary.Applied.Documents != maxOpenPacks*4 {
		t.Fatalf("restored %d notes, expected %d", summary.Applied.Documents, maxOpenPacks*4)
	}
	// Bodies come out of pack slices; a stale handle would truncate or mix them.
	for i := 0; i < maxOpenPacks*4; i++ {
		id := fmt.Sprintf("doc_%03d", i)
		restored, err := target.GetDocument(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		original, err := source.GetDocument(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if restored.Body != original.Body || restored.Title != original.Title {
			t.Fatalf("note %s did not survive a restore spanning many packs", id)
		}
	}
}

// TestPacksCarryEachObjectOnce is the packed-layout half of deduplication. The
// loose layout gets it free — identical objects address the same path — but a
// pack writer only learns an object's hash after streaming it, so packs used to
// carry duplicate copies of bytes the archive already held.
func TestPacksCarryEachObjectOnce(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	packed := filepath.Join(t.TempDir(), "packed")
	report, err := Export(ctx, fixture.store, packed, packedOptions())
	if err != nil {
		t.Fatal(err)
	}
	// The fixture deliberately holds two notes with identical bodies.
	if report.DeduplicatedObjects == 0 {
		t.Fatalf("packed export collapsed nothing: %+v", report)
	}

	seen := map[string]int{}
	for _, entry := range readIndexEntries(t, packed) {
		if entry.Kind == "pack" {
			continue
		}
		seen[entry.SHA256]++
	}
	for hash, count := range seen {
		if count > 1 {
			t.Fatalf("object %s is indexed %d times", hash, count)
		}
	}

	// The bytes themselves, not merely the index, must appear once per pack.
	inPacks := map[string]int{}
	for _, entry := range readIndexEntries(t, packed) {
		if entry.Kind != "pack" {
			continue
		}
		file, err := os.Open(filepath.Join(packed, filepath.FromSlash(entry.Location.Path)))
		if err != nil {
			t.Fatal(err)
		}
		entries, err := readPackTrailer(file, DefaultLimits())
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, packed := range entries {
			inPacks[packed.SHA256]++
		}
	}
	for hash, count := range inPacks {
		if count > 1 {
			t.Fatalf("pack files carry object %s %d times", hash, count)
		}
	}
	if _, err := VerifyDirectory(packed, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
}
