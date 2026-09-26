# v1.0 J42: a bracketed destination keeps its spaces

J41 found this while fixing where a target ends: a Markdown link's destination
was cut at the first space, because that is how the optional title after a
destination is stripped.

```
[text](<My Note.md>)   ->  target "<My"
```

CommonMark's angle-bracketed form exists precisely so a destination can hold
spaces, and it is what Obsidian's own documentation recommends for a note whose
name has one — `[text](<Target Note>)`, alongside `[text](Target%20Note)`. So a
note named `My Note.md`, linked the documented way, resolved to nothing. Spaces
in note names are ordinary, which made this a more common trigger than the
parentheses J41 was about.

It was deferred out of J41 because it changes what a target *contains* rather
than where it *ends*, and what it changes is the meaning of a space — the thing
that separates a destination from its title. Getting that wrong would turn a
title into part of a path for every link in every library.

## What was recorded before (J42-A)

| body | target | span |
|---|---|---|
| `[text](<a b>)` | `<a` | whole |
| `[text](<My Note.md>)` | `<My` | whole |
| `[text](<a>)` | `<a>` | whole |
| `[text](<a> "title")` | `<a>` | whole |
| `[text](<a b> "title")` | `<a` | whole |
| `[text](<a (b) c>)` | `<a` | whole |
| `[text](<a ) b>)` | `<a` | `[text](<a )` |
| `[text](<a) b>)` | `<a` | `[text](<a)` |
| `[text](<a\>b>)` | `<a\>b>` | whole |
| `[text](<a > b>)` | `<a` | whole |
| `[text](<>)` | `<>` | whole |
| `[text](<a b#h>)` | `<a`, no anchor | whole |

Twelve rows wrong, two of them with a truncated span as well: inside the
brackets a parenthesis is an ordinary character, and J41's depth rule was ending
the link on one.

Three rows had to stay as they were, and say so: an unclosed bracket falls back
to the ordinary rule, an unclosed bracket with no parenthesis is not a link, and
the percent-encoded alternative already works — it is what a note could use
before this.

`internal/markdownlinks/j42a_angle_test.go` stated each row as it was, marked
with what J42-B should make it.

## The rule (J42-B)

One helper finds the bracket, and both the span scan and the target read go
through it:

- `angleClose` returns the `>` that closes the `<`. An unescaped `<` or `>` may
  not appear inside, a backslash escapes the byte after it, and a line ends the
  search.
- `targetEnd` looks for the closing parenthesis **after** that bracket, so a
  parenthesis inside the brackets no longer ends the link.
- `markdownTarget` returns what sits between the brackets, spaces and all. The
  brackets are not part of the destination, and anything after the closing
  bracket is a title.

Everything else is untouched. A destination that does not begin with `<` is still
cut at the first space, and a `<` with no closing bracket falls back to that rule
rather than becoming a different kind of wrong — the same promise J41-B made
about unbalanced parentheses.

### The reference learned it too

J32-Y holds each scan to a second implementation over 20,000 generated bodies.
The Markdown reference gained the bracket rule written differently again:
`referenceAngleClose` jumps between the bytes that can matter with `IndexAny`
where `angleClose` examines each one, and the parenthesis resolution is factored
out so the bracketed and ordinary paths share it. The old regexp is still the
cross-check wherever no destination carries a parenthesis or a backslash.

## What it was costing (J42-C)

| link | before | after |
|---|---|---|
| `[note](<My Note.md>)` | target `<My`, unresolved, left as prose | resolves, rewritten to `document://…` |
| `![shot](<assets/my shot.png>)` | target `<assets/my`, unresolved, resource unreferenced | resolves, rewritten, resource referenced |
| `[titled](<My Note.md> "A title")` | target `<My`, unresolved | resolves |
| `[pct](My%20Note.md)` | resolves | resolves, unchanged |

The preview needs nothing: an unresolved bracketed link renders inert like any
other, because the preview strips the `href` of a scheme it does not know, and a
resolved one is a `document://` or `resource://` URI by the time it is stored.

**A library imported before J42** has these links unrewritten, because their
targets did not resolve — so the stale body is the vault's own text and the
simulation of it is exact rather than a reconstruction. A reimport is the repair,
it revises only that note, and J36-C's invariant still holds: a reimport of what
this build wrote adds no revision to anything.
`internal/importers/obsidian/j42c_angle_test.go` carries the whole sequence.

## Found here, deferred to J43

Rewriting a Markdown link **drops its title**. The replacement text is built from
the display text and the resolved URI alone, so `[a](Target.md "A title")` comes
back as `[a](document://…)`. That has always been true and J42 did not cause it —
a title may follow any destination — but J42 made it easier to meet, because the
bracketed form is where a title most often appears.

A title is the author's text. Losing it on import is the same class of harm as
losing a link: the note comes back from a round trip saying less than it said.
The test records what happens rather than endorsing it.

## Archive

`notrios-v1.0-j42-36309b6.zip`, built from commit `36309b6` — the commit that
completed this item's work, with the record above in it and the ledger not yet
flipped. 26,488,501 bytes, 2,948 entries, accepted by `check_release_zip.py`.

**Built with `NOTRIOS_AGENT_USAGE_GUARD=off`, authorized by the owner for this
archive on 2026-09-26.** The seven-day bucket stood at 17.0% remaining against a
20.0% required reserve; the five-hour bucket had rolled into a new window and had
94.0%. The weekly bucket resets 2026-09-29.

There are no measurement directories here. This item's evidence is its tests:
`internal/markdownlinks/j42a_angle_test.go` for what a bracketed destination is
taken to be, `internal/markdownlinks/j32y_scanner_test.go` for the reference that
learned the same rule differently and the subset the old pattern still
cross-checks, and `internal/importers/obsidian/j42c_angle_test.go` for the
import, the rows, the no-op reimport and the repair.
