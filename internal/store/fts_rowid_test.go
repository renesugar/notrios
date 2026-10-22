package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestJ18SearchIsUnchangedByTheRowidMapping is J18's correctness boundary:
// speed is not the property being defended here, sameness is.
//
// The change swapped `DELETE FROM documents_fts WHERE document_id = ?` for a
// delete by recorded rowid at ten call sites. If the mapping is ever wrong, the
// wrong row leaves the index — and the symptom is not a crash, it is a note
// that has quietly stopped being findable, or one that is found after being
// deleted. Neither shows up in a timing.
//
// So this exercises the paths that write the index and asserts what a reader
// would notice: the right documents come back, and only those.
func TestJ18SearchIsUnchangedByTheRowidMapping(t *testing.T) {
	ctx := context.Background()
	st := openTemporaryStore(t)

	created := map[string]string{}
	for index, body := range []string{
		"alpha kestrel banana", "beta kestrel cherry", "gamma osprey date",
	} {
		title := fmt.Sprintf("note %d", index)
		document, err := st.CreateDocument(ctx, CreateDocumentRequest{
			CollectionID: "default", Title: title, Body: body,
		})
		if err != nil {
			t.Fatal(err)
		}
		created[title] = document.ID
	}

	hits := func(query string) []string {
		found, err := st.Search(ctx, SearchRequest{Query: query, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		ids := []string{}
		for _, hit := range found.Hits {
			ids = append(ids, hit.ID)
		}
		return ids
	}

	if got := hits("kestrel"); len(got) != 2 {
		t.Fatalf("after create, kestrel matches %d notes, want 2: %v", len(got), got)
	}

	// An update rewrites the index row. The old body must stop matching and the
	// new one must start -- which is exactly what a stale rowid would break,
	// by deleting somebody else's row and leaving this one behind.
	updateBody(t, st, ctx, created["note 0"], "note 0", "alpha osprey banana")
	if got := hits("kestrel"); len(got) != 1 || got[0] != created["note 1"] {
		t.Fatalf("after update, kestrel matches %v, want only %s", got, created["note 1"])
	}
	if got := hits("osprey"); len(got) != 2 {
		t.Fatalf("after update, osprey matches %d notes, want 2: %v", len(got), got)
	}

	// A second update on the same document: the mapping must have been
	// refreshed by the first one, not left pointing at the row it removed.
	updateBody(t, st, ctx, created["note 0"], "note 0", "alpha wren banana")
	if got := hits("osprey"); len(got) != 1 || got[0] != created["note 2"] {
		t.Fatalf("after the second update, osprey matches %v, want only %s", got, created["note 2"])
	}
	if got := hits("wren"); len(got) != 1 || got[0] != created["note 0"] {
		t.Fatalf("wren matches %v, want only %s", got, created["note 0"])
	}

	// Deletion removes the document from the index and nothing else from it.
	deleteNote(t, st, ctx, created["note 1"])
	if got := hits("kestrel"); len(got) != 0 {
		t.Fatalf("a deleted note is still findable: %v", got)
	}
	if got := hits("banana"); len(got) != 1 || got[0] != created["note 0"] {
		t.Fatalf("deleting one note disturbed another: banana matches %v", got)
	}
}

// The mapping must hold exactly one row per indexed document. A leak leaves a
// rowid that a later delete would follow to whatever row has taken its place.
func TestJ18MappingTracksTheIndexExactly(t *testing.T) {
	ctx := context.Background()
	st := openTemporaryStore(t)

	ids := []string{}
	for index := 0; index < 5; index++ {
		document, err := st.CreateDocument(ctx, CreateDocumentRequest{
			CollectionID: "default", Title: fmt.Sprintf("n%d", index), Body: "body text",
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, document.ID)
	}
	for _, id := range ids[:2] {
		updateBody(t, st, ctx, id, "updated", "body text changed")
	}
	deleteNote(t, st, ctx, ids[4])

	indexed, err := countForTest(st, "SELECT count(*) FROM documents_fts")
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := countForTest(st, "SELECT count(*) FROM documents_fts_rowid")
	if err != nil {
		t.Fatal(err)
	}
	if indexed != mapped {
		t.Fatalf("%d rows in the index and %d in the mapping; they must agree", indexed, mapped)
	}
	// And every mapping entry points at a row that is really there.
	dangling, err := countForTest(st,
		`SELECT count(*) FROM documents_fts_rowid m
		 WHERE NOT EXISTS (SELECT 1 FROM documents_fts f WHERE f.rowid = m.fts_rowid)`)
	if err != nil {
		t.Fatal(err)
	}
	if dangling != 0 {
		t.Fatalf("%d mapping rows point at nothing", dangling)
	}
}

func openTemporaryStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := OpenSQLite(filepath.Join(t.TempDir(), "notes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("TZ", "UTC")
	return st
}

// countForTest reads one integer through the store's own connection, so the
// assertion sees exactly what the store sees.
func countForTest(s *SQLiteStore, query string) (int, error) {
	text, err := s.syncJournalTextForTest(`SELECT CAST((` + query + `) AS TEXT)`)
	if err != nil {
		return 0, err
	}
	value := 0
	if _, err := fmt.Sscanf(text, "%d", &value); err != nil {
		return 0, err
	}
	return value, nil
}

// updateBody reads the current revision first, because UpdateDocument refuses a
// write that does not name the revision it is replacing -- the store's
// optimistic-concurrency guard, and not something a test should route around.
func updateBody(t *testing.T, s *SQLiteStore, ctx context.Context, id, title, body string) {
	t.Helper()
	current, err := s.GetDocument(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: id, Title: title, Body: body, BaseRevisionID: current.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
}

// deleteNote names the revision it removes, for the same reason updateBody does.
func deleteNote(t *testing.T, s *SQLiteStore, ctx context.Context, id string) {
	t.Helper()
	current, err := s.GetDocument(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDocument(ctx, DeleteDocumentRequest{
		ID: id, BaseRevisionID: current.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
}
