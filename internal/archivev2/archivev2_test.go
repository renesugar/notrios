package archivev2

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

const goldenFixture = "testdata/golden-minimal"

func TestGoldenArchiveVerifies(t *testing.T) {
	report, err := VerifyDirectory(goldenFixture, DefaultLimits())
	if err != nil {
		raw, readErr := os.ReadFile(filepath.Join(goldenFixture, "manifest.json"))
		if readErr == nil {
			var manifest Manifest
			if json.Unmarshal(raw, &manifest) == nil {
				digest, _ := ComputeCommitSHA256(manifest)
				t.Logf("computed commit digest: %s", digest)
			}
		}
		t.Fatal(err)
	}
	if report.SnapshotID != "snap_golden" || report.DatabaseID != "db_archive" || report.Objects != 4 ||
		report.IndexObjects != 1 || report.Records != 12 || report.Blobs != 3 || report.RecordChunks != 1 ||
		report.Counts.Documents != 1 || report.Counts.SearchNotebooks != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestManifestLastAndStrictManifestAdmission(t *testing.T) {
	t.Run("manifest absent is incomplete", func(t *testing.T) {
		_, err := VerifyDirectory(t.TempDir(), DefaultLimits())
		if err == nil || !strings.Contains(err.Error(), "incomplete archive") {
			t.Fatalf("expected incomplete archive error, got %v", err)
		}
	})

	for _, test := range []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{name: "unsupported version", mutate: func(m *Manifest) { m.Version = 3 }, want: "unsupported archive"},
		// An archive that needs a schema newer than this build understands.
		// Expressed relative to the reader so the case keeps its meaning as the
		// schema advances, rather than becoming the reader's own version.
		{name: "schema range", mutate: func(m *Manifest) {
			m.Compatibility.MinimumSchemaVersion = store.CurrentSchemaVersion + 1
			m.Compatibility.MaximumSchemaVersion = store.CurrentSchemaVersion + 1
			m.Compatibility.SourceSchemaVersion = store.CurrentSchemaVersion + 1
		}, want: "unsupported schema"},
		{name: "required capability", mutate: func(m *Manifest) {
			m.Compatibility.RequiredCapabilities = append(m.Compatibility.RequiredCapabilities, "unknown.required.v1")
		}, want: "unsupported required capability"},
		{name: "traversal", mutate: func(m *Manifest) { m.Index[0].Location.Path = "../object" }, want: "does not match its content hash"},
		{name: "inconsistent counts", mutate: func(m *Manifest) { m.Counts.Documents++ }, want: "counts do not match"},
		{name: "index media type", mutate: func(m *Manifest) { m.Index[0].MediaType = "not a mime" }, want: "media type"},
		{name: "object totals drift", mutate: func(m *Manifest) { m.Totals.Objects++ }, want: "objects but the index declares"},
		{name: "byte totals drift", mutate: func(m *Manifest) { m.Totals.Bytes++ }, want: "index totals do not match"},
		{name: "index entry count drift", mutate: func(m *Manifest) {
			m.Index[0].Entries++
			m.Totals.Objects++
			m.Totals.Blobs++
		}, want: "entries but holds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := copyFixture(t)
			manifest := readManifest(t, root)
			test.mutate(&manifest)
			if test.name == "required capability" {
				// Keep deterministic capability ordering so the test reaches the
				// unsupported-capability check rather than the ordering check.
				manifest.Compatibility.RequiredCapabilities = []string{
					CapabilityIdentity, CapabilitySHA256Objects, CapabilityRecordsJSONL,
					CapabilityRevisions, "unknown.required.v1",
				}
			}
			writeManifest(t, root, manifest, true)
			_, err := VerifyDirectory(root, DefaultLimits())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}

	t.Run("unknown field", func(t *testing.T) {
		root := copyFixture(t)
		path := filepath.Join(root, "manifest.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw = []byte(strings.Replace(string(raw), `"version": 2,`, `"version": 2, "surprise": true,`, 1))
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("expected strict JSON error, got %v", err)
		}
	})

	t.Run("duplicate field", func(t *testing.T) {
		root := copyFixture(t)
		path := filepath.Join(root, "manifest.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw = []byte(strings.Replace(string(raw), `"version": 2,`, `"version": 2, "version": 2,`, 1))
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
			t.Fatalf("expected duplicate-field error, got %v", err)
		}
	})

	t.Run("commit corruption", func(t *testing.T) {
		root := copyFixture(t)
		manifest := readManifest(t, root)
		manifest.Snapshot.ID = "snap_modified"
		writeManifest(t, root, manifest, false)
		if _, err := VerifyDirectory(root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "commit checksum") {
			t.Fatalf("expected commit checksum error, got %v", err)
		}
	})
}

func TestObjectCorruptionMissingExtraAndSymlinkAreRejected(t *testing.T) {
	objectPath := firstBlobPath(t, goldenFixture)

	t.Run("corrupt", func(t *testing.T) {
		root := copyFixture(t)
		path := filepath.Join(root, filepath.FromSlash(objectPath))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw[0] ^= 1
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected checksum error, got %v", err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		root := copyFixture(t)
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(objectPath))); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(root, DefaultLimits()); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected missing-object error, got %v", err)
		}
	})

	t.Run("extra", func(t *testing.T) {
		root := copyFixture(t)
		if err := os.WriteFile(filepath.Join(root, "extra"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "unexpected") {
			t.Fatalf("expected extra-file error, got %v", err)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		root := copyFixture(t)
		path := filepath.Join(root, filepath.FromSlash(objectPath))
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(goldenFixture, filepath.FromSlash(objectPath)), path); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDirectory(root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink error, got %v", err)
		}
	})
}

