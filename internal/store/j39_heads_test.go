package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/syncdelta"
)

// TestJ39ImportDocumentHeadsAnswersWithoutTheBody pins what an importer's skip
// relies on (v1.0 J39): the head of a note is its current revision's content
// hash, its title and notebook, and the digest of its references; a note in the
// trash has none; and the digest agrees with the one an importer takes over the
// references it attached, in whatever order it listed them.
func TestJ39ImportDocumentHeadsAnswersWithoutTheBody(t *testing.T) {
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
	for _, id := range []string{"res_one", "res_two"} {
		if _, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: id, Filename: id + ".bin", Content: bytes.NewBufferString(id)}); err != nil {
			t.Fatal(err)
		}
	}
	refs := []AttachResourceRequest{
		{DocumentID: "doc_with_refs", ResourceID: "res_two", RelationType: "embed", AnchorJSON: `{"raw_target":"two"}`},
		{DocumentID: "doc_with_refs", ResourceID: "res_one", RelationType: "link", AnchorJSON: `{"raw_target":"one"}`},
	}
	state := func(key, target string) ImportItemState {
		return ImportItemState{SourceSystem: "obsidian", SourceKey: "heads", CollectionID: "default", ItemKey: key, ItemType: "markdown", Fingerprint: key, TargetID: target, Action: "create"}
	}
	body := "# With refs\n"
	if err := st.ApplyImportDocumentBatch(ctx, ImportDocumentBatchRequest{
		Checkpoint: ImportCheckpoint{SourceSystem: "obsidian", SourceKey: "heads", CollectionID: "default", InventoryFingerprint: "x", Phase: "notes", Status: "running", ReportJSON: "{}"},
		Documents: []ImportDocumentMutation{
			{
				Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_with_refs", Title: "With refs", Body: body, Message: "import"},
				Source:    SetDocumentSourceRequest{DocumentID: "doc_with_refs", SourceSystem: "obsidian", ExternalID: "with.md"},
				Resources: refs, State: state("note:with.md", "doc_with_refs"),
			},
			{
				Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_plain", Title: "Plain", Body: "# Plain\n", Message: "import"},
				Source: SetDocumentSourceRequest{DocumentID: "doc_plain", SourceSystem: "obsidian", ExternalID: "plain.md"},
				State:  state("note:plain.md", "doc_plain"),
			},
			{
				Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_trashed", Title: "Trashed", Body: "# Trashed\n", Message: "import"},
				Source: SetDocumentSourceRequest{DocumentID: "doc_trashed", SourceSystem: "obsidian", ExternalID: "trashed.md"},
				State:  state("note:trashed.md", "doc_trashed"),
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	trashed, err := st.GetDocument(ctx, "doc_trashed")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: trashed.ID, BaseRevisionID: trashed.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}

	heads, err := st.ImportDocumentHeads(ctx, []string{"doc_with_refs", "doc_plain", "doc_trashed", "doc_missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(heads) != 2 {
		t.Fatalf("only the two live notes have heads: %#v", heads)
	}
	with := heads["doc_with_refs"]
	document, err := st.GetDocument(ctx, "doc_with_refs")
	if err != nil {
		t.Fatal(err)
	}
	if with.CurrentRevisionID != document.CurrentRevisionID || with.Title != "With refs" || with.NotebookID != document.NotebookID ||
		with.ContentSHA256 != syncdelta.SHA256HexString(body) {
		t.Fatalf("head %#v does not describe %#v", with, document)
	}
	reversed := []AttachResourceRequest{refs[1], refs[0]}
	if with.ResourceRefsSHA256 != ResourceRefsDigest(reversed) || with.ResourceRefsSHA256 != ResourceRefsDigest(refs) {
		t.Fatalf("the stored references' digest %s should match the attached ones'", with.ResourceRefsSHA256)
	}
	if heads["doc_plain"].ResourceRefsSHA256 != ResourceRefsDigest(nil) {
		t.Fatalf("a note with no references has the empty digest")
	}
	if sum := sha256.Sum256(nil); emptyResourceRefsDigest != hex.EncodeToString(sum[:]) {
		t.Fatalf("emptyResourceRefsDigest is not the SHA-256 of nothing")
	}

	if err := st.DetachDocumentResource(ctx, "doc_with_refs", "res_one"); err != nil {
		t.Fatal(err)
	}
	heads, err = st.ImportDocumentHeads(ctx, []string{"doc_with_refs"})
	if err != nil {
		t.Fatal(err)
	}
	if heads["doc_with_refs"].ResourceRefsSHA256 == with.ResourceRefsSHA256 {
		t.Fatal("detaching a reference must change the digest")
	}
}
