package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
)

// RenameTag renames a tag and, optionally, everything under it.
//
// The dry run and the apply are the same code. The run happens inside one
// transaction and a dry run rolls it back, so the report is produced by the
// statements that would do the work rather than by a second implementation
// predicting them. A rename with cascading merges is exactly where a predictor
// and an applier drift apart, and this is the case where being wrong is
// expensive: a tag that disappeared into the wrong parent is tedious to undo
// by hand.
//
// Tags in Notrios are relational, not text in note bodies, so a rename never
// rewrites a note. A `#project` that happens to appear in someone's prose is
// prose, and this does not touch it.
func (s *SQLiteStore) RenameTag(ctx context.Context, req TagRenameRequest) (TagRenameResult, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return TagRenameResult{}, err
	}
	from := strings.TrimSpace(req.From)
	to := strings.TrimSpace(req.To)
	if from == "" || to == "" {
		return TagRenameResult{}, fmt.Errorf("%w: both from and to tag names are required", ErrInvalidInput)
	}
	if err := validateTagName(to); err != nil {
		return TagRenameResult{}, err
	}
	// Renaming a tag to somewhere inside its own subtree would make a tag its
	// own ancestor and turn the rename into a reshuffle whose result depends on
	// the order the tags happen to come out in. Refusing keeps the mapping from
	// old name to new name injective and order-independent.
	if isTagDescendant(to, from) && !equalFoldASCII(to, from) {
		return TagRenameResult{}, fmt.Errorf("%w: cannot rename %q into its own subtree (%q)", ErrInvalidInput, from, to)
	}

	result := TagRenameResult{From: from, To: to, IncludeChildren: req.IncludeChildren, DryRun: req.DryRun, Changes: []TagRenameChange{}, Warnings: []string{}}

	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.tagRowsLocked()
	if err != nil {
		return TagRenameResult{}, err
	}
	sources := []tagRow{}
	children := 0
	for _, tag := range all {
		depth, ok := tagPrefixDepth(tag.name, from)
		if !ok {
			continue
		}
		if depth > 0 {
			children++
			if !req.IncludeChildren {
				continue
			}
		}
		sources = append(sources, tag)
	}
	if len(sources) == 0 {
		return TagRenameResult{}, fmt.Errorf("%w: no tag named %q", ErrNotFound, from)
	}
	if len(sources) > MaxTagRenameTags {
		return TagRenameResult{}, fmt.Errorf("%w: rename would touch %d tags, over the %d ceiling", ErrInvalidInput, len(sources), MaxTagRenameTags)
	}
	// Shallowest first. When the destination sits above the source — renaming
	// `a/x` to `a` — a deeper tag's new name may be a shallower tag's old one,
	// and doing the shallow one first frees the name before it is needed.
	sortTagRowsByName(sources)

	if children > 0 && !req.IncludeChildren {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d child tag(s) keep the %q prefix; pass include_children to rename them too", children, from+TagHierarchySeparator))
	}
	saved, err := s.searchNotebooksMentioningLocked(from)
	if err != nil {
		return TagRenameResult{}, err
	}
	if len(saved) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d saved search(es) mention %q and are not rewritten: %s", len(saved), from, strings.Join(saved, ", ")))
	}

	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return TagRenameResult{}, err
	}
	// "settled" rather than "committed": a dry run ends this transaction with a
	// ROLLBACK of its own, and the deferred one must not fire after it.
	settled := false
	defer func() {
		if !settled {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	touched := map[string]bool{}
	for _, source := range sources {
		dest := renamedTagName(source.name, from, to)
		change := TagRenameChange{TagID: source.id, From: source.name, To: dest, Action: TagRenameActionRename}

		notes, err := s.tagNoteDocumentIDsLocked(source.id)
		if err != nil {
			return TagRenameResult{}, err
		}
		change.Notes = int64(len(notes))
		for _, id := range notes {
			touched[id] = true
		}
		// The projection carries a note's tags, so a rename changes what the
		// derived index should say about every note holding the tag. Nothing
		// else in the transaction would enqueue that.
		for _, id := range notes {
			if err := s.enqueueProjectionLocked(id, "upsert"); err != nil {
				return TagRenameResult{}, err
			}
		}

		existing, found, err := s.findTagByNameLocked(dest)
		if err != nil {
			return TagRenameResult{}, err
		}
		if found && existing.ID != source.id {
			gained, err := s.countLocked(`SELECT COUNT(1) FROM note_tags nt JOIN documents d ON d.id = nt.document_id
				WHERE nt.tag_id = ? AND d.deleted_at IS NULL
				  AND NOT EXISTS (SELECT 1 FROM note_tags x WHERE x.document_id = nt.document_id AND x.tag_id = ?)`, source.id, existing.ID)
			if err != nil {
				return TagRenameResult{}, err
			}
			// UPDATE OR IGNORE moves the rows whose (document, destination)
			// pair is free; the DELETE clears the ones it skipped, which are
			// exactly the notes that already carried both tags.
			if err := s.execPreparedLocked(`UPDATE OR IGNORE note_tags SET tag_id = ? WHERE tag_id = ?`, existing.ID, source.id); err != nil {
				return TagRenameResult{}, err
			}
			if err := s.execPreparedLocked(`DELETE FROM note_tags WHERE tag_id = ?`, source.id); err != nil {
				return TagRenameResult{}, err
			}
			if err := s.execPreparedLocked(`DELETE FROM tags WHERE id = ?`, source.id); err != nil {
				return TagRenameResult{}, err
			}
			change.Action = TagRenameActionMerge
			change.MergedIntoTagID = existing.ID
			change.NotesGained = gained
		} else {
			if err := s.execPreparedLocked(`UPDATE tags SET name = ? WHERE id = ?`, dest, source.id); err != nil {
				if isUniqueConstraintErr(err) {
					return TagRenameResult{}, fmt.Errorf("%w: tag name %q is already in use", ErrNameConflict, dest)
				}
				return TagRenameResult{}, err
			}
			change.NotesGained = change.Notes
		}
		result.Changes = append(result.Changes, change)
	}
	result.Notes = int64(len(touched))

	if req.DryRun {
		// The rollback is the dry run. Everything above ran for real against
		// the real rows, which is why the report cannot disagree with an apply.
		if err := s.execLocked("ROLLBACK"); err != nil {
			return TagRenameResult{}, err
		}
		settled = true
		return result, nil
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return TagRenameResult{}, err
	}
	settled = true
	return result, nil
}

// PreviewNotebookDeletion reports what deleting a notebook would do.
//
// It is read-only and says the thing a confirmation dialog cannot infer: the
// notes are not deleted, they move to the Trash, and they are re-homed to the
// default notebook so restoring one later has somewhere to put it.
func (s *SQLiteStore) PreviewNotebookDeletion(ctx context.Context, id string) (NotebookDeletionPreview, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return NotebookDeletionPreview{}, err
	}
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()

	nb, err := s.getNotebookLocked(id)
	if err != nil {
		return NotebookDeletionPreview{}, err
	}
	ids, err := s.notebookSubtreeIDsLocked(id)
	if err != nil {
		return NotebookDeletionPreview{}, err
	}
	preview := NotebookDeletionPreview{
		NotebookID:       nb.ID,
		Name:             nb.Name,
		Notebooks:        int64(len(ids)),
		DescendantNames:  []string{},
		RehomeNotebookID: DefaultNotebookID,
		Deletable:        true,
	}
	switch {
	case nb.Builtin:
		preview.Deletable = false
		preview.Reason = "builtin notebooks cannot be deleted"
	case id == DefaultNotebookID:
		preview.Deletable = false
		preview.Reason = "the default notebook cannot be deleted"
	}

	for _, descendantID := range ids[1:] {
		if len(preview.DescendantNames) >= MaxNotebookPreviewNames {
			preview.Truncated = true
			break
		}
		descendant, err := s.getNotebookLocked(descendantID)
		if err != nil {
			return NotebookDeletionPreview{}, err
		}
		preview.DescendantNames = append(preview.DescendantNames, descendant.Name)
	}

	in := "(" + placeholders(len(ids)) + ")"
	if preview.Notes, err = s.countLocked(`SELECT COUNT(1) FROM documents WHERE notebook_id IN `+in+` AND deleted_at IS NULL`, ids...); err != nil {
		return NotebookDeletionPreview{}, err
	}
	if preview.TrashedNotes, err = s.countLocked(`SELECT COUNT(1) FROM documents WHERE notebook_id IN `+in+` AND deleted_at IS NOT NULL`, ids...); err != nil {
		return NotebookDeletionPreview{}, err
	}
	return preview, nil
}

