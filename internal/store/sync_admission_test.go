package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
)

func newAdmissionReplica(t *testing.T, databaseID string) *SQLiteStore {
	t.Helper()
	st := newSyncJournalTestStore(t)
	if _, err := st.AdoptDatabaseIdentity(context.Background(), databaseID); err != nil {
		t.Fatalf("AdoptDatabaseIdentity: %v", err)
	}
	if _, err := st.EnrollLocalJournal(context.Background(), "G5 model replica"); err != nil {
		t.Fatalf("EnrollLocalJournal: %v", err)
	}
	return st
}

func configureAllAdmissionPeers(t *testing.T, replicas []*SQLiteStore) []syncstate.Handshake {
	t.Helper()
	ctx := context.Background()
	handshakes := make([]syncstate.Handshake, len(replicas))
	for index, replica := range replicas {
		handshake, err := replica.LocalSyncHandshake(ctx)
		if err != nil {
			t.Fatalf("LocalSyncHandshake %d: %v", index, err)
		}
		handshakes[index] = handshake
	}
	for receiver, replica := range replicas {
		for source, handshake := range handshakes {
			if receiver == source {
				continue
			}
			if err := replica.ConfigureSyncAdmissionPeer(ctx, handshake); err != nil {
				t.Fatalf("configure receiver %d source %d: %v", receiver, source, err)
			}
		}
	}
	return handshakes
}

func refreshHandshake(t *testing.T, st *SQLiteStore) syncstate.Handshake {
	t.Helper()
	handshake, err := st.LocalSyncHandshake(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return handshake
}

func localOperations(t *testing.T, st *SQLiteStore, replicaID string) []syncstate.Operation {
	t.Helper()
	operations, err := st.ListSyncOperations(context.Background(), replicaID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	return operations
}

func TestSyncAdmissionThreeReplicasShuffleDuplicateDropThenConverge(t *testing.T) {
	ctx := context.Background()
	replicas := []*SQLiteStore{
		newAdmissionReplica(t, "db_g5_model"),
		newAdmissionReplica(t, "db_g5_model"),
		newAdmissionReplica(t, "db_g5_model"),
	}
	handshakes := configureAllAdmissionPeers(t, replicas)
	for index, replica := range replicas {
		if _, err := replica.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_g5_%d", index), Title: fmt.Sprintf("Replica %d", index), Body: "immutable fixture",
		}); err != nil {
			t.Fatal(err)
		}
		handshakes[index] = refreshHandshake(t, replica)
	}

	// The second operation arrives first and is durably pending. Neither the
	// vector nor acknowledgement crosses the missing first operation.
	sourceOps := localOperations(t, replicas[0], handshakes[0].ReplicaID)
	if len(sourceOps) != 2 {
		t.Fatalf("source operation count = %d", len(sourceOps))
	}
	result, err := replicas[1].AdmitSyncOperations(ctx, handshakes[0], []syncstate.Operation{sourceOps[1], sourceOps[1]})
	if err != nil {
		t.Fatal(err)
	}
	if result.Admitted != 0 || result.Pending != 1 || result.Duplicates != 1 || result.Acknowledgement[handshakes[0].ReplicaID] != 0 {
		t.Fatalf("out-of-order result: %+v", result)
	}
	if gaps := syncCount(t, replicas[1], `SELECT COUNT(*) FROM sync_state_gaps WHERE subject_replica_id = ? AND start_sequence = 1 AND end_sequence = 1`, handshakes[0].ReplicaID); gaps != 1 {
		t.Fatalf("explicit gap count = %d", gaps)
	}
	plan, err := replicas[1].PlanMissingSyncOperations(ctx, handshakes[0].StateVector)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ranges) != 1 || plan.Ranges[0].Start != 1 || plan.Ranges[0].End != 1 {
		t.Fatalf("missing plan = %+v", plan)
	}
	result, err = replicas[1].AdmitSyncOperations(ctx, handshakes[0], []syncstate.Operation{sourceOps[0]})
	if err != nil {
		t.Fatal(err)
	}
	if result.Admitted != 2 || result.Pending != 0 || result.Acknowledgement[handshakes[0].ReplicaID] != 2 {
		t.Fatalf("gap close result: %+v", result)
	}

	// Eventual full delivery includes deliberate duplicates and different
	// source/receiver orders. Every replica ends with the same operation IDs and
	// identical contiguous vectors.
	for receiver, replica := range replicas {
		for offset := range replicas {
			source := (receiver + offset + 1) % len(replicas)
			if source == receiver {
				continue
			}
			operations := localOperations(t, replicas[source], handshakes[source].ReplicaID)
			shuffled := []syncstate.Operation{operations[1], operations[0], operations[1]}
			if _, err := replica.AdmitSyncOperations(ctx, handshakes[source], shuffled); err != nil {
				t.Fatalf("receiver %d source %d: %v", receiver, source, err)
			}
		}
	}
	wantVector := syncstate.Vector{}
	wantOperationIDs := []string{}
	for _, handshake := range handshakes {
		wantVector[handshake.ReplicaID] = 2
		wantOperationIDs = append(wantOperationIDs,
			fmt.Sprintf("%s:%020d", handshake.ReplicaID, 1),
			fmt.Sprintf("%s:%020d", handshake.ReplicaID, 2))
	}
	sort.Strings(wantOperationIDs)
	for index, replica := range replicas {
		vector, err := replica.SyncStateVector(ctx)
		if err != nil || !reflect.DeepEqual(vector, wantVector) {
			t.Fatalf("replica %d vector = %v, %v; want %v", index, vector, err, wantVector)
		}
		gotIDs := []string{}
		for _, handshake := range handshakes {
			for _, operation := range localOperations(t, replica, handshake.ReplicaID) {
				gotIDs = append(gotIDs, operation.OperationID)
			}
		}
		sort.Strings(gotIDs)
		if !reflect.DeepEqual(gotIDs, wantOperationIDs) {
			t.Fatalf("replica %d operation IDs = %v", index, gotIDs)
		}
	}
}

