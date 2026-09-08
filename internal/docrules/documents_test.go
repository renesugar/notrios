package docrules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/docrules"
)

// repositoryRoot is two directories up from this package.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("locating repository root: %v", err)
	}
	return root
}

func TestEveryRootDocumentSaysWhereItsRulesAre(t *testing.T) {
	if problems := docrules.Check(repositoryRoot(t)); len(problems) > 0 {
		for _, problem := range problems {
			t.Errorf("documents: %s", problem)
		}
	}
}

func TestTheInventoryCoversEveryRootDocument(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := docrules.Load(root)
	if err != nil {
		t.Fatalf("loading the inventory: %v", err)
	}
	listed := map[string]bool{}
	for _, doc := range registry.Documents {
		listed[doc.Path] = true
	}
	for _, exempt := range registry.Exempt {
		listed[exempt.Path] = true
	}
	found, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		t.Fatalf("listing root documents: %v", err)
	}
	if len(found) != len(listed) {
		t.Errorf("the repository root has %d documents and the inventory lists %d", len(found), len(listed))
	}
}

// TestANewRootDocumentMustDeclareItself is the ratchet. Without it the rules
// apply only to the documents that existed on the day they were written, which
// is the same failure in a slower form.
func TestANewRootDocumentMustDeclareItself(t *testing.T) {
	root := t.TempDir()
	copyRepositoryDocuments(t, root)

	if problems := docrules.Check(root); len(problems) > 0 {
		t.Fatalf("the copied repository should pass before the new document: %v", problems)
	}

	write(t, filepath.Join(root, "NEW_THING.md"), "# New Thing\n\nSomething true today.\n")

	problems := docrules.Check(root)
	if len(problems) == 0 {
		t.Fatal("a root document in neither list was accepted")
	}
	if !strings.Contains(strings.Join(problems, "\n"), "NEW_THING.md") {
		t.Errorf("the failure does not name the new document: %v", problems)
	}
}

func TestADocumentThatStopsPointingAtTheRulesFails(t *testing.T) {
	root := t.TempDir()
	copyRepositoryDocuments(t, root)

	path := filepath.Join(root, "CONTEXT_MAP.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the map: %v", err)
	}
	stripped := strings.ReplaceAll(string(body), "AGENTS.md", "somewhere")
	write(t, path, stripped)

	problems := docrules.Check(root)
	if len(problems) == 0 {
		t.Fatal("a document with no pointer was accepted")
	}
	if !strings.Contains(strings.Join(problems, "\n"), "CONTEXT_MAP.md") {
		t.Errorf("the failure does not name the document: %v", problems)
	}
}

// TestPointingAtTheWrongPlaceIsNotEnough guards the half-right case: a document
// that mentions AGENTS.md but not the section its reader needs sends them to
// three hundred lines and lets them find it.
func TestPointingAtTheWrongPlaceIsNotEnough(t *testing.T) {
	root := t.TempDir()
	copyRepositoryDocuments(t, root)

	path := filepath.Join(root, "PACKAGING.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading packaging: %v", err)
	}
	vague := strings.ReplaceAll(string(body),
		"**How to keep this document current is in [`AGENTS.md`](AGENTS.md)** — under \"Keeping the reference documents current\".",
		"See `AGENTS.md`.")
	write(t, path, vague)

	problems := docrules.Check(root)
	if len(problems) == 0 {
		t.Fatal("a document naming AGENTS.md but no section was accepted")
	}
}

// TestAnAgentsSectionCannotBeRenamedAway catches the other direction: the
// documents keep their pointers and the section they point to disappears.
func TestAnAgentsSectionCannotBeRenamedAway(t *testing.T) {
	root := t.TempDir()
	copyRepositoryDocuments(t, root)

	path := filepath.Join(root, "AGENTS.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	renamed := strings.Replace(string(body),
		"## Keeping the reference documents current",
		"## Documents", 1)
	write(t, path, renamed)

	problems := docrules.Check(root)
	if len(problems) == 0 {
		t.Fatal("documents pointing at a section that no longer exists were accepted")
	}
}

func TestExemptionsMustSayWhy(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := docrules.Load(root)
	if err != nil {
		t.Fatalf("loading the inventory: %v", err)
	}
	if len(registry.Exempt) == 0 {
		t.Skip("nothing is exempt")
	}
	for _, exempt := range registry.Exempt {
		if strings.TrimSpace(exempt.Reason) == "" {
			t.Errorf("%s is exempt without a reason", exempt.Path)
		}
	}
}

