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
	"sort"
	"strconv"

	"github.com/renesugar/notrios/internal/syncbody"
	"github.com/renesugar/notrios/internal/syncdelta"
)

// maxStoredConflictRegions and maxStoredConflictRegionBytes bound what a
// conflict row keeps for display. The three revisions it names hold the
// complete text, so this is a convenience rather than the record of record,
// and it must not turn one disagreement into a multi-megabyte row.
const (
	maxStoredConflictRegions     = 16
	maxStoredConflictRegionBytes = 8 << 10
)

// maxMergeRounds bounds the head-reduction loop. Each round merges two heads
// into one, so a document with n heads finishes in n-1 rounds; the bound exists
// so a defect cannot turn admission into an unbounded loop.
const maxMergeRounds = 64

// syncRevisionRow is one revision as this replica holds it.
type syncRevisionRow struct {
	id            string
	documentID    string
	title         string
	mimeType      string
	body          string
	contentSHA256 string
	contentLength int
	parents       []string
}

// reconcileSyncRevisionsLocked converges note bodies for the documents a batch
// touched. It runs inside the caller's admission transaction, after G6 has
// applied metadata, so a merge sees the converged title and lifecycle rather
// than a half-applied one.
func (s *SQLiteStore) reconcileSyncRevisionsLocked(documentIDs []string) error {
	candidates, err := s.syncRevisionCandidateDocumentsLocked(documentIDs)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	if err := s.materializeAdmittedRevisionsLocked(); err != nil {
		return err
	}
	for _, documentID := range candidates {
		if err := s.convergeDocumentBodyLocked(documentID); err != nil {
			return err
		}
	}
	return nil
}

// syncRevisionCandidateDocumentsLocked returns the documents this pass must
// look at: the ones a revision operation named, the ones already carrying a
// conflict (so a resolution clears it), and the ones waiting for a body (so a
// newly available base is retried).
func (s *SQLiteStore) syncRevisionCandidateDocumentsLocked(documentIDs []string) ([]string, error) {
	unique := map[string]bool{}
	for _, id := range documentIDs {
		if id != "" {
			unique[id] = true
		}
	}
	for _, query := range []string{
		`SELECT DISTINCT document_id FROM sync_document_conflicts`,
		`SELECT DISTINCT document_id FROM sync_revision_pending_bodies`,
	} {
		stmt, err := s.prepareLocked(query)
		if err != nil {
			return nil, err
		}
		for {
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_DONE {
				break
			}
			if rc != C.SQLITE_ROW {
				C.sqlite3_finalize(stmt)
				return nil, s.stepErrLocked(rc)
			}
			unique[columnText(stmt, 0)] = true
		}
		C.sqlite3_finalize(stmt)
	}
	candidates := make([]string, 0, len(unique))
	for id := range unique {
		candidates = append(candidates, id)
	}
	sort.Strings(candidates)
	return candidates, nil
}

// materializeAdmittedRevisionsLocked turns revision operations into revision
// rows. It repeats until no further progress is possible, because a delta's
// base may itself be a revision admitted in the same batch.
//
// Nothing here writes a body it has not verified against the operation's exact
// hash and length. A delta that names an absent base, fails to decode, or
// reconstructs to different bytes is refused and recorded as a pending body;
// the alternative — applying it and hoping — would put unverified content into
// canonical storage, which the whole slice exists to prevent.
func (s *SQLiteStore) materializeAdmittedRevisionsLocked() error {
	return s.withSyncApplyGuardLocked(func() error {
		for round := 0; round < maxMergeRounds; round++ {
			operations, err := s.unmaterializedRevisionOperationsLocked()
			if err != nil {
				return err
			}
			if len(operations) == 0 {
				return nil
			}
			progress := false
			reasons := make(map[string]string, len(operations))
			for _, operation := range operations {
				materialized, reason, err := s.materializeRevisionLocked(operation)
				if err != nil {
					return err
				}
				progress = progress || materialized
				reasons[operation.revisionID] = reason
			}
			if !progress {
				// Everything that remains is waiting for bytes this replica
				// does not have. Record why, once, so the state is visible
				// rather than silently missing.
				for _, operation := range operations {
					if reasons[operation.revisionID] == "" {
						continue
					}
					if err := s.recordPendingRevisionBodyLocked(operation, reasons[operation.revisionID]); err != nil {
						return err
					}
				}
				return nil
			}
		}
		return nil
	})
}

