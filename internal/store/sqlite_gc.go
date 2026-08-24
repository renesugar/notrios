package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type gcBlobRemoval struct {
	SHA256      string
	StoragePath string
	SizeBytes   int64
}

// GarbageCollect plans or applies retention-aware deletion of unreferenced
// logical resources. Every candidate is rechecked inside the apply
// transaction; any surviving reference keeps both resource and blob.
func (s *SQLiteStore) GarbageCollect(ctx context.Context, req GarbageCollectionRequest) (GarbageCollectionReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return GarbageCollectionReport{}, err
	}
	now := req.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	req.Policy = normalizeGarbageCollectionPolicy(req.Policy)
	gate := req.Gate
	if gate == nil {
		gate = LocalRetentionGate{}
	}
	report := GarbageCollectionReport{
		DryRun: !req.Apply,
		AsOf:   now,
		Policy: GarbageCollectionPolicySummary{
			UnreferencedSeconds:   int64(req.Policy.UnreferencedFor / time.Second),
			PurgedResourceSeconds: int64(req.Policy.PurgedResourceFor / time.Second),
			Gate:                  retentionGateName(gate),
		},
		Eligible: []GarbageCollectionCandidate{},
		Retained: []GarbageCollectionCandidate{},
		Removed:  []GarbageCollectionCandidate{},
		Warnings: []string{},
	}

	s.mu.Lock()
	candidates, referencedCount, err := s.loadGarbageCollectionCandidatesLocked()
	s.mu.Unlock()
	if err != nil {
		return GarbageCollectionReport{}, err
	}
	report.ReferencedResourceCount = referencedCount

	for _, candidate := range candidates {
		retention := req.Policy.UnreferencedFor
		if candidate.UnreferencedReason == "purged_document" {
			retention = req.Policy.PurgedResourceFor
		}
		candidate.RetentionSeconds = int64(retention / time.Second)
		if candidate.UnreferencedAt.IsZero() {
			candidate.Decision = "retained_missing_unreferenced_timestamp"
			report.Retained = append(report.Retained, candidate)
			continue
		}
		candidate.EligibleAt = candidate.UnreferencedAt.Add(retention)
		if now.Before(candidate.EligibleAt) {
			candidate.Decision = "retained_retention_not_elapsed"
			report.Retained = append(report.Retained, candidate)
			continue
		}
		allowed, reason, err := gate.CanCollect(ctx, candidate)
		if err != nil {
			return GarbageCollectionReport{}, fmt.Errorf("retention gate for resource %s: %w", candidate.Resource.ID, err)
		}
		reason = strings.TrimSpace(reason)
		if !allowed {
			if reason == "" {
				reason = "retention_gate_denied"
			}
			candidate.Decision = "retained_" + reason
			report.Retained = append(report.Retained, candidate)
			continue
		}
		if reason == "" {
			reason = "retention_satisfied"
		}
		candidate.Decision = "eligible_" + reason
		report.Eligible = append(report.Eligible, candidate)
	}

	if !req.Apply || len(report.Eligible) == 0 {
		return report, nil
	}
	removed, retained, blobs, bytesRemoved, warnings, err := s.applyGarbageCollection(ctx, report.Eligible)
	if err != nil {
		return GarbageCollectionReport{}, err
	}
	report.Removed = removed
	report.Retained = append(report.Retained, retained...)
	report.BlobsRemoved = len(blobs)
	report.BytesRemoved = bytesRemoved
	report.Warnings = append(report.Warnings, warnings...)
	return report, nil
}

func normalizeGarbageCollectionPolicy(policy GarbageCollectionPolicy) GarbageCollectionPolicy {
	if policy.UnreferencedFor < 0 {
		policy.UnreferencedFor = 0
	}
	if policy.PurgedResourceFor < 0 {
		policy.PurgedResourceFor = 0
	}
	return policy
}

func retentionGateName(gate RetentionGate) string {
	switch gate.(type) {
	case LocalRetentionGate, *LocalRetentionGate:
		return "local"
	case SyncRetentionGate, *SyncRetentionGate:
		return "sync_ack_snapshot"
	default:
		return "custom"
	}
}

