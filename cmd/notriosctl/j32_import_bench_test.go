package main

import (
	"encoding/json"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ32ImportBench is one measured import for the v1.0 J32 harness
// (performance/v1.0-j32). It is skipped unless the harness runs it.
//
// It runs the CLI's own import command in this process, so what is measured is
// the command a user runs, and reports what only the process can see: wall time
// around the command, what the Go runtime allocated, the live heap after a
// forced collection, and how many statements SQLite was asked to prepare. Peak
// RSS and CPU come from /usr/bin/time around the whole process, which is why
// the harness runs one import per process.
//
// Arguments arrive through this test binary's environment rather than through a
// flag or variable in the shipped CLI:
//
//	NOTRIOS_J32_BENCH_ARGS    JSON array: the arguments after "notriosctl"
//	NOTRIOS_J32_BENCH_STDOUT  where the command's own output goes
//	NOTRIOS_J32_BENCH_REPORT  where this measurement is written, as JSON
//	NOTRIOS_J32_BENCH_CPUPROFILE  optional: where to write a CPU profile, which
//	                              is for finding what to change, never for a
//	                              measured run
//	NOTRIOS_J32_BENCH_MEMPROFILE  optional: where to write an allocation
//	                              profile, on the same terms
//	NOTRIOS_J32_BENCH_HEAPPROFILE optional: where to write an in-use heap
//	                              profile, taken after the forced collection,
//	                              for finding what an import leaves behind
func TestJ32ImportBench(t *testing.T) {
	report := os.Getenv("NOTRIOS_J32_BENCH_REPORT")
	if report == "" {
		t.Skip("run by performance/v1.0-j32/run_bench.py")
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("NOTRIOS_J32_BENCH_ARGS")), &args); err != nil || len(args) < 2 || args[0] != "import" {
		t.Fatalf("NOTRIOS_J32_BENCH_ARGS must be a JSON array starting with \"import\": %v", err)
	}
	output, err := os.Create(os.Getenv("NOTRIOS_J32_BENCH_STDOUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()

	if profile := os.Getenv("NOTRIOS_J32_BENCH_CPUPROFILE"); profile != "" {
		file, err := os.Create(profile)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := pprof.StartCPUProfile(file); err != nil {
			t.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	stdout := os.Stdout
	os.Stdout = output
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	statementsBefore := store.PreparedStatements()
	commitsBefore := store.Commits()
	started := time.Now()
	runImport(args[1:])
	wall := time.Since(started)
	statements := store.PreparedStatements() - statementsBefore
	commits := store.Commits() - commitsBefore
	runtime.ReadMemStats(&after)
	os.Stdout = stdout

	// Live heap is read after the command has returned, so it is what the
	// command left behind rather than what it held at its peak; peak RSS is the
	// measure of that.
	//
	// Collected twice, deliberately. HeapAlloc counts reachable objects *and*
	// unreachable ones the collector has not swept yet, and runtime.GC
	// completes the mark while leaving the sweep lazy. With one collection the
	// number therefore moves with how many collections a run happened to do: a
	// change that allocated 8% less ran 34-36 collections instead of 37-38 and
	// read 70-150 KB *higher*, bimodally and with no relation to its wall time
	// (v1.0 J32-H). The second collection sweeps what the first marked, so what
	// is reported is what is still held.
	var settled runtime.MemStats
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&settled)

	if profile := os.Getenv("NOTRIOS_J32_BENCH_MEMPROFILE"); profile != "" {
		file, err := os.Create(profile)
		if err != nil {
			t.Fatal(err)
		}
		// The allocation profile, not the in-use one: what a change to how a
		// note is buffered would move is total allocation.
		if err := pprof.Lookup("allocs").WriteTo(file, 0); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	if profile := os.Getenv("NOTRIOS_J32_BENCH_HEAPPROFILE"); profile != "" {
		file, err := os.Create(profile)
		if err != nil {
			t.Fatal(err)
		}
		// The in-use profile, after the collection above: what the import is
		// still holding, which is what live_heap_bytes reports as one number.
		if err := pprof.Lookup("heap").WriteTo(file, 0); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	body, err := json.MarshalIndent(map[string]any{
		"wall_seconds":        wall.Seconds(),
		"total_alloc_bytes":   after.TotalAlloc - before.TotalAlloc,
		"mallocs":             after.Mallocs - before.Mallocs,
		"gc_cycles":           after.NumGC - before.NumGC,
		"heap_sys_bytes":      after.HeapSys,
		"live_heap_bytes":     settled.HeapAlloc,
		"prepared_statements": statements,
		"commits":             commits,
		"go_version":          runtime.Version(),
		"gomaxprocs":          runtime.GOMAXPROCS(0),
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
