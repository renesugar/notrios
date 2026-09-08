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
	// 212 -> 214 manual sections, 137 -> 139 executables and 373 -> 377
	// denominator in v0.8 H5: docs/installation.md replaced one hand-rolled
	// "optional local installation" section with three describing make install,
	// uninstall and purge -- so three arrive and one retires, not three net.
	// Three examples retired with the old section and five arrived.
	// 214 -> 215 manual sections, 139 -> 141 executables and 377 -> 380
	// denominator in v0.8 H6: docs/installation.md gained "Installing from a
	// package" with two examples.
	// 215 -> 216 manual sections, 141 -> 142 executables and 380 -> 382
	// denominator in v0.8 H9 slice D: docs/cli.md gained "Where the key
	// material is kept" with one example.
	// 216 -> 220 manual sections and 15 -> 16 fragments in v0.8 H14 slice B:
	// docs/features.md, four sections, one generated feature list.
	// 220 -> 224 and 16 -> 17 in v0.8 H14 slice C: docs/journeys-cli.md.
	// 224 -> 228 and 17 -> 18 in v0.8 H14 slice D: docs/journeys-gui.md and
	// its regenerate example.
	// 228 -> 229 and 18 -> 19 in v0.8 H14 slice E: the computed surface
	// comparison, one section on docs/features.md.
	// 229 -> 230 sections and 143 -> 142 executables when the journey pages
	// were rewritten as user documentation: the meta sections describing
	// hashes and regenerate commands went, taking one example with them.
	// 19 -> 20 fragments in v0.8 H15: "What the GUI does not do" on
	// docs/journeys-gui.md became generated. It had been a hand-kept paragraph
	// and it was wrong twice -- still calling tags read-only after tagging was
	// built, still calling importing command-line work after the desktop app
	// grew an import dialog. Neither was caught, because prose is not a claim
	// any gate checks; deriving the list from the registry is what makes it
	// one. The section count does not move: the section is still there, and now
	// carries a fragment.
	// 230 -> 231 sections in v0.8 H15: docs/gui.md gained "Saving, and what
	// happens to unsaved changes". Prose, with no example and no fragment --
	// the behaviour it describes is covered by the web suite and by the three
	// desktop tests that drive the real close dialog, neither of which this
	// page can register as a runnable example.
	// 231 -> 261 sections and 20 -> 21 fragments in v0.8 H18: docs/features.md
	// stopped being a bullet list. The capability catalogue renders one `###`
	// section per capability -- twenty-nine of them -- plus one hand-written
	// section introducing the new generated surface table, which is the extra
	// fragment. The twenty-nine are graded `generated` rather than
	// `unverified`, because a heading a generator emitted is the fragment's
	// claim counted twice, not prose somebody has to go and check.
	// 261 -> 262 sections and 142 -> 144 executables in v0.8 H22: the
	// collections section in docs/cli.md and the discovery block in
	// docs/query-language.md. The first is a bracketed-flag synopsis and joins
	// the illustrative set; the second is literal and runs.
	// 262 -> 263 sections and 144 -> 147 executables in v0.8 H24: the
	// "Reading JSON output" section and its three gomplate pipelines.
	// 263 -> 264 sections and 147 -> 148 executables in v0.8 H21: the
	// note-reading section in docs/cli.md and its synopsis.
	// 264 -> 265 sections and 148 -> 150 executables in v0.8 H26: the tags show
	// section in docs/cli.md and its two examples.
	if report.ManualSections != 265 || report.Fragments != 21 || report.Claims != 4 ||
		report.Executables != 150 || report.Journeys != 9 {
		t.Fatalf("unexpected coverage surface: %+v", report)
	}
	// 71 -> 72 executed in v0.8 H22: the two discovery commands in
	// docs/query-language.md, which are literal and run.
	// 72 -> 73 executed in v0.8 H26: the shell existence check.
	if report.Counts[GradeExecuted] != 73 ||
		// 272 -> 276 unverified in v0.8 H4 slice D: two new docs/cli.md
		// sections and their two registered synopsis examples.
		// 276 -> 285 unverified in v0.8 H4 slice E: five new sections and four
		// new registered examples, every one of them unverified prose or an
		// example that changes this user's state and so is covered by
		// sandboxed Go tests instead.
		// 285 -> 286 unverified in v0.8 H4a: the new section is prose.
		// 286 -> 287 unverified in v0.8 H4b: the new section is prose.
		// 287 -> 291 unverified in v0.8 H5: three new prose sections and five
		// registered examples, less the three that retired with the old one.
		// Each new example either installs into this user's home or deletes
		// their library, so all five are reviewed unrun reasons covered by
		// executed tests in scripts/test_lifecycle.py.
		// 294 -> 296 unverified in v0.8 H9 slice D: one new prose section and
		// one registered example. The example moves key material into this
		// user's real credential store, so it carries a reviewed
		// shared-user-state reason and is executed against a sandboxed library
		// by cmd/notriosctl TestMigrateCredentialsRoundTrip instead.
		// 296 -> 300 unverified and 382 -> 387 denominator in v0.8 H14 slice B:
		// four new prose sections and one generated fragment.
		// 15 -> 16 generated and 400 -> 401 denominator in v0.8 H15: the
		// generated "What the GUI does not do" list. One unit arrives and none
		// leaves, because the prose it replaced was inside a section that is
		// still counted.
		// 16 -> 46 generated in v0.8 H18: the twenty-nine capability sections
		// the features page now renders, plus the surface table's fragment.
		report.Counts[GradeGenerated] != 46 ||
		// 310 -> 311 unverified and 401 -> 402 denominator in v0.8 H15: the new
		// prose section on docs/gui.md. One unit arrives and none leaves.
		// 311 -> 312 unverified and 402 -> 433 denominator in v0.8 H18: one new
		// hand-written section, and thirty new units of which twenty-nine are
		// generated.
		// 312 -> 314 unverified and 433 -> 436 denominator in v0.8 H22: two new
		// hand-written sections (the collections command in docs/cli.md and the
		// discovery note in docs/query-language.md) and one more executed
		// example, which is 71 -> 72 executed.
		// 314 -> 318 unverified and 436 -> 440 denominator in v0.8 H24: the new
		// section and its three examples, none of them run here because
		// gomplate is not a dependency of this repository.
		// 318 -> 320 unverified and 440 -> 442 denominator in v0.8 H21.
		// 320 -> 322 unverified and 442 -> 445 denominator in v0.8 H26.
		report.Counts[GradeClaimed] != 4 || report.Counts[GradeUnverified] != 322 ||
		report.Denominator != 445 {
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
