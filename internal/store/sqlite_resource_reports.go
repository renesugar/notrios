package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type computedPerceptualHash struct {
	Algorithm string
	Hash      string
}

// SetPerceptualHashHook installs the optional H5 hashing extension. Notrios
// ships without a hook, so production behavior is inert unless the embedding
// application explicitly supplies one.
func (s *SQLiteStore) SetPerceptualHashHook(hook PerceptualHashHook) error {
	if hook != nil {
		if _, err := normalizePerceptualAlgorithm(hook.Algorithm()); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.perceptualHashHook = hook
	return nil
}

func normalizePerceptualAlgorithm(value string) (string, error) {
	algorithm := strings.ToLower(strings.TrimSpace(value))
	if algorithm == "" {
		return "", fmt.Errorf("%w: perceptual hook algorithm is required", ErrInvalidInput)
	}
	if algorithm == "sha256" {
		return "", fmt.Errorf("%w: sha256 is reserved for exact hashes", ErrInvalidInput)
	}
	for _, char := range algorithm {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return "", fmt.Errorf("%w: perceptual hook algorithm %q contains an unsupported character", ErrInvalidInput, value)
		}
	}
	return algorithm, nil
}

func normalizePerceptualHash(value string) (string, error) {
	hash := strings.ToLower(strings.TrimSpace(value))
	if hash == "" {
		return "", fmt.Errorf("%w: perceptual hook returned an empty hash", ErrInvalidInput)
	}
	if len(hash) > 4096 {
		return "", fmt.Errorf("%w: perceptual hook hash exceeds 4096 bytes", ErrInvalidInput)
	}
	return hash, nil
}

func (s *SQLiteStore) computePerceptualHash(ctx context.Context, blob storedBlob, mimeType string) (*computedPerceptualHash, error) {
	s.mu.Lock()
	hook := s.perceptualHashHook
	s.mu.Unlock()
	if hook == nil || !hook.SupportsMIME(mimeType) {
		return nil, nil
	}
	algorithm, err := normalizePerceptualAlgorithm(hook.Algorithm())
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	existing, found, err := s.resourceHashLocked(blob.SHA256, algorithm)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if found {
		return &computedPerceptualHash{Algorithm: algorithm, Hash: existing}, nil
	}

	content, err := os.Open(filepath.Join(s.assetRoot, blob.StoragePath))
	if err != nil {
		return nil, fmt.Errorf("open blob for perceptual hash: %w", err)
	}
	defer content.Close()
	value, err := hook.Compute(ctx, content, mimeType)
	if err != nil {
		return nil, fmt.Errorf("compute %s perceptual hash: %w", algorithm, err)
	}
	hash, err := normalizePerceptualHash(value)
	if err != nil {
		return nil, err
	}
	return &computedPerceptualHash{Algorithm: algorithm, Hash: hash}, nil
}

func (s *SQLiteStore) resourceHashLocked(blobSHA256, algorithm string) (string, bool, error) {
	stmt, err := s.prepareLocked(`SELECT hash FROM resource_hashes WHERE blob_sha256 = ? AND algo = ?`)
	if err != nil {
		return "", false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{blobSHA256, algorithm}); err != nil {
		return "", false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return columnText(stmt, 0), true, nil
	case C.SQLITE_DONE:
		return "", false, nil
	default:
		return "", false, s.stepErrLocked(rc)
	}
}

// checkPerceptualReviewRuleLocked is deliberately a validation-only slot:
// perceptual matches can be review signals but can never block admission.
func (s *SQLiteStore) checkPerceptualReviewRuleLocked(algorithm, hash string) error {
	_, err := s.countLocked(`SELECT COUNT(*) FROM media_hash_rules
		WHERE algo = ? AND hash = ? AND kind = 'perceptual' AND action = 'review'`,
		algorithm, hash)
	return err
}

// ResourceReport returns exact duplicate groups, physical blobs that have no
// document references, direct per-notebook live-note usage, and optional
// perceptual review signals.
func (s *SQLiteStore) ResourceReport(ctx context.Context) (ResourceReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return ResourceReport{}, err
	}

	s.mu.Lock()
	report, candidates, resourcesByBlob, hook, err := s.resourceReportLocked()
	s.mu.Unlock()
	if err != nil {
		return ResourceReport{}, err
	}
	if hook == nil {
		return report, nil
	}

	suggestions, err := hook.SuggestNearDuplicates(ctx, candidates)
	if err != nil {
		return ResourceReport{}, fmt.Errorf("suggest near duplicates with %s: %w", report.Perceptual.Algorithm, err)
	}
	nearDuplicates, err := validateNearDuplicateSuggestions(report.Perceptual.Algorithm, suggestions, resourcesByBlob)
	if err != nil {
		return ResourceReport{}, err
	}
	report.Perceptual.NearDuplicates = nearDuplicates
	return report, nil
}

