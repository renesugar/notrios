package markdownlinks

import "testing"

// j41Case is one body and the single Markdown row it produces.
type j41Case struct {
	name    string
	body    string
	raw     string // the recorded target
	matched string // the bytes the span covers
	none    bool   // no candidate at all
}

// j41Cases states what a Markdown link's target is taken to be, case by case.
// CommonMark allows a destination to contain "zero or more balanced pairs of
// unescaped parentheses", which is not an exotic shape: a Wikipedia URL, a Python
// docs anchor and several citation styles all carry one. J41-A stated these rows
// as they were, each marked with what it should become; J41-B moved them, and
// each comment now records what it was.
var j41Cases = []j41Case{
	{
		// Was `https://example.org/Foo_(bar` with the final `)` left as prose,
		// which is the defect this item was opened for.
		name:    "a URL with a balanced pair",
		body:    "See [text](https://example.org/Foo_(bar)) here.\n",
		raw:     "https://example.org/Foo_(bar)",
		matched: "[text](https://example.org/Foo_(bar))",
	},
	{
		// Was `a(b`.
		name:    "a balanced pair with a tail",
		body:    "See [text](a(b)c) here.\n",
		raw:     "a(b)c",
		matched: "[text](a(b)c)",
	},
	{
		// Was `a(b(c`. CommonMark asks for at least three levels; depth
		// counting has no limit.
		name:    "two levels of nesting",
		body:    "See [text](a(b(c))d) here.\n",
		raw:     "a(b(c))d",
		matched: "[text](a(b(c))d)",
	},
	{
		// Was `a(b`, so the image did not resolve.
		name:    "an embed with a balanced pair",
		body:    "See ![alt](a(b).png) here.\n",
		raw:     "a(b).png",
		matched: "![alt](a(b).png)",
	},
	{
		// Was `a(b`. Two links on one line each end at their own parenthesis.
		name:    "two links on one line each keep their own target",
		body:    "See [one](a(b)) and [two](c(d)) here.\n",
		raw:     "a(b)",
		matched: "[one](a(b))",
	},
	{
		// Was `a\(b\`. Escaped parentheses are text: they count toward nothing,
		// so these two cancel each other and the unescaped one closes.
		name:    "escaped parentheses in a target",
		body:    "See [text](a\\(b\\)) here.\n",
		raw:     "a\\(b\\)",
		matched: "[text](a\\(b\\))",
	},
	{
		// Was `a\`. An escaped parenthesis closes nothing.
		name:    "an escaped closing parenthesis in a target",
		body:    "See [text](a\\)b) here.\n",
		raw:     "a\\)b",
		matched: "[text](a\\)b)",
	},
	{
		// Unbalanced input must not become a different kind of wrong: with no
		// closing parenthesis to find, the target ends at the first one, which
		// is what this scan has always returned. Unchanged by J41-B.
		name:    "an unclosed pair still ends at the first parenthesis",
		body:    "See [text](a(b) here.\n",
		raw:     "a(b",
		matched: "[text](a(b)",
	},
	{
		// Unchanged by J41-B: depth is zero, so the first `)` closes.
		name:    "an extra closing parenthesis still ends the target",
		body:    "See [text](a)b) here.\n",
		raw:     "a",
		matched: "[text](a)",
	},
	{
		// A backslash does not buy a line either: the escape ends at the line.
		name: "an escape does not carry a target across a line",
		body: "See [text](a\\\nb) here.\n",
		none: true,
	},
	{
		// Unchanged by J41-B.
		name: "a target may not cross a line",
		body: "See [text](a(b\nc)) here.\n",
		none: true,
	},
	{
		// J42 reads this form: the destination is what sits between the
		// brackets, spaces and all. J41 had already given the row its whole
		// span, because the parentheses inside the brackets balance; before
		// that the span was `[text](<a (b) c>` and the target `<a`.
		name:    "an angle-bracketed destination is read",
		body:    "See [text](<a (b) c>) here.\n",
		raw:     "a (b) c",
		matched: "[text](<a (b) c>)",
	},
	{
		// Was target `a(b` with the span stopping before the title.
		name:    "a title after a balanced pair is still stripped",
		body:    "See [text](a(b) \"title\") here.\n",
		raw:     "a(b)",
		matched: "[text](a(b) \"title\")",
	},
	{
		// Was `[[Target]](x`. J37's one-row rule is unaffected: this is still a
		// single literal href, now recorded whole.
		name:    "a literal href containing a parenthesis",
		body:    "See [text]([[Target]](x)) here.\n",
		raw:     "[[Target]](x)",
		matched: "[text]([[Target]](x))",
	},
}

func TestJ41TargetExtent(t *testing.T) {
	for _, testCase := range j41Cases {
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
			if candidate.SourceFormat != "markdown" {
				t.Fatalf("want a markdown row first, got %q", candidate.SourceFormat)
			}
			if candidate.RawTarget != testCase.raw {
				t.Errorf("target: got %q, want %q", candidate.RawTarget, testCase.raw)
			}
			matched := testCase.body[candidate.StartByte:candidate.EndByte]
			if matched != testCase.matched {
				t.Errorf("span: got %q, want %q", matched, testCase.matched)
			}
		})
	}
}
