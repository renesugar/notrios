package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/renesugar/notrios/internal/stablelink"
)

// Stable-link resolution statuses. They are deliberately distinct from the
// link-graph `resolution_status` values, because this answers a different
// question: not "what kind of link is this" but "what should opening it do".
const (
	// StableLinkResolved means the link names this database and a live note.
	StableLinkResolved = "resolved"
	// StableLinkTrashed means the note exists but is in the Trash. The caller
	// decides whether to open it there; silently reporting it as missing would
	// hide a note the user can still restore.
	StableLinkTrashed = "trashed"
	// StableLinkStaleTarget means this database has no such note. The link is
	// well formed and points here; the note is gone.
	StableLinkStaleTarget = "stale_target"
	// StableLinkForeignDatabase means the link names a different logical
	// database. Nothing local is looked up, because document IDs are unique
	// per database rather than globally.
	StableLinkForeignDatabase = "foreign_database"
	// StableLinkStaleAnchor means the note is here but the block or heading the
	// anchor names is not. Block identity is content-based and a heading slug
	// follows its text, so editing either breaks anchors into it; a reader
	// deserves to be told that rather than being dropped at the top of a note
	// that no longer contains what the link pointed at.
	StableLinkStaleAnchor = "stale_anchor"
)

// StableLinkResolution is the answer to "what does this notrios:// link name
// in this database?". It carries no filesystem path: the caller already chose
// which database to ask.
type StableLinkResolution struct {
	URI             string
	DatabaseID      string
	DocumentID      string
	Anchor          string
	Status          string
	LocalDatabaseID string
	// DocumentURI and Title are populated only for a link this database can
	// actually open, so a foreign link never leaks whether an ID exists here.
	DocumentURI string
	Title       string
	NotebookID  string
	// BlockID is the resolved block when the link carries an anchor.
	BlockID   string
	BlockKind string
}

// StableDocumentURI renders the external link for one of this database's
// documents.
func (s *SQLiteStore) StableDocumentURI(ctx context.Context, documentID string) (string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(documentID) == "" {
		return "", fmt.Errorf("%w: a document ID is required", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	databaseID, err := s.databaseIDLocked()
	if err != nil {
		return "", err
	}
	return stablelink.Format(databaseID, documentID), nil
}

// ResolveStableLink resolves an external link against this database.
//
// It never consults the filesystem, never opens another database, and never
// falls back to a local document when the link names a foreign database.
// Choosing which database answers a link is the caller's decision — the
// profile registry makes it explicitly and prompts on ambiguity — so this
// method's only job is to answer honestly for the database it was given.
func (s *SQLiteStore) ResolveStableLink(ctx context.Context, raw string) (StableLinkResolution, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return StableLinkResolution{}, err
	}
	parsed, err := stablelink.Parse(raw)
	if err != nil {
		return StableLinkResolution{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	localID, err := s.databaseIDLocked()
	if err != nil {
		return StableLinkResolution{}, err
	}
	resolution := StableLinkResolution{
		URI:             parsed.String(),
		DatabaseID:      parsed.DatabaseID,
		DocumentID:      parsed.DocumentID,
		Anchor:          parsed.Anchor,
		LocalDatabaseID: localID,
	}
	if parsed.DatabaseID != localID {
		resolution.Status = StableLinkForeignDatabase
		return resolution, nil
	}
	document, err := s.getDocumentLocked(parsed.DocumentID)
	if err == nil {
		resolution.Status = StableLinkResolved
		resolution.DocumentURI = document.URI
		resolution.Title = document.Title
		resolution.NotebookID = document.NotebookID
		if parsed.Anchor != "" {
			// A notrios:// link is a URI, so its anchor's percent-escapes mean
			// what RFC 3986 says they mean. The raw anchor is kept in the
			// resolution so the reply echoes what was asked.
			anchor := stablelink.DecodeAnchor(strings.TrimPrefix(parsed.Anchor, "^"))
			block, blockErr := s.findDocumentBlockLocked(parsed.DocumentID, anchor)
			switch {
			case blockErr == nil:
				resolution.BlockID = block.ID
				resolution.BlockKind = block.Kind
			case errors.Is(blockErr, ErrNotFound):
				resolution.Status = StableLinkStaleAnchor
			default:
				return StableLinkResolution{}, blockErr
			}
		}
		return resolution, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return StableLinkResolution{}, err
	}
	trashed, title, notebookID, err := s.trashedDocumentLocked(parsed.DocumentID)
	if err != nil {
		return StableLinkResolution{}, err
	}
	if trashed {
		resolution.Status = StableLinkTrashed
		resolution.DocumentURI = DocumentURI("", parsed.DocumentID)
		resolution.Title = title
		resolution.NotebookID = notebookID
		return resolution, nil
	}
	resolution.Status = StableLinkStaleTarget
	return resolution, nil
}

func (s *SQLiteStore) trashedDocumentLocked(documentID string) (bool, string, string, error) {
	stmt, err := s.prepareLocked(`SELECT title, COALESCE(notebook_id, '') FROM documents
		WHERE id = ? AND deleted_at IS NOT NULL`)
	if err != nil {
		return false, "", "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return false, "", "", err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return false, "", "", nil
	}
	if rc != C.SQLITE_ROW {
		return false, "", "", s.stepErrLocked(rc)
	}
	return true, columnText(stmt, 0), columnText(stmt, 1), nil
}
