package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	if got := DefaultPath(); got != "/tmp/explicit.json" {
		t.Fatalf("DefaultPath: %q", got)
	}
	t.Setenv("NOTRIOS_PROFILE_REGISTRY", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/config")
	if got := DefaultPath(); got != filepath.Join("/tmp/config", "notrios", "profiles.json") {
		t.Fatalf("DefaultPath: %q", got)
	}
}
