package store

import (
	"context"
	"path/filepath"
	"testing"
)

// A database created before schema v15 has block rows without slugs. The
// upgrade must add the column without losing rows, and a rebuild must fill the
// slugs in for notes nobody edits.
func TestSchemaV15UpgradeFromV14(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "v14.sqlite")
	st, err := OpenSQLiteWithAssetStore(dbPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Headings", Body: "# Section One\n\nBody.\n"})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-v15 state: rows present, no slug values, version 14.
	for _, statement := range []string{
		`UPDATE document_blocks SET heading_slug = NULL`,
		`PRAGMA user_version = 14`,
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
		t.Fatalf("upgrade from v14: %v", err)
	}
	status, err := reopened.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version after upgrade: %d", status.SchemaVersion)
	}
	blocks, err := reopened.ListDocumentBlocks(ctx, doc.ID)
	if err != nil || len(blocks) != 2 {
		t.Fatalf("upgrade lost block rows: %d %v", len(blocks), err)
	}
	if blocks[0].HeadingSlug != "" {
		t.Fatal("the upgrade must not invent slugs for rows it did not parse")
	}
	if err := reopened.RebuildDocumentBlocks(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := reopened.ListDocumentBlocks(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt[0].HeadingSlug != "section-one" {
		t.Fatalf("rebuild should fill the slug in: %+v", rebuilt[0])
	}
}
