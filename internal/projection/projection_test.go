package projection

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return st
}

func TestOutboxDrivenProjection(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	w := Writer{Dir: t.TempDir()}

	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Projected note", Body: "projection body"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if _, err := st.AddDocumentTag(ctx, doc.ID, "note taking"); err != nil {
		t.Fatalf("AddDocumentTag: %v", err)
	}
	if _, err := st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{
		DocumentID: doc.ID, SourceSystem: "twitter", ExternalID: "p1",
		Author: "Alice Smith", AuthorID: "alice@example.social",
		ThreadID: "thread-1", PublishedAt: "2026-07-13T18:42:07Z",
	}); err != nil {
		t.Fatalf("SetDocumentSource: %v", err)
	}

	report, err := SyncOutbox(ctx, st, w, 100)
	if err != nil {
		t.Fatalf("SyncOutbox: %v", err)
	}
	if report.Written == 0 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}

	path := filepath.Join(w.Dir, "notes", doc.ID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("projected file missing: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		`title: "Projected note"`,
		`author: "Alice Smith"`,
		`author_id: "alice@example.social"`,
		`thread_id: "thread-1"`,
		`notebook: "Notes"`,
		`  - "note taking"`,
		"projection body",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("projection missing %q:\n%s", want, content)
		}
	}

	// Outbox is drained: a second pass does nothing.
	report, _ = SyncOutbox(ctx, st, w, 100)
	if report.Written != 0 || report.Removed != 0 {
		t.Fatalf("outbox not drained: %+v", report)
	}

	// Soft delete removes the projected file on the next pass.
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	report, err = SyncOutbox(ctx, st, w, 100)
	if err != nil || report.Removed != 1 {
		t.Fatalf("delete sync: %+v err=%v", report, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("projected file should be removed, stat err=%v", err)
	}
}

func TestProjectionIndexesNotebookAncestorsForRecursiveSearch(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	w := Writer{Dir: t.TempDir()}
	parent, _ := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work"})
	child, _ := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Reports", ParentID: parent.ID})
	doc, _ := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Nested", Body: "ancestry", NotebookID: child.ID})

	if report, err := SyncOutbox(ctx, st, w, 100); err != nil || report.Written != 1 {
		t.Fatalf("SyncOutbox = %+v, %v", report, err)
	}
	data, err := os.ReadFile(filepath.Join(w.Dir, "notes", doc.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{`notebook: "Reports"`, "notebook_ancestors:\n", `  - "Work"`} {
		if !strings.Contains(content, want) {
			t.Fatalf("nested projection missing %q:\n%s", want, content)
		}
	}
}

func TestFullSyncProjectsExistingNotes(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	w := Writer{Dir: t.TempDir()}

	var ids []string
	for i := 0; i < 3; i++ {
		doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "note", Body: "body"})
		if err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		ids = append(ids, doc.ID)
	}

	report, err := FullSync(ctx, st, w)
	if err != nil || report.Written != 3 || report.Failed != 0 {
		t.Fatalf("FullSync: %+v err=%v", report, err)
	}
	for _, id := range ids {
		if _, err := os.Stat(filepath.Join(w.Dir, "notes", id+".md")); err != nil {
			t.Fatalf("missing projection for %s: %v", id, err)
		}
	}
}

func TestDrainOutboxUsesBoundedBatches(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	w := Writer{Dir: t.TempDir()}
	for index := 0; index < 7; index++ {
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_bounded_%d", index), Title: "bounded", Body: "body",
		}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := DrainOutbox(ctx, st, w, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if first.Jobs != 4 || first.Batches != 2 || first.Backlog != 3 || first.Due != 3 {
		t.Fatalf("first bounded drain: %+v", first)
	}
	second, err := DrainOutbox(ctx, st, w, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if second.Jobs != 3 || second.Batches != 2 || second.Backlog != 0 {
		t.Fatalf("second bounded drain: %+v", second)
	}
}

func TestFailedProjectionBacksOffWithoutBlockingLaterJobs(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	w := Writer{Dir: t.TempDir()}
	if err := st.Exec(ctx, `INSERT INTO index_outbox(object_type, object_id, operation)
		VALUES('document', '../unsafe', 'upsert')`); err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		PreferredID: "doc_after_failure", Title: "later", Body: "still projected",
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := DrainOutbox(ctx, st, w, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed != 1 || report.Written != 1 || report.Backlog != 1 || report.Due != 0 {
		t.Fatalf("failure should be delayed while later job completes: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(w.Dir, "notes", doc.ID+".md")); err != nil {
		t.Fatalf("later projection missing: %v", err)
	}
}

func TestReconcileRepairsMissingStaleOrphanedAndTemporaryFiles(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	w := Writer{Dir: t.TempDir()}
	ids := []string{}
	for index := 0; index < 3; index++ {
		doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_reconcile_%d", index),
			Title:       fmt.Sprintf("Reconcile %d", index),
			Body:        fmt.Sprintf("canonical body %d", index),
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, doc.ID)
	}
	initial, err := Reconcile(ctx, st, w, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !initial.Complete || initial.Canonical != 3 || initial.Missing != 3 || initial.Repaired != 3 {
		t.Fatalf("initial reconcile: %+v", initial)
	}
	notesDir := filepath.Join(w.Dir, "notes")
	if err := os.WriteFile(filepath.Join(notesDir, ids[0]+".md"), []byte("hostile stale projection"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(notesDir, ids[1]+".md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, "doc_orphan.md"), []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, ".notrios-projection-crash.tmp"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(w.Dir, "outside"), filepath.Join(notesDir, "doc_symlink.md")); err != nil {
		t.Fatal(err)
	}
	repaired, err := Reconcile(ctx, st, w, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !repaired.Complete || repaired.Missing != 1 || repaired.Stale != 1 ||
		repaired.Orphaned != 3 || repaired.Repaired != 5 {
		t.Fatalf("repair reconcile: %+v", repaired)
	}
	for _, name := range []string{"doc_orphan.md", ".notrios-projection-crash.tmp", "doc_symlink.md"} {
		if _, err := os.Lstat(filepath.Join(notesDir, name)); !os.IsNotExist(err) {
			t.Fatalf("orphan %s survived: %v", name, err)
		}
	}
	final, err := Reconcile(ctx, st, w, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !final.Complete || final.Missing != 0 || final.Stale != 0 || final.Orphaned != 0 || final.Repaired != 0 {
		t.Fatalf("reconciliation did not converge: %+v", final)
	}
}

func TestReconcileTraversesEveryCanonicalCollection(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.Exec(ctx, `INSERT INTO collections(id, name) VALUES('other', 'Other')`); err != nil {
		t.Fatal(err)
	}
	for _, collectionID := range []string{"default", "other"} {
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: "doc_" + collectionID, CollectionID: collectionID,
			Title: "collection " + collectionID, Body: "body",
		}); err != nil {
			t.Fatal(err)
		}
	}
	report, err := Reconcile(ctx, st, Writer{Dir: t.TempDir()}, 1)
	if err != nil || !report.Complete || report.Canonical != 2 || report.Missing != 2 || report.Repaired != 2 {
		t.Fatalf("multi-collection reconciliation: %+v err=%v", report, err)
	}
}
