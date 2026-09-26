package obsidian

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestJ37CImportRecordsOneRowPerSpan follows J37-B's decision through an import:
// what the body keeps, what rows the store ends up with, and what a reimport of
// a library written by an older build does about it.
func TestJ37CImportRecordsOneRowPerSpan(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Target.md"), "# Target\n\nThe target note.\n")
	writeFile(t, filepath.Join(dir, "Note.md"),
		"# Note\n\nSee [label]([[Target]]) and [[Target]] here.\n")
	st := openTestStore(t)

	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	noteID := documentID("Note.md")
	targetID := documentID("Target.md")
	targetURI := store.DocumentURI("default", targetID)

	note, err := st.GetDocument(ctx, noteID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	// The literal href is left alone; the top-level wiki link is rewritten.
	if !strings.Contains(note.Body, "[label]([[Target]])") {
		t.Errorf("the literal href should be untouched, body was:\n%s", note.Body)
	}
	if !strings.Contains(note.Body, "[["+targetURI+"]]") {
		t.Errorf("the top-level wiki link should be rewritten, body was:\n%s", note.Body)
	}

	rows := outgoingRows(t, ctx, st, noteID)
	if len(rows) != 2 {
		t.Fatalf("want one row per span, got %d: %#v", len(rows), rows)
	}
	var literal, resolved int
	for _, row := range rows {
		switch {
		case row.RawTarget == "[[Target]]":
			literal++
			if row.ResolutionStatus != "unresolved" {
				t.Errorf("a literal href names no note, so it is unresolved: %#v", row)
			}
			if row.TargetDocumentID != "" {
				t.Errorf("a literal href must not point at a note: %#v", row)
			}
			if row.AnchorType != "" || row.AnchorValue != "" {
				t.Errorf("a literal href has no anchor to split off: %#v", row)
			}
		case row.TargetDocumentID == targetID && row.ResolutionStatus == "resolved":
			resolved++
		default:
			t.Errorf("unexpected row: %#v", row)
		}
	}
	if literal != 1 || resolved != 1 {
		t.Fatalf("want one literal and one resolved row, got %d and %d", literal, resolved)
	}

	// J36-C's invariant still holds: a reimport of what this build wrote adds no
	// revision to anything.
	before := j36cRevisionCounts(t, ctx, st, noteID, targetID)
	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("reimport failed: %v", err)
	}
	if report.NotesUnchanged != 2 || report.NotesUpdated != 0 {
		t.Fatalf("a no-op reimport should change nothing: %#v", report)
	}
	after := j36cRevisionCounts(t, ctx, st, noteID, targetID)
	for id, count := range before {
		if after[id] != count {
			t.Errorf("revisions of %s: %d became %d on a no-op reimport", id, count, after[id])
		}
	}

	// A library imported before J37 had the nested wiki link rewritten too, so
	// its body reads `[label]([[document://…]])`. A reimport repairs it, and the
	// row that pointed at Target from inside the parentheses goes with it.
	prefix := strings.Replace(note.Body, "[label]([[Target]])", "[label]([["+targetURI+"]])", 1)
	if prefix == note.Body {
		t.Fatal("could not build the pre-J37 body")
	}
	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: noteID, Title: note.Title, Body: prefix, BodyMIMEType: note.BodyMIMEType,
		BaseRevisionID: note.CurrentRevisionID, Message: "simulate a pre-J37 import",
	}); err != nil {
		t.Fatalf("doctor note: %v", err)
	}
	// What that body means under J37's rule, and it is the migration finding:
	// the nested rewrite is now read as a literal href `[[document://…]]`,
	// brackets included, which resolves to nothing. Pre-J37 the note had two
	// edges to Target — the outer row unresolved and the inner wiki row
	// resolved — so the edge from inside the parentheses disappears from the
	// index until the note is reimported. That body was written by the bug this
	// item removes, and a reimport is what repairs it.
	staleRows := outgoingRows(t, ctx, st, noteID)
	if len(staleRows) != 2 {
		t.Fatalf("the pre-J37 body should record two spans, got %d: %#v", len(staleRows), staleRows)
	}
	staleReaching := 0
	for _, row := range staleRows {
		if row.TargetDocumentID == targetID {
			staleReaching++
			continue
		}
		if row.RawTarget != "[["+targetURI+"]]" || row.ResolutionStatus != "unresolved" {
			t.Errorf("want the nested rewrite read as an unresolved literal href, got %#v", row)
		}
	}
	if staleReaching != 1 {
		t.Errorf("only the top-level link should still reach Target, got %d", staleReaching)
	}

	report, err = Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("repair reimport failed: %v", err)
	}
	if report.NotesUpdated != 1 || report.NotesUnchanged != 1 {
		t.Fatalf("the reimport should revise the one stale note: %#v", report)
	}
	repaired, err := st.GetDocument(ctx, noteID)
	if err != nil {
		t.Fatalf("get repaired note: %v", err)
	}
	if !strings.Contains(repaired.Body, "[label]([[Target]])") {
		t.Errorf("the reimport should restore the literal href, body was:\n%s", repaired.Body)
	}
	rows = outgoingRows(t, ctx, st, noteID)
	if len(rows) != 2 {
		t.Fatalf("want one row per span after repair, got %d", len(rows))
	}
	pointing, repairedLiteral := 0, 0
	for _, row := range rows {
		if row.TargetDocumentID == targetID {
			pointing++
		}
		if row.RawTarget == "[[Target]]" {
			repairedLiteral++
		}
	}
	if pointing != 1 {
		t.Errorf("only the top-level wiki link should reach Target, got %d rows that do", pointing)
	}
	if repairedLiteral != 1 {
		t.Errorf("the literal href should read as written in the vault again, rows were %#v", rows)
	}
}

func outgoingRows(t *testing.T, ctx context.Context, st store.Store, documentID string) []store.DocumentLink {
	t.Helper()
	page, err := st.ListDocumentLinks(ctx, documentID, "outgoing")
	if err != nil {
		t.Fatalf("list links of %s: %v", documentID, err)
	}
	return page.Outgoing
}
