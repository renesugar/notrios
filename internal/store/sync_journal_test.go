package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func newSyncJournalTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return st
}

func syncCount(t *testing.T, st *SQLiteStore, query string, values ...string) int64 {
	t.Helper()
	count, err := st.syncJournalCountForTest(query, values...)
	if err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return count
}

func syncText(t *testing.T, st *SQLiteStore, query string, values ...string) string {
	t.Helper()
	value, err := st.syncJournalTextForTest(query, values...)
	if err != nil {
		t.Fatalf("read %q: %v", query, err)
	}
	return value
}

func TestSyncJournalDisabledUntilExplicitEnrollment(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_before", Title: "before", Body: "not journaled"}); err != nil {
		t.Fatalf("CreateDocument before enrollment: %v", err)
	}
	status, err := st.JournalStatus(ctx)
	if err != nil {
		t.Fatalf("JournalStatus: %v", err)
	}
	if status.Enabled || syncCount(t, st, `SELECT COUNT(*) FROM sync_operations`) != 0 {
		t.Fatalf("journal must be empty before enrollment: %+v", status)
	}

	status, err = st.EnrollLocalJournal(ctx, "test enrollment")
	if err != nil {
		t.Fatalf("EnrollLocalJournal: %v", err)
	}
	if !status.Enabled || status.ReplicaID == "" || status.SnapshotBoundaryID == "" || status.LastSequence != 0 {
		t.Fatalf("unexpected enrollment status: %+v", status)
	}
	if syncCount(t, st, `SELECT COUNT(*) FROM sync_snapshot_boundaries`) != 1 {
		t.Fatal("enrollment must create exactly one snapshot boundary")
	}
	if got := syncText(t, st, `SELECT state_vector_json FROM sync_snapshot_boundaries WHERE id = ?`, status.SnapshotBoundaryID); got != fmt.Sprintf(`{"%s":0}`, status.ReplicaID) {
		t.Fatalf("boundary vector = %q", got)
	}
	if again, err := st.EnrollLocalJournal(ctx, "idempotent replay"); err != nil || again.SnapshotBoundaryID != status.SnapshotBoundaryID {
		t.Fatalf("enrollment must be idempotent: %+v err=%v", again, err)
	}
	if syncCount(t, st, `SELECT COUNT(*) FROM sync_snapshot_boundaries`) != 1 {
		t.Fatal("idempotent enrollment created another boundary")
	}
}

func TestSyncJournalSequencesCanonicalMutationsAndExcludesDerivedRows(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	status, err := st.EnrollLocalJournal(ctx, "sequence test")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_seq", Title: "one", Body: "body"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if _, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: doc.ID, Title: "two", Body: "body two", BaseRevisionID: doc.CurrentRevisionID,
	}); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
	operations, err := st.ListLocalOperations(ctx, 0, 100)
	if err != nil {
		t.Fatalf("ListLocalOperations: %v", err)
	}
	if len(operations) != 4 {
		t.Fatalf("create/update should capture document+revision each, got %d: %+v", len(operations), operations)
	}
	for i, operation := range operations {
		wantSequence := int64(i + 1)
		if operation.ReplicaID != status.ReplicaID || operation.Sequence != wantSequence {
			t.Fatalf("operation %d identity/sequence: %+v", i, operation)
		}
		if operation.OperationID != fmt.Sprintf("%s:%020d", status.ReplicaID, wantSequence) {
			t.Fatalf("operation id %q", operation.OperationID)
		}
		if !json.Valid([]byte(operation.PayloadJSON)) {
			t.Fatalf("operation %d has invalid JSON: %q", i, operation.PayloadJSON)
		}
	}
	if operations[0].RecordType != "document" || operations[1].RecordType != "revision" ||
		operations[2].RecordType != "revision" || operations[3].RecordType != "document" {
		t.Fatalf("unexpected operation order/types: %+v", operations)
	}

	before := len(operations)
	if err := st.RebuildDocumentLinks(ctx, doc.ID); err != nil {
		t.Fatalf("RebuildDocumentLinks: %v", err)
	}
	if err := st.RebuildDocumentBlocks(ctx, doc.ID); err != nil {
		t.Fatalf("RebuildDocumentBlocks: %v", err)
	}
	after, err := st.ListLocalOperations(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != before {
		t.Fatalf("derived rebuilds entered journal: before=%d after=%d", before, len(after))
	}
	if syncCount(t, st, `SELECT COUNT(*) FROM sync_journal_capture`) != 0 {
		t.Fatal("transient capture seam retained rows")
	}
	if got := syncCount(t, st, `SELECT contiguous_sequence FROM sync_state_vectors WHERE observer_replica_id = ? AND subject_replica_id = ?`, status.ReplicaID, status.ReplicaID); got != 4 {
		t.Fatalf("local state vector = %d, want 4", got)
	}
}

