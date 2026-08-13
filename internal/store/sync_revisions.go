package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/renesugar/notrios/internal/syncbody"
	"github.com/renesugar/notrios/internal/syncdelta"
	"github.com/renesugar/notrios/internal/syncstate"
)

// revisionBackfillBatch bounds one upgrade transaction. Hashing every existing
// revision of a large library is a one-time cost, but it must not be one
// transaction: a 382,206-note archive holds more revisions than that.
const revisionBackfillBatch = 2_000

// payloadMetadataAllowance reserves room in a revision operation payload for
// everything that is not the body: identifiers, title, MIME type, message,
// parents, and the exact hash and length.
const payloadMetadataAllowance = 8 << 10

// ensureSchemaV22 adds G7's revision objects, transfer deltas, and durable body
// conflicts. Like v21 it adds columns independently before the idempotent SQL,
// so an interrupted upgrade resumes rather than being mistaken for a complete
// one, and it backfills the exact content identity of every existing revision
// before the capture trigger starts requiring it.
func (s *SQLiteStore) ensureSchemaV22(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	existing := map[string]bool{}
	if err == nil {
		stmt, prepareErr := s.prepareLocked(`PRAGMA table_info(document_revisions)`)
		if prepareErr != nil {
			err = prepareErr
		} else {
			for C.sqlite3_step(stmt) == C.SQLITE_ROW {
				existing[columnText(stmt, 1)] = true
			}
			C.sqlite3_finalize(stmt)
		}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if version >= 22 {
		return nil
	}
	for _, column := range []struct{ name, definition string }{
		{"content_sha256", `content_sha256 TEXT NOT NULL DEFAULT ''`},
		{"content_length", `content_length INTEGER NOT NULL DEFAULT -1`},
		{"parent_revision_ids", `parent_revision_ids TEXT NOT NULL DEFAULT '[]'`},
	} {
		if existing[column.name] {
			continue
		}
		if err := s.Exec(ctx, `ALTER TABLE document_revisions ADD COLUMN `+column.definition+`;`); err != nil {
			return err
		}
	}
	// The backfill's parent-chain statement looks every revision's predecessor
	// up by document, and `document_revisions` carried no index before v22, so
	// the index has to exist before the backfill rather than with the rest of
	// the migration. Without it a restore of a real library does not finish.
	if err := s.Exec(ctx, `CREATE INDEX IF NOT EXISTS document_revisions_document_created_idx
		ON document_revisions(document_id, created_at);`); err != nil {
		return err
	}
	if err := s.backfillRevisionObjects(ctx); err != nil {
		return err
	}
	migration, err := migrationFS.ReadFile("migrations/0022_sync_revisions.sql")
	if err != nil {
		return fmt.Errorf("read schema v22 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

// backfillRevisionObjects gives every pre-v22 revision the exact content hash,
// byte length, and parent it needs to be a merge ancestor or a delta base.
//
// The parent chain is synthesized from each document's own revision order,
// which is the honest reading of the history it replaces: before v22 a document
// had one linear sequence of revisions and no branching, so the revision before
// this one *is* its parent. Nothing here invents a merge.
func (s *SQLiteStore) backfillRevisionObjects(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		done, err := s.backfillRevisionBatch()
		if err != nil {
			return err
		}
		if done {
			break
		}
	}
	return s.linkLinearRevisionParents(ctx)
}

// linkLinearRevisionParents gives every parentless revision the revision that
// precedes it in its own document. It is idempotent and safe to run on a live
// database: a genuine first revision has nothing before it and keeps its empty
// parent list, and a revision that already names parents — including a G7 merge
// with two — is never touched.
//
// The tiebreak is rowid, not id. `created_at` is a SQLite CURRENT_TIMESTAMP
// with one-second resolution, so several revisions of one note routinely share
// it, and revision ids are random — ordering by id would hand a note's history
// an arbitrary direction and make the older revision the child of the newer
// one. Rowid is insertion order, which for a pre-v22 database is exactly the
// order the revisions were written in.
func (s *SQLiteStore) linkLinearRevisionParents(ctx context.Context) error {
	return s.Exec(ctx, linkLinearRevisionParentsSQL)
}

// linkLinearRevisionParentsSQL is exported to the restore path as a statement
// rather than a method, because FinalizeRestoredDocuments already holds the
// store mutex and its own transaction. Calling the method there deadlocked:
// the mutex is released by a deferred unlock that runs *after* the return
// value is computed, so a relink placed on the return line waited on a lock
// its own frame still held.
const linkLinearRevisionParentsSQL = `
		UPDATE document_revisions
		   SET parent_revision_ids = (
		       SELECT json_array(previous.id)
		         FROM document_revisions previous
		        WHERE previous.document_id = document_revisions.document_id
		          AND (previous.created_at < document_revisions.created_at
		               OR (previous.created_at = document_revisions.created_at
		                   AND previous.rowid < document_revisions.rowid))
		        ORDER BY previous.created_at DESC, previous.rowid DESC
		        LIMIT 1)
		 WHERE parent_revision_ids = '[]'
		   AND EXISTS (
		       SELECT 1 FROM document_revisions previous
		        WHERE previous.document_id = document_revisions.document_id
		          AND (previous.created_at < document_revisions.created_at
		               OR (previous.created_at = document_revisions.created_at
		                   AND previous.rowid < document_revisions.rowid)));`

func (s *SQLiteStore) backfillRevisionBatch() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, body FROM document_revisions WHERE content_sha256 = '' LIMIT ` + strconv.Itoa(revisionBackfillBatch))
	if err != nil {
		return false, err
	}
	type pending struct {
		id, sha string
		length  int
	}
	var batch []pending
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			C.sqlite3_finalize(stmt)
			return false, s.stepErrLocked(rc)
		}
		body := columnText(stmt, 1)
		batch = append(batch, pending{columnText(stmt, 0), syncdelta.SHA256Hex([]byte(body)), len(body)})
	}
	C.sqlite3_finalize(stmt)
	if len(batch) == 0 {
		return true, nil
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	for _, row := range batch {
		if err := s.execPreparedLocked(
			`UPDATE document_revisions SET content_sha256 = ?, content_length = ? WHERE id = ?`,
			row.sha, strconv.Itoa(row.length), row.id); err != nil {
			return false, err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return false, err
	}
	committed = true
	return false, nil
}

// insertRevisionLocked is the single place a note revision becomes a row. It
// binds the revision to its exact content and its named parent, and — when the
// journal is enrolled — decides how the revision will travel before the capture
// trigger reads that decision.
func (s *SQLiteStore) insertRevisionLocked(revisionID, documentID, title, body, mimeType, message, parentRevisionID string) error {
	parents := syncbody.NormalizeParents([]string{parentRevisionID})
	encodedParents, err := json.Marshal(parents)
	if err != nil {
		return err
	}
	contentSHA := syncdelta.SHA256Hex([]byte(body))
	if err := s.prepareRevisionTransferLocked(revisionID, parentRevisionID, body, contentSHA); err != nil {
		return err
	}
	return s.execPreparedLocked(
		`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message, content_sha256, content_length, parent_revision_ids)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		revisionID, documentID, title, body, mimeType, message,
		contentSHA, strconv.Itoa(len(body)), string(encodedParents))
}

