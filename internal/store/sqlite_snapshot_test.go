package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteSnapshotImageIsConsistentAndSanitized(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source, err := OpenSQLiteWithAssetStore(filepath.Join(root, "source.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := source.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	document, err := source.CreateDocument(ctx, CreateDocumentRequest{Title: "snapshot canonical row", Body: "survives\n"})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Exec(ctx, `
		INSERT INTO batch_operations(request_key, operation, mode, response, request_sha256)
		VALUES('local-path', 'test', 'atomic', '/private/source/path', 'abc');
		INSERT INTO sync_apply_guard(singleton) VALUES(1);
		INSERT INTO import_checkpoints(source_system, source_key, collection_id, inventory_fingerprint, phase, status)
		VALUES('test', '/private/import', 'default', 'fingerprint', 'items', 'running');`); err != nil {
		t.Fatal(err)
	}

	imagePath := filepath.Join(root, "image.sqlite")
	state, err := source.CreateSQLiteSnapshotImage(ctx, imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != CurrentSchemaVersion || state.DatabaseID == "" || state.SourceReplicaID == "" {
		t.Fatalf("unexpected image state: %+v", state)
	}
	inspected, err := InspectSQLiteSnapshot(ctx, imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.DatabaseID != state.DatabaseID || inspected.SourceReplicaID != state.SourceReplicaID {
		t.Fatalf("inspected state mismatch: got %+v want %+v", inspected, state)
	}
	raw, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range [][]byte{[]byte("/private/source/path"), []byte("/private/import")} {
		if bytes.Contains(raw, privateValue) {
			t.Fatalf("sanitized image retained deleted private bytes %q", privateValue)
		}
	}
	image, err := OpenSQLiteSnapshotReadOnly(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	got, err := image.GetDocument(ctx, document.ID)
	if err != nil || got.Body != "survives\n" {
		t.Fatalf("canonical row did not survive: doc=%+v err=%v", got, err)
	}
	for _, table := range []string{"batch_operations", "sync_apply_guard", "import_checkpoints"} {
		image.mu.Lock()
		count, countErr := image.countLocked("SELECT COUNT(*) FROM " + table)
		image.mu.Unlock()
		if countErr != nil || count != 0 {
			t.Fatalf("local table %s: count=%d err=%v", table, count, countErr)
		}
	}
}
