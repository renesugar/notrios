package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestKeyTableIsCurrent is the gate. The generator is only worth having if
// something fails when the table and the code disagree, and the README's
// project status had just finished proving what an ungated derived document
// becomes.
func TestKeyTableIsCurrent(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))

	comments, err := fieldComments(filepath.Join(root, "internal/config"))
	if err != nil {
		t.Fatal(err)
	}
	rows, sections := configRows(comments)
	rendered, err := render(filepath.Join(root, docPath), rows, sections)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, docPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != rendered {
		t.Error("the configuration key table is stale; run: go run ./cmd/docconfig --write")
	}
}

// Every key must reach the table, and every row must correspond to a key. The
// walk is recursive over anonymous struct types, and a table that quietly
// stopped descending would still look like a table.
func TestEverySettableKeyIsListed(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
	comments, err := fieldComments(filepath.Join(root, "internal/config"))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := configRows(comments)

	contents, err := os.ReadFile(filepath.Join(root, docPath))
	if err != nil {
		t.Fatal(err)
	}
	body := string(contents)
	for _, r := range rows {
		if !strings.Contains(body, "| `"+r.path+"` |") {
			t.Errorf("%s is settable and not in the table", r.path)
		}
	}

	// The nested case, named so it cannot silently stop being covered:
	// sync.rest lives one level deeper than every other section.
	found := false
	for _, r := range rows {
		if strings.HasPrefix(r.path, "sync.rest.") {
			found = true
			break
		}
	}
	if !found {
		t.Error("no sync.rest.* key was collected, so the walk is not descending past one level")
	}
}
