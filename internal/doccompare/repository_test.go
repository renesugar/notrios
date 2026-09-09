package doccompare_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/renesugar/notrios/internal/doccompare"
	"github.com/renesugar/notrios/internal/docfeatures"
	"github.com/renesugar/notrios/internal/docjourneys"
)

func differences(t *testing.T) []doccompare.Difference {
	t.Helper()
	root := filepath.Join("..", "..")
	registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
	if err != nil {
		t.Fatal(err)
	}
	cli, err := docjourneys.Load(filepath.Join(root, "docs", "docjourneys", "CLI_JOURNEYS.json"))
	if err != nil {
		t.Fatal(err)
	}
	gui, err := docjourneys.LoadGUI(filepath.Join(root, "docs", "docjourneys", "GUI_JOURNEYS.json"))
	if err != nil {
		t.Fatal(err)
	}
	return doccompare.Compare(registry, cli, gui)
}

// TestOnlyKnownCapabilityGapsAreUnexplained is the finding this item was
// written to produce, turned into a check.
//
// A capability one surface has and the other does not is fine when someone has
// said why -- importing reads directories on this machine, profiles are about
// this machine, credential migration is forbidden a REST surface. What is not
// fine is one nobody has accounted for, and there is exactly one of those.
//
// Tagging a note is reachable over REST and MCP and from neither the command
// line nor the interface. It was found by reading pages by hand; it is now
// found by comparing two catalogues. If this list changes, either a gap was
// closed -- lower it and say so -- or a new one arrived, which is the case
// worth failing over.
func TestOnlyKnownCapabilityGapsAreUnexplained(t *testing.T) {
	// Was ["tag-a-note"] until v0.8 H14 added `tags add`, `tags remove` and
	// `tags list`, which closed the command-line half of the gap this item
	// opened with. The GUI half is open and is now an *explained* asymmetry --
	// the feature says so -- rather than an unaccounted one, which is the
	// distinction this check exists to keep.
	//
	// Empty is the strongest state this can be in: every capability that one
	// surface has and another lacks now carries a written reason.
	want := []string{}

	got := []string{}
	for _, difference := range doccompare.Unexplained(differences(t)) {
		got = append(got, difference.Feature)
	}
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("unexplained capability gaps are %v, want %v; a surface a capability does not reach "+
			"needs a surface_note in FEATURES.json saying why, or it is a product gap", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexplained capability gaps are %v, want %v", got, want)
		}
	}
}

// TestEveryDifferenceIsClassified guards the comparison itself: a difference
// with no kind would be a row in a report that says nothing.
func TestEveryDifferenceIsClassified(t *testing.T) {
	for _, difference := range differences(t) {
		if difference.Kind == "" || difference.Feature == "" || difference.Title == "" {
			t.Errorf("unclassified difference: %+v", difference)
		}
	}
}
