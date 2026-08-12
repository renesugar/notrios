// Package profiles is the minimum local registry needed to answer "which
// database on this machine is the one a notrios:// link names?".
//
// The registry maps a logical database ID to a local database path. That
// direction matters: a stable link carries the database ID, and the mapping to
// a path is local configuration that the user states explicitly. Nothing here
// scans the filesystem, guesses from a path, or falls back to "the only
// database I can find" — resolving a link must never open a database the user
// did not associate with that identity.
//
// Several local routing entries may hold replicas of one logical database.
// That remains explicit stable-link ambiguity and is reported with every
// candidate. G3 runtime profiles additionally bind distinct replica IDs; an
// unrotated filesystem clone may remain visible for routing diagnostics but
// validation/startup refuse it until an explicit adopt or fork.
package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Format version of the registry file. A reader that does not recognize the
// version refuses the file rather than interpreting unknown fields.
const Version = 2

// Limits keep a hand-edited or corrupted registry bounded.
const (
	MaxProfiles  = 128
	MaxNameBytes = 64
	MaxPathBytes = 4096
)

var (
	// ErrUnknownDatabase means no local profile claims that database ID.
	ErrUnknownDatabase = errors.New("no local profile is registered for that database")
	// ErrAmbiguousDatabase means several local profiles hold copies of the
	// same logical database. Candidates travel with the error.
	ErrAmbiguousDatabase = errors.New("several local profiles hold that database")
	// ErrUnknownProfile means the caller named a profile that is not registered.
	ErrUnknownProfile = errors.New("no such profile")
	// ErrInvalidRegistry means the registry file is unusable. It is never
	// silently repaired or partially applied: a half-read registry could
	// resolve a link to the wrong database.
	ErrInvalidRegistry = errors.New("invalid profile registry")
	// ErrInvalidProfile means a profile being registered is not valid.
	ErrInvalidProfile = errors.New("invalid profile")
)

