package joplinraw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

type joplinProfileResult struct {
	Mode              string        `json:"mode"`
	Notes             int           `json:"notes"`
	Folders           int           `json:"folders"`
	Tags              int           `json:"tags"`
	Resources         int           `json:"resources"`
	BatchSize         int           `json:"batch_size"`
	DryRunDuration    time.Duration `json:"dry_run_duration"`
	ImportDuration    time.Duration `json:"import_duration"`
	NoOpDuration      time.Duration `json:"no_op_duration"`
	Resumed           bool          `json:"resumed"`
	BatchesCompleted  int           `json:"batches_completed"`
	NotesPlanned      int           `json:"notes_planned"`
	NotesImported     int           `json:"notes_imported"`
	NotesUnchanged    int           `json:"notes_unchanged_on_no_op"`
	ResourcesPlanned  int           `json:"resources_planned"`
	ResourcesImported int           `json:"resources_imported"`
	CanonicalBatches  int           `json:"canonical_document_batches"`
	LinkBatches       int           `json:"link_rebuild_batches"`
	ManifestBytes     int64         `json:"temporary_manifest_bytes"`
	RevisionsStable   bool          `json:"revision_count_stable_on_no_op"`
	PeakGoSysBytes    uint64        `json:"peak_go_sys_bytes"`
	GoVersion         string        `json:"go_version"`
	GOOS              string        `json:"goos"`
	GOARCH            string        `json:"goarch"`
}

type realJoplinProfileResult struct {
	Label                    string         `json:"label"`
	Mode                     string         `json:"mode"`
	MetadataFilesSeen        int            `json:"metadata_files_seen"`
	ItemsSeen                int            `json:"items_seen"`
	ItemTypeCounts           map[string]int `json:"item_type_counts"`
	MalformedItems           int            `json:"malformed_items"`
	UnsupportedItems         int            `json:"unsupported_items"`
	IgnoredFiles             int            `json:"ignored_files"`
	NotesSeen                int            `json:"notes_seen"`
	NotebooksSeen            int            `json:"notebooks_seen"`
	TagsSeen                 int            `json:"tags_seen"`
	ResourcesSeen            int            `json:"resources_seen"`
	ResourcesMissing         int            `json:"resources_missing_content"`
	LinksRewritten           int            `json:"links_rewritten"`
	UnresolvedLinks          int            `json:"unresolved_links"`
	AttachmentsPlanned       int            `json:"attachments_planned"`
	WarningCount             int            `json:"warning_count"`
	ElapsedMilliseconds      int64          `json:"elapsed_milliseconds"`
	GoSystemBytes            uint64         `json:"go_system_bytes"`
	SourceDirectoryUnchanged bool           `json:"source_directory_unchanged"`
	GoVersion                string         `json:"go_version"`
	GOOS                     string         `json:"goos"`
	GOARCH                   string         `json:"goarch"`
}

