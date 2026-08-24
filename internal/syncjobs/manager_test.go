package syncjobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

type fakeTarget struct {
	plan map[string]any
	run  func(context.Context, string, int64, Progress) (map[string]any, error)
}

func (f fakeTarget) Plan(_ context.Context, _ string, _ int64) (map[string]any, error) {
	return f.plan, nil
}
func (f fakeTarget) Run(ctx context.Context, kind string, budget int64, progress Progress) (map[string]any, error) {
	return f.run(ctx, kind, budget, progress)
}

func newManagerTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestG15ManagerPlansRunsAndPersistsDurableProgress(t *testing.T) {
	st := newManagerTestStore(t)
	m := New(st)
	targetID := store.SyncTargetID("fake:success")
	if err := m.Register(targetID, fakeTarget{
		plan: map[string]any{"wanted_resources": 2},
		run: func(_ context.Context, kind string, budget int64, progress Progress) (map[string]any, error) {
			if kind != store.JobKindSyncIncremental || budget != 1000 {
				t.Fatalf("kind=%q budget=%d", kind, budget)
			}
			if err := progress("pull", 2, 4, 400, map[string]any{"last_phase": "pull"}); err != nil {
				return nil, err
			}
			if err := progress("push", 4, 4, 700, map[string]any{"last_phase": "push"}); err != nil {
				return nil, err
			}
			return map[string]any{"operations": 4}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := m.Plan(context.Background(), targetID, store.JobKindSyncIncremental, 1000)
	if err != nil || plan["target_id"] != targetID || plan["wanted_resources"] != 2 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	queued, err := m.Start(context.Background(), store.CreateSyncJobRequest{
		Kind: store.JobKindSyncIncremental, Actor: store.SyncJobActorREST, TargetID: targetID, ByteBudget: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	finished, err := m.RunNext(context.Background(), "test-worker")
	if err != nil || finished.Job.ID != queued.Job.ID || finished.Job.State != store.JobSucceeded || finished.BytesUsed != 700 {
		t.Fatalf("finished=%+v err=%v", finished, err)
	}
	if finished.Checkpoint["last_phase"] != "push" || finished.Job.Summary["operations"] != float64(4) {
		t.Fatalf("durable result=%+v", finished)
	}
}

func TestG15ManagerClassifiesOfflineQuotaAndBudgetForBackoff(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"offline", &RetryableError{Code: "offline", Err: errors.New("private provider detail")}, "offline"},
		{"quota", &RetryableError{Code: "quota", Err: errors.New("account name must not persist")}, "quota"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newManagerTestStore(t)
			m := New(st)
			now := time.Now().UTC().Truncate(time.Second)
			m.now = func() time.Time { return now }
			targetID := store.SyncTargetID("fake:" + tc.name)
			_ = m.Register(targetID, fakeTarget{run: func(context.Context, string, int64, Progress) (map[string]any, error) {
				return nil, tc.err
			}})
			queued, _ := m.Start(context.Background(), store.CreateSyncJobRequest{
				Kind: store.JobKindSyncIncremental, Actor: store.SyncJobActorService, TargetID: targetID, ByteBudget: 100,
			})
			retrying, err := m.RunNext(context.Background(), "worker")
			if err != nil || retrying.Job.State != store.JobQueued || retrying.RetryCode != tc.code || retrying.NextAttemptAt.IsZero() {
				t.Fatalf("retrying=%+v err=%v", retrying, err)
			}
			if retrying.Job.Error == tc.err.Error() || queued.Job.ID != retrying.Job.ID {
				t.Fatalf("provider detail leaked or job changed: %+v", retrying)
			}
		})
	}
}

func TestG15ManagerCancellationStopsAtCheckpoint(t *testing.T) {
	st := newManagerTestStore(t)
	m := New(st)
	targetID := store.SyncTargetID("fake:cancel")
	boundary := make(chan struct{})
	resume := make(chan struct{})
	_ = m.Register(targetID, fakeTarget{run: func(_ context.Context, _ string, _ int64, progress Progress) (map[string]any, error) {
		if err := progress("pull", 1, 2, 10, map[string]any{"last_phase": "pull"}); err != nil {
			return nil, err
		}
		close(boundary)
		<-resume
		if err := progress("push", 2, 2, 20, map[string]any{"last_phase": "push"}); err != nil {
			return map[string]any{"processed": 1}, err
		}
		return nil, errors.New("expected cancellation")
	}})
	queued, _ := m.Start(context.Background(), store.CreateSyncJobRequest{
		Kind: store.JobKindSyncIncremental, Actor: store.SyncJobActorMCP, TargetID: targetID, ByteBudget: 100,
	})
	done := make(chan struct {
		job store.SyncJob
		err error
	}, 1)
	go func() {
		job, err := m.RunNext(context.Background(), "worker")
		done <- struct {
			job store.SyncJob
			err error
		}{job, err}
	}()
	<-boundary
	if _, err := st.RequestJobCancel(context.Background(), queued.Job.ID); err != nil {
		t.Fatal(err)
	}
	close(resume)
	result := <-done
	if result.err != nil || result.job.Job.State != store.JobCancelled || result.job.Checkpoint["last_phase"] != "push" {
		t.Fatalf("cancelled=%+v err=%v", result.job, result.err)
	}
}
