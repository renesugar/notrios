package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchemaV8UpgradeBackfillsOnlyUnreferencedResources(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade-v8.sqlite")
	st, err := OpenSQLiteWithAssetStore(path, t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	unreferenced, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_v8_old", Content: strings.NewReader("old orphan")})
	if err != nil {
		t.Fatalf("CreateResource unreferenced: %v", err)
	}
	referenced, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_v8_ref", Content: strings.NewReader("old referenced")})
	if err != nil {
		t.Fatalf("CreateResource referenced: %v", err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_v8_ref", Title: "Reference"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: doc.ID, ResourceID: referenced.ID}); err != nil {
		t.Fatalf("AttachDocumentResource: %v", err)
	}
	for _, statement := range []string{
		`DROP INDEX resources_unreferenced_idx;`,
		`ALTER TABLE resources DROP COLUMN unreferenced_reason;`,
		`ALTER TABLE resources DROP COLUMN unreferenced_at;`,
		`PRAGMA user_version = 7;`,
	} {
		if err := st.Exec(ctx, statement); err != nil {
			t.Fatalf("rewind %q: %v", statement, err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	st, err = OpenSQLiteWithAssetStore(path, t.TempDir())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap after rewind: %v", err)
	}
	status, err := st.Status(ctx)
	if err != nil || status.SchemaVersion != 8 {
		t.Fatalf("schema status: %+v err=%v", status, err)
	}
	report, err := st.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{UnreferencedFor: 365 * 24 * time.Hour},
		Now:    unreferenced.CreatedAt.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}
	if len(report.Retained) != 1 || report.Retained[0].Resource.ID != unreferenced.ID ||
		report.Retained[0].UnreferencedReason != "legacy_unreferenced" ||
		report.ReferencedResourceCount != 1 {
		t.Fatalf("upgrade backfill report: %+v", report)
	}
}

func TestGarbageCollectionDryRunThenApply(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_gc", Filename: "gc.txt", MIMEType: "text/plain", Content: strings.NewReader("collect me"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}

	initial, err := st.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{UnreferencedFor: 30 * 24 * time.Hour, PurgedResourceFor: 90 * 24 * time.Hour},
		Now:    resource.CreatedAt.Add(29 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GarbageCollect retained: %v", err)
	}
	if !initial.DryRun || len(initial.Eligible) != 0 || len(initial.Retained) != 1 ||
		initial.Retained[0].Decision != "retained_retention_not_elapsed" {
		t.Fatalf("unexpected pre-retention report: %+v", initial)
	}

	dryRun, err := st.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{UnreferencedFor: 30 * 24 * time.Hour, PurgedResourceFor: 90 * 24 * time.Hour},
		Now:    resource.CreatedAt.Add(31 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GarbageCollect dry run: %v", err)
	}
	if !dryRun.DryRun || len(dryRun.Eligible) != 1 || dryRun.Eligible[0].Resource.ID != resource.ID || len(dryRun.Removed) != 0 {
		t.Fatalf("unexpected dry-run report: %+v", dryRun)
	}
	if _, err := st.GetResource(ctx, resource.ID); err != nil {
		t.Fatalf("dry run removed resource: %v", err)
	}

	applied, err := st.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{UnreferencedFor: 30 * 24 * time.Hour, PurgedResourceFor: 90 * 24 * time.Hour},
		Now:    resource.CreatedAt.Add(31 * 24 * time.Hour),
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("GarbageCollect apply: %v", err)
	}
	if applied.DryRun || len(applied.Removed) != 1 || applied.Removed[0].Resource.ID != resource.ID ||
		applied.BlobsRemoved != 1 || applied.BytesRemoved != int64(len("collect me")) {
		t.Fatalf("unexpected apply report: %+v", applied)
	}
	if _, err := st.GetResource(ctx, resource.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resource should be removed, got %v", err)
	}
}

func TestGarbageCollectionReferencesTrashAndPurgeRetention(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	first, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_gc_first", Title: "First"})
	if err != nil {
		t.Fatalf("CreateDocument first: %v", err)
	}
	second, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_gc_second", Title: "Second"})
	if err != nil {
		t.Fatalf("CreateDocument second: %v", err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_gc_shared_ref", Filename: "shared.txt", Content: strings.NewReader("shared reference"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	for _, documentID := range []string{first.ID, second.ID} {
		if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: documentID, ResourceID: resource.ID}); err != nil {
			t.Fatalf("AttachDocumentResource %s: %v", documentID, err)
		}
	}
	if err := st.DetachDocumentResource(ctx, first.ID, resource.ID); err != nil {
		t.Fatalf("DetachDocumentResource first: %v", err)
	}

	policy := GarbageCollectionPolicy{UnreferencedFor: 30 * 24 * time.Hour, PurgedResourceFor: 90 * 24 * time.Hour}
	report, err := st.GarbageCollect(ctx, GarbageCollectionRequest{Policy: policy, Now: time.Now().UTC().Add(365 * 24 * time.Hour)})
	if err != nil {
		t.Fatalf("GarbageCollect with surviving reference: %v", err)
	}
	if len(report.Eligible) != 0 || len(report.Retained) != 0 || report.ReferencedResourceCount != 1 {
		t.Fatalf("surviving reference must protect resource: %+v", report)
	}

	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: second.ID, BaseRevisionID: second.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	report, err = st.GarbageCollect(ctx, GarbageCollectionRequest{Policy: policy, Now: time.Now().UTC().Add(365 * 24 * time.Hour)})
	if err != nil {
		t.Fatalf("GarbageCollect with trash reference: %v", err)
	}
	if len(report.Eligible) != 0 || report.ReferencedResourceCount != 1 {
		t.Fatalf("trash reference must protect resource: %+v", report)
	}

	if err := st.PurgeDocument(ctx, second.ID); err != nil {
		t.Fatalf("PurgeDocument: %v", err)
	}
	afterPurge, err := st.GarbageCollect(ctx, GarbageCollectionRequest{Policy: policy, Now: time.Now().UTC().Add(31 * 24 * time.Hour)})
	if err != nil {
		t.Fatalf("GarbageCollect after purge: %v", err)
	}
	if len(afterPurge.Eligible) != 0 || len(afterPurge.Retained) != 1 ||
		afterPurge.Retained[0].UnreferencedReason != "purged_document" ||
		afterPurge.Retained[0].RetentionSeconds != int64((90*24*time.Hour)/time.Second) {
		t.Fatalf("purged resource must use longer retention: %+v", afterPurge)
	}
}

func TestGarbageCollectionKeepsSharedBlobWithReferencedResource(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_gc_blob", Title: "Blob owner"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	referenced, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_gc_referenced", Content: strings.NewReader("same blob")})
	if err != nil {
		t.Fatalf("CreateResource referenced: %v", err)
	}
	unreferenced, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_gc_unreferenced", Content: strings.NewReader("same blob")})
	if err != nil {
		t.Fatalf("CreateResource unreferenced: %v", err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: doc.ID, ResourceID: referenced.ID}); err != nil {
		t.Fatalf("AttachDocumentResource: %v", err)
	}

	report, err := st.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{},
		Now:    time.Now().UTC().Add(time.Hour),
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}
	if len(report.Removed) != 1 || report.Removed[0].Resource.ID != unreferenced.ID || report.BlobsRemoved != 0 || report.BytesRemoved != 0 {
		t.Fatalf("shared blob report: %+v", report)
	}
	if _, content, err := st.OpenResourceContent(ctx, referenced.ID); err != nil {
		t.Fatalf("referenced resource/blob must survive: %v", err)
	} else {
		_ = content.Close()
	}
}

