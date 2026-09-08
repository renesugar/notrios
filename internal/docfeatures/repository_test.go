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
// v0.8 H23 did not move these numbers and did change what they mean. The
// command-line inventory came from the printHelp literal, so it counted 69
// forms and could not see eight commands that existed; it now comes from
// internal/clispec, counts 79, and is checked against the dispatcher. The
// thirteen surfaces that became visible were claimed rather than allowed to
// raise the baseline.
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

// The tags asymmetry guard was deleted on 2026-09-03, and this note is what it
// asked for.
//
// It watched the capability v0.8 H14 opened with: tagging a note was reachable
// over REST and MCP and from neither surface a person uses. Its own comment
// said that if it stopped reporting, either the gap had been closed -- in which
// case delete it and say so -- or the check had broken. The gap was closed:
// `tags add`, `tags remove` and `tags list` on the command line, and a tag
// control in the editor toolbar.
//
// Nothing is left unguarded by removing it. doccompare's unexplained-gap list
// is pinned empty, so a capability that one surface has and another lacks
// cannot appear again without a written reason or a failing test.
