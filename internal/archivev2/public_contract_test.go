package archivev2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const publicFixtureRoot = "../../contracts/archive-v2/fixtures"

// TestPublicGoldenFixturesAreReproducible keeps the published contract
// fixtures independent of Export. The same invented one-of-each-record corpus
// is encoded manually into loose and packed layouts, including a schema-27
// sync-era archive that proves sync did not create archive-v3.
func TestPublicGoldenFixturesAreReproducible(t *testing.T) {
	generated := t.TempDir()
	loose := filepath.Join(generated, "loose-schema12")
	packed := filepath.Join(generated, "packed-schema12")
	syncEra := filepath.Join(generated, "sync-era-schema27-packed")
	buildGoldenArchive(t, loose)
	buildPublicPackedGolden(t, packed, MinimumSchemaVersion, "snap_golden_packed")
	buildPublicPackedGolden(t, syncEra, 27, "snap_sync_era")

	fixtures := map[string]string{
		"loose-schema12":           loose,
		"packed-schema12":          packed,
		"sync-era-schema27-packed": syncEra,
	}
	for name, source := range fixtures {
		destination := filepath.Join(publicFixtureRoot, name)
		if os.Getenv("NOTRIOS_UPDATE_PUBLIC_CONTRACT") == "1" {
			if err := os.RemoveAll(destination); err != nil {
				t.Fatal(err)
			}
			if err := copyTree(source, destination); err != nil {
				t.Fatal(err)
			}
		}
		compareTrees(t, source, destination)
		if _, err := VerifyDirectory(destination, DefaultLimits()); err != nil {
			t.Fatalf("published %s fixture does not verify: %v", name, err)
		}
	}
}

func TestDeclarationProbeFixturesAreReproducible(t *testing.T) {
	loose := filepath.Join(publicFixtureRoot, "loose-schema12")
	base := readManifest(t, loose)
	probes := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "unknown-required-capability", mutate: func(manifest *Manifest) {
			manifest.Compatibility.RequiredCapabilities = append(manifest.Compatibility.RequiredCapabilities, "vendor.required.future.v1")
			sort.Strings(manifest.Compatibility.RequiredCapabilities)
		}},
		{name: "unknown-optional-capability", mutate: func(manifest *Manifest) {
			manifest.Compatibility.OptionalCapabilities = []string{"vendor.optional.future.v1"}
		}},
	}
	for _, probe := range probes {
		manifest := base
		manifest.Compatibility.RequiredCapabilities = append([]string{}, base.Compatibility.RequiredCapabilities...)
		manifest.Compatibility.OptionalCapabilities = append([]string{}, base.Compatibility.OptionalCapabilities...)
		probe.mutate(&manifest)
		if err := FinalizeManifest(&manifest); err != nil {
			t.Fatal(err)
		}
		want, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, '\n')
		assertOrUpdatePublicFile(t, filepath.Join(publicFixtureRoot, probe.name, "manifest.json"), want)
	}

	entries := readIndexEntries(t, loose)
	var blob IndexEntry
	for _, entry := range entries {
		if entry.Kind == "blob" {
			blob = entry
			break
		}
	}
	if blob.SHA256 == "" {
		t.Fatal("loose public fixture has no blob")
	}
	zero := Counts{}
	blob.RecordCounts = &zero
	want, err := json.MarshalIndent(blob, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	assertOrUpdatePublicFile(t, filepath.Join(publicFixtureRoot, "previous-explicit-zero-record-counts", "index-entry.json"), want)
}

