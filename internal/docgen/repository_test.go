package docgen

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestRepositoryEnumerationsMatchFiniteRegistries(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
	resolver := RepositoryResolver(root)
	tests := []struct {
		anchor string
		count  int
	}{
		// 59 -> 62 and 49 -> 52 in v0.8 H4 slice C: data.state_dir,
		// data.cache_dir and data.runtime_dir separate what a purge and a
		// backup treat differently.
		{configAnchor, 62},
		{defaultAnchor, 52},
		// 56 -> 58 in v0.8 H4 slice D: `notriosctl paths` and
		// `notriosctl config show`.
		// 58 -> 59 in v0.8 H4 slice E: `notriosctl migrate`.
		{cliHelpAnchor, 59},
		{restAnchor, 109},
		{mcpToolsAnchor, 46},
		{mcpSyncScopeAnchor, 7},
		{mcpScopesAnchor, 4},
		{mcpToolScopeAnchor, 46},
		{guiJourneyAnchor, 37},
	}
	for _, test := range tests {
		values, err := resolver(test.anchor)
		if err != nil {
			t.Fatalf("%s: %v", test.anchor, err)
		}
		if len(values) != test.count {
			t.Errorf("%s count = %d, want %d", test.anchor, len(values), test.count)
		}
		for index := 1; index < len(values); index++ {
			if values[index] == values[index-1] {
				t.Errorf("%s duplicate value %q", test.anchor, values[index])
			}
		}
	}
	if _, err := resolver("go:github.com/renesugar/notrios/internal/missing#Registry"); err == nil {
		t.Fatal("unknown enumeration anchor was accepted")
	}
}

// TestDocsAreCurrent is the deterministic CI gate required by G18f. It
// regenerates both audiences in memory and compares them with the committed
// Markdown consumed by the documentation site and Help seeder.
func TestDocsAreCurrent(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
	generator := Generator{Resolver: RepositoryResolver(root)}
	for _, audience := range []string{"user", "api"} {
		drift, err := generator.Check(root, audience)
		if err != nil {
			t.Fatalf("%s generation: %v", audience, err)
		}
		if len(drift) != 0 {
			t.Fatalf("%s documentation is stale: %v; run go run ./cmd/docgen --user --api", audience, drift)
		}
	}
}
