package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J22: a store opened without a database file and without an asset store makes
// a private temporary asset root (defaultAssetRoot). Nothing removed it, so every
// such open left a notrios-assets-* directory behind: 16,888 of them were found
// in /tmp by J7. A store removes the root it created, and only that one.

func writeAssetProbe(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "ab"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ab", "probe"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// removeIfLeaked keeps a failing run of this test from adding to the leak it
// tests for. It removes only a notrios-assets-* directory directly in the temp
// root.
func removeIfLeaked(t *testing.T, root string) {
	t.Cleanup(func() {
		if filepath.Dir(root) == filepath.Clean(os.TempDir()) && strings.HasPrefix(filepath.Base(root), "notrios-assets-") {
			_ = os.RemoveAll(root)
		}
	})
}

func TestJ22StoreRemovesTheTempAssetRootItCreated(t *testing.T) {
	opens := map[string]func() (*SQLiteStore, error){
		"OpenSQLite(:memory:)":                     func() (*SQLiteStore, error) { return OpenSQLite(":memory:") },
		"OpenSQLite(empty path)":                   func() (*SQLiteStore, error) { return OpenSQLite("") },
		"OpenSQLiteWithAssetStore(:memory:, \"\")": func() (*SQLiteStore, error) { return OpenSQLiteWithAssetStore(":memory:", "") },
	}
	for name, open := range opens {
		t.Run(name, func(t *testing.T) {
			st, err := open()
			if err != nil {
				t.Fatal(err)
			}
			root := st.AssetRoot()
			removeIfLeaked(t, root)
			if filepath.Dir(root) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(root), "notrios-assets-") {
				t.Fatalf("expected a private temp asset root, got %q", root)
			}
			writeAssetProbe(t, root)
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Close left the temp asset root it created: %s (lstat: %v)", root, err)
			}
			if err := st.Close(); err != nil {
				t.Fatalf("a second Close must stay a no-op: %v", err)
			}
		})
	}
}

func TestJ22StoreNeverRemovesAnAssetRootItDidNotCreate(t *testing.T) {
	dir := t.TempDir()
	fakeTempRoot, err := os.MkdirTemp(dir, "notrios-assets-")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func() (*SQLiteStore, error){
		"the directory beside a database file": func() (*SQLiteStore, error) {
			return OpenSQLite(filepath.Join(dir, "library", "notes.sqlite"))
		},
		"a caller-supplied root for an in-memory database": func() (*SQLiteStore, error) {
			return OpenSQLiteWithAssetStore(":memory:", filepath.Join(dir, "supplied"))
		},
		"a caller-supplied root named like a temp root": func() (*SQLiteStore, error) {
			return OpenSQLiteWithAssetStore(":memory:", fakeTempRoot)
		},
	}
	for name, open := range cases {
		t.Run(name, func(t *testing.T) {
			st, err := open()
			if err != nil {
				t.Fatal(err)
			}
			root := st.AssetRoot()
			writeAssetProbe(t, root)
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "ab", "probe")); err != nil {
				t.Fatalf("Close removed an asset root it did not create: %s (%v)", root, err)
			}
		})
	}
}
