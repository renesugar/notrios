package projection

import (
	"context"
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