// copyRepositoryDocuments reproduces the root documents and the inventory in a
// temporary directory, so a test can break one without breaking the repository.
func copyRepositoryDocuments(t *testing.T, into string) {
	t.Helper()
	root := repositoryRoot(t)

	found, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		t.Fatalf("listing root documents: %v", err)
	}
	for _, path := range found {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		write(t, filepath.Join(into, filepath.Base(path)), string(body))
	}

	registry, err := os.ReadFile(filepath.Join(root, docrules.RegistryPath))
	if err != nil {
		t.Fatalf("reading the inventory: %v", err)
	}
	target := filepath.Join(into, docrules.RegistryPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(target), err)
	}
	write(t, target, string(registry))
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// TestTheTableInAgentsIsGenerated keeps the prose honest: the section a reader
// arrives at describes twenty-five documents, and nobody retypes that list.
func TestTheTableInAgentsIsGenerated(t *testing.T) {
	if problems := docrules.CheckTable(repositoryRoot(t)); len(problems) > 0 {
		for _, problem := range problems {
			t.Errorf("documents table: %s", problem)
		}
	}
}

func TestAChangedRegistryMakesTheTableStale(t *testing.T) {
	root := t.TempDir()
	copyRepositoryDocuments(t, root)

	if problems := docrules.CheckTable(root); len(problems) > 0 {
		t.Fatalf("the copy should start current: %v", problems)
	}

	path := filepath.Join(root, docrules.RegistryPath)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the inventory: %v", err)
	}
	const home = "how a coding agent works here"
	if !strings.Contains(string(body), home) {
		t.Fatalf("the inventory no longer contains %q, so this test is checking nothing", home)
	}
	write(t, path, strings.Replace(string(body), home, "how agents work here", 1))

	if problems := docrules.CheckTable(root); len(problems) == 0 {
		t.Fatal("a registry change left the generated table looking current")
	}
}

// TestTheAtlasRecordsEveryPackage is the rule CODING_STANDARDS.md carried as a
// sentence -- "Update CONTEXT_MAP.md when adding major files or packages" --
// which nothing enforced, so eleven packages went unrecorded.
func TestTheAtlasRecordsEveryPackage(t *testing.T) {
	if problems := docrules.CheckAtlas(repositoryRoot(t)); len(problems) > 0 {
		for _, problem := range problems {
			t.Errorf("atlas: %s", problem)
		}
	}
}

func TestANewPackageMustBeRecordedInTheAtlas(t *testing.T) {
	root := t.TempDir()
	copyRepositoryDocuments(t, root)
	copyAtlasTree(t, root)

	if problems := docrules.CheckAtlas(root); len(problems) > 0 {
		t.Fatalf("the copy should start clean: %v", problems)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "newthing"), 0o755); err != nil {
		t.Fatalf("creating a package: %v", err)
	}
	problems := docrules.CheckAtlas(root)
	if len(problems) == 0 {
		t.Fatal("a package the atlas does not record was accepted")
	}
	if !strings.Contains(strings.Join(problems, "\n"), "internal/newthing") {
		t.Errorf("the failure does not name the package: %v", problems)
	}
}

// TestAPackageNameIsNotMatchedAsAWord guards the check itself. A grep for bare
// names reported one missing package where there were six, because `paths` and
// `media` are ordinary words that appear in prose.
func TestAPackageNameIsNotMatchedAsAWord(t *testing.T) {
	root := t.TempDir()
	copyRepositoryDocuments(t, root)
	copyAtlasTree(t, root)

	atlas := filepath.Join(root, "CONTEXT_MAP.md")
	body, err := os.ReadFile(atlas)
	if err != nil {
		t.Fatalf("reading the atlas: %v", err)
	}
	// Remove the entry and leave the word behind in a sentence.
	stripped := strings.ReplaceAll(string(body), "`internal/paths/`", "the paths package")
	write(t, atlas, stripped)

	problems := docrules.CheckAtlas(root)
	if len(problems) == 0 {
		t.Fatal("a package mentioned only as a word was counted as recorded")
	}
}

func TestTheAtlasRootDocumentListIsGenerated(t *testing.T) {
	if problems := docrules.CheckAtlasBlock(repositoryRoot(t)); len(problems) > 0 {
		for _, problem := range problems {
			t.Errorf("atlas list: %s", problem)
		}
	}
}

// copyAtlasTree reproduces the package directories the atlas has to account for.
func copyAtlasTree(t *testing.T, into string) {
	t.Helper()
	root := repositoryRoot(t)
	registry, err := docrules.Load(root)
	if err != nil {
		t.Fatalf("loading the inventory: %v", err)
	}
	for _, parent := range registry.Atlas.Roots {
		entries, err := os.ReadDir(filepath.Join(root, parent))
		if err != nil {
			t.Fatalf("reading %s: %v", parent, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if err := os.MkdirAll(filepath.Join(into, parent, entry.Name()), 0o755); err != nil {
				t.Fatalf("creating %s: %v", entry.Name(), err)
			}
		}
	}
}
