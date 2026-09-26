package markdownlinks

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// referenceMarkdownLocs presents referenceMarkdownSpans in the shape
// FindAllStringSubmatchIndex used to return, so referenceExtract reads as it did.
func referenceMarkdownLocs(body string) [][]int {
	var locs [][]int
	for _, found := range referenceMarkdownSpans(body) {
		locs = append(locs, []int{found.start, found.end, found.bangStart, found.bangEnd,
			found.firstStart, found.firstEnd, found.targetStart, found.targetEnd})
	}
	return locs
}

// referenceExtract is Extract written against second implementations of its two
// scans, which is what this file exists to hold them to. The wiki scan is still
// held to its regexp; the Markdown scan outgrew one in J41.
//
// It carried the de-duplication that sat in Extract before J32-Y, keys and all,
// including the fact that they could never match. J37 decided what that
// de-duplication should have been — a wiki link overlapping a Markdown link is
// part of it, not a row of its own — so the reference implements that rule the
// slow way, scanning every claimed range per wiki match, while Extract walks
// both sequences with a single index. The old duplicate-row behaviour is not
// kept here: it is recorded in performance/v1.0-j32/README.md, and what replaced
// it is pinned in j37_nested_test.go.
func referenceExtract(body string) []Candidate {
	matches := []Candidate{}
	claimed := [][2]int{}
	cursor := newLineCursor(body)
	for _, loc := range referenceMarkdownLocs(body) {
		claimed = append(claimed, [2]int{loc[0], loc[1]})
		rawTarget := markdownTarget(body[loc[6]:loc[7]])
		candidate := Candidate{
			RelationType: relationFromBang(body[loc[2]:loc[3]]),
			SourceFormat: "markdown",
			DisplayText:  body[loc[4]:loc[5]],
			RawTarget:    rawTarget,
			StartByte:    loc[0],
			EndByte:      loc[1],
		}
		decorateCandidate(body, cursor, &candidate, literalHref(rawTarget))
		matches = append(matches, candidate)
	}
	cursor = newLineCursor(body)
	for _, loc := range wikiLinkRE.FindAllStringSubmatchIndex(body, -1) {
		if len(loc) < 6 {
			continue
		}
		overlapped := false
		for _, claim := range claimed {
			if claim[0] < loc[1] && loc[0] < claim[1] {
				overlapped = true
				break
			}
		}
		if overlapped {
			continue
		}
		inner := strings.TrimSpace(body[loc[4]:loc[5]])
		rawTarget, display := splitWikiTarget(inner)
		candidate := Candidate{
			RelationType: relationFromBang(body[loc[2]:loc[3]]),
			SourceFormat: "obsidian-wikilink",
			DisplayText:  display,
			RawTarget:    rawTarget,
			StartByte:    loc[0],
			EndByte:      loc[1],
		}
		decorateCandidate(body, cursor, &candidate, false)
		matches = append(matches, candidate)
	}
	return matches
}

// referenceMarkdownSpans is a second implementation of what scanMarkdownLinks
// finds, written differently on purpose: it collects the unescaped parentheses
// of a destination first and resolves the closing one from that list, where the
// scanner decides while walking. Two implementations of the same small rule,
// compared over every generated body, is what this file did with a regexp until
// J41 — and a regexp can no longer express the rule, because RE2 has no
// recursion and the rule counts depth.
func referenceMarkdownSpans(body string) []span {
	var spans []span
	resume := 0
	for open := 0; open < len(body); open++ {
		if open < resume || body[open] != '[' {
			continue
		}
		text := -1
		for index := open + 1; index < len(body); index++ {
			if body[index] == '\n' {
				break
			}
			if body[index] == ']' {
				text = index
				break
			}
		}
		if text < 0 || text+1 >= len(body) || body[text+1] != '(' {
			continue
		}
		end := -1
		if text+2 < len(body) && body[text+2] == '<' {
			if bracket, ok := referenceAngleClose(body, text+2); ok {
				if after, ok := referenceCloseParen(body, bracket+1); ok {
					end = after
				}
			}
		}
		if end < 0 {
			if after, ok := referenceCloseParen(body, text+2); ok {
				end = after
			}
		}
		if end < 0 || end == text+2 {
			continue
		}
		found := span{
			start: open, end: end + 1,
			bangStart: open, bangEnd: open,
			firstStart: open + 1, firstEnd: text,
			targetStart: text + 2, targetEnd: end,
		}
		if open > 0 && body[open-1] == '!' && open-1 >= resume {
			found.start, found.bangStart = open-1, open-1
		}
		spans = append(spans, found)
		resume = found.end
	}
	return spans
}

