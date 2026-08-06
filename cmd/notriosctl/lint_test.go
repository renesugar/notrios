package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The exit code is the answer: a clean library exits 0 and a library with
// findings exits 1, so `notriosctl lint --quiet` works in a hook or a script.
func TestLintExitCodeReportsFindings(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")

	clean := runCLI(t, binary, "lint", "--db", dbPath, "--asset-store", assetStore)
	if clean.exitCode != 0 {
		t.Fatalf("an empty library should be clean: exit %d %s", clean.exitCode, clean.stdout)
	}
	if decodeCLIJSON(t, clean.stdout)["total_findings"].(float64) != 0 {
		t.Fatalf("unexpected findings: %s", clean.stdout)
	}

	// Import a note whose link points nowhere.
	archiveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(archiveDir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, contents string) {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", `{"format":"notrios-archive","version":1,"query":"","notes":1}`)
	write("notebooks.json", `[{"path":"Docs"}]`)
	write(filepath.Join("notes", "doc_broken.md"),
		"---\nid: doc_broken\ntitle: Broken\nnotebook: Docs\n---\n\n[gone](document://default/documents/doc_absent)\n")
	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir); result.exitCode != 0 {
		t.Fatalf("seed import: %s", result.stderr)
	}

	dirty := runCLI(t, binary, "lint", "--db", dbPath, "--asset-store", assetStore)
	if dirty.exitCode != 1 {
		t.Fatalf("findings should exit 1, got %d", dirty.exitCode)
	}
	report := decodeCLIJSON(t, dirty.stdout)
	if report["total_findings"].(float64) < 1 {
		t.Fatalf("expected the broken link to be reported: %s", dirty.stdout)
	}

	quiet := runCLI(t, binary, "lint", "--db", dbPath, "--asset-store", assetStore, "--quiet")
	if quiet.exitCode != 1 || strings.TrimSpace(quiet.stdout) != "" {
		t.Fatalf("quiet mode: exit %d stdout %q", quiet.exitCode, quiet.stdout)
	}

	// Selecting a check the library does not violate is clean again.
	selective := runCLI(t, binary, "lint", "--db", dbPath, "--asset-store", assetStore, "--checks", "missing_title")
	if selective.exitCode != 0 {
		t.Fatalf("selective lint: exit %d %s", selective.exitCode, selective.stdout)
	}

	if listed := runCLI(t, binary, "lint", "--list-checks"); listed.exitCode != 0 ||
		!strings.Contains(listed.stdout, "broken_document_link") {
		t.Fatalf("--list-checks: exit %d %s", listed.exitCode, listed.stdout)
	}
}
