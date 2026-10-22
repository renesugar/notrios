package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/renesugar/notrios/internal/syncstate"
)

// J23: two replicas paired before an upgrade must keep syncing once both have
// upgraded, without weakening what an explicit pairing guards.
//
// v1.0 J7-B found the defect with real binaries. Pairing stores the peer's
// exact handshake in sync_peer_compatibility (schema 27, range 24-27 on a
// pre-1.0 build). Once both replicas run 1.0, the peer reports 28 with range
// 24-28, admission requires an exact match, and every batch is refused.
//
// A 1.0 build cannot pair with a schema-27 handshake (ValidateHandshake
// refuses it), so each test configures the peer normally and then rewrites the
// stored row to what an upgraded library holds: the peer as it was at pairing.

// staleUpgradedPeer returns a receiver that has paired with source, with the
// stored compatibility rewound to the peer's pre-upgrade handshake, and the
// source's current handshake with one operation to admit.
func staleUpgradedPeer(t *testing.T, databaseID string, storedSchema, storedMax int) (*SQLiteStore, syncstate.Handshake, []syncstate.Operation) {
	t.Helper()
	ctx := context.Background()
	receiver := newAdmissionReplica(t, databaseID)
	source := newAdmissionReplica(t, databaseID)
	peer := refreshHandshake(t, source)
	if err := receiver.ConfigureSyncAdmissionPeer(ctx, peer); err != nil {
		t.Fatalf("ConfigureSyncAdmissionPeer: %v", err)
	}
	rewind := fmt.Sprintf(`UPDATE sync_peer_compatibility SET schema_version = %d, max_compatible_schema = %d WHERE replica_id = '%s'`,
		storedSchema, storedMax, peer.ReplicaID)
	if err := receiver.Exec(ctx, rewind); err != nil {
		t.Fatalf("rewind stored compatibility: %v", err)
	}
	operation := noopOperation(peer.ReplicaID, 1)
	peer.StateVector[peer.ReplicaID] = 1
	return receiver, peer, []syncstate.Operation{operation}
}

func storedPeerSchema(t *testing.T, st *SQLiteStore, replicaID string, schema, max int) bool {
	t.Helper()
	return syncCount(t, st, `SELECT COUNT(*) FROM sync_peer_compatibility
		WHERE replica_id = ? AND schema_version = `+fmt.Sprint(schema)+` AND max_compatible_schema = `+fmt.Sprint(max), replicaID) == 1
}

func upgradeAudits(t *testing.T, st *SQLiteStore, replicaID string) int64 {
	t.Helper()
	return syncCount(t, st, `SELECT COUNT(*) FROM sync_audit_events
		WHERE event_type = 'peer.compatibility_upgraded' AND subject_id = ?`, replicaID)
}

// J23-A: the defect J7-B found, as a failing test first.
func TestJ23UpgradedPeerIsAdmittedAndItsStoredCompatibilityUpdated(t *testing.T) {
	ctx := context.Background()
	if CurrentSchemaVersion < 28 || syncstate.MaxCompatibleSchema < 28 {
		t.Skipf("the upgrade case needs a schema-28 build; this one is %d, range up to %d", CurrentSchemaVersion, syncstate.MaxCompatibleSchema)
	}
	receiver, peer, operations := staleUpgradedPeer(t, "db_j23_upgrade", 27, 27)

	result, err := receiver.AdmitSyncOperations(ctx, peer, operations)
	if err != nil {
		t.Fatalf("an upgraded peer was refused: %v", err)
	}
	if result.Admitted != 1 {
		t.Fatalf("admitted %d operations, want 1", result.Admitted)
	}
	if !storedPeerSchema(t, receiver, peer.ReplicaID, peer.SchemaVersion, peer.MaxCompatibleSchema) {
		t.Fatalf("stored compatibility was not updated to schema %d, range up to %d", peer.SchemaVersion, peer.MaxCompatibleSchema)
	}
	if got := upgradeAudits(t, receiver, peer.ReplicaID); got != 1 {
		t.Fatalf("peer.compatibility_upgraded audit events = %d, want 1", got)
	}

	// Once updated, the peer matches its stored row, so a later batch is
	// admitted without recording another upgrade.
	next := noopOperation(peer.ReplicaID, 2)
	peer.StateVector[peer.ReplicaID] = 2
	if _, err := receiver.AdmitSyncOperations(ctx, peer, []syncstate.Operation{next}); err != nil {
		t.Fatalf("admission after the update: %v", err)
	}
	if got := upgradeAudits(t, receiver, peer.ReplicaID); got != 1 {
		t.Fatalf("a second batch recorded another upgrade: %d events", got)
	}
}

