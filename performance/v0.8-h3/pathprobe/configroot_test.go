package pathprobe

// Retired by H4 slice B, which routed both consumers through internal/paths:
//
//   - TestRelativeXDGConfigHomeIsAcceptedByProfilesAndRefusedByTheStandardLibrary
//   - TestProfileRegistryFallsBackToACurrentDirectoryRelativePath
//
// Both characterized the two config-root resolvers disagreeing. There is one
// resolver now, so there is nothing left to disagree. The behaviour they
// asserted is inverted and kept as regression tests in internal/profiles:
// TestDefaultPathIgnoresARelativeConfigHome and
// TestDefaultPathRefusesRatherThanInventingAPath, plus
// TestProfilesAndSyncKeysAgreeOnTheConfigRoot.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/profiles"
)

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