func (s *SQLiteStore) resourceReportLocked() (ResourceReport, []PerceptualHashCandidate, map[string][]string, PerceptualHashHook, error) {
	report := ResourceReport{
		ExactDuplicates:   []ExactDuplicateGroup{},
		UnreferencedBlobs: []UnreferencedBlob{},
		NotebookUsage:     []NotebookResourceUsage{},
		Perceptual: PerceptualHashReport{
			PolicyReviews:  []PerceptualPolicyReview{},
			NearDuplicates: []NearDuplicateReview{},
		},
	}
	if err := s.loadExactDuplicatesLocked(&report); err != nil {
		return report, nil, nil, nil, err
	}
	if err := s.loadUnreferencedBlobsLocked(&report); err != nil {
		return report, nil, nil, nil, err
	}
	if err := s.loadNotebookResourceUsageLocked(&report); err != nil {
		return report, nil, nil, nil, err
	}
	candidates, resourcesByBlob, err := s.loadPerceptualReportLocked(&report)
	if err != nil {
		return report, nil, nil, nil, err
	}
	hook := s.perceptualHashHook
	if hook != nil {
		algorithm, err := normalizePerceptualAlgorithm(hook.Algorithm())
		if err != nil {
			return report, nil, nil, nil, err
		}
		report.Perceptual.HookEnabled = true
		report.Perceptual.Algorithm = algorithm
		candidates, resourcesByBlob, err = s.loadPerceptualCandidatesLocked(algorithm)
		if err != nil {
			return report, nil, nil, nil, err
		}
	}
	return report, candidates, resourcesByBlob, hook, nil
}

func (s *SQLiteStore) loadExactDuplicatesLocked(report *ResourceReport) error {
	stmt, err := s.prepareLocked(`SELECT b.sha256, b.mime_type, b.size_bytes,
			(SELECT COUNT(*) FROM resources r WHERE r.blob_sha256 = b.sha256),
			(SELECT COUNT(*) FROM document_resource_refs rr JOIN resources r ON r.id = rr.resource_id WHERE r.blob_sha256 = b.sha256)
		FROM blobs b
		WHERE (SELECT COUNT(*) FROM resources r WHERE r.blob_sha256 = b.sha256) > 1
		ORDER BY b.sha256`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return nil
		}
		if rc != C.SQLITE_ROW {
			return s.stepErrLocked(rc)
		}
		group := ExactDuplicateGroup{
			SHA256:         columnText(stmt, 0),
			MIMEType:       columnText(stmt, 1),
			SizeBytes:      columnInt64(stmt, 2),
			ResourceCount:  int(columnInt64(stmt, 3)),
			ReferenceCount: int(columnInt64(stmt, 4)),
			CollectionIDs:  []string{},
			Resources:      []ResourceReportItem{},
		}
		items, err := s.resourceReportItemsForBlobLocked(group.SHA256)
		if err != nil {
			return err
		}
		group.Resources = items
		collections := map[string]struct{}{}
		for _, item := range items {
			collections[item.Resource.CollectionID] = struct{}{}
		}
		for collectionID := range collections {
			group.CollectionIDs = append(group.CollectionIDs, collectionID)
		}
		sort.Strings(group.CollectionIDs)
		group.CrossCollection = len(group.CollectionIDs) > 1
		report.ExactDuplicates = append(report.ExactDuplicates, group)
	}
}

func (s *SQLiteStore) loadUnreferencedBlobsLocked(report *ResourceReport) error {
	stmt, err := s.prepareLocked(`SELECT b.sha256, b.mime_type, b.size_bytes
		FROM blobs b
		WHERE NOT EXISTS (
			SELECT 1
			FROM resources r
			JOIN document_resource_refs rr ON rr.resource_id = r.id
			WHERE r.blob_sha256 = b.sha256
		)
		ORDER BY b.sha256`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return nil
		}
		if rc != C.SQLITE_ROW {
			return s.stepErrLocked(rc)
		}
		blob := UnreferencedBlob{
			SHA256:    columnText(stmt, 0),
			MIMEType:  columnText(stmt, 1),
			SizeBytes: columnInt64(stmt, 2),
			Resources: []Resource{},
		}
		items, err := s.resourceReportItemsForBlobLocked(blob.SHA256)
		if err != nil {
			return err
		}
		for _, item := range items {
			blob.Resources = append(blob.Resources, item.Resource)
		}
		report.UnreferencedBlobs = append(report.UnreferencedBlobs, blob)
	}
}

