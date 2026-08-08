// Package jobs ties a durable job record to one long-running operation.
//
// Two cancellation mechanisms exist because the operations differ, and picking
// one for both would have been worse than admitting that:
//
//   - **A durable batch boundary.** The importers already call `AfterBatch`
//     *after* committing and checkpointing a batch, and abort when it returns an
//     error. Stopping there leaves a state the next run resumes from, so that
//     hook is where cancellation belongs — it was the right seam before this
//     package existed.
//   - **Context cancellation.** Archive export has no such boundary; it does
//     have a `context.Context` threaded through every store read. Cancelling it
//     aborts at the next read, and because the manifest is written last and is
//     the completion marker, a stopped export leaves no archive that could pass
//     as complete.
//
// Both are driven from the same place, so a caller does not choose.
package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// ErrCancelled is returned from Progress when someone asked the job to stop.
// It is deliberately distinguishable from a failure: a cancelled run is not a
// broken one, and reporting it as `failed` would make an operator hunt for a
// fault that does not exist.
var ErrCancelled = errors.New("job cancelled")

// Runner owns one job record for the life of one operation.
type Runner struct {
	store store.Store
	id    string

	mu        sync.Mutex
	cancelCtx context.CancelFunc
	stopPoll  chan struct{}
	pollDone  chan struct{}
	settled   bool
}

// Start records a job, marks it running, and returns a context that is
// cancelled when someone asks the job to stop.
//
// The returned context is the one to hand to the operation. Even an operation
// with no progress hook becomes cancellable through it, and a job that never
// reports progress still gets a heartbeat, so silence means "the process is
// gone" rather than "this one does not report".
func Start(ctx context.Context, st store.Store, req store.CreateJobRequest) (*Runner, context.Context, error) {
	job, err := st.CreateJob(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	if _, err := st.StartJob(ctx, job.ID); err != nil {
		return nil, nil, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	runner := &Runner{
		store:     st,
		id:        job.ID,
		cancelCtx: cancel,
		stopPoll:  make(chan struct{}),
		pollDone:  make(chan struct{}),
	}
	go runner.poll(runCtx)
	return runner, runCtx, nil
}

// ID is the record this runner owns.
func (r *Runner) ID() string { return r.id }

// poll keeps the heartbeat fresh and notices a cancel request for operations
// that never call Progress.
func (r *Runner) poll(ctx context.Context) {
	defer close(r.pollDone)
	ticker := time.NewTicker(store.JobHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopPoll:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// A background heartbeat is a plain write, so it uses a fresh
			// context: once the run context is cancelled this would otherwise
			// fail, exactly when the record most needs updating.
			cancelled, err := r.store.ReportJobProgress(context.Background(), r.id, store.JobProgress{})
			if err == nil && cancelled {
				r.requestStop()
				return
			}
		}
	}
}

// requestStop cancels the operation's context.
func (r *Runner) requestStop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancelCtx != nil {
		r.cancelCtx()
	}
}

// Progress reports a point the operation has reached and returns ErrCancelled
// when it should stop.
//
// Call it only where stopping is safe. For the importers that is `AfterBatch`,
// which runs after the batch is committed and the checkpoint saved.
func (r *Runner) Progress(phase string, processed, total int) error {
	cancelled, err := r.store.ReportJobProgress(context.Background(), r.id, store.JobProgress{
		Phase: phase, Processed: int64(processed), Total: int64(total),
	})
	if err != nil {
		return err
	}
	if cancelled {
		r.requestStop()
		return ErrCancelled
	}
	return nil
}

// Cancel asks this run to stop, as a local Ctrl-C does.
func (r *Runner) Cancel(ctx context.Context) error {
	_, err := r.store.RequestJobCancel(ctx, r.id)
	return err
}

// Finish settles the record from the operation's outcome.
//
// It classifies rather than asking the caller to: a run that stopped because it
// was cancelled is `cancelled`, whether the cancellation surfaced as
// ErrCancelled from a batch boundary or as a cancelled context from a store
// read. Those are the same event reaching the caller by two routes, and
// recording one of them as a failure would be a lie about what happened.
func (r *Runner) Finish(summary map[string]any, failure error) (store.Job, error) {
	r.mu.Lock()
	if r.settled {
		r.mu.Unlock()
		return r.store.GetJob(context.Background(), r.id)
	}
	r.settled = true
	r.mu.Unlock()

	close(r.stopPoll)
	<-r.pollDone
	r.requestStop()

	state := store.JobSucceeded
	switch {
	case failure == nil:
	case errors.Is(failure, ErrCancelled), errors.Is(failure, context.Canceled):
		state = store.JobCancelled
		failure = nil
	default:
		state = store.JobFailed
	}
	// A cancelled run reports the work it did finish, because it stopped at a
	// checkpoint rather than losing it.
	return r.store.FinishJob(context.Background(), r.id, state, summary, failure)
}
