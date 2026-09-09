package main

import (
	"encoding/json"
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

// TestNoteCreateWritesANote covers the command that closes v0.8 H14's most
// visible gap: a note could be made from the store, REST and MCP, and from
// nowhere a script could reach.
func TestNoteCreateWritesANote(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	args := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}

	created := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", "Reed beds", "--body", "Seen at dusk."}, args...)...)
	if created.exitCode != 0 {
		t.Fatalf("notes create failed: %s", created.stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(created.stdout), &decoded); err != nil {
		t.Fatalf("%v in %q", err, created.stdout)
	}
	documentID, _ := decoded["document_id"].(string)
	if documentID == "" || decoded["title"] != "Reed beds" {
		t.Fatalf("unexpected create output: %v", decoded)
	}
	// The note must survive into an export, which is the observable state
	// change rather than the command's own report of itself.
	out := filepath.Join(sandbox, "out")
	export := runCLIIn(t, sandbox, binary, append(append([]string{"export", "archive"}, args...), out)...)
	if export.exitCode != 0 {
		t.Fatalf("export failed: %s", export.stderr)
	}
	if !strings.Contains(export.stdout+export.stderr, "1") {
		t.Fatalf("the created note does not appear in an export: %s", export.stdout)
	}
}

func TestNoteCreateRefusals(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	args := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}

	if result := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--body", "x"}, args...)...); result.exitCode == 0 {
		t.Errorf("a note with no title must be refused")
	}
	if result := runCLIIn(t, sandbox, binary,
		append([]string{"notes", "create", "--title", "t", "--body", "x", "--body-file", "/nonexistent"}, args...)...); result.exitCode == 0 {
		t.Errorf("passing both --body and --body-file must be refused")
	}
	// A notebook that does not exist is named in the refusal. This is the
	// contrast the journey catalogue found: `import --collection` fails the
	// same case with a raw FOREIGN KEY constraint error.
	result := runCLIIn(t, sandbox, binary,
		append([]string{"notes", "create", "--title", "t", "--body", "x", "--notebook", "Nowhere"}, args...)...)
	if result.exitCode == 0 {
		t.Fatalf("an unknown notebook must be refused")
	}
	if !strings.Contains(result.stderr, "Nowhere") {
		t.Errorf("the refusal must name the notebook the user asked for; got %q", result.stderr)
	}
}

// TestNoteDeleteAndRestore covers the pair v0.8 H15 found missing: deleting was
// reachable from the store, REST, MCP and the interface and from not the
// command line.
func TestNoteDeleteAndRestore(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}

	created := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", "Reed beds", "--body", "dusk"}, roots...)...)
	var note map[string]any
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatal(err)
	}
	id, _ := note["document_id"].(string)

	deleted := runCLIIn(t, sandbox, binary, append([]string{"notes", "delete", "--document", id}, roots...)...)
	if deleted.exitCode != 0 {
		t.Fatalf("notes delete: %s", deleted.stderr)
	}
	// The undo is printed with the delete, so a person who changes their mind
	// does not have to go and find out how.
	if !strings.Contains(deleted.stdout, "notes restore --document "+id) {
		t.Errorf("delete does not say how to undo itself: %s", deleted.stdout)
	}

	// A trashed note must still be visible. Reporting "no note" here would be
	// the answer someone gets immediately after deleting, while trying to
	// confirm what happened.
	// Both forms must say so. The Markdown form names it in the front matter,
	// because a note exported as though it were an ordinary one is a wrong
	// answer with the shape of a right one.
	shown := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", id}, roots...)...)
	if shown.exitCode != 0 {
		t.Fatalf("a note in Trash must still be visible: %s", shown.stderr)
	}
	if !strings.Contains(shown.stdout, "trashed:") {
		t.Errorf("the Markdown form does not report that the note is in Trash: %s", shown.stdout)
	}
	shownJSON := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", id, "--json"}, roots...)...)
	if !strings.Contains(shownJSON.stdout, "trashed_at") {
		t.Errorf("the JSON form does not report that the note is in Trash: %s", shownJSON.stdout)
	}

	restored := runCLIIn(t, sandbox, binary, append([]string{"notes", "restore", "--document", id}, roots...)...)
	if restored.exitCode != 0 {
		t.Fatalf("notes restore: %s", restored.stderr)
	}
	after := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", id, "--json"}, roots...)...)
	if strings.Contains(after.stdout, "trashed_at") {
		t.Errorf("the note is still marked trashed after restore: %s", after.stdout)
	}

	// Restoring something that is not in Trash is refused by name rather than
	// quietly succeeding.
	again := runCLIIn(t, sandbox, binary, append([]string{"notes", "restore", "--document", id}, roots...)...)
	if again.exitCode == 0 {
		t.Errorf("restoring a note that is not in Trash must be refused")
	}
	if !strings.Contains(again.stderr, id) {
		t.Errorf("the refusal must name the note: %q", again.stderr)
	}
}
