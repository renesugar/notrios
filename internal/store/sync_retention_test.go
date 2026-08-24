package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

func TestSyncRetentionRequiresOneSnapshotAndEveryActivePeer(t *testing.T) {
	ctx := context.Background()
	st := newSyncJournalTestStore(t)
	journal, err := st.EnrollLocalJournal(ctx, "retention matrix")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_retention", Title: "retained", Body: "body"}); err != nil {
		t.Fatal(err)
	}
	vector, err := st.SyncStateVector(ctx)
	if err != nil {
		t.Fatal(err)
	}
	identity, _ := st.GetDatabaseIdentity(ctx)
	for _, peerID := range []string{"replica_phone", "replica_tablet"} {
		peer := syncstate.NewHandshake(identity.DatabaseID, peerID, CurrentSchemaVersion, syncstate.Vector{peerID: 0})
		if err := st.ConfigureSyncAdmissionPeer(ctx, peer); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := st.Exec(ctx, `UPDATE sync_operations SET created_at='2025-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	now := old.Add(365 * 24 * time.Hour)
	blocked, err := st.PlanSyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.SnapshotID != "" || blocked.EligibleOperations != 0 {
		t.Fatalf("time alone authorized collection: %+v", blocked)
	}
	if err := st.RecordVerifiedSyncSnapshot(ctx, VerifiedSyncSnapshot{SnapshotID: "snapshot_retention",
		CommitSHA256: strings.Repeat("a", 64), CreatedAt: now, Vector: vector}); err != nil {
		t.Fatal(err)
	}
	stillBlocked, err := st.PlanSyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if stillBlocked.EligibleOperations != 0 {
		t.Fatalf("unacknowledged phone did not hold the floor: %+v", stillBlocked.Subjects)
	}
	for _, peerID := range []string{"replica_phone", "replica_tablet"} {
		ackVector := syncstate.CloneVector(vector)
		ackVector[peerID] = 0
		peer := syncstate.NewHandshake(identity.DatabaseID, peerID, CurrentSchemaVersion, ackVector)
		if err := st.RecordSyncPeerAcknowledgement(ctx, peer); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := st.PlanSyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if plan.EligibleOperations == 0 || plan.Digest == "" {
		t.Fatalf("fully covered history was not planned: %+v", plan)
	}
	if _, err := st.ApplySyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now, ExpectedDigest: strings.Repeat("0", 64)}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale dry-run digest applied: %v", err)
	}
	applied, err := st.ApplySyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now, ExpectedDigest: plan.Digest})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.RemovedOperations != plan.EligibleOperations {
		t.Fatalf("apply did not match dry run: %+v", applied)
	}
	if err := st.RecordVerifiedSyncSnapshot(ctx, VerifiedSyncSnapshot{SnapshotID: "zz_snapshot_stale",
		CommitSHA256: strings.Repeat("e", 64), CreatedAt: now, Vector: syncstate.Vector{journal.ReplicaID: 0}}); err != nil {
		t.Fatal(err)
	}
	repair, err := st.PlanSyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now})
	if err != nil || repair.Repair.SnapshotID != "snapshot_retention" {
		t.Fatalf("repair selected a snapshot below the collected floor: %+v err=%v", repair.Repair, err)
	}
	if _, err := st.ListSyncOperations(ctx, journal.ReplicaID, 0, 100); !errors.Is(err, ErrSyncFullResyncRequired) {
		t.Fatalf("below-floor peer was not sent to catch-up: %v", err)
	}
	if _, err := st.GetDocument(ctx, "doc_retention"); err != nil {
		t.Fatalf("checkpoint compaction changed canonical state: %v", err)
	}
}

func TestPeerRetirementPropagatesAndCannotReenroll(t *testing.T) {
	ctx := context.Background()
	source := newSyncJournalTestStore(t)
	receiver := newSyncJournalTestStore(t)
	sourceIdentity, _ := source.GetDatabaseIdentity(ctx)
	if _, err := receiver.AdoptDatabaseIdentity(ctx, sourceIdentity.DatabaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := source.EnrollLocalJournal(ctx, "source"); err != nil {
		t.Fatal(err)
	}
	if _, err := receiver.EnrollLocalJournal(ctx, "receiver"); err != nil {
		t.Fatal(err)
	}
	sourceHandshake, _ := source.LocalSyncHandshake(ctx)
	receiverHandshake, _ := receiver.LocalSyncHandshake(ctx)
	target := syncstate.NewHandshake(sourceIdentity.DatabaseID, "replica_drawer_phone", CurrentSchemaVersion, syncstate.Vector{"replica_drawer_phone": 0})
	for _, configured := range []struct {
		store *SQLiteStore
		peer  syncstate.Handshake
	}{{source, target}, {receiver, target}, {receiver, sourceHandshake}, {source, receiverHandshake}} {
		if err := configured.store.ConfigureSyncAdmissionPeer(ctx, configured.peer); err != nil {
			t.Fatal(err)
		}
	}
	signer, _ := syncwire.NewMemorySigner()
	if _, err := receiver.EnrollPeerSigningKey(ctx, sourceHandshake.ReplicaID, signer.Public(), "retirement signer"); err != nil {
		t.Fatal(err)
	}
	retired, err := source.RetireSyncPeer(ctx, RetireSyncPeerRequest{ReplicaID: target.ReplicaID, Reason: "device recycled", Signer: signer})
	if err != nil {
		t.Fatal(err)
	}
	if retired.Status != "retired" || retired.DecisionSequence == 0 {
		t.Fatalf("retirement result: %+v", retired)
	}
	updatedSource, _ := source.LocalSyncHandshake(ctx)
	operations, err := source.ListSyncOperations(ctx, updatedSource.ReplicaID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	updatedSource.StateVector = syncstate.CloneVector(updatedSource.StateVector)
	result, err := receiver.AdmitSyncOperations(ctx, updatedSource, operations)
	if err != nil || result.Admitted == 0 {
		t.Fatalf("propagate retirement: %+v %v", result, err)
	}
	if active, err := receiver.SyncPeerEnrolled(ctx, target.ReplicaID); err != nil || active {
		t.Fatalf("retired peer remains active: %v %v", active, err)
	}
	if err := receiver.ConfigureSyncAdmissionPeer(ctx, target); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale credentials silently re-enrolled: %v", err)
	}
}

func TestSignedPurgeRetainsDeathCertificateThenCollectsPayload(t *testing.T) {
	ctx := context.Background()
	st := newSyncJournalTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_death", Title: "Gone", Body: "retained until safe"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollLocalJournal(ctx, "signed purge"); err != nil {
		t.Fatal(err)
	}
	signer, _ := syncwire.NewMemorySigner()
	if err := st.PurgeDocumentWithCertificate(ctx, doc.ID, signer); err != nil {
		t.Fatal(err)
	}
	trash, err := st.ListTrash(ctx, DocumentPageRequest{Limit: 10})
	if err != nil || len(trash.Documents) != 0 {
		t.Fatalf("death-certified document remained in Trash: %+v %v", trash, err)
	}
	if syncCount(t, st, `SELECT COUNT(*) FROM sync_death_certificates WHERE document_id=?`, doc.ID) != 1 ||
		syncCount(t, st, `SELECT COUNT(*) FROM sync_tombstone_payloads WHERE document_id=? AND collected_at IS NULL`, doc.ID) != 1 {
		t.Fatal("signed purge did not retain death identity and payload marker")
	}
	vector, _ := st.SyncStateVector(ctx)
	now := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := st.Exec(ctx, `UPDATE sync_operations SET created_at='2026-01-01T00:00:00Z';
		UPDATE sync_tombstone_payloads SET purged_at='2026-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordVerifiedSyncSnapshot(ctx, VerifiedSyncSnapshot{SnapshotID: "snapshot_death",
		CommitSHA256: strings.Repeat("b", 64), CreatedAt: now, Vector: vector}); err != nil {
		t.Fatal(err)
	}
	plan, err := st.PlanSyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now})
	if err != nil || len(plan.Tombstones) != 1 {
		t.Fatalf("tombstone plan: %+v %v", plan, err)
	}
	if _, err := st.ApplySyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, Now: now, ExpectedDigest: plan.Digest}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetDocumentIncludingTrashed(ctx, doc.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("retained payload was not collected: %v", err)
	}
	if syncCount(t, st, `SELECT COUNT(*) FROM sync_death_certificates WHERE document_id=?`, doc.ID) != 1 {
		t.Fatal("payload collection removed resurrection protection")
	}
	st.mu.Lock()
	err = st.reconcileSyncMetadataLocked()
	st.mu.Unlock()
	if err != nil || syncCount(t, st, `SELECT COUNT(*) FROM sync_death_certificates WHERE document_id=?`, doc.ID) != 1 {
		t.Fatalf("compacted death certificate did not survive repair: %v", err)
	}
}

