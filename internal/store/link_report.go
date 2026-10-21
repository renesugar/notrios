package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"strings"
)

// What a report of ambiguous links reads from the library (v1.0 J40-C).
//
// The report itself is assembled in internal/linkreport, because it also asks
// the Obsidian importer's own code which notes a name could mean, and the store
// cannot import the importer. These are the store's half: the links that open
// nothing, the provenance an import left, and the resolver's own candidate
// lookups. All of it is read under the store's lock, one statement per call,
// and none of it is written.

// LinkReportRow is one link that opens nothing, in a note the report measures:
// not in the Trash and not in a read-only notebook, as lint measures.
type LinkReportRow struct {
	DocumentID   string
	CollectionID string
	RawTarget    string
	SourceFormat string
	RelationType string
	// Status is ambiguous, unresolved, invalid or target_deleted.
	Status    string
	StartByte int
	EndByte   int
	Line      int
	Column    int
}

// LinkReportNote is a note as the report names it, with its provenance when it
// has any.
type LinkReportNote struct {
	DocumentID   string
	CollectionID string
	Title        string
	URI          string
	Source       DocumentSource
}

// LinkReportInputs is everything the report reads in one go.
type LinkReportInputs struct {
	// Rows are in source order: by note, then by line and column.
	Rows []LinkReportRow
	// Notes holds the measured notes that carry a row, and every measured note
	// with provenance from SourceSystem, keyed by ID.
	Notes map[string]LinkReportNote
}

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

// LinkCandidate is a note or a resource a link's text could mean.
type LinkCandidate struct {
	ID    string
	Kind  string // "note" or "attachment"
	Title string // the note's title, or the resource's file name
	URI   string
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
