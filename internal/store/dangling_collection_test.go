package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestLintFindsANoteWhoseCollectionHasNoRow is v0.8 H16-D's detection half.
//
// `documents.collection_id` is a foreign key and PRAGMA foreign_keys is ON, so
// no supported write produces this. A physical restore suspends the constraint
// while it installs an image, so a library can arrive in this state -- and
// until now nothing said so. The test has to break the database on purpose,
// which is exactly what a detector is for.
func TestLintFindsANoteWhoseCollectionHasNoRow(t *testing.T) {
	root := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatalf("opening a library: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrapping: %v", err)
	}
	if _, err := st.EnsureCollection(ctx, "joplin-2026-01", "Joplin export, January 2026"); err != nil {
		t.Fatalf("creating a collection: %v", err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Imported", Body: "body", CollectionID: "joplin-2026-01"})
	if err != nil {
		t.Fatalf("creating a note: %v", err)
	}

	clean, err := st.LintWorkspace(ctx, store.LintRequest{Checks: []string{store.LintDanglingCollection}})
	if err != nil {
		t.Fatalf("linting: %v", err)
	}
	if total := findingsFor(clean, store.LintDanglingCollection); total != 0 {
		t.Fatalf("a healthy library reported %d dangling collections", total)
	}

	// The state a restore can leave behind, made deliberately.
	if err := st.Exec(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Skipf("cannot suspend the constraint to build the broken state: %v", err)
	}
	if err := st.Exec(ctx, `DELETE FROM collections WHERE id = 'joplin-2026-01'`); err != nil {
		t.Fatalf("removing the collection row: %v", err)
	}

	broken, err := st.LintWorkspace(ctx, store.LintRequest{Checks: []string{store.LintDanglingCollection}})
	if err != nil {
		t.Fatalf("linting the broken library: %v", err)
	}
	if findingsFor(broken, store.LintDanglingCollection) != 1 {
		t.Fatalf("a note naming a collection with no row was not reported: %+v", broken)
	}
	if !strings.Contains(detailFor(broken, store.LintDanglingCollection), "joplin-2026-01") {
		t.Errorf("the finding does not name the missing collection: %+v", broken)
	}
	_ = doc
}

func findingsFor(report store.LintReport, check string) int {
	for _, result := range report.Checks {
		if result.Check == check {
			return int(result.Count)
		}
	}
	return 0
}

func detailFor(report store.LintReport, check string) string {
	for _, result := range report.Checks {
		if result.Check == check {
			for _, finding := range result.Findings {
				return finding.Detail
			}
		}
	}
	return ""
}