func (s *SQLiteStore) loadGarbageCollectionCandidatesLocked() ([]GarbageCollectionCandidate, int, error) {
	referenced, err := s.countLocked(`SELECT COUNT(*) FROM resources r
		WHERE EXISTS (SELECT 1 FROM document_resource_refs rr WHERE rr.resource_id = r.id)`)
	if err != nil {
		return nil, 0, err
	}
	stmt, err := s.prepareLocked(`SELECT r.id, r.collection_id, COALESCE(r.filename, ''), r.mime_type,
			b.size_bytes, b.sha256, r.created_at, COALESCE(r.unreferenced_at, ''),
			COALESCE(r.unreferenced_reason, '')
		FROM resources r
		JOIN blobs b ON b.sha256 = r.blob_sha256
		WHERE NOT EXISTS (SELECT 1 FROM document_resource_refs rr WHERE rr.resource_id = r.id)
		ORDER BY COALESCE(r.unreferenced_at, ''), r.id`)
	if err != nil {
		return nil, 0, err
	}
	defer C.sqlite3_finalize(stmt)
	candidates := []GarbageCollectionCandidate{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return candidates, int(referenced), nil
		}
		if rc != C.SQLITE_ROW {
			return nil, 0, s.stepErrLocked(rc)
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
		unreferencedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 7)))
		resource := Resource{
			ID:           columnText(stmt, 0),
			CollectionID: columnText(stmt, 1),
			Filename:     columnText(stmt, 2),
			MIMEType:     columnText(stmt, 3),
			SizeBytes:    columnInt64(stmt, 4),
			SHA256:       columnText(stmt, 5),
			CreatedAt:    createdAt,
		}
		resource.URI = ResourceURI(resource.CollectionID, resource.ID)
		reason := columnText(stmt, 8)
		if reason == "" {
			reason = "unknown"
		}
		candidates = append(candidates, GarbageCollectionCandidate{
			Resource:           resource,
			UnreferencedAt:     unreferencedAt,
			UnreferencedReason: reason,
		})
	}
}

