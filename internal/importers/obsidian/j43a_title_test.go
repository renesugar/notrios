package obsidian

import "testing"

// TestJ43TitleSurvivesARewrite pins that a title comes back from a rewrite
// exactly as it was written. Before J43-B the replacement text was built from the
// display text and the resolved URI alone, so every row below lost whatever the
// author wrote between the quotes.
func TestJ43TitleSurvivesARewrite(t *testing.T) {
	const uri = "document://default/documents/doc-target"
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// Was `[shown](uri)`.
			name: "a link with a title",
			body: "See [shown](Target.md \"A title\") here.\n",
			want: "See [shown](" + uri + " \"A title\") here.\n",
		},
		{
			// Was `![shown](uri)`.
			name: "an embed with a title",
			body: "See ![shown](Target.md \"A title\") here.\n",
			want: "See ![shown](" + uri + " \"A title\") here.\n",
		},
		{
			// Was `[shown](uri)`. The brackets go, the title stays.
			name: "a bracketed destination with a title",
			body: "See [shown](<Target.md> \"A title\") here.\n",
			want: "See [shown](" + uri + " \"A title\") here.\n",
		},
		{
			// Was `[shown](uri)`. A title may be delimited three ways, which is
			// why the bytes are copied rather than parsed.
			name: "a title in single quotes",
			body: "See [shown](Target.md 'A title') here.\n",
			want: "See [shown](" + uri + " 'A title') here.\n",
		},
		{
			// Was `[shown](uri)`.
			name: "a title in parentheses",
			body: "See [shown](Target.md (A title)) here.\n",
			want: "See [shown](" + uri + " (A title)) here.\n",
		},
		{
			// A wiki link's display text is not a title, and was already carried.
			name: "a wiki link keeps its display text",
			body: "A [[Target|shown text]] here.\n",
			want: "A [[" + uri + "|shown text]] here.\n",
		},
		{
			// The guard: a link with no title is written byte for byte as before.
			name: "a link with no title",
			body: "Text [shown](Target.md) text.\n",
			want: "Text [shown](" + uri + ") text.\n",
		},
		{
			// An anchor and a title together, in that order.
			name: "an anchor and a title",
			body: "See [shown](Target.md#Heading \"A title\") here.\n",
			want: "See [shown](" + uri + "#Heading \"A title\") here.\n",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, _, _, _ := rewriteObsidianLinks(testCase.body, "Note.md", "default", j32dNamespace())
			if got != testCase.want {
				t.Errorf("body:\n got %q\nwant %q", got, testCase.want)
			}
		})
	}
}
