package archivev2

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// errInjected is the simulated fault. A crash cannot be raised in-process, so
// the harness fails a chosen call instead and then abandons the target exactly
// as a killed process would: whatever was already committed stays committed.
var errInjected = fmt.Errorf("injected fault")

// faultyTarget wraps a real store and fails the nth call to one method. It
// delegates everything else, so the rows written before the fault are the rows
// a real interruption would leave behind.
type faultyTarget struct {
	*store.SQLiteStore
	method string
	after  int
	calls  map[string]int
}

func newFaultyTarget(inner *store.SQLiteStore, method string, after int) *faultyTarget {
	return &faultyTarget{SQLiteStore: inner, method: method, after: after, calls: map[string]int{}}
}

// trip reports whether this call is the one that should fail.
func (f *faultyTarget) trip(method string) bool {
	if method != f.method {
		return false
	}
	f.calls[method]++
	return f.calls[method] > f.after
}

func (f *faultyTarget) ApplyRestoreRecords(ctx context.Context, batch store.RestoreRecords, additive bool) ([]store.RestoreConflict, error) {
	if f.trip("ApplyRestoreRecords") {
		return nil, errInjected
	}
	return f.SQLiteStore.ApplyRestoreRecords(ctx, batch, additive)
}

func (f *faultyTarget) AdmitRestoredBlob(ctx context.Context, expectedSHA256, mimeType string, content io.Reader) (int64, error) {
	if f.trip("AdmitRestoredBlob") {
		return 0, errInjected
	}
	return f.SQLiteStore.AdmitRestoredBlob(ctx, expectedSHA256, mimeType, content)
}

func (f *faultyTarget) AdmitRestoredSourceBundle(ctx context.Context, expectedSHA256 string, content io.Reader) (int64, string, error) {
	if f.trip("AdmitRestoredSourceBundle") {
		return 0, "", errInjected
	}
	return f.SQLiteStore.AdmitRestoredSourceBundle(ctx, expectedSHA256, content)
}

func (f *faultyTarget) FinalizeRestoredDocuments(ctx context.Context) error {
	if f.trip("FinalizeRestoredDocuments") {
		return errInjected
	}
	return f.SQLiteStore.FinalizeRestoredDocuments(ctx)
}

func (f *faultyTarget) AdoptDatabaseIdentity(ctx context.Context, databaseID string) (store.DatabaseIdentity, error) {
	if f.trip("AdoptDatabaseIdentity") {
		return store.DatabaseIdentity{}, errInjected
	}
	return f.SQLiteStore.AdoptDatabaseIdentity(ctx, databaseID)
}

func (f *faultyTarget) CompleteRestore(ctx context.Context) error {
	if f.trip("CompleteRestore") {
		return errInjected
	}
	return f.SQLiteStore.CompleteRestore(ctx)
}

var _ store.RestoreTarget = (*faultyTarget)(nil)

// TestInterruptedRestoreIsMarkedIncomplete injects a fault at each stage that
// commits canonical state and requires the same outcome every time: the
// restore fails, and the library it left behind says so.
//
// Every stage is covered rather than one, because the marker only earns trust
// if no stage can complete without it — a single sampled stage would leave the
// others free to regress.
func TestInterruptedRestoreIsMarkedIncomplete(t *testing.T) {
	stages := []struct {
		method string
		after  int
	}{
		{"ApplyRestoreRecords", 1},       // during containers/content
		{"AdmitRestoredBlob", 0},         // first resource blob
		{"AdmitRestoredSourceBundle", 0}, // first exact source bundle
		{"FinalizeRestoredDocuments", 0}, // titles and search index
		{"AdoptDatabaseIdentity", 0},     // identity not yet adopted
		{"CompleteRestore", 0},           // the marker write itself
	}
	for _, stage := range stages {
		t.Run(stage.method, func(t *testing.T) {
			ctx := context.Background()
			fixture := newExportFixture(t)
			archive := filepath.Join(t.TempDir(), "archive")
			exported, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive))
			if err != nil {
				t.Fatal(err)
			}

			inner := newEmptyStore(t)
			target := newFaultyTarget(inner, stage.method, stage.after)
			if _, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt}); err == nil {
				t.Fatal("a restore interrupted mid-way reported success")
			}

			marker, found, err := inner.PendingRestore(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatal("an interrupted restore left no marker; the partial library looks complete")
			}
			if marker.CommitSHA256 != exported.CommitSHA256 {
				t.Fatalf("marker names commit %s, archive is %s", marker.CommitSHA256, exported.CommitSHA256)
			}
			if marker.Intent != string(RestoreAdopt) {
				t.Fatalf("marker records intent %q", marker.Intent)
			}
		})
	}
}

