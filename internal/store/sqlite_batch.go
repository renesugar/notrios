package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RunBatch applies one bounded organizer transaction.
//
// The two modes differ in what a failure does, never in what the report says.
// Atomic runs everything in one transaction and rolls the lot back on the first
// failure, marking the items that had succeeded `rolled_back` — they were not
// wrong, they were undone. Best-effort gives each item its own transaction and
// keeps going.
//
// Both take the store lock for the whole run. A batch is bounded work and
// interleaving another writer between items would make "atomic" a lie.
func (s *SQLiteStore) RunBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return BatchResult{}, err
	}
	req = NormalizeBatchRequest(req)
	if err := validateBatchRequest(req); err != nil {
		return BatchResult{}, err
	}
	fingerprint := batchFingerprint(req)

	s.mu.Lock()
	defer s.mu.Unlock()

	if key := strings.TrimSpace(req.RequestKey); key != "" {
		stored, found, storedFingerprint, err := s.lookupBatchLocked(key)
		if err != nil {
			return BatchResult{}, err
		}
		if found {
			// A key reused with different arguments is a client bug, and
			// answering with the earlier unrelated result would hide it.
			if storedFingerprint != fingerprint {
				return BatchResult{}, fmt.Errorf("%w: request key %q was already used for a different request", ErrInvalidInput, key)
			}
			stored.Replayed = true
			return stored, nil
		}
	}

	result := BatchResult{RequestKey: req.RequestKey, Operation: req.Operation, Mode: req.Mode, Items: []BatchItemResult{}}
	var runErr error
	if req.Mode == BatchModeAtomic {
		runErr = s.runAtomicBatchLocked(req, &result)
	} else {
		runErr = s.runBestEffortBatchLocked(req, &result)
	}
	if runErr != nil {
		return BatchResult{}, runErr
	}
	tallyBatch(&result)

	if key := strings.TrimSpace(req.RequestKey); key != "" {
		if err := s.recordBatchLocked(key, req, fingerprint, result); err != nil {
			return BatchResult{}, err
		}
	}
	return result, nil
}

// runAtomicBatchLocked applies every item inside one transaction.
func (s *SQLiteStore) runAtomicBatchLocked(req BatchRequest, result *BatchResult) error {
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	for _, item := range req.Items {
		outcome := s.applyBatchItemLocked(req, item)
		result.Items = append(result.Items, outcome)
		if outcome.Status != BatchStatusFailed {
			continue
		}
		// One failure ends an atomic run. Everything already applied is undone
		// by the rollback, and the report says so rather than leaving the
		// caller to infer it from the mode.
		if err := s.execLocked("ROLLBACK"); err != nil {
			return err
		}
		committed = true
		for i := range result.Items {
			if result.Items[i].Status == BatchStatusApplied {
				result.Items[i].Status = BatchStatusRolledBack
			}
		}
		// Items after the failure were never attempted; saying nothing about
		// them would leave a report shorter than the request.
		for _, remaining := range req.Items[len(result.Items):] {
			result.Items = append(result.Items, BatchItemResult{
				DocumentID: remaining.DocumentID,
				Status:     BatchStatusSkipped,
				Reason:     "not attempted: an earlier item failed and the run is atomic",
			})
		}
		return nil
	}

	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// runBestEffortBatchLocked gives each item its own transaction.
func (s *SQLiteStore) runBestEffortBatchLocked(req BatchRequest, result *BatchResult) error {
	for _, item := range req.Items {
		if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
			return err
		}
		outcome := s.applyBatchItemLocked(req, item)
		if outcome.Status == BatchStatusFailed {
			if err := s.execLocked("ROLLBACK"); err != nil {
				return err
			}
		} else if err := s.execLocked("COMMIT"); err != nil {
			return err
		}
		result.Items = append(result.Items, outcome)
	}
	return nil
}

