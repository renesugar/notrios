package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/query"
)

// Query-language execution for SQLite. Positive text-only trees retain FTS5
// relevance ordering. Trees containing metadata or unary negation compile to
// exact correlated predicates and use chronological keysets; neither path
// performs unbounded OFFSET work.
func (s *SQLiteStore) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	req = NormalizeSearchRequest(req)
	parsed, err := query.Parse(req.Query, time.Now())
	if err != nil {
		return SearchResponse{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.searchQueryLocked(req, parsed)
}

func (s *SQLiteStore) searchQueryLocked(req SearchRequest, q query.Query) (SearchResponse, error) {
	// A search spans every collection unless the query narrows it. This used to
	// pin every search to one collection chosen by the caller, defaulted to
	// `default`, which made a note imported under any other provenance
	// unfindable from the search box and made `collection:` meaningless: the
	// two predicates were ANDed, so naming any collection but the caller's
	// matched nothing at all.
	//
	// CollectionID remains as an optional narrowing for callers that genuinely
	// mean one collection -- an MCP tool given a `collection` argument -- and
	// empty now means all of them rather than `default`.
	where := []string{}
	args := []string{}
	if collection := strings.TrimSpace(req.CollectionID); collection != "" {
		where = append(where, "d.collection_id = ?")
		args = append(args, collection)
	}
	if q.Trashed {
		where = append(where, "d.deleted_at IS NOT NULL")
	} else {
		where = append(where, "d.deleted_at IS NULL")
	}

	ftsAnchor, remainder := splitFTSAnchor(q)
	useFTSRelevance := ftsAnchor != nil
	// An explicit chronological request takes the predicate path, which
	// compiles text terms exactly and orders by the `(updated_at, id)` keyset.
	// Relevance is never forced the other way: without a positive text anchor
	// there is nothing to rank, so the caller is refused above rather than
	// handed an arbitrary order under a name that promises one.
	if useFTSRelevance && req.Sort == SortUpdated {
		useFTSRelevance = false
		remainder = q.Root
	}
	binding := searchCursorBinding(req, q, useFTSRelevance)
	var sql string
	if useFTSRelevance {
		where = append([]string{"documents_fts MATCH ?"}, where...)
		args = append([]string{ftsMatchExpr(ftsAnchor)}, args...)
		predicate, predicateArgs, err := s.compileSQLExprLocked(remainder, false)
		if err != nil {
			return SearchResponse{}, err
		}
		if predicate != "" && predicate != "1" {
			where = append(where, predicate)
			args = append(args, predicateArgs...)
		}
		sql = `WITH ranked AS (
			SELECT d.id AS id, d.collection_id AS collection_id, d.title AS title,
				snippet(documents_fts, 3, '<mark>', '</mark>', '…', 32) AS hit_snippet,
				bm25(documents_fts, 5.0, 1.0) AS score,
				COALESCE(d.notebook_id, '') AS notebook_id,
				d.updated_at AS sort_time
			FROM documents_fts
			JOIN documents d ON d.id = documents_fts.document_id
			JOIN document_revisions r ON r.id = d.current_revision_id
			WHERE ` + strings.Join(where, " AND ") +
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
		predicate, predicateArgs, err := s.compileSQLExprLocked(q.Root, q.Trashed)
		if err != nil {
			return SearchResponse{}, err
		}
		if predicate != "" && predicate != "1" {
			where = append(where, predicate)
			args = append(args, predicateArgs...)
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
			JOIN document_revisions r ON r.id = d.current_revision_id
			WHERE ` + strings.Join(where, " AND ") +
			" ORDER BY d.updated_at DESC, d.id DESC"
	}
	if req.countOnly {
		// The same predicate, counted rather than paged. A count built from a
		// second compilation of the query would be a second answer to the same
		// question, and the two would eventually disagree; paging the whole
		// result set to count it would be honest and slow, which on a large
		// library is its own kind of wrong.
		counted, err := s.countSearchLocked(useFTSRelevance, where, args)
		if err != nil {
			return SearchResponse{}, err
		}
		return SearchResponse{Total: counted, Counted: true}, nil
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
		if useFTSRelevance {
			resp.NextCursor = encodeRelevanceCursor(binding, last.Score, last.ID)
		} else {
			resp.NextCursor = encodeChronologicalCursor(binding, last.sortTime, last.ID)
		}
	}
	return resp, nil
}

// countSearchLocked counts the rows the compiled predicate matches.
//
// The joins mirror the two select shapes exactly: an FTS-anchored search counts
// through documents_fts, and a predicate search counts through documents. A
// count over different joins than the listing would answer a different question
// while looking like the same one.
func (s *SQLiteStore) countSearchLocked(useFTS bool, where, args []string) (int64, error) {
	from := `FROM documents d
			JOIN document_revisions r ON r.id = d.current_revision_id`
	if useFTS {
		from = `FROM documents_fts
			JOIN documents d ON d.id = documents_fts.document_id
			JOIN document_revisions r ON r.id = d.current_revision_id`
	}
	sql := "SELECT COUNT(*) " + from + " WHERE " + strings.Join(where, " AND ")
	stmt, err := s.prepareLocked(sql)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return 0, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return int64(C.sqlite3_column_int64(stmt, 0)), nil
	case C.SQLITE_DONE:
		return 0, nil
	default:
		return 0, s.stepErrLocked(rc)
	}
}

func splitFTSAnchor(q query.Query) (anchor, remainder *query.Expr) {
	if q.Trashed || !q.HasPositiveTextAnchor() {
		return nil, q.Root
	}
	if q.PositiveTextOnly() {
		return q.Root, &query.Expr{Op: query.OpMatchAll}
	}
	anchors := []*query.Expr{}
	remaining := []*query.Expr{}
	for _, child := range q.Root.Children {
		if (query.Query{Root: child}).PositiveTextOnly() {
			anchors = append(anchors, child)
		} else {
			remaining = append(remaining, child)
		}
	}
	return expressionGroup(query.OpAnd, anchors), expressionGroup(query.OpAnd, remaining)
}

func expressionGroup(op query.Op, children []*query.Expr) *query.Expr {
	if len(children) == 0 {
		return &query.Expr{Op: query.OpMatchAll}
	}
	if len(children) == 1 {
		return children[0]
	}
	return &query.Expr{Op: op, Children: children}
}

// searchCursorBinding fingerprints everything a cursor is only valid against.
// Sort belongs in it: replaying a relevance cursor against a chronological
// request would decode the wrong boundary.
func searchCursorBinding(req SearchRequest, q query.Query, relevance bool) string {
	mode := "chronological"
	sortOrder := "updated_at:desc,id:desc"
	if relevance {
		mode = "relevance"
		sortOrder = "bm25:asc,id:asc"
	}
	return cursorBinding("search", req.CollectionID, mode, sortOrder, q.Canonical())
}

func (s *SQLiteStore) compileSQLExprLocked(expr *query.Expr, trashed bool) (string, []string, error) {
	if expr == nil || expr.Op == query.OpMatchAll {
		return "1", nil, nil
	}
	if expr.Op == query.OpMatchNone {
		return "0", nil, nil
	}
	switch expr.Op {
	case query.OpNot:
		if len(expr.Children) != 1 {
			return "", nil, fmt.Errorf("%w: malformed NOT expression", ErrInvalidInput)
		}
		child, args, err := s.compileSQLExprLocked(expr.Children[0], trashed)
		return "NOT (" + child + ")", args, err
	case query.OpAnd, query.OpOr:
		joiner := " AND "
		if expr.Op == query.OpOr {
			joiner = " OR "
		}
		parts := make([]string, 0, len(expr.Children))
		args := []string{}
		for _, child := range expr.Children {
			part, childArgs, err := s.compileSQLExprLocked(child, trashed)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, "("+part+")")
			args = append(args, childArgs...)
		}
		return "(" + strings.Join(parts, joiner) + ")", args, nil
	case query.OpTerm:
		if expr.Term == nil {
			return "", nil, fmt.Errorf("%w: malformed term expression", ErrInvalidInput)
		}
		return s.compileSQLTermLocked(*expr.Term, trashed)
	default:
		return "", nil, fmt.Errorf("%w: unsupported query expression %q", ErrInvalidInput, expr.Op)
	}
}

func (s *SQLiteStore) compileSQLTermLocked(term query.Term, trashed bool) (string, []string, error) {
	switch term.Field {
	case query.FieldText, query.FieldTitle:
		if trashed || query.ContainsSymbol(term.Text) {
			pattern := "%" + escapeLike(term.Text) + "%"
			if term.Field == query.FieldTitle {
				return `d.title LIKE ? ESCAPE '\'`, []string{pattern}, nil
			}
			return `(d.title LIKE ? ESCAPE '\' OR r.body LIKE ? ESCAPE '\')`, []string{pattern, pattern}, nil
		}
		match := ftsQuote(term.Text)
		if term.Field == query.FieldTitle {
			match = "title:" + match
		}
		return `d.id IN (
			SELECT documents_fts.document_id FROM documents_fts
			WHERE documents_fts MATCH ?
		)`, []string{match}, nil
	case query.FieldNotebook:
		ids, err := s.notebooksByNamesLocked([]string{term.Text})
		if err != nil {
			return "", nil, err
		}
		if len(ids) == 0 {
			return "0", nil, nil
		}
		return "d.notebook_id IN (" + placeholders(len(ids)) + ")", ids, nil
	case query.FieldCollection:
		// A column on the document, not a join: every note carries exactly one
		// collection, which is the whole difference between provenance and a
		// tag. Matched case-insensitively for the same reason notebook names
		// are, since a collection ID is typed by a person at import time.
		return "d.collection_id = ? COLLATE NOCASE", []string{term.Text}, nil
	case query.FieldTag:
		return `d.id IN (
			SELECT qnt.document_id FROM note_tags qnt
			JOIN tags qt ON qt.id = qnt.tag_id
			WHERE qt.name = ? COLLATE NOCASE
		)`, []string{term.Text}, nil
	case query.FieldAuthor:
		return `d.id IN (
			SELECT qds.document_id FROM document_sources qds
			WHERE qds.author LIKE ? ESCAPE '\'
		)`, []string{"%" + escapeLike(term.Text) + "%"}, nil
	case query.FieldAuthorID:
		return `d.id IN (
			SELECT qds.document_id FROM document_sources qds
			WHERE qds.author_id = ? COLLATE NOCASE
		)`, []string{term.Text}, nil
	case query.FieldSince, query.FieldUntil:
		op := ">="
		if term.Field == query.FieldUntil {
			op = "<="
		}
		timeExpr := `COALESCE(
			(SELECT qds.published_ts FROM document_sources qds WHERE qds.document_id = d.id),
			CAST(strftime('%s', d.created_at) AS INTEGER)
		)`
		return timeExpr + " " + op + " CAST(? AS INTEGER)", []string{strconv.FormatInt(term.UnixValue, 10)}, nil
	default:
		return "", nil, fmt.Errorf("%w: unsupported query field %q", ErrInvalidInput, term.Field)
	}
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

func ftsMatchExpr(expr *query.Expr) string {
	if expr == nil || expr.Op == query.OpMatchAll {
		return `""`
	}
	if expr.Op == query.OpMatchNone {
		return `noteid:"__notrios_no_match__"`
	}
	switch expr.Op {
	case query.OpTerm:
		value := ftsQuote(expr.Term.Text)
		if expr.Term.Field == query.FieldTitle {
			return "title:" + value
		}
		return value
	case query.OpAnd, query.OpOr:
		joiner := " AND "
		if expr.Op == query.OpOr {
			joiner = " OR "
		}
		parts := make([]string, 0, len(expr.Children))
		for _, child := range expr.Children {
			parts = append(parts, "("+ftsMatchExpr(child)+")")
		}
		return "(" + strings.Join(parts, joiner) + ")"
	default:
		return `noteid:"__notrios_no_match__"`
	}
}

func ftsQuote(text string) string {
	return `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