type fullJoplinProfileResult struct {
	Label                     string                    `json:"label"`
	Mode                      string                    `json:"mode"`
	ItemsSeen                 int                       `json:"items_seen"`
	ItemTypeCounts            map[string]int            `json:"item_type_counts"`
	NotesSeen                 int                       `json:"notes_seen"`
	NotebooksSeen             int                       `json:"notebooks_seen"`
	TagsSeen                  int                       `json:"tags_seen"`
	ResourcesSeen             int                       `json:"resources_seen"`
	BatchSize                 int                       `json:"batch_size"`
	InterruptedAfterNotes     int                       `json:"interrupted_after_notes"`
	InterruptedMilliseconds   int64                     `json:"interrupted_milliseconds"`
	ResumeMilliseconds        int64                     `json:"resume_milliseconds"`
	NoOpMilliseconds          int64                     `json:"no_op_milliseconds"`
	TotalMilliseconds         int64                     `json:"total_milliseconds"`
	NotesImported             int                       `json:"notes_imported"`
	NotesUnchangedOnNoOp      int                       `json:"notes_unchanged_on_no_op"`
	CanonicalBatches          int                       `json:"canonical_document_batches"`
	LinkBatches               int                       `json:"link_rebuild_batches"`
	LinksRewritten            int                       `json:"links_rewritten"`
	UnresolvedJoplinLinks     int                       `json:"unresolved_joplin_links"`
	AttachmentsCreated        int                       `json:"attachments_created"`
	SearchReady               bool                      `json:"search_ready"`
	SearchHitSampleSize       int                       `json:"search_hit_sample_size"`
	Resumed                   bool                      `json:"resumed"`
	RevisionCountStableOnNoOp bool                      `json:"revision_count_stable_on_no_op"`
	TemporaryManifestBytes    int64                     `json:"temporary_manifest_bytes"`
	PeakGoSystemBytes         uint64                    `json:"peak_go_system_bytes"`
	SQLite                    store.SQLiteImportMetrics `json:"sqlite"`
	CPUModel                  string                    `json:"cpu_model"`
	LogicalCPUs               int                       `json:"logical_cpus"`
	SystemMemoryKiB           int64                     `json:"system_memory_kib"`
	GoVersion                 string                    `json:"go_version"`
	GOOS                      string                    `json:"goos"`
	GOARCH                    string                    `json:"goarch"`
	SourceDirectoryUnchanged  bool                      `json:"source_directory_unchanged"`
}