// applyBatchItemLocked runs one item and never returns an error: a per-item
// failure is a value on the report, because that is the whole point of a batch
// that reports per item.
func (s *SQLiteStore) applyBatchItemLocked(req BatchRequest, item BatchItem) BatchItemResult {
	out := BatchItemResult{DocumentID: item.DocumentID, Status: BatchStatusApplied}
	fail := func(err error) BatchItemResult {
		out.Status = BatchStatusFailed
		out.Error = err.Error()
		return out
	}

	switch req.Operation {
	case BatchOpMove:
		current, err := s.getDocumentLocked(item.DocumentID)
		if err != nil {
			return fail(err)
		}
		if current.NotebookID == req.NotebookID {
			out.Status = BatchStatusSkipped
			out.Reason = "already in that notebook"
			return out
		}
		if _, err := s.moveDocumentToNotebookLocked(item.DocumentID, req.NotebookID); err != nil {
			return fail(err)
		}

	case BatchOpAddTags:
		changed := false
		for _, tag := range req.Tags {
			_, added, err := s.addDocumentTagLocked(item.DocumentID, tag)
			if err != nil {
				return fail(err)
			}
			changed = changed || added
		}
		if !changed {
			out.Status = BatchStatusSkipped
			out.Reason = "already carried every tag"
		}

	case BatchOpRemoveTags:
		changed := false
		for _, tag := range req.Tags {
			removed, err := s.removeDocumentTagLocked(item.DocumentID, tag)
			if errors.Is(err, ErrNotFound) {
				continue // no such tag in the library: nothing to remove
			}
			if err != nil {
				return fail(err)
			}
			changed = changed || removed
		}
		if !changed {
			out.Status = BatchStatusSkipped
			out.Reason = "carried none of those tags"
		}

	case BatchOpTrash:
		revID, err := NewID("rev")
		if err != nil {
			return fail(err)
		}
		if err := s.deleteDocumentLocked(NormalizeDeleteRequest(DeleteDocumentRequest{
			ID:             item.DocumentID,
			BaseRevisionID: item.BaseRevisionID,
			Message:        "batch trash",
		}), revID); err != nil {
			return fail(err)
		}

	case BatchOpRestore:
		if _, err := s.restoreDocumentLocked(item.DocumentID); err != nil {
			return fail(err)
		}

	case BatchOpDuplicate:
		copied, err := s.duplicateDocumentLocked(item.DocumentID)
		if err != nil {
			return fail(err)
		}
		out.NewDocumentID = copied.ID

	default:
		return fail(fmt.Errorf("%w: unknown operation %q", ErrInvalidInput, req.Operation))
	}
	return out
}

