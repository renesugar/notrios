package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the vendored SQLite build (v0.8 H1 slice C). The
// amalgamation, its compile options, and its linkage are a security and
// compatibility contract; a silent change to any of them is the kind of thing
// nobody notices until a database behaves differently on one platform.
//
// Everything here interrogates the *running* library through SQL rather than
// reading compile-time constants, so it proves what was actually linked. Exact
// provenance lives in csqlite/PROVENANCE.json.

const (
	pinnedSQLiteVersion  = "3.53.4"
	pinnedSQLiteSourceID = "2026-07-24 19:02:57 bf7c7f30031888f4e796e429ab3978879485813aaca6f641c7b33e4e09459bcc"
)

func TestVendoredSQLiteVersionMatchesThePin(t *testing.T) {
	backing := openTestStoreForPin(t)
	ctx := context.Background()

	version, err := backing.sqliteVersion(ctx)
	if err != nil {
		t.Fatalf("query version: %v", err)
	}
	if version != pinnedSQLiteVersion {
		t.Errorf("runtime sqlite_version() = %q, want the pinned %q", version, pinnedSQLiteVersion)
	}

	sourceID, err := backing.queryTextColumn(ctx, `SELECT sqlite_source_id()`)
	if err != nil {
		t.Fatalf("query source id: %v", err)
	}
	if len(sourceID) != 1 || sourceID[0] != pinnedSQLiteSourceID {
		t.Errorf("runtime sqlite_source_id() = %q, want %q", sourceID, pinnedSQLiteSourceID)
	}
}

// The options are read back from the library rather than compared against the
// cgo flag list, so a flag that is present but ineffective still fails.
func TestVendoredSQLiteCompileOptions(t *testing.T) {
	backing := openTestStoreForPin(t)
	options, err := backing.queryTextColumn(context.Background(), `PRAGMA compile_options`)
	if err != nil {
		t.Fatalf("read compile options: %v", err)
	}
	active := make(map[string]bool, len(options))
	for _, option := range options {
		active[option] = true
	}
	for _, required := range []string{
		"THREADSAFE=1",
		"ENABLE_FTS5",
		"DQS=0",
		"OMIT_LOAD_EXTENSION",
		"SECURE_DELETE",
	} {
		if !active[required] {
			t.Errorf("compile option %q is not active; got %v", required, options)
		}
	}
}

// SQLITE_DQS=0 turns a double-quoted string literal into an error instead of a
// silently misread identifier, so a mistyped column name fails loudly rather
// than returning an empty result. This proves the define reached the compiler.
func TestDoubleQuotedStringsAreRejected(t *testing.T) {
	backing := openTestStoreForPin(t)
	if _, err := backing.queryTextColumn(context.Background(), `SELECT "not an identifier"`); err == nil {
		t.Fatal("a double-quoted string literal was accepted; SQLITE_DQS=0 is not in effect")
	}
	// A single-quoted literal is still an ordinary string.
	rows, err := backing.queryTextColumn(context.Background(), `SELECT 'an actual string'`)
	if err != nil {
		t.Fatalf("single-quoted literal was rejected: %v", err)
	}
	if len(rows) != 1 || rows[0] != "an actual string" {
		t.Errorf("single-quoted literal returned %v", rows)
	}
}

// Imported notes are untrusted input and nothing in the product loads an
// extension, so the entry points are compiled out entirely.
func TestLoadableExtensionsAreCompiledOut(t *testing.T) {
	backing := openTestStoreForPin(t)
	if _, err := backing.queryTextColumn(context.Background(), `SELECT load_extension('nonexistent')`); err == nil {
		t.Fatal("load_extension() is callable; OMIT_LOAD_EXTENSION is not in effect")
	}
}

func TestFTS5AndJSONAreAvailable(t *testing.T) {
	backing := openTestStoreForPin(t)
	ctx := context.Background()
	rows, err := backing.queryTextColumn(ctx, `SELECT json_valid('{"a":1}')`)
	if err != nil {
		t.Errorf("JSON support is missing: %v", err)
	} else if len(rows) != 1 || rows[0] != "1" {
		t.Errorf("json_valid returned %v", rows)
	}
	// Bootstrap already builds FTS5 tables, so reaching this point proves the
	// module loads; this confirms it by name for a clearer failure.
	if _, err := backing.queryTextColumn(ctx, `SELECT fts5_version()`); err != nil {
		options, optionsErr := backing.queryTextColumn(ctx, `PRAGMA compile_options`)
		if optionsErr != nil {
			t.Fatalf("read compile options: %v", optionsErr)
		}
		found := false
		for _, option := range options {
			if option == "ENABLE_FTS5" {
				found = true
				break
			}
		}
		if !found {
			t.Error("ENABLE_FTS5 is not active")
		}
	}
}

// WAL is the journal mode the store and every snapshot path assume. It has to
// be checked on a file-backed database: an in-memory one reports "memory"
// because WAL needs a real file to put the write-ahead log beside, which is
// SQLite behaving correctly rather than the pin being wrong.
func TestWALJournalModeOnAFileDatabase(t *testing.T) {
	backing, err := OpenSQLite(filepath.Join(t.TempDir(), "pin.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = backing.Close() })
	if err := backing.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	rows, err := backing.queryTextColumn(context.Background(), `PRAGMA journal_mode`)
	if err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if len(rows) != 1 || !strings.EqualFold(rows[0], "wal") {
		t.Errorf("journal_mode = %v, want wal", rows)
	}
}

func openTestStoreForPin(t *testing.T) *SQLiteStore {
	t.Helper()
	backing, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = backing.Close() })
	if err := backing.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	return backing
}
