package archivev2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packedOptions() ExportOptions {
	options := exportOptions(TargetFullArchive)
	options.Pack = true
	// Small packs so the fixture exercises more than one container.
	options.PackTargetBytes = 512
	return options
}

// TestPackedExportVerifiesAndCollapsesFiles is the point of the layout: the
// same archive content, far fewer filesystem objects.
func TestPackedExportVerifiesAndCollapsesFiles(t *testing.T) {
	fixture := newExportFixture(t)
	loose := filepath.Join(t.TempDir(), "loose")
	packed := filepath.Join(t.TempDir(), "packed")

	looseReport, err := Export(context.Background(), fixture.store, loose, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}
	packedReport, err := Export(context.Background(), fixture.store, packed, packedOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !packedReport.Verified || !packedReport.FullBackup {
		t.Fatalf("packed archive did not verify: %+v", packedReport)
	}
	if packedReport.PackObjects == 0 {
		t.Fatalf("packed export produced no packs: %+v", packedReport)
	}
	if packedReport.Counts != looseReport.Counts {
		t.Fatalf("layout changed the archived records: %+v vs %+v", packedReport.Counts, looseReport.Counts)
	}
	if packedReport.SelectionManifestSHA256 != looseReport.SelectionManifestSHA256 {
		t.Fatal("layout changed the bound selection digest")
	}
	if countFiles(t, packed) >= countFiles(t, loose) {
		t.Fatalf("packing did not reduce file count: %d packed vs %d loose", countFiles(t, packed), countFiles(t, loose))
	}
	if _, err := VerifyDirectory(packed, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
}

// TestPackedArchiveDeclaresItsCapability keeps a pack-unaware reader safe: it
// must reject rather than misread.
func TestPackedArchiveDeclaresItsCapability(t *testing.T) {
	fixture := newExportFixture(t)
	packed := filepath.Join(t.TempDir(), "packed")
	if _, err := Export(context.Background(), fixture.store, packed, packedOptions()); err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, packed)
	if !contains(manifest.Compatibility.RequiredCapabilities, CapabilityObjectPack) {
		t.Fatalf("packed archive does not require %s", CapabilityObjectPack)
	}

	loose := filepath.Join(t.TempDir(), "loose")
	if _, err := Export(context.Background(), fixture.store, loose, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	if contains(readManifest(t, loose).Compatibility.RequiredCapabilities, CapabilityObjectPack) {
		t.Fatal("a loose archive should not require pack support")
	}

	// Dropping the declaration while still using packs must be refused.
	stripped := []string{}
	for _, capability := range manifest.Compatibility.RequiredCapabilities {
		if capability != CapabilityObjectPack {
			stripped = append(stripped, capability)
		}
	}
	manifest.Compatibility.RequiredCapabilities = stripped
	writeManifest(t, packed, manifest, true)
	if _, err := VerifyDirectory(packed, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "uses packs without requiring") {
		t.Fatalf("expected an undeclared-pack rejection, got %v", err)
	}
}

// TestPackedObjectCorruptionIsRejected proves placement cannot launder
// content: a packed object still has to hash to its own identity.
func TestPackedObjectCorruptionIsRejected(t *testing.T) {
	fixture := newExportFixture(t)
	packed := filepath.Join(t.TempDir(), "packed")
	if _, err := Export(context.Background(), fixture.store, packed, packedOptions()); err != nil {
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
	if _, err := VerifyDirectory(packed, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected a checksum failure, got %v", err)
	}
}

// TestPackTrailerIsSelfDescribing checks the property that lets a pack be
// verified, and later resumed from, without the archive index.
func TestPackTrailerIsSelfDescribing(t *testing.T) {
	fixture := newExportFixture(t)
	packed := filepath.Join(t.TempDir(), "packed")
	if _, err := Export(context.Background(), fixture.store, packed, packedOptions()); err != nil {
		t.Fatal(err)
	}
	entries := readIndexEntries(t, packed)
	contained := map[string]bool{}
	for _, entry := range entries {
		if entry.Location.Layout == LayoutPack {
			contained[entry.SHA256] = true
		}
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Kind != "pack" {
			continue
		}
		file, err := os.Open(filepath.Join(packed, filepath.FromSlash(entry.Location.Path)))
		if err != nil {
			t.Fatal(err)
		}
		trailer, err := readPackTrailer(file, DefaultLimits())
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(trailer) == 0 {
			t.Fatalf("pack %s has an empty trailer", entry.SHA256)
		}
		for _, item := range trailer {
			if !contained[item.SHA256] {
				t.Fatalf("pack trailer names %s, which the index does not place in a pack", item.SHA256)
			}
			seen[item.SHA256] = true
		}
	}
	// Trailers may list the same content twice: packed writing cannot consult
	// the filesystem for deduplication, so duplicate bytes are written and the
	// duplicate index entries collapse when the index is published. Every
	// packed object must still be described exactly once as a set.
	if len(seen) != len(contained) {
		t.Fatalf("pack trailers describe %d distinct objects but the index places %d", len(seen), len(contained))
	}
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

// TestPackedTotalsCountStorageBytesOnce guards a defect the real-corpus A/B
// exposed: summing every index entry counted a packed object's bytes twice,
// once for the object and once inside its pack, which inflated the archive's
// measured size and halved the effective MaxTotalBytes bound.
func TestPackedTotalsCountStorageBytesOnce(t *testing.T) {
	fixture := newExportFixture(t)
	packed := filepath.Join(t.TempDir(), "packed")
	report, err := Export(context.Background(), fixture.store, packed, packedOptions())
	if err != nil {
		t.Fatal(err)
	}
	// totals.Bytes covers objects; index chunks are listed by the manifest
	// rather than by the index, so they are excluded here too.
	skip := map[string]bool{filepath.Join(packed, "manifest.json"): true}
	for _, indexObject := range readManifest(t, packed).Index {
		skip[filepath.Join(packed, filepath.FromSlash(indexObject.Location.Path))] = true
	}
	var onDisk int64
	err = filepath.WalkDir(packed, func(current string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if skip[current] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		onDisk += info.Size()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Bytes != onDisk {
		t.Fatalf("manifest totals report %d object bytes but the archive holds %d on disk", report.Bytes, onDisk)
	}
}
