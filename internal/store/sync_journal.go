package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const MaxLocalOperationPage = 10_000

// SyncRecordClassification is the exact G4 record vocabulary. The class is a
// merge-model promise for later slices, not an implementation of convergence:
// scalar records are field registers, immutable records are indivisible, and
// memberships are independent elements.
type SyncRecordClassification struct {
	RecordType string
	Class      string
	Mutations  []string
}

var syncRecordClassifications = []SyncRecordClassification{
	{RecordType: "collection", Class: "field_register", Mutations: []string{"record.create", "record.update", "record.delete"}},
	{RecordType: "document", Class: "field_register", Mutations: []string{"record.create", "record.update", "document.trash", "document.restore", "document.purge"}},
	{RecordType: "revision", Class: "immutable_record", Mutations: []string{"record.create"}},
	{RecordType: "notebook", Class: "field_register", Mutations: []string{"record.create", "record.update", "record.delete"}},
	{RecordType: "tag", Class: "field_register", Mutations: []string{"record.create", "record.update", "record.delete"}},
	{RecordType: "document_tag", Class: "membership_element", Mutations: []string{"membership.add", "membership.remove"}},
	{RecordType: "search_notebook", Class: "field_register", Mutations: []string{"record.create", "record.update", "record.delete"}},
	{RecordType: "resource", Class: "immutable_record", Mutations: []string{"record.create", "record.update", "record.delete"}},
	{RecordType: "document_resource", Class: "membership_element", Mutations: []string{"membership.add", "membership.update", "membership.remove"}},
	{RecordType: "document_source", Class: "field_register", Mutations: []string{"record.create", "record.update", "record.delete"}},
	{RecordType: "source_bundle_item", Class: "immutable_record", Mutations: []string{"record.create", "record.update", "record.delete"}},
}

func SyncRecordClassifications() []SyncRecordClassification {
	out := make([]SyncRecordClassification, len(syncRecordClassifications))
	for i, item := range syncRecordClassifications {
		out[i] = item
		out[i].Mutations = append([]string(nil), item.Mutations...)
	}
	return out
}

// SyncJournalStatus reports only local durable journal state. It does not
// claim that a transport, peer, or cryptographic session exists.
type SyncJournalStatus struct {
	Enabled            bool
	ReplicaID          string
	LastSequence       int64
	SnapshotBoundaryID string
	EnabledAt          time.Time
}

// SyncOperation is one immutable local operation. PayloadJSON is deliberately
// readable in G4; G9 owns the bounded canonical wire codec and encryption.
type SyncOperation struct {
	ReplicaID   string
	Sequence    int64
	OperationID string
	Kind        string
	RecordType  string
	RecordID    string
	PayloadJSON string
	HLCWallMS   int64
	HLCLogical  int64
	CreatedAt   time.Time
}

