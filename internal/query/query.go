// Package query parses the Notrios user search language documented in
// SEARCH_QUERY_LANGUAGE.md. The parser is backend-agnostic: the store compiles
// the parsed query to SQLite FTS5 + SQL filters, and a future Recoll backend
// can compile the same structure to Recoll query syntax.
package query

import (
	"strconv"
	"strings"
	"time"
)

// Term is one free-text element. Phrase terms came from double quotes and
// must match as a phrase.
type Term struct {
	Text   string
	Phrase bool
}

// Query is the parsed form of a user search string. All fields combine with
// AND; repeated field filters also AND (e.g. two tag: filters require both).
type Query struct {
	Terms     []Term   // unqualified terms searched over title and body
	Title     []Term   // title: terms
	Notebooks []string // notebook: names (case-insensitive)
	Tags      []string // tag: names (case-insensitive)
	Authors   []string // author: display-name words/phrases
	AuthorIDs []string // authorid: canonical identities
	Since     int64    // UTC Unix seconds, inclusive lower bound; 0 = unset
	Until     int64    // UTC Unix seconds, inclusive upper bound; 0 = unset
	Trashed   bool     // is:trashed — reserved for the Trash search notebook
}

// IsEmpty reports whether the query has no constraints at all (the "All
// notes" query).
func (q Query) IsEmpty() bool {
	return len(q.Terms) == 0 && len(q.Title) == 0 && len(q.Notebooks) == 0 &&
		len(q.Tags) == 0 && len(q.Authors) == 0 && len(q.AuthorIDs) == 0 &&
		q.Since == 0 && q.Until == 0 && !q.Trashed
}

// HasTextTerms reports whether an FTS text match is needed.
func (q Query) HasTextTerms() bool {
	return len(q.Terms) > 0 || len(q.Title) > 0
}

// Parse interprets a user query string. now supplies "today" and the selected
// timezone for date-only and time-only since:/until: values.
func Parse(input string, now time.Time) Query {
	q := Query{}
	for _, token := range tokenize(input) {
		op, value, isOp := splitOperator(token)
		if !isOp {
			q.Terms = append(q.Terms, termFromToken(token))
			continue
		}
		switch strings.ToLower(op) {
		case "title":
			q.Title = append(q.Title, termFromToken(value))
		case "notebook":
			q.Notebooks = append(q.Notebooks, unquote(value))
		case "tag":
			q.Tags = append(q.Tags, unquote(value))
		case "author":
			q.Authors = append(q.Authors, unquote(value))
		case "authorid":
			q.AuthorIDs = append(q.AuthorIDs, unquote(value))
		case "since":
			if ts, ok := parseTimeBound(unquote(value), now, false); ok {
				q.Since = ts
			}
		case "until":
			if ts, ok := parseTimeBound(unquote(value), now, true); ok {
				q.Until = ts
			}
		case "is":
			if strings.EqualFold(unquote(value), "trashed") {
				q.Trashed = true
			}
		default:
			// Unknown operators fall back to a literal term so users are
			// never surprised by silently dropped text (e.g. "re:invoice").
			q.Terms = append(q.Terms, termFromToken(token))
		}
	}
	return q
}

// tokenize splits on whitespace while keeping double-quoted spans (including
// op:"quoted value" forms) together.
func tokenize(input string) []string {
	tokens := []string{}
	var current strings.Builder
	inQuote := false
	for _, r := range input {
		switch {
		case r == '"':
			inQuote = !inQuote
			current.WriteRune(r)
		case !inQuote && (r == ' ' || r == '\t' || r == '\n' || r == '\r'):
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// splitOperator recognizes op:value tokens. A quoted token ("exact phrase")
// or a token without a colon before any quote is not an operator.
func splitOperator(token string) (op, value string, isOp bool) {
	if strings.HasPrefix(token, `"`) {
		return "", "", false
	}
	idx := strings.Index(token, ":")
	if idx <= 0 || idx == len(token)-1 {
		return "", "", false
	}
	op = token[:idx]
	for _, r := range op {
		if !isLetter(r) {
			return "", "", false
		}
	}
	return op, token[idx+1:], true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func termFromToken(token string) Term {
	if strings.HasPrefix(token, `"`) || strings.HasSuffix(token, `"`) {
		return Term{Text: unquote(token), Phrase: true}
	}
	return Term{Text: token}
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, `"`)
	value = strings.TrimSuffix(value, `"`)
	return strings.TrimSpace(value)
}

// parseTimeBound normalizes a since:/until: value to UTC Unix seconds.
//
//   - Full timestamps ("2026-07-13T18:42:07", with or without zone) are exact.
//   - Date-only values mean start of day for since: and end of day (23:59:59)
//     for until:, in the selected timezone.
//   - Time-only values ("14:30:00", "14:30") mean today at that time.
//   - All-digit values are Unix epoch seconds (milliseconds when large).
func parseTimeBound(value string, now time.Time, endOfDay bool) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	loc := now.Location()
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC().Unix(), true
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t.UTC().Unix(), true
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", value, loc); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return t.UTC().Unix(), true
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			day := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
			return day.UTC().Unix(), true
		}
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
		if n > 1_000_000_000_000 {
			return n / 1000, true
		}
		return n, true
	}
	return 0, false
}
