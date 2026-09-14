package store

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestJ21SchemaShapes writes one library for each way a library reaches the
// current schema, so their schemas can be compared before and after J21 changes
// what an open runs:
//
//   - fresh: a new file, bootstrapped once
//   - reopened: a current library, bootstrapped again
//   - from-N: a current library with user_version lowered to N, then
//     bootstrapped. This takes the migration path, lock and backup included.
//
// The from-N libraries hold the current schema with only the version lowered.
// They exercise every step's guard and statements, not a genuinely old schema.
// J21-B also migrates a real v27 library for that reason.
//
//	NOTRIOS_J21_DIR=/somewhere/new go test ./internal/store -run TestJ21SchemaShapes -count=1 -v
//	python3 performance/v1.0-j21/schema_digest.py /somewhere/new/*.sqlite
func TestJ21SchemaShapes(t *testing.T) {
	dir := os.Getenv("NOTRIOS_J21_DIR")
	if dir == "" {
		t.Skip("set NOTRIOS_J21_DIR to a new directory to write the libraries into")
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		t.Fatalf("%s is not empty", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	bootstrap := func(path string) {
		t.Helper()
		st, err := OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Bootstrap(ctx); err != nil {
			_ = st.Close()
			t.Fatal(err)
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	}

	fresh := filepath.Join(dir, "fresh.sqlite")
	bootstrap(fresh)

	reopened := filepath.Join(dir, "reopened.sqlite")
	bootstrap(reopened)
	bootstrap(reopened)

	for _, version := range []int{4, 18, 19, 21, 22, 27} {
		path := filepath.Join(dir, "from-"+strconv.Itoa(version)+".sqlite")
		bootstrap(path)
		st, err := OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Exec(ctx, "PRAGMA user_version = "+strconv.Itoa(version)+";"); err != nil {
			t.Fatal(err)
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
		bootstrap(path)
	}
	t.Logf("wrote libraries to %s", dir)
}
