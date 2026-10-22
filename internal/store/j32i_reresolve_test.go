package store

import (
	"context"
	"path/filepath"
	"testing"
)

func j32iStore(t *testing.T) *SQLiteStore {
	t.Helper()
	root := t.TempDir()
	st, err := OpenSQLiteWithAssetStore(filepath.Join(root, "store.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

func j32iLinks(t *testing.T, st *SQLiteStore, documentID string) []DocumentLink {
	t.Helper()
	page, err := st.ListDocumentLinks(context.Background(), documentID, "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	return page.Outgoing
}

// TestReresolveMatchesReparsing is the J32-I invariant: re-resolving a
// document's stored rows produces what parsing its body again produces, for a
// link that resolves, one that does not, an external one, an anchor-only one,
// and a resource link.
func TestReresolveMatchesReparsing(t *testing.T) {
	ctx := context.Background()
	st := j32iStore(t)

	target, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_target", Title: "Target", Body: "# Target\n"})
	if err != nil {
		t.Fatal(err)
	}
	body := "See [[document://default/documents/" + target.ID + "]] and [[document://default/documents/doc_missing]].\n\n" +
		"External [text](https://example.com/page) and an anchor [here](#section) too.\n\n" +
		"## Section\n\nWords.\n"
	source, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_source", Title: "Source", Body: body})
	if err != nil {
		t.Fatal(err)
	}

	parsed := j32iLinks(t, st, source.ID)
	if len(parsed) == 0 {
		t.Fatal("the body produced no links")
	}

	st.mu.Lock()
	reresolved, err := st.reresolveDocumentLinksLocked(source.ID, source.CollectionID)
	st.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !reresolved {
		t.Fatal("a document written by this path must be re-resolvable")
	}

	after := j32iLinks(t, st, source.ID)
	if len(after) != len(parsed) {
		t.Fatalf("re-resolution changed the row count: %d became %d", len(parsed), len(after))
	}
	for index := range parsed {
		if after[index] != parsed[index] {
			t.Errorf("link %d differs:\n after %+v\n parse %+v", index, after[index], parsed[index])
		}
	}
}

// TestReresolveResolvesATargetWrittenLater is why the final pass exists: a note
// linking to one imported later must become resolved without its body being
// parsed again.
func TestReresolveResolvesATargetWrittenLater(t *testing.T) {
	ctx := context.Background()
	st := j32iStore(t)

	source, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_source",
		Title:       "Source",
		Body:        "Link to [[document://default/documents/doc_later]] before it exists.\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	before := j32iLinks(t, st, source.ID)
	if len(before) != 1 || before[0].ResolutionStatus != "unresolved" {
		t.Fatalf("expected one unresolved link, got %+v", before)
	}

	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_later", Title: "Later", Body: "# Later\n"}); err != nil {
		t.Fatal(err)
	}

	st.mu.Lock()
	reresolved, err := st.reresolveDocumentLinksLocked(source.ID, source.CollectionID)
	st.mu.Unlock()
	if err != nil || !reresolved {
		t.Fatalf("re-resolution: %v %v", reresolved, err)
	}
	after := j32iLinks(t, st, source.ID)
	if len(after) != 1 || after[0].ResolutionStatus != "resolved" || after[0].TargetDocumentID != "doc_later" {
		t.Fatalf("the link should now resolve to doc_later: %+v", after)
	}
	// Everything the body decided must be untouched.
	if after[0].RawTarget != before[0].RawTarget || after[0].SourceStartByte != before[0].SourceStartByte ||
		after[0].SourceLine != before[0].SourceLine || after[0].SourceColumn != before[0].SourceColumn ||
		after[0].Context != before[0].Context || after[0].RelationType != before[0].RelationType {
		t.Errorf("re-resolution changed what the body decided:\n before %+v\n  after %+v", before[0], after[0])
	}
}

// TestReresolveDeclinesWithoutBlockRows keeps the fallback: a document this
// path cannot vouch for is parsed as before.
func TestReresolveDeclinesWithoutBlockRows(t *testing.T) {
	ctx := context.Background()
	st := j32iStore(t)
	document, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_x", Title: "X", Body: "Body [[document://default/documents/doc_y]]\n"})
	if err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	err = st.execPreparedLocked(`DELETE FROM document_blocks WHERE document_id = ?`, document.ID)
	st.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	reresolved, err := st.reresolveDocumentLinksLocked(document.ID, document.CollectionID)
	st.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if reresolved {
		t.Fatal("a document with no block rows must not be vouched for")
	}
}
