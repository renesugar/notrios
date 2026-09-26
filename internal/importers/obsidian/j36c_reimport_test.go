package obsidian

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// j36cVault is a vault that J36-B and J36-D both change the reading of, next to
// notes they leave alone:
//
//	note.md                 a name shared with folder/note.md
//	folder/note.md
//	folder/linker.md        [[note]] — J36-D sends this to the root note
//	pathy.md                [[wrong/path/unique]] — J36-B leaves this broken
//	only/deep/unique.md     what the base-name fallback used to catch
//	plain.md                no links at all
func j36cVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "note.md"), "# Root Note\n\nAt the vault root.\n")
	writeFile(t, filepath.Join(dir, "folder", "note.md"), "# Folder Note\n\nInside a folder.\n")
	writeFile(t, filepath.Join(dir, "folder", "linker.md"), "# Linker\n\nSee [[note]] for the rest.\n")
	writeFile(t, filepath.Join(dir, "pathy.md"), "# Pathy\n\nSee [[wrong/path/unique]] for the rest.\n")
	writeFile(t, filepath.Join(dir, "only", "deep", "unique.md"), "# Unique\n\nOnly one of me.\n")
	writeFile(t, filepath.Join(dir, "plain.md"), "# Plain\n\nNothing links from here.\n")
	return dir
}

func j36cRevisionCounts(t *testing.T, ctx context.Context, st store.Store, ids ...string) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, id := range ids {
		revisions, err := st.ListDocumentRevisions(ctx, id)
		if err != nil {
			t.Fatalf("list revisions of %s: %v", id, err)
		}
		counts[id] = len(revisions)
	}
	return counts
}

