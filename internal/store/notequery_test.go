package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func newNoteQueryFixture(t *testing.T) *SQLiteStore {
	t.Helper()
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	notebook, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Recipes"})
	if err != nil {
		t.Fatal(err)
	}
	for index, entry := range []struct {
		id, title, body, tag string
	}{
		{"nq_a", "Sourdough", "flour water salt", "todo"},
		{"nq_b", "Focaccia", "flour water olive oil", "todo"},
		{"nq_c", "Done bread", "already baked", "done"},
	} {
		doc, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: entry.id, Title: entry.title, Body: entry.body, NotebookID: notebook.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddDocumentTag(ctx, doc.ID, entry.tag); err != nil {
			t.Fatal(err)
		}
		// Distinct, real timestamps so a chronological order is deterministic:
		// nq_a is oldest and nq_c newest.
		stamp := fmt.Sprintf("2026-01-0%d 00:00:00", index+1)
		if err := st.Exec(ctx, "UPDATE documents SET updated_at = '"+stamp+"' WHERE id = '"+entry.id+"'"); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

func run(t *testing.T, st *SQLiteStore, block string) NoteQueryResult {
	t.Helper()
	result, err := st.RunNoteQuery(context.Background(), NoteQueryRequest{Block: block})
	if err != nil {
		t.Fatalf("RunNoteQuery returned an error rather than a rendered message: %v", err)
	}
	return result
}

func titles(result NoteQueryResult) []string {
	out := make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		out = append(out, row.Title)
	}
	return out
}

func TestNoteQueryRunsTheQueryLanguage(t *testing.T) {
	st := newNoteQueryFixture(t)

	result := run(t, st, "query: tag:todo")
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("rows = %v, want the two todo notes", titles(result))
	}
	// The whole Q1 grammar is available because the block reaches the same
	// parser: negation, fields, grouping, uppercase OR.
	negated := run(t, st, "query: flour -tag:done")
	if len(negated.Rows) != 2 {
		t.Fatalf("negation not honoured: %v", titles(negated))
	}
	grouped := run(t, st, `query: (Sourdough OR Focaccia) tag:todo`)
	if len(grouped.Rows) != 2 {
		t.Fatalf("grouping not honoured: %v", titles(grouped))
	}
}

func TestNoteQueryFieldsAreOptInAndOrdered(t *testing.T) {
	st := newNoteQueryFixture(t)

	// A block that asks for nothing gets a title and nothing else — a query
	// block is not a way to pull note bodies into a page that only wanted a
	// list.
	bare := run(t, st, "query: tag:todo")
	if bare.Rows[0].Snippet != "" || bare.Rows[0].Notebook != "" || len(bare.Rows[0].Tags) != 0 {
		t.Fatalf("unrequested fields were returned: %+v", bare.Rows[0])
	}
	if bare.Rows[0].Title == "" || bare.Rows[0].URI == "" {
		t.Fatalf("a row always carries a title and a URI: %+v", bare.Rows[0])
	}

	full := run(t, st, "query: tag:todo\nfields: snippet, tags, notebook, updated")
	row := full.Rows[0]
	if row.Notebook != "Recipes" || len(row.Tags) == 0 || row.UpdatedAt == nil || row.Snippet == "" {
		t.Fatalf("requested fields missing: %+v", row)
	}
	// Rendered in canonical order regardless of how they were written.
	if strings.Join(full.Spec.Fields, ",") != "title,notebook,tags,updated,snippet" {
		t.Fatalf("fields = %v, want canonical order", full.Spec.Fields)
	}
}

func TestNoteQueryLimitAndTruncation(t *testing.T) {
	st := newNoteQueryFixture(t)

	limited := run(t, st, "query: flour\nlimit: 1")
	if len(limited.Rows) != 1 {
		t.Fatalf("limit not applied: %d rows", len(limited.Rows))
	}
	if !limited.Truncated {
		t.Fatalf("more notes match, so the block must say it was truncated: %+v", limited)
	}
	full := run(t, st, "query: flour\nlimit: 10")
	if full.Truncated {
		t.Fatalf("nothing was hidden, so truncated must be false: %+v", full)
	}
	if full.Spec.Limit != 10 {
		t.Fatalf("spec should report what ran: %+v", full.Spec)
	}
}

// The order a block asks for is the order it gets. Before E7 the search chose
// relevance for a text query and chronological otherwise, with no way to say.
func TestNoteQuerySortIsHonouredRatherThanImplied(t *testing.T) {
	st := newNoteQueryFixture(t)

	updated := run(t, st, "query: flour\nsort: updated")
	if updated.Error != "" {
		t.Fatalf("unexpected error: %s", updated.Error)
	}
	if len(updated.Rows) != 2 || updated.Rows[0].Title != "Focaccia" {
		// nq_b (Focaccia) has the later updated_at.
		t.Fatalf("chronological order not honoured for a text query: %v", titles(updated))
	}
	relevance := run(t, st, "query: flour\nsort: relevance")
	if relevance.Error != "" || len(relevance.Rows) != 2 {
		t.Fatalf("relevance sort failed: %+v", relevance)
	}
	if updated.Spec.Sort != "updated" || relevance.Spec.Sort != "relevance" {
		t.Fatalf("the spec must report the order that ran: %q / %q", updated.Spec.Sort, relevance.Spec.Sort)
	}
}

