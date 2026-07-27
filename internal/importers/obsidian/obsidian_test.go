package obsidian

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func TestImportObsidianVaultFixture(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".obsidian", "workspace.json"), `{"ignored":true}`)
	writeFile(t, filepath.Join(dir, "Projects", "Source Note.md"), `---
aliases:
  - Source Alias
tags: [project, imported]
---
# Source Heading

See [[Target Note]] and [[Missing Note]].

Embed target: ![[Target Note#^block-a]]

![Diagram](../assets/diagram.png)
`)
	writeFile(t, filepath.Join(dir, "Projects", "Target Note.md"), `---
title: Target Note
aliases:
  - Target Alias
---
# Target

Referenced block. ^block-a
`)
	writeFile(t, filepath.Join(dir, "assets", "diagram.png"), "PNGDATA")

	st := openTestStore(t)
	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if report.MarkdownSeen != 2 || report.NotesImported != 2 || report.ResourcesImported != 1 || report.AttachmentsCreated != 1 || report.LinkIndexesRefreshed != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}

	sourceID := "doc_obsidian_projects_source_note"
	targetID := "doc_obsidian_projects_target_note"
	doc, err := st.GetDocument(ctx, sourceID)
	if err != nil {
		t.Fatalf("get imported doc: %v", err)
	}
	if doc.Title != "Source Heading" {
		t.Fatalf("unexpected title: %q", doc.Title)
	}
	if !strings.Contains(doc.Body, "source_system: obsidian") || !strings.Contains(doc.Body, "obsidian_path: \"Projects/Source Note.md\"") {
		t.Fatalf("frontmatter missing source metadata: %s", doc.Body)
	}
	if src, err := st.GetDocumentSource(ctx, sourceID); err != nil || src.SourceSystem != "obsidian" || src.ExternalID != "Projects/Source Note.md" {
		t.Fatalf("provenance row missing: %+v err=%v", src, err)
	}
	if !strings.Contains(doc.Body, "aliases:") || !strings.Contains(doc.Body, "tags: [project, imported]") {
		t.Fatalf("original frontmatter was not preserved: %s", doc.Body)
	}

	refs, err := st.ListDocumentResources(ctx, sourceID)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(refs) != 1 || refs[0].ResourceID != "res_obsidian_assets_diagram" {
		t.Fatalf("unexpected resource refs: %#v", refs)
	}

	links, err := st.ListDocumentLinks(ctx, sourceID, "both")
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	var sawResolvedDoc, sawEmbed, sawResource, sawUnresolved bool
	for _, link := range links.Outgoing {
		if link.TargetDocumentID == targetID && link.ResolutionStatus == "resolved" {
			sawResolvedDoc = true
		}
		if link.TargetDocumentID == targetID && link.RelationType == "embed" && link.AnchorType == "block" && link.AnchorValue == "block-a" {
			sawEmbed = true
		}
		if link.TargetResourceID == "res_obsidian_assets_diagram" && link.ResolutionStatus == "resolved" {
			sawResource = true
		}
		if link.RawTarget == "Missing Note" && link.ResolutionStatus == "unresolved" {
			sawUnresolved = true
		}
	}
	if !sawResolvedDoc || !sawEmbed || !sawResource || !sawUnresolved {
		t.Fatalf("missing expected links: %#v", links.Outgoing)
	}

	backlinks, err := st.ListDocumentLinks(ctx, targetID, "incoming")
	if err != nil {
		t.Fatalf("list backlinks: %v", err)
	}
	if len(backlinks.Incoming) < 2 {
		t.Fatalf("expected backlinks from source note, got %#v", backlinks.Incoming)
	}
	search, err := st.Search(ctx, store.SearchRequest{Query: "Source Heading", Limit: 5})
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

	// Trashing an imported note and re-running the import must not
	// resurrect it (or crash on the reserved document ID).
	trashed, _ := st.GetDocument(ctx, targetID)
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
	if _, err := st.GetDocument(ctx, targetID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("trashed note must stay trashed: %v", err)
	}
}

func TestObsidianDryRunDoesNotWrite(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Note.md"), "# Dry Run\n")
	writeFile(t, filepath.Join(dir, "image.png"), "PNGDATA")
	st := openTestStore(t)
	report, err := Import(ctx, st, dir, Options{CollectionID: "default", DryRun: true})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if !report.DryRun || report.MarkdownSeen != 1 || report.ResourcesSeen != 1 || report.NotesImported != 1 {
		t.Fatalf("unexpected dry-run report: %#v", report)
	}
	if _, err := st.GetDocument(ctx, "doc_obsidian_note"); err == nil {
		t.Fatal("dry run wrote a document")
	}
	if _, err := st.GetImportCheckpoint(ctx, sourceSystem, filepath.Clean(dir), "default"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry run wrote a checkpoint: %v", err)
	}
}

