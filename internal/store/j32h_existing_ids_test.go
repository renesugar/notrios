package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestExistingDocumentIDsAgreesWithGetDocuments holds the presence-only lookup
// to the one it replaced: the same ids present, the same absent, including a
// deleted document and one whose current revision row is gone (v1.0 J32-H).
func TestExistingDocumentIDsAgreesWithGetDocuments(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, err := OpenSQLiteWithAssetStore(filepath.Join(root, "store.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	present, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_present", Title: "Present", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_deleted", Title: "Deleted", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: deleted.ID, BaseRevisionID: deleted.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	orphan, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_orphan", Title: "Orphan", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	// A document whose current revision row is missing is absent from
	// GetDocuments because of the join, so it must be absent here too.
	st.mu.Lock()
	err = st.execPreparedLocked(`UPDATE documents SET current_revision_id = 'rev_missing' WHERE id = ?`, orphan.ID)
	st.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	ids := []string{present.ID, deleted.ID, orphan.ID, "doc_never_existed"}
	documents, err := st.GetDocuments(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := st.ExistingDocumentIDs(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		_, inDocuments := documents[id]
		if existing[id] != inDocuments {
			t.Errorf("%s: presence says %v, GetDocuments says %v", id, existing[id], inDocuments)
		}
	}
	if !existing[present.ID] {
		t.Error("the live document must be present")
	}
	if existing[deleted.ID] || existing[orphan.ID] || existing["doc_never_existed"] {
		t.Errorf("only the live document should be present: %v", existing)
	}

	empty, err := st.ExistingDocumentIDs(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("no ids: %v, %v", empty, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := st.ExistingDocumentIDs(cancelled, ids); err == nil {
		t.Error("a cancelled context must not be served")
	}
}
