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
	// 24 -> 23 in v0.8 H14 slice F: the migrate-credentials journey, added
	// because the pilot showed a guessable command cannot measure a page.
	// 23 -> 22 in v0.8 H14: the tagging journey, which could not be written
	// until the command line could tag a note.
	// 22 -> 21 in v0.8 H15: the Obsidian import journey.
	// 21 -> 19 in v0.8 H15: snapshots and sync-exchange gained journeys.
	// 19 -> 18 in v0.8 H15: the notebook journey.
	// 18 -> 17 in v0.8 H22: finding the notebooks and collections a query can
	// name. Collections had no journey because the command line could not list
	// them, which is the shape of most of what is left on this list.
	// 17 -> 16 in v0.8 H19: finding a note and acting on the id the search
	// returned. Searching had no journey because it had no command.
	//
	// 16 -> 6 in v0.8 H15-H: ten journeys, for reading a note, tags, templates
	// and tasks, the graph, links, saved searches, jobs, the pre-0.8 migration,
	// remote media and publishing.
	//
	// The six that remain are not a backlog of the same kind, and the
	// difference is worth writing down so nobody works on the wrong ones:
	//
	//   - query-blocks, batch-operations and mcp-endpoint have no command line
	//     at all, so no command-line journey can exist for them;
	//   - attachments has only the reading half -- resources report, resources
	//     get, notes resources -- because attaching a file is REST, MCP or the
	//     GUI, so a journey could only demonstrate reading nothing;
	//   - sync-peers and sync-recovery need two replicas, and the journey
	//     runner offers one `{db}`. That is a limit of the harness rather than
	//     of the commands, and it is the one worth lifting if this number is to
	//     fall again.
	const baseline = 6

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

// TestGUIScreenshotsMatchTheirSteps runs without a browser, which is the point:
// the expensive capture is opt-in, but a stale or missing screenshot fails an
// ordinary test run.
//
// A picture that no longer matches the interface is worse than no picture. It
// is a confident claim, and nothing about a stale PNG announces itself.
func TestGUIScreenshotsMatchTheirSteps(t *testing.T) {
	root := filepath.Join("..", "..")
	catalogue, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
	if err != nil {
		t.Fatalf("loading the GUI journey catalogue: %v", err)
	}
	manifest, err := docjourneys.LoadImages(filepath.Join(root, "docs", "images", "journeys", "MANIFEST.json"))
	if err != nil {
		t.Fatalf("loading the screenshot manifest: %v", err)
	}
	for _, problem := range docjourneys.VerifyImages(filepath.Join(root, "docs"), catalogue, manifest) {
		// Two runners, two commands: a browser journey is photographed by
		// Playwright, and a desktop-only one -- import, export, snapshots --
		// by the harness that drives the real application under Xvfb.
		t.Errorf("%s\n\nregenerate with: NOTRIOS_GUI_JOURNEYS=1 go test ./cmd/notriosctl -run TestGUIJourneyCapture\n"+
			"or, for a desktop-driven journey: NOTRIOS_GUI_DESKTOP_RUN=1 go test ./cmd/notriosctl -run TestDesktopJourneyCapture", problem)
	}
}

// TestEveryGUIJourneyNamesARealFeature is the same tie as the command-line
// catalogue has, for the same reason.
func TestEveryGUIJourneyNamesARealFeature(t *testing.T) {
	root := filepath.Join("..", "..")
	catalogue, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, feature := range registry.Features {
		known[feature.ID] = true
	}
	for _, journey := range catalogue.Journeys {
		if !known[journey.Feature] {
			t.Errorf("GUI journey %q names feature %q, which is not in the registry", journey.ID, journey.Feature)
		}
	}
}

// TestTaggingGainsAGUIJourneyWhenTheInterfaceCanTag is a standing instruction
// rather than a note to remember.
//
// The interface cannot add or remove a tag today, so there is no interface
// journey for it and there should not be one -- a journey for something the
// surface cannot do would be a lie with pictures. When that changes, whoever
// adds the capability will record a `gui` surface on the feature, and this
// fails until the journey exists to match.
//
// It is written this way because "add the journey later" is the kind of
// intention that survives in a plan and not in a repository.
func TestTaggingGainsAGUIJourneyWhenTheInterfaceCanTag(t *testing.T) {
	root := filepath.Join("..", "..")
	registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalogue, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, feature := range registry.Features {
		if feature.ID != "tag-a-note" {
			continue
		}
		if len(feature.GUI) == 0 {
			return // still command-line only; nothing to document in the interface
		}
		for _, journey := range catalogue.Journeys {
			if journey.Feature == "tag-a-note" {
				return
			}
		}
		t.Fatal("the interface can tag a note now, so docs/journeys-gui.md needs a journey for it; " +
			"add one to GUI_JOURNEYS.json and capture it with NOTRIOS_GUI_JOURNEYS=1")
	}
}