// J23-B: everything else an explicit pairing pins is still refused, and a
// refusal changes neither the stored row nor the audit log.
func TestJ23PinnedCompatibilityChangesAreStillRefused(t *testing.T) {
	ctx := context.Background()
	if CurrentSchemaVersion < 28 || syncstate.MaxCompatibleSchema < 28 {
		t.Skipf("needs a schema-28 build; this one is %d", CurrentSchemaVersion)
	}

	t.Run("a schema lower than the stored one", func(t *testing.T) {
		// Stored at 28; the peer now claims 27 with a range that still admits
		// this build, so ValidateHandshake passes and only the pinned check can
		// refuse.
		receiver, peer, operations := staleUpgradedPeer(t, "db_j23_downgrade", 28, 28)
		peer.SchemaVersion = 27
		if _, err := receiver.AdmitSyncOperations(ctx, peer, operations); !errors.Is(err, ErrConflict) {
			t.Fatalf("a lowered schema was not refused as a conflict: %v", err)
		}
		if !storedPeerSchema(t, receiver, peer.ReplicaID, 28, 28) || upgradeAudits(t, receiver, peer.ReplicaID) != 0 {
			t.Fatal("a refused downgrade changed the stored row or the audit log")
		}
	})

	t.Run("a schema rise together with a capability change", func(t *testing.T) {
		receiver, peer, operations := staleUpgradedPeer(t, "db_j23_capability", 27, 27)
		peer.OptionalCapabilities = []string{"sync.fixture.v1"}
		if _, err := receiver.AdmitSyncOperations(ctx, peer, operations); !errors.Is(err, ErrConflict) {
			t.Fatalf("a capability change riding on a schema rise was not refused: %v", err)
		}
		if !storedPeerSchema(t, receiver, peer.ReplicaID, 27, 27) || upgradeAudits(t, receiver, peer.ReplicaID) != 0 {
			t.Fatal("a refused capability change changed the stored row or the audit log")
		}
	})

	t.Run("a schema rise together with a protocol range change", func(t *testing.T) {
		receiver, peer, operations := staleUpgradedPeer(t, "db_j23_protocol", 27, 27)
		peer.ProtocolMaxMinor = peer.ProtocolMaxMinor + 1
		if _, err := receiver.AdmitSyncOperations(ctx, peer, operations); !errors.Is(err, ErrConflict) {
			t.Fatalf("a protocol range change riding on a schema rise was not refused: %v", err)
		}
		if !storedPeerSchema(t, receiver, peer.ReplicaID, 27, 27) || upgradeAudits(t, receiver, peer.ReplicaID) != 0 {
			t.Fatal("a refused protocol change changed the stored row or the audit log")
		}
	})

	t.Run("a schema rise together with a lower range floor", func(t *testing.T) {
		receiver, peer, operations := staleUpgradedPeer(t, "db_j23_floor", 27, 27)
		peer.MinCompatibleSchema = peer.MinCompatibleSchema - 1
		if _, err := receiver.AdmitSyncOperations(ctx, peer, operations); err == nil {
			t.Fatal("a changed range floor riding on a schema rise was admitted")
		}
		if !storedPeerSchema(t, receiver, peer.ReplicaID, 27, 27) || upgradeAudits(t, receiver, peer.ReplicaID) != 0 {
			t.Fatal("a refused range-floor change changed the stored row or the audit log")
		}
	})
}
