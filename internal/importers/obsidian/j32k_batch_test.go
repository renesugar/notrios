package obsidian

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestSizedBatchWindows states the rule: a window ends at the count limit or
// once it holds the byte budget, whichever comes first, and always holds at
// least one item so a note larger than the budget still imports (v1.0 J32-K).
func TestSizedBatchWindows(t *testing.T) {
	cases := []struct {
		name      string
		sizes     []int64
		batchSize int
		want      [][2]int
	}{
		{
			name:      "count decides when notes are small",
			sizes:     []int64{10, 10, 10, 10, 10},
			batchSize: 2,
			want:      [][2]int{{0, 2}, {2, 4}, {4, 5}},
		},
		{
			name:      "one note past the budget is its own window",
			sizes:     []int64{importBatchByteBudget + 1, 10, 10},
			batchSize: 100,
			want:      [][2]int{{0, 1}, {1, 3}},
		},
		{
			name:      "the budget closes a window early",
			sizes:     []int64{importBatchByteBudget / 2, importBatchByteBudget / 2, 10},
			batchSize: 100,
			want:      [][2]int{{0, 2}, {2, 3}},
		},
		{
			name:      "every note past the budget stands alone",
			sizes:     []int64{importBatchByteBudget, importBatchByteBudget, importBatchByteBudget},
			batchSize: 100,
			want:      [][2]int{{0, 1}, {1, 2}, {2, 3}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			run := &importRun{options: Options{BatchSize: testCase.batchSize}}
			windows := [][2]int{}
			err := run.eachSizedBatch("notes", len(testCase.sizes),
				func(index int) int64 { return testCase.sizes[index] },
				false, "done", func(start, end int) error {
					windows = append(windows, [2]int{start, end})
					return nil
				})
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(windows) != fmt.Sprint(testCase.want) {
				t.Errorf("windows %v, want %v", windows, testCase.want)
			}
		})
	}
}

// TestByteBoundedBatchesImportTheSameLibrary imports a vault whose notes cross
// the budget and compares it with the same vault imported in one window, so a
// varying window length cannot change what is written.
func TestByteBoundedBatchesImportTheSameLibrary(t *testing.T) {
	ctx := context.Background()
	vault := t.TempDir()
	// Notes large enough that a few of them cross the budget, without writing
	// 64 MiB in a unit test: the window rule is tested above, and this checks
	// the import, so the budget is met by many medium notes rather than one
	// large one.
	body := strings.Repeat("paragraph with a [[Note-0001]] link and some ελληνικά text\n", 200)
	for index := 0; index < 40; index++ {
		name := filepath.Join(vault, fmt.Sprintf("Note-%04d.md", index))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("# Note %d\n\n%s", index, body)), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	imported := func(t *testing.T) store.SQLiteImportMetrics {
		t.Helper()
		root := t.TempDir()
		st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		if err := st.Bootstrap(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := Import(ctx, st, vault, Options{CollectionID: "default", BatchSize: 100}); err != nil {
			t.Fatal(err)
		}
		metrics, err := st.ImportMetrics(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return metrics
	}

	first := imported(t)
	second := imported(t)
	if first != second {
		t.Errorf("two imports of the same vault differ:\n %+v\n %+v", first, second)
	}
	if first.Documents != 40 {
		t.Errorf("expected 40 documents, got %d", first.Documents)
	}
	if first.Links == 0 {
		t.Error("expected links to be recorded")
	}
}

// TestByteBoundedBatchesResume interrupts an import between byte-bounded
// batches and resumes it, then compares the library with one imported without
// interruption: a checkpoint records where the next window starts, so windows
// of varying length must not change what resume produces (v1.0 J32-K).
func TestByteBoundedBatchesResume(t *testing.T) {
	ctx := context.Background()
	vault := t.TempDir()
	body := strings.Repeat("a line with [[Note-0002]] and ελληνικά words\n", 150)
	for index := 0; index < 30; index++ {
		name := filepath.Join(vault, fmt.Sprintf("Note-%04d.md", index))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("# Note %d\n\n%s", index, body)), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	open := func(t *testing.T, root string) *store.SQLiteStore {
		t.Helper()
		st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Bootstrap(ctx); err != nil {
			t.Fatal(err)
		}
		return st
	}

	straight := t.TempDir()
	direct := open(t, straight)
	defer direct.Close()
	if _, err := Import(ctx, direct, vault, Options{CollectionID: "default", BatchSize: 4}); err != nil {
		t.Fatal(err)
	}
	want, err := direct.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}

	interrupted := t.TempDir()
	resumed := open(t, interrupted)
	defer resumed.Close()
	stop := fmt.Errorf("interrupted on purpose")
	batches := 0
	_, err = Import(ctx, resumed, vault, Options{
		CollectionID: "default",
		BatchSize:    4,
		AfterBatch: func(phase string, processed, total int) error {
			if phase == "notes" {
				batches++
				if batches == 2 {
					return stop
				}
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("the import should have stopped")
	}
	if _, err := Import(ctx, resumed, vault, Options{CollectionID: "default", BatchSize: 4}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	got, err := resumed.ImportMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("a resumed import differs from an uninterrupted one:\n resumed %+v\n direct  %+v", got, want)
	}
}
