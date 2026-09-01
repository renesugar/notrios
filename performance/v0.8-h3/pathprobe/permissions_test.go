package pathprobe

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/publish"
)

// Notrios protects the copies of a library carefully and the library itself
// carelessly.
//
// Every derived artifact — sync carrier spools, sync backups, backup staging,
// restore review, the catch-up inbox, the profile registry, generated profile
// configs, sync keys, snapshot images — is created 0700/0600. The primary
// roots created by config.EnsureDirectories are created 0755: the directory
// holding the database, the asset store, the projections, the search index and
// the quarantine of untrusted downloads.
//
// So on a shared machine the encrypted backup of a user's notes is owner-only
// and the notes are world-readable. H4 must create every mutable root 0700.
//
// The test sets the umask to zero so it observes the mode the program asked
// for rather than the mode this machine's umask happened to allow. A umask of
// 077 would mask the defect without fixing it, which is exactly why the
// requested mode is the thing worth asserting.
func TestPrimaryDataRootsAreCreatedWorldReadable(t *testing.T) {
	previous := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previous) })

	root := t.TempDir()
	cfg := config.Default()
	cfg.Data.Directory = filepath.Join(root, "data")
	cfg.Data.DatabasePath = filepath.Join(root, "data", "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(root, "data", "assets")
	cfg.Data.ProjectionDir = filepath.Join(root, "data", "projections")
	cfg.SearchSidecar.IndexDir = filepath.Join(root, "data", "search-index")
	cfg.RemoteMedia.QuarantineDir = filepath.Join(root, "data", "quarantine")

	if err := config.EnsureDirectories(cfg); err != nil {
		t.Fatalf("ensure directories: %v", err)
	}

	for _, path := range []string{
		cfg.Data.Directory,
		cfg.Data.AssetStore,
		cfg.Data.ProjectionDir,
		cfg.SearchSidecar.IndexDir,
		cfg.RemoteMedia.QuarantineDir,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		mode := info.Mode().Perm()
		if mode&0o077 == 0 {
			t.Fatalf("H4 may have landed: %s is already owner-only (%04o)", path, mode)
		}
		if mode != 0o755 {
			t.Fatalf("unexpected mode %04o on %s", mode, path)
		}
	}
}

// The inconsistency is sharper than "two different constants in two packages":
// two code paths create *the same directory* with different modes, so the
// permissions on a user's data directory depend on which one ran first.
//
// config.EnsureDirectories creates it 0755 at startup. publish.Save creates it
// 0700 on its way to writing publish-profiles.json. Whichever happens first on
// a given machine decides, and nothing reconciles them afterwards.
func TestTheDataDirectoryGetsDifferentModesDependingOnWhoCreatesIt(t *testing.T) {
	previous := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previous) })

	mode := func(create func(dataDir string) error) os.FileMode {
		root := t.TempDir()
		dataDir := filepath.Join(root, "data")
		if err := create(dataDir); err != nil {
			t.Fatalf("create %s: %v", dataDir, err)
		}
		info, err := os.Stat(dataDir)
		if err != nil {
			t.Fatalf("stat %s: %v", dataDir, err)
		}
		return info.Mode().Perm()
	}

	viaConfig := mode(func(dataDir string) error {
		cfg := config.Default()
		cfg.Data.Directory = dataDir
		cfg.Data.DatabasePath = filepath.Join(dataDir, "notes.sqlite")
		cfg.Data.AssetStore = filepath.Join(dataDir, "assets")
		cfg.Data.ProjectionDir = filepath.Join(dataDir, "projections")
		cfg.SearchSidecar.IndexDir = filepath.Join(dataDir, "search-index")
		cfg.RemoteMedia.QuarantineDir = filepath.Join(dataDir, "quarantine")
		return config.EnsureDirectories(cfg)
	})

	viaPublish := mode(func(dataDir string) error {
		return publish.File{Version: publish.Version}.Save(publish.DefaultPath(dataDir))
	})

	if viaConfig == viaPublish {
		t.Fatalf("H4 may have landed: both paths now create the data directory %04o", viaConfig)
	}
	if viaConfig != 0o755 || viaPublish != 0o700 {
		t.Fatalf("unexpected modes: config=%04o publish=%04o", viaConfig, viaPublish)
	}
}
