package joplinraw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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

	// Trashing an imported note and re-running the import must not
	// resurrect it (or crash on the reserved document ID).
	trashed, _ := st.GetDocument(ctx, "doc_joplin_note2")
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: trashed.ID, BaseRevisionID: trashed.CurrentRevisionID}); err != nil {
		t.Fatalf("trash imported note: %v", err)
	}
	rerun, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("re-import with trashed note: %v", err)
	}
	if rerun.NotesImported != 0 {
		t.Fatalf("trashed note resurrected: %#v", rerun)
	}
	if _, err := st.GetDocument(ctx, "doc_joplin_note2"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("trashed note must stay trashed: %v", err)
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

func TestH8HierarchyTagsAndExactSourceBundle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "folder-root.md"), "id: root\ntitle: Projects\ntype_: 2\n")
	writeFile(t, filepath.Join(dir, "folder-child.md"), "id: child\nparent_id: root\ntitle: Alpha\ntype_: 2\n")
	writeFile(t, filepath.Join(dir, "tag-used.md"), "id: tag-used\ntitle: research\ntype_: 5\n")
	writeFile(t, filepath.Join(dir, "tag-empty.md"), "id: tag-empty\ntitle: someday\ntype_: 5\n")
	writeFile(t, filepath.Join(dir, "note-tag.md"), "id: relation\nnote_id: exact-note\ntag_id: tag-used\ntype_: 6\n")
	raw := []byte("BodyLabel: this is note content, not metadata\r\n\r\nunknown_future_property: keep me\r\ntitle: Exact Note\r\nid: exact-note\r\nparent_id: child\r\ntype_: 1\r\n")
	if err := os.WriteFile(filepath.Join(dir, "exact-note.md"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	st := openTestStore(t)
	report, err := Import(ctx, st, dir, Options{CollectionID: "default", PreserveSource: true, BatchSize: 2})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if report.NotebooksCreated != 2 || report.TagsCreated != 2 || report.TagsApplied != 1 {
		t.Fatalf("hierarchy/tag report: %#v", report)
	}
	root, err := st.GetNotebook(ctx, "nb_joplin_root")
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.GetNotebook(ctx, "nb_joplin_child")
	if err != nil {
		t.Fatal(err)
	}
	if root.ParentID != "" || child.ParentID != root.ID || root.Name != "Projects" || child.Name != "Alpha" {
		t.Fatalf("notebook hierarchy not restored: root=%+v child=%+v", root, child)
	}
	doc, err := st.GetDocument(ctx, "doc_joplin_exact-note")
	if err != nil {
		t.Fatal(err)
	}
	if doc.NotebookID != child.ID {
		t.Fatalf("note notebook = %q, want %q", doc.NotebookID, child.ID)
	}
	tags, err := st.ListDocumentTags(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "research" {
		t.Fatalf("real note tags = %#v", tags)
	}
	allTags, err := st.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(allTags) != 2 {
		t.Fatalf("unassigned source tag was not preserved: %#v", allTags)
	}
	item, reader, err := st.OpenSourceBundleItem(ctx, sourceSystem, report.SourceKey, "default", "1:exact-note:exact-note.md")
	if err != nil {
		t.Fatalf("open source bundle item: %v", err)
	}
	bundled, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read source bundle: read=%v close=%v", readErr, closeErr)
	}
	if !bytes.Equal(bundled, raw) {
		t.Fatalf("source bundle did not preserve exact bytes:\n got %q\nwant %q", bundled, raw)
	}
	wantOrder := []string{"unknown_future_property", "title", "id", "parent_id", "type_"}
	if !reflect.DeepEqual(item.PropertyOrder, wantOrder) {
		t.Fatalf("property order = %#v, want %#v", item.PropertyOrder, wantOrder)
	}
	writeFile(t, filepath.Join(dir, "tag-used.md"), "id: tag-used\ntitle: renamed-research\ntype_: 5\n")
	renamed, err := Import(ctx, st, dir, Options{CollectionID: "default", PreserveSource: true, BatchSize: 2})
	if err != nil {
		t.Fatalf("rename source tag: %v", err)
	}
	if renamed.TagsUpdated != 1 {
		t.Fatalf("tag rename report: %#v", renamed)
	}
	tags, err = st.ListDocumentTags(ctx, doc.ID)
	if err != nil || len(tags) != 1 || tags[0].Name != "renamed-research" {
		t.Fatalf("stable source tag rename failed: tags=%#v err=%v", tags, err)
	}
	if err := os.Remove(filepath.Join(dir, "note-tag.md")); err != nil {
		t.Fatal(err)
	}
	removed, err := Import(ctx, st, dir, Options{CollectionID: "default", BatchSize: 2})
	if err != nil {
		t.Fatalf("remove source tag relation: %v", err)
	}
	tags, err = st.ListDocumentTags(ctx, doc.ID)
	if err != nil || len(tags) != 0 || removed.TagsRemoved != 1 {
		t.Fatalf("source tag relation removal failed: report=%#v tags=%#v err=%v", removed, tags, err)
	}
	allTags, err = st.ListTags(ctx)
	if err != nil || len(allTags) != 2 {
		t.Fatalf("unassigned source tags must remain: tags=%#v err=%v", allTags, err)
	}
}

func TestH8DryRunMatchesRealImportActions(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "folder.md"), "id: work\ntitle: Work\ntype_: 2\n")
	writeFile(t, filepath.Join(dir, "tag.md"), "id: tag1\ntitle: topic\ntype_: 5\n")
	writeFile(t, filepath.Join(dir, "relation.md"), "id: rel1\nnote_id: note1\ntag_id: tag1\ntype_: 6\n")
	writeFile(t, filepath.Join(dir, "note.md"), "Body\n\nid: note1\nparent_id: work\ntitle: Planned\ntype_: 1\n")
	writeFile(t, filepath.Join(dir, "resource.md"), "id: res1\ntitle: file.bin\nfilename: file.bin\ntype_: 4\n")
	writeFile(t, filepath.Join(dir, "resources", "res1"), "payload")

	st := openTestStore(t)
	config, dry, err := DryRun(ctx, st, dir, Options{CollectionID: "default", PreserveSource: true, BatchSize: 2})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if _, err := st.GetImportCheckpoint(ctx, sourceSystem, filepath.Clean(dir), "default"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry run wrote a checkpoint: %v", err)
	}
	if _, err := st.GetNotebook(ctx, "nb_joplin_work"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry run wrote a notebook: %v", err)
	}
	if tags, err := st.ListTags(ctx); err != nil || len(tags) != 0 {
		t.Fatalf("dry run wrote tags: tags=%#v err=%v", tags, err)
	}
	if _, err := st.GetSourceBundleItem(ctx, sourceSystem, filepath.Clean(dir), "default", "1:note1:note.md"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry run wrote a source bundle: %v", err)
	}
	actual, err := Import(ctx, st, dir, Options{CollectionID: "default", PreserveSource: true, BatchSize: 2, Config: &config})
	if err != nil {
		t.Fatalf("actual import: %v", err)
	}
	dryActions := []int{
		dry.NotesImported, dry.NotesUpdated, dry.NotesUnchanged,
		dry.NotebooksCreated, dry.NotebooksMerged,
		dry.TagsCreated, dry.TagsExisting, dry.TagsApplied,
		dry.ResourcesImported, dry.ResourcesUpdated, dry.ResourcesExisting, dry.ResourcesSkipped,
		dry.SourceBundleItems, int(dry.SourceBundleBytes),
	}
	actualActions := []int{
		actual.NotesImported, actual.NotesUpdated, actual.NotesUnchanged,
		actual.NotebooksCreated, actual.NotebooksMerged,
		actual.TagsCreated, actual.TagsExisting, actual.TagsApplied,
		actual.ResourcesImported, actual.ResourcesUpdated, actual.ResourcesExisting, actual.ResourcesSkipped,
		actual.SourceBundleItems, int(actual.SourceBundleBytes),
	}
	if !reflect.DeepEqual(dryActions, actualActions) {
		t.Fatalf("dry-run actions differ:\n dry=%v\nreal=%v", dryActions, actualActions)
	}
	if dry.CheckpointStatus != "dry-run" || actual.CheckpointStatus != "completed" {
		t.Fatalf("checkpoint statuses: dry=%q actual=%q", dry.CheckpointStatus, actual.CheckpointStatus)
	}
}

func TestH8InterruptedImportResumesAtDurableBatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	for index := 0; index < 7; index++ {
		id := fmt.Sprintf("note-%02d", index)
		writeFile(t, filepath.Join(dir, id+".md"), fmt.Sprintf("Body %d\n\nid: %s\ntitle: Note %d\ntype_: 1\n", index, id, index))
	}
	st := openTestStore(t)
	interrupted := errors.New("simulated interruption")
	_, err := Import(ctx, st, dir, Options{
		CollectionID: "default",
		BatchSize:    2,
		AfterBatch: func(phase string, processed, total int) error {
			if phase == "notes" && processed == 2 {
				return interrupted
			}
			return nil
		},
	})
	if !errors.Is(err, interrupted) {
		t.Fatalf("first import error = %v, want interruption", err)
	}
	checkpoint, err := st.GetImportCheckpoint(ctx, sourceSystem, filepath.Clean(dir), "default")
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if checkpoint.Phase != "notes" || checkpoint.NextIndex != 2 || checkpoint.Status != "running" {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
	report, err := Import(ctx, st, dir, Options{CollectionID: "default", BatchSize: 2})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !report.Resumed || report.NotesImported != 7 || report.CheckpointStatus != "completed" {
		t.Fatalf("resume report: %#v", report)
	}
	for index := 0; index < 7; index++ {
		id := fmt.Sprintf("doc_joplin_note-%02d", index)
		if _, err := st.GetDocument(ctx, id); err != nil {
			t.Fatalf("missing resumed document %s: %v", id, err)
		}
	}
}

