package obsidian

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// The bodies J32-D pins before the assembly changes (v1.0 J32-D). They cover
// what the splice loop's order decides: a wiki link nested inside a Markdown
// link, replacements that touch, the same link repeated, a link at the very
// start and end of a body, multi-byte text around a link, and a link that does
// not resolve and so is left alone.
var j32dRewriteCases = []struct {
	name string
	body string
}{
	{"nested wiki inside markdown", "See [label]([[Target]]) here.\n"},
	{"nested embed and markdown", "Nested ![[Target]] inside [a]([[Target]]).\n"},
	{"adjacent", "[[Target]][[Other]]\n"},
	{"repeated", "A [[Target]] and [[Target]] and [[Target]].\n"},
	{"at both ends", "[[Target]] middle [[Other]]"},
	{"multi-byte around", "ünïcode ελληνικά [[Target]] ελληνικά ünïcode\n"},
	{"markdown link to a note", "Text [shown](Target.md) text.\n"},
	{"unresolved stays", "Missing [[Nowhere]] here.\n"},
	{"display text kept", "A [[Target|shown text]] here.\n"},
	{"anchors", "See [[Target#Heading]] and [[Target#^block]].\n"},
}

func j32dNamespace() linkNamespace {
	return linkNamespace{
		notesByPath:  map[string]string{"target": "doc-target", "other": "doc-other"},
		notesByName:  map[string][]string{"target": {"doc-target"}, "other": {"doc-other"}},
		assetsByPath: map[string]string{},
		assetsByBase: map[string][]string{},
	}
}

// TestRewriteObsidianLinksPinned states, byte for byte, what today's assembly
// writes. J32-D changes how the body is assembled and nothing else, so every
// one of these must still hold afterwards.
func TestRewriteObsidianLinksPinned(t *testing.T) {
	want := map[string]string{
		// J37 changed these two by decision, not by accident. A wiki link
		// written where a Markdown link's href belongs is a literal href, so it
		// is not a link to rewrite; `[label]([[Target]])` is a mistake for
		// `[[Target|label]]`, and Obsidian reads it the same way. The top-level
		// embed in the second case is still rewritten, which is the point of
		// keeping both in one body.
		"nested wiki inside markdown": "See [label]([[Target]]) here.\n",
		"nested embed and markdown":   "Nested ![[document://default/documents/doc-target]] inside [a]([[Target]]).\n",
		"adjacent":                    "[[document://default/documents/doc-target]][[document://default/documents/doc-other]]\n",
		"repeated":                    "A [[document://default/documents/doc-target]] and [[document://default/documents/doc-target]] and [[document://default/documents/doc-target]].\n",
		"at both ends":                "[[document://default/documents/doc-target]] middle [[document://default/documents/doc-other]]",
		"multi-byte around":           "ünïcode ελληνικά [[document://default/documents/doc-target]] ελληνικά ünïcode\n",
		"markdown link to a note":     "Text [shown](document://default/documents/doc-target) text.\n",
		"unresolved stays":            "Missing [[Nowhere]] here.\n",
		"display text kept":           "A [[document://default/documents/doc-target|shown text]] here.\n",
		"anchors":                     "See [[document://default/documents/doc-target#Heading]] and [[document://default/documents/doc-target#^block]].\n",
	}
	for _, testCase := range j32dRewriteCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, _, count, warnings := rewriteObsidianLinks(testCase.body, "Note.md", "default", j32dNamespace())
			if expected, ok := want[testCase.name]; ok && got != expected {
				t.Errorf("body:\n got %q\nwant %q", got, expected)
			}
			if len(warnings) != 0 {
				t.Errorf("unexpected warnings: %v", warnings)
			}
			if count != strings.Count(got, "document://") {
				t.Errorf("replacement count %d, body holds %d rewritten targets", count, strings.Count(got, "document://"))
			}
		})
	}
}

// TestRewriteObsidianLinksManyLinks pins a body with many links, which is what
// the one-pass assembly is for: the result must be the same as the splice
// loop's, at a size where an off-by-one in the pass would show.
func TestRewriteObsidianLinksManyLinks(t *testing.T) {
	parts := make([]string, 0, 400)
	for index := 0; index < 200; index++ {
		parts = append(parts, fmt.Sprintf("line %d [[Target]] and [[Other]] ελληνικά\n", index))
	}
	body := strings.Join(parts, "")
	got, _, count, warnings := rewriteObsidianLinks(body, "Note.md", "default", j32dNamespace())
	if count != 400 || len(warnings) != 0 {
		t.Fatalf("count %d, warnings %v", count, warnings)
	}
	if strings.Count(got, "document://default/documents/doc-target") != 200 ||
		strings.Count(got, "document://default/documents/doc-other") != 200 {
		t.Fatal("not every link was rewritten")
	}
	for index := 0; index < 200; index++ {
		want := fmt.Sprintf("line %d [[document://default/documents/doc-target]] and [[document://default/documents/doc-other]] ελληνικά", index)
		if !strings.Contains(got, want) {
			t.Fatalf("line %d reads %q", index, want)
		}
	}
}

// TestApplyReplacementsSameStartIsDecided covers the one case the splice loop
// did not decide: two replacements starting at the same byte were ordered
// arbitrarily by its sort, so there is nothing to reproduce. The widest is
// applied first, and this says so (v1.0 J32-D).
func TestApplyReplacementsSameStartIsDecided(t *testing.T) {
	body := "zero [[One]] two ![[Three]] four"
	got := applyReplacements(body, []replacement{{5, 12, "<inner>"}, {5, 27, "<outer>"}})
	if want := "zero <inner> four"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// spliceReplacements is what applyReplacements replaced: the descending splice
// loop, kept here so the one-pass assembly can be held to it.
func spliceReplacements(body string, replacements []replacement) string {
	sorted := append([]replacement(nil), replacements...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start > sorted[j].start })
	for _, change := range sorted {
		body = body[:change.start] + change.text + body[change.end:]
	}
	return body
}

// TestApplyReplacementsMatchesTheSpliceLoop holds the one-pass assembly to the
// loop it replaced, including the overlapping case it deliberately hands back
// to that loop (v1.0 J32-D).
func TestApplyReplacementsMatchesTheSpliceLoop(t *testing.T) {
	body := "zero [[One]] two ![[Three]] four [five](six.md) seven ελληνικά [[Eight]] nine"
	cases := []struct {
		name         string
		replacements []replacement
	}{
		{"none", nil},
		{"one", []replacement{{5, 12, "<1>"}}},
		{"several, given out of order", []replacement{
			{33, 47, "<md>"}, {5, 12, "<1>"}, {62, 71, "<8>"}, {17, 27, "<3>"},
		}},
		{"adjacent", []replacement{{5, 12, "<1>"}, {12, 17, "<gap>"}}},
		{"whole body", []replacement{{0, len(body), "<all>"}}},
		{"empty text", []replacement{{5, 12, ""}}},
		{"overlapping", []replacement{{5, 27, "<outer>"}, {17, 27, "<inner>"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			want := spliceReplacements(body, testCase.replacements)
			got := applyReplacements(body, append([]replacement(nil), testCase.replacements...))
			if got != want {
				t.Errorf("one pass wrote %q, the splice loop writes %q", got, want)
			}
		})
	}
}
