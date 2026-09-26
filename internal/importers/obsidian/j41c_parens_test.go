package obsidian

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ41CParenthesesInATargetResolve follows J41-B through an import. A target
// ending at the first `)` did not just record a short string: a note or
// attachment whose file name carries a parenthesis never resolved at all, so the
// link was left as prose. Now it resolves and is rewritten, and an external URL
// is recorded whole.
func TestJ41CParenthesesInATargetResolve(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Note_(draft).md"), "# Draft\n\nA note whose name has a pair.\n")
	writeFile(t, filepath.Join(dir, "assets", "img_(1).png"), "PNGDATA")
	writeFile(t, filepath.Join(dir, "Index.md"), strings.Join([]string{
		"# Index", "",
		"See [draft](Note_(draft).md) for the rest.",
		"",
		"![shot](assets/img_(1).png)",
		"",
		"And [ext](https://example.org/Foo_(bar)) outside.",
		"",
	}, "\n"))
	st := openTestStore(t)

	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	indexID := documentID("Index.md")
	draftID := documentID("Note_(draft).md")
	shotID := resourceID("assets/img_(1).png")
	draftURI := store.DocumentURI("default", draftID)
	shotURI := store.ResourceURI("default", shotID)
	externalURL := "https://example.org/Foo_(bar)"

	index, err := st.GetDocument(ctx, indexID)
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	if !strings.Contains(index.Body, "[draft]("+draftURI+")") {
		t.Errorf("the note link should resolve and be rewritten, body was:\n%s", index.Body)
	}
	if !strings.Contains(index.Body, "![shot]("+shotURI+")") {
		t.Errorf("the attachment link should resolve and be rewritten, body was:\n%s", index.Body)
	}
	if !strings.Contains(index.Body, "[ext]("+externalURL+")") {
		t.Errorf("the external URL should be left whole, body was:\n%s", index.Body)
	}

	rows := outgoingRows(t, ctx, st, indexID)
	if len(rows) != 3 {
		t.Fatalf("want three rows, got %d: %#v", len(rows), rows)
	}
	var sawNote, sawResource, sawExternal bool
	for _, row := range rows {
		switch {
		case row.TargetDocumentID == draftID && row.ResolutionStatus == "resolved":
			sawNote = true
		case row.TargetResourceID == shotID && row.ResolutionStatus == "resolved":
			sawResource = true
		case row.ResolutionStatus == "external":
			sawExternal = true
			if row.RawTarget != externalURL {
				t.Errorf("the external row should record the whole URL, got %q", row.RawTarget)
			}
		default:
			t.Errorf("unexpected row: %#v", row)
		}
	}
	if !sawNote || !sawResource || !sawExternal {
		t.Errorf("rows were %#v", rows)
	}

	// The attachment is referenced, which is what a truncated target used to
	// cost: a resource nothing pointed at.
	refs, err := st.ListDocumentResources(ctx, indexID)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(refs) != 1 || refs[0].ResourceID != shotID {
		t.Fatalf("want the attachment referenced once, got %#v", refs)
	}

	// J36-C's invariant, again: a reimport of what this build wrote changes
	// nothing.
	before := j36cRevisionCounts(t, ctx, st, indexID, draftID)
	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("reimport failed: %v", err)
	}
	if report.NotesUnchanged != 2 || report.NotesUpdated != 0 {
		t.Fatalf("a no-op reimport should change nothing: %#v", report)
	}
	for id, count := range before {
		if after := j36cRevisionCounts(t, ctx, st, id)[id]; after != count {
			t.Errorf("revisions of %s: %d became %d on a no-op reimport", id, count, after)
		}
	}

	// A library imported before J41 has these links unrewritten, because neither
	// target resolved. That is the vault's own text, so putting it back is an
	// exact simulation, and a reimport is the repair.
	stale := strings.NewReplacer(
		"[draft]("+draftURI+")", "[draft](Note_(draft).md)",
		"![shot]("+shotURI+")", "![shot](assets/img_(1).png)",
	).Replace(index.Body)
	if stale == index.Body {
		t.Fatal("could not build the pre-J41 body")
	}
	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: indexID, Title: index.Title, Body: stale, BodyMIMEType: index.BodyMIMEType,
		BaseRevisionID: index.CurrentRevisionID, Message: "simulate a pre-J41 import",
	}); err != nil {
		t.Fatalf("doctor index: %v", err)
	}
	report, err = Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("repair reimport failed: %v", err)
	}
	if report.NotesUpdated != 1 || report.NotesUnchanged != 1 {
		t.Fatalf("the reimport should revise the one stale note: %#v", report)
	}
	repaired, err := st.GetDocument(ctx, indexID)
	if err != nil {
		t.Fatalf("get repaired index: %v", err)
	}
	if !strings.Contains(repaired.Body, "[draft]("+draftURI+")") ||
		!strings.Contains(repaired.Body, "![shot]("+shotURI+")") {
		t.Errorf("the reimport should rewrite both links, body was:\n%s", repaired.Body)
	}
}
