package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import "context"

// SQLiteImportMetrics is aggregate, content-free evidence for importer
// performance and canonical consistency checks.
type SQLiteImportMetrics struct {
	Documents         int64  `json:"documents"`
	Revisions         int64  `json:"revisions"`
	FTSRows           int64  `json:"fts_rows"`
	Sources           int64  `json:"sources"`
	TagRelations      int64  `json:"tag_relations"`
	ResourceRelations int64  `json:"resource_relations"`
	Links             int64  `json:"links"`
	UnresolvedLinks   int64  `json:"unresolved_links"`
	PendingOutbox     int64  `json:"pending_outbox"`
	ImportStates      int64  `json:"import_states"`
	DatabaseBytes     int64  `json:"database_bytes"`
	PageSize          int64  `json:"page_size"`
	PageCount         int64  `json:"page_count"`
	FreelistCount     int64  `json:"freelist_count"`
	CacheSize         int64  `json:"cache_size"`
	Synchronous       int64  `json:"synchronous"`
	ForeignKeys       int64  `json:"foreign_keys"`
	JournalMode       string `json:"journal_mode"`
}

func (s *SQLiteStore) ImportMetrics(ctx context.Context) (SQLiteImportMetrics, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SQLiteImportMetrics{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metrics := SQLiteImportMetrics{}
	counts := []struct {
		target *int64
		query  string
	}{
		{&metrics.Documents, `SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`},
		{&metrics.Revisions, `SELECT COUNT(*) FROM document_revisions`},
		{&metrics.FTSRows, `SELECT COUNT(*) FROM documents_fts`},
		{&metrics.Sources, `SELECT COUNT(*) FROM document_sources`},
		{&metrics.TagRelations, `SELECT COUNT(*) FROM note_tags`},
		{&metrics.ResourceRelations, `SELECT COUNT(*) FROM document_resource_refs`},
		{&metrics.Links, `SELECT COUNT(*) FROM document_links`},
		{&metrics.UnresolvedLinks, `SELECT COUNT(*) FROM document_links WHERE resolution_status = 'unresolved'`},
		{&metrics.PendingOutbox, `SELECT COUNT(*) FROM index_outbox WHERE completed_at IS NULL`},
		{&metrics.ImportStates, `SELECT COUNT(*) FROM import_item_states`},
		{&metrics.PageSize, `PRAGMA page_size`},
		{&metrics.PageCount, `PRAGMA page_count`},
		{&metrics.FreelistCount, `PRAGMA freelist_count`},
		{&metrics.CacheSize, `PRAGMA cache_size`},
		{&metrics.Synchronous, `PRAGMA synchronous`},
		{&metrics.ForeignKeys, `PRAGMA foreign_keys`},
	}
	for _, count := range counts {
		value, err := s.countLocked(count.query)
		if err != nil {
			return SQLiteImportMetrics{}, err
		}
		*count.target = value
	}
	metrics.DatabaseBytes = metrics.PageSize * metrics.PageCount
	stmt, err := s.prepareLocked(`PRAGMA journal_mode`)
	if err != nil {
		return SQLiteImportMetrics{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_ROW {
		metrics.JournalMode = columnText(stmt, 0)
	} else if rc != C.SQLITE_DONE {
		err = s.stepErrLocked(rc)
	}
	C.sqlite3_finalize(stmt)
	if err != nil {
		return SQLiteImportMetrics{}, err
	}
	return metrics, nil
}