type denyCollectionGate struct{}

func (denyCollectionGate) CanCollect(context.Context, GarbageCollectionCandidate) (bool, string, error) {
	return false, "peer_acknowledgement_pending", nil
}

func TestGarbageCollectionSyncAwareGateCanRetainExpiredCandidate(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_gc_gate", Content: strings.NewReader("gate")})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	report, err := st.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{},
		Now:    time.Now().UTC().Add(time.Hour),
		Apply:  true,
		Gate:   denyCollectionGate{},
	})
	if err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}
	if len(report.Removed) != 0 || len(report.Retained) != 1 ||
		report.Retained[0].Decision != "retained_peer_acknowledgement_pending" ||
		report.Policy.Gate != "custom" {
		t.Fatalf("gate report: %+v", report)
	}
	if _, err := st.GetResource(ctx, resource.ID); err != nil {
		t.Fatalf("gate must retain resource: %v", err)
	}
}

type attachDuringCollectionGate struct {
	store      *SQLiteStore
	documentID string
	resourceID string
}

func (g attachDuringCollectionGate) CanCollect(ctx context.Context, _ GarbageCollectionCandidate) (bool, string, error) {
	_, err := g.store.AttachDocumentResource(ctx, AttachResourceRequest{
		DocumentID: g.documentID,
		ResourceID: g.resourceID,
	})
	return true, "test_gate_allowed", err
}

func TestGarbageCollectionApplyRechecksReferencesAfterPlanning(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLiteWithAssetStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "doc_gc_race", Title: "Late reference"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{PreferredID: "res_gc_race", Content: strings.NewReader("race")})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	report, err := st.GarbageCollect(ctx, GarbageCollectionRequest{
		Policy: GarbageCollectionPolicy{},
		Now:    time.Now().UTC().Add(time.Hour),
		Apply:  true,
		Gate: attachDuringCollectionGate{
			store: st, documentID: doc.ID, resourceID: resource.ID,
		},
	})
	if err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}
	if len(report.Removed) != 0 || len(report.Retained) != 1 ||
		report.Retained[0].Decision != "retained_state_changed_before_apply" {
		t.Fatalf("late reference must win: %+v", report)
	}
	if refs, err := st.ListDocumentResources(ctx, doc.ID); err != nil || len(refs) != 1 {
		t.Fatalf("late reference missing: refs=%+v err=%v", refs, err)
	}
}
