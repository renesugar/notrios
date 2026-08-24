package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/syncbody"
	"github.com/renesugar/notrios/internal/syncdelta"
)

// SyncConflictRevision is one side of the bounded, local conflict comparison.
// Bodies are returned only by the local UI endpoint; G15's REST/MCP status
// surface deliberately continues to expose content-free summaries.
type SyncConflictRevision struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type SyncConflictDetail struct {
	SyncConflictSummary
	Title          string               `json:"title"`
	BodyMIMEType   string               `json:"body_mime_type"`
	Base           SyncConflictRevision `json:"base"`
	Local          SyncConflictRevision `json:"local"`
	Remote         SyncConflictRevision `json:"remote"`
	Regions        []syncbody.Region    `json:"regions"`
	RegionCount    int                  `json:"region_count"`
	RegionsClipped bool                 `json:"regions_clipped,omitempty"`
}

type ResolveSyncConflictRequest struct {
	ConflictID string
	Title      string
	Body       string
}

// GetSyncConflictDetail returns the exact three revision bodies a person needs
// to make a decision. "Local" means the head this replica currently displays;
// it is presentation state, not an assertion that the other revision matters
// less. Revision IDs remain the protocol identities.
func (s *SQLiteStore) GetSyncConflictDetail(ctx context.Context, conflictID string) (SyncConflictDetail, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SyncConflictDetail{}, err
	}
	if strings.TrimSpace(conflictID) == "" {
		return SyncConflictDetail{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncConflictDetailLocked(conflictID)
}

func (s *SQLiteStore) syncConflictDetailLocked(conflictID string) (SyncConflictDetail, error) {
	stmt, err := s.prepareLocked(`SELECT c.id, c.document_id, COALESCE(c.base_revision_id, ''),
		c.revision_a, c.revision_b, c.kind, c.detected_at, c.region_count,
		c.regions_json, c.regions_truncated, d.title, d.body_mime_type, d.current_revision_id
		FROM sync_document_conflicts c JOIN documents d ON d.id = c.document_id
		WHERE c.id = ?`)
	if err != nil {
		return SyncConflictDetail{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{conflictID}); err != nil {
		return SyncConflictDetail{}, err
	}
	if rc := C.sqlite3_step(stmt); rc == C.SQLITE_DONE {
		return SyncConflictDetail{}, ErrNotFound
	} else if rc != C.SQLITE_ROW {
		return SyncConflictDetail{}, s.stepErrLocked(rc)
	}
	detail := SyncConflictDetail{
		SyncConflictSummary: SyncConflictSummary{
			ID: columnText(stmt, 0), DocumentID: columnText(stmt, 1), BaseRevisionID: columnText(stmt, 2),
			RevisionA: columnText(stmt, 3), RevisionB: columnText(stmt, 4), Kind: columnText(stmt, 5),
			CreatedAt: columnText(stmt, 6),
		},
		RegionCount: int(columnInt64(stmt, 7)), RegionsClipped: columnInt64(stmt, 9) != 0,
		Title: columnText(stmt, 10), BodyMIMEType: columnText(stmt, 11),
	}
	_ = json.Unmarshal([]byte(columnText(stmt, 8)), &detail.Regions)
	current := columnText(stmt, 12)
	load := func(revisionID string) (SyncConflictRevision, error) {
		if revisionID == "" {
			return SyncConflictRevision{}, nil
		}
		revision, err := s.getDocumentRevisionLocked(detail.DocumentID, revisionID)
		if err != nil {
			return SyncConflictRevision{}, err
		}
		return SyncConflictRevision{ID: revision.ID, Title: revision.Title, Body: revision.Body}, nil
	}
	if detail.Base, err = load(detail.BaseRevisionID); err != nil {
		return SyncConflictDetail{}, err
	}
	localID, remoteID := detail.RevisionA, detail.RevisionB
	if current == detail.RevisionB {
		localID, remoteID = detail.RevisionB, detail.RevisionA
	}
	if detail.Local, err = load(localID); err != nil {
		return SyncConflictDetail{}, err
	}
	if detail.Remote, err = load(remoteID); err != nil {
		return SyncConflictDetail{}, err
	}
	return detail, nil
}

// ResolveSyncConflict records the person's answer as a revision whose parents
// are both conflicting heads. A normal one-parent edit cannot resolve a graph
// with two heads; making that distinction here prevents the UI from merely
// hiding a conflict that will return on the next exchange.
func (s *SQLiteStore) ResolveSyncConflict(ctx context.Context, req ResolveSyncConflictRequest) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if strings.TrimSpace(req.ConflictID) == "" {
		return Document{}, ErrNotFound
	}
	maxBodyBytes := syncbody.DefaultLimits().MaxBodyBytes
	if len(req.Body) > maxBodyBytes {
		return Document{}, fmt.Errorf("%w: document body exceeds %d bytes", ErrInvalidInput, maxBodyBytes)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return Document{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	detail, err := s.syncConflictDetailLocked(req.ConflictID)
	if err != nil {
		return Document{}, err
	}
	current, err := s.getDocumentLocked(detail.DocumentID)
	if err != nil {
		return Document{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = current.Title
	}
	parents := syncbody.NormalizeParents([]string{detail.RevisionA, detail.RevisionB})
	contentSHA := syncdelta.SHA256Hex([]byte(req.Body))
	revisionID := syncbody.MergeRevisionID(detail.DocumentID, parents, contentSHA)
	encodedParents, err := json.Marshal(parents)
	if err != nil {
		return Document{}, err
	}
	if exists, err := s.countLocked(`SELECT COUNT(*) FROM document_revisions WHERE id = ?`, revisionID); err != nil {
		return Document{}, err
	} else if exists == 0 {
		// Multi-parent resolutions always use the complete-body transfer choice;
		// a delta has one named base and cannot honestly represent both parents.
		if err := s.prepareRevisionTransferLocked(revisionID, "", req.Body, contentSHA); err != nil {
			return Document{}, err
		}
		if err := s.execPreparedLocked(`INSERT INTO document_revisions
			(id, document_id, title, body, body_mime_type, message, content_sha256, content_length, parent_revision_ids)
			VALUES(?, ?, ?, ?, ?, 'resolved synchronization conflict', ?, ?, ?)`,
			revisionID, detail.DocumentID, title, req.Body, detail.BodyMIMEType,
			contentSHA, strconv.Itoa(len(req.Body)), string(encodedParents)); err != nil {
			return Document{}, err
		}
	}
	if err := s.execPreparedLocked(`UPDATE documents SET title = ?, body_mime_type = ?, current_revision_id = ?,
		updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`,
		title, detail.BodyMIMEType, revisionID, detail.DocumentID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id = ?`, detail.DocumentID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body)
		VALUES(?, ?, ?, ?)`, detail.DocumentID, current.CollectionID, title, req.Body); err != nil {
		return Document{}, err
	}
	if err := s.rebuildDocumentLinksLocked(detail.DocumentID, current.CollectionID, req.Body); err != nil {
		return Document{}, err
	}
	if err := s.enqueueProjectionLocked(detail.DocumentID, "upsert"); err != nil {
		return Document{}, err
	}
	if err := s.convergeDocumentBodyLocked(detail.DocumentID); err != nil {
		return Document{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return Document{}, err
	}
	committed = true
	return s.getDocumentLocked(detail.DocumentID)
}

type SyncResourceStatus struct {
	ID           string `json:"id"`
	Filename     string `json:"filename,omitempty"`
	MIMEType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	Availability string `json:"availability"`
	Pinned       bool   `json:"pinned"`
	Requested    bool   `json:"requested"`
}

func (s *SQLiteStore) ListSyncResourceStatus(ctx context.Context, limit int) ([]SyncResourceStatus, bool, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT r.id, COALESCE(r.filename, ''), r.mime_type, b.size_bytes,
		b.availability, COALESCE(m.pinned, 0), COALESCE(m.requested, 0)
		FROM resources r JOIN blobs b ON b.sha256 = r.blob_sha256
		LEFT JOIN sync_blob_materialization m ON m.blob_sha256 = b.sha256
		WHERE b.availability = 'unavailable' OR COALESCE(m.pinned, 0) = 1
		ORDER BY b.availability ASC, COALESCE(m.pinned, 0) DESC, r.created_at DESC, r.id
		LIMIT ?`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strconv.Itoa(limit + 1)}); err != nil {
		return nil, false, err
	}
	items := []SyncResourceStatus{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			if len(items) == limit {
				return items, true, nil
			}
			items = append(items, SyncResourceStatus{
				ID: columnText(stmt, 0), Filename: columnText(stmt, 1), MIMEType: columnText(stmt, 2),
				SizeBytes: columnInt64(stmt, 3), Availability: columnText(stmt, 4),
				Pinned: columnInt64(stmt, 5) != 0, Requested: columnInt64(stmt, 6) != 0,
			})
		case C.SQLITE_DONE:
			return items, false, nil
		default:
			return nil, false, s.stepErrLocked(rc)
		}
	}
}

// SetSyncResourceIntent changes only local materialization policy. Requesting
// bytes is monotonic until they arrive; unpinning does not cancel an already
// published request or delete locally held bytes.
func (s *SQLiteStore) SetSyncResourceIntent(ctx context.Context, resourceID string, pinned, requested bool) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(resourceID) == "" {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT blob_sha256 FROM resources WHERE id = ?`)
	if err != nil {
		return err
	}
	if err := bindAll(stmt, []string{resourceID}); err != nil {
		C.sqlite3_finalize(stmt)
		return err
	}
	if rc := C.sqlite3_step(stmt); rc == C.SQLITE_DONE {
		C.sqlite3_finalize(stmt)
		return ErrNotFound
	} else if rc != C.SQLITE_ROW {
		C.sqlite3_finalize(stmt)
		return s.stepErrLocked(rc)
	}
	sha := columnText(stmt, 0)
	C.sqlite3_finalize(stmt)
	return s.execPreparedLocked(`INSERT INTO sync_blob_materialization(blob_sha256, pinned, requested, last_reason)
		VALUES(?, ?, ?, 'user') ON CONFLICT(blob_sha256) DO UPDATE SET pinned = excluded.pinned,
		requested = MAX(sync_blob_materialization.requested, excluded.requested), last_reason = 'user',
		updated_at = CURRENT_TIMESTAMP`, sha, boolNumber(pinned), boolNumber(requested))
}

type SyncRepairEvent struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	SubjectID string         `json:"subject_id"`
	Details   map[string]any `json:"details,omitempty"`
}

func (s *SQLiteStore) ListSyncRepairEvents(ctx context.Context, limit int) ([]SyncRepairEvent, bool, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, event_type, subject_id, details_json
		FROM sync_repair_events ORDER BY hlc_wall_ms DESC, hlc_logical DESC, id DESC LIMIT ?`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strconv.Itoa(limit + 1)}); err != nil {
		return nil, false, err
	}
	events := []SyncRepairEvent{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			if len(events) == limit {
				return events, true, nil
			}
			event := SyncRepairEvent{ID: columnText(stmt, 0), Kind: columnText(stmt, 1), SubjectID: columnText(stmt, 2)}
			_ = json.Unmarshal([]byte(columnText(stmt, 3)), &event.Details)
			events = append(events, event)
		case C.SQLITE_DONE:
			return events, false, nil
		default:
			return nil, false, s.stepErrLocked(rc)
		}
	}
}
