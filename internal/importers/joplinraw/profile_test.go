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
	Resumed           bool          `json:"resumed"`
	BatchesCompleted  int           `json:"batches_completed"`
	NotesPlanned      int           `json:"notes_planned"`
	NotesImported     int           `json:"notes_imported"`
	ResourcesPlanned  int           `json:"resources_planned"`
	ResourcesImported int           `json:"resources_imported"`
	PeakGoSysBytes    uint64        `json:"peak_go_sys_bytes"`
	GoVersion         string        `json:"go_version"`
	GOOS              string        `json:"goos"`
	GOARCH            string        `json:"goarch"`
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
	if count == 100000 {
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		result := joplinProfileResult{
			Mode: "dry-run-inventory", Notes: count, Folders: folderCount, Tags: tagCount,
			Resources: resourceCount, BatchSize: batchSize, DryRunDuration: dryDuration,
			BatchesCompleted: dry.BatchesCompleted, NotesPlanned: dry.NotesImported,
			ResourcesPlanned: dry.ResourcesImported, PeakGoSysBytes: memory.Sys,
			GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		}
		writeJoplinProfileResult(t, result)
		return
	}
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
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result := joplinProfileResult{
		Mode: "dry-run-interrupt-resume", Notes: count, Folders: folderCount, Tags: tagCount, Resources: resourceCount,
		BatchSize: batchSize, DryRunDuration: dryDuration, ImportDuration: importDuration,
		Resumed: actual.Resumed, BatchesCompleted: actual.BatchesCompleted,
		NotesPlanned: dry.NotesImported, NotesImported: actual.NotesImported,
		ResourcesPlanned: dry.ResourcesImported, ResourcesImported: actual.ResourcesImported,
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