type revisionOperation struct {
	revisionID      string
	documentID      string
	title           string
	mimeType        string
	message         string
	parents         []string
	contentSHA256   string
	contentLength   int
	body            *string
	delta           *revisionDeltaPayload
	authorReplicaID string
	authorSequence  int64
}

func (s *SQLiteStore) unmaterializedRevisionOperationsLocked() ([]revisionOperation, error) {
	stmt, err := s.prepareLocked(`
		SELECT o.record_id, o.payload_json, o.replica_id, o.sequence
		  FROM sync_operations o
		 WHERE o.record_type = 'revision' AND o.kind = 'revision.create'
		   AND NOT EXISTS (SELECT 1 FROM document_revisions r WHERE r.id = o.record_id)
		 ORDER BY o.hlc_wall_ms, o.hlc_logical, o.replica_id, o.sequence`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var operations []revisionOperation
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return operations, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		var payload struct {
			DocumentID    string                `json:"document_id"`
			Title         string                `json:"title"`
			BodyMIMEType  string                `json:"body_mime_type"`
			Message       string                `json:"message"`
			Parents       []string              `json:"parents"`
			ContentSHA256 string                `json:"content_sha256"`
			ContentLength int                   `json:"content_length"`
			Body          *string               `json:"body"`
			Delta         *revisionDeltaPayload `json:"delta"`
		}
		if err := json.Unmarshal([]byte(columnText(stmt, 1)), &payload); err != nil {
			return nil, fmt.Errorf("revision operation payload: %w", err)
		}
		if payload.DocumentID == "" || len(payload.ContentSHA256) != 64 || payload.ContentLength < 0 {
			return nil, fmt.Errorf("%w: revision operation %q does not name its content", ErrInvalidInput, columnText(stmt, 0))
		}
		operations = append(operations, revisionOperation{
			revisionID: columnText(stmt, 0), documentID: payload.DocumentID,
			title: payload.Title, mimeType: defaultMIME(payload.BodyMIMEType), message: payload.Message,
			parents:       syncbody.NormalizeParents(payload.Parents),
			contentSHA256: payload.ContentSHA256, contentLength: payload.ContentLength,
			body: payload.Body, delta: payload.Delta,
			authorReplicaID: columnText(stmt, 2), authorSequence: columnInt64(stmt, 3),
		})
	}
}

