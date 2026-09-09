package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"strconv"
	"strings"
)

// index_outbox operations (Notrios redesign task R7). Document mutations
// enqueue jobs inside their transaction; the projection worker drains pending
// jobs after commit and mirrors managed notes into the filesystem projection
// that the Recoll sidecar indexes. Failures never invalidate the note save.

func (s *SQLiteStore) enqueueProjectionLocked(objectID, operation string) error {
	return s.execPreparedLocked(`INSERT INTO index_outbox(object_type, object_id, operation) VALUES('document', ?, ?)`, objectID, operation)
}

// PendingProjectionJobs returns incomplete outbox jobs in sequence order.
// Repeated jobs for the same document are returned as-is; the worker applies
// them in order, so the final state always wins.
func (s *SQLiteStore) PendingProjectionJobs(ctx context.Context, limit int) ([]OutboxJob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT sequence, object_type, object_id, operation,
			attempt_count, COALESCE(next_attempt_at, '')
		FROM index_outbox
		WHERE completed_at IS NULL
		  AND (next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP)
		ORDER BY sequence LIMIT ` + itoa(limit))
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	jobs := []OutboxJob{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			jobs = append(jobs, OutboxJob{
				Sequence:      columnInt64(stmt, 0),
				ObjectType:    columnText(stmt, 1),
				ObjectID:      columnText(stmt, 2),
				Operation:     columnText(stmt, 3),
				AttemptCount:  int(columnInt64(stmt, 4)),
				NextAttemptAt: parseSQLiteTime(columnText(stmt, 5)),
			})
		case C.SQLITE_DONE:
			return jobs, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// CompleteProjectionJob marks a job done, or records the failure and leaves
// it pending for retry.
func (s *SQLiteStore) CompleteProjectionJob(ctx context.Context, sequence int64, jobErr error) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := strings.TrimSpace(strconv.FormatInt(sequence, 10))
	if jobErr != nil {
		message := jobErr.Error()
		if len(message) > 2048 {
			message = message[:2048]
		}
		return s.execPreparedLocked(`UPDATE index_outbox
			SET attempt_count = attempt_count + 1,
			    error_text = ?,
			    next_attempt_at = datetime('now', '+' ||
			      MIN(3600, 5 * (1 << MIN(attempt_count, 10))) || ' seconds')
			WHERE sequence = `+seq, message)
	}
	return s.execLocked(`UPDATE index_outbox
		SET completed_at = CURRENT_TIMESTAMP, error_text = NULL, next_attempt_at = NULL
		WHERE sequence = ` + seq)
}

func (s *SQLiteStore) ProjectionQueueStatus(ctx context.Context) (ProjectionQueueStatus, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return ProjectionQueueStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN attempt_count > 0 THEN 1 ELSE 0 END), 0),
			COALESCE(MIN(created_at), ''),
			COALESCE(MIN(next_attempt_at), '')
		FROM index_outbox WHERE completed_at IS NULL`)
	if err != nil {
		return ProjectionQueueStatus{}, err
	}
	defer C.sqlite3_finalize(stmt)
	rc := C.sqlite3_step(stmt)
	if rc != C.SQLITE_ROW {
		if rc == C.SQLITE_DONE {
			return ProjectionQueueStatus{}, nil
		}
		return ProjectionQueueStatus{}, s.stepErrLocked(rc)
	}
	return ProjectionQueueStatus{
		Pending:         int(columnInt64(stmt, 0)),
		Due:             int(columnInt64(stmt, 1)),
		Failed:          int(columnInt64(stmt, 2)),
		OldestCreatedAt: parseSQLiteTime(columnText(stmt, 3)),
		NextAttemptAt:   parseSQLiteTime(columnText(stmt, 4)),
	}, nil
}
