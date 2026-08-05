package archivev2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestManifestSizeIsIndependentOfArchiveSize is the regression test for the
// P3 container ceiling. Listing objects inline cost about 645 bytes each
// inside a 4 MiB manifest, which stopped an archive near 6,500 objects; with
// the inventory in index chunks the manifest stays flat while the object count
// grows.
func TestManifestSizeIsIndependentOfArchiveSize(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(directory, "notes.sqlite"), filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	const notes = 2000
	for index := 0; index < notes; index++ {
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_bulk_%06d", index),
			Title:       fmt.Sprintf("Bulk %06d", index),
			Body:        fmt.Sprintf("body %06d", index),
		}); err != nil {
			t.Fatal(err)
		}
	}

	destination := filepath.Join(directory, "archive")
	report, err := Export(ctx, st, destination, exportOptions(TargetFullArchive))
	if err != nil {
		t.Fatal(err)
	}
	if report.Objects <= notes {
		t.Fatalf("expected at least one object per note, got %d", report.Objects)
	}
	if !report.Verified {
		t.Fatal("bulk archive did not verify")
	}
	info, err := os.Stat(filepath.Join(destination, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The inline form would have needed roughly 1.3 MB here.
	if info.Size() > 32<<10 {
		t.Fatalf("manifest grew with the archive: %d bytes for %d objects", info.Size(), report.Objects)
	}
	if report.IndexObjects < 1 {
		t.Fatalf("archive published no index chunk: %+v", report)
	}
}

// TestIndexChunkingAndFanoutLayout covers the two layout decisions the
// container revision made: the index splits into bounded chunks, and objects
// use a two-level fanout so a large archive does not pile millions of files
// into 256 directories.
func TestIndexChunkingAndFanoutLayout(t *testing.T) {
	fixture := newExportFixture(t)
	destination := filepath.Join(t.TempDir(), "archive")
	options := exportOptions(TargetFullArchive)
	options.Limits = DefaultLimits()
	options.Limits.MaxIndexEntriesPerObject = 3
	report, err := Export(context.Background(), fixture.store, destination, options)
	if err != nil {
		t.Fatal(err)
	}
	if report.IndexObjects < 2 {
		t.Fatalf("small index chunks should produce several index objects: %+v", report)
	}
	if _, err := VerifyDirectory(destination, DefaultLimits()); err != nil {
		t.Fatal(err)
	}

	entries := readIndexEntries(t, destination)
	if len(entries) != report.Objects {
		t.Fatalf("index holds %d entries for %d objects", len(entries), report.Objects)
	}
	previous := ""
	for _, entry := range entries {
		if entry.Location.Layout != LayoutFanout {
			t.Fatalf("unexpected object layout %q", entry.Location.Layout)
		}
		want := "objects/sha256/" + entry.SHA256[:2] + "/" + entry.SHA256[2:4] + "/" + entry.SHA256
		if entry.Location.Path != want {
			t.Fatalf("object %s is at %q, want %q", entry.SHA256, entry.Location.Path, want)
		}
		if entry.SHA256 <= previous {
			t.Fatal("index entries are not sorted by unique object hash")
		}
		previous = entry.SHA256
	}
}

// TestCorruptIndexChunkIsRejected proves the checksum chain still reaches
// every object even though the manifest no longer names them: tampering with
// an index chunk breaks its own hash, and rehashing it breaks the manifest.
func TestCorruptIndexChunkIsRejected(t *testing.T) {
	fixture := newExportFixture(t)
	destination := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(context.Background(), fixture.store, destination, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, destination)
	path := filepath.Join(destination, filepath.FromSlash(manifest.Index[0].Location.Path))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 1
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(destination, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected an index checksum error, got %v", err)
	}
}

// TestFullArchiveUsesTheDirectPlannerBound guards the default the real
// 382,206-note corpus caught: PlanSelection caps at 100,000 documents for
// REST/MCP callers, but export runs in-process and must use the direct Store
// planner maximum or a real full backup is refused outright.
func TestFullArchiveUsesTheDirectPlannerBound(t *testing.T) {
	options, err := normalizeExportOptions(ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if options.MaxDocuments != store.MaxSelectionDocuments {
		t.Fatalf("default max_documents is %d, want the direct planner bound %d", options.MaxDocuments, store.MaxSelectionDocuments)
	}
	explicit, err := normalizeExportOptions(ExportOptions{MaxDocuments: 5})
	if err != nil {
		t.Fatal(err)
	}
	if explicit.MaxDocuments != 5 {
		t.Fatalf("an explicit bound was overridden: %d", explicit.MaxDocuments)
	}
}

func TestSpoolJoinDetectsMissingDuplicateAndUnreferenced(t *testing.T) {
	newPair := func(t *testing.T) (*keySpool, *keySpool) {
		t.Helper()
		root := t.TempDir()
		declarations, err := newKeySpool(root, "declarations")
		if err != nil {
			t.Fatal(err)
		}
		references, err := newKeySpool(root, "references")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = declarations.close()
			_ = references.close()
		})
		return declarations, references
	}
	add := func(t *testing.T, spool *keySpool, keys ...string) {
		t.Helper()
		for _, key := range keys {
			if err := spool.add(key); err != nil {
				t.Fatal(err)
			}
		}
		if err := spool.flush(); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("balanced", func(t *testing.T) {
		declarations, references := newPair(t)
		add(t, declarations, "blob:aa:1", "document:doc_a", "document:doc_b")
		add(t, references, "blob:aa:1", "document:doc_a", "document:doc_a")
		if err := joinSpools(declarations, references, keyBlob); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing declaration", func(t *testing.T) {
		declarations, references := newPair(t)
		add(t, declarations, "document:doc_a")
		add(t, references, "document:doc_missing")
		err := joinSpools(declarations, references, keyBlob)
		if err == nil || !strings.Contains(err.Error(), "missing or mismatched") {
			t.Fatalf("expected a missing-declaration error, got %v", err)
		}
	})

	t.Run("duplicate declaration", func(t *testing.T) {
		declarations, references := newPair(t)
		add(t, declarations, "document:doc_a", "document:doc_a")
		add(t, references, "document:doc_a")
		err := joinSpools(declarations, references, keyBlob)
		if err == nil || !strings.Contains(err.Error(), "duplicate archive declaration") {
			t.Fatalf("expected a duplicate error, got %v", err)
		}
	})

	t.Run("unreferenced blob", func(t *testing.T) {
		declarations, references := newPair(t)
		add(t, declarations, "blob:aa:1", "blob:bb:2")
		add(t, references, "blob:aa:1")
		err := joinSpools(declarations, references, keyBlob)
		if err == nil || !strings.Contains(err.Error(), "unreferenced archive object") {
			t.Fatalf("expected an unreferenced-blob error, got %v", err)
		}
	})

	t.Run("size mismatch is a missing declaration", func(t *testing.T) {
		declarations, references := newPair(t)
		add(t, declarations, "blob:aa:1")
		add(t, references, "blob:aa:2")
		err := joinSpools(declarations, references, keyBlob)
		if err == nil {
			t.Fatal("a blob referenced with the wrong length was accepted")
		}
	})

	t.Run("keys sort globally across buckets", func(t *testing.T) {
		root := t.TempDir()
		spool, err := newKeySpool(root, "keys")
		if err != nil {
			t.Fatal(err)
		}
		defer spool.close()
		want := []string{}
		for index := 0; index < 512; index++ {
			key := fmt.Sprintf("%064x", index*7919)
			want = append(want, key)
		}
		for index := len(want) - 1; index >= 0; index-- {
			if err := spool.add(want[index]); err != nil {
				t.Fatal(err)
			}
		}
		if err := spool.flush(); err != nil {
			t.Fatal(err)
		}
		got := []string{}
		if err := spool.forEachSorted(func(key string) error {
			got = append(got, key)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("spool returned %d keys, want %d", len(got), len(want))
		}
		for index := 1; index < len(got); index++ {
			if got[index] <= got[index-1] {
				t.Fatalf("spool keys are not globally sorted at %d", index)
			}
		}
	})
}

// TestArchivesWrittenBeforeOptionalRecordCountsStillVerify pins backward
// compatibility. `omitempty` never applied to a struct, so every archive
// written before RecordCounts became a pointer carries an explicit all-zero
// record_counts on its blob entries. Absence and an explicit zero mean the
// same thing — no records — and a reader that rejected the older spelling
// would make every archive already on disk unreadable for a cosmetic change.
func TestArchivesWrittenBeforeOptionalRecordCountsStillVerify(t *testing.T) {
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(context.Background(), fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, archive)

	// Rewrite one index chunk in the older spelling, then re-anchor the chain.
	target := manifest.Index[0]
	path := filepath.Join(archive, filepath.FromSlash(target.Location.Path))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := [][]byte{}
	restated := 0
	for _, line := range bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n")) {
		var entry IndexEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Kind == "blob" && entry.RecordCounts == nil {
			entry.RecordCounts = &Counts{}
			restated++
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		rewritten = append(rewritten, encoded)
	}
	if restated == 0 {
		t.Fatal("fixture had no blob entries to restate in the older form")
	}
	body := append(bytes.Join(rewritten, []byte("\n")), '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	updated := hex.EncodeToString(digest[:])
	moved := filepath.Join(archive, filepath.FromSlash(fanoutObjectPath(updated)))
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	manifest.Index[0].SHA256 = updated
	manifest.Index[0].SizeBytes = int64(len(body))
	manifest.Index[0].Location = newFanoutLocation(updated)
	writeManifest(t, archive, manifest, true)

	if _, err := VerifyDirectory(archive, DefaultLimits()); err != nil {
		t.Fatalf("an archive in the older record_counts spelling was rejected: %v", err)
	}
}
