package archivev2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The golden fixture is built here rather than by Export on purpose: the
// verifier must be checked against an archive it did not produce, so a writer
// bug cannot become the expected result. Set NOTRIOS_UPDATE_GOLDEN=1 to
// rewrite the committed fixture after a deliberate format change.
const goldenTimestamp = "2026-08-03T17:00:00Z"

var goldenBlobs = []struct {
	content   string
	mediaType string
}{
	{content: "hello\n", mediaType: "text/markdown; charset=utf-8"},
	{content: "title\nid: alpha\n", mediaType: "application/octet-stream"},
	{content: "PNG\n", mediaType: "image/png"},
}

func goldenRecordLines() []string {
	body := goldenDigest(goldenBlobs[0].content)
	bundle := goldenDigest(goldenBlobs[1].content)
	image := goldenDigest(goldenBlobs[2].content)
	return []string{
		`{"type":"collection","payload":{"id":"default","name":"Default","capabilities":["documents","links","resources"],"settings_json":{},"created_at":"` + goldenTimestamp + `"}}`,
		`{"type":"notebook","payload":{"id":"nb_notes","name":"Notes","builtin":false,"position":0,"created_at":"` + goldenTimestamp + `","updated_at":"` + goldenTimestamp + `"}}`,
		`{"type":"search_notebook","payload":{"id":"snb_all_notes","name":"All notes","query":"","builtin":true,"sort_anchor":"first","created_at":"` + goldenTimestamp + `"}}`,
		`{"type":"tag","payload":{"id":"tag_todo","name":"todo","created_at":"` + goldenTimestamp + `"}}`,
		`{"type":"document","payload":{"id":"doc_alpha","collection_id":"default","notebook_id":"nb_notes","current_revision_id":"rev_alpha","created_at":"` + goldenTimestamp + `","updated_at":"` + goldenTimestamp + `"}}`,
		`{"type":"revision","payload":{"id":"rev_alpha","document_id":"doc_alpha","title":"Alpha","body":{"sha256":"` + body + `","size_bytes":6,"media_type":"text/markdown; charset=utf-8"},"body_mime_type":"text/markdown; charset=utf-8","metadata_json":{},"message":"created","created_at":"` + goldenTimestamp + `"}}`,
		`{"type":"document_tag","payload":{"document_id":"doc_alpha","tag_id":"tag_todo"}}`,
		`{"type":"resource","payload":{"id":"res_image","collection_id":"default","blob":{"sha256":"` + image + `","size_bytes":4,"media_type":"image/png"},"filename":"image.png","mime_type":"image/png","metadata_json":{},"created_at":"` + goldenTimestamp + `"}}`,
		`{"type":"document_resource","payload":{"document_id":"doc_alpha","resource_id":"res_image","relation_type":"attachment","ordinal":0,"anchor_json":{}}}`,
		`{"type":"link","payload":{"id":"link_self","source_document_id":"doc_alpha","target_document_id":"doc_alpha","target_uri":"document://default/documents/doc_alpha","relation_type":"link","source_format":"markdown","raw_target":"notrios://databases/db_archive/documents/doc_alpha","source_start_byte":0,"source_end_byte":12,"source_line":1,"source_column":1,"resolution_status":"resolved"}}`,
		`{"type":"provenance","payload":{"document_id":"doc_alpha","source_system":"joplin","external_id":"source-alpha","metadata_json":{},"created_at":"` + goldenTimestamp + `","updated_at":"` + goldenTimestamp + `"}}`,
		`{"type":"source_bundle","payload":{"source_system":"joplin","source_key_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","collection_id":"default","item_key_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","item_type":"note","external_id":"source-alpha","relative_path":"alpha.md","content":{"sha256":"` + bundle + `","size_bytes":16,"media_type":"application/octet-stream"},"property_order_json":["id","title"],"updated_at":"` + goldenTimestamp + `"}}`,
	}
}

