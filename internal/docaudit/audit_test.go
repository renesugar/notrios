package docaudit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fixtureClaimAnchor = "go:github.com/renesugar/notrios/internal/fixture#TestSurfaceClaim"

func TestRepositoryAuditReportsHonestCoverage(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
	var resolvedTS []string
	report, err := Audit(Options{
		Root: root, InventoryPath: "performance/v0.7-g18a/INVENTORY.json",
		RegistryPath: "docs/docaudit/registry.json", TemplatePath: "docs/docgen/templates.json",
		TSResolver: func(_ string, anchors []string) error {
			resolvedTS = append(resolvedTS, anchors...)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 203 manual sections: v0.8 H2b added "Links that leave Notrios" and H2
	// added "Diagrams" to docs/gui.md. Both count as unverified prose, which is
	// the honest grade — the desktop hand-off was proven as far as xdg-open but
	// not observed in a running GUI, and diagram rendering is verified in
	// Chromium but not in the Wails webview.
	// 203 -> 205 manual sections and 358 -> 362 denominator in v0.8 H4 slice D:
	// the `paths` and `config show` sections in docs/cli.md, each contributing a
	// section and a registered example.
	// 205 -> 210 manual sections, 133 -> 137 executables and 362 -> 371
	// denominator in v0.8 H4 slice E: docs/cli.md gained `migrate` and its two
	// subsections, docs/installation.md gained "Upgrading from before 0.8", and
	// docs/troubleshooting.md gained "Finding your notes". Four examples are
	// registered against them, and one uninstall example in
	// docs/installation.md shifted index when a new block was added above it.
	// 210 -> 211 manual sections and 371 -> 372 denominator in v0.8 H4a: the
	// "two default addresses" section in docs/service.md, which is the one
	// place both defaults are stated together.
	// 211 -> 212 manual sections and 372 -> 373 denominator in v0.8 H4b: the
	// "schema migrations and their backup" section in docs/service.md.
	if report.ManualSections != 212 || report.Fragments != 15 || report.Claims != 4 ||
		report.Executables != 137 || report.Journeys != 9 {
		t.Fatalf("unexpected coverage surface: %+v", report)
	}
	if report.Counts[GradeExecuted] != 71 || report.Counts[GradeGenerated] != 11 ||
		// 272 -> 276 unverified in v0.8 H4 slice D: two new docs/cli.md
		// sections and their two registered synopsis examples.
		// 276 -> 285 unverified in v0.8 H4 slice E: five new sections and four
		// new registered examples, every one of them unverified prose or an
		// example that changes this user's state and so is covered by
		// sandboxed Go tests instead.
		// 285 -> 286 unverified in v0.8 H4a: the new section is prose.
		// 286 -> 287 unverified in v0.8 H4b: the new section is prose.
		report.Counts[GradeClaimed] != 4 || report.Counts[GradeUnverified] != 287 ||
		report.Denominator != 373 {
		t.Fatalf("coverage counts hide or lose units: counts=%v denominator=%d", report.Counts, report.Denominator)
	}
	if len(resolvedTS) == 0 {
		t.Fatal("repository audit did not send GUI owners through the TypeScript resolver")
	}
	if len(report.Surfaces) != 8 || report.Surfaces[4].ID != "openapi" ||
		report.Surfaces[4].OperationIDs == nil || *report.Surfaces[4].OperationIDs != 0 {
		t.Fatalf("frozen finite surfaces or zero REST operation IDs disappeared: %+v", report.Surfaces)
	}
}

func TestAuditMutationFailures(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T, root string, registry *Registry, inventory *Inventory)
		message string
	}{
		{"unknown audience", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			replaceFile(t, root, "internal/fixture/surface.go", "doc user fixture-surface", "doc public fixture-surface")
		}, "unknown audience"},
		{"mixed user API", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			replaceFile(t, root, "internal/fixture/surface.go", "//notrios:help", "//notrios:doc api second-fragment\n//notrios:help")
		}, "mixed user/API"},
		{"duplicate fragment", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			writeFile(t, root, "internal/fixture/duplicate.go", "package fixture\n\n// Duplicate.\n//notrios:doc user fixture-surface\n//notrios:help service configuration-reference\nfunc Duplicate() {}\n")
		}, "duplicate fragment"},
		{"unattached directive", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			writeFile(t, root, "internal/fixture/unattached.go", "package fixture\n\n//notrios:doc user unattached-fragment\n")
		}, "not attached to a named declaration"},
		{"invalid directive", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			replaceFile(t, root, "internal/fixture/surface.go", "//notrios:help", "//notrios:unknown")
		}, "invalid notrios directive"},
		{"missing topic", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			replaceFile(t, root, "internal/fixture/surface.go", "service configuration-reference", "missing nowhere")
		}, "missing topic/section"},
		{"dangling claim", func(_ *testing.T, _ string, registry *Registry, _ *Inventory) { registry.Claims = nil }, "dangling claim"},
		{"duplicate registered claim", func(_ *testing.T, _ string, registry *Registry, _ *Inventory) {
			registry.Claims = append(registry.Claims, registry.Claims[0])
		}, "duplicate or invalid registered claim"},
		{"duplicate claim", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			writeFile(t, root, "internal/fixture/duplicate.go", "package fixture\n\n// Duplicate claim.\n//notrios:doc user second-fragment\n//notrios:help service configuration-reference\n//notrios:claim fixture-surface-check "+fixtureClaimAnchor+"\nfunc Duplicate() {}\n")
		}, "duplicate claim"},
		{"orphan registered check", func(_ *testing.T, _ string, registry *Registry, _ *Inventory) {
			registry.Claims = append(registry.Claims, RegisteredClaim{ID: "orphan-check", CheckAnchor: fixtureClaimAnchor})
		}, "orphan registered check"},
		{"missing enumerated declaration", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			replaceFile(t, root, "internal/fixture/surface.go", "internal/fixture#Surface", "internal/fixture#Missing")
		}, "resolved 0 declarations"},
		{"deleted source anchor", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			replaceFile(t, root, "internal/fixture/surface.go", "const Surface", "const RenamedSurface")
		}, "resolved 0 declarations"},
		{"deleted registered check", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			if err := os.Remove(filepath.Join(root, "internal/fixture/surface_test.go")); err != nil {
				t.Fatal(err)
			}
		}, "resolved 0 declarations"},
		{"unaccounted executable", func(t *testing.T, root string, _ *Registry, _ *Inventory) {
			replaceFile(t, root, "docs/service.md", "Body.\n", "Body.\n\n```sh\nnotriosctl version\n```\n")
		}, "unaccounted executable example"},
		{"duplicate registered executable", func(_ *testing.T, _ string, registry *Registry, _ *Inventory) {
			reason := &ExampleUnrun{Code: "illustrative-placeholder", Detail: "fixture"}
			registry.Executables = []RegisteredExample{{ID: "duplicate-example", Path: "docs/service.md", Section: "configuration-reference", Language: "sh", SHA256: "unused", State: GradeUnverified, Unrun: reason}, {ID: "duplicate-example", Path: "docs/service.md", Section: "configuration-reference", Language: "sh", SHA256: "unused", State: GradeUnverified, Unrun: reason}}
		}, "duplicate registered executable"},
		{"orphan registered executable", func(_ *testing.T, _ string, registry *Registry, _ *Inventory) {
			registry.Executables = append(registry.Executables, RegisteredExample{ID: "orphan-example", Path: "docs/service.md", Section: "configuration-reference", Language: "sh", SHA256: "unused", State: GradeUnverified, Unrun: &ExampleUnrun{Code: "illustrative-placeholder", Detail: "fixture"}})
		}, "orphan registered executable"},
		{"unaccounted journey", func(_ *testing.T, _ string, _ *Registry, inventory *Inventory) {
			inventory.Surfaces = []InventorySurface{{ID: "gui_journeys", Journeys: []InventoryJourney{{ID: "missing-journey", Owner: "ts:web/src/App.tsx#App", ProposedActions: 1}}}}
		}, "unaccounted executable journey"},
		{"duplicate registered journey", func(_ *testing.T, _ string, registry *Registry, _ *Inventory) {
			journey := RegisteredJourney{ID: "duplicate-journey", Owner: "ts:web/src/App.tsx#App", Path: "docs/service.md", Section: "configuration-reference", ProposedActions: 1, State: GradeUnverified}
			registry.Journeys = []RegisteredJourney{journey, journey}
		}, "duplicate registered journey"},
		{"orphan registered journey", func(_ *testing.T, _ string, registry *Registry, _ *Inventory) {
			registry.Journeys = append(registry.Journeys, RegisteredJourney{ID: "orphan-journey", Owner: "ts:web/src/App.tsx#App", Path: "docs/service.md", Section: "configuration-reference", ProposedActions: 1, State: GradeUnverified})
		}, "orphan registered journey"},
		{"unknown source-symbol kind", func(t *testing.T, root string, registry *Registry, _ *Inventory) {
			registry.Claims[0].CheckAnchor = "file:surface_test.go:1"
			replaceFile(t, root, "internal/fixture/surface.go", fixtureClaimAnchor, "file:surface_test.go:1")
		}, "unknown source-symbol"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, registry, inventory := fixtureRepository(t, true)
			test.mutate(t, root, &registry, &inventory)
			writeJSON(t, root, "registry.json", registry)
			writeJSON(t, root, "inventory.json", inventory)
			_, err := Audit(Options{Root: root, InventoryPath: "inventory.json", RegistryPath: "registry.json"})
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error = %v, want substring %q", err, test.message)
			}
		})
	}
}

