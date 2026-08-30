package docgen

import (
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, template, source, markdown string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"docs/docgen", "internal/fixture"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "docs/docgen/templates.json"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/fixture/source.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/page.md"), []byte(markdown), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestGenerateDeterministicAndPreservesManualBytes(t *testing.T) {
	root := fixture(t, `{"schema":"notrios.docgen.templates.v1","pages":[{"path":"docs/page.md","sections":[{"slug":"settings","slots":[{"id":"second","audience":"user"},{"id":"first","audience":"user"}]}]}]}`,
		`package fixture
// First value.
//notrios:doc user first
var First = 1
// Second value.
//notrios:doc user second
var Second = 2
`, "# Settings\nmanual before\nmanual after\n# Next\nkeep\n")
	g := Generator{}
	got, err := g.Generate(root, "user")
	if err != nil {
		t.Fatal(err)
	}
	want := "# Settings\n<!-- notrios:generated:user:settings:begin -->\n<!-- source: go:github.com/renesugar/notrios/internal/fixture#Second -->\nSecond value.\n<!-- source: go:github.com/renesugar/notrios/internal/fixture#First -->\nFirst value.\n<!-- notrios:generated:user:settings:end -->\nmanual before\nmanual after\n# Next\nkeep\n"
	if string(got["docs/page.md"]) != want {
		t.Fatalf("generated:\n%s", got["docs/page.md"])
	}
	got2, err := g.Generate(root, "user")
	if err != nil {
		t.Fatal(err)
	}
	if string(got2["docs/page.md"]) != want {
		t.Fatal("generation not stable")
	}
}

func TestAudienceSeparationAndFreshness(t *testing.T) {
	root := fixture(t, `{"schema":"notrios.docgen.templates.v1","pages":[{"path":"docs/page.md","sections":[{"slug":"settings","slots":[{"id":"u","audience":"user"},{"id":"a","audience":"api"}]}]}]}`,
		`package fixture
// User.
//notrios:doc user u
var U = 1
// API.
//notrios:doc api a
var A = 2
`, "# Settings\nmanual\n")
	g := Generator{}
	if err := g.Write(root, "user"); err != nil {
		t.Fatal(err)
	}
	if drift, err := g.Check(root, "user"); err != nil || len(drift) != 0 {
		t.Fatalf("freshness %v %v", drift, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "docs/page.md"))
	if string(b) == "" || contains(string(b), "API.") {
		t.Fatal("API leaked into user output")
	}
	b = []byte(containsReplace(string(b), "User.", "Changed."))
	os.WriteFile(filepath.Join(root, "docs/page.md"), b, 0644)
	drift, err := g.Check(root, "user")
	if err != nil || len(drift) != 1 {
		t.Fatalf("mutation drift %v %v", drift, err)
	}
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
func containsReplace(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func TestMissingSectionAndFragmentRefused(t *testing.T) {
	root := fixture(t, `{"schema":"notrios.docgen.templates.v1","pages":[{"path":"docs/page.md","sections":[{"slug":"missing","slots":[{"id":"u","audience":"user"}]}]}]}`,
		`package fixture
// User.
//notrios:doc user u
var U = 1
`, "# Settings\n")
	if _, err := (Generator{}).Generate(root, "user"); err == nil {
		t.Fatal("missing section accepted")
	}
}
