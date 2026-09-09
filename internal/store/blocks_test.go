package store

import (
	"context"
	"errors"
	"testing"
)

func blockTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

const blockBody = "# Title\n\nFirst paragraph.\n\n- an item\n\nTagged paragraph. ^marked\n"

func TestSaveRebuildsBlocksInTheSameTransaction(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Blocks", Body: blockBody})
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := st.ListDocumentBlocks(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 4 {
		t.Fatalf("expected four blocks, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Kind != "heading" || blocks[0].HeadingLevel != 1 {
		t.Fatalf("first block: %+v", blocks[0])
	}
	if blocks[3].Marker != "marked" {
		t.Fatalf("authored marker was not stored: %+v", blocks[3])
	}
	for i, block := range blocks {
		if block.Ordinal != i || block.DocumentID != doc.ID || block.ContentSHA256 == "" {
			t.Fatalf("block %d: %+v", i, block)
		}
	}

	// An update re-derives them; a block whose text did not change keeps its ID.
	before := blocks[1].ID
	updated, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: doc.ID, Title: "Blocks", Body: "# Title\n\nFirst paragraph.\n\n- an item\n\nTagged paragraph. ^marked\n\nAppended paragraph.\n",
		BaseRevisionID: doc.CurrentRevisionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := st.ListDocumentBlocks(ctx, updated.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 5 {
		t.Fatalf("expected five blocks after the append, got %d", len(after))
	}
	if after[1].ID != before {
		t.Fatal("an untouched block must keep its identity across a save")
	}
}

// Blocks are derived state and must not outlive their document.
func TestBlocksAreRemovedWithTheDocument(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Doomed", Body: blockBody})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	if err := st.PurgeDocument(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	count, err := st.countLocked(`SELECT COUNT(*) FROM document_blocks`)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("purging a note left %d block rows behind", count)
	}
}

// The author's marker outranks the derived ID: it is a name the author wrote,
// and it survives an edit to the block's text, which a content-derived ID
// deliberately does not.
func TestFindDocumentBlockPrefersTheAuthoredMarker(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Blocks", Body: blockBody})
	if err != nil {
		t.Fatal(err)
	}
	byMarker, err := st.FindDocumentBlock(ctx, doc.ID, "marked")
	if err != nil {
		t.Fatal(err)
	}
	if byMarker.Marker != "marked" {
		t.Fatalf("marker lookup: %+v", byMarker)
	}
	byID, err := st.FindDocumentBlock(ctx, doc.ID, byMarker.ID)
	if err != nil || byID.ID != byMarker.ID {
		t.Fatalf("derived-ID lookup: %+v %v", byID, err)
	}
	if _, err := st.FindDocumentBlock(ctx, doc.ID, "no-such-anchor"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Editing the marked block's text mints a new derived ID, but the author's
	// marker still finds it.
	updated, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: doc.ID, Title: "Blocks", Body: "# Title\n\nFirst paragraph.\n\n- an item\n\nTagged paragraph, revised. ^marked\n",
		BaseRevisionID: doc.CurrentRevisionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	afterEdit, err := st.FindDocumentBlock(ctx, updated.ID, "marked")
	if err != nil {
		t.Fatal(err)
	}
	if afterEdit.ID == byMarker.ID {
		t.Fatal("editing the text must mint a new content-derived ID")
	}
	if _, err := st.FindDocumentBlock(ctx, updated.ID, byMarker.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the old content ID must stop resolving: %v", err)
	}
}

func TestListDocumentBlocksCountsBacklinksByEitherSpelling(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	target, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_target", Title: "Target", Body: blockBody})
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := st.ListDocumentBlocks(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	derived := blocks[1].ID

	body := "[by marker](" + target.URI + "#^marked)\n\n[by id](" + target.URI + "#^" + derived + ")\n"
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Source", Body: body}); err != nil {
		t.Fatal(err)
	}
	counted, err := st.ListDocumentBlocks(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]DocumentBlock{}
	for _, block := range counted {
		byID[block.ID] = block
	}
	if got := byID[derived].Backlinks; got != 1 {
		t.Fatalf("derived-ID anchor backlink count: %d", got)
	}
	marked := counted[3]
	if marked.Marker != "marked" || marked.Backlinks != 1 {
		t.Fatalf("authored-marker backlink count: %+v", marked)
	}
}

// A database upgraded to schema v14 has no block rows for notes nobody has
// edited since. RebuildDocumentBlocks is how they are filled in, and it must
// not write a revision to do it.
func TestRebuildDocumentBlocksAddsNoRevision(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Blocks", Body: blockBody})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(ctx, `DELETE FROM document_blocks`); err != nil {
		t.Fatal(err)
	}
	if err := st.RebuildDocumentBlocks(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	blocks, err := st.ListDocumentBlocks(ctx, doc.ID)
	if err != nil || len(blocks) != 4 {
		t.Fatalf("rebuild produced %d blocks: %v", len(blocks), err)
	}
	revisions, err := st.ListDocumentRevisions(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 {
		t.Fatalf("rebuilding blocks wrote %d revisions", len(revisions))
	}
}

func TestListDocumentBlocksRejectsUnknownDocuments(t *testing.T) {
	st := blockTestStore(t)
	if _, err := st.ListDocumentBlocks(context.Background(), "doc_missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

const headingBody = "# Getting Started\n\nIntro paragraph.\n\n## Install & Setup\n\nSteps.\n\n## Getting Started\n\nRepeat heading.\n"

func TestHeadingBlocksStoreSlugs(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Headings", Body: headingBody})
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := st.ListDocumentBlocks(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string]string{}
	for _, block := range blocks {
		if block.Kind == "heading" {
			slugs[block.HeadingSlug] = block.ID
		} else if block.HeadingSlug != "" {
			t.Fatalf("only headings carry slugs: %+v", block)
		}
	}
	for _, want := range []string{"getting-started", "install-setup", "getting-started-1"} {
		if slugs[want] == "" {
			t.Fatalf("missing slug %q: %+v", want, slugs)
		}
	}
}

// A heading anchor may arrive slugged (what a stable link carries) or as the
// heading's text (what Obsidian writes and the importer preserves). Both must
// reach the same heading.
func TestFindDocumentBlockResolvesHeadingAnchorsEitherSpelling(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Headings", Body: headingBody})
	if err != nil {
		t.Fatal(err)
	}
	bySlug, err := st.FindDocumentBlock(ctx, doc.ID, "install-setup")
	if err != nil {
		t.Fatal(err)
	}
	byText, err := st.FindDocumentBlock(ctx, doc.ID, "Install & Setup")
	if err != nil {
		t.Fatal(err)
	}
	if bySlug.ID != byText.ID || bySlug.Kind != "heading" {
		t.Fatalf("spellings disagree: %+v vs %+v", bySlug, byText)
	}

	// Renaming the heading breaks the anchor rather than silently pointing at
	// whatever now occupies that position.
	if _, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: doc.ID, Title: "Headings", Body: "# Getting Started\n\nIntro paragraph.\n\n## Installation\n\nSteps.\n",
		BaseRevisionID: mustCurrentRevision(t, st, doc.ID),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.FindDocumentBlock(ctx, doc.ID, "install-setup"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a renamed heading must stop resolving: %v", err)
	}
	if _, err := st.FindDocumentBlock(ctx, doc.ID, "installation"); err != nil {
		t.Fatalf("the new heading should resolve: %v", err)
	}
}

// An author-written marker outranks a heading slug: precedence is marker, then
// block ID, then slug.
func TestAnchorPrecedenceIsMarkerThenIDThenSlug(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	// A paragraph whose marker is spelled exactly like the heading's slug.
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{
		Title: "Precedence",
		Body:  "# Summary\n\nThe paragraph the author named. ^summary\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := st.FindDocumentBlock(ctx, doc.ID, "summary")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Kind != "paragraph" || resolved.Marker != "summary" {
		t.Fatalf("the author's marker must win over a heading slug: %+v", resolved)
	}
}

func TestBacklinkCountsIncludeHeadingAnchors(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	target, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_headings", Title: "Headings", Body: headingBody})
	if err != nil {
		t.Fatal(err)
	}
	// Two spellings of the same anchor. The slug form works in a Markdown link;
	// the heading-text form needs a wikilink, because in Markdown a space ends
	// an unquoted URL — which is part of why a stable link carries the slug.
	body := "[slugged](" + target.URI + "#install-setup)\n\n[[Headings#Install & Setup]]\n"
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Source", Body: body}); err != nil {
		t.Fatal(err)
	}
	blocks, err := st.ListDocumentBlocks(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range blocks {
		if block.HeadingSlug == "install-setup" {
			if block.Backlinks != 2 {
				t.Fatalf("both spellings should count against one heading: %+v", block)
			}
			return
		}
	}
	t.Fatal("heading not found")
}

func mustCurrentRevision(t *testing.T, st *SQLiteStore, documentID string) string {
	t.Helper()
	document, err := st.GetDocument(context.Background(), documentID)
	if err != nil {
		t.Fatal(err)
	}
	return document.CurrentRevisionID
}

// Percent-escapes are read as escapes only inside a URI-schemed link, which is
// the one place something declared itself a URI. A bare Markdown anchor is text
// the author typed.
func TestPercentEscapesDecodeOnlyInsideURISchemedLinks(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_encoded", Title: "Encoded", Body: headingBody,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Inside a stable link: decoded, so it reaches `install-setup`.
	resolution, err := st.ResolveStableLink(ctx,
		"notrios://databases/"+identity.DatabaseID+"/documents/"+target.ID+"#Install%20%26%20Setup")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Status != StableLinkResolved || resolution.BlockKind != "heading" {
		t.Fatalf("an escaped anchor in a URI should resolve: %+v", resolution)
	}

	// The same bytes in a bare Markdown anchor stay literal and do not resolve.
	source, err := st.CreateDocument(ctx, CreateDocumentRequest{
		Title: "Source",
		Body: "[uri form](" + target.URI + "#Install%20%26%20Setup)\n\n" +
			"[bare form](#Install%20%26%20Setup)\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := st.LintWorkspace(ctx, LintRequest{Checks: []string{LintUnresolvedHeadingAnchor}})
	if err != nil {
		t.Fatal(err)
	}
	// Only the bare one is unresolved: the URI form decoded and matched.
	if report.Checks[0].Count != 1 {
		t.Fatalf("expected exactly the bare anchor to be unresolved, got %d", report.Checks[0].Count)
	}
	if finding := report.Checks[0].Findings[0]; finding.DocumentID != source.ID || finding.Line != 3 {
		t.Fatalf("the wrong anchor was reported: %+v", finding)
	}

	// The decoded URI anchor counts as a backlink against the heading it names.
	blocks, err := st.ListDocumentBlocks(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range blocks {
		if block.HeadingSlug == "install-setup" && block.Backlinks != 1 {
			t.Fatalf("a decoded URI anchor should count as a backlink: %+v", block)
		}
	}
}

// A heading whose text really contains a percent sign keeps working: nothing
// in a bare anchor is reinterpreted, and a URI spelling encodes it as %25.
func TestHeadingsContainingPercentSigns(t *testing.T) {
	ctx := context.Background()
	st := blockTestStore(t)
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_percent", Title: "Percent", Body: "## 100% Coverage\n\nBody.\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	block, err := st.FindDocumentBlock(ctx, doc.ID, "100% Coverage")
	if err != nil || block.HeadingSlug != "100-coverage" {
		t.Fatalf("heading text lookup: %+v %v", block, err)
	}
	resolution, err := st.ResolveStableLink(ctx,
		"notrios://databases/"+identity.DatabaseID+"/documents/"+doc.ID+"#100%25%20Coverage")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Status != StableLinkResolved || resolution.BlockKind != "heading" {
		t.Fatalf("a properly encoded percent should resolve: %+v", resolution)
	}
}
