package markdownlinks

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// referenceExtract is Extract written with the two regexps instead of the
// hand-written scanners, which is what this file exists to hold them to.
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
	for _, loc := range markdownLinkRE.FindAllStringSubmatchIndex(body, -1) {
		if len(loc) < 8 {
			continue
		}
		claimed = append(claimed, [2]int{loc[0], loc[1]})
		rawTarget := strings.TrimSpace(body[loc[6]:loc[7]])
		rawTarget = stripMarkdownTitle(rawTarget)
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

// TestScannersAgreeWithTheirRegexps holds the two matchers to the patterns they
// replace, offset for offset, on every body (v1.0 J32-Y).
func TestScannersAgreeWithTheirRegexps(t *testing.T) {
	for _, body := range j32yBodies() {
		var markdown []span
		scanMarkdownLinks(body, func(found span) { markdown = append(markdown, found) })
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

		var wiki []span
		scanWikiLinks(body, func(found span) { wiki = append(wiki, found) })
		want = wikiLinkRE.FindAllStringSubmatchIndex(body, -1)
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

// TestExtractMatchesTheRegexpExtract is the whole function held to its old
// self: every candidate, every field, in order.
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