// duplicateDocumentLocked copies one note.
//
// What a copy inherits is a product decision, taken here and recorded in
// `plans/v0.6/`: the body, the notebook, the tags, and the resource references
// (those are content-addressed, so sharing a blob is correct rather than
// wasteful). The title gains " (copy)".
//
// What it does **not** inherit is external identity. A `document_sources` row
// says "this note *is* that Joplin note"; two notes claiming it would make
// re-import ambiguous and would show up in lint as a duplicate source ID. The
// copy is a new local note that happens to have the same words.
//
// Revision history is likewise not copied. A duplicate starts at revision one;
// carrying another note's history would attribute edits that never happened to
// this note.
func (s *SQLiteStore) duplicateDocumentLocked(documentID string) (Document, error) {
	source, err := s.getDocumentLocked(documentID)
	if err != nil {
		return Document{}, err
	}
	if IsReadOnlyNotebook(source.NotebookID) {
		return Document{}, fmt.Errorf("%w: notes in the %s notebook cannot be duplicated", ErrProtected, ReadOnlyNotebookName(source.NotebookID))
	}
	newID, err := NewID("doc")
	if err != nil {
		return Document{}, err
	}
	revID, err := NewID("rev")
	if err != nil {
		return Document{}, err
	}
	title := source.Title + " (copy)"

	if err := s.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type, current_revision_id) VALUES(?, ?, ?, ?, ?, ?)`,
		newID, source.CollectionID, source.NotebookID, title, source.BodyMIMEType, revID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message) VALUES(?, ?, ?, ?, ?, ?)`,
		revID, newID, title, source.Body, source.BodyMIMEType, "duplicated from "+source.ID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`,
		newID, source.CollectionID, title, source.Body); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO note_tags(document_id, tag_id) SELECT ?, tag_id FROM note_tags WHERE document_id = ?`, newID, source.ID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO document_resource_refs(document_id, resource_id, relation_type, ordinal, anchor_json)
		SELECT ?, resource_id, relation_type, ordinal, anchor_json FROM document_resource_refs WHERE document_id = ?`, newID, source.ID); err != nil {
		return Document{}, err
	}
	if err := s.rebuildDocumentLinksLocked(newID, source.CollectionID, source.Body); err != nil {
		return Document{}, err
	}
	if err := s.enqueueProjectionLocked(newID, "upsert"); err != nil {
		return Document{}, err
	}
	return s.getDocumentLocked(newID)
}

// ---- validation, tallying, and the idempotency ledger ----------------------

func validateBatchRequest(req BatchRequest) error {
	switch req.Mode {
	case BatchModeAtomic, BatchModeBestEffort:
	default:
		return fmt.Errorf("%w: mode must be %q or %q", ErrInvalidInput, BatchModeAtomic, BatchModeBestEffort)
	}
	known := false
	for _, op := range BatchOperations() {
		if req.Operation == op {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("%w: operation must be one of %s", ErrInvalidInput, strings.Join(BatchOperations(), ", "))
	}
	if len(req.Items) == 0 {
		return fmt.Errorf("%w: at least one item is required", ErrInvalidInput)
	}
	if len(req.Items) > MaxBatchItems {
		return fmt.Errorf("%w: %d items is over the %d ceiling", ErrInvalidInput, len(req.Items), MaxBatchItems)
	}
	if len(req.RequestKey) > MaxBatchRequestKeyBytes {
		return fmt.Errorf("%w: request key is over %d bytes", ErrInvalidInput, MaxBatchRequestKeyBytes)
	}
	seen := map[string]bool{}
	for _, item := range req.Items {
		id := strings.TrimSpace(item.DocumentID)
		if id == "" {
			return fmt.Errorf("%w: every item needs a document_id", ErrInvalidInput)
		}
		// A note listed twice would be applied twice and reported twice, and
		// for `duplicate` would silently make two copies.
		if seen[id] {
			return fmt.Errorf("%w: document %q appears more than once", ErrInvalidInput, id)
		}
		seen[id] = true
		if req.Operation == BatchOpTrash && strings.TrimSpace(item.BaseRevisionID) == "" {
			return fmt.Errorf("%w: %s requires base_revision_id on every item", ErrPreconditionRequired, BatchOpTrash)
		}
	}
	switch req.Operation {
	case BatchOpMove:
		if strings.TrimSpace(req.NotebookID) == "" {
			return fmt.Errorf("%w: move requires notebook_id", ErrInvalidInput)
		}
	case BatchOpAddTags, BatchOpRemoveTags:
		if len(req.Tags) == 0 {
			return fmt.Errorf("%w: %s requires at least one tag", ErrInvalidInput, req.Operation)
		}
		if len(req.Tags) > MaxBatchTags {
			return fmt.Errorf("%w: %d tags is over the %d ceiling", ErrInvalidInput, len(req.Tags), MaxBatchTags)
		}
		for _, tag := range req.Tags {
			if strings.TrimSpace(tag) == "" {
				return fmt.Errorf("%w: tag names cannot be empty", ErrInvalidInput)
			}
		}
	}
	return nil
}

func tallyBatch(result *BatchResult) {
	result.Applied, result.Skipped, result.Failed, result.RolledBack = 0, 0, 0, 0
	for _, item := range result.Items {
		switch item.Status {
		case BatchStatusApplied:
			result.Applied++
		case BatchStatusSkipped:
			result.Skipped++
		case BatchStatusFailed:
			result.Failed++
		case BatchStatusRolledBack:
			result.RolledBack++
		}
	}
}

// batchFingerprint identifies the request a key was used for. It covers the
// arguments, not the key, so a key reused with different work is detectable.
func batchFingerprint(req BatchRequest) string {
	scrubbed := req
	scrubbed.RequestKey = ""
	encoded, err := json.Marshal(scrubbed)
	if err != nil {
		// Marshalling a struct of strings cannot fail; treat any surprise as a
		// mismatch rather than silently colliding with another request.
		return "unfingerprintable"
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (s *SQLiteStore) lookupBatchLocked(key string) (BatchResult, bool, string, error) {
	stmt, err := s.prepareLocked(`SELECT response, request_sha256 FROM batch_operations WHERE request_key = ?`)
	if err != nil {
		return BatchResult{}, false, "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{key}); err != nil {
		return BatchResult{}, false, "", err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return BatchResult{}, false, "", nil
	}
	if rc != C.SQLITE_ROW {
		return BatchResult{}, false, "", s.stepErrLocked(rc)
	}
	var stored BatchResult
	if err := json.Unmarshal([]byte(columnText(stmt, 0)), &stored); err != nil {
		return BatchResult{}, false, "", err
	}
	return stored, true, columnText(stmt, 1), nil
}

func (s *SQLiteStore) recordBatchLocked(key string, req BatchRequest, fingerprint string, result BatchResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.execPreparedLocked(
		`INSERT OR REPLACE INTO batch_operations(request_key, operation, mode, response, request_sha256) VALUES(?, ?, ?, ?, ?)`,
		key, req.Operation, req.Mode, string(encoded), fingerprint)
}
