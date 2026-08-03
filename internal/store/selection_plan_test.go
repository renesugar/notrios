package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func newSelectionTestStore(t *testing.T) *SQLiteStore {
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

func TestSelectionPlanPublicationPrivacyAndDeterminism(t *testing.T) {
	ctx := context.Background()
	st := newSelectionTestStore(t)
	public, err := st.CreateNotebook(ctx, CreateNotebookRequest{PreferredID: "nb_public", Name: "Public"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.CreateNotebook(ctx, CreateNotebookRequest{PreferredID: "nb_public_child", ParentID: public.ID, Name: "Child"})
	if err != nil {
		t.Fatal(err)
	}
	childDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_child", NotebookID: child.ID, Title: "Child", Body: "child"})
	if err != nil {
		t.Fatal(err)
	}
	privateDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_private", NotebookID: child.ID, Title: "Private", Body: "private"})
	if err != nil {
		t.Fatal(err)
	}
	outsideDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_outside", Title: "Outside", Body: "outside"})
	if err != nil {
		t.Fatal(err)
	}
	publicDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_public", NotebookID: public.ID, Title: "Public",
		Body: "[child](" + childDoc.URI + ") [private](" + privateDoc.URI + ") [missing](Missing Note) [external](https://example.com)",
	})
	if err != nil {
		t.Fatal(err)
	}
	trashedDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_trashed", Title: "Trashed", Body: "trash"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: trashedDoc.ID, BaseRevisionID: trashedDoc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	for _, tagged := range []struct {
		id, tag string
	}{{publicDoc.ID, "publish"}, {childDoc.ID, "publish"}, {privateDoc.ID, "private"}, {outsideDoc.ID, "publish"}} {
		if _, err := st.AddDocumentTag(ctx, tagged.id, tagged.tag); err != nil {
			t.Fatal(err)
		}
	}
	publicResource, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_public", Filename: "public.bin", Content: bytes.NewBufferString("public-resource")})
	if err != nil {
		t.Fatal(err)
	}
	privateResource, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_private", Filename: "private.bin", Content: bytes.NewBufferString("private-resource")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: publicDoc.ID, ResourceID: publicResource.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: privateDoc.ID, ResourceID: privateResource.ID}); err != nil {
		t.Fatal(err)
	}
	for _, source := range []SetDocumentSourceRequest{
		{DocumentID: publicDoc.ID, SourceSystem: "joplin", ExternalID: "ext-public", SourceURL: "file:///private/source", MetadataJSON: `{"local_path":"private"}`},
		{DocumentID: privateDoc.ID, SourceSystem: "joplin", ExternalID: "ext-private"},
	} {
		if _, err := st.SetDocumentSource(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
	for _, bundle := range []PutSourceBundleItemRequest{
		{SourceSystem: "joplin", SourceKey: "/home/private/export", CollectionID: "default", ItemKey: "private/path/public.md", ItemType: "note", ExternalID: "ext-public", RelativePath: "public.md", Content: strings.NewReader("public source")},
		{SourceSystem: "joplin", SourceKey: "/home/private/export", CollectionID: "default", ItemKey: "private/path/private.md", ItemType: "note", ExternalID: "ext-private", RelativePath: "private.md", Content: strings.NewReader("private source")},
	} {
		if _, err := st.PutSourceBundleItem(ctx, bundle); err != nil {
			t.Fatal(err)
		}
	}

	req := SelectionPlanRequest{
		Target:      SelectionTargetPublicationHandoff,
		Selection:   SelectionSpec{NotebookIDs: []string{public.ID}},
		Policy:      PrivacyPolicy{MaxResourceBytes: 5},
		DetailLimit: 100,
	}
	plan, err := st.PlanSelection(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Counts.SelectedDocuments != 2 || plan.Counts.ExcludedDocuments != 1 {
		t.Fatalf("unexpected document counts: %+v", plan.Counts)
	}
	if plan.Counts.ReachableResources != 1 || plan.Counts.OversizedResources != 1 {
		t.Fatalf("unexpected resource counts: %+v", plan.Counts)
	}
	if plan.Counts.AvailableSourceBundles != 1 || plan.Counts.IncludedSourceBundles != 0 {
		t.Fatalf("unexpected source bundle policy counts: %+v", plan.Counts)
	}
	if plan.Counts.InternalLinks != 1 || plan.Counts.PrivateLinks != 1 || plan.Counts.BrokenLinks != 1 || plan.Counts.ExternalLinks != 1 {
		t.Fatalf("unexpected link counts: %+v links=%+v", plan.Counts, plan.Links)
	}
	if plan.Policy.IncludeSourceBundles || plan.Policy.IncludeProvenance || plan.Policy.IncludePrivateMetadata || plan.Policy.IncludeTrashed || plan.Policy.LinkAction != "plain_text" {
		t.Fatalf("publication defaults are not privacy-safe: %+v", plan.Policy)
	}
	if len(plan.ManifestSHA256) != 64 || strings.Contains(mustJSON(t, plan), "file:///private/source") || strings.Contains(mustJSON(t, plan), "local_path") || strings.Contains(mustJSON(t, plan), "/home/private/export") || strings.Contains(mustJSON(t, plan), "private/path") || strings.Contains(mustJSON(t, plan), "storage_path\":\"source-bundles") {
		t.Fatalf("plan leaked private source metadata or produced invalid digest: %s", mustJSON(t, plan))
	}
	before, err := st.GetDocument(ctx, publicDoc.ID)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := st.PlanSelection(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	after, err := st.GetDocument(ctx, publicDoc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, repeated) {
		t.Fatalf("plan is not deterministic\nfirst=%s\nsecond=%s", mustJSON(t, plan), mustJSON(t, repeated))
	}
	if before.CurrentRevisionID != after.CurrentRevisionID || before.Body != after.Body {
		t.Fatal("read-only plan changed canonical document state")
	}

	full, err := st.PlanSelection(ctx, SelectionPlanRequest{Target: SelectionTargetFullArchive, DetailLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if full.Counts.SelectedDocuments != 5 || full.Counts.IncludedSourceBundles != 2 || !full.Policy.IncludeTrashed {
		t.Fatalf("full archive scope mismatch: %+v", full)
	}
}

func TestSelectionPlanSelectorsLimitsAndDetailCap(t *testing.T) {
	ctx := context.Background()
	st := newSelectionTestStore(t)
	nb, err := st.CreateNotebook(ctx, CreateNotebookRequest{PreferredID: "nb_select", Name: "Select"})
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_alpha", NotebookID: nb.ID, Title: "Alpha", Body: "alpha body"})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_beta", NotebookID: nb.ID, Title: "Beta", Body: "beta body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDocumentTag(ctx, alpha.ID, "chosen"); err != nil {
		t.Fatal(err)
	}

	all, err := st.PlanSelection(ctx, SelectionPlanRequest{
		Target:      SelectionTargetSubsetTransfer,
		Selection:   SelectionSpec{NotebookIDs: []string{nb.ID}, Tags: []string{"chosen"}, Match: "all"},
		DetailLimit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if all.Counts.SelectedDocuments != 1 || all.Documents[0].ID != alpha.ID || all.Counts.ExcludedDocuments != 1 {
		t.Fatalf("match-all or detail cap mismatch: %+v", all)
	}
	any, err := st.PlanSelection(ctx, SelectionPlanRequest{
		Target:      SelectionTargetSubsetTransfer,
		Selection:   SelectionSpec{Query: `title:"Alpha"`, DocumentIDs: []string{beta.ID, "doc_missing"}, Match: "any"},
		DetailLimit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if any.Counts.SelectedDocuments != 2 || any.Counts.ExcludedDocuments != 1 || !any.Truncated || len(any.Documents) != 1 {
		t.Fatalf("query/document selector mismatch: %+v", any)
	}

	tooMany := make([]string, MaxSelectionDocumentIDs+1)
	for i := range tooMany {
		tooMany[i] = "doc_limit_" + itoa(i)
	}
	_, err = st.PlanSelection(ctx, SelectionPlanRequest{Target: SelectionTargetSubsetTransfer, Selection: SelectionSpec{DocumentIDs: tooMany}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected bounded explicit-ID error, got %v", err)
	}
	_, err = st.PlanSelection(ctx, SelectionPlanRequest{Target: SelectionTargetPublicationHandoff})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected unscoped publication rejection, got %v", err)
	}
	_, err = st.PlanSelection(ctx, SelectionPlanRequest{Target: SelectionTargetSubsetTransfer, Selection: SelectionSpec{Query: "is:trashed"}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected trashed-query policy rejection, got %v", err)
	}
}
