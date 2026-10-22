package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ17ImportRealVaultOnDisk imports a real vault into a library on a real
// disk, for CPU profiling with `go test -cpuprofile`.
//
// It exists because the profile test beside it cannot answer J17's question.
// TestObsidianImporterProfile builds its library in t.TempDir(), and on this
// machine /tmp is tmpfs: every commit lands in RAM, so any cost of syncing the
// write-ahead log to disk is invisible to it. J5's slow import ran against a
// library on a spinning disk. A profile of the importer has to run where the
// importer was slow.
//
//	NOTRIOS_J17_VAULT=/path/to/vault NOTRIOS_J17_DB=/disk/profile.sqlite \
//	  go test ./internal/importers/obsidian -run TestJ17ImportRealVaultOnDisk \
//	  -count=1 -v -cpuprofile cpu.out -timeout 60m
//
// A CPU profile shows where the process computes, not where it waits. So the
// test also logs wall time, and the gap between wall time and the profile's
// sampled CPU time is itself a measurement.
func TestJ17ImportRealVaultOnDisk(t *testing.T) {
	vault := os.Getenv("NOTRIOS_J17_VAULT")
	dbPath := os.Getenv("NOTRIOS_J17_DB")
	if vault == "" || dbPath == "" {
		t.Skip("set NOTRIOS_J17_VAULT and NOTRIOS_J17_DB to profile a real import on disk")
	}
	// The variable names a path to create, never one to replace: a mistyped
	// path must not cost somebody their library.
	assets := filepath.Join(filepath.Dir(dbPath), filepath.Base(dbPath)+"-assets")
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm", assets} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s already exists; NOTRIOS_J17_DB must name a new file", path)
		}
	}

	st, err := store.OpenSQLiteWithAssetStore(dbPath, assets)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	report, err := Import(ctx, st, vault, Options{BatchSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	t.Logf("imported %d notes in %s (%.1f ms/note) into %s",
		report.NotesImported, elapsed.Round(time.Millisecond),
		float64(elapsed.Milliseconds())/float64(max(report.NotesImported, 1)), dbPath)
}