func TestValidateExampleExecutionRejectsVacuousContracts(t *testing.T) {
	valid := RegisteredExample{ID: "example", Execution: &ExampleExecution{
		Surface: "cli", Fixture: "empty-library", Case: "cli-version",
		Expected:      ExampleExpected{Kind: "exit", Status: 0},
		Postcondition: ExamplePostcondition{Kind: "version-output", Detail: "reports a non-empty version"},
	}}
	mutations := []struct {
		name string
		edit func(*RegisteredExample)
	}{
		{"missing execution", func(value *RegisteredExample) { value.Execution = nil }},
		{"unknown surface", func(value *RegisteredExample) { value.Execution.Surface = "shell" }},
		{"missing fixture", func(value *RegisteredExample) { value.Execution.Fixture = "" }},
		{"missing case", func(value *RegisteredExample) { value.Execution.Case = "" }},
		{"unknown expected kind", func(value *RegisteredExample) { value.Execution.Expected.Kind = "stdout" }},
		{"missing postcondition", func(value *RegisteredExample) { value.Execution.Postcondition.Detail = "" }},
		{"duplicate substitution", func(value *RegisteredExample) {
			value.Execution.Substitutions = []ExampleSubstitution{{Token: "<id>", Source: "seed.note"}, {Token: "<id>", Source: "seed.other"}}
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			value := valid
			copyExecution := *valid.Execution
			value.Execution = &copyExecution
			mutation.edit(&value)
			if err := validateExampleExecution(value); err == nil {
				t.Fatal("mutated execution contract passed")
			}
		})
	}
}

