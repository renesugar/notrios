package store

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSQLiteStoreResourceReportExactUnreferencedAndNotebookUsage(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if err := st.Exec(ctx, `INSERT INTO collections(id, name) VALUES('other', 'Other')`); err != nil {
		t.Fatalf("insert collection: %v", err)
	}
	images, err := st.CreateNotebook(ctx, CreateNotebookRequest{PreferredID: "nb_images", Name: "Images"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	notesDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_notes", Title: "Notes resource"})
	if err != nil {
		t.Fatalf("CreateDocument notes: %v", err)
	}
	imagesDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_images", NotebookID: images.ID, Title: "Images resource"})
	if err != nil {
		t.Fatalf("CreateDocument images: %v", err)
	}
	trashedDoc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_trashed", Title: "Trashed resource"})
	if err != nil {
		t.Fatalf("CreateDocument trashed: %v", err)
	}

	first, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_first", CollectionID: "default", Filename: "first.png", MIMEType: "image/png", Content: strings.NewReader("same bytes"),
	})
	if err != nil {
		t.Fatalf("CreateResource first: %v", err)
	}
	second, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_second", CollectionID: "other", Filename: "second.png", MIMEType: "image/png", Content: strings.NewReader("same bytes"),
	})
	if err != nil {
		t.Fatalf("CreateResource second: %v", err)
	}
	orphan, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_orphan", Filename: "orphan.bin", Content: strings.NewReader("unreferenced bytes"),
	})
	if err != nil {
		t.Fatalf("CreateResource orphan: %v", err)
	}
	trashHeld, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_trash_held", Filename: "trash-held.bin", Content: strings.NewReader("held by trash"),
	})
	if err != nil {
		t.Fatalf("CreateResource trash-held: %v", err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: notesDoc.ID, ResourceID: first.ID}); err != nil {
		t.Fatalf("attach first: %v", err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: imagesDoc.ID, ResourceID: second.ID}); err != nil {
		t.Fatalf("attach second: %v", err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: trashedDoc.ID, ResourceID: trashHeld.ID}); err != nil {
		t.Fatalf("attach trash-held: %v", err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: trashedDoc.ID, BaseRevisionID: trashedDoc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument trashed: %v", err)
	}

	report, err := st.ResourceReport(ctx)
	if err != nil {
		t.Fatalf("ResourceReport: %v", err)
	}
	if len(report.ExactDuplicates) != 1 {
		t.Fatalf("exact duplicate groups: %+v", report.ExactDuplicates)
	}
	duplicate := report.ExactDuplicates[0]
	if duplicate.SHA256 != first.SHA256 || duplicate.ResourceCount != 2 || duplicate.ReferenceCount != 2 || !duplicate.CrossCollection {
		t.Fatalf("unexpected duplicate group: %+v", duplicate)
	}
	if len(duplicate.CollectionIDs) != 2 || duplicate.CollectionIDs[0] != "default" || duplicate.CollectionIDs[1] != "other" {
		t.Fatalf("duplicate collections: %+v", duplicate.CollectionIDs)
	}
	if len(report.UnreferencedBlobs) != 1 || report.UnreferencedBlobs[0].SHA256 != orphan.SHA256 ||
		len(report.UnreferencedBlobs[0].Resources) != 1 || report.UnreferencedBlobs[0].Resources[0].ID != orphan.ID {
		t.Fatalf("unreferenced blobs: %+v", report.UnreferencedBlobs)
	}
	usage := map[string]NotebookResourceUsage{}
	for _, item := range report.NotebookUsage {
		usage[item.NotebookID] = item
	}
	if got := usage[DefaultNotebookID]; got.DocumentCount != 1 || got.ReferenceCount != 1 || got.ResourceCount != 1 ||
		got.UniqueBlobCount != 1 || got.ReferencedBytes != first.SizeBytes || got.UniqueBytes != first.SizeBytes {
		t.Fatalf("Notes usage: %+v", got)
	}
	if got := usage[images.ID]; got.DocumentCount != 1 || got.ReferenceCount != 1 || got.ResourceCount != 1 ||
		got.UniqueBlobCount != 1 || got.UniqueBytes != second.SizeBytes {
		t.Fatalf("Images usage: %+v", got)
	}
	if report.Perceptual.HookEnabled || report.Perceptual.StoredHashes != 0 ||
		len(report.Perceptual.PolicyReviews) != 0 || len(report.Perceptual.NearDuplicates) != 0 {
		t.Fatalf("default perceptual report must be inert: %+v", report.Perceptual)
	}
}

