package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LintWorkspace reports what is broken in a library without changing anything.
//
// Every check is one ordered SQL statement whose rows are streamed: each row is
// counted and folded into the digest, then discarded unless it is one of the
// first DetailLimit examples. A library with a million broken links therefore
// produces an accurate count and a stable digest while holding at most the
// capped examples in memory.
func (s *SQLiteStore) LintWorkspace(ctx context.Context, req LintRequest) (LintReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return LintReport{}, err
	}
	// Empty means every collection; see store.CollectionScopeSQL. Lint that
	// reported on one provenance while another rotted was the defect, and it
	// was invisible because the report says which collection it looked at as
	// though that had been asked for.
	collectionID := strings.TrimSpace(req.CollectionID)
	limit := req.DetailLimit
	if limit <= 0 {
		limit = DefaultLintDetailItems
	}
	if limit > MaxLintDetailItems {
		return LintReport{}, fmt.Errorf("%w: detail_limit must be %d or less", ErrInvalidInput, MaxLintDetailItems)
	}
	selected, err := normalizeLintChecks(req.Checks)
	if err != nil {
		return LintReport{}, err
	}

	report := LintReport{
		Version:      1,
		CollectionID: collectionID,
		DetailLimit:  limit,
		Checks:       []LintCheckResult{},
		Warnings:     []string{},
	}
	digest := sha256.New()
	writeDigestFields(digest, "workspace-lint-v1", collectionID)

	s.mu.Lock()
	defer s.mu.Unlock()
	// Every check whose findings come from document_links runs in one scan;
	// six separate statements each scanned that table and dominated the cost.
	linkResults, err := s.runLinkLintChecksLocked(selected, collectionID, limit, digest)
	if err != nil {
		return LintReport{}, err
	}
	for _, check := range LintChecks() {
		if !selected[check] {
			continue
		}
		if result, ok := linkResults[check]; ok {
			report.Checks = append(report.Checks, *result)
			report.TotalFindings += result.Count
			continue
		}
		result, err := s.runLintCheckLocked(check, collectionID, limit, digest)
		if err != nil {
			return LintReport{}, err
		}
		report.Checks = append(report.Checks, result)
		report.TotalFindings += result.Count
	}
	if report.TotalFindings > 0 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("%d findings; this report is read-only and changed nothing", report.TotalFindings))
	}
	report.ReportSHA256 = hex.EncodeToString(digest.Sum(nil))
	return report, nil
}

// itoaLint keeps digest field formatting in one place, shared by the single
// link scan and the per-table statements.
func itoaLint(value int) string { return strconv.Itoa(value) }

func normalizeLintChecks(requested []string) (map[string]bool, error) {
	known := map[string]bool{}
	for _, check := range LintChecks() {
		known[check] = true
	}
	if len(requested) == 0 {
		return known, nil
	}
	selected := map[string]bool{}
	for _, check := range requested {
		trimmed := strings.ToLower(strings.TrimSpace(check))
		if trimmed == "" {
			continue
		}
		if !known[trimmed] {
			return nil, fmt.Errorf("%w: unknown lint check %q", ErrInvalidInput, trimmed)
		}
		selected[trimmed] = true
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("%w: no known lint checks were requested", ErrInvalidInput)
	}
	return selected, nil
}

// lintQuery is one check's statement plus how to read a row into a finding.
type lintQuery struct {
	sql  string
	args []string
	scan func(stmt *C.sqlite3_stmt) LintFinding
}

func (s *SQLiteStore) runLintCheckLocked(check, collectionID string, limit int, digest hash.Hash) (LintCheckResult, error) {
	query := lintQueryFor(check, collectionID)
	stmt, err := s.prepareLocked(query.sql)
	if err != nil {
		return LintCheckResult{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, query.args); err != nil {
		return LintCheckResult{}, err
	}
	started := time.Now()
	result := LintCheckResult{Check: check, Findings: []LintFinding{}}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return LintCheckResult{}, s.stepErrLocked(rc)
		}
		finding := query.scan(stmt)
		finding.Check = check
		result.Count++
		// Fold every finding into the digest, including the ones the cap hides,
		// so the digest describes the library rather than the page.
		writeDigestFields(digest, check, finding.DocumentID, finding.ResourceID,
			strconv.Itoa(finding.Line), strconv.Itoa(finding.Column), finding.TargetSHA256, finding.Detail)
		if len(result.Findings) < limit {
			result.Findings = append(result.Findings, finding)
		}
	}
	result.Truncated = result.Count > len(result.Findings)
	result.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
	return result, nil
}

