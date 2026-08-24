package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

var ErrSyncFullResyncRequired = errors.New("sync history was collected; a verified snapshot catch-up is required")

const (
	DefaultSyncHistoryRetention = 90 * 24 * time.Hour
	DefaultSyncPeerWarning      = 30 * 24 * time.Hour
	MaxRetirementReasonBytes    = 512
)

func (s *SQLiteStore) ensureSchemaV27(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	s.mu.Unlock()
	if err != nil || version >= 27 {
		return err
	}
	migration, err := migrationFS.ReadFile("migrations/0027_sync_retention.sql")
	if err != nil {
		return fmt.Errorf("read schema v27 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

type VerifiedSyncSnapshot struct {
	SnapshotID   string           `json:"snapshot_id"`
	CommitSHA256 string           `json:"commit_sha256"`
	CreatedAt    time.Time        `json:"created_at"`
	Vector       syncstate.Vector `json:"vector"`
}

// RecordVerifiedSyncSnapshot records only the aggregate boundary of a fully
// verified physical snapshot. It stores no path or backup bytes and is local
// retention evidence, not an acknowledgement by a peer.
func (s *SQLiteStore) RecordVerifiedSyncSnapshot(ctx context.Context, snapshot VerifiedSyncSnapshot) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshot.SnapshotID = strings.TrimSpace(snapshot.SnapshotID)
	snapshot.CommitSHA256 = strings.ToLower(strings.TrimSpace(snapshot.CommitSHA256))
	if snapshot.SnapshotID == "" || len(snapshot.CommitSHA256) != 64 {
		return fmt.Errorf("%w: verified snapshot identity is incomplete", ErrInvalidInput)
	}
	if _, err := hex.DecodeString(snapshot.CommitSHA256); err != nil {
		return fmt.Errorf("%w: verified snapshot commit is not SHA-256", ErrInvalidInput)
	}
	if err := syncstate.ValidateVector(snapshot.Vector); err != nil {
		return err
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	existing, err := s.countLocked(`SELECT COUNT(*) FROM sync_verified_snapshots
		WHERE snapshot_id = ? AND commit_sha256 <> ?`, snapshot.SnapshotID, snapshot.CommitSHA256)
	if err != nil {
		return err
	}
	if existing != 0 {
		return fmt.Errorf("%w: snapshot id was already verified with different bytes", ErrConflict)
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_verified_snapshots(snapshot_id, commit_sha256, created_at)
		VALUES(?, ?, ?) ON CONFLICT(snapshot_id) DO UPDATE SET verified_at=CURRENT_TIMESTAMP`,
		snapshot.SnapshotID, snapshot.CommitSHA256, snapshot.CreatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`DELETE FROM sync_verified_snapshot_vectors WHERE snapshot_id = ?`, snapshot.SnapshotID); err != nil {
		return err
	}
	replicas := sortedVectorKeys(snapshot.Vector)
	for _, replicaID := range replicas {
		if err := s.execPreparedLocked(`INSERT INTO sync_verified_snapshot_vectors(snapshot_id, replica_id, sequence)
			VALUES(?, ?, ?)`, snapshot.SnapshotID, replicaID, strconv.FormatInt(snapshot.Vector[replicaID], 10)); err != nil {
			return err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

type SyncPeerRetentionStatus struct {
	ReplicaID          string    `json:"replica_id"`
	Status             string    `json:"status"`
	LastAcknowledged   time.Time `json:"last_acknowledged,omitempty"`
	WarningAt          time.Time `json:"warning_at,omitempty"`
	HorizonAt          time.Time `json:"horizon_at,omitempty"`
	Warning            bool      `json:"warning"`
	BeyondHorizon      bool      `json:"beyond_horizon"`
	FullResyncRequired bool      `json:"full_resync_required"`
}

type SyncRetentionSubject struct {
	ReplicaID          string `json:"replica_id"`
	CurrentSequence    int64  `json:"current_sequence"`
	CurrentFloor       int64  `json:"current_floor"`
	AgeFloor           int64  `json:"age_floor"`
	SnapshotFloor      int64  `json:"snapshot_floor"`
	AcknowledgedFloor  int64  `json:"acknowledged_floor"`
	EligibleFloor      int64  `json:"eligible_floor"`
	EligibleOperations int64  `json:"eligible_operations"`
	EligibleBytes      int64  `json:"eligible_bytes"`
}

type SyncTombstoneCandidate struct {
	DocumentID string    `json:"document_id"`
	ReplicaID  string    `json:"replica_id"`
	Sequence   int64     `json:"sequence"`
	PurgedAt   time.Time `json:"purged_at"`
}

type SyncRepairPlan struct {
	Ready            bool             `json:"ready"`
	SnapshotID       string           `json:"snapshot_id,omitempty"`
	SnapshotVector   syncstate.Vector `json:"snapshot_vector"`
	NewerOperations  int64            `json:"newer_operations"`
	InstallAutomatic bool             `json:"install_automatic"`
}

type SyncRetentionReport struct {
	DryRun              bool                      `json:"dry_run"`
	Applied             bool                      `json:"applied"`
	AsOf                time.Time                 `json:"as_of"`
	HistorySeconds      int64                     `json:"history_seconds"`
	WarningSeconds      int64                     `json:"warning_seconds"`
	SnapshotID          string                    `json:"snapshot_id,omitempty"`
	SnapshotCreatedAt   time.Time                 `json:"snapshot_created_at,omitempty"`
	Subjects            []SyncRetentionSubject    `json:"subjects"`
	Peers               []SyncPeerRetentionStatus `json:"peers"`
	Tombstones          []SyncTombstoneCandidate  `json:"tombstones"`
	EligibleOperations  int64                     `json:"eligible_operations"`
	EligibleBytes       int64                     `json:"eligible_bytes"`
	RemovedOperations   int64                     `json:"removed_operations"`
	CollectedTombstones int64                     `json:"collected_tombstones"`
	Digest              string                    `json:"digest"`
	Warnings            []string                  `json:"warnings"`
	Repair              SyncRepairPlan            `json:"repair"`
}

type SyncRetentionRequest struct {
	HistoryFor     time.Duration
	WarningBefore  time.Duration
	Now            time.Time
	Apply          bool
	ExpectedDigest string
}

func normalizeSyncRetentionRequest(req SyncRetentionRequest) SyncRetentionRequest {
	if req.HistoryFor <= 0 {
		req.HistoryFor = DefaultSyncHistoryRetention
	}
	if req.WarningBefore <= 0 || req.WarningBefore >= req.HistoryFor {
		req.WarningBefore = DefaultSyncPeerWarning
		if req.WarningBefore >= req.HistoryFor {
			req.WarningBefore = req.HistoryFor / 3
		}
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	} else {
		req.Now = req.Now.UTC()
	}
	req.ExpectedDigest = strings.TrimSpace(req.ExpectedDigest)
	return req
}

func (s *SQLiteStore) PlanSyncRetention(ctx context.Context, req SyncRetentionRequest) (SyncRetentionReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncRetentionReport{}, err
	}
	req = normalizeSyncRetentionRequest(req)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planSyncRetentionLocked(req)
}

func (s *SQLiteStore) ApplySyncRetention(ctx context.Context, req SyncRetentionRequest) (SyncRetentionReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncRetentionReport{}, err
	}
	req = normalizeSyncRetentionRequest(req)
	if req.ExpectedDigest == "" {
		return SyncRetentionReport{}, fmt.Errorf("%w: apply requires the exact dry-run digest", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	report, err := s.planSyncRetentionLocked(req)
	if err != nil {
		return SyncRetentionReport{}, err
	}
	if report.Digest != req.ExpectedDigest {
		return SyncRetentionReport{}, fmt.Errorf("%w: retention state changed; review a new dry run", ErrConflict)
	}
	if report.SnapshotID == "" {
		return SyncRetentionReport{}, fmt.Errorf("%w: no verified snapshot floor is retained", ErrConflict)
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncRetentionReport{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.retainRevisionOrdersLocked(report.Subjects); err != nil {
		return SyncRetentionReport{}, err
	}
	for _, tombstone := range report.Tombstones {
		if err := s.collectTombstonePayloadLocked(tombstone.DocumentID); err != nil {
			return SyncRetentionReport{}, err
		}
	}
	// Capture the exact current canonical checkpoint after payload collection
	// but before operation compaction, so its floors include every operation the
	// checkpoint already represents.
	if err := s.captureSyncMetadataBaselineLocked(true); err != nil {
		return SyncRetentionReport{}, err
	}
	for _, subject := range report.Subjects {
		if subject.EligibleFloor <= subject.CurrentFloor {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO sync_retention_floors(replica_id, sequence, snapshot_id)
			VALUES(?, ?, ?) ON CONFLICT(replica_id) DO UPDATE SET sequence=excluded.sequence,
			snapshot_id=excluded.snapshot_id, advanced_at=CURRENT_TIMESTAMP`, subject.ReplicaID,
			strconv.FormatInt(subject.EligibleFloor, 10), report.SnapshotID); err != nil {
			return SyncRetentionReport{}, err
		}
		if err := s.execPreparedLocked(`DELETE FROM sync_operations WHERE replica_id = ? AND sequence <= ?`,
			subject.ReplicaID, strconv.FormatInt(subject.EligibleFloor, 10)); err != nil {
			return SyncRetentionReport{}, err
		}
	}
	if err := s.execLocked(`DELETE FROM sync_operation_dependencies
		WHERE NOT EXISTS (SELECT 1 FROM sync_operations o
			WHERE o.replica_id=operation_replica_id AND o.sequence=operation_sequence)
		   OR EXISTS (SELECT 1 FROM sync_retention_floors f
			WHERE f.replica_id=dependency_replica_id AND f.sequence>=dependency_sequence)`); err != nil {
		return SyncRetentionReport{}, err
	}
	auditID, err := NewID("audit")
	if err != nil {
		return SyncRetentionReport{}, err
	}
	details, _ := json.Marshal(map[string]any{"digest": report.Digest, "snapshot_id": report.SnapshotID,
		"operations": report.EligibleOperations, "tombstones": len(report.Tombstones)})
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, subject_id, details_json)
		VALUES(?, 'retention.applied', ?, ?)`, auditID, report.SnapshotID, string(details)); err != nil {
		return SyncRetentionReport{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncRetentionReport{}, err
	}
	committed = true
	report.DryRun = false
	report.Applied = true
	report.RemovedOperations = report.EligibleOperations
	report.CollectedTombstones = int64(len(report.Tombstones))
	return report, nil
}

func (s *SQLiteStore) planSyncRetentionLocked(req SyncRetentionRequest) (SyncRetentionReport, error) {
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return SyncRetentionReport{}, err
	}
	vector, err := s.syncStateVectorLocked(identity.ReplicaID)
	if err != nil {
		return SyncRetentionReport{}, err
	}
	snapshot, found, err := s.latestVerifiedSyncSnapshotLocked()
	if err != nil {
		return SyncRetentionReport{}, err
	}
	activePeers, allPeers, err := s.retentionPeersLocked(req, vector)
	if err != nil {
		return SyncRetentionReport{}, err
	}
	report := SyncRetentionReport{DryRun: true, AsOf: req.Now,
		HistorySeconds: int64(req.HistoryFor / time.Second), WarningSeconds: int64(req.WarningBefore / time.Second),
		Subjects: []SyncRetentionSubject{}, Peers: allPeers, Tombstones: []SyncTombstoneCandidate{}, Warnings: []string{}}
	if found {
		report.SnapshotID, report.SnapshotCreatedAt = snapshot.SnapshotID, snapshot.CreatedAt
		report.Repair = SyncRepairPlan{Ready: true, SnapshotID: snapshot.SnapshotID,
			SnapshotVector: syncstate.CloneVector(snapshot.Vector), InstallAutomatic: false}
	} else {
		report.Warnings = append(report.Warnings, "No verified retained snapshot exists; collection is blocked.")
		report.Repair = SyncRepairPlan{SnapshotVector: syncstate.Vector{}, InstallAutomatic: false}
	}
	cutoff := req.Now.Add(-req.HistoryFor)
	for _, replicaID := range sortedVectorKeys(vector) {
		current := vector[replicaID]
		currentFloor, err := s.retentionFloorLocked(replicaID)
		if err != nil {
			return SyncRetentionReport{}, err
		}
		ageFloor, err := s.operationAgeFloorLocked(replicaID, cutoff)
		if err != nil {
			return SyncRetentionReport{}, err
		}
		snapshotFloor := snapshot.Vector[replicaID]
		ackFloor := current
		for _, peerID := range activePeers {
			ack, err := s.syncPeerAcknowledgementLocked(peerID, replicaID)
			if err != nil {
				return SyncRetentionReport{}, err
			}
			if ack < ackFloor {
				ackFloor = ack
			}
		}
		eligible := minInt64(current, ageFloor, snapshotFloor, ackFloor)
		if !found || eligible < currentFloor {
			eligible = currentFloor
		}
		count, bytes, err := s.operationRangeUsageLocked(replicaID, currentFloor, eligible)
		if err != nil {
			return SyncRetentionReport{}, err
		}
		subject := SyncRetentionSubject{ReplicaID: replicaID, CurrentSequence: current, CurrentFloor: currentFloor,
			AgeFloor: ageFloor, SnapshotFloor: snapshotFloor, AcknowledgedFloor: ackFloor,
			EligibleFloor: eligible, EligibleOperations: count, EligibleBytes: bytes}
		report.Subjects = append(report.Subjects, subject)
		report.EligibleOperations += count
		report.EligibleBytes += bytes
		if current > snapshotFloor {
			report.Repair.NewerOperations += current - snapshotFloor
		}
	}
	report.Tombstones, err = s.eligibleTombstonesLocked(report.Subjects, cutoff)
	if err != nil {
		return SyncRetentionReport{}, err
	}
	digestInput := struct {
		Snapshot string                   `json:"snapshot"`
		Subjects []SyncRetentionSubject   `json:"subjects"`
		Tombs    []SyncTombstoneCandidate `json:"tombstones"`
	}{report.SnapshotID, report.Subjects, report.Tombstones}
	raw, _ := json.Marshal(digestInput)
	digest := sha256.Sum256(raw)
	report.Digest = hex.EncodeToString(digest[:])
	return report, nil
}

func (s *SQLiteStore) latestVerifiedSyncSnapshotLocked() (VerifiedSyncSnapshot, bool, error) {
	stmt, err := s.prepareLocked(`SELECT snapshot_id, commit_sha256, created_at FROM sync_verified_snapshots
		ORDER BY verified_at DESC, snapshot_id DESC`)
	if err != nil {
		return VerifiedSyncSnapshot{}, false, err
	}
	candidates := []VerifiedSyncSnapshot{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			created, parseErr := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 2)))
			if parseErr != nil {
				C.sqlite3_finalize(stmt)
				return VerifiedSyncSnapshot{}, false, parseErr
			}
			candidates = append(candidates, VerifiedSyncSnapshot{SnapshotID: columnText(stmt, 0), CommitSHA256: columnText(stmt, 1), CreatedAt: created, Vector: syncstate.Vector{}})
		case C.SQLITE_DONE:
			C.sqlite3_finalize(stmt)
			goto loaded
		default:
			C.sqlite3_finalize(stmt)
			return VerifiedSyncSnapshot{}, false, s.stepErrLocked(rc)
		}
	}

loaded:
	for _, snapshot := range candidates {
		vectorStmt, vectorErr := s.prepareLocked(`SELECT replica_id, sequence FROM sync_verified_snapshot_vectors
		WHERE snapshot_id = ? ORDER BY replica_id`)
		if vectorErr != nil {
			return VerifiedSyncSnapshot{}, false, vectorErr
		}
		if err := bindAll(vectorStmt, []string{snapshot.SnapshotID}); err != nil {
			C.sqlite3_finalize(vectorStmt)
			return VerifiedSyncSnapshot{}, false, err
		}
		for {
			switch rc := C.sqlite3_step(vectorStmt); rc {
			case C.SQLITE_ROW:
				snapshot.Vector[columnText(vectorStmt, 0)] = columnInt64(vectorStmt, 1)
			case C.SQLITE_DONE:
				C.sqlite3_finalize(vectorStmt)
				covered, coverErr := s.snapshotCoversRetentionFloorsLocked(snapshot.Vector)
				if coverErr != nil {
					return VerifiedSyncSnapshot{}, false, coverErr
				}
				if covered {
					return snapshot, true, nil
				}
				goto nextSnapshot
			default:
				C.sqlite3_finalize(vectorStmt)
				return VerifiedSyncSnapshot{}, false, s.stepErrLocked(rc)
			}
		}
	nextSnapshot:
	}
	return VerifiedSyncSnapshot{Vector: syncstate.Vector{}}, false, nil
}

func (s *SQLiteStore) snapshotCoversRetentionFloorsLocked(vector syncstate.Vector) (bool, error) {
	stmt, err := s.prepareLocked(`SELECT replica_id, sequence FROM sync_retention_floors ORDER BY replica_id`)
	if err != nil {
		return false, err
	}
	defer C.sqlite3_finalize(stmt)
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			if vector[columnText(stmt, 0)] < columnInt64(stmt, 1) {
				return false, nil
			}
		case C.SQLITE_DONE:
			return true, nil
		default:
			return false, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) retentionPeersLocked(req SyncRetentionRequest, vector syncstate.Vector) ([]string, []SyncPeerRetentionStatus, error) {
	stmt, err := s.prepareLocked(`SELECT replica_id, status, enrolled_at, COALESCE(retired_at, '')
		FROM sync_replicas WHERE role='peer' ORDER BY replica_id`)
	if err != nil {
		return nil, nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var active []string
	peers := []SyncPeerRetentionStatus{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			replicaID, status := columnText(stmt, 0), columnText(stmt, 1)
			last, err := s.peerLastAcknowledgedLocked(replicaID, columnText(stmt, 2))
			if err != nil {
				return nil, nil, err
			}
			peer := SyncPeerRetentionStatus{ReplicaID: replicaID, Status: status, LastAcknowledged: last}
			peer.HorizonAt = last.Add(req.HistoryFor)
			peer.WarningAt = peer.HorizonAt.Add(-req.WarningBefore)
			peer.Warning = status == "active" && !req.Now.Before(peer.WarningAt)
			peer.BeyondHorizon = status == "active" && !req.Now.Before(peer.HorizonAt)
			for subject := range vector {
				floor, floorErr := s.retentionFloorLocked(subject)
				if floorErr != nil {
					return nil, nil, floorErr
				}
				ack, ackErr := s.syncPeerAcknowledgementLocked(replicaID, subject)
				if ackErr != nil {
					return nil, nil, ackErr
				}
				peer.FullResyncRequired = peer.FullResyncRequired || ack < floor
			}
			if status == "retired" || status == "revoked" {
				peer.FullResyncRequired = true
			} else if status == "active" {
				active = append(active, replicaID)
			}
			peers = append(peers, peer)
		case C.SQLITE_DONE:
			return active, peers, nil
		default:
			return nil, nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) peerLastAcknowledgedLocked(replicaID, enrolledAt string) (time.Time, error) {
	stmt, err := s.prepareLocked(`SELECT COALESCE(MAX(acknowledged_at), ?) FROM sync_peer_acknowledgements
		WHERE peer_replica_id = ?`)
	if err != nil {
		return time.Time{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{enrolledAt, replicaID}); err != nil {
		return time.Time{}, err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_ROW {
		return time.Time{}, s.stepErrLocked(rc)
	}
	return time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 0)))
}

func (s *SQLiteStore) retentionFloorLocked(replicaID string) (int64, error) {
	stmt, err := s.prepareLocked(`SELECT sequence FROM sync_retention_floors WHERE replica_id = ?`)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID}); err != nil {
		return 0, err
	}
	if rc := C.sqlite3_step(stmt); rc == C.SQLITE_DONE {
		return 0, nil
	} else if rc != C.SQLITE_ROW {
		return 0, s.stepErrLocked(rc)
	}
	return columnInt64(stmt, 0), nil
}

func (s *SQLiteStore) operationAgeFloorLocked(replicaID string, cutoff time.Time) (int64, error) {
	stmt, err := s.prepareLocked(`SELECT COALESCE(MAX(sequence), 0) FROM sync_operations
		WHERE replica_id = ? AND created_at <= ?`)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, cutoff.UTC().Format(time.RFC3339Nano)}); err != nil {
		return 0, err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_ROW {
		return 0, s.stepErrLocked(rc)
	}
	return columnInt64(stmt, 0), nil
}

func (s *SQLiteStore) operationRangeUsageLocked(replicaID string, after, through int64) (int64, int64, error) {
	if through <= after {
		return 0, 0, nil
	}
	stmt, err := s.prepareLocked(`SELECT COUNT(*), COALESCE(SUM(length(payload_json)), 0) FROM sync_operations
		WHERE replica_id = ? AND sequence > ? AND sequence <= ?`)
	if err != nil {
		return 0, 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID, strconv.FormatInt(after, 10), strconv.FormatInt(through, 10)}); err != nil {
		return 0, 0, err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_ROW {
		return 0, 0, s.stepErrLocked(rc)
	}
	return columnInt64(stmt, 0), columnInt64(stmt, 1), nil
}

func (s *SQLiteStore) eligibleTombstonesLocked(subjects []SyncRetentionSubject, cutoff time.Time) ([]SyncTombstoneCandidate, error) {
	floors := map[string]int64{}
	for _, subject := range subjects {
		floors[subject.ReplicaID] = subject.EligibleFloor
	}
	stmt, err := s.prepareLocked(`SELECT document_id, purge_replica_id, purge_sequence, purged_at
		FROM sync_tombstone_payloads WHERE collected_at IS NULL AND purged_at <= ? ORDER BY document_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{cutoff.UTC().Format(time.RFC3339Nano)}); err != nil {
		return nil, err
	}
	items := []SyncTombstoneCandidate{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			purged, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 3)))
			if err != nil {
				return nil, err
			}
			item := SyncTombstoneCandidate{DocumentID: columnText(stmt, 0), ReplicaID: columnText(stmt, 1), Sequence: columnInt64(stmt, 2), PurgedAt: purged}
			if floors[item.ReplicaID] >= item.Sequence {
				items = append(items, item)
			}
		case C.SQLITE_DONE:
			return items, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) retainRevisionOrdersLocked(subjects []SyncRetentionSubject) error {
	for _, subject := range subjects {
		if subject.EligibleFloor <= subject.CurrentFloor {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO sync_retained_revision_orders(
			revision_id, hlc_wall_ms, hlc_logical, replica_id, sequence)
			SELECT record_id, hlc_wall_ms, hlc_logical, replica_id, sequence FROM sync_operations
			WHERE replica_id=? AND sequence<=? AND record_type='revision' AND kind='revision.create'
			ON CONFLICT(revision_id) DO NOTHING`, subject.ReplicaID, strconv.FormatInt(subject.EligibleFloor, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) collectTombstonePayloadLocked(documentID string) error {
	if err := s.execPreparedLocked(`UPDATE resources SET unreferenced_at=CURRENT_TIMESTAMP,
		unreferenced_reason='purged_document' WHERE id IN
		(SELECT resource_id FROM document_resource_refs WHERE document_id=?)
		AND NOT EXISTS (SELECT 1 FROM document_resource_refs other
			WHERE other.resource_id=resources.id AND other.document_id<>?)`, documentID, documentID); err != nil {
		return err
	}
	if err := s.execLocked(`INSERT INTO sync_apply_guard(singleton) VALUES(1) ON CONFLICT(singleton) DO NOTHING`); err != nil {
		return err
	}
	defer func() { _ = s.execLocked(`DELETE FROM sync_apply_guard`) }()
	for _, statement := range []string{
		`DELETE FROM note_tags WHERE document_id=?`,
		`DELETE FROM document_resource_refs WHERE document_id=?`,
		`DELETE FROM document_links WHERE source_document_id=?`,
		`DELETE FROM document_blocks WHERE document_id=?`,
		`DELETE FROM documents_fts WHERE document_id=?`,
		`DELETE FROM sync_document_conflicts WHERE document_id=?`,
		`DELETE FROM sync_revision_pending_bodies WHERE document_id=?`,
		`UPDATE document_links SET target_document_id=NULL, resolution_status='target_deleted' WHERE target_document_id=?`,
		`UPDATE documents SET current_revision_id=NULL WHERE id=?`,
		`DELETE FROM document_revisions WHERE document_id=?`,
		`DELETE FROM documents WHERE id=?`,
	} {
		if err := s.execPreparedLocked(statement, documentID); err != nil {
			return err
		}
	}
	if err := s.execPreparedLocked(`UPDATE sync_tombstone_payloads SET collected_at=CURRENT_TIMESTAMP WHERE document_id=?`, documentID); err != nil {
		return err
	}
	return s.enqueueProjectionLocked(documentID, "delete")
}

// SyncRetentionGate is a frozen dry-run decision used by the existing local
// resource collector. It is intentionally conservative: every subject's
// current vector must be covered by the verified snapshot and active-peer ack
// floor before any unreferenced resource is removed.
type SyncRetentionGate struct {
	allowed bool
	reason  string
}

func (g SyncRetentionGate) CanCollect(context.Context, GarbageCollectionCandidate) (bool, string, error) {
	return g.allowed, g.reason, nil
}

func (s *SQLiteStore) BuildSyncRetentionGate(ctx context.Context, req SyncRetentionRequest) (RetentionGate, error) {
	report, err := s.PlanSyncRetention(ctx, req)
	if err != nil {
		return nil, err
	}
	if report.SnapshotID == "" {
		return SyncRetentionGate{reason: "no_verified_snapshot"}, nil
	}
	for _, subject := range report.Subjects {
		if subject.SnapshotFloor < subject.CurrentSequence || subject.AcknowledgedFloor < subject.CurrentSequence {
			return SyncRetentionGate{reason: "sync_watermark_pending"}, nil
		}
	}
	return SyncRetentionGate{allowed: true, reason: "sync_ack_snapshot_satisfied"}, nil
}

type RetireSyncPeerRequest struct {
	ReplicaID string
	Reason    string
	Signer    syncwire.Signer
}

type RetireSyncPeerResult struct {
	ReplicaID           string   `json:"replica_id"`
	Status              string   `json:"status"`
	DecisionReplicaID   string   `json:"decision_replica_id"`
	DecisionSequence    int64    `json:"decision_sequence"`
	UnacknowledgedPeers []string `json:"unacknowledged_peers"`
}

func (s *SQLiteStore) RetireSyncPeer(ctx context.Context, req RetireSyncPeerRequest) (RetireSyncPeerResult, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return RetireSyncPeerResult{}, err
	}
	req.ReplicaID = strings.TrimSpace(req.ReplicaID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.ReplicaID == "" || req.Signer == nil {
		return RetireSyncPeerResult{}, fmt.Errorf("%w: retirement needs a peer and local signer", ErrInvalidInput)
	}
	if len(req.Reason) > MaxRetirementReasonBytes {
		return RetireSyncPeerResult{}, fmt.Errorf("%w: retirement reason exceeds %d bytes", ErrInvalidInput, MaxRetirementReasonBytes)
	}
	auditID, err := NewID("audit")
	if err != nil {
		return RetireSyncPeerResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return RetireSyncPeerResult{}, err
	}
	role, databaseID, status, found, err := s.syncReplicaLocked(req.ReplicaID)
	if err != nil {
		return RetireSyncPeerResult{}, err
	}
	if !found || role != "peer" || databaseID != local.DatabaseID {
		return RetireSyncPeerResult{}, ErrNotFound
	}
	if status == "retired" || status == "revoked" {
		return s.retireSyncPeerResultLocked(req.ReplicaID)
	}
	journal, found, err := s.journalStatusLocked()
	if err != nil || !found {
		return RetireSyncPeerResult{}, fmt.Errorf("%w: local journal is not enrolled", ErrConflict)
	}
	sequence := journal.LastSequence + 1
	signature := syncwire.SignReplicaRetirement(req.Signer, local.DatabaseID, req.ReplicaID, local.ReplicaID, sequence)
	payload, err := json.Marshal(map[string]any{"database_id": local.DatabaseID, "replica_id": req.ReplicaID,
		"decision_replica_id": local.ReplicaID, "sequence": sequence, "signer_key_id": req.Signer.SignerKeyID(),
		"signature": signature, "reason": req.Reason})
	if err != nil {
		return RetireSyncPeerResult{}, err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return RetireSyncPeerResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`INSERT INTO sync_journal_capture(kind, record_type, record_id, payload_json)
		VALUES('replica.retire', 'replica', ?, ?)`, req.ReplicaID, string(payload)); err != nil {
		return RetireSyncPeerResult{}, err
	}
	operation := syncstate.Operation{ReplicaID: local.ReplicaID, Sequence: sequence, Kind: "replica.retire",
		RecordType: "replica", RecordID: req.ReplicaID, Payload: payload}
	if err := s.applyRetirementOperationLocked(operation, true); err != nil {
		return RetireSyncPeerResult{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'peer.retired', ?, ?, ?)`, auditID, local.ReplicaID, req.ReplicaID,
		auditDetails(map[string]string{"reason": boundedReason(req.Reason), "signer_key_id": req.Signer.SignerKeyID()})); err != nil {
		return RetireSyncPeerResult{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return RetireSyncPeerResult{}, err
	}
	committed = true
	return s.retireSyncPeerResultLocked(req.ReplicaID)
}

// PurgeDocumentWithCertificate makes permanent deletion a signed replicated
// decision while retaining the payload until the G17 watermark permits its
// collection. The ordinary PurgeDocument remains the unsynchronized path.
func (s *SQLiteStore) PurgeDocumentWithCertificate(ctx context.Context, documentID string, signer syncwire.Signer) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	documentID = strings.TrimSpace(documentID)
	if documentID == "" || signer == nil {
		return fmt.Errorf("%w: signed purge needs a document and local signer", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, _, _, err := s.trashedDocumentStateLocked(documentID); err != nil {
		return err
	}
	if sourced, err := s.documentHasSourceLocked(documentID); err != nil {
		return err
	} else if sourced {
		return fmt.Errorf("%w: externally-sourced notes cannot be permanently deleted", ErrProtected)
	}
	journal, found, err := s.journalStatusLocked()
	if err != nil || !found {
		return fmt.Errorf("%w: local journal is not enrolled", ErrConflict)
	}
	sequence := journal.LastSequence + 1
	signature := syncwire.SignDeathCertificate(signer, documentID, journal.ReplicaID, sequence)
	payload, err := json.Marshal(map[string]any{"document_id": documentID, "signer_replica_id": journal.ReplicaID,
		"signer_key_id": signer.SignerKeyID(), "sequence": sequence, "signature": signature})
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
	if err := s.execPreparedLocked(`INSERT INTO sync_journal_capture(kind, record_type, record_id, payload_json)
		VALUES('document.purge', 'document', ?, ?)`, documentID, string(payload)); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_tombstone_payloads(document_id, purge_replica_id, purge_sequence)
		VALUES(?, ?, ?) ON CONFLICT(document_id) DO NOTHING`, documentID, journal.ReplicaID, strconv.FormatInt(sequence, 10)); err != nil {
		return err
	}
	if err := s.reconcileSyncMetadataLocked(); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

type retirementPayload struct {
	DatabaseID        string `json:"database_id"`
	ReplicaID         string `json:"replica_id"`
	DecisionReplicaID string `json:"decision_replica_id"`
	Sequence          int64  `json:"sequence"`
	SignerKeyID       string `json:"signer_key_id"`
	Signature         string `json:"signature"`
	Reason            string `json:"reason"`
}

func (s *SQLiteStore) applyRetirementOperationLocked(operation syncstate.Operation, localDecision bool) error {
	var payload retirementPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("retirement payload: %w", err)
	}
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return err
	}
	if payload.DatabaseID != identity.DatabaseID || payload.ReplicaID != operation.RecordID ||
		payload.DecisionReplicaID != operation.ReplicaID || payload.Sequence != operation.Sequence ||
		payload.SignerKeyID == "" || payload.Signature == "" || len(payload.Reason) > MaxRetirementReasonBytes {
		return fmt.Errorf("%w: malformed replica retirement", ErrInvalidInput)
	}
	if !localDecision {
		key, found, err := s.peerSigningKeyLocked(payload.SignerKeyID)
		if err != nil {
			return err
		}
		if !found || key.Status != "active" || key.ReplicaID != operation.ReplicaID {
			return fmt.Errorf("%w: retirement signer is not an active enrolled peer", ErrConflict)
		}
		verifier := oneKeyVerifier{keyID: payload.SignerKeyID, public: key.PublicKey}
		if err := syncwire.VerifyReplicaRetirement(verifier, payload.SignerKeyID, payload.Signature,
			payload.DatabaseID, payload.ReplicaID, payload.DecisionReplicaID, payload.Sequence); err != nil {
			return fmt.Errorf("%w: retirement signature: %v", ErrConflict, err)
		}
	}
	role, databaseID, _, found, err := s.syncReplicaLocked(payload.ReplicaID)
	if err != nil {
		return err
	}
	if !found || role != "peer" || databaseID != identity.DatabaseID {
		return fmt.Errorf("%w: retirement names an unknown peer", ErrConflict)
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_peer_retirements(replica_id, decision_replica_id,
		decision_sequence, signer_key_id, signature, reason) VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(replica_id) DO NOTHING`, payload.ReplicaID, payload.DecisionReplicaID,
		strconv.FormatInt(payload.Sequence, 10), payload.SignerKeyID, payload.Signature, payload.Reason); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`UPDATE sync_replicas SET status='retired', retired_at=CURRENT_TIMESTAMP
		WHERE replica_id=? AND role='peer'`, payload.ReplicaID); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`UPDATE sync_peer_keys SET status='revoked', revoked_at=CURRENT_TIMESTAMP,
		reason=? WHERE replica_id=? AND status='active'`, "retired: "+boundedReason(payload.Reason), payload.ReplicaID); err != nil {
		return err
	}
	return s.execPreparedLocked(`UPDATE sync_catchup_permissions SET permitted_source=0 WHERE replica_id=?`, payload.ReplicaID)
}

