package joplinraw

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
)

// TestJ19InventoryMemory measures the live heap the Joplin RAW importer's
// inventory holds for its whole run, on a real export. Live heap is read after
// a full GC before and after, so transient allocation is not counted as held
// memory. NOTRIOS_J19_HEAP writes an in-use heap profile while the inventory is
// still live, which -memprofile cannot do.
//
//	NOTRIOS_J19_JOPLIN=/path/to/export go test ./internal/importers/joplinraw \
//	  -run TestJ19InventoryMemory -count=1 -v -timeout 60m
func TestJ19InventoryMemory(t *testing.T) {
	source := os.Getenv("NOTRIOS_J19_JOPLIN")
	if source == "" {
		t.Skip("set NOTRIOS_J19_JOPLIN to measure")
	}
	ctx := context.Background()

	before := j19LiveHeap()
	started := time.Now()
	inv, err := readInventory(ctx, source, false)
	if err != nil {
		t.Fatal(err)
	}
	walked := time.Since(started)
	after := j19LiveHeap()

	t.Logf("%d note IDs, %d folders, %d tags, %d resources; inventory read in %s",
		len(inv.NoteIDs), len(inv.Folders), len(inv.Tags), len(inv.Resources), walked.Round(time.Millisecond))
	t.Logf("live heap held by the inventory: %.1f MiB (%.0f bytes per note)",
		float64(after-before)/(1<<20), float64(after-before)/float64(max(len(inv.NoteIDs), 1)))
	// A digest of the whole inventory, for proving that a memory change leaves
	// every value the import reads identical. encoding/json sorts map keys, so
	// two runs over the same export agree exactly when the inventories do.
	if os.Getenv("NOTRIOS_J19_DIGEST") != "" {
		raw, err := json.Marshal(inv)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		t.Logf("inventory digest: %s (%d bytes of JSON)", hex.EncodeToString(sum[:]), len(raw))
	}
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
}

func j19LiveHeap() int64 {
	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return int64(stats.HeapAlloc)
}
