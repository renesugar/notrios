package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"strconv"

	"github.com/renesugar/notrios/internal/markdownlinks"
)

// documentCollectionLocked reads a live document's collection without reading
// its body, for a caller that needs only the scope a resolution happens in
// (v1.0 J32-I).
func (s *SQLiteStore) documentCollectionLocked(documentID string) (string, bool, error) {
	stmt, err := s.prepareLocked(`SELECT collection_id FROM documents WHERE id = ? AND deleted_at IS NULL`)
	if err != nil {
		return "", false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
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

// reresolveDocumentLinksLocked resolves a document's links again from the rows
// it already has, without parsing its body (v1.0 J32-I).
//
// The import writes every note's links when the note is written, and then runs
// a final pass over every note because a note may link to one imported later.
// That pass used to parse each body a second time, which a profile put at
// 173 s of a 516 s import -- an identical parse over identical bytes, since a
// body does not change between the two passes. What can change is the
// resolution, and `document_links` already stores every part of the candidate a
// resolution is computed from.
//
// It reports false when it will not vouch for the rows: a document with no
// block rows was not written by this path, so the caller parses it as before. A
// note with text always produces at least one block, so a document with blocks
// and no links genuinely has none.
func (s *SQLiteStore) reresolveDocumentLinksLocked(documentID, collectionID string) (bool, error) {
	hasBlocks, err := s.countLocked(`SELECT COUNT(*) FROM (SELECT 1 FROM document_blocks WHERE document_id = ? LIMIT 1)`, documentID)
	if err != nil {
		return false, err
	}
	if hasBlocks == 0 {
		return false, nil
	}

	type row struct {
		id        int64
		candidate markdownlinks.Candidate
	}
	// Read every row before updating any: an update while the select is still
	// stepping would be a scan reading its own writes.
	stmt, err := s.prepareLocked(`SELECT id, relation_type, source_format, raw_target,
			COALESCE(display_text, ''), COALESCE(anchor_type, ''), COALESCE(anchor_value, ''),
			COALESCE(context, ''), COALESCE(source_start_byte, 0), COALESCE(source_end_byte, 0),
			COALESCE(source_line, 0), COALESCE(source_column, 0)
		FROM document_links WHERE source_document_id = ? ORDER BY id`)
	if err != nil {
		return false, err
	}
	if err := bindAll(stmt, []string{documentID}); err != nil {
		C.sqlite3_finalize(stmt)
		return false, err
	}
	rows := []row{}
	for stepping := true; stepping; {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			rows = append(rows, row{
				id: int64(C.sqlite3_column_int64(stmt, 0)),
				candidate: markdownlinks.Candidate{
					RelationType: columnText(stmt, 1),
					SourceFormat: columnText(stmt, 2),
					RawTarget:    columnText(stmt, 3),
					DisplayText:  columnText(stmt, 4),
					AnchorType:   columnText(stmt, 5),
					AnchorValue:  columnText(stmt, 6),
					Context:      columnText(stmt, 7),
					StartByte:    int(C.sqlite3_column_int64(stmt, 8)),
					EndByte:      int(C.sqlite3_column_int64(stmt, 9)),
					Line:         int(C.sqlite3_column_int64(stmt, 10)),
					Column:       int(C.sqlite3_column_int64(stmt, 11)),
				},
			})
		case C.SQLITE_DONE:
			stepping = false
		default:
			err := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return false, err
		}
	}
	C.sqlite3_finalize(stmt)
	if len(rows) == 0 {
		return true, nil
	}

	// Only what a resolution decides is written back. The candidate's own
	// fields -- what was written, where it sits in the body -- cannot change
	// while the body does not.
	update, err := s.prepareRepeatedLocked(`UPDATE document_links
		SET target_document_id = NULLIF(?, ''), target_resource_id = NULLIF(?, ''),
			target_uri = NULLIF(?, ''), resolution_status = ?
		WHERE id = ?`)
	if err != nil {
		return false, err
	}
	defer update.close()
	for _, current := range rows {
		link := s.resolveLinkCandidateLocked(documentID, collectionID, current.candidate)
		if err := update.exec(link.TargetDocumentID, link.TargetResourceID, link.TargetURI,
			link.ResolutionStatus, strconv.FormatInt(current.id, 10)); err != nil {
			return false, err
		}
	}
	return true, nil
}
