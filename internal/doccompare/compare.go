// Package doccompare puts the command line and the interface side by side and
// reports where they disagree.
//
// The comparison is computed rather than written, because a hand-maintained
// list of differences is a list that stops being true. It draws on three things
// that already exist: which surfaces each capability claims, which capabilities
// have a command-line journey, and which have an interface journey.
//
// Two kinds of disagreement come out, and they are not the same problem.
//
// A *capability* difference is a thing one surface can do and the other cannot.
// Some of those are deliberate -- importing reads directories on this machine,
// so it is command line only -- and the registry records the reason beside them.
// One that carries no reason is a gap rather than a design.
//
// A *documentation* difference is a capability both surfaces offer where only
// one has been written down. Nothing is missing from the product; something is
// missing from the pages, which is a smaller failure and a longer job.
//
// This package reports. It does not decide which differences ought to be
// closed, because that is a product question and this is a documentation item.
package doccompare

import (
	"fmt"
	"sort"

	"github.com/renesugar/notrios/internal/docfeatures"
	"github.com/renesugar/notrios/internal/docjourneys"
)

type Kind string

const (
	// CapabilityOnlyCLI and CapabilityOnlyGUI: one surface offers it, the other
	// does not.
	CapabilityOnlyCLI Kind = "capability-only-cli"
	CapabilityOnlyGUI Kind = "capability-only-gui"
	// CapabilityNeither: reachable over REST or MCP and from neither surface a
	// person uses. This is the shape of the tags gap, and the most serious of
	// the three: the capability exists and nobody can reach it without writing
	// a program.
	CapabilityNeither Kind = "capability-neither"
	// DocumentedOnlyCLI and DocumentedOnlyGUI: both surfaces offer it, one has
	// a journey.
	DocumentedOnlyCLI Kind = "documented-only-cli"
	DocumentedOnlyGUI Kind = "documented-only-gui"
	// DocumentedNeither: both surfaces offer it and neither has a journey.
	DocumentedNeither Kind = "documented-neither"
)

type Difference struct {
	Feature string
	Title   string
	Kind    Kind
	// Explanation is the registry's surface note. Empty means the difference is
	// unexplained, which is what separates a decision from a gap.
	Explanation string
}

func (d Difference) Explained() bool { return d.Explanation != "" }

// Compare reports every difference between the two surfaces.
func Compare(registry docfeatures.Registry, cli docjourneys.Catalogue, gui docjourneys.GUICatalogue) []Difference {
	cliJourneys, guiJourneys := map[string]bool{}, map[string]bool{}
	for _, journey := range cli.Journeys {
		cliJourneys[journey.Feature] = true
	}
	for _, journey := range gui.Journeys {
		guiJourneys[journey.Feature] = true
	}

	differences := []Difference{}
	for _, feature := range registry.Features {
		hasCLI, hasGUI := len(feature.CLI) > 0, len(feature.GUI) > 0
		difference := Difference{Feature: feature.ID, Title: feature.Title, Explanation: feature.SurfaceNote}
		switch {
		case hasCLI && !hasGUI:
			difference.Kind = CapabilityOnlyCLI
		case !hasCLI && hasGUI:
			difference.Kind = CapabilityOnlyGUI
		case !hasCLI && !hasGUI:
			difference.Kind = CapabilityNeither
		default:
			// Both surfaces offer it, so any difference left is about the
			// documentation rather than the product.
			switch {
			case cliJourneys[feature.ID] && !guiJourneys[feature.ID]:
				difference.Kind = DocumentedOnlyCLI
			case !cliJourneys[feature.ID] && guiJourneys[feature.ID]:
				difference.Kind = DocumentedOnlyGUI
			case !cliJourneys[feature.ID] && !guiJourneys[feature.ID]:
				difference.Kind = DocumentedNeither
			default:
				continue // documented on both sides; nothing to report
			}
			// A surface note explains a capability difference, not a missing
			// journey. Carrying it here would make an undocumented capability
			// look accounted for.
			difference.Explanation = ""
		}
		differences = append(differences, difference)
	}
	sort.Slice(differences, func(i, j int) bool {
		if differences[i].Kind != differences[j].Kind {
			return differences[i].Kind < differences[j].Kind
		}
		return differences[i].Feature < differences[j].Feature
	})
	return differences
}

// Unexplained is every capability difference with no reason recorded beside it.
// These are the findings this item exists to surface: a capability one surface
// has and the other does not, that nobody has said is deliberate.
func Unexplained(differences []Difference) []Difference {
	gaps := []Difference{}
	for _, difference := range differences {
		switch difference.Kind {
		case CapabilityOnlyCLI, CapabilityOnlyGUI, CapabilityNeither:
			if !difference.Explained() {
				gaps = append(gaps, difference)
			}
		}
	}
	return gaps
}

var descriptions = map[Kind]string{
	CapabilityOnlyCLI: "command line only",
	CapabilityOnlyGUI: "interface only",
	CapabilityNeither: "neither the command line nor the interface; reachable only over REST or MCP",
	DocumentedOnlyCLI: "both surfaces, but only the command-line journey is written",
	DocumentedOnlyGUI: "both surfaces, but only the interface journey is written",
	DocumentedNeither: "both surfaces, and neither journey is written yet",
}

// Lines renders the comparison for the generated fragment.
//
//notrios:doc user surface-comparison
//notrios:help features where-the-surfaces-disagree
//notrios:enumerates go:github.com/renesugar/notrios/internal/doccompare#Difference
func Lines(differences []Difference) []string {
	lines := make([]string, 0, len(differences))
	for _, difference := range differences {
		line := fmt.Sprintf("%s — %s", difference.Title, descriptions[difference.Kind])
		if difference.Explained() {
			line += ". " + difference.Explanation
		} else if difference.Kind == CapabilityOnlyCLI ||
			difference.Kind == CapabilityOnlyGUI || difference.Kind == CapabilityNeither {
			line += ". **No reason is recorded for this, so it is a gap rather than a decision.**"
		}
		lines = append(lines, line)
	}
	return lines
}