func TestH8ResumeFromFinalRunningCheckpointDoesNotReplay(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "note.md"), "Body\n\nid: note1\ntitle: Final checkpoint\ntype_: 1\n")
	st := openTestStore(t)
	interrupted := errors.New("crash after final batch")
	_, err := Import(ctx, st, dir, Options{AfterBatch: func(phase string, processed, total int) error {
		if phase == "notes" {
			return interrupted
		}
		return nil
	}})
	if !errors.Is(err, interrupted) {
		t.Fatalf("first import error = %v", err)
	}
	checkpoint, err := st.GetImportCheckpoint(ctx, sourceSystem, filepath.Clean(dir), "default")
	if err != nil || checkpoint.Phase != "done" || checkpoint.Status != "running" {
		t.Fatalf("final running checkpoint=%+v err=%v", checkpoint, err)
	}
	report, err := Import(ctx, st, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Resumed || report.NotesImported != 1 || report.NotesUpdated != 0 || report.NotesUnchanged != 0 {
		t.Fatalf("final checkpoint replayed work: %#v", report)
	}
}

func TestH8ResourceFingerprintUpdatesStableResource(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "resource.md"), "id: res1\ntitle: file.bin\nfilename: file.bin\ntype_: 4\n")
	writeFile(t, filepath.Join(dir, "note.md"), "![file](:/res1)\n\nid: note1\ntitle: Resource note\ntype_: 1\n")
	contentPath := filepath.Join(dir, "resources", "res1")
	writeFile(t, contentPath, "version one")
	st := openTestStore(t)
	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); err != nil {
		t.Fatal(err)
	}
	before, err := st.GetResource(ctx, "res_joplin_res1")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, contentPath, "version two")
	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	after, err := st.GetResource(ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.ResourcesUpdated != 1 || before.ID != after.ID || before.SHA256 == after.SHA256 {
		t.Fatalf("resource was not updated stably: report=%#v before=%+v after=%+v", report, before, after)
	}
	refs, err := st.ListDocumentResources(ctx, "doc_joplin_note1")
	if err != nil || len(refs) != 1 || refs[0].ResourceID != before.ID {
		t.Fatalf("resource reference not preserved: refs=%#v err=%v", refs, err)
	}
}

