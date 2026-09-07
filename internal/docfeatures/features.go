// Package docfeatures answers a question the repository's other documentation
// machinery does not: is there any page that tells a reader a capability
// exists?
//
// Every existing gate checks that documentation is consistent with itself --
// regenerated, hashed, anchored, counted. All of them stay green over a
// capability no page mentions at all. Tagging a note is the worked example:
// REST has POST and DELETE /api/v1/documents/{document_id}/tags/{tag}, MCP has
// tag_note and untag_note, and neither the CLI guide nor the GUI guide says a
// word about either. Nothing failed, because nothing was inconsistent.
//
// So this package inverts the direction. Instead of asking whether the prose
// matches the code, it asks whether every surface the code offers is claimed by
// some feature a reader could find. An unclaimed surface is a capability nobody
// can discover, which is a documentation defect even when every other gate is
// green.
package docfeatures

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Feature is one capability a person could set out to use, and the surfaces
// that offer it. The surfaces are the machine-checkable part; the title and
// summary are written by a person, because "what can I do with this?" is not
// answerable by listing flags.
type Feature struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	// CLI holds usage-form prefixes rather than whole forms, because a form
	// carries its flags and those change without the capability changing.
	CLI  []string `json:"cli,omitempty"`
	REST []string `json:"rest,omitempty"`
	MCP  []string `json:"mcp,omitempty"`
	GUI  []string `json:"gui,omitempty"`
	// Library is the shared C-ABI surface a future client links against. It is
	// empty on every feature today and is rendered as such: the library exists
	// (H1), but nothing in it is claimed as a way to perform one of these
	// capabilities, and a column of hopeful ticks would be the one thing this
	// page must never contain. Check refuses a claim here until a repository
	// inventory of that surface exists to validate it against.
	Library []string `json:"library,omitempty"`
	// SurfaceNote explains an asymmetry the author decided is deliberate. A
	// feature missing a surface without one is reported as a gap.
	SurfaceNote string `json:"surface_note,omitempty"`
}

// Registry is the feature catalogue: what a person can do with Notrios, and
// which surfaces offer each capability.
//
//notrios:doc user feature-surface
//notrios:help features what-you-can-do
//notrios:enumerates go:github.com/renesugar/notrios/internal/docfeatures#Registry
type Registry struct {
	Schema   string    `json:"schema"`
	Features []Feature `json:"features"`
}

// Everything here is command-line or API work today. Where a capability is
// absent for a reason rather than for want of doing it, the reason is given.
//
//notrios:doc user gui-absent-capabilities
//notrios:help journeys-gui what-the-gui-does-not-do
//notrios:enumerates go:github.com/renesugar/notrios/internal/docfeatures#(Registry).WithoutGUILines
func (r Registry) WithoutGUILines() []string {
	// This section used to be a paragraph somebody kept up to date by hand, and
	// it stopped being true twice in one day: it still said tags were read-only
	// in the interface after tagging was built, and still said importing was
	// command-line work after the desktop app grew an import dialog. The
	// coverage gates could not see either, because prose is not a claim they
	// check. Deriving the list from the same registry the rest of the page uses
	// means the page cannot say the interface lacks something it has.
	lines := []string{}
	for _, feature := range r.Features {
		if len(feature.GUI) > 0 {
			continue
		}
		line := fmt.Sprintf("**%s** — %s", feature.Title, feature.Summary)
		if feature.SurfaceNote != "" {
			line += " " + feature.SurfaceNote
		}
		lines = append(lines, line)
	}
	return lines
}

