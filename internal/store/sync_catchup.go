package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/renesugar/notrios/internal/synccatchup"
	"github.com/renesugar/notrios/internal/syncstate"
)

// ensureSchemaV24 adds G10's catch-up sessions and source permissions. It is a
// CREATE-only migration: nothing existing changes meaning.
func (s *SQLiteStore) ensureSchemaV24(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if version >= 24 {
		return nil
	}
	migration, err := migrationFS.ReadFile("migrations/0024_sync_catchup.sql")
	if err != nil {
		return fmt.Errorf("read schema v24 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

// PermitSnapshotSource records whether an enrolled peer may answer this
// replica's backup requests. It is deliberately separate from enrollment,
// because answering means handing over a complete copy of the library.
func (s *SQLiteStore) PermitSnapshotSource(ctx context.Context, replicaID string, permitted bool) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if replicaID == "" {
		return fmt.Errorf("%w: a replica id is required", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execPreparedLocked(
		`INSERT INTO sync_catchup_permissions(replica_id, permitted_source) VALUES(?, ?)
		 ON CONFLICT(replica_id) DO UPDATE SET permitted_source = excluded.permitted_source, granted_at = CURRENT_TIMESTAMP`,
		replicaID, boolNumber(permitted))
}

// SnapshotSourcePolicy reads the permission table. It satisfies
// synccatchup.SourcePolicy, so the selection rules never touch SQLite.
type SnapshotSourcePolicy struct{ store *SQLiteStore }

// SnapshotSources returns this replica's source policy.
func (s *SQLiteStore) SnapshotSources() *SnapshotSourcePolicy { return &SnapshotSourcePolicy{store: s} }

func (p *SnapshotSourcePolicy) PermittedSource(replicaID string) bool {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	count, err := p.store.countLocked(
		`SELECT COUNT(*) FROM sync_catchup_permissions c
		   JOIN sync_replicas r ON r.replica_id = c.replica_id
		  WHERE c.replica_id = ? AND c.permitted_source = 1 AND r.role = 'peer' AND r.status = 'active'`,
		replicaID)
	return err == nil && count == 1
}

var _ synccatchup.SourcePolicy = (*SnapshotSourcePolicy)(nil)

// CatchupSession is one durable catch-up.
type CatchupSession struct {
	ID             string
	Role           string
	State          synccatchup.State
	DatabaseID     string
	PeerReplicaID  string
	RequestNonce   string
	SnapshotID     string
	SnapshotVector syncstate.Vector
	ArchiveSHA256  string
	ArchiveLength  int64
	ReceivedBytes  int64
	WrappingMode   synccatchup.WrappingMode
	Intent         string
	ExpiresAt      string
	LastReason     string
}

// BeginCatchup records a request this replica has made. The session exists
// before anything is transferred, so an interruption leaves a trace.
func (s *SQLiteStore) BeginCatchup(ctx context.Context, request synccatchup.Request) (CatchupSession, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return CatchupSession{}, err
	}
	id, err := NewID("catchup")
	if err != nil {
		return CatchupSession{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(
		`INSERT INTO sync_catchup_sessions(id, role, state, database_id, request_nonce, wrapping_mode)
		 VALUES(?, 'requester', ?, ?, ?, ?)`,
		id, string(synccatchup.StateRequested), request.DatabaseID, request.Nonce, string(request.WrappingMode)); err != nil {
		return CatchupSession{}, err
	}
	return s.catchupSessionLocked(id)
}

// AcceptCatchupOffer records the response this replica chose. Selection itself
// is synccatchup's; what lands here is the decision.
func (s *SQLiteStore) AcceptCatchupOffer(ctx context.Context, sessionID string, offer synccatchup.Offer) (CatchupSession, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return CatchupSession{}, err
	}
	vector, err := json.Marshal(offer.Response.SnapshotVector)
	if err != nil {
		return CatchupSession{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireCatchupTransitionLocked(sessionID, synccatchup.StateOffered); err != nil {
		return CatchupSession{}, err
	}
	if err := s.execPreparedLocked(
		`UPDATE sync_catchup_sessions SET state = ?, peer_replica_id = ?, snapshot_id = ?,
			snapshot_vector_json = ?, archive_sha256 = ?, archive_length = ?, expires_at = ?,
			updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(synccatchup.StateOffered), offer.Response.ResponderReplicaID, offer.Response.SnapshotID,
		string(vector), offer.Response.ArchiveSHA256, strconv.FormatInt(offer.Response.ArchiveLength, 10),
		offer.Response.ExpiresAt, sessionID); err != nil {
		return CatchupSession{}, err
	}
	return s.catchupSessionLocked(sessionID)
}

// AdvanceCatchup moves a session, refusing any transition the state machine
// does not allow. `reason` is kept so a failed or cancelled catch-up says why
// rather than only that it stopped.
func (s *SQLiteStore) AdvanceCatchup(ctx context.Context, sessionID string, state synccatchup.State, reason string) (CatchupSession, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return CatchupSession{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireCatchupTransitionLocked(sessionID, state); err != nil {
		return CatchupSession{}, err
	}
	if err := s.execPreparedLocked(
		`UPDATE sync_catchup_sessions SET state = ?, last_reason = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(state), reason, sessionID); err != nil {
		return CatchupSession{}, err
	}
	return s.catchupSessionLocked(sessionID)
}

// RecordCatchupProgress notes how much of the archive has arrived, which is
// what lets an interrupted transfer resume instead of restarting.
func (s *SQLiteStore) RecordCatchupProgress(ctx context.Context, sessionID string, receivedBytes int64) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if receivedBytes < 0 {
		return fmt.Errorf("%w: negative progress", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execPreparedLocked(
		`UPDATE sync_catchup_sessions SET received_bytes = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND state IN ('transferring', 'offered')`,
		strconv.FormatInt(receivedBytes, 10), sessionID)
}

// SetCatchupIntent records the explicit restore intent. There is no default:
// G10's boundary is that no restore is automatically destructive, and an
// unstated intent is exactly how one becomes so.
func (s *SQLiteStore) SetCatchupIntent(ctx context.Context, sessionID, intent string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	switch intent {
	case "replace", "merge", "fork", "adopt":
	default:
		return fmt.Errorf("%w: restore intent must be replace, merge, fork, or adopt", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execPreparedLocked(
		`UPDATE sync_catchup_sessions SET intent = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, intent, sessionID)
}

// CutoverToSnapshot installs the restored snapshot's state vector as this
// replica's own observation, so the next exchange asks only for what came
// after it. It is the moment a restored library stops being a pile of rows and
// becomes a participant again.
//
// It deliberately writes no peer acknowledgement. G10's boundary says a backup
// is not an acknowledgement and must not hold back collection: this replica
// having a copy says nothing about what any *peer* has durably admitted.
func (s *SQLiteStore) CutoverToSnapshot(ctx context.Context, sessionID string) (CatchupSession, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return CatchupSession{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, err := s.catchupSessionLocked(sessionID)
	if err != nil {
		return CatchupSession{}, err
	}
	if session.Intent == "" {
		return CatchupSession{}, fmt.Errorf("%w: cutover requires an explicit restore intent", ErrInvalidInput)
	}
	if err := s.requireCatchupTransitionLocked(sessionID, synccatchup.StateCutover); err != nil {
		return CatchupSession{}, err
	}
	if err := syncstate.ValidateVector(session.SnapshotVector); err != nil {
		return CatchupSession{}, err
	}
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return CatchupSession{}, err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return CatchupSession{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	for subject, sequence := range session.SnapshotVector {
		if subject == identity.ReplicaID {
			// A restored replica does not inherit the sequence of whichever
			// replica it was restored from. G3 mints a fresh identity for a
			// copied database, and G4's enrollment gives it a boundary of its
			// own starting at zero.
			continue
		}
		if err := s.execPreparedLocked(
			`INSERT INTO sync_state_vectors(observer_replica_id, subject_replica_id, contiguous_sequence)
			 VALUES(?, ?, ?)
			 ON CONFLICT(observer_replica_id, subject_replica_id) DO UPDATE SET
				contiguous_sequence = MAX(sync_state_vectors.contiguous_sequence, excluded.contiguous_sequence),
				updated_at = CURRENT_TIMESTAMP`,
			identity.ReplicaID, subject, strconv.FormatInt(sequence, 10)); err != nil {
			return CatchupSession{}, err
		}
		// The snapshot stands in for the operations it contains. Without this
		// floor the next operation would look like a gap, because admission
		// requires a predecessor row that a restored replica correctly does
		// not have.
		if err := s.execPreparedLocked(
			`INSERT INTO sync_catchup_floors(replica_id, sequence, snapshot_id) VALUES(?, ?, ?)
			 ON CONFLICT(replica_id) DO UPDATE SET
				sequence = MAX(sync_catchup_floors.sequence, excluded.sequence),
				snapshot_id = excluded.snapshot_id, established_at = CURRENT_TIMESTAMP`,
			subject, strconv.FormatInt(sequence, 10), session.SnapshotID); err != nil {
			return CatchupSession{}, err
		}
	}
	if err := s.execPreparedLocked(
		`UPDATE sync_catchup_sessions SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(synccatchup.StateCutover), sessionID); err != nil {
		return CatchupSession{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return CatchupSession{}, err
	}
	committed = true
	return s.catchupSessionLocked(sessionID)
}

// ExpireCatchupSessions moves past-expiry sessions that are still waiting into
// the expired state. It never touches one that has begun restoring: canonical
// rows are being written, and a clock is not a reason to abandon them.
func (s *SQLiteStore) ExpireCatchupSessions(ctx context.Context, now time.Time) (int, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before, err := s.countLocked(
		`SELECT COUNT(*) FROM sync_catchup_sessions
		  WHERE state IN ('requested', 'offered', 'transferring') AND expires_at <> '' AND expires_at <= ?`,
		now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	if err := s.execPreparedLocked(
		`UPDATE sync_catchup_sessions SET state = 'expired', last_reason = 'response expired',
			updated_at = CURRENT_TIMESTAMP
		  WHERE state IN ('requested', 'offered', 'transferring') AND expires_at <> '' AND expires_at <= ?`,
		now.UTC().Format(time.RFC3339Nano)); err != nil {
		return 0, err
	}
	return int(before), nil
}

// GetCatchupSession reads one session.
func (s *SQLiteStore) GetCatchupSession(ctx context.Context, sessionID string) (CatchupSession, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return CatchupSession{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.catchupSessionLocked(sessionID)
}

func (s *SQLiteStore) requireCatchupTransitionLocked(sessionID string, to synccatchup.State) error {
	session, err := s.catchupSessionLocked(sessionID)
	if err != nil {
		return err
	}
	return synccatchup.Transition(session.State, to)
}

func (s *SQLiteStore) catchupSessionLocked(sessionID string) (CatchupSession, error) {
	stmt, err := s.prepareLocked(`SELECT id, role, state, database_id, peer_replica_id, request_nonce,
		snapshot_id, snapshot_vector_json, archive_sha256, archive_length, received_bytes,
		wrapping_mode, intent, expires_at, last_reason FROM sync_catchup_sessions WHERE id = ?`)
	if err != nil {
		return CatchupSession{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{sessionID}); err != nil {
		return CatchupSession{}, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		session := CatchupSession{
			ID: columnText(stmt, 0), Role: columnText(stmt, 1), State: synccatchup.State(columnText(stmt, 2)),
			DatabaseID: columnText(stmt, 3), PeerReplicaID: columnText(stmt, 4), RequestNonce: columnText(stmt, 5),
			SnapshotID: columnText(stmt, 6), ArchiveSHA256: columnText(stmt, 8),
			ArchiveLength: columnInt64(stmt, 9), ReceivedBytes: columnInt64(stmt, 10),
			WrappingMode: synccatchup.WrappingMode(columnText(stmt, 11)),
			Intent:       columnText(stmt, 12), ExpiresAt: columnText(stmt, 13), LastReason: columnText(stmt, 14),
		}
		if err := json.Unmarshal([]byte(columnText(stmt, 7)), &session.SnapshotVector); err != nil {
			return CatchupSession{}, err
		}
		return session, nil
	case C.SQLITE_DONE:
		return CatchupSession{}, ErrNotFound
	default:
		return CatchupSession{}, s.stepErrLocked(rc)
	}
}

// catchupFloorCoversLocked reports whether a restored snapshot already accounts
// for a replica's sequence. It is the only thing allowed to stand in for a
// missing predecessor operation, and it exists because a replica built from a
// snapshot legitimately has canonical state without the operations that
// produced it.
func (s *SQLiteStore) catchupFloorCoversLocked(replicaID string, sequence int64) (bool, error) {
	if sequence <= 0 {
		return false, nil
	}
	count, err := s.countLocked(
		`SELECT COUNT(*) FROM sync_catchup_floors WHERE replica_id = ? AND sequence >= ?`,
		replicaID, strconv.FormatInt(sequence, 10))
	if err != nil {
		return false, err
	}
	return count == 1, nil
}
