package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSchemaV11ProjectionRetryBackoffAndQueueStatus(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema status: %+v err=%v", status, err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_retry", Title: "retry"}); err != nil {
		t.Fatal(err)
	}
	jobs, err := st.PendingProjectionJobs(ctx, 10)
	if err != nil || len(jobs) != 1 || jobs[0].AttemptCount != 0 {
		t.Fatalf("initial jobs: %+v err=%v", jobs, err)
	}
	if err := st.CompleteProjectionJob(ctx, jobs[0].Sequence, errors.New("temporary projection failure")); err != nil {
		t.Fatal(err)
	}
	jobs, err = st.PendingProjectionJobs(ctx, 10)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("backoff must hide immediate retry: %+v err=%v", jobs, err)
	}
	queue, err := st.ProjectionQueueStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if queue.Pending != 1 || queue.Due != 0 || queue.Failed != 1 ||
		queue.NextAttemptAt.IsZero() || !queue.NextAttemptAt.After(time.Now().Add(-time.Second)) {
		t.Fatalf("unexpected delayed queue status: %+v", queue)
	}
	if err := st.Exec(ctx, `UPDATE index_outbox SET next_attempt_at = datetime('now', '-1 second') WHERE completed_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	jobs, err = st.PendingProjectionJobs(ctx, 10)
	if err != nil || len(jobs) != 1 || jobs[0].AttemptCount != 1 {
		t.Fatalf("due retry: %+v err=%v", jobs, err)
	}
	if err := st.CompleteProjectionJob(ctx, jobs[0].Sequence, nil); err != nil {
		t.Fatal(err)
	}
	queue, err = st.ProjectionQueueStatus(ctx)
	if err != nil || queue.Pending != 0 || queue.Due != 0 || queue.Failed != 0 {
		t.Fatalf("completed queue: %+v err=%v", queue, err)
	}
}

func TestSchemaV10UpgradeAddsProjectionRetrySchedule(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DROP INDEX index_outbox_pending_idx;`,
		`ALTER TABLE index_outbox DROP COLUMN next_attempt_at;`,
		`PRAGMA user_version = 10;`,
	} {
		if err := st.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("v10 to v11 bootstrap: %v", err)
	}
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("upgraded status: %+v err=%v", status, err)
	}
	if _, err := st.ProjectionQueueStatus(ctx); err != nil {
		t.Fatalf("new queue column unavailable: %v", err)
	}
}
