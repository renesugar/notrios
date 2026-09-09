package publish

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func samplePublicProfile(name string) Profile {
	return Profile{
		Name:      name,
		Target:    store.SelectionTargetPublicationHandoff,
		Selection: store.SelectionSpec{NotebookIDs: []string{"nb_public"}},
		Policy:    store.PrivacyPolicy{LinkAction: store.SelectionLinkActionPlainText},
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "publish-profiles.json")
	file, err := File{Version: Version}.Upsert(samplePublicProfile("research"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Save(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("a profile says which notes are private and must stay owner-only, got %v", perm)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := loaded.ByName("RESEARCH")
	if err != nil || profile.Selection.NotebookIDs[0] != "nb_public" {
		t.Fatalf("round trip: %+v %v", profile, err)
	}
	if _, err := loaded.ByName("absent"); !errors.Is(err, ErrUnknownProfile) {
		t.Fatalf("expected ErrUnknownProfile, got %v", err)
	}
}

// A full archive carries trashed notes, every revision, provenance, and exact
// source bundles. Letting it hide behind a publication profile name would make
// the most dangerous export the easiest one to run by accident.
func TestFullArchiveIsNotAPublicationProfile(t *testing.T) {
	profile := samplePublicProfile("everything")
	profile.Target = store.SelectionTargetFullArchive
	err := Validate(profile)
	if !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected a refusal, got %v", err)
	}
}

func TestProfileMustSelectSomething(t *testing.T) {
	profile := samplePublicProfile("empty")
	profile.Selection = store.SelectionSpec{}
	if err := Validate(profile); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("an empty selection would publish the whole library: %v", err)
	}
}

func TestProfileMayNotPublishTrash(t *testing.T) {
	profile := samplePublicProfile("trashy")
	includeTrashed := true
	profile.Policy.IncludeTrashed = &includeTrashed
	if err := Validate(profile); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected a refusal, got %v", err)
	}
}

func TestInvalidProfilesAreRejected(t *testing.T) {
	cases := map[string]func(*Profile){
		"no name":         func(p *Profile) { p.Name = "" },
		"padded name":     func(p *Profile) { p.Name = " research " },
		"path separator":  func(p *Profile) { p.Name = "a/b" },
		"unknown target":  func(p *Profile) { p.Target = "everything" },
		"unknown action":  func(p *Profile) { p.Policy.LinkAction = "delete" },
		"oversized value": func(p *Profile) { p.Selection.Tags = []string{string(make([]byte, MaxSelectorSize+1))} },
	}
	for name, mutate := range cases {
		profile := samplePublicProfile("valid")
		mutate(&profile)
		if err := Validate(profile); !errors.Is(err, ErrInvalidProfile) {
			t.Fatalf("%s: expected ErrInvalidProfile, got %v", name, err)
		}
	}
}

func TestUpsertReplacesByNameAndPreservesCreation(t *testing.T) {
	original := samplePublicProfile("research")
	file, err := File{Version: Version}.Upsert(original)
	if err != nil {
		t.Fatal(err)
	}
	updated := samplePublicProfile("RESEARCH")
	updated.Policy.LinkAction = store.SelectionLinkActionRedact
	file, err = file.Upsert(updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Profiles) != 1 || file.Profiles[0].Policy.LinkAction != store.SelectionLinkActionRedact {
		t.Fatalf("re-saving a name should update it in place: %+v", file.Profiles)
	}
	if _, err := file.Remove("missing"); !errors.Is(err, ErrUnknownProfile) {
		t.Fatalf("expected ErrUnknownProfile, got %v", err)
	}
	if file, err = file.Remove("research"); err != nil || len(file.Profiles) != 0 {
		t.Fatalf("remove: %+v %v", file.Profiles, err)
	}
}

// A profile file that cannot be trusted must not be partially applied: half a
// publication policy is how private notes get published.
func TestCorruptProfileFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"bad json":       `{"version":1,"profiles":[`,
		"future version": `{"version":99,"profiles":[]}`,
		"unknown field":  `{"version":1,"profiles":[],"publish_now":true}`,
		"full archive":   `{"version":1,"profiles":[{"name":"a","target":"full_archive","selection":{"tags":["x"]},"policy":{},"created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"}]}`,
		"empty selector": `{"version":1,"profiles":[{"name":"a","target":"publication_handoff","selection":{},"policy":{},"created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"}]}`,
		"duplicate name": `{"version":1,"profiles":[{"name":"a","target":"publication_handoff","selection":{"tags":["x"]},"policy":{},"created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"},{"name":"A","target":"publication_handoff","selection":{"tags":["y"]},"policy":{},"created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"}]}`,
	}
	for name, contents := range cases {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); !errors.Is(err, ErrInvalidFile) {
			t.Fatalf("%s: expected ErrInvalidFile, got %v", name, err)
		}
	}
}

// Publishing is gated on the digest of a reviewed plan, so "I checked this
// yesterday" cannot stand in for a review of what would go out today.
func TestRequireReviewedPlan(t *testing.T) {
	if err := RequireReviewedPlan("abc123", "abc123"); err != nil {
		t.Fatalf("matching digests must pass: %v", err)
	}
	if err := RequireReviewedPlan("ABC123", "abc123"); err != nil {
		t.Fatalf("digest comparison should not be case sensitive: %v", err)
	}
	if err := RequireReviewedPlan("", "abc123"); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("publishing without a review must fail: %v", err)
	}
	err := RequireReviewedPlan("stale", "abc123")
	if !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("a changed library must stop the publication: %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "re-run the plan") {
		t.Fatalf("the error must say what to do next: %q", got)
	}
}

func TestDefaultPathPrefersTheExplicitOverride(t *testing.T) {
	t.Setenv("NOTRIOS_PUBLISH_PROFILES", "/tmp/explicit.json")
	if got := DefaultPath("/data"); got != "/tmp/explicit.json" {
		t.Fatalf("DefaultPath: %q", got)
	}
	t.Setenv("NOTRIOS_PUBLISH_PROFILES", "")
	if got := DefaultPath("/data"); got != filepath.Join("/data", "publish-profiles.json") {
		t.Fatalf("DefaultPath: %q", got)
	}
}
