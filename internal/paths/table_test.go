package paths_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/renesugar/notrios/internal/paths"
)

// H4's plan item binds this package to one obligation above all others:
// "reproduce RESOLUTION_TABLE.json from its Go resolver". That table is H3's
// contract, generated from a Python model, and this is the test that makes the
// obligation real rather than aspirational.
//
// It compares the mode, every root, and the notice codes. It deliberately does
// not compare notice prose: Python's repr quotes with ' and Go's %q quotes with
// ", so comparing English would fail on punctuation while saying nothing about
// behaviour. That is why notices carry codes at all.

type tableScenario struct {
	Scenario    string            `json:"scenario"`
	Mode        string            `json:"mode"`
	Roots       map[string]string `json:"roots"`
	NoticeCodes []string          `json:"notice_codes"`
	Error       string            `json:"error"`
	Inputs      struct {
		Env               map[string]string `json:"env"`
		OSName            string            `json:"os_name"`
		ExecutableDir     string            `json:"executable_dir"`
		PortableMarker    bool              `json:"portable_marker"`
		SourceCheckout    bool              `json:"source_checkout"`
		Explicit          map[string]string `json:"explicit"`
		RuntimeDirPrivate string            `json:"runtime_dir_is_private"`
	} `json:"inputs"`
}

type resolutionTable struct {
	Schema    string          `json:"schema"`
	Total     int             `json:"total"`
	Failed    int             `json:"failed"`
	Scenarios []tableScenario `json:"scenarios"`
}

func loadTable(t *testing.T) resolutionTable {
	t.Helper()
	path := filepath.Join("..", "..", "performance", "v0.8-h3", "RESOLUTION_TABLE.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the H3 resolution table: %v", err)
	}
	var table resolutionTable
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatalf("parse the H3 resolution table: %v", err)
	}
	if table.Schema != "notrios.path-resolution-table/1" {
		t.Fatalf("unexpected table schema %q", table.Schema)
	}
	if table.Failed != 0 {
		t.Fatalf("the recorded table itself has %d failures", table.Failed)
	}
	return table
}

func TestReproducesH3ResolutionTable(t *testing.T) {
	table := loadTable(t)
	if len(table.Scenarios) < 15 {
		t.Fatalf("only %d scenarios; the table is too small to be worth reproducing", len(table.Scenarios))
	}

	for _, scenario := range table.Scenarios {
		t.Run(scenario.Scenario, func(t *testing.T) {
			options := paths.Options{
				Env:            scenario.Inputs.Env,
				GOOS:           scenario.Inputs.OSName,
				ExecutableDir:  scenario.Inputs.ExecutableDir,
				PortableMarker: scenario.Inputs.PortableMarker,
				SourceCheckout: scenario.Inputs.SourceCheckout,
				Explicit:       scenario.Inputs.Explicit,
			}
			if options.Env == nil {
				// A scenario with no env is "there are no variables", not
				// "read this machine's". Passing nil would silently consult the
				// developer's own HOME and make the test pass for the wrong
				// reason on their laptop and fail in CI.
				options.Env = map[string]string{}
			}
			// The model records an injected privacy predicate as "<injected>";
			// the only scenario that injects one injects a false.
			if scenario.Inputs.RuntimeDirPrivate == "<injected>" {
				options.RuntimeDirIsPrivate = func(string) bool { return false }
			}

			resolved, err := paths.Resolve(options)

			if scenario.Error != "" {
				if err == nil {
					t.Fatalf("expected a refusal like %q, got roots %v", scenario.Error, resolved.Roots)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}

			if string(resolved.Mode) != scenario.Mode {
				t.Errorf("mode = %q, table says %q", resolved.Mode, scenario.Mode)
			}
			for name, expected := range scenario.Roots {
				if actual := resolved.Root(name); actual != expected {
					t.Errorf("root %s = %q, table says %q", name, actual, expected)
				}
			}
			for name := range resolved.Roots {
				if _, ok := scenario.Roots[name]; !ok {
					t.Errorf("resolved a root %q the table does not record", name)
				}
			}
			if codes := resolved.Codes(); !reflect.DeepEqual(codes, scenario.NoticeCodes) {
				t.Errorf("notice codes = %v, table says %v", codes, scenario.NoticeCodes)
			}
		})
	}
}

// The table is only worth reproducing if it actually exercises the rules. A
// table of fifteen happy paths would be reproduced by a resolver that ignored
// its environment entirely.
func TestTableCoversTheRulesThatMatter(t *testing.T) {
	table := loadTable(t)
	seen := map[string]bool{}
	for _, scenario := range table.Scenarios {
		for _, code := range scenario.NoticeCodes {
			seen[code] = true
		}
		if scenario.Error != "" {
			seen["refusal"] = true
		}
	}
	for _, required := range []string{
		paths.NoticeXDGRelativeIgnored,
		paths.NoticeXDGIgnoredOnPlatform,
		paths.NoticeRuntimeDirUnset,
		paths.NoticeRuntimeDirNotPrivate,
		paths.NoticePortableSelected,
		paths.NoticeSourceSelected,
		paths.NoticeExplicitOverride,
		"refusal",
	} {
		if !seen[required] {
			t.Errorf("the table exercises no scenario producing %q", required)
		}
	}
}
