package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/synckeys"
)

func sampleProfile(name, databaseID, path string) Profile {
	return Profile{
		Name:         name,
		DatabaseID:   databaseID,
		DatabasePath: path,
		RegisteredAt: time.Unix(0, 0).UTC(),
	}
}

func TestMissingRegistryIsAnEmptyRegistry(t *testing.T) {
	registry, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("a machine with no registered profiles is normal: %v", err)
	}
	if len(registry.Profiles) != 0 {
		t.Fatalf("unexpected profiles: %+v", registry.Profiles)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "profiles.json")
	registry := Registry{Version: Version}
	registry, err := registry.Upsert(sampleProfile("work", "db_one", "/srv/work/notes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err = registry.Upsert(sampleProfile("personal", "db_two", "/srv/personal/notes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("registry records local paths and must stay owner-only, got %v", perm)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 2 || loaded.Profiles[0].Name != "personal" {
		t.Fatalf("unexpected registry: %+v", loaded.Profiles)
	}
}

func TestUpsertReplacesByNameAndRemoveForgets(t *testing.T) {
	registry := Registry{Version: Version}
	registry, err := registry.Upsert(sampleProfile("work", "db_one", "/old/notes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err = registry.Upsert(sampleProfile("WORK", "db_one", "/new/notes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Profiles) != 1 || registry.Profiles[0].DatabasePath != "/new/notes.sqlite" {
		t.Fatalf("re-registering a name should update it in place: %+v", registry.Profiles)
	}
	if _, err := registry.Remove("missing"); !errors.Is(err, ErrUnknownProfile) {
		t.Fatalf("expected ErrUnknownProfile, got %v", err)
	}
	registry, err = registry.Remove("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Profiles) != 0 {
		t.Fatalf("remove left profiles: %+v", registry.Profiles)
	}
}

func TestResolveRequiresAnUnambiguousMatch(t *testing.T) {
	registry := Registry{Version: Version}
	for _, profile := range []Profile{
		sampleProfile("laptop", "db_shared", "/home/a/notes.sqlite"),
		sampleProfile("desktop", "db_shared", "/home/b/notes.sqlite"),
		sampleProfile("archive", "db_only", "/home/c/notes.sqlite"),
	} {
		var err error
		registry, err = registry.Upsert(profile)
		if err != nil {
			t.Fatal(err)
		}
	}

	if _, err := registry.Resolve("db_unregistered", ""); !errors.Is(err, ErrUnknownDatabase) {
		t.Fatalf("expected ErrUnknownDatabase, got %v", err)
	}

	only, err := registry.Resolve("db_only", "")
	if err != nil || only.Name != "archive" {
		t.Fatalf("single match: %+v %v", only, err)
	}

	_, err = registry.Resolve("db_shared", "")
	var ambiguity *AmbiguityError
	if !errors.As(err, &ambiguity) {
		t.Fatalf("clones of one database must be reported as ambiguous, got %v", err)
	}
	if len(ambiguity.Candidates) != 2 || ambiguity.Candidates[0].Name != "desktop" {
		t.Fatalf("ambiguity must carry every candidate in stable order: %+v", ambiguity.Candidates)
	}

	chosen, err := registry.Resolve("db_shared", "laptop")
	if err != nil || chosen.Name != "laptop" {
		t.Fatalf("naming a profile should settle ambiguity: %+v %v", chosen, err)
	}
}

// Naming a profile resolves ambiguity; it must never redirect a link into a
// different database, which would open the wrong notes under an
// explicit-looking instruction.
func TestPreferredProfileCannotRedirectToAnotherDatabase(t *testing.T) {
	registry := Registry{Version: Version}
	registry, err := registry.Upsert(sampleProfile("work", "db_work", "/srv/work/notes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err = registry.Upsert(sampleProfile("personal", "db_personal", "/srv/personal/notes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve("db_work", "personal"); !errors.Is(err, ErrUnknownDatabase) {
		t.Fatalf("expected a refusal, got %v", err)
	}
	if _, err := registry.Resolve("db_work", "nonexistent"); !errors.Is(err, ErrUnknownProfile) {
		t.Fatalf("expected ErrUnknownProfile, got %v", err)
	}
}

func TestInvalidProfilesAreRejected(t *testing.T) {
	registry := Registry{Version: Version}
	cases := map[string]Profile{
		"empty name":        sampleProfile("", "db_a", "/srv/notes.sqlite"),
		"path separator":    sampleProfile("a/b", "db_a", "/srv/notes.sqlite"),
		"relative path":     sampleProfile("rel", "db_a", "notes.sqlite"),
		"missing database":  sampleProfile("nodb", "", "/srv/notes.sqlite"),
		"control character": sampleProfile("bad\x00name", "db_a", "/srv/notes.sqlite"),
	}
	for name, profile := range cases {
		if _, err := registry.Upsert(profile); !errors.Is(err, ErrInvalidProfile) {
			t.Fatalf("%s: expected ErrInvalidProfile, got %v", name, err)
		}
	}
}

// A corrupt registry must fail loudly. Reading it partially could resolve a
// link to the wrong database.
func TestCorruptRegistryIsRefusedRatherThanRepaired(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"bad json":        `{"version": 1, "profiles": [`,
		"future version":  `{"version": 99, "profiles": []}`,
		"unknown field":   `{"version": 1, "profiles": [], "surprise": true}`,
		"duplicate names": `{"version":1,"profiles":[{"name":"a","database_id":"db_1","database_path":"/x","registered_at":"1970-01-01T00:00:00Z"},{"name":"A","database_id":"db_2","database_path":"/y","registered_at":"1970-01-01T00:00:00Z"}]}`,
		"relative path":   `{"version":1,"profiles":[{"name":"a","database_id":"db_1","database_path":"x","registered_at":"1970-01-01T00:00:00Z"}]}`,
	}
	for name, contents := range cases {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); !errors.Is(err, ErrInvalidRegistry) {
			t.Fatalf("%s: expected ErrInvalidRegistry, got %v", name, err)
		}
	}
}

func TestDefaultPathPrefersTheExplicitOverride(t *testing.T) {
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "/tmp/explicit.json")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if got != "/tmp/explicit.json" {
		t.Fatalf("DefaultPath: %q", got)
	}

	t.Chdir(t.TempDir())
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/config")
	got, err = DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if want := filepath.Join("/tmp/config", "notrios", "profiles.json"); got != want {
		t.Fatalf("DefaultPath = %q, want %q", got, want)
	}
}

// The two defects H3 found, now inverted: a relative XDG_CONFIG_HOME is ignored
// rather than resolved against the working directory, and an unresolvable
// config root is refused rather than replaced with a relative path.
//
// The second matters most. A registry at .notrios/profiles.json means the
// database a notrios:// link resolves to depends on the directory the process
// was started in, so two invocations from two directories are two different
// machines as far as link routing is concerned.
func TestDefaultPathIgnoresARelativeConfigHome(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")
	t.Setenv("XDG_CONFIG_HOME", "relative/config")
	t.Setenv("HOME", "/home/probe")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("a relative XDG_CONFIG_HOME produced the relative registry path %q", got)
	}
	if want := filepath.Join("/home/probe", ".config", "notrios", "profiles.json"); got != want {
		t.Fatalf("DefaultPath = %q, want the specified fallback %q", got, want)
	}
}

