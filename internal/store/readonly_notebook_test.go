package store

import (
	"context"
	"errors"
	"testing"
)

// The trap this slice was warned about. "Builtin" and "read-only" are two
// different sets, and the default Notes notebook is in the first and not the
// second: it is created by bootstrap and cannot be deleted, but its content is
// the user's. Treating it as read-only would exclude most of the library —
// silently — from the graph report, from publications, and from lint.
//
// This test exists so that widening the set by reaching for the more familiar
// word fails here rather than in a user's library.
func TestTheDefaultNotebookIsNotReadOnly(t *testing.T) {
	if IsReadOnlyNotebook(DefaultNotebookID) {
		t.Fatalf("the default %q notebook holds the user's own notes; only system-authored notebooks are read-only", DefaultNotebookID)
	}
	for _, id := range []string{HelpNotebookID, ReportsNotebookID} {
		if !IsReadOnlyNotebook(id) {
			t.Fatalf("%q is system-authored and must be read-only", id)
		}
		if ReadOnlyNotebookName(id) == id {
			t.Fatalf("%q should have a human name for refusal messages", id)
		}
	}
	if IsReadOnlyNotebook("nb_anything_else") {
		t.Fatal("the set is closed; a user notebook is never read-only")
	}
}

// Undeletable is the wider set, and DeleteNotebook needs both checks because
// Notes is `builtin = 0` in the database. Asserting the pair here keeps the two
// rules visibly distinct.
func TestBuiltinAndReadOnlyAreDifferentSets(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)

	notes, err := st.GetNotebook(ctx, DefaultNotebookID)
	if err != nil {
		t.Fatalf("GetNotebook(Notes): %v", err)
	}
	if notes.Builtin {
		t.Fatal("Notes is deliberately builtin = 0; its content belongs to the user")
	}
	reports, err := st.GetNotebook(ctx, ReportsNotebookID)
	if err != nil {
		t.Fatalf("GetNotebook(Reports): %v", err)
	}
	if !reports.Builtin || reports.Name != "Reports" {
		t.Fatalf("Reports should be seeded as a builtin notebook: %+v", reports)
	}

	// Undeletable covers both, by two different routes.
	for _, id := range []string{DefaultNotebookID, HelpNotebookID, ReportsNotebookID} {
		if err := st.DeleteNotebook(ctx, id); !errors.Is(err, ErrProtected) {
			t.Fatalf("DeleteNotebook(%q) = %v, want ErrProtected", id, err)
		}
	}
}

// Reports is protected the same way Help is, and it gained that protection from
// the predicate rather than from thirteen new comparisons.
func TestReportsNotebookIsProtectedLikeHelp(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)

	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Ordinary", Body: "hello\n"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if _, err := st.MoveDocumentToNotebook(ctx, doc.ID, ReportsNotebookID); !errors.Is(err, ErrProtected) {
		t.Fatalf("moving a note into Reports = %v, want ErrProtected", err)
	}

	report, err := st.CreateDocument(ctx, CreateDocumentRequest{
		Title: "Generated", Body: "x\n", NotebookID: ReportsNotebookID,
	})
	if err != nil {
		t.Fatalf("CreateDocument in Reports: %v", err)
	}
	if _, err := st.MoveDocumentToNotebook(ctx, report.ID, DefaultNotebookID); !errors.Is(err, ErrProtected) {
		t.Fatalf("moving a note out of Reports = %v, want ErrProtected", err)
	}
	newName := "Renamed"
	if _, err := st.UpdateNotebook(ctx, UpdateNotebookRequest{ID: ReportsNotebookID, Name: &newName}); !errors.Is(err, ErrProtected) {
		t.Fatalf("renaming Reports = %v, want ErrProtected", err)
	}
}