func noopOperation(replicaID string, sequence int64, dependencies ...syncstate.OperationRef) syncstate.Operation {
	return syncstate.Operation{
		ReplicaID: replicaID, Sequence: sequence,
		OperationID: fmt.Sprintf("%s:%020d", replicaID, sequence),
		Kind:        "sync.noop", RecordType: "sync_noop", RecordID: fmt.Sprintf("noop_%d", sequence),
		Payload: json.RawMessage(`{"noop":true}`), HLC: syncstate.HLC{WallMS: sequence * 1000},
		CreatedAt:    time.Unix(sequence, 0).UTC().Format(time.RFC3339Nano),
		Dependencies: dependencies,
	}
}

func TestSyncAdmissionDependencyQueuePlansAndDrainsAcrossReplicas(t *testing.T) {
	ctx := context.Background()
	receiver := newAdmissionReplica(t, "db_g5_dependencies")
	sourceA := newAdmissionReplica(t, "db_g5_dependencies")
	sourceB := newAdmissionReplica(t, "db_g5_dependencies")
	handshakes := configureAllAdmissionPeers(t, []*SQLiteStore{receiver, sourceA, sourceB})
	a := handshakes[1]
	b := handshakes[2]
	b1 := noopOperation(b.ReplicaID, 1, syncstate.OperationRef{ReplicaID: a.ReplicaID, Sequence: 1})
	b.StateVector[b.ReplicaID] = 1
	result, err := receiver.AdmitSyncOperations(ctx, b, []syncstate.Operation{b1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Admitted != 0 || result.Pending != 1 || result.Acknowledgement[b.ReplicaID] != 0 {
		t.Fatalf("dependency should be pending: %+v", result)
	}
	remoteVector := syncstate.Vector{a.ReplicaID: 1, b.ReplicaID: 1}
	plan, err := receiver.PlanMissingSyncOperations(ctx, remoteVector)
	if err != nil {
		t.Fatal(err)
	}
	wantDependency := []syncstate.OperationRef{{ReplicaID: a.ReplicaID, Sequence: 1}}
	if !reflect.DeepEqual(plan.Dependencies, wantDependency) {
		t.Fatalf("missing dependencies = %v", plan.Dependencies)
	}
	a.StateVector[a.ReplicaID] = 1
	result, err = receiver.AdmitSyncOperations(ctx, a, []syncstate.Operation{noopOperation(a.ReplicaID, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Admitted != 2 {
		t.Fatalf("cross-replica drain admitted %d, want 2", result.Admitted)
	}
	vector, err := receiver.SyncStateVector(ctx)
	if err != nil || vector[a.ReplicaID] != 1 || vector[b.ReplicaID] != 1 {
		t.Fatalf("vector after dependency drain = %v, %v", vector, err)
	}
}

func TestSyncAdmissionRequiresConfiguredCompatiblePeerAndExactReplay(t *testing.T) {
	ctx := context.Background()
	receiver := newAdmissionReplica(t, "db_g5_compat")
	source := newAdmissionReplica(t, "db_g5_compat")
	peer := refreshHandshake(t, source)
	operation := noopOperation(peer.ReplicaID, 1)
	peer.StateVector[peer.ReplicaID] = 1
	if _, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{operation}); !errors.Is(err, ErrConflict) {
		t.Fatalf("unconfigured admission error = %v", err)
	}
	if syncCount(t, receiver, `SELECT COUNT(*) FROM sync_replicas WHERE replica_id = ?`, peer.ReplicaID) != 0 {
		t.Fatal("handshake auto-enrolled a peer")
	}
	if err := receiver.ConfigureSyncAdmissionPeer(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if _, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{operation}); err != nil {
		t.Fatal(err)
	}
	result, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{operation})
	if err != nil || result.Duplicates != 1 || result.Admitted != 0 {
		t.Fatalf("exact replay result = %+v, %v", result, err)
	}
	conflict := operation
	conflict.RecordID = "different"
	if _, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{conflict}); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting replay error = %v", err)
	}
	unknown := noopOperation(peer.ReplicaID, 2)
	unknown.Kind = "future.apply"
	unknown.RecordType = "future_record"
	if _, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{unknown}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown required record error = %v", err)
	}
	changed := peer
	changed.OptionalCapabilities = []string{"sync.fixture.v1"}
	if _, err := receiver.AdmitSyncOperations(ctx, changed, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed configured compatibility error = %v", err)
	}

	// Wall-clock skew does not order delivery in G5: only the durable sequence
	// does. A later sequence with an old timestamp still advances contiguously.
	skewed := noopOperation(peer.ReplicaID, 2)
	skewed.CreatedAt = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	peer.StateVector[peer.ReplicaID] = 2
	if result, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{skewed}); err != nil || result.Acknowledgement[peer.ReplicaID] != 2 {
		t.Fatalf("clock-skew admission = %+v, %v", result, err)
	}
	tooFar := noopOperation(peer.ReplicaID, 2+syncstate.MaxSequenceSkew+1)
	peer.StateVector[peer.ReplicaID] = tooFar.Sequence
	if _, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{tooFar}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sequence skew error = %v", err)
	}

	tooMany := make([]syncstate.Operation, syncstate.MaxPendingOperations+1)
	if _, err := receiver.AdmitSyncOperations(ctx, peer, tooMany); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("operation count bound error = %v", err)
	}
	largePayload := json.RawMessage(`"` + strings.Repeat("x", syncstate.MaxPayloadBytes-2) + `"`)
	tooLarge := make([]syncstate.Operation, 33)
	for index := range tooLarge {
		tooLarge[index] = noopOperation(peer.ReplicaID, int64(index+3))
		tooLarge[index].Payload = largePayload
	}
	peer.StateVector[peer.ReplicaID] = tooLarge[len(tooLarge)-1].Sequence
	if _, err := receiver.AdmitSyncOperations(ctx, peer, tooLarge); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("encoded batch bound error = %v", err)
	}
}