// ---- tag-name helpers -------------------------------------------------------

// equalFoldASCII matches SQLite's NOCASE collation, which folds only A-Z. Using
// Go's Unicode-aware fold here would make two tags the database considers
// distinct look like one, which is how a rename would touch a tag nobody asked
// it to.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// tagPrefixDepth reports whether name is prefix or one of its descendants, and
// how many segments below prefix it sits. Comparison is segment-by-segment, so
// `projects` is not a child of `project` and a prefix never matches mid-segment.
func tagPrefixDepth(name, prefix string) (int, bool) {
	nameSegments := strings.Split(name, TagHierarchySeparator)
	prefixSegments := strings.Split(prefix, TagHierarchySeparator)
	if len(nameSegments) < len(prefixSegments) {
		return 0, false
	}
	for i, segment := range prefixSegments {
		if !equalFoldASCII(nameSegments[i], segment) {
			return 0, false
		}
	}
	return len(nameSegments) - len(prefixSegments), true
}

// isTagDescendant reports whether name is at or below prefix.
func isTagDescendant(name, prefix string) bool {
	_, ok := tagPrefixDepth(name, prefix)
	return ok
}

// renamedTagName swaps the `from` prefix for `to`, keeping the segments below.
func renamedTagName(name, from, to string) string {
	nameSegments := strings.Split(name, TagHierarchySeparator)
	prefixLen := len(strings.Split(from, TagHierarchySeparator))
	rest := nameSegments[prefixLen:]
	if len(rest) == 0 {
		return to
	}
	return to + TagHierarchySeparator + strings.Join(rest, TagHierarchySeparator)
}

