// Package query parses the bounded, backend-neutral Notrios search language
// documented in SEARCH_QUERY_LANGUAGE.md.
package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	// MaxInputBytes bounds work before tokenization.
	MaxInputBytes = 4096
	// MaxTokens bounds parser work and the size of compiled backend queries.
	MaxTokens = 256
	// MaxDepth bounds parenthesized expression nesting.
	MaxDepth = 16
)

var (
	ErrTooLong       = errors.New("search query is too long")
	ErrTooManyTokens = errors.New("search query has too many tokens")
	ErrTooDeep       = errors.New("search query is nested too deeply")
	ErrSyntax        = errors.New("invalid search query syntax")
)

// Op identifies one expression node operation.
type Op string

const (
	OpMatchAll  Op = "all"
	OpMatchNone Op = "none"
	OpTerm      Op = "term"
	OpAnd       Op = "and"
	OpOr        Op = "or"
	OpNot       Op = "not"
)

// Field identifies a supported query leaf. Unknown word:value forms use
// FieldText and retain the complete token as searchable text.
type Field string

const (
	FieldText     Field = "text"
	FieldTitle    Field = "title"
	FieldNotebook Field = "notebook"
	// FieldCollection matches a note's provenance rather than its filing.
	// `category:` is an alias for `notebook:` and always has been, so it is
	// not available for this and a separate name is the honest one.
	FieldCollection Field = "collection"
	FieldTag        Field = "tag"
	FieldAuthor     Field = "author"
	FieldAuthorID   Field = "authorid"
	FieldSince      Field = "since"
	FieldUntil      Field = "until"
	FieldTrashed    Field = "trashed"
)

// Term is one typed expression leaf. UnixValue is used by since:/until:.
type Term struct {
	Field     Field  `json:"field"`
	Text      string `json:"text,omitempty"`
	Phrase    bool   `json:"phrase,omitempty"`
	UnixValue int64  `json:"unix_value,omitempty"`
}

// Expr is a bounded expression tree. Term is populated only for OpTerm;
// Children contains one node for OpNot and two or more for OpAnd/OpOr.
type Expr struct {
	Op       Op      `json:"op"`
	Term     *Term   `json:"term,omitempty"`
	Children []*Expr `json:"children,omitempty"`
}

// Query is the parsed user query. Trashed switches the canonical row scope;
// is:trashed is accepted only as a positive AND constraint because the
// derived Recoll projection intentionally contains no deleted notes.
type Query struct {
	Root    *Expr `json:"root,omitempty"`
	Trashed bool  `json:"trashed,omitempty"`
}

// IsEmpty reports whether the query is the All notes scope.
func (q Query) IsEmpty() bool {
	return q.Root == nil || q.Root.Op == OpMatchAll
}

// HasTextTerms reports whether any title/body FTS leaf is present.
func (q Query) HasTextTerms() bool {
	return walk(q.Root, func(expr *Expr) bool {
		return expr.Op == OpTerm && expr.Term != nil &&
			(expr.Term.Field == FieldText || expr.Term.Field == FieldTitle)
	})
}

// PositiveTextOnly reports whether FTS5 can evaluate the entire expression
// directly while retaining relevance ordering. Unary negation and metadata
// leaves use the exact SQL predicate compiler instead.
func (q Query) PositiveTextOnly() bool {
	if q.Trashed || q.Root == nil || q.Root.Op == OpMatchAll || q.Root.Op == OpMatchNone {
		return false
	}
	valid := true
	walk(q.Root, func(expr *Expr) bool {
		switch expr.Op {
		case OpNot:
			valid = false
		case OpTerm:
			if expr.Term == nil || (expr.Term.Field != FieldText && expr.Term.Field != FieldTitle) {
				valid = false
			} else if ContainsSymbol(expr.Term.Text) {
				valid = false
			}
		}
		return false
	})
	return valid
}

