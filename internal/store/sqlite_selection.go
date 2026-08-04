package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/query"
)

const selectionReadBatch = 400

type normalizedSelectionRequest struct {
	request            SelectionPlanRequest
	policy             EffectivePrivacyPolicy
	includeDescendants bool
	parsedQuery        query.Query
	hasQuery           bool
	unscopedFull       bool
}

// PlanSelection performs a content-free, read-only traversal of canonical
// SQLite state. The Store mutex supplies one in-process snapshot while the
// planner streams bounded ID batches; note bodies and resource bytes are never
// read. Counts and the digest cover the full selection even when REST/MCP-safe
// detail arrays are truncated.
func (s *SQLiteStore) PlanSelection(ctx context.Context, req SelectionPlanRequest) (SelectionPlan, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SelectionPlan{}, err
	}
	norm, err := normalizeSelectionPlanRequest(req)
	if err != nil {
		return SelectionPlan{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, _, err := s.planSelectionLocked(ctx, norm)
	return plan, err
}

func normalizeSelectionPlanRequest(req SelectionPlanRequest) (normalizedSelectionRequest, error) {
	req.Target = strings.ToLower(strings.TrimSpace(req.Target))
	switch req.Target {
	case SelectionTargetFullArchive, SelectionTargetSubsetTransfer, SelectionTargetPublicationHandoff:
	default:
		return normalizedSelectionRequest{}, fmt.Errorf("%w: target must be full_archive, subset_transfer, or publication_handoff", ErrInvalidInput)
	}
	req.Selection.CollectionID = strings.TrimSpace(req.Selection.CollectionID)
	if req.Selection.CollectionID == "" {
		req.Selection.CollectionID = "default"
	}
	req.Selection.Match = strings.ToLower(strings.TrimSpace(req.Selection.Match))
	if req.Selection.Match == "" {
		req.Selection.Match = "any"
	}
	if req.Selection.Match != "any" && req.Selection.Match != "all" {
		return normalizedSelectionRequest{}, fmt.Errorf("%w: selection.match must be any or all", ErrInvalidInput)
	}
	var err error
	if req.Selection.NotebookIDs, err = normalizedLimitedValues(req.Selection.NotebookIDs, MaxSelectionSelectors, false, "notebook_ids"); err != nil {
		return normalizedSelectionRequest{}, err
	}
	if req.Selection.Tags, err = normalizedLimitedValues(req.Selection.Tags, MaxSelectionSelectors, true, "tags"); err != nil {
		return normalizedSelectionRequest{}, err
	}
	if req.Selection.DocumentIDs, err = normalizedLimitedValues(req.Selection.DocumentIDs, MaxSelectionDocumentIDs, false, "document_ids"); err != nil {
		return normalizedSelectionRequest{}, err
	}
	req.Selection.Query = strings.TrimSpace(req.Selection.Query)
	hasSelectors := len(req.Selection.NotebookIDs) > 0 || len(req.Selection.Tags) > 0 || req.Selection.Query != "" || len(req.Selection.DocumentIDs) > 0
	if !hasSelectors && req.Target != SelectionTargetFullArchive {
		return normalizedSelectionRequest{}, fmt.Errorf("%w: subset and publication plans require at least one selector", ErrInvalidInput)
	}
	if req.DetailLimit <= 0 {
		req.DetailLimit = 100
	}
	if req.DetailLimit > MaxSelectionDetailItems {
		return normalizedSelectionRequest{}, fmt.Errorf("%w: detail_limit must be %d or less", ErrInvalidInput, MaxSelectionDetailItems)
	}
	if req.MaxDocuments <= 0 {
		req.MaxDocuments = 100_000
	}
	if req.MaxDocuments > MaxSelectionDocuments {
		return normalizedSelectionRequest{}, fmt.Errorf("%w: max_documents must be %d or less", ErrInvalidInput, MaxSelectionDocuments)
	}
	if req.Policy.MaxResourceBytes < 0 {
		return normalizedSelectionRequest{}, fmt.Errorf("%w: max_resource_bytes cannot be negative", ErrInvalidInput)
	}
	if req.Policy.ExcludeTags, err = normalizedLimitedValues(req.Policy.ExcludeTags, MaxSelectionSelectors, true, "policy.exclude_tags"); err != nil {
		return normalizedSelectionRequest{}, err
	}
	if req.Policy.PrivateTags, err = normalizedLimitedValues(req.Policy.PrivateTags, MaxSelectionSelectors, true, "policy.private_tags"); err != nil {
		return normalizedSelectionRequest{}, err
	}

	includeDescendants := true
	if req.Selection.IncludeNotebookDescendants != nil {
		includeDescendants = *req.Selection.IncludeNotebookDescendants
	}
	policy := defaultSelectionPolicy(req.Target)
	policy.ExcludeTags = req.Policy.ExcludeTags
	if len(req.Policy.PrivateTags) > 0 {
		policy.PrivateTags = req.Policy.PrivateTags
	}
	if action := strings.ToLower(strings.TrimSpace(req.Policy.LinkAction)); action != "" {
		if action != "retain" && action != "report" && action != "plain_text" && action != "redact" {
			return normalizedSelectionRequest{}, fmt.Errorf("%w: link_action must be retain, report, plain_text, or redact", ErrInvalidInput)
		}
		policy.LinkAction = action
	}
	if req.Policy.IncludeSourceBundles != nil {
		policy.IncludeSourceBundles = *req.Policy.IncludeSourceBundles
	}
	if req.Policy.IncludeProvenance != nil {
		policy.IncludeProvenance = *req.Policy.IncludeProvenance
	}
	if req.Policy.IncludePrivateMetadata != nil {
		policy.IncludePrivateMetadata = *req.Policy.IncludePrivateMetadata
	}
	if req.Policy.IncludeTrashed != nil {
		policy.IncludeTrashed = *req.Policy.IncludeTrashed
	}
	policy.MaxResourceBytes = req.Policy.MaxResourceBytes
	policy.ExcludeTags = sortedUnique(append(policy.ExcludeTags, policy.PrivateTags...))

	norm := normalizedSelectionRequest{request: req, policy: policy, includeDescendants: includeDescendants, unscopedFull: !hasSelectors && req.Target == SelectionTargetFullArchive}
	if req.Selection.Query != "" {
		parsed, parseErr := query.Parse(req.Selection.Query, time.Now())
		if parseErr != nil {
			return normalizedSelectionRequest{}, fmt.Errorf("%w: %v", ErrInvalidInput, parseErr)
		}
		if parsed.Trashed && !policy.IncludeTrashed {
			return normalizedSelectionRequest{}, fmt.Errorf("%w: trashed query requires include_trashed", ErrInvalidInput)
		}
		norm.parsedQuery = parsed
		norm.hasQuery = true
	}
	return norm, nil
}

func defaultSelectionPolicy(target string) EffectivePrivacyPolicy {
	switch target {
	case SelectionTargetFullArchive:
		return EffectivePrivacyPolicy{LinkAction: "retain", IncludeSourceBundles: true, IncludeProvenance: true, IncludePrivateMetadata: true, IncludeTrashed: true, ExcludeTags: []string{}, PrivateTags: []string{}}
	case SelectionTargetPublicationHandoff:
		private := []string{"confidential", "draft", "private"}
		return EffectivePrivacyPolicy{LinkAction: "plain_text", IncludeSourceBundles: false, IncludeProvenance: false, IncludePrivateMetadata: false, IncludeTrashed: false, ExcludeTags: append([]string(nil), private...), PrivateTags: private}
	default:
		return EffectivePrivacyPolicy{LinkAction: "report", IncludeSourceBundles: true, IncludeProvenance: true, IncludePrivateMetadata: false, IncludeTrashed: false, ExcludeTags: []string{}, PrivateTags: []string{}}
	}
}

func normalizedLimitedValues(values []string, max int, fold bool, field string) ([]string, error) {
	if len(values) > max {
		return nil, fmt.Errorf("%w: %s accepts at most %d values", ErrInvalidInput, field, max)
	}
	seen := map[string]string{}
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("%w: %s values cannot be empty", ErrInvalidInput, field)
		}
		if len(value) > MaxSelectionSelectorBytes {
			return nil, fmt.Errorf("%w: %s values must be %d bytes or less", ErrInvalidInput, field, MaxSelectionSelectorBytes)
		}
		key := value
		if fold {
			key = strings.ToLower(value)
			value = key
		}
		seen[key] = value
	}
	out := make([]string, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// planSelectionLocked produces the capped REST/MCP-safe plan plus the complete
// in-process identity sets an archive writer needs. Both come from one
// traversal so a dry run and an export cannot disagree.
func (s *SQLiteStore) planSelectionLocked(ctx context.Context, norm normalizedSelectionRequest) (SelectionPlan, SelectionResolution, error) {
	plan := SelectionPlan{
		Version: 1, Target: norm.request.Target, Policy: norm.policy,
		Documents: []SelectionDocumentManifest{}, Resources: []SelectionResourceManifest{},
		Links: []SelectionLinkDecision{}, SourceBundles: []SelectionSourceBundleManifest{},
		Exclusions: []SelectionExclusion{}, MetadataDecisions: []SelectionMetadataDecision{}, Warnings: []string{},
	}
	excludedDocumentCount := 0
	exclusionDigest := sha256.New()
	candidates := map[string]uint8{}
	categoryCount := 0
	var allMask uint8
	categoryReasons := []string{}
	addCategory := func(reason string, ids []string) error {
		if categoryCount >= 8 {
			return fmt.Errorf("%w: too many selector categories", ErrInvalidInput)
		}
		bit := uint8(1 << categoryCount)
		categoryCount++
		allMask |= bit
		categoryReasons = append(categoryReasons, reason)
		for _, id := range ids {
			candidates[id] |= bit
		}
		if len(candidates) > norm.request.MaxDocuments {
			return fmt.Errorf("%w: selection exceeds max_documents=%d", ErrInvalidInput, norm.request.MaxDocuments)
		}
		return nil
	}

	if norm.unscopedFull {
		ids, err := s.selectDocumentIDsLocked(ctx, norm.request.Selection.CollectionID, norm.policy.IncludeTrashed, "1", nil, norm.request.MaxDocuments)
		if err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
		if err := addCategory("full_archive", ids); err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
	}
	if len(norm.request.Selection.NotebookIDs) > 0 {
		notebookIDs := []string{}
		for _, id := range norm.request.Selection.NotebookIDs {
			if _, err := s.getNotebookLocked(id); err != nil {
				if err == ErrNotFound {
					return SelectionPlan{}, SelectionResolution{}, fmt.Errorf("%w: notebook %q was not found", ErrInvalidInput, id)
				}
				return SelectionPlan{}, SelectionResolution{}, err
			}
			if norm.includeDescendants {
				subtree, err := s.notebookSubtreeIDsLocked(id)
				if err != nil {
					return SelectionPlan{}, SelectionResolution{}, err
				}
				notebookIDs = append(notebookIDs, subtree...)
			} else {
				notebookIDs = append(notebookIDs, id)
			}
		}
		notebookIDs = uniqueStrings(notebookIDs)
		predicate := "d.notebook_id IN (" + placeholders(len(notebookIDs)) + ")"
		ids, err := s.selectDocumentIDsLocked(ctx, norm.request.Selection.CollectionID, norm.policy.IncludeTrashed, predicate, notebookIDs, norm.request.MaxDocuments)
		if err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
		if err := addCategory("notebook", ids); err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
	}
	if len(norm.request.Selection.Tags) > 0 {
		predicate := `d.id IN (SELECT nt.document_id FROM note_tags nt JOIN tags t ON t.id = nt.tag_id WHERE lower(t.name) IN (` + placeholders(len(norm.request.Selection.Tags)) + `))`
		ids, err := s.selectDocumentIDsLocked(ctx, norm.request.Selection.CollectionID, norm.policy.IncludeTrashed, predicate, norm.request.Selection.Tags, norm.request.MaxDocuments)
		if err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
		if err := addCategory("tag", ids); err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
	}
	if norm.hasQuery {
		predicate, args, err := s.compileSQLExprLocked(norm.parsedQuery.Root, norm.parsedQuery.Trashed)
		if err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
		ids, err := s.selectDocumentIDsWithTrashScopeLocked(ctx, norm.request.Selection.CollectionID, norm.parsedQuery.Trashed, predicate, args, norm.request.MaxDocuments)
		if err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
		if err := addCategory("query", ids); err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
	}
	if len(norm.request.Selection.DocumentIDs) > 0 {
		ids, missing, trashed, err := s.selectExplicitDocumentIDsLocked(ctx, norm.request.Selection.CollectionID, norm.request.Selection.DocumentIDs, norm.policy.IncludeTrashed)
		if err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
		for _, id := range missing {
			appendSelectionExclusion(&plan, norm.request.DetailLimit, &excludedDocumentCount, exclusionDigest, SelectionExclusion{Kind: "document", ID: id, Reason: "missing_document"})
		}
		for _, id := range trashed {
			appendSelectionExclusion(&plan, norm.request.DetailLimit, &excludedDocumentCount, exclusionDigest, SelectionExclusion{Kind: "document", ID: id, Reason: "trashed_not_allowed"})
		}
		if err := addCategory("document_id", ids); err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
	}

	if norm.request.Selection.Match == "all" && categoryCount > 1 {
		for _, id := range sortedCandidateIDs(candidates) {
			if candidates[id] != allMask {
				delete(candidates, id)
				appendSelectionExclusion(&plan, norm.request.DetailLimit, &excludedDocumentCount, exclusionDigest, SelectionExclusion{Kind: "document", ID: id, Reason: "did_not_match_all_selectors"})
			}
		}
	}
	if len(candidates) > norm.request.MaxDocuments {
		return SelectionPlan{}, SelectionResolution{}, fmt.Errorf("%w: selection exceeds max_documents=%d", ErrInvalidInput, norm.request.MaxDocuments)
	}

	selectedIDs := sortedCandidateIDs(candidates)
	if len(norm.policy.ExcludeTags) > 0 && len(selectedIDs) > 0 {
		excluded, err := s.documentsWithTagsLocked(ctx, selectedIDs, norm.policy.ExcludeTags)
		if err != nil {
			return SelectionPlan{}, SelectionResolution{}, err
		}
		excludedIDs := make([]string, 0, len(excluded))
		for id := range excluded {
			excludedIDs = append(excludedIDs, id)
		}
		sort.Strings(excludedIDs)
		for _, id := range excludedIDs {
			tag := excluded[id]
			delete(candidates, id)
			appendSelectionExclusion(&plan, norm.request.DetailLimit, &excludedDocumentCount, exclusionDigest, SelectionExclusion{Kind: "document", ID: id, Reason: "excluded_tag:" + tag})
		}
		selectedIDs = sortedCandidateIDs(candidates)
	}

	documents, err := s.loadSelectionDocumentsLocked(ctx, selectedIDs, candidates, categoryReasons)
	if err != nil {
		return SelectionPlan{}, SelectionResolution{}, err
	}
	plan.Counts.SelectedDocuments = len(documents)
	for _, document := range documents {
		if len(plan.Documents) < norm.request.DetailLimit {
			plan.Documents = append(plan.Documents, document)
		} else {
			plan.Truncated = true
		}
	}

	resources, err := s.loadSelectionResourcesLocked(ctx, selectedIDs, norm.policy.MaxResourceBytes)
	if err != nil {
		return SelectionPlan{}, SelectionResolution{}, err
	}
	plan.Counts.ReachableResources = len(resources)
	for _, resource := range resources {
		if resource.Oversized {
			plan.Counts.OversizedResources++
		}
		if len(plan.Resources) < norm.request.DetailLimit {
			plan.Resources = append(plan.Resources, resource)
		} else {
			plan.Truncated = true
		}
	}

	linkDigest := sha256.New()
	if err := s.scanSelectionLinksLocked(ctx, selectedIDs, candidates, norm.policy, norm.request.DetailLimit, &plan, linkDigest); err != nil {
		return SelectionPlan{}, SelectionResolution{}, err
	}

	bundles, bundleKeys, err := s.loadSelectionSourceBundlesLocked(ctx, selectedIDs, norm.unscopedFull)
	if err != nil {
		return SelectionPlan{}, SelectionResolution{}, err
	}
	plan.Counts.AvailableSourceBundles = len(bundles)
	if norm.policy.IncludeSourceBundles {
		plan.Counts.IncludedSourceBundles = len(bundles)
		for _, bundle := range bundles {
			if len(plan.SourceBundles) < norm.request.DetailLimit {
				plan.SourceBundles = append(plan.SourceBundles, bundle)
			} else {
				plan.Truncated = true
			}
		}
	}

	provenanceCount, err := s.countSelectionProvenanceLocked(ctx, selectedIDs)
	if err != nil {
		return SelectionPlan{}, SelectionResolution{}, err
	}
	plan.MetadataDecisions = selectionMetadataDecisions(norm.policy, provenanceCount, len(bundles), len(documents))
	plan.Counts.ExcludedDocuments = excludedDocumentCount
	plan.Warnings = selectionWarnings(plan)
	plan.ManifestSHA256 = selectionManifestDigest(norm, documents, resources, linkDigest, bundles, exclusionDigest, plan.MetadataDecisions)

	resolution := SelectionResolution{DocumentIDs: selectedIDs, ResourceIDs: make([]string, 0, len(resources))}
	for _, resource := range resources {
		resolution.ResourceIDs = append(resolution.ResourceIDs, resource.ID)
	}
	if norm.policy.IncludeSourceBundles {
		resolution.SourceBundleKeys = bundleKeys
	} else {
		resolution.SourceBundleKeys = []SourceBundleKey{}
	}
	return plan, resolution, nil
}

func appendSelectionExclusion(plan *SelectionPlan, detailLimit int, documentCount *int, digest hash.Hash, exclusion SelectionExclusion) {
	if exclusion.Kind == "document" {
		*documentCount = *documentCount + 1
	}
	writeDigestFields(digest, exclusion.Kind, exclusion.ID, exclusion.Reason)
	if len(plan.Exclusions) < detailLimit {
		plan.Exclusions = append(plan.Exclusions, exclusion)
	} else {
		plan.Truncated = true
	}
}

func sortedCandidateIDs(candidates map[string]uint8) []string {
	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func (s *SQLiteStore) selectDocumentIDsLocked(ctx context.Context, collectionID string, includeTrashed bool, predicate string, args []string, limit int) ([]string, error) {
	deletedScope := "d.deleted_at IS NULL"
	if includeTrashed {
		deletedScope = "1"
	}
	return s.selectDocumentIDsInScopeLocked(ctx, collectionID, deletedScope, predicate, args, limit)
}

func (s *SQLiteStore) selectDocumentIDsWithTrashScopeLocked(ctx context.Context, collectionID string, trashedOnly bool, predicate string, args []string, limit int) ([]string, error) {
	deleted := "d.deleted_at IS NULL"
	if trashedOnly {
		deleted = "d.deleted_at IS NOT NULL"
	}
	return s.selectDocumentIDsInScopeLocked(ctx, collectionID, deleted, predicate, args, limit)
}

func (s *SQLiteStore) selectDocumentIDsInScopeLocked(ctx context.Context, collectionID, deletedScope, predicate string, args []string, limit int) ([]string, error) {
	sql := `SELECT d.id FROM documents d JOIN document_revisions r ON r.id = d.current_revision_id
		WHERE d.collection_id = ? AND ` + deletedScope + ` AND (` + predicate + `)
		ORDER BY d.id LIMIT ` + itoa(limit+1)
	values := append([]string{collectionID}, args...)
	ids, err := s.readIDQueryLocked(ctx, sql, values)
	if err != nil {
		return nil, err
	}
	if len(ids) > limit {
		return nil, fmt.Errorf("%w: selection exceeds max_documents=%d", ErrInvalidInput, limit)
	}
	return ids, nil
}

func (s *SQLiteStore) readIDQueryLocked(ctx context.Context, sql string, args []string) ([]string, error) {
	stmt, err := s.prepareLocked(sql)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return nil, err
	}
	ids := []string{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			ids = append(ids, columnText(stmt, 0))
		case C.SQLITE_DONE:
			return ids, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) selectExplicitDocumentIDsLocked(ctx context.Context, collectionID string, requested []string, includeTrashed bool) (selected, missing, trashed []string, err error) {
	found := map[string]bool{}
	for start := 0; start < len(requested); start += selectionReadBatch {
		end := min(start+selectionReadBatch, len(requested))
		batch := requested[start:end]
		stmt, prepErr := s.prepareLocked(`SELECT id, deleted_at IS NOT NULL FROM documents WHERE collection_id = ? AND id IN (` + lookupPlaceholders(len(batch)) + `) ORDER BY id`)
		if prepErr != nil {
			return nil, nil, nil, prepErr
		}
		values := append([]string{collectionID}, batch...)
		if bindErr := bindAll(stmt, values); bindErr != nil {
			C.sqlite3_finalize(stmt)
			return nil, nil, nil, bindErr
		}
		for {
			if ctxErr := ctx.Err(); ctxErr != nil {
				C.sqlite3_finalize(stmt)
				return nil, nil, nil, ctxErr
			}
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				id := columnText(stmt, 0)
				found[id] = true
				if columnInt64(stmt, 1) != 0 && !includeTrashed {
					trashed = append(trashed, id)
				} else {
					selected = append(selected, id)
				}
				continue
			}
			if rc == C.SQLITE_DONE {
				break
			}
			stepErr := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return nil, nil, nil, stepErr
		}
		C.sqlite3_finalize(stmt)
	}
	for _, id := range requested {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(selected)
	sort.Strings(missing)
	sort.Strings(trashed)
	return selected, missing, trashed, nil
}

func (s *SQLiteStore) documentsWithTagsLocked(ctx context.Context, documentIDs, tags []string) (map[string]string, error) {
	result := map[string]string{}
	for start := 0; start < len(documentIDs); start += selectionReadBatch {
		end := min(start+selectionReadBatch, len(documentIDs))
		batch := documentIDs[start:end]
		sql := `SELECT nt.document_id, lower(t.name) FROM note_tags nt JOIN tags t ON t.id = nt.tag_id
			WHERE nt.document_id IN (` + lookupPlaceholders(len(batch)) + `)
			AND lower(t.name) IN (` + placeholders(len(tags)) + `)
			ORDER BY nt.document_id, lower(t.name)`
		stmt, err := s.prepareLocked(sql)
		if err != nil {
			return nil, err
		}
		values := append(append([]string{}, batch...), tags...)
		if err := bindAll(stmt, values); err != nil {
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		for {
			if err := ctx.Err(); err != nil {
				C.sqlite3_finalize(stmt)
				return nil, err
			}
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				id, tag := columnText(stmt, 0), columnText(stmt, 1)
				if _, exists := result[id]; !exists {
					result[id] = tag
				}
				continue
			}
			if rc == C.SQLITE_DONE {
				break
			}
			err := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		C.sqlite3_finalize(stmt)
	}
	return result, nil
}

func (s *SQLiteStore) loadSelectionDocumentsLocked(ctx context.Context, ids []string, candidates map[string]uint8, categoryReasons []string) ([]SelectionDocumentManifest, error) {
	documents := make([]SelectionDocumentManifest, 0, len(ids))
	for start := 0; start < len(ids); start += selectionReadBatch {
		end := min(start+selectionReadBatch, len(ids))
		batch := ids[start:end]
		stmt, err := s.prepareLocked(`SELECT id, collection_id, COALESCE(notebook_id, ''), current_revision_id, deleted_at IS NOT NULL
			FROM documents WHERE id IN (` + lookupPlaceholders(len(batch)) + `) ORDER BY id`)
		if err != nil {
			return nil, err
		}
		if err := bindAll(stmt, batch); err != nil {
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		for {
			if err := ctx.Err(); err != nil {
				C.sqlite3_finalize(stmt)
				return nil, err
			}
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				id := columnText(stmt, 0)
				reasons := []string{}
				for index, reason := range categoryReasons {
					if candidates[id]&uint8(1<<index) != 0 {
						reasons = append(reasons, reason)
					}
				}
				collectionID := columnText(stmt, 1)
				documents = append(documents, SelectionDocumentManifest{ID: id, URI: DocumentURI(collectionID, id), CollectionID: collectionID, NotebookID: columnText(stmt, 2), CurrentRevisionID: columnText(stmt, 3), Deleted: columnInt64(stmt, 4) != 0, InclusionReasons: reasons})
				continue
			}
			if rc == C.SQLITE_DONE {
				break
			}
			err := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		C.sqlite3_finalize(stmt)
	}
	sort.Slice(documents, func(i, j int) bool { return documents[i].ID < documents[j].ID })
	return documents, nil
}

func (s *SQLiteStore) loadSelectionResourcesLocked(ctx context.Context, documentIDs []string, maxBytes int64) ([]SelectionResourceManifest, error) {
	byID := map[string]SelectionResourceManifest{}
	for start := 0; start < len(documentIDs); start += selectionReadBatch {
		end := min(start+selectionReadBatch, len(documentIDs))
		batch := documentIDs[start:end]
		stmt, err := s.prepareLocked(`SELECT DISTINCT r.id, r.collection_id, r.mime_type, b.size_bytes, b.sha256
			FROM document_resource_refs rr JOIN resources r ON r.id = rr.resource_id JOIN blobs b ON b.sha256 = r.blob_sha256
			WHERE rr.document_id IN (` + lookupPlaceholders(len(batch)) + `) ORDER BY r.id`)
		if err != nil {
			return nil, err
		}
		if err := bindAll(stmt, batch); err != nil {
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		for {
			if err := ctx.Err(); err != nil {
				C.sqlite3_finalize(stmt)
				return nil, err
			}
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				id, collectionID, size := columnText(stmt, 0), columnText(stmt, 1), columnInt64(stmt, 3)
				byID[id] = SelectionResourceManifest{ID: id, URI: ResourceURI(collectionID, id), CollectionID: collectionID, MIMEType: columnText(stmt, 2), SizeBytes: size, SHA256: columnText(stmt, 4), Oversized: maxBytes > 0 && size > maxBytes}
				continue
			}
			if rc == C.SQLITE_DONE {
				break
			}
			err := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		C.sqlite3_finalize(stmt)
	}
	resources := make([]SelectionResourceManifest, 0, len(byID))
	for _, resource := range byID {
		resources = append(resources, resource)
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	return resources, nil
}

func (s *SQLiteStore) scanSelectionLinksLocked(ctx context.Context, documentIDs []string, selected map[string]uint8, policy EffectivePrivacyPolicy, detailLimit int, plan *SelectionPlan, digest hash.Hash) error {
	for start := 0; start < len(documentIDs); start += selectionReadBatch {
		end := min(start+selectionReadBatch, len(documentIDs))
		batch := documentIDs[start:end]
		stmt, err := s.prepareLocked(`SELECT source_document_id, COALESCE(target_document_id, ''), resolution_status,
			COALESCE(raw_target, ''), relation_type, source_format,
			COALESCE(source_start_byte, 0), COALESCE(source_end_byte, 0),
			COALESCE(source_line, 0), COALESCE(source_column, 0)
			FROM document_links WHERE source_document_id IN (` + lookupPlaceholders(len(batch)) + `)
			ORDER BY source_document_id, id`)
		if err != nil {
			return err
		}
		if err := bindAll(stmt, batch); err != nil {
			C.sqlite3_finalize(stmt)
			return err
		}
		for {
			if err := ctx.Err(); err != nil {
				C.sqlite3_finalize(stmt)
				return err
			}
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				decision := classifySelectionLink(columnText(stmt, 0), columnText(stmt, 1), columnText(stmt, 2), selected, policy.LinkAction)
				decision.LinkSHA256 = selectionLinkFingerprint(
					decision.SourceDocumentID, decision.TargetDocumentID, decision.ResolutionStatus,
					columnText(stmt, 3), columnText(stmt, 4), columnText(stmt, 5),
					columnInt64(stmt, 6), columnInt64(stmt, 7), columnInt64(stmt, 8), columnInt64(stmt, 9),
				)
				switch decision.Classification {
				case "internal":
					plan.Counts.InternalLinks++
				case "private":
					plan.Counts.PrivateLinks++
				case "broken":
					plan.Counts.BrokenLinks++
				case "external":
					plan.Counts.ExternalLinks++
				}
				writeDigestFields(digest, decision.SourceDocumentID, decision.TargetDocumentID, decision.LinkSHA256, decision.Classification, decision.ResolutionStatus, decision.Action)
				if len(plan.Links) < detailLimit {
					plan.Links = append(plan.Links, decision)
				} else {
					plan.Truncated = true
				}
				continue
			}
			if rc == C.SQLITE_DONE {
				break
			}
			err := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return err
		}
		C.sqlite3_finalize(stmt)
	}
	return nil
}

func selectionLinkFingerprint(sourceID, targetID, status, rawTarget, relationType, sourceFormat string, startByte, endByte, line, column int64) string {
	digest := sha256.New()
	writeDigestFields(digest, sourceID, targetID, status, rawTarget, relationType, sourceFormat, fmt.Sprint(startByte), fmt.Sprint(endByte), fmt.Sprint(line), fmt.Sprint(column))
	return hex.EncodeToString(digest.Sum(nil))
}

func classifySelectionLink(sourceID, targetID, status string, selected map[string]uint8, linkAction string) SelectionLinkDecision {
	decision := SelectionLinkDecision{SourceDocumentID: sourceID, TargetDocumentID: targetID, ResolutionStatus: status, Action: "retain"}
	if targetID != "" {
		if _, ok := selected[targetID]; ok {
			decision.Classification = "internal"
			return decision
		}
		decision.Classification = "private"
		decision.Action = linkAction
		return decision
	}
	switch strings.ToLower(status) {
	case "unresolved", "ambiguous", "invalid", "target_deleted":
		decision.Classification = "broken"
		decision.Action = linkAction
	default:
		decision.Classification = "external"
	}
	return decision
}

// selectionSourceBundle pairs the content-free manifest entry a plan reports
// with the raw composite key only an in-process writer may resolve.
type selectionSourceBundle struct {
	manifest SelectionSourceBundleManifest
	key      SourceBundleKey
}

func (s *SQLiteStore) loadSelectionSourceBundlesLocked(ctx context.Context, documentIDs []string, all bool) ([]SelectionSourceBundleManifest, []SourceBundleKey, error) {
	byKey := map[string]selectionSourceBundle{}
	read := func(sql string, args []string) error {
		stmt, err := s.prepareLocked(sql)
		if err != nil {
			return err
		}
		defer C.sqlite3_finalize(stmt)
		if err := bindAll(stmt, args); err != nil {
			return err
		}
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				rawSourceKey, rawItemKey := columnText(stmt, 1), columnText(stmt, 3)
				item := SelectionSourceBundleManifest{SourceSystem: columnText(stmt, 0), SourceKeySHA256: sha256Text(rawSourceKey), CollectionID: columnText(stmt, 2), ItemKeySHA256: sha256Text(rawItemKey), ItemType: columnText(stmt, 4), SHA256: columnText(stmt, 5), SizeBytes: columnInt64(stmt, 6)}
				key := strings.Join([]string{item.SourceSystem, rawSourceKey, item.CollectionID, rawItemKey}, "\x00")
				byKey[key] = selectionSourceBundle{manifest: item, key: SourceBundleKey{SourceSystem: item.SourceSystem, SourceKey: rawSourceKey, CollectionID: item.CollectionID, ItemKey: rawItemKey}}
				continue
			}
			if rc == C.SQLITE_DONE {
				return nil
			}
			return s.stepErrLocked(rc)
		}
	}
	columns := `sbi.source_system, sbi.source_key, sbi.collection_id, sbi.item_key, sbi.item_type, sbi.sha256, sbi.size_bytes`
	if all {
		if err := read(`SELECT `+columns+` FROM source_bundle_items sbi ORDER BY sbi.source_system, sbi.source_key, sbi.collection_id, sbi.item_key`, nil); err != nil {
			return nil, nil, err
		}
	} else {
		for start := 0; start < len(documentIDs); start += selectionReadBatch {
			end := min(start+selectionReadBatch, len(documentIDs))
			batch := documentIDs[start:end]
			sql := `SELECT DISTINCT ` + columns + ` FROM source_bundle_items sbi
				JOIN document_sources ds ON ds.source_system = sbi.source_system AND ds.external_id = sbi.external_id
				WHERE ds.document_id IN (` + lookupPlaceholders(len(batch)) + `)
				ORDER BY sbi.source_system, sbi.source_key, sbi.collection_id, sbi.item_key`
			if err := read(sql, batch); err != nil {
				return nil, nil, err
			}
		}
	}
	ordered := make([]selectionSourceBundle, 0, len(byKey))
	for _, item := range byKey {
		ordered = append(ordered, item)
	}
	manifestSortKey := func(item SelectionSourceBundleManifest) string {
		return strings.Join([]string{item.SourceSystem, item.SourceKeySHA256, item.CollectionID, item.ItemKeySHA256}, "\x00")
	}
	sort.Slice(ordered, func(i, j int) bool {
		return manifestSortKey(ordered[i].manifest) < manifestSortKey(ordered[j].manifest)
	})
	bundles := make([]SelectionSourceBundleManifest, 0, len(ordered))
	keys := make([]SourceBundleKey, 0, len(ordered))
	for _, item := range ordered {
		bundles = append(bundles, item.manifest)
		keys = append(keys, item.key)
	}
	return bundles, keys, nil
}

func (s *SQLiteStore) countSelectionProvenanceLocked(ctx context.Context, documentIDs []string) (int, error) {
	count := 0
	for start := 0; start < len(documentIDs); start += selectionReadBatch {
		end := min(start+selectionReadBatch, len(documentIDs))
		batch := documentIDs[start:end]
		stmt, err := s.prepareLocked(`SELECT COUNT(*) FROM document_sources WHERE document_id IN (` + lookupPlaceholders(len(batch)) + `)`)
		if err != nil {
			return 0, err
		}
		if err := bindAll(stmt, batch); err != nil {
			C.sqlite3_finalize(stmt)
			return 0, err
		}
		if err := ctx.Err(); err != nil {
			C.sqlite3_finalize(stmt)
			return 0, err
		}
		rc := C.sqlite3_step(stmt)
		if rc != C.SQLITE_ROW {
			err := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return 0, err
		}
		count += int(columnInt64(stmt, 0))
		C.sqlite3_finalize(stmt)
	}
	return count, nil
}

func selectionMetadataDecisions(policy EffectivePrivacyPolicy, provenance, bundles, documents int) []SelectionMetadataDecision {
	provenanceAction := "strip"
	if policy.IncludeProvenance {
		provenanceAction = "preserve"
	}
	privateAction := "strip"
	if policy.IncludePrivateMetadata {
		privateAction = "preserve"
	}
	bundleAction := "exclude"
	if policy.IncludeSourceBundles {
		bundleAction = "include"
	}
	return []SelectionMetadataDecision{
		{Field: "private_tags", Action: privateAction, Reason: "target privacy policy", AffectedItems: documents},
		{Field: "source.external_id", Action: provenanceAction, Reason: "source provenance policy", AffectedItems: provenance},
		{Field: "source.source_url", Action: provenanceAction, Reason: "source provenance policy", AffectedItems: provenance},
		{Field: "source.metadata_json", Action: privateAction, Reason: "private source metadata policy", AffectedItems: provenance},
		{Field: "source_bundle.items", Action: bundleAction, Reason: "exact source bundle policy", AffectedItems: bundles},
		{Field: "source_bundle.storage_path", Action: "strip", Reason: "local filesystem paths never cross planner APIs", AffectedItems: bundles},
	}
}

func selectionWarnings(plan SelectionPlan) []string {
	warnings := []string{}
	if plan.Counts.PrivateLinks > 0 {
		warnings = append(warnings, fmt.Sprintf("%d links target notes outside the selected public/subset manifest", plan.Counts.PrivateLinks))
	}
	if plan.Counts.BrokenLinks > 0 {
		warnings = append(warnings, fmt.Sprintf("%d selected-note links are unresolved, ambiguous, invalid, or target deleted", plan.Counts.BrokenLinks))
	}
	if plan.Counts.OversizedResources > 0 {
		warnings = append(warnings, fmt.Sprintf("%d reachable resources exceed max_resource_bytes", plan.Counts.OversizedResources))
	}
	if plan.Counts.AvailableSourceBundles > 0 && !plan.Policy.IncludeSourceBundles {
		warnings = append(warnings, fmt.Sprintf("%d associated exact source-bundle items are excluded by policy", plan.Counts.AvailableSourceBundles))
	}
	if plan.Truncated {
		warnings = append(warnings, "detail arrays are truncated; counts and manifest_sha256 cover the complete bounded plan")
	}
	return warnings
}

func selectionManifestDigest(norm normalizedSelectionRequest, documents []SelectionDocumentManifest, resources []SelectionResourceManifest, linkDigest hash.Hash, bundles []SelectionSourceBundleManifest, exclusionDigest hash.Hash, metadata []SelectionMetadataDecision) string {
	digest := sha256.New()
	writeDigestFields(digest, "selection-plan-v1", norm.request.Target, norm.request.Selection.CollectionID, norm.request.Selection.Match, fmt.Sprint(norm.includeDescendants), norm.parsedQuery.Canonical(), norm.policy.LinkAction, fmt.Sprint(norm.policy.IncludeSourceBundles), fmt.Sprint(norm.policy.IncludeProvenance), fmt.Sprint(norm.policy.IncludePrivateMetadata), fmt.Sprint(norm.policy.IncludeTrashed), fmt.Sprint(norm.policy.MaxResourceBytes), strings.Join(norm.policy.ExcludeTags, ","), strings.Join(norm.policy.PrivateTags, ","))
	for _, document := range documents {
		writeDigestFields(digest, "document", document.ID, document.URI, document.CollectionID, document.NotebookID, document.CurrentRevisionID, fmt.Sprint(document.Deleted), strings.Join(document.InclusionReasons, ","))
	}
	for _, resource := range resources {
		writeDigestFields(digest, "resource", resource.ID, resource.URI, resource.CollectionID, resource.MIMEType, fmt.Sprint(resource.SizeBytes), resource.SHA256, fmt.Sprint(resource.Oversized))
	}
	writeDigestFields(digest, "links", hex.EncodeToString(linkDigest.Sum(nil)))
	if norm.policy.IncludeSourceBundles {
		for _, bundle := range bundles {
			writeDigestFields(digest, "bundle", bundle.SourceSystem, bundle.SourceKeySHA256, bundle.CollectionID, bundle.ItemKeySHA256, bundle.ItemType, bundle.SHA256, fmt.Sprint(bundle.SizeBytes))
		}
	}
	writeDigestFields(digest, "exclusions", hex.EncodeToString(exclusionDigest.Sum(nil)))
	for _, decision := range metadata {
		writeDigestFields(digest, "metadata", decision.Field, decision.Action, decision.Reason, fmt.Sprint(decision.AffectedItems))
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func writeDigestFields(digest hash.Hash, fields ...string) {
	for _, field := range fields {
		_, _ = digest.Write([]byte(fmt.Sprintf("%d:", len(field))))
		_, _ = digest.Write([]byte(field))
	}
}

func sha256Text(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
