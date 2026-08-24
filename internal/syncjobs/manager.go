// Package syncjobs drains G15's closed durable sync outbox. It is intentionally
// not a scheduler: callers explicitly enqueue one of four known operations;
// the worker only resumes due rows and never creates work on a cadence.
package syncjobs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synccarrier"
)

var targetIDPattern = regexp.MustCompile(`^target_[a-f0-9]{32,64}$`)

var ErrCancelled = errors.New("sync job cancelled")

// Progress is called only after a target has crossed a durable boundary.
type Progress func(phase string, processed, total, bytesUsed int64, checkpoint map[string]any) error

// Target is one configured transport destination. Implementations retain
// paths, URLs, and credentials in memory; the manager knows only its opaque ID.
type Target interface {
	Plan(ctx context.Context, kind string, byteBudget int64) (map[string]any, error)
	Run(ctx context.Context, kind string, byteBudget int64, progress Progress) (map[string]any, error)
}

type discoverTarget interface {
	Discover(ctx context.Context) (map[string]any, error)
}

type RetryableError struct {
	Code string
	Err  error
}

func (e *RetryableError) Error() string {
	if e.Err == nil {
		return "retryable sync error: " + e.Code
	}
	return e.Err.Error()
}
func (e *RetryableError) Unwrap() error { return e.Err }

type Manager struct {
	store store.Store

	mu      sync.RWMutex
	targets map[string]Target
	wake    chan struct{}
	now     func() time.Time
}

func New(st store.Store) *Manager {
	return &Manager{store: st, targets: map[string]Target{}, wake: make(chan struct{}, 1), now: time.Now}
}

func (m *Manager) Register(targetID string, target Target) error {
	if target == nil {
		return fmt.Errorf("%w: sync target is required", store.ErrInvalidInput)
	}
	if !targetIDPattern.MatchString(targetID) || len(targetID) > store.MaxSyncJobTargetBytes {
		return fmt.Errorf("%w: invalid opaque target ID", store.ErrInvalidInput)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targets[targetID] = target
	return nil
}

func (m *Manager) HasTarget(targetID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.targets[targetID]
	return ok
}

func (m *Manager) Plan(ctx context.Context, targetID, kind string, byteBudget int64) (map[string]any, error) {
	target, err := m.target(targetID)
	if err != nil {
		return nil, err
	}
	if kind != store.JobKindSyncIncremental && kind != store.JobKindSyncResourceFetch {
		return nil, fmt.Errorf("%w: this surface only plans ordinary incremental or resource-fetch sync", store.ErrInvalidInput)
	}
	if byteBudget <= 0 || byteBudget > store.MaxSyncJobByteBudget {
		return nil, fmt.Errorf("%w: invalid sync byte budget", store.ErrInvalidInput)
	}
	plan, err := target.Plan(ctx, kind, byteBudget)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		plan = map[string]any{}
	}
	plan["target_id"] = targetID
	plan["kind"] = kind
	plan["byte_budget"] = byteBudget
	return plan, nil
}

// Discover performs the configured carrier's read-only discovery pass. It is
// intentionally outside the durable outbox: discovery publishes, enrolls, and
// admits nothing, so repeating it after a crash has no state to recover.
func (m *Manager) Discover(ctx context.Context, targetID string) (map[string]any, error) {
	target, err := m.target(targetID)
	if err != nil {
		return nil, err
	}
	discoverer, ok := target.(discoverTarget)
	if !ok {
		return nil, fmt.Errorf("%w: configured target does not support discovery", store.ErrInvalidInput)
	}
	return discoverer.Discover(ctx)
}

func (m *Manager) Start(ctx context.Context, req store.CreateSyncJobRequest) (store.SyncJob, error) {
	if !m.HasTarget(req.TargetID) {
		return store.SyncJob{}, fmt.Errorf("%w: sync target is not configured", store.ErrNotFound)
	}
	job, err := m.store.CreateSyncJob(ctx, req)
	if err == nil {
		m.signal()
	}
	return job, err
}

