package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

func writeConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// An installed binary must not take its configuration from whatever directory
// it was launched in. That file decides the database path, the listen address,
// the public base URL and the remote-media policy, including whether private
// networks may be fetched.
//
// H3 observed the old behaviour; this is its inverse.
func TestLoadDefaultIgnoresAWorkingDirectoryConfigWhenInstalled(t *testing.T) {
	compiled := config.Default()

	// An isolated environment with no user config: whatever is found must be
	// the compiled default.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	// A directory that is not a checkout, holding a plausible-looking config.
	elsewhere := t.TempDir()
	writeConfig(t, filepath.Join(elsewhere, config.ExampleRelativePath),
		"server:\n  listen_addr: 127.0.0.1:59999\ndata:\n  database_path: /planted/notes.sqlite\n")
	t.Chdir(elsewhere)

	loaded, err := config.LoadDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server.ListenAddr != compiled.Server.ListenAddr {
		t.Errorf("listen address came from the working directory: %q", loaded.Server.ListenAddr)
	}

	// The database must not be the planted one. It is not the compiled default
	// either: with nothing configured, the resolved data root applies, which is
	// the point of an installed layout. Asserting the compiled default here
	// would be asserting that resolution had not happened.
	if loaded.Data.DatabasePath == "/planted/notes.sqlite" {
		t.Errorf("database path came from the working directory: %q", loaded.Data.DatabasePath)
	}
	wantPrefix := filepath.Join(home, ".local", "share", "notrios")
	if !strings.HasPrefix(loaded.Data.DatabasePath, wantPrefix) {
		t.Errorf("database path = %q, want it under the resolved data root %q", loaded.Data.DatabasePath, wantPrefix)
	}
}

// In a checkout the example is still read: that is the developer convenience
// the old behaviour existed for, kept where it is correct.
func TestLoadDefaultReadsTheCheckoutExampleInSourceMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	checkout := t.TempDir()
	writeConfig(t, filepath.Join(checkout, "go.mod"), "module github.com/renesugar/notrios\n")
	writeConfig(t, filepath.Join(checkout, "PLAN.md"), "# plan\n")
	writeConfig(t, filepath.Join(checkout, "AGENTS.md"), "# agents\n")
	writeConfig(t, filepath.Join(checkout, config.ExampleRelativePath),
		"server:\n  listen_addr: 127.0.0.1:58888\n")
	t.Chdir(checkout)

	loaded, err := config.LoadDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server.ListenAddr != "127.0.0.1:58888" {
		t.Fatalf("the checkout example was not read: %q", loaded.Server.ListenAddr)
	}
}

// The user's own config outranks the checkout example, so a developer who has
// written one gets theirs rather than the sample.
func TestLoadDefaultPrefersTheUserConfigOverTheCheckoutExample(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	writeConfig(t, filepath.Join(configHome, "notrios", "config.yaml"),
		"server:\n  listen_addr: 127.0.0.1:57777\n")

	checkout := t.TempDir()
	writeConfig(t, filepath.Join(checkout, "go.mod"), "module github.com/renesugar/notrios\n")
	writeConfig(t, filepath.Join(checkout, "PLAN.md"), "# plan\n")
	writeConfig(t, filepath.Join(checkout, "AGENTS.md"), "# agents\n")
	writeConfig(t, filepath.Join(checkout, config.ExampleRelativePath),
		"server:\n  listen_addr: 127.0.0.1:58888\n")
	t.Chdir(checkout)

	loaded, err := config.LoadDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server.ListenAddr != "127.0.0.1:57777" {
		t.Fatalf("the user's own config did not win: %q", loaded.Server.ListenAddr)
	}
}

// Every mutable root is created owner-only.
//
// H3 found the opposite: EnsureDirectories created the primary roots 0755 while
// every derived artifact -- sync backups, carrier spools, staging, the profile
// registry, sync keys, snapshot images -- was created 0700. On a shared machine
// the encrypted backup of a user's notes was owner-only and the notes were
// world-readable.
//
// The umask is set to zero so this observes the mode the program asked for
// rather than the mode this machine's umask happened to allow. A umask of 077
// would hide a regression here without preventing it.
func TestEveryMutableRootIsCreatedOwnerOnly(t *testing.T) {
	previous := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previous) })

	root := t.TempDir()
	cfg := config.Default()
	cfg.Data.Directory = filepath.Join(root, "data")
	cfg.Data.DatabasePath = filepath.Join(root, "data", "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(root, "data", "assets")
	cfg.Data.StateDir = filepath.Join(root, "state")
	cfg.Data.CacheDir = filepath.Join(root, "cache")
	cfg.Data.RuntimeDir = filepath.Join(root, "runtime")
	cfg.Data.ProjectionDir = filepath.Join(root, "cache", "projections")
	cfg.SearchSidecar.IndexDir = filepath.Join(root, "cache", "search-index")
	cfg.RemoteMedia.QuarantineDir = filepath.Join(root, "state", "quarantine")

	if err := config.EnsureDirectories(cfg); err != nil {
		t.Fatalf("ensure directories: %v", err)
	}

	for _, path := range []string{
		cfg.Data.Directory, cfg.Data.AssetStore, cfg.Data.StateDir, cfg.Data.CacheDir,
		cfg.Data.RuntimeDir, cfg.Data.ProjectionDir, cfg.SearchSidecar.IndexDir,
		cfg.RemoteMedia.QuarantineDir,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Errorf("%s is %04o; every mutable root must be owner-only", path, mode)
		}
	}
}

