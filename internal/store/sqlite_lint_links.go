package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"hash"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/markdownblocks"
	"github.com/renesugar/notrios/internal/stablelink"
)

// headingSlugCacheLimit bounds what the lint pass remembers while checking
// heading anchors. Beyond it the cache is cleared rather than grown: a lint run
// must not hold state proportional to the library.
const headingSlugCacheLimit = 512

// headingSlugCache holds one document's heading slugs at a time so a note
// linked from many places is fetched once.
type headingSlugCache struct {
	documents map[string]map[string]bool
}

func newHeadingSlugCache() *headingSlugCache {
	return &headingSlugCache{documents: map[string]map[string]bool{}}
}

func (c *headingSlugCache) slugsFor(s *SQLiteStore, documentID string) (map[string]bool, error) {
	if slugs, ok := c.documents[documentID]; ok {
		return slugs, nil
	}
	slugs, err := s.headingSlugsLocked(documentID)
	if err != nil {
		return nil, err
	}
	if len(c.documents) >= headingSlugCacheLimit {
		c.documents = map[string]map[string]bool{}
	}
	c.documents[documentID] = slugs
	return slugs, nil
}

// linkLintChecks are the checks whose findings all come from `document_links`.
//
// They used to be six separate statements, and each one scanned the link table:
// at 100,000 notes that measured ~1.7-2.0 s apiece, about 10 s of the 12 s a
// full lint took, and it grew linearly with the library. A lint pass should read
// the library once, so these run as a single ordered scan that classifies each
// row into whichever checks it violates.
func linkLintChecks() []string {
	return []string{
		LintBrokenDocumentLink,
		LintBrokenResourceLink,
		LintAmbiguousLink,
		LintUnresolvedBlockAnchor,
		LintUnresolvedHeadingAnchor,
		LintUnlocalizedRemoteMedia,
		LintMissingAltText,
	}
}

// linkLintRow is one candidate link, already narrowed by SQL to rows that
// violate at least one check.
type linkLintRow struct {
	documentID        string
	line              int
	column            int
	rawTarget         string
	resolutionStatus  string
	anchorType        string
	anchorValue       string
	hasTargetDoc      bool
	hasTargetResource bool
	relationType      string
	displayText       string
	blockMissing      bool
	headingMissing    bool
	targetDocument    string
}

// The WHERE clause is the union of every link check's condition, so one pass
// visits exactly the rows at least one check cares about. The ORDER BY is what
// makes both the examples and the digest deterministic.
var linkLintSQL = `SELECT l.source_document_id, l.source_line, l.source_column,
		COALESCE(l.raw_target, ''), COALESCE(l.resolution_status, ''),
		COALESCE(l.anchor_type, ''), COALESCE(l.anchor_value, ''),
		l.target_document_id IS NOT NULL, l.target_resource_id IS NOT NULL,
		COALESCE(l.relation_type, ''), COALESCE(l.display_text, ''),
		CASE WHEN l.anchor_type = 'block' AND COALESCE(l.anchor_value, '') != '' AND l.target_document_id IS NOT NULL
			THEN NOT EXISTS (
				SELECT 1 FROM document_blocks b
				WHERE b.document_id = l.target_document_id
					AND (b.marker = l.anchor_value OR b.id = l.anchor_value)
			)
			ELSE 0 END,
		COALESCE(l.target_document_id, '')
	FROM document_links l
	JOIN documents d ON d.id = l.source_document_id
	WHERE ` + CollectionScopeSQL("d") + ` AND d.deleted_at IS NULL AND ` + notSystemAuthoredSQL("d") + ` AND (
		l.resolution_status IN ('unresolved', 'invalid', 'target_deleted', 'ambiguous')
		OR (l.anchor_type IN ('block', 'heading') AND COALESCE(l.anchor_value, '') != '' AND l.target_document_id IS NOT NULL)
		OR (l.relation_type IN ('image', 'embed') AND (l.raw_target LIKE 'http://%' OR l.raw_target LIKE 'https://%'))
		OR (l.relation_type IN ('image', 'embed') AND TRIM(COALESCE(l.display_text, '')) = '')
	)
	ORDER BY l.source_document_id, l.source_line, l.source_column`

