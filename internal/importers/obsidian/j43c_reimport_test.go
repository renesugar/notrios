package obsidian

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ43CTitlesSurviveAnImport carries J43-B through an import and through the
// repair of a library imported before it.
func TestJ43CTitlesSurviveAnImport(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Target.md"), "# Target\n\nThe target.\n")
	writeFile(t, filepath.Join(dir, "assets", "shot.png"), "PNGDATA")
	writeFile(t, filepath.Join(dir, "Index.md"), strings.Join([]string{
		"# Index", "",
		"A [link](Target.md \"Why this matters\") here.",
		"",
		"An ![embed](assets/shot.png 'The photo') here.",
		"",
		"A [plain](Target.md) link with no title.",
		"",
	}, "\n"))
	st := openTestStore(t)

	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	indexID := documentID("Index.md")
	targetID := documentID("Target.md")
	shotID := resourceID("assets/shot.png")
	targetURI := store.DocumentURI("default", targetID)
	shotURI := store.ResourceURI("default", shotID)

	index, err := st.GetDocument(ctx, indexID)
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	for _, want := range []string{
		"[link](" + targetURI + " \"Why this matters\")",
		"![embed](" + shotURI + " 'The photo')",
		"[plain](" + targetURI + ")",
	} {
		if !strings.Contains(index.Body, want) {
			t.Errorf("want %s in the body, which was:\n%s", want, index.Body)
		}
	}

	// The rows are unchanged by this: a row records where a link points, and a
	// title is not part of that.
	rows := outgoingRows(t, ctx, st, indexID)
	if len(rows) != 3 {
		t.Fatalf("want three rows, got %d: %#v", len(rows), rows)
	}
	for _, row := range rows {
		if strings.Contains(row.RawTarget, "Why this matters") || strings.Contains(row.RawTarget, "The photo") {
			t.Errorf("a title should not reach the row's target: %#v", row)
		}
		if row.ResolutionStatus != "resolved" {
			t.Errorf("want every row resolved: %#v", row)
		}
	}

	// J36-C's invariant.
	before := j36cRevisionCounts(t, ctx, st, indexID, targetID)
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

	// A library imported before J43 has the titles gone from its bodies, because
	// the rewrite built each link from the display text and the URI alone. That
	// is an exact simulation: those bytes are what the old importer wrote.
	stale := strings.NewReplacer(
		"[link]("+targetURI+" \"Why this matters\")", "[link]("+targetURI+")",
		"![embed]("+shotURI+" 'The photo')", "![embed]("+shotURI+")",
	).Replace(index.Body)
	if stale == index.Body {
		t.Fatal("could not build the pre-J43 body")
	}
	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: indexID, Title: index.Title, Body: stale, BodyMIMEType: index.BodyMIMEType,
		BaseRevisionID: index.CurrentRevisionID, Message: "simulate a pre-J43 import",
	}); err != nil {
		t.Fatalf("doctor index: %v", err)
	}
	staleRevisions := j36cRevisionCounts(t, ctx, st, indexID, targetID)

	report, err = Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("repair reimport failed: %v", err)
	}
	if report.NotesUpdated != 1 || report.NotesUnchanged != 1 {
		t.Fatalf("the reimport should revise the one stale note: %#v", report)
	}
	after := j36cRevisionCounts(t, ctx, st, indexID, targetID)
	if after[indexID] != staleRevisions[indexID]+1 {
		t.Errorf("the stale note should gain one revision, %d became %d",
			staleRevisions[indexID], after[indexID])
	}
	if after[targetID] != staleRevisions[targetID] {
		t.Errorf("the untouched note should gain none, %d became %d",
			staleRevisions[targetID], after[targetID])
	}
	repaired, err := st.GetDocument(ctx, indexID)
	if err != nil {
		t.Fatalf("get repaired index: %v", err)
	}
	if !strings.Contains(repaired.Body, "\"Why this matters\"") ||
		!strings.Contains(repaired.Body, "'The photo'") {
		t.Errorf("the reimport should put both titles back, body was:\n%s", repaired.Body)
	}
}
