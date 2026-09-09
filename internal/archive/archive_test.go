package archive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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

func writeMinimalArchive(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"format":"notrios-archive","version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes", "note.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestImportRejectsSymlinkedNoteBeforePersistence(t *testing.T) {
	dir := writeMinimalArchive(t)
	target := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(target, []byte("untrusted"), 0o644); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(dir, "notes", "note.md")
	if err := os.Remove(note); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, note); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	st := newTestStore(t)
	if _, err := Import(context.Background(), st, dir, ImportOptions{}); err == nil {
		t.Fatal("symlinked note was accepted")
	}
	if page, err := st.ListNotebookDocuments(context.Background(), store.DefaultNotebookID, store.DocumentPageRequest{Limit: 10}); err != nil || len(page.Documents) != 0 {
		t.Fatalf("symlink rejection occurred after persistence: documents=%d err=%v", len(page.Documents), err)
	}
}

func TestImportRejectsSymlinkedResourceBeforePersistence(t *testing.T) {
	dir := writeMinimalArchive(t)
	if err := os.MkdirAll(filepath.Join(dir, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(target, []byte("untrusted"), 0o644); err != nil {
		t.Fatal(err)
	}
	resource := filepath.Join(dir, "resources", "res_1__payload.bin")
	if err := os.Symlink(target, resource); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	st := newTestStore(t)
	if _, err := Import(context.Background(), st, dir, ImportOptions{}); err == nil {
		t.Fatal("symlinked resource was accepted")
	}
	if page, err := st.ListNotebookDocuments(context.Background(), store.DefaultNotebookID, store.DocumentPageRequest{Limit: 10}); err != nil || len(page.Documents) != 0 {
		t.Fatalf("symlink rejection occurred after persistence: documents=%d err=%v", len(page.Documents), err)
	}
}

func TestImportRejectsSymlinkedArchiveDirectories(t *testing.T) {
	for _, kind := range []string{"notes", "resources"} {
		t.Run(kind, func(t *testing.T) {
			dir := writeMinimalArchive(t)
			outside := t.TempDir()
			if kind == "notes" {
				if err := os.WriteFile(filepath.Join(outside, "note.md"), []byte("outside"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(outside, "res_1__payload.bin"), []byte("outside"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Join(dir, kind)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(dir, kind)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if _, _, _, err := loadArchive(dir); err == nil {
				t.Fatalf("symlinked %s directory was accepted", kind)
			}
		})
	}
}

func TestImportRejectsSpecialAndOversizedArchiveFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO test is not portable to Windows")
	}
	dir := writeMinimalArchive(t)
	fifo := filepath.Join(dir, "notes", "special.md")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	if _, _, _, err := loadArchive(dir); err == nil {
		t.Fatal("FIFO was accepted")
	}
	if err := os.Remove(fifo); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(dir, "notes", "large.md")
	if err := os.WriteFile(large, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(large, MaxArchiveFileBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := loadArchive(dir); err == nil {
		t.Fatal("oversized archive file was accepted")
	}
}

func TestLegacyImportIgnoresSidecarsAndAcceptsResourceOverMetadataLimit(t *testing.T) {
	dir := writeMinimalArchive(t)
	if err := os.WriteFile(filepath.Join(dir, "notes", "editor.sidecar"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	resources := filepath.Join(dir, "resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "README"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(resources, "res_large__payload.bin")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxArchiveFileBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	st := newTestStore(t)
	report, err := Import(context.Background(), st, dir, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.ResourcesCreated != 1 {
		t.Fatalf("resource over metadata limit was not imported: %+v", report)
	}
}

func TestArchivePreflightEnforcesCountAndAggregateLimits(t *testing.T) {
	old := legacyArchiveLimits
	t.Cleanup(func() { legacyArchiveLimits = old })
	dir := writeMinimalArchive(t)
	if err := os.WriteFile(filepath.Join(dir, "notes", "second.md"), []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyArchiveLimits.MaxNotes = 1
	if _, _, _, err := loadArchive(dir); err == nil {
		t.Fatal("note count limit was not enforced")
	}

	dir = writeMinimalArchive(t)
	resources := filepath.Join(dir, "resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "res_1__payload.bin"), []byte("12"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyArchiveLimits.MaxNotes = 1_000_000
	legacyArchiveLimits.MaxResourceBytes = 1
	if _, _, _, err := loadArchive(dir); err == nil {
		t.Fatal("resource aggregate limit was not enforced")
	}

	dir = writeMinimalArchive(t)
	if err := os.WriteFile(filepath.Join(dir, "notes", "ignored.sidecar"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyArchiveLimits.MaxResourceBytes = 1 << 40
	legacyArchiveLimits.MaxScannedEntries = 1
	if _, _, _, err := loadArchive(dir); err == nil {
		t.Fatal("total scanned-entry limit was not enforced for ignored entries")
	}
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

// A note that links to another note in the same archive is imported before its
// target as often as not, so the link is recorded unresolved at that moment.
// Import must resolve them afterwards: otherwise every internal link in an
// imported library stays broken until each note is edited, and a later
// publication would rewrite them all out of the published bodies.
func TestImportResolvesLinksBetweenNotesInTheSameArchive(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", `{"format":"notrios-archive","version":1,"query":"","notes":2}`)
	write("notebooks.json", `[{"path":"Docs"}]`)
	// The linking note sorts first, so its target does not exist yet when it is
	// written.
	write(filepath.Join("notes", "doc_a.md"),
		"---\nid: doc_a\ntitle: A\nnotebook: Docs\n---\n\nSee [B](document://default/documents/doc_b).\n")
	write(filepath.Join("notes", "doc_b.md"),
		"---\nid: doc_b\ntitle: B\nnotebook: Docs\n---\n\nThe target.\n")

	st := newTestStore(t)
	if _, err := Import(ctx, st, dir, ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	links, err := st.ListDocumentLinks(ctx, "doc_a", "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Outgoing) != 1 {
		t.Fatalf("expected one outgoing link, got %+v", links.Outgoing)
	}
	if got := links.Outgoing[0]; got.ResolutionStatus != "resolved" || got.TargetDocumentID != "doc_b" {
		t.Fatalf("import left an internal link unresolved: %+v", got)
	}
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
