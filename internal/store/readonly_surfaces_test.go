package store

import (
	"context"
	"strings"
	"testing"
)

// A publication is the one handoff that leaves the user's machine, and nothing
// before F5 stopped it from dumping Notrios' own documentation and generated
// reports onto someone's site.
//
// The exclusion is reported rather than silent: a dry run exists so a person
// can see what a publication will and will not carry.
func TestPublicationExcludesReadOnlyNotebooksAndSaysSo(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)

	mine, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "My note", Body: "mine\n"})
	if err != nil {
		t.Fatal(err)
	}
	for _, notebook := range ReadOnlyNotebookIDs() {
		if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: "sel_" + notebook, Title: "System note", NotebookID: notebook, Body: "system\n",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Named explicitly, which is the strong form: even asked for by notebook,
	// a read-only notebook does not travel in a publication.
	plan, err := st.PlanSelection(ctx, SelectionPlanRequest{
		Target: SelectionTargetPublicationHandoff,
		Selection: SelectionSpec{
			NotebookIDs: append([]string{DefaultNotebookID}, ReadOnlyNotebookIDs()...),
		},
		DetailLimit: 100,
	})
	if err != nil {
		t.Fatalf("PlanSelection: %v", err)
	}
	if !plan.Policy.ExcludeReadOnlyNotebooks {
		t.Fatalf("a publication should report the rule it applied: %+v", plan.Policy)
	}
	selected := map[string]bool{}
	for _, doc := range plan.Documents {
		selected[doc.ID] = true
	}
	// The user's own note is carried. Getting the predicate wrong the obvious
	// way — treating every builtin notebook as read-only — would drop the
	// default Notes notebook and ship almost nothing.
	if !selected[mine.ID] {
		t.Fatalf("the user's own notes are the point of a publication: %+v", plan.Documents)
	}
	for _, notebook := range ReadOnlyNotebookIDs() {
		if selected["sel_"+notebook] {
			t.Fatalf("%s is Notrios' own content and must not be published", notebook)
		}
	}
	reasons := map[string]bool{}
	for _, exclusion := range plan.Exclusions {
		reasons[exclusion.Reason] = true
	}
	for _, notebook := range ReadOnlyNotebookIDs() {
		if !reasons["read_only_notebook:"+notebook] {
			t.Fatalf("the dry run must name why %s was left out: %+v", notebook, plan.Exclusions)
		}
	}
}

// The other two targets differ for reasons, not by omission. A full archive is
// a backup and must be faithful; a subset transfer moves notes between the
// user's own databases, where their own Help and Reports are not a disclosure.
func TestOnlyPublicationExcludesReadOnlyNotebooks(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "sel_help_note", Title: "Help", NotebookID: HelpNotebookID, Body: "docs\n",
	}); err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{SelectionTargetFullArchive, SelectionTargetSubsetTransfer} {
		plan, err := st.PlanSelection(ctx, SelectionPlanRequest{
			Target:      target,
			Selection:   SelectionSpec{NotebookIDs: []string{HelpNotebookID}},
			DetailLimit: 100,
		})
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if plan.Policy.ExcludeReadOnlyNotebooks {
			t.Fatalf("%s must stay faithful: %+v", target, plan.Policy)
		}
		found := false
		for _, doc := range plan.Documents {
			if doc.ID == "sel_help_note" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s should carry every note; excluding one would make restore lossy: %+v", target, plan.Documents)
		}
	}
}