// runLinkLintChecksLocked evaluates every selected link check in one scan.
//
// A row may violate more than one check — a broken link that is also an
// untitled image, say — and contributes to each, exactly as the separate
// statements did.
func (s *SQLiteStore) runLinkLintChecksLocked(selected map[string]bool, collectionID string, limit int, digest hash.Hash) (map[string]*LintCheckResult, error) {
	results := map[string]*LintCheckResult{}
	wanted := false
	for _, check := range linkLintChecks() {
		if selected[check] {
			results[check] = &LintCheckResult{Check: check, Findings: []LintFinding{}}
			wanted = true
		}
	}
	if !wanted {
		return results, nil
	}

	started := time.Now()
	headings := newHeadingSlugCache()
	stmt, err := s.prepareLocked(linkLintSQL)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, append([]string{collectionID}, readOnlyNotebookArgs()...)); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		row := linkLintRow{
			documentID:        columnText(stmt, 0),
			line:              int(C.sqlite3_column_int(stmt, 1)),
			column:            int(C.sqlite3_column_int(stmt, 2)),
			rawTarget:         columnText(stmt, 3),
			resolutionStatus:  columnText(stmt, 4),
			anchorType:        columnText(stmt, 5),
			anchorValue:       columnText(stmt, 6),
			hasTargetDoc:      C.sqlite3_column_int(stmt, 7) != 0,
			hasTargetResource: C.sqlite3_column_int(stmt, 8) != 0,
			relationType:      columnText(stmt, 9),
			displayText:       columnText(stmt, 10),
			blockMissing:      C.sqlite3_column_int(stmt, 11) != 0,
			targetDocument:    columnText(stmt, 12),
		}
		// A heading anchor cannot be checked in SQL: matching it means
		// slugifying the written anchor with the same Unicode rules the parser
		// uses, which lives in Go. The target's heading slugs are fetched once
		// per document and cached, so a note linked from a hundred places costs
		// one query rather than a hundred.
		if row.anchorType == "heading" && row.anchorValue != "" && row.targetDocument != "" {
			slugs, err := headings.slugsFor(s, row.targetDocument)
			if err != nil {
				return nil, err
			}
			anchor := row.anchorValue
			if stablelink.IsURITarget(row.rawTarget) {
				anchor = stablelink.DecodeAnchor(anchor)
			}
			row.headingMissing = !slugs[markdownblocks.Slugify(anchor)]
		}
		// Check order is fixed so the digest does not depend on map iteration.
		for _, check := range linkLintChecks() {
			result, ok := results[check]
			if !ok {
				continue
			}
			finding, matched := classifyLinkLint(check, row)
			if !matched {
				continue
			}
			finding.Check = check
			result.Count++
			writeDigestFields(digest, check, finding.DocumentID, finding.ResourceID,
				itoaLint(finding.Line), itoaLint(finding.Column), finding.TargetSHA256, finding.Detail)
			if len(result.Findings) < limit {
				result.Findings = append(result.Findings, finding)
			}
		}
	}
	elapsed := float64(time.Since(started).Microseconds()) / 1000
	for _, result := range results {
		result.Truncated = result.Count > len(result.Findings)
		// One scan serves them all, so the cost is reported once against the
		// scan rather than invented per check.
		result.ElapsedMS = elapsed / float64(len(results))
	}
	return results, nil
}

// classifyLinkLint decides whether one row violates one check, preserving the
// semantics each separate statement had.
func classifyLinkLint(check string, row linkLintRow) (LintFinding, bool) {
	broken := row.resolutionStatus == "unresolved" || row.resolutionStatus == "invalid" || row.resolutionStatus == "target_deleted"
	isMedia := row.relationType == "image" || row.relationType == "embed"
	finding := LintFinding{
		DocumentID:   row.documentID,
		Line:         row.line,
		Column:       row.column,
		TargetSHA256: sha256Text(row.rawTarget),
	}
	switch check {
	case LintBrokenDocumentLink:
		if broken && !row.hasTargetResource {
			finding.Detail = row.resolutionStatus
			return finding, true
		}
	case LintBrokenResourceLink:
		if broken && strings.HasPrefix(row.rawTarget, "resource://") {
			finding.Detail = row.resolutionStatus
			return finding, true
		}
	case LintAmbiguousLink:
		if row.resolutionStatus == "ambiguous" {
			return finding, true
		}
	case LintUnresolvedBlockAnchor:
		if row.blockMissing {
			finding.TargetSHA256 = sha256Text(row.anchorValue)
			finding.Detail = "no block matches this anchor"
			return finding, true
		}
	case LintUnresolvedHeadingAnchor:
		if row.headingMissing {
			finding.TargetSHA256 = sha256Text(row.anchorValue)
			finding.Detail = "no heading matches this anchor"
			return finding, true
		}
	case LintUnlocalizedRemoteMedia:
		if isMedia && (strings.HasPrefix(row.rawTarget, "http://") || strings.HasPrefix(row.rawTarget, "https://")) {
			return finding, true
		}
	case LintMissingAltText:
		if isMedia && strings.TrimSpace(row.displayText) == "" {
			return finding, true
		}
	}
	return LintFinding{}, false
}

// The store's half of the ambiguous-links report (v1.0 J40-C): its types and
// what they are for are in link_report.go. The SQL lives here, beside lint's
// own link scan, rather than in a new file that calls the C API.

var linkReportRowsSQL = `SELECT l.source_document_id, d.collection_id, COALESCE(l.raw_target, ''),
		COALESCE(l.source_format, ''), COALESCE(l.relation_type, ''), COALESCE(l.resolution_status, ''),
		COALESCE(l.source_start_byte, 0), COALESCE(l.source_end_byte, 0), l.source_line, l.source_column
	FROM document_links l
	JOIN documents d ON d.id = l.source_document_id
	WHERE ` + CollectionScopeSQL("d") + ` AND ` + measuredDocumentSQL("d") + `
		AND l.resolution_status IN ('ambiguous', 'unresolved', 'invalid', 'target_deleted')
	ORDER BY l.source_document_id, l.source_line, l.source_column, l.source_start_byte`

