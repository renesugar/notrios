package pathprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/profiles"
	"github.com/renesugar/notrios/internal/synckeys"
)

// Notrios resolves "the user's config root" twice, in two packages, by two
// different rules. That is invisible until the environment is unusual, and
// then the two answers disagree about where a user's own files are.
//
// The XDG base directory specification says a relative XDG_CONFIG_HOME is
// invalid and must be ignored. Go's os.UserConfigDir implements that by
// refusing. internal/profiles hand-rolls the lookup and accepts it, which
// resolves the registry against the current working directory instead.
//
// H4 should delete the hand-rolled lookup in favour of one resolver. When it
// does, this test fails, which is the intent.
func TestRelativeXDGConfigHomeIsAcceptedByProfilesAndRefusedByTheStandardLibrary(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative/config")
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")

	registry := profiles.DefaultPath()
	if filepath.IsAbs(registry) {
		t.Fatalf("H4 may have landed: profiles.DefaultPath returned an absolute path %q for a relative XDG_CONFIG_HOME", registry)
	}
	if registry != filepath.Join("relative", "config", "notrios", "profiles.json") {
		t.Fatalf("unexpected registry path %q", registry)
	}

	// The same environment, asked through the standard library, fails closed.
	if _, err := os.UserConfigDir(); err == nil {
		t.Fatal("expected os.UserConfigDir to refuse a relative XDG_CONFIG_HOME")
	} else if !strings.Contains(err.Error(), "relative") {
		t.Fatalf("unexpected error from os.UserConfigDir: %v", err)
	}

	// synckeys goes through os.UserConfigDir, so in this environment Notrios
	// cannot find its sync keys but will happily invent a registry location.
	if _, err := synckeys.DefaultPath("db_probe"); err == nil {
		t.Fatal("expected synckeys.DefaultPath to fail where os.UserConfigDir fails")
	}
}

// With no HOME at all the registry becomes a bare relative path, so the
// profile registry a stable link resolves through depends on the directory the
// process happens to have been started in. Two invocations of notriosctl from
// two directories are then two different machines as far as link routing is
// concerned.
func TestProfileRegistryFallsBackToACurrentDirectoryRelativePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")
	t.Setenv("HOME", "")

	registry := profiles.DefaultPath()
	if filepath.IsAbs(registry) {
		t.Fatalf("H4 may have landed: expected a relative fallback, got %q", registry)
	}
	if registry != filepath.Join(".notrios", "profiles.json") {
		t.Fatalf("unexpected fallback registry path %q", registry)
	}
}

// The config root is also where profile *data* ends up. A generated runtime
// profile puts its database, assets, projections, search index and quarantine
// under the directory holding the registry — the config root — so on Linux a
// user's notes are written inside ~/.config rather than XDG_DATA_HOME.
//
// This is the single largest layout change H4 has to make, and the one that
// needs migration rather than a new default.
func TestGeneratedProfileDataIsPlacedUnderTheConfigRoot(t *testing.T) {
	configRoot := t.TempDir()
	registry := filepath.Join(configRoot, "notrios", "profiles.json")
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", registry)

	profile, err := profiles.Create(t.Context(), profiles.CreateOptions{
		Name:         "h3probe",
		RegistryPath: registry,
		ListenAddr:   "127.0.0.1:8080",
		SyncTarget:   profiles.SyncNone,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if !strings.HasPrefix(profile.DatabasePath, filepath.Dir(registry)) {
		t.Fatalf("H4 may have landed: database %q is no longer under the config root %q",
			profile.DatabasePath, filepath.Dir(registry))
	}
	if _, err := os.Stat(profile.DatabasePath); err != nil {
		t.Fatalf("expected the database to exist at %q: %v", profile.DatabasePath, err)
	}
}