// TestJ2RealExportProfile is opt-in because its input may be private. Its JSON
// deliberately contains aggregate counts only: never source paths, item paths,
// titles, warning text, or note bodies. The source directory metadata check
// also catches accidental default config writes during the dry run.
func TestJ2RealExportProfile(t *testing.T) {
	source := os.Getenv("NOTRIOS_JOPLIN_REAL_SOURCE")
	if source == "" {
		t.Skip("set NOTRIOS_JOPLIN_REAL_SOURCE to a read-only Joplin RAW export")
	}
	label := os.Getenv("NOTRIOS_JOPLIN_REAL_LABEL")
	if label == "" {
		label = "private-corpus"
	}
	for _, character := range label {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
			t.Fatalf("NOTRIOS_JOPLIN_REAL_LABEL must contain only lowercase letters, digits, and hyphens")
		}
	}
	before, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	storeDir := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(storeDir, "profile.sqlite"), filepath.Join(storeDir, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, report, err := DryRun(ctx, st, source, Options{BatchSize: 500})
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	after, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result := realJoplinProfileResult{
		Label: label, Mode: "full-dry-run", MetadataFilesSeen: report.MetadataFilesSeen,
		ItemsSeen: report.ItemsSeen, ItemTypeCounts: report.ItemTypeCounts,
		MalformedItems: report.MalformedItems, UnsupportedItems: report.UnsupportedItems,
		IgnoredFiles: report.IgnoredFiles, NotesSeen: report.NotesSeen,
		NotebooksSeen: report.NotebooksSeen, TagsSeen: report.TagsSeen,
		ResourcesSeen: report.ResourcesSeen, ResourcesMissing: report.ResourcesMissing,
		LinksRewritten:  report.LinksRewritten,
		UnresolvedLinks: report.UnresolvedLinks, AttachmentsPlanned: report.AttachmentsCreated,
		WarningCount: len(report.Warnings), ElapsedMilliseconds: elapsed.Milliseconds(),
		GoSystemBytes: memory.Sys, SourceDirectoryUnchanged: before.ModTime() == after.ModTime() && before.Size() == after.Size(),
		GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}
	output := os.Getenv("NOTRIOS_JOPLIN_REAL_OUTPUT")
	if output == "" {
		t.Fatal("set NOTRIOS_JOPLIN_REAL_OUTPUT to a private-safe JSON output path")
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("real Joplin profile completed: label=%s items=%d notes=%d elapsed=%s", label, report.ItemsSeen, report.NotesSeen, elapsed.Round(time.Millisecond))
}

// TestJ3RealExportFullImportProfile is deliberately opt-in and aggregate-only.
// It performs a checkpointed interruption, complete resume, search-readiness
// check, and complete no-op re-import against one real RAW export.
func TestJ3RealExportFullImportProfile(t *testing.T) {
	source := os.Getenv("NOTRIOS_JOPLIN_FULL_SOURCE")
	if source == "" {
		t.Skip("set NOTRIOS_JOPLIN_FULL_SOURCE to a read-only Joplin RAW export")
	}
	output := os.Getenv("NOTRIOS_JOPLIN_FULL_OUTPUT")
	if output == "" {
		t.Fatal("set NOTRIOS_JOPLIN_FULL_OUTPUT to a private-safe JSON output path")
	}
	label := os.Getenv("NOTRIOS_JOPLIN_FULL_LABEL")
	if label == "" {
		label = "private-corpus"
	}
	for _, character := range label {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
			t.Fatal("NOTRIOS_JOPLIN_FULL_LABEL must contain only lowercase letters, digits, and hyphens")
		}
	}
	const batchSize = 500
	before, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	storeDir := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(storeDir, "full-import.sqlite"), filepath.Join(storeDir, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	totalStarted := time.Now()
	interruptedStarted := time.Now()
	interrupted := errors.New("intentional J3 checkpoint interruption")
	didInterrupt := false
	partial, err := Import(ctx, st, source, Options{BatchSize: batchSize, AfterBatch: func(phase string, processed, _ int) error {
		if phase == "notes" && !didInterrupt && processed > 0 {
			didInterrupt = true
			return interrupted
		}
		return nil
	}})
	if !errors.Is(err, interrupted) {
		t.Fatalf("interrupted import error=%v", err)
	}
	interruptedDuration := time.Since(interruptedStarted)
	if partial.NotesImported <= 0 || partial.NotesImported > batchSize {
		t.Fatalf("atomic interruption committed %d notes, want 1..%d", partial.NotesImported, batchSize)
	}

	resumeStarted := time.Now()
	completed, err := Import(ctx, st, source, Options{BatchSize: batchSize})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	resumeDuration := time.Since(resumeStarted)
	if !completed.Resumed || completed.NotesImported != completed.NotesSeen {
		t.Fatalf("resume report=%#v", completed)
	}
	beforeNoOp, err := st.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	search, err := st.Search(ctx, store.SearchRequest{Query: "joplin", Limit: 5})
	if err != nil || len(search.Hits) == 0 {
		t.Fatalf("search readiness: hits=%d err=%v", len(search.Hits), err)
	}

	noOpStarted := time.Now()
	noOp, err := Import(ctx, st, source, Options{BatchSize: batchSize})
	if err != nil {
		t.Fatalf("no-op re-import: %v", err)
	}
	noOpDuration := time.Since(noOpStarted)
	if noOp.NotesImported != 0 || noOp.NotesUpdated != 0 || noOp.NotesUnchanged != noOp.NotesSeen {
		t.Fatalf("no-op report=%#v", noOp)
	}
	afterNoOp, err := st.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if afterNoOp.Documents != int64(completed.NotesSeen) || afterNoOp.FTSRows != afterNoOp.Documents ||
		afterNoOp.Sources != afterNoOp.Documents || afterNoOp.Revisions != beforeNoOp.Revisions {
		t.Fatalf("canonical aggregate mismatch before=%+v after=%+v", beforeNoOp, afterNoOp)
	}
	after, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result := fullJoplinProfileResult{
		Label: label, Mode: "interrupt-resume-no-op-search", ItemsSeen: completed.ItemsSeen,
		ItemTypeCounts: completed.ItemTypeCounts, NotesSeen: completed.NotesSeen,
		NotebooksSeen: completed.NotebooksSeen, TagsSeen: completed.TagsSeen, ResourcesSeen: completed.ResourcesSeen,
		BatchSize: batchSize, InterruptedAfterNotes: partial.NotesImported,
		InterruptedMilliseconds: interruptedDuration.Milliseconds(), ResumeMilliseconds: resumeDuration.Milliseconds(),
		NoOpMilliseconds: noOpDuration.Milliseconds(), TotalMilliseconds: time.Since(totalStarted).Milliseconds(),
		NotesImported: completed.NotesImported, NotesUnchangedOnNoOp: noOp.NotesUnchanged,
		CanonicalBatches: completed.CanonicalBatches, LinkBatches: completed.LinkBatches,
		LinksRewritten: completed.LinksRewritten, UnresolvedJoplinLinks: completed.UnresolvedLinks,
		AttachmentsCreated: completed.AttachmentsCreated, SearchReady: true, SearchHitSampleSize: len(search.Hits),
		Resumed: completed.Resumed, RevisionCountStableOnNoOp: beforeNoOp.Revisions == afterNoOp.Revisions,
		TemporaryManifestBytes: completed.ManifestBytes, PeakGoSystemBytes: memory.Sys, SQLite: afterNoOp,
		CPUModel: cpuModel(), LogicalCPUs: runtime.NumCPU(), SystemMemoryKiB: systemMemoryKiB(),
		GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		SourceDirectoryUnchanged: before.ModTime() == after.ModTime() && before.Size() == after.Size(),
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("J3 full import completed: label=%s notes=%d total=%s database=%d", label, completed.NotesSeen, time.Since(totalStarted).Round(time.Second), afterNoOp.DatabaseBytes)
}

func cpuModel() string {
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if key, value, found := strings.Cut(line, ":"); found && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func systemMemoryKiB() int64 {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			value, _ := strconv.ParseInt(fields[1], 10, 64)
			return value
		}
	}
	return 0
}

// TestJoplinImporterProfile is an opt-in generated profile. It exercises the
// full dry-run planner followed by an interrupted/resumed real import. The
// supported sizes deliberately span hundreds through 100k source notes.
func TestJoplinImporterProfile(t *testing.T) {
	value := os.Getenv("NOTRIOS_JOPLIN_PROFILE")
	if value == "" {
		t.Skip("set NOTRIOS_JOPLIN_PROFILE to 100, 10000, or 100000")
	}
	count, err := strconv.Atoi(value)
	if err != nil || (count != 100 && count != 10000 && count != 100000) {
		t.Fatalf("NOTRIOS_JOPLIN_PROFILE must be 100, 10000, or 100000; got %q", value)
	}
	const (
		folderCount   = 10
		tagCount      = 20
		resourceCount = 20
		batchSize     = 100
	)
	ctx := context.Background()
	dir := t.TempDir()
	for index := 0; index < folderCount; index++ {
		parent := ""
		if index > 0 {
			parent = fmt.Sprintf("parent_id: folder-%02d\n", index-1)
		}
		writeFile(t, filepath.Join(dir, fmt.Sprintf("folder-%02d.md", index)),
			fmt.Sprintf("id: folder-%02d\n%stitle: Folder %02d\ntype_: 2\n", index, parent, index))
	}
	for index := 0; index < tagCount; index++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("tag-%02d.md", index)),
			fmt.Sprintf("id: tag-%02d\ntitle: tag-%02d\ntype_: 5\n", index, index))
	}
	for index := 0; index < resourceCount; index++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("resource-%02d.md", index)),
			fmt.Sprintf("id: resource-%02d\ntitle: resource-%02d.bin\nfilename: resource-%02d.bin\ntype_: 4\n", index, index, index))
		writeFile(t, filepath.Join(dir, "resources", fmt.Sprintf("resource-%02d", index)),
			fmt.Sprintf("generated resource %d", index))
	}
	for index := 0; index < count; index++ {
		id := fmt.Sprintf("profile-%06d", index)
		folderID := fmt.Sprintf("folder-%02d", index%folderCount)
		resourceID := fmt.Sprintf("resource-%02d", index%resourceCount)
		writeFile(t, filepath.Join(dir, id+".md"),
			fmt.Sprintf("Profile body %d\n\n![resource](:/%s)\n\nfuture_%d: exact unknown value\nid: %s\nparent_id: %s\ntitle: Profile %d\ntype_: 1\n",
				index, resourceID, index, id, folderID, index))
		writeFile(t, filepath.Join(dir, "relations", id+".md"),
			fmt.Sprintf("id: relation-%06d\nnote_id: %s\ntag_id: tag-%02d\ntype_: 6\n", index, id, index%tagCount))
	}

	storeDir := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(storeDir, "profile.sqlite"), filepath.Join(storeDir, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	dryStarted := time.Now()
	config, dry, err := DryRun(ctx, st, dir, Options{BatchSize: batchSize})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	dryDuration := time.Since(dryStarted)
	interrupted := errors.New("profile interruption")
	didInterrupt := false
	importStarted := time.Now()
	_, err = Import(ctx, st, dir, Options{
		BatchSize: batchSize,
		Config:    &config,
		AfterBatch: func(phase string, processed, total int) error {
			if phase == "notes" && !didInterrupt && processed >= total/2 {
				didInterrupt = true
				return interrupted
			}
			return nil
		},
	})
	if !errors.Is(err, interrupted) {
		t.Fatalf("interrupted import error = %v", err)
	}
	actual, err := Import(ctx, st, dir, Options{BatchSize: batchSize, Config: &config})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	importDuration := time.Since(importStarted)
	if dry.NotesImported != actual.NotesImported || dry.ResourcesImported != actual.ResourcesImported ||
		dry.NotebooksCreated != actual.NotebooksCreated || dry.TagsCreated != actual.TagsCreated {
		t.Fatalf("dry run differed from real run: dry=%#v actual=%#v", dry, actual)
	}
	if !actual.Resumed || actual.NotesImported != count {
		t.Fatalf("profile did not resume/import all notes: %#v", actual)
	}
	beforeNoOp, err := st.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	noOpStarted := time.Now()
	noOp, err := Import(ctx, st, dir, Options{BatchSize: batchSize, Config: &config})
	if err != nil {
		t.Fatalf("no-op: %v", err)
	}
	noOpDuration := time.Since(noOpStarted)
	afterNoOp, err := st.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if noOp.NotesUnchanged != count || noOp.NotesImported != 0 || noOp.NotesUpdated != 0 || beforeNoOp.Revisions != afterNoOp.Revisions {
		t.Fatalf("no-op was not revision-stable: report=%#v before=%+v after=%+v", noOp, beforeNoOp, afterNoOp)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result := joplinProfileResult{
		Mode: "dry-run-interrupt-resume", Notes: count, Folders: folderCount, Tags: tagCount, Resources: resourceCount,
		BatchSize: batchSize, DryRunDuration: dryDuration, ImportDuration: importDuration, NoOpDuration: noOpDuration,
		Resumed: actual.Resumed, BatchesCompleted: actual.BatchesCompleted,
		NotesPlanned: dry.NotesImported, NotesImported: actual.NotesImported, NotesUnchanged: noOp.NotesUnchanged,
		ResourcesPlanned: dry.ResourcesImported, ResourcesImported: actual.ResourcesImported,
		CanonicalBatches: actual.CanonicalBatches, LinkBatches: actual.LinkBatches,
		ManifestBytes: actual.ManifestBytes, RevisionsStable: beforeNoOp.Revisions == afterNoOp.Revisions,
		PeakGoSysBytes: memory.Sys, GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}
	writeJoplinProfileResult(t, result)
}

func writeJoplinProfileResult(t *testing.T, result joplinProfileResult) {
	t.Helper()
	output := os.Getenv("NOTRIOS_JOPLIN_PROFILE_OUTPUT")
	if output != "" {
		raw, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Joplin profile: %+v", result)
}