// Lint's link scan filtered on collection and `deleted_at` and nothing else,
// while `documentIsWritableLocked` refuses to touch a read-only note. So lint
// reported broken links inside Notrios' own documentation, `notriosctl fix`
// structurally could not repair them, and the user could not edit the note
// either. A finding nobody can act on is noise.
//
// This is a visible change to lint output on any library with Help seeded.
func TestLintSkipsNotesNobodyCanFix(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	broken := "[gone](document://default/documents/doc_missing_target)\n"

	// The same body in an ordinary note, so the assertion below cannot pass by
	// lint having found nothing at all.
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "lint_mine", Title: "Mine", Body: broken,
	}); err != nil {
		t.Fatal(err)
	}
	for _, notebook := range ReadOnlyNotebookIDs() {
		if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: "lint_" + notebook, Title: "System", NotebookID: notebook, Body: broken,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"lint_mine", "lint_" + ReportsNotebookID, "lint_" + HelpNotebookID} {
		if err := st.RebuildDocumentLinks(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	report, err := st.LintWorkspace(ctx, LintRequest{DetailLimit: 100})
	if err != nil {
		t.Fatalf("LintWorkspace: %v", err)
	}
	reported := map[string]map[string]bool{}
	for _, check := range report.Checks {
		for _, finding := range check.Findings {
			if reported[check.Check] == nil {
				reported[check.Check] = map[string]bool{}
			}
			reported[check.Check][finding.DocumentID] = true
		}
	}
	if !reported[LintBrokenDocumentLink]["lint_mine"] {
		t.Fatalf("lint should still report a broken link the user can fix: %s", mustJSON(t, report))
	}
	for _, notebook := range ReadOnlyNotebookIDs() {
		if reported[LintBrokenDocumentLink]["lint_"+notebook] {
			t.Fatalf("nobody can edit a note in %s, so a content finding there is noise: %s", notebook, mustJSON(t, report))
		}
	}

	// **But not every check.** The exclusion covers findings about a note's
	// *content*, which is what nobody can act on. `projection_backlog` is not
	// one of those: it says the search index is behind, the note is searchable
	// like any other, and hiding it would hide a real operational problem in
	// exchange for nothing. Pinned here so the distinction is deliberate rather
	// than a place the filter was forgotten.
	if !reported[LintProjectionBacklog]["lint_"+HelpNotebookID] {
		t.Fatalf("indexing drift is an operational fact about every note: %s", mustJSON(t, report))
	}
}

// v0.6 F7 reconciliation found that the batch path reached notes the
// single-note routes refuse. `move` and `duplicate` were guarded and `trash`,
// `add_tags`, and `remove_tags` were not — so a Help note could be tagged and
// sent to the Trash through `POST /api/v1/batch` while `DELETE
// /api/v1/documents/{id}` answered 403 for the same note.
//
// Asserted for every operation rather than the three that were broken, because
// a per-operation guard is exactly how three of five came to be missed.
func TestBatchRefusesEveryOperationOnAReadOnlyNote(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	help, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "batch_help", Title: "Help page", NotebookID: HelpNotebookID, Body: "docs\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		operation string
		request   BatchRequest
	}{
		{BatchOpTrash, BatchRequest{Items: []BatchItem{{DocumentID: help.ID, BaseRevisionID: help.CurrentRevisionID}}}},
		{BatchOpAddTags, BatchRequest{Tags: []string{"mine"}, Items: []BatchItem{{DocumentID: help.ID}}}},
		{BatchOpRemoveTags, BatchRequest{Tags: []string{"mine"}, Items: []BatchItem{{DocumentID: help.ID}}}},
		{BatchOpMove, BatchRequest{NotebookID: DefaultNotebookID, Items: []BatchItem{{DocumentID: help.ID}}}},
		{BatchOpDuplicate, BatchRequest{Items: []BatchItem{{DocumentID: help.ID}}}},
	} {
		request := tc.request
		request.Operation = tc.operation
		request.Mode = BatchModeBestEffort
		report, err := st.RunBatch(ctx, request)
		if err != nil {
			t.Fatalf("%s: %v", tc.operation, err)
		}
		if len(report.Items) != 1 || report.Items[0].Status != BatchStatusFailed {
			t.Fatalf("%s should be refused on a read-only note: %+v", tc.operation, report.Items)
		}
		if !strings.Contains(report.Items[0].Error, "read-only") &&
			!strings.Contains(report.Items[0].Error, "cannot be") {
			t.Fatalf("%s: the refusal should say why: %q", tc.operation, report.Items[0].Error)
		}
	}

	// Nothing happened to the note through any of them.
	after, err := st.GetDocument(ctx, help.ID)
	if err != nil || !after.DeletedAt.IsZero() || after.NotebookID != HelpNotebookID {
		t.Fatalf("the note should be untouched: %+v err=%v", after, err)
	}
	tags, err := st.ListDocumentTags(ctx, help.ID)
	if err != nil || len(tags) != 0 {
		t.Fatalf("a read-only note should carry no tag it was refused: %+v err=%v", tags, err)
	}
}

// Restore is the deliberate exemption: it can only apply to a note already in
// the Trash, and refusing it would strand one there.
func TestBatchRestoreStillWorksForAnAlreadyTrashedReadOnlyNote(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Ordinary", Body: "x\n"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	report, err := st.RunBatch(ctx, BatchRequest{
		Operation: BatchOpRestore, Mode: BatchModeBestEffort,
		Items: []BatchItem{{DocumentID: doc.ID}},
	})
	if err != nil || report.Items[0].Status != BatchStatusApplied {
		t.Fatalf("restore should still work: %+v err=%v", report.Items, err)
	}
}
