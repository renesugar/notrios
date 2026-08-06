package store

/*
#cgo pkg-config: sqlite3
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// PlanWorkspaceFix computes the exact edits a fix run would make, without
// making them. It is the default mode of `notriosctl fix` for the same reason
// dry run is the default for garbage collection: the operator reads what will
// happen to their notes before it happens.
func (s *SQLiteStore) PlanWorkspaceFix(ctx context.Context, req FixRequest) (FixPlan, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return FixPlan{}, err
	}
	collectionID := strings.TrimSpace(req.CollectionID)
	if collectionID == "" {
		collectionID = "default"
	}
	kinds, err := normalizeFixKinds(req.Kinds)
	if err != nil {
		return FixPlan{}, err
	}
	limit := req.MaxDocuments
	if limit <= 0 {
		limit = MaxFixDocuments
	}
	if limit > MaxFixDocuments {
		return FixPlan{}, fmt.Errorf("%w: max_documents must be %d or less", ErrInvalidInput, MaxFixDocuments)
	}

	plan := FixPlan{
		Version:      1,
		CollectionID: collectionID,
		Kinds:        sortedFixKinds(kinds),
		Documents:    []FixDocumentPlan{},
		Warnings:     []string{},
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	candidates, err := s.fixCandidatesLocked(collectionID, strings.TrimSpace(req.DocumentID), kinds)
	if err != nil {
		return FixPlan{}, err
	}
	for _, documentID := range candidates {
		if len(plan.Documents) >= limit {
			plan.Truncated = true
			break
		}
		document, err := s.getDocumentLocked(documentID)
		if err != nil {
			// A note trashed between the candidate query and here is simply not
			// fixed; it is not an error worth failing a whole plan for.
			continue
		}
		if !s.documentIsWritableLocked(document) {
			continue
		}
		edits, err := s.planDocumentFixLocked(document, kinds)
		if err != nil {
			return FixPlan{}, err
		}
		if len(edits) == 0 {
			continue
		}
		plan.Documents = append(plan.Documents, FixDocumentPlan{
			DocumentID:     document.ID,
			BaseRevisionID: document.CurrentRevisionID,
			Edits:          edits,
		})
		plan.TotalEdits += len(edits)
	}
	plan.TotalDocuments = len(plan.Documents)
	if plan.Truncated {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("more than %d notes need fixing; run again after applying these", limit))
	}
	return plan, nil
}

// ApplyDocumentFix repairs one note.
//
// The revision precondition is the whole safety story: the plan was computed
// against a specific revision, and if the note changed since — a concurrent
// edit, another fix run, a sync — the edit offsets no longer describe this body
// and the update fails rather than cutting the note at stale positions.
func (s *SQLiteStore) ApplyDocumentFix(ctx context.Context, plan FixDocumentPlan) (FixApplyResult, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return FixApplyResult{}, err
	}
	result := FixApplyResult{DocumentID: plan.DocumentID}
	if strings.TrimSpace(plan.BaseRevisionID) == "" {
		return result, fmt.Errorf("%w: a base revision is required to fix a note", ErrPreconditionRequired)
	}
	document, err := s.GetDocument(ctx, plan.DocumentID)
	if err != nil {
		return result, err
	}
	if document.CurrentRevisionID != plan.BaseRevisionID {
		return result, fmt.Errorf("%w: %s changed since the plan was made", ErrConflict, plan.DocumentID)
	}

	body, applied, skipped := applyFixEdits(document.Body, plan.Edits)
	result.Applied = applied
	result.Skipped = skipped
	if applied == 0 {
		return result, nil
	}
	updated, err := s.UpdateDocument(ctx, UpdateDocumentRequest{
		ID:             document.ID,
		Title:          document.Title,
		Body:           body,
		BodyMIMEType:   document.BodyMIMEType,
		BaseRevisionID: plan.BaseRevisionID,
		Message:        fmt.Sprintf("workspace fix: %d edits", applied),
	})
	if err != nil {
		return result, err
	}
	result.RevisionID = updated.CurrentRevisionID
	return result, nil
}

// applyFixEdits replaces spans from the end backwards, so an earlier
// replacement cannot invalidate a later offset, and refuses any span whose
// bytes are not what the plan recorded. A stale span is skipped, never applied
// at an arbitrary position.
func applyFixEdits(body string, edits []FixEdit) (string, int, int) {
	ordered := make([]FixEdit, len(edits))
	copy(ordered, edits)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartByte > ordered[j].StartByte })

	applied, skipped := 0, 0
	previousStart := len(body) + 1
	for _, edit := range ordered {
		if edit.StartByte < 0 || edit.EndByte > len(body) || edit.StartByte >= edit.EndByte || edit.EndByte > previousStart {
			skipped++
			continue
		}
		if body[edit.StartByte:edit.EndByte] != edit.Before {
			skipped++
			continue
		}
		body = body[:edit.StartByte] + edit.After + body[edit.EndByte:]
		previousStart = edit.StartByte
		applied++
	}
	return body, applied, skipped
}

// planDocumentFixLocked computes one note's edits from its stored links.
func (s *SQLiteStore) planDocumentFixLocked(document Document, kinds map[string]bool) ([]FixEdit, error) {
	links, err := s.outgoingLinksLocked(document.ID)
	if err != nil {
		return nil, err
	}
	edits := []FixEdit{}
	for _, link := range links {
		if link.SourceStartByte < 0 || link.SourceEndByte > len(document.Body) || link.SourceStartByte >= link.SourceEndByte {
			continue
		}
		before := document.Body[link.SourceStartByte:link.SourceEndByte]
		// Only Markdown inline links are rewritten. A wikilink means something
		// different in the source vault and rewriting it would change the
		// author's chosen syntax, not repair it.
		if link.SourceFormat != "markdown" || !strings.HasPrefix(before, "[") && !strings.HasPrefix(before, "![") {
			continue
		}
		after, kind, ok := s.repairLink(link, before)
		if !ok || !kinds[kind] {
			continue
		}
		edits = append(edits, FixEdit{
			Kind:      kind,
			Line:      link.SourceLine,
			Column:    link.SourceColumn,
			StartByte: link.SourceStartByte,
			EndByte:   link.SourceEndByte,
			Before:    before,
			After:     after,
		})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].StartByte < edits[j].StartByte })
	return edits, nil
}

// repairLink decides what one link should become, or that it should be left
// alone. Each repair is a rewriting of syntax around content the author already
// wrote; none of them invents meaning.
func (s *SQLiteStore) repairLink(link DocumentLink, before string) (string, string, bool) {
	anchor := anchorSuffixForFix(link)
	display := link.DisplayText
	relation := link.RelationType

	// A link that resolved by title or filename is rewritten to the canonical
	// URI it already points at, so renaming the target cannot break it.
	if link.ResolutionStatus == "resolved" {
		canonical := ""
		switch {
		case link.TargetDocumentID != "":
			canonical = DocumentURI("", link.TargetDocumentID)
		case link.TargetResourceID != "":
			canonical = ResourceURI("", link.TargetResourceID)
		}
		if canonical != "" && link.RawTarget != canonical {
			return renderMarkdownLink(relation, display, canonical+anchor), FixNonCanonicalLinkTarget, true
		}
	}

	// An image with no alt text gets the resource's filename as a placeholder.
	// It is a name the author already chose for the file, not a description
	// invented for them — which is exactly why this fix is opt-in.
	if (relation == "image" || relation == "embed") && strings.TrimSpace(display) == "" {
		if link.TargetResourceID == "" {
			return "", "", false
		}
		resource, err := s.getResourceLocked(link.TargetResourceID)
		if err != nil {
			return "", "", false
		}
		alt := altTextFromFilename(resource.Filename)
		if alt == "" {
			return "", "", false
		}
		target := link.RawTarget
		if target == "" {
			target = ResourceURI("", link.TargetResourceID)
		}
		return renderMarkdownLink(relation, alt, target+anchor), FixMissingAltText, true
	}
	return "", "", false
}

// altTextFromFilename turns `kitchen-plan.final.png` into `kitchen plan final`.
func altTextFromFilename(filename string) string {
	name := strings.TrimSpace(filepath.Base(strings.TrimSpace(filename)))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	if ext := filepath.Ext(name); ext != "" && ext != name {
		name = strings.TrimSuffix(name, ext)
	}
	replaced := strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == '.' {
			return ' '
		}
		return r
	}, name)
	return strings.Join(strings.Fields(replaced), " ")
}

func renderMarkdownLink(relation, display, target string) string {
	prefix := ""
	if relation == "image" || relation == "embed" {
		prefix = "!"
	}
	return prefix + "[" + display + "](" + target + ")"
}

func anchorSuffixForFix(link DocumentLink) string {
	if link.AnchorValue == "" {
		return ""
	}
	if link.AnchorType == "block" {
		return "#^" + link.AnchorValue
	}
	return "#" + link.AnchorValue
}

// documentIsWritableLocked keeps fixes away from notes the API itself refuses
// to edit: Help notes are read-only and trashed notes are not part of the
// workspace.
func (s *SQLiteStore) documentIsWritableLocked(document Document) bool {
	return document.NotebookID != HelpNotebookID && document.DeletedAt.IsZero()
}

// fixCandidatesLocked lists the notes worth planning, ordered for determinism.
func (s *SQLiteStore) fixCandidatesLocked(collectionID, documentID string, kinds map[string]bool) ([]string, error) {
	conditions := []string{}
	if kinds[FixNonCanonicalLinkTarget] {
		conditions = append(conditions, `(l.resolution_status = 'resolved'
			AND (l.target_document_id IS NOT NULL OR l.target_resource_id IS NOT NULL)
			AND l.raw_target NOT LIKE 'document://%' AND l.raw_target NOT LIKE 'resource://%')`)
	}
	if kinds[FixMissingAltText] {
		conditions = append(conditions, `(l.relation_type IN ('image', 'embed')
			AND TRIM(COALESCE(l.display_text, '')) = '' AND l.target_resource_id IS NOT NULL)`)
	}
	if len(conditions) == 0 {
		return []string{}, nil
	}
	query := `SELECT DISTINCT l.source_document_id
		FROM document_links l
		JOIN documents d ON d.id = l.source_document_id
		WHERE d.collection_id = ? AND d.deleted_at IS NULL
			AND l.source_format = 'markdown'
			AND (` + strings.Join(conditions, " OR ") + `)`
	args := []string{collectionID}
	if documentID != "" {
		query += ` AND l.source_document_id = ?`
		args = append(args, documentID)
	}
	query += ` ORDER BY l.source_document_id`

	stmt, err := s.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return nil, err
	}
	ids := []string{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		ids = append(ids, columnText(stmt, 0))
	}
	return ids, nil
}

func (s *SQLiteStore) outgoingLinksLocked(documentID string) ([]DocumentLink, error) {
	stmt, err := s.prepareLocked(`SELECT COALESCE(target_document_id, ''), COALESCE(target_resource_id, ''),
			COALESCE(raw_target, ''), COALESCE(display_text, ''), COALESCE(relation_type, ''),
			COALESCE(source_format, ''), COALESCE(anchor_type, ''), COALESCE(anchor_value, ''),
			COALESCE(source_start_byte, 0), COALESCE(source_end_byte, 0),
			COALESCE(source_line, 0), COALESCE(source_column, 0), COALESCE(resolution_status, '')
		FROM document_links WHERE source_document_id = ? ORDER BY source_start_byte`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return nil, err
	}
	links := []DocumentLink{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		links = append(links, DocumentLink{
			SourceDocumentID: documentID,
			TargetDocumentID: columnText(stmt, 0),
			TargetResourceID: columnText(stmt, 1),
			RawTarget:        columnText(stmt, 2),
			DisplayText:      columnText(stmt, 3),
			RelationType:     columnText(stmt, 4),
			SourceFormat:     columnText(stmt, 5),
			AnchorType:       columnText(stmt, 6),
			AnchorValue:      columnText(stmt, 7),
			SourceStartByte:  int(C.sqlite3_column_int(stmt, 8)),
			SourceEndByte:    int(C.sqlite3_column_int(stmt, 9)),
			SourceLine:       int(C.sqlite3_column_int(stmt, 10)),
			SourceColumn:     int(C.sqlite3_column_int(stmt, 11)),
			ResolutionStatus: columnText(stmt, 12),
		})
	}
	return links, nil
}

func normalizeFixKinds(requested []string) (map[string]bool, error) {
	known := map[string]bool{}
	for _, kind := range FixKinds() {
		known[kind] = true
	}
	selected := map[string]bool{}
	if len(requested) == 0 {
		for _, kind := range DefaultFixKinds() {
			selected[kind] = true
		}
		return selected, nil
	}
	for _, kind := range requested {
		trimmed := strings.ToLower(strings.TrimSpace(kind))
		if trimmed == "" {
			continue
		}
		if !known[trimmed] {
			return nil, fmt.Errorf("%w: unknown fix kind %q", ErrInvalidInput, trimmed)
		}
		selected[trimmed] = true
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("%w: no known fix kinds were requested", ErrInvalidInput)
	}
	return selected, nil
}

func sortedFixKinds(kinds map[string]bool) []string {
	out := []string{}
	for _, kind := range FixKinds() {
		if kinds[kind] {
			out = append(out, kind)
		}
	}
	return out
}
