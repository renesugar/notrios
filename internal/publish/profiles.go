// Package publish stores named publication profiles: the saved selection and
// privacy decisions that produce a scoped, sanitized archive-v2 handoff.
//
// A profile is a decision, not a script. It records what to publish and what to
// withhold; it never records an output path, a command to run, or anything
// taken from note content. Publishing is executed explicitly against a reviewed
// plan (see `RequireReviewedPlan`), because the same profile can produce a
// different set of notes tomorrow.
package publish

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// Version of the profile file. An unrecognized version is refused rather than
// interpreted, because guessing at a publication policy is how private notes
// get published.
const Version = 1

const (
	MaxProfiles     = 128
	MaxNameBytes    = 64
	MaxSelectors    = store.MaxSelectionSelectors
	MaxSelectorSize = store.MaxSelectionSelectorBytes
)

var (
	// ErrInvalidProfile means a profile is not usable as written.
	ErrInvalidProfile = errors.New("invalid publish profile")
	// ErrUnknownProfile means no profile has that name.
	ErrUnknownProfile = errors.New("no such publish profile")
	// ErrInvalidFile means the profile file cannot be trusted. It is never
	// partially applied.
	ErrInvalidFile = errors.New("invalid publish profile file")
	// ErrPlanChanged means the library no longer matches the reviewed plan.
	ErrPlanChanged = errors.New("the reviewed plan no longer matches this library")
)

