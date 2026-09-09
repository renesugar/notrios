package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// taskNote creates a note and marks it as carrying tasks.
//
// The tag is what makes a checkbox a task. These tests are about extraction,
// identity and bounds, so they tag and move on; the gate itself is tested
// below, where it is the subject rather than a precondition.
func taskNote(t *testing.T, st *SQLiteStore, title, body string) Document {
	t.Helper()
	doc := untaggedNote(t, st, title, body)
	if _, err := st.AddDocumentTag(context.Background(), doc.ID, "task"); err != nil {
		t.Fatalf("AddDocumentTag(%q): %v", title, err)
	}
	return doc
}

// untaggedNote is the same note without the tag, for the cases that are about
// notes nobody marked.
func untaggedNote(t *testing.T, st *SQLiteStore, title, body string) Document {
	t.Helper()
	doc, err := st.CreateDocument(context.Background(), CreateDocumentRequest{Title: title, Body: body})
	if err != nil {
		t.Fatalf("CreateDocument(%q): %v", title, err)
	}
	return doc
}

func TestListTasksExtractsCheckboxes(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	taskNote(t, st, "Chores", "# Chores\n\n- [ ] buy milk\n- [x] pay rent\n- not a task\n\nSome prose.\n")
	taskNote(t, st, "No tasks", "# Reading\n\nJust prose, no boxes.\n")

	all, err := st.ListTasks(ctx, TaskListRequest{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(all.Tasks) != 2 || all.OpenCount != 1 || all.DoneCount != 1 {
		t.Fatalf("expected one open and one done: %+v", all)
	}
	// A plain list item is not a task.
	for _, task := range all.Tasks {
		if strings.Contains(task.Text, "not a task") {
			t.Fatalf("a bullet without a checkbox is not a task: %+v", task)
		}
	}
	// The note with no boxes is never even read.
	if all.DocumentsScanned != 1 {
		t.Fatalf("the prefilter should skip notes with no checkbox: scanned %d", all.DocumentsScanned)
	}

	open, err := st.ListTasks(ctx, TaskListRequest{State: TaskStateOpen})
	if err != nil {
		t.Fatalf("ListTasks(open): %v", err)
	}
	if len(open.Tasks) != 1 || open.Tasks[0].Text != "buy milk" {
		t.Fatalf("open filter wrong: %+v", open.Tasks)
	}
	// Counts describe everything found, not everything returned — the same
	// contract lint's capped examples have.
	if open.OpenCount != 1 || open.DoneCount != 1 {
		t.Fatalf("counts must cover both states even when filtered: %+v", open)
	}
}

// A task is addressed by the block model, not by a line number, so it keeps its
// identity when the note is edited *around* it.
func TestTaskIdentitySurvivesEditsAroundIt(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	doc := taskNote(t, st, "Chores", "# Chores\n\n- [ ] buy milk\n")

	before, err := st.ListTasks(ctx, TaskListRequest{DocumentID: doc.ID})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(before.Tasks) != 1 {
		t.Fatalf("expected one task: %+v", before.Tasks)
	}

	// Insert a paragraph above it and append one below: the task moves, its
	// line number changes, its text does not.
	updated, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID:             doc.ID,
		Title:          "Chores",
		Body:           "# Chores\n\nA new paragraph first.\n\n- [ ] buy milk\n\nAnd one after.\n",
		BaseRevisionID: doc.CurrentRevisionID,
	})
	if err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
	after, err := st.ListTasks(ctx, TaskListRequest{DocumentID: updated.ID})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(after.Tasks) != 1 {
		t.Fatalf("expected one task: %+v", after.Tasks)
	}
	if after.Tasks[0].BlockID != before.Tasks[0].BlockID {
		t.Fatalf("identity should survive edits around the task: %q vs %q",
			before.Tasks[0].BlockID, after.Tasks[0].BlockID)
	}
	if after.Tasks[0].Ordinal == before.Tasks[0].Ordinal {
		t.Fatal("the task should have moved, or this test proves nothing")
	}
}

