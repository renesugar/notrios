package store

import (
	"context"
	"testing"
)

// The property every one of these paths lacked: a note that came from somewhere
// else is still the reader's note.
//
// Someone who migrated from Joplin has a library where most notes carry a
// collection that is not `default`. Until this was fixed, lint reported their
// library as clean while it rotted, fix could not reach the notes that needed
// repair, and `export archive-v2 --target full_archive` wrote a "complete
// backup" that omitted almost everything they owned. Nothing warned: the scope
// defaulted to `default` and each report named that collection as though it had
// been asked for.
//
// One fixture, four questions, because the defect was one defaulting rule
// copied into four places -- and a test per path is what stops it being copied
// into a fifth.
func newMigratedLibrary(t *testing.T) (*SQLiteStore, string) {
	t.Helper()
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateCollection(ctx, Collection{ID: "joplin-2026-09", Name: "Joplin, September 2026"}); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// The note the migration is about to be judged on, and it carries one
	// problem of each kind these paths look for: a link written the way another
	// application writes them -- resolvable, but not canonical, which is
	// exactly what `fix` rewrites -- and a link to a note that no longer
	// exists, which is what `lint` reports and nothing can mechanically repair.
	target, err := st.CreateDocument(ctx, CreateDocumentRequest{
		CollectionID: "joplin-2026-09", Title: "Kitchen", Body: "# Kitchen\n\nBody.\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	migrated, err := st.CreateDocument(ctx, CreateDocumentRequest{
		CollectionID: "joplin-2026-09",
		Title:        "Note from Joplin",
		Body: "See [the plan](Kitchen).\n\n" +
			"[gone](document://joplin-2026-09/documents/doc_missing)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	_ = target
	return st, migrated.ID
}

func TestLintSeesNotesFromEveryCollection(t *testing.T) {
	st, migrated := newMigratedLibrary(t)
	report, err := st.LintWorkspace(context.Background(), LintRequest{})
	if err != nil {
		t.Fatalf("LintWorkspace: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		for _, finding := range check.Findings {
			if finding.DocumentID == migrated {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("lint never looked at the migrated note; %d findings in collection %q",
			report.TotalFindings, report.CollectionID)
	}
}

func TestFixReachesNotesFromEveryCollection(t *testing.T) {
	st, migrated := newMigratedLibrary(t)
	plan, err := st.PlanWorkspaceFix(context.Background(), FixRequest{})
	if err != nil {
		t.Fatalf("PlanWorkspaceFix: %v", err)
	}
	found := false
	for _, document := range plan.Documents {
		if document.DocumentID == migrated {
			found = true
		}
	}
	if !found {
		t.Fatalf("fix cannot reach the migrated note; it planned %d documents in collection %q",
			len(plan.Documents), plan.CollectionID)
	}
}

func TestSelectionCoversNotesFromEveryCollection(t *testing.T) {
	st, migrated := newMigratedLibrary(t)
	plan, err := st.PlanSelection(context.Background(), SelectionPlanRequest{
		Target: "full_archive",
	})
	if err != nil {
		t.Fatalf("PlanSelection: %v", err)
	}
	found := false
	for _, document := range plan.Documents {
		if document.ID == migrated {
			found = true
		}
	}
	if !found {
		t.Fatalf("a full archive would omit the migrated note; it selected %d documents", len(plan.Documents))
	}
}

// Narrowing still works, because provenance is exactly what somebody might want
// to ask about: the widening is of the default, not of the vocabulary.
func TestAskingForOneCollectionStillNarrows(t *testing.T) {
	st, migrated := newMigratedLibrary(t)
	plan, err := st.PlanWorkspaceFix(context.Background(), FixRequest{CollectionID: "default"})
	if err != nil {
		t.Fatalf("PlanWorkspaceFix: %v", err)
	}
	for _, document := range plan.Documents {
		if document.DocumentID == migrated {
			t.Fatal("asking for the default collection returned a note from another one")
		}
	}
}