type testPerceptualHook struct {
	computes int
}

func (h *testPerceptualHook) Algorithm() string { return "test-dhash" }

func (h *testPerceptualHook) SupportsMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func (h *testPerceptualHook) Compute(_ context.Context, content io.Reader, _ string) (string, error) {
	h.computes++
	data, err := io.ReadAll(content)
	if err != nil {
		return "", err
	}
	switch string(data) {
	case "image-a":
		return "aaaa", nil
	case "image-b":
		return "aaab", nil
	default:
		return "ffff", nil
	}
}

func (h *testPerceptualHook) SuggestNearDuplicates(_ context.Context, candidates []PerceptualHashCandidate) ([]PerceptualHashSuggestion, error) {
	if len(candidates) != 2 {
		return nil, nil
	}
	return []PerceptualHashSuggestion{{
		LeftBlobSHA256:  candidates[1].BlobSHA256,
		RightBlobSHA256: candidates[0].BlobSHA256,
		Distance:        1,
		Reason:          "test-only near match",
	}}, nil
}

func TestSQLiteStorePerceptualHookStoresReviewsButNeverDeduplicates(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	hook := &testPerceptualHook{}
	if err := st.SetPerceptualHashHook(hook); err != nil {
		t.Fatalf("SetPerceptualHashHook: %v", err)
	}
	if err := st.AddMediaHashRule(ctx, MediaHashRule{
		Algo: "test-dhash", Hash: "aaaa", Kind: "perceptual", Action: "review", Reason: "review list",
	}); err != nil {
		t.Fatalf("AddMediaHashRule: %v", err)
	}

	first, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_a", Filename: "a.png", MIMEType: "image/png", Content: strings.NewReader("image-a"),
	})
	if err != nil {
		t.Fatalf("CreateResource first: %v", err)
	}
	second, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_b", Filename: "b.png", MIMEType: "image/png", Content: strings.NewReader("image-b"),
	})
	if err != nil {
		t.Fatalf("CreateResource second: %v", err)
	}
	if first.SHA256 == second.SHA256 {
		t.Fatal("perceptual similarity must never collapse distinct exact blobs")
	}

	report, err := st.ResourceReport(ctx)
	if err != nil {
		t.Fatalf("ResourceReport: %v", err)
	}
	if !report.Perceptual.HookEnabled || report.Perceptual.Algorithm != "test-dhash" || report.Perceptual.StoredHashes != 2 {
		t.Fatalf("perceptual state: %+v", report.Perceptual)
	}
	if len(report.Perceptual.PolicyReviews) != 1 || report.Perceptual.PolicyReviews[0].ResourceIDs[0] != first.ID {
		t.Fatalf("policy reviews: %+v", report.Perceptual.PolicyReviews)
	}
	if len(report.Perceptual.NearDuplicates) != 1 {
		t.Fatalf("near duplicates: %+v", report.Perceptual.NearDuplicates)
	}
	near := report.Perceptual.NearDuplicates[0]
	if near.Distance != 1 || near.Reason != "test-only near match" {
		t.Fatalf("near duplicate details: %+v", near)
	}
	if hook.computes != 2 {
		t.Fatalf("expected one hook computation per unique blob, got %d", hook.computes)
	}

	duplicate, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_a_again", Filename: "a-again.png", MIMEType: "image/png", Content: strings.NewReader("image-a"),
	})
	if err != nil {
		t.Fatalf("CreateResource exact duplicate: %v", err)
	}
	if duplicate.SHA256 != first.SHA256 || hook.computes != 2 {
		t.Fatalf("stored perceptual hash should be reused for exact duplicate: duplicate=%+v computes=%d", duplicate, hook.computes)
	}
}

func TestPerceptualHashRulesCannotBlock(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	err = st.AddMediaHashRule(context.Background(), MediaHashRule{
		Algo: "test-dhash", Hash: "aaaa", Kind: "perceptual", Action: "block",
	})
	if err == nil {
		t.Fatal("perceptual block rule must be rejected")
	}
}
