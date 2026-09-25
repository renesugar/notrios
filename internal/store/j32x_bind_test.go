package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestBindPreservesEmptyAndNulBytes covers what changed when binding stopped
// copying values into C memory first (v1.0 J32-X):
//
//   - an empty string must still be an empty string and not NULL, which a null
//     pointer would have made it;
//   - a value containing a NUL byte is now stored whole. C.CString bound with
//     length -1, so SQLite saw the value only up to the first NUL and the rest
//     was silently dropped. This pins the new behaviour, which is the one that
//     keeps what the caller passed.
func TestBindPreservesEmptyAndNulBytes(t *testing.T) {
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

	empty, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_empty", Title: "Empty", Body: ""})
	if err != nil {
		t.Fatal(err)
	}
	read, err := st.GetDocument(ctx, empty.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Body != "" {
		t.Errorf("empty body came back as %q", read.Body)
	}

	body := "before\x00after ελληνικά\x00end"
	embedded, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_nul", Title: "Nul", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	read, err = st.GetDocument(ctx, embedded.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Body != body {
		t.Errorf("body with NUL bytes came back as %q (%d bytes), want %q (%d bytes)",
			read.Body, len(read.Body), body, len(body))
	}
	if !strings.Contains(read.Body, "after") {
		t.Error("everything after the first NUL byte was dropped")
	}
}