func TestSyncResourceRetentionGateIsConservative(t *testing.T) {
	ctx := context.Background()
	st := newSyncJournalTestStore(t)
	if _, err := st.EnrollLocalJournal(ctx, "resource gate"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_gate", Content: strings.NewReader("bytes")}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(365 * 24 * time.Hour)
	gate, err := st.BuildSyncRetentionGate(ctx, SyncRetentionRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	allowed, reason, _ := gate.CanCollect(ctx, GarbageCollectionCandidate{})
	if allowed || reason != "no_verified_snapshot" {
		t.Fatalf("resource gate allowed time-only collection: %v %q", allowed, reason)
	}
	vector, _ := st.SyncStateVector(ctx)
	if err := st.RecordVerifiedSyncSnapshot(ctx, VerifiedSyncSnapshot{SnapshotID: "snapshot_resource",
		CommitSHA256: strings.Repeat("c", 64), CreatedAt: now, Vector: vector}); err != nil {
		t.Fatal(err)
	}
	identity, _ := st.GetDatabaseIdentity(ctx)
	peer := syncstate.NewHandshake(identity.DatabaseID, "replica_resource_keeper", CurrentSchemaVersion, syncstate.Vector{"replica_resource_keeper": 0})
	if err := st.ConfigureSyncAdmissionPeer(ctx, peer); err != nil {
		t.Fatal(err)
	}
	gate, err = st.BuildSyncRetentionGate(ctx, SyncRetentionRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	allowed, reason, _ = gate.CanCollect(ctx, GarbageCollectionCandidate{})
	if allowed || reason != "sync_watermark_pending" {
		t.Fatalf("resource still needed by one peer was allowed: %v %q", allowed, reason)
	}
	peer.StateVector = syncstate.CloneVector(vector)
	peer.StateVector[peer.ReplicaID] = 0
	if err := st.RecordSyncPeerAcknowledgement(ctx, peer); err != nil {
		t.Fatal(err)
	}
	gate, err = st.BuildSyncRetentionGate(ctx, SyncRetentionRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	allowed, reason, _ = gate.CanCollect(ctx, GarbageCollectionCandidate{})
	if !allowed || reason != "sync_ack_snapshot_satisfied" {
		t.Fatalf("fully covered resource remained blocked: %v %q", allowed, reason)
	}
}

func TestPhoneInDrawerCredentialRevocationDoesNotReleaseFloorButRetirementDoes(t *testing.T) {
	ctx := context.Background()
	st := newSyncJournalTestStore(t)
	if _, err := st.EnrollLocalJournal(ctx, "phone in drawer"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "old operation", Body: "retained"}); err != nil {
		t.Fatal(err)
	}
	identity, _ := st.GetDatabaseIdentity(ctx)
	const peerID = "replica_phone_in_drawer"
	peer := syncstate.NewHandshake(identity.DatabaseID, peerID, CurrentSchemaVersion, syncstate.Vector{peerID: 0})
	if err := st.ConfigureSyncAdmissionPeer(ctx, peer); err != nil {
		t.Fatal(err)
	}
	peerSigner, _ := syncwire.NewMemorySigner()
	keyID, err := st.EnrollPeerSigningKey(ctx, peerID, peerSigner.Public(), "drawer fixture")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := old.Add(120 * 24 * time.Hour)
	if err := st.Exec(ctx, `UPDATE sync_operations SET created_at='2026-01-01T00:00:00Z';
		UPDATE sync_replicas SET enrolled_at='2026-01-01T00:00:00Z' WHERE replica_id='replica_phone_in_drawer'`); err != nil {
		t.Fatal(err)
	}
	vector, _ := st.SyncStateVector(ctx)
	if err := st.RecordVerifiedSyncSnapshot(ctx, VerifiedSyncSnapshot{SnapshotID: "snapshot_drawer",
		CommitSHA256: strings.Repeat("d", 64), CreatedAt: now, Vector: vector}); err != nil {
		t.Fatal(err)
	}
	if err := st.RevokePeerSigningKey(ctx, keyID, "lost credential"); err != nil {
		t.Fatal(err)
	}
	blocked, err := st.PlanSyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, WarningBefore: 30 * 24 * time.Hour, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.EligibleOperations != 0 || len(blocked.Peers) != 1 || !blocked.Peers[0].BeyondHorizon {
		t.Fatalf("credential revocation released or hid the offline peer: %+v", blocked)
	}
	ownerSigner, _ := syncwire.NewMemorySigner()
	retired, err := st.RetireSyncPeer(ctx, RetireSyncPeerRequest{ReplicaID: peerID, Reason: "owner retirement after review", Signer: ownerSigner})
	if err != nil {
		t.Fatal(err)
	}
	if len(retired.UnacknowledgedPeers) != 0 {
		t.Fatalf("retired target remained an active acknowledgement peer: %+v", retired)
	}
	released, err := st.PlanSyncRetention(ctx, SyncRetentionRequest{HistoryFor: 90 * 24 * time.Hour, WarningBefore: 30 * 24 * time.Hour, Now: now})
	if err != nil || released.EligibleOperations == 0 || released.Peers[0].Status != "retired" {
		t.Fatalf("explicit retirement did not release the covered old floor: %+v err=%v", released, err)
	}
}
