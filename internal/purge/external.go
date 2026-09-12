package purge

import (
	"fmt"
	"path/filepath"
	"sort"
)

// ExternalRef is one path a profile names that lies outside every root this
// installation owns.
//
// A user with several profiles can keep a library anywhere: `profile register`
// takes a database path, and nothing requires it to be under the data root. The
// oracle has always refused to delete such a path -- H3 wrote the rule and two
// of its fixtures exercise it -- but until v1.0 J3-D nothing ever *told* the
// oracle one existed, because both callers passed an empty list. The rule was
// unreachable, and the plan purge printed silently omitted the one library the
// user was most likely to care about.
type ExternalRef struct {
	// Path is the external location, already resolved.
	Path string `json:"path"`
	// Profile is the registry name that names it, for attribution: "which of my
	// profiles is this?" is the first question a user asks about such a line.
	Profile string `json:"profile"`
	// Field is which part of the profile named it -- database, asset store or
	// config -- so the answer to "why is this listed?" is in the listing.
	Field string `json:"field"`
}

// ExternalSteps turns enumerated external paths into plan steps.
//
// The action is "enumerate", which every other part of this package treats as
// inert: CreateBackup copies only backup_then_delete, and Remove deletes only
// backup_then_delete and dispose. That is the point. These paths are reported
// so a user can see them and act on them, and nothing here touches them.
//
// They are measured, because a line saying a library exists somewhere is much
// less useful than one saying how big it is.
func ExternalSteps(refs []ExternalRef, owned []string) []Step {
	sorted := make([]ExternalRef, len(refs))
	copy(sorted, refs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Field < sorted[j].Field
	})

	steps := make([]Step, 0, len(sorted))
	for _, ref := range sorted {
		files, total, keys := measureTree(ref.Path)
		reason := fmt.Sprintf("profile %q names it as its %s; outside every root this "+
			"installation owns, so purge lists it and neither deletes nor copies it",
			ref.Profile, ref.Field)

		// The ancestor case, said plainly instead of left to be inferred. A
		// profile can name a directory that *contains* the roots -- an asset
		// store set to ~/.local/share does it -- and then "not deleted" is true
		// of the directory and badly misleading about what is inside it.
		if contained := rootsInside(ref.Path, owned); len(contained) > 0 {
			reason += fmt.Sprintf("; it contains %d root(s) listed above, and those are still "+
				"deleted -- only this directory itself is left alone", len(contained))
		}

		steps = append(steps, Step{
			Category:        "external",
			Path:            ref.Path,
			Policy:          BackupPolicy("external"),
			Action:          "enumerate",
			Reason:          reason,
			Bytes:           total,
			Files:           files,
			SyncKeyMaterial: keys,
		})
	}
	return steps
}

// rootsInside reports which owned roots lie under path.
func rootsInside(path string, owned []string) []string {
	resolved := realpath(filepath.Clean(path))
	inside := []string{}
	for _, root := range owned {
		if root == "" {
			continue
		}
		if isWithin(realpath(filepath.Clean(root)), resolved) {
			inside = append(inside, root)
		}
	}
	sort.Strings(inside)
	return inside
}

// ExternalPaths picks the paths in refs that lie outside every owned root.
//
// Containment is decided with realpath in both directions, the same way the
// oracle decides it, because a path under the data root is already covered by
// the step that deletes that root -- listing it twice would tell a user their
// library is both deleted and kept.
func ExternalPaths(refs []ExternalRef, owned []string) []ExternalRef {
	resolvedOwned := make([]string, 0, len(owned))
	for _, root := range owned {
		if root == "" {
			continue
		}
		resolvedOwned = append(resolvedOwned, realpath(filepath.Clean(root)))
	}

	external := make([]ExternalRef, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.Path == "" || !filepath.IsAbs(ref.Path) {
			// A relative profile path is not resolved here on purpose. Which
			// directory it would resolve against depends on where the process
			// was started, and internal/profiles already refuses to invent that
			// answer for the registry itself.
			continue
		}
		resolved := realpath(filepath.Clean(ref.Path))
		inside := false
		for _, root := range resolvedOwned {
			if isWithin(resolved, root) {
				inside = true
				break
			}
		}
		if inside || seen[resolved] {
			continue
		}
		seen[resolved] = true
		ref.Path = resolved
		external = append(external, ref)
	}
	return external
}
