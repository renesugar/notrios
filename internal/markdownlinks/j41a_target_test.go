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

// j41Cases states what a Markdown link's target is taken to be, case by case,
// as it is recorded today. CommonMark allows a destination to contain "zero or
// more balanced pairs of unescaped parentheses", which is not an exotic shape: a
// Wikipedia URL, a Python docs anchor and several citation styles all carry one.
// Nine of these rows are wrong because of it, each marked with what J41-B should
// make it, so that the change can be seen to move those rows and no others.
var j41Cases = []j41Case{
	{
		// J41-B: the URL loses its tail and the final `)` is left as prose.
		name:    "a URL with a balanced pair",
		body:    "See [text](https://example.org/Foo_(bar)) here.\n",
		raw:     "https://example.org/Foo_(bar",
		matched: "[text](https://example.org/Foo_(bar)",
	},
	{
		// J41-B: should be `a(b)c`.
		name:    "a balanced pair with a tail",
		body:    "See [text](a(b)c) here.\n",
		raw:     "a(b",
		matched: "[text](a(b)",
	},
	{
		// J41-B: should be `a(b(c))d`.
		name:    "two levels of nesting",
		body:    "See [text](a(b(c))d) here.\n",
		raw:     "a(b(c",
		matched: "[text](a(b(c)",
	},
	{
		// J41-B: should be `a(b).png`, so the image resolves.
		name:    "an embed with a balanced pair",
		body:    "See ![alt](a(b).png) here.\n",
		raw:     "a(b",
		matched: "![alt](a(b)",
	},
	{
		// J41-B: should be `a(b)`.
		name:    "two links on one line each keep their own target",
		body:    "See [one](a(b)) and [two](c(d)) here.\n",
		raw:     "a(b",
		matched: "[one](a(b)",
	},
	{
		// J41-B: should be `a\(b\)`. Escaped parentheses do not count toward
		// the balance, so these two cancel and the unescaped one closes.
		name:    "escaped parentheses in a target",
		body:    "See [text](a\\(b\\)) here.\n",
		raw:     "a\\(b\\",
		matched: "[text](a\\(b\\)",
	},
	{
		// J41-B: should be `a\)b` — an escaped parenthesis does not close anything.
		name:    "an escaped closing parenthesis in a target",
		body:    "See [text](a\\)b) here.\n",
		raw:     "a\\",
		matched: "[text](a\\)",
	},
	{
		// Unbalanced input must not become a different kind of wrong: the
		// target still ends at the first closing parenthesis, as it always has.
		name:    "an unclosed pair still ends at the first parenthesis",
		body:    "See [text](a(b) here.\n",
		raw:     "a(b",
		matched: "[text](a(b)",
	},
	{
		name:    "an extra closing parenthesis still ends the target",
		body:    "See [text](a)b) here.\n",
		raw:     "a",
		matched: "[text](a)",
	},
	{
		name: "a target may not cross a line",
		body: "See [text](a(b\nc)) here.\n",
		none: true,
	},
	{
		// An angle-bracketed destination may hold spaces and unbalanced
		// parentheses. Notrios does not read that form: the title rule cuts the
		// target at the first space, so only `<a` survives. Stated here rather
		// than fixed, because supporting it means changing what a space in a
		// target means (J42).
		name:    "an angle-bracketed destination is not read",
		body:    "See [text](<a (b) c>) here.\n",
		raw:     "<a",
		matched: "[text](<a (b)",
	},
	{
		// J41-B: the target should be `a(b)` and the span should reach past the title.
		name:    "a title after a balanced pair is still stripped",
		body:    "See [text](a(b) \"title\") here.\n",
		raw:     "a(b",
		matched: "[text](a(b)",
	},
	{
		// J37's rule still holds: this is one literal href, not two rows.
		// J41-B: should be `[[Target]](x)`; J37's one-row rule is unaffected either way.
		name:    "a literal href containing a parenthesis",
		body:    "See [text]([[Target]](x)) here.\n",
		raw:     "[[Target]](x",
		matched: "[text]([[Target]](x)",
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
