package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

const fixture = `[
  {
    "uuid": "aaaa-1111",
    "name": "Recoll design chat",
    "created_at": "2026-07-13T18:00:00.000000Z",
    "chat_messages": [
      {"uuid": "m1", "sender": "human", "created_at": "2026-07-13T18:00:01Z", "text": "How should the projection work?"},
      {"uuid": "m2", "sender": "assistant", "created_at": "2026-07-13T18:00:05Z", "text": "", "content": [{"type": "text", "text": "Drive it from the outbox."}]},
      {"uuid": "m3", "sender": "assistant", "created_at": "2026-07-13T18:00:06Z", "text": ""}
    ]
  },
  {
    "uuid": "bbbb-2222",
    "name": "",
    "created_at": "2026-07-14T09:00:00Z",
    "chat_messages": [
      {"uuid": "m4", "sender": "human", "created_at": "2026-07-14T09:00:01Z", "text": "Untitled chat body"}
    ]
  }
]`

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

func TestImportClaudeFixture(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	st := newTestStore(t)

	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if report.NotesImported != 2 || report.MessagesImported != 3 || report.MessagesSkipped != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}

	doc, err := st.GetDocument(ctx, "doc_claude_aaaa_1111")
	if err != nil {
		t.Fatalf("conversation doc: %v", err)
	}
	if doc.Title != "Recoll design chat" || doc.NotebookID != NotebookID {
		t.Fatalf("title/notebook wrong: %+v", doc)
	}
	// Message order and content-block extraction.
	userIdx := strings.Index(doc.Body, "How should the projection")
	assistantIdx := strings.Index(doc.Body, "Drive it from the outbox")
	if userIdx < 0 || assistantIdx < 0 || userIdx > assistantIdx {
		t.Fatalf("message order/content wrong: %s", doc.Body)
	}
	if !strings.Contains(doc.Body, "## User —") || !strings.Contains(doc.Body, "## Assistant —") {
		t.Fatalf("role headings missing: %s", doc.Body)
	}

	nb, err := st.GetNotebook(ctx, NotebookID)
	if err != nil || nb.Name != "Claude" {
		t.Fatalf("Claude notebook wrong: %+v err=%v", nb, err)
	}

	src, err := st.GetDocumentSource(ctx, doc.ID)
	if err != nil || src.SourceSystem != "claude" || src.ThreadID != "aaaa-1111" || src.PublishedTS == 0 {
		t.Fatalf("provenance wrong: %+v err=%v", src, err)
	}

	// Fallback title, idempotency, purge protection.
	doc2, _ := st.GetDocument(ctx, "doc_claude_bbbb_2222")
	if !strings.HasPrefix(doc2.Title, "Claude conversation 2026-07-14") {
		t.Fatalf("fallback title wrong: %q", doc2.Title)
	}
	report2, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil || report2.NotesUnchanged != 2 {
		t.Fatalf("re-import: %+v err=%v", report2, err)
	}
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: doc2.ID, BaseRevisionID: doc2.CurrentRevisionID}); err != nil {
		t.Fatalf("trash: %v", err)
	}
	if err := st.PurgeDocument(ctx, doc2.ID); !errors.Is(err, store.ErrProtected) {
		t.Fatalf("imported conversations must be purge-protected: %v", err)
	}
	report3, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil || report3.NotesImported != 0 {
		t.Fatalf("trashed conversation resurrected: %+v err=%v", report3, err)
	}
}

func TestClaudeDryRun(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	st := newTestStore(t)
	report, err := Import(ctx, st, dir, Options{DryRun: true})
	if err != nil || !report.DryRun || report.NotesImported != 2 {
		t.Fatalf("dry run: %+v err=%v", report, err)
	}
	if _, err := st.GetNotebook(ctx, NotebookID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry run must not create the notebook: %v", err)
	}
}
