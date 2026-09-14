package obsidian

import (
	"context"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
)

// TestJ19InventoryMemory measures the external performance review's claim that
// the importer's whole-vault maps cost hundreds of megabytes at 400,000 notes.
// It builds exactly what an import holds for its whole run — the inventory and
// the link namespace — and reports the live heap they keep, after a full GC
// before and after, so transient allocation is not counted as held memory.
//
//	NOTRIOS_J19_VAULT=/path/to/vault go test ./internal/importers/obsidian \
//	  -run TestJ19InventoryMemory -count=1 -v -timeout 60m
func TestJ19InventoryMemory(t *testing.T) {
	vault := getenvOrSkip(t, "NOTRIOS_J19_VAULT")
	ctx := context.Background()

	before := liveHeap()
	started := time.Now()
	// false is what an import without --preserve-source holds, the CLI default.
	inv, err := readInventory(ctx, vault, os.Getenv("NOTRIOS_J19_RETAIN_FILES") != "")
	if err != nil {
		t.Fatal(err)
	}
	walked := time.Since(started)
	afterInventory := liveHeap()
	namespace := buildLinkNamespace(inv)
	afterNamespace := liveHeap()

	t.Logf("%d files, %d notes, %d assets, %d folders; inventory walked in %s",
		len(inv.Files), len(inv.Notes), len(inv.Assets), len(inv.Folders), walked.Round(time.Millisecond))
	t.Logf("live heap held by the inventory: %.1f MiB (%.0f bytes per file)",
		mib(afterInventory-before), float64(afterInventory-before)/float64(max(len(inv.Notes)+len(inv.Assets), 1)))
	t.Logf("live heap held by the link namespace: %.1f MiB (%.0f bytes per note)",
		mib(afterNamespace-afterInventory), float64(afterNamespace-afterInventory)/float64(max(len(inv.Notes), 1)))
	t.Logf("together: %.1f MiB", mib(afterNamespace-before))
	// -memprofile is written after the test returns, when nothing here is live
	// any more, so the held-memory breakdown is written while it still is.
	if path := os.Getenv("NOTRIOS_J19_HEAP"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		if err := pprof.WriteHeapProfile(file); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	runtime.KeepAlive(inv)
	runtime.KeepAlive(namespace)
}

func getenvOrSkip(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Skipf("set %s to measure", name)
	}
	return value
}

func liveHeap() int64 {
	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return int64(stats.HeapAlloc)
}

func mib(bytes int64) float64 {
	return float64(bytes) / (1 << 20)
}