var linkReportNotesSQL = `SELECT d.id, d.collection_id, COALESCE(d.title, ''),
		COALESCE(s.source_system, ''), COALESCE(s.external_id, ''), COALESCE(s.metadata_json, '')
	FROM documents d
	LEFT JOIN document_sources s ON s.document_id = d.id
	WHERE ` + CollectionScopeSQL("d") + ` AND ` + measuredDocumentSQL("d") + `
		AND (s.source_system = ? OR EXISTS (
			SELECT 1 FROM document_links l
			WHERE l.source_document_id = d.id
				AND l.resolution_status IN ('ambiguous', 'unresolved', 'invalid', 'target_deleted')))
	ORDER BY d.id`

// LinkReportInputs reads the links that open nothing and the notes the report
// needs to name and to rebuild sourceSystem's namespace from. An empty
// collection ID means every collection.
func (s *SQLiteStore) LinkReportInputs(ctx context.Context, collectionID, sourceSystem string) (LinkReportInputs, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return LinkReportInputs{}, err
	}
	collectionID = strings.TrimSpace(collectionID)
	s.mu.Lock()
	defer s.mu.Unlock()

	inputs := LinkReportInputs{Rows: []LinkReportRow{}, Notes: map[string]LinkReportNote{}}
	stmt, err := s.prepareLocked(linkReportRowsSQL)
	if err != nil {
		return LinkReportInputs{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, append([]string{collectionID}, readOnlyNotebookArgs()...)); err != nil {
		return LinkReportInputs{}, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return LinkReportInputs{}, s.stepErrLocked(rc)
		}
		inputs.Rows = append(inputs.Rows, LinkReportRow{
			DocumentID:   columnText(stmt, 0),
			CollectionID: columnText(stmt, 1),
			RawTarget:    columnText(stmt, 2),
			SourceFormat: columnText(stmt, 3),
			RelationType: columnText(stmt, 4),
			Status:       columnText(stmt, 5),
			StartByte:    int(C.sqlite3_column_int64(stmt, 6)),
			EndByte:      int(C.sqlite3_column_int64(stmt, 7)),
			Line:         int(C.sqlite3_column_int(stmt, 8)),
			Column:       int(C.sqlite3_column_int(stmt, 9)),
		})
	}

	notes, err := s.prepareLocked(linkReportNotesSQL)
	if err != nil {
		return LinkReportInputs{}, err
	}
	defer C.sqlite3_finalize(notes)
	args := append([]string{collectionID}, readOnlyNotebookArgs()...)
	if err := bindAll(notes, append(args, sourceSystem)); err != nil {
		return LinkReportInputs{}, err
	}
	for {
		rc := C.sqlite3_step(notes)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return LinkReportInputs{}, s.stepErrLocked(rc)
		}
		note := LinkReportNote{
			DocumentID:   columnText(notes, 0),
			CollectionID: columnText(notes, 1),
			Title:        columnText(notes, 2),
		}
		note.URI = DocumentURI(note.CollectionID, note.DocumentID)
		note.Source = DocumentSource{
			DocumentID:   note.DocumentID,
			SourceSystem: columnText(notes, 3),
			ExternalID:   columnText(notes, 4),
			MetadataJSON: columnText(notes, 5),
		}
		inputs.Notes[note.DocumentID] = note
	}
	return inputs, nil
}

// LinkCandidates asks the resolver's own question of a link the store calls
// ambiguous: which notes its text names by title, and, if none, which
// resources by file name. A title match decides first, as it does when a link
// is resolved, so the candidates are of one kind. Both are complete; the
// report caps what it shows.
func (s *SQLiteStore) LinkCandidates(ctx context.Context, collectionID, rawTarget string) ([]LinkCandidate, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	documents, err := s.documentsByLinkTitleLocked(collectionID, rawTarget, 0)
	if err != nil {
		return nil, err
	}
	candidates := []LinkCandidate{}
	if len(documents) > 0 {
		for _, id := range documents {
			title, err := s.documentTitleLocked(id)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, LinkCandidate{ID: id, Kind: "note", Title: title, URI: DocumentURI(collectionID, id)})
		}
		return candidates, nil
	}
	resources, err := s.resourcesByLinkFilenameLocked(collectionID, rawTarget, 0)
	if err != nil {
		return nil, err
	}
	for _, id := range resources {
		resource, err := s.getResourceLocked(id)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, LinkCandidate{ID: id, Kind: "attachment", Title: resource.Filename, URI: ResourceURI(collectionID, id)})
	}
	return candidates, nil
}

func (s *SQLiteStore) documentTitleLocked(id string) (string, error) {
	stmt, err := s.prepareLocked(`SELECT COALESCE(title, '') FROM documents WHERE id = ?`)
	if err != nil {
		return "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return "", err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return columnText(stmt, 0), nil
	case C.SQLITE_DONE:
		return "", ErrNotFound
	default:
		return "", s.stepErrLocked(rc)
	}
}
