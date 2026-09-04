package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestNotebookCreateAndList covers the commands added when it became clear that
// the command line's inability to make a notebook was a missing adapter rather
// than a boundary. The capability lives in the store; REST, MCP, the interface
// and this are four ways to reach it.
func TestNotebookCreateAndList(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}

	created := runCLIIn(t, sandbox, binary, append([]string{"notebooks", "create", "--name", "Field notes"}, roots...)...)
	if created.exitCode != 0 {
		t.Fatalf("notebooks create: %s", created.stderr)
	}
	var notebook map[string]any
	if err := json.Unmarshal([]byte(created.stdout), &notebook); err != nil {
		t.Fatal(err)
	}
	if notebook["name"] != "Field notes" || notebook["notebook_id"] == "" {
		t.Fatalf("unexpected notebook: %v", notebook)
	}

	// Nesting resolves the parent by name, and refuses an ambiguous one for the
	// reason `notes move` gives: names are unique only among siblings.
	nested := runCLIIn(t, sandbox, binary, append([]string{"notebooks", "create", "--name", "Dusk", "--parent", "Field notes"}, roots...)...)
	if nested.exitCode != 0 {
		t.Fatalf("nested notebooks create: %s", nested.stderr)
	}
	if !strings.Contains(nested.stdout, notebook["notebook_id"].(string)) {
		t.Errorf("the nested notebook does not record its parent: %s", nested.stdout)
	}

	// A note can now be filed into a notebook made from the same surface, which
	// is what "create a note in a specific notebook" needed.
	note := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", "Reed beds", "--body", "dusk", "--notebook", "Field notes"}, roots...)...)
	if note.exitCode != 0 {
		t.Fatalf("filing a note into a new notebook: %s", note.stderr)
	}
	if !strings.Contains(note.stdout, notebook["notebook_id"].(string)) {
		t.Errorf("the note was not filed into the notebook: %s", note.stdout)
	}

	query := runCLIIn(t, sandbox, binary, append([]string{"notebooks", "create", "--name", "Todo", "--query", "tag:todo"}, roots...)...)
	if query.exitCode != 0 {
		t.Fatalf("query notebook: %s", query.stderr)
	}
	if !strings.Contains(query.stdout, "tag:todo") {
		t.Errorf("the query notebook does not record its query: %s", query.stdout)
	}

	listed := runCLIIn(t, sandbox, binary, append([]string{"notebooks", "list"}, roots...)...)
	for _, want := range []string{"Field notes", "Dusk", "Todo", "query_notebooks"} {
		if !strings.Contains(listed.stdout, want) {
			t.Errorf("notebooks list omits %q: %s", want, listed.stdout)
		}
	}
}

// TestQueryNotebookRefusesAParent guards a distinction worth keeping: a
// notebook's contents are what someone filed there, and a query notebook's are
// whatever matches. Nesting the second would suggest a containment that is not
// there.
func TestQueryNotebookRefusesAParent(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}

	runCLIIn(t, sandbox, binary, append([]string{"notebooks", "create", "--name", "Field notes"}, roots...)...)
	result := runCLIIn(t, sandbox, binary,
		append([]string{"notebooks", "create", "--name", "Todo", "--query", "tag:todo", "--parent", "Field notes"}, roots...)...)
	if result.exitCode == 0 {
		t.Fatalf("a query notebook with a parent must be refused")
	}
	if !strings.Contains(result.stderr, "contents come from the query") {
		t.Errorf("the refusal must say why: %q", result.stderr)
	}
}