// A configuration that names a path keeps it. This is what makes "no implicit
// migration" true at the only place it could stop being true: an installed
// binary meeting an old checkout-relative config still reads the old locations.
func TestAConfiguredPathIsNeverRelocated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeConfig(t, configPath, "data:\n  directory: ./data\n  database_path: ./data/notes.sqlite\n")

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Data.Directory != "./data" {
		t.Errorf("a stated directory was relocated to %q", loaded.Data.Directory)
	}
	if loaded.Data.DatabasePath != "./data/notes.sqlite" {
		t.Errorf("a stated database path was relocated to %q", loaded.Data.DatabasePath)
	}
	// Roots the file did not mention are still filled in, or an old config
	// would leave the new roots empty.
	if loaded.Data.StateDir == "" || loaded.Data.CacheDir == "" {
		t.Errorf("unstated roots were left empty: state=%q cache=%q", loaded.Data.StateDir, loaded.Data.CacheDir)
	}
}

// One directory means one directory.
//
// A configuration that states a data directory and nothing else must keep its
// derived roots under it. This is what makes every configuration written before
// H4 behave exactly as it did: without it, an existing config naming ./data
// kept its database there and silently moved its sync spools, backups and
// catch-up inbox into ~/.local/state -- a surprise for a user, and for an
// isolated test or a sandboxed deployment an escape from the directory that was
// supposed to contain everything.
//
// This was found by two end-to-end tests failing, not by review.
func TestAStatedDataDirectoryContainsItsDerivedRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	library := filepath.Join(t.TempDir(), "library")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeConfig(t, configPath, "data:\n  directory: "+library+"\n")

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	for name, got := range map[string]string{
		"state":      loaded.Data.StateDir,
		"cache":      loaded.Data.CacheDir,
		"runtime":    loaded.Data.RuntimeDir,
		"database":   loaded.Data.DatabasePath,
		"assets":     loaded.Data.AssetStore,
		"projection": loaded.Data.ProjectionDir,
		"index":      loaded.SearchSidecar.IndexDir,
		"quarantine": loaded.RemoteMedia.QuarantineDir,
	} {
		if !strings.HasPrefix(got, library) {
			t.Errorf("%s resolved to %q, outside the stated data directory %q", name, got, library)
		}
	}
}

// Load("") and LoadDefault() must answer the same question.
//
// They did not, and it was a real bug: notriosctl's `doctor` called LoadDefault
// while every other subcommand called config.Load("") through
// openStoreFromFlags. So `doctor` reported the user's configured database while
// `export`, `import` and the rest quietly operated on a different, empty one
// created from compiled defaults. An end-to-end example caught it -- the export
// saw a fresh database's four default notebooks and none of the seeded content.
func TestLoadWithNoPathIsTheSameAsLoadDefault(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	writeConfig(t, filepath.Join(configHome, "notrios", "config.yaml"),
		"data:\n  database_path: /chosen/notes.sqlite\n")
	t.Chdir(t.TempDir())

	viaEmpty, err := config.Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	viaDefault, err := config.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}

	if viaEmpty.Data.DatabasePath != viaDefault.Data.DatabasePath {
		t.Fatalf("Load(\"\") = %q but LoadDefault = %q",
			viaEmpty.Data.DatabasePath, viaDefault.Data.DatabasePath)
	}
	if viaEmpty.Data.DatabasePath != "/chosen/notes.sqlite" {
		t.Fatalf("the user's configured database was ignored: %q", viaEmpty.Data.DatabasePath)
	}
}

// UseDataDirectory is the same rule ApplyResolvedRoots applies to a stated
// data directory, exported because callers that build a Config by hand need it.
// Several tests set Data.Directory and silently kept the compiled ./data
// defaults for the search index and quarantine, creating directories in the
// source tree relative to whatever their working directory happened to be.
func TestUseDataDirectoryPlacesEveryRoot(t *testing.T) {
	cfg := config.Default()
	config.UseDataDirectory(&cfg, "/srv/library", nil)

	for name, got := range map[string]string{
		"directory":  cfg.Data.Directory,
		"database":   cfg.Data.DatabasePath,
		"assets":     cfg.Data.AssetStore,
		"state":      cfg.Data.StateDir,
		"cache":      cfg.Data.CacheDir,
		"runtime":    cfg.Data.RuntimeDir,
		"projection": cfg.Data.ProjectionDir,
		"index":      cfg.SearchSidecar.IndexDir,
		"quarantine": cfg.RemoteMedia.QuarantineDir,
	} {
		if !strings.HasPrefix(got, "/srv/library") {
			t.Errorf("%s = %q, outside the given directory", name, got)
		}
	}

	// A path the caller already stated is left alone.
	kept := config.Default()
	kept.RemoteMedia.QuarantineDir = "/elsewhere/quarantine"
	config.UseDataDirectory(&kept, "/srv/library", map[string]bool{"remote_media.quarantine_dir": true})
	if kept.RemoteMedia.QuarantineDir != "/elsewhere/quarantine" {
		t.Errorf("a stated path was overwritten: %q", kept.RemoteMedia.QuarantineDir)
	}
}
