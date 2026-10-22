package markdownlinks

import "testing"

// TestJ37NestedWikiLinkIsAnHref pins J37-B's decision: a wiki link written where
// a Markdown link's href belongs is a literal href, so only the outer row
// survives, and the target keeps every byte it was written with.
func TestJ37NestedWikiLinkIsAnHref(t *testing.T) {
	type want struct {
		format, relation, raw, display, anchorType, anchorValue, matched string
	}
	cases := []struct {
		name string
		body string
		want []want
	}{
		{
			name: "a wiki link in a link's target is one literal href",
			body: "See [text]([[Target]]) here.\n",
			want: []want{{"markdown", "link", "[[Target]]", "text", "", "", "[text]([[Target]])"}},
		},
		{
			name: "a wiki link in an embed's target is one literal href",
			body: "See ![text]([[Target]]) here.\n",
			want: []want{{"markdown", "embed", "[[Target]]", "text", "", "", "![text]([[Target]])"}},
		},
		{
			// The artifact J37-A found: splitting this would record the target
			// as "[[Target" and invent the heading "heading]]".
			name: "a literal href keeps its anchor bytes",
			body: "See [text]([[Target#heading]]) here.\n",
			want: []want{{"markdown", "link", "[[Target#heading]]", "text", "", "", "[text]([[Target#heading]])"}},
		},
		{
			name: "a block anchor in a literal href is not split either",
			body: "See [text]([[Target#^block-a]]) here.\n",
			want: []want{{"markdown", "link", "[[Target#^block-a]]", "text", "", "", "[text]([[Target#^block-a]])"}},
		},
		{
			name: "a wiki link at the top level is still a wiki link",
			body: "See [[Target]] here.\n",
			want: []want{{"obsidian-wikilink", "link", "Target", "Target", "", "", "[[Target]]"}},
		},
		{
			name: "the pipe form this is a mistake for is untouched",
			body: "See [[Target|text]] here.\n",
			want: []want{{"obsidian-wikilink", "link", "Target", "text", "", "", "[[Target|text]]"}},
		},
		{
			name: "an ordinary Markdown link still splits its anchor",
			body: "See [text](Target#heading) here.\n",
			want: []want{{"markdown", "link", "Target", "text", "heading", "heading", "[text](Target#heading)"}},
		},
		{
			name: "a wiki link after a Markdown link is still reported",
			body: "See [text](Target) and [[Other]] here.\n",
			want: []want{
				{"markdown", "link", "Target", "text", "", "", "[text](Target)"},
				{"obsidian-wikilink", "link", "Other", "Other", "", "", "[[Other]]"},
			},
		},
		{
			name: "a wiki link before a Markdown link is still reported",
			body: "See [[Other]] and [text](Target) here.\n",
			want: []want{
				{"markdown", "link", "Target", "text", "", "", "[text](Target)"},
				{"obsidian-wikilink", "link", "Other", "Other", "", "", "[[Other]]"},
			},
		},
		{
			name: "one suppressed target does not suppress a later wiki link",
			body: "[a]([[One]]) then [[Two]] then [b]([[Three]]) then [[Four]].\n",
			want: []want{
				{"markdown", "link", "[[One]]", "a", "", "", "[a]([[One]])"},
				{"markdown", "link", "[[Three]]", "b", "", "", "[b]([[Three]])"},
				{"obsidian-wikilink", "link", "Two", "Two", "", "", "[[Two]]"},
				{"obsidian-wikilink", "link", "Four", "Four", "", "", "[[Four]]"},
			},
		},
		{
			name: "an embedded wiki link inside a link's target is still the href",
			body: "See [text](![[Target]]) here.\n",
			want: []want{{"markdown", "link", "![[Target]]", "text", "", "", "[text](![[Target]])"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Extract(testCase.body)
			if len(got) != len(testCase.want) {
				for _, candidate := range got {
					t.Logf("got %s raw=%q span=%q", candidate.SourceFormat, candidate.RawTarget,
						testCase.body[candidate.StartByte:candidate.EndByte])
				}
				t.Fatalf("got %d candidates, want %d", len(got), len(testCase.want))
			}
			for index, expected := range testCase.want {
				candidate := got[index]
				matched := testCase.body[candidate.StartByte:candidate.EndByte]
				actual := want{candidate.SourceFormat, candidate.RelationType, candidate.RawTarget,
					candidate.DisplayText, candidate.AnchorType, candidate.AnchorValue, matched}
				if actual != expected {
					t.Errorf("candidate %d:\n got %+v\nwant %+v", index, actual, expected)
				}
			}
		})
	}
}

// TestJ37NoTwoRowsOverlap is the constraint J37-B was written with: an
// overlapping rewrite is the case J32-D has to hand back to the splice loop, so
// no two surviving rows may cover the same bytes.
func TestJ37NoTwoRowsOverlap(t *testing.T) {
	bodies := []string{
		"See [text]([[Target]]) here.\n",
		"See ![text]([[Target#^b]]) and [[Other]] and [x](y) here.\n",
		"[a]([[One]]) [[Two]] [b]([[Three]]) [[Four]]\n",
		"![alt]([[Image.png]]) ![[Image.png]]\n",
		// The shape that made overlap rather than containment the test: the
		// wiki match starts inside the Markdown link and ends past it.
		"[a](x[[b)]]\n",
		"[a](x[[b)]] [[c]]\n",
	}
	// And every body the J32-Y reference test uses, 20,000 of them built from
	// the pieces the two patterns can meet, so the invariant is not resting on
	// the shapes I thought to write down.
	bodies = append(bodies, j32yBodies()...)
	for _, body := range bodies {
		candidates := Extract(body)
		for i := range candidates {
			for j := range candidates {
				if i >= j {
					continue
				}
				a, b := candidates[i], candidates[j]
				if a.StartByte < b.EndByte && b.StartByte < a.EndByte {
					t.Errorf("%q: %q [%d,%d) overlaps %q [%d,%d)", body,
						body[a.StartByte:a.EndByte], a.StartByte, a.EndByte,
						body[b.StartByte:b.EndByte], b.StartByte, b.EndByte)
				}
			}
		}
	}
}
