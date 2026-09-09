package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/synccatchup"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

func catchupFixture(t *testing.T) (*SQLiteStore, *SQLiteStore, syncstate.Handshake, time.Time) {
	t.Helper()
	source := newAdmissionReplica(t, "db_g10")
	fresh := newAdmissionReplica(t, "db_g10")
	handshakes := configureAllAdmissionPeers(t, []*SQLiteStore{source, fresh})
	return source, fresh, handshakes[0], time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
}

func TestSnapshotSourcesRequireExplicitPermission(t *testing.T) {
	ctx := context.Background()
	source, fresh, sourceHandshake, _ := catchupFixture(t)
	_ = source
	policy := fresh.SnapshotSources()

	// The peer is enrolled — that is what configureAllAdmissionPeers did — and
	// still may not answer, because handing over a complete copy of a library
	// is a separate decision from being able to exchange operations with it.
	if policy.PermittedSource(sourceHandshake.ReplicaID) {
		t.Fatal("an enrolled peer was treated as a permitted snapshot source")
	}
	if err := fresh.PermitSnapshotSource(ctx, sourceHandshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	if !policy.PermittedSource(sourceHandshake.ReplicaID) {
		t.Fatal("an explicitly permitted peer was still refused")
	}
	if err := fresh.PermitSnapshotSource(ctx, sourceHandshake.ReplicaID, false); err != nil {
		t.Fatal(err)
	}
	if policy.PermittedSource(sourceHandshake.ReplicaID) {
		t.Fatal("permission was not revocable")
	}
	// A replica nobody enrolled is never a source, however permissive the row.
	if err := fresh.PermitSnapshotSource(ctx, "replica-stranger", true); err != nil {
		t.Fatal(err)
	}
	if policy.PermittedSource("replica-stranger") {
		t.Fatal("permission alone made an unenrolled replica a source")
	}
}

func TestCatchupSessionSurvivesEveryStep(t *testing.T) {
	ctx := context.Background()
	source, fresh, sourceHandshake, now := catchupFixture(t)
	_ = source
	if err := fresh.PermitSnapshotSource(ctx, sourceHandshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	signer, err := syncwire.NewMemorySigner()
	if err != nil {
		t.Fatal(err)
	}
	verifier := syncwire.NewMemoryVerifier()
	keyID := verifier.Enroll(signer.Public())

	request, err := synccatchup.NewRequest("db_g10", "replica-fresh", CurrentSchemaVersion, synccatchup.WrapPeerKey, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	session, err := fresh.BeginCatchup(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if session.State != synccatchup.StateRequested || session.Role != "requester" {
		t.Fatalf("session = %+v", session)
	}

	snapshotVector := syncstate.Vector{sourceHandshake.ReplicaID: 12, "replica-other": 3}
	response := synccatchup.Response{
		DatabaseID: "db_g10", ResponderReplicaID: sourceHandshake.ReplicaID, RequestNonce: request.Nonce,
		ProtocolMajor: syncwire.ProtocolMajor, ProtocolMinor: syncwire.ProtocolMinor,
		SchemaVersion: CurrentSchemaVersion, SnapshotID: "snap_one", SnapshotVector: snapshotVector,
		ArchiveSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ArchiveLength: 8192, WrappingMode: synccatchup.WrapPeerKey,
		ExpiresAt: now.Add(synccatchup.DefaultTTL).Format(time.RFC3339Nano),
	}
	offer := synccatchup.Offer{Response: response, SignerKeyID: keyID, Signature: synccatchup.SignResponse(signer, response)}
	chosen, err := synccatchup.Select(request, []synccatchup.Offer{offer}, fresh.SnapshotSources(), verifier, CurrentSchemaVersion, now)
	if err != nil {
		t.Fatal(err)
	}
	if session, err = fresh.AcceptCatchupOffer(ctx, session.ID, chosen); err != nil {
		t.Fatal(err)
	}
	if session.State != synccatchup.StateOffered || session.ArchiveLength != 8192 {
		t.Fatalf("after accepting: %+v", session)
	}

	// An interrupted transfer keeps its progress, so resuming does not restart.
	if _, err := fresh.AdvanceCatchup(ctx, session.ID, synccatchup.StateTransferring, ""); err != nil {
		t.Fatal(err)
	}
	if err := fresh.RecordCatchupProgress(ctx, session.ID, 3000); err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.AdvanceCatchup(ctx, session.ID, synccatchup.StateTransferring, "connection dropped"); err != nil {
		t.Fatal(err)
	}
	if err := fresh.RecordCatchupProgress(ctx, session.ID, 8192); err != nil {
		t.Fatal(err)
	}
	session, err = fresh.GetCatchupSession(ctx, session.ID)
	if err != nil || session.ReceivedBytes != 8192 {
		t.Fatalf("resume state: %+v %v", session, err)
	}

	for _, state := range []synccatchup.State{synccatchup.StateVerifying, synccatchup.StateRestoring} {
		if _, err := fresh.AdvanceCatchup(ctx, session.ID, state, ""); err != nil {
			t.Fatalf("advance to %s: %v", state, err)
		}
	}

	// Cutover refuses without an explicit intent: nothing here restores
	// destructively by default.
	if _, err := fresh.CutoverToSnapshot(ctx, session.ID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("cutover without an intent: %v", err)
	}
	if err := fresh.SetCatchupIntent(ctx, session.ID, "adopt"); err != nil {
		t.Fatal(err)
	}
	if err := fresh.SetCatchupIntent(ctx, session.ID, "obliterate"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("an invented intent was accepted")
	}
	if session, err = fresh.CutoverToSnapshot(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	if session.State != synccatchup.StateCutover {
		t.Fatalf("state after cutover = %s", session.State)
	}

	// The snapshot's vector is now this replica's own observation, so the next
	// exchange asks only for what came after it.
	vector, err := fresh.SyncStateVector(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if vector[sourceHandshake.ReplicaID] != 12 || vector["replica-other"] != 3 {
		t.Fatalf("state vector after cutover = %+v", vector)
	}
	plan, err := synccatchup.RemainingWork(vector, syncstate.Vector{sourceHandshake.ReplicaID: 20, "replica-other": 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ranges) != 1 || plan.Ranges[0].Start != 13 || plan.Ranges[0].End != 20 {
		t.Fatalf("remaining work = %+v", plan.Ranges)
	}

	// A backup is not an acknowledgement. Nothing about restoring a snapshot
	// says a peer has durably admitted anything, so nothing may hold back
	// collection on that basis.
	acks := queryRows(t, fresh, `SELECT COUNT(*) AS total FROM sync_peer_acknowledgements`)
	if acks[0]["total"] != "0" {
		t.Fatalf("cutover wrote %s peer acknowledgements", acks[0]["total"])
	}

	if _, err := fresh.AdvanceCatchup(ctx, session.ID, synccatchup.StateComplete, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.AdvanceCatchup(ctx, session.ID, synccatchup.StateTransferring, ""); !errors.Is(err, synccatchup.ErrBadTransition) {
		t.Fatal("a completed catch-up was restarted")
	}
}

func TestCatchupExpiryAndCancellation(t *testing.T) {
	ctx := context.Background()
	_, fresh, sourceHandshake, now := catchupFixture(t)
	if err := fresh.PermitSnapshotSource(ctx, sourceHandshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	signer, _ := syncwire.NewMemorySigner()
	verifier := syncwire.NewMemoryVerifier()
	keyID := verifier.Enroll(signer.Public())

	newSession := func(expiry time.Time) CatchupSession {
		t.Helper()
		request, err := synccatchup.NewRequest("db_g10", "replica-fresh", CurrentSchemaVersion, synccatchup.WrapPeerKey, nil, now)
		if err != nil {
			t.Fatal(err)
		}
		session, err := fresh.BeginCatchup(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		response := synccatchup.Response{
			DatabaseID: "db_g10", ResponderReplicaID: sourceHandshake.ReplicaID, RequestNonce: request.Nonce,
			ProtocolMajor: syncwire.ProtocolMajor, ProtocolMinor: syncwire.ProtocolMinor,
			SchemaVersion: CurrentSchemaVersion, SnapshotID: "snap", SnapshotVector: syncstate.Vector{sourceHandshake.ReplicaID: 1},
			ArchiveSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			ArchiveLength: 10, WrappingMode: synccatchup.WrapPeerKey,
			ExpiresAt: expiry.Format(time.RFC3339Nano),
		}
		offer := synccatchup.Offer{Response: response, SignerKeyID: keyID, Signature: synccatchup.SignResponse(signer, response)}
		session, err = fresh.AcceptCatchupOffer(ctx, session.ID, offer)
		if err != nil {
			t.Fatal(err)
		}
		return session
	}

	stale := newSession(now.Add(-time.Hour))
	live := newSession(now.Add(time.Hour))
	expired, err := fresh.ExpireCatchupSessions(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if expired != 1 {
		t.Fatalf("expired %d sessions, want 1", expired)
	}
	staleAfter, _ := fresh.GetCatchupSession(ctx, stale.ID)
	liveAfter, _ := fresh.GetCatchupSession(ctx, live.ID)
	if staleAfter.State != synccatchup.StateExpired || liveAfter.State != synccatchup.StateOffered {
		t.Fatalf("stale=%s live=%s", staleAfter.State, liveAfter.State)
	}

	// A restore that has begun writing rows is not abandoned by a clock.
	for _, state := range []synccatchup.State{synccatchup.StateTransferring, synccatchup.StateVerifying, synccatchup.StateRestoring} {
		if _, err := fresh.AdvanceCatchup(ctx, live.ID, state, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fresh.ExpireCatchupSessions(ctx, now.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	restoring, _ := fresh.GetCatchupSession(ctx, live.ID)
	if restoring.State != synccatchup.StateRestoring {
		t.Fatalf("a restore in progress was expired: %s", restoring.State)
	}

	// Cancellation before any canonical write is fine, and is terminal.
	cancelled := newSession(now.Add(time.Hour))
	if _, err := fresh.AdvanceCatchup(ctx, cancelled.ID, synccatchup.StateCancelled, "user cancelled"); err != nil {
		t.Fatal(err)
	}
	after, _ := fresh.GetCatchupSession(ctx, cancelled.ID)
	if after.State != synccatchup.StateCancelled || after.LastReason != "user cancelled" {
		t.Fatalf("cancelled session = %+v", after)
	}
}

func TestSchemaV24AddsCatchupTables(t *testing.T) {
	ctx := context.Background()
	st := newAdmissionReplica(t, "db_g10_schema")
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema status: %+v err=%v", status, err)
	}
	for _, table := range []string{"sync_catchup_sessions", "sync_catchup_permissions"} {
		rows := queryRows(t, st, `SELECT COUNT(*) AS total FROM sqlite_master WHERE type='table' AND name=?`, table)
		if rows[0]["total"] != "1" {
			t.Fatalf("%s is missing", table)
		}
	}
}

// A replica built from a snapshot has canonical state without the operations
// that produced it. Admission must accept the operation after the snapshot
// anyway, and must treat one the snapshot already contains as an inert
// duplicate rather than a conflict.
func TestSnapshotFloorLetsAdmissionResumeAfterCutover(t *testing.T) {
	ctx := context.Background()
	source, fresh, sourceHandshake, now := catchupFixture(t)
	if err := fresh.PermitSnapshotSource(ctx, sourceHandshake.ReplicaID, true); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 6; index++ {
		if _, err := source.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_floor_%d", index), Title: "Note", Body: "body\n",
		}); err != nil {
			t.Fatal(err)
		}
	}
	handshake := refreshHandshake(t, source)
	all := localOperations(t, source, handshake.ReplicaID)
	if len(all) < 6 {
		t.Fatalf("fixture produced %d operations", len(all))
	}
	cut := len(all) / 2
	snapshotVector := syncstate.Vector{handshake.ReplicaID: all[cut-1].Sequence}

	signer, _ := syncwire.NewMemorySigner()
	request, err := synccatchup.NewRequest("db_g10", "replica-fresh", CurrentSchemaVersion, synccatchup.WrapPeerKey, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	session, err := fresh.BeginCatchup(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	response := synccatchup.Response{
		DatabaseID: "db_g10", ResponderReplicaID: handshake.ReplicaID, RequestNonce: request.Nonce,
		SnapshotID: "snap_floor", SnapshotVector: snapshotVector,
		ArchiveSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ArchiveLength: 1, WrappingMode: synccatchup.WrapPeerKey,
		ExpiresAt: now.Add(synccatchup.DefaultTTL).Format(time.RFC3339Nano),
	}
	offer := synccatchup.Offer{Response: response, Signature: synccatchup.SignResponse(signer, response)}
	if session, err = fresh.AcceptCatchupOffer(ctx, session.ID, offer); err != nil {
		t.Fatal(err)
	}
	for _, state := range []synccatchup.State{synccatchup.StateTransferring, synccatchup.StateVerifying, synccatchup.StateRestoring} {
		if _, err := fresh.AdvanceCatchup(ctx, session.ID, state, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := fresh.SetCatchupIntent(ctx, session.ID, "adopt"); err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.CutoverToSnapshot(ctx, session.ID); err != nil {
		t.Fatal(err)
	}

	// The operations the snapshot already contains are inert.
	result, err := fresh.AdmitSyncOperations(ctx, handshake, all[:cut])
	if err != nil {
		t.Fatalf("re-sending contained operations must not be a conflict: %v", err)
	}
	if result.Admitted != 0 || result.Duplicates != cut {
		t.Fatalf("contained operations: %+v", result)
	}
	// And the operation immediately after the floor is contiguous, even though
	// no predecessor row exists.
	result, err = fresh.AdmitSyncOperations(ctx, handshake, all[cut:])
	if err != nil {
		t.Fatalf("the tail after a snapshot did not admit: %v", err)
	}
	if result.Admitted != len(all)-cut {
		t.Fatalf("tail: %+v", result)
	}
	vector, err := fresh.SyncStateVector(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if vector[handshake.ReplicaID] != all[len(all)-1].Sequence {
		t.Fatalf("vector after the tail = %+v", vector)
	}
	// Without a floor, a missing predecessor is still a conflict: the floor is
	// the only thing allowed to stand in for one.
	other := newAdmissionReplica(t, "db_g10")
	if err := other.ConfigureSyncAdmissionPeer(ctx, handshake); err != nil {
		t.Fatal(err)
	}
	without, err := other.AdmitSyncOperations(ctx, handshake, all[cut:])
	if err != nil {
		t.Fatal(err)
	}
	// It is not an error — a gap is an ordinary state — but nothing is
	// admitted: the operations wait for the history that would make them
	// contiguous, which is exactly what a snapshot supplies instead.
	if without.Admitted != 0 || without.Pending == 0 {
		t.Fatalf("a replica with no snapshot admitted a history with a hole in it: %+v", without)
	}
}
