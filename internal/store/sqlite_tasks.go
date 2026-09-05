package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/renesugar/notrios/internal/markdownblocks"
)

// checkboxRE matches a task once the list marker has been stripped by the block
// parser, so `- [ ] buy milk` arrives here as `[ ] buy milk`.
var checkboxRE = regexp.MustCompile(`^\[([ xX])\][ \t]+(.*)$`)

// ListTasks extracts checkbox items from note bodies.
//
// Computed on read, deliberately. `document_blocks` stores a hash and byte
// offsets but **not** block text, so there is nothing to query for "is this
// item checked" — the body has to be read either way. That makes extraction a
// whole-library scan of the same shape and cost as lint, and the decision this
// slice was given says no table until a query needs one a scan cannot serve.
//
// Restricting to one note or one notebook turns that scan into a small one,
// which is the usual case for a client showing a note's own tasks.
func (s *SQLiteStore) ListTasks(ctx context.Context, req TaskListRequest) (TaskList, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return TaskList{}, err
	}
	switch strings.TrimSpace(strings.ToLower(req.State)) {
	case "", TaskStateOpen, TaskStateDone:
	default:
		return TaskList{}, fmt.Errorf("%w: state must be %q, %q, or empty", ErrInvalidInput, TaskStateOpen, TaskStateDone)
	}
	wantState := strings.TrimSpace(strings.ToLower(req.State))
	if req.Limit <= 0 {
		req.Limit = DefaultTaskRows
	}
	if req.Limit > MaxTaskRows {
		req.Limit = MaxTaskRows
	}
	// Empty means every collection; see CollectionScopeSQL.
	collectionID := strings.TrimSpace(req.CollectionID)

	// A checkbox is `[` plus a space or an x plus `]`, so a LIKE prefilter
	// removes the notes that cannot possibly contain one before any body is
	// parsed. It is a filter, never the answer: the parser still decides.
	docs, err := s.taskCandidateDocuments(collectionID, strings.TrimSpace(req.DocumentID),
		strings.TrimSpace(req.NotebookID), req.Untagged)
	if err != nil {
		return TaskList{}, err
	}
	result := TaskList{Tasks: []Task{}}
	if len(docs) > MaxTaskDocuments {
		docs = docs[:MaxTaskDocuments]
		result.DocumentsTruncated = true
	}
	result.DocumentsScanned = len(docs)

	for _, doc := range docs {
		for _, block := range markdownblocks.Extract(doc.ID, doc.Body) {
			if block.Kind != markdownblocks.KindListItem {
				continue
			}
			match := checkboxRE.FindStringSubmatch(block.Text)
			if match == nil {
				continue
			}
			state := TaskStateOpen
			if match[1] != " " {
				state = TaskStateDone
			}
			if state == TaskStateOpen {
				result.OpenCount++
			} else {
				result.DoneCount++
			}
			if wantState != "" && state != wantState {
				continue
			}
			if len(result.Tasks) >= req.Limit {
				result.Truncated = true
				continue // keep counting: the counts describe everything found
			}
			// Extract already computed the content-derived ID, so this is the
			// same identity `document_blocks` stores and a `#^` link resolves.
			anchor := "^" + block.ID
			if block.Marker != "" {
				anchor = "^" + block.Marker
			}
			result.Tasks = append(result.Tasks, Task{
				DocumentID:    doc.ID,
				DocumentTitle: doc.Title,
				NotebookID:    doc.NotebookID,
				BlockID:       block.ID,
				Marker:        block.Marker,
				Ordinal:       block.Ordinal,
				State:         state,
				Text:          strings.TrimSpace(match[2]),
				URI:           DocumentURI(doc.CollectionID, doc.ID) + "#" + anchor,
			})
		}
	}
	return result, nil
}

// taskCandidateDocuments returns the notes that might contain a checkbox.
func (s *SQLiteStore) taskCandidateDocuments(collectionID, documentID, notebookID string, untagged bool) ([]Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	where := CollectionScopeSQL("d") + ` AND d.deleted_at IS NULL`
	args := []string{collectionID}
	if documentID != "" {
		where += ` AND d.id = ?`
		args = append(args, documentID)
	}
	if notebookID != "" {
		where += ` AND d.notebook_id = ?`
		args = append(args, notebookID)
	}
	// Cheap prefilter. `[` is not a full-text token, so FTS cannot help here.
	where += ` AND (r.body LIKE '%[ ]%' OR r.body LIKE '%[x]%' OR r.body LIKE '%[X]%')`
	if !untagged {
		// Only notes their author marked as carrying tasks. This is a
		// correctness rule first -- a checkbox in a quoted example is not work
		// somebody owes -- and it bounds the scan as a consequence, since the
		// tagged set is a small fraction of a library and note_tags_tag_idx
		// covers the lookup.
		//
		// The hierarchy is included: `todo/survey` is a task tag, because a tag
		// tree that stops meaning what its root means would be a surprise
		// nowhere else in this library.
		where += ` AND EXISTS (SELECT 1 FROM note_tags nt JOIN tags t ON t.id = nt.tag_id
			WHERE nt.document_id = d.id AND (` + taskTagPredicate() + `))`
		for _, tag := range TaskTags {
			args = append(args, tag, tag+TagHierarchySeparator+"%")
		}
	}
	return s.documentsWhereLocked(where, args)
}

// documentsMatchingBodyLocked finds notes whose body matches a LIKE pattern.
func (s *SQLiteStore) documentsMatchingBodyLocked(collectionID, pattern string) ([]Document, error) {
	return s.documentsWhereLocked(
		CollectionScopeSQL("d")+` AND d.deleted_at IS NULL AND r.body LIKE ? ESCAPE '\'`,
		[]string{collectionID, pattern})
}

// documentsWhereLocked reads current documents with their bodies.
func (s *SQLiteStore) documentsWhereLocked(where string, args []string) ([]Document, error) {
	stmt, err := s.prepareLocked(`SELECT d.id, d.collection_id, COALESCE(d.notebook_id, ''), d.title, r.body,
			COALESCE(r.body_mime_type, d.body_mime_type), d.current_revision_id
		FROM documents d
		JOIN document_revisions r ON r.id = d.current_revision_id
		WHERE ` + where + `
		ORDER BY d.id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return nil, err
	}
	docs := []Document{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			doc := Document{
				ID:                columnText(stmt, 0),
				CollectionID:      columnText(stmt, 1),
				NotebookID:        columnText(stmt, 2),
				Title:             columnText(stmt, 3),
				Body:              columnText(stmt, 4),
				BodyMIMEType:      columnText(stmt, 5),
				CurrentRevisionID: columnText(stmt, 6),
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

// taskTagPredicate matches a task tag or anything beneath it, case-insensitively.
func taskTagPredicate() string {
	clauses := make([]string, 0, len(TaskTags))
	for range TaskTags {
		clauses = append(clauses, "t.name = ? COLLATE NOCASE OR t.name LIKE ? ESCAPE '\\'")
	}
	return strings.Join(clauses, " OR ")
}