// HasPositiveTextAnchor reports whether a positive text-only subtree is a
// mandatory top-level AND constraint. SQLite can drive such mixed expressions
// from FTS5 and apply the remaining metadata/negation predicates exactly.
func (q Query) HasPositiveTextAnchor() bool {
	if q.Trashed || q.Root == nil {
		return false
	}
	if q.PositiveTextOnly() {
		return true
	}
	if q.Root.Op != OpAnd {
		return false
	}
	for _, child := range q.Root.Children {
		if (Query{Root: child}).PositiveTextOnly() {
			return true
		}
	}
	return false
}

// ContainsSymbol reports whether text contains a Unicode "other symbol"
// codepoint. FTS tokenizers commonly discard these (including emoji), so the
// backends use an explicit exact-symbol path.
func ContainsSymbol(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.So, r) {
			return true
		}
	}
	return false
}

// SymbolKeys returns stable lowercase keys used by the Recoll projection.
func SymbolKeys(text string) []string {
	keys := []string{}
	seen := map[rune]bool{}
	for _, r := range text {
		if unicode.Is(unicode.So, r) && !seen[r] {
			seen[r] = true
			keys = append(keys, fmt.Sprintf("u%x", r))
		}
	}
	return keys
}

// Canonical returns the stable expression representation used in cursor and
// merged-snapshot bindings.
func (q Query) Canonical() string {
	raw, _ := json.Marshal(q)
	return string(raw)
}

func walk(expr *Expr, visit func(*Expr) bool) bool {
	if expr == nil {
		return false
	}
	if visit(expr) {
		return true
	}
	for _, child := range expr.Children {
		if walk(child, visit) {
			return true
		}
	}
	return false
}

// Parse interprets one user query. now supplies the selected timezone/day for
// date-only and time-only since:/until: values.
func Parse(input string, now time.Time) (Query, error) {
	if len(input) > MaxInputBytes {
		return Query{}, fmt.Errorf("%w: maximum is %d bytes", ErrTooLong, MaxInputBytes)
	}
	tokens, err := lex(input)
	if err != nil {
		return Query{}, err
	}
	if len(tokens) > MaxTokens {
		return Query{}, fmt.Errorf("%w: maximum is %d", ErrTooManyTokens, MaxTokens)
	}
	if len(tokens) == 0 {
		return Query{Root: &Expr{Op: OpMatchAll}}, nil
	}
	p := parser{tokens: tokens, now: now}
	root, err := p.parseOr(0)
	if err != nil {
		return Query{}, err
	}
	if p.pos != len(tokens) {
		return Query{}, fmt.Errorf("%w near %q", ErrSyntax, tokens[p.pos].raw)
	}
	root, trashed, err := extractTrashScope(root, false, false)
	if err != nil {
		return Query{}, err
	}
	root = simplify(root)
	return Query{Root: root, Trashed: trashed}, nil
}

type tokenKind uint8

const (
	tokenWord tokenKind = iota
	tokenOr
	tokenNot
	tokenLParen
	tokenRParen
)

type token struct {
	kind tokenKind
	raw  string
}

// lex preserves URLs, emoji, unknown colon tokens, and embedded hyphens. An
// opening parenthesis inside an existing token is treated as literal and its
// matching close is kept with that token (for URL/path compatibility).
func lex(input string) ([]token, error) {
	runes := []rune(input)
	tokens := make([]token, 0)
	for i := 0; i < len(runes); {
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		switch runes[i] {
		case '(':
			tokens = append(tokens, token{kind: tokenLParen, raw: "("})
			i++
			continue
		case ')':
			tokens = append(tokens, token{kind: tokenRParen, raw: ")"})
			i++
			continue
		case '-':
			tokens = append(tokens, token{kind: tokenNot, raw: "-"})
			i++
			continue
		}

		var b strings.Builder
		inQuote := false
		embeddedParens := 0
		for i < len(runes) {
			r := runes[i]
			if r == '"' {
				inQuote = !inQuote
				b.WriteRune(r)
				i++
				continue
			}
			if !inQuote {
				if unicode.IsSpace(r) {
					break
				}
				if r == '(' {
					embeddedParens++
					b.WriteRune(r)
					i++
					continue
				}
				if r == ')' {
					if embeddedParens == 0 {
						break
					}
					embeddedParens--
					b.WriteRune(r)
					i++
					continue
				}
			}
			b.WriteRune(r)
			i++
		}
		if inQuote {
			return nil, fmt.Errorf("%w: unterminated quote", ErrSyntax)
		}
		raw := b.String()
		if raw == "" {
			continue
		}
		kind := tokenWord
		if raw == "OR" {
			kind = tokenOr
		}
		tokens = append(tokens, token{kind: kind, raw: raw})
	}
	return tokens, nil
}

