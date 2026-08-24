package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchemaV26UpgradeAddsSyncOutboxToV25Database(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.sqlite")
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(context.Background(), `DROP TABLE sync_job_audit; DROP TABLE sync_jobs; PRAGMA user_version = 25;`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{
		Kind: JobKindSyncIncremental, Actor: SyncJobActorCLI,
		TargetID: SyncTargetID("directory:upgrade"), ByteBudget: 1024,
	}); err != nil {
		t.Fatalf("create after v25 upgrade: %v", err)
	}
}

func newSyncJobStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestG15SyncJobOutboxLimitsTargetsAndKeepsLocationsOut(t *testing.T) {
	st := newSyncJobStore(t)
	target := SyncTargetID("directory:/home/private/sync:credential-secret")
	created, err := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{
		Kind: JobKindSyncIncremental, Actor: SyncJobActorMCP, TargetID: target, ByteBudget: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Job.State != JobQueued || created.TargetID != target || created.MaxAttempts != DefaultSyncJobAttempts {
		t.Fatalf("created sync job = %+v", created)
	}
	if len(created.Job.Parameters) != 0 || strings.Contains(created.TargetID, "private") || strings.Contains(created.TargetID, "secret") {
		t.Fatalf("target location or secret escaped into job: %+v", created)
	}
	if _, err := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{
		Kind: JobKindSyncIncremental, Actor: SyncJobActorMCP, TargetID: "/home/private/sync", ByteBudget: 1,
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("path-shaped target error = %v", err)
	}
	status, err := st.Status(context.Background())
	if err != nil || status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema = %d, want %d (err %v)", status.SchemaVersion, CurrentSchemaVersion, err)
	}
}

func TestG15SyncJobClaimIsOnePerTargetAndTwoTargetsProgress(t *testing.T) {
	st := newSyncJobStore(t)
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	targetA := SyncTargetID("directory:a")
	targetB := SyncTargetID("rest:b")
	firstA, _ := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{Kind: JobKindSyncIncremental, Actor: SyncJobActorCLI, TargetID: targetA, ByteBudget: 100})
	_, _ = st.CreateSyncJob(context.Background(), CreateSyncJobRequest{Kind: JobKindSyncResourceFetch, Actor: SyncJobActorREST, TargetID: targetA, ByteBudget: 100})
	firstB, _ := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{Kind: JobKindSyncIncremental, Actor: SyncJobActorREST, TargetID: targetB, ByteBudget: 100})

	claimedA, err := st.ClaimSyncJob(context.Background(), "worker-a", now)
	if err != nil || claimedA.Job.ID != firstA.Job.ID || claimedA.Attempt != 1 {
		t.Fatalf("claim A = %+v, %v", claimedA, err)
	}
	claimedB, err := st.ClaimSyncJob(context.Background(), "worker-b", now)
	if err != nil || claimedB.Job.ID != firstB.Job.ID {
		t.Fatalf("claim B = %+v, %v", claimedB, err)
	}
	if _, err := st.ClaimSyncJob(context.Background(), "worker-c", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("third claim error = %v, want no due job", err)
	}
}

