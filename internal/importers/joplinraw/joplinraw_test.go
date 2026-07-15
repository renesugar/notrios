package joplinraw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func TestParseItemSupportsBodyFirstAndMetadataFirst(t *testing.T) {
	bodyFirst := "Hello Joplin\n\nid: note1\ntitle: Body First\ntype_: 1\n"
	item, ok := parseItem("note1.md", bodyFirst)
	if !ok {
		t.Fatal("body-first item did not parse")
	}
	if item.Body != "Hello Joplin" || item.Fields["title"] != "Body First" {
		t.Fatalf("unexpected body-first parse: %#v body=%q", item.Fields, item.Body)
	}

	metadataFirst := "id: note2\ntitle: Metadata First\ntype_: 1\n\nBody text\n"
	item, ok = parseItem("note2.md", metadataFirst)
	if !ok {
		t.Fatal("metadata-first item did not parse")
	}
	if item.Body != "Body text" || item.Fields["title"] != "Metadata First" {
		t.Fatalf("unexpected metadata-first parse: %#v body=%q", item.Fields, item.Body)
	}
}

func TestImportJoplinRawFixture(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "folder-root.md"), "id: folder1\ntitle: Imported Notebook\ntype_: 2\n")
	writeFile(t, filepath.Join(dir, "tag.md"), "id: tag1\ntitle: imported\ntype_: 5\n")
	writeFile(t, filepath.Join(dir, "note-tag.md"), "id: nt1\nnote_id: note1\ntag_id: tag1\ntype_: 6\n")
	writeFile(t, filepath.Join(dir, "note1.md"), "# Hello\n\nSee [Target](:/note2).\n\n![Image](:/res1)\n\nid: note1\nparent_id: folder1\ntitle: Source Note\ntype_: 1\n")
	writeFile(t, filepath.Join(dir, "note2.md"), "id: note2\nparent_id: folder1\ntitle: Target Note\ntype_: 1\n\n# Target\n")
	writeFile(t, filepath.Join(dir, "res1.md"), "id: res1\ntitle: diagram.png\nfilename: diagram.png\nmime: image/png\nfile_extension: png\ntype_: 4\n")
	if err := os.MkdirAll(filepath.Join(dir, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "resources", "res1.png"), "PNGDATA")

	st := openTestStore(t)
	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if report.NotesImported != 2 || report.ResourcesImported != 1 || report.LinksRewritten != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	doc, err := st.GetDocument(ctx, "doc_joplin_note1")
	if err != nil {
		t.Fatalf("get imported doc: %v", err)
	}
	if !strings.Contains(doc.Body, "document://default/documents/doc_joplin_note2") {
		t.Fatalf("note link was not rewritten: %s", doc.Body)
	}
	if !strings.Contains(doc.Body, "resource://default/resources/res_joplin_res1") {
		t.Fatalf("resource link was not rewritten: %s", doc.Body)
	}
	if !strings.Contains(doc.Body, "joplin_notebook: \"Imported Notebook\"") || !strings.Contains(doc.Body, "joplin_tags:") {
		t.Fatalf("frontmatter missing notebook/tags: %s", doc.Body)
	}
	refs, err := st.ListDocumentResources(ctx, doc.ID)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(refs) != 1 || refs[0].ResourceID != "res_joplin_res1" {
		t.Fatalf("unexpected refs: %#v", refs)
	}
	src, err := st.GetDocumentSource(ctx, doc.ID)
	if err != nil || src.SourceSystem != "joplin" || src.ExternalID != "note1" {
		t.Fatalf("provenance row missing: %+v err=%v", src, err)
	}
	if found, err := st.FindDocumentBySource(ctx, "joplin", "note2"); err != nil || found != "doc_joplin_note2" {
		t.Fatalf("FindDocumentBySource: %q err=%v", found, err)
	}
	search, err := st.Search(ctx, store.SearchRequest{Query: "Hello", Limit: 5})
	if err != nil {
		t.Fatalf("search imported docs: %v", err)
	}
	if len(search.Hits) == 0 {
		t.Fatal("imported note was not searchable")
	}

	report2, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}
	if report2.NotesUnchanged != 2 || report2.ResourcesExisting != 1 {
		t.Fatalf("second import was not idempotent enough: %#v", report2)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	assetDir := filepath.Join(t.TempDir(), "assets")
	st, err := store.OpenSQLiteWithAssetStore(":memory:", assetDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}