func TestH8NotebookConflictRequiresRenameConfig(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "folder.md"), "id: work\ntitle: Work\ntype_: 2\n")
	writeFile(t, filepath.Join(dir, "note.md"), "Body\n\nid: note1\nparent_id: work\ntitle: Imported\ntype_: 1\n")
	st := openTestStore(t)
	notebook, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	sourceDoc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{NotebookID: notebook.ID, Title: "Other source", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{DocumentID: sourceDoc.ID, SourceSystem: "obsidian", ExternalID: "other"}); err != nil {
		t.Fatal(err)
	}
	config, dry, err := DryRun(ctx, st, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(dry.NotebookConflicts) != 1 || config.Renames["Work"] == "" {
		t.Fatalf("dry run did not suggest rename: report=%#v config=%#v", dry, config)
	}
	if _, err := Import(ctx, st, dir, Options{}); !errors.Is(err, store.ErrNameConflict) {
		t.Fatalf("import without config error = %v", err)
	}
	report, err := Import(ctx, st, dir, Options{Config: &config})
	if err != nil {
		t.Fatalf("configured import: %v", err)
	}
	if report.NotebooksCreated != 1 {
		t.Fatalf("configured report: %#v", report)
	}
	imported, err := st.GetDocument(ctx, "doc_joplin_note1")
	if err != nil {
		t.Fatal(err)
	}
	target, err := st.GetNotebook(ctx, imported.NotebookID)
	if err != nil {
		t.Fatal(err)
	}
	if target.Name != config.Renames["Work"] {
		t.Fatalf("renamed target = %q, want %q", target.Name, config.Renames["Work"])
	}
}

func TestH8PlainNotebookMergeRemainsIdempotentAfterSourceBinding(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "folder.md"), "id: work\ntitle: Work\ntype_: 2\n")
	writeFile(t, filepath.Join(dir, "note.md"), "Body\n\nid: note1\nparent_id: work\ntitle: Imported\ntype_: 1\n")
	st := openTestStore(t)
	plain, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := Import(ctx, st, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if first.NotebooksMerged != 1 {
		t.Fatalf("first import did not merge plain notebook: %#v", first)
	}
	doc, err := st.GetDocument(ctx, "doc_joplin_note1")
	if err != nil || doc.NotebookID != plain.ID {
		t.Fatalf("merged placement: doc=%+v err=%v", doc, err)
	}
	second, err := Import(ctx, st, dir, Options{})
	if err != nil {
		t.Fatalf("rerun after merge became a conflict: %v", err)
	}
	if second.NotebooksSkipped != 1 || second.NotesUnchanged != 1 {
		t.Fatalf("merge rerun not idempotent: %#v", second)
	}
}

func TestH8GeneratedHundredsUseBoundedBatches(t *testing.T) {
	count := 240
	if value := os.Getenv("NOTRIOS_JOPLIN_SCALE_ITEMS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100000 {
			t.Fatalf("invalid NOTRIOS_JOPLIN_SCALE_ITEMS=%q", value)
		}
		count = parsed
	}
	ctx := context.Background()
	dir := t.TempDir()
	for index := 0; index < count; index++ {
		id := fmt.Sprintf("generated-%06d", index)
		writeFile(t, filepath.Join(dir, id+".md"), fmt.Sprintf("Body %d\n\nid: %s\ntitle: Generated %d\nunknown_%d: retained\ntype_: 1\n", index, id, index, index))
	}
	st := openTestStore(t)
	report, err := Import(ctx, st, dir, Options{BatchSize: 37})
	if err != nil {
		t.Fatal(err)
	}
	if report.NotesImported != count {
		t.Fatalf("imported %d, want %d: %#v", report.NotesImported, count, report)
	}
	if report.BatchesCompleted < (count+36)/37 {
		t.Fatalf("batch count %d did not reflect bounded batches", report.BatchesCompleted)
	}
	rerun, err := Import(ctx, st, dir, Options{BatchSize: 37})
	if err != nil {
		t.Fatal(err)
	}
	if rerun.NotesUnchanged != count {
		t.Fatalf("rerun unchanged=%d, want %d", rerun.NotesUnchanged, count)
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