func TestH9HierarchyRichLinksAndExactSourceBundle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	sourceBytes := []byte("---\r\nunknown: keep-order\r\naliases: [Source Alias]\r\n---\r\n# Source\r\n\r\nRelative [[../../Reference/Target#Heading]].\r\nAlias embed ![[Target Alias#^block-a]].\r\nAsset ![[../../assets/pic.png]].\r\n")
	writeBytes(t, filepath.Join(dir, "Projects", "Sub", "Source.md"), sourceBytes)
	writeFile(t, filepath.Join(dir, "Reference", "Target.md"), "---\naliases:\n  - Target Alias\ncustom: untouched\n---\n# Target\n\nBlock. ^block-a\n")
	assetBytes := []byte{0x89, 'P', 'N', 'G', 0, 1, 2, 3}
	writeBytes(t, filepath.Join(dir, "assets", "pic.png"), assetBytes)

	st := openTestStore(t)
	report, err := Import(ctx, st, dir, Options{CollectionID: "default", PreserveSource: true, BatchSize: 1})
	if err != nil {
		t.Fatalf("import rich vault: %v", err)
	}
	if report.NotebooksCreated != 4 || report.SourceBundleItems != 3 || report.ResourcesImported != 1 || report.LinksRewritten != 3 {
		t.Fatalf("unexpected rich report: %#v", report)
	}
	doc, err := st.GetDocument(ctx, "doc_obsidian_projects_sub_source")
	if err != nil {
		t.Fatal(err)
	}
	if doc.NotebookID != "nb_obsidian_projects_sub" {
		t.Fatalf("note hierarchy was not restored: %#v", doc)
	}
	source, err := st.GetDocumentSource(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	exactFrontmatter := []byte("unknown: keep-order\r\naliases: [Source Alias]\r\n")
	if !strings.Contains(source.MetadataJSON, sha256Hex(exactFrontmatter)) {
		t.Fatalf("exact frontmatter fingerprint missing from provenance: %s", source.MetadataJSON)
	}
	if !strings.Contains(doc.Body, "document://default/documents/doc_obsidian_reference_target#Heading") ||
		!strings.Contains(doc.Body, "document://default/documents/doc_obsidian_reference_target#^block-a") ||
		!strings.Contains(doc.Body, "resource://default/resources/res_obsidian_assets_pic") {
		t.Fatalf("rich links were not canonicalized: %s", doc.Body)
	}
	links, err := st.ListDocumentLinks(ctx, doc.ID, "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	var heading, block, resource bool
	for _, link := range links.Outgoing {
		heading = heading || link.TargetDocumentID == "doc_obsidian_reference_target" && link.AnchorType == "heading" && link.AnchorValue == "Heading"
		block = block || link.TargetDocumentID == "doc_obsidian_reference_target" && link.RelationType == "embed" && link.AnchorType == "block" && link.AnchorValue == "block-a"
		resource = resource || link.TargetResourceID == "res_obsidian_assets_pic" && link.RelationType == "embed"
	}
	if !heading || !block || !resource {
		t.Fatalf("rich graph edges missing: %#v", links.Outgoing)
	}
	bundle, reader, err := st.OpenSourceBundleItem(ctx, sourceSystem, filepath.Clean(dir), "default", "file:Projects/Sub/Source.md")
	if err != nil {
		t.Fatal(err)
	}
	gotSource, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	if !reflect.DeepEqual(gotSource, sourceBytes) || !reflect.DeepEqual(bundle.PropertyOrder, []string{"unknown", "aliases"}) {
		t.Fatalf("Markdown source was not preserved exactly: item=%#v bytes=%q", bundle, gotSource)
	}
	_, assetReader, err := st.OpenSourceBundleItem(ctx, sourceSystem, filepath.Clean(dir), "default", "file:assets/pic.png")
	if err != nil {
		t.Fatal(err)
	}
	gotAsset, err := io.ReadAll(assetReader)
	if err != nil {
		t.Fatal(err)
	}
	_ = assetReader.Close()
	if !reflect.DeepEqual(gotAsset, assetBytes) {
		t.Fatalf("asset source changed: %v", gotAsset)
	}
}

func TestH9DryRunConflictConfigAndActionParity(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Projects", "Note.md"), "# Imported\n")
	writeFile(t, filepath.Join(dir, "image.png"), "v1")
	st := openTestStore(t)
	existingNotebook, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Projects"})
	if err != nil {
		t.Fatal(err)
	}
	existingDoc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		CollectionID: "default", NotebookID: existingNotebook.ID, Title: "Other", Body: "body", BodyMIMEType: "text/markdown",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{
		DocumentID: existingDoc.ID, SourceSystem: "other", ExternalID: "other-1",
	}); err != nil {
		t.Fatal(err)
	}

	config, dry, err := DryRun(ctx, st, dir, Options{CollectionID: "default", PreserveSource: true, BatchSize: 2})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !reflect.DeepEqual(dry.NotebookConflicts, []string{"Projects"}) || config.Renames["Projects"] != "Projects (Obsidian)" {
		t.Fatalf("conflict plan missing: config=%#v report=%#v", config, dry)
	}
	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); !errors.Is(err, store.ErrNameConflict) {
		t.Fatalf("real import should require conflict config: %v", err)
	}
	config.Conflicts = nil
	_, configuredDry, err := DryRun(ctx, st, dir, Options{
		CollectionID: "default", PreserveSource: true, BatchSize: 2, Config: &config,
	})
	if err != nil {
		t.Fatalf("configured dry run: %v", err)
	}
	actual, err := Import(ctx, st, dir, Options{
		CollectionID: "default", PreserveSource: true, BatchSize: 2, Config: &config,
	})
	if err != nil {
		t.Fatalf("configured import: %v", err)
	}
	if actual.NotesImported != configuredDry.NotesImported || actual.ResourcesImported != configuredDry.ResourcesImported ||
		actual.SourceBundleItems != configuredDry.SourceBundleItems || actual.NotebooksCreated != configuredDry.NotebooksCreated {
		t.Fatalf("dry-run action diff diverged: dry=%#v actual=%#v", configuredDry, actual)
	}
	notebooks, err := st.ListNotebooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, notebook := range notebooks {
		found = found || notebook.ID == "nb_obsidian_projects" && notebook.Name == "Projects (Obsidian)"
	}
	if !found {
		t.Fatalf("configured renamed notebook missing: %#v", notebooks)
	}
}