func TestKnownDriftRemainsVisibleWhenAClaimAnchorIsAdded(t *testing.T) {
	root, registry, inventory := fixtureRepository(t, false)
	writeJSON(t, root, "registry.json", registry)
	writeJSON(t, root, "inventory.json", inventory)
	report, err := Audit(Options{Root: root, InventoryPath: "inventory.json", RegistryPath: "registry.json"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts[GradeUnverified] != 1 || report.Counts[GradeClaimed] != 0 {
		t.Fatalf("known drift was not exposed as unverified: %v", report.Counts)
	}

	root, registry, inventory = fixtureRepository(t, true)
	writeJSON(t, root, "registry.json", registry)
	writeJSON(t, root, "inventory.json", inventory)
	report, err = Audit(Options{Root: root, InventoryPath: "inventory.json", RegistryPath: "registry.json"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts[GradeUnverified] != 1 || report.Counts[GradeGenerated] != 1 {
		t.Fatalf("claim anchor hid the still-authoritative manual: %v", report.Counts)
	}
}

func fixtureRepository(t *testing.T, anchored bool) (string, Registry, Inventory) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "docs/service.md", "# Service\n\n## Configuration reference\n\nThe latest migration is schema v20. Body.\n")
	writeFile(t, root, "go.mod", "module github.com/renesugar/notrios\n\ngo 1.25.0\n")
	registry := Registry{Schema: RegistrySchema}
	if anchored {
		writeFile(t, root, "internal/fixture/surface.go", "package fixture\n\n// Surface is the canonical schema registry.\n//notrios:doc user fixture-surface\n//notrios:help service configuration-reference\n//notrios:enumerates go:github.com/renesugar/notrios/internal/fixture#Surface\n//notrios:claim fixture-surface-check "+fixtureClaimAnchor+"\nconst Surface = 27\n")
		writeFile(t, root, "internal/fixture/surface_test.go", "package fixture\n\nimport \"testing\"\n\nfunc TestSurfaceClaim(t *testing.T) { if Surface != 27 { t.Fatal(Surface) } }\n")
		registry.Claims = []RegisteredClaim{{ID: "fixture-surface-check", CheckAnchor: fixtureClaimAnchor}}
	}
	var inventory Inventory
	inventory.Schema = "fixture"
	inventory.Documents = []InventoryDocument{{Path: "docs/service.md", Sections: []InventorySection{{ID: "configuration-reference", Title: "Configuration reference"}}}}
	inventory.GradeBaseline.Denominator = 1
	return root, registry, inventory
}

func writeFile(t *testing.T, root, path, contents string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func replaceFile(t *testing.T, root, path, old, replacement string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(path))
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(contents), old, replacement, 1)
	if updated == string(contents) {
		t.Fatalf("mutation target %q not found in %s", old, path)
	}
	if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, root, path string, value any) {
	t.Helper()
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, path, string(encoded)+"\n")
}
