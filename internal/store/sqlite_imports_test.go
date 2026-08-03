package store

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"
)

func TestJ3ImportDocumentBatchIsAtomicAndPreservesCanonicalIndexes(t *testing.T) {
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
	if _, _, err := st.UpsertTag(ctx, "tag_joplin_topic", "topic"); err != nil {
		t.Fatal(err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_import", Filename: "file.bin", Content: bytes.NewBufferString("resource")})
	if err != nil {
		t.Fatal(err)
	}

	checkpoint := ImportCheckpoint{
		SourceSystem: "joplin_raw", SourceKey: "batch-source", CollectionID: "default",
		InventoryFingerprint: "inventory-one", Phase: "notes", NextIndex: 2,
		TotalItems: 4, ProcessedItems: 2, Status: "running", ReportJSON: `{"notes_imported":2}`,
	}
	state := func(key, target string) ImportItemState {
		return ImportItemState{SourceSystem: "joplin_raw", SourceKey: "batch-source", CollectionID: "default", ItemKey: key, ItemType: "1", Fingerprint: key + "-sha", TargetID: target, Action: "create"}
	}
	request := ImportDocumentBatchRequest{Checkpoint: checkpoint, Documents: []ImportDocumentMutation{
		{
			Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_import_target", Title: "Target", Body: "target searchable", Message: "batch import"},
			Source: SetDocumentSourceRequest{DocumentID: "doc_import_target", SourceSystem: "joplin", ExternalID: "target"},
			State:  state("1:target:target.md", "doc_import_target"),
		},
		{
			Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_import_source", Title: "Source", Body: "[target](document://default/documents/doc_import_target) batch searchable", Message: "batch import"},
			Source:    SetDocumentSourceRequest{DocumentID: "doc_import_source", SourceSystem: "joplin", ExternalID: "source"},
			AddTags:   []string{"topic"},
			Resources: []AttachResourceRequest{{DocumentID: "doc_import_source", ResourceID: resource.ID, RelationType: "referenced"}},
			State:     state("1:source:source.md", "doc_import_source"),
		},
	}}
	if err := st.ApplyImportDocumentBatch(ctx, request); err != nil {
		t.Fatal(err)
	}
	document, err := st.GetDocument(ctx, "doc_import_source")
	if err != nil {
		t.Fatal(err)
	}
	if revisions, err := st.ListDocumentRevisions(ctx, document.ID); err != nil || len(revisions) != 1 {
		t.Fatalf("revisions=%#v err=%v", revisions, err)
	}
	if source, err := st.GetDocumentSource(ctx, document.ID); err != nil || source.ExternalID != "source" {
		t.Fatalf("source=%+v err=%v", source, err)
	}
	if tags, err := st.ListDocumentTags(ctx, document.ID); err != nil || len(tags) != 1 || tags[0].Name != "topic" {
		t.Fatalf("tags=%#v err=%v", tags, err)
	}
	if refs, err := st.ListDocumentResources(ctx, document.ID); err != nil || len(refs) != 1 || refs[0].ResourceID != resource.ID {
		t.Fatalf("refs=%#v err=%v", refs, err)
	}
	links, err := st.ListDocumentLinks(ctx, document.ID, "outgoing")
	if err != nil || len(links.Outgoing) != 1 || links.Outgoing[0].TargetDocumentID != "doc_import_target" || links.Outgoing[0].ResolutionStatus != "resolved" {
		t.Fatalf("links=%#v err=%v", links, err)
	}
	if search, err := st.Search(ctx, SearchRequest{Query: "batch searchable"}); err != nil || len(search.Hits) != 1 || search.Hits[0].ID != document.ID {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	if jobs, err := st.PendingProjectionJobs(ctx, 10); err != nil || len(jobs) != 2 {
		t.Fatalf("outbox=%#v err=%v", jobs, err)
	}
	if saved, err := st.GetImportCheckpoint(ctx, "joplin_raw", "batch-source", "default"); err != nil || saved.NextIndex != 2 {
		t.Fatalf("checkpoint=%+v err=%v", saved, err)
	}

	checkpoint.NextIndex = 3
	checkpoint.ProcessedItems = 3
	checkpoint.ReportJSON = `{"notes_updated":1}`
	request = ImportDocumentBatchRequest{Checkpoint: checkpoint, Documents: []ImportDocumentMutation{{
		Action: "update", Document: CreateDocumentRequest{PreferredID: document.ID, Title: "Source updated", Body: "fresh indexed phrase", Message: "batch update"},
		BaseRevisionID: document.CurrentRevisionID,
		Source:         SetDocumentSourceRequest{DocumentID: document.ID, SourceSystem: "joplin", ExternalID: "source"},
		RemoveTags:     []string{"topic"},
		State:          ImportItemState{SourceSystem: "joplin_raw", SourceKey: "batch-source", CollectionID: "default", ItemKey: "1:source:source.md", ItemType: "1", Fingerprint: "updated-sha", TargetID: document.ID, Action: "update"},
	}}}
	if err := st.ApplyImportDocumentBatch(ctx, request); err != nil {
		t.Fatal(err)
	}
	if revisions, err := st.ListDocumentRevisions(ctx, document.ID); err != nil || len(revisions) != 2 {
		t.Fatalf("updated revisions=%#v err=%v", revisions, err)
	}
	if old, err := st.Search(ctx, SearchRequest{Query: "batch searchable"}); err != nil || len(old.Hits) != 0 {
		t.Fatalf("old FTS content remained: search=%#v err=%v", old, err)
	}
	if fresh, err := st.Search(ctx, SearchRequest{Query: "fresh indexed phrase"}); err != nil || len(fresh.Hits) != 1 || fresh.Hits[0].ID != document.ID {
		t.Fatalf("updated FTS missing: search=%#v err=%v", fresh, err)
	}
	if tags, err := st.ListDocumentTags(ctx, document.ID); err != nil || len(tags) != 0 {
		t.Fatalf("removed tags=%#v err=%v", tags, err)
	}
	metrics, err := st.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Documents != 2 || metrics.Revisions != 3 || metrics.FTSRows != 2 || metrics.Sources != 2 ||
		metrics.ResourceRelations != 1 || metrics.PendingOutbox != 3 || metrics.ImportStates != 2 ||
		metrics.DatabaseBytes == 0 || metrics.JournalMode != "wal" || metrics.ForeignKeys != 1 {
		t.Fatalf("import metrics=%+v", metrics)
	}
	updated, err := st.GetDocument(ctx, document.ID)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint.NextIndex = 4
	checkpoint.ProcessedItems = 4
	checkpoint.ReportJSON = `{"notes_unchanged":1}`
	if err := st.ApplyImportDocumentBatch(ctx, ImportDocumentBatchRequest{Checkpoint: checkpoint, Documents: []ImportDocumentMutation{{
		Action: "unchanged", Document: CreateDocumentRequest{PreferredID: updated.ID, Title: updated.Title, Body: updated.Body},
		SkipSource: true, SkipState: true,
	}}}); err != nil {
		t.Fatal(err)
	}
	unchangedStates, err := st.GetImportItemStates(ctx, "joplin_raw", "batch-source", "default", []string{"1:source:source.md"})
	unchangedState := unchangedStates["1:source:source.md"]
	if err != nil || unchangedState.Action != "update" || unchangedState.Fingerprint != "updated-sha" {
		t.Fatalf("skipped state changed=%+v err=%v", unchangedState, err)
	}
	if revisions, err := st.ListDocumentRevisions(ctx, document.ID); err != nil || len(revisions) != 2 {
		t.Fatalf("unchanged revisions=%#v err=%v", revisions, err)
	}

	failedCheckpoint := checkpoint
	failedCheckpoint.NextIndex = 5
	err = st.ApplyImportDocumentBatch(ctx, ImportDocumentBatchRequest{Checkpoint: failedCheckpoint, Documents: []ImportDocumentMutation{
		{
			Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_rolled_back", Title: "Must roll back"},
			Source: SetDocumentSourceRequest{DocumentID: "doc_rolled_back", SourceSystem: "joplin", ExternalID: "rolled-back"},
			State:  state("1:rolled:rolled.md", "doc_rolled_back"),
		},
		{
			Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_invalid", NotebookID: "nb_missing", Title: "Invalid"},
			Source: SetDocumentSourceRequest{DocumentID: "doc_invalid", SourceSystem: "joplin", ExternalID: "invalid"},
			State:  state("1:invalid:invalid.md", "doc_invalid"),
		},
	}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("rollback error=%v", err)
	}
	if _, err := st.GetDocument(ctx, "doc_rolled_back"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial document committed: %v", err)
	}
	if saved, err := st.GetImportCheckpoint(ctx, "joplin_raw", "batch-source", "default"); err != nil || saved.NextIndex != 4 {
		t.Fatalf("failed batch advanced checkpoint: %+v err=%v", saved, err)
	}
}

func TestJ3TemporaryImportManifestBatchesAndIndexesRelations(t *testing.T) {
	ctx := context.Background()
	manifest, err := OpenImportManifest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manifest.Close() })
	records := []ImportManifestRecord{
		{Kind: "notes", SortKey: "b.md", LookupKey: "b", Payload: []byte(`{"id":"b"}`)},
		{Kind: "notes", SortKey: "a.md", LookupKey: "a", Payload: []byte(`{"id":"a"}`)},
		{Kind: "note_tags", SortKey: "tag-2", LookupKey: "a", Payload: []byte("tag-2")},
		{Kind: "note_tags", SortKey: "tag-1", LookupKey: "a", Payload: []byte("tag-1")},
	}
	if err := manifest.Put(ctx, records); err != nil {
		t.Fatal(err)
	}
	batch, err := manifest.Batch(ctx, "notes", 0, 1)
	if err != nil || len(batch) != 1 || batch[0].LookupKey != "a" {
		t.Fatalf("ordered batch=%#v err=%v", batch, err)
	}
	next, err := manifest.BatchAfter(ctx, "notes", batch[0].SortKey, 2)
	if err != nil || len(next) != 1 || next[0].LookupKey != "b" {
		t.Fatalf("keyset batch=%#v err=%v", next, err)
	}
	relations, err := manifest.Lookup(ctx, "note_tags", []string{"a", "missing"})
	if err != nil || len(relations["a"]) != 2 || string(relations["a"][0].Payload) != "tag-1" {
		t.Fatalf("relations=%#v err=%v", relations, err)
	}
	if manifest.SizeBytes() == 0 {
		t.Fatal("manifest size was not reported")
	}
}

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
	if err != nil || status.SchemaVersion != 11 {
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
