package joplinraw

import (
	"strings"
	"testing"
)

// TestJ43AJoplinKeepsTitles reads the Joplin rewriter for the defect J42-C found
// in the Obsidian one. It does not have it: this rewriter substitutes the `:/id`
// token where it stands and copies everything around it unchanged, so a title
// after the destination is never seen and never lost. The Obsidian rewriter
// rebuilds the link from its parts instead, which is why only that one drops it.
func TestJ43AJoplinKeepsTitles(t *testing.T) {
	const noteID = "0123456789abcdef0123456789abcdef"
	const resourceID = "fedcba9876543210fedcba9876543210"
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "a link with a title",
			line: "See [shown](:/" + noteID + " \"A title\") here.",
			want: "See [shown](document://default/documents/doc-note \"A title\") here.",
		},
		{
			name: "an embed with a title",
			line: "See ![shown](:/" + resourceID + " \"A title\") here.",
			want: "See ![shown](resource://default/resources/res-one \"A title\") here.",
		},
		{
			name: "a link with no title",
			line: "See [shown](:/" + noteID + ") here.",
			want: "See [shown](document://default/documents/doc-note) here.",
		},
		{
			name: "an unresolved id keeps the line as it was",
			line: "See [shown](:/ffffffffffffffffffffffffffffffff \"A title\") here.",
			want: "See [shown](:/ffffffffffffffffffffffffffffffff \"A title\") here.",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var output strings.Builder
			inlineCodeLength := 0
			result := linkRewriteResult{}
			resources := map[string]resourceReference{}
			rewriteJoplinLinkLine(&output, testCase.line, &inlineCodeLength,
				map[string]string{noteID: "doc-note"},
				map[string]string{resourceID: "res-one"},
				"default", &result, resources)
			if got := output.String(); got != testCase.want {
				t.Errorf("line:\n got %q\nwant %q", got, testCase.want)
			}
		})
	}
}