// A broken block renders a message. It must never fail the note around it,
// which is why these are values rather than errors.
func TestNoteQueryReportsBadBlocksWithoutFailing(t *testing.T) {
	st := newNoteQueryFixture(t)

	for name, block := range map[string]string{
		"no query line":   "limit: 5",
		"unknown key":     "query: tag:todo\nexec: rm -rf /",
		"not key value":   "query: tag:todo\njust some prose",
		"bad limit":       "query: tag:todo\nlimit: none",
		"limit over cap":  "query: tag:todo\nlimit: 101",
		"bad sort":        "query: tag:todo\nsort: sideways",
		"unknown field":   "query: tag:todo\nfields: body",
		"repeated key":    "query: a\nquery: b",
		"malformed query": "query: (unclosed",
	} {
		result := run(t, st, block)
		if result.Error == "" {
			t.Fatalf("%s: expected a rendered error, got %d rows", name, len(result.Rows))
		}
		if len(result.Rows) != 0 {
			t.Fatalf("%s: a failed block must return no rows", name)
		}
	}

	// The unknown-key message names the keys that do work, so the error is
	// actionable inside the note.
	unknown := run(t, st, "query: tag:todo\nexec: whatever")
	for _, key := range NoteQueryKeys() {
		if !strings.Contains(unknown.Error, key) {
			t.Fatalf("error should list the usable keys, got %q", unknown.Error)
		}
	}
}

func TestNoteQueryRefusesOversizedBlocks(t *testing.T) {
	st := newNoteQueryFixture(t)

	long := run(t, st, "query: "+strings.Repeat("a", MaxNoteQueryBlockBytes))
	if long.Error == "" {
		t.Fatalf("an oversized block must be refused")
	}
	manyLines := run(t, st, "query: tag:todo"+strings.Repeat("\n# comment", MaxNoteQueryLines+1))
	if manyLines.Error == "" {
		t.Fatalf("a block with too many lines must be refused")
	}
}

func TestNoteQueryIgnoresBlankLinesAndComments(t *testing.T) {
	st := newNoteQueryFixture(t)
	result := run(t, st, "\n# what is still to do\n\nquery: tag:todo\n\nlimit: 5\n")
	if result.Error != "" || len(result.Rows) != 2 {
		t.Fatalf("comments and blank lines should be ignored: %+v", result)
	}
}

// A query block reaches the ordinary search, so it sees exactly what its author
// could search for and nothing else — no trashed notes, no other collection.
func TestNoteQuerySeesOnlyWhatSearchSees(t *testing.T) {
	st := newNoteQueryFixture(t)
	ctx := context.Background()

	doc, err := st.GetDocument(ctx, "nq_a")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: "nq_a", BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	result := run(t, st, "query: tag:todo")
	for _, row := range result.Rows {
		if row.DocumentID == "nq_a" {
			t.Fatalf("a trashed note appeared in a query block: %v", titles(result))
		}
	}
}

// Evaluating a block is a read. Nothing about running one may change a note.
func TestNoteQueryWritesNothing(t *testing.T) {
	st := newNoteQueryFixture(t)
	ctx := context.Background()

	before, err := st.GetDocument(ctx, "nq_b")
	if err != nil {
		t.Fatal(err)
	}
	revisionsBefore, err := st.ListDocumentRevisions(ctx, "nq_b")
	if err != nil {
		t.Fatal(err)
	}
	run(t, st, "query: tag:todo\nfields: tags, notebook, snippet, updated\nlimit: 50")
	after, err := st.GetDocument(ctx, "nq_b")
	if err != nil {
		t.Fatal(err)
	}
	revisionsAfter, err := st.ListDocumentRevisions(ctx, "nq_b")
	if err != nil {
		t.Fatal(err)
	}
	if before.CurrentRevisionID != after.CurrentRevisionID || len(revisionsBefore) != len(revisionsAfter) {
		t.Fatalf("running a query block changed the library")
	}
}

func TestNoteQuerySpecParsing(t *testing.T) {
	spec, err := parseNoteQueryBlock("query: tag:todo\nlimit: 3\nsort: relevance\nfields: updated")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Query != "tag:todo" || spec.Limit != 3 || spec.Sort != "relevance" {
		t.Fatalf("spec = %+v", spec)
	}
	// Title is always present even when the block does not name it.
	if len(spec.Fields) != 2 || spec.Fields[0] != NoteQueryFieldTitle {
		t.Fatalf("fields = %v, want title first", spec.Fields)
	}
	// The default order is chronological: relevance needs text to rank, and a
	// block with no text terms has none.
	defaults, err := parseNoteQueryBlock("query: tag:todo")
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Sort != NoteQuerySortUpdated || defaults.Limit != DefaultNoteQueryRows {
		t.Fatalf("defaults = %+v", defaults)
	}
}
