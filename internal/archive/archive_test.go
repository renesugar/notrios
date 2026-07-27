package archive

import (
	"context"
	"errors"
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

// buildSourceStore populates a store with nested notebooks, a tagged note
// with a resource, and a Twitter-sourced note.
func buildSourceStore(t *testing.T) (*store.SQLiteStore, store.Document) {
	t.Helper()
	ctx := context.Background()
	st := newTestStore(t)

	work, _ := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work", IconEmoji: "💼"})
	reports, _ := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Reports", ParentID: work.ID})

	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "July report", Body: "quarterly numbers", NotebookID: reports.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDocumentTag(ctx, doc.ID, "todo"); err != nil {
		t.Fatal(err)
	}
	res, err := st.CreateResource(ctx, store.CreateResourceRequest{Filename: "chart.png", MIMEType: "image/png", Content: strings.NewReader("PNGDATA")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{DocumentID: doc.ID, ResourceID: res.ID}); err != nil {
		t.Fatal(err)
	}

	// A Twitter-sourced note in a "Twitter" notebook (source-bound).
	tw, _ := st.CreateNotebook(ctx, store.CreateNotebookRequest{PreferredID: "nb_twitter", Name: "Twitter"})
	twDoc, _ := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "tweet", Body: "tweet body", NotebookID: tw.ID})
	_, _ = st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{DocumentID: twDoc.ID, SourceSystem: "twitter", ExternalID: "t1"})
	return st, doc
}

func TestExportImportRoundTripWithRenames(t *testing.T) {
	ctx := context.Background()
	src, doc := buildSourceStore(t)
	out := filepath.Join(t.TempDir(), "archive")

	// Export everything (empty query = All notes).
	report, err := Export(ctx, src, out, ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if report.Notes != 2 || report.Resources != 1 || report.Notebooks < 3 {
		t.Fatalf("export report: %+v", report)
	}
	raw, err := os.ReadFile(filepath.Join(out, "notes", doc.ID+".md"))
	if err != nil {
		t.Fatalf("exported note missing: %v", err)
	}
	if !strings.Contains(string(raw), `notebook: "Work/Reports"`) {
		t.Fatalf("notebook path missing from export:\n%s", raw)
	}

	// Import into a fresh store that has its own source-bound "Twitter"
	// notebook: the dry run must flag the conflict and prefill a rename.
	dst := newTestStore(t)
	twNB, _ := dst.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Twitter"})
	twDoc, _ := dst.CreateDocument(ctx, store.CreateDocumentRequest{Title: "existing tweet", Body: "x", NotebookID: twNB.ID})
	_, _ = dst.SetDocumentSource(ctx, store.SetDocumentSourceRequest{DocumentID: twDoc.ID, SourceSystem: "twitter", ExternalID: "t9"})

	cfg, dryReport, err := DryRun(ctx, dst, out)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(dryReport.Conflicts) != 1 || dryReport.Conflicts[0] != "Twitter" {
		t.Fatalf("conflict not detected: %+v", dryReport)
	}
	if cfg.Renames["Twitter"] == "" {
		t.Fatalf("rename suggestion missing: %+v", cfg)
	}
	if len(dryReport.Creates) == 0 {
		t.Fatalf("creates missing: %+v", dryReport)
	}
	// Dry run wrote nothing.
	if page, _ := dst.ListNotebookDocuments(ctx, store.DefaultNotebookID, store.DocumentPageRequest{Limit: 10}); len(page.Documents) != 0 {
		t.Fatalf("dry run must not import notes")
	}

	// Importing without the rename is refused before any write.
	if _, err := Import(ctx, dst, out, ImportOptions{}); !errors.Is(err, store.ErrNameConflict) {
		t.Fatalf("conflicting import must be refused: %v", err)
	}

	// Import with the generated config applies the rename.
	importReport, err := Import(ctx, dst, out, ImportOptions{Config: &cfg})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if importReport.NotesImported != 2 || importReport.ResourcesCreated != 1 {
		t.Fatalf("import report: %+v", importReport)
	}

	// Notebook structure preserved; renamed Twitter notebook exists.
	imported, err := dst.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("imported note: %v", err)
	}
	nb, _ := dst.GetNotebook(ctx, imported.NotebookID)
	if nb.Name != "Reports" {
		t.Fatalf("nested notebook lost: %+v", nb)
	}
	parent, _ := dst.GetNotebook(ctx, nb.ParentID)
	if parent.Name != "Work" || parent.IconEmoji != "💼" {
		t.Fatalf("parent notebook lost: %+v", parent)
	}
	renamed := cfg.Renames["Twitter"]
	notebooks, _ := dst.ListNotebooks(ctx)
	foundRenamed := false
	for _, n := range notebooks {
		if n.Name == renamed {
			foundRenamed = true
		}
	}
	if !foundRenamed {
		t.Fatalf("renamed notebook %q missing", renamed)
	}

	// Tags and resources came along.
	tags, _ := dst.ListDocumentTags(ctx, doc.ID)
	if len(tags) != 1 || tags[0].Name != "todo" {
		t.Fatalf("tags lost: %+v", tags)
	}
	refs, _ := dst.ListDocumentResources(ctx, doc.ID)
	if len(refs) != 1 || refs[0].Resource.Filename != "chart.png" {
		t.Fatalf("resource attachment lost: %+v", refs)
	}

	// Re-imported notes are plain local notes: no provenance, purgeable.
	if _, err := dst.GetDocumentSource(ctx, doc.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("archive import must not create provenance: %v", err)
	}

	// Idempotent re-import.
	again, err := Import(ctx, dst, out, ImportOptions{Config: &cfg})
	if err != nil || again.NotesImported != 0 || again.NotesUnchanged != 2 || again.NotebooksCreated != 0 {
		t.Fatalf("re-import not idempotent: %+v err=%v", again, err)
	}
}

func TestQueryScopedExport(t *testing.T) {
	ctx := context.Background()
	src, doc := buildSourceStore(t)
	out := filepath.Join(t.TempDir(), "scoped")

	report, err := Export(ctx, src, out, ExportOptions{Query: "tag:todo"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if report.Notes != 1 {
		t.Fatalf("query scope not applied: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(out, "notes", doc.ID+".md")); err != nil {
		t.Fatalf("scoped note missing: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(out, "notes"))
	if len(entries) != 1 {
		t.Fatalf("unexpected notes exported: %d", len(entries))
	}
}