func TestDefaultPathRefusesRatherThanInventingAPath(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	got, err := DefaultPath()
	if err == nil {
		t.Fatalf("expected a refusal, got %q", got)
	}
	if got != "" {
		t.Fatalf("a refusal must not also return a path, got %q", got)
	}
}

// The defect H3 found was not that either resolver was wrong on its own terms.
// It was that there were two of them, and they disagreed: given a relative
// XDG_CONFIG_HOME, internal/synckeys refused as the specification requires and
// internal/profiles invented a location under the working directory. Two halves
// of one application disagreed about where the user's own files were.
//
// This asserts the property that replaced them, rather than the implementation
// detail that both happen to call the same function.
func TestProfilesAndSyncKeysAgreeOnTheConfigRoot(t *testing.T) {
	environments := []struct {
		name                       string
		configHome, home, expectIn string
	}{
		{"absolute XDG_CONFIG_HOME", "/tmp/h4-config", "/home/probe", "/tmp/h4-config/notrios"},
		{"relative XDG_CONFIG_HOME is ignored", "relative/config", "/home/probe", "/home/probe/.config/notrios"},
		{"no XDG_CONFIG_HOME", "", "/home/probe", "/home/probe/.config/notrios"},
	}
	for _, environment := range environments {
		t.Run(environment.name, func(t *testing.T) {
			// Out of the checkout: inside one this is source mode, where the
			// config root is checkout-local by design and the XDG variables are
			// deliberately not consulted.
			t.Chdir(t.TempDir())
			t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")
			t.Setenv("XDG_CONFIG_HOME", environment.configHome)
			t.Setenv("HOME", environment.home)

			registry, registryErr := DefaultPath()
			keys, keysErr := synckeys.DefaultPath("db_agreement")

			if (registryErr == nil) != (keysErr == nil) {
				t.Fatalf("one resolver succeeded and the other did not: registry=%v keys=%v", registryErr, keysErr)
			}
			if registryErr != nil {
				return
			}
			if got := filepath.Dir(registry); got != environment.expectIn {
				t.Errorf("registry is in %q, want %q", got, environment.expectIn)
			}
			if got := filepath.Dir(keys); got != environment.expectIn {
				t.Errorf("sync keys are in %q, want %q", got, environment.expectIn)
			}
			if filepath.Dir(registry) != filepath.Dir(keys) {
				t.Errorf("the two consumers disagree: %q vs %q", filepath.Dir(registry), filepath.Dir(keys))
			}
		})
	}
}

// Both must fail together, too. A registry that resolves while the sync keys
// beside it do not is the same split-brain in a different disguise.
func TestProfilesAndSyncKeysRefuseTogether(t *testing.T) {
	// Installed mode: a checkout resolves its own roots and needs no home at
	// all, which is the isolation property, not a refusal.
	t.Chdir(t.TempDir())
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	if _, err := DefaultPath(); err == nil {
		t.Error("the profile registry resolved with no config root")
	}
	if _, err := synckeys.DefaultPath("db_agreement"); err == nil {
		t.Error("sync keys resolved with no config root")
	}
}