// TestJ36CReimportRepairsOnlyAffectedNotes covers a library imported before
// J36-B and J36-D: a reimport has to move the links those slices changed, and
// leave every other note alone. The pre-fix state is put back by hand, because
// that is exactly what an older build left in the database — the vault files are
// untouched, so only the rewritten body differs.
func TestJ36CReimportRepairsOnlyAffectedNotes(t *testing.T) {
	ctx := context.Background()
	dir := j36cVault(t)
	st := openTestStore(t)

	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); err != nil {
		t.Fatalf("first import failed: %v", err)
	}

	rootID := documentID("note.md")
	folderNoteID := documentID("folder/note.md")
	linkerID := documentID("folder/linker.md")
	pathyID := documentID("pathy.md")
	uniqueID := documentID("only/deep/unique.md")
	plainID := documentID("plain.md")
	all := []string{rootID, folderNoteID, linkerID, pathyID, uniqueID, plainID}

	rootURI := store.DocumentURI("default", rootID)
	folderURI := store.DocumentURI("default", folderNoteID)
	uniqueURI := store.DocumentURI("default", uniqueID)

	linker, err := st.GetDocument(ctx, linkerID)
	if err != nil {
		t.Fatalf("get linker: %v", err)
	}
	if !strings.Contains(linker.Body, rootURI) || strings.Contains(linker.Body, folderURI) {
		t.Fatalf("J36-D: [[note]] should resolve to the root note, body was:\n%s", linker.Body)
	}
	pathy, err := st.GetDocument(ctx, pathyID)
	if err != nil {
		t.Fatalf("get pathy: %v", err)
	}
	if strings.Contains(pathy.Body, uniqueURI) {
		t.Fatalf("J36-B: [[wrong/path/unique]] should stay unresolved, body was:\n%s", pathy.Body)
	}
	if !strings.Contains(pathy.Body, "wrong/path/unique") {
		t.Fatalf("the broken link text should be left as it is, body was:\n%s", pathy.Body)
	}

	// Put the pre-fix bodies back: the note-relative target J36-D moved, and
	// the base-name target J36-B stopped accepting.
	prefixLinker := strings.ReplaceAll(linker.Body, rootURI, folderURI)
	if prefixLinker == linker.Body {
		t.Fatal("could not build the pre-J36-D body")
	}
	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: linkerID, Title: linker.Title, Body: prefixLinker, BodyMIMEType: linker.BodyMIMEType,
		BaseRevisionID: linker.CurrentRevisionID, Message: "simulate a pre-J36-D import",
	}); err != nil {
		t.Fatalf("doctor linker: %v", err)
	}
	// A wikilink is rewritten in place as [[uri]], so that is what a pre-fix
	// build wrote here once the base-name fallback had accepted the target.
	prefixPathy := strings.Replace(pathy.Body, "[[wrong/path/unique]]", "[["+uniqueURI+"]]", 1)
	if prefixPathy == pathy.Body {
		t.Fatalf("could not build the pre-J36-B body, body was:\n%s", pathy.Body)
	}
	if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: pathyID, Title: pathy.Title, Body: prefixPathy, BodyMIMEType: pathy.BodyMIMEType,
		BaseRevisionID: pathy.CurrentRevisionID, Message: "simulate a pre-J36-B import",
	}); err != nil {
		t.Fatalf("doctor pathy: %v", err)
	}

	before := j36cRevisionCounts(t, ctx, st, all...)

	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("reimport failed: %v", err)
	}
	if report.NotesUpdated != 2 || report.NotesUnchanged != 4 || report.NotesImported != 0 {
		t.Fatalf("a reimport should revise the two notes whose links moved and no others: %#v", report)
	}

	after := j36cRevisionCounts(t, ctx, st, all...)
	for _, id := range all {
		want := before[id]
		if id == linkerID || id == pathyID {
			want++
		}
		if after[id] != want {
			t.Errorf("revisions of %s: before %d, after %d, want %d", id, before[id], after[id], want)
		}
	}

	// The repaired bodies, and a link index rebuilt from them rather than the
	// bodies that were in the database a moment ago.
	linker, err = st.GetDocument(ctx, linkerID)
	if err != nil {
		t.Fatalf("get linker after reimport: %v", err)
	}
	if !strings.Contains(linker.Body, rootURI) || strings.Contains(linker.Body, folderURI) {
		t.Fatalf("the reimport did not move the link to the root note:\n%s", linker.Body)
	}
	links, err := st.ListDocumentLinks(ctx, linkerID, "outgoing")
	if err != nil {
		t.Fatalf("list linker links: %v", err)
	}
	for _, link := range links.Outgoing {
		if link.TargetDocumentID == folderNoteID {
			t.Fatalf("the link index still points at folder/note.md: %#v", link)
		}
	}
	pathy, err = st.GetDocument(ctx, pathyID)
	if err != nil {
		t.Fatalf("get pathy after reimport: %v", err)
	}
	if strings.Contains(pathy.Body, uniqueURI) {
		t.Fatalf("the reimport left the base-name target in place:\n%s", pathy.Body)
	}
}

// TestJ36CNoOpReimportCreatesNoRevisions is the other half of the question: a
// vault the fixes do not touch must come back from a reimport with no new
// revision on any note, not merely with the counters saying "unchanged".
func TestJ36CNoOpReimportCreatesNoRevisions(t *testing.T) {
	ctx := context.Background()
	dir := j36cVault(t)
	st := openTestStore(t)

	if _, err := Import(ctx, st, dir, Options{CollectionID: "default"}); err != nil {
		t.Fatalf("first import failed: %v", err)
	}
	all := []string{
		documentID("note.md"), documentID("folder/note.md"), documentID("folder/linker.md"),
		documentID("pathy.md"), documentID("only/deep/unique.md"), documentID("plain.md"),
	}
	before := j36cRevisionCounts(t, ctx, st, all...)

	report, err := Import(ctx, st, dir, Options{CollectionID: "default"})
	if err != nil {
		t.Fatalf("reimport failed: %v", err)
	}
	if report.NotesUnchanged != 6 || report.NotesUpdated != 0 || report.NotesImported != 0 {
		t.Fatalf("a no-op reimport should change nothing: %#v", report)
	}
	after := j36cRevisionCounts(t, ctx, st, all...)
	for _, id := range all {
		if after[id] != before[id] {
			t.Errorf("revisions of %s: %d became %d on a no-op reimport", id, before[id], after[id])
		}
	}
}
