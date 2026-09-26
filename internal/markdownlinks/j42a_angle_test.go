package markdownlinks

import "testing"

// j42Cases states what a Markdown link's destination is taken to be when it is
// written in CommonMark's angle-bracketed form, as it is recorded today.
//
// That form exists so a destination can hold spaces, and it is what Obsidian's
// own documentation recommends for a note whose name has one —
// `[text](<Target Note>)`, alongside `[text](Target%20Note)`. Notrios cuts a
// target at the first space, because that is how the optional title after a
// destination is stripped, so the form reads as `<a` and resolves to nothing.
//
// Each row that is wrong is marked with what J42-B should make it, so the change
// can be seen to move those rows and no others.
var j42Cases = []j41Case{
	{
		// J42-B: should be `a b`.
		name:    "a destination with a space",
		body:    "See [text](<a b>) here.\n",
		raw:     "<a",
		matched: "[text](<a b>)",
	},
	{
		// J42-B: should be `My Note.md`, which is the case the form exists for.
		name:    "a note whose name has a space",
		body:    "See [text](<My Note.md>) here.\n",
		raw:     "<My",
		matched: "[text](<My Note.md>)",
	},
	{
		// J42-B: should be `a`. The brackets are not part of the destination.
		name:    "a bracketed destination with no space",
		body:    "See [text](<a>) here.\n",
		raw:     "<a>",
		matched: "[text](<a>)",
	},
	{
		// J42-B: should be `a`.
		name:    "a bracketed destination with a title",
		body:    "See [text](<a> \"title\") here.\n",
		raw:     "<a>",
		matched: "[text](<a> \"title\")",
	},
	{
		// J42-B: should be `a b`.
		name:    "a spaced destination with a title",
		body:    "See [text](<a b> \"title\") here.\n",
		raw:     "<a",
		matched: "[text](<a b> \"title\")",
	},
	{
		// J42-B: should be `a (b) c`. J41 already gave this row its whole span,
		// because the parentheses inside the brackets balance.
		name:    "a spaced destination with balanced parentheses",
		body:    "See [text](<a (b) c>) here.\n",
		raw:     "<a",
		matched: "[text](<a (b) c>)",
	},
	{
		// J42-B: should be `a ) b`, and the span should reach the real closing
		// parenthesis. Inside the brackets a parenthesis is an ordinary
		// character, so J41's depth rule must not end the link on it.
		name:    "an unbalanced parenthesis inside the brackets",
		body:    "See [text](<a ) b>) here.\n",
		raw:     "<a",
		matched: "[text](<a )",
	},
	{
		// J42-B: should be `a) b`, span `[text](<a) b>)`.
		name:    "a closing parenthesis immediately inside the brackets",
		body:    "See [text](<a) b>) here.\n",
		raw:     "<a",
		matched: "[text](<a)",
	},
	{
		// J42-B: should be `a\>b` — a backslash escapes the byte after it, so
		// this `>` does not close the destination.
		name:    "an escaped closing bracket",
		body:    "See [text](<a\\>b>) here.\n",
		raw:     "<a\\>b>",
		matched: "[text](<a\\>b>)",
	},
	{
		// J42-B: should be `a`. The destination ends at the first unescaped `>`,
		// and what follows it before the parenthesis is not part of it.
		name:    "an unescaped bracket ends the destination",
		body:    "See [text](<a > b>) here.\n",
		raw:     "<a",
		matched: "[text](<a > b>)",
	},
	{
		// J42-B: should be the empty destination CommonMark allows.
		name:    "an empty bracketed destination",
		body:    "See [text](<>) here.\n",
		raw:     "<>",
		matched: "[text](<>)",
	},
	{
		// J42-B: should be `a b` with the heading anchor `h` split off as usual.
		name:    "a spaced destination with an anchor",
		body:    "See [text](<a b#h>) here.\n",
		raw:     "<a",
		matched: "[text](<a b#h>)",
	},
	{
		// Unchanged by J42-B: with no closing bracket there is no bracketed
		// destination, so the ordinary rule applies and cuts at the space.
		name:    "an unclosed bracket falls back to the ordinary rule",
		body:    "See [text](<a b) here.\n",
		raw:     "<a",
		matched: "[text](<a b)",
	},
	{
		// Unchanged by J42-B.
		name: "an unclosed bracket with no parenthesis is not a link",
		body: "See [text](<a b here.\n",
		none: true,
	},
	{
		// Unchanged by J42-B: the percent-encoded alternative already works, and
		// is what a note can use today.
		name:    "a percent-encoded space already works",
		body:    "See [text](a%20b) here.\n",
		raw:     "a%20b",
		matched: "[text](a%20b)",
	},
}

func TestJ42AngleDestination(t *testing.T) {
	for _, testCase := range j42Cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Extract(testCase.body)
			if testCase.none {
				if len(got) != 0 {
					t.Fatalf("want no candidate, got %#v", got)
				}
				return
			}
			if len(got) == 0 {
				t.Fatal("want one candidate, got none")
			}
			candidate := got[0]
			if candidate.RawTarget != testCase.raw {
				t.Errorf("target: got %q, want %q", candidate.RawTarget, testCase.raw)
			}
			if matched := testCase.body[candidate.StartByte:candidate.EndByte]; matched != testCase.matched {
				t.Errorf("span: got %q, want %q", matched, testCase.matched)
			}
		})
	}
}
