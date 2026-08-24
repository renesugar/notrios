package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	syncTargetIDPattern = regexp.MustCompile(`^target_[a-f0-9]{32,64}$`)
	syncWorkerPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	syncRetryCodes      = map[string]bool{
		"offline": true, "quota": true, "temporary": true, "byte_budget": true,
	}
)

const sqliteTimeLayout = "2006-01-02 15:04:05"

// ensureSchemaV26 adds G15's sync-only durable outbox and bounded audit trail.
func (s *SQLiteStore) ensureSchemaV26(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if version >= 26 {
		return nil
	}
	migration, err := migrationFS.ReadFile("migrations/0026_sync_jobs.sql")
	if err != nil {
		return fmt.Errorf("read schema v26 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

// SyncTargetID converts a configured target identity into the opaque value
// safe to persist and expose. Callers provide a canonical type+location string;
// only its digest leaves this function.
func SyncTargetID(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("target_%x", sum[:])
}

// SyncRetryDelay is exponential backoff with stable ±20% jitter. Stable jitter
// makes restart tests deterministic while still preventing replicas created at
// the same instant from retrying in lockstep. Attempt 1 waits about five
// seconds; the result is capped at fifteen minutes.
func SyncRetryDelay(jobID string, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exponent := attempt - 1
	if exponent > 8 {
		exponent = 8
	}
	base := 5 * time.Second * time.Duration(1<<exponent)
	if base > 15*time.Minute {
		base = 15 * time.Minute
	}
	digest := sha256.Sum256([]byte(jobID + ":" + strconv.Itoa(attempt)))
	// 800..1200 permille, inclusive.
	permille := int64(800 + binary.BigEndian.Uint16(digest[:2])%401)
	delay := time.Duration(int64(base) * permille / 1000)
	if delay > 15*time.Minute {
		return 15 * time.Minute
	}
	return delay
}

func (r *CreateSyncJobRequest) validate() error {
	r.Kind = strings.TrimSpace(r.Kind)
	r.Actor = strings.ToLower(strings.TrimSpace(r.Actor))
	r.TargetID = strings.TrimSpace(r.TargetID)
	if !IsSyncJobKind(r.Kind) {
		return fmt.Errorf("%w: sync job kind is not supported", ErrInvalidInput)
	}
	switch r.Actor {
	case SyncJobActorCLI, SyncJobActorREST, SyncJobActorMCP, SyncJobActorService:
	default:
		return fmt.Errorf("%w: sync job actor is not supported", ErrInvalidInput)
	}
	if !syncTargetIDPattern.MatchString(r.TargetID) {
		return fmt.Errorf("%w: sync target must be an opaque target digest", ErrInvalidInput)
	}
	if len(r.TargetID) > MaxSyncJobTargetBytes {
		return fmt.Errorf("%w: sync target exceeds %d bytes", ErrInvalidInput, MaxSyncJobTargetBytes)
	}
	if r.ByteBudget <= 0 || r.ByteBudget > MaxSyncJobByteBudget {
		return fmt.Errorf("%w: sync byte budget must be between 1 and %d", ErrInvalidInput, MaxSyncJobByteBudget)
	}
	if r.MaxAttempts == 0 {
		r.MaxAttempts = DefaultSyncJobAttempts
	}
	if r.MaxAttempts < 1 || r.MaxAttempts > MaxSyncJobAttempts {
		return fmt.Errorf("%w: max attempts must be between 1 and %d", ErrInvalidInput, MaxSyncJobAttempts)
	}
	return nil
}

// CreateSyncJob atomically adds the ordinary job record and its sync outbox
// metadata. No executable command or target location is persisted.
func (s *SQLiteStore) CreateSyncJob(ctx context.Context, req CreateSyncJobRequest) (SyncJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJob{}, err
	}
	if err := req.validate(); err != nil {
		return SyncJob{}, err
	}
	jobID, err := NewID("job")
	if err != nil {
		return SyncJob{}, err
	}
	eventID, err := NewID("audit")
	if err != nil {
		return SyncJob{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`INSERT INTO jobs(id, kind, status, collection_id, parameters)
		VALUES(?, ?, ?, 'default', '[]')`, jobID, req.Kind, JobQueued); err != nil {
		return SyncJob{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_jobs(job_id, actor, target_id, max_attempts, byte_budget)
		VALUES(?, ?, ?, ?, ?)`, jobID, req.Actor, req.TargetID, strconv.Itoa(req.MaxAttempts), strconv.FormatInt(req.ByteBudget, 10)); err != nil {
		return SyncJob{}, err
	}
	if err := s.insertSyncJobAuditLocked(eventID, jobID, "queued", map[string]any{
		"actor": req.Actor, "byte_budget": req.ByteBudget,
	}); err != nil {
		return SyncJob{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncJob{}, err
	}
	committed = true
	return s.getSyncJobLocked(jobID)
}

// ClaimSyncJob leases the oldest due job while holding an IMMEDIATE
// transaction. A second process therefore cannot claim the same job or another
// job for the same target. Stale leases are first put back in the queue with
// their durable checkpoint intact.
func (s *SQLiteStore) ClaimSyncJob(ctx context.Context, workerID string, now time.Time) (SyncJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJob{}, err
	}
	workerID = strings.TrimSpace(workerID)
	if !syncWorkerPattern.MatchString(workerID) {
		return SyncJob{}, fmt.Errorf("%w: invalid sync worker ID", ErrInvalidInput)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	cutoff := now.Add(-JobHeartbeatTimeout)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	stale, err := s.syncJobIDsLocked(`SELECT sj.job_id FROM sync_jobs sj JOIN jobs j ON j.id = sj.job_id
		WHERE j.status = ? AND COALESCE(j.heartbeat_at, j.started_at, j.created_at) < ?`, JobRunning, formatSQLiteTime(cutoff))
	if err != nil {
		return SyncJob{}, err
	}
	for _, jobID := range stale {
		staleJob, getErr := s.getSyncJobLocked(jobID)
		if getErr != nil {
			return SyncJob{}, getErr
		}
		if staleJob.Job.CancelRequested {
			if err := s.finishSyncJobLocked(jobID, staleJob.LeaseOwner, JobCancelled,
				map[string]any{"attempts": staleJob.Attempt}, nil); err != nil {
				return SyncJob{}, err
			}
			if err := s.insertNewSyncJobAuditLocked(jobID, "cancelled", map[string]any{"after_interruption": true}); err != nil {
				return SyncJob{}, err
			}
			continue
		}
		if staleJob.Attempt >= staleJob.MaxAttempts {
			if err := s.finishSyncJobLocked(jobID, staleJob.LeaseOwner, JobFailed,
				map[string]any{"attempts": staleJob.Attempt, "retry_code": "temporary"}, errors.New("sync retry limit reached")); err != nil {
				return SyncJob{}, err
			}
			if err := s.insertNewSyncJobAuditLocked(jobID, "retry_exhausted", map[string]any{"code": "temporary", "after_interruption": true}); err != nil {
				return SyncJob{}, err
			}
			continue
		}
		delay := SyncRetryDelay(jobID, staleJob.Attempt)
		next := now.Add(delay)
		if err := s.execPreparedLocked(`UPDATE jobs SET status = ?, started_at = NULL, finished_at = NULL,
			heartbeat_at = ?, error = 'sync worker interrupted' WHERE id = ?`, JobQueued, formatSQLiteTime(now), jobID); err != nil {
			return SyncJob{}, err
		}
		if err := s.execPreparedLocked(`UPDATE sync_jobs SET lease_owner = '', retry_code = 'temporary',
			next_attempt_at = ?, bytes_used = 0 WHERE job_id = ?`, formatSQLiteTime(next), jobID); err != nil {
			return SyncJob{}, err
		}
		if err := s.insertNewSyncJobAuditLocked(jobID, "interrupted", map[string]any{"retry_in_ms": delay.Milliseconds()}); err != nil {
			return SyncJob{}, err
		}
	}

	// A queued job cancelled before claim settles without running, with the
	// cancellation represented in the same durable audit transaction.
	preCancelled, err := s.syncJobIDsLocked(`SELECT sj.job_id FROM sync_jobs sj JOIN jobs j ON j.id = sj.job_id
		WHERE j.status = ? AND j.cancel_requested = 1`, JobQueued)
	if err != nil {
		return SyncJob{}, err
	}
	for _, jobID := range preCancelled {
		if err := s.execPreparedLocked(`UPDATE jobs SET status = ?, finished_at = ?, heartbeat_at = ? WHERE id = ?`,
			JobCancelled, formatSQLiteTime(now), formatSQLiteTime(now), jobID); err != nil {
			return SyncJob{}, err
		}
		if err := s.insertNewSyncJobAuditLocked(jobID, "cancelled", map[string]any{"before_claim": true}); err != nil {
			return SyncJob{}, err
		}
	}

	stmt, err := s.prepareLocked(`SELECT sj.job_id
		FROM sync_jobs sj JOIN jobs j ON j.id = sj.job_id
		WHERE j.status = ? AND j.cancel_requested = 0
		  AND (sj.next_attempt_at IS NULL OR sj.next_attempt_at = '' OR sj.next_attempt_at <= ?)
		  AND NOT EXISTS (
			SELECT 1 FROM sync_jobs active JOIN jobs running ON running.id = active.job_id
			WHERE active.target_id = sj.target_id AND active.job_id <> sj.job_id AND running.status = ?
		  )
		ORDER BY j.rowid ASC LIMIT 1`)
	if err != nil {
		return SyncJob{}, err
	}
	if err := bindAll(stmt, []string{JobQueued, formatSQLiteTime(now), JobRunning}); err != nil {
		C.sqlite3_finalize(stmt)
		return SyncJob{}, err
	}
	jobID := ""
	if rc := C.sqlite3_step(stmt); rc == C.SQLITE_ROW {
		jobID = columnText(stmt, 0)
	} else if rc != C.SQLITE_DONE {
		C.sqlite3_finalize(stmt)
		return SyncJob{}, s.stepErrLocked(rc)
	}
	C.sqlite3_finalize(stmt)
	if jobID == "" {
		if err := s.execLocked("COMMIT"); err != nil {
			return SyncJob{}, err
		}
		committed = true
		return SyncJob{}, fmt.Errorf("%w: no due sync job", ErrNotFound)
	}
	if err := s.execPreparedLocked(`UPDATE jobs SET status = ?, started_at = ?, finished_at = NULL,
		heartbeat_at = ?, error = '' WHERE id = ? AND status = ?`,
		JobRunning, formatSQLiteTime(now), formatSQLiteTime(now), jobID, JobQueued); err != nil {
		return SyncJob{}, err
	}
	if err := s.execPreparedLocked(`UPDATE sync_jobs SET attempt = attempt + 1, lease_owner = ?,
		next_attempt_at = NULL, retry_code = '', bytes_used = 0 WHERE job_id = ?`, workerID, jobID); err != nil {
		return SyncJob{}, err
	}
	if err := s.insertNewSyncJobAuditLocked(jobID, "claimed", map[string]any{"worker": workerID}); err != nil {
		return SyncJob{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncJob{}, err
	}
	committed = true
	return s.getSyncJobLocked(jobID)
}

func (s *SQLiteStore) CheckpointSyncJob(ctx context.Context, jobID, workerID string, checkpoint SyncJobCheckpoint) (SyncJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJob{}, err
	}
	encoded, err := json.Marshal(checkpoint.Checkpoint)
	if err != nil {
		return SyncJob{}, err
	}
	if len(encoded) > MaxSyncJobCheckpointBytes || checkpoint.Processed < 0 || checkpoint.Total < 0 || checkpoint.BytesUsed < 0 {
		return SyncJob{}, fmt.Errorf("%w: invalid or oversized sync checkpoint", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	current, err := s.getSyncJobLocked(jobID)
	if err != nil {
		return SyncJob{}, err
	}
	storedState, err := s.storedJobStateLocked(jobID)
	if err != nil {
		return SyncJob{}, err
	}
	if storedState != JobRunning || current.LeaseOwner != workerID {
		return SyncJob{}, fmt.Errorf("%w: sync job is not leased by this worker", ErrConflict)
	}
	if checkpoint.BytesUsed < current.BytesUsed || checkpoint.BytesUsed > current.ByteBudget {
		return SyncJob{}, fmt.Errorf("%w: sync byte budget exceeded or moved backward", ErrConflict)
	}
	if err := s.execPreparedLocked(`UPDATE jobs SET phase = ?, processed = ?, total = ?, heartbeat_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = ?`, checkpoint.Phase, strconv.FormatInt(checkpoint.Processed, 10),
		strconv.FormatInt(checkpoint.Total, 10), jobID, JobRunning); err != nil {
		return SyncJob{}, err
	}
	if err := s.execPreparedLocked(`UPDATE sync_jobs SET bytes_used = ?, checkpoint = ? WHERE job_id = ?`,
		strconv.FormatInt(checkpoint.BytesUsed, 10), string(encoded), jobID); err != nil {
		return SyncJob{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncJob{}, err
	}
	committed = true
	return s.getSyncJobLocked(jobID)
}

func (s *SQLiteStore) RescheduleSyncJob(ctx context.Context, jobID, workerID, retryCode string, now time.Time) (SyncJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJob{}, err
	}
	retryCode = strings.ToLower(strings.TrimSpace(retryCode))
	if !syncRetryCodes[retryCode] {
		return SyncJob{}, fmt.Errorf("%w: unsupported sync retry code", ErrInvalidInput)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	current, err := s.getSyncJobLocked(jobID)
	if err != nil {
		return SyncJob{}, err
	}
	state, err := s.storedJobStateLocked(jobID)
	if err != nil {
		return SyncJob{}, err
	}
	if state != JobRunning || current.LeaseOwner != workerID {
		return SyncJob{}, fmt.Errorf("%w: sync job is not leased by this worker", ErrConflict)
	}
	if current.Attempt >= current.MaxAttempts {
		if err := s.finishSyncJobLocked(jobID, workerID, JobFailed, map[string]any{
			"attempts": current.Attempt, "retry_code": retryCode,
		}, errors.New("sync retry limit reached")); err != nil {
			return SyncJob{}, err
		}
		if err := s.insertNewSyncJobAuditLocked(jobID, "retry_exhausted", map[string]any{"code": retryCode}); err != nil {
			return SyncJob{}, err
		}
		if err := s.execLocked("COMMIT"); err != nil {
			return SyncJob{}, err
		}
		committed = true
		return s.getSyncJobLocked(jobID)
	}
	delay := SyncRetryDelay(jobID, current.Attempt)
	next := now.UTC().Add(delay)
	if err := s.execPreparedLocked(`UPDATE jobs SET status = ?, started_at = NULL, finished_at = NULL,
		heartbeat_at = ?, error = ? WHERE id = ?`, JobQueued, formatSQLiteTime(now), "sync retry pending: "+retryCode, jobID); err != nil {
		return SyncJob{}, err
	}
	if err := s.execPreparedLocked(`UPDATE sync_jobs SET lease_owner = '', retry_code = ?, next_attempt_at = ?,
		bytes_used = 0 WHERE job_id = ?`, retryCode, formatSQLiteTime(next), jobID); err != nil {
		return SyncJob{}, err
	}
	if err := s.insertNewSyncJobAuditLocked(jobID, "retry_scheduled", map[string]any{
		"code": retryCode, "retry_in_ms": delay.Milliseconds(), "attempt": current.Attempt,
	}); err != nil {
		return SyncJob{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncJob{}, err
	}
	committed = true
	return s.getSyncJobLocked(jobID)
}

func (s *SQLiteStore) FinishSyncJob(ctx context.Context, jobID, workerID, state string, summary map[string]any, failure error) (SyncJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJob{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.finishSyncJobLocked(jobID, workerID, state, summary, failure); err != nil {
		return SyncJob{}, err
	}
	event := "completed"
	if state == JobFailed {
		event = "failed"
	} else if state == JobCancelled {
		event = "cancelled"
	}
	if err := s.insertNewSyncJobAuditLocked(jobID, event, map[string]any{"state": state}); err != nil {
		return SyncJob{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncJob{}, err
	}
	committed = true
	return s.getSyncJobLocked(jobID)
}

func (s *SQLiteStore) finishSyncJobLocked(jobID, workerID, state string, summary map[string]any, failure error) error {
	if !IsSettledJobState(state) || state == JobInterrupted {
		return fmt.Errorf("%w: invalid sync finish state", ErrInvalidInput)
	}
	current, err := s.getSyncJobLocked(jobID)
	if err != nil {
		return err
	}
	storedState, err := s.storedJobStateLocked(jobID)
	if err != nil {
		return err
	}
	if storedState != JobRunning || current.LeaseOwner != workerID {
		return fmt.Errorf("%w: sync job is not leased by this worker", ErrConflict)
	}
	encoded, err := json.Marshal(summary)
	if err != nil || len(encoded) > MaxJobSummaryBytes {
		return fmt.Errorf("%w: invalid or oversized sync summary", ErrInvalidInput)
	}
	message := ""
	if failure != nil {
		// Sync adapters can report URLs and provider messages. Keep them in local
		// logs, not in a durable/MCP-visible record.
		message = "sync operation failed"
	}
	if err := s.execPreparedLocked(`UPDATE jobs SET status = ?, summary = ?, error = ?, finished_at = CURRENT_TIMESTAMP,
		heartbeat_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?`, state, string(encoded), message, jobID, JobRunning); err != nil {
		return err
	}
	return s.execPreparedLocked(`UPDATE sync_jobs SET lease_owner = '', next_attempt_at = NULL WHERE job_id = ?`, jobID)
}

// RetrySyncJob is the explicit operator retry. reset=false preserves the
// durable phase checkpoint; reset=true clears it and the attempt counter. The
// latter is intentionally kept out of MCP by the HTTP/MCP adapter.
func (s *SQLiteStore) RetrySyncJob(ctx context.Context, jobID string, reset bool, now time.Time) (SyncJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJob{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return SyncJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	current, err := s.getSyncJobLocked(jobID)
	if err != nil {
		return SyncJob{}, err
	}
	state, err := s.storedJobStateLocked(jobID)
	if err != nil {
		return SyncJob{}, err
	}
	if state == JobRunning {
		return SyncJob{}, fmt.Errorf("%w: a running sync job cannot be retried", ErrConflict)
	}
	checkpoint := current.Checkpoint
	attempt := current.Attempt
	if reset {
		checkpoint = map[string]any{}
		attempt = 0
	}
	encoded, _ := json.Marshal(checkpoint)
	if err := s.execPreparedLocked(`UPDATE jobs SET status = ?, cancel_requested = 0, started_at = NULL,
		finished_at = NULL, heartbeat_at = ?, error = '', summary = '{}'
		WHERE id = ?`, JobQueued, formatSQLiteTime(now), jobID); err != nil {
		return SyncJob{}, err
	}
	if err := s.execPreparedLocked(`UPDATE sync_jobs SET attempt = ?, next_attempt_at = ?, retry_code = '',
		bytes_used = 0, checkpoint = ?, lease_owner = '' WHERE job_id = ?`, strconv.Itoa(attempt),
		formatSQLiteTime(now), string(encoded), jobID); err != nil {
		return SyncJob{}, err
	}
	event := "manual_retry"
	if reset {
		event = "manual_reset"
	}
	if err := s.insertNewSyncJobAuditLocked(jobID, event, map[string]any{"checkpoint_preserved": !reset}); err != nil {
		return SyncJob{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return SyncJob{}, err
	}
	committed = true
	return s.getSyncJobLocked(jobID)
}

func (s *SQLiteStore) GetSyncJob(ctx context.Context, jobID string) (SyncJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJob{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getSyncJobLocked(strings.TrimSpace(jobID))
}

func (s *SQLiteStore) getSyncJobLocked(jobID string) (SyncJob, error) {
	job, err := s.getJobLocked(jobID)
	if err != nil {
		return SyncJob{}, err
	}
	stmt, err := s.prepareLocked(`SELECT actor, target_id, attempt, max_attempts, COALESCE(next_attempt_at, ''),
		retry_code, byte_budget, bytes_used, checkpoint, lease_owner FROM sync_jobs WHERE job_id = ?`)
	if err != nil {
		return SyncJob{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{jobID}); err != nil {
		return SyncJob{}, err
	}
	if rc := C.sqlite3_step(stmt); rc == C.SQLITE_DONE {
		return SyncJob{}, fmt.Errorf("%w: job %q is not a sync job", ErrNotFound, jobID)
	} else if rc != C.SQLITE_ROW {
		return SyncJob{}, s.stepErrLocked(rc)
	}
	result := SyncJob{
		Job: job, Actor: columnText(stmt, 0), TargetID: columnText(stmt, 1),
		Attempt: int(columnInt64(stmt, 2)), MaxAttempts: int(columnInt64(stmt, 3)),
		NextAttemptAt: parseSQLiteTime(columnText(stmt, 4)), RetryCode: columnText(stmt, 5),
		ByteBudget: columnInt64(stmt, 6), BytesUsed: columnInt64(stmt, 7), LeaseOwner: columnText(stmt, 9),
	}
	_ = json.Unmarshal([]byte(columnText(stmt, 8)), &result.Checkpoint)
	return result, nil
}

func (s *SQLiteStore) ListSyncJobAudit(ctx context.Context, jobID string, limit int) (SyncJobAuditList, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncJobAuditList{}, err
	}
	if limit <= 0 {
		limit = DefaultSyncJobAuditRows
	}
	if limit > MaxSyncJobAuditRows {
		limit = MaxSyncJobAuditRows
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, job_id, event_type, details_json, created_at
		FROM sync_job_audit WHERE job_id = ? ORDER BY rowid DESC LIMIT ?`)
	if err != nil {
		return SyncJobAuditList{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{jobID, strconv.Itoa(limit + 1)}); err != nil {
		return SyncJobAuditList{}, err
	}
	result := SyncJobAuditList{Events: []SyncJobAuditEvent{}}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return SyncJobAuditList{}, s.stepErrLocked(rc)
		}
		if len(result.Events) == limit {
			result.Truncated = true
			break
		}
		event := SyncJobAuditEvent{ID: columnText(stmt, 0), JobID: columnText(stmt, 1),
			EventType: columnText(stmt, 2), CreatedAt: parseSQLiteTime(columnText(stmt, 4))}
		_ = json.Unmarshal([]byte(columnText(stmt, 3)), &event.Details)
		result.Events = append(result.Events, event)
	}
	return result, nil
}

func (s *SQLiteStore) ListSyncConflicts(ctx context.Context, limit int) (SyncConflictPage, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncConflictPage{}, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, document_id, base_revision_id, revision_a, revision_b, kind, created_at
		FROM sync_document_conflicts ORDER BY created_at DESC, id DESC LIMIT ?`)
	if err != nil {
		return SyncConflictPage{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strconv.Itoa(limit + 1)}); err != nil {
		return SyncConflictPage{}, err
	}
	result := SyncConflictPage{Conflicts: []SyncConflictSummary{}}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return SyncConflictPage{}, s.stepErrLocked(rc)
		}
		if len(result.Conflicts) == limit {
			result.Truncated = true
			break
		}
		result.Conflicts = append(result.Conflicts, SyncConflictSummary{
			ID: columnText(stmt, 0), DocumentID: columnText(stmt, 1), BaseRevisionID: columnText(stmt, 2),
			RevisionA: columnText(stmt, 3), RevisionB: columnText(stmt, 4), Kind: columnText(stmt, 5), CreatedAt: columnText(stmt, 6),
		})
	}
	return result, nil
}

func (s *SQLiteStore) insertNewSyncJobAuditLocked(jobID, eventType string, details map[string]any) error {
	eventID, err := NewID("audit")
	if err != nil {
		return err
	}
	return s.insertSyncJobAuditLocked(eventID, jobID, eventType, details)
}

func (s *SQLiteStore) insertSyncJobAuditLocked(eventID, jobID, eventType string, details map[string]any) error {
	encoded, err := json.Marshal(details)
	if err != nil || len(encoded) > MaxJobSummaryBytes {
		return fmt.Errorf("%w: invalid sync audit details", ErrInvalidInput)
	}
	return s.execPreparedLocked(`INSERT INTO sync_job_audit(id, job_id, event_type, details_json) VALUES(?, ?, ?, ?)`,
		eventID, jobID, eventType, string(encoded))
}

func (s *SQLiteStore) syncJobIDsLocked(query string, args ...string) ([]string, error) {
	stmt, err := s.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return nil, err
	}
	var ids []string
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return ids, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		ids = append(ids, columnText(stmt, 0))
	}
}

func (s *SQLiteStore) storedJobStateLocked(jobID string) (string, error) {
	stmt, err := s.prepareLocked(`SELECT status FROM jobs WHERE id = ?`)
	if err != nil {
		return "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{jobID}); err != nil {
		return "", err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return columnText(stmt, 0), nil
	case C.SQLITE_DONE:
		return "", fmt.Errorf("%w: job %q", ErrNotFound, jobID)
	default:
		return "", s.stepErrLocked(rc)
	}
}

func formatSQLiteTime(value time.Time) string { return value.UTC().Format(sqliteTimeLayout) }