// RunNext executes at most one due job. ErrNotFound means the outbox currently
// has no claimable work and is an ordinary idle result.
func (m *Manager) RunNext(ctx context.Context, workerID string) (store.SyncJob, error) {
	job, err := m.store.ClaimSyncJob(ctx, workerID, m.now())
	if err != nil {
		return store.SyncJob{}, err
	}
	target, targetErr := m.target(job.TargetID)
	if targetErr != nil {
		settled, finishErr := m.store.FinishSyncJob(context.Background(), job.Job.ID, workerID, store.JobFailed,
			map[string]any{"reason": "target_unavailable"}, targetErr)
		if finishErr != nil {
			return store.SyncJob{}, finishErr
		}
		return settled, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	pollDone := make(chan struct{})
	go m.pollCancellation(runCtx, cancel, job.Job.ID, pollDone)

	progress := func(phase string, processed, total, bytesUsed int64, checkpoint map[string]any) error {
		updated, progressErr := m.store.CheckpointSyncJob(context.Background(), job.Job.ID, workerID, store.SyncJobCheckpoint{
			Phase: phase, Processed: processed, Total: total, BytesUsed: bytesUsed, Checkpoint: checkpoint,
		})
		if progressErr != nil {
			return progressErr
		}
		if updated.Job.CancelRequested {
			cancel()
			return ErrCancelled
		}
		return nil
	}

	summary, runErr := target.Run(runCtx, job.Job.Kind, job.ByteBudget, progress)
	cancel()
	<-pollDone

	latest, getErr := m.store.GetSyncJob(context.Background(), job.Job.ID)
	if getErr != nil {
		return store.SyncJob{}, getErr
	}
	if latest.Job.CancelRequested || errors.Is(runErr, ErrCancelled) {
		return m.store.FinishSyncJob(context.Background(), job.Job.ID, workerID, store.JobCancelled, summary, nil)
	}
	if runErr == nil {
		return m.store.FinishSyncJob(context.Background(), job.Job.ID, workerID, store.JobSucceeded, summary, nil)
	}
	if code := retryCode(runErr); code != "" {
		return m.store.RescheduleSyncJob(context.Background(), job.Job.ID, workerID, code, m.now())
	}
	return m.store.FinishSyncJob(context.Background(), job.Job.ID, workerID, store.JobFailed,
		map[string]any{"reason": "non_retryable"}, runErr)
}

// Serve drains explicitly queued work and wakes for due retries. The ticker is
// not a sync cadence and never enqueues a job; it only recovers durable rows
// after restart or a backoff deadline.
func (m *Manager) Serve(ctx context.Context, workerID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		for {
			_, err := m.RunNext(ctx, workerID)
			if errors.Is(err, store.ErrNotFound) {
				break
			}
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				// A database/control-plane fault must not become a tight retry
				// loop. The next wake/tick gets a fresh attempt.
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-ticker.C:
		}
	}
}

func (m *Manager) pollCancellation(ctx context.Context, cancel context.CancelFunc, jobID string, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(store.JobHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			requested, err := m.store.TouchJob(context.Background(), jobID)
			if err == nil && requested {
				cancel()
				return
			}
		}
	}
}

func (m *Manager) target(targetID string) (Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	target := m.targets[targetID]
	if target == nil {
		return nil, fmt.Errorf("%w: sync target is not configured", store.ErrNotFound)
	}
	return target, nil
}

func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// StartWake asks a running worker to recheck the durable queue after an
// operator retry. It creates no work and is harmless when no worker is active.
func (m *Manager) StartWake() { m.signal() }

func retryCode(err error) string {
	var retryable *RetryableError
	if errors.As(err, &retryable) {
		switch retryable.Code {
		case "offline", "quota", "temporary", "byte_budget":
			return retryable.Code
		}
	}
	if errors.Is(err, synccarrier.ErrByteBudget) {
		return "byte_budget"
	}
	if errors.Is(err, synccarrier.ErrCarrierUnavailable) {
		return "offline"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "temporary"
	}
	return ""
}
