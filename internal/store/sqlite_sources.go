package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Source-provenance operations for the SQLite store (schema v6, Notrios
// redesign task R4). Rows in document_sources mark a note as externally
// sourced; PurgeDocument refuses such notes because permanent deletion is only
// allowed for notes stored purely in the local database.

func (s *SQLiteStore) SetDocumentSource(ctx context.Context, req SetDocumentSourceRequest) (DocumentSource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DocumentSource{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.setDocumentSourceLocked(req); err != nil {
		return DocumentSource{}, err
	}
	return s.getDocumentSourceLocked(strings.TrimSpace(req.DocumentID))
}

func (s *SQLiteStore) setDocumentSourceLocked(req SetDocumentSourceRequest) error {
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.SourceSystem = strings.TrimSpace(strings.ToLower(req.SourceSystem))
	if req.DocumentID == "" || req.SourceSystem == "" {
		return fmt.Errorf("%w: document ID and source system are required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.MetadataJSON) == "" {
		req.MetadataJSON = "{}"
	}
	publishedTS := parsePublishedTS(req.PublishedAt)
	// The document must exist, but may be trashed: importers can record
	// provenance for notes that are later marked deleted at the source.
	exists, err := s.documentExistsLocked(req.DocumentID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: document %q", ErrNotFound, req.DocumentID)
	}
	if err := s.execPreparedLocked(`INSERT INTO document_sources(document_id, source_system, external_id, author, author_id, thread_id, reply_to, source_url, published_at, published_ts, metadata_json)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		ON CONFLICT(document_id) DO UPDATE SET
			source_system = excluded.source_system,
			external_id = excluded.external_id,
			author = excluded.author,
			author_id = excluded.author_id,
			thread_id = excluded.thread_id,
			reply_to = excluded.reply_to,
			source_url = excluded.source_url,
			published_at = excluded.published_at,
			published_ts = excluded.published_ts,
			metadata_json = excluded.metadata_json,
			updated_at = CURRENT_TIMESTAMP`,
		req.DocumentID, req.SourceSystem, req.ExternalID, req.Author, req.AuthorID,
		req.ThreadID, req.ReplyTo, req.SourceURL, req.PublishedAt, formatTS(publishedTS), req.MetadataJSON); err != nil {
		return err
	}
	return nil
}

// parsePublishedTS derives UTC Unix seconds from an ISO 8601 timestamp or an
// all-digit epoch (milliseconds when the magnitude says so). Zero means
// unknown and is stored as NULL.
func parsePublishedTS(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC().Unix()
		}
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
		if n > 1_000_000_000_000 { // epoch milliseconds
			return n / 1000
		}
		return n
	}
	return 0
}

func formatTS(ts int64) string {
	if ts == 0 {
		return ""
	}
	return strconv.FormatInt(ts, 10)
}

func (s *SQLiteStore) GetDocumentSource(ctx context.Context, documentID string) (DocumentSource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DocumentSource{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDocumentSourceLocked(strings.TrimSpace(documentID))
}

// GetDocumentSources resolves one bounded reconciliation/import batch without
// issuing a query per note. Documents without provenance are simply omitted.
func (s *SQLiteStore) GetDocumentSources(ctx context.Context, documentIDs []string) (map[string]DocumentSource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string]DocumentSource, len(documentIDs))
	if len(documentIDs) == 0 {
		return result, nil
	}
	if err := validateLookupItems(documentIDs); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT ` + documentSourceColumns + `
		FROM document_sources WHERE document_id IN (` + lookupPlaceholders(len(documentIDs)) + `)`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, documentIDs); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			source := documentSourceFromStmt(stmt)
			result[source.DocumentID] = source
		case C.SQLITE_DONE:
			return result, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

const documentSourceColumns = `document_id, source_system, external_id, author, author_id, thread_id, reply_to, source_url, published_at, COALESCE(published_ts, 0), metadata_json, created_at, updated_at`

func (s *SQLiteStore) getDocumentSourceLocked(documentID string) (DocumentSource, error) {
	stmt, err := s.prepareLocked(`SELECT ` + documentSourceColumns + ` FROM document_sources WHERE document_id = ?`)
	if err != nil {
		return DocumentSource{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return DocumentSource{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return DocumentSource{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return DocumentSource{}, s.stepErrLocked(rc)
	}
	return documentSourceFromStmt(stmt), nil
}

func documentSourceFromStmt(stmt *C.sqlite3_stmt) DocumentSource {
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 11)))
	updatedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 12)))
	return DocumentSource{
		DocumentID:   columnText(stmt, 0),
		SourceSystem: columnText(stmt, 1),
		ExternalID:   columnText(stmt, 2),
		Author:       columnText(stmt, 3),
		AuthorID:     columnText(stmt, 4),
		ThreadID:     columnText(stmt, 5),
		ReplyTo:      columnText(stmt, 6),
		SourceURL:    columnText(stmt, 7),
		PublishedAt:  columnText(stmt, 8),
		PublishedTS:  columnInt64(stmt, 9),
		MetadataJSON: columnText(stmt, 10),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

// FindDocumentBySource returns the document holding the given source item, so
// importers can stay idempotent across re-runs even without deterministic IDs.
func (s *SQLiteStore) FindDocumentBySource(ctx context.Context, sourceSystem, externalID string) (string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT document_id FROM document_sources WHERE source_system = ? AND external_id = ? LIMIT 1`)
	if err != nil {
		return "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strings.TrimSpace(strings.ToLower(sourceSystem)), strings.TrimSpace(externalID)}); err != nil {
		return "", err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return "", ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return "", s.stepErrLocked(rc)
	}
	return columnText(stmt, 0), nil
}

// ListThreadDocuments returns the provenance rows of a conversation thread in
// chronological order (published time, then external ID for stable ties).
// Notes marked deleted are excluded, matching the rule that trashed
// externally-sourced notes disappear from queries without being purged.
func (s *SQLiteStore) ListThreadDocuments(ctx context.Context, threadID string) ([]DocumentSource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil, fmt.Errorf("%w: thread ID is required", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT ds.document_id, ds.source_system, ds.external_id, ds.author, ds.author_id, ds.thread_id, ds.reply_to, ds.source_url, ds.published_at, COALESCE(ds.published_ts, 0), ds.metadata_json, ds.created_at, ds.updated_at
		FROM document_sources ds
		JOIN documents d ON d.id = ds.document_id
		WHERE ds.thread_id = ? AND d.deleted_at IS NULL
		ORDER BY COALESCE(ds.published_ts, 0), ds.external_id, ds.document_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{threadID}); err != nil {
		return nil, err
	}
	sources := []DocumentSource{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			sources = append(sources, documentSourceFromStmt(stmt))
		case C.SQLITE_DONE:
			return sources, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// documentHasSourceLocked reports whether the note is externally sourced.
func (s *SQLiteStore) documentHasSourceLocked(documentID string) (bool, error) {
	count, err := s.countLocked(`SELECT COUNT(1) FROM document_sources WHERE document_id = ?`, documentID)
	return count > 0, err
}

// NotebookHasSourcedDocuments reports whether a notebook holds any
// externally-sourced notes (including trashed ones). Import tooling treats
// such notebooks as bound to their data source: archive imports must not
// merge plain notes into them.
func (s *SQLiteStore) NotebookHasSourcedDocuments(ctx context.Context, notebookID string) (bool, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	count, err := s.countLocked(`SELECT COUNT(1) FROM documents d JOIN document_sources ds ON ds.document_id = d.id WHERE d.notebook_id = ?`, strings.TrimSpace(notebookID))
	return count > 0, err
}