type parser struct {
	tokens []token
	pos    int
	now    time.Time
}

func (p *parser) parseOr(depth int) (*Expr, error) {
	left, err := p.parseAnd(depth)
	if err != nil {
		return nil, err
	}
	children := []*Expr{left}
	for p.peek(tokenOr) {
		p.pos++
		if p.pos >= len(p.tokens) {
			return nil, fmt.Errorf("%w: OR is missing its right operand", ErrSyntax)
		}
		right, err := p.parseAnd(depth)
		if err != nil {
			return nil, err
		}
		children = append(children, right)
	}
	return combine(OpOr, children), nil
}

func (p *parser) parseAnd(depth int) (*Expr, error) {
	if !p.startsUnary() {
		return nil, fmt.Errorf("%w near %q", ErrSyntax, p.currentRaw())
	}
	children := []*Expr{}
	for p.startsUnary() {
		child, err := p.parseUnary(depth)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return combine(OpAnd, children), nil
}

func (p *parser) parseUnary(depth int) (*Expr, error) {
	if p.peek(tokenNot) {
		p.pos++
		if !p.startsUnary() {
			return nil, fmt.Errorf("%w: - is missing its operand", ErrSyntax)
		}
		child, err := p.parseUnary(depth)
		if err != nil {
			return nil, err
		}
		return &Expr{Op: OpNot, Children: []*Expr{child}}, nil
	}
	return p.parsePrimary(depth)
}

func (p *parser) parsePrimary(depth int) (*Expr, error) {
	if p.peek(tokenLParen) {
		if depth >= MaxDepth {
			return nil, fmt.Errorf("%w: maximum is %d", ErrTooDeep, MaxDepth)
		}
		p.pos++
		if p.peek(tokenRParen) {
			return nil, fmt.Errorf("%w: empty parentheses", ErrSyntax)
		}
		expr, err := p.parseOr(depth + 1)
		if err != nil {
			return nil, err
		}
		if !p.peek(tokenRParen) {
			return nil, fmt.Errorf("%w: missing closing parenthesis", ErrSyntax)
		}
		p.pos++
		return expr, nil
	}
	if p.pos >= len(p.tokens) || p.tokens[p.pos].kind != tokenWord {
		return nil, fmt.Errorf("%w near %q", ErrSyntax, p.currentRaw())
	}
	raw := p.tokens[p.pos].raw
	p.pos++
	return p.parseLeaf(raw)
}

func (p *parser) parseLeaf(raw string) (*Expr, error) {
	op, value, isOp := splitOperator(raw)
	if !isOp {
		term := termFromToken(raw)
		term.Field = FieldText
		return termExpr(term), nil
	}
	field := strings.ToLower(op)
	term := termFromToken(value)
	switch field {
	case "title":
		term.Field = FieldTitle
	case "notebook", "category":
		if strings.EqualFold(term.Text, "All notes") {
			return &Expr{Op: OpMatchAll}, nil
		}
		term.Field = FieldNotebook
	case "collection":
		// Exact, and never a prefix: collection IDs are chosen at import time
		// and a prefix match would silently pull in `joplin-raw-2026-08` when
		// somebody asked for `joplin-raw-2026-07`.
		term.Field = FieldCollection
	case "tag":
		term.Field = FieldTag
	case "author":
		term.Field = FieldAuthor
	case "authorid":
		term.Field = FieldAuthorID
	case "since", "until":
		ts, ok := parseTimeBound(term.Text, p.now, field == "until")
		if !ok {
			return nil, fmt.Errorf("%w: invalid %s value %q", ErrSyntax, field, term.Text)
		}
		term.Text = ""
		term.Phrase = false
		term.UnixValue = ts
		if field == "since" {
			term.Field = FieldSince
		} else {
			term.Field = FieldUntil
		}
	case "is":
		if !strings.EqualFold(term.Text, "trashed") {
			term = termFromToken(raw)
			term.Field = FieldText
		} else {
			term = Term{Field: FieldTrashed}
		}
	default:
		term = termFromToken(raw)
		term.Field = FieldText
	}
	return termExpr(term), nil
}

func (p *parser) peek(kind tokenKind) bool {
	return p.pos < len(p.tokens) && p.tokens[p.pos].kind == kind
}

func (p *parser) startsUnary() bool {
	if p.pos >= len(p.tokens) {
		return false
	}
	kind := p.tokens[p.pos].kind
	return kind == tokenWord || kind == tokenNot || kind == tokenLParen
}

func (p *parser) currentRaw() string {
	if p.pos >= len(p.tokens) {
		return "end of query"
	}
	return p.tokens[p.pos].raw
}

func termExpr(term Term) *Expr {
	return &Expr{Op: OpTerm, Term: &term}
}

func combine(op Op, children []*Expr) *Expr {
	if len(children) == 1 {
		return children[0]
	}
	flat := make([]*Expr, 0, len(children))
	for _, child := range children {
		if child != nil && child.Op == op {
			flat = append(flat, child.Children...)
		} else {
			flat = append(flat, child)
		}
	}
	return &Expr{Op: op, Children: flat}
}

func extractTrashScope(expr *Expr, negated, underOr bool) (*Expr, bool, error) {
	if expr == nil {
		return expr, false, nil
	}
	if expr.Op == OpTerm && expr.Term != nil && expr.Term.Field == FieldTrashed {
		if negated || underOr {
			return nil, false, fmt.Errorf("%w: is:trashed must be a positive AND constraint", ErrSyntax)
		}
		return &Expr{Op: OpMatchAll}, true, nil
	}
	trashed := false
	for index, child := range expr.Children {
		rewritten, found, err := extractTrashScope(child, negated || expr.Op == OpNot, underOr || expr.Op == OpOr)
		if err != nil {
			return nil, false, err
		}
		expr.Children[index] = rewritten
		trashed = trashed || found
	}
	return expr, trashed, nil
}

func simplify(expr *Expr) *Expr {
	if expr == nil || expr.Op == OpTerm || expr.Op == OpMatchAll || expr.Op == OpMatchNone {
		return expr
	}
	for index, child := range expr.Children {
		expr.Children[index] = simplify(child)
	}
	if expr.Op == OpNot {
		child := expr.Children[0]
		if child.Op == OpMatchAll {
			return &Expr{Op: OpMatchNone}
		}
		if child.Op == OpMatchNone {
			return &Expr{Op: OpMatchAll}
		}
		return expr
	}
	children := make([]*Expr, 0, len(expr.Children))
	for _, child := range expr.Children {
		if expr.Op == OpAnd && child.Op == OpMatchNone || expr.Op == OpOr && child.Op == OpMatchAll {
			return child
		}
		if expr.Op == OpAnd && child.Op == OpMatchAll || expr.Op == OpOr && child.Op == OpMatchNone {
			continue
		}
		children = append(children, child)
	}
	if len(children) == 0 {
		if expr.Op == OpAnd {
			return &Expr{Op: OpMatchAll}
		}
		return &Expr{Op: OpMatchNone}
	}
	if len(children) == 1 {
		return children[0]
	}
	expr.Children = children
	return expr
}

// splitOperator recognizes letter-only op:value tokens. Quoted standalone
// terms and tokens with an empty value are not operators.
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
		if !isASCIIAlpha(r) {
			return "", "", false
		}
	}
	return op, token[idx+1:], true
}

func isASCIIAlpha(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func termFromToken(token string) Term {
	token = strings.TrimSpace(token)
	quoted := len(token) >= 2 && strings.HasPrefix(token, `"`) && strings.HasSuffix(token, `"`)
	if quoted {
		token = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(token, `"`), `"`))
	}
	return Term{Text: token, Phrase: quoted}
}

// parseTimeBound normalizes since:/until: to inclusive UTC Unix seconds.
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
