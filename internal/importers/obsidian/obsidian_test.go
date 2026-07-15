package obsidian

import (
	"context"
	"os"
	"path/filepath"
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