func TestPublishedJSONSchemasCompileAndValidateFixtures(t *testing.T) {
	schemaRoot := filepath.Join("..", "..", "contracts", "archive-v2", "schemas")
	compiler := jsonschema.NewCompiler()
	compile := func(name string) *jsonschema.Schema {
		t.Helper()
		schema, err := compiler.Compile(filepath.Join(schemaRoot, name))
		if err != nil {
			t.Fatalf("compile %s: %v", name, err)
		}
		return schema
	}
	manifestSchema := compile("manifest.schema.json")
	indexSchema := compile("index-entry.schema.json")
	recordSchema := compile("record-envelope.schema.json")
	trailerSchema := compile("pack-trailer-entry.schema.json")
	physicalSchema := compile("physical-manifest-refusal.schema.json")

	for _, name := range []string{"loose-schema12", "packed-schema12", "sync-era-schema27-packed", "unknown-required-capability", "unknown-optional-capability"} {
		validateSchemaFile(t, manifestSchema, filepath.Join(publicFixtureRoot, name, "manifest.json"))
	}
	validateSchemaFile(t, physicalSchema, filepath.Join(publicFixtureRoot, "physical-refusal", "manifest.json"))
	validateSchemaFile(t, indexSchema, filepath.Join(publicFixtureRoot, "previous-explicit-zero-record-counts", "index-entry.json"))

	loose := filepath.Join(publicFixtureRoot, "loose-schema12")
	for _, entry := range readIndexEntries(t, loose) {
		if entry.Kind != "records" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(loose, filepath.FromSlash(entry.Location.Path)))
		if err != nil {
			t.Fatal(err)
		}
		for number, line := range bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'}) {
			validateSchemaBytes(t, recordSchema, line, "record line "+fmt.Sprint(number+1))
		}
	}
	packed := filepath.Join(publicFixtureRoot, "packed-schema12")
	for _, entry := range readIndexEntries(t, packed) {
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
		for number, item := range trailer {
			raw, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			validateSchemaBytes(t, trailerSchema, raw, "pack trailer entry "+fmt.Sprint(number+1))
		}
	}
}

