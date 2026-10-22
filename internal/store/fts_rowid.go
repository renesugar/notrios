package store

import (
	"context"
	"fmt"
	"strings"
)

// The full-text index cannot be searched by document.
//
// `documents_fts` declares `document_id UNINDEXED`, which is what FTS5 offers
// for a column it should store and not tokenise -- and it means SQLite has no
// index to find a row by it. `DELETE FROM documents_fts WHERE document_id = ?`
// therefore plans as `SCAN documents_fts VIRTUAL TABLE`, reading every row in
// the index to remove one. v1.0 J18 measured that at 2,068 ms on a
// 382,206-note library, which was essentially the entire cost of writing a
// document; deleting by rowid on the same library took 19 ms.
//
// So the rowid FTS5 assigns is recorded in `documents_fts_rowid` as the write
// happens, and deletions go through it. These two helpers exist so that the
// pairing -- write the index, record the rowid; delete the row, forget the
// rowid -- is in one place rather than repeated at each of the ten call sites
// that touch the index.
//
// Both take a locked store: every caller is already inside the transaction
// that owns the change, and the mapping must live or die with it.

// replaceDocumentFTSLocked removes a document's row from the full-text index
// and inserts its replacement, keeping the rowid mapping in step.
//
// A document with no mapping deletes nothing, which is correct rather than
// merely harmless: the mapping is populated for every existing row by migration
// 0028, so a missing entry means there was no row to remove.
func (s *SQLiteStore) replaceDocumentFTSLocked(documentID, collectionID, title, body string) error {
	if err := s.deleteDocumentFTSLocked(documentID); err != nil {
		return err
	}
	return s.insertDocumentFTSLocked(documentID, collectionID, title, body)
}

// insertDocumentFTSLocked adds one document to the index and records the rowid
// SQLite gave it.
func (s *SQLiteStore) insertDocumentFTSLocked(documentID, collectionID, title, body string) error {
	if err := s.execPreparedLocked(
		`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`,
		documentID, collectionID, title, body); err != nil {
		return err
	}
	// last_insert_rowid() is the rowid of the row just written, and FTS5
	// reports it like any other table.
	return s.execPreparedLocked(
		`INSERT OR REPLACE INTO documents_fts_rowid(document_id, fts_rowid) VALUES(?, last_insert_rowid())`,
		documentID)
}

// deleteDocumentFTSLocked removes one document from the index by its recorded
// rowid, and forgets the mapping.
func (s *SQLiteStore) deleteDocumentFTSLocked(documentID string) error {
	if err := s.execPreparedLocked(
		`DELETE FROM documents_fts WHERE rowid = (SELECT fts_rowid FROM documents_fts_rowid WHERE document_id = ?)`,
		documentID); err != nil {
		return err
	}
	return s.execPreparedLocked(`DELETE FROM documents_fts_rowid WHERE document_id = ?`, documentID)
}

// deleteDocumentsFTSLocked removes a bounded set of documents from the index.
//
// The set form exists because notebook deletion removes every document in a
// notebook at once, and doing that one statement per document would trade one
// scan for a thousand round trips. The `IN` is over the mapping table, whose
// `document_id` is a primary key and whose `fts_rowid` is indexed, so neither
// side scans.
func (s *SQLiteStore) deleteDocumentsFTSLocked(documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(documentIDs)), ",")
	arguments := append([]string(nil), documentIDs...)
	if err := s.execPreparedLocked(
		`DELETE FROM documents_fts WHERE rowid IN (SELECT fts_rowid FROM documents_fts_rowid WHERE document_id IN (`+
			placeholders+`))`, arguments...); err != nil {
		return err
	}
	return s.execPreparedLocked(
		`DELETE FROM documents_fts_rowid WHERE document_id IN (`+placeholders+`)`, arguments...)
}

// ensureSchemaV28 creates and populates the rowid mapping.
//
// It runs for a fresh database as well as an existing one, because
// `applySchema` is the path both take: a new library gets the table empty, and
// an existing one gets it filled by the single scan in the migration. That
// scan costs about what one document write used to cost on a large library --
// once, against once per write forever.
func (s *SQLiteStore) ensureSchemaV28(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	s.mu.Unlock()
	if err != nil || version >= 28 {
		return err
	}
	migration, err := migrationFS.ReadFile("migrations/0028_fts_rowid_map.sql")
	if err != nil {
		return fmt.Errorf("read schema v28 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}
