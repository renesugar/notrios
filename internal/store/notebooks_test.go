package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func newNotebookTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return st
}

func TestNotebookAndTrashKeysetPages(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		doc, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: fmt.Sprintf("page_doc_%02d", i),
			Title:       fmt.Sprintf("Page %02d", i),
		})
		if err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		if err := st.DeleteDocument(ctx, DeleteDocumentRequest{
			ID:             doc.ID,
			BaseRevisionID: doc.CurrentRevisionID,
		}); err != nil {
			t.Fatalf("DeleteDocument: %v", err)
		}
	}

	trash1, err := st.ListTrash(ctx, DocumentPageRequest{Limit: 2})
	if err != nil || len(trash1.Documents) != 2 || trash1.NextCursor == "" {
		t.Fatalf("trash page 1: %+v err=%v", trash1, err)
	}
	trash2, err := st.ListTrash(ctx, DocumentPageRequest{Limit: 2, Cursor: trash1.NextCursor})
	if err != nil || len(trash2.Documents) != 2 || trash2.NextCursor == "" {
		t.Fatalf("trash page 2: %+v err=%v", trash2, err)
	}
	trash3, err := st.ListTrash(ctx, DocumentPageRequest{Limit: 2, Cursor: trash2.NextCursor})
	if err != nil || len(trash3.Documents) != 1 || trash3.NextCursor != "" {
		t.Fatalf("trash page 3: %+v err=%v", trash3, err)
	}

	for _, page := range []DocumentPage{trash1, trash2, trash3} {
		for _, doc := range page.Documents {
			if _, err := st.RestoreDocument(ctx, doc.ID); err != nil {
				t.Fatalf("RestoreDocument: %v", err)
			}
		}
	}
	notebook1, err := st.ListNotebookDocuments(ctx, DefaultNotebookID, DocumentPageRequest{Limit: 3})
	if err != nil || len(notebook1.Documents) != 3 || notebook1.NextCursor == "" {
		t.Fatalf("notebook page 1: %+v err=%v", notebook1, err)
	}
	notebook2, err := st.ListNotebookDocuments(ctx, DefaultNotebookID, DocumentPageRequest{Limit: 3, Cursor: notebook1.NextCursor})
	if err != nil || len(notebook2.Documents) != 2 || notebook2.NextCursor != "" {
		t.Fatalf("notebook page 2: %+v err=%v", notebook2, err)
	}
	if _, err := st.ListNotebookDocuments(ctx, HelpNotebookID, DocumentPageRequest{
		Limit:  3,
		Cursor: notebook1.NextCursor,
	}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("notebook cursor must be bound to its notebook: %v", err)
	}
}

func TestBootstrapCreatesBuiltinNotebooks(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	notes, err := st.GetNotebook(ctx, DefaultNotebookID)
	if err != nil || notes.Name != "Notes" || notes.Builtin {
		t.Fatalf("default notebook: %+v err=%v", notes, err)
	}
	// The read-only builtins, in the order they sit above Trash. Bootstrap
	// reaches an existing database too — `INSERT OR IGNORE` runs on every open —
	// so Reports needed no migration.
	for id, name := range map[string]string{HelpNotebookID: "Help", ReportsNotebookID: "Reports"} {
		nb, err := st.GetNotebook(ctx, id)
		if err != nil || nb.Name != name || !nb.Builtin {
			t.Fatalf("%s notebook: %+v err=%v", name, nb, err)
		}
	}
	recovered, err := st.GetNotebook(ctx, RecoveredNotebookID)
	if err != nil || recovered.Name != "Recovered" || !recovered.Builtin || IsReadOnlyNotebook(recovered.ID) {
		t.Fatalf("Recovered notebook: %+v err=%v", recovered, err)
	}

	sns, err := st.ListSearchNotebooks(ctx)
	if err != nil {
		t.Fatalf("ListSearchNotebooks: %v", err)
	}
	if len(sns) != 2 {
		t.Fatalf("expected 2 builtin search notebooks, got %d", len(sns))
	}
	if sns[0].ID != AllNotesSearchNotebookID || sns[0].SortAnchor != "first" || !sns[0].Builtin {
		t.Fatalf("All notes must sort first: %+v", sns[0])
	}
	if sns[len(sns)-1].ID != TrashSearchNotebookID || sns[len(sns)-1].SortAnchor != "last" {
		t.Fatalf("Trash must sort last: %+v", sns[len(sns)-1])
	}
}

