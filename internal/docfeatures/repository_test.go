// External test package: docgen now imports docfeatures to render the
// generated fragment, so an in-package test importing docgen would be a cycle.
package docfeatures_test

import (
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/docfeatures"
	"github.com/renesugar/notrios/internal/docgen"
)

// unclaimedBaseline is a ratchet, not a target.
//
// The registry starts almost empty, so nearly every surface is unclaimed and a
// gate that demanded zero would be red from the day it was written and would be
// switched off within a week. A ratchet is the honest alternative: the backlog
// is allowed to exist and is not allowed to grow. A new REST operation or CLI
// command added without a feature entry pushes a count above its baseline and
// fails here, which is the property that matters -- the documentation gap stops
// getting worse while it is being worked down.
//
// Every number here should only ever move down. Lowering one is the point of
// the work; raising one needs a reason written beside it.
var unclaimedBaseline = map[string]int{
	"cli":  0,
	"rest": 0,
	"mcp":  0,
}

func repositoryReport(t *testing.T) docfeatures.Report {
	t.Helper()
	root := filepath.Join("..", "..")
	surfaces, err := docgen.Surfaces(root)
	if err != nil {
		t.Fatalf("reading the repository surfaces: %v", err)
	}
	registry, err := docfeatures.Load(filepath.Join(root, "docs", "docfeatures", "FEATURES.json"))
	if err != nil {
		t.Fatalf("loading the features registry: %v", err)
	}
	return docfeatures.Check(registry, docfeatures.RepositorySurfaces(surfaces))
}

func TestUnclaimedSurfacesDoNotGrow(t *testing.T) {
	report := repositoryReport(t)
	for kind, baseline := range unclaimedBaseline {
		count := len(report.Unclaimed[kind])
		if count > baseline {
			t.Errorf("%d %s surfaces are claimed by no feature, up from %d; "+
				"a new capability needs an entry in docs/docfeatures/FEATURES.json.\nunclaimed: %v",
				count, kind, baseline, report.Unclaimed[kind])
		}
		if count < baseline {
			t.Errorf("only %d %s surfaces are now unclaimed, down from %d; "+
				"lower the baseline in unclaimedBaseline to lock the improvement in", count, kind, baseline)
		}
	}
}

// TestNoFeatureClaimsASurfaceThatIsGone is the other direction, and it is
// absolute rather than ratcheted: a page describing a capability that no longer
// exists is a defect from the moment it happens, and there is no backlog of
// them to work down.
func TestNoFeatureClaimsASurfaceThatIsGone(t *testing.T) {
	report := repositoryReport(t)
	for kind, phantom := range report.Phantom {
		if len(phantom) > 0 {
			t.Errorf("features claim %s surfaces that do not exist: %v", kind, phantom)
		}
	}
}

// TestTheTagsAsymmetryIsStillReported is the positive control this whole item
// turns on. Tagging a note was reachable through REST and MCP and through
// neither the command line nor the GUI, and it was found by reading the
// documentation by hand. Half of it is now closed and half is not. If this stops being reported, either the gap was
// closed -- in which case delete this test and say so -- or the check stopped
// working, which is the case worth failing over.
func TestTheTagsAsymmetryIsStillReported(t *testing.T) {
	report := repositoryReport(t)
	for _, asymmetry := range report.Asymmetric {
		if asymmetry.Feature != "tag-a-note" {
			continue
		}
		missing := map[string]bool{}
		for _, name := range asymmetry.Missing {
			missing[name] = true
		}
		// Was cli and gui. v0.8 H14 added `tags add`, `tags remove` and
		// `tags list`, so the command-line half is closed and this now guards
		// the half that is not: the GUI still shows tags as navigation only.
		// The test is narrowed rather than deleted, because the remaining gap
		// is the same gap and deserves the same guard.
		if missing["cli"] {
			t.Fatalf("the command line can tag a note now; this guard should no longer expect it missing")
		}
		if !missing["gui"] {
			t.Fatalf("tagging is no longer missing from the GUI: %v", asymmetry.Missing)
		}
		if asymmetry.Note != "" {
			t.Logf("the asymmetry now carries a note, so it is documented rather than a gap: %s", asymmetry.Note)
		}
		return
	}
	t.Fatal("the tags asymmetry is no longer reported; either it was fixed or the check stopped working")
}