// Lines renders the catalogue for the generated fragment: one line per
// feature, naming the surfaces that offer it.
//
// The surfaces are listed rather than described because they are the checked
// half -- a reader who wants to know whether something is available at the
// command line should be reading a fact, not a claim someone typed. The titles
// and summaries beside them are written by a person, because "what can I do
// with this?" is not answerable by listing flags.
func (r Registry) Lines() []string {
	lines := make([]string, 0, len(r.Features))
	for _, feature := range r.Features {
		// One block per capability: a heading somebody can link to, the
		// summary as its opening sentence, and the surface note as the
		// paragraph that says where it is and why it is not everywhere.
		// Written as one value with newlines in it, which the generator
		// passes through because it begins with Markdown of its own.
		block := "### " + feature.Title + "\n\n" + feature.Summary
		if feature.SurfaceNote != "" {
			block += "\n\n" + feature.SurfaceNote
		}
		block += "\n\n" + surfaceSentence(feature)
		lines = append(lines, block, "")
	}
	return lines
}

// surfaceSentence says where a capability can be reached, in the page's own
// words rather than as a count in brackets.
//
// The counts it replaces ("CLI 3, REST 9, MCP 4, GUI 2") measured the API and
// read as a score. What a reader wants is whether the thing they are holding
// can do it, which is a list of surfaces; the exact operation counts live in
// the table further down, where a column is the right shape for a number.
func surfaceSentence(feature Feature) string {
	where := []string{}
	for _, pair := range []struct {
		name  string
		items []string
	}{
		{"the desktop app", feature.GUI},
		{"the command line", feature.CLI},
		{"the REST API", feature.REST},
		{"MCP", feature.MCP},
		{"the shared library", feature.Library},
	} {
		if len(pair.items) > 0 {
			where = append(where, pair.name)
		}
	}
	switch len(where) {
	case 0:
		return "*Not reachable from any surface yet.*"
	case 1:
		return "*Available on " + where[0] + ".*"
	default:
		return "*Available on " + strings.Join(where[:len(where)-1], ", ") + " and " + where[len(where)-1] + ".*"
	}
}

// SurfaceTable is the capability-by-adapter table: one row per capability, one
// column per surface, and a number saying how many operations that surface
// spends on it.
//
// It answers a question the prose above cannot answer quickly -- "can I do this
// from the command line?" -- and it is generated from the same registry, so it
// cannot drift from the sentences beside it. An empty cell is not an oversight;
// it is a capability that surface does not offer, and the reason is in that
// capability's own section.
//
//notrios:doc user feature-surface-table
//notrios:help features where-each-capability-lives
//notrios:enumerates go:github.com/renesugar/notrios/internal/docfeatures#(Registry).SurfaceTable
func (r Registry) SurfaceTable() []string {
	rows := []string{
		"| Capability | Desktop app | Command line | REST | MCP | Shared library |",
		"|---|---|---|---|---|---|",
	}
	for _, feature := range r.Features {
		cell := func(items []string) string {
			if len(items) == 0 {
				return "—"
			}
			return fmt.Sprintf("%d", len(items))
		}
		rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s | %s | %s |",
			feature.Title, cell(feature.GUI), cell(feature.CLI),
			cell(feature.REST), cell(feature.MCP), cell(feature.Library)))
	}
	return rows
}

// Asymmetry is a capability that some surfaces offer and others do not.
type Asymmetry struct {
	Feature string
	Has     []string
	Missing []string
	Note    string
}

type Report struct {
	Features   int
	Claimed    map[string]int
	Unclaimed  map[string][]string
	Phantom    map[string][]string
	Asymmetric []Asymmetry
}

const Schema = "notrios.docfeatures.registry.v1"

func Load(path string) (Registry, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, err
	}
	var registry Registry
	if err := json.Unmarshal(contents, &registry); err != nil {
		return Registry{}, err
	}
	if registry.Schema != Schema {
		return Registry{}, fmt.Errorf("unexpected features schema %q", registry.Schema)
	}
	return registry, nil
}

// matchesCLI is prefix matching because a usage form carries its flags. A
// feature claims `notriosctl import joplin-raw`, and the form it matches is the
// whole line including every optional argument.
func matchesCLI(claim, form string) bool {
	return strings.HasPrefix(form, claim)
}