func TestVerificationLimitsAndPathDepth(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxRecords = 10
	if _, err := VerifyDirectory(goldenFixture, limits); err == nil || !strings.Contains(err.Error(), "record count") {
		t.Fatalf("expected record limit error, got %v", err)
	}
	if validRelativePath("../private", DefaultLimits()) || validRelativePath("a\\b", DefaultLimits()) || validRelativePath("/absolute", DefaultLimits()) {
		t.Fatal("unsafe source-bundle path accepted")
	}
	deep := []byte(strings.Repeat("[", DefaultLimits().MaxJSONDepth+1) + strings.Repeat("]", DefaultLimits().MaxJSONDepth+1))
	if err := validateJSONDepth(deep, DefaultLimits().MaxJSONDepth); err == nil {
		t.Fatal("deep JSON accepted")
	}
}

func TestRecordReferenceAndPayloadConsistencyRejected(t *testing.T) {
	for _, test := range []struct {
		name    string
		old     string
		replace string
		want    string
	}{
		{name: "missing current revision", old: `"current_revision_id":"rev_alpha"`, replace: `"current_revision_id":"rev_missing"`, want: `references missing or mismatched "revision:rev_missing`},
		{name: "resource MIME disagreement", old: `"filename":"image.png","mime_type":"image/png"`, replace: `"filename":"image.png","mime_type":"image/jpeg"`, want: "resource MIME mismatch"},
		{name: "source path traversal", old: `"relative_path":"alpha.md"`, replace: `"relative_path":"../alpha.md"`, want: "invalid source_bundle record"},
		{name: "unknown record type", old: `"type":"tag"`, replace: `"type":"future_tag"`, want: "unsupported record type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := copyFixture(t)
			mutateRecordsObject(t, root, test.old, test.replace)
			if _, err := VerifyDirectory(root, DefaultLimits()); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestRestoreIntentIdentityRules(t *testing.T) {
	manifest := readManifest(t, goldenFixture)
	if _, err := PlanRestoreIdentity(manifest, RestoreIdentityRequest{}); err == nil {
		t.Fatal("implicit restore intent accepted")
	}
	replace, err := PlanRestoreIdentity(manifest, RestoreIdentityRequest{Intent: RestoreReplace, TargetDatabaseID: "db_target"})
	if err != nil || replace.ResultDatabaseID != "db_archive" || !replace.PreserveArchiveUniverse || !replace.MintNewReplicaID {
		t.Fatalf("replace: %+v err=%v", replace, err)
	}
	adopt, err := PlanRestoreIdentity(manifest, RestoreIdentityRequest{Intent: RestoreAdopt, TargetEmpty: true})
	if err != nil || adopt.ResultDatabaseID != "db_archive" || !adopt.PreserveArchiveUniverse {
		t.Fatalf("adopt: %+v err=%v", adopt, err)
	}
	merge, err := PlanRestoreIdentity(manifest, RestoreIdentityRequest{Intent: RestoreMerge, TargetDatabaseID: "db_target"})
	if err != nil || merge.ResultDatabaseID != "db_target" || !merge.TreatRecordsAsForeign || merge.MintNewReplicaID {
		t.Fatalf("merge: %+v err=%v", merge, err)
	}
	fork, err := PlanRestoreIdentity(manifest, RestoreIdentityRequest{Intent: RestoreFork, NewDatabaseID: "db_forked"})
	if err != nil || fork.ResultDatabaseID != "db_forked" || fork.PreserveArchiveUniverse {
		t.Fatalf("fork: %+v err=%v", fork, err)
	}
	for _, request := range []RestoreIdentityRequest{
		{Intent: RestoreReplace, TargetDatabaseID: "db_target", TargetEmpty: true},
		{Intent: RestoreAdopt, TargetDatabaseID: "db_target", TargetEmpty: true},
		{Intent: RestoreMerge, TargetEmpty: true},
		{Intent: RestoreFork, NewDatabaseID: "db_archive"},
	} {
		if _, err := PlanRestoreIdentity(manifest, request); err == nil {
			t.Fatalf("unsafe identity request accepted: %+v", request)
		}
	}
}

func copyFixture(t *testing.T) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "archive")
	err := filepath.WalkDir(goldenFixture, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(goldenFixture, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

func readManifest(t *testing.T, root string) Manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func writeManifest(t *testing.T, root string, manifest Manifest, finalize bool) {
	t.Helper()
	if finalize {
		if err := FinalizeManifest(&manifest); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// firstBlobPath returns the archive path of a blob object, read from the
// index chunk rather than from the manifest.
func firstBlobPath(t *testing.T, root string) string {
	t.Helper()
	for _, entry := range readIndexEntries(t, root) {
		if entry.Kind == "blob" {
			return entry.Location.Path
		}
	}
	t.Fatal("fixture has no blob object")
	return ""
}

func readIndexEntries(t *testing.T, root string) []IndexEntry {
	t.Helper()
	manifest := readManifest(t, root)
	entries := []IndexEntry{}
	for _, indexObject := range manifest.Index {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(indexObject.Location.Path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
			var entry IndexEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatal(err)
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

// mutateRecordsObject edits the fixture's record chunk and republishes the
// index and manifest so only the intended inconsistency remains.
func mutateRecordsObject(t *testing.T, root, old, replacement string) {
	t.Helper()
	entries := readIndexEntries(t, root)
	index := -1
	for i, entry := range entries {
		if entry.Kind == "records" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("records object missing")
	}
	entry := entries[index]
	oldPath := filepath.Join(root, filepath.FromSlash(entry.Location.Path))
	raw, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := []byte(strings.Replace(string(raw), old, replacement, 1))
	if string(updated) == string(raw) {
		t.Fatalf("mutation target %q absent", old)
	}
	digest := sha256.Sum256(updated)
	entry.SHA256 = fmt.Sprintf("%x", digest[:])
	entry.Location = newFanoutLocation(entry.SHA256)
	entry.SizeBytes = int64(len(updated))
	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(root, filepath.FromSlash(entry.Location.Path))
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	entries[index] = entry
	republishIndex(t, root, entries)
}

// republishIndex rewrites the fixture's single index chunk and manifest from
// the given entries, removing the previous chunk.
func republishIndex(t *testing.T, root string, entries []IndexEntry) {
	t.Helper()
	manifest := readManifest(t, root)
	for _, indexObject := range manifest.Index {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(indexObject.Location.Path))); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].SHA256 < entries[j].SHA256 })
	var chunk strings.Builder
	var totals ObjectTotals
	for _, entry := range entries {
		encoded, err := encodeIndexEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		chunk.Write(encoded)
		totals.Objects++
		totals.Bytes += entry.SizeBytes
		if entry.Kind == "records" {
			totals.RecordChunks++
		} else {
			totals.Blobs++
		}
	}
	payload := chunk.String()
	digest := sha256.Sum256([]byte(payload))
	hash := fmt.Sprintf("%x", digest[:])
	location := newFanoutLocation(hash)
	path := filepath.Join(root, filepath.FromSlash(location.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest.Index = []IndexObject{{SHA256: hash, SizeBytes: int64(len(payload)), Entries: len(entries), MediaType: IndexMediaType, Location: location}}
	manifest.Totals = totals
	writeManifest(t, root, manifest, true)
	pruneEmptyDirectories(t, filepath.Join(root, "objects"))
}

func pruneEmptyDirectories(t *testing.T, root string) {
	t.Helper()
	var directories []string
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, current)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for index := len(directories) - 1; index > 0; index-- {
		_ = os.Remove(directories[index])
	}
}
