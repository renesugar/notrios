package tempspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func mustOpen(t *testing.T, root string) *Space {
	t.Helper()
	space, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = space.Close() })
	return space
}

func mustWork(t *testing.T, space *Space) string {
	t.Helper()
	dir, err := space.MkdirTemp("notrios-import-manifest-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.sqlite"), []byte("spool"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// crash releases a Space's lock the way a killed process does, leaving its
// directory and lock file behind.
func crash(t *testing.T, space *Space) {
	t.Helper()
	space.mu.Lock()
	defer space.mu.Unlock()
	space.closed = true
	if err := space.lock.Close(); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func within(path, dir string) bool {
	relative, err := filepath.Rel(dir, path)
	return err == nil && relative != "." && !strings.HasPrefix(relative, "..")
}

func TestNilSpaceUsesAPrivateSystemTempDirectory(t *testing.T) {
	var space *Space
	dir, err := space.MkdirTemp("notrios-j22-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if filepath.Dir(dir) != filepath.Clean(os.TempDir()) {
		t.Fatalf("with no instance, temp work belongs directly in the system temp root, got %s", dir)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("temp work must be owner-only: %v %v", info, err)
	}
	if space.Close() != nil || space.Dir() != "" || space.Root() != "" || space.Swept() != nil {
		t.Fatal("a nil Space must be inert")
	}
}

func TestEachProcessGetsItsOwnLockedDirectory(t *testing.T) {
	root := t.TempDir()
	first, second := mustOpen(t, root), mustOpen(t, root)
	if first.Dir() == second.Dir() {
		t.Fatalf("two processes of one instance shared %s", first.Dir())
	}
	for _, space := range []*Space{first, second} {
		if filepath.Dir(space.Dir()) != root {
			t.Fatalf("process directory %s is not in the instance temp directory %s", space.Dir(), root)
		}
		if info, err := os.Stat(space.Dir()); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("process directory must be owner-only: %v %v", info, err)
		}
		work := mustWork(t, space)
		if filepath.Dir(work) != space.Dir() {
			t.Fatalf("temp work %s is not in its process directory %s", work, space.Dir())
		}
		probe, err := os.OpenFile(space.Dir()+lockSuffix, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := lock(probe); !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("an open Space's lock must be held, got %v", err)
		}
		_ = probe.Close()
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if exists(first.Dir()) || exists(first.Dir()+lockSuffix) {
		t.Fatal("Close must remove the process directory and its lock file")
	}
	if !exists(second.Dir()) {
		t.Fatal("closing one process's Space removed another's")
	}
	if _, err := first.MkdirTemp("x-*"); !errors.Is(err, ErrClosed) {
		t.Fatalf("a closed Space must refuse new work, got %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("a second Close must be a no-op: %v", err)
	}
}

func TestTwoInstancesNeverShareATempDirectory(t *testing.T) {
	rootA, rootB := t.TempDir(), t.TempDir()
	a, b := mustOpen(t, rootA), mustOpen(t, rootB)
	workA, workB := mustWork(t, a), mustWork(t, b)
	if !within(workA, rootA) || within(workA, rootB) || !within(workB, rootB) || within(workB, rootA) {
		t.Fatalf("instance temp work crossed instances: %s, %s", workA, workB)
	}
}

func TestOpenRemovesItsOwnInstancesCrashLeftovers(t *testing.T) {
	root := t.TempDir()
	crashed := mustOpen(t, root)
	work := mustWork(t, crashed)
	crash(t, crashed)
	orphanLock := filepath.Join(root, processPrefix+"0123456789abcdef"+lockSuffix)
	if err := os.WriteFile(orphanLock, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	next := mustOpen(t, root)
	if exists(work) || exists(crashed.Dir()) || exists(crashed.Dir()+lockSuffix) || exists(orphanLock) {
		t.Fatal("a crashed process's unlocked leftovers in this instance's temp directory must be removed")
	}
	if swept := next.Swept(); len(swept) != 1 || swept[0] != crashed.Dir() {
		t.Fatalf("Swept must name what was removed, got %v", swept)
	}
}

func TestOpenLeavesALiveProcessesDirectoryAlone(t *testing.T) {
	root := t.TempDir()
	live := mustOpen(t, root)
	work := mustWork(t, live)
	next := mustOpen(t, root)
	if !exists(work) || !exists(live.Dir()+lockSuffix) {
		t.Fatal("opening a Space removed a live process's temp work")
	}
	if len(next.Swept()) != 0 {
		t.Fatalf("nothing was a leftover, but Swept reports %v", next.Swept())
	}
}

func TestOpenNeverTouchesAnotherInstanceOrAnythingItDidNotMake(t *testing.T) {
	rootA, rootB := t.TempDir(), t.TempDir()
	crashedB := mustOpen(t, rootB)
	workB := mustWork(t, crashedB)
	crash(t, crashedB)

	outside := t.TempDir()
	precious := filepath.Join(outside, "notes.md")
	if err := os.WriteFile(precious, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	const id = "fedcba9876543210"
	unrelated := map[string]func(string) error{
		"notes.txt":                     func(p string) error { return os.WriteFile(p, []byte("x"), 0o600) },
		"p-nolock":                      func(p string) error { return os.Mkdir(p, 0o700) },
		"p-NOTHEX.lock":                 func(p string) error { return os.WriteFile(p, nil, 0o600) },
		processPrefix + id:              func(p string) error { return os.Symlink(outside, p) },
		processPrefix + id + lockSuffix: func(p string) error { return os.WriteFile(p, nil, 0o600) },
		"notrios-import-manifest-123":   func(p string) error { return os.Mkdir(p, 0o700) },
	}
	for name, make := range unrelated {
		if err := make(filepath.Join(rootA, name)); err != nil {
			t.Fatal(err)
		}
	}

	mustOpen(t, rootA)
	if !exists(workB) || !exists(crashedB.Dir()+lockSuffix) {
		t.Fatal("opening one instance's Space removed another instance's leftovers")
	}
	for name := range unrelated {
		if !exists(filepath.Join(rootA, name)) {
			t.Fatalf("Open removed %s, which no process directory pair accounts for", name)
		}
	}
	if !exists(precious) {
		t.Fatal("Open followed a symlink out of the instance temp directory")
	}
}

func TestOpenRequiresADirectory(t *testing.T) {
	if _, err := Open(" "); err == nil {
		t.Fatal("an empty instance temp directory must be refused")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); err == nil {
		t.Fatal("a file must not be accepted as an instance temp directory")
	}
}
