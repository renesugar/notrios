package main

import (
	"os"
	"path/filepath"
	"testing"
)

// seedTagLibrary imports two notes carrying a small tag hierarchy.
func seedTagLibrary(t *testing.T, binary, dbPath, assetStore string) {
	t.Helper()
	archiveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(archiveDir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, contents string) {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", `{"format":"notrios-archive","version":1,"query":"","notes":2}`)
	write("notebooks.json", `[{"path":"Docs"}]`)
	write(filepath.Join("notes", "doc_one.md"),
		"---\nid: doc_one\ntitle: One\nnotebook: Docs\ntags:\n  - project\n  - project/alpha\n---\n\nfirst\n")
	write(filepath.Join("notes", "doc_two.md"),
		"---\nid: doc_two\ntitle: Two\nnotebook: Docs\ntags:\n  - project/alpha\n---\n\nsecond\n")
	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir); result.exitCode != 0 {
		t.Fatalf("seed import: %s", result.stderr)
	}
}

// The dry run is the default and it changes nothing; --apply is the only way to
// alter the library. A rename that a script did not mean to run should be a
// report, not a repair job.
func TestTagRenameCLIDryRunIsDefault(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedTagLibrary(t, binary, dbPath, assetStore)

	dry := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work", "--include-children")
	if dry.exitCode != 0 {
		t.Fatalf("dry run: exit %d %s", dry.exitCode, dry.stderr)
	}
	report := decodeCLIJSON(t, dry.stdout)
	if report["dry_run"] != true {
		t.Fatalf("dry_run must default to true: %s", dry.stdout)
	}
	changes, ok := report["changes"].([]any)
	if !ok || len(changes) != 2 {
		t.Fatalf("expected two planned changes: %s", dry.stdout)
	}
	if report["notes"].(float64) != 2 {
		t.Fatalf("expected two affected notes: %s", dry.stdout)
	}

	// Nothing moved: the same dry run reports the same plan.
	again := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work", "--include-children")
	if again.exitCode != 0 || len(decodeCLIJSON(t, again.stdout)["changes"].([]any)) != 2 {
		t.Fatalf("dry run was not repeatable: exit %d %s", again.exitCode, again.stdout)
	}

	applied := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work", "--include-children", "--apply")
	if applied.exitCode != 0 {
		t.Fatalf("apply: exit %d %s", applied.exitCode, applied.stderr)
	}
	if decodeCLIJSON(t, applied.stdout)["dry_run"] != false {
		t.Fatalf("apply reported a dry run: %s", applied.stdout)
	}

	// The old name is gone, so planning it again is a not-found failure.
	after := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work")
	if after.exitCode != 1 {
		t.Fatalf("renaming a vanished tag should fail: exit %d %s", after.exitCode, after.stdout)
	}
}

// A dry run whose plan contains a merge exits 1, so a script that meant to
// rename and would instead have combined two hierarchies stops.
func TestTagRenameCLIExitsNonZeroOnPlannedMerge(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedTagLibrary(t, binary, dbPath, assetStore)

	merge := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project/alpha", "--to", "project")
	if merge.exitCode != 1 {
		t.Fatalf("a planned merge should exit 1: exit %d %s", merge.exitCode, merge.stdout)
	}
	report := decodeCLIJSON(t, merge.stdout)
	changes := report["changes"].([]any)
	if len(changes) != 1 || changes[0].(map[string]any)["action"] != "merge" {
		t.Fatalf("expected a reported merge: %s", merge.stdout)
	}
}