// materializeRevisionLocked reports whether it produced a verified revision row,
// and when it did not, why the revision's bytes are unavailable.
func (s *SQLiteStore) materializeRevisionLocked(operation revisionOperation) (bool, string, error) {
	body, available, reason, err := s.reconstructRevisionBodyLocked(operation)
	if err != nil {
		return false, "", err
	}
	if !available {
		return false, reason, nil
	}
	if len(body) != operation.contentLength || syncdelta.SHA256Hex(body) != operation.contentSHA256 {
		// The operation contradicts itself. Refusing keeps the unverifiable
		// bytes out of canonical storage; the revision stays a named object
		// this replica does not hold.
		alreadyRefused, err := s.countLocked(
			`SELECT COUNT(*) FROM sync_revision_pending_bodies WHERE revision_id = ? AND reason = 'unverified'`,
			operation.revisionID)
		if err != nil {
			return false, "", err
		}
		if err := s.recordPendingRevisionBodyLocked(operation, "unverified"); err != nil {
			return false, "", err
		}
		if alreadyRefused > 0 {
			// Every later admission re-examines this operation. The refusal is
			// audited once, not once per pass.
			return false, "unverified", nil
		}
		return false, "unverified", s.recordRevisionAuditLocked("revision.unverified", operation)
	}
	encodedParents, err := json.Marshal(operation.parents)
	if err != nil {
		return false, "", err
	}
	if err := s.execPreparedLocked(
		`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message, content_sha256, content_length, parent_revision_ids)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
		operation.revisionID, operation.documentID, operation.title, string(body), operation.mimeType,
		operation.message, operation.contentSHA256, strconv.Itoa(operation.contentLength), string(encodedParents)); err != nil {
		return false, "", err
	}
	return true, "", s.execPreparedLocked(`DELETE FROM sync_revision_pending_bodies WHERE revision_id = ?`, operation.revisionID)
}

// reconstructRevisionBodyLocked returns the complete body an operation carries,
// either inline or by applying its delta to a base this replica already holds.
// The third return names why the bytes are unavailable, which is the difference
// between a revision that will arrive later and one that never will:
// "missing_base" is waiting, "unverified" is a refusal, "oversize" needs the
// object transfer G8 and G9 own.
func (s *SQLiteStore) reconstructRevisionBodyLocked(operation revisionOperation) ([]byte, bool, string, error) {
	if operation.body != nil {
		return []byte(*operation.body), true, "", nil
	}
	if operation.delta == nil {
		return nil, false, "oversize", nil
	}
	baseBody, baseSHA, found, err := s.revisionBodyLocked(operation.delta.BaseRevisionID)
	if err != nil {
		return nil, false, "", err
	}
	if !found || baseSHA != operation.delta.BaseSHA256 {
		return nil, false, "missing_base", nil
	}
	raw, err := base64.StdEncoding.DecodeString(operation.delta.Base64)
	if err != nil {
		return nil, false, "unverified", nil
	}
	reconstructed, err := syncdelta.ReconstructBody(context.Background(), []byte(baseBody), syncdelta.Delta{
		Format:       operation.delta.Format,
		BaseSHA256:   operation.delta.BaseSHA256,
		BaseLength:   operation.delta.BaseLength,
		ResultSHA256: operation.delta.ResultSHA256,
		ResultLength: operation.delta.ResultLength,
		Bytes:        raw,
	})
	if err != nil {
		// A refused patch is not an admission failure: the operation stays,
		// and the complete object remains obtainable from its publisher.
		return nil, false, "unverified", nil
	}
	return reconstructed, true, "", nil
}

func (s *SQLiteStore) recordPendingRevisionBodyLocked(operation revisionOperation, reason string) error {
	encodedParents, err := json.Marshal(operation.parents)
	if err != nil {
		return err
	}
	return s.execPreparedLocked(
		`INSERT INTO sync_revision_pending_bodies(revision_id, document_id, title, body_mime_type, message,
			parent_revision_ids, content_sha256, content_length, reason, author_replica_id, author_sequence)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 -- "unverified" is terminal: the operation contradicts itself, and no
		 -- later round can make it verifiable. Letting a subsequent "waiting for
		 -- bytes" pass overwrite it would turn a refusal into a promise.
		 ON CONFLICT(revision_id) DO UPDATE SET reason = CASE
			WHEN sync_revision_pending_bodies.reason = 'unverified' THEN 'unverified'
			ELSE excluded.reason END`,
		operation.revisionID, operation.documentID, operation.title, operation.mimeType, operation.message,
		string(encodedParents), operation.contentSHA256, strconv.Itoa(operation.contentLength), reason,
		operation.authorReplicaID, strconv.FormatInt(operation.authorSequence, 10))
}

// convergeDocumentBodyLocked reduces one document to a single head, merging
// concurrent edits or recording a durable conflict, and then points the
// document at the revision that convergence chose.
func (s *SQLiteStore) convergeDocumentBodyLocked(documentID string) error {
	if err := s.withSyncApplyGuardLocked(func() error {
		return s.execPreparedLocked(`DELETE FROM sync_document_conflicts WHERE document_id = ?`, documentID)
	}); err != nil {
		return err
	}
	for round := 0; round < maxMergeRounds; round++ {
		rows, complete, err := s.documentRevisionRowsLocked(documentID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if !complete {
			// Some revision names a parent whose bytes have not arrived. The
			// graph is incomplete, so any merge base found in it could be
			// wrong. Waiting is the only honest option; the next admission
			// that supplies the parent retries this document.
			return nil
		}
		graph, err := syncbody.NewGraph(revisionsFor(rows))
		if err != nil {
			return err
		}
		heads := graph.Heads()
		if len(heads) <= 1 {
			return s.settleDocumentLocked(documentID, rows, graph, heads)
		}
		merged, err := s.mergeTwoHeadsLocked(documentID, rows, graph, heads[0], heads[1])
		if err != nil {
			return err
		}
		if !merged {
			return s.settleDocumentLocked(documentID, rows, graph, heads)
		}
	}
	return nil
}

func (s *SQLiteStore) settleDocumentLocked(documentID string, rows map[string]syncRevisionRow, graph syncbody.Graph, heads []string) error {
	if err := s.finishDocumentConvergenceLocked(documentID, rows, heads); err != nil {
		return err
	}
	if len(heads) == 0 {
		return nil
	}
	_, head, err := s.documentCurrentStateLocked(documentID)
	if err != nil {
		return err
	}
	return s.detectDeleteEditConflictLocked(documentID, head, rows, graph)
}

// detectDeleteEditConflictLocked records the one conflict that is not about
// text at all: a document deleted on one replica while another was editing it.
//
// Every `document.trash` operation names the revision that was current when it
// was made, so the question has an exact answer rather than a heuristic. If the
// deleting replica was looking at the revision the document ended up on, the
// deletion accounted for everything and is an ordinary trash. If it was looking
// at anything else, edits existed that it never saw, and no lifecycle
// last-writer rule can honour both intents — G6 already decided which one wins,
// and this records what the loser was so a person can undo it.
func (s *SQLiteStore) detectDeleteEditConflictLocked(documentID, head string, rows map[string]syncRevisionRow, graph syncbody.Graph) error {
	if head == "" {
		return nil
	}
	trashed, err := s.countLocked(`SELECT COUNT(*) FROM documents WHERE id = ? AND deleted_at IS NOT NULL`, documentID)
	if err != nil || trashed == 0 {
		return err
	}
	stmt, err := s.prepareLocked(
		`SELECT payload_json FROM sync_operations
		  WHERE record_type = 'document' AND record_id = ? AND kind = 'document.trash'
		  ORDER BY hlc_wall_ms DESC, hlc_logical DESC, replica_id DESC, sequence DESC LIMIT 1`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return nil
	}
	if rc != C.SQLITE_ROW {
		return s.stepErrLocked(rc)
	}
	var payload struct {
		CurrentRevisionID string `json:"current_revision_id"`
	}
	if err := json.Unmarshal([]byte(columnText(stmt, 0)), &payload); err != nil {
		return fmt.Errorf("trash operation payload: %w", err)
	}
	seen := payload.CurrentRevisionID
	if seen == "" || seen == head {
		return nil
	}
	if _, held := rows[seen]; !held {
		return nil
	}
	return s.recordBodyConflictLocked(documentID, seen, seen, head, syncbody.Result{
		Kind: syncbody.ConflictDeleteEdit,
		Regions: []syncbody.Region{{
			Kind: syncbody.ConflictDeleteEdit, BaseLine: 1, LocalLine: 1, RemoteLine: 1,
			Base: clipConflictText(rows[seen].body), Local: "", Remote: clipConflictText(rows[head].body),
		}},
	})
}

// mergeTwoHeadsLocked reports whether it created a merge revision. A false
// return means the two heads conflict and the conflict has been recorded.
func (s *SQLiteStore) mergeTwoHeadsLocked(documentID string, rows map[string]syncRevisionRow, graph syncbody.Graph, left, right string) (bool, error) {
	baseID, err := graph.MergeBase(left, right)
	if err != nil {
		return false, s.recordBodyConflictLocked(documentID, "", left, right, syncbody.Result{
			Kind:    syncbody.ConflictBoundsExceeded,
			Regions: []syncbody.Region{{Kind: syncbody.ConflictBoundsExceeded, BaseLine: 1, LocalLine: 1, RemoteLine: 1}},
		})
	}
	result := syncbody.Merge(rows[baseID].body, rows[left].body, rows[right].body, syncbody.Limits{})
	if !result.Clean {
		return false, s.recordBodyConflictLocked(documentID, baseID, left, right, result)
	}

	// The merge revision's identity is derived from the document, its parents,
	// and the merged bytes, so a peer computing the same merge produces the
	// same revision rather than a rival head. Both may emit it; the second copy
	// to arrive is an exact replay.
	mergedBody := result.Merged
	parents := syncbody.NormalizeParents([]string{left, right})
	revisionID := syncbody.MergeRevisionID(documentID, parents, syncdelta.SHA256Hex([]byte(mergedBody)))
	if _, exists := rows[revisionID]; exists {
		return false, nil
	}
	title, mimeType, err := s.documentTitleAndMIMELocked(documentID)
	if err != nil {
		return false, err
	}
	encodedParents, err := json.Marshal(parents)
	if err != nil {
		return false, err
	}
	// Deliberately not guarded: a merge this replica computed is a local
	// authored act and must reach the journal like any other revision.
	if err := s.prepareRevisionTransferLocked(revisionID, left, mergedBody, syncdelta.SHA256Hex([]byte(mergedBody))); err != nil {
		return false, err
	}
	if err := s.execPreparedLocked(
		`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message, content_sha256, content_length, parent_revision_ids)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		revisionID, documentID, title, mergedBody, mimeType, "merged "+left+" and "+right,
		syncdelta.SHA256Hex([]byte(mergedBody)), strconv.Itoa(len(mergedBody)), string(encodedParents)); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLiteStore) recordBodyConflictLocked(documentID, baseRevisionID, left, right string, result syncbody.Result) error {
	pair := syncbody.NormalizeParents([]string{left, right})
	if len(pair) != 2 {
		return fmt.Errorf("%w: a conflict needs two distinct revisions", ErrInvalidInput)
	}
	regions := result.Regions
	truncated := false
	if len(regions) > maxStoredConflictRegions {
		regions, truncated = regions[:maxStoredConflictRegions], true
	}
	stored := make([]syncbody.Region, 0, len(regions))
	for _, region := range regions {
		region.Base, region.Local, region.Remote =
			clipConflictText(region.Base), clipConflictText(region.Local), clipConflictText(region.Remote)
		truncated = truncated || region.Truncated
		stored = append(stored, region)
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return s.withSyncApplyGuardLocked(func() error {
		return s.execPreparedLocked(
			`INSERT INTO sync_document_conflicts(id, document_id, base_revision_id, revision_a, revision_b, kind,
				region_count, regions_json, regions_truncated)
			 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET kind=excluded.kind, region_count=excluded.region_count,
				regions_json=excluded.regions_json, regions_truncated=excluded.regions_truncated`,
			syncbody.ConflictID(documentID, baseRevisionID, pair), documentID, baseRevisionID, pair[0], pair[1],
			result.Kind, strconv.Itoa(len(result.Regions)), string(encoded), boolNumber(truncated))
	})
}

func clipConflictText(value string) string {
	if len(value) <= maxStoredConflictRegionBytes {
		return value
	}
	cut := maxStoredConflictRegionBytes
	for cut > 0 && value[cut]&0xC0 == 0x80 {
		cut--
	}
	return value[:cut]
}

// finishDocumentConvergenceLocked points the document at the revision
// convergence selected and refreshes everything derived from its body.
//
// With one head the choice is that head. With two the document still has to
// show something, so it shows the newer of the two by the same
// (wall, logical, replica, sequence) order G6 uses for metadata — a rule both
// replicas evaluate identically, so a conflicted document does not additionally
// disagree about which side it is displaying.
func (s *SQLiteStore) finishDocumentConvergenceLocked(documentID string, rows map[string]syncRevisionRow, heads []string) error {
	if len(heads) == 0 {
		return nil
	}
	selected := heads[0]
	if len(heads) > 1 {
		bestOrder, err := s.revisionOrderLocked(selected)
		if err != nil {
			return err
		}
		for _, candidate := range heads[1:] {
			order, err := s.revisionOrderLocked(candidate)
			if err != nil {
				return err
			}
			if compareRevisionOrder(order, bestOrder) > 0 {
				selected, bestOrder = candidate, order
			}
		}
	}
	row, found := rows[selected]
	if !found {
		return nil
	}
	collectionID, currentRevisionID, err := s.documentCurrentStateLocked(documentID)
	if err != nil {
		return err
	}
	if currentRevisionID == selected {
		return nil
	}
	return s.withSyncApplyGuardLocked(func() error {
		if err := s.execPreparedLocked(
			`UPDATE documents SET current_revision_id = ?, body_mime_type = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			selected, row.mimeType, documentID); err != nil {
			return err
		}
		title, _, err := s.documentTitleAndMIMELocked(documentID)
		if err != nil {
			return err
		}
		if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id = ?`, documentID); err != nil {
			return err
		}
		deleted, err := s.countLocked(`SELECT COUNT(*) FROM documents WHERE id = ? AND deleted_at IS NOT NULL`, documentID)
		if err != nil {
			return err
		}
		if deleted == 0 {
			if err := s.execPreparedLocked(
				`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`,
				documentID, collectionID, title, row.body); err != nil {
				return err
			}
		}
		if err := s.rebuildDocumentLinksLocked(documentID, collectionID, row.body); err != nil {
			return err
		}
		return s.enqueueProjectionLocked(documentID, "upsert")
	})
}

type revisionOrder struct {
	wallMS    int64
	logical   int64
	replicaID string
	sequence  int64
}

func compareRevisionOrder(left, right revisionOrder) int {
	switch {
	case left.wallMS != right.wallMS:
		return compareInt64(left.wallMS, right.wallMS)
	case left.logical != right.logical:
		return compareInt64(left.logical, right.logical)
	case left.replicaID != right.replicaID:
		if left.replicaID < right.replicaID {
			return -1
		}
		return 1
	default:
		return compareInt64(left.sequence, right.sequence)
	}
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// revisionOrderLocked reads a revision's protocol order from the journal. A
// revision created before enrollment has no operation and therefore the zero
// order, which correctly places every snapshot revision before every
// synchronized one.
func (s *SQLiteStore) revisionOrderLocked(revisionID string) (revisionOrder, error) {
	stmt, err := s.prepareLocked(
		`SELECT hlc_wall_ms, hlc_logical, replica_id, sequence FROM sync_operations
		  WHERE record_type = 'revision' AND record_id = ? ORDER BY hlc_wall_ms, hlc_logical, replica_id, sequence LIMIT 1`)
	if err != nil {
		return revisionOrder{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{revisionID}); err != nil {
		return revisionOrder{}, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return revisionOrder{columnInt64(stmt, 0), columnInt64(stmt, 1), columnText(stmt, 2), columnInt64(stmt, 3)}, nil
	case C.SQLITE_DONE:
		return revisionOrder{}, nil
	default:
		return revisionOrder{}, s.stepErrLocked(rc)
	}
}

// documentRevisionRowsLocked reads one document's revisions. The second return
// reports whether the graph is complete: a revision naming a parent this
// replica does not hold makes every ancestor question unanswerable.
func (s *SQLiteStore) documentRevisionRowsLocked(documentID string) (map[string]syncRevisionRow, bool, error) {
	stmt, err := s.prepareLocked(
		`SELECT id, title, body, body_mime_type, content_sha256, content_length, parent_revision_ids
		   FROM document_revisions WHERE document_id = ? ORDER BY id`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return nil, false, err
	}
	rows := map[string]syncRevisionRow{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, false, s.stepErrLocked(rc)
		}
		var parents []string
		if err := json.Unmarshal([]byte(columnText(stmt, 6)), &parents); err != nil {
			return nil, false, fmt.Errorf("revision %s parents: %w", columnText(stmt, 0), err)
		}
		rows[columnText(stmt, 0)] = syncRevisionRow{
			id: columnText(stmt, 0), documentID: documentID, title: columnText(stmt, 1), body: columnText(stmt, 2),
			mimeType: columnText(stmt, 3), contentSHA256: columnText(stmt, 4), contentLength: int(columnInt64(stmt, 5)),
			parents: syncbody.NormalizeParents(parents),
		}
	}
	for _, row := range rows {
		for _, parent := range row.parents {
			if _, held := rows[parent]; !held {
				return rows, false, nil
			}
		}
	}
	return rows, true, nil
}

func revisionsFor(rows map[string]syncRevisionRow) []syncbody.Revision {
	revisions := make([]syncbody.Revision, 0, len(rows))
	for _, row := range rows {
		revisions = append(revisions, syncbody.Revision{
			ID: row.id, DocumentID: row.documentID, Parents: row.parents,
			ContentSHA256: row.contentSHA256, ContentLength: row.contentLength,
		})
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].ID < revisions[j].ID })
	return revisions
}

