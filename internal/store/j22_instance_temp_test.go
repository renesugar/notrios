package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J22: several Notrios instances can run on one machine, so an instance's
// temporary work goes in its own temp directory, never in the shared system
// temp root.

func isWithin(path, dir string) bool {
	relative, err := filepath.Rel(dir, path)
	return err == nil && relative != "." && !strings.HasPrefix(relative, "..")
}

func TestJ22InstanceTemporaryWorkStaysInItsOwnTempDirectory(t *testing.T) {
	base := t.TempDir()
	type instance struct {
		store   *SQLiteStore
		tempDir string
	}
	open := func(name string) instance {
		dir := filepath.Join(base, name)
		tempDir := filepath.Join(dir, "cache", "tmp")
		st, err := OpenInstanceSQLite(filepath.Join(dir, "data", "notes.sqlite"), filepath.Join(dir, "data", "assets"), tempDir)
		if err != nil {
			t.Fatal(err)
		}
		return instance{store: st, tempDir: tempDir}
	}
	a, b := open("a"), open("b")

	for _, current := range []instance{a, b} {
		space := TempSpaceOf(current.store)
		if space == nil || space.Root() != current.tempDir {
			t.Fatalf("an instance store must carry its own temp space in %s, got %v", current.tempDir, space)
		}
		manifest, err := OpenImportManifestIn(space)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(manifest.root) != space.Dir() {
			t.Fatalf("the import manifest %s is not in the instance's process directory %s", manifest.root, space.Dir())
		}
		// t.TempDir itself lives under the system temp root, so the claim is that
		// the manifest is not placed directly in it, where every instance's
		// temporary work used to go.
		if filepath.Dir(manifest.root) == filepath.Clean(os.TempDir()) {
			t.Fatalf("an instance's import manifest went to the shared temp root: %s", manifest.root)
		}
		if err := manifest.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(manifest.root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("closing the manifest left %s", manifest.root)
		}
	}
	if isWithin(a.store.TempSpace().Dir(), b.tempDir) || isWithin(b.store.TempSpace().Dir(), a.tempDir) {
		t.Fatal("two instances' temp work shared a directory")
	}

	aDir := a.store.TempSpace().Dir()
	if err := a.store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(aDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("closing the store must close its temp space, but %s remains", aDir)
	}
	if _, err := os.Lstat(b.store.TempSpace().Dir()); err != nil {
		t.Fatalf("closing one instance's store removed another's temp space: %v", err)
	}
	if err := b.store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestJ22StoreWithoutAnInstanceKeepsTemporaryWorkPrivate(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if TempSpaceOf(st) != nil {
		t.Fatal("a store opened without an instance must have no instance temp space")
	}
	manifest, err := OpenImportManifestIn(TempSpaceOf(st))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(manifest.root) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(manifest.root), "notrios-import-manifest-") {
		t.Fatalf("with no instance, the manifest belongs in a private system temp directory, got %s", manifest.root)
	}
	if err := manifest.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(manifest.root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("closing the manifest left %s", manifest.root)
	}
}

func TestJ22AnEmptyTempDirOpensAStoreWithNoInstanceTempSpace(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenInstanceSQLite(filepath.Join(dir, "notes.sqlite"), filepath.Join(dir, "assets"), " ")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.TempSpace() != nil {
		t.Fatal("an empty temp directory must mean no instance temp space")
	}
}