// EnrollLocalJournal establishes the one-time full-snapshot boundary and then
// enables capture. Existing canonical rows are represented by that boundary;
// they are never replayed into an unbounded synthetic pre-enrollment log.
func (s *SQLiteStore) EnrollLocalJournal(ctx context.Context, reason string) (SyncJournalStatus, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJournalStatus{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "explicit sync enrollment"
	}
	if len(reason) > 256 {
		return SyncJournalStatus{}, fmt.Errorf("%w: enrollment reason is over 256 bytes", ErrInvalidInput)
	}
	boundaryID, err := NewID("boundary")
	if err != nil {
		return SyncJournalStatus{}, err
	}
	auditID, err := NewID("audit")
	if err != nil {
		return SyncJournalStatus{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if status, found, err := s.journalStatusLocked(); err != nil {
		return SyncJournalStatus{}, err
	} else if found {
		identity, identityErr := s.getDatabaseIdentityLocked()
		if identityErr != nil {
			return SyncJournalStatus{}, identityErr
		}
		if status.ReplicaID != identity.ReplicaID {
			return SyncJournalStatus{}, fmt.Errorf("%w: journal replica %q does not match database replica %q", ErrConflict, status.ReplicaID, identity.ReplicaID)
		}
		return status, nil
	}
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return SyncJournalStatus{}, err
	}
	vectorBytes, err := json.Marshal(map[string]int64{identity.ReplicaID: 0})
	if err != nil {
		return SyncJournalStatus{}, err
	}
	detailsBytes, err := json.Marshal(map[string]string{
		"boundary_id": boundaryID,
		"database_id": identity.DatabaseID,
		"reason":      reason,
	})
	if err != nil {
		return SyncJournalStatus{}, err
	}

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncJournalStatus{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.captureSyncMetadataBaselineLocked(true); err != nil {
		return SyncJournalStatus{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_replicas(replica_id, database_id, role, status)
		VALUES(?, ?, 'local', 'active')
		ON CONFLICT(replica_id) DO UPDATE SET database_id = excluded.database_id,
			role = 'local', status = 'active', retired_at = NULL`, identity.ReplicaID, identity.DatabaseID); err != nil {
		return SyncJournalStatus{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_snapshot_boundaries(id, replica_id, sequence, state_vector_json, reason)
		VALUES(?, ?, 0, ?, ?)`, boundaryID, identity.ReplicaID, string(vectorBytes), reason); err != nil {
		return SyncJournalStatus{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_local_journal(singleton, replica_id, last_sequence, snapshot_boundary_id)
		VALUES(1, ?, 0, ?)`, identity.ReplicaID, boundaryID); err != nil {
		return SyncJournalStatus{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_state_vectors(observer_replica_id, subject_replica_id, contiguous_sequence)
		VALUES(?, ?, 0)`, identity.ReplicaID, identity.ReplicaID); err != nil {
		return SyncJournalStatus{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'journal.enrolled', ?, ?, ?)`, auditID, identity.ReplicaID, boundaryID, string(detailsBytes)); err != nil {
		return SyncJournalStatus{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncJournalStatus{}, err
	}
	committed = true
	status, _, err := s.journalStatusLocked()
	return status, err
}

func (s *SQLiteStore) JournalStatus(ctx context.Context) (SyncJournalStatus, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJournalStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	status, _, err := s.journalStatusLocked()
	return status, err
}

func (s *SQLiteStore) journalStatusLocked() (SyncJournalStatus, bool, error) {
	stmt, err := s.prepareLocked(`SELECT replica_id, last_sequence, snapshot_boundary_id, enabled_at
		FROM sync_local_journal WHERE singleton = 1`)
	if err != nil {
		return SyncJournalStatus{}, false, err
	}
	defer C.sqlite3_finalize(stmt)
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return SyncJournalStatus{}, false, nil
	}
	if rc != C.SQLITE_ROW {
		return SyncJournalStatus{}, false, s.stepErrLocked(rc)
	}
	enabledAt, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 3)))
	if err != nil {
		return SyncJournalStatus{}, false, fmt.Errorf("parse sync journal enabled_at: %w", err)
	}
	return SyncJournalStatus{
		Enabled:            true,
		ReplicaID:          columnText(stmt, 0),
		LastSequence:       int64(C.sqlite3_column_int64(stmt, 1)),
		SnapshotBoundaryID: columnText(stmt, 2),
		EnabledAt:          enabledAt,
	}, true, nil
}

// ListLocalOperations returns a bounded sequence page for G4 validation and
// later G5 range planning. It is an internal store API, not a REST/MCP surface.
func (s *SQLiteStore) ListLocalOperations(ctx context.Context, afterSequence int64, limit int) ([]SyncOperation, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if afterSequence < 0 {
		return nil, fmt.Errorf("%w: after sequence cannot be negative", ErrInvalidInput)
	}
	if limit <= 0 || limit > MaxLocalOperationPage {
		return nil, fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidInput, MaxLocalOperationPage)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	status, found, err := s.journalStatusLocked()
	if err != nil || !found {
		return []SyncOperation{}, err
	}
	stmt, err := s.prepareLocked(`SELECT replica_id, sequence, operation_id, kind, record_type, record_id, payload_json, hlc_wall_ms, hlc_logical, created_at
		FROM sync_operations WHERE replica_id = ? AND sequence > ? ORDER BY sequence LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{status.ReplicaID, fmt.Sprintf("%d", afterSequence), fmt.Sprintf("%d", limit)}); err != nil {
		return nil, err
	}
	operations := []SyncOperation{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			createdAt, parseErr := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 9)))
			if parseErr != nil {
				return nil, fmt.Errorf("parse sync operation created_at: %w", parseErr)
			}
			operations = append(operations, SyncOperation{
				ReplicaID:   columnText(stmt, 0),
				Sequence:    int64(C.sqlite3_column_int64(stmt, 1)),
				OperationID: columnText(stmt, 2),
				Kind:        columnText(stmt, 3),
				RecordType:  columnText(stmt, 4),
				RecordID:    columnText(stmt, 5),
				PayloadJSON: columnText(stmt, 6),
				HLCWallMS:   columnInt64(stmt, 7),
				HLCLogical:  columnInt64(stmt, 8),
				CreatedAt:   createdAt,
			})
		case C.SQLITE_DONE:
			return operations, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) syncJournalCountForTest(query string, values ...string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.countLocked(query, values...)
}

func (s *SQLiteStore) syncJournalTextForTest(query string, values ...string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(query)
	if err != nil {
		return "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return "", err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_ROW {
		return "", s.stepErrLocked(rc)
	}
	return columnText(stmt, 0), nil
}

// retireLocalJournalLocked disconnects sequence allocation from an identity
// that is about to stop being this writable copy. Historical operations and
// its snapshot boundary remain immutable; the new replica must explicitly
// enroll and starts from a new full-snapshot boundary.
func (s *SQLiteStore) retireLocalJournalLocked(auditID, newReplicaID, reason string) error {
	status, found, err := s.journalStatusLocked()
	if err != nil || !found {
		return err
	}
	detailsBytes, err := json.Marshal(map[string]string{
		"new_replica_id": newReplicaID,
		"reason":         reason,
	})
	if err != nil {
		return err
	}
	if err := s.execPreparedLocked(`UPDATE sync_replicas
		SET status = 'retired', retired_at = CURRENT_TIMESTAMP
		WHERE replica_id = ? AND role = 'local'`, status.ReplicaID); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`DELETE FROM sync_local_journal WHERE singleton = 1`); err != nil {
		return err
	}
	return s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'journal.identity_retired', ?, ?, ?)`, auditID, status.ReplicaID, newReplicaID, string(detailsBytes))
}
