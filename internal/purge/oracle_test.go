package purge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The Go oracle must answer exactly what H3's reference oracle answers.
//
// Two implementations of a deletion rule is the worst possible duplication, so
// this does not restate the expected verdicts: it reads them out of the
// fixtures the Python oracle generates, and drives the same cases by the same
// names. A rule that changes on one side without the other fails here, and a
// case added to the Python suite that this file does not cover fails too.
type fixtureCase struct {
	Case     string `json:"case"`
	Expected string `json:"expected"`
}

func loadFixtures(t *testing.T) map[string]string {
	t.Helper()
	path := filepath.Join("..", "..", "performance", "v0.8-h3", "PURGE_ORACLE_FIXTURES.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read H3 fixtures: %v", err)
	}
	var document struct {
		Cases []fixtureCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decode H3 fixtures: %v", err)
	}
	expected := map[string]string{}
	for _, c := range document.Cases {
		expected[c.Case] = c.Expected
	}
	if len(expected) == 0 {
		t.Fatal("the H3 fixtures carry no cases")
	}
	return expected
}

func TestOracleAgreesWithTheH3Reference(t *testing.T) {
	expected := loadFixtures(t)
	covered := map[string]bool{}

	check := func(name string, got Decision) {
		t.Helper()
		covered[name] = true
		want, ok := expected[name]
		if !ok {
			t.Fatalf("%s: no such case in the H3 fixtures", name)
		}
		if actual := fmt.Sprintf("%s/%s", got.Verdict, got.Rule); actual != want {
			t.Errorf("%s: this oracle says %s, H3's reference says %s", name, actual, want)
		}
	}

	sandbox := t.TempDir()
	home := filepath.Join(sandbox, "home", "user")
	state := filepath.Join(home, ".local", "state", "notrios")
	cache := filepath.Join(home, ".cache", "notrios")
	outside := filepath.Join(sandbox, "elsewhere")
	externalParent := filepath.Join(sandbox, "mounted-volume")
	external := filepath.Join(externalParent, "external-library")
	for _, path := range []string{state, cache, outside, external} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	env := Environment{OwnedRoots: []string{state, cache}, Home: home,
		ExternalProfilePaths: []string{external}}

	// The refusals that matter most.
	check("empty target", Decide("", env))
	check("whitespace target", Decide("   ", env))
	check("relative target", Decide("data/notes", env))
	check("filesystem root", Decide("/", env))
	check("system root /usr", Decide("/usr", env))
	check("system root /etc", Decide("/etc", env))
	check("home itself", Decide(home, env))
	check("ancestor of home", Decide(filepath.Dir(home), env))
	// Concatenated rather than filepath.Join'd: Join *cleans* its result, so it
	// would present an already-normalized path and the normal-form rule would
	// never be reached. Python's os.path.join does not clean, which is what the
	// reference passes -- and the difference silently tested the wrong rule
	// until the fixture comparison caught it.
	check("dot-dot traversal", Decide(state+"/../../..", env))
	check("outside every owned root", Decide(outside, env))
	check("external profile path", Decide(external, env))
	check("parent of an external profile path", Decide(externalParent, env))
	check("no owned roots declared",
		Decide(filepath.Join(state, "spool"), Environment{Home: home}))

	// Symlinks.
	link := filepath.Join(state, "link-to-outside")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	check("symlink out of an owned root", Decide(link, env))

	inner := filepath.Join(state, "spool")
	if err := os.MkdirAll(inner, 0o700); err != nil {
		t.Fatal(err)
	}
	innerLink := filepath.Join(state, "link-to-inside")
	if err := os.Symlink(inner, innerLink); err != nil {
		t.Fatal(err)
	}
	check("symlink within an owned root", Decide(innerLink, env))

	rootAlias := filepath.Join(state, "alias-to-root")
	if err := os.Symlink(state, rootAlias); err != nil {
		t.Fatal(err)
	}
	check("symlink onto the owned root", Decide(rootAlias, env))

	// The allowances.
	check("directory inside an owned root", Decide(inner, env))
	check("the owned root itself", Decide(cache, env))

	absent := filepath.Join(cache, "search-index")
	check("path that is not there", Decide(absent, env))

	// Idempotency: remove it, ask again, get the same non-error answer.
	if err := os.MkdirAll(absent, 0o700); err != nil {
		t.Fatal(err)
	}
	first := Decide(absent, env)
	if err := os.RemoveAll(absent); err != nil {
		t.Fatal(err)
	}
	second := Decide(absent, env)
	check("repeated purge, first pass", first)
	check("repeated purge, second pass", second)

	// Mount boundary, modelled: a test cannot mount a filesystem, so the
	// device lookup is injected exactly as the reference injects it.
	crossing := Environment{OwnedRoots: []string{state}, Home: home,
		DeviceOf: func(path string) (uint64, error) {
			if path == state {
				return 1, nil
			}
			return 2, nil
		}}
	check("target on another filesystem", Decide(inner, crossing))

	// Backup policy, by the same names the fixtures use.
	for _, category := range []string{"config", "data", "state", "cache", "runtime",
		"program_assets", "external", "a-category-nobody-classified"} {
		name := "backup policy for " + category
		covered[name] = true
		if want, ok := expected[name]; !ok {
			t.Fatalf("%s: no such case in the H3 fixtures", name)
		} else if got := BackupPolicy(category); got != want {
			t.Errorf("%s: this oracle says %s, H3's reference says %s", name, got, want)
		}
	}

	// Every fixture case must be exercised here. A case added to the reference
	// that this file does not drive would otherwise pass by not being asked.
	for name := range expected {
		if !covered[name] {
			t.Errorf("H3 fixture case %q is not covered by this port", name)
		}
	}
}
