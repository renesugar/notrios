package docfeatures

import "testing"

func fixture() RepositorySurfaces {
	return RepositorySurfaces{
		CLI:  []string{"notriosctl lint [--db path]", "notriosctl gc [--db path]"},
		REST: []string{"GET /api/v1/documents", "POST /api/v1/documents"},
		MCP:  []string{"list_notes", "create_note"},
		GUI:  []string{"Open a search result — executed (search-open)"},
	}
}

// TestUnclaimedSurfacesAreReported is the check nothing else performs: a
// capability no feature names is one no reader can discover.
func TestUnclaimedSurfacesAreReported(t *testing.T) {
	registry := Registry{Schema: Schema, Features: []Feature{{
		ID: "read-notes", Title: "Read notes", Summary: "List and open notes.",
		CLI: []string{"notriosctl lint"}, REST: []string{"GET /api/v1/documents"},
		MCP: []string{"list_notes"},
	}}}
	report := Check(registry, fixture())
	for kind, want := range map[string]string{
		"cli": "notriosctl gc [--db path]", "rest": "POST /api/v1/documents", "mcp": "create_note",
	} {
		found := false
		for _, item := range report.Unclaimed[kind] {
			if item == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s surface %q was not reported as unclaimed: %v", kind, want, report.Unclaimed[kind])
		}
	}
	if report.Claimed["rest"] != 1 {
		t.Errorf("claimed rest = %d, want 1", report.Claimed["rest"])
	}
}

// TestPhantomClaimsAreReported catches the opposite drift: a page describing a
// capability that no longer exists. A consistency check cannot see it, because
// the prose is perfectly consistent with itself.
func TestPhantomClaimsAreReported(t *testing.T) {
	registry := Registry{Schema: Schema, Features: []Feature{{
		ID: "gone", Title: "Removed", Summary: "Describes something deleted.",
		REST: []string{"GET /api/v1/removed"}, MCP: []string{"removed_tool"},
		CLI: []string{"notriosctl removed"},
	}}}
	report := Check(registry, fixture())
	for _, kind := range []string{"cli", "rest", "mcp"} {
		if len(report.Phantom[kind]) == 0 {
			t.Errorf("a claim on a nonexistent %s surface was not reported", kind)
		}
	}
}

// TestAsymmetryIsReportedNotFailed is the tags case in miniature. A capability
// reachable through REST and MCP but not the command line is a finding for an
// author to explain or fix, not an error the build should refuse.
func TestAsymmetryIsReportedNotFailed(t *testing.T) {
	registry := Registry{Schema: Schema, Features: []Feature{{
		ID: "tag-a-note", Title: "Tag a note", Summary: "Attach and remove tags.",
		REST: []string{"GET /api/v1/documents"}, MCP: []string{"list_notes"},
	}}}
	report := Check(registry, fixture())
	if len(report.Asymmetric) != 1 {
		t.Fatalf("want one asymmetry, got %v", report.Asymmetric)
	}
	asymmetry := report.Asymmetric[0]
	if asymmetry.Feature != "tag-a-note" {
		t.Fatalf("wrong feature reported: %v", asymmetry)
	}
	missing := map[string]bool{}
	for _, name := range asymmetry.Missing {
		missing[name] = true
	}
	if !missing["cli"] || !missing["gui"] {
		t.Fatalf("the missing surfaces must name cli and gui: %v", asymmetry.Missing)
	}
}

// TestCLIClaimsMatchByPrefix keeps the registry from carrying a copy of every
// flag, which would make it a second usage message that drifts.
func TestCLIClaimsMatchByPrefix(t *testing.T) {
	registry := Registry{Schema: Schema, Features: []Feature{{
		ID: "maintenance", Title: "Maintenance", Summary: "Tidy a library.",
		CLI: []string{"notriosctl lint", "notriosctl gc"},
	}}}
	report := Check(registry, fixture())
	if len(report.Unclaimed["cli"]) != 0 {
		t.Fatalf("prefix claims did not match the full usage forms: %v", report.Unclaimed["cli"])
	}
}
