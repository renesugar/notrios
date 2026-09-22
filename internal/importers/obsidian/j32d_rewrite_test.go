package obsidian

import (
	"fmt"
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
		"nested wiki inside markdown": "See [label]([[document://default/documents/doc-target]]) here.\n",
		"nested embed and markdown":   "Nested ![[document://default/documents/doc-target]] inside [a]([[document://default/documents/doc-target]]).\n",
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
