package markdownblocks

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// j32sLines covers what the three matchers have to agree with the regexps
// about: markers of every shape, indentation with each whitespace byte, digits
// with both terminators, near-misses, and lines that are only whitespace.
func j32sLines() []string {
	lines := []string{
		"", " ", "\t", "\t \f\r", "-", "- ", "-\titem", "- item", "  - item",
		"* item", "+ item", "*item", "+", "1. item", "1) item", "12. item",
		"1.item", "1 item", "0. zero", "999) many", "1.", "1)", "-  двойной",
		"#", "# ", "# Heading", "###### Six", "####### Seven", "#NoSpace",
		"## \tTabbed heading", "  # Indented heading", "#\tTab after hash",
		"| a | b |", "|a|", "| a | b | ", "  | a |  ", "a | b |", "| a | b",
		"|", "||", " || ", "|x", "x|", "text", "  text  ", "text with | pipe",
		"- [ ] task", "* **bold** item", "1. 2. nested numbers", "-- not a list",
		"ελληνικά", "- ελληνικά", "# ελληνικά ünïcode",
	}
	random := rand.New(rand.NewSource(32))
	alphabet := []string{"-", "*", "+", "#", "|", " ", "\t", "1", ".", ")", "a", "\f", "\r"}
	for attempt := 0; attempt < 4000; attempt++ {
		var builder strings.Builder
		for length := random.Intn(8); length >= 0; length-- {
			builder.WriteString(alphabet[random.Intn(len(alphabet))])
		}
		lines = append(lines, builder.String())
	}
	return lines
}

// TestMatchersAgreeWithTheirRegexps holds each hand-written matcher to the
// regexp it replaced, on every line: same decision, same text (v1.0 J32-S).
func TestMatchersAgreeWithTheirRegexps(t *testing.T) {
	for _, line := range j32sLines() {
		t.Run(fmt.Sprintf("%q", line), func(t *testing.T) {
			markerEnd, isItem := matchListMarker(line)
			if want := listItemRE.MatchString(line); isItem != want {
				t.Fatalf("list item: matcher says %v, regexp says %v", isItem, want)
			}
			if isItem {
				if got, want := line[markerEnd:], listItemRE.ReplaceAllString(line, ""); got != want {
					t.Errorf("list item text: matcher %q, regexp %q", got, want)
				}
			}

			level, headingText, isHeading := matchHeading(line)
			match := headingRE.FindStringSubmatch(line)
			if isHeading != (match != nil) {
				t.Fatalf("heading: matcher says %v, regexp says %v", isHeading, match != nil)
			}
			if isHeading {
				if level != len(match[1]) || headingText != match[2] {
					t.Errorf("heading: matcher %d/%q, regexp %d/%q", level, headingText, len(match[1]), match[2])
				}
			}

			if got, want := matchTableRow(line), tableRowRE.MatchString(line); got != want {
				t.Errorf("table row: matcher says %v, regexp says %v", got, want)
			}
		})
	}
}

// TestSlugifyUnchanged pins slugs across the shapes that exercise lowercasing,
// Unicode, hyphen collapsing and the length limit, which J32-S rewrote.
func TestSlugifyUnchanged(t *testing.T) {
	cases := map[string]string{
		"Simple Heading":           "simple-heading",
		"  Padded  ":               "padded",
		"Multiple   Spaces":        "multiple-spaces",
		"Hyphen-Already":           "hyphen-already",
		"Under_score":              "under-score",
		"ΕΛΛΗΝΙΚΆ Κείμενο":         "ελληνικά-κείμενο",
		"ÜNÏCODE Heading":          "ünïcode-heading",
		"123 Numbers":              "123-numbers",
		"!!!":                      "",
		"--leading and trailing--": "leading-and-trailing",
		"Tabs\tand spaces":         "tabs-and-spaces",
		strings.Repeat("x", 200):   strings.Repeat("x", MaxSlugBytes),
	}
	for text, want := range cases {
		if got := Slugify(text); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", text, got, want)
		}
	}
}
