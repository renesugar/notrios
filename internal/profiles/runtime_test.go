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
	isolateRoots(t)
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
	isolateRoots(t)
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
	isolateRoots(t)
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
	isolateRoots(t)
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
	isolateRoots(t)
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

// H3's largest finding, inverted: a generated profile's database, asset store,
// projections, search index and quarantine used to default to
// <config>/profiles/<id>/data -- a user's entire library inside ~/.config, the
// one root they are most likely to sync with a dotfile manager or commit to a
// repository.
//
// The generated .yaml stays in the config root, because it is a config file.
func TestGeneratedProfileDataGoesUnderTheDataRoot(t *testing.T) {
	home := isolateRoots(t)
	configHome := filepath.Join(home, ".config")
	dataHome := filepath.Join(home, ".local", "share")

	registry := filepath.Join(configHome, "notrios", "profiles.json")
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", registry)

	profile, err := Create(t.Context(), CreateOptions{
		Name: "h4slicec", RegistryPath: registry,
		ListenAddr: "127.0.0.1:8080", SyncTarget: SyncNone,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	configRoot := filepath.Join(configHome, "notrios")
	if strings.HasPrefix(profile.DatabasePath, configRoot) {
		t.Errorf("the database is still inside the config root: %s", profile.DatabasePath)
	}
	if strings.HasPrefix(profile.AssetStore, configRoot) {
		t.Errorf("the asset store is still inside the config root: %s", profile.AssetStore)
	}
	if want := filepath.Join(dataHome, "notrios"); !strings.HasPrefix(profile.DatabasePath, want) {
		t.Errorf("database %q is not under the data root %q", profile.DatabasePath, want)
	}

	// The generated config file is a config file and stays where those live.
	if !strings.HasPrefix(profile.ConfigPath, configRoot) {
		t.Errorf("the generated profile config left the config root: %s", profile.ConfigPath)
	}
}

// An explicit --data-dir still wins, and keeps everything together under it: a
// caller who named one directory meant one directory.
func TestAnExplicitProfileDataDirectoryIsRespected(t *testing.T) {
	home := isolateRoots(t)
	registry := filepath.Join(home, ".config", "notrios", "profiles.json")
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", registry)

	chosen := filepath.Join(t.TempDir(), "my-library")
	profile, err := Create(t.Context(), CreateOptions{
		Name: "explicit", RegistryPath: registry, DataDirectory: chosen,
		ListenAddr: "127.0.0.1:8080", SyncTarget: SyncNone,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if !strings.HasPrefix(profile.DatabasePath, chosen) {
		t.Errorf("database %q is not under the chosen directory %q", profile.DatabasePath, chosen)
	}
	if !strings.HasPrefix(profile.AssetStore, chosen) {
		t.Errorf("asset store %q is not under the chosen directory %q", profile.AssetStore, chosen)
	}
}

// isolateRoots points every resolved root at a temporary directory.
//
// These tests used to get their data directory beside the registry they had
// already placed in a temp directory, so isolation came for free from the
// defect H4 removed. With profile data resolved from the data root, a test that
// says nothing would write into the checkout's ./data -- so it has to say
// something.
func isolateRoots(t *testing.T) string {
	t.Helper()
	// Out of the checkout as well as into a temp home. Mode detection is
	// filesystem-based: a process whose working directory is inside a checkout
	// is in source mode, where the data root is ./data and the XDG variables
	// are deliberately not consulted. Without this the tests wrote
	// internal/profiles/data into the source tree, which is both pollution and
	// a test that was not exercising the layout it claimed to.
	t.Chdir(t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	return home
}

// `profile list` is the command a user runs to find a profile name for the
// command line, so it reports the port each profile will bind. The port is read
// from the profile's own config rather than duplicated into the registry, where
// a second copy would be free to disagree the moment somebody edited the file.
func TestListReportsThePortFromEachProfileConfig(t *testing.T) {
	home := isolateRoots(t)
	registry := filepath.Join(home, ".config", "notrios", "profiles.json")
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", registry)

	wanted := map[string]string{
		"personal_notes": "127.0.0.1:18201",
		"work_notes":     "127.0.0.1:18202",
		"website_notes":  "127.0.0.1:18203",
	}
	for name, listen := range wanted {
		if _, err := Create(t.Context(), CreateOptions{
			Name: name, RegistryPath: registry, ListenAddr: listen, SyncTarget: SyncNone,
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	summaries, err := List(registry)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(summaries) != len(wanted) {
		t.Fatalf("listed %d profiles, want %d", len(summaries), len(wanted))
	}
	for _, summary := range summaries {
		if summary.Kind != "runtime" {
			t.Errorf("%s: kind = %q", summary.Name, summary.Kind)
		}
		if summary.Problem != "" {
			t.Errorf("%s: unexpected problem %q", summary.Name, summary.Problem)
		}
		if got := summary.ListenAddr; got != wanted[summary.Name] {
			t.Errorf("%s: listen = %q, want %q", summary.Name, got, wanted[summary.Name])
		}
	}
}

// One unreadable profile reports itself rather than hiding the others. A
// listing is the first thing a user runs; failing the whole command because one
// config file went missing would tell them nothing about the nine that are fine.
func TestListSurvivesOneUnreadableProfile(t *testing.T) {
	home := isolateRoots(t)
	registry := filepath.Join(home, ".config", "notrios", "profiles.json")
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", registry)

	for name, listen := range map[string]string{"good": "127.0.0.1:18211", "broken": "127.0.0.1:18212"} {
		if _, err := Create(t.Context(), CreateOptions{
			Name: name, RegistryPath: registry, ListenAddr: listen, SyncTarget: SyncNone,
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	broken, err := Show(registry, "broken")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(broken.ConfigPath); err != nil {
		t.Fatal(err)
	}

	summaries, err := List(registry)
	if err != nil {
		t.Fatalf("list must not fail because one profile is unreadable: %v", err)
	}
	byName := map[string]Summary{}
	for _, summary := range summaries {
		byName[summary.Name] = summary
	}
	if got := byName["good"].ListenAddr; got != "127.0.0.1:18211" {
		t.Errorf("the healthy profile lost its port: %q", got)
	}
	if byName["broken"].Problem == "" {
		t.Error("the unreadable profile did not report why")
	}
	if byName["broken"].ListenAddr != "" {
		t.Error("the unreadable profile reported a port it could not have read")
	}
}
