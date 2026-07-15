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
	seen := map[string]bool{}
	for _, loc := range markdownLinkRE.FindAllStringSubmatchIndex(body, -1) {
		if len(loc) < 8 {
			continue
		}
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
		decorateCandidate(body, &candidate)
		key := candidateKey(candidate)
		seen[key] = true
		matches = append(matches, candidate)
	}
	for _, loc := range wikiLinkRE.FindAllStringSubmatchIndex(body, -1) {
		if len(loc) < 6 {
			continue
		}
		key := body[loc[0]:loc[1]]
		if seen[key] {
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
		decorateCandidate(body, &candidate)
		matches = append(matches, candidate)
	}
	return matches
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

func decorateCandidate(body string, candidate *Candidate) {
	candidate.Line, candidate.Column = lineColumn(body, candidate.StartByte)
	candidate.Context = contextAround(body, candidate.StartByte, candidate.EndByte, 80)
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

func candidateKey(candidate Candidate) string {
	return candidate.SourceFormat + ":" + candidate.RawTarget + ":" + candidate.DisplayText
}
