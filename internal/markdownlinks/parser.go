// Package markdownlinks extracts a conservative MVP set of links from Markdown.
// It intentionally favors deterministic, easy-to-test behavior over full
// CommonMark coverage. A later task can replace it with a unified/remark-style
// AST parser without changing the store/API link model.
package markdownlinks

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// The two matchers below replace the regexps they are named after, which a CPU
// profile put at 256.8 s of a 516 s import -- 89.6% of all samples were regexp
// matching (v1.0 J32-Y). The regexps stay in the package as the reference the
// tests hold these to, body for body, so what is recognised cannot drift.
//
// Neither pattern needs a regexp engine: both character classes exclude the
// terminator that ends them, so each match is found by scanning forward once,
// with no backtracking and no alternatives to weigh.
//
// Each match is handed to a visitor as it is found, rather than collected into
// a slice first: a slice that grows once per note cost 0.7% more allocated
// bytes than the regexps did on a corpus of small notes (measured, v1.0 J32-Y).
//
// span is one match and its groups, holding the byte offsets
// FindAllStringSubmatchIndex would have reported.
type span struct {
	start, end           int
	bangStart, bangEnd   int
	firstStart, firstEnd int // a Markdown link's text, a wiki link's inner
	targetStart          int // a Markdown link's target; zero for a wiki link
	targetEnd            int
}

// scanMarkdownLinks finds `(!?)\[([^\]\n]*)\]\(([^)\n]+)\)`, leftmost first
// and non-overlapping, as FindAllStringSubmatchIndex does.
//
// A `!` belongs to a match only when it sits at or after the point the scan
// resumed from: one already inside the previous match was consumed by it.
func scanMarkdownLinks(body string, visit func(span)) {
	resume := 0
	for index := resume; index < len(body); {
		offset := strings.IndexByte(body[index:], '[')
		if offset < 0 {
			break
		}
		open := index + offset
		text := open + 1
		for text < len(body) && body[text] != ']' && body[text] != '\n' {
			text++
		}
		if text >= len(body) || body[text] != ']' || text+1 >= len(body) || body[text+1] != '(' {
			index = open + 1
			continue
		}
		target := text + 2
		for target < len(body) && body[target] != ')' && body[target] != '\n' {
			target++
		}
		if target >= len(body) || body[target] != ')' || target == text+2 {
			index = open + 1
			continue
		}
		found := span{
			start: open, end: target + 1,
			bangStart: open, bangEnd: open,
			firstStart: open + 1, firstEnd: text,
			targetStart: text + 2, targetEnd: target,
		}
		if open > resume-1 && open > 0 && body[open-1] == '!' && open-1 >= resume {
			found.start, found.bangStart = open-1, open-1
		}
		visit(found)
		resume = found.end
		index = resume
	}
}

// scanWikiLinks finds `(!?)\[\[([^\]\n]+)\]\]` on the same terms.
func scanWikiLinks(body string, visit func(span)) {
	resume := 0
	for index := resume; index+1 < len(body); {
		offset := strings.Index(body[index:], "[[")
		if offset < 0 {
			break
		}
		open := index + offset
		inner := open + 2
		for inner < len(body) && body[inner] != ']' && body[inner] != '\n' {
			inner++
		}
		if inner == open+2 || inner+1 >= len(body) || body[inner] != ']' || body[inner+1] != ']' {
			index = open + 1
			continue
		}
		found := span{
			start: open, end: inner + 2,
			bangStart: open, bangEnd: open,
			firstStart: open + 2, firstEnd: inner,
		}
		if open > 0 && body[open-1] == '!' && open-1 >= resume {
			found.start, found.bangStart = open-1, open-1
		}
		visit(found)
		resume = found.end
		index = resume
	}
}

// Candidate is one raw link-like object extracted from Markdown source.
type Candidate struct {
	RelationType string
	SourceFormat string
	RawTarget    string
	DisplayText  string
	AnchorType   string
	AnchorValue  string
	Context      string
	StartByte    int
	EndByte      int
	Line         int
	Column       int
}

var (
	markdownLinkRE = regexp.MustCompile(`(!?)\[([^\]\n]*)\]\(([^)\n]+)\)`)
	wikiLinkRE     = regexp.MustCompile(`(!?)\[\[([^\]\n]+)\]\]`)
)

