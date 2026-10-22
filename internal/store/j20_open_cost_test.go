package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestJ20OpenCost times what every notriosctl command pays before it does any
// work: opening a library and bootstrapping it. J20-A found search about 7 s
// slower than J5 at 382,206 notes, and `collections list`, which reads almost
// nothing, took 12 s. Run it with -cpuprofile to see where the time goes:
//
//	NOTRIOS_J20_LIBRARY=/disk/copy.sqlite go test ./internal/store \
//	  -run TestJ20OpenCost -count=1 -v -cpuprofile open.out
//
// Point it at a copy. Bootstrap may write to the library, as the CLI would.
func TestJ20OpenCost(t *testing.T) {
	path := os.Getenv("NOTRIOS_J20_LIBRARY")
	if path == "" {
		t.Skip("set NOTRIOS_J20_LIBRARY to a copy of a real library")
	}
	for pass := 1; pass <= 2; pass++ {
		started := time.Now()
		st, err := OpenSQLiteWithAssetStore(path, filepath.Join(filepath.Dir(path), "assets"))
		if err != nil {
			t.Fatal(err)
		}
		opened := time.Since(started)
		bootStarted := time.Now()
		if err := st.Bootstrap(context.Background()); err != nil {
			_ = st.Close()
			t.Fatal(err)
		}
		booted := time.Since(bootStarted)
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("pass %d: open %s, bootstrap %s", pass, opened.Round(time.Millisecond), booted.Round(time.Millisecond))
	}
}