func goldenDigest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// buildGoldenArchive writes a complete manifest-last archive containing one
// record of every type plus its three referenced blobs.
func buildGoldenArchive(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []IndexEntry{}
	write := func(content, mediaType, kind string, records int, counts Counts) IndexEntry {
		hash := goldenDigest(content)
		entry := IndexEntry{
			SHA256: hash, Kind: kind, MediaType: mediaType,
			SizeBytes: int64(len(content)), Records: records, RecordCounts: counts,
			Location: newFanoutLocation(hash),
		}
		path := filepath.Join(root, filepath.FromSlash(entry.Location.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return entry
	}
	for _, blob := range goldenBlobs {
		entries = append(entries, write(blob.content, blob.mediaType, "blob", 0, Counts{}))
	}
	lines := goldenRecordLines()
	recordCounts := Counts{
		Collections: 1, Notebooks: 1, SearchNotebooks: 1, Tags: 1, Documents: 1, Revisions: 1,
		DocumentTags: 1, Resources: 1, DocumentResources: 1, Links: 1, Provenance: 1, SourceBundles: 1,
	}
	records := strings.Join(lines, "\n") + "\n"
	entries = append(entries, write(records, RecordsMediaType, "records", len(lines), recordCounts))

	sort.Slice(entries, func(i, j int) bool { return entries[i].SHA256 < entries[j].SHA256 })
	var indexChunk strings.Builder
	var totals ObjectTotals
	for _, entry := range entries {
		encoded, err := encodeIndexEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		indexChunk.Write(encoded)
		totals.Objects++
		totals.Bytes += entry.SizeBytes
		if entry.Kind == "records" {
			totals.RecordChunks++
		} else {
			totals.Blobs++
		}
	}
	indexEntry := write(indexChunk.String(), IndexMediaType, "index", 0, Counts{})

	manifest := Manifest{
		Format:  FormatName,
		Version: FormatVersion,
		Snapshot: SnapshotMetadata{
			ID: "snap_golden", CreatedAt: goldenTimestamp, Target: TargetFullArchive,
			DatabaseID: "db_archive", SourceReplicaID: "replica_source",
			Consistency: "sqlite_read_transaction", CollectionIDs: []string{"default"},
			SelectionManifestSHA256: strings.Repeat("c", 64),
		},
		Compatibility: Compatibility{
			MinimumReaderVersion: FormatVersion, SourceSchemaVersion: MinimumSchemaVersion,
			MinimumSchemaVersion: MinimumSchemaVersion, MaximumSchemaVersion: MinimumSchemaVersion,
			RequiredCapabilities: RequiredCapabilities(), OptionalCapabilities: []string{},
		},
		Index: []IndexObject{{
			SHA256: indexEntry.SHA256, SizeBytes: indexEntry.SizeBytes, Entries: len(entries),
			MediaType: IndexMediaType, Location: indexEntry.Location,
		}},
		Totals: totals,
		Counts: recordCounts,
	}
	if err := FinalizeManifest(&manifest); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGoldenFixtureIsReproducible keeps the committed fixture honest: it must
// equal what the independent generator produces, byte for byte.
func TestGoldenFixtureIsReproducible(t *testing.T) {
	generated := filepath.Join(t.TempDir(), "golden")
	buildGoldenArchive(t, generated)
	if os.Getenv("NOTRIOS_UPDATE_GOLDEN") == "1" {
		if err := os.RemoveAll(goldenFixture); err != nil {
			t.Fatal(err)
		}
		if err := copyTree(generated, goldenFixture); err != nil {
			t.Fatal(err)
		}
		t.Log("committed golden fixture updated")
	}
	compareTrees(t, generated, goldenFixture)
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

func compareTrees(t *testing.T, expected, actual string) {
	t.Helper()
	expectedFiles := treeFiles(t, expected)
	actualFiles := treeFiles(t, actual)
	if len(expectedFiles) != len(actualFiles) {
		t.Fatalf("fixture has %d files, generator produced %d", len(actualFiles), len(expectedFiles))
	}
	for path, content := range expectedFiles {
		other, ok := actualFiles[path]
		if !ok {
			t.Fatalf("committed fixture is missing %q", path)
		}
		if content != other {
			t.Fatalf("committed fixture differs from the generator at %q", path)
		}
	}
}

func treeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