// TestCompletedRestoreClearsTheMarker is the other half: a marker that is
// never cleared would condemn every healthy library, so the success path has
// to be proven too.
func TestCompletedRestoreClearsTheMarker(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	target := newEmptyStore(t)
	if _, err := Restore(ctx, target, archive, RestoreOptions{Intent: RestoreAdopt}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := target.PendingRestore(ctx); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("a completed restore left the library marked incomplete")
	}
}

// TestInterruptedRestoreRefusesToBeContinuedAndReplaceRecoversIt covers what a
// user does next. Adopting or merging into a half-restored library would blend
// two partial states into one that matches no archive; replace clears first,
// so it is the recovery path — and it must produce the same library a clean
// restore would.
func TestInterruptedRestoreRefusesToBeContinuedAndReplaceRecoversIt(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}

	inner := newEmptyStore(t)
	interrupted := newFaultyTarget(inner, "ApplyRestoreRecords", 2)
	if _, err := Restore(ctx, interrupted, archive, RestoreOptions{Intent: RestoreAdopt}); err == nil {
		t.Fatal("the interrupted restore reported success")
	}

	for _, intent := range []RestoreIntent{RestoreAdopt, RestoreMerge, RestoreFork} {
		_, err := Restore(ctx, inner, archive, RestoreOptions{Intent: intent})
		if err == nil {
			t.Fatalf("%s continued into an interrupted library", intent)
		}
		if !strings.Contains(err.Error(), "interrupted restore") {
			t.Fatalf("%s failed for an unrelated reason: %v", intent, err)
		}
	}

	// Replace clears the partial state and rebuilds.
	summary, err := Restore(ctx, inner, archive, RestoreOptions{Intent: RestoreReplace})
	if err != nil {
		t.Fatalf("replace could not recover an interrupted library: %v", err)
	}
	if _, found, err := inner.PendingRestore(ctx); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("recovery left the library still marked incomplete")
	}

	// And the recovered library matches a clean restore of the same archive,
	// with no residue from the interrupted attempt.
	clean := newEmptyStore(t)
	if _, err := Restore(ctx, clean, archive, RestoreOptions{Intent: RestoreAdopt}); err != nil {
		t.Fatal(err)
	}
	recovered := filepath.Join(t.TempDir(), "recovered")
	reference := filepath.Join(t.TempDir(), "reference")
	if _, err := Export(ctx, inner, recovered, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(ctx, clean, reference, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	if strings.Join(archivedObjects(t, recovered), "\n") != strings.Join(archivedObjects(t, reference), "\n") {
		t.Fatal("a recovered library differs from a cleanly restored one")
	}
	if summary.Intent != string(RestoreReplace) {
		t.Fatalf("recovery reported intent %q", summary.Intent)
	}
}

// TestRestoreRejectsObjectsThatChangeAfterVerification closes the window
// between the two passes. Verification reads every object, then restore reads
// them again; bit rot, a concurrent writer, or a shared filesystem can change
// one in between. Blob and bundle admission already re-hash, so this covers
// the note bodies, which are the content that matters most.
func TestRestoreRejectsObjectsThatChangeAfterVerification(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	archive := filepath.Join(t.TempDir(), "archive")
	if _, err := Export(ctx, fixture.store, archive, exportOptions(TargetFullArchive)); err != nil {
		t.Fatal(err)
	}
	manifest, err := readManifestFile(archive, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	run := &restoreRun{
		root:     archive,
		options:  RestoreOptions{Limits: DefaultLimits(), BatchSize: defaultRestoreBatch},
		manifest: manifest,
		source:   newPackSource(DefaultLimits()),
	}
	defer run.source.close()
	if err := run.buildObjectIndex(ctx); err != nil {
		t.Fatal(err)
	}
	defer run.closeObjectIndex()

	// Pick a note body, read it cleanly, then tamper with the object.
	var body IndexEntry
	for _, entry := range readIndexEntries(t, archive) {
		if entry.Kind == "blob" {
			body = entry
			break
		}
	}
	if body.SHA256 == "" {
		t.Fatal("archive carried no blob object")
	}
	reference := BlobReference{SHA256: body.SHA256, SizeBytes: body.SizeBytes}
	if _, err := run.readBlobText(reference); err != nil {
		t.Fatalf("an untampered object was rejected: %v", err)
	}

	path := filepath.Join(archive, filepath.FromSlash(body.Location.Path))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 1
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := run.readBlobText(reference)
	if err == nil {
		t.Fatalf("a tampered object was accepted into canonical state as %q", text)
	}
	if !strings.Contains(err.Error(), "changed after verification") {
		t.Fatalf("tampering was reported as something else: %v", err)
	}
}
