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

// J32-J measured the checkpoint consolidation and reverted it: the duplicate
// write was an autocommitted statement rather than a second transaction, so
// removing it left the commit count unchanged and the clock unmoved. What is
// kept from that slice is the counter that showed it, and the test below, which
// is about resume rather than about how many times a checkpoint is written.

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