func (s *SQLiteStore) applyGarbageCollection(ctx context.Context, eligible []GarbageCollectionCandidate) ([]GarbageCollectionCandidate, []GarbageCollectionCandidate, []gcBlobRemoval, int64, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, 0, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return nil, nil, nil, 0, nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	removed := []GarbageCollectionCandidate{}
	retained := []GarbageCollectionCandidate{}
	blobs := []gcBlobRemoval{}
	for _, candidate := range eligible {
		current, storagePath, refCount, err := s.garbageCollectionStateLocked(candidate.Resource.ID)
		if err != nil {
			if err == ErrNotFound {
				candidate.Decision = "retained_state_changed_before_apply"
				retained = append(retained, candidate)
				continue
			}
			return nil, nil, nil, 0, nil, err
		}
		if refCount != 0 || !current.UnreferencedAt.Equal(candidate.UnreferencedAt) ||
			current.UnreferencedReason != candidate.UnreferencedReason ||
			current.Resource.SHA256 != candidate.Resource.SHA256 {
			candidate.Decision = "retained_state_changed_before_apply"
			retained = append(retained, candidate)
			continue
		}
		if err := s.execPreparedLocked(`DELETE FROM resources
			WHERE id = ? AND NOT EXISTS (
				SELECT 1 FROM document_resource_refs rr WHERE rr.resource_id = resources.id
			)`, candidate.Resource.ID); err != nil {
			return nil, nil, nil, 0, nil, err
		}
		candidate.Decision = "removed_retention_satisfied"
		removed = append(removed, candidate)

		resourceCount, err := s.countLocked(`SELECT COUNT(*) FROM resources WHERE blob_sha256 = ?`, candidate.Resource.SHA256)
		if err != nil {
			return nil, nil, nil, 0, nil, err
		}
		if resourceCount != 0 {
			continue
		}
		if err := s.execPreparedLocked(`DELETE FROM resource_hashes WHERE blob_sha256 = ?`, candidate.Resource.SHA256); err != nil {
			return nil, nil, nil, 0, nil, err
		}
		if err := s.execPreparedLocked(`DELETE FROM blobs WHERE sha256 = ?`, candidate.Resource.SHA256); err != nil {
			return nil, nil, nil, 0, nil, err
		}
		// The G8 transfer state describes an object that no longer exists.
		// Leaving it would make a later admission of the same bytes think it
		// had already fetched chunks it no longer holds.
		for _, table := range []string{"sync_blob_chunks", "sync_blob_manifests", "sync_blob_sources", "sync_blob_materialization"} {
			if err := s.execPreparedLocked(`DELETE FROM `+table+` WHERE blob_sha256 = ?`, candidate.Resource.SHA256); err != nil {
				return nil, nil, nil, 0, nil, err
			}
		}
		blobs = append(blobs, gcBlobRemoval{
			SHA256:      candidate.Resource.SHA256,
			StoragePath: storagePath,
			SizeBytes:   candidate.Resource.SizeBytes,
		})
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return nil, nil, nil, 0, nil, err
	}
	committed = true

	// Keep the store mutex until every committed blob row has had its physical
	// file removed. Otherwise a concurrent admission of the same SHA could
	// recreate the row between COMMIT and unlink, then lose its new backing
	// file.
	var bytesRemoved int64
	warnings := []string{}
	for _, blob := range blobs {
		if strings.TrimSpace(blob.StoragePath) == "" {
			// A blob admitted from a peer but never materialized has no file
			// to unlink. It is an ordinary state rather than a suspicious one,
			// so it produces no warning — only the partial transfer it may
			// have staged needs clearing.
			if err := os.RemoveAll(filepath.Join(s.assetRoot, stagingDirectory, blob.SHA256)); err != nil && !os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("clear staged chunks for %s: %v", blob.SHA256, err))
			}
			continue
		}
		path, ok := safeAssetPath(s.assetRoot, blob.StoragePath)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("blob %s has unsafe storage path; database row removed but file was not touched", blob.SHA256))
			continue
		}
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("remove blob %s: %v", blob.SHA256, err))
			}
			continue
		}
		bytesRemoved += blob.SizeBytes
	}
	return removed, retained, blobs, bytesRemoved, warnings, nil
}

func (s *SQLiteStore) garbageCollectionStateLocked(resourceID string) (GarbageCollectionCandidate, string, int, error) {
	stmt, err := s.prepareLocked(`SELECT r.id, r.collection_id, COALESCE(r.filename, ''), r.mime_type,
			b.size_bytes, b.sha256, r.created_at, COALESCE(r.unreferenced_at, ''),
			COALESCE(r.unreferenced_reason, ''), b.storage_path,
			(SELECT COUNT(*) FROM document_resource_refs rr WHERE rr.resource_id = r.id)
		FROM resources r JOIN blobs b ON b.sha256 = r.blob_sha256
		WHERE r.id = ?`)
	if err != nil {
		return GarbageCollectionCandidate{}, "", 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{resourceID}); err != nil {
		return GarbageCollectionCandidate{}, "", 0, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return GarbageCollectionCandidate{}, "", 0, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return GarbageCollectionCandidate{}, "", 0, s.stepErrLocked(rc)
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
	unreferencedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 7)))
	resource := Resource{
		ID:           columnText(stmt, 0),
		CollectionID: columnText(stmt, 1),
		Filename:     columnText(stmt, 2),
		MIMEType:     columnText(stmt, 3),
		SizeBytes:    columnInt64(stmt, 4),
		SHA256:       columnText(stmt, 5),
		CreatedAt:    createdAt,
	}
	resource.URI = ResourceURI(resource.CollectionID, resource.ID)
	return GarbageCollectionCandidate{
		Resource:           resource,
		UnreferencedAt:     unreferencedAt,
		UnreferencedReason: columnText(stmt, 8),
	}, columnText(stmt, 9), int(columnInt64(stmt, 10)), nil
}

func safeAssetPath(root, relative string) (string, bool) {
	clean := filepath.Clean(strings.TrimSpace(relative))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(root, clean), true
}