func TestG15SyncCheckpointRetryCancelAndReset(t *testing.T) {
	st := newSyncJobStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	created, err := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{
		Kind: JobKindSyncIncremental, Actor: SyncJobActorMCP, TargetID: SyncTargetID("rest:primary"),
		ByteBudget: 1000, MaxAttempts: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimSyncJob(context.Background(), "worker", now)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := map[string]any{"phase": "pull", "vector_count": 2}
	updated, err := st.CheckpointSyncJob(context.Background(), claimed.Job.ID, "worker", SyncJobCheckpoint{
		Phase: "pull", Processed: 2, Total: 4, BytesUsed: 400, Checkpoint: checkpoint,
	})
	if err != nil || updated.Job.Phase != "pull" || updated.BytesUsed != 400 {
		t.Fatalf("checkpoint = %+v, %v", updated, err)
	}
	if _, err := st.CheckpointSyncJob(context.Background(), claimed.Job.ID, "worker", SyncJobCheckpoint{BytesUsed: 1001}); !errors.Is(err, ErrConflict) {
		t.Fatalf("budget error = %v", err)
	}

	retrying, err := st.RescheduleSyncJob(context.Background(), claimed.Job.ID, "worker", "offline", now)
	if err != nil {
		t.Fatal(err)
	}
	wantDelay := SyncRetryDelay(created.Job.ID, 1)
	if retrying.Job.State != JobQueued || retrying.RetryCode != "offline" || !retrying.NextAttemptAt.Equal(now.Add(wantDelay).Truncate(time.Second)) {
		t.Fatalf("retry = %+v, want delay %s", retrying, wantDelay)
	}
	if _, err := st.ClaimSyncJob(context.Background(), "early", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("early retry claim error = %v", err)
	}
	reclaimed, err := st.ClaimSyncJob(context.Background(), "worker-2", retrying.NextAttemptAt)
	if err != nil || reclaimed.Attempt != 2 || reclaimed.Checkpoint["phase"] != "pull" {
		t.Fatalf("reclaim = %+v, %v", reclaimed, err)
	}
	if _, err := st.RequestJobCancel(context.Background(), reclaimed.Job.ID); err != nil {
		t.Fatal(err)
	}
	cancelledView, err := st.CheckpointSyncJob(context.Background(), reclaimed.Job.ID, "worker-2", SyncJobCheckpoint{
		Phase: "push", Processed: 3, Total: 4, BytesUsed: 500, Checkpoint: map[string]any{"phase": "push"},
	})
	if err != nil || !cancelledView.Job.CancelRequested {
		t.Fatalf("cancel checkpoint = %+v, %v", cancelledView, err)
	}
	if _, err := st.FinishSyncJob(context.Background(), reclaimed.Job.ID, "worker-2", JobCancelled, map[string]any{"processed": 3}, nil); err != nil {
		t.Fatal(err)
	}
	reset, err := st.RetrySyncJob(context.Background(), reclaimed.Job.ID, true, now.Add(time.Hour))
	if err != nil || reset.Attempt != 0 || len(reset.Checkpoint) != 0 || reset.Job.CancelRequested {
		t.Fatalf("reset = %+v, %v", reset, err)
	}
	audit, err := st.ListSyncJobAudit(context.Background(), reclaimed.Job.ID, 20)
	if err != nil || len(audit.Events) < 6 {
		t.Fatalf("audit = %+v, %v", audit, err)
	}
}

func TestG15StaleHeartbeatRequeuesCheckpointWithoutDoubleClaim(t *testing.T) {
	st := newSyncJobStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	created, _ := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{
		Kind: JobKindSyncIncremental, Actor: SyncJobActorService, TargetID: SyncTargetID("directory:stale"), ByteBudget: 100,
	})
	claimed, err := st.ClaimSyncJob(context.Background(), "dead-worker", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CheckpointSyncJob(context.Background(), claimed.Job.ID, "dead-worker", SyncJobCheckpoint{
		Phase: "pull", BytesUsed: 10, Checkpoint: map[string]any{"durable": true},
	}); err != nil {
		t.Fatal(err)
	}
	// Checkpoint uses CURRENT_TIMESTAMP, so make the crash instant explicit.
	st.mu.Lock()
	err = st.execPreparedLocked(`UPDATE jobs SET heartbeat_at = ? WHERE id = ?`, formatSQLiteTime(now.Add(-3*time.Minute)), created.Job.ID)
	st.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimSyncJob(context.Background(), "new-worker", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale job should respect backoff, got %v", err)
	}
	requeued, err := st.GetSyncJob(context.Background(), created.Job.ID)
	if err != nil || requeued.Job.State != JobQueued || requeued.Checkpoint["durable"] != true || requeued.RetryCode != "temporary" {
		t.Fatalf("requeued = %+v, %v", requeued, err)
	}
	reclaimed, err := st.ClaimSyncJob(context.Background(), "new-worker", requeued.NextAttemptAt)
	if err != nil || reclaimed.Attempt != 2 || reclaimed.Checkpoint["durable"] != true {
		t.Fatalf("reclaimed = %+v, %v", reclaimed, err)
	}
}

func TestG15StaleFinalAttemptSettlesInsteadOfExceedingRetryLimit(t *testing.T) {
	st := newSyncJobStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	created, err := st.CreateSyncJob(context.Background(), CreateSyncJobRequest{
		Kind: JobKindSyncIncremental, Actor: SyncJobActorService,
		TargetID: SyncTargetID("rest:final-attempt"), ByteBudget: 100, MaxAttempts: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimSyncJob(context.Background(), "dead-worker", now); err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	err = st.execPreparedLocked(`UPDATE jobs SET heartbeat_at = ? WHERE id = ?`, formatSQLiteTime(now.Add(-3*time.Minute)), created.Job.ID)
	st.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimSyncJob(context.Background(), "new-worker", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim after exhausted crash = %v", err)
	}
	finished, err := st.GetSyncJob(context.Background(), created.Job.ID)
	if err != nil || finished.Job.State != JobFailed || finished.Attempt != 1 {
		t.Fatalf("finished = %+v, %v", finished, err)
	}
}

func TestSyncRetryDelayIsDeterministicBoundedAndJittered(t *testing.T) {
	seen := map[time.Duration]bool{}
	for attempt := 1; attempt <= 40; attempt++ {
		first := SyncRetryDelay("job_one", attempt)
		if first != SyncRetryDelay("job_one", attempt) {
			t.Fatal("retry jitter changed for the same job and attempt")
		}
		if first <= 0 || first > 15*time.Minute {
			t.Fatalf("attempt %d delay = %s", attempt, first)
		}
		seen[first] = true
	}
	if len(seen) < 5 || SyncRetryDelay("job_one", 1) == SyncRetryDelay("job_two", 1) {
		t.Fatalf("jitter did not vary: %+v", seen)
	}
}