// Extract returns Markdown inline links/images plus Obsidian wikilinks/embeds.
func Extract(body string) []Candidate {
	matches := []Candidate{}
	// Where the Markdown links sit, in ascending order because the scan is
	// leftmost-first and non-overlapping. The wiki pass walks them with one
	// index rather than searching, so this costs one small slice per note and
	// no work per link.
	claimed := []byteRange{}
	// One cursor per pass: each pass reports its matches in ascending order,
	// and the second starts again at the beginning of the body.
	cursor := newLineCursor(body)
	scanMarkdownLinks(body, func(found span) {
		claimed = append(claimed, byteRange{start: found.start, end: found.end})
		rawTarget := strings.TrimSpace(body[found.targetStart:found.targetEnd])
		rawTarget = stripMarkdownTitle(rawTarget)
		candidate := Candidate{
			RelationType: relationFromBang(body[found.bangStart:found.bangEnd]),
			SourceFormat: "markdown",
			DisplayText:  body[found.firstStart:found.firstEnd],
			RawTarget:    rawTarget,
			StartByte:    found.start,
			EndByte:      found.end,
		}
		decorateCandidate(body, cursor, &candidate, literalHref(rawTarget))
		matches = append(matches, candidate)
	})
	cursor = newLineCursor(body)
	// What sat here was a de-duplication that could never fire: the Markdown
	// pass stored keys of the form "markdown:target:display" while this pass
	// looked up the raw matched text, so no key ever matched, and every link
	// both passes found was reported twice (v1.0 J32-Y). J37 decided what
	// should happen instead: a wiki link inside a Markdown link's target is a
	// literal href, so only the Markdown row survives. `[text]([[Target]])` is
	// a mistake for `[[Target|text]]`, and Obsidian reads it the same way —
	// the brackets are part of the destination, and no backlink to Target is
	// made. A `[[Target]]` anywhere else is still a wiki link.
	claim := 0
	scanWikiLinks(body, func(found span) {
		// Both sequences ascend and neither overlaps itself, so one index walks
		// them together. Overlap rather than containment is the test: a body
		// like `[a](x[[b)]]` has a wiki match that starts inside the Markdown
		// link and ends past it, and keeping both would leave two rows over the
		// same bytes — the thing this decision exists to prevent. Dropping every
		// wiki match that overlaps a Markdown link makes what remains provably
		// disjoint.
		for claim < len(claimed) && claimed[claim].end <= found.start {
			claim++
		}
		if claim < len(claimed) && claimed[claim].start < found.end {
			return
		}
		inner := strings.TrimSpace(body[found.firstStart:found.firstEnd])
		rawTarget, display := splitWikiTarget(inner)
		candidate := Candidate{
			RelationType: relationFromBang(body[found.bangStart:found.bangEnd]),
			SourceFormat: "obsidian-wikilink",
			DisplayText:  display,
			RawTarget:    rawTarget,
			StartByte:    found.start,
			EndByte:      found.end,
		}
		decorateCandidate(body, cursor, &candidate, false)
		matches = append(matches, candidate)
	})
	return matches
}

// byteRange is a half-open span of the body.
type byteRange struct{ start, end int }

// literalHref reports whether a Markdown link's target is a wiki link written
// where an href belongs. Such a target names nothing, so it keeps every byte it
// was written with: splitting an anchor off `[[Target#heading]]` would record
// the target as `[[Target` and invent the heading `heading]]`, neither of which
// appears in the note (v1.0 J37).
func literalHref(rawTarget string) bool {
	return strings.HasPrefix(rawTarget, "[[")
}

func relationFromBang(bang string) string {
	if bang == "!" {
		return "embed"
	}
	return "link"
}

func stripMarkdownTitle(target string) string {
	// Basic Markdown links may include an optional title after whitespace.
	// Keep URIs with escaped spaces for later parser hardening, but support the
	// common [label](target "title") form here.
	if i := strings.IndexAny(target, " \t"); i > 0 {
		return strings.TrimSpace(target[:i])
	}
	return target
}

func splitWikiTarget(inner string) (string, string) {
	parts := strings.SplitN(inner, "|", 2)
	rawTarget := strings.TrimSpace(parts[0])
	display := rawTarget
	if len(parts) == 2 {
		display = strings.TrimSpace(parts[1])
	}
	return rawTarget, display
}

func decorateCandidate(body string, cursor *lineCursor, candidate *Candidate, literal bool) {
	candidate.Line, candidate.Column = cursor.at(candidate.StartByte)
	candidate.Context = contextAround(body, candidate.StartByte, candidate.EndByte, 80)
	if literal {
		return
	}
	base, anchorType, anchorValue := splitAnchor(candidate.RawTarget)
	candidate.RawTarget = base
	candidate.AnchorType = anchorType
	candidate.AnchorValue = anchorValue
}

func splitAnchor(raw string) (base string, anchorType string, anchorValue string) {
	idx := strings.Index(raw, "#")
	if idx < 0 {
		return raw, "", ""
	}
	base = raw[:idx]
	anchor := raw[idx+1:]
	if strings.HasPrefix(anchor, "^") {
		return base, "block", strings.TrimPrefix(anchor, "^")
	}
	if anchor != "" {
		return base, "heading", anchor
	}
	return base, "", ""
}

// lineColumn is what lineCursor must agree with: the straightforward reading,
// kept as the reference the tests compare against.
func lineColumn(body string, offset int) (line int, column int) {
	line = 1
	column = 1
	for i, r := range body {
		if i >= offset {
			break
		}
		if r == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

// lineCursor answers the same question as lineColumn, walking forward instead
// of restarting (v1.0 J32-E).
//
// Extract asks for coordinates in ascending order within each pass, so the
// bytes between one candidate and the next are all that a cursor has to read.
// Restarting at byte zero every time made a note with thousands of links
// decode itself thousands of times, and a note that is one long line is the
// worst case, because a column counts runes since the last newline.
//
// Asked for an offset behind it, it starts over, so an answer never depends on
// what was asked before it.
type lineCursor struct {
	body   string
	offset int
	line   int
	column int
}

func newLineCursor(body string) *lineCursor {
	return &lineCursor{body: body, line: 1, column: 1}
}

func (c *lineCursor) at(offset int) (int, int) {
	if offset < c.offset {
		c.offset, c.line, c.column = 0, 1, 1
	}
	for index, r := range c.body[c.offset:] {
		absolute := c.offset + index
		if absolute >= offset {
			c.offset = absolute
			return c.line, c.column
		}
		if r == '\n' {
			c.line++
			c.column = 1
			continue
		}
		c.column++
	}
	c.offset = len(c.body)
	return c.line, c.column
}

func contextAround(body string, start, end, radius int) string {
	left := start - radius
	if left < 0 {
		left = 0
	}
	right := end + radius
	if right > len(body) {
		right = len(body)
	}
	for left > 0 && !utf8.RuneStart(body[left]) {
		left--
	}
	for right < len(body) && !utf8.RuneStart(body[right]) {
		right++
	}
	return strings.TrimSpace(body[left:right])
}
