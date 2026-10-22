// Package installreport answers, exhaustively, where a Notrios installation
// keeps things (v1.0 J11).
//
// `notriosctl paths` reports the resolved roots, and `scripts/lifecycle.py`
// writes an install manifest of the program files it copied. Neither lists the
// user's data, which is the half a purge deletes, so neither can answer "did
// the purge work". This walks the roots, reads the profile registry so every
// registered library is named including the ones outside those roots, and
// reads the install manifest when it is there.
//
// It records paths. It never opens a note, a database or a configuration file.
package installreport

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/profiles"
	"github.com/renesugar/notrios/internal/purge"
)

// Kinds of entry.
const (
	KindDirectory = "directory"
	KindFile      = "file"
	KindSymlink   = "symlink"
	KindMissing   = "missing"
)

// Entry is one path the installation occupies.
type Entry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	// Category is the root a path belongs to ("config", "data", …), or
	// "program" for an installed program file, or "profile" for a registered
	// library's path.
	Category string `json:"category"`
	// Owned says whether a purge of this installation removes the path. A
	// profile that keeps its library outside the roots is not owned, and purge
	// deliberately leaves it alone.
	Owned   bool   `json:"owned"`
	Profile string `json:"profile,omitempty"`
	Field   string `json:"field,omitempty"`
	Bytes   int64  `json:"bytes,omitempty"`
}

// Report is the structure and manifest of one installation.
type Report struct {
	Schema string `json:"schema"`
	Mode   string `json:"mode"`
	// Roots are the resolved roots, by name.
	Roots map[string]string `json:"roots"`
	// Structure is every directory the installation occupies, and Manifest
	// every file, both sorted by path.
	Structure []Entry `json:"structure"`
	Manifest  []Entry `json:"manifest"`
	// Program describes the installed program files, when an install manifest
	// was found, and Notes says why it was not when it was not.
	Program *Program `json:"program,omitempty"`
	Notes   []string `json:"notes"`
	Counts  Counts   `json:"counts"`
}

// Program is what the installer recorded writing.
type Program struct {
	ManifestPath string `json:"manifest_path"`
	Version      string `json:"version,omitempty"`
	InstalledAt  string `json:"installed_at,omitempty"`
	Prefix       string `json:"prefix,omitempty"`
	Entries      int    `json:"entries"`
}

// Counts summarise the report, so a reader can see its size before reading it.
type Counts struct {
	Directories int   `json:"directories"`
	Files       int   `json:"files"`
	Bytes       int64 `json:"bytes"`
	External    int   `json:"external"`
	Profiles    int   `json:"profiles"`
}

// Schema names this report's shape.
const Schema = "notrios.install-report/1"

// Options describes what to report on. Every field exists so a test can ask
// about a temporary tree instead of this machine.
type Options struct {
	// Roots are the resolved roots, by name, as paths.Resolution gives them.
	Roots map[string]string
	Mode  string
	// RegistryPath is the profile registry. An empty path means the default
	// location; a registry that is absent is a note, not an error.
	RegistryPath string
	// InstallManifest is the installer's MANIFEST.json. An empty path means
	// "look under the data root", which is where lifecycle.py writes it.
	InstallManifest string
}

// Build walks the installation and returns its structure and manifest.
func Build(options Options) (Report, error) {
	report := Report{
		Schema: Schema,
		Mode:   options.Mode,
		Roots:  map[string]string{},
		Notes:  []string{},
	}
	for name, path := range options.Roots {
		report.Roots[name] = path
	}

	owned := make([]string, 0, len(options.Roots))
	for _, path := range options.Roots {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			if absolute, err := filepath.Abs(trimmed); err == nil {
				owned = append(owned, absolute)
			}
		}
	}

	// seen holds every path already listed, and claimed every directory already
	// walked, so a path that two roots both contain is reported once.
	seen := map[string]bool{}
	claimed := map[string]string{}

	// The roots, walked in a stable order so two runs of the same installation
	// produce the same report.
	//
	// Every path is made absolute first. A source checkout resolves relative
	// roots ("data", "data/assets"), and a manifest of relative paths is worse
	// than useless to the post-purge check, which runs from somewhere else
	// entirely -- it would test whatever happened to sit beside the caller.
	for _, name := range sortedKeys(options.Roots) {
		root := strings.TrimSpace(options.Roots[name])
		if root == "" {
			continue
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			report.Notes = append(report.Notes, fmt.Sprintf("the %s root %s could not be made absolute (%v)", name, root, err))
			continue
		}
		report.Roots[name] = absolute
		// Two roots can resolve to one directory -- an installed layout puts
		// the program's assets under the data root -- and listing its files
		// twice would inflate every count and make the post-purge check ask
		// about each path twice. The first root to claim a path keeps it, and
		// the overlap is said out loud rather than silently absorbed.
		if owner, taken := claimed[absolute]; taken {
			report.Notes = append(report.Notes, fmt.Sprintf(
				"the %s root is the %s root (%s); its contents are listed once", name, owner, absolute))
			continue
		}
		claimed[absolute] = name
		if err := walkRoot(&report, name, absolute, seen); err != nil {
			return Report{}, err
		}
	}

	registered, external, notes := registeredProfiles(options.RegistryPath, owned)
	report.Notes = append(report.Notes, notes...)
	report.Counts.Profiles = registered
	for _, entry := range external {
		report.Manifest = append(report.Manifest, entry)
		report.Counts.External++
	}

	if program, note := installedProgram(options, report.Roots); program != nil {
		report.Program = program
	} else if note != "" {
		report.Notes = append(report.Notes, note)
	}

	sortEntries(report.Structure)
	sortEntries(report.Manifest)
	report.Counts.Directories = len(report.Structure)
	for _, entry := range report.Manifest {
		if entry.Kind == KindFile {
			report.Counts.Files++
			report.Counts.Bytes += entry.Bytes
		}
	}
	return report, nil
}

