// External test package so this can import docfeatures and docgen without
// creating a cycle through the generated-fragment resolver.
package docjourneys_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/renesugar/notrios/internal/docfeatures"
	"github.com/renesugar/notrios/internal/docjourneys"
)

func load(t *testing.T) (docjourneys.Catalogue, docfeatures.Registry) {
	t.Helper()
	root := filepath.Join("..", "..")
	catalogue, err := docjourneys.Load(filepath.Join(root, "docs", "docjourneys", "CLI_JOURNEYS.json"))
	if err != nil {
		t.Fatalf("loading the journey catalogue: %v", err)
	}
	registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
	if err != nil {
		t.Fatalf("loading the features registry: %v", err)
	}
	return catalogue, registry
}

// TestEveryJourneyNamesARealFeature ties the two catalogues together. A journey
// for a capability the features page does not list would be a task nobody could
// find their way to, and a typo in the link would silently orphan it.
func TestEveryJourneyNamesARealFeature(t *testing.T) {
	catalogue, registry := load(t)
	known := map[string]bool{}
	for _, feature := range registry.Features {
		known[feature.ID] = true
	}
	for _, journey := range catalogue.Journeys {
		if !known[journey.Feature] {
			t.Errorf("journey %q names feature %q, which is not in the registry", journey.ID, journey.Feature)
		}
	}
}

// TestFeaturesWithoutAJourneyAreTracked is a ratchet in the same spirit as the
// surface coverage one: most features have no command-line journey yet, and
// that backlog may not grow.
//
// It is worth separating from the surface check because the two say different
// things. A surface with no feature is a capability nobody can discover at all.
// A feature with no journey is a capability someone can find but has not been
// shown how to use, which is a milder failure and a longer job.
func TestFeaturesWithoutAJourneyAreTracked(t *testing.T) {
	const baseline = 24

	catalogue, registry := load(t)
	covered := map[string]bool{}
	for _, journey := range catalogue.Journeys {
		covered[journey.Feature] = true
	}
	uncovered := []string{}
	for _, feature := range registry.Features {
		if !covered[feature.ID] {
			uncovered = append(uncovered, feature.ID)
		}
	}
	sort.Strings(uncovered)
	switch {
	case len(uncovered) > baseline:
		t.Errorf("%d features have no command-line journey, up from %d: %v",
			len(uncovered), baseline, uncovered)
	case len(uncovered) < baseline:
		t.Errorf("only %d features now lack a journey, down from %d; lower the baseline to lock it in",
			len(uncovered), baseline)
	}
}
