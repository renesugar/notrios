package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// A database created before schema v16 has no title or filename index. The
// upgrade adds both without touching a row, and the lookups that used to scan
// come back index-backed.
func TestSchemaV16UpgradeFromV15(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "v15.sqlite")
	st, err := OpenSQLiteWithAssetStore(dbPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "v16_doc", Title: "Kitchen Plan", Body: "Body.\n",
	}); err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-v16 state: the indexes absent, version 15.
	for _, statement := range []string{
		`DROP INDEX IF EXISTS documents_title_idx`,
		`DROP INDEX IF EXISTS resources_filename_idx`,
		`PRAGMA user_version = 15`,
	} {
		if err := st.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLiteWithAssetStore(dbPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatalf("upgrade from v15: %v", err)
	}
	status, err := reopened.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version after upgrade: %d", status.SchemaVersion)
	}
	doc, err := reopened.GetDocument(ctx, "v16_doc")
	if err != nil || doc.Title != "Kitchen Plan" {
		t.Fatalf("upgrade lost the note: %+v %v", doc, err)
	}
	suggestions, err := reopened.SuggestDocuments(ctx, DocumentSuggestionRequest{Query: "kit"})
	if err != nil || len(suggestions.Suggestions) != 1 {
		t.Fatalf("suggestions after upgrade: %+v %v", suggestions, err)
	}
}

// The index is not decoration: without it a title lookup is a full scan of the
// document table, once per link that names a note by title. These are the exact
// statements the resolver and the suggester run.
func TestTitleAndFilenameLookupsAreIndexBacked(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"suggestion prefix scan": `SELECT id, title, COALESCE(notebook_id, '') FROM documents
			WHERE collection_id = 'default' AND deleted_at IS NULL AND title LIKE 'kit%' ESCAPE '\'
			ORDER BY title COLLATE NOCASE, id LIMIT 12`,
		"link resolution by title": `SELECT id FROM documents
			WHERE collection_id = 'default' AND deleted_at IS NULL AND title = 'Kitchen' COLLATE NOCASE
			ORDER BY id LIMIT 2`,
		"link resolution by filename": `SELECT id FROM resources
			WHERE collection_id = 'default' AND filename = 'diagram.png' COLLATE NOCASE
			ORDER BY id LIMIT 2`,
	}
	for name, query := range cases {
		plan, err := st.explainQueryPlan(ctx, query)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		joined := strings.Join(plan, " ")
		if strings.Contains(joined, "SCAN documents") || strings.Contains(joined, "SCAN resources") {
			t.Fatalf("%s scans instead of searching: %v", name, plan)
		}
		if !strings.Contains(joined, "documents_title_idx") && !strings.Contains(joined, "resources_filename_idx") {
			t.Fatalf("%s does not use the v16 index: %v", name, plan)
		}
	}
}
