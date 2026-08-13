package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
)

func newMetadataReceiver(t *testing.T, databaseID string) (*SQLiteStore, Document) {
	t.Helper()
	ctx := context.Background()
	st := newSyncJournalTestStore(t)
	if _, err := st.AdoptDatabaseIdentity(ctx, databaseID); err != nil {
		t.Fatal(err)
	}
	for _, notebook := range []CreateNotebookRequest{{PreferredID: "nb_a", Name: "A"}, {PreferredID: "nb_b", Name: "B"}} {
		if _, err := st.CreateNotebook(ctx, notebook); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := st.UpsertTag(ctx, "tag_a", "alpha"); err != nil {
		t.Fatal(err)
	}
	document, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_a", NotebookID: "nb_a", Title: "base", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollLocalJournal(ctx, "G6 common baseline"); err != nil {
		t.Fatal(err)
	}
	return st, document
}

func metadataOperation(replica string, sequence, wall int64, kind, recordType, recordID, payload string) syncstate.Operation {
	return syncstate.Operation{
		ReplicaID: replica, Sequence: sequence, OperationID: fmt.Sprintf("%s:%020d", replica, sequence),
		Kind: kind, RecordType: recordType, RecordID: recordID, Payload: json.RawMessage(payload),
		HLC: syncstate.HLC{WallMS: wall}, CreatedAt: time.UnixMilli(wall).UTC().Format(time.RFC3339Nano),
	}
}

func configureMetadataPeers(t *testing.T, st *SQLiteStore, databaseID string, count int64) map[string]syncstate.Handshake {
	t.Helper()
	peers := map[string]syncstate.Handshake{}
	for _, replicaID := range []string{"replica_a", "replica_b"} {
		peer := syncstate.NewHandshake(databaseID, replicaID, CurrentSchemaVersion, syncstate.Vector{replicaID: count})
		if err := st.ConfigureSyncAdmissionPeer(context.Background(), peer); err != nil {
			t.Fatal(err)
		}
		peers[replicaID] = peer
	}
	return peers
}

func metadataOperationSets() map[string][]syncstate.Operation {
	return map[string][]syncstate.Operation{
		"replica_a": {
			metadataOperation("replica_a", 1, 100, "record.update", "document", "doc_a", `{"title":"remote title"}`),
			metadataOperation("replica_a", 2, 200, "membership.add", "document_tag", "doc_a:tag_a", `{"document_id":"doc_a","tag_id":"tag_a"}`),
			metadataOperation("replica_a", 3, 300, "record.update", "notebook", "nb_a", `{"parent_id":"nb_b","name":"Same"}`),
			metadataOperation("replica_a", 4, 400, "document.trash", "document", "doc_a", `{}`),
		},
		"replica_b": {
			metadataOperation("replica_b", 1, 150, "record.update", "document", "doc_a", `{"notebook_id":"nb_b"}`),
			metadataOperation("replica_b", 2, 250, "membership.remove", "document_tag", "doc_a:tag_a", `{"document_id":"doc_a","tag_id":"tag_a"}`),
			metadataOperation("replica_b", 3, 350, "record.update", "notebook", "nb_b", `{"parent_id":"nb_a","name":"Same"}`),
			metadataOperation("replica_b", 4, 450, "document.restore", "document", "doc_a", `{}`),
		},
	}
}

