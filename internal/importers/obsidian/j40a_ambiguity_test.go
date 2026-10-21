package obsidian

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// j40aSurfaces is what each surface says about the one link a case is about:
// the stored row, the editor's buffer check, lint's two link counts, and
// whether the importer warned.
type j40aSurfaces struct {
	// kept is whether the note still holds the link exactly as written.
	kept        bool
	row         string
	buffer      string
	lintAmbig   int
	lintBroken  int
	importerSaw bool
}

func j40aObserve(t *testing.T, st *store.SQLiteStore, documentID, written, raw string, warnings []string) j40aSurfaces {
	t.Helper()
	ctx := context.Background()
	document, err := st.GetDocument(ctx, documentID)
	if err != nil {
		t.Fatalf("get %s: %v", documentID, err)
	}
	got := j40aSurfaces{kept: strings.Contains(document.Body, written)}
	page, err := st.ListDocumentLinks(ctx, documentID, "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range page.Outgoing {
		if link.RawTarget == raw {
			got.row = link.ResolutionStatus
		}
	}
	checked, err := st.CheckLinks(ctx, store.CheckLinksRequest{CollectionID: "default", DocumentID: documentID, Body: document.Body})
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range checked.Links {
		if link.RawTarget == raw {
			got.buffer = link.Status
		}
	}
	lint, err := st.LintWorkspace(ctx, store.LintRequest{CollectionID: "default", Checks: []string{store.LintAmbiguousLink, store.LintBrokenDocumentLink}})
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range lint.Checks {
		switch check.Check {
		case store.LintAmbiguousLink:
			got.lintAmbig = check.Count
		case store.LintBrokenDocumentLink:
			got.lintBroken = check.Count
		}
	}
	for _, warning := range warnings {
		if strings.HasPrefix(warning, "ambiguous Obsidian link ") && strings.Contains(warning, "\""+raw+"\"") {
			got.importerSaw = true
		}
	}
	return got
}

// TestJ40AWhatEachSurfaceSaysOfAnAmbiguousLink states, for each way a link
// becomes ambiguous, what is recorded today (v1.0 J40-A). The preview's side is
// web/src/__tests__/j40a-preview.test.tsx, and Library Health shows lint's
// counts and nothing else (library-health.test.tsx).
//
// The finding the item was planned around is confirmed, and is narrower than
// expected. The importer decides by file name and leaves an ambiguous link as
// written; the store then re-resolves that text by note title. So whether the
// row says "ambiguous" depends on how the notes sharing the name were titled:
// by their file names, yes; by a heading or front matter, as J32's collision
// corpus is, no, and the row says "unresolved". An alias the importer found
// ambiguous is never ambiguous to the store, which does not resolve aliases.
// An attachment name is ambiguous to both, because a resource keeps its file
// name. A report built only from rows would miss the vault-name and alias
// cases, which is what J40-E has to answer.
func TestJ40AWhatEachSurfaceSaysOfAnAmbiguousLink(t *testing.T) {
	cases := []struct {
		name  string
		vault map[string]string
		// store, when set, builds the library without an import.
		store func(t *testing.T, st *store.SQLiteStore) string
		from  string
		// written is the link as the note holds it, and raw its target.
		written string
		raw     string
		want    j40aSurfaces
	}{
		{
			name: "two notes share a title, written in Notrios",
			store: func(t *testing.T, st *store.SQLiteStore) string {
				ctx := context.Background()
				for _, id := range []string{"doc_shared_one", "doc_shared_two"} {
					if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: id, CollectionID: "default", Title: "Shared", Body: "# Shared\n"}); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_from", CollectionID: "default", Title: "From", Body: "# From\n\nSee [[Shared]].\n"}); err != nil {
					t.Fatal(err)
				}
				return "doc_from"
			},
			written: "[[Shared]]", raw: "Shared",
			want: j40aSurfaces{kept: true, row: "ambiguous", buffer: "ambiguous", lintAmbig: 1},
		},
		{
			name: "vault files share a name, titled by their file names",
			vault: map[string]string{
				"a/Foo.md": "Under a.\n",
				"b/Foo.md": "Under b.\n",
				"from.md":  "# From\n\nSee [[Foo]].\n",
			},
			from: "from.md", written: "[[Foo]]", raw: "Foo",
			want: j40aSurfaces{kept: true, row: "ambiguous", buffer: "ambiguous", lintAmbig: 1, importerSaw: true},
		},
		{
			name: "vault files share a name, titled by headings (J36-E, J32's collision shape)",
			vault: map[string]string{
				"a/index.md": "# Topic A\n",
				"b/index.md": "# Topic B\n",
				"from.md":    "# From\n\nSee [[index]].\n",
			},
			from: "from.md", written: "[[index]]", raw: "index",
			want: j40aSurfaces{kept: true, row: "unresolved", buffer: "unresolved", lintBroken: 1, importerSaw: true},
		},
		{
			name: "two notes claim one alias",
			vault: map[string]string{
				"one.md":  "---\naliases: [Shared]\n---\n# One\n",
				"two.md":  "---\naliases: [Shared]\n---\n# Two\n",
				"from.md": "# From\n\nSee [[Shared]].\n",
			},
			from: "from.md", written: "[[Shared]]", raw: "Shared",
			want: j40aSurfaces{kept: true, row: "unresolved", buffer: "unresolved", lintBroken: 1, importerSaw: true},
		},
		{
			name: "attachments share a file name",
			vault: map[string]string{
				"a/pic.png": "\x89PNG\r\n\x1a\na",
				"b/pic.png": "\x89PNG\r\n\x1a\nb",
				"from.md":   "# From\n\n![[pic.png]]\n",
			},
			from: "from.md", written: "![[pic.png]]", raw: "pic.png",
			want: j40aSurfaces{kept: true, row: "ambiguous", buffer: "ambiguous", lintAmbig: 1, importerSaw: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := openTestStore(t)
			var from string
			var warnings []string
			if tc.store != nil {
				from = tc.store(t, st)
			} else {
				dir := t.TempDir()
				for rel, body := range tc.vault {
					writeBytes(t, filepath.Join(dir, filepath.FromSlash(rel)), []byte(body))
				}
				report, err := Import(context.Background(), st, dir, Options{CollectionID: "default"})
				if err != nil {
					t.Fatalf("import: %v", err)
				}
				from, warnings = documentID(tc.from), report.Warnings
			}
			if got := j40aObserve(t, st, from, tc.written, tc.raw, warnings); got != tc.want {
				t.Fatalf("surfaces:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
