package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// migratedLibrary is the case these commands exist for: notes that came from
// somewhere else and carry the collection identifier saying so.
func migratedLibrary(t *testing.T) (*store.SQLiteStore, map[string][]store.Document) {
	t.Helper()
	root := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatalf("opening a library: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrapping the library: %v", err)
	}
	for _, collection := range []struct{ id, name string }{
		{"joplin", "Joplin"},
		{"obsidian", "Obsidian"},
		{"empty-import", "Empty import"},
	} {
		if _, err := st.EnsureCollection(ctx, collection.id, collection.name); err != nil {
			t.Fatalf("creating collection %s: %v", collection.id, err)
		}
	}
	created := map[string][]store.Document{}
	for _, note := range []struct{ title, collection string }{
		{"from joplin one", "joplin"},
		{"from joplin two", "joplin"},
		{"from obsidian", "obsidian"},
		{"written here", "default"},
	} {
		document, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			Title: note.title, Body: "body", CollectionID: note.collection,
		})
		if err != nil {
			t.Fatalf("creating %q: %v", note.title, err)
		}
		created[note.collection] = append(created[note.collection], document)
	}
	return st, created
}

func TestCollectionRowsCountTheNotesThatNameEachCollection(t *testing.T) {
	st, _ := migratedLibrary(t)
	rows := collectionRows(context.Background(), st)

	counts := map[string]int{}
	for _, row := range rows {
		counts[row.ID] = row.Notes
	}
	for id, want := range map[string]int{"joplin": 2, "obsidian": 1, "default": 1} {
		if counts[id] != want {
			t.Errorf("collection %q has %d notes, want %d", id, counts[id], want)
		}
	}
	if _, ok := counts["empty-import"]; !ok {
		t.Error("a collection with no notes was not listed; it is the one worth seeing after an import")
	}
	if counts["empty-import"] != 0 {
		t.Errorf("empty-import has %d notes, want 0", counts["empty-import"])
	}
}

// TestATrashedNoteDoesNotInflateACollectionCount guards the number people would
// act on: a count that includes Trash says an import landed more than it did.
func TestATrashedNoteDoesNotInflateACollectionCount(t *testing.T) {
	st, created := migratedLibrary(t)
	ctx := context.Background()

	before := 0
	for _, row := range collectionRows(ctx, st) {
		if row.ID == "joplin" {
			before = row.Notes
		}
	}
	first := created["joplin"][0]
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{
		ID: first.ID, BaseRevisionID: first.CurrentRevisionID,
	}); err != nil {
		t.Fatalf("trashing %s: %v", first.ID, err)
	}
	after := 0
	for _, row := range collectionRows(ctx, st) {
		if row.ID == "joplin" {
			after = row.Notes
		}
	}
	if after != before-1 {
		t.Errorf("joplin counted %d notes after one was trashed, want %d", after, before-1)
	}
}

// TestTheCollectionsCommandIsDescribedAndReachable is the pairing this item
// exists to prevent going wrong again: a command that works and that no help
// text admits to.
func TestTheCollectionsCommandIsDescribedAndReachable(t *testing.T) {
	dispatched := dispatchTree(t)
	for _, command := range []string{"collections list", "collections show"} {
		if !dispatched[command] {
			t.Errorf("%q is not dispatched", command)
		}
	}
	out, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli.md"))
	if err != nil {
		t.Fatalf("reading the CLI guide: %v", err)
	}
	for _, command := range []string{"notriosctl collections list", "notriosctl collections show"} {
		if !strings.Contains(string(out), command) {
			t.Errorf("the published CLI guide does not mention %q", command)
		}
	}
}