func TestH9InterruptedImportResumesAndUpdatesAsset(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	for index := 0; index < 5; index++ {
		writeFile(t, filepath.Join(dir, "Notes", fmt.Sprintf("Note-%d.md", index)), fmt.Sprintf("# Note %d\n\n![[../asset.bin]]\n", index))
	}
	writeFile(t, filepath.Join(dir, "asset.bin"), "version-one")
	st := openTestStore(t)
	interrupted := errors.New("interrupt after first note batch")
	_, err := Import(ctx, st, dir, Options{
		CollectionID: "default", BatchSize: 2,
		AfterBatch: func(phase string, processed, total int) error {
			if phase == "notes" && processed == 2 {
				return interrupted
			}
			return nil
		},
	})
	if !errors.Is(err, interrupted) {
		t.Fatalf("expected interruption, got %v", err)
	}
	checkpoint, err := st.GetImportCheckpoint(ctx, sourceSystem, filepath.Clean(dir), "default")
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Phase != "notes" || checkpoint.NextIndex != 2 || checkpoint.Status != "running" {
		t.Fatalf("unexpected checkpoint: %#v", checkpoint)
	}
	resumed, err := Import(ctx, st, dir, Options{CollectionID: "default", BatchSize: 2})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !resumed.Resumed || resumed.NotesImported != 5 || resumed.CheckpointStatus != "completed" {
		t.Fatalf("resume report lost prior progress: %#v", resumed)
	}
	writeFile(t, filepath.Join(dir, "asset.bin"), "version-two")
	updated, err := Import(ctx, st, dir, Options{CollectionID: "default", BatchSize: 2})
	if err != nil {
		t.Fatalf("asset update import: %v", err)
	}
	if updated.ResourcesUpdated != 1 || updated.NotesUnchanged != 5 {
		t.Fatalf("changed asset was not refreshed stably: %#v", updated)
	}
	resource, err := st.GetResource(ctx, "res_obsidian_asset")
	if err != nil {
		t.Fatal(err)
	}
	if resource.SHA256 != sha256Hex([]byte("version-two")) {
		t.Fatalf("resource bytes did not update: %#v", resource)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	writeBytes(t, path, []byte(body))
}

func writeBytes(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
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