// validateTagName rejects destination names that could never be addressed as a
// hierarchy: an empty segment has no name to type, and a leading or trailing
// separator produces one.
func validateTagName(name string) error {
	for _, segment := range strings.Split(name, TagHierarchySeparator) {
		if strings.TrimSpace(segment) == "" {
			return fmt.Errorf("%w: tag name %q has an empty path segment", ErrInvalidInput, name)
		}
	}
	return nil
}

type tagRow struct {
	id   string
	name string
}

// sortTagRowsByName orders rows by lowercased name. Insertion sort: the list is
// bounded by MaxTagRenameTags and this keeps the comparison in one place.
func sortTagRowsByName(rows []tagRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && strings.ToLower(rows[j].name) < strings.ToLower(rows[j-1].name); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func (s *SQLiteStore) tagRowsLocked() ([]tagRow, error) {
	stmt, err := s.prepareLocked(`SELECT id, name FROM tags`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	rows := []tagRow{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			rows = append(rows, tagRow{id: columnText(stmt, 0), name: columnText(stmt, 1)})
		case C.SQLITE_DONE:
			return rows, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// tagNoteDocumentIDsLocked lists the current (non-trashed) notes carrying a tag.
func (s *SQLiteStore) tagNoteDocumentIDsLocked(tagID string) ([]string, error) {
	stmt, err := s.prepareLocked(`SELECT nt.document_id FROM note_tags nt JOIN documents d ON d.id = nt.document_id
		WHERE nt.tag_id = ? AND d.deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{tagID}); err != nil {
		return nil, err
	}
	ids := []string{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			ids = append(ids, columnText(stmt, 0))
		case C.SQLITE_DONE:
			return ids, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// searchNotebooksMentioningLocked names the saved searches whose query text
// contains the tag name. A rename does not rewrite them: a saved search is text
// the user wrote, and guessing which occurrences of a word are the tag is the
// kind of guess that silently changes what a search means.
func (s *SQLiteStore) searchNotebooksMentioningLocked(tagName string) ([]string, error) {
	stmt, err := s.prepareLocked(`SELECT name FROM search_notebooks WHERE query LIKE ? ESCAPE '\' ORDER BY name COLLATE NOCASE LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{"%" + escapeLikePrefix(tagName) + "%"}); err != nil {
		return nil, err
	}
	names := []string{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			names = append(names, columnText(stmt, 0))
		case C.SQLITE_DONE:
			return names, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}