func (s *SQLiteStore) documentTitleAndMIMELocked(documentID string) (string, string, error) {
	stmt, err := s.prepareLocked(`SELECT title, body_mime_type FROM documents WHERE id = ?`)
	if err != nil {
		return "", "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return "", "", err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return columnText(stmt, 0), defaultMIME(columnText(stmt, 1)), nil
	case C.SQLITE_DONE:
		return "", "text/markdown", nil
	default:
		return "", "", s.stepErrLocked(rc)
	}
}

// SyncBodyStatus is the read-only summary of G7 body convergence. It is
// deliberately three counts rather than a query surface: nothing outside the
// store composes SQL against these tables, and a report that named documents or
// carried conflicting text would be a disclosure rather than a status.
type SyncBodyStatus struct {
	Conflicts       int
	ConflictKinds   map[string]int
	PendingBodies   int
	PendingReasons  map[string]int
	MergeRevisions  int
	StoredDeltas    int
	DeltaBytes      int64
	DeltaBodyBytes  int64
	CompleteRecords int
}

// SyncBodyStatus reports the local G7 convergence counters.
func (s *SQLiteStore) SyncBodyStatus(ctx context.Context) (SyncBodyStatus, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncBodyStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var status SyncBodyStatus
	for _, counter := range []struct {
		query string
		into  *int
	}{
		{`SELECT COUNT(*) FROM sync_document_conflicts`, &status.Conflicts},
		{`SELECT COUNT(*) FROM sync_revision_pending_bodies`, &status.PendingBodies},
		{`SELECT COUNT(*) FROM document_revisions WHERE json_array_length(parent_revision_ids) = 2`, &status.MergeRevisions},
		{`SELECT COUNT(*) FROM sync_revision_deltas`, &status.StoredDeltas},
		{`SELECT COUNT(*) FROM document_revisions`, &status.CompleteRecords},
	} {
		value, err := s.countLocked(counter.query)
		if err != nil {
			return SyncBodyStatus{}, err
		}
		*counter.into = int(value)
	}
	for _, counter := range []struct {
		query string
		into  *int64
	}{
		{`SELECT COALESCE(SUM(delta_length), 0) FROM sync_revision_deltas`, &status.DeltaBytes},
		{`SELECT COALESCE(SUM(result_length), 0) FROM sync_revision_deltas`, &status.DeltaBodyBytes},
	} {
		value, err := s.countLocked(counter.query)
		if err != nil {
			return SyncBodyStatus{}, err
		}
		*counter.into = value
	}
	// Counting conflicts without naming their kind hides the difference between
	// two people editing one sentence and a merge that ran out of budget. Only
	// the second is a reason to change anything in the code.
	status.ConflictKinds = map[string]int{}
	status.PendingReasons = map[string]int{}
	for _, group := range []struct {
		query string
		into  map[string]int
	}{
		{`SELECT kind, COUNT(*) FROM sync_document_conflicts GROUP BY kind ORDER BY kind`, status.ConflictKinds},
		{`SELECT reason, COUNT(*) FROM sync_revision_pending_bodies GROUP BY reason ORDER BY reason`, status.PendingReasons},
	} {
		stmt, err := s.prepareLocked(group.query)
		if err != nil {
			return SyncBodyStatus{}, err
		}
		for {
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_DONE {
				break
			}
			if rc != C.SQLITE_ROW {
				C.sqlite3_finalize(stmt)
				return SyncBodyStatus{}, s.stepErrLocked(rc)
			}
			group.into[columnText(stmt, 0)] = int(columnInt64(stmt, 1))
		}
		C.sqlite3_finalize(stmt)
	}
	return status, nil
}