func metadataSnapshot(t *testing.T, st *SQLiteStore) string {
	t.Helper()
	queries := []string{
		`SELECT COALESCE(json_group_array(json_object('id',id,'collection',collection_id,'notebook',COALESCE(notebook_id,''),'title',title,'mime',body_mime_type,'deleted',COALESCE(deleted_at,''))), '[]') FROM (SELECT * FROM documents WHERE id='doc_a' ORDER BY id)`,
		`SELECT COALESCE(json_group_array(json_object('id',id,'parent',COALESCE(parent_id,''),'name',name)), '[]') FROM (SELECT * FROM notebooks WHERE id IN ('nb_a','nb_b') ORDER BY id)`,
		`SELECT COALESCE(json_group_array(document_id || ':' || tag_id), '[]') FROM (SELECT * FROM note_tags ORDER BY document_id, tag_id)`,
		`SELECT COALESCE(json_group_array(json_object('id',id,'type',event_type,'subject',subject_id,'details',json(details_json))), '[]') FROM (SELECT * FROM sync_repair_events ORDER BY id)`,
	}
	parts := make([]string, 0, len(queries))
	for _, query := range queries {
		parts = append(parts, syncText(t, st, query))
	}
	return strings.Join(parts, "\n")
}

func assertMetadataStoreInvariants(t *testing.T, st *SQLiteStore) {
	t.Helper()
	if got := syncCount(t, st, `SELECT COUNT(*) FROM pragma_foreign_key_check`); got != 0 {
		t.Fatalf("foreign-key violations = %d", got)
	}
	if got := syncCount(t, st, `SELECT COUNT(*) FROM documents WHERE id='doc_a' AND collection_id<>'' AND notebook_id<>''`); got != 1 {
		t.Fatalf("document home invariant count = %d", got)
	}
	for _, id := range []string{"nb_a", "nb_b"} {
		if _, err := st.GetNotebook(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSyncMetadataAdmissionConvergesCanonicalRowsAndRepairReport(t *testing.T) {
	const databaseID = "db_g6_convergence"
	first, _ := newMetadataReceiver(t, databaseID)
	second, _ := newMetadataReceiver(t, databaseID)
	firstPeers := configureMetadataPeers(t, first, databaseID, 4)
	secondPeers := configureMetadataPeers(t, second, databaseID, 4)
	sets := metadataOperationSets()
	for _, replicaID := range []string{"replica_a", "replica_b"} {
		if _, err := first.AdmitSyncOperations(context.Background(), firstPeers[replicaID], sets[replicaID]); err != nil {
			t.Fatalf("first %s: %v", replicaID, err)
		}
		assertMetadataStoreInvariants(t, first)
	}
	for _, replicaID := range []string{"replica_b", "replica_a"} {
		if _, err := second.AdmitSyncOperations(context.Background(), secondPeers[replicaID], sets[replicaID]); err != nil {
			t.Fatalf("second %s: %v", replicaID, err)
		}
		assertMetadataStoreInvariants(t, second)
	}
	if left, right := metadataSnapshot(t, first), metadataSnapshot(t, second); left != right {
		t.Fatalf("canonical projections diverged\nleft: %s\nright:%s", left, right)
	}
	document, err := first.GetDocument(context.Background(), "doc_a")
	if err != nil {
		t.Fatal(err)
	}
	if document.Title != "remote title" || document.NotebookID != "nb_b" || !document.DeletedAt.IsZero() {
		t.Fatalf("document = %+v", document)
	}
	if got := syncCount(t, first, `SELECT COUNT(*) FROM note_tags WHERE document_id='doc_a' AND tag_id='tag_a'`); got != 0 {
		t.Fatal("later membership remove did not win")
	}
	a, _ := first.GetNotebook(context.Background(), "nb_a")
	b, _ := first.GetNotebook(context.Background(), "nb_b")
	if b.ParentID != "nb_a" || a.ParentID != "" {
		t.Fatalf("notebook repair a=%+v b=%+v", a, b)
	}
	if got := syncCount(t, first, `SELECT COUNT(*) FROM sync_repair_events WHERE event_type='notebook.cycle'`); got != 1 {
		t.Fatalf("cycle repair events = %d", got)
	}
}

func TestLocalJournalAssignsMonotonicHLCAndSparseDocumentFields(t *testing.T) {
	ctx := context.Background()
	st, document := newMetadataReceiver(t, "db_g6_sparse")
	if _, err := st.UpdateDocument(ctx, UpdateDocumentRequest{ID: document.ID, Title: "changed", Body: "changed body", BodyMIMEType: document.BodyMIMEType, BaseRevisionID: document.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	operations, err := st.ListLocalOperations(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var update *SyncOperation
	for index := range operations {
		if operations[index].RecordType == "document" && operations[index].Kind == "record.update" {
			update = &operations[index]
		}
	}
	if update == nil {
		t.Fatalf("operations = %+v", operations)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(update.PayloadJSON), &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["title"]; !ok {
		t.Fatalf("sparse fields = %v", fields)
	}
	for _, untouched := range []string{"collection_id", "notebook_id"} {
		if _, ok := fields[untouched]; ok {
			t.Fatalf("untouched field %q was journaled: %v", untouched, fields)
		}
	}
	for index, operation := range operations {
		if operation.HLCWallMS <= 0 || operation.HLCLogical < 0 {
			t.Fatalf("operation %d HLC = %d/%d", index, operation.HLCWallMS, operation.HLCLogical)
		}
		if index > 0 {
			prior := operations[index-1]
			if operation.HLCWallMS < prior.HLCWallMS || operation.HLCWallMS == prior.HLCWallMS && operation.HLCLogical <= prior.HLCLogical {
				t.Fatalf("HLC moved backward: %+v then %+v", prior, operation)
			}
		}
	}
	if reflect.DeepEqual(fields, map[string]json.RawMessage{}) {
		t.Fatal("empty sparse update")
	}
}

func TestEnrolledLocalPurgeIsGatedUntilCryptographicSigning(t *testing.T) {
	st, document := newMetadataReceiver(t, "db_g6_purge_gate")
	ctx := context.Background()
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: document.ID, BaseRevisionID: document.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	if err := st.PurgeDocument(ctx, document.ID); err == nil || !strings.Contains(err.Error(), "signed death certificate") {
		t.Fatalf("purge error = %v", err)
	}
	if got := syncCount(t, st, `SELECT COUNT(*) FROM documents WHERE id=?`, document.ID); got != 1 {
		t.Fatalf("gated purge removed document: %d", got)
	}
}

func TestSchemaV21HasDurableConvergenceTables(t *testing.T) {
	st := newSyncJournalTestStore(t)
	status, err := st.Status(context.Background())
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("status = %+v, %v", status, err)
	}
	for _, table := range []string{"sync_hlc_clock", "sync_field_registers", "sync_lifecycle_registers", "sync_membership_registers", "sync_death_certificates", "sync_repair_events"} {
		if got := syncCount(t, st, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table); got != 1 {
			t.Fatalf("missing table %s", table)
		}
	}
}

func TestSyncMetadataRejectsBackwardReplicaHLCAtomically(t *testing.T) {
	const databaseID = "db_g6_hlc_backwards"
	st, _ := newMetadataReceiver(t, databaseID)
	peer := syncstate.NewHandshake(databaseID, "replica_clock", CurrentSchemaVersion, syncstate.Vector{"replica_clock": 2})
	if err := st.ConfigureSyncAdmissionPeer(context.Background(), peer); err != nil {
		t.Fatal(err)
	}
	operations := []syncstate.Operation{
		metadataOperation("replica_clock", 1, 200, "record.update", "document", "doc_a", `{"title":"first"}`),
		metadataOperation("replica_clock", 2, 100, "record.update", "document", "doc_a", `{"title":"backward"}`),
	}
	if _, err := st.AdmitSyncOperations(context.Background(), peer, operations); err == nil || !strings.Contains(err.Error(), "HLC moved backward") {
		t.Fatalf("backward HLC error = %v", err)
	}
	if got := syncCount(t, st, `SELECT COUNT(*) FROM sync_operations WHERE replica_id='replica_clock'`); got != 0 {
		t.Fatalf("backward HLC transaction admitted %d operations", got)
	}
}
