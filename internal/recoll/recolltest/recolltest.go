// Package recolltest keeps a test that runs a real Recoll binary away from the
// machine it runs on.
//
// Recoll can run external indexers that find their own data — the Joplin one
// that Recoll 1.44 enables looks for a library under HOME — so isolating the
// configuration directory and the XDG directories was not enough. A test that
// left HOME alone read the owner's real Joplin library during `go test ./...`
// (v1.0 J44). Every test that runs recollindex or recollq calls Isolate, and
// TestEveryLiveRecollTestIsolates fails the build if one does not.
package recolltest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Isolate points HOME and every XDG base directory at fresh directories under
// one t.TempDir(), and returns that root. Anything a Recoll process, or an
// indexer it starts, resolves through the environment then lands inside the
// test's own directory.
func Isolate(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	directories := map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "xdg-runtime"),
	}
	for name, directory := range directories {
		// The runtime directory must be private or Recoll refuses it.
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		t.Setenv(name, directory)
	}
	// Shared system configuration directories are searched as well; pointing
	// them at an empty directory keeps a desktop's own settings out.
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "xdg-config-dirs"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "xdg-data-dirs"))
	return root
}

// AssertNoExternalIndexers fails the test unless confDir holds a backends file
// that declares no section. A missing file is a failure too: recollindex would
// copy its own sample in, and that sample is what enabled the Joplin indexer.
func AssertNoExternalIndexers(t testing.TB, confDir string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(confDir, "backends"))
	if err != nil {
		t.Fatalf("the sidecar's Recoll configuration has no backends file, so Recoll may install its own: %v", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "[") {
			t.Fatalf("the sidecar's Recoll configuration declares an external indexer %s:\n%s", trimmed, content)
		}
	}
}
