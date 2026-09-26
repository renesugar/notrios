package obsidian

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ42CSpacedNamesResolve follows J42-B through an import. A note whose name
// contains a space is linked with the bracketed form, which is what Obsidian
// documents for it, and that form resolved to nothing because the target was cut
// at the first space.
func TestJ42CSpacedNamesResolve(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "My Note.md"), "# My Note\n\nA name with a space.\n")
	writeFile(t, filepath.Join(dir, "assets", "my shot.png"), "PNGDATA")
	writeFile(t, filepath.Join(dir, "Index.md"), strings.Join([]string{
		"# Index", "",
		"Bracketed [note](<My Note.md>) here.",
		"",
		"Embedded ![shot](<assets/my shot.png>) here.",
		"",
		"Encoded [pct](My%20Note.md) here.",
		"",
		"Titled [titled](<My Note.md> \"A title\") here.",
		"",
	}, "\n"))
	st := openTestStore(t)

	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	indexID := documentID("Index.md")
	noteID := documentID("My Note.md")
	shotID := resourceID("assets/my shot.png")
	noteURI := store.DocumentURI("default", noteID)
	shotURI := store.ResourceURI("default", shotID)

	index, err := st.GetDocument(ctx, indexID)
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	if !strings.Contains(index.Body, "[note]("+noteURI+")") {
		t.Errorf("the bracketed note link should resolve, body was:\n%s", index.Body)
	}
	if !strings.Contains(index.Body, "![shot]("+shotURI+")") {
		t.Errorf("the bracketed attachment link should resolve, body was:\n%s", index.Body)
	}
	if !strings.Contains(index.Body, "[pct]("+noteURI+")") {
		t.Errorf("the percent-encoded alternative should still resolve, body was:\n%s", index.Body)
	}
	if !strings.Contains(index.Body, "[titled]("+noteURI+")") {
		t.Errorf("a bracketed link with a title should resolve, body was:\n%s", index.Body)
	}
	// Rewriting a Markdown link has always dropped its title, for an ordinary
	// destination as much as a bracketed one. Recorded here as what happens, not
	// endorsed: it is J43.
	if strings.Contains(index.Body, "\"A title\"") {
		t.Errorf("unexpected: the title survived the rewrite, body was:\n%s", index.Body)
	}

	rows := outgoingRows(t, ctx, st, indexID)
	if len(rows) != 4 {
		t.Fatalf("want four rows, got %d: %#v", len(rows), rows)
	}
	toNote, toShot := 0, 0
	for _, row := range rows {
		switch {
		case row.TargetDocumentID == noteID && row.ResolutionStatus == "resolved":
			toNote++
		case row.TargetResourceID == shotID && row.ResolutionStatus == "resolved":
			toShot++
		default:
			t.Errorf("unexpected row: %#v", row)
		}
	}
	if toNote != 3 || toShot != 1 {
		t.Errorf("want three note rows and one resource row, got %d and %d", toNote, toShot)
	}

	refs, err := st.ListDocumentResources(ctx, indexID)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(refs) != 1 || refs[0].ResourceID != shotID {
		t.Fatalf("want the attachment referenced once, got %#v", refs)
	}

	// J36-C's invariant.
	before := j36cRevisionCounts(t, ctx, st, indexID, noteID)
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

	// A library imported before J42 has the bracketed links unrewritten, because
	// their targets did not resolve. That is the vault's own text, so the
	// simulation is exact, and the percent-encoded link was already rewritten.
	stale := strings.NewReplacer(
		"[note]("+noteURI+")", "[note](<My Note.md>)",
		"![shot]("+shotURI+")", "![shot](<assets/my shot.png>)",
		"[titled]("+noteURI+")", "[titled](<My Note.md> \"A title\")",
	).Replace(index.Body)
	if stale == index.Body {
		t.Fatal("could not build the pre-J42 body")
	}
	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: indexID, Title: index.Title, Body: stale, BodyMIMEType: index.BodyMIMEType,
		BaseRevisionID: index.CurrentRevisionID, Message: "simulate a pre-J42 import",
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
	for _, want := range []string{"[note](" + noteURI + ")", "![shot](" + shotURI + ")", "[titled](" + noteURI + ")"} {
		if !strings.Contains(repaired.Body, want) {
			t.Errorf("the reimport should rewrite %s, body was:\n%s", want, repaired.Body)
		}
	}
}
