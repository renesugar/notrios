package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Notebook, tag, search-notebook, and trash operations for the SQLite store
// (schema v5, Notrios redesign task R3). Sibling notebook names and search
// notebook names are case-insensitively unique via NOCASE unique indexes;
// UNIQUE violations surface as ErrNameConflict.

func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (s *SQLiteStore) CreateNotebook(ctx context.Context, req CreateNotebookRequest) (Notebook, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Notebook{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ParentID = strings.TrimSpace(req.ParentID)
	req.PreferredID = strings.TrimSpace(req.PreferredID)
	if req.Name == "" {
		return Notebook{}, fmt.Errorf("%w: notebook name is required", ErrInvalidInput)
	}
	id := req.PreferredID
	if id == "" {
		var err error
		id, err = NewID("nb")
		if err != nil {
			return Notebook{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if req.ParentID != "" {
		if exists, err := s.notebookExistsLocked(req.ParentID); err != nil {
			return Notebook{}, err
		} else if !exists {
			return Notebook{}, fmt.Errorf("%w: parent notebook %q", ErrNotFound, req.ParentID)
		}
	}
	err := s.execPreparedLocked(`INSERT INTO notebooks(id, parent_id, name, icon_emoji, builtin, position)
		VALUES(?, NULLIF(?, ''), ?, ?, 0, ?)`, id, req.ParentID, req.Name, req.IconEmoji, itoa(req.Position))
	if isUniqueConstraintErr(err) {
		return Notebook{}, fmt.Errorf("%w: a sibling notebook is already named %q (names are case-insensitive)", ErrNameConflict, req.Name)
	}
	if err != nil {
		return Notebook{}, err
	}
	return s.getNotebookLocked(id)
}

func (s *SQLiteStore) GetNotebook(ctx context.Context, id string) (Notebook, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Notebook{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getNotebookLocked(id)
}

func (s *SQLiteStore) getNotebookLocked(id string) (Notebook, error) {
	stmt, err := s.prepareLocked(`SELECT id, COALESCE(parent_id, ''), name, icon_emoji, builtin, position, created_at, updated_at
		FROM notebooks WHERE id = ?`)
	if err != nil {
		return Notebook{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return Notebook{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return Notebook{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return Notebook{}, s.stepErrLocked(rc)
	}
	return notebookFromStmt(stmt), nil
}

func notebookFromStmt(stmt *C.sqlite3_stmt) Notebook {
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
	updatedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 7)))
	return Notebook{
		ID:        columnText(stmt, 0),
		ParentID:  columnText(stmt, 1),
		Name:      columnText(stmt, 2),
		IconEmoji: columnText(stmt, 3),
		Builtin:   columnInt64(stmt, 4) != 0,
		Position:  int(columnInt64(stmt, 5)),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func (s *SQLiteStore) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, COALESCE(parent_id, ''), name, icon_emoji, builtin, position, created_at, updated_at
		FROM notebooks ORDER BY COALESCE(parent_id, ''), position, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	notebooks := []Notebook{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			notebooks = append(notebooks, notebookFromStmt(stmt))
		case C.SQLITE_DONE:
			return notebooks, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) UpdateNotebook(ctx context.Context, req UpdateNotebookRequest) (Notebook, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Notebook{}, err
	}
	req.ID = strings.TrimSpace(req.ID)
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.getNotebookLocked(req.ID)
	if err != nil {
		return Notebook{}, err
	}
	if current.Builtin && (req.Name != nil || req.ParentID != nil) {
		return Notebook{}, fmt.Errorf("%w: builtin notebook %q cannot be renamed or moved", ErrProtected, current.Name)
	}

	name := current.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return Notebook{}, fmt.Errorf("%w: notebook name is required", ErrInvalidInput)
		}
	}
	parentID := current.ParentID
	if req.ParentID != nil {
		parentID = strings.TrimSpace(*req.ParentID)
		if parentID == req.ID {
			return Notebook{}, fmt.Errorf("%w: notebook cannot be its own parent", ErrInvalidInput)
		}
		if parentID != "" {
			if exists, err := s.notebookExistsLocked(parentID); err != nil {
				return Notebook{}, err
			} else if !exists {
				return Notebook{}, fmt.Errorf("%w: parent notebook %q", ErrNotFound, parentID)
			}
			cycle, err := s.notebookHasAncestorLocked(parentID, req.ID)
			if err != nil {
				return Notebook{}, err
			}
			if cycle {
				return Notebook{}, fmt.Errorf("%w: cannot move a notebook under its own descendant", ErrInvalidInput)
			}
		}
	}
	iconEmoji := current.IconEmoji
	if req.IconEmoji != nil {
		iconEmoji = strings.TrimSpace(*req.IconEmoji)
	}
	position := current.Position
	if req.Position != nil {
		position = *req.Position
	}

	err = s.execPreparedLocked(`UPDATE notebooks SET name = ?, parent_id = NULLIF(?, ''), icon_emoji = ?, position = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		name, parentID, iconEmoji, itoa(position), req.ID)
	if isUniqueConstraintErr(err) {
		return Notebook{}, fmt.Errorf("%w: a sibling notebook is already named %q (names are case-insensitive)", ErrNameConflict, name)
	}
	if err != nil {
		return Notebook{}, err
	}
	return s.getNotebookLocked(req.ID)
}

// notebookHasAncestorLocked reports whether ancestorID appears on the parent
// chain of notebookID (inclusive of notebookID itself).
func (s *SQLiteStore) notebookHasAncestorLocked(notebookID, ancestorID string) (bool, error) {
	seen := map[string]bool{}
	current := notebookID
	for current != "" && !seen[current] {
		if current == ancestorID {
			return true, nil
		}
		seen[current] = true
		nb, err := s.getNotebookLocked(current)
		if err != nil {
			return false, err
		}
		current = nb.ParentID
	}
	return false, nil
}

func (s *SQLiteStore) notebookExistsLocked(id string) (bool, error) {
	count, err := s.countLocked(`SELECT COUNT(1) FROM notebooks WHERE id = ?`, id)
	return count > 0, err
}

// DeleteNotebook deletes a notebook and its descendant notebooks. Their
// current notes move to the Trash (soft delete, no note data is lost) and are
// re-homed to the default notebook so restore-from-trash always has a valid
// destination. Builtin notebooks and the default notebook cannot be deleted.
func (s *SQLiteStore) DeleteNotebook(ctx context.Context, id string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()

	nb, err := s.getNotebookLocked(id)
	if err != nil {
		return err
	}
	if nb.Builtin {
		return fmt.Errorf("%w: builtin notebook %q cannot be deleted", ErrProtected, nb.Name)
	}
	if id == DefaultNotebookID {
		return fmt.Errorf("%w: the default notebook cannot be deleted", ErrProtected)
	}

	ids, err := s.notebookSubtreeIDsLocked(id)
	if err != nil {
		return err
	}
	in := "(" + placeholders(len(ids)) + ")"

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id IN (SELECT id FROM documents WHERE notebook_id IN `+in+` AND deleted_at IS NULL)`, ids...); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`UPDATE documents SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE notebook_id IN `+in+` AND deleted_at IS NULL`, ids...); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`UPDATE documents SET notebook_id = '`+DefaultNotebookID+`' WHERE notebook_id IN `+in, ids...); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`DELETE FROM notebooks WHERE id IN `+in, ids...); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// notebookSubtreeIDsLocked returns id plus all descendant notebook IDs.
func (s *SQLiteStore) notebookSubtreeIDsLocked(id string) ([]string, error) {
	ids := []string{id}
	frontier := []string{id}
	for len(frontier) > 0 {
		in := "(" + placeholders(len(frontier)) + ")"
		stmt, err := s.prepareLocked(`SELECT id FROM notebooks WHERE parent_id IN ` + in)
		if err != nil {
			return nil, err
		}
		if err := bindAll(stmt, frontier); err != nil {
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		next := []string{}
		for {
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				next = append(next, columnText(stmt, 0))
				continue
			}
			if rc == C.SQLITE_DONE {
				break
			}
			err := s.stepErrLocked(rc)
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		C.sqlite3_finalize(stmt)
		ids = append(ids, next...)
		frontier = next
	}
	return ids, nil
}

func (s *SQLiteStore) MoveDocumentToNotebook(ctx context.Context, documentID, notebookID string) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	documentID = strings.TrimSpace(documentID)
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		notebookID = DefaultNotebookID
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.getDocumentLocked(documentID)
	if err != nil {
		return Document{}, err
	}
	if doc.NotebookID == HelpNotebookID {
		return Document{}, fmt.Errorf("%w: Help notes cannot be moved", ErrProtected)
	}
	if notebookID == HelpNotebookID {
		return Document{}, fmt.Errorf("%w: notes cannot be moved into the Help notebook", ErrProtected)
	}
	if exists, err := s.notebookExistsLocked(notebookID); err != nil {
		return Document{}, err
	} else if !exists {
		return Document{}, fmt.Errorf("%w: notebook %q", ErrNotFound, notebookID)
	}
	if err := s.execPreparedLocked(`UPDATE documents SET notebook_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, notebookID, documentID); err != nil {
		return Document{}, err
	}
	return s.getDocumentLocked(documentID)
}

func (s *SQLiteStore) AddDocumentTag(ctx context.Context, documentID, tagName string) (Tag, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Tag{}, err
	}
	documentID = strings.TrimSpace(documentID)
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return Tag{}, fmt.Errorf("%w: tag name is required", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.getDocumentLocked(documentID); err != nil {
		return Tag{}, err
	}
	tag, found, err := s.findTagByNameLocked(tagName)
	if err != nil {
		return Tag{}, err
	}
	if !found {
		id, err := NewID("tag")
		if err != nil {
			return Tag{}, err
		}
		if err := s.execPreparedLocked(`INSERT INTO tags(id, name) VALUES(?, ?)`, id, tagName); err != nil {
			return Tag{}, err
		}
		tag = Tag{ID: id, Name: tagName}
	}
	if err := s.execPreparedLocked(`INSERT OR IGNORE INTO note_tags(document_id, tag_id) VALUES(?, ?)`, documentID, tag.ID); err != nil {
		return Tag{}, err
	}
	count, err := s.tagNoteCountLocked(tag.ID)
	if err != nil {
		return Tag{}, err
	}
	tag.NoteCount = count
	return tag, nil
}

func (s *SQLiteStore) RemoveDocumentTag(ctx context.Context, documentID, tagName string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	documentID = strings.TrimSpace(documentID)
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, found, err := s.findTagByNameLocked(strings.TrimSpace(tagName))
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if err := s.execPreparedLocked(`DELETE FROM note_tags WHERE document_id = ? AND tag_id = ?`, documentID, tag.ID); err != nil {
		return err
	}
	// Unreferenced tags disappear from the sidebar entirely.
	return s.execPreparedLocked(`DELETE FROM tags WHERE id = ? AND NOT EXISTS (SELECT 1 FROM note_tags WHERE tag_id = ?)`, tag.ID, tag.ID)
}

func (s *SQLiteStore) findTagByNameLocked(name string) (Tag, bool, error) {
	stmt, err := s.prepareLocked(`SELECT id, name FROM tags WHERE name = ? COLLATE NOCASE`)
	if err != nil {
		return Tag{}, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{name}); err != nil {
		return Tag{}, false, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return Tag{}, false, nil
	}
	if rc != C.SQLITE_ROW {
		return Tag{}, false, s.stepErrLocked(rc)
	}
	return Tag{ID: columnText(stmt, 0), Name: columnText(stmt, 1)}, true, nil
}

// tagNoteCountLocked counts current non-deleted notes carrying the tag.
func (s *SQLiteStore) tagNoteCountLocked(tagID string) (int64, error) {
	return s.countLocked(`SELECT COUNT(1) FROM note_tags nt JOIN documents d ON d.id = nt.document_id WHERE nt.tag_id = ? AND d.deleted_at IS NULL`, tagID)
}

func (s *SQLiteStore) ListDocumentTags(ctx context.Context, documentID string) ([]Tag, error) {
	return s.listTags(ctx, `SELECT t.id, t.name, (SELECT COUNT(1) FROM note_tags nt2 JOIN documents d2 ON d2.id = nt2.document_id WHERE nt2.tag_id = t.id AND d2.deleted_at IS NULL)
		FROM tags t JOIN note_tags nt ON nt.tag_id = t.id
		WHERE nt.document_id = ?
		ORDER BY t.name COLLATE NOCASE`, strings.TrimSpace(documentID))
}

func (s *SQLiteStore) ListTags(ctx context.Context) ([]Tag, error) {
	return s.listTags(ctx, `SELECT t.id, t.name, (SELECT COUNT(1) FROM note_tags nt JOIN documents d ON d.id = nt.document_id WHERE nt.tag_id = t.id AND d.deleted_at IS NULL)
		FROM tags t
		ORDER BY t.name COLLATE NOCASE`)
}

func (s *SQLiteStore) listTags(ctx context.Context, query string, values ...string) ([]Tag, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	tags := []Tag{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			tags = append(tags, Tag{
				ID:        columnText(stmt, 0),
				Name:      columnText(stmt, 1),
				NoteCount: columnInt64(stmt, 2),
			})
		case C.SQLITE_DONE:
			return tags, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) CreateSearchNotebook(ctx context.Context, req CreateSearchNotebookRequest) (SearchNotebook, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SearchNotebook{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Query = strings.TrimSpace(req.Query)
	if req.Name == "" {
		return SearchNotebook{}, fmt.Errorf("%w: search notebook name is required", ErrInvalidInput)
	}
	id := strings.TrimSpace(req.PreferredID)
	if id == "" {
		var err error
		id, err = NewID("snb")
		if err != nil {
			return SearchNotebook{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.execPreparedLocked(`INSERT INTO search_notebooks(id, name, icon_emoji, query, builtin, sort_anchor) VALUES(?, ?, ?, ?, 0, 'normal')`,
		id, req.Name, req.IconEmoji, req.Query)
	if isUniqueConstraintErr(err) {
		return SearchNotebook{}, fmt.Errorf("%w: a search notebook is already named %q (names are case-insensitive)", ErrNameConflict, req.Name)
	}
	if err != nil {
		return SearchNotebook{}, err
	}
	return s.getSearchNotebookLocked(id)
}

func (s *SQLiteStore) getSearchNotebookLocked(id string) (SearchNotebook, error) {
	stmt, err := s.prepareLocked(`SELECT id, name, icon_emoji, query, builtin, sort_anchor, created_at FROM search_notebooks WHERE id = ?`)
	if err != nil {
		return SearchNotebook{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return SearchNotebook{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return SearchNotebook{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return SearchNotebook{}, s.stepErrLocked(rc)
	}
	return searchNotebookFromStmt(stmt), nil
}

func searchNotebookFromStmt(stmt *C.sqlite3_stmt) SearchNotebook {
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
	return SearchNotebook{
		ID:         columnText(stmt, 0),
		Name:       columnText(stmt, 1),
		IconEmoji:  columnText(stmt, 2),
		Query:      columnText(stmt, 3),
		Builtin:    columnInt64(stmt, 4) != 0,
		SortAnchor: columnText(stmt, 5),
		CreatedAt:  createdAt,
	}
}

// ListSearchNotebooks returns search notebooks in sidebar order: "All notes"
// first, user search notebooks by name, and "Trash" last.
func (s *SQLiteStore) ListSearchNotebooks(ctx context.Context) ([]SearchNotebook, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, name, icon_emoji, query, builtin, sort_anchor, created_at
		FROM search_notebooks
		ORDER BY CASE sort_anchor WHEN 'first' THEN 0 WHEN 'last' THEN 2 ELSE 1 END, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	notebooks := []SearchNotebook{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			notebooks = append(notebooks, searchNotebookFromStmt(stmt))
		case C.SQLITE_DONE:
			return notebooks, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// DeleteSearchNotebook removes a user search notebook (its saved query only —
// never any notes). Builtin search notebooks cannot be deleted.
func (s *SQLiteStore) DeleteSearchNotebook(ctx context.Context, id string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	nb, err := s.getSearchNotebookLocked(strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if nb.Builtin {
		return fmt.Errorf("%w: builtin search notebook %q cannot be deleted", ErrProtected, nb.Name)
	}
	return s.execPreparedLocked(`DELETE FROM search_notebooks WHERE id = ?`, nb.ID)
}

// ListTrash returns soft-deleted documents, newest deletions first. Trashed
// documents are excluded from every other query surface.
func (s *SQLiteStore) ListTrash(ctx context.Context, limit int) ([]Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT d.id, d.collection_id, d.title, r.body, COALESCE(r.body_mime_type, d.body_mime_type), d.current_revision_id, d.created_at, d.updated_at, COALESCE(d.deleted_at, ''), COALESCE(d.notebook_id, '')
		FROM documents d
		JOIN document_revisions r ON r.id = d.current_revision_id
		WHERE d.deleted_at IS NOT NULL
		ORDER BY d.deleted_at DESC, d.id
		LIMIT ` + itoa(limit))
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	docs := []Document{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
			updatedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 7)))
			deletedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 8)))
			doc := Document{
				ID:                columnText(stmt, 0),
				CollectionID:      columnText(stmt, 1),
				Title:             columnText(stmt, 2),
				Body:              columnText(stmt, 3),
				BodyMIMEType:      columnText(stmt, 4),
				CurrentRevisionID: columnText(stmt, 5),
				CreatedAt:         createdAt,
				UpdatedAt:         updatedAt,
				DeletedAt:         deletedAt,
				NotebookID:        columnText(stmt, 9),
			}
			doc.URI = DocumentURI(doc.CollectionID, doc.ID)
			docs = append(docs, doc)
		case C.SQLITE_DONE:
			return docs, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// RestoreDocument undeletes a trashed document: it becomes visible in queries
