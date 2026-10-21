package linkreport

import (
	"context"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/markdownlinks"
	"github.com/renesugar/notrios/internal/store"
)

func lintAmbiguous(t *testing.T, st *store.SQLiteStore) (int, int) {
	t.Helper()
	lint, err := st.LintWorkspace(context.Background(), store.LintRequest{CollectionID: "default", Checks: []string{store.LintAmbiguousLink, store.LintBrokenDocumentLink}})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, check := range lint.Checks {
		counts[check.Check] = check.Count
	}
	return counts[store.LintAmbiguousLink], counts[store.LintBrokenDocumentLink]
}

// TestJ40DTheNoteQuotesAndLinksNothingItReports: the note links the notes and
// candidates it names and nothing else. Each link it reports is quoted as
// literal text, so the extractor finds no wiki link, no name and no path in it.
func TestJ40DTheNoteQuotesAndLinksNothingItReports(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// A title that would be syntax if it were written as it is.
	if _, err := f.st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: "doc_hostile", CollectionID: "default",
		Title: "a](x) [[Twin]] <script>*b*", Body: "# Hostile\n\n[[Twin]]\n"}); err != nil {
		t.Fatal(err)
	}
	document, report, err := WriteNote(ctx, f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if document.ID != NoteID || document.NotebookID != store.ReportsNotebookID || document.Title != NoteTitle {
		t.Fatalf("the report note: %+v", document)
	}
	if report.NotesWithAmbiguousLinks != 4 {
		t.Fatalf("four notes now: %+v", report)
	}
	for _, candidate := range markdownlinks.Extract(document.Body) {
		target := candidate.RawTarget
		if candidate.SourceFormat != "markdown" || !(strings.HasPrefix(target, "document://") || strings.HasPrefix(target, "resource://")) {
			t.Errorf("the note holds a link that is not a URI it wrote: %+v", candidate)
		}
	}
	for _, quoted := range []string{`\[\[index\|the index\]\]`, `\!\[\[pic\.png\]\]`, `\[twin\]\(Twin\)`, `a\]\(x\) \[\[Twin\]\] \<script\>\*b\* ([open](document://default/documents/doc_hostile))`} {
		if !strings.Contains(document.Body, quoted) {
			t.Errorf("the note should hold %s:\n%s", quoted, document.Body)
		}
	}
	if !strings.Contains(document.Body, "[Topic A](document://") || !strings.Contains(document.Body, "1 other link opens nothing") {
		t.Fatalf("candidates are links, and the unresolved count closes the note:\n%s", document.Body)
	}
}

// TestJ40DTheNoteIsOverwrittenAndNeverCounted: regenerating revises the one
// note, and neither lint nor the next report measures it, though it quotes
// every ambiguous link in the library.
func TestJ40DTheNoteIsOverwrittenAndNeverCounted(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ambiguousBefore, brokenBefore := lintAmbiguous(t, f.st)
	first, before, err := WriteNote(ctx, f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	second, after, err := WriteNote(ctx, f.st, Request{CollectionID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.CurrentRevisionID == first.CurrentRevisionID {
		t.Fatalf("regenerated in place: %s/%s then %s/%s", first.ID, first.CurrentRevisionID, second.ID, second.CurrentRevisionID)
	}
	if after.ReportSHA256 != before.ReportSHA256 || after.AmbiguousLinks != before.AmbiguousLinks {
		t.Fatalf("the note entered its own report: %+v then %+v", before, after)
	}
	if ambiguous, broken := lintAmbiguous(t, f.st); ambiguous != ambiguousBefore || broken != brokenBefore {
		t.Fatalf("lint counted the note: %d/%d then %d/%d", ambiguousBefore, brokenBefore, ambiguous, broken)
	}
}

func TestJ40DLiteralEscapesEverySyntaxCharacter(t *testing.T) {
	cases := map[string]string{
		"[[index]]":      `\[\[index\]\]`,
		"![[a|b]]":       `\!\[\[a\|b\]\]`,
		"[t](u \"v\")":   `\[t\]\(u \"v\"\)`,
		"line\nbreak":    "line break",
		"plain words":    "plain words",
		"Ünïcödé 日本":     "Ünïcödé 日本",
		"<b>&amp;`code`": `\<b\>\&amp\;\` + "`code\\`",
	}
	for in, want := range cases {
		if got := literal(in); got != want {
			t.Errorf("literal(%q) = %q, want %q", in, got, want)
		}
	}
}
