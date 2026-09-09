package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newJobTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return st
}

func newJob(t *testing.T, st *SQLiteStore) Job {
	t.Helper()
	job, err := st.CreateJob(context.Background(), CreateJobRequest{
		Kind: JobKindImportObsidian,
		Parameters: []JobParameter{
			{Name: "vault", Value: "/home/someone/Vault", Path: true},
			{Name: "collection", Value: "default"},
		},
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	return job
}

// The ordinary life of a record: queued, running with progress, settled.
func TestJobLifecycle(t *testing.T) {
	ctx := context.Background()
	st := newJobTestStore(t)

	job := newJob(t, st)
	if job.State != JobQueued || job.Settled() {
		t.Fatalf("a new job has not started: %+v", job)
	}

	started, err := st.StartJob(ctx, job.ID)
	if err != nil || started.State != JobRunning {
		t.Fatalf("StartJob: %+v err=%v", started, err)
	}
	if started.StartedAt.IsZero() || started.HeartbeatAt.IsZero() {
		t.Fatalf("a running job reports when it started and when it was last heard from: %+v", started)
	}

	cancelled, err := st.ReportJobProgress(ctx, job.ID, JobProgress{Phase: "notes", Processed: 40, Total: 100})
	if err != nil {
		t.Fatalf("ReportJobProgress: %v", err)
	}
	if cancelled {
		t.Fatal("nothing asked this job to stop")
	}
	mid, err := st.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if mid.Phase != "notes" || mid.Processed != 40 || mid.Total != 100 {
		t.Fatalf("progress was not recorded: %+v", mid)
	}

	done, err := st.FinishJob(ctx, job.ID, JobSucceeded, map[string]any{"notes_imported": 100}, nil)
	if err != nil {
		t.Fatalf("FinishJob: %v", err)
	}
	if done.State != JobSucceeded || !done.Settled() || done.FinishedAt.IsZero() {
		t.Fatalf("a finished job is settled and says when: %+v", done)
	}
	if done.Summary["notes_imported"] != float64(100) {
		t.Fatalf("the summary should survive the round trip: %+v", done.Summary)
	}
}

// Cancellation is cooperative, and the answer arrives at the boundary where
// stopping is safe: the progress report the importer already makes after a
// durable checkpoint.
func TestCancellationIsAnsweredAtTheProgressBoundary(t *testing.T) {
	ctx := context.Background()
	st := newJobTestStore(t)
	job := newJob(t, st)
	if _, err := st.StartJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := st.RequestJobCancel(ctx, job.ID); err != nil {
		t.Fatalf("RequestJobCancel: %v", err)
	}
	// Still running: asking is not stopping. The work decides when.
	pending, err := st.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.State != JobRunning || !pending.CancelRequested {
		t.Fatalf("a cancel request is a flag, not a state: %+v", pending)
	}

	cancelled, err := st.ReportJobProgress(ctx, job.ID, JobProgress{Phase: "notes", Processed: 50, Total: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("the next durable boundary must learn it was cancelled")
	}
	final, err := st.FinishJob(ctx, job.ID, JobCancelled, map[string]any{"notes_imported": 50}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if final.State != JobCancelled {
		t.Fatalf("state = %q, want cancelled", final.State)
	}
	// The work done before the stop is reported, not discarded. That is the
	// whole reason cancellation waits for a checkpoint.
	if final.Summary["notes_imported"] != float64(50) {
		t.Fatalf("a cancelled job still reports what it finished: %+v", final.Summary)
	}
}

// Cancelling a settled job is already true, so it is not an error. A
// retry-on-timeout script must not report a problem it does not have.
func TestCancellingASettledJobIsNotAnError(t *testing.T) {
	ctx := context.Background()
	st := newJobTestStore(t)
	job := newJob(t, st)
	if _, err := st.StartJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.FinishJob(ctx, job.ID, JobSucceeded, nil, nil); err != nil {
		t.Fatal(err)
	}
	after, err := st.RequestJobCancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("cancelling a finished job should be a no-op, got %v", err)
	}
	if after.State != JobSucceeded || after.CancelRequested {
		t.Fatalf("a settled job must not be reopened by a cancel: %+v", after)
	}
}

// A process that dies cannot write its own epitaph. Silence past the timeout is
// reported as `interrupted` — derived on read, never stored.
func TestASilentRunningJobIsReportedAsInterrupted(t *testing.T) {
	ctx := context.Background()
	st := newJobTestStore(t)
	job := newJob(t, st)
	if _, err := st.StartJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}

	live, err := st.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live.State != JobRunning {
		t.Fatalf("a job heard from a moment ago is running: %+v", live)
	}

	// Backdate the heartbeat rather than sleeping for the timeout.
	stale := time.Now().UTC().Add(-2 * JobHeartbeatTimeout).Format("2006-01-02 15:04:05")
	if err := st.Exec(ctx, `UPDATE jobs SET heartbeat_at = '`+stale+`'`); err != nil {
		t.Fatal(err)
	}
	gone, err := st.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gone.State != JobInterrupted {
		t.Fatalf("state = %q, want interrupted", gone.State)
	}
	if !gone.Settled() {
		t.Fatal("an interrupted job will not change again on its own, so a --wait must stop waiting")
	}
	// Derived, not written: the stored status is untouched, so a process that
	// was merely slow can carry on and finish normally.
	recovered, err := st.FinishJob(ctx, job.ID, JobSucceeded, nil, nil)
	if err != nil {
		t.Fatalf("a slow job that comes back must still be able to finish: %v", err)
	}
	if recovered.State != JobSucceeded {
		t.Fatalf("state = %q, want succeeded", recovered.State)
	}
}

// `interrupted` can never be written, because a process able to write it is a
// process that was not interrupted.
func TestInterruptedCannotBeFinishedInto(t *testing.T) {
	ctx := context.Background()
	st := newJobTestStore(t)
	job := newJob(t, st)
	if _, err := st.FinishJob(ctx, job.ID, JobInterrupted, nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("FinishJob(interrupted) = %v, want ErrInvalidInput", err)
	}
	if _, err := st.FinishJob(ctx, job.ID, "nonsense", nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("FinishJob(nonsense) = %v, want ErrInvalidInput", err)
	}
}

// A kind names a specific operation with specific parameters. An open set would
// make `jobs show --command` a guess.
func TestJobKindsAreClosed(t *testing.T) {
	_, err := newJobTestStore(t).CreateJob(context.Background(), CreateJobRequest{Kind: "rm_minus_rf"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateJob with an unknown kind = %v, want ErrInvalidInput", err)
	}
}

// Listing is newest first and bounded, and `truncated` distinguishes "there is
// more" from "that was exactly all".
func TestJobListingIsNewestFirstAndBounded(t *testing.T) {
	ctx := context.Background()
	st := newJobTestStore(t)
	ids := []string{}
	for i := 0; i < 4; i++ {
		ids = append(ids, newJob(t, st).ID)
	}
	if _, err := st.StartJob(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}

	all, err := st.ListJobs(ctx, JobListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Jobs) != 4 || all.Truncated {
		t.Fatalf("expected four complete rows: %+v", all)
	}
	if all.Jobs[0].ID != ids[3] {
		t.Fatalf("newest first: got %q, want %q", all.Jobs[0].ID, ids[3])
	}

	capped, err := st.ListJobs(ctx, JobListRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(capped.Jobs) != 2 || !capped.Truncated {
		t.Fatalf("a capped listing says so: %+v", capped)
	}
	exact, err := st.ListJobs(ctx, JobListRequest{Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	if exact.Truncated {
		t.Fatal("a listing that showed everything must not claim truncation")
	}

	running, err := st.ListJobs(ctx, JobListRequest{State: JobRunning})
	if err != nil {
		t.Fatal(err)
	}
	if len(running.Jobs) != 1 || running.Jobs[0].ID != ids[0] {
		t.Fatalf("state filter: %+v", running)
	}
}

// The parameters are kept so a run can be reproduced, and each one says whether
// its value names a place on this machine. Raw argv would have made that
// impossible to separate after the fact.
func TestJobParametersRecordWhichValuesArePaths(t *testing.T) {
	job, err := newJobTestStore(t).GetJob(context.Background(), "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown job is not found, got %+v err=%v", job, err)
	}

	st := newJobTestStore(t)
	stored, err := st.GetJob(context.Background(), newJob(t, st).ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Parameters) != 2 {
		t.Fatalf("parameters should survive: %+v", stored.Parameters)
	}
	if !stored.Parameters[0].Path || stored.Parameters[0].Value != "/home/someone/Vault" {
		t.Fatalf("the vault directory is a path: %+v", stored.Parameters[0])
	}
	if stored.Parameters[1].Path {
		t.Fatalf("a collection ID is not a path: %+v", stored.Parameters[1])
	}
}

// A database created before v18 has no jobs table. The upgrade adds it on the
// next open, without touching a row.
func TestSchemaV18UpgradeFromV17(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "v17.sqlite")
	st, err := OpenSQLiteWithAssetStore(dbPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "v18_doc", Title: "Kitchen Plan", Body: "Body.\n",
	}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`DROP TABLE IF EXISTS jobs`, `PRAGMA user_version = 17`} {
		if err := st.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLiteWithAssetStore(dbPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	status, err := reopened.Status(ctx)
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d (err %v)", status.SchemaVersion, CurrentSchemaVersion, err)
	}
	if _, err := reopened.CreateJob(ctx, CreateJobRequest{Kind: JobKindExportArchiveV2}); err != nil {
		t.Fatalf("the upgraded database should accept a job: %v", err)
	}
	doc, err := reopened.GetDocument(ctx, "v18_doc")
	if err != nil || doc.Title != "Kitchen Plan" {
		t.Fatalf("the upgrade must not touch existing notes: %+v err=%v", doc, err)
	}
}