func (s *SQLiteStore) resourceReportItemsForBlobLocked(blobSHA256 string) ([]ResourceReportItem, error) {
	stmt, err := s.prepareLocked(`SELECT r.id, r.collection_id, COALESCE(r.filename, ''), r.mime_type,
			b.size_bytes, b.sha256, r.created_at, COUNT(rr.resource_id)
		FROM resources r
		JOIN blobs b ON b.sha256 = r.blob_sha256
		LEFT JOIN document_resource_refs rr ON rr.resource_id = r.id
		WHERE r.blob_sha256 = ?
		GROUP BY r.id, r.collection_id, r.filename, r.mime_type, b.size_bytes, b.sha256, r.created_at
		ORDER BY r.collection_id, r.id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{blobSHA256}); err != nil {
		return nil, err
	}
	items := []ResourceReportItem{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return items, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
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
		items = append(items, ResourceReportItem{
			Resource:       resource,
			ReferenceCount: int(columnInt64(stmt, 7)),
		})
	}
}

func (s *SQLiteStore) loadNotebookResourceUsageLocked(report *ResourceReport) error {
	stmt, err := s.prepareLocked(`SELECT n.id, n.name,
			(SELECT COUNT(DISTINCT d.id)
			 FROM documents d JOIN document_resource_refs rr ON rr.document_id = d.id
			 WHERE d.notebook_id = n.id AND d.deleted_at IS NULL),
			(SELECT COUNT(*)
			 FROM documents d JOIN document_resource_refs rr ON rr.document_id = d.id
			 WHERE d.notebook_id = n.id AND d.deleted_at IS NULL),
			(SELECT COUNT(DISTINCT rr.resource_id)
			 FROM documents d JOIN document_resource_refs rr ON rr.document_id = d.id
			 WHERE d.notebook_id = n.id AND d.deleted_at IS NULL),
			(SELECT COUNT(DISTINCT r.blob_sha256)
			 FROM documents d
			 JOIN document_resource_refs rr ON rr.document_id = d.id
			 JOIN resources r ON r.id = rr.resource_id
			 WHERE d.notebook_id = n.id AND d.deleted_at IS NULL),
			COALESCE((SELECT SUM(b.size_bytes)
			 FROM documents d
			 JOIN document_resource_refs rr ON rr.document_id = d.id
			 JOIN resources r ON r.id = rr.resource_id
			 JOIN blobs b ON b.sha256 = r.blob_sha256
			 WHERE d.notebook_id = n.id AND d.deleted_at IS NULL), 0),
			COALESCE((SELECT SUM(b.size_bytes)
			 FROM blobs b
			 WHERE EXISTS (
				SELECT 1 FROM documents d
				JOIN document_resource_refs rr ON rr.document_id = d.id
				JOIN resources r ON r.id = rr.resource_id
				WHERE d.notebook_id = n.id AND d.deleted_at IS NULL
				  AND r.blob_sha256 = b.sha256
			 )), 0)
		FROM notebooks n
		ORDER BY n.name COLLATE NOCASE, n.id`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return nil
		}
		if rc != C.SQLITE_ROW {
			return s.stepErrLocked(rc)
		}
		report.NotebookUsage = append(report.NotebookUsage, NotebookResourceUsage{
			NotebookID:      columnText(stmt, 0),
			NotebookName:    columnText(stmt, 1),
			DocumentCount:   int(columnInt64(stmt, 2)),
			ReferenceCount:  int(columnInt64(stmt, 3)),
			ResourceCount:   int(columnInt64(stmt, 4)),
			UniqueBlobCount: int(columnInt64(stmt, 5)),
			ReferencedBytes: columnInt64(stmt, 6),
			UniqueBytes:     columnInt64(stmt, 7),
		})
	}
}

func (s *SQLiteStore) loadPerceptualReportLocked(report *ResourceReport) ([]PerceptualHashCandidate, map[string][]string, error) {
	count, err := s.countLocked(`SELECT COUNT(*) FROM resource_hashes WHERE algo <> 'sha256'`)
	if err != nil {
		return nil, nil, err
	}
	report.Perceptual.StoredHashes = int(count)

	stmt, err := s.prepareLocked(`SELECT rh.algo, rh.hash, rh.blob_sha256, COALESCE(mhr.reason, ''), r.id
		FROM resource_hashes rh
		JOIN media_hash_rules mhr ON mhr.algo = rh.algo AND mhr.hash = rh.hash
		JOIN resources r ON r.blob_sha256 = rh.blob_sha256
		WHERE mhr.kind = 'perceptual' AND mhr.action = 'review'
		ORDER BY rh.algo, rh.hash, rh.blob_sha256, r.id`)
	if err != nil {
		return nil, nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var current *PerceptualPolicyReview
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, nil, s.stepErrLocked(rc)
		}
		algorithm := columnText(stmt, 0)
		hash := columnText(stmt, 1)
		blobSHA256 := columnText(stmt, 2)
		if current == nil || current.Algorithm != algorithm || current.Hash != hash || current.BlobSHA256 != blobSHA256 {
			report.Perceptual.PolicyReviews = append(report.Perceptual.PolicyReviews, PerceptualPolicyReview{
				Algorithm:   algorithm,
				Hash:        hash,
				BlobSHA256:  blobSHA256,
				ResourceIDs: []string{},
				Reason:      columnText(stmt, 3),
			})
			current = &report.Perceptual.PolicyReviews[len(report.Perceptual.PolicyReviews)-1]
		}
		current.ResourceIDs = append(current.ResourceIDs, columnText(stmt, 4))
	}
	return []PerceptualHashCandidate{}, map[string][]string{}, nil
}

func (s *SQLiteStore) loadPerceptualCandidatesLocked(algorithm string) ([]PerceptualHashCandidate, map[string][]string, error) {
	stmt, err := s.prepareLocked(`SELECT rh.blob_sha256, rh.hash, b.mime_type
		FROM resource_hashes rh
		JOIN blobs b ON b.sha256 = rh.blob_sha256
		WHERE rh.algo = ?
		ORDER BY rh.blob_sha256`)
	if err != nil {
		return nil, nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{algorithm}); err != nil {
		return nil, nil, err
	}
	candidates := []PerceptualHashCandidate{}
	resourcesByBlob := map[string][]string{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return candidates, resourcesByBlob, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, nil, s.stepErrLocked(rc)
		}
		candidate := PerceptualHashCandidate{
			BlobSHA256: columnText(stmt, 0),
			Hash:       columnText(stmt, 1),
			MIMEType:   columnText(stmt, 2),
		}
		candidates = append(candidates, candidate)
		items, err := s.resourceReportItemsForBlobLocked(candidate.BlobSHA256)
		if err != nil {
			return nil, nil, err
		}
		for _, item := range items {
			resourcesByBlob[candidate.BlobSHA256] = append(resourcesByBlob[candidate.BlobSHA256], item.Resource.ID)
		}
	}
}

func validateNearDuplicateSuggestions(algorithm string, suggestions []PerceptualHashSuggestion, resourcesByBlob map[string][]string) ([]NearDuplicateReview, error) {
	reviews := []NearDuplicateReview{}
	seen := map[string]struct{}{}
	for _, suggestion := range suggestions {
		left := strings.TrimSpace(suggestion.LeftBlobSHA256)
		right := strings.TrimSpace(suggestion.RightBlobSHA256)
		leftResources, leftOK := resourcesByBlob[left]
		rightResources, rightOK := resourcesByBlob[right]
		if !leftOK || !rightOK {
			return nil, fmt.Errorf("%w: %s hook suggested an unknown blob pair", ErrInvalidInput, algorithm)
		}
		if left == right {
			continue
		}
		if math.IsNaN(suggestion.Distance) || math.IsInf(suggestion.Distance, 0) || suggestion.Distance < 0 {
			return nil, fmt.Errorf("%w: %s hook returned an invalid distance", ErrInvalidInput, algorithm)
		}
		if right < left {
			left, right = right, left
			leftResources, rightResources = rightResources, leftResources
		}
		key := left + "\x00" + right
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		reviews = append(reviews, NearDuplicateReview{
			Algorithm:        algorithm,
			LeftBlobSHA256:   left,
			RightBlobSHA256:  right,
			LeftResourceIDs:  append([]string(nil), leftResources...),
			RightResourceIDs: append([]string(nil), rightResources...),
			Distance:         suggestion.Distance,
			Reason:           strings.TrimSpace(suggestion.Reason),
		})
	}
	sort.Slice(reviews, func(i, j int) bool {
		if reviews[i].LeftBlobSHA256 != reviews[j].LeftBlobSHA256 {
			return reviews[i].LeftBlobSHA256 < reviews[j].LeftBlobSHA256
		}
		return reviews[i].RightBlobSHA256 < reviews[j].RightBlobSHA256
	})
	return reviews, nil
}
