package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"errors"
	"strings"

	"github.com/renesugar/notrios/internal/markdownblocks"
	"github.com/renesugar/notrios/internal/markdownlinks"
)

// SuggestDocuments answers what a partial link target could name.
//
// Two passes, in this order, because they cost different things:
//
//  1. A **title prefix** range scan over the schema-v16
//     `(collection_id, deleted_at, title COLLATE NOCASE)` index. SQLite turns
//     `title LIKE 'kit%'` into a range on that index — the NOCASE collation is
//     the whole reason it can — so the scan starts at the first match, walks in
//     title order, and stops one row past the limit. Its cost is the page, not
//     the library, which is what makes this safe to call on a keystroke.
//  2. If that did not fill the page, a bounded **interior word** pass over
//     FTS5's already-indexed `title` column, so typing "plan" also finds
//     "Kitchen Plan". This one is capped at maxSuggestionWordCandidates
//     because an FTS5 prefix term can match a large part of a library, and
//     ranking all of it is exactly the whole-library query a keystroke must not
//     start.
//
// The cap on the second pass has an honest consequence worth stating: for a
// query matching more notes than the candidate budget, the interior matches
// shown are the first the index yields, not the best. The first pass has no
// such limitation, which is why it runs first and why its results rank above.
func (s *SQLiteStore) SuggestDocuments(ctx context.Context, req DocumentSuggestionRequest) (DocumentSuggestionResponse, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DocumentSuggestionResponse{}, err
	}
	if err := req.validate(); err != nil {
		return DocumentSuggestionResponse{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	resp := DocumentSuggestionResponse{Suggestions: []DocumentSuggestion{}, Limit: req.Limit}
	seen := map[string]bool{}
	if req.ExcludeDocumentID != "" {
		seen[req.ExcludeDocumentID] = true
	}

	prefix, err := s.suggestByTitlePrefixLocked(req, seen)
	if err != nil {
		return DocumentSuggestionResponse{}, err
	}
	// One row beyond the limit is read so "there are more" is a fact rather
	// than a guess.
	if len(prefix) > req.Limit {
		resp.Truncated = true
		prefix = prefix[:req.Limit]
	}
	resp.Suggestions = append(resp.Suggestions, prefix...)

	if len(resp.Suggestions) < req.Limit {
		words, more, err := s.suggestByTitleWordLocked(req, seen, req.Limit-len(resp.Suggestions))
		if err != nil {
			return DocumentSuggestionResponse{}, err
		}
		resp.Suggestions = append(resp.Suggestions, words...)
		if more {
			resp.Truncated = true
		}
	}
	return resp, nil
}

func (s *SQLiteStore) suggestByTitlePrefixLocked(req DocumentSuggestionRequest, seen map[string]bool) ([]DocumentSuggestion, error) {
	stmt, err := s.prepareLocked(`SELECT id, title, COALESCE(notebook_id, '')
		FROM documents
		WHERE ` + CollectionScopeSQL("") + ` AND deleted_at IS NULL AND title LIKE ? ESCAPE '\'
		ORDER BY title COLLATE NOCASE, id
		LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	// The limit is one higher than asked for, plus one for a possible excluded
	// self-match, so a full page can still be reported as truncated correctly.
	limit := req.Limit + 2
	if err := bindAll(stmt, []string{req.CollectionID, escapeLikePrefix(req.Query) + "%", itoaInt(limit)}); err != nil {
		return nil, err
	}
	out := []DocumentSuggestion{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return out, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		id := columnText(stmt, 0)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, DocumentSuggestion{
			DocumentID: id,
			Title:      columnText(stmt, 1),
			URI:        DocumentURI(req.CollectionID, id),
			NotebookID: columnText(stmt, 2),
			Match:      SuggestionMatchTitlePrefix,
		})
	}
}

// suggestByTitleWordLocked finds titles where a later word starts with the
// query. It reads at most maxSuggestionWordCandidates rows in index order and
// stops; see SuggestDocuments for what that bound gives up.
func (s *SQLiteStore) suggestByTitleWordLocked(req DocumentSuggestionRequest, seen map[string]bool, want int) ([]DocumentSuggestion, bool, error) {
	term := ftsPrefixTerm(req.Query)
	if term == "" {
		return nil, false, nil
	}
	stmt, err := s.prepareLocked(`SELECT f.document_id, d.title, COALESCE(d.notebook_id, '')
		FROM documents_fts f
		JOIN documents d ON d.id = f.document_id AND d.deleted_at IS NULL
		WHERE documents_fts MATCH ? AND ` + CollectionScopeSQL("f") + `
		LIMIT ?`)
	if err != nil {
		return nil, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{"title:" + term, req.CollectionID, itoaInt(maxSuggestionWordCandidates)}); err != nil {
		return nil, false, err
	}
	out := []DocumentSuggestion{}
	more := false
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return out, more, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, false, s.stepErrLocked(rc)
		}
		id := columnText(stmt, 0)
		if seen[id] {
			continue
		}
		if len(out) >= want {
			more = true
			continue
		}
		seen[id] = true
		out = append(out, DocumentSuggestion{
			DocumentID: id,
			Title:      columnText(stmt, 1),
			URI:        DocumentURI(req.CollectionID, id),
			NotebookID: columnText(stmt, 2),
			Match:      SuggestionMatchWordPrefix,
		})
	}
}

// CheckLinks resolves every link in an unsaved buffer without saving it.
//
// It runs the canonical extractor and the canonical resolver — the same two the
// save path uses — so a marker an editor draws matches the link record the save
// would write. Nothing is stored: the body is parsed, classified, and dropped.
//
// Anchors into the note being edited are resolved against **the submitted
// body**, not the saved block rows. While someone is typing, the buffer is the
// truth about its own headings, and checking a just-typed `#new-section` against
// yesterday's saved blocks would mark a correct link broken.
func (s *SQLiteStore) CheckLinks(ctx context.Context, req CheckLinksRequest) (CheckLinksResponse, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return CheckLinksResponse{}, err
	}
	if err := req.validate(); err != nil {
		return CheckLinksResponse{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	resp := CheckLinksResponse{Links: []CheckedLink{}}
	// A self-anchor target: the anchors this buffer currently offers.
	selfAnchors := bufferAnchors(req.DocumentID, req.Body)

	for _, candidate := range markdownlinks.Extract(req.Body) {
		if err := ctx.Err(); err != nil {
			return CheckLinksResponse{}, err
		}
		link := s.resolveLinkCandidateLocked(req.DocumentID, req.CollectionID, candidate)
		checked := CheckedLink{
			RawTarget:        candidate.RawTarget,
			DisplayText:      candidate.DisplayText,
			RelationType:     candidate.RelationType,
			SourceFormat:     candidate.SourceFormat,
			AnchorType:       candidate.AnchorType,
			AnchorValue:      candidate.AnchorValue,
			Status:           link.ResolutionStatus,
			TargetDocumentID: link.TargetDocumentID,
			TargetResourceID: link.TargetResourceID,
			TargetURI:        link.TargetURI,
			StartByte:        candidate.StartByte,
			EndByte:          candidate.EndByte,
			Line:             candidate.Line,
			Column:           candidate.Column,
		}
		// A link that resolved by title or filename already knows the URI it
		// points at. Offering it is the same repair `notriosctl fix` applies,
		// made available before the note is even saved.
		if link.ResolutionStatus == "resolved" && !strings.EqualFold(strings.TrimSpace(candidate.RawTarget), link.TargetURI) {
			if link.TargetDocumentID != "" || link.TargetResourceID != "" {
				checked.CanonicalTarget = link.TargetURI
			}
		}
		if checked.Status == "resolved" && checked.AnchorValue != "" && checked.TargetDocumentID != "" {
			resolved, err := s.checkAnchorLocked(checked.TargetDocumentID, checked.AnchorValue, req.DocumentID, selfAnchors)
			if err != nil {
				return CheckLinksResponse{}, err
			}
			if !resolved {
				checked.Status = "stale_anchor"
			}
		}
		resp.Total++
		if !checked.Resolved() {
			resp.Unresolved++
		}
		if len(resp.Links) < MaxCheckedLinks {
			resp.Links = append(resp.Links, checked)
		} else {
			resp.Truncated = true
		}
	}
	return resp, nil
}

// checkAnchorLocked reports whether an anchor names something in the target.
func (s *SQLiteStore) checkAnchorLocked(targetDocumentID, anchor, editedDocumentID string, selfAnchors map[string]bool) (bool, error) {
	if editedDocumentID != "" && targetDocumentID == editedDocumentID {
		return selfAnchors[strings.TrimSpace(anchor)] ||
			selfAnchors[markdownblocks.Slugify(anchor)], nil
	}
	if _, err := s.findDocumentBlockLocked(targetDocumentID, anchor); err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// bufferAnchors collects every anchor the submitted body currently offers:
// authored markers, derived block IDs, and heading slugs.
func bufferAnchors(documentID, body string) map[string]bool {
	anchors := map[string]bool{}
	if documentID == "" {
		return anchors
	}
	for _, block := range markdownblocks.Extract(documentID, body) {
		if block.Marker != "" {
			anchors[block.Marker] = true
		}
		if block.ID != "" {
			anchors[block.ID] = true
		}
		if block.Slug != "" {
			anchors[block.Slug] = true
		}
	}
	return anchors
}

// escapeLikePrefix makes a user-typed prefix safe as a LIKE pattern. Without
// it, typing `%` would match every note and `_` would match any character —
// wrong answers rather than a security problem, but wrong is enough.
func escapeLikePrefix(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

// ftsPrefixTerm turns a typed prefix into one FTS5 prefix term. Anything that
// is not a letter or digit is dropped rather than quoted: FTS5's own tokenizer
// discarded it at index time, so matching on it could never succeed.
func ftsPrefixTerm(query string) string {
	var b strings.Builder
	for _, r := range query {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r > 127:
			b.WriteRune(r)
		default:
			// A token boundary ends the term: FTS5 matches one token at a time
			// and the first word is the one worth completing.
			if b.Len() > 0 {
				return `"` + b.String() + `"*`
			}
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return `"` + b.String() + `"*`
}

func itoaInt(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
