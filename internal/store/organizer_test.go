package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
)

func newOrganizerTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	return newNotebookTestStore(t)
}

// tagNote creates a note and gives it every named tag.
func tagNote(t *testing.T, st *SQLiteStore, title string, tags ...string) Document {
	t.Helper()
	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: title, Body: title})
	if err != nil {
		t.Fatalf("CreateDocument(%q): %v", title, err)
	}
	for _, tag := range tags {
		if _, err := st.AddDocumentTag(ctx, doc.ID, tag); err != nil {
			t.Fatalf("AddDocumentTag(%q, %q): %v", title, tag, err)
		}
	}
	return doc
}

func tagNames(t *testing.T, st *SQLiteStore) []string {
	t.Helper()
	tags, err := st.ListTags(context.Background())
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	sort.Strings(names)
	return names
}

func noteTagNames(t *testing.T, st *SQLiteStore, documentID string) []string {
	t.Helper()
	tags, err := st.ListDocumentTags(context.Background(), documentID)
	if err != nil {
		t.Fatalf("ListDocumentTags: %v", err)
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	sort.Strings(names)
	return names
}

// A dry run must report exactly what an apply does and change nothing. This is
// the whole promise of the feature, so it is asserted by running both against
// identical libraries and comparing the reports field by field.
func TestTagRenameDryRunMatchesApply(t *testing.T) {
	ctx := context.Background()
	build := func() *SQLiteStore {
		st := newOrganizerTestStore(t)
		tagNote(t, st, "one", "project", "project/alpha")
		tagNote(t, st, "two", "project/alpha", "project/beta")
		tagNote(t, st, "three", "unrelated")
		return st
	}

	dryStore := build()
	dry, err := dryStore.RenameTag(ctx, TagRenameRequest{From: "project", To: "work", IncludeChildren: true, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if got := tagNames(t, dryStore); strings.Join(got, ",") != "project,project/alpha,project/beta,unrelated" {
		t.Fatalf("dry run changed tags: %v", got)
	}

	applyStore := build()
	applied, err := applyStore.RenameTag(ctx, TagRenameRequest{From: "project", To: "work", IncludeChildren: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := tagNames(t, applyStore); strings.Join(got, ",") != "unrelated,work,work/alpha,work/beta" {
		t.Fatalf("apply left the wrong tags: %v", got)
	}

	if len(dry.Changes) != len(applied.Changes) {
		t.Fatalf("dry run reported %d changes, apply %d", len(dry.Changes), len(applied.Changes))
	}
	for i := range dry.Changes {
		d, a := dry.Changes[i], applied.Changes[i]
		if d.From != a.From || d.To != a.To || d.Action != a.Action || d.Notes != a.Notes || d.NotesGained != a.NotesGained {
			t.Fatalf("change %d differs:\n dry: %+v\n apply: %+v", i, d, a)
		}
	}
	if dry.Notes != applied.Notes || dry.Notes != 2 {
		t.Fatalf("touched-note counts differ or are wrong: dry=%d apply=%d", dry.Notes, applied.Notes)
	}
	if !dry.DryRun || applied.DryRun {
		t.Fatalf("DryRun flags wrong: dry=%v apply=%v", dry.DryRun, applied.DryRun)
	}
}

// Without include_children the parent moves alone, and the report says the
// children were left behind rather than leaving the caller to notice.
func TestTagRenameWithoutChildrenWarnsAndLeavesThem(t *testing.T) {
	st := newOrganizerTestStore(t)
	tagNote(t, st, "one", "project", "project/alpha", "project/alpha/deep")

	result, err := st.RenameTag(context.Background(), TagRenameRequest{From: "project", To: "work"})
	if err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].To != "work" {
		t.Fatalf("expected exactly the parent renamed: %+v", result.Changes)
	}
	if got := tagNames(t, st); strings.Join(got, ",") != "project/alpha,project/alpha/deep,work" {
		t.Fatalf("children must stay put: %v", got)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "2 child tag") {
		t.Fatalf("expected a warning naming the two children, got %v", result.Warnings)
	}
}

// `projects` is a different tag from `project`, not a child of it. A prefix
// match on the raw string would take it along.
func TestTagRenameDoesNotMatchMidSegment(t *testing.T) {
	st := newOrganizerTestStore(t)
	tagNote(t, st, "one", "project", "projects", "project/alpha")

	if _, err := st.RenameTag(context.Background(), TagRenameRequest{From: "project", To: "work", IncludeChildren: true}); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if got := tagNames(t, st); strings.Join(got, ",") != "projects,work,work/alpha" {
		t.Fatalf("`projects` must be untouched: %v", got)
	}
}

// Renaming onto an existing name merges. The note carrying both ends up with
// one tag, the note carrying only the source gains the destination, and the
// report separates "how many notes had the source" from "how many changed".
func TestTagRenameMergesIntoExistingTag(t *testing.T) {
	st := newOrganizerTestStore(t)
	both := tagNote(t, st, "both", "draft", "todo")
	only := tagNote(t, st, "only", "draft")

	result, err := st.RenameTag(context.Background(), TagRenameRequest{From: "draft", To: "todo"})
	if err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Action != TagRenameActionMerge {
		t.Fatalf("expected a merge: %+v", result.Changes)
	}
	if result.Changes[0].Notes != 2 || result.Changes[0].NotesGained != 1 {
		t.Fatalf("merge counts wrong: %+v", result.Changes[0])
	}
	if got := tagNames(t, st); strings.Join(got, ",") != "todo" {
		t.Fatalf("source tag must be gone: %v", got)
	}
	if got := noteTagNames(t, st, both.ID); strings.Join(got, ",") != "todo" {
		t.Fatalf("note carrying both must end with one tag: %v", got)
	}
	if got := noteTagNames(t, st, only.ID); strings.Join(got, ",") != "todo" {
		t.Fatalf("note carrying only the source must gain the destination: %v", got)
	}
}

// Renaming a child up onto its parent's name is the order-sensitive case: the
// deeper tag's new name is the shallower tag's old one, so it only works if the
// shallow rename happens first.
func TestTagRenameShiftsHierarchyUpwards(t *testing.T) {
	st := newOrganizerTestStore(t)
	tagNote(t, st, "one", "a/x", "a/x/x")

	if _, err := st.RenameTag(context.Background(), TagRenameRequest{From: "a/x", To: "a", IncludeChildren: true}); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if got := tagNames(t, st); strings.Join(got, ",") != "a,a/x" {
		t.Fatalf("hierarchy did not shift up cleanly: %v", got)
	}
}

// A case-only rename is a rename of the same row, not a merge with itself.
func TestTagRenameCaseOnly(t *testing.T) {
	st := newOrganizerTestStore(t)
	tagNote(t, st, "one", "Project")

	result, err := st.RenameTag(context.Background(), TagRenameRequest{From: "project", To: "PROJECT"})
	if err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Action != TagRenameActionRename {
		t.Fatalf("expected a plain rename: %+v", result.Changes)
	}
	if got := tagNames(t, st); strings.Join(got, ",") != "PROJECT" {
		t.Fatalf("case rename failed: %v", got)
	}
}

func TestTagRenameRefusals(t *testing.T) {
	st := newOrganizerTestStore(t)
	tagNote(t, st, "one", "project")
	ctx := context.Background()

	cases := []struct {
		name string
		req  TagRenameRequest
		want error
	}{
		{"missing tag", TagRenameRequest{From: "absent", To: "work"}, ErrNotFound},
		{"empty destination", TagRenameRequest{From: "project", To: "  "}, ErrInvalidInput},
		{"empty segment", TagRenameRequest{From: "project", To: "work//alpha"}, ErrInvalidInput},
		{"trailing separator", TagRenameRequest{From: "project", To: "work/"}, ErrInvalidInput},
		{"into own subtree", TagRenameRequest{From: "project", To: "project/old"}, ErrInvalidInput},
	}
	for _, tc := range cases {
		if _, err := st.RenameTag(ctx, tc.req); !errors.Is(err, tc.want) {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.want, err)
		}
	}
	if got := tagNames(t, st); strings.Join(got, ",") != "project" {
		t.Fatalf("a refused rename must change nothing: %v", got)
	}
}

// The projection carries a note's tags, so every note holding a renamed tag has
// to be re-projected. Nothing else in the transaction would enqueue that.
func TestTagRenameEnqueuesProjectionForAffectedNotes(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	tagNote(t, st, "one", "project")
	tagNote(t, st, "two", "project")
	tagNote(t, st, "three", "unrelated")

	drainProjectionJobs(t, st)
	if _, err := st.RenameTag(ctx, TagRenameRequest{From: "project", To: "work"}); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	jobs, err := st.PendingProjectionJobs(ctx, 100)
	if err != nil {
		t.Fatalf("PendingProjectionJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected one projection job per affected note, got %d", len(jobs))
	}

	// And a dry run enqueues nothing, because the rollback takes the outbox
	// rows with it.
	drainProjectionJobs(t, st)
	if _, err := st.RenameTag(ctx, TagRenameRequest{From: "work", To: "project", DryRun: true}); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	jobs, err = st.PendingProjectionJobs(ctx, 100)
	if err != nil {
		t.Fatalf("PendingProjectionJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("a dry run must leave no projection jobs, got %d", len(jobs))
	}
}

func drainProjectionJobs(t *testing.T, st *SQLiteStore) {
	t.Helper()
	ctx := context.Background()
	jobs, err := st.PendingProjectionJobs(ctx, 1000)
	if err != nil {
		t.Fatalf("PendingProjectionJobs: %v", err)
	}
	for _, job := range jobs {
		if err := st.CompleteProjectionJob(ctx, job.Sequence, nil); err != nil {
			t.Fatalf("CompleteProjectionJob: %v", err)
		}
	}
}

// The preview has to say the part a confirmation dialog cannot infer: the notes
// move to the Trash and are re-homed, rather than being deleted.
func TestPreviewNotebookDeletion(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)

	parent, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	child, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Reports", ParentID: parent.ID})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	live := tagNote(t, st, "live")
	trashed := tagNote(t, st, "trashed")
	if _, err := st.MoveDocumentToNotebook(ctx, live.ID, child.ID); err != nil {
		t.Fatalf("MoveDocumentToNotebook: %v", err)
	}
	if _, err := st.MoveDocumentToNotebook(ctx, trashed.ID, parent.ID); err != nil {
		t.Fatalf("MoveDocumentToNotebook: %v", err)
	}
	trashedDoc, err := st.GetDocument(ctx, trashed.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: trashed.ID, BaseRevisionID: trashedDoc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	preview, err := st.PreviewNotebookDeletion(ctx, parent.ID)
	if err != nil {
		t.Fatalf("PreviewNotebookDeletion: %v", err)
	}
	if preview.Notebooks != 2 || len(preview.DescendantNames) != 1 || preview.DescendantNames[0] != "Reports" {
		t.Fatalf("subtree wrong: %+v", preview)
	}
	if preview.Notes != 1 || preview.TrashedNotes != 1 {
		t.Fatalf("note counts wrong: %+v", preview)
	}
	if preview.RehomeNotebookID != DefaultNotebookID || !preview.Deletable {
		t.Fatalf("rehome/deletable wrong: %+v", preview)
	}

	// The preview must not have deleted anything.
	if _, err := st.GetNotebook(ctx, child.ID); err != nil {
		t.Fatalf("preview deleted the notebook: %v", err)
	}

	// And it reports protection instead of pretending a builtin can go.
	protected, err := st.PreviewNotebookDeletion(ctx, HelpNotebookID)
	if err != nil {
		t.Fatalf("PreviewNotebookDeletion(help): %v", err)
	}
	if protected.Deletable || protected.Reason == "" {
		t.Fatalf("Help notebook must preview as undeletable: %+v", protected)
	}
}

// The preview's counts have to match what DeleteNotebook actually does, or the
// dialog they feed is a guess.
func TestPreviewNotebookDeletionMatchesDeletion(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)

	nb, err := st.CreateNotebook(ctx, CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	for _, title := range []string{"a", "b", "c"} {
		doc := tagNote(t, st, title)
		if _, err := st.MoveDocumentToNotebook(ctx, doc.ID, nb.ID); err != nil {
			t.Fatalf("MoveDocumentToNotebook: %v", err)
		}
	}
	preview, err := st.PreviewNotebookDeletion(ctx, nb.ID)
	if err != nil {
		t.Fatalf("PreviewNotebookDeletion: %v", err)
	}
	if err := st.DeleteNotebook(ctx, nb.ID); err != nil {
		t.Fatalf("DeleteNotebook: %v", err)
	}
	page, err := st.ListTrash(ctx, DocumentPageRequest{Limit: 100})
	if err != nil {
		t.Fatalf("ListTrash: %v", err)
	}
	if int64(len(page.Documents)) != preview.Notes {
		t.Fatalf("preview said %d notes, %d reached the Trash", preview.Notes, len(page.Documents))
	}
	for _, doc := range page.Documents {
		if doc.NotebookID != preview.RehomeNotebookID {
			t.Fatalf("note %s was re-homed to %s, preview said %s", doc.ID, doc.NotebookID, preview.RehomeNotebookID)
		}
	}
}

// A trashed note has to be readable to be recoverable. Everything else — every
// write path, and the agent-facing reads — still stops at the Trash, so this
// asserts both halves rather than only the new one.
func TestGetDocumentIncludingTrashed(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	doc := tagNote(t, st, "recoverable")
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	if _, err := st.GetDocument(ctx, doc.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDocument must still stop at the Trash, got %v", err)
	}
	trashed, err := st.GetDocumentIncludingTrashed(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocumentIncludingTrashed: %v", err)
	}
	if trashed.DeletedAt.IsZero() {
		t.Fatalf("a trashed note must carry DeletedAt; that is what makes it read-only above the store: %+v", trashed)
	}
	if trashed.Title != "recoverable" || trashed.Body == "" {
		t.Fatalf("the note has to come back whole to be reviewable: %+v", trashed)
	}

	// A live note reads identically through both, with no DeletedAt.
	live := tagNote(t, st, "live")
	both, err := st.GetDocumentIncludingTrashed(ctx, live.ID)
	if err != nil || !both.DeletedAt.IsZero() {
		t.Fatalf("a live note must read back untrashed: %+v %v", both, err)
	}
	if _, err := st.GetDocumentIncludingTrashed(ctx, "doc_absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing note is still not found, got %v", err)
	}
}
