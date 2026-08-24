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
	"strings"
	"time"
)

const jobColumns = `id, kind, status, collection_id, parameters, phase, processed, total,
	summary, error, cancel_requested, created_at,
	COALESCE(started_at, ''), COALESCE(finished_at, ''), COALESCE(heartbeat_at, '')`

// CreateJob records a run that is about to start.
func (s *SQLiteStore) CreateJob(ctx context.Context, req CreateJobRequest) (Job, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	if err := req.validate(); err != nil {
		return Job{}, err
	}
	parameters, err := json.Marshal(req.Parameters)
	if err != nil {
		return Job{}, err
	}
	if len(parameters) > MaxJobParameterBytes {
		return Job{}, fmt.Errorf("%w: job parameters exceed %d bytes", ErrInvalidInput, MaxJobParameterBytes)
	}
	id, err := NewID("job")
	if err != nil {
		return Job{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(
		`INSERT INTO jobs(id, kind, status, collection_id, parameters) VALUES(?, ?, ?, ?, ?)`,
		id, req.Kind, JobQueued, req.CollectionID, string(parameters)); err != nil {
		return Job{}, err
	}
	return s.getJobLocked(id)
}

// StartJob moves a queued record to running and starts its heartbeat.
func (s *SQLiteStore) StartJob(ctx context.Context, jobID string) (Job, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(
		`UPDATE jobs SET status = ?, started_at = CURRENT_TIMESTAMP, heartbeat_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status = ?`, JobRunning, jobID, JobQueued); err != nil {
		return Job{}, err
	}
	return s.getJobLocked(jobID)
}

// TouchJob refreshes a running job's heartbeat without changing its last
// durable progress point. Timer heartbeats used to call ReportJobProgress with
// an empty value, which erased useful phase/progress information every five
// seconds.
func (s *SQLiteStore) TouchJob(ctx context.Context, jobID string) (bool, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(
		`UPDATE jobs SET heartbeat_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?`,
		jobID, JobRunning); err != nil {
		return false, err
	}
	job, err := s.getJobLocked(jobID)
	if err != nil {
		return false, err
	}
	return job.CancelRequested, nil
}

// ReportJobProgress records a progress point and refreshes the heartbeat.
//
// It returns whether cancellation has been requested, because the caller is
// already at a durable boundary and that is the only place stopping is safe.
// Two round trips — "am I cancelled?" then "here is my progress" — would give
// the same answer at twice the cost, and the answer is only actionable here.
func (s *SQLiteStore) ReportJobProgress(ctx context.Context, jobID string, progress JobProgress) (bool, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(
		`UPDATE jobs SET phase = ?, processed = ?, total = ?, heartbeat_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status = ?`,
		progress.Phase, strconv.FormatInt(progress.Processed, 10), strconv.FormatInt(progress.Total, 10),
		jobID, JobRunning); err != nil {
		return false, err
	}
	job, err := s.getJobLocked(jobID)
	if err != nil {
		return false, err
	}
	return job.CancelRequested, nil
}

// FinishJob settles a record. `state` must be a settled state.
func (s *SQLiteStore) FinishJob(ctx context.Context, jobID, state string, summary map[string]any, failure error) (Job, error) {
	ctx = contextOrBackground(ctx)
	if !IsSettledJobState(state) || state == JobInterrupted {
		// JobInterrupted is derived from silence and can never be written: a
		// process that could write it is a process that was not interrupted.
		return Job{}, fmt.Errorf("%w: %q is not a state a job can be finished in", ErrInvalidInput, state)
	}
	encoded := "{}"
	if len(summary) > 0 {
		raw, err := json.Marshal(summary)
		if err != nil {
			return Job{}, err
		}
		if len(raw) > MaxJobSummaryBytes {
			return Job{}, fmt.Errorf("%w: job summary exceeds %d bytes", ErrInvalidInput, MaxJobSummaryBytes)
		}
		encoded = string(raw)
	}
	message := ""
	if failure != nil {
		message = failure.Error()
		if len(message) > MaxJobErrorBytes {
			message = message[:MaxJobErrorBytes]
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(
		`UPDATE jobs SET status = ?, summary = ?, error = ?, finished_at = CURRENT_TIMESTAMP,
			heartbeat_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status IN (?, ?)`,
		state, encoded, message, jobID, JobQueued, JobRunning); err != nil {
		return Job{}, err
	}
	return s.getJobLocked(jobID)
}

// RequestJobCancel asks a job to stop at its next durable boundary.
//
// Cooperative, and that is the whole point: the importers commit and checkpoint
// per batch, so stopping between batches leaves a state the next run resumes
// from. Killing the work mid-batch would leave the same durable state and a
// worse story about it.
func (s *SQLiteStore) RequestJobCancel(ctx context.Context, jobID string) (Job, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.getJobLocked(jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Settled() {
		// Not an error: asking a finished job to stop is already true. Failing
		// here would make a retry-on-timeout script report a problem it does
		// not have.
		return job, nil
	}
	if err := s.execPreparedLocked(`UPDATE jobs SET cancel_requested = 1 WHERE id = ?`, jobID); err != nil {
		return Job{}, err
	}
	return s.getJobLocked(jobID)
}

// GetJob reads one record.
func (s *SQLiteStore) GetJob(ctx context.Context, jobID string) (Job, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getJobLocked(strings.TrimSpace(jobID))
}

// ListJobs returns recent records, newest first.
func (s *SQLiteStore) ListJobs(ctx context.Context, req JobListRequest) (JobList, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return JobList{}, err
	}
	if req.Limit <= 0 {
		req.Limit = DefaultJobRows
	}
	if req.Limit > MaxJobRows {
		req.Limit = MaxJobRows
	}
	kind := strings.TrimSpace(req.Kind)
	state := strings.TrimSpace(strings.ToLower(req.State))

	s.mu.Lock()
	defer s.mu.Unlock()

	where := "1"
	args := []string{}
	if kind != "" {
		where += " AND kind = ?"
		args = append(args, kind)
	}
	// The stored status is filtered here; `interrupted` is derived, so it is
	// matched after the read rather than pretending SQL can see it.
	if state != "" && state != JobInterrupted {
		where += " AND status = ?"
		args = append(args, state)
	}
	// Ordered by rowid, which is insertion order — **not** by `created_at`.
	// SQLite's CURRENT_TIMESTAMP has one-second resolution, so several jobs
	// started in quick succession carry the same timestamp, and a tiebreak on
	// the random job ID would have listed them in an arbitrary order that looked
	// like chronology. Insertion order is the real answer and costs nothing.
	//
	// One row past the limit, so `truncated` distinguishes "there is more" from
	// "that was exactly all".
	stmt, err := s.prepareLocked(`SELECT ` + jobColumns + ` FROM jobs WHERE ` + where +
		` ORDER BY rowid DESC LIMIT ?`)
	if err != nil {
		return JobList{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, append(args, strconv.Itoa(req.Limit+1))); err != nil {
		return JobList{}, err
	}

	result := JobList{Jobs: []Job{}}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return JobList{}, s.stepErrLocked(rc)
		}
		job := scanJob(stmt)
		if state == JobInterrupted && job.State != JobInterrupted {
			continue
		}
		if len(result.Jobs) >= req.Limit {
			result.Truncated = true
			break
		}
		result.Jobs = append(result.Jobs, job)
	}
	return result, nil
}

func (s *SQLiteStore) getJobLocked(jobID string) (Job, error) {
	stmt, err := s.prepareLocked(`SELECT ` + jobColumns + ` FROM jobs WHERE id = ?`)
	if err != nil {
		return Job{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{jobID}); err != nil {
		return Job{}, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return scanJob(stmt), nil
	case C.SQLITE_DONE:
		return Job{}, fmt.Errorf("%w: job %q", ErrNotFound, jobID)
	default:
		return Job{}, s.stepErrLocked(rc)
	}
}

// scanJob reads one row and derives the state a caller should see.
func scanJob(stmt *C.sqlite3_stmt) Job {
	job := Job{
		ID:              columnText(stmt, 0),
		Kind:            columnText(stmt, 1),
		State:           columnText(stmt, 2),
		CollectionID:    columnText(stmt, 3),
		Phase:           columnText(stmt, 5),
		Processed:       columnInt64(stmt, 6),
		Total:           columnInt64(stmt, 7),
		Error:           columnText(stmt, 9),
		CancelRequested: columnInt64(stmt, 10) != 0,
		CreatedAt:       parseSQLiteTime(columnText(stmt, 11)),
		StartedAt:       parseSQLiteTime(columnText(stmt, 12)),
		FinishedAt:      parseSQLiteTime(columnText(stmt, 13)),
		HeartbeatAt:     parseSQLiteTime(columnText(stmt, 14)),
	}
	_ = json.Unmarshal([]byte(columnText(stmt, 4)), &job.Parameters)
	summary := map[string]any{}
	if err := json.Unmarshal([]byte(columnText(stmt, 8)), &summary); err == nil && len(summary) > 0 {
		job.Summary = summary
	}
	job.State = derivedJobState(job, time.Now())
	return job
}

// derivedJobState turns silence into an answer.
//
// A process that dies cannot write its own epitaph, so a job left `running`
// with a stale heartbeat is reported as interrupted. This is computed on read
// rather than written by a sweeper for a concrete reason: two Notrios processes
// against one database — `notriosd` serving the GUI while `notriosctl` runs an
// import — would otherwise take turns declaring each other's work dead.
//
// **The record says the run stopped; it never says nothing happened.** Work
// completed before the interruption is durable and checkpointed, and running
// the same import again resumes from there.
func derivedJobState(job Job, now time.Time) string {
	if job.State != JobRunning {
		return job.State
	}
	beat := job.HeartbeatAt
	if beat.IsZero() {
		beat = job.StartedAt
	}
	if beat.IsZero() || now.Sub(beat) <= JobHeartbeatTimeout {
		return JobRunning
	}
	return JobInterrupted
}