// prepareRevisionTransferLocked chooses between a named-parent delta, an inline
// complete body, and neither. It writes the transient row the capture trigger
// consumes; with no row the trigger falls back to inlining a small body, which
// is what keeps a code path that predates G7 from journaling a bodyless object.
func (s *SQLiteStore) prepareRevisionTransferLocked(revisionID, parentRevisionID, body, contentSHA string) error {
	enrolled, err := s.countLocked(`SELECT COUNT(*) FROM sync_local_journal WHERE singleton = 1`)
	if err != nil || enrolled == 0 {
		return err
	}
	inline, inlineFits := inlineRevisionBody(body)

	if parentRevisionID != "" {
		parentBody, parentSHA, found, err := s.revisionBodyLocked(parentRevisionID)
		if err != nil {
			return err
		}
		if found {
			delta, beneficial, err := syncdelta.EncodeBodyDelta(context.Background(), []byte(parentBody), []byte(body))
			if err != nil {
				return err
			}
			if beneficial && delta.BaseSHA256 == parentSHA && delta.ResultSHA256 == contentSHA {
				encoded, err := json.Marshal(revisionDeltaPayload{
					Format:         delta.Format,
					BaseRevisionID: parentRevisionID,
					BaseSHA256:     delta.BaseSHA256,
					BaseLength:     delta.BaseLength,
					ResultSHA256:   delta.ResultSHA256,
					ResultLength:   delta.ResultLength,
					Base64:         base64.StdEncoding.EncodeToString(delta.Bytes),
				})
				if err != nil {
					return err
				}
				if err := s.execPreparedLocked(
					`INSERT INTO sync_revision_deltas(revision_id, base_revision_id, format, base_sha256, base_length, result_sha256, result_length, delta_base64, delta_length)
					 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
					 ON CONFLICT(revision_id) DO UPDATE SET base_revision_id=excluded.base_revision_id, format=excluded.format,
					   base_sha256=excluded.base_sha256, base_length=excluded.base_length, result_sha256=excluded.result_sha256,
					   result_length=excluded.result_length, delta_base64=excluded.delta_base64, delta_length=excluded.delta_length`,
					revisionID, parentRevisionID, delta.Format, delta.BaseSHA256, strconv.Itoa(delta.BaseLength),
					delta.ResultSHA256, strconv.Itoa(delta.ResultLength),
					base64.StdEncoding.EncodeToString(delta.Bytes), strconv.Itoa(len(delta.Bytes))); err != nil {
					return err
				}
				return s.execPreparedLocked(
					`INSERT INTO sync_revision_transfer(revision_id, inline_body, delta_json) VALUES(?, NULL, ?)
					 ON CONFLICT(revision_id) DO UPDATE SET inline_body=NULL, delta_json=excluded.delta_json`,
					revisionID, string(encoded))
			}
		}
	}
	if inlineFits {
		return s.execPreparedLocked(
			`INSERT INTO sync_revision_transfer(revision_id, inline_body, delta_json) VALUES(?, ?, NULL)
			 ON CONFLICT(revision_id) DO UPDATE SET inline_body=excluded.inline_body, delta_json=NULL`,
			revisionID, inline)
	}
	// Neither representation fits an operation. The complete object stays in
	// document_revisions, so a peer can still obtain it once G8 and G9 supply
	// object transfer; what travels now is the revision's identity.
	return s.execPreparedLocked(
		`INSERT INTO sync_revision_transfer(revision_id, inline_body, delta_json) VALUES(?, NULL, NULL)
		 ON CONFLICT(revision_id) DO UPDATE SET inline_body=NULL, delta_json=NULL`,
		revisionID)
}