// Check compares the registry against what the repository actually offers.
//
// It reports in both directions on purpose. An unclaimed surface is an
// undiscoverable capability. A phantom claim -- a feature naming a surface that
// no longer exists -- is a page describing something that was removed, which is
// the more embarrassing of the two and the one a consistency check cannot see
// because the prose is perfectly consistent with itself.
func Check(registry Registry, surfaces RepositorySurfaces) Report {
	report := Report{
		Features:  len(registry.Features),
		Claimed:   map[string]int{},
		Unclaimed: map[string][]string{},
		Phantom:   map[string][]string{},
	}

	// The shared library has no repository inventory to check a claim against,
	// so a claim there cannot be verified and is refused rather than believed.
	// The column exists in the table because the surface exists; the day the
	// ABI publishes what it offers, this becomes a check like the others.
	for _, feature := range registry.Features {
		if len(feature.Library) > 0 {
			report.Phantom["library"] = append(report.Phantom["library"],
				fmt.Sprintf("%s: the shared library publishes no inventory to verify a claim against", feature.ID))
		}
	}

	claimedCLI := map[string]bool{}
	for _, feature := range registry.Features {
		for _, claim := range feature.CLI {
			matched := false
			for _, form := range surfaces.CLI {
				if matchesCLI(claim, form) {
					claimedCLI[form] = true
					matched = true
				}
			}
			if !matched {
				report.Phantom["cli"] = append(report.Phantom["cli"], fmt.Sprintf("%s: %s", feature.ID, claim))
			}
		}
	}
	exact := func(kind string, claims func(Feature) []string, available []string) map[string]bool {
		set := map[string]bool{}
		for _, item := range available {
			set[item] = false
		}
		for _, feature := range registry.Features {
			for _, claim := range claims(feature) {
				if _, ok := set[claim]; !ok {
					report.Phantom[kind] = append(report.Phantom[kind], fmt.Sprintf("%s: %s", feature.ID, claim))
					continue
				}
				set[claim] = true
			}
		}
		return set
	}
	restSet := exact("rest", func(f Feature) []string { return f.REST }, surfaces.REST)
	mcpSet := exact("mcp", func(f Feature) []string { return f.MCP }, surfaces.MCP)

	for _, form := range surfaces.CLI {
		if claimedCLI[form] {
			report.Claimed["cli"]++
		} else {
			report.Unclaimed["cli"] = append(report.Unclaimed["cli"], form)
		}
	}
	for kind, set := range map[string]map[string]bool{"rest": restSet, "mcp": mcpSet} {
		for item, claimed := range set {
			if claimed {
				report.Claimed[kind]++
			} else {
				report.Unclaimed[kind] = append(report.Unclaimed[kind], item)
			}
		}
	}

	// The asymmetry report is the point of recording surfaces per feature
	// rather than merely counting them. A capability reachable through REST and
	// MCP but not the command line is exactly the tags defect, and it is a
	// finding rather than an error: the author either explains it or fixes it.
	for _, feature := range registry.Features {
		has, missing := []string{}, []string{}
		for _, pair := range []struct {
			name  string
			items []string
		}{{"cli", feature.CLI}, {"rest", feature.REST}, {"mcp", feature.MCP}, {"gui", feature.GUI}} {
			if len(pair.items) > 0 {
				has = append(has, pair.name)
			} else {
				missing = append(missing, pair.name)
			}
		}
		if len(missing) > 0 && len(has) > 0 {
			report.Asymmetric = append(report.Asymmetric, Asymmetry{
				Feature: feature.ID, Has: has, Missing: missing, Note: feature.SurfaceNote,
			})
		}
	}

	for kind := range report.Unclaimed {
		sort.Strings(report.Unclaimed[kind])
	}
	for kind := range report.Phantom {
		sort.Strings(report.Phantom[kind])
	}
	sort.Slice(report.Asymmetric, func(i, j int) bool {
		return report.Asymmetric[i].Feature < report.Asymmetric[j].Feature
	})
	return report
}

// RepositorySurfaces mirrors docgen's exported shape without importing it, so
// this package can be tested against a fixture.
type RepositorySurfaces struct {
	CLI  []string
	REST []string
	MCP  []string
	GUI  []string
}
