package store

/*
#include <sqlite3.h>
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
	stmt, err := s.prepareLocked(`SELECT sequence, object_type, object_id, operation FROM index_outbox WHERE completed_at IS NULL ORDER BY sequence LIMIT ` + itoa(limit))
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
				Sequence:   columnInt64(stmt, 0),
				ObjectType: columnText(stmt, 1),
				ObjectID:   columnText(stmt, 2),
				Operation:  columnText(stmt, 3),
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
		return s.execPreparedLocked(`UPDATE index_outbox SET attempt_count = attempt_count + 1, error_text = ? WHERE sequence = `+seq, jobErr.Error())
	}
	return s.execLocked(`UPDATE index_outbox SET completed_at = CURRENT_TIMESTAMP, error_text = NULL WHERE sequence = ` + seq)
}