// syncRevisionRowsForTest is the read seam G7's fixtures use to inspect the
// tables this file writes. Column names come from the statement, so a test
// reads what the schema calls things rather than a positional copy of it.
func (s *SQLiteStore) syncRevisionRowsForTest(query string, values ...string) ([]map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return nil, err
	}
	columns := int(C.sqlite3_column_count(stmt))
	var rows []map[string]string
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return rows, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		row := make(map[string]string, columns)
		for index := 0; index < columns; index++ {
			row[C.GoString(C.sqlite3_column_name(stmt, C.int(index)))] = columnText(stmt, index)
		}
		rows = append(rows, row)
	}
}

// recordRevisionAuditLocked leaves a durable trace of a refused revision. A
// refusal that only shows as an absence is indistinguishable from a message
// that never arrived.
func (s *SQLiteStore) recordRevisionAuditLocked(eventType string, operation revisionOperation) error {
	auditID, err := NewID("audit")
	if err != nil {
		return err
	}
	details, err := json.Marshal(map[string]string{
		"document_id":    operation.documentID,
		"content_sha256": operation.contentSHA256,
		"content_length": strconv.Itoa(operation.contentLength),
	})
	if err != nil {
		return err
	}
	return s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, ?, ?, ?, ?)`, auditID, eventType, operation.authorReplicaID, operation.revisionID, string(details))
}

// withSyncApplyGuardLocked runs canonical writes that must not become local
// operations. Applying a peer's change is not authoring one, and a guard that
// leaked would make two replicas echo each other's edits forever.
//
// Calls must not nest: the guard is a presence flag rather than a counter, so
// an inner call's exit would drop the outer call's protection.
func (s *SQLiteStore) withSyncApplyGuardLocked(apply func() error) error {
	if err := s.execLocked(`INSERT OR IGNORE INTO sync_apply_guard(singleton) VALUES(1)`); err != nil {
		return err
	}
	err := apply()
	if clearErr := s.execLocked(`DELETE FROM sync_apply_guard`); err == nil {
		err = clearErr
	}
	return err
}
