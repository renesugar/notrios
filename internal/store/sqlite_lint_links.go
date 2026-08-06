package store

/*
#cgo pkg-config: sqlite3
#include <sqlite3.h>
*/
import "C"

import (
	"hash"
	"strings"
	"time"
)

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
}

// The WHERE clause is the union of every link check's condition, so one pass
// visits exactly the rows at least one check cares about. The ORDER BY is what
// makes both the examples and the digest deterministic.
const linkLintSQL = `SELECT l.source_document_id, l.source_line, l.source_column,
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
			ELSE 0 END
	FROM document_links l
	JOIN documents d ON d.id = l.source_document_id
	WHERE d.collection_id = ? AND d.deleted_at IS NULL AND (
		l.resolution_status IN ('unresolved', 'invalid', 'target_deleted', 'ambiguous')
		OR (l.anchor_type = 'block' AND COALESCE(l.anchor_value, '') != '' AND l.target_document_id IS NOT NULL)
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
	stmt, err := s.prepareLocked(linkLintSQL)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{collectionID}); err != nil {
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
