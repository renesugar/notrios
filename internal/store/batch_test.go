package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
)

func batchItems(ids ...string) []BatchItem {
	items := make([]BatchItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, BatchItem{DocumentID: id})
	}
	return items
}

func statuses(result BatchResult) string {
	parts := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		parts = append(parts, item.Status)
	}
	return strings.Join(parts, ",")
}

func notebookOf(t *testing.T, st *SQLiteStore, id string) string {
	t.Helper()
	doc, err := st.GetDocument(context.Background(), id)
	if err != nil {
		t.Fatalf("GetDocument(%s): %v", id, err)
	}
	return doc.NotebookID
}

// Best-effort keeps going and reports every item. An organizer sweep over a
// hundred notes should not be abandoned because one of them is protected.
func TestBatchBestEffortReportsEveryItem(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	work, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	first := tagNote(t, st, "first")
	second := tagNote(t, st, "second")
	if _, err := st.MoveDocumentToNotebook(ctx, second.ID, work.ID); err != nil {
		t.Fatalf("MoveDocumentToNotebook: %v", err)
	}

	result, err := st.RunBatch(ctx, BatchRequest{
		Operation:  BatchOpMove,
		NotebookID: work.ID,
		Items:      batchItems(first.ID, second.ID, "doc_absent"),
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	// Applied, skipped (already there), failed (no such note) — three distinct
	// outcomes, which is the whole reason a batch reports per item.
	if got := statuses(result); got != "applied,skipped,failed" {
		t.Fatalf("expected one of each outcome, got %s (%+v)", got, result.Items)
	}
	if result.Applied != 1 || result.Skipped != 1 || result.Failed != 1 {
		t.Fatalf("tally wrong: %+v", result)
	}
	if result.Items[1].Reason == "" || result.Items[2].Error == "" {
		t.Fatalf("a skip needs a reason and a failure needs an error: %+v", result.Items)
	}
	// The work that could be done was done.
	if notebookOf(t, st, first.ID) != work.ID {
		t.Fatal("the applicable note should have moved")
	}
}

// Atomic undoes everything on the first failure, and says so per item. An item
// that succeeded and was undone is not "failed" — nothing was wrong with it.
func TestBatchAtomicRollsBackAndSaysWhichItemsWereUndone(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	work, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	first := tagNote(t, st, "first")
	third := tagNote(t, st, "third")

	result, err := st.RunBatch(ctx, BatchRequest{
		Operation:  BatchOpMove,
		Mode:       BatchModeAtomic,
		NotebookID: work.ID,
		Items:      batchItems(first.ID, "doc_absent", third.ID),
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if got := statuses(result); got != "rolled_back,failed,skipped" {
		t.Fatalf("expected rolled_back,failed,skipped; got %s (%+v)", got, result.Items)
	}
	if !strings.Contains(result.Items[2].Reason, "not attempted") {
		t.Fatalf("an unattempted item must say so: %+v", result.Items[2])
	}
	// The report is never shorter than the request.
	if len(result.Items) != 3 {
		t.Fatalf("every requested item needs an outcome: %+v", result.Items)
	}
	// And nothing actually moved.
	if notebookOf(t, st, first.ID) == work.ID {
		t.Fatal("an atomic run that failed must leave the library untouched")
	}
	if notebookOf(t, st, third.ID) == work.ID {
		t.Fatal("an unattempted item must not have been applied")
	}
}

func TestBatchAtomicCommitsWhenEveryItemSucceeds(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	work, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	first := tagNote(t, st, "first")
	second := tagNote(t, st, "second")

	result, err := st.RunBatch(ctx, BatchRequest{
		Operation: BatchOpMove, Mode: BatchModeAtomic, NotebookID: work.ID,
		Items: batchItems(first.ID, second.ID),
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if result.Applied != 2 || result.Failed != 0 {
		t.Fatalf("expected both applied: %+v", result)
	}
	if notebookOf(t, st, first.ID) != work.ID || notebookOf(t, st, second.ID) != work.ID {
		t.Fatal("both notes should have moved")
	}
}

// A batch is retried exactly when something went wrong, so the ledger has to
// outlive the process. The replay returns the *first* run's outcomes verbatim.
func TestBatchIsIdempotentAcrossAReopen(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/notes.sqlite"
	open := func() *SQLiteStore {
		st, err := OpenSQLite(path)
		if err != nil {
			t.Fatalf("OpenSQLite: %v", err)
		}
		if err := st.Bootstrap(ctx); err != nil {
			t.Fatalf("Bootstrap: %v", err)
		}
		return st
	}

	st := open()
	note := tagNote(t, st, "only")
	request := BatchRequest{
		RequestKey: "retry-me",
		Operation:  BatchOpAddTags,
		Tags:       []string{"batched"},
		Items:      batchItems(note.ID),
	}
	first, err := st.RunBatch(ctx, request)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if first.Replayed || first.Applied != 1 {
		t.Fatalf("first run should apply: %+v", first)
	}
	_ = st.Close()

	// A new process, the same key: the work must not happen twice.
	st = open()
	t.Cleanup(func() { _ = st.Close() })
	replay, err := st.RunBatch(ctx, request)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !replay.Replayed {
		t.Fatalf("a replay must say it is one: %+v", replay)
	}
	if replay.Applied != first.Applied || len(replay.Items) != len(first.Items) {
		t.Fatalf("a replay must return the first run's outcomes: %+v vs %+v", replay, first)
	}
	if got := noteTagNames(t, st, note.ID); strings.Join(got, ",") != "batched" {
		t.Fatalf("the tag should exist exactly once: %v", got)
	}
}

// Reusing a key for different work is a client bug; answering with the earlier
// unrelated result would hide it.
func TestBatchRefusesAKeyReusedForDifferentWork(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	note := tagNote(t, st, "only")

	base := BatchRequest{RequestKey: "k", Operation: BatchOpAddTags, Tags: []string{"one"}, Items: batchItems(note.ID)}
	if _, err := st.RunBatch(ctx, base); err != nil {
		t.Fatalf("first run: %v", err)
	}
	changed := base
	changed.Tags = []string{"two"}
	if _, err := st.RunBatch(ctx, changed); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected a refusal for a reused key, got %v", err)
	}
	if got := noteTagNames(t, st, note.ID); strings.Join(got, ",") != "one" {
		t.Fatalf("the second request must not have applied: %v", got)
	}
}

// What a copy inherits is a decision, and the one thing it must never inherit
// is external identity: two notes claiming to *be* the same imported note make
// re-import ambiguous and show up in lint as a duplicate source ID.
func TestBatchDuplicateCopiesContentButNotExternalIdentity(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	work, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	source := tagNote(t, st, "original", "alpha", "beta")
	if _, err := st.MoveDocumentToNotebook(ctx, source.ID, work.ID); err != nil {
		t.Fatalf("MoveDocumentToNotebook: %v", err)
	}
	if _, err := st.SetDocumentSource(ctx, SetDocumentSourceRequest{
		DocumentID: source.ID, SourceSystem: "joplin", ExternalID: "ext-1",
	}); err != nil {
		t.Fatalf("SetDocumentSource: %v", err)
	}

	result, err := st.RunBatch(ctx, BatchRequest{Operation: BatchOpDuplicate, Items: batchItems(source.ID)})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	copyID := result.Items[0].NewDocumentID
	if copyID == "" || copyID == source.ID {
		t.Fatalf("expected a new document: %+v", result.Items[0])
	}
	copied, err := st.GetDocument(ctx, copyID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if copied.Title != "original (copy)" {
		t.Fatalf("title should mark the copy: %q", copied.Title)
	}
	if copied.Body != source.Body {
		t.Fatal("the body should be copied")
	}
	if copied.NotebookID != work.ID {
		t.Fatalf("the copy should stay in the same notebook: %q", copied.NotebookID)
	}
	if got := noteTagNames(t, st, copyID); strings.Join(got, ",") != "alpha,beta" {
		t.Fatalf("tags should be copied: %v", got)
	}
	// The decisive one.
	if _, err := st.GetDocumentSource(ctx, copyID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a copy must not claim the original's external identity, got %v", err)
	}
	// And the original keeps its own.
	if _, err := st.GetDocumentSource(ctx, source.ID); err != nil {
		t.Fatalf("the original should keep its identity: %v", err)
	}
	// The copy is findable, which means it was indexed.
	page, err := st.Search(ctx, SearchRequest{Query: `"original (copy)"`, Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	found := false
	for _, hit := range page.Hits {
		if hit.ID == copyID {
			found = true
		}
	}
	if !found {
		t.Fatal("a duplicated note must be searchable")
	}
}

// Trash writes a revision, so every item carries its own precondition — a batch
// is a set of independent notes and one of them having moved on is not a reason
// to refuse the rest.
func TestBatchTrashIsRevisionPreconditionedPerItem(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	fresh := tagNote(t, st, "fresh")
	stale := tagNote(t, st, "stale")

	if _, err := st.RunBatch(ctx, BatchRequest{
		Operation: BatchOpTrash,
		Items:     []BatchItem{{DocumentID: fresh.ID}},
	}); !errors.Is(err, ErrPreconditionRequired) {
		t.Fatalf("a missing base revision must be refused up front, got %v", err)
	}

	result, err := st.RunBatch(ctx, BatchRequest{
		Operation: BatchOpTrash,
		Items: []BatchItem{
			{DocumentID: fresh.ID, BaseRevisionID: fresh.CurrentRevisionID},
			{DocumentID: stale.ID, BaseRevisionID: "rev_stale"},
		},
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if got := statuses(result); got != "applied,failed" {
		t.Fatalf("expected the stale item alone to fail: %s (%+v)", got, result.Items)
	}
	if _, err := st.GetDocument(ctx, fresh.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("the applicable note should be trashed")
	}
	if _, err := st.GetDocument(ctx, stale.ID); err != nil {
		t.Fatal("the stale note should be untouched")
	}
}

func TestBatchRestoreBringsNotesBack(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	one := tagNote(t, st, "one")
	two := tagNote(t, st, "two")
	for _, doc := range []Document{one, two} {
		if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
			t.Fatalf("DeleteDocument: %v", err)
		}
	}
	result, err := st.RunBatch(ctx, BatchRequest{Operation: BatchOpRestore, Mode: BatchModeAtomic, Items: batchItems(one.ID, two.ID)})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if result.Applied != 2 {
		t.Fatalf("expected both restored: %+v", result)
	}
	for _, id := range []string{one.ID, two.ID} {
		if _, err := st.GetDocument(ctx, id); err != nil {
			t.Fatalf("note %s should be back: %v", id, err)
		}
	}
}

// Tag operations distinguish "changed something" from "there was nothing to do".
func TestBatchTagOperationsReportSkipsHonestly(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	tagged := tagNote(t, st, "tagged", "keep")
	untagged := tagNote(t, st, "untagged")

	added, err := st.RunBatch(ctx, BatchRequest{Operation: BatchOpAddTags, Tags: []string{"keep"}, Items: batchItems(tagged.ID, untagged.ID)})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if got := statuses(added); got != "skipped,applied" {
		t.Fatalf("a tag the note already had is a skip: %s (%+v)", got, added.Items)
	}

	removed, err := st.RunBatch(ctx, BatchRequest{Operation: BatchOpRemoveTags, Tags: []string{"absent"}, Items: batchItems(tagged.ID)})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if got := statuses(removed); got != "skipped" {
		t.Fatalf("removing a tag nothing carries is a skip, not a failure: %s", got)
	}
	if got := noteTagNames(t, st, tagged.ID); strings.Join(got, ",") != "keep" {
		t.Fatalf("nothing should have changed: %v", got)
	}
}

func TestBatchRefusals(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	note := tagNote(t, st, "only")

	oversized := make([]BatchItem, MaxBatchItems+1)
	for i := range oversized {
		oversized[i] = BatchItem{DocumentID: "doc_" + itoa(i)}
	}

	cases := []struct {
		name string
		req  BatchRequest
		want error
	}{
		{"unknown operation", BatchRequest{Operation: "explode", Items: batchItems(note.ID)}, ErrInvalidInput},
		{"unknown mode", BatchRequest{Operation: BatchOpRestore, Mode: "maybe", Items: batchItems(note.ID)}, ErrInvalidInput},
		{"no items", BatchRequest{Operation: BatchOpRestore, Items: []BatchItem{}}, ErrInvalidInput},
		{"over the ceiling", BatchRequest{Operation: BatchOpRestore, Items: oversized}, ErrInvalidInput},
		{"repeated document", BatchRequest{Operation: BatchOpDuplicate, Items: batchItems(note.ID, note.ID)}, ErrInvalidInput},
		{"move without a notebook", BatchRequest{Operation: BatchOpMove, Items: batchItems(note.ID)}, ErrInvalidInput},
		{"tags without a tag", BatchRequest{Operation: BatchOpAddTags, Items: batchItems(note.ID)}, ErrInvalidInput},
		{"empty tag name", BatchRequest{Operation: BatchOpAddTags, Tags: []string{" "}, Items: batchItems(note.ID)}, ErrInvalidInput},
	}
	for _, tc := range cases {
		if _, err := st.RunBatch(ctx, tc.req); !errors.Is(err, tc.want) {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.want, err)
		}
	}
	if got := tagNames(t, st); len(got) != 0 {
		sort.Strings(got)
		t.Fatalf("a refused batch must change nothing: %v", got)
	}
}

// Help notes are protected everywhere else; a batch is not a way around that.
func TestBatchCannotReachProtectedNotes(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	help, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Help note", Body: "x", NotebookID: HelpNotebookID})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	work, _ := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})

	moved, err := st.RunBatch(ctx, BatchRequest{Operation: BatchOpMove, NotebookID: work.ID, Items: batchItems(help.ID)})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if moved.Failed != 1 {
		t.Fatalf("a Help note must not be movable by batch: %+v", moved.Items)
	}
	duplicated, err := st.RunBatch(ctx, BatchRequest{Operation: BatchOpDuplicate, Items: batchItems(help.ID)})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if duplicated.Failed != 1 {
		t.Fatalf("a Help note must not be duplicable by batch: %+v", duplicated.Items)
	}
}
