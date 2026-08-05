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
