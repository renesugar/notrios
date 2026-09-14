package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestJ21CurrentLibraryRunsNoMigrationOnOpen fails if opening a library that
// is already at the current schema runs a migration step's work again.
//
// v1.0 J20-A found that every open did exactly that. The unguarded steps
// lowered user_version to 18, so every guarded step from V19 on ran again. At
// 382,206 notes that was about 12 s per open, and read-only commands rewrote
// every full-text mapping row.
//
// Neither expensive re-run changes a correct library, so correct data cannot
// reveal them. The test plants one value each step would repair:
//   - V28 rebuilds documents_fts_rowid from documents_fts, and would correct a
//     mapping row whose rowid is wrong.
//   - V22 backfills revisions whose content_sha256 is empty, and would fill it
//     in again.
//
// The planted values are deliberately wrong, and they exist only in this
// test's library. If either is repaired, the step ran.
func TestJ21CurrentLibraryRunsNoMigrationOnOpen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "notes.sqlite")

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		CollectionID: "default", NotebookID: DefaultNotebookID,
		Title: "Sentinel", Body: "a body the full-text index holds", BodyMIMEType: "text/markdown",
	}); err != nil {
		t.Fatal(err)
	}
	const shift = 1_000_000
	if err := st.Exec(ctx, `UPDATE documents_fts_rowid SET fts_rowid = fts_rowid + 1000000`); err != nil {
		t.Fatal(err)
	}
	if err := st.Exec(ctx, `UPDATE document_revisions SET content_sha256 = ''`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	reopened.mu.Lock()
	defer reopened.mu.Unlock()
	version, err := reopened.pragmaUserVersionLocked()
	if err != nil {
		t.Fatal(err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("user_version after reopening = %d, want %d", version, CurrentSchemaVersion)
	}
	shifted, err := reopened.countLocked(`SELECT count(*) FROM documents_fts_rowid WHERE fts_rowid >= 1000000`)
	if err != nil {
		t.Fatal(err)
	}
	if shifted != 1 {
		t.Errorf("schema v28's mapping rebuild ran again on open: the planted rowid (+%d) was repaired", shift)
	}
	blank, err := reopened.countLocked(`SELECT count(*) FROM document_revisions WHERE content_sha256 = ''`)
	if err != nil {
		t.Fatal(err)
	}
	if blank != 1 {
		t.Errorf("schema v22's revision backfill ran again on open: the planted empty content_sha256 was filled")
	}
}