// Profile is one local database this machine knows about.
type Profile struct {
	Name         string    `json:"name"`
	ProfileID    string    `json:"profile_id,omitempty"`
	DatabaseID   string    `json:"database_id"`
	ReplicaID    string    `json:"replica_id,omitempty"`
	DatabasePath string    `json:"database_path"`
	AssetStore   string    `json:"asset_store,omitempty"`
	ConfigPath   string    `json:"config_path,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
}

// Registry is the on-disk file contents.
type Registry struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// AmbiguityError carries every candidate so a caller can present a choice.
type AmbiguityError struct {
	DatabaseID string
	Candidates []Profile
}

func (e *AmbiguityError) Error() string {
	names := make([]string, 0, len(e.Candidates))
	for _, candidate := range e.Candidates {
		names = append(names, candidate.Name)
	}
	return fmt.Sprintf("%s: %s is held by %s", ErrAmbiguousDatabase.Error(), e.DatabaseID, strings.Join(names, ", "))
}

func (e *AmbiguityError) Unwrap() error { return ErrAmbiguousDatabase }

// DefaultPath returns the registry location. NOTRIOS_PROFILE_REGISTRY wins so
// tests and alternative setups never touch the user's real registry.
func DefaultPath() string {
	if explicit := strings.TrimSpace(os.Getenv("NOTRIOS_PROFILE_REGISTRY")); explicit != "" {
		return explicit
	}
	if configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); configHome != "" {
		return filepath.Join(configHome, "notrios", "profiles.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".notrios", "profiles.json")
	}
	return filepath.Join(home, ".config", "notrios", "profiles.json")
}

// Load reads the registry. A missing file is an empty registry, not an error:
// a machine that has never registered a profile is a normal state.
func Load(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Registry{Version: Version}, nil
	}
	if err != nil {
		return Registry{}, err
	}
	var registry Registry
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return Registry{}, fmt.Errorf("%w: %s: %v", ErrInvalidRegistry, path, err)
	}
	if registry.Version != 1 && registry.Version != Version {
		return Registry{}, fmt.Errorf("%w: %s declares version %d, this build reads versions 1 and %d", ErrInvalidRegistry, path, registry.Version, Version)
	}
	if len(registry.Profiles) > MaxProfiles {
		return Registry{}, fmt.Errorf("%w: %d profiles exceeds the limit of %d", ErrInvalidRegistry, len(registry.Profiles), MaxProfiles)
	}
	seen := map[string]bool{}
	seenProfileIDs := map[string]bool{}
	for _, profile := range registry.Profiles {
		if err := validate(profile); err != nil {
			return Registry{}, fmt.Errorf("%w: %s: %v", ErrInvalidRegistry, path, err)
		}
		key := strings.ToLower(profile.Name)
		if seen[key] {
			return Registry{}, fmt.Errorf("%w: duplicate profile name %q", ErrInvalidRegistry, profile.Name)
		}
		seen[key] = true
		if profile.ProfileID != "" {
			if seenProfileIDs[profile.ProfileID] {
				return Registry{}, fmt.Errorf("%w: duplicate runtime profile ID %q", ErrInvalidRegistry, profile.ProfileID)
			}
			seenProfileIDs[profile.ProfileID] = true
		}
	}
	return registry, nil
}

// Save writes the registry atomically with owner-only permissions: it records
// local filesystem paths, which are private configuration.
func (r Registry) Save(path string) error {
	r.Version = Version
	sort.Slice(r.Profiles, func(i, j int) bool {
		return strings.ToLower(r.Profiles[i].Name) < strings.ToLower(r.Profiles[j].Name)
	})
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".profiles-*.json")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
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
	if err := os.Chmod(tempName, 0o600); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

// Upsert adds or replaces a profile by name. Registering the same name again
// updates it in place, so re-registering after moving a database is one
// command rather than forget-then-register.
func (r Registry) Upsert(profile Profile) (Registry, error) {
	if err := validate(profile); err != nil {
		return r, err
	}
	updated := Registry{Version: Version, Profiles: []Profile{}}
	replaced := false
	for _, existing := range r.Profiles {
		if strings.EqualFold(existing.Name, profile.Name) {
			updated.Profiles = append(updated.Profiles, profile)
			replaced = true
			continue
		}
		updated.Profiles = append(updated.Profiles, existing)
	}
	if !replaced {
		if len(updated.Profiles) >= MaxProfiles {
			return r, fmt.Errorf("%w: the registry already holds %d profiles", ErrInvalidProfile, MaxProfiles)
		}
		updated.Profiles = append(updated.Profiles, profile)
	}
	return updated, nil
}

// Remove forgets a profile. It never touches the database the profile names:
// forgetting is a registry edit, not a deletion.
func (r Registry) Remove(name string) (Registry, error) {
	updated := Registry{Version: Version, Profiles: []Profile{}}
	found := false
	for _, existing := range r.Profiles {
		if strings.EqualFold(existing.Name, name) {
			found = true
			continue
		}
		updated.Profiles = append(updated.Profiles, existing)
	}
	if !found {
		return r, fmt.Errorf("%w: %q", ErrUnknownProfile, name)
	}
	return updated, nil
}

// ByName finds one profile by its local label.
func (r Registry) ByName(name string) (Profile, bool) {
	for _, profile := range r.Profiles {
		if strings.EqualFold(profile.Name, name) {
			return profile, true
		}
	}
	return Profile{}, false
}

// Candidates lists every profile holding a copy of one logical database, in
// stable name order.
func (r Registry) Candidates(databaseID string) []Profile {
	matches := []Profile{}
	for _, profile := range r.Profiles {
		if profile.DatabaseID == databaseID {
			matches = append(matches, profile)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return strings.ToLower(matches[i].Name) < strings.ToLower(matches[j].Name)
	})
	return matches
}

// Resolve chooses the profile that answers a link.
//
// With no preferred profile it succeeds only when exactly one profile holds
// the database. Zero is ErrUnknownDatabase and more than one is an
// *AmbiguityError carrying the candidates — the caller prompts, and this
// package never picks.
//
// A preferred profile must itself hold that database. Naming a profile is how
// a user resolves ambiguity, not how they redirect a link: honouring
// `--profile work` for a link belonging to another database would open the
// wrong notes under an explicit-looking instruction.
func (r Registry) Resolve(databaseID, preferredName string) (Profile, error) {
	if strings.TrimSpace(databaseID) == "" {
		return Profile{}, fmt.Errorf("%w: a database ID is required", ErrUnknownDatabase)
	}
	candidates := r.Candidates(databaseID)
	if preferred := strings.TrimSpace(preferredName); preferred != "" {
		profile, ok := r.ByName(preferred)
		if !ok {
			return Profile{}, fmt.Errorf("%w: %q", ErrUnknownProfile, preferred)
		}
		if profile.DatabaseID != databaseID {
			return Profile{}, fmt.Errorf("%w: profile %q holds database %s, the link names %s",
				ErrUnknownDatabase, profile.Name, profile.DatabaseID, databaseID)
		}
		return profile, nil
	}
	switch len(candidates) {
	case 0:
		return Profile{}, fmt.Errorf("%w: %s", ErrUnknownDatabase, databaseID)
	case 1:
		return candidates[0], nil
	default:
		return Profile{}, &AmbiguityError{DatabaseID: databaseID, Candidates: candidates}
	}
}

func validate(profile Profile) error {
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		return fmt.Errorf("%w: a profile name is required", ErrInvalidProfile)
	}
	if name != profile.Name {
		return fmt.Errorf("%w: profile names may not have leading or trailing whitespace", ErrInvalidProfile)
	}
	if len(name) > MaxNameBytes {
		return fmt.Errorf("%w: profile name is %d bytes, limit %d", ErrInvalidProfile, len(name), MaxNameBytes)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return fmt.Errorf("%w: profile name contains an unsupported character", ErrInvalidProfile)
		}
	}
	if strings.TrimSpace(profile.DatabaseID) == "" {
		return fmt.Errorf("%w: a database ID is required", ErrInvalidProfile)
	}
	if len(profile.DatabaseID) > MaxNameBytes*2 {
		return fmt.Errorf("%w: database ID is too long", ErrInvalidProfile)
	}
	path := strings.TrimSpace(profile.DatabasePath)
	if path == "" {
		return fmt.Errorf("%w: a database path is required", ErrInvalidProfile)
	}
	if len(path) > MaxPathBytes || len(profile.AssetStore) > MaxPathBytes || len(profile.ConfigPath) > MaxPathBytes {
		return fmt.Errorf("%w: path exceeds %d bytes", ErrInvalidProfile, MaxPathBytes)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: database path must be absolute, got %q", ErrInvalidProfile, path)
	}
	if profile.ConfigPath != "" && !filepath.IsAbs(profile.ConfigPath) {
		return fmt.Errorf("%w: config path must be absolute, got %q", ErrInvalidProfile, profile.ConfigPath)
	}
	if (profile.ProfileID == "") != (profile.ConfigPath == "") {
		return fmt.Errorf("%w: runtime profiles require both profile_id and config_path", ErrInvalidProfile)
	}
	if len(profile.ProfileID) > MaxNameBytes*2 || len(profile.ReplicaID) > MaxNameBytes*2 {
		return fmt.Errorf("%w: local identity is too long", ErrInvalidProfile)
	}
	if profile.ProfileID != "" && strings.TrimSpace(profile.ReplicaID) == "" {
		return fmt.Errorf("%w: runtime profiles require a replica_id binding", ErrInvalidProfile)
	}
	return nil
}
