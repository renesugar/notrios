package docrules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Atlas says which document records where code lives, and which directories it
// has to account for.
type Atlas struct {
	Document string      `json:"document"`
	Roots    []string    `json:"roots"`
	Exempt   []Exemption `json:"exempt"`
}

// CheckAtlas reports packages the codebase has and the atlas does not.
//
// CODING_STANDARDS.md carried this as a sentence -- "Update CONTEXT_MAP.md when
// adding major files or packages" -- and nothing enforced it, so eleven
// packages went unrecorded, including every documentation gate that now fails
// builds. A rule in a document nobody consults while adding a package is a rule
// that describes an intention.
//
// It checks packages rather than "major files or directories" because a package
// is a durable, enumerable thing and "major" is a judgement nobody can automate.
// That is a narrower promise than the sentence made, and it is one that can be
// kept.
func CheckAtlas(root string) []string {
	registry, err := Load(root)
	if err != nil {
		return []string{err.Error()}
	}
	if strings.TrimSpace(registry.Atlas.Document) == "" {
		return []string{RegistryPath + ": no atlas document is named"}
	}

	contents, err := os.ReadFile(filepath.Join(root, registry.Atlas.Document))
	if err != nil {
		return []string{fmt.Sprintf("cannot read %s: %v", registry.Atlas.Document, err)}
	}
	atlas := string(contents)

	exempt := map[string]string{}
	problems := []string{}
	for _, entry := range registry.Atlas.Exempt {
		if strings.TrimSpace(entry.Reason) == "" {
			problems = append(problems, fmt.Sprintf("%s is exempt from the atlas without a reason", entry.Path))
		}
		exempt[entry.Path] = entry.Reason
	}

	missing := []string{}
	found := map[string]bool{}
	for _, parent := range registry.Atlas.Roots {
		entries, err := os.ReadDir(filepath.Join(root, parent))
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read %s: %v", parent, err))
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := parent + "/" + entry.Name()
			found[path] = true
			if _, ok := exempt[path]; ok {
				continue
			}
			// Matched as a backticked path rather than as a bare name, so a
			// package called `store` is not counted as recorded by every
			// sentence that happens to use the word -- which is how a grep for
			// bare names reported one missing package where there were six.
			//
			// A prefix match, so a directory that only holds other packages is
			// recorded by them: `internal/importers/joplinraw/` accounts for
			// `internal/importers`.
			if !strings.Contains(atlas, "`"+path) {
				missing = append(missing, path)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		problems = append(problems, fmt.Sprintf(
			"%d packages are in the tree and not in %s: %v\nadd an entry saying what each is for, "+
				"or exempt it with a reason in %s", len(missing), registry.Atlas.Document, missing, RegistryPath))
	}

	// An exemption for a package that no longer exists is an exemption nobody
	// will notice has stopped applying.
	for _, entry := range registry.Atlas.Exempt {
		if !found[entry.Path] {
			problems = append(problems, fmt.Sprintf(
				"%s is exempt from the atlas and is not in the tree", entry.Path))
		}
	}
	sort.Strings(problems)
	return problems
}
