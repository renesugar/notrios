package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedNotebookLibrary imports one note plus two notebooks that share a name
// under different parents — the ambiguity `notes move` has to refuse.
func seedNotebookLibrary(t *testing.T, binary, dbPath, assetStore string) {
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
	write("manifest.json", `{"format":"notrios-archive","version":1,"query":"","notes":1}`)
	write("notebooks.json", `[{"path":"Inbox"},{"path":"Contacts/Work"},{"path":"Personal/Work"}]`)
	write(filepath.Join("notes", "doc_one.md"),
		"---\nid: doc_one\ntitle: One\nnotebook: Inbox\n---\n\nfirst\n")
	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir); result.exitCode != 0 {
		t.Fatalf("seed import: %s", result.stderr)
	}
}

// The CLI could not move a note at all before v0.6 F0, though the store, REST,
// and MCP all could.
func TestNoteMoveCLI(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedNotebookLibrary(t, binary, dbPath, assetStore)

	moved := runCLI(t, binary, "notes", "move", "--db", dbPath, "--asset-store", assetStore,
		"--document", "doc_one", "--notebook", "Inbox")
	if moved.exitCode != 0 {
		t.Fatalf("move by name: exit %d %s", moved.exitCode, moved.stderr)
	}
	report := decodeCLIJSON(t, moved.stdout)
	notebookID, _ := report["notebook_id"].(string)
	if notebookID == "" {
		t.Fatalf("expected the destination notebook in the report: %s", moved.stdout)
	}

	// Moving by ID works too, and is the unambiguous form.
	again := runCLI(t, binary, "notes", "move", "--db", dbPath, "--asset-store", assetStore,
		"--document", "doc_one", "--notebook", notebookID)
	if again.exitCode != 0 {
		t.Fatalf("move by ID: exit %d %s", again.exitCode, again.stderr)
	}
}

// Notebook names are unique only among siblings, so `Work` names two
// notebooks. Picking whichever came back first is exactly the silent misfiling
// this command exists to correct.
func TestNoteMoveCLIRefusesAmbiguousNotebookName(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedNotebookLibrary(t, binary, dbPath, assetStore)

	result := runCLI(t, binary, "notes", "move", "--db", dbPath, "--asset-store", assetStore,
		"--document", "doc_one", "--notebook", "Work")
	if result.exitCode != 1 {
		t.Fatalf("an ambiguous notebook name must fail: exit %d %s", result.exitCode, result.stdout)
	}
	if !strings.Contains(result.stderr, "ambiguous") {
		t.Fatalf("the error should name the ambiguity: %s", result.stderr)
	}

	missing := runCLI(t, binary, "notes", "move", "--db", dbPath, "--asset-store", assetStore,
		"--document", "doc_one", "--notebook", "Nowhere")
	if missing.exitCode != 1 || !strings.Contains(missing.stderr, "no notebook") {
		t.Fatalf("a missing notebook should fail clearly: exit %d %s", missing.exitCode, missing.stderr)
	}
}
