package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func newRunnerStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return st
}

func startRunner(t *testing.T, st *store.SQLiteStore) (*Runner, context.Context) {
	t.Helper()
	runner, ctx, err := Start(context.Background(), st, store.CreateJobRequest{
		Kind:       store.JobKindImportObsidian,
		Parameters: []store.JobParameter{{Name: "_vault-dir", Value: "/tmp/vault", Path: true}},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return runner, ctx
}

// The ordinary path: progress flows, the record settles as succeeded.
func TestRunnerRecordsASuccessfulRun(t *testing.T) {
	st := newRunnerStore(t)
	runner, _ := startRunner(t, st)

	if err := runner.Progress("notes", 25, 100); err != nil {
		t.Fatalf("Progress: %v", err)
	}
	job, err := runner.Finish(map[string]any{"notes_imported": 100}, nil)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if job.State != store.JobSucceeded || job.Summary["notes_imported"] != float64(100) {
		t.Fatalf("unexpected outcome: %+v", job)
	}
}

// A cancelled run is not a broken one. Recording it as `failed` would send an
// operator hunting for a fault that does not exist — and the two routes
// cancellation can arrive by are the same event, so both must classify alike.
func TestCancellationIsRecordedAsCancelledByEitherRoute(t *testing.T) {
	t.Run("from a batch boundary", func(t *testing.T) {
		st := newRunnerStore(t)
		runner, ctx := startRunner(t, st)
		if _, err := st.RequestJobCancel(context.Background(), runner.ID()); err != nil {
			t.Fatal(err)
		}

		err := runner.Progress("notes", 50, 100)
		if !errors.Is(err, ErrCancelled) {
			t.Fatalf("Progress after a cancel request = %v, want ErrCancelled", err)
		}
		// The operation's context is cancelled too, so work that is not between
		// batches stops at its next store read rather than running on.
		if ctx.Err() == nil {
			t.Fatal("the run context should be cancelled once the job is")
		}

		job, err := runner.Finish(map[string]any{"notes_imported": 50}, err)
		if err != nil {
			t.Fatal(err)
		}
		if job.State != store.JobCancelled {
			t.Fatalf("state = %q, want cancelled", job.State)
		}
		if job.Error != "" {
			t.Fatalf("a cancellation is not an error to report: %q", job.Error)
		}
		// The work finished before the stop is reported, which is the reason
		// cancellation waits for a checkpoint at all.
		if job.Summary["notes_imported"] != float64(50) {
			t.Fatalf("a cancelled run still reports what it completed: %+v", job.Summary)
		}
	})

	t.Run("from a cancelled context", func(t *testing.T) {
		st := newRunnerStore(t)
		runner, _ := startRunner(t, st)
		// What archive export surfaces: no batch boundary, so cancellation
		// arrives as a failed store read carrying context.Canceled.
		job, err := runner.Finish(nil, context.Canceled)
		if err != nil {
			t.Fatal(err)
		}
		if job.State != store.JobCancelled {
			t.Fatalf("state = %q, want cancelled", job.State)
		}
	})
}

// A real failure keeps its message, because that is what an operator needs.
func TestRunnerRecordsAFailure(t *testing.T) {
	st := newRunnerStore(t)
	runner, _ := startRunner(t, st)
	job, err := runner.Finish(nil, errors.New("vault directory is unreadable"))
	if err != nil {
		t.Fatal(err)
	}
	if job.State != store.JobFailed || job.Error != "vault directory is unreadable" {
		t.Fatalf("unexpected outcome: %+v", job)
	}
}

// Finishing twice must not reopen a settled record: the CLI settles a job in
// one place, and a deferred cleanup that also settled it would overwrite the
// real outcome with a later, emptier one.
func TestFinishIsIdempotent(t *testing.T) {
	st := newRunnerStore(t)
	runner, _ := startRunner(t, st)
	if _, err := runner.Finish(map[string]any{"notes_imported": 7}, nil); err != nil {
		t.Fatal(err)
	}
	again, err := runner.Finish(nil, errors.New("late failure"))
	if err != nil {
		t.Fatalf("a second Finish should be a no-op, got %v", err)
	}
	if again.State != store.JobSucceeded || again.Summary["notes_imported"] != float64(7) {
		t.Fatalf("the first outcome must stand: %+v", again)
	}
}