func TestNotebookNamesCaseInsensitiveAmongSiblings(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	work, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work", IconEmoji: "💼"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	if work.IconEmoji != "💼" {
		t.Fatalf("emoji not stored: %+v", work)
	}
	if _, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "wOrK"}); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("expected ErrNameConflict for case-differing sibling, got %v", err)
	}
	// The same name is allowed under a different parent.
	child, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "work", ParentID: work.ID})
	if err != nil {
		t.Fatalf("nested same-name notebook should be allowed: %v", err)
	}
	if child.ParentID != work.ID {
		t.Fatalf("nesting lost: %+v", child)
	}
}

func TestNotebookNestingAndMoveCycles(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	contacts, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Contacts"})
	plumbers, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Plumbers", ParentID: contacts.ID})

	// A parent cannot be moved under its own descendant.
	if _, err := st.UpdateNotebook(ctx, UpdateNotebookRequest{ID: contacts.ID, ParentID: &plumbers.ID}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected cycle rejection, got %v", err)
	}

	// Renaming and moving to top level work.
	newName := "Electricians"
	top := ""
	updated, err := st.UpdateNotebook(ctx, UpdateNotebookRequest{ID: plumbers.ID, Name: &newName, ParentID: &top})
	if err != nil || updated.Name != "Electricians" || updated.ParentID != "" {
		t.Fatalf("UpdateNotebook: %+v err=%v", updated, err)
	}

	// Builtin notebooks cannot be renamed or deleted.
	if _, err := st.UpdateNotebook(ctx, UpdateNotebookRequest{ID: HelpNotebookID, Name: &newName}); !errors.Is(err, ErrProtected) {
		t.Fatalf("expected ErrProtected renaming Help, got %v", err)
	}
	if err := st.DeleteNotebook(ctx, HelpNotebookID); !errors.Is(err, ErrProtected) {
		t.Fatalf("expected ErrProtected deleting Help, got %v", err)
	}
	if err := st.DeleteNotebook(ctx, DefaultNotebookID); !errors.Is(err, ErrProtected) {
		t.Fatalf("expected ErrProtected deleting default notebook, got %v", err)
	}
}

func TestDocumentsJoinDefaultNotebookAndMove(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "In default", Body: "body"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if doc.NotebookID != DefaultNotebookID {
		t.Fatalf("expected default notebook membership, got %q", doc.NotebookID)
	}

	twitter, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Twitter", IconEmoji: "🐦"})
	moved, err := st.MoveDocumentToNotebook(ctx, doc.ID, twitter.ID)
	if err != nil || moved.NotebookID != twitter.ID {
		t.Fatalf("MoveDocumentToNotebook: %+v err=%v", moved, err)
	}
	if _, err := st.MoveDocumentToNotebook(ctx, doc.ID, HelpNotebookID); !errors.Is(err, ErrProtected) {
		t.Fatalf("expected ErrProtected moving into Help, got %v", err)
	}
	if _, err := st.MoveDocumentToNotebook(ctx, doc.ID, "nb_missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing notebook, got %v", err)
	}
}

