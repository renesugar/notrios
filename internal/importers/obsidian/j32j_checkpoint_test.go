package obsidian

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func j32jVault(t *testing.T, notes int) string {
	t.Helper()
	vault := t.TempDir()
	for index := 0; index < notes; index++ {
		name := filepath.Join(vault, fmt.Sprintf("Note-%03d.md", index))
		body := fmt.Sprintf("# Note %d\n\nBody with [[Note-%03d]] ελληνικά.\n", index, (index+1)%notes)
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return vault
}

func j32jStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	root := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

// TestImportCommitsOncePerBatch states what J32-J removed: the notes and
// link-rebuild phases pass their checkpoint to the store with the batch, so it
// commits atomically with the rows it describes, and the walker no longer writes
// the same checkpoint again in a transaction of its own.
func TestImportCommitsOncePerBatch(t *testing.T) {
	ctx := context.Background()
	vault := j32jVault(t, 20)
	st := j32jStore(t)

	before := store.Commits()
	report, err := Import(ctx, st, vault, Options{CollectionID: "default", BatchSize: 5})
	if err != nil {
		t.Fatal(err)
	}
	commits := store.Commits() - before

	// Four notes batches and four link-rebuild batches, plus the phases that
	// still checkpoint through the walker and the import's own bookkeeping. What
	// matters is the shape: fewer commits than the old two-per-batch, and at
	// least one per batch.
	if report.BatchesCompleted == 0 {
		t.Fatal("no batches were reported")
	}
	if commits < int64(report.BatchesCompleted) {
		t.Errorf("%d commits for %d batches: a batch must still be durable",
			commits, report.BatchesCompleted)
	}
	if commits >= int64(report.BatchesCompleted)*2 {
		t.Errorf("%d commits for %d batches: the duplicate checkpoint write is back",
			commits, report.BatchesCompleted)
	}
}

// TestResumeAfterProgressFailure injects the failure the plan named: the batch
// and its checkpoint have committed, and then publishing progress fails. The
// resumed import must produce what an uninterrupted one produces.
func TestResumeAfterProgressFailure(t *testing.T) {
	ctx := context.Background()
	vault := j32jVault(t, 20)

	direct := j32jStore(t)
	if _, err := Import(ctx, direct, vault, Options{CollectionID: "default", BatchSize: 5}); err != nil {
		t.Fatal(err)
	}
	want, err := direct.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}

	resumed := j32jStore(t)
	failed := 0
	_, err = Import(ctx, resumed, vault, Options{
		CollectionID: "default",
		BatchSize:    5,
		AfterBatch: func(phase string, processed, total int) error {
			// After the second notes batch has committed, fail where progress
			// would be published.
			if phase == "notes" {
				failed++
				if failed == 2 {
					return fmt.Errorf("progress publication failed")
				}
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("the import should have stopped")
	}
	if _, err := Import(ctx, resumed, vault, Options{CollectionID: "default", BatchSize: 5}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	got, err := resumed.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("a resume after a progress failure differs from an uninterrupted import:\n got  %+v\n want %+v", got, want)
	}
}
