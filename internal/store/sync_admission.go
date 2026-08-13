package store

/*
#include <sqlite3.h>
#include <stdlib.h>

static int notes_sync_bind_blob(sqlite3_stmt *stmt, int idx, void *value, int size) {
	return sqlite3_bind_blob(stmt, idx, value, size, SQLITE_TRANSIENT);
}

static int notes_sync_bind_text(sqlite3_stmt *stmt, int idx, char *value) {
	return sqlite3_bind_text(stmt, idx, value, -1, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
	"unsafe"

	"github.com/renesugar/notrios/internal/syncstate"
)

// SyncAdmissionResult is returned only after the admission transaction commits.
// Its acknowledgement is therefore a durable contiguous vector, not a receipt
// for pending or rejected bytes.
type SyncAdmissionResult struct {
	Received        int
	Admitted        int
	Pending         int
	Duplicates      int
	Acknowledgement syncstate.Vector
}

// LocalSyncHandshake describes the local admission compatibility and durable
// vector. It is an internal value; G9/G13 still own authenticated artifacts and
// peer enrollment, and no REST/MCP surface exposes it in G5.
func (s *SQLiteStore) LocalSyncHandshake(ctx context.Context) (syncstate.Handshake, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return syncstate.Handshake{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.localSyncHandshakeLocked()
}

func (s *SQLiteStore) localSyncHandshakeLocked() (syncstate.Handshake, error) {
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return syncstate.Handshake{}, err
	}
	status, found, err := s.journalStatusLocked()
	if err != nil {
		return syncstate.Handshake{}, err
	}
	if !found || status.ReplicaID != identity.ReplicaID {
		return syncstate.Handshake{}, fmt.Errorf("%w: local sync journal is not enrolled", ErrConflict)
	}
	vector, err := s.syncStateVectorLocked(identity.ReplicaID)
	if err != nil {
		return syncstate.Handshake{}, err
	}
	return syncstate.NewHandshake(identity.DatabaseID, identity.ReplicaID, CurrentSchemaVersion, vector), nil
}

// ConfigureSyncAdmissionPeer is the explicit local G5 fixture seam. A
// successful compatibility handshake does not call this method itself and can
// never create a peer. Production proof-of-possession enrollment remains G13.
func (s *SQLiteStore) ConfigureSyncAdmissionPeer(ctx context.Context, peer syncstate.Handshake) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	auditID, err := NewID("audit")
	if err != nil {
		return err
	}
	requiredJSON, err := json.Marshal(peer.RequiredCapabilities)
	if err != nil {
		return err
	}
	optionalJSON, err := json.Marshal(peer.OptionalCapabilities)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return err
	}
	if err := syncstate.ValidateHandshake(local.DatabaseID, local.ReplicaID, CurrentSchemaVersion, peer); err != nil {
		return err
	}
	configured, err := s.syncPeerCompatibilityLocked(peer.ReplicaID)
	if err != nil {
		return err
	}
	if configured != nil {
		if !sameSyncCompatibility(*configured, peer) {
			return fmt.Errorf("%w: configured peer compatibility changed", ErrConflict)
		}
		role, databaseID, status, found, err := s.syncReplicaLocked(peer.ReplicaID)
		if err != nil {
			return err
		}
		if !found || role != "peer" || status != "active" || databaseID != peer.DatabaseID {
			return fmt.Errorf("%w: configured peer is not active for this database", ErrConflict)
		}
		return nil
	}
	detailsJSON, err := json.Marshal(map[string]any{
		"protocol_major": peer.ProtocolMajor,
		"schema_version": peer.SchemaVersion,
	})
	if err != nil {
		return err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`INSERT INTO sync_replicas(replica_id, database_id, role, status, capabilities_json)
		VALUES(?, ?, 'peer', 'active', ?)
		ON CONFLICT(replica_id) DO NOTHING`, peer.ReplicaID, peer.DatabaseID, string(requiredJSON)); err != nil {
		return err
	}
	role, databaseID, status, found, err := s.syncReplicaLocked(peer.ReplicaID)
	if err != nil {
		return err
	}
	if !found || role != "peer" || status != "active" || databaseID != peer.DatabaseID {
		return fmt.Errorf("%w: replica is not an active peer for this database", ErrConflict)
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_peer_compatibility(
		replica_id, protocol_major, protocol_min_minor, protocol_max_minor,
		schema_version, min_compatible_schema, max_compatible_schema,
		required_capabilities_json, optional_capabilities_json
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		peer.ReplicaID,
		strconv.Itoa(peer.ProtocolMajor), strconv.Itoa(peer.ProtocolMinMinor), strconv.Itoa(peer.ProtocolMaxMinor),
		strconv.Itoa(peer.SchemaVersion), strconv.Itoa(peer.MinCompatibleSchema), strconv.Itoa(peer.MaxCompatibleSchema),
		string(requiredJSON), string(optionalJSON)); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_state_vectors(observer_replica_id, subject_replica_id, contiguous_sequence)
		VALUES(?, ?, 0) ON CONFLICT(observer_replica_id, subject_replica_id) DO NOTHING`, local.ReplicaID, peer.ReplicaID); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'peer.admission_configured', ?, ?, ?)`, auditID, local.ReplicaID, peer.ReplicaID, string(detailsJSON)); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLiteStore) SyncStateVector(ctx context.Context) (syncstate.Vector, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return nil, err
	}
	return s.syncStateVectorLocked(identity.ReplicaID)
}

func (s *SQLiteStore) syncStateVectorLocked(observerReplicaID string) (syncstate.Vector, error) {
	stmt, err := s.prepareLocked(`SELECT subject_replica_id, contiguous_sequence
		FROM sync_state_vectors WHERE observer_replica_id = ? ORDER BY subject_replica_id
		LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{observerReplicaID, strconv.Itoa(syncstate.MaxStateVectorEntries + 1)}); err != nil {
		return nil, err
	}
	vector := syncstate.Vector{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			if len(vector) == syncstate.MaxStateVectorEntries {
				return nil, fmt.Errorf("%w: persisted state vector exceeds limit", syncstate.ErrInvalidState)
			}
			vector[columnText(stmt, 0)] = columnInt64(stmt, 1)
		case C.SQLITE_DONE:
			return vector, syncstate.ValidateVector(vector)
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// AdmitSyncOperations durably queues or admits operations from one explicitly
// configured source replica. G6 metadata is applied in the same transaction;
// G7/G8-owned body and resource records remain admitted but unapplied.
func (s *SQLiteStore) AdmitSyncOperations(ctx context.Context, peer syncstate.Handshake, operations []syncstate.Operation) (SyncAdmissionResult, error) {
	return s.admitSyncOperations(ctx, peer, operations, nil)
}

func (s *SQLiteStore) admitSyncOperations(ctx context.Context, peer syncstate.Handshake, operations []syncstate.Operation, beforeCommit func() error) (SyncAdmissionResult, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncAdmissionResult{}, err
	}
	if len(operations) > syncstate.MaxPendingOperations {
		return SyncAdmissionResult{}, fmt.Errorf("%w: admission batch exceeds %d operations", ErrInvalidInput, syncstate.MaxPendingOperations)
	}
	type normalizedOperation struct {
		operation syncstate.Operation
		encoded   []byte
	}
	normalized := make([]normalizedOperation, 0, len(operations))
	byID := map[string][]byte{}
	inputDuplicates := 0
	var batchBytes int64
	for index, operation := range operations {
		if index%128 == 0 {
			if err := ctx.Err(); err != nil {
				return SyncAdmissionResult{}, err
			}
		}
		if operation.ReplicaID != peer.ReplicaID {
			return SyncAdmissionResult{}, fmt.Errorf("%w: operation source does not match handshake replica", ErrInvalidInput)
		}
		if operation.Sequence > peer.StateVector[peer.ReplicaID] {
			return SyncAdmissionResult{}, fmt.Errorf("%w: operation sequence exceeds the source's advertised contiguous vector", ErrInvalidInput)
		}
		operation, encoded, err := syncstate.NormalizeOperation(operation)
		if err != nil {
			return SyncAdmissionResult{}, err
		}
		if !knownSyncOperation(operation.RecordType, operation.Kind) {
			return SyncAdmissionResult{}, fmt.Errorf("%w: unknown required operation %q/%q", ErrInvalidInput, operation.RecordType, operation.Kind)
		}
		if prior, found := byID[operation.OperationID]; found {
			if !bytes.Equal(prior, encoded) {
				return SyncAdmissionResult{}, fmt.Errorf("%w: conflicting operation replay %q", ErrConflict, operation.OperationID)
			}
			inputDuplicates++
			continue
		}
		byID[operation.OperationID] = encoded
		batchBytes += int64(len(encoded))
		if batchBytes > syncstate.MaxAdmissionBatchBytes {
			return SyncAdmissionResult{}, fmt.Errorf("%w: admission batch exceeds %d encoded bytes", ErrInvalidInput, syncstate.MaxAdmissionBatchBytes)
		}
		normalized = append(normalized, normalizedOperation{operation: operation, encoded: encoded})
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].operation.Sequence < normalized[j].operation.Sequence })

	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return SyncAdmissionResult{}, err
	}
	if err := syncstate.ValidateHandshake(local.DatabaseID, local.ReplicaID, CurrentSchemaVersion, peer); err != nil {
		return SyncAdmissionResult{}, err
	}
	if err := s.validateConfiguredSyncPeerLocked(peer); err != nil {
		return SyncAdmissionResult{}, err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncAdmissionResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	result := SyncAdmissionResult{Received: len(operations), Duplicates: inputDuplicates}
	for index, item := range normalized {
		if index%128 == 0 {
			if err := ctx.Err(); err != nil {
				return SyncAdmissionResult{}, err
			}
		}
		contiguous, err := s.syncContiguousLocked(local.ReplicaID, item.operation.ReplicaID)
		if err != nil {
			return SyncAdmissionResult{}, err
		}
		if item.operation.Sequence > contiguous && item.operation.Sequence-contiguous > syncstate.MaxSequenceSkew {
			return SyncAdmissionResult{}, fmt.Errorf("%w: operation sequence %d is more than %d ahead of contiguous %d", ErrInvalidInput, item.operation.Sequence, syncstate.MaxSequenceSkew, contiguous)
		}
		stored, found, err := s.syncStoredOperationBytesLocked(item.operation.ReplicaID, item.operation.Sequence, item.operation.OperationID)
		if err != nil {
			return SyncAdmissionResult{}, err
		}
		if found {
			if !bytes.Equal(stored, item.encoded) {
				return SyncAdmissionResult{}, fmt.Errorf("%w: conflicting admitted replay %q", ErrConflict, item.operation.OperationID)
			}
			result.Duplicates++
			continue
		}
		pending, found, err := s.syncPendingOperationBytesLocked(item.operation.ReplicaID, item.operation.Sequence, item.operation.OperationID)
		if err != nil {
			return SyncAdmissionResult{}, err
		}
		if found {
			if !bytes.Equal(pending, item.encoded) {
				return SyncAdmissionResult{}, fmt.Errorf("%w: conflicting pending replay %q", ErrConflict, item.operation.OperationID)
			}
			result.Duplicates++
			continue
		}
		if item.operation.Sequence <= contiguous {
			return SyncAdmissionResult{}, fmt.Errorf("%w: sequence %d is below the durable vector without an admitted operation", ErrConflict, item.operation.Sequence)
		}
		if err := s.insertSyncPendingLocked(item.operation, item.encoded); err != nil {
			return SyncAdmissionResult{}, err
		}
	}
	count, encodedBytes, err := s.syncPendingUsageLocked(peer.ReplicaID)
	if err != nil {
		return SyncAdmissionResult{}, err
	}
	if count > syncstate.MaxPendingOperations || encodedBytes > syncstate.MaxPendingBytes {
		return SyncAdmissionResult{}, fmt.Errorf("%w: pending admission limit exceeded for replica %q", ErrConflict, peer.ReplicaID)
	}
	var touchedDocuments []string
	result.Admitted, touchedDocuments, err = s.drainSyncPendingLocked(local.ReplicaID)
	if err != nil {
		return SyncAdmissionResult{}, err
	}
	if result.Admitted > 0 {
		if err := s.reconcileSyncMetadataLocked(); err != nil {
			return SyncAdmissionResult{}, err
		}
		// Bodies converge after metadata, in the same transaction, so a merge
		// sees the title and lifecycle the batch settled rather than the ones
		// it started with.
		if err := s.reconcileSyncRevisionsLocked(touchedDocuments); err != nil {
			return SyncAdmissionResult{}, err
		}
	}
	if err := s.rebuildSyncGapsLocked(local.ReplicaID); err != nil {
		return SyncAdmissionResult{}, err
	}
	result.Pending, _, err = s.syncPendingUsageLocked(peer.ReplicaID)
	if err != nil {
		return SyncAdmissionResult{}, err
	}
	result.Acknowledgement, err = s.syncStateVectorLocked(local.ReplicaID)
	if err != nil {
		return SyncAdmissionResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return SyncAdmissionResult{}, err
	}
	if beforeCommit != nil {
		if err := beforeCommit(); err != nil {
			return SyncAdmissionResult{}, err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncAdmissionResult{}, err
	}
	committed = true
	return result, nil
}

func (s *SQLiteStore) insertSyncPendingLocked(operation syncstate.Operation, encoded []byte) error {
	stmt, err := s.prepareLocked(`INSERT INTO sync_pending_admissions(
		replica_id, sequence, operation_id, encoded_operation, encoded_size, reason
	) VALUES(?, ?, ?, ?, ?, 'awaiting_sequence_or_dependency')`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{operation.ReplicaID, strconv.FormatInt(operation.Sequence, 10), operation.OperationID}); err != nil {
		return err
	}
	var pointer unsafe.Pointer
	if len(encoded) > 0 {
		pointer = C.CBytes(encoded)
		defer C.free(pointer)
	}
	if rc := C.notes_sync_bind_blob(stmt, 4, pointer, C.int(len(encoded))); rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite bind pending operation blob failed")
	}
	if err := bindTextAt(stmt, 5, strconv.Itoa(len(encoded))); err != nil {
		return err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_DONE {
		return s.stepErrLocked(rc)
	}
	return nil
}

func bindTextAt(stmt *C.sqlite3_stmt, index int, value string) error {
	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))
	if rc := C.notes_sync_bind_text(stmt, C.int(index), cvalue); rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite bind parameter %d failed", index)
	}
	return nil
}

// drainSyncPendingLocked admits every newly contiguous pending operation and
// reports the documents whose bodies a revision operation touched, so body
// convergence can be scoped to them rather than to the whole library.
func (s *SQLiteStore) drainSyncPendingLocked(observerReplicaID string) (int, []string, error) {
	admitted := 0
	var touchedDocuments []string
	for {
		replicas, err := s.syncPendingReplicaIDsLocked()
		if err != nil {
			return 0, nil, err
		}
		progress := false
		for _, replicaID := range replicas {
			contiguous, err := s.syncContiguousLocked(observerReplicaID, replicaID)
			if err != nil {
				return 0, nil, err
			}
			encoded, found, err := s.syncPendingAtLocked(replicaID, contiguous+1)
			if err != nil {
				return 0, nil, err
			}
			if !found {
				continue
			}
			operation, err := syncstate.DecodeOperation(encoded)
			if err != nil {
				return 0, nil, err
			}
			ready := true
			for _, dependency := range operation.Dependencies {
				exists, err := s.syncOperationExistsLocked(dependency.ReplicaID, dependency.Sequence)
				if err != nil {
					return 0, nil, err
				}
				if !exists {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			if err := s.insertAdmittedSyncOperationLocked(operation); err != nil {
				return 0, nil, err
			}
			if err := s.execPreparedLocked(`DELETE FROM sync_pending_admissions WHERE replica_id = ? AND sequence = ?`, replicaID, strconv.FormatInt(operation.Sequence, 10)); err != nil {
				return 0, nil, err
			}
			if err := s.execPreparedLocked(`INSERT INTO sync_state_vectors(observer_replica_id, subject_replica_id, contiguous_sequence)
				VALUES(?, ?, ?) ON CONFLICT(observer_replica_id, subject_replica_id) DO UPDATE SET
				contiguous_sequence = excluded.contiguous_sequence, updated_at = CURRENT_TIMESTAMP`,
				observerReplicaID, replicaID, strconv.FormatInt(operation.Sequence, 10)); err != nil {
				return 0, nil, err
			}
			admitted++
			progress = true
			if operation.RecordType == "revision" {
				var payload struct {
					DocumentID string `json:"document_id"`
				}
				if err := json.Unmarshal(operation.Payload, &payload); err != nil {
					return 0, nil, fmt.Errorf("revision operation payload: %w", err)
				}
				touchedDocuments = append(touchedDocuments, payload.DocumentID)
			}
		}
		if !progress {
			return admitted, touchedDocuments, nil
		}
	}
}

func (s *SQLiteStore) insertAdmittedSyncOperationLocked(operation syncstate.Operation) error {
	if operation.Sequence > 1 {
		stmt, err := s.prepareLocked(`SELECT hlc_wall_ms, hlc_logical FROM sync_operations WHERE replica_id=? AND sequence=?`)
		if err != nil {
			return err
		}
		if err := bindAll(stmt, []string{operation.ReplicaID, strconv.FormatInt(operation.Sequence-1, 10)}); err != nil {
			C.sqlite3_finalize(stmt)
			return err
		}
		rc := C.sqlite3_step(stmt)
		if rc != C.SQLITE_ROW {
			C.sqlite3_finalize(stmt)
			if rc == C.SQLITE_DONE {
				return fmt.Errorf("%w: admitted operation has no contiguous HLC predecessor", ErrConflict)
			}
			return s.stepErrLocked(rc)
		}
		priorWall, priorLogical := columnInt64(stmt, 0), columnInt64(stmt, 1)
		C.sqlite3_finalize(stmt)
		if operation.HLC.WallMS < priorWall || operation.HLC.WallMS == priorWall && operation.HLC.Logical < priorLogical {
			return fmt.Errorf("%w: replica HLC moved backward at sequence %d", ErrConflict, operation.Sequence)
		}
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_operations(
		replica_id, sequence, operation_id, kind, record_type, record_id, payload_json, hlc_wall_ms, hlc_logical, created_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, operation.ReplicaID, strconv.FormatInt(operation.Sequence, 10),
		operation.OperationID, operation.Kind, operation.RecordType, operation.RecordID, string(operation.Payload),
		strconv.FormatInt(operation.HLC.WallMS, 10), strconv.FormatInt(operation.HLC.Logical, 10), operation.CreatedAt); err != nil {
		return err
	}
	for _, dependency := range operation.Dependencies {
		if err := s.execPreparedLocked(`INSERT INTO sync_operation_dependencies(
			operation_replica_id, operation_sequence, dependency_replica_id, dependency_sequence
		) VALUES(?, ?, ?, ?)`, operation.ReplicaID, strconv.FormatInt(operation.Sequence, 10),
			dependency.ReplicaID, strconv.FormatInt(dependency.Sequence, 10)); err != nil {
			return err
		}
	}
	return nil
}

// PlanMissingSyncOperations plans only ranges the named remote reports as
// contiguous and omits operations already durably pending. Missing dependencies
// are listed separately and never misreported as a sequence gap.
func (s *SQLiteStore) PlanMissingSyncOperations(ctx context.Context, remote syncstate.Vector) (syncstate.MissingPlan, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return syncstate.MissingPlan{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return syncstate.MissingPlan{}, err
	}
	local, err := s.syncStateVectorLocked(identity.ReplicaID)
	if err != nil {
		return syncstate.MissingPlan{}, err
	}
	present, err := s.syncPendingRefsLocked()
	if err != nil {
		return syncstate.MissingPlan{}, err
	}
	plan, err := syncstate.PlanMissing(local, remote, present)
	if err != nil {
		return syncstate.MissingPlan{}, err
	}
	dependencies, more, err := s.syncMissingDependenciesLocked(remote)
	if err != nil {
		return syncstate.MissingPlan{}, err
	}
	plan.Dependencies = dependencies
	plan.MoreAvailable = plan.MoreAvailable || more
	return plan, nil
}

// RecordSyncPeerAcknowledgement records only monotonic positions no higher
// than this replica's own durable vector. A caller obtains the vector from a
// successful peer admission result; pending/rejected work cannot create one.
func (s *SQLiteStore) RecordSyncPeerAcknowledgement(ctx context.Context, peer syncstate.Handshake) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := syncstate.ValidateVector(peer.StateVector); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return err
	}
	if err := syncstate.ValidateHandshake(local.DatabaseID, local.ReplicaID, CurrentSchemaVersion, peer); err != nil {
		return err
	}
	if err := s.validateConfiguredSyncPeerLocked(peer); err != nil {
		return err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	for subjectReplicaID, sequence := range peer.StateVector {
		known := local.StateVector[subjectReplicaID]
		if subjectReplicaID != local.ReplicaID {
			_, databaseID, _, found, err := s.syncReplicaLocked(subjectReplicaID)
			if err != nil {
				return err
			}
			if !found || databaseID != local.DatabaseID {
				return fmt.Errorf("%w: acknowledgement names unknown replica %q", ErrConflict, subjectReplicaID)
			}
		}
		if sequence > known {
			return fmt.Errorf("%w: acknowledgement %s:%d exceeds durable local position %d", ErrConflict, subjectReplicaID, sequence, known)
		}
		prior, err := s.syncPeerAcknowledgementLocked(peer.ReplicaID, subjectReplicaID)
		if err != nil {
			return err
		}
		if sequence < prior {
			return fmt.Errorf("%w: acknowledgement moved backward for %q", ErrConflict, subjectReplicaID)
		}
		if err := s.execPreparedLocked(`INSERT INTO sync_peer_acknowledgements(peer_replica_id, subject_replica_id, contiguous_sequence)
			VALUES(?, ?, ?) ON CONFLICT(peer_replica_id, subject_replica_id) DO UPDATE SET
			contiguous_sequence = excluded.contiguous_sequence, acknowledged_at = CURRENT_TIMESTAMP`,
			peer.ReplicaID, subjectReplicaID, strconv.FormatInt(sequence, 10)); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// ListSyncOperations returns a bounded page for one source replica, including
// dependencies. It is used by local G5 model fixtures and is not a data plane.
func (s *SQLiteStore) ListSyncOperations(ctx context.Context, replicaID string, afterSequence int64, limit int) ([]syncstate.Operation, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if afterSequence < 0 || limit <= 0 || limit > MaxLocalOperationPage {
		return nil, fmt.Errorf("%w: invalid sync operation page", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT sequence, operation_id, kind, record_type, record_id, payload_json, hlc_wall_ms, hlc_logical, created_at
		FROM sync_operations WHERE replica_id = ? AND sequence > ? ORDER BY sequence LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, strconv.FormatInt(afterSequence, 10), strconv.Itoa(limit)}); err != nil {
		return nil, err
	}
	operations := []syncstate.Operation{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			createdAt, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 8)))
			if err != nil {
				return nil, err
			}
			operation := syncstate.Operation{
				ReplicaID: replicaID, Sequence: columnInt64(stmt, 0), OperationID: columnText(stmt, 1),
				Kind: columnText(stmt, 2), RecordType: columnText(stmt, 3), RecordID: columnText(stmt, 4),
				Payload: json.RawMessage(columnText(stmt, 5)), HLC: syncstate.HLC{WallMS: columnInt64(stmt, 6), Logical: columnInt64(stmt, 7)},
				CreatedAt:    createdAt.UTC().Format(time.RFC3339Nano),
				Dependencies: []syncstate.OperationRef{},
			}
			operation.Dependencies, err = s.syncOperationDependenciesLocked(replicaID, operation.Sequence)
			if err != nil {
				return nil, err
			}
			operations = append(operations, operation)
		case C.SQLITE_DONE:
			return operations, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func knownSyncOperation(recordType, kind string) bool {
	if recordType == "sync_noop" && kind == "sync.noop" {
		return true
	}
	for _, classification := range syncRecordClassifications {
		if classification.RecordType != recordType {
			continue
		}
		for _, mutation := range classification.Mutations {
			if mutation == kind {
				return true
			}
		}
	}
	return false
}

func sameSyncCompatibility(left, right syncstate.Handshake) bool {
	return left.ProtocolMajor == right.ProtocolMajor && left.ProtocolMinMinor == right.ProtocolMinMinor &&
		left.ProtocolMaxMinor == right.ProtocolMaxMinor && left.SchemaVersion == right.SchemaVersion &&
		left.MinCompatibleSchema == right.MinCompatibleSchema && left.MaxCompatibleSchema == right.MaxCompatibleSchema &&
		equalStrings(left.RequiredCapabilities, right.RequiredCapabilities) && equalStrings(left.OptionalCapabilities, right.OptionalCapabilities)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (s *SQLiteStore) syncPeerCompatibilityLocked(replicaID string) (*syncstate.Handshake, error) {
	stmt, err := s.prepareLocked(`SELECT protocol_major, protocol_min_minor, protocol_max_minor,
		schema_version, min_compatible_schema, max_compatible_schema,
		required_capabilities_json, optional_capabilities_json
		FROM sync_peer_compatibility WHERE replica_id = ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID}); err != nil {
		return nil, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return nil, nil
	case C.SQLITE_ROW:
		peer := &syncstate.Handshake{
			ReplicaID:     replicaID,
			ProtocolMajor: int(C.sqlite3_column_int(stmt, 0)), ProtocolMinMinor: int(C.sqlite3_column_int(stmt, 1)),
			ProtocolMaxMinor: int(C.sqlite3_column_int(stmt, 2)), SchemaVersion: int(C.sqlite3_column_int(stmt, 3)),
			MinCompatibleSchema: int(C.sqlite3_column_int(stmt, 4)), MaxCompatibleSchema: int(C.sqlite3_column_int(stmt, 5)),
		}
		if err := json.Unmarshal([]byte(columnText(stmt, 6)), &peer.RequiredCapabilities); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(columnText(stmt, 7)), &peer.OptionalCapabilities); err != nil {
			return nil, err
		}
		return peer, nil
	default:
		return nil, s.stepErrLocked(rc)
	}
}

func (s *SQLiteStore) validateConfiguredSyncPeerLocked(peer syncstate.Handshake) error {
	configured, err := s.syncPeerCompatibilityLocked(peer.ReplicaID)
	if err != nil {
		return err
	}
	if configured == nil {
		return fmt.Errorf("%w: peer %q is not explicitly configured for admission", ErrConflict, peer.ReplicaID)
	}
	if !sameSyncCompatibility(*configured, peer) {
		return fmt.Errorf("%w: peer compatibility differs from explicit configuration", ErrConflict)
	}
	role, databaseID, status, found, err := s.syncReplicaLocked(peer.ReplicaID)
	if err != nil {
		return err
	}
	if !found || role != "peer" || status != "active" || databaseID != peer.DatabaseID {
		return fmt.Errorf("%w: peer is not active for this database", ErrConflict)
	}
	return nil
}

func (s *SQLiteStore) syncReplicaLocked(replicaID string) (role, databaseID, status string, found bool, err error) {
	stmt, err := s.prepareLocked(`SELECT role, database_id, status FROM sync_replicas WHERE replica_id = ?`)
	if err != nil {
		return "", "", "", false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID}); err != nil {
		return "", "", "", false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return "", "", "", false, nil
	case C.SQLITE_ROW:
		return columnText(stmt, 0), columnText(stmt, 1), columnText(stmt, 2), true, nil
	default:
		return "", "", "", false, s.stepErrLocked(rc)
	}
}

func (s *SQLiteStore) syncContiguousLocked(observer, subject string) (int64, error) {
	stmt, err := s.prepareLocked(`SELECT contiguous_sequence FROM sync_state_vectors
		WHERE observer_replica_id = ? AND subject_replica_id = ?`)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{observer, subject}); err != nil {
		return 0, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return 0, nil
	case C.SQLITE_ROW:
		return columnInt64(stmt, 0), nil
	default:
		return 0, s.stepErrLocked(rc)
	}
}

func (s *SQLiteStore) syncStoredOperationBytesLocked(replicaID string, sequence int64, operationID string) ([]byte, bool, error) {
	stmt, err := s.prepareLocked(`SELECT replica_id, sequence, operation_id, kind, record_type, record_id, payload_json, hlc_wall_ms, hlc_logical, created_at
		FROM sync_operations WHERE (replica_id = ? AND sequence = ?) OR operation_id = ? LIMIT 1`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, strconv.FormatInt(sequence, 10), operationID}); err != nil {
		return nil, false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return nil, false, nil
	case C.SQLITE_ROW:
		storedReplicaID := columnText(stmt, 0)
		storedSequence := columnInt64(stmt, 1)
		createdAt, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 9)))
		if err != nil {
			return nil, false, err
		}
		dependencies, err := s.syncOperationDependenciesLocked(storedReplicaID, storedSequence)
		if err != nil {
			return nil, false, err
		}
		_, encoded, err := syncstate.NormalizeOperation(syncstate.Operation{
			ReplicaID: storedReplicaID, Sequence: storedSequence, OperationID: columnText(stmt, 2), Kind: columnText(stmt, 3),
			RecordType: columnText(stmt, 4), RecordID: columnText(stmt, 5), Payload: json.RawMessage(columnText(stmt, 6)),
			HLC:       syncstate.HLC{WallMS: columnInt64(stmt, 7), Logical: columnInt64(stmt, 8)},
			CreatedAt: createdAt.UTC().Format(time.RFC3339Nano), Dependencies: dependencies,
		})
		return encoded, true, err
	default:
		return nil, false, s.stepErrLocked(rc)
	}
}

func (s *SQLiteStore) syncPendingOperationBytesLocked(replicaID string, sequence int64, operationID string) ([]byte, bool, error) {
	stmt, err := s.prepareLocked(`SELECT encoded_operation FROM sync_pending_admissions
		WHERE (replica_id = ? AND sequence = ?) OR operation_id = ? LIMIT 1`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, strconv.FormatInt(sequence, 10), operationID}); err != nil {
		return nil, false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return nil, false, nil
	case C.SQLITE_ROW:
		return columnBlob(stmt, 0), true, nil
	default:
		return nil, false, s.stepErrLocked(rc)
	}
}

func (s *SQLiteStore) syncPendingAtLocked(replicaID string, sequence int64) ([]byte, bool, error) {
	stmt, err := s.prepareLocked(`SELECT encoded_operation FROM sync_pending_admissions WHERE replica_id = ? AND sequence = ?`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, strconv.FormatInt(sequence, 10)}); err != nil {
		return nil, false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return nil, false, nil
	case C.SQLITE_ROW:
		return columnBlob(stmt, 0), true, nil
	default:
		return nil, false, s.stepErrLocked(rc)
	}
}

func columnBlob(stmt *C.sqlite3_stmt, index int) []byte {
	size := C.sqlite3_column_bytes(stmt, C.int(index))
	if size == 0 {
		return []byte{}
	}
	pointer := C.sqlite3_column_blob(stmt, C.int(index))
	return C.GoBytes(pointer, size)
}

func (s *SQLiteStore) syncOperationDependenciesLocked(replicaID string, sequence int64) ([]syncstate.OperationRef, error) {
	stmt, err := s.prepareLocked(`SELECT dependency_replica_id, dependency_sequence
		FROM sync_operation_dependencies WHERE operation_replica_id = ? AND operation_sequence = ?
		ORDER BY dependency_replica_id, dependency_sequence`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, strconv.FormatInt(sequence, 10)}); err != nil {
		return nil, err
	}
	dependencies := []syncstate.OperationRef{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			dependencies = append(dependencies, syncstate.OperationRef{ReplicaID: columnText(stmt, 0), Sequence: columnInt64(stmt, 1)})
		case C.SQLITE_DONE:
			return dependencies, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) syncOperationExistsLocked(replicaID string, sequence int64) (bool, error) {
	stmt, err := s.prepareLocked(`SELECT 1 FROM sync_operations WHERE replica_id = ? AND sequence = ?`)
	if err != nil {
		return false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, strconv.FormatInt(sequence, 10)}); err != nil {
		return false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return true, nil
	case C.SQLITE_DONE:
		return false, nil
	default:
		return false, s.stepErrLocked(rc)
	}
}

func (s *SQLiteStore) syncPendingReplicaIDsLocked() ([]string, error) {
	stmt, err := s.prepareLocked(`SELECT DISTINCT replica_id FROM sync_pending_admissions ORDER BY replica_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	replicas := []string{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			replicas = append(replicas, columnText(stmt, 0))
		case C.SQLITE_DONE:
			return replicas, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) syncPendingUsageLocked(replicaID string) (int, int64, error) {
	stmt, err := s.prepareLocked(`SELECT COUNT(*), COALESCE(SUM(encoded_size), 0)
		FROM sync_pending_admissions WHERE replica_id = ?`)
	if err != nil {
		return 0, 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID}); err != nil {
		return 0, 0, err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_ROW {
		return 0, 0, s.stepErrLocked(rc)
	}
	return int(C.sqlite3_column_int(stmt, 0)), columnInt64(stmt, 1), nil
}

func (s *SQLiteStore) syncPendingRefsLocked() ([]syncstate.OperationRef, error) {
	stmt, err := s.prepareLocked(`SELECT replica_id, sequence FROM sync_pending_admissions ORDER BY replica_id, sequence`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	refs := []syncstate.OperationRef{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			refs = append(refs, syncstate.OperationRef{ReplicaID: columnText(stmt, 0), Sequence: columnInt64(stmt, 1)})
		case C.SQLITE_DONE:
			return refs, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) syncMissingDependenciesLocked(remote syncstate.Vector) ([]syncstate.OperationRef, bool, error) {
	stmt, err := s.prepareLocked(`SELECT encoded_operation FROM sync_pending_admissions ORDER BY replica_id, sequence`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	missing := map[syncstate.OperationRef]bool{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			operation, err := syncstate.DecodeOperation(columnBlob(stmt, 0))
			if err != nil {
				return nil, false, err
			}
			for _, dependency := range operation.Dependencies {
				if remote[dependency.ReplicaID] < dependency.Sequence {
					continue
				}
				exists, err := s.syncOperationExistsLocked(dependency.ReplicaID, dependency.Sequence)
				if err != nil {
					return nil, false, err
				}
				if exists {
					continue
				}
				pending, _, err := s.syncPendingAtLocked(dependency.ReplicaID, dependency.Sequence)
				if err != nil {
					return nil, false, err
				}
				if len(pending) == 0 {
					missing[dependency] = true
				}
			}
		case C.SQLITE_DONE:
			refs := make([]syncstate.OperationRef, 0, len(missing))
			for ref := range missing {
				refs = append(refs, ref)
			}
			sort.Slice(refs, func(i, j int) bool {
				if refs[i].ReplicaID == refs[j].ReplicaID {
					return refs[i].Sequence < refs[j].Sequence
				}
				return refs[i].ReplicaID < refs[j].ReplicaID
			})
			if len(refs) > syncstate.MaxPendingOperations {
				return refs[:syncstate.MaxPendingOperations], true, nil
			}
			return refs, false, nil
		default:
			return nil, false, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) rebuildSyncGapsLocked(observerReplicaID string) error {
	if err := s.execPreparedLocked(`DELETE FROM sync_state_gaps WHERE observer_replica_id = ?`, observerReplicaID); err != nil {
		return err
	}
	replicas, err := s.syncPendingReplicaIDsLocked()
	if err != nil {
		return err
	}
	for _, replicaID := range replicas {
		contiguous, err := s.syncContiguousLocked(observerReplicaID, replicaID)
		if err != nil {
			return err
		}
		stmt, err := s.prepareLocked(`SELECT sequence FROM sync_pending_admissions WHERE replica_id = ? ORDER BY sequence`)
		if err != nil {
			return err
		}
		if err := bindAll(stmt, []string{replicaID}); err != nil {
			C.sqlite3_finalize(stmt)
			return err
		}
		cursor := contiguous + 1
		for {
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_DONE {
				break
			}
			if rc != C.SQLITE_ROW {
				C.sqlite3_finalize(stmt)
				return s.stepErrLocked(rc)
			}
			sequence := columnInt64(stmt, 0)
			if sequence > cursor {
				if err := s.execPreparedLocked(`INSERT INTO sync_state_gaps(
					observer_replica_id, subject_replica_id, start_sequence, end_sequence
				) VALUES(?, ?, ?, ?)`, observerReplicaID, replicaID,
					strconv.FormatInt(cursor, 10), strconv.FormatInt(sequence-1, 10)); err != nil {
					C.sqlite3_finalize(stmt)
					return err
				}
			}
			if sequence >= cursor {
				cursor = sequence + 1
			}
		}
		C.sqlite3_finalize(stmt)
	}
	return nil
}

func (s *SQLiteStore) syncPeerAcknowledgementLocked(peerReplicaID, subjectReplicaID string) (int64, error) {
	stmt, err := s.prepareLocked(`SELECT contiguous_sequence FROM sync_peer_acknowledgements
		WHERE peer_replica_id = ? AND subject_replica_id = ?`)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{peerReplicaID, subjectReplicaID}); err != nil {
		return 0, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return 0, nil
	case C.SQLITE_ROW:
		return columnInt64(stmt, 0), nil
	default:
		return 0, s.stepErrLocked(rc)
	}
}