// referenceCloseParen resolves the closing parenthesis by collecting a
// destination's unescaped parentheses first and walking that list, where the
// scanner decides while walking the bytes.
func referenceCloseParen(body string, from int) (int, bool) {
	type paren struct {
		at   int
		open bool
	}
	var parens []paren
	for index := from; index < len(body); index++ {
		if body[index] == '\n' {
			break
		}
		if body[index] == '\\' {
			if index+1 < len(body) && body[index+1] == '\n' {
				break
			}
			index++
			continue
		}
		if body[index] == '(' {
			parens = append(parens, paren{index, true})
		}
		if body[index] == ')' {
			parens = append(parens, paren{index, false})
		}
	}
	depth, first := 0, -1
	for _, found := range parens {
		if found.open {
			depth++
			continue
		}
		if first < 0 {
			first = found.at
		}
		if depth == 0 {
			return found.at, true
		}
		depth--
	}
	if first < 0 {
		return 0, false
	}
	return first, true
}

// referenceAngleClose finds the `>` that closes a bracketed destination by
// jumping between the bytes that can matter, where angleClose examines each one.
func referenceAngleClose(text string, open int) (int, bool) {
	for index := open + 1; index < len(text); {
		next := strings.IndexAny(text[index:], "\n<>\\")
		if next < 0 {
			return 0, false
		}
		at := index + next
		switch text[at] {
		case '\n', '<':
			return 0, false
		case '>':
			return at, true
		default:
			index = at + 2
		}
	}
	return 0, false
}

// j32yBodies covers what the two patterns can meet: brackets that do not close,
// nested and repeated brackets, a bang in every position, links split across
// lines, empty text and empty targets, titles, pipes, anchors, and multi-byte
// text.
func j32yBodies() []string {
	bodies := []string{
		"", "[", "]", "[]", "[]()", "[a]()", "[](x)", "[a](x)", "![a](x)", "!![a](x)",
		"[a](x) [b](y)", "[a][b](x)", "[[a]]", "![[a]]", "[[]]", "[[a|b]]", "[[a#h]]",
		"[[[a]]]", "[[a]b]]", "[a]([[b]])", "[[a]](x)", "text [a](x) text [[b]] text",
		"[a\n](x)", "[a](x\ny)", "[a](  spaced  )", "[a](x \"title\")", "[a](<x>)",
		"[ünïcode](ελληνικά)", "[[ünïcode|ελληνικά]]", "![[a#^block]]",
		"[a](x)[[b]]", "[[b]][a](x)", "!", "!!", "![", "![[", "![[]]",
		"a [b] c (d) e", "[](  )", "[a](())", "[a](b(c))", "[[a]] [[a]] [[a]]",
		"[same](x) [same](x)", "\n\n[a](x)\n\n", "[a](x", "a](x)", "[[a", "a]]",
	}
	pieces := []string{"[", "]", "(", ")", "!", "a", " ", "\n", "|", "#", "[[", "]]", "](", "ü", "ελ"}
	random := rand.New(rand.NewSource(32))
	for attempt := 0; attempt < 20000; attempt++ {
		var builder strings.Builder
		for length := random.Intn(12); length >= 0; length-- {
			builder.WriteString(pieces[random.Intn(len(pieces))])
		}
		bodies = append(bodies, builder.String())
	}
	return bodies
}

