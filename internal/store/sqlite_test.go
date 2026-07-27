package store

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSQLiteStoreCreateReadSearch(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	doc, err := st.CreateDocument(context.Background(), CreateDocumentRequest{
		Title: "SQLite and MCP",
		Body:  "The companion service uses SQLite FTS5 and exposes REST plus MCP.",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if doc.ID == "" || doc.CurrentRevisionID == "" || doc.URI == "" {
		t.Fatalf("created document missing IDs: %+v", doc)
	}

	got, err := st.GetDocument(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.Title != doc.Title || got.Body != doc.Body {
		t.Fatalf("read mismatch got=%+v want=%+v", got, doc)
	}

	search, err := st.Search(context.Background(), SearchRequest{Query: "SQLite MCP", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(search.Hits) != 1 {
		t.Fatalf("expected 1 search hit, got %d", len(search.Hits))
	}
	if search.Hits[0].ID != doc.ID {
		t.Fatalf("wrong search hit: %+v", search.Hits[0])
	}
}

func TestSQLiteStoreDefaultCollection(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	collections, err := st.ListCollections(context.Background())
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if len(collections) != 1 || collections[0].ID != "default" {
		t.Fatalf("unexpected collections: %+v", collections)
	}
}

func TestSQLiteStoreStatus(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	status, err := st.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Driver != "sqlite" || status.State != "open" || status.Path != ":memory:" || status.SchemaVersion != 9 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSQLiteStoreUpdateCreatesRevisionAndRefreshesSearch(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	created, err := st.CreateDocument(context.Background(), CreateDocumentRequest{
		Title: "Original title",
		Body:  "alpha oldterm",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	updated, err := st.UpdateDocument(context.Background(), UpdateDocumentRequest{
		ID:             created.ID,
		Title:          "Updated title",
		Body:           "beta newterm",
		BaseRevisionID: created.CurrentRevisionID,
		Message:        "test update",
	})
	if err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
	if updated.CurrentRevisionID == created.CurrentRevisionID {
		t.Fatalf("expected a new revision id after update")
	}
	if updated.Title != "Updated title" || updated.Body != "beta newterm" {
		t.Fatalf("unexpected updated document: %+v", updated)
	}

	revisions, err := st.ListDocumentRevisions(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ListDocumentRevisions: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d: %+v", len(revisions), revisions)
	}

	oldSearch, err := st.Search(context.Background(), SearchRequest{Query: "oldterm", Limit: 10})
	if err != nil {
		t.Fatalf("Search oldterm: %v", err)
	}
	if len(oldSearch.Hits) != 0 {
		t.Fatalf("old term should not remain in FTS after update: %+v", oldSearch.Hits)
	}
	newSearch, err := st.Search(context.Background(), SearchRequest{Query: "newterm", Limit: 10})
	if err != nil {
		t.Fatalf("Search newterm: %v", err)
	}
	if len(newSearch.Hits) != 1 || newSearch.Hits[0].ID != created.ID {
		t.Fatalf("expected updated document in FTS, got %+v", newSearch.Hits)
	}
}

func TestSQLiteStoreUpdateRequiresCurrentRevision(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	created, err := st.CreateDocument(context.Background(), CreateDocumentRequest{Title: "Conflict", Body: "one"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	_, err = st.UpdateDocument(context.Background(), UpdateDocumentRequest{ID: created.ID, Title: "No base", Body: "two"})
	if !errors.Is(err, ErrPreconditionRequired) {
		t.Fatalf("expected ErrPreconditionRequired, got %v", err)
	}
	_, err = st.UpdateDocument(context.Background(), UpdateDocumentRequest{ID: created.ID, Title: "Stale", Body: "two", BaseRevisionID: "rev_stale"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestSQLiteStoreSoftDeleteAndRestoreRevision(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	created, err := st.CreateDocument(context.Background(), CreateDocumentRequest{Title: "Restore me", Body: "durable history"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	originalRevisionID := created.CurrentRevisionID
	if err := st.DeleteDocument(context.Background(), DeleteDocumentRequest{ID: created.ID, BaseRevisionID: created.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if _, err := st.GetDocument(context.Background(), created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted document should not be readable as current document, got %v", err)
	}
	search, err := st.Search(context.Background(), SearchRequest{Query: "durable", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(search.Hits) != 0 {
		t.Fatalf("deleted document should not be searchable: %+v", search.Hits)
	}
	revisions, err := st.ListDocumentRevisions(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ListDocumentRevisions after delete: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected create and delete revisions, got %+v", revisions)
	}
	deleteRevisionID := revisions[0].ID
	if deleteRevisionID == originalRevisionID {
		deleteRevisionID = revisions[1].ID
	}
	restored, err := st.RestoreDocumentRevision(context.Background(), RestoreRevisionRequest{
		DocumentID:     created.ID,
		RevisionID:     originalRevisionID,
		BaseRevisionID: deleteRevisionID,
	})
	if err != nil {
		t.Fatalf("RestoreDocumentRevision: %v", err)
	}
	if restored.Title != "Restore me" || restored.Body != "durable history" {
		t.Fatalf("unexpected restored document: %+v", restored)
	}
	search, err = st.Search(context.Background(), SearchRequest{Query: "durable", Limit: 10})
	if err != nil {
		t.Fatalf("Search after restore: %v", err)
	}
	if len(search.Hits) != 1 || search.Hits[0].ID != created.ID {
		t.Fatalf("restored document should be searchable: %+v", search.Hits)
	}
}

func TestSQLiteStoreResourceUploadAttachDownloadAndSafeDelete(t *testing.T) {
	assetRoot := t.TempDir()
	st, err := OpenSQLiteWithAssetStore(":memory:", assetRoot)
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	doc, err := st.CreateDocument(context.Background(), CreateDocumentRequest{
		Title: "Resource test",
		Body:  "![one](resource://default/resources/pending)",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	res, err := st.CreateResource(context.Background(), CreateResourceRequest{
		Filename: "diagram.txt",
		MIMEType: "text/plain",
		Content:  strings.NewReader("resource body"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	if res.ID == "" || res.SHA256 == "" || res.SizeBytes != int64(len("resource body")) {
		t.Fatalf("unexpected resource metadata: %+v", res)
	}

	ref, err := st.AttachDocumentResource(context.Background(), AttachResourceRequest{
		DocumentID:   doc.ID,
		ResourceID:   res.ID,
		RelationType: "embedded",
		Ordinal:      7,
	})
	if err != nil {
		t.Fatalf("AttachDocumentResource: %v", err)
	}
	if ref.RelationType != "embedded" || ref.Ordinal != 7 || ref.Resource.SHA256 != res.SHA256 {
		t.Fatalf("unexpected reference: %+v", ref)
	}

	refs, err := st.ListDocumentResources(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("ListDocumentResources: %v", err)
	}
	if len(refs) != 1 || refs[0].ResourceID != res.ID {
		t.Fatalf("unexpected refs: %+v", refs)
	}

	got, content, err := st.OpenResourceContent(context.Background(), res.ID)
	if err != nil {
		t.Fatalf("OpenResourceContent: %v", err)
	}
	data, err := io.ReadAll(content)
	_ = content.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got.ID != res.ID || string(data) != "resource body" {
		t.Fatalf("content mismatch got=%+v body=%q", got, string(data))
	}

	if err := st.DeleteResource(context.Background(), res.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("DeleteResource should conflict while referenced, got %v", err)
	}
	if err := st.DetachDocumentResource(context.Background(), doc.ID, res.ID); err != nil {
		t.Fatalf("DetachDocumentResource: %v", err)
	}
	if err := st.DeleteResource(context.Background(), res.ID); err != nil {
		t.Fatalf("DeleteResource after detach: %v", err)
	}
}

func TestSQLiteStoreResourceDeduplicatesBlobs(t *testing.T) {
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	first, err := st.CreateResource(context.Background(), CreateResourceRequest{Filename: "a.txt", Content: strings.NewReader("same")})
	if err != nil {
		t.Fatalf("CreateResource first: %v", err)
	}
	second, err := st.CreateResource(context.Background(), CreateResourceRequest{Filename: "b.txt", Content: strings.NewReader("same")})
	if err != nil {
		t.Fatalf("CreateResource second: %v", err)
	}
	if first.ID == second.ID || first.SHA256 != second.SHA256 {
		t.Fatalf("expected separate logical resources sharing one blob: first=%+v second=%+v", first, second)
	}
}

func TestSQLiteStoreExtractsDocumentResourceExternalAndBacklinks(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	target, err := st.CreateDocument(context.Background(), CreateDocumentRequest{Title: "Target Note", Body: "target"})
	if err != nil {
		t.Fatalf("CreateDocument target: %v", err)
	}
	res, err := st.CreateResource(context.Background(), CreateResourceRequest{Filename: "diagram.png", MIMEType: "image/png", Content: strings.NewReader("image-bytes")})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	source, err := st.CreateDocument(context.Background(), CreateDocumentRequest{
		Title: "Source",
		Body:  "See [target](" + target.URI + "#Heading) and ![diagram](" + res.URI + ") and [[Missing Note]] and [web](https://example.com).",
	})
	if err != nil {
		t.Fatalf("CreateDocument source: %v", err)
	}

	page, err := st.ListDocumentLinks(context.Background(), source.ID, "both")
	if err != nil {
		t.Fatalf("ListDocumentLinks source: %v", err)
	}
	if len(page.Outgoing) != 4 {
		t.Fatalf("expected 4 outgoing links, got %#v", page.Outgoing)
	}
	statuses := map[string]bool{}
	for _, link := range page.Outgoing {
		statuses[link.ResolutionStatus+":"+link.RawTarget] = true
	}
	if !statuses["resolved:"+target.URI] || !statuses["resolved:"+res.URI] || !statuses["unresolved:Missing Note"] || !statuses["external:https://example.com"] {
		t.Fatalf("unexpected outgoing statuses: %#v", page.Outgoing)
	}

	backlinks, err := st.ListDocumentLinks(context.Background(), target.ID, "incoming")
	if err != nil {
		t.Fatalf("ListDocumentLinks target incoming: %v", err)
	}
	if len(backlinks.Incoming) != 1 || backlinks.Incoming[0].SourceDocumentID != source.ID {
		t.Fatalf("expected one backlink from source, got %#v", backlinks.Incoming)
	}
}

func TestSQLiteStoreRebuildsLinksOnUpdate(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	target, err := st.CreateDocument(context.Background(), CreateDocumentRequest{Title: "Target", Body: "target"})
	if err != nil {
		t.Fatalf("CreateDocument target: %v", err)
	}
	source, err := st.CreateDocument(context.Background(), CreateDocumentRequest{Title: "Source", Body: "[[Target]]"})
	if err != nil {
		t.Fatalf("CreateDocument source: %v", err)
	}
	page, err := st.ListDocumentLinks(context.Background(), target.ID, "incoming")
	if err != nil {
		t.Fatalf("ListDocumentLinks incoming: %v", err)
	}
	if len(page.Incoming) != 1 {
		t.Fatalf("expected initial backlink, got %#v", page.Incoming)
	}
	updated, err := st.UpdateDocument(context.Background(), UpdateDocumentRequest{ID: source.ID, Title: "Source", Body: "no links now", BaseRevisionID: source.CurrentRevisionID})
	if err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
	_ = updated
	page, err = st.ListDocumentLinks(context.Background(), target.ID, "incoming")
	if err != nil {
		t.Fatalf("ListDocumentLinks incoming after update: %v", err)
	}
	if len(page.Incoming) != 0 {
		t.Fatalf("expected backlink removal after update, got %#v", page.Incoming)
	}
}
