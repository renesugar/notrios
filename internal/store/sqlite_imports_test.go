package store

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSchemaV10ImportStateBatchLookupsAndSourceBundle(t *testing.T) {
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
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != 10 {
		t.Fatalf("schema status=%+v err=%v", status, err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_batch", Title: "Batch", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_batch", Filename: "batch.bin", Content: bytes.NewBufferString("resource"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.UpsertTag(ctx, "tag_source", "source-tag"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDocumentTag(ctx, doc.ID, "source-tag"); err != nil {
		t.Fatal(err)
	}
	documents, err := st.GetDocuments(ctx, []string{"missing", doc.ID})
	if err != nil || documents[doc.ID].Title != "Batch" {
		t.Fatalf("batch documents=%+v err=%v", documents, err)
	}
	resources, err := st.GetResources(ctx, []string{resource.ID, "missing"})
	if err != nil || resources[resource.ID].SHA256 == "" {
		t.Fatalf("batch resources=%+v err=%v", resources, err)
	}
	tags, err := st.GetDocumentTags(ctx, []string{doc.ID})
	if err != nil || len(tags[doc.ID]) != 1 || tags[doc.ID][0].Name != "source-tag" {
		t.Fatalf("batch tags=%+v err=%v", tags, err)
	}

	checkpoint := ImportCheckpoint{
		SourceSystem: "joplin_raw", SourceKey: "/source", CollectionID: "default",
		InventoryFingerprint: "inventory-sha", Phase: "notes", NextIndex: 200,
		TotalItems: 1000, ProcessedItems: 400, Status: "running", ReportJSON: `{"notes_imported":200}`,
	}
	if err := st.PutImportCheckpoint(ctx, checkpoint); err != nil {
		t.Fatal(err)
	}
	storedCheckpoint, err := st.GetImportCheckpoint(ctx, "joplin_raw", "/source", "default")
	if err != nil || storedCheckpoint.Phase != "notes" || storedCheckpoint.NextIndex != 200 ||
		storedCheckpoint.ReportJSON != checkpoint.ReportJSON {
		t.Fatalf("checkpoint=%+v err=%v", storedCheckpoint, err)
	}
	states := []ImportItemState{
		{SourceSystem: "joplin_raw", SourceKey: "/source", CollectionID: "default", ItemKey: "1:a:a.md", ItemType: "1", Fingerprint: "a", TargetID: doc.ID, Action: "create"},
		{SourceSystem: "joplin_raw", SourceKey: "/source", CollectionID: "default", ItemKey: "4:b:b.md", ItemType: "4", Fingerprint: "b", TargetID: resource.ID, Action: "create"},
	}
	if err := st.PutImportItemStates(ctx, states); err != nil {
		t.Fatal(err)
	}
	storedStates, err := st.GetImportItemStates(ctx, "joplin_raw", "/source", "default", []string{"4:b:b.md", "missing"})
	if err != nil || storedStates["4:b:b.md"].TargetID != resource.ID {
		t.Fatalf("states=%+v err=%v", storedStates, err)
	}

	exact := []byte("body\r\n\r\nunknown: first\r\ntitle: second\r\ntype_: 1\r\n")
	order := []string{"unknown", "title", "type_"}
	bundle, err := st.PutSourceBundleItem(ctx, PutSourceBundleItemRequest{
		SourceSystem: "joplin_raw", SourceKey: "/source", CollectionID: "default",
		ItemKey: "1:a:a.md", ItemType: "1", ExternalID: "a", RelativePath: "a.md",
		PropertyOrder: order, Content: bytes.NewReader(exact),
	})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.StoragePath == "" || bundle.SHA256 == resource.SHA256 {
		t.Fatalf("source bundle must have its own content address: %+v", bundle)
	}
	opened, reader, err := st.OpenSourceBundleItem(ctx, "joplin_raw", "/source", "default", "1:a:a.md")
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("bundle read=%v close=%v", readErr, closeErr)
	}
	if !bytes.Equal(got, exact) || !reflect.DeepEqual(opened.PropertyOrder, order) {
		t.Fatalf("bundle bytes/order changed: item=%+v bytes=%q", opened, got)
	}
	report, err := st.ResourceReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.UnreferencedBlobs) != 1 || report.UnreferencedBlobs[0].SHA256 != resource.SHA256 {
		t.Fatalf("source bundle leaked into resource report: %+v", report.UnreferencedBlobs)
	}
}
