package store

/*
#cgo pkg-config: sqlite3
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/markdownblocks"
)

// DocumentBlock is one addressable region of a note.
//
// Blocks are derived state, like FTS5 rows and links: they are rebuilt from the
// body in the same transaction as the save that produced them, and they never
// outlive their document.
type DocumentBlock struct {
	ID           string
	DocumentID   string
	Ordinal      int
	Kind         string
	HeadingLevel int
	Marker       string
	// HeadingSlug is the URI-safe name a `#section-title` anchor resolves
	// against. Only heading blocks have one.
	HeadingSlug   string
	ContentSHA256 string
	StartByte     int
	EndByte       int
	// Backlinks counts links from other notes that name this block. It is
	// populated by ListDocumentBlocks and left zero elsewhere.
	Backlinks int
}

// ListDocumentBlocks returns a note's blocks in document order, each with the
// number of links that name it.
//
// Blocks per note are bounded by the parser, so this returns a whole note's
// blocks rather than a page: the bound is the note, not the library.
func (s *SQLiteStore) ListDocumentBlocks(ctx context.Context, documentID string) ([]DocumentBlock, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(documentID) == "" {
		return nil, fmt.Errorf("%w: a document ID is required", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.getDocumentLocked(documentID); err != nil {
		return nil, err
	}
	blocks, err := s.listDocumentBlocksLocked(documentID)
	if err != nil {
		return nil, err
	}
	// An anchor can be written as the author's marker, the derived block ID, or
	// a heading slug, so a backlink count has to consider all three.
	counts, err := s.blockAnchorCountsLocked(documentID)
	if err != nil {
		return nil, err
	}
	for i := range blocks {
		// An anchor can be written three ways — the author's marker, the derived
		// block ID, or a heading slug — and each of them names this block.
		blocks[i].Backlinks = counts[blocks[i].ID] + counts[blocks[i].Marker] + counts[blocks[i].HeadingSlug]
	}
	return blocks, nil
}

func (s *SQLiteStore) listDocumentBlocksLocked(documentID string) ([]DocumentBlock, error) {
	stmt, err := s.prepareLocked(`SELECT id, ordinal, kind, heading_level, COALESCE(marker, ''), content_sha256, start_byte, end_byte, COALESCE(heading_slug, '')
		FROM document_blocks WHERE document_id = ? ORDER BY ordinal`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return nil, err
	}
	blocks := []DocumentBlock{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		blocks = append(blocks, DocumentBlock{
			ID:            columnText(stmt, 0),
			DocumentID:    documentID,
			Ordinal:       int(C.sqlite3_column_int(stmt, 1)),
			Kind:          columnText(stmt, 2),
			HeadingLevel:  int(C.sqlite3_column_int(stmt, 3)),
			Marker:        columnText(stmt, 4),
			ContentSHA256: columnText(stmt, 5),
			StartByte:     int(C.sqlite3_column_int(stmt, 6)),
			EndByte:       int(C.sqlite3_column_int(stmt, 7)),
			HeadingSlug:   columnText(stmt, 8),
		})
	}
	return blocks, nil
}

// blockAnchorCountsLocked counts incoming anchors by the name they resolve to.
//
// A heading anchor may be written as the heading's text or as its slug, so
// heading values are normalized the same way resolution normalizes them;
// otherwise `#Section Title` and `#section-title` would count as two different
// anchors against one heading.
func (s *SQLiteStore) blockAnchorCountsLocked(documentID string) (map[string]int, error) {
	stmt, err := s.prepareLocked(`SELECT anchor_type, anchor_value, COUNT(*) FROM document_links
		WHERE target_document_id = ? AND anchor_type IN ('block', 'heading')
			AND anchor_value IS NOT NULL AND anchor_value != ''
		GROUP BY anchor_type, anchor_value`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		value := columnText(stmt, 1)
		if columnText(stmt, 0) == "heading" {
			value = markdownblocks.Slugify(value)
		}
		counts[value] += int(C.sqlite3_column_int(stmt, 2))
	}
	delete(counts, "")
	return counts, nil
}

// FindDocumentBlock resolves an anchor value against one note's blocks.
//
// Precedence is marker, then block ID, then heading slug. The author's own
// marker wins because it is a name the author wrote and it survives edits to
// the block's text, which a content-derived ID deliberately does not
// (PROJECT_DECISIONS.md 17). A heading slug comes last because it is the least
// specific: it names a section rather than an exact piece of text.
func (s *SQLiteStore) FindDocumentBlock(ctx context.Context, documentID, anchor string) (DocumentBlock, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DocumentBlock{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findDocumentBlockLocked(documentID, anchor)
}

func (s *SQLiteStore) findDocumentBlockLocked(documentID, anchor string) (DocumentBlock, error) {
	anchor = strings.TrimSpace(anchor)
	if documentID == "" || anchor == "" {
		return DocumentBlock{}, fmt.Errorf("%w: a document ID and anchor are required", ErrInvalidInput)
	}
	// A heading anchor may arrive already slugged (`#section-title`, what a
	// stable link carries) or as the heading's text (`#Section Title`, what
	// Obsidian writes and what the importer preserves). Normalizing here means
	// both spellings reach the same heading without the URI parser having to
	// accept spaces or percent-escapes.
	slug := markdownblocks.Slugify(anchor)
	stmt, err := s.prepareLocked(`SELECT id, ordinal, kind, heading_level, COALESCE(marker, ''), content_sha256, start_byte, end_byte, COALESCE(heading_slug, '')
		FROM document_blocks WHERE document_id = ? AND (marker = ? OR id = ? OR (heading_slug IS NOT NULL AND heading_slug = ?))
		ORDER BY CASE WHEN marker = ? THEN 0 WHEN id = ? THEN 1 ELSE 2 END, ordinal LIMIT 1`)
	if err != nil {
		return DocumentBlock{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID, anchor, anchor, slug, anchor, anchor}); err != nil {
		return DocumentBlock{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return DocumentBlock{}, fmt.Errorf("%w: block %q in document %s", ErrNotFound, anchor, documentID)
	}
	if rc != C.SQLITE_ROW {
		return DocumentBlock{}, s.stepErrLocked(rc)
	}
	return DocumentBlock{
		ID:            columnText(stmt, 0),
		DocumentID:    documentID,
		Ordinal:       int(C.sqlite3_column_int(stmt, 1)),
		Kind:          columnText(stmt, 2),
		HeadingLevel:  int(C.sqlite3_column_int(stmt, 3)),
		Marker:        columnText(stmt, 4),
		ContentSHA256: columnText(stmt, 5),
		StartByte:     int(C.sqlite3_column_int(stmt, 6)),
		EndByte:       int(C.sqlite3_column_int(stmt, 7)),
		HeadingSlug:   columnText(stmt, 8),
	}, nil
}

// RebuildDocumentBlocks re-derives one note's blocks without writing a
// revision. Importers use it after a batch, and it is how a database upgraded
// to schema v14/v15 fills in blocks and heading slugs for notes nobody has
// edited since.
func (s *SQLiteStore) RebuildDocumentBlocks(ctx context.Context, documentID string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.getDocumentLocked(documentID)
	if err != nil {
		return err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.rebuildDocumentBlocksLocked(document.ID, document.Body); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// rebuildDocumentBlocksLocked replaces a note's block rows. It runs inside the
// caller's transaction, alongside the link rebuild, so blocks and links always
// describe the same body.
func (s *SQLiteStore) rebuildDocumentBlocksLocked(documentID, body string) error {
	if err := s.execPreparedLocked(`DELETE FROM document_blocks WHERE document_id = ?`, documentID); err != nil {
		return err
	}
	for _, block := range markdownblocks.Extract(documentID, body) {
		marker := block.Marker
		if err := s.execPreparedLocked(`INSERT INTO document_blocks(
			id, document_id, ordinal, kind, heading_level, marker, content_sha256, start_byte, end_byte, heading_slug
		) VALUES(?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''))`,
			block.ID, documentID, strconv.Itoa(block.Ordinal), block.Kind, strconv.Itoa(block.Level),
			marker, block.ContentSHA256, strconv.Itoa(block.StartByte), strconv.Itoa(block.EndByte), block.Slug); err != nil {
			return err
		}
	}
	return nil
}

// headingSlugsLocked returns one document's heading slugs. Blocks per note are
// bounded by the parser, so this set is bounded by a note rather than by the
// library.
func (s *SQLiteStore) headingSlugsLocked(documentID string) (map[string]bool, error) {
	stmt, err := s.prepareLocked(`SELECT heading_slug FROM document_blocks
		WHERE document_id = ? AND heading_slug IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return nil, err
	}
	slugs := map[string]bool{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		if slug := columnText(stmt, 0); slug != "" {
			slugs[slug] = true
		}
	}
	return slugs, nil
}