func TestSyncJournalCanonicalVocabularyThroughStoreWrites(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	if _, err := st.EnrollLocalJournal(ctx, "vocabulary test"); err != nil {
		t.Fatal(err)
	}
	nb, err := st.CreateNotebook(ctx, CreateNotebookRequest{PreferredID: "nb_sync", Name: "Sync"})
	if err != nil {
		t.Fatal(err)
	}
	icon := "S"
	if _, err := st.UpdateNotebook(ctx, UpdateNotebookRequest{ID: nb.ID, IconEmoji: &icon}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateSearchNotebook(ctx, CreateSearchNotebookRequest{PreferredID: "snb_sync", Name: "Sync query", Query: "tag:sync"}); err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_vocab", Title: "Vocabulary", Body: "body", NotebookID: nb.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetDocumentSource(ctx, SetDocumentSourceRequest{DocumentID: doc.ID, SourceSystem: "test", ExternalID: "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutSourceBundleItem(ctx, PutSourceBundleItemRequest{
		SourceSystem: "test", SourceKey: "bundle", CollectionID: "default",
		ItemKey: "note.md", ItemType: "note", ExternalID: "one", RelativePath: "note.md",
		PropertyOrder: []string{"title"}, Content: strings.NewReader("exact source"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDocumentTag(ctx, doc.ID, "sync"); err != nil {
		t.Fatal(err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_sync", CollectionID: "default", Filename: "a.bin",
		MIMEType: "application/octet-stream", Content: strings.NewReader("one"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: doc.ID, ResourceID: resource.ID, RelationType: "attachment"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateResource(ctx, UpdateResourceRequest{ID: resource.ID, Filename: "b.bin", MIMEType: "application/octet-stream", Content: strings.NewReader("two")}); err != nil {
		t.Fatal(err)
	}
	if err := st.DetachDocumentResource(ctx, doc.ID, resource.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteResource(ctx, resource.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveDocumentTag(ctx, doc.ID, "sync"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSearchNotebook(ctx, "snb_sync"); err != nil {
		t.Fatal(err)
	}

	operations, err := st.ListLocalOperations(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, operation := range operations {
		seen[operation.RecordType] = true
	}
	for _, recordType := range []string{
		"notebook", "search_notebook", "document", "revision", "document_source",
		"tag", "document_tag", "resource", "document_resource", "source_bundle_item",
	} {
		if !seen[recordType] {
			t.Errorf("record type %q was not captured; seen=%v", recordType, seen)
		}
	}
}

func TestSyncJournalAtomicBatchAndImportRollbackLeaveNoOperations(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_rollback", Title: "rollback", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	destination, err := st.CreateNotebook(ctx, CreateNotebookRequest{PreferredID: "nb_destination", Name: "Destination"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollLocalJournal(ctx, "rollback test"); err != nil {
		t.Fatal(err)
	}

	result, err := st.RunBatch(ctx, BatchRequest{
		Mode: BatchModeAtomic, Operation: BatchOpMove, NotebookID: destination.ID,
		Items: []BatchItem{{DocumentID: doc.ID}, {DocumentID: "doc_missing"}},
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if result.RolledBack != 1 || result.Failed != 1 {
		t.Fatalf("unexpected batch result: %+v", result)
	}
	if operations, _ := st.ListLocalOperations(ctx, 0, 100); len(operations) != 0 {
		t.Fatalf("rolled-back batch retained operations: %+v", operations)
	}
	current, err := st.GetDocument(ctx, doc.ID)
	if err != nil || current.NotebookID != DefaultNotebookID {
		t.Fatalf("rolled-back document moved: %+v err=%v", current, err)
	}

	err = st.ApplyImportDocumentBatch(ctx, ImportDocumentBatchRequest{Documents: []ImportDocumentMutation{
		{Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_import_first", Title: "first"}},
		{Action: "create", Document: CreateDocumentRequest{PreferredID: "doc_import_bad", Title: "bad", NotebookID: "nb_missing"}},
	}})
	if err == nil {
		t.Fatal("invalid import batch unexpectedly succeeded")
	}
	if operations, _ := st.ListLocalOperations(ctx, 0, 100); len(operations) != 0 {
		t.Fatalf("rolled-back import retained operations: %+v", operations)
	}
	if _, err := st.GetDocument(ctx, "doc_import_first"); err != ErrNotFound {
		t.Fatalf("first import row survived rollback: %v", err)
	}
}

func TestSyncJournalRestoreBatchCommitsAndRollsBackWithOperations(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	if _, err := st.EnrollLocalJournal(ctx, "restore batch test"); err != nil {
		t.Fatal(err)
	}
	if conflicts, err := st.ApplyRestoreRecords(ctx, RestoreRecords{Collections: []ExportCollection{{
		ID: "collection_restored", Name: "Restored",
	}}}, false); err != nil || len(conflicts) != 0 {
		t.Fatalf("restore collection: conflicts=%+v err=%v", conflicts, err)
	}
	operations, err := st.ListLocalOperations(ctx, 0, 10)
	if err != nil || len(operations) != 1 || operations[0].RecordType != "collection" {
		t.Fatalf("restore operations=%+v err=%v", operations, err)
	}
	before := len(operations)
	_, err = st.ApplyRestoreRecords(ctx, RestoreRecords{
		Collections: []ExportCollection{{ID: "collection_rolled_back", Name: "Rollback"}},
		Notebooks:   []Notebook{{ID: "nb_invalid_restore", ParentID: "nb_missing", Name: "Invalid"}},
	}, false)
	if err == nil {
		t.Fatal("invalid restore batch unexpectedly succeeded")
	}
	operations, listErr := st.ListLocalOperations(ctx, 0, 10)
	if listErr != nil || len(operations) != before {
		t.Fatalf("restore rollback operations=%+v err=%v", operations, listErr)
	}
	if got := syncCount(t, st, `SELECT COUNT(*) FROM collections WHERE id = 'collection_rolled_back'`); got != 0 {
		t.Fatalf("restore rollback retained collection: %d", got)
	}
}

func TestSyncJournalIdentityRotationRetiresBoundaryAndRequiresReenrollment(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	first, err := st.EnrollLocalJournal(ctx, "first identity")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_old_replica", Title: "old"}); err != nil {
		t.Fatal(err)
	}
	rotated, err := st.RotateReplicaIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.ReplicaID == first.ReplicaID {
		t.Fatal("replica identity did not rotate")
	}
	status, err := st.JournalStatus(ctx)
	if err != nil || status.Enabled {
		t.Fatalf("rotation must disable old allocator: %+v err=%v", status, err)
	}
	if state := syncText(t, st, `SELECT status FROM sync_replicas WHERE replica_id = ?`, first.ReplicaID); state != "retired" {
		t.Fatalf("old replica state = %q", state)
	}
	second, err := st.EnrollLocalJournal(ctx, "new identity")
	if err != nil {
		t.Fatal(err)
	}
	if second.ReplicaID != rotated.ReplicaID || second.LastSequence != 0 || second.SnapshotBoundaryID == first.SnapshotBoundaryID {
		t.Fatalf("unexpected reenrollment: first=%+v second=%+v identity=%+v", first, second, rotated)
	}
}

func TestSyncJournalCloseRollsBackUncommittedCanonicalAndOperationRows(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(root, "notes.sqlite")
	st, err := OpenSQLiteWithAssetStore(dbPath, filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollLocalJournal(ctx, "crash rollback test"); err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	if err := st.execLocked("BEGIN IMMEDIATE"); err != nil {
		st.mu.Unlock()
		t.Fatal(err)
	}
	if err := st.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type)
		VALUES('doc_uncommitted', 'default', ?, 'uncommitted', 'text/markdown')`, DefaultNotebookID); err != nil {
		st.mu.Unlock()
		t.Fatal(err)
	}
	st.mu.Unlock()
	if err := st.Close(); err != nil {
		t.Fatalf("Close with transaction: %v", err)
	}

	reopened, err := OpenSQLiteWithAssetStore(dbPath, filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if got := syncCount(t, reopened, `SELECT COUNT(*) FROM documents WHERE id = 'doc_uncommitted'`); got != 0 {
		t.Fatalf("uncommitted canonical row survived close: %d", got)
	}
	status, err := reopened.JournalStatus(ctx)
	if err != nil || status.LastSequence != 0 {
		t.Fatalf("uncommitted operation survived close: %+v err=%v", status, err)
	}
}

func TestSyncJournalCapturesTrashRestorePurgeAndStructuralChanges(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	nb, err := st.CreateNotebook(ctx, CreateNotebookRequest{PreferredID: "nb_lifecycle", Name: "Lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_lifecycle", Title: "Lifecycle", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollLocalJournal(ctx, "lifecycle test"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.MoveDocumentToNotebook(ctx, doc.ID, nb.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RestoreDocument(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := st.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: restored.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	if err := st.PurgeDocument(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteNotebook(ctx, nb.ID); err != nil {
		t.Fatal(err)
	}
	operations, err := st.ListLocalOperations(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, operation := range operations {
		seen[operation.Kind]++
	}
	for _, kind := range []string{"record.update", "document.trash", "document.restore", "document.purge", "record.delete"} {
		if seen[kind] == 0 {
			t.Errorf("missing lifecycle operation %q: %v", kind, seen)
		}
	}
}

func TestSchemaV18UpgradeCreatesCompleteReplicationTables(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	st.mu.Lock()
	for _, statement := range []string{
		`DROP TABLE sync_peer_compatibility`,
		`DROP TABLE sync_journal_capture`,
		`DROP TABLE sync_audit_events`,
		`DROP TABLE sync_pending_admissions`,
		`DROP TABLE sync_peer_acknowledgements`,
		`DROP TABLE sync_state_gaps`,
		`DROP TABLE sync_state_vectors`,
		`DROP TABLE sync_operation_dependencies`,
		`DROP TABLE sync_operations`,
		`DROP TABLE sync_local_journal`,
		`DROP TABLE sync_snapshot_boundaries`,
		`DROP TABLE sync_replicas`,
		`PRAGMA user_version = 18`,
	} {
		if err := st.execLocked(statement); err != nil {
			st.mu.Unlock()
			t.Fatalf("prepare v18 fixture %q: %v", statement, err)
		}
	}
	st.mu.Unlock()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap upgrade: %v", err)
	}
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema status: %+v err=%v", status, err)
	}
	for _, table := range []string{
		"sync_replicas", "sync_snapshot_boundaries", "sync_local_journal",
		"sync_operations", "sync_operation_dependencies", "sync_state_vectors",
		"sync_state_gaps", "sync_peer_acknowledgements", "sync_pending_admissions",
		"sync_audit_events", "sync_journal_capture", "sync_peer_compatibility",
	} {
		if got := syncCount(t, st, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table); got != 1 {
			t.Errorf("table %s count = %d", table, got)
		}
	}
	if got := syncCount(t, st, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'sync_capture_%'`); got < 20 {
		t.Fatalf("canonical capture trigger count = %d", got)
	}
}

func TestSyncRecordClassificationTableIsCompleteAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, classification := range SyncRecordClassifications() {
		if classification.RecordType == "" || classification.Class == "" || len(classification.Mutations) == 0 {
			t.Fatalf("incomplete classification: %+v", classification)
		}
		if seen[classification.RecordType] {
			t.Fatalf("duplicate classification for %q", classification.RecordType)
		}
		seen[classification.RecordType] = true
	}
}

func TestSyncJournalCaptureTriggerMatrix(t *testing.T) {
	st := newSyncJournalTestStore(t)
	matrix := map[string][]string{
		"collection":         {"collections_insert", "collections_update", "collections_delete"},
		"document":           {"documents_insert", "documents_update", "documents_delete"},
		"revision":           {"revisions_insert"},
		"notebook":           {"notebooks_insert", "notebooks_update", "notebooks_delete"},
		"tag":                {"tags_insert", "tags_update", "tags_delete"},
		"document_tag":       {"note_tags_insert", "note_tags_delete"},
		"search_notebook":    {"search_notebooks_insert", "search_notebooks_update", "search_notebooks_delete"},
		"resource":           {"resources_insert", "resources_update", "resources_delete"},
		"document_resource":  {"resource_refs_insert", "resource_refs_update", "resource_refs_delete"},
		"document_source":    {"sources_insert", "sources_update", "sources_delete"},
		"source_bundle_item": {"source_bundles_insert", "source_bundles_update", "source_bundles_delete"},
	}
	classified := map[string]bool{}
	for _, item := range SyncRecordClassifications() {
		classified[item.RecordType] = true
	}
	for recordType, suffixes := range matrix {
		if !classified[recordType] {
			t.Errorf("triggered record %q has no classification", recordType)
		}
		for _, suffix := range suffixes {
			name := "sync_capture_" + suffix
			if got := syncCount(t, st, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?`, name); got != 1 {
				t.Errorf("trigger %s count = %d", name, got)
			}
		}
	}
	for recordType := range classified {
		if _, ok := matrix[recordType]; !ok {
			t.Errorf("classified record %q has no trigger matrix entry", recordType)
		}
	}
}
