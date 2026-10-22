package obsidian

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/markdownlinks"
	"github.com/renesugar/notrios/internal/store"
)

// j40eVault holds every way a note link can be ambiguous to the import, and
// links that are not: a name shared by file names under headings, an alias two
// notes claim, a name one note has as its file name and another as its title,
// a name a note's own folder settles, a path, and an attachment name, which is
// the store's to find and not this namespace's.
var j40eVault = map[string]string{
	"a/index.md":    "# Topic A\n\nUp to [[index]].\n",
	"b/index.md":    "# Topic B\n",
	"one.md":        "---\naliases: [Shared]\n---\n# One\n",
	"two.md":        "---\naliases: [Shared]\n---\n# Two\n",
	"y/plan.md":     "# Plan\n",
	"x/notes.md":    "# plan\n",
	"from.md":       "# From\n\n[[index]], [[Shared|s]], [[plan#Head]], [[a/index]], [[one]], ![[pic.png]], [m](index.md).\n",
	"a/pic.png":     "\x89PNG\r\n\x1a\na",
	"b/pic.png":     "\x89PNG\r\n\x1a\nb",
	"other/solo.md": "# Solo\n\nSee [[two]] and [[missing]].\n",
}

func j40eImport(t *testing.T) (*store.SQLiteStore, string, Report) {
	t.Helper()
	st := openTestStore(t)
	dir := t.TempDir()
	for rel, body := range j40eVault {
		writeBytes(t, filepath.Join(dir, filepath.FromSlash(rel)), []byte(body))
	}
	report, err := Import(context.Background(), st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return st, dir, report
}

// j40eNamespace rebuilds the namespace from the library alone.
func j40eNamespace(t *testing.T, st *store.SQLiteStore) Namespace {
	t.Helper()
	ctx := context.Background()
	ids := []string{}
	for rel := range j40eVault {
		if strings.HasSuffix(rel, ".md") {
			ids = append(ids, documentID(rel))
		}
	}
	sources, err := st.GetDocumentSources(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	notes := []ProvenanceNote{}
	for _, id := range ids {
		document, err := st.GetDocument(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		note, ok := ProvenanceNoteFrom(sources[id], document.Title)
		if !ok {
			t.Fatalf("%s has no Obsidian provenance: %+v", id, sources[id])
		}
		notes = append(notes, note)
	}
	return NamespaceFromProvenance(notes)
}

// TestJ40EProvenanceAgreesWithTheImport: for every link in every note, the
// namespace rebuilt from provenance calls a link ambiguous exactly when the
// import warned that it was, and names the notes the import's namespace held
// for it. Attachment names are the store's, and are left to it.
func TestJ40EProvenanceAgreesWithTheImport(t *testing.T) {
	st, _, report := j40eImport(t)
	ns := j40eNamespace(t, st)

	warned := map[string]bool{}
	for _, warning := range report.Warnings {
		warned[warning] = true
	}
	derived := map[string][]string{}
	for rel, body := range j40eVault {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		for _, candidate := range markdownlinks.Extract(body) {
			raw := strings.TrimSpace(candidate.RawTarget)
			key := fmt.Sprintf("ambiguous Obsidian link %q in %s", raw, rel)
			var matches []string
			if ambiguity, ok := ns.Ambiguity(rel, raw); ok {
				matches = ambiguity.Notes
				derived[key] = matches
			}
			if isNote := looksLikeMarkdownNote(raw); isNote && warned[key] != (matches != nil) {
				t.Errorf("%s: the import warned %v, provenance found %v", key, warned[key], matches)
			}
		}
	}

	want := map[string][]string{
		`ambiguous Obsidian link "index" in from.md`:    {documentID("a/index.md"), documentID("b/index.md")},
		`ambiguous Obsidian link "Shared" in from.md`:   {documentID("one.md"), documentID("two.md")},
		`ambiguous Obsidian link "index.md" in from.md`: {documentID("a/index.md"), documentID("b/index.md")},
		`ambiguous Obsidian link "plan" in from.md`:     {documentID("x/notes.md"), documentID("y/plan.md")},
	}
	for _, ids := range want {
		sort.Strings(ids)
	}
	if !reflect.DeepEqual(derived, want) {
		t.Fatalf("derived candidates:\n got %v\nwant %v", derived, want)
	}
	// The attachment name was ambiguous to the import, and is not this
	// namespace's to say: the store's own row says so.
	if !warned[`ambiguous Obsidian link "pic.png" in from.md`] {
		t.Fatalf("the import should have warned about the attachment name: %v", report.Warnings)
	}
}

// TestJ40EAFolderSettlesItsOwnName: a note links [[index]] from a folder that
// holds an index.md, and the import resolves it to that one. So does the
// rebuilt namespace, which is the vault-relative rule and not a name lookup.
func TestJ40EAFolderSettlesItsOwnName(t *testing.T) {
	st, _, _ := j40eImport(t)
	ns := j40eNamespace(t, st)
	if got, ok := ns.Ambiguity("a/index.md", "index"); ok {
		t.Fatalf("a/index.md's own [[index]] resolves to itself, not to %v", got)
	}
	if got, ok := ns.Ambiguity("from.md", "a/index"); ok {
		t.Fatalf("a path names one note, not %v", got)
	}
	// Every link that names the same thing is told the same name, which is
	// what lets a report count a shared list once.
	first, _ := ns.Ambiguity("from.md", "index")
	second, _ := ns.Ambiguity("other/solo.md", "Index.md")
	if first.Name != "index" || second.Name != first.Name {
		t.Fatalf("one name, two keys: %q and %q", first.Name, second.Name)
	}
}