// TestScannersAgreeWithTheirRegexps holds the two matchers to their references,
// offset for offset, on every body (v1.0 J32-Y, J41).
func TestScannersAgreeWithTheirRegexps(t *testing.T) {
	for _, body := range j32yBodies() {
		var markdown []span
		scanMarkdownLinks(body, func(found span) { markdown = append(markdown, found) })
		if reference := referenceMarkdownSpans(body); !reflect.DeepEqual(markdown, reference) {
			t.Fatalf("markdown in %q: scanner %+v, reference %+v", body, markdown, reference)
		}
		// Where no destination carries a parenthesis, the rule J41 added cannot
		// have applied, and the pattern this scan replaced must still agree
		// exactly. That keeps the old pattern a live cross-check for the shapes
		// it can still express.
		parenFree := true
		for _, found := range markdown {
			if strings.ContainsAny(body[found.targetStart:found.targetEnd], "()\\") {
				parenFree = false
				break
			}
		}
		if parenFree {
			want := markdownLinkRE.FindAllStringSubmatchIndex(body, -1)
			if len(markdown) != len(want) {
				t.Fatalf("markdown in %q: scanner found %d, regexp found %d", body, len(markdown), len(want))
			}
			for index, found := range markdown {
				loc := want[index]
				if found.start != loc[0] || found.end != loc[1] ||
					found.bangStart != loc[2] || found.bangEnd != loc[3] ||
					found.firstStart != loc[4] || found.firstEnd != loc[5] ||
					found.targetStart != loc[6] || found.targetEnd != loc[7] {
					t.Fatalf("markdown %d in %q: scanner %+v, regexp %v", index, body, found, loc)
				}
			}
		}

		var wiki []span
		scanWikiLinks(body, func(found span) { wiki = append(wiki, found) })
		want := wikiLinkRE.FindAllStringSubmatchIndex(body, -1)
		if len(wiki) != len(want) {
			t.Fatalf("wiki in %q: scanner found %d, regexp found %d", body, len(wiki), len(want))
		}
		for index, found := range wiki {
			loc := want[index]
			if found.start != loc[0] || found.end != loc[1] ||
				found.bangStart != loc[2] || found.bangEnd != loc[3] ||
				found.firstStart != loc[4] || found.firstEnd != loc[5] {
				t.Fatalf("wiki %d in %q: scanner %+v, regexp %v", index, body, found, loc)
			}
		}
	}
}

// TestExtractMatchesTheRegexpExtract is the whole function held to the reference
// implementations: every candidate, every field, in order.
func TestExtractMatchesTheRegexpExtract(t *testing.T) {
	for _, body := range j32yBodies() {
		got, want := Extract(body), referenceExtract(body)
		if !reflect.DeepEqual(got, want) {
			if len(got) != len(want) {
				t.Fatalf("%q: got %d candidates, want %d", body, len(got), len(want))
			}
			for index := range want {
				if got[index] != want[index] {
					t.Fatalf("%q candidate %d:\n got %+v\nwant %+v", body, index, got[index], want[index])
				}
			}
		}
	}
}

func BenchmarkExtractLinkDenseNote(b *testing.B) {
	var builder strings.Builder
	for index := 0; index < 20_000; index++ {
		fmt.Fprintf(&builder, "paragraph %d with [[Target-%d]] and [text](Other-%d.md) plus ελληνικά\n", index, index, index)
	}
	body := builder.String()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		if len(Extract(body)) == 0 {
			b.Fatal("no candidates")
		}
	}
}

func BenchmarkExtractLinkDenseNoteReference(b *testing.B) {
	var builder strings.Builder
	for index := 0; index < 20_000; index++ {
		fmt.Fprintf(&builder, "paragraph %d with [[Target-%d]] and [text](Other-%d.md) plus ελληνικά\n", index, index, index)
	}
	body := builder.String()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		if len(referenceExtract(body)) == 0 {
			b.Fatal("no candidates")
		}
	}
}