func TestDeleteNotebookTrashesNotesRecursively(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	parent, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Project"})
	child, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Subproject", ParentID: parent.ID})
	docParent, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "parent note", Body: "alpha", NotebookID: parent.ID})
	docChild, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "child note", Body: "beta", NotebookID: child.ID})

	if err := st.DeleteNotebook(ctx, parent.ID); err != nil {
		t.Fatalf("DeleteNotebook: %v", err)
	}
	if _, err := st.GetNotebook(ctx, child.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("child notebook should be deleted, got %v", err)
	}
	// Notes were trashed, not lost, and are excluded from reads/search.
	if _, err := st.GetDocument(ctx, docParent.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("trashed note should not be readable, got %v", err)
	}
	res, err := st.Search(ctx, SearchRequest{Query: "beta", Limit: 10})
	if err != nil || len(res.Hits) != 0 {
		t.Fatalf("trashed notes must not appear in search: %+v err=%v", res, err)
	}
	trash, err := st.ListTrash(ctx, DocumentPageRequest{Limit: 10})
	if err != nil || len(trash.Documents) != 2 {
		t.Fatalf("expected 2 trashed notes, got %d err=%v", len(trash.Documents), err)
	}
	// Restore lands in the default notebook because the original is gone.
	restored, err := st.RestoreDocument(ctx, docChild.ID)
	if err != nil || restored.NotebookID != DefaultNotebookID {
		t.Fatalf("RestoreDocument: %+v err=%v", restored, err)
	}
	res, err = st.Search(ctx, SearchRequest{Query: "beta", Limit: 10})
	if err != nil || len(res.Hits) != 1 {
		t.Fatalf("restored note must be searchable again: %+v err=%v", res, err)
	}
}

func TestTrashRestoreAndPurge(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	doc, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Trash me", Body: "gamma delta"})
	if _, err := st.AddDocumentTag(ctx, doc.ID, "todo"); err != nil {
		t.Fatalf("AddDocumentTag: %v", err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	trash, err := st.ListTrash(ctx, DocumentPageRequest{Limit: 10})
	if err != nil || len(trash.Documents) != 1 || trash.Documents[0].ID != doc.ID || trash.Documents[0].DeletedAt.IsZero() {
		t.Fatalf("ListTrash: %+v err=%v", trash, err)
	}

	// Purging a non-trashed document is refused.
	other, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Keep", Body: "keep"})
	if err := st.PurgeDocument(ctx, other.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound purging non-trashed note, got %v", err)
	}

	if err := st.PurgeDocument(ctx, doc.ID); err != nil {
		t.Fatalf("PurgeDocument: %v", err)
	}
	trash, _ = st.ListTrash(ctx, DocumentPageRequest{Limit: 10})
	if len(trash.Documents) != 0 {
		t.Fatalf("trash should be empty after purge, got %d", len(trash.Documents))
	}
	if _, err := st.ListDocumentRevisions(ctx, doc.ID); err == nil {
		// Revisions are gone; listing returns empty rather than error in some
		// stores, so verify emptiness explicitly.
		revs, _ := st.ListDocumentRevisions(ctx, doc.ID)
		if len(revs) != 0 {
			t.Fatalf("revisions should be purged, got %d", len(revs))
		}
	}
	tags, _ := st.ListTags(ctx, TagQuery{})
	if len(tags.Tags) != 0 {
		t.Fatalf("orphaned tags should be removed after purge, got %+v", tags)
	}
}

func TestTagsAddListRemoveCounts(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	doc1, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "one", Body: "one"})
	doc2, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "two", Body: "two"})

	if _, err := st.AddDocumentTag(ctx, doc1.ID, "Shopping Mall"); err != nil {
		t.Fatalf("AddDocumentTag: %v", err)
	}
	// Case-insensitive reuse of the same tag; count reflects both notes.
	tag, err := st.AddDocumentTag(ctx, doc2.ID, "shopping mall")
	if err != nil {
		t.Fatalf("AddDocumentTag (reuse): %v", err)
	}
	if tag.NoteCount != 2 || !strings.EqualFold(tag.Name, "shopping mall") {
		t.Fatalf("tag reuse mismatch: %+v", tag)
	}

	all, err := st.ListTags(ctx, TagQuery{})
	if err != nil || len(all.Tags) != 1 || all.Tags[0].NoteCount != 2 {
		t.Fatalf("ListTags: %+v err=%v", all, err)
	}
	docTags, err := st.ListDocumentTags(ctx, doc1.ID)
	if err != nil || len(docTags) != 1 {
		t.Fatalf("ListDocumentTags: %+v err=%v", docTags, err)
	}

	if err := st.RemoveDocumentTag(ctx, doc1.ID, "SHOPPING MALL"); err != nil {
		t.Fatalf("RemoveDocumentTag: %v", err)
	}
	all, _ = st.ListTags(ctx, TagQuery{})
	if len(all.Tags) != 1 || all.Tags[0].NoteCount != 1 {
		t.Fatalf("count after removal: %+v", all)
	}
	if err := st.RemoveDocumentTag(ctx, doc2.ID, "shopping mall"); err != nil {
		t.Fatalf("RemoveDocumentTag: %v", err)
	}
	all, _ = st.ListTags(ctx, TagQuery{})
	if len(all.Tags) != 0 {
		t.Fatalf("unreferenced tag should be deleted: %+v", all)
	}
}

