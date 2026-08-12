package profiles

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

func createRuntimeProfile(t *testing.T, registry, name, db, assets, listen string, extra func(*CreateOptions)) Profile {
	t.Helper()
	options := CreateOptions{Name: name, RegistryPath: registry, DatabasePath: db, AssetStore: assets, ListenAddr: listen}
	if extra != nil {
		extra(&options)
	}
	profile, err := Create(context.Background(), options)
	if err != nil {
		t.Fatalf("Create %s: %v", name, err)
	}
	return profile
}

func TestRuntimeProfilesAreIsolatedOwnerOnlyAndRedacted(t *testing.T) {
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")
	first := createRuntimeProfile(t, registry, "work", filepath.Join(root, "work.sqlite"), filepath.Join(root, "work-assets"), "127.0.0.1:18121", func(options *CreateOptions) {
		options.SyncTarget = SyncREST
		options.SyncRESTBaseURL = "https://sync.example.invalid/work"
		options.CredentialRef = "secret-service:notrios/work"
	})
	second := createRuntimeProfile(t, registry, "personal", filepath.Join(root, "personal.sqlite"), filepath.Join(root, "personal-assets"), "127.0.0.1:18122", nil)
	if first.DatabaseID == second.DatabaseID || first.ReplicaID == second.ReplicaID || first.ProfileID == second.ProfileID {
		t.Fatalf("fresh profiles shared identity: first=%+v second=%+v", first, second)
	}
	for _, path := range []string{registry, first.ConfigPath, second.ConfigPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %v, want 0600", path, info.Mode().Perm())
		}
	}
	report := Validate(context.Background(), registry, "")
	if !report.Valid || report.Checked != 2 {
		t.Fatalf("validation: %+v", report)
	}
	view, err := Show(registry, "work")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !view.CredentialConfigured || strings.Contains(string(encoded), "secret-service:notrios/work") {
		t.Fatalf("profile view did not redact credential reference: %s", encoded)
	}
}

func TestRuntimeProfileRefusesPathAndPortCollisions(t *testing.T) {
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")
	sharedDB := filepath.Join(root, "one.sqlite")
	createRuntimeProfile(t, registry, "one", sharedDB, filepath.Join(root, "one-assets"), "127.0.0.1:18131", nil)

	_, err := Create(context.Background(), CreateOptions{Name: "same-db", RegistryPath: registry, DatabasePath: sharedDB, AssetStore: filepath.Join(root, "two-assets"), ListenAddr: "127.0.0.1:18132"})
	if err == nil || !strings.Contains(err.Error(), "database path") {
		t.Fatalf("duplicate database path was not refused: %v", err)
	}
	_, err = Create(context.Background(), CreateOptions{Name: "same-port", RegistryPath: registry, DatabasePath: filepath.Join(root, "three.sqlite"), AssetStore: filepath.Join(root, "three-assets"), ListenAddr: "localhost:18131"})
	if err == nil || !strings.Contains(err.Error(), "collide") {
		t.Fatalf("duplicate loopback port was not refused: %v", err)
	}
}

func TestCopiedDatabaseRequiresExplicitAdoptOrFork(t *testing.T) {
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")
	original := createRuntimeProfile(t, registry, "original", filepath.Join(root, "original.sqlite"), filepath.Join(root, "original-assets"), "127.0.0.1:18141", nil)

	copyDatabase := func(name string) string {
		t.Helper()
		target := filepath.Join(root, name+".sqlite")
		bytes, err := os.ReadFile(original.DatabasePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, bytes, 0o600); err != nil {
			t.Fatal(err)
		}
		return target
	}

	copyPath := copyDatabase("copy-refused")
	_, err := Create(context.Background(), CreateOptions{Name: "copy-refused", RegistryPath: registry, DatabasePath: copyPath, AssetStore: filepath.Join(root, "copy-refused-assets"), ListenAddr: "127.0.0.1:18142"})
	if err == nil || !strings.Contains(err.Error(), "copied database") {
		t.Fatalf("raw copied database was not refused: %v", err)
	}

	adoptedPath := copyDatabase("adopted")
	adopted := createRuntimeProfile(t, registry, "adopted", adoptedPath, filepath.Join(root, "adopted-assets"), "127.0.0.1:18143", func(options *CreateOptions) {
		options.CopiedDatabaseAs = "adopt"
	})
	if adopted.DatabaseID != original.DatabaseID || adopted.ReplicaID == original.ReplicaID {
		t.Fatalf("adopt must preserve database and mint replica: original=%+v adopted=%+v", original, adopted)
	}
	loaded, err := Load(registry)
	if err != nil {
		t.Fatal(err)
	}
	candidates := loaded.Candidates(original.DatabaseID)
	if len(candidates) != 2 {
		t.Fatalf("stable-link ambiguity must remain explicit after adoption: %+v", candidates)
	}

	forkPath := copyDatabase("forked")
	forked := createRuntimeProfile(t, registry, "forked", forkPath, filepath.Join(root, "forked-assets"), "127.0.0.1:18144", func(options *CreateOptions) {
		options.CopiedDatabaseAs = "fork"
	})
	if forked.DatabaseID == original.DatabaseID || forked.ReplicaID == original.ReplicaID {
		t.Fatalf("fork must mint database and replica: original=%+v forked=%+v", original, forked)
	}
}

func TestRuntimeProfileValidationFindsStaleConfigAndIdentity(t *testing.T) {
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")
	profile := createRuntimeProfile(t, registry, "stale", filepath.Join(root, "stale.sqlite"), filepath.Join(root, "stale-assets"), "127.0.0.1:18151", nil)
	cfg, err := config.Load(profile.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Profile.ID = "profile_wrong"
	if err := config.WriteProfileFile(profile.ConfigPath, cfg); err != nil {
		t.Fatal(err)
	}
	report := Validate(context.Background(), registry, "stale")
	if report.Valid || !hasIssue(report, "stale_binding") {
		t.Fatalf("stale binding not detected: %+v", report)
	}
	if err := os.Remove(profile.ConfigPath); err != nil {
		t.Fatal(err)
	}
	report = Validate(context.Background(), registry, "stale")
	if report.Valid || !hasIssue(report, "stale_config") {
		t.Fatalf("missing config not detected: %+v", report)
	}
}

func TestRuntimeProfileStartupRefusesChangedDatabaseIdentity(t *testing.T) {
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")
	profile := createRuntimeProfile(t, registry, "changed", filepath.Join(root, "changed.sqlite"), filepath.Join(root, "changed-assets"), "127.0.0.1:18161", nil)
	st, err := store.OpenSQLiteWithAssetStore(profile.DatabasePath, profile.AssetStore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.RotateReplicaIdentity(context.Background()); err != nil {
		st.Close()
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	report := Validate(context.Background(), registry, "changed")
	if report.Valid || !hasIssue(report, "identity_changed") {
		t.Fatalf("changed identity not detected: %+v", report)
	}
	cfg, err := config.Load(profile.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateStartup(context.Background(), cfg); err == nil {
		t.Fatal("startup accepted a database whose bound replica identity changed")
	}
}

func hasIssue(report ValidationReport, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