// Profile is one saved publication decision.
type Profile struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Target is publication_handoff for a public projection, or
	// subset_transfer when the recipient is trusted with provenance and
	// history. Full archives are never a publication.
	Target string `json:"target"`
	// Selection and Policy are handed to the shared P1 planner unchanged, so a
	// profile can express nothing the planner cannot review.
	Selection store.SelectionSpec `json:"selection"`
	Policy    store.PrivacyPolicy `json:"policy"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// File is the on-disk contents.
type File struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// DefaultPath returns the profile file for a data directory. Publication
// profiles name notebooks and tags of one library, so they live beside it
// rather than in a user-wide location.
func DefaultPath(dataDirectory string) string {
	if trimmed := strings.TrimSpace(os.Getenv("NOTRIOS_PUBLISH_PROFILES")); trimmed != "" {
		return trimmed
	}
	if strings.TrimSpace(dataDirectory) == "" {
		dataDirectory = "."
	}
	return filepath.Join(dataDirectory, "publish-profiles.json")
}

// Load reads the profile file. A missing file is an empty set.
func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return File{Version: Version}, nil
	}
	if err != nil {
		return File{}, err
	}
	var file File
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return File{}, fmt.Errorf("%w: %s: %v", ErrInvalidFile, path, err)
	}
	if file.Version != Version {
		return File{}, fmt.Errorf("%w: %s declares version %d, this build reads version %d", ErrInvalidFile, path, file.Version, Version)
	}
	if len(file.Profiles) > MaxProfiles {
		return File{}, fmt.Errorf("%w: %d profiles exceeds the limit of %d", ErrInvalidFile, len(file.Profiles), MaxProfiles)
	}
	seen := map[string]bool{}
	for _, profile := range file.Profiles {
		if err := Validate(profile); err != nil {
			return File{}, fmt.Errorf("%w: %s: %v", ErrInvalidFile, path, err)
		}
		key := strings.ToLower(profile.Name)
		if seen[key] {
			return File{}, fmt.Errorf("%w: duplicate profile name %q", ErrInvalidFile, profile.Name)
		}
		seen[key] = true
	}
	return file, nil
}

// Save writes the file atomically with owner-only permissions: a profile
// describes which notes are private.
func (f File) Save(path string) error {
	f.Version = Version
	sort.Slice(f.Profiles, func(i, j int) bool {
		return strings.ToLower(f.Profiles[i].Name) < strings.ToLower(f.Profiles[j].Name)
	})
	encoded, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".publish-profiles-*.json")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err := temp.Write(encoded); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Upsert adds or replaces a profile by name.
func (f File) Upsert(profile Profile) (File, error) {
	if err := Validate(profile); err != nil {
		return f, err
	}
	updated := File{Version: Version, Profiles: []Profile{}}
	replaced := false
	for _, existing := range f.Profiles {
		if strings.EqualFold(existing.Name, profile.Name) {
			profile.CreatedAt = existing.CreatedAt
			updated.Profiles = append(updated.Profiles, profile)
			replaced = true
			continue
		}
		updated.Profiles = append(updated.Profiles, existing)
	}
	if !replaced {
		if len(updated.Profiles) >= MaxProfiles {
			return f, fmt.Errorf("%w: the file already holds %d profiles", ErrInvalidProfile, MaxProfiles)
		}
		updated.Profiles = append(updated.Profiles, profile)
	}
	return updated, nil
}

// Remove deletes a profile. It publishes nothing and unpublishes nothing: a
// site already generated from this profile is not affected.
func (f File) Remove(name string) (File, error) {
	updated := File{Version: Version, Profiles: []Profile{}}
	found := false
	for _, existing := range f.Profiles {
		if strings.EqualFold(existing.Name, name) {
			found = true
			continue
		}
		updated.Profiles = append(updated.Profiles, existing)
	}
	if !found {
		return f, fmt.Errorf("%w: %q", ErrUnknownProfile, name)
	}
	return updated, nil
}

// ByName finds one profile.
func (f File) ByName(name string) (Profile, error) {
	for _, profile := range f.Profiles {
		if strings.EqualFold(profile.Name, name) {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("%w: %q", ErrUnknownProfile, name)
}

// PlanRequest turns a profile into the shared planner request. The same
// request is used for the reviewed dry run and for the export, so what was
// reviewed is what is published.
func (p Profile) PlanRequest(detailLimit int) store.SelectionPlanRequest {
	return store.SelectionPlanRequest{
		Target:      p.Target,
		Selection:   p.Selection,
		Policy:      p.Policy,
		DetailLimit: detailLimit,
	}
}

// Validate enforces what a publication profile may say.
func Validate(profile Profile) error {
	name := strings.TrimSpace(profile.Name)
	if name == "" || name != profile.Name {
		return fmt.Errorf("%w: a profile name is required and may not have surrounding whitespace", ErrInvalidProfile)
	}
	if len(name) > MaxNameBytes {
		return fmt.Errorf("%w: profile name is %d bytes, limit %d", ErrInvalidProfile, len(name), MaxNameBytes)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return fmt.Errorf("%w: profile name contains an unsupported character", ErrInvalidProfile)
		}
	}
	switch profile.Target {
	case store.SelectionTargetPublicationHandoff, store.SelectionTargetSubsetTransfer:
	case store.SelectionTargetFullArchive:
		// A full archive carries trashed notes, revision history, provenance,
		// and exact source bundles. Calling that a publication profile would
		// make the most dangerous export the easiest one to run by name.
		return fmt.Errorf("%w: full_archive is a backup, not a publication; use `notriosctl export archive-v2`", ErrInvalidProfile)
	default:
		return fmt.Errorf("%w: target must be publication_handoff or subset_transfer", ErrInvalidProfile)
	}
	if selectorCount(profile.Selection) == 0 {
		// The planner enforces this too. Refusing it at save time means the
		// mistake surfaces when the profile is written rather than when it is
		// run against a library.
		return fmt.Errorf("%w: a publication profile must select something; an empty selection would publish the whole library", ErrInvalidProfile)
	}
	if len(profile.Selection.NotebookIDs) > MaxSelectors || len(profile.Selection.Tags) > MaxSelectors {
		return fmt.Errorf("%w: at most %d notebook and tag selectors", ErrInvalidProfile, MaxSelectors)
	}
	for _, value := range append(append([]string{}, profile.Selection.NotebookIDs...), profile.Selection.Tags...) {
		if len(value) > MaxSelectorSize {
			return fmt.Errorf("%w: selector value exceeds %d bytes", ErrInvalidProfile, MaxSelectorSize)
		}
	}
	switch strings.TrimSpace(profile.Policy.LinkAction) {
	case "", store.SelectionLinkActionRetain, store.SelectionLinkActionReport,
		store.SelectionLinkActionPlainText, store.SelectionLinkActionRedact:
	default:
		return fmt.Errorf("%w: link_action must be retain, report, plain_text, or redact", ErrInvalidProfile)
	}
	if profile.Policy.IncludeTrashed != nil && *profile.Policy.IncludeTrashed {
		return fmt.Errorf("%w: a publication may not include trashed notes", ErrInvalidProfile)
	}
	return nil
}

func selectorCount(selection store.SelectionSpec) int {
	count := len(selection.NotebookIDs) + len(selection.Tags) + len(selection.DocumentIDs)
	if strings.TrimSpace(selection.Query) != "" {
		count++
	}
	return count
}

// RequireReviewedPlan is the gate between reviewing a publication and running
// it.
//
// The reviewed digest covers the complete selection: which notes, which
// resources, which link and metadata decisions. If anything about the library
// changed since the review — a note gained a private tag, a new note joined the
// notebook, a link broke — the digest changes and publishing stops. That is the
// point: a publication profile is not a promise about a fixed set of notes, and
// "I checked this yesterday" is not a review of what would go out today.
func RequireReviewedPlan(reviewed, actual string) error {
	reviewed = strings.ToLower(strings.TrimSpace(reviewed))
	if reviewed == "" {
		return fmt.Errorf("%w: publishing requires the manifest digest from a reviewed plan", ErrPlanChanged)
	}
	if reviewed != strings.ToLower(strings.TrimSpace(actual)) {
		return fmt.Errorf("%w: reviewed %s, this library now plans %s; re-run the plan and review the difference", ErrPlanChanged, reviewed, actual)
	}
	return nil
}
