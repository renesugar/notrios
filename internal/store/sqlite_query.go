package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/query"
)

// Query-language execution for the SQLite backend. Chronological results use
// an (updated_at DESC, id DESC) keyset. FTS5 relevance results use the stable
// (bm25 score ASC, id ASC) boundary; neither path performs OFFSET work.
func (s *SQLiteStore) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	req = NormalizeSearchRequest(req)
	parsed := query.Parse(req.Query, time.Now())

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.searchQueryLocked(req, parsed)
}

func (s *SQLiteStore) searchQueryLocked(req SearchRequest, q query.Query) (SearchResponse, error) {
	where := []string{"d.collection_id = ?"}
	args := []string{req.CollectionID}

	if q.Trashed {
		where = append(where, "d.deleted_at IS NOT NULL")
	} else {
		where = append(where, "d.deleted_at IS NULL")
	}

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

	for _, tagName := range q.Tags {
		tag, found, err := s.findTagByNameLocked(tagName)
		if err != nil {
			return SearchResponse{}, err
		}
		if !found {
			return SearchResponse{Hits: []SearchHit{}}, nil
		}
		where = append(where, `d.id IN (
			SELECT nt.document_id FROM note_tags nt WHERE nt.tag_id = ?
		)`)
		args = append(args, tag.ID)
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
	binding := searchCursorBinding(req, q, useFTS)
	var sql string
	if useFTS {
		where = append([]string{"documents_fts MATCH ?"}, where...)
		args = append([]string{ftsMatchExpr(q)}, args...)
		sql = `WITH ranked AS (
			SELECT d.id AS id, d.collection_id AS collection_id, d.title AS title,
				snippet(documents_fts, 3, '<mark>', '</mark>', '…', 32) AS hit_snippet,
				bm25(documents_fts, 5.0, 1.0) AS score,
				COALESCE(d.notebook_id, '') AS notebook_id,
				d.updated_at AS sort_time
			FROM documents_fts
			JOIN documents d ON d.id = documents_fts.document_id
			JOIN document_revisions r ON r.id = d.current_revision_id` +
			sourceJoin(needSources) +
			" WHERE " + strings.Join(where, " AND ") +
			`) SELECT id, collection_id, title, hit_snippet, score, notebook_id, sort_time
			FROM ranked`
		if strings.TrimSpace(req.Cursor) != "" {
			score, id, err := decodeRelevanceCursor(req.Cursor, binding)
			if err != nil {
				return SearchResponse{}, err
			}
			sql += ` WHERE (score > CAST(? AS REAL) OR (score = CAST(? AS REAL) AND id > ?))`
			scoreText := strconv.FormatFloat(score, 'g', -1, 64)
			args = append(args, scoreText, scoreText, id)
		}
		sql += " ORDER BY score ASC, id ASC"
	} else {
		for _, term := range append(append([]query.Term{}, q.Terms...), q.Title...) {
			where = append(where, `(d.title LIKE ? ESCAPE '\' OR r.body LIKE ? ESCAPE '\')`)
			pattern := "%" + escapeLike(term.Text) + "%"
			args = append(args, pattern, pattern)
		}
		if strings.TrimSpace(req.Cursor) != "" {
			timestamp, id, err := decodeChronologicalCursor(req.Cursor, binding)
			if err != nil {
				return SearchResponse{}, err
			}
			where = append(where, `(d.updated_at, d.id) < (?, ?)`)
			args = append(args, timestamp, id)
		}
		sql = `SELECT d.id, d.collection_id, d.title, substr(r.body, 1, 240), 0.0,
				COALESCE(d.notebook_id, ''), d.updated_at
			FROM documents d
			JOIN document_revisions r ON r.id = d.current_revision_id` +
			sourceJoin(needSources) +
			" WHERE " + strings.Join(where, " AND ") +
			" ORDER BY d.updated_at DESC, d.id DESC"
	}
	sql += " LIMIT " + itoa(req.Limit+1)

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
		last := resp.Hits[len(resp.Hits)-1]
		if useFTS {
			resp.NextCursor = encodeRelevanceCursor(binding, last.Score, last.ID)
		} else {
			resp.NextCursor = encodeChronologicalCursor(binding, last.sortTime, last.ID)
		}
	}
	return resp, nil
}

func searchCursorBinding(req SearchRequest, q query.Query, relevance bool) string {
	parsed, _ := json.Marshal(q)
	mode := "chronological"
	sortOrder := "updated_at:desc,id:desc"
	if relevance {
		mode = "relevance"
		sortOrder = "bm25:asc,id:asc"
	}
	return cursorBinding("search", req.Query, req.CollectionID, mode, sortOrder, string(parsed))
}

func sourceJoin(need bool) string {
	if !need {
		return ""
	}
	return " LEFT JOIN document_sources ds ON ds.document_id = d.id"
}

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
