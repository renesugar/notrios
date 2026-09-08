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
		// 62 -> 63 and 52 -> 53 in v0.8 H9 slice B:
		// sync.rest.credential_store names where the secret protecting the
		// key material lives.
		{configAnchor, 63},
		{defaultAnchor, 53},
		// 56 -> 58 in v0.8 H4 slice D: `notriosctl paths` and
		// `notriosctl config show`.
		// 58 -> 59 in v0.8 H4 slice E: `notriosctl migrate`.
		// 59 -> 60 in v0.8 H14 slice B: `notriosctl sync migrate-credentials`,
		// which H9 slice D added to printSyncUsage and docs/cli.md but not to
		// this registry, so `notriosctl --help` never mentioned it. The
		// features coverage work found it.
		// 60 -> 61 in v0.8 H14: `notriosctl notes create`, added because the
		// journey catalogue could not document writing a note without it.
		// 61 -> 64 in v0.8 H14: `tags add`, `tags remove` and `tags list`,
		// which close the command-line half of the tagging gap this milestone
		// opened with.
		// 64 -> 66 in v0.8 H15: `notebooks create` and `notebooks list`, added
		// once it was clear the command line's inability to make a notebook was
		// a missing adapter rather than a boundary.
		// 66 -> 69 in v0.8 H15: `tasks list`, `templates list` and `templates
		// create`. Only the last is a write. Nothing creates a task, because
		// writing `- [ ]` into a note is how one comes to exist and
		// `notes create` already does that; what was missing was asking what
		// remains, and instantiating the template that shapes the note.
		// 69 -> 79 in v0.8 H23. Eight commands existed and were named in no
		// help text -- `notes show`, `notes edit`, `notes delete`,
		// `notes restore`, `sync handshake`, `sync retention`, `sync retire`
		// and `sync start` -- and two combined lines (`sync init|status`,
		// `publish profile list|delete`) hid two more behind a pipe. The
		// inventory now comes from internal/clispec and is checked against the
		// dispatcher, so this count measures the command line rather than a
		// string literal. The anchor still names printHelp because that is
		// where a reader arrives; printHelp renders the registry.
		{cliHelpAnchor, 79},
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