// inlineRevisionBody reports whether a complete body fits an operation payload,
// measured against its actual JSON encoding rather than its raw length. A body
// of control characters encodes six bytes per byte, so a raw-length rule would
// be wrong for exactly the inputs least likely to be tested.
func inlineRevisionBody(body string) (string, bool) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", false
	}
	return body, len(encoded)+payloadMetadataAllowance <= syncstate.MaxPayloadBytes
}

type revisionDeltaPayload struct {
	Format         string `json:"format"`
	BaseRevisionID string `json:"base_revision_id"`
	BaseSHA256     string `json:"base_sha256"`
	BaseLength     int    `json:"base_length"`
	ResultSHA256   string `json:"result_sha256"`
	ResultLength   int    `json:"result_length"`
	Base64         string `json:"base64"`
}

func (s *SQLiteStore) revisionBodyLocked(revisionID string) (string, string, bool, error) {
	stmt, err := s.prepareLocked(`SELECT body, content_sha256 FROM document_revisions WHERE id = ?`)
	if err != nil {
		return "", "", false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{revisionID}); err != nil {
		return "", "", false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return columnText(stmt, 0), columnText(stmt, 1), true, nil
	case C.SQLITE_DONE:
		return "", "", false, nil
	default:
		return "", "", false, s.stepErrLocked(rc)
	}
}