type oneKeyVerifier struct {
	keyID  string
	public ed25519.PublicKey
}

func (v oneKeyVerifier) PublicKey(keyID string) (ed25519.PublicKey, bool) {
	if keyID != v.keyID || len(v.public) != ed25519.PublicKeySize {
		return nil, false
	}
	return v.public, true
}

func (s *SQLiteStore) retireSyncPeerResultLocked(replicaID string) (RetireSyncPeerResult, error) {
	stmt, err := s.prepareLocked(`SELECT decision_replica_id, decision_sequence FROM sync_peer_retirements WHERE replica_id=?`)
	if err != nil {
		return RetireSyncPeerResult{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{replicaID}); err != nil {
		return RetireSyncPeerResult{}, err
	}
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_ROW {
		return RetireSyncPeerResult{}, ErrNotFound
	}
	result := RetireSyncPeerResult{ReplicaID: replicaID, Status: "retired", DecisionReplicaID: columnText(stmt, 0), DecisionSequence: columnInt64(stmt, 1), UnacknowledgedPeers: []string{}}
	peers, err := s.listActivePeerIDsLocked()
	if err != nil {
		return RetireSyncPeerResult{}, err
	}
	for _, peerID := range peers {
		ack, err := s.syncPeerAcknowledgementLocked(peerID, result.DecisionReplicaID)
		if err != nil {
			return RetireSyncPeerResult{}, err
		}
		if ack < result.DecisionSequence {
			result.UnacknowledgedPeers = append(result.UnacknowledgedPeers, peerID)
		}
	}
	return result, nil
}

func (s *SQLiteStore) listActivePeerIDsLocked() ([]string, error) {
	stmt, err := s.prepareLocked(`SELECT replica_id FROM sync_replicas WHERE role='peer' AND status='active' ORDER BY replica_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var peers []string
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			peers = append(peers, columnText(stmt, 0))
		case C.SQLITE_DONE:
			return peers, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) applyAdmittedRetirementsLocked() error {
	stmt, err := s.prepareLocked(`SELECT o.replica_id, o.sequence, o.kind, o.record_type, o.record_id, o.payload_json
		FROM sync_operations o LEFT JOIN sync_peer_retirements r ON r.replica_id=o.record_id
		WHERE o.record_type='replica' AND o.kind='replica.retire' AND r.replica_id IS NULL
		ORDER BY o.hlc_wall_ms, o.hlc_logical, o.replica_id, o.sequence`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	var operations []syncstate.Operation
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			operations = append(operations, syncstate.Operation{ReplicaID: columnText(stmt, 0), Sequence: columnInt64(stmt, 1),
				Kind: columnText(stmt, 2), RecordType: columnText(stmt, 3), RecordID: columnText(stmt, 4), Payload: json.RawMessage(columnText(stmt, 5))})
		case C.SQLITE_DONE:
			for _, operation := range operations {
				if err := s.applyRetirementOperationLocked(operation, false); err != nil {
					return err
				}
			}
			return nil
		default:
			return s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) verifyAndMarkDeathOperationLocked(operation syncstate.Operation) error {
	if operation.Kind != "document.purge" || operation.RecordType != "document" {
		return nil
	}
	var payload struct {
		DocumentID      string `json:"document_id"`
		SignerReplicaID string `json:"signer_replica_id"`
		SignerKeyID     string `json:"signer_key_id"`
		Sequence        int64  `json:"sequence"`
		Signature       string `json:"signature"`
	}
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return fmt.Errorf("death certificate payload: %w", err)
	}
	if payload.DocumentID != operation.RecordID || payload.SignerReplicaID != operation.ReplicaID ||
		payload.Sequence != operation.Sequence || payload.SignerKeyID == "" || payload.Signature == "" {
		return fmt.Errorf("%w: malformed death certificate", ErrInvalidInput)
	}
	key, found, err := s.peerSigningKeyLocked(payload.SignerKeyID)
	if err != nil {
		return err
	}
	if !found || key.Status != "active" || key.ReplicaID != operation.ReplicaID {
		return fmt.Errorf("%w: death certificate signer is not an active enrolled peer", ErrConflict)
	}
	if err := syncwire.VerifyDeathCertificate(oneKeyVerifier{keyID: payload.SignerKeyID, public: key.PublicKey},
		payload.SignerKeyID, payload.Signature, payload.DocumentID, payload.SignerReplicaID, payload.Sequence); err != nil {
		return fmt.Errorf("%w: death certificate signature: %v", ErrConflict, err)
	}
	return s.execPreparedLocked(`INSERT INTO sync_tombstone_payloads(document_id, purge_replica_id, purge_sequence, purged_at)
		VALUES(?, ?, ?, ?) ON CONFLICT(document_id) DO NOTHING`, payload.DocumentID, payload.SignerReplicaID,
		strconv.FormatInt(payload.Sequence, 10), operation.CreatedAt)
}

func sortedVectorKeys(vector syncstate.Vector) []string {
	keys := make([]string, 0, len(vector))
	for key := range vector {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func minInt64(values ...int64) int64 {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

// Keep the retained baseline order type tied to the convergence core at
// compile time; it prevents a future change from silently dropping a field.