// lintQueryFor returns the statement for one check that is not part of the
// single link scan: these read one row per finding from their own table, so
// there is nothing to share.
//
// Each is ordered by stable keys so a repeated run produces identical findings
// and an identical digest. None of them selects a title, a body, or a raw link
// target: what crosses this boundary is an ID, a location, and a hash.
func lintQueryFor(check, collectionID string) lintQuery {
	switch check {
	case LintDuplicateSourceID:
		// Two notes claiming the same external identity: an importer collision
		// or a hand-edited provenance row. IDs are primary keys, so canonical
		// duplication can only appear here.
		return lintQuery{
			sql: `SELECT s.document_id, 0, 0, s.source_system || ':' || s.external_id, 'duplicate external identity'
				FROM document_sources s
				JOIN documents d ON d.id = s.document_id
				WHERE ` + CollectionScopeSQL("d") + ` AND d.deleted_at IS NULL AND ` + notSystemAuthoredSQL("d") + `
					AND EXISTS (
						SELECT 1 FROM document_sources o
						JOIN documents od ON od.id = o.document_id
						WHERE o.source_system = s.source_system AND o.external_id = s.external_id
							AND o.document_id != s.document_id AND od.deleted_at IS NULL
					)
				ORDER BY s.source_system, s.external_id, s.document_id`,
			args: append([]string{collectionID}, readOnlyNotebookArgs()...),
			scan: scanLocatedFinding,
		}
	case LintMissingTitle:
		return lintQuery{
			sql: `SELECT id, 0, 0, '', 'empty title' FROM documents d
				WHERE ` + CollectionScopeSQL("") + ` AND deleted_at IS NULL AND ` + notSystemAuthoredSQL("d") + `
					AND TRIM(COALESCE(title, '')) = ''
				ORDER BY id`,
			args: append([]string{collectionID}, readOnlyNotebookArgs()...),
			scan: scanLocatedFinding,
		}
	case LintUnreferencedResource:
		return lintQuery{
			sql: `SELECT '', 0, 0, r.id, 'no document references this resource'
				FROM resources r
				WHERE ` + CollectionScopeSQL("r") + `
					AND NOT EXISTS (SELECT 1 FROM document_resource_refs ref WHERE ref.resource_id = r.id)
				ORDER BY r.id`,
			args: []string{collectionID},
			scan: func(stmt *C.sqlite3_stmt) LintFinding {
				return LintFinding{
					ResourceID: columnText(stmt, 3),
					Detail:     columnText(stmt, 4),
				}
			},
		}
	case LintDanglingCollection:
		// The repair is decided and deliberately not built: recreate the
		// missing row, preserving the identifier, rather than adopting the note
		// into `default`, which would rewrite the one fact the field carries.
		// Nothing builds it because no supported write path produces this
		// state; the answer is recorded so that whoever first observes one does
		// not have to decide it under pressure.
		return lintQuery{
			sql: `SELECT d.id, 0, 0, '', d.collection_id
				FROM documents d
				LEFT JOIN collections c ON c.id = d.collection_id
				WHERE c.id IS NULL AND d.deleted_at IS NULL
				ORDER BY d.collection_id, d.id`,
			args: nil,
			scan: func(stmt *C.sqlite3_stmt) LintFinding {
				return LintFinding{
					DocumentID: columnText(stmt, 0),
					Detail:     "names collection " + strconv.Quote(columnText(stmt, 4)) + ", which has no row",
				}
			},
		}
	case LintProjectionBacklog:
		// Drift between canonical notes and the derived projection. A full
		// filesystem reconciliation belongs to the sidecar; what lint can say
		// from the database is how many jobs are stuck, which is the signal
		// that drift is accumulating.
		return lintQuery{
			sql: `SELECT object_id, 0, 0, '', CASE WHEN attempt_count > 0 THEN 'retrying' ELSE 'pending' END
				FROM index_outbox WHERE completed_at IS NULL AND object_type = 'document'
				ORDER BY sequence`,
			args: nil,
			scan: func(stmt *C.sqlite3_stmt) LintFinding {
				return LintFinding{
					DocumentID: columnText(stmt, 0),
					Detail:     columnText(stmt, 4),
				}
			},
		}
	default:
		// normalizeLintChecks rejects unknown names and the link checks never
		// reach here, so this is unreachable except through a programming
		// error; a query that returns nothing is safer than a panic in a
		// read-only report.
		return lintQuery{
			sql:  `SELECT '', 0, 0, '', '' WHERE 0`,
			scan: scanLocatedFinding,
		}
	}
}

// scanLocatedFinding reads the shared "identifier, location, target, detail"
// row shape.
func scanLocatedFinding(stmt *C.sqlite3_stmt) LintFinding {
	return LintFinding{
		DocumentID:   columnText(stmt, 0),
		Line:         int(C.sqlite3_column_int(stmt, 1)),
		Column:       int(C.sqlite3_column_int(stmt, 2)),
		TargetSHA256: sha256Text(columnText(stmt, 3)),
		Detail:       columnText(stmt, 4),
	}
}

// sortedLintChecks is used by tests and the CLI to present checks predictably.
func sortedLintChecks() []string {
	checks := append([]string{}, LintChecks()...)
	sort.Strings(checks)
	return checks
}
