package obsidian

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

type obsidianProfileResult struct {
	Mode              string        `json:"mode"`
	Notes             int           `json:"notes"`
	Folders           int           `json:"folders"`
	Resources         int           `json:"resources"`
	BatchSize         int           `json:"batch_size"`
	DryRunDuration    time.Duration `json:"dry_run_duration"`
	ImportDuration    time.Duration `json:"import_duration"`
	Resumed           bool          `json:"resumed"`
	BatchesCompleted  int           `json:"batches_completed"`
	NotesPlanned      int           `json:"notes_planned"`
	NotesImported     int           `json:"notes_imported"`
	ResourcesPlanned  int           `json:"resources_planned"`
	ResourcesImported int           `json:"resources_imported"`
	LinksRewritten    int           `json:"links_rewritten"`
	SourceBundleItems int           `json:"source_bundle_items"`
	PeakGoSysBytes    uint64        `json:"peak_go_sys_bytes"`
	GoVersion         string        `json:"go_version"`
	GOOS              string        `json:"goos"`
	GOARCH            string        `json:"goarch"`
}

// TestObsidianImporterProfile is an opt-in generated profile. The 100 and
// 10k tiers interrupt and resume a real import. The 100k and 500k tiers
// exercise the production inventory and dry-run planner without writes.
func TestObsidianImporterProfile(t *testing.T) {
	value := os.Getenv("NOTRIOS_OBSIDIAN_PROFILE")
	if value == "" {
		t.Skip("set NOTRIOS_OBSIDIAN_PROFILE to 100, 10000, 100000, or 500000")
	}
	count, err := strconv.Atoi(value)
	if err != nil || count != 100 && count != 10000 && count != 100000 && count != 500000 {
		t.Fatalf("NOTRIOS_OBSIDIAN_PROFILE must be 100, 10000, 100000, or 500000; got %q", value)
	}
	const (
		folderCount   = 10
		resourceCount = 20
		batchSize     = 100
	)
	ctx := context.Background()
	dir := t.TempDir()
	for index := 0; index < resourceCount; index++ {
		writeFile(t, filepath.Join(dir, "assets", fmt.Sprintf("resource-%02d.bin", index)), fmt.Sprintf("generated resource %d", index))
	}
	for index := 0; index < count; index++ {
		folder := filepath.Join(dir, fmt.Sprintf("Folder-%02d", index%folderCount), "Notes")
		previous := (index - 1 + count) % count
		writeFile(t, filepath.Join(folder, fmt.Sprintf("Profile-%06d.md", index)),
			fmt.Sprintf("---\naliases: [Alias %06d]\nunknown_%d: retained\n---\n# Profile %d\n\nRelative [[Profile-%06d#Heading]].\nAlias ![[Alias %06d#^block]].\nAsset ![[../../assets/resource-%02d.bin]].\n\nBlock ^block\n",
				index, index, index, previous, previous, index%resourceCount))
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
	options := Options{BatchSize: batchSize, PreserveSource: true}
	dryStarted := time.Now()
	config, dry, err := DryRun(ctx, st, dir, options)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	dryDuration := time.Since(dryStarted)
	if count >= 100000 {
		writeObsidianProfileResult(t, obsidianProfileResult{
			Mode: "dry-run-inventory", Notes: count, Folders: dry.NotebooksSeen, Resources: resourceCount,
			BatchSize: batchSize, DryRunDuration: dryDuration, BatchesCompleted: dry.BatchesCompleted,
			NotesPlanned: dry.NotesImported, ResourcesPlanned: dry.ResourcesImported,
			LinksRewritten: dry.LinksRewritten, SourceBundleItems: dry.SourceBundleItems,
			PeakGoSysBytes: peakGoSysBytes(), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		})
		return
	}
	interrupted := errors.New("profile interruption")
	didInterrupt := false
	importStarted := time.Now()
	_, err = Import(ctx, st, dir, Options{
		BatchSize: batchSize, PreserveSource: true, Config: &config,
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
	actual, err := Import(ctx, st, dir, Options{BatchSize: batchSize, PreserveSource: true, Config: &config})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	importDuration := time.Since(importStarted)
	if dry.NotesImported != actual.NotesImported || dry.ResourcesImported != actual.ResourcesImported ||
		dry.NotebooksCreated != actual.NotebooksCreated || dry.SourceBundleItems != actual.SourceBundleItems {
		t.Fatalf("dry run differed from real run: dry=%#v actual=%#v", dry, actual)
	}
	if !actual.Resumed || actual.NotesImported != count {
		t.Fatalf("profile did not resume/import all notes: %#v", actual)
	}
	writeObsidianProfileResult(t, obsidianProfileResult{
		Mode: "dry-run-interrupt-resume", Notes: count, Folders: dry.NotebooksSeen, Resources: resourceCount,
		BatchSize: batchSize, DryRunDuration: dryDuration, ImportDuration: importDuration,
		Resumed: actual.Resumed, BatchesCompleted: actual.BatchesCompleted,
		NotesPlanned: dry.NotesImported, NotesImported: actual.NotesImported,
		ResourcesPlanned: dry.ResourcesImported, ResourcesImported: actual.ResourcesImported,
		LinksRewritten: actual.LinksRewritten, SourceBundleItems: actual.SourceBundleItems,
		PeakGoSysBytes: peakGoSysBytes(), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	})
}

func peakGoSysBytes() uint64 {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return memory.Sys
}

func writeObsidianProfileResult(t *testing.T, result obsidianProfileResult) {
	t.Helper()
	if output := os.Getenv("NOTRIOS_OBSIDIAN_PROFILE_OUTPUT"); output != "" {
		raw, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Obsidian profile: %+v", result)
}
