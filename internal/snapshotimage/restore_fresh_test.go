package snapshotimage_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/store"
)

// seedSnapshot makes a one-note library and photographs it.
func seedSnapshot(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dbPath := filepath.Join(root, "first.sqlite")
	assets := filepath.Join(root, "assets")

	st, err := store.OpenSQLiteWithAssetStore(dbPath, assets)
	if err != nil {
		t.Fatalf("opening the source library: %v", err)
	}
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrapping: %v", err)
	}
	if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Reed beds", Body: "dusk"}); err != nil {
		t.Fatalf("creating a note: %v", err)
	}
	snapshot := filepath.Join(root, "snapshot")
	if _, err := snapshotimage.Create(ctx, st, assets, snapshot, snapshotimage.CreateOptions{}); err != nil {
		st.Close()
		t.Fatalf("creating the snapshot: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("closing the source library: %v", err)
	}
	return snapshot, root
}

// TestAdoptIntoAFreshPathIsNotAnError is the defect. Restoring into a path with
// no library is the documented way to make a second replica, and is what
// `sync fetch-backup` tells a person to do next -- and it failed with
// `sqlite prepare: no such table: database_identity`, an internal error for a
// situation that is not an error at all.
func TestAdoptIntoAFreshPathIsNotAnError(t *testing.T) {
	snapshot, root := seedSnapshot(t)
	target := filepath.Join(root, "second.sqlite")

	report, err := snapshotimage.Restore(context.Background(), snapshot, snapshotimage.RestoreOptions{
		Intent: "adopt", TargetDatabase: target, TargetAssetRoot: filepath.Join(root, "second-assets"),
	})
	if err != nil {
		t.Fatalf("adopting into a fresh path: %v", err)
	}
	if report.NewReplicaID == "" {
		t.Error("a fresh replica was restored without a new replica id")
	}
	if report.DerivedDocuments != 1 {
		t.Errorf("restored %d documents, want 1", report.DerivedDocuments)
	}

	// The skipped safety step is reported rather than passed over in silence:
	// a caller has to be able to see that nothing was protected, and why that
	// was the right answer.
	if report.EmergencySnapshot != "" {
		t.Errorf("an emergency snapshot was taken of a library that does not exist: %q", report.EmergencySnapshot)
	}
	if report.EmergencySkipped == "" {
		t.Error("no emergency snapshot was taken and the report does not say why")
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("encoding the report: %v", err)
	}
	if !strings.Contains(string(encoded), "emergency_snapshot_skipped") {
		t.Errorf("the reported reason is not in the JSON a caller reads: %s", encoded)
	}
}

// TestReplaceIntoAFreshPathIsRefused keeps the distinction: adopt makes a
// replica where there was none, replace overwrites one that exists.
func TestReplaceIntoAFreshPathIsRefused(t *testing.T) {
	snapshot, root := seedSnapshot(t)
	target := filepath.Join(root, "third.sqlite")

	_, err := snapshotimage.Restore(context.Background(), snapshot, snapshotimage.RestoreOptions{
		Intent: "replace", TargetDatabase: target, TargetAssetRoot: filepath.Join(root, "third-assets"),
	})
	if err == nil {
		t.Fatal("replace was accepted against a path holding no library")
	}
	if !strings.Contains(err.Error(), "adopt") {
		t.Errorf("the refusal does not name the intent that would work: %v", err)
	}
}

// TestAFileThatIsNotALibraryIsNotOverwritten is the case worth being careful
// about: a probe that fails must not be read as "nothing here".
func TestAFileThatIsNotALibraryIsNotOverwritten(t *testing.T) {
	snapshot, root := seedSnapshot(t)
	target := filepath.Join(root, "somebody-elses.sqlite")
	const contents = "important data\n"
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing the decoy: %v", err)
	}

	if _, err := snapshotimage.Restore(context.Background(), snapshot, snapshotimage.RestoreOptions{
		Intent: "adopt", TargetDatabase: target, TargetAssetRoot: filepath.Join(root, "decoy-assets"),
	}); err == nil {
		t.Fatal("a file that is not a Notrios library was accepted as a restore target")
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading the decoy back: %v", err)
	}
	if string(after) != contents {
		t.Errorf("the file was modified: %q", string(after))
	}
}

// TestAnEmptyFileFromAFailedRunIsStillFresh guards the second attempt. The
// first failing run left a zero-byte database behind, so a stat-only check
// would call it a library and fail again the same way.
func TestAnEmptyFileFromAFailedRunIsStillFresh(t *testing.T) {
	snapshot, root := seedSnapshot(t)
	target := filepath.Join(root, "retried.sqlite")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatalf("writing the empty file: %v", err)
	}

	report, err := snapshotimage.Restore(context.Background(), snapshot, snapshotimage.RestoreOptions{
		Intent: "adopt", TargetDatabase: target, TargetAssetRoot: filepath.Join(root, "retried-assets"),
	})
	if err != nil {
		t.Fatalf("adopting over an empty file left by a failed run: %v", err)
	}
	if report.EmergencySkipped == "" {
		t.Error("an empty file was treated as a library worth protecting")
	}
}
