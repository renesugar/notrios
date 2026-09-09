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
	//   - attachments was in this list for the same reason, and v0.8 H27 took it
	//     out: `resources add` puts local bytes in and prints the resource://
	//     URI, and `notes append` places it, so the journey is real work rather
	//     than a demonstration of reading nothing;
	//   - sync-peers and sync-recovery are blocked on the harness, though not
	//     for the reason first recorded here. Two libraries are expressible:
	//     Substitute replaces anywhere in an argument and `{sandbox}` is
	//     offered, so a second library at `{sandbox}/second/notes.sqlite`
	//     works. The obstacle is the pairing ceremony -- `sync invite` prints a
	//     one-use code carried deliberately out of band, and the runner
	//     captures only `document_id` from a step, so nothing later can spend
	//     it. Letting a step name a value to capture from its JSON is the lift.
	//
	// Only that last one is a limitation of the tooling. query-blocks and
	// mcp-endpoint are boundaries, and the plan says why for each.
	// 6 -> 5 in v0.8 H27: attaching a file from the command line.
	// 5 -> 4 in v0.8 H17: acting on many notes at once. It was in the first
	// group -- no command line at all -- because a batch takes a list of
	// identifiers and nothing at a terminal produced one. H19 built `search`,
	// so a query names the set instead, and the entry moved from "cannot be
	// written" to written. Two of the three in that group were absent adapters
	// wearing a boundary's clothes; the same reading is worth giving the third.
	const baseline = 4

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

// TestEveryGUIFeatureHasAJourney is the invariant H15-G leaves behind.
//
// It was a backlog until v0.8 H15-G: twelve capabilities the interface offered
// and nobody had been shown how to use, tracked by a ratchet like the
// command-line one above. The backlog is empty, so it stops being a number that
// may not grow and becomes a rule -- a capability the interface can perform and
// nobody has documented is now a failure rather than an entry on a list.
//
// The way out, for a surface that genuinely should not have a journey, is to
// stop claiming it in FEATURES.json. That is the honest lever: this test
// compares what a capability says it offers against what has been demonstrated,
// so the two can only disagree deliberately.
func TestEveryGUIFeatureHasAJourney(t *testing.T) {
	root := filepath.Join("..", "..")
	registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalogue, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, journey := range catalogue.Journeys {
		covered[journey.Feature] = true
	}
	uncovered := []string{}
	for _, feature := range registry.Features {
		if len(feature.GUI) > 0 && !covered[feature.ID] {
			uncovered = append(uncovered, feature.ID)
		}
	}
	sort.Strings(uncovered)
	if len(uncovered) > 0 {
		t.Errorf("these capabilities claim an interface surface and have no interface journey: %v.\n"+
			"Add one to docs/docjourneys/GUI_JOURNEYS.json and capture it with "+
			"NOTRIOS_GUI_JOURNEYS=1 go test ./cmd/notriosctl -run TestGUIJourneyCapture", uncovered)
	}
}

// TestTaggingGainsAGUIJourneyWhenTheInterfaceCanTag is a standing instruction
// rather than a note to remember.
//
// It is kept although the test above now subsumes it. That one asks whether a
// claimed surface has a journey; this one asks whether tagging has *claimed* a
// surface yet, which is a question about the product rather than about the
// catalogue, and it was written before the interface could tag at all.
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
