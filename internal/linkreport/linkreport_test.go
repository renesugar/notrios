package linkreport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/importers/obsidian"
	"github.com/renesugar/notrios/internal/store"
)

// The library J40-A's cases come from, in one: an Obsidian vault whose names
// the import found ambiguous by file name and by alias, an attachment name, a
// title two notes written in Notrios share, a link that names nothing, and the
// same kinds of link in places the report does not measure.
var vault = map[string]string{
	"a/index.md": "# Topic A\n",
	"b/index.md": "# Topic B\n",
	"one.md":     "---\naliases: [Shared]\n---\n# One\n",
	"two.md":     "---\naliases: [Shared]\n---\n# Two\n",
	"from.md":    "# From\n\nSee [[index|the index]] and [[Shared]], ![[pic.png]], [[nowhere]].\n",
	"also.md":    "# Also\n\nAnd [[index]] again.\n",
	"a/pic.png":  "\x89PNG\r\n\x1a\na",
	"b/pic.png":  "\x89PNG\r\n\x1a\nb",
}

type fixture struct {
	st  *store.SQLiteStore
	ids map[string]string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "store.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "vault")
	for rel, body := range vault {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := obsidian.Import(ctx, st, dir, obsidian.Options{CollectionID: "default"}); err != nil {
		t.Fatalf("import: %v", err)
	}
	f := fixture{st: st, ids: map[string]string{}}
	for rel := range vault {
		if strings.HasSuffix(rel, ".md") {
			id, err := st.FindDocumentBySource(ctx, "obsidian", rel)
			if err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			f.ids[rel] = id
		}
	}
	create := func(id, notebook, title, body string) {
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: id, CollectionID: "default", NotebookID: notebook, Title: title, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	create("doc_twin_one", "", "Twin", "# Twin\n")
	create("doc_twin_two", "", "Twin", "# Twin\n")
	create("doc_writer", "", "Writer", "# Writer\n\nWhich [[Twin]]? Or [twin](Twin).\n")
	// Not measured: a read-only notebook, and the Trash.
	create("doc_reported", store.ReportsNotebookID, "Reported", "# Reported\n\n[[Twin]]\n")
	create("doc_trashed", "", "Trashed", "# Trashed\n\n[[Twin]]\n")
	trashed, err := st.GetDocument(ctx, "doc_trashed")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: trashed.ID, BaseRevisionID: trashed.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	return f
}

func linkWritten(note Note, written string) (Link, bool) {
	for _, link := range note.Links {
		if link.Written == written {
			return link, true
		}
	}
	return Link{}, false
}

func noteTitled(report Report, title string) (Note, bool) {
	for _, note := range report.Notes {
		if note.Title == title {
			return note, true
		}
	}
	return Note{}, false
}

func titles(candidates []Candidate) []string {
	out := []string{}
	for _, candidate := range candidates {
		out = append(out, candidate.Title)
	}
	return out
}

func TestJ40CEveryAmbiguousLinkWithTheNotesItMightMean(t *testing.T) {
	f := newFixture(t)
	report, err := Generate(context.Background(), f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}

	// from.md: two the import found, one attachment name; also.md: one the
	// import found; Writer: two by title. [[nowhere]] opens nothing and is only
	// counted. The read-only and trashed notes are not measured.
	if report.NotesWithAmbiguousLinks != 3 || report.AmbiguousLinks != 6 ||
		report.FoundByImportCount != 3 || report.FoundByLibraryCount != 3 || report.UnresolvedLinks != 1 {
		t.Fatalf("counts: %+v", report)
	}
	got := []string{}
	for _, note := range report.Notes {
		got = append(got, note.Title)
	}
	if strings.Join(got, ",") != "Also,From,Writer" {
		t.Fatalf("notes in title order: %v", got)
	}

	from, _ := noteTitled(report, "From")
	index, ok := linkWritten(from, "[[index|the index]]")
	if !ok || index.FoundBy != FoundByImport || index.Kind != KindName || index.CandidateCount != 2 ||
		strings.Join(titles(index.Candidates), ",") != "Topic A,Topic B" {
		t.Fatalf("[[index|the index]]: %+v", index)
	}
	// Line 7 of the note as stored: the import put four lines of provenance
	// front matter above the vault's three.
	if index.Line != 7 || index.RawTarget != "index" {
		t.Fatalf("[[index|the index]] located at %d, target %q", index.Line, index.RawTarget)
	}
	shared, ok := linkWritten(from, "[[Shared]]")
	if !ok || shared.FoundBy != FoundByImport || strings.Join(titles(shared.Candidates), ",") != "One,Two" {
		t.Fatalf("[[Shared]]: %+v", shared)
	}
	pic, ok := linkWritten(from, "![[pic.png]]")
	if !ok || pic.FoundBy != FoundByLibrary || pic.Kind != KindAttachment || strings.Join(titles(pic.Candidates), ",") != "pic.png,pic.png" {
		t.Fatalf("![[pic.png]]: %+v", pic)
	}
	for _, candidate := range pic.Candidates {
		if candidate.Kind != "attachment" || !strings.HasPrefix(candidate.URI, "resource://") || candidate.StableURI != "" {
			t.Fatalf("an attachment candidate is named by its resource URI only: %+v", candidate)
		}
	}
	if _, listed := linkWritten(from, "[[nowhere]]"); listed {
		t.Fatal("a link that names nothing is lint's, not this report's")
	}

	writer, _ := noteTitled(report, "Writer")
	if len(writer.Links) != 2 {
		t.Fatalf("Writer's links: %+v", writer.Links)
	}
	for _, link := range writer.Links {
		if link.FoundBy != FoundByLibrary || link.Kind != KindTitle || link.CandidateCount != 2 {
			t.Fatalf("Writer's %s: %+v", link.Written, link)
		}
		for _, candidate := range link.Candidates {
			if candidate.Title != "Twin" || !strings.HasPrefix(candidate.URI, "document://") || !strings.HasPrefix(candidate.StableURI, "notrios://") {
				t.Fatalf("a note candidate is named by title, document URI and stable URL: %+v", candidate)
			}
		}
	}
	if !strings.HasPrefix(writer.StableURI, "notrios://") || writer.URI != store.DocumentURI("default", "doc_writer") {
		t.Fatalf("the note is named by both URIs: %+v", writer)
	}
	if len(report.Warnings) != 1 || report.Warnings[0] != ImportNamespaceLimit {
		t.Fatalf("an import's candidates carry their limit: %v", report.Warnings)
	}
}

// TestJ40CTheCapsHideAndCountNothingElse: a smaller report shows less and says
// so, and its counts and digest are the full report's.
func TestJ40CTheCapsHideAndCountNothingElse(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	full, err := Generate(ctx, f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	small, err := Generate(ctx, f.st, Request{CollectionID: "default", DetailLimit: 1, CandidateLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if small.ReportSHA256 != full.ReportSHA256 || small.AmbiguousLinks != full.AmbiguousLinks ||
		small.NotesWithAmbiguousLinks != full.NotesWithAmbiguousLinks || small.UnresolvedLinks != full.UnresolvedLinks {
		t.Fatalf("the caps changed what was counted:\nfull  %+v\nsmall %+v", full, small)
	}
	if len(small.Notes) != 1 || !small.Truncated || full.Truncated {
		t.Fatalf("one note shown of three, and said: %+v", small)
	}
	note := small.Notes[0]
	if len(note.Links) != 1 || note.LinkCount != 1 {
		t.Fatalf("Also has one link: %+v", note)
	}
	if link := note.Links[0]; len(link.Candidates) != 1 || link.CandidateCount != 2 || !link.Truncated {
		t.Fatalf("one candidate shown of two, and said: %+v", link)
	}
	again, err := Generate(ctx, f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if again.ReportSHA256 != full.ReportSHA256 {
		t.Fatal("the same library gave two digests")
	}
	if capped := normalize(Request{DetailLimit: 1 << 20, CandidateLimit: 1 << 20}); capped.DetailLimit != MaxDetailLimit || capped.CandidateLimit != MaxCandidateLimit {
		t.Fatalf("the ceilings hold: %+v", capped)
	}
}

// TestJ40CTheDigestMovesWithTheLibrary: a new candidate changes the digest
// even though no link changed, because what the owner must choose between did.
func TestJ40CTheDigestMovesWithTheLibrary(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	before, err := Generate(ctx, f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_twin_three", CollectionID: "default", Title: "Twin", Body: "# Twin\n"}); err != nil {
		t.Fatal(err)
	}
	after, err := Generate(ctx, f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if after.ReportSHA256 == before.ReportSHA256 {
		t.Fatal("a third Twin must move the digest")
	}
	writer, _ := noteTitled(after, "Writer")
	if writer.Links[0].CandidateCount != 3 {
		t.Fatalf("three Twins now: %+v", writer.Links[0])
	}
}
