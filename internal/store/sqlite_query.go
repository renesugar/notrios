package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/query"
)

// Query-language execution for the SQLite backend (Notrios redesign task R6).
// User queries (SEARCH_QUERY_LANGUAGE.md) are parsed by internal/query and
// compiled here to FTS5 + SQL filters. Search notebooks execute through this
// path: the empty query is "All notes" and `is:trashed` is the Trash query.

const maxSearchOffset = 100000

func (s *SQLiteStore) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	req = NormalizeSearchRequest(req)
	parsed := query.Parse(req.Query, time.Now())

	offset := 0
	if strings.TrimSpace(req.Cursor) != "" {
		var err error
		offset, err = decodeSearchCursor(req.Cursor, req.Query, req.CollectionID)
		if err != nil {
			return SearchResponse{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.searchQueryLocked(req, parsed, offset)
}

func (s *SQLiteStore) searchQueryLocked(req SearchRequest, q query.Query, offset int) (SearchResponse, error) {
	where := []string{"d.collection_id = ?"}
	args := []string{req.CollectionID}

	if q.Trashed {
		where = append(where, "d.deleted_at IS NOT NULL")
	} else {
		where = append(where, "d.deleted_at IS NULL")
	}

	// notebook: names expand to matching notebooks plus their descendants;
	// no matching notebook means no results.
	if len(q.Notebooks) > 0 {
		ids, err := s.notebooksByNamesLocked(q.Notebooks)
		if err != nil {
			return SearchResponse{}, err
		}
		if len(ids) == 0 {
			return SearchResponse{Hits: []SearchHit{}}, nil
		}
		where = append(where, "d.notebook_id IN ("+placeholders(len(ids))+")")
		args = append(args, ids...)
	}

	for _, tag := range q.Tags {
		where = append(where, `EXISTS (SELECT 1 FROM note_tags nt JOIN tags t ON t.id = nt.tag_id WHERE nt.document_id = d.id AND t.name = ? COLLATE NOCASE)`)
		args = append(args, tag)
	}

	needSources := len(q.Authors) > 0 || len(q.AuthorIDs) > 0 || q.Since > 0 || q.Until > 0
	for _, author := range q.Authors {
		where = append(where, `ds.author LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(author)+"%")
	}
	for _, authorID := range q.AuthorIDs {
		where = append(where, `ds.author_id = ? COLLATE NOCASE`)
		args = append(args, authorID)
	}
	// since:/until: compare the published timestamp when provenance exists,
	// falling back to the note's local creation time.
	// Bound parameters arrive as TEXT through this cgo adapter, so cast them:
	// SQLite would otherwise order every INTEGER below every TEXT value.
	timeExpr := `COALESCE(ds.published_ts, CAST(strftime('%s', d.created_at) AS INTEGER))`
	if q.Since > 0 {
		where = append(where, timeExpr+" >= CAST(? AS INTEGER)")
		args = append(args, strconv.FormatInt(q.Since, 10))
	}
	if q.Until > 0 {
		where = append(where, timeExpr+" <= CAST(? AS INTEGER)")
		args = append(args, strconv.FormatInt(q.Until, 10))
	}

	useFTS := q.HasTextTerms() && !q.Trashed
	var sql string
	switch {
	case useFTS:
		where = append([]string{"documents_fts MATCH ?"}, where...)
		args = append([]string{ftsMatchExpr(q)}, args...)
		sql = `SELECT d.id, d.collection_id, d.title,
			snippet(documents_fts, 3, '<mark>', '</mark>', '…', 32) AS snippet,
			bm25(documents_fts, 5.0, 1.0) AS score
			FROM documents_fts
			JOIN documents d ON d.id = documents_fts.document_id
			JOIN document_revisions r ON r.id = d.current_revision_id` +
			sourceJoin(needSources) +
			" WHERE " + strings.Join(where, " AND ") +
			" ORDER BY score, d.id"
	default:
		// Trash browsing and metadata-only queries match text with LIKE so
		// trashed notes (absent from FTS) stay findable.
		for _, term := range append(append([]query.Term{}, q.Terms...), q.Title...) {
			where = append(where, `(d.title LIKE ? ESCAPE '\' OR r.body LIKE ? ESCAPE '\')`)
			pattern := "%" + escapeLike(term.Text) + "%"
			args = append(args, pattern, pattern)
		}
		sql = `SELECT d.id, d.collection_id, d.title, substr(r.body, 1, 240), 0.0
			FROM documents d
			JOIN document_revisions r ON r.id = d.current_revision_id` +
			sourceJoin(needSources) +
			" WHERE " + strings.Join(where, " AND ") +
			" ORDER BY d.updated_at DESC, d.id DESC"
	}
	sql += " LIMIT " + itoa(req.Limit+1) + " OFFSET " + itoa(offset)

	stmt, err := s.prepareLocked(sql)
	if err != nil {
		return SearchResponse{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return SearchResponse{}, err
	}
	resp, err := s.readHitsLocked(stmt)
	if err != nil {
		return SearchResponse{}, err
	}
	if len(resp.Hits) > req.Limit {
		resp.Hits = resp.Hits[:req.Limit]
		next := offset + req.Limit
		if next <= maxSearchOffset {
			resp.NextCursor = encodeSearchCursor(next, req.Query, req.CollectionID)
		}
	}
	return resp, nil
}

func sourceJoin(need bool) string {
	if !need {
		return ""
	}
	return " LEFT JOIN document_sources ds ON ds.document_id = d.id"
}

// notebooksByNamesLocked resolves case-insensitive notebook names to notebook
// IDs, including all descendants of each match.
func (s *SQLiteStore) notebooksByNamesLocked(names []string) ([]string, error) {
	seen := map[string]bool{}
	ids := []string{}
	for _, name := range names {
		stmt, err := s.prepareLocked(`SELECT id FROM notebooks WHERE name = ? COLLATE NOCASE`)
		if err != nil {
			return nil, err
		}
		if err := bindAll(stmt, []string{name}); err != nil {
			C.sqlite3_finalize(stmt)
			return nil, err
		}
		matches := []string{}
		for {
			rc := C.sqlite3_step(stmt)
			if rc == C.SQLITE_ROW {
				matches = append(matches, columnText(stmt, 0))
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
		for _, match := range matches {
			subtree, err := s.notebookSubtreeIDsLocked(match)
			if err != nil {
				return nil, err
			}
			for _, id := range subtree {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	return ids, nil
}

// ftsMatchExpr builds the FTS5 MATCH expression: free terms search title and
// body, title: terms use the FTS column filter, and everything joins with AND.
func ftsMatchExpr(q query.Query) string {
	parts := make([]string, 0, len(q.Terms)+len(q.Title))
	for _, term := range q.Terms {
		parts = append(parts, ftsQuote(term.Text))
	}
	for _, term := range q.Title {
		parts = append(parts, "title:"+ftsQuote(term.Text))
	}
	if len(parts) == 0 {
		return `""`
	}
	return strings.Join(parts, " AND ")
}

func ftsQuote(text string) string {
	return `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

// Search cursors are opaque offset tokens bound to the query and collection
// so a cursor cannot be replayed against a different search.
func searchCursorChecksum(queryText, collectionID string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(queryText))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(collectionID))
	return h.Sum32()
}

func encodeSearchCursor(offset int, queryText, collectionID string) string {
	raw := fmt.Sprintf("q1:%d:%08x", offset, searchCursorChecksum(queryText, collectionID))
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeSearchCursor(cursor, queryText, collectionID string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if err != nil {
		return 0, fmt.Errorf("%w: cursor is invalid", ErrInvalidInput)
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 || parts[0] != "q1" {
		return 0, fmt.Errorf("%w: cursor is invalid", ErrInvalidInput)
	}
	offset, err := strconv.Atoi(parts[1])
	if err != nil || offset < 0 || offset > maxSearchOffset {
		return 0, fmt.Errorf("%w: cursor is invalid", ErrInvalidInput)
	}
	if parts[2] != fmt.Sprintf("%08x", searchCursorChecksum(queryText, collectionID)) {
		return 0, fmt.Errorf("%w: cursor does not match this query", ErrInvalidInput)
	}
	return offset, nil
}