func TestSyncAdmissionRestartAndInjectedPreCommitFailure(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	path := filepath.Join(directory, "receiver.sqlite")
	receiver, err := OpenSQLiteWithAssetStore(path, filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	identity, err := receiver.AdoptDatabaseIdentity(ctx, "db_g5_restart")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receiver.EnrollLocalJournal(ctx, "restart test"); err != nil {
		t.Fatal(err)
	}
	peer := syncstate.NewHandshake(identity.DatabaseID, "replica_restart_peer", CurrentSchemaVersion, syncstate.Vector{"replica_restart_peer": 2})
	if err := receiver.ConfigureSyncAdmissionPeer(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if _, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{noopOperation(peer.ReplicaID, 2)}); err != nil {
		t.Fatal(err)
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}
	receiver, err = OpenSQLiteWithAssetStore(path, filepath.Join(directory, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	if err := receiver.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected before commit")
	if _, err := receiver.admitSyncOperations(ctx, peer, []syncstate.Operation{noopOperation(peer.ReplicaID, 1)}, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatalf("injected error = %v", err)
	}
	vector, err := receiver.SyncStateVector(ctx)
	if err != nil || vector[peer.ReplicaID] != 0 {
		t.Fatalf("rollback vector = %v, %v", vector, err)
	}
	if pending := syncCount(t, receiver, `SELECT COUNT(*) FROM sync_pending_admissions WHERE replica_id = ?`, peer.ReplicaID); pending != 1 {
		t.Fatalf("rollback pending count = %d, want original 1", pending)
	}
	result, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{noopOperation(peer.ReplicaID, 1)})
	if err != nil || result.Admitted != 2 || result.Acknowledgement[peer.ReplicaID] != 2 {
		t.Fatalf("restart admission = %+v, %v", result, err)
	}
}

func TestSyncAdmissionAcknowledgementBoundsAndSequenceExhaustion(t *testing.T) {
	ctx := context.Background()
	receiver := newAdmissionReplica(t, "db_g5_ack")
	source := newAdmissionReplica(t, "db_g5_ack")
	configureAllAdmissionPeers(t, []*SQLiteStore{receiver, source})
	receiverHandshake := refreshHandshake(t, receiver)
	sourceHandshake := refreshHandshake(t, source)
	if _, err := receiver.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_exhaustion", Title: "before", Body: "body"}); err != nil {
		t.Fatal(err)
	}
	receiverHandshake = refreshHandshake(t, receiver)
	ack := sourceHandshake
	ack.StateVector = syncstate.Vector{
		sourceHandshake.ReplicaID:   0,
		receiverHandshake.ReplicaID: receiverHandshake.StateVector[receiverHandshake.ReplicaID],
	}
	if err := receiver.RecordSyncPeerAcknowledgement(ctx, ack); err != nil {
		t.Fatal(err)
	}
	ack.StateVector[receiverHandshake.ReplicaID]++
	if err := receiver.RecordSyncPeerAcknowledgement(ctx, ack); !errors.Is(err, ErrConflict) {
		t.Fatalf("ahead acknowledgement error = %v", err)
	}
	ack.StateVector[receiverHandshake.ReplicaID] = 0
	if err := receiver.RecordSyncPeerAcknowledgement(ctx, ack); !errors.Is(err, ErrConflict) {
		t.Fatalf("backward acknowledgement error = %v", err)
	}

	receiver.mu.Lock()
	err := receiver.execPreparedLocked(`UPDATE sync_local_journal SET last_sequence = ? WHERE singleton = 1`, fmt.Sprintf("%d", syncstate.MaxSequence))
	receiver.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receiver.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_after_exhaustion", Title: "after", Body: "body"}); err == nil {
		t.Fatal("sequence exhaustion did not refuse canonical write")
	}
	if _, err := receiver.GetDocument(ctx, "doc_after_exhaustion"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("exhausted write was not rolled back: %v", err)
	}
}

func TestSchemaV19UpgradeAddsG5CompatibilityState(t *testing.T) {
	st := newSyncJournalTestStore(t)
	ctx := context.Background()
	st.mu.Lock()
	if err := st.execLocked(`DROP TABLE sync_peer_compatibility; PRAGMA user_version = 19`); err != nil {
		st.mu.Unlock()
		t.Fatal(err)
	}
	st.mu.Unlock()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema status = %+v, %v", status, err)
	}
	if syncCount(t, st, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'sync_peer_compatibility'`) != 1 {
		t.Fatal("missing sync_peer_compatibility")
	}
	if syncCount(t, st, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'sync_journal_sequence_exhaustion'`) != 1 {
		t.Fatal("missing sequence exhaustion trigger")
	}
}
