package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/renesugar/notrios/internal/syncstate"
)

// SnapshotActivationReport describes the new writable identity and the
// bounded incremental boundary installed into a staged physical image.
type SnapshotActivationReport struct {
	DatabaseID       string
	SourceReplicaID  string
	NewReplicaID     string
	SnapshotID       string
	VectorEntries    int
	FloorEntries     int
	DerivedDocuments int64
}

// ActivatePhysicalSnapshot turns a verified, private staged image into a new
// writable replica. The caller must not expose the image at the live database
// path until this succeeds.
//
// The image contains the source allocator as historical evidence. Rotation
// retires that allocator, enrollment captures a new sequence-zero baseline,
// and this method then makes the source an active peer and installs the
// snapshot vector/floors for the new observer. It writes no peer
// acknowledgement: possessing a backup is not an acknowledgement.
func (s *SQLiteStore) ActivatePhysicalSnapshot(ctx context.Context, snapshotID, sourceReplicaID string, vector, floors syncstate.Vector) (SnapshotActivationReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SnapshotActivationReport{}, err
	}
	if snapshotID == "" || sourceReplicaID == "" {
		return SnapshotActivationReport{}, fmt.Errorf("%w: snapshot and source replica ids are required", ErrInvalidInput)
	}
	if err := syncstate.ValidateVector(vector); err != nil {
		return SnapshotActivationReport{}, err
	}
	if err := syncstate.ValidateVector(floors); err != nil {
		return SnapshotActivationReport{}, err
	}
	before, err := s.GetDatabaseIdentity(ctx)
	if err != nil {
		return SnapshotActivationReport{}, err
	}
	if before.ReplicaID != sourceReplicaID {
		return SnapshotActivationReport{}, fmt.Errorf("%w: staged image source replica does not match the verified manifest", ErrConflict)
	}
	rotated, err := s.RotateReplicaIdentity(ctx)
	if err != nil {
		return SnapshotActivationReport{}, err
	}
	if _, err := s.EnrollLocalJournal(ctx, "physical snapshot restore "+snapshotID); err != nil {
		return SnapshotActivationReport{}, err
	}

	auditID, err := NewID("audit")
	if err != nil {
		return SnapshotActivationReport{}, err
	}
	details, err := json.Marshal(map[string]string{
		"snapshot_id": snapshotID, "source_replica_id": sourceReplicaID,
		"new_replica_id": rotated.ReplicaID,
	})
	if err != nil {
		return SnapshotActivationReport{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SnapshotActivationReport{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`UPDATE sync_replicas SET role = 'peer', status = 'active', retired_at = NULL
		WHERE replica_id = ?`, sourceReplicaID); err != nil {
		return SnapshotActivationReport{}, err
	}
	for subject, sequence := range vector {
		if subject == rotated.ReplicaID {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO sync_state_vectors(observer_replica_id, subject_replica_id, contiguous_sequence)
			VALUES(?, ?, ?) ON CONFLICT(observer_replica_id, subject_replica_id) DO UPDATE SET
			contiguous_sequence = MAX(sync_state_vectors.contiguous_sequence, excluded.contiguous_sequence),
			updated_at = CURRENT_TIMESTAMP`, rotated.ReplicaID, subject, strconv.FormatInt(sequence, 10)); err != nil {
			return SnapshotActivationReport{}, err
		}
		if err := s.execPreparedLocked(`INSERT INTO sync_catchup_floors(replica_id, sequence, snapshot_id) VALUES(?, ?, ?)
			ON CONFLICT(replica_id) DO UPDATE SET sequence = MAX(sync_catchup_floors.sequence, excluded.sequence),
			snapshot_id = excluded.snapshot_id, established_at = CURRENT_TIMESTAMP`, subject, strconv.FormatInt(sequence, 10), snapshotID); err != nil {
			return SnapshotActivationReport{}, err
		}
	}
	for subject, sequence := range floors {
		if err := s.execPreparedLocked(`INSERT INTO sync_catchup_floors(replica_id, sequence, snapshot_id) VALUES(?, ?, ?)
			ON CONFLICT(replica_id) DO UPDATE SET sequence = MAX(sync_catchup_floors.sequence, excluded.sequence),
			snapshot_id = excluded.snapshot_id, established_at = CURRENT_TIMESTAMP`, subject, strconv.FormatInt(sequence, 10), snapshotID); err != nil {
			return SnapshotActivationReport{}, err
		}
	}

	// SQLite FTS, parsed links, and blocks are admitted as exact-schema state.
	// Recoll and filesystem projections are external and never enter a physical
	// snapshot, so enqueue every current document with one set-based statement;
	// the ordinary bounded drain performs the rebuild after cutover.
	if err := s.execLocked(`DELETE FROM index_outbox;
		INSERT INTO index_outbox(object_type, object_id, operation)
		SELECT 'document', id, CASE WHEN deleted_at IS NULL THEN 'upsert' ELSE 'delete' END
		FROM documents ORDER BY id`); err != nil {
		return SnapshotActivationReport{}, err
	}
	derived, err := s.countLocked(`SELECT COUNT(*) FROM documents`, nil...)
	if err != nil {
		return SnapshotActivationReport{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'snapshot.physical_activated', ?, ?, ?)`, auditID, rotated.ReplicaID, snapshotID, string(details)); err != nil {
		return SnapshotActivationReport{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SnapshotActivationReport{}, err
	}
	committed = true
	return SnapshotActivationReport{
		DatabaseID: rotated.DatabaseID, SourceReplicaID: sourceReplicaID,
		NewReplicaID: rotated.ReplicaID, SnapshotID: snapshotID,
		VectorEntries: len(vector), FloorEntries: len(floors), DerivedDocuments: derived,
	}, nil
}