func TestPublishedContractRegistryMatchesReader(t *testing.T) {
	type registryProfile struct {
		ReaderVersion                 int      `json:"reader_version"`
		MinimumSchemaVersion          int      `json:"minimum_schema_version"`
		SupportedRequiredCapabilities []string `json:"supported_required_capabilities"`
		HistoricalCommit              string   `json:"historical_commit"`
	}
	var registry struct {
		Format               string                     `json:"format"`
		Version              int                        `json:"version"`
		MinimumSchemaVersion int                        `json:"minimum_schema_version"`
		CurrentSchemaVersion int                        `json:"current_schema_version"`
		Capabilities         map[string][]string        `json:"capabilities"`
		ReaderProfiles       map[string]registryProfile `json:"reader_profiles"`
		HardLimits           struct {
			MaxManifestBytes         int64 `json:"max_manifest_bytes"`
			MaxObjects               int   `json:"max_objects"`
			MaxIndexObjects          int   `json:"max_index_objects"`
			MaxIndexEntriesPerObject int   `json:"max_index_entries_per_object"`
			MaxIndexEntryBytes       int   `json:"max_index_entry_bytes"`
			MaxIndexObjectBytes      int64 `json:"max_index_object_bytes"`
			MaxRecords               int   `json:"max_records"`
			MaxRecordsPerObject      int   `json:"max_records_per_object"`
			MaxRecordBytes           int   `json:"max_record_bytes"`
			MaxRecordObjectBytes     int64 `json:"max_record_object_bytes"`
			MaxBlobBytes             int64 `json:"max_blob_bytes"`
			MaxTotalBytes            int64 `json:"max_total_bytes"`
			MaxPathBytes             int   `json:"max_path_bytes"`
			MaxPathDepth             int   `json:"max_path_depth"`
			MaxJSONDepth             int   `json:"max_json_depth"`
			MaxCollections           int   `json:"max_collections"`
			MaxNotebooks             int   `json:"max_notebooks"`
			MaxNotebookDepth         int   `json:"max_notebook_depth"`
			MaxCapabilities          int   `json:"max_capabilities"`
			MaxCapabilityNameBytes   int   `json:"max_capability_name_bytes"`
		} `json:"hard_limits"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "contracts", "archive-v2", "contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	if registry.Format != FormatName || registry.Version != FormatVersion || registry.MinimumSchemaVersion != MinimumSchemaVersion || registry.CurrentSchemaVersion != store.CurrentSchemaVersion {
		t.Fatalf("published version bounds drifted: %+v", registry)
	}
	base := RequiredCapabilities()
	packed := append(append([]string{}, base...), CapabilityObjectPack)
	sort.Strings(packed)
	if fmt.Sprint(registry.Capabilities["base_required"]) != fmt.Sprint(base) || fmt.Sprint(registry.Capabilities["packed_required"]) != fmt.Sprint(packed) {
		t.Fatalf("published capabilities drifted: %+v", registry.Capabilities)
	}
	for _, profile := range ReaderProfiles() {
		published, ok := registry.ReaderProfiles[profile.Name]
		if !ok || published.ReaderVersion != profile.ArchiveVersion || published.MinimumSchemaVersion != MinimumSchemaVersion || fmt.Sprint(published.SupportedRequiredCapabilities) != fmt.Sprint(profile.SupportedCapabilities) || published.HistoricalCommit != profile.HistoricalCommit {
			t.Fatalf("published profile %s drifted: %+v", profile.Name, published)
		}
	}
	limits := DefaultLimits()
	publishedLimits := registry.HardLimits
	if publishedLimits.MaxManifestBytes != limits.MaxManifestBytes || publishedLimits.MaxObjects != limits.MaxObjects || publishedLimits.MaxIndexObjects != limits.MaxIndexObjects ||
		publishedLimits.MaxIndexEntriesPerObject != limits.MaxIndexEntriesPerObject || publishedLimits.MaxIndexEntryBytes != limits.MaxIndexEntryBytes || publishedLimits.MaxIndexObjectBytes != limits.MaxIndexObjectBytes ||
		publishedLimits.MaxRecords != limits.MaxRecords || publishedLimits.MaxRecordsPerObject != limits.MaxRecordsPerObject || publishedLimits.MaxRecordBytes != limits.MaxRecordBytes ||
		publishedLimits.MaxRecordObjectBytes != limits.MaxRecordObjectBytes || publishedLimits.MaxBlobBytes != limits.MaxBlobBytes || publishedLimits.MaxTotalBytes != limits.MaxTotalBytes ||
		publishedLimits.MaxPathBytes != limits.MaxPathBytes || publishedLimits.MaxPathDepth != limits.MaxPathDepth || publishedLimits.MaxJSONDepth != limits.MaxJSONDepth ||
		publishedLimits.MaxCollections != limits.MaxCollections || publishedLimits.MaxNotebooks != limits.MaxNotebooks || publishedLimits.MaxNotebookDepth != limits.MaxNotebookDepth ||
		publishedLimits.MaxCapabilities != limits.MaxCapabilities || publishedLimits.MaxCapabilityNameBytes != limits.MaxCapabilityNameBytes {
		t.Fatalf("published hard limits drifted: %+v vs %+v", publishedLimits, limits)
	}
}

func TestPublishedSyncWireVectorsRemainSeparateAndExact(t *testing.T) {
	for _, name := range []string{"golden-inputs.json", "goldens.json"} {
		source, err := os.ReadFile(filepath.Join("..", "..", "performance", "v0.7-g9", name))
		if err != nil {
			t.Fatal(err)
		}
		published, err := os.ReadFile(filepath.Join("..", "..", "contracts", "archive-v2", "sync-wire", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(source, published) {
			t.Fatalf("published sync-wire vector %s drifted from G9", name)
		}
	}
}

func TestPhysicalRefusalGoldenIsManifestOnlyAndReproducible(t *testing.T) {
	cleared := append([]string(nil), store.SnapshotLocalTables...)
	sort.Strings(cleared)
	manifest := snapshotimage.Manifest{
		Format: snapshotimage.FormatName, Version: snapshotimage.FormatVersion,
		ContentSHA256: strings.Repeat("8", 64),
		Snapshot: snapshotimage.SnapshotMetadata{
			ID: "snap_physical_refusal", CreatedAt: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
			Consistency: "sqlite-online-backup", DatabaseID: "db_contract", SourceReplicaID: "replica_contract",
			Vector: syncstate.Vector{}, Floors: syncstate.Vector{},
		},
		Compatibility: snapshotimage.Compatibility{
			ApplicationID: snapshotimage.ApplicationID, ApplicationVersion: "0.7.0-contract",
			SourceSchemaVersion: store.CurrentSchemaVersion, MinimumSchemaVersion: store.CurrentSchemaVersion,
			MaximumSchemaVersion:   store.CurrentSchemaVersion,
			RequiredCapabilities:   []string{snapshotimage.CapabilityArchiveV2, snapshotimage.CapabilitySQLiteImage},
			SemanticFallbackFormat: snapshotimage.SemanticFallback,
		},
		Database: snapshotimage.FileDescriptor{Path: snapshotimage.DatabaseFile, SHA256: strings.Repeat("9", 64), SizeBytes: 4096},
		External: snapshotimage.ExternalManifest{
			Layout: "deterministic-ustar", PackTarget: snapshotimage.DefaultPackTargetBytes,
			PackEntries: snapshotimage.DefaultPackMaxEntries, Objects: 0, PayloadBytes: 0, Packs: []snapshotimage.PackDescriptor{},
		},
		ClearedTables: cleared,
	}
	commit, err := snapshotimage.ComputeCommitSHA256(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.CommitSHA256 = commit
	want, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	path := filepath.Join(publicFixtureRoot, "physical-refusal", "manifest.json")
	if os.Getenv("NOTRIOS_UPDATE_PUBLIC_CONTRACT") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("physical refusal manifest differs from the independent sanitized generator")
	}
	profile, _ := ReaderProfileByName(ReaderProfileCurrent)
	report, err := EvaluateCompatibility(path, profile, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "refuse" || report.ReasonCode != "physical_snapshot_requires_semantic_fallback" || report.SemanticFallbackFormat != snapshotimage.SemanticFallback {
		t.Fatalf("physical compatibility report = %+v", report)
	}
	if _, err := snapshotimage.VerifyDirectory(context.Background(), filepath.Dir(path), snapshotimage.DefaultLimits()); err == nil || !strings.Contains(err.Error(), snapshotimage.DatabaseFile) {
		t.Fatalf("manifest-only physical fixture should pass manifest admission and stop at absent payload: %v", err)
	}
}

func buildPublicPackedGolden(t *testing.T, root string, sourceSchema int, snapshotID string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	recordCounts := Counts{
		Collections: 1, Notebooks: 1, SearchNotebooks: 1, Tags: 1, Documents: 1, Revisions: 1,
		DocumentTags: 1, Resources: 1, DocumentResources: 1, Links: 1, Provenance: 1, SourceBundles: 1,
	}
	type object struct {
		content   []byte
		kind      string
		mediaType string
		records   int
		counts    *Counts
		hash      string
	}
	objects := make([]object, 0, len(goldenBlobs)+1)
	for _, blob := range goldenBlobs {
		objects = append(objects, object{content: []byte(blob.content), kind: "blob", mediaType: blob.mediaType, hash: goldenDigest(blob.content)})
	}
	records := []byte(strings.Join(goldenRecordLines(), "\n") + "\n")
	objects = append(objects, object{content: records, kind: "records", mediaType: RecordsMediaType, records: len(goldenRecordLines()), counts: &recordCounts, hash: goldenDigest(string(records))})
	sort.Slice(objects, func(i, j int) bool { return objects[i].hash < objects[j].hash })

	var payload bytes.Buffer
	trailer := make([]packTrailerEntry, 0, len(objects))
	for _, object := range objects {
		offset := int64(payload.Len())
		payload.Write(object.content)
		trailer = append(trailer, packTrailerEntry{
			SHA256: object.hash, Offset: offset, Length: int64(len(object.content)),
			Kind: object.kind, MediaType: object.mediaType, Records: object.records, RecordCounts: object.counts,
		})
	}
	trailerOffset := int64(payload.Len())
	for _, entry := range trailer {
		line, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		payload.Write(line)
		payload.WriteByte('\n')
	}
	trailerLength := int64(payload.Len()) - trailerOffset
	footer := make([]byte, packFooterBytes)
	binary.BigEndian.PutUint64(footer[0:8], uint64(trailerOffset))
	binary.BigEndian.PutUint64(footer[8:16], uint64(trailerLength))
	copy(footer[16:], packMagic[:])
	payload.Write(footer)
	packBytes := payload.Bytes()
	packDigest := sha256.Sum256(packBytes)
	packHash := hex.EncodeToString(packDigest[:])
	writePublicObject(t, root, packHash, packBytes)

	entries := make([]IndexEntry, 0, len(objects)+1)
	for index, object := range objects {
		contained := trailer[index]
		entries = append(entries, IndexEntry{
			SHA256: object.hash, Kind: object.kind, MediaType: object.mediaType,
			SizeBytes: int64(len(object.content)), Records: object.records, RecordCounts: object.counts,
			Location: ObjectLocation{Layout: LayoutPack, PackSHA256: packHash, Offset: contained.Offset, Length: contained.Length},
		})
	}
	entries = append(entries, IndexEntry{
		SHA256: packHash, Kind: "pack", MediaType: PackMediaType,
		SizeBytes: int64(len(packBytes)), Location: newFanoutLocation(packHash),
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].SHA256 < entries[j].SHA256 })
	var indexChunk bytes.Buffer
	for _, entry := range entries {
		line, err := encodeIndexEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		indexChunk.Write(line)
	}
	indexHash := goldenDigest(indexChunk.String())
	writePublicObject(t, root, indexHash, indexChunk.Bytes())

	required := append(RequiredCapabilities(), CapabilityObjectPack)
	sort.Strings(required)
	manifest := Manifest{
		Format: FormatName, Version: FormatVersion,
		Snapshot: SnapshotMetadata{
			ID: snapshotID, CreatedAt: goldenTimestamp, Target: TargetFullArchive,
			DatabaseID: "db_archive", SourceReplicaID: "replica_source",
			Consistency: "sqlite_read_transaction", CollectionIDs: []string{"default"},
			SelectionManifestSHA256: strings.Repeat("c", 64),
		},
		Compatibility: Compatibility{
			MinimumReaderVersion: FormatVersion, SourceSchemaVersion: sourceSchema,
			MinimumSchemaVersion: MinimumSchemaVersion, MaximumSchemaVersion: sourceSchema,
			RequiredCapabilities: required, OptionalCapabilities: []string{},
		},
		Index: []IndexObject{{
			SHA256: indexHash, SizeBytes: int64(indexChunk.Len()), Entries: len(entries),
			MediaType: IndexMediaType, Location: newFanoutLocation(indexHash),
		}},
		Totals: ObjectTotals{Objects: len(entries), Bytes: int64(len(packBytes)), RecordChunks: 1, Blobs: len(goldenBlobs), Packs: 1},
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

func writePublicObject(t *testing.T, root, hash string, content []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(fanoutObjectPath(hash)))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertOrUpdatePublicFile(t *testing.T, path string, want []byte) {
	t.Helper()
	if os.Getenv("NOTRIOS_UPDATE_PUBLIC_CONTRACT") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("published fixture differs from deterministic generator: %s", path)
	}
}

func validateSchemaFile(t *testing.T, schema *jsonschema.Schema, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	validateSchemaBytes(t, schema, raw, path)
}

func validateSchemaBytes(t *testing.T, schema *jsonschema.Schema, raw []byte, label string) {
	t.Helper()
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse %s: %v", label, err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("validate %s: %v", label, err)
	}
}
