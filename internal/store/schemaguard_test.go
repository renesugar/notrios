package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A database written by a newer Notrios is refused, not opened.
//
// Before this guard the failure was silent and destructive. ensureSchemaV4
// through V18 run unconditionally and end with `PRAGMA user_version = n`, so an
// older binary opening a newer database re-ran fifteen old migrations against a
// schema it did not understand and then rewrote the recorded version downward —
// destroying the evidence that the database had ever been newer — and reported
// success.
func TestBootstrapRefusesADatabaseFromTheFuture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.sqlite")

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	// Pretend a newer Notrios has since written this database.
	if err := st.Exec(context.Background(), "PRAGMA user_version = 28;"); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	err = reopened.Bootstrap(context.Background())
	if err == nil {
		t.Fatal("a database newer than this build was opened rather than refused")
	}
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("expected ErrSchemaTooNew, got %v", err)
	}
	// The message must name both versions: "it is newer" without saying how
	// much newer leaves the reader unable to tell which build they need.
	for _, want := range []string{"28", "27", "Upgrade Notrios"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}

	// And the version was left alone. This is the part that used to be lost.
	reopened.mu.Lock()
	version, versionErr := reopened.pragmaUserVersionLocked()
	reopened.mu.Unlock()
	if versionErr != nil {
		t.Fatal(versionErr)
	}
	if version != 28 {
		t.Fatalf("the refusal rewrote user_version to %d; the evidence that the database was newer is gone", version)
	}
}

// The ordinary upgrade path is unaffected: an older database is migrated
// forward on open, which is what happens when a user installs a new Notrios
// over an existing library.
func TestBootstrapStillMigratesAnOlderDatabaseForward(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.sqlite")

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Wind the recorded version back, as an older release would have left it.
	if err := st.Exec(context.Background(), "PRAGMA user_version = 19;"); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Bootstrap(context.Background()); err != nil {
		t.Fatalf("an older database must still migrate forward: %v", err)
	}
	reopened.mu.Lock()
	version, versionErr := reopened.pragmaUserVersionLocked()
	reopened.mu.Unlock()
	if versionErr != nil {
		t.Fatal(versionErr)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("migrated to %d, want %d", version, CurrentSchemaVersion)
	}
}

// A database at exactly the supported version opens, and a fresh one (0) is
// unaffected by the guard.
func TestBootstrapAcceptsCurrentAndFreshDatabases(t *testing.T) {
	for _, name := range []string{"fresh", "current"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notes.sqlite")
			st, err := OpenSQLite(path)
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			if err := st.Bootstrap(context.Background()); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if name == "current" {
				if err := st.Bootstrap(context.Background()); err != nil {
					t.Fatalf("reopening at the current version must succeed: %v", err)
				}
			}
		})
	}
}