// and search again, in the notebook it is currently assigned to.
func (s *SQLiteStore) RestoreDocument(ctx context.Context, id string) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()

	title, body, collectionID, err := s.trashedDocumentStateLocked(id)
	if err != nil {
		return Document{}, err
	}

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return Document{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`UPDATE documents SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id = ?`, id); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`, id, collectionID, title, body); err != nil {
		return Document{}, err
	}
	if err := s.rebuildDocumentLinksLocked(id, collectionID, body); err != nil {
		return Document{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return Document{}, err
	}
	committed = true
	return s.getDocumentLocked(id)
}

// PurgeDocument permanently deletes a trashed document and its revisions.
// Only notes stored purely in the local database may be purged;
// externally-sourced notes (document_sources row) are refused.
func (s *SQLiteStore) PurgeDocument(ctx context.Context, id string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, _, _, err := s.trashedDocumentStateLocked(id); err != nil {
		return err
	}
	// Only notes stored purely in the local database may be purged;
	// externally-sourced notes stay excluded from queries instead.
	if sourced, err := s.documentHasSourceLocked(id); err != nil {
		return err
	} else if sourced {
		return fmt.Errorf("%w: externally-sourced notes cannot be permanently deleted", ErrProtected)
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
	statements := []string{
		`DELETE FROM note_tags WHERE document_id = ?`,
		`DELETE FROM document_resource_refs WHERE document_id = ?`,
		`DELETE FROM document_links WHERE source_document_id = ?`,
		`UPDATE document_links SET target_document_id = NULL, resolution_status = 'target_deleted' WHERE target_document_id = ?`,
		`DELETE FROM documents_fts WHERE document_id = ?`,
		`DELETE FROM document_revisions WHERE document_id = ?`,
	}
	for _, statement := range statements {
		if err := s.execPreparedLocked(statement, id); err != nil {
			return err
		}
	}
	// Break the FK from documents.current_revision_id before deleting the row.
	if err := s.execPreparedLocked(`UPDATE documents SET current_revision_id = NULL WHERE id = ?`, id); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`DELETE FROM documents WHERE id = ?`, id); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	// Orphaned tags vanish from the sidebar.
	return s.execLocked(`DELETE FROM tags WHERE NOT EXISTS (SELECT 1 FROM note_tags WHERE tag_id = tags.id)`)
}

// trashedDocumentStateLocked returns current title/body/collection for a
// document that must be in the trash (ErrNotFound covers both missing and
// not-deleted documents).
func (s *SQLiteStore) trashedDocumentStateLocked(id string) (title, body, collectionID string, err error) {
	stmt, err := s.prepareLocked(`SELECT d.title, r.body, d.collection_id
		FROM documents d
		JOIN document_revisions r ON r.id = d.current_revision_id
		WHERE d.id = ? AND d.deleted_at IS NOT NULL`)
	if err != nil {
		return "", "", "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return "", "", "", err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return "", "", "", fmt.Errorf("%w: document %q is not in the trash", ErrNotFound, id)
	}
	if rc != C.SQLITE_ROW {
		return "", "", "", s.stepErrLocked(rc)
	}
	return columnText(stmt, 0), columnText(stmt, 1), columnText(stmt, 2), nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