func TestSearchNotebookLifecycle(t *testing.T) {
	st := newNotebookTestStore(t)
	ctx := context.Background()

	todo, err := st.CreateSearchNotebook(ctx, CreateSearchNotebookRequest{Name: "TODO", Query: "tag:todo", IconEmoji: "✅"})
	if err != nil {
		t.Fatalf("CreateSearchNotebook: %v", err)
	}
	if todo.Builtin || todo.SortAnchor != "normal" || todo.Query != "tag:todo" {
		t.Fatalf("unexpected search notebook: %+v", todo)
	}
	if _, err := st.CreateSearchNotebook(ctx, CreateSearchNotebookRequest{Name: "todo", Query: "x"}); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("expected ErrNameConflict, got %v", err)
	}
	if _, err := st.CreateSearchNotebook(ctx, CreateSearchNotebookRequest{Name: "Broken", Query: "alpha OR"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid saved expression must be rejected, got %v", err)
	}

	// Sidebar order: All notes, TODO, Trash.
	sns, _ := st.ListSearchNotebooks(ctx)
	if len(sns) != 3 || sns[0].ID != AllNotesSearchNotebookID || sns[1].ID != todo.ID || sns[2].ID != TrashSearchNotebookID {
		t.Fatalf("sidebar order wrong: %+v", sns)
	}

	// Deleting a user search notebook never deletes notes.
	doc, _ := st.CreateDocument(ctx, CreateDocumentRequest{Title: "note stays", Body: "epsilon"})
	if err := st.DeleteSearchNotebook(ctx, todo.ID); err != nil {
		t.Fatalf("DeleteSearchNotebook: %v", err)
	}
	if _, err := st.GetDocument(ctx, doc.ID); err != nil {
		t.Fatalf("note must survive search-notebook deletion: %v", err)
	}
	if err := st.DeleteSearchNotebook(ctx, AllNotesSearchNotebookID); !errors.Is(err, ErrProtected) {
		t.Fatalf("expected ErrProtected for All notes, got %v", err)
	}
	if err := st.DeleteSearchNotebook(ctx, TrashSearchNotebookID); !errors.Is(err, ErrProtected) {
		t.Fatalf("expected ErrProtected for Trash, got %v", err)
	}
}

func TestSchemaV4DatabaseUpgradesToV5(t *testing.T) {
	// Simulate a pre-notebook database: create the v4 shape, then bootstrap.
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	v4 := `CREATE TABLE documents (
		id TEXT PRIMARY KEY,
		collection_id TEXT NOT NULL,
		title TEXT NOT NULL,
		body_mime_type TEXT NOT NULL DEFAULT 'text/markdown',
		current_revision_id TEXT,
		deleted_at TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	INSERT INTO documents(id, collection_id, title) VALUES('doc_old', 'default', 'Old note');
	PRAGMA user_version = 4;`
	if err := st.Exec(ctx, v4); err != nil {
		t.Fatalf("create v4 shape: %v", err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap over v4 database: %v", err)
	}
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("expected schema version %d, got %+v err=%v", CurrentSchemaVersion, status, err)
	}
	// Existing rows are backfilled into the default notebook.
	count, err := st.countLocked(`SELECT COUNT(1) FROM documents WHERE id = 'doc_old' AND notebook_id = ?`, DefaultNotebookID)
	if err != nil || count != 1 {
		t.Fatalf("backfill failed: count=%d err=%v", count, err)
	}
}