// The honest limit, asserted rather than left to be discovered: a block ID is
// derived from the block's text and a checkbox is part of that text, so ticking
// a task changes its derived ID. An author-written ^marker is the identity that
// survives it — the same rule every other block follows.
func TestTickingATaskChangesItsDerivedIDButNotItsMarker(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	doc := taskNote(t, st, "Chores", "- [ ] buy milk ^milk\n")

	before, _ := st.ListTasks(ctx, TaskListRequest{DocumentID: doc.ID})
	if len(before.Tasks) != 1 || before.Tasks[0].Marker != "milk" {
		t.Fatalf("expected a marked task: %+v", before.Tasks)
	}
	if !strings.HasSuffix(before.Tasks[0].URI, "#^milk") {
		t.Fatalf("a marked task should be addressed by its marker: %q", before.Tasks[0].URI)
	}

	updated, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: doc.ID, Title: "Chores", Body: "- [x] buy milk ^milk\n",
		BaseRevisionID: doc.CurrentRevisionID,
	})
	if err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
	after, _ := st.ListTasks(ctx, TaskListRequest{DocumentID: updated.ID})
	if len(after.Tasks) != 1 || after.Tasks[0].State != TaskStateDone {
		t.Fatalf("expected the task done: %+v", after.Tasks)
	}
	if after.Tasks[0].BlockID == before.Tasks[0].BlockID {
		t.Fatal("ticking changes the block's text, so its derived ID must change; if this ever passes, the block model's contract changed")
	}
	if after.Tasks[0].Marker != before.Tasks[0].Marker {
		t.Fatal("the authored marker is the identity that survives ticking")
	}
	if !strings.HasSuffix(after.Tasks[0].URI, "#^milk") {
		t.Fatalf("a marked task keeps its address across ticking: %q", after.Tasks[0].URI)
	}
}

func TestListTasksBoundsRowsButNotCounts(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	body := strings.Builder{}
	for i := 0; i < 12; i++ {
		body.WriteString("- [ ] task " + itoa(i) + "\n")
	}
	taskNote(t, st, "Many", body.String())

	limited, err := st.ListTasks(ctx, TaskListRequest{Limit: 5})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(limited.Tasks) != 5 || !limited.Truncated {
		t.Fatalf("expected a capped, truncated list: %d %v", len(limited.Tasks), limited.Truncated)
	}
	// The count still describes the library, which is the whole point.
	if limited.OpenCount != 12 {
		t.Fatalf("counts must be complete even when rows are capped: %d", limited.OpenCount)
	}
}

func TestListTasksRefusesAnUnknownState(t *testing.T) {
	st := newOrganizerTestStore(t)
	if _, err := st.ListTasks(context.Background(), TaskListRequest{State: "maybe"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected a refusal, got %v", err)
	}
}

// Trashed notes are out of the live library, so their tasks are too.
func TestListTasksSkipsTrashedNotes(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	doc := taskNote(t, st, "Chores", "- [ ] buy milk\n")
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	result, err := st.ListTasks(ctx, TaskListRequest{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(result.Tasks) != 0 || result.OpenCount != 0 {
		t.Fatalf("a trashed note's tasks must not appear: %+v", result)
	}
}

// A checkbox is ordinary Markdown. It appears in quoted text, in code samples,
// in a note explaining how to write a checklist. Treating every one as work
// somebody owes turns the task list into a report on the library's
// punctuation, so a note has to say that its boxes are tasks.
func TestTasksComeOnlyFromNotesMarkedAsCarryingThem(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	untaggedNote(t, st, "Notes on Markdown", "You write a checklist like this:\n\n- [ ] an example\n")
	taskNote(t, st, "Chores", "- [ ] buy milk\n")

	list, err := st.ListTasks(ctx, TaskListRequest{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(list.Tasks) != 1 || list.Tasks[0].Text != "buy milk" {
		t.Fatalf("only the marked note carries tasks: %+v", list.Tasks)
	}
	// The counts describe what was found, so they must not count the example
	// either: a progress figure inflated by somebody's documentation is worse
	// than no figure.
	if list.OpenCount != 1 {
		t.Fatalf("the unmarked note was counted: %+v", list)
	}
}

// The opposite failure is work written down and then invisible because a tag
// was forgotten. Asking for everything is one flag.
func TestUntaggedTasksCanBeAskedFor(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	untaggedNote(t, st, "Notes on Markdown", "- [ ] an example\n")
	taskNote(t, st, "Chores", "- [ ] buy milk\n")

	list, err := st.ListTasks(ctx, TaskListRequest{Untagged: true})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if list.OpenCount != 2 {
		t.Fatalf("asking for everything should find both: %+v", list)
	}
}

// A tag tree that stopped meaning what its root means would be a surprise
// found nowhere else in this library.
func TestTaskTagsIncludeTheirHierarchyAndIgnoreCase(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	for name, tag := range map[string]string{
		"Survey":  "todo/survey",
		"Shouted": "TASK",
		"Plain":   "todo",
	} {
		doc := untaggedNote(t, st, name, "- [ ] "+strings.ToLower(name)+"\n")
		if _, err := st.AddDocumentTag(ctx, doc.ID, tag); err != nil {
			t.Fatalf("AddDocumentTag(%q): %v", tag, err)
		}
	}
	// And one tagged something that merely starts with the same letters.
	nearMiss := untaggedNote(t, st, "Tasking", "- [ ] not a task tag\n")
	if _, err := st.AddDocumentTag(ctx, nearMiss.ID, "tasking"); err != nil {
		t.Fatalf("AddDocumentTag: %v", err)
	}

	list, err := st.ListTasks(ctx, TaskListRequest{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if list.OpenCount != 3 {
		t.Fatalf("expected the three task-tagged notes and not `tasking`: %+v", list)
	}
}
