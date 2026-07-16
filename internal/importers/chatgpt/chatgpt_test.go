package chatgpt

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
    "title": "Search engine question",
    "create_time": 1783958400.5,
    "conversation_id": "conv-1",
    "current_node": "n3",
    "mapping": {
      "n0": {"id": "n0", "parent": "", "message": null},
      "n1": {"id": "n1", "parent": "n0", "message": {"id": "m1", "author": {"role": "system"}, "create_time": 1783958400, "content": {"content_type": "text", "parts": [""]}}},
      "n2": {"id": "n2", "parent": "n1", "message": {"id": "m2", "author": {"role": "user"}, "create_time": 1783958401, "content": {"content_type": "text", "parts": ["Which engine should I use for note search?"]}}},
      "n2b": {"id": "n2b", "parent": "n1", "message": {"id": "m2b", "author": {"role": "user"}, "create_time": 1783958401, "content": {"content_type": "text", "parts": ["ABANDONED BRANCH — must not appear"]}}},
      "n3": {"id": "n3", "parent": "n2", "message": {"id": "m3", "author": {"role": "assistant"}, "create_time": 1783958402, "content": {"content_type": "text", "parts": ["Recoll over a projection works well."]}}}
    }
  },
  {
    "title": "",
    "create_time": 1783958500,
    "conversation_id": "conv-2",
    "current_node": "x1",
    "mapping": {
      "x1": {"id": "x1", "parent": "", "message": {"id": "mx", "author": {"role": "user"}, "create_time": 1783958500, "content": {"content_type": "text", "parts": ["Untitled conversation body"]}}}
    }
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

func TestImportChatGPTFixture(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	st := newTestStore(t)

	// Directory form resolves conversations.json inside.
	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if report.NotesImported != 2 || report.MessagesImported != 3 || report.MessagesSkipped != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}

	doc, err := st.GetDocument(ctx, "doc_chatgpt_conv_1")
	if err != nil {
		t.Fatalf("conversation doc: %v", err)
	}
	if doc.Title != "Search engine question" || doc.NotebookID != NotebookID {
		t.Fatalf("title/notebook wrong: %+v", doc)
	}
	// Main-path ordering: user question before assistant answer; the
	// abandoned branch and the empty system message are excluded.
	userIdx := strings.Index(doc.Body, "Which engine")
	assistantIdx := strings.Index(doc.Body, "Recoll over a projection")
	if userIdx < 0 || assistantIdx < 0 || userIdx > assistantIdx {
		t.Fatalf("message order wrong: %s", doc.Body)
	}
	if strings.Contains(doc.Body, "ABANDONED BRANCH") {
		t.Fatalf("non-current branch leaked into note: %s", doc.Body)
	}
	if !strings.Contains(doc.Body, "## User —") || !strings.Contains(doc.Body, "## Assistant —") {
		t.Fatalf("role headings missing: %s", doc.Body)
	}

	// Untitled conversation falls back to a dated title.
	doc2, _ := st.GetDocument(ctx, "doc_chatgpt_conv_2")
	if !strings.HasPrefix(doc2.Title, "ChatGPT conversation 2026-") {
		t.Fatalf("fallback title wrong: %q", doc2.Title)
	}

	// Provenance: thread = conversation, published from create_time.
	src, err := st.GetDocumentSource(ctx, doc.ID)
	if err != nil || src.SourceSystem != "chatgpt" || src.ThreadID != "conv-1" || src.PublishedTS == 0 {
		t.Fatalf("provenance wrong: %+v err=%v", src, err)
	}

	// Idempotent re-run; trashed conversations stay trashed.
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: doc2.ID, BaseRevisionID: doc2.CurrentRevisionID}); err != nil {
		t.Fatalf("trash: %v", err)
	}
	report2, err := Import(ctx, st, filepath.Join(dir, "conversations.json"), Options{CollectionID: "default"})
	if err != nil || report2.NotesUnchanged != 2 || report2.NotesImported != 0 {
		t.Fatalf("re-import: %+v err=%v", report2, err)
	}
	if _, err := st.GetDocument(ctx, doc2.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("trashed conversation must stay trashed: %v", err)
	}
	if err := st.PurgeDocument(ctx, doc2.ID); !errors.Is(err, store.ErrProtected) {
		t.Fatalf("imported conversations must be purge-protected: %v", err)
	}
}

func TestChatGPTDryRun(t *testing.T) {
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
