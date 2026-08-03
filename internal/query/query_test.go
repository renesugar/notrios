package query

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)

func mustParse(t *testing.T, input string) Query {
	t.Helper()
	q, err := Parse(input, now)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	return q
}

func terms(q Query) []Term {
	out := []Term{}
	walk(q.Root, func(expr *Expr) bool {
		if expr.Op == OpTerm && expr.Term != nil {
			out = append(out, *expr.Term)
		}
		return false
	})
	return out
}

func TestParseBooleanPrecedenceGroupingAndNegation(t *testing.T) {
	q := mustParse(t, `alpha OR beta gamma`)
	if q.Root.Op != OpOr || len(q.Root.Children) != 2 || q.Root.Children[1].Op != OpAnd {
		t.Fatalf("AND must bind more tightly than OR: %+v", q.Root)
	}

	q = mustParse(t, `(alpha OR beta) -tag:private "exact phrase"`)
	if q.Root.Op != OpAnd || len(q.Root.Children) != 3 || q.Root.Children[0].Op != OpOr || q.Root.Children[1].Op != OpNot {
		t.Fatalf("grouped expression wrong: %+v", q.Root)
	}
	gotTerms := terms(q)
	if len(gotTerms) != 4 || gotTerms[2].Field != FieldTag || gotTerms[2].Text != "private" ||
		gotTerms[3].Field != FieldText || !gotTerms[3].Phrase || gotTerms[3].Text != "exact phrase" {
		t.Fatalf("typed leaves wrong: %+v", gotTerms)
	}

	q = mustParse(t, `alpha or beta`)
	if q.Root.Op != OpAnd || len(terms(q)) != 3 {
		t.Fatalf("lowercase or must remain searchable text: %+v", q)
	}
}

func TestParseOperatorsAliasesAndLiteralFallbacks(t *testing.T) {
	q := mustParse(t, `apples title:"multiple words" notebook:"My Work" category:Archive tag:"shopping mall" author:"Alice Smith" authorid:alice@example.social re:invoice https://example.com/a_(b) well-known 😀`)
	got := terms(q)
	wantFields := []Field{FieldText, FieldTitle, FieldNotebook, FieldNotebook, FieldTag, FieldAuthor, FieldAuthorID, FieldText, FieldText, FieldText, FieldText}
	if len(got) != len(wantFields) {
		t.Fatalf("terms = %+v", got)
	}
	for index, field := range wantFields {
		if got[index].Field != field {
			t.Fatalf("term %d field = %q, want %q (%+v)", index, got[index].Field, field, got)
		}
	}
	if got[1].Text != "multiple words" || !got[1].Phrase || got[7].Text != "re:invoice" ||
		got[8].Text != "https://example.com/a_(b)" || got[9].Text != "well-known" || got[10].Text != "😀" {
		t.Fatalf("literal preservation wrong: %+v", got)
	}

	for _, input := range []string{`category:"All notes"`, `notebook:"all NOTES"`} {
		if parsed := mustParse(t, input); !parsed.IsEmpty() || parsed.Trashed {
			t.Fatalf("%q must simplify to All notes: %+v", input, parsed)
		}
	}
	q = mustParse(t, `category:"All notes" tag:todo`)
	if q.Root.Op != OpTerm || q.Root.Term.Field != FieldTag {
		t.Fatalf("All notes must remove only its notebook constraint: %+v", q)
	}
}

func TestParseTimeBoundsAndTrashScope(t *testing.T) {
	q := mustParse(t, "since:2026-07-01 until:2026-07-31")
	got := terms(q)
	wantSince := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix()
	wantUntil := time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC).Unix()
	if got[0].Field != FieldSince || got[0].UnixValue != wantSince || got[1].Field != FieldUntil || got[1].UnixValue != wantUntil {
		t.Fatalf("time bounds = %+v", got)
	}

	zone := time.FixedZone("UTC+2", 2*3600)
	localNow := time.Date(2026, 7, 15, 10, 0, 0, 0, zone)
	local, err := Parse("since:2026-07-15", localNow)
	if err != nil || terms(local)[0].UnixValue != time.Date(2026, 7, 15, 0, 0, 0, 0, zone).UTC().Unix() {
		t.Fatalf("timezone date bound = %+v, %v", local, err)
	}

	q = mustParse(t, `is:trashed lettuce`)
	if !q.Trashed || len(terms(q)) != 1 || terms(q)[0].Text != "lettuce" {
		t.Fatalf("trash scope = %+v", q)
	}
	for _, input := range []string{`-is:trashed`, `is:trashed OR lettuce`} {
		if _, err := Parse(input, now); !errors.Is(err, ErrSyntax) {
			t.Fatalf("Parse(%q) error = %v, want syntax error", input, err)
		}
	}
}

func TestParseBoundsAndSyntaxErrors(t *testing.T) {
	cases := []struct {
		input string
		want  error
	}{
		{strings.Repeat("x", MaxInputBytes+1), ErrTooLong},
		{strings.Repeat("x ", MaxTokens+1), ErrTooManyTokens},
		{strings.Repeat("(", MaxDepth+1) + "x" + strings.Repeat(")", MaxDepth+1), ErrTooDeep},
		{`"unterminated`, ErrSyntax},
		{`(alpha OR beta`, ErrSyntax},
		{`alpha OR`, ErrSyntax},
		{`since:not-a-date`, ErrSyntax},
	}
	for _, tc := range cases {
		if _, err := Parse(tc.input, now); !errors.Is(err, tc.want) {
			t.Errorf("Parse(%q) error = %v, want %v", tc.input, err, tc.want)
		}
	}
	if q := mustParse(t, ""); !q.IsEmpty() || q.HasTextTerms() || q.PositiveTextOnly() {
		t.Fatalf("empty query flags = %+v", q)
	}
}

func TestCanonicalExpressionIsStable(t *testing.T) {
	left := mustParse(t, `alpha   OR (beta tag:todo)`)
	right := mustParse(t, `alpha OR ( beta tag:todo )`)
	if left.Canonical() != right.Canonical() {
		t.Fatalf("equivalent tokenization must canonicalize equally:\n%s\n%s", left.Canonical(), right.Canonical())
	}
	if left.PositiveTextOnly() {
		t.Fatal("metadata expression must use the SQL predicate compiler")
	}
	if !mustParse(t, `alpha OR "beta gamma"`).PositiveTextOnly() {
		t.Fatal("positive text-only tree should retain FTS relevance")
	}
	if mustParse(t, `alpha -beta`).PositiveTextOnly() {
		t.Fatal("unary negation requires exact SQL predicates")
	}
	if !mustParse(t, `alpha -tag:private`).HasPositiveTextAnchor() {
		t.Fatal("mandatory text in a mixed AND should be an FTS anchor")
	}
	if mustParse(t, `alpha OR tag:todo`).HasPositiveTextAnchor() {
		t.Fatal("text in one OR branch cannot anchor the whole expression")
	}
}
