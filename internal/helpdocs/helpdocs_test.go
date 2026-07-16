package helpdocs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func TestSeedHelpNotebook(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	docs := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(docs, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.md", "# Notrios\n\nWelcome to the help.")
	write("api/rest.md", "# REST API\n\nEndpoints live under /api/v1.")

	report, err := Seed(ctx, st, docs)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if report.NotesCreated != 2 {
		t.Fatalf("expected 2 created, got %+v", report)
	}

	doc, err := st.GetDocument(ctx, "doc_help_api_rest")
	if err != nil || doc.Title != "REST API" || doc.NotebookID != store.HelpNotebookID {
		t.Fatalf("help note wrong: %+v err=%v", doc, err)
	}

	// notebook:help finds the docs.
	res, err := st.Search(ctx, store.SearchRequest{Query: `notebook:help endpoints`, Limit: 10})
	if err != nil || len(res.Hits) != 1 || res.Hits[0].ID != "doc_help_api_rest" {
		t.Fatalf("notebook:help search: %+v err=%v", res, err)
	}

	// Re-seeding is idempotent; updates flow through; removed files vanish.
	report, _ = Seed(ctx, st, docs)
	if report.NotesKept != 2 || report.NotesCreated != 0 {
		t.Fatalf("re-seed not idempotent: %+v", report)
	}
	write("index.md", "# Notrios\n\nWelcome to the improved help.")
	if err := os.Remove(filepath.Join(docs, "api", "rest.md")); err != nil {
		t.Fatal(err)
	}
	report, err = Seed(ctx, st, docs)
	if err != nil || report.NotesUpdated != 1 || report.NotesRemoved != 1 {
		t.Fatalf("update/removal seed wrong: %+v err=%v", report, err)
	}
	if _, err := st.GetDocument(ctx, "doc_help_api_rest"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("removed help note should be gone: %v", err)
	}
	updated, _ := st.GetDocument(ctx, "doc_help_index")
	if !strings.Contains(updated.Body, "improved help") {
		t.Fatalf("help note not updated: %s", updated.Body)
	}
}