// walkRoot adds one root's directories and files. A root that does not exist is
// a note rather than an error: a fresh installation has not created its cache
// yet, and saying so is more useful than refusing to report at all.
func walkRoot(report *Report, name, root string, seen map[string]bool) error {
	info, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			report.Notes = append(report.Notes, fmt.Sprintf("the %s root %s does not exist yet", name, root))
			return nil
		}
		return fmt.Errorf("read the %s root %s: %w", name, root, err)
	}
	if !info.IsDir() {
		if seen[root] {
			return nil
		}
		seen[root] = true
		report.Manifest = append(report.Manifest, Entry{Path: root, Kind: kindOf(info), Category: name, Owned: true, Bytes: sizeOf(info)})
		return nil
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A directory this user cannot read is reported, not fatal: the
			// report is most needed when something is wrong with the layout.
			report.Notes = append(report.Notes, fmt.Sprintf("%s could not be read (%v)", path, err))
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			report.Notes = append(report.Notes, fmt.Sprintf("%s could not be described (%v)", path, statErr))
			return nil
		}
		if seen[path] {
			return nil
		}
		seen[path] = true
		item := Entry{Path: path, Kind: kindOf(info), Category: name, Owned: true, Bytes: sizeOf(info)}
		if entry.IsDir() {
			item.Bytes = 0
			report.Structure = append(report.Structure, item)
			return nil
		}
		report.Manifest = append(report.Manifest, item)
		return nil
	})
}

// registeredProfiles returns how many profiles are registered and an entry for
// every path one keeps outside the owned roots.
func registeredProfiles(registryPath string, owned []string) (int, []Entry, []string) {
	notes := []string{}
	path := strings.TrimSpace(registryPath)
	if path == "" {
		resolved, err := profiles.DefaultPath()
		if err != nil {
			return 0, nil, []string{"the profile registry location could not be resolved (" + err.Error() +
				"), so this report cannot say whether a profile keeps its library elsewhere"}
		}
		path = resolved
	}
	registry, err := profiles.Load(path)
	if err != nil {
		return 0, nil, []string{"the profile registry at " + path + " could not be read (" + err.Error() +
			"), so this report cannot say whether a profile keeps its library elsewhere"}
	}
	// Load answers a missing registry with an empty one, which is the same
	// thing a registry with no profiles means here: nothing is kept elsewhere.
	if len(registry.Profiles) == 0 {
		return 0, nil, []string{"no profile is registered, so no library is kept outside these roots"}
	}

	refs := []purge.ExternalRef{}
	for _, profile := range registry.Profiles {
		for _, named := range []struct{ field, path string }{
			{"database", profile.DatabasePath},
			{"asset store", profile.AssetStore},
			{"config", profile.ConfigPath},
		} {
			if strings.TrimSpace(named.path) == "" {
				continue
			}
			refs = append(refs, purge.ExternalRef{Path: named.path, Profile: profile.Name, Field: named.field})
		}
	}

	entries := []Entry{}
	for _, ref := range purge.ExternalPaths(refs, owned) {
		path := ref.Path
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
		entry := Entry{Path: path, Category: "profile", Owned: false, Profile: ref.Profile, Field: ref.Field, Kind: KindMissing}
		if info, err := os.Lstat(path); err == nil {
			entry.Kind = kindOf(info)
			entry.Bytes = sizeOf(info)
		}
		entries = append(entries, entry)
	}
	if len(entries) > 0 {
		notes = append(notes, fmt.Sprintf("%d path(s) belong to a profile outside these roots; a purge keeps them",
			len(entries)))
	}
	return len(registry.Profiles), entries, notes
}

// installedProgram reads the installer's manifest, which records the program
// files rather than the user's data.
func installedProgram(options Options, roots map[string]string) (*Program, string) {
	path := strings.TrimSpace(options.InstallManifest)
	if path == "" {
		data := strings.TrimSpace(roots["data"])
		if data == "" {
			return nil, ""
		}
		path = filepath.Join(data, "MANIFEST.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "no install manifest at " + path + ", so no program files are listed: " +
				"this is a source checkout or a package that records them elsewhere"
		}
		return nil, "the install manifest at " + path + " could not be read (" + err.Error() + ")"
	}
	var manifest struct {
		Version     string `json:"version"`
		InstalledAt string `json:"installed_at"`
		Prefix      string `json:"prefix"`
		Entries     []struct {
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, "the install manifest at " + path + " is not readable JSON (" + err.Error() + ")"
	}
	return &Program{
		ManifestPath: path, Version: manifest.Version, InstalledAt: manifest.InstalledAt,
		Prefix: manifest.Prefix, Entries: len(manifest.Entries),
	}, ""
}

// Paths returns one absolute path per line: the shape the post-purge check
// reads, with each path marked owned or external, because the check requires
// the owned ones to be gone and the external ones to still be there.
func (r Report) Paths() []string {
	lines := make([]string, 0, len(r.Structure)+len(r.Manifest))
	for _, entry := range append(append([]Entry{}, r.Structure...), r.Manifest...) {
		state := "owned"
		if !entry.Owned {
			state = "external"
		}
		lines = append(lines, state+"\t"+entry.Path)
	}
	sort.Strings(lines)
	return lines
}

func kindOf(info os.FileInfo) string {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return KindSymlink
	case info.IsDir():
		return KindDirectory
	}
	return KindFile
}

func sizeOf(info os.FileInfo) int64 {
	if info.Mode().IsRegular() {
		return info.Size()
	}
	return 0
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
}
