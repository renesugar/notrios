# v1.0 J37: one span of a note, one row

J32-Y found a de-duplication in `markdownlinks.Extract` that could never fire:

```go
seen[candidateKey(candidate)] = true        // "markdown:" + rawTarget + ":" + displayText
…
if seen[body[loc[0]:loc[1]]] { continue }   // the raw matched text, e.g. "[[Target]]"
```

The keys written and the keys looked up were of different kinds, so no lookup
ever matched and every link both patterns found had always been reported twice.
J32-Y removed the dead map — dead code should not sit in a parser pretending to
guard something — but removing it decided nothing. This item is that decision.

## What was recorded before (J37-A)

| body | rows |
|---|---|
| `[text]([[Target]])` | `markdown` raw `[[Target]]` display `text` span [4,22); `obsidian-wikilink` raw `Target` span [11,21) |
| `![text]([[Target]])` | `markdown` **embed** raw `[[Target]]` span [4,23); `obsidian-wikilink` **link** raw `Target` span [12,22) |
| `[[Target]]` | `obsidian-wikilink` raw `Target` span [4,14) |
| `[[Target\|text]]` | `obsidian-wikilink` raw `Target` display `text` |

Two rows described one span, with different targets and overlapping byte ranges.
J32-D met the same shape from the other side: its one-pass body assembly hands
overlapping replacements back to the older splice loop precisely because two
rewrites can cover the same bytes.

The probe also found two artifacts that had nothing to do with duplication:

- `[text]([[Target#heading]])` recorded the outer row as raw `[[Target` with the
  anchor `heading]]`. The anchor split ran on a literal href and invented a
  heading that does not appear in the note.
- `[outer]([inner](deep))` already recorded **one** row, raw `[inner](deep`,
  spanning `[outer]([inner](deep)` — truncated before the final parenthesis.

## The decision (J37-B)

**A wiki link written where a Markdown link's href belongs is a literal href.
Only the outer Markdown row survives. A `[[Target]]` anywhere else is still a
wiki link.**

The owner's reading is that `[text]([[Target]])` is a syntax error for
`[[Target|text]]`, and that what belongs inside a Markdown link's parentheses is
an href.

Obsidian reads it the same way, and it is the same conclusion the one person who
hit it reached. `[Woodworking]([[Woodworking]])` makes no backlink to
Woodworking, and clicking it in viewing mode "creates a new file called
`[[Woodworking]]`" — the brackets are part of the destination. The answer given
was to use the pipe form
(<https://forum.obsidian.md/t/wikilinks-inside-markdown-links-not-recognized-as-normal-backlinks/34139>).

Notrios' outer row already recorded exactly that — raw target `[[Target]]`,
display text `text` — so this suppresses a nested candidate rather than
inventing a row.

One row per span satisfies the constraint the decision was made under: the
ranges that remain do not overlap, so no overlapping rewrite reaches the path
J32-D has to work around.

## What shipped (J37-C)

**Overlap, not containment.** The rule drops a wiki match that overlaps a
Markdown link, which is stronger than dropping one contained in its target.
`[a](x[[b)]]` produces a wiki match that starts inside the Markdown link and
ends past it; containment would have kept both and left two rows over the same
bytes, which is the case this decision exists to prevent. Because Markdown
matches do not overlap each other and neither do wiki matches, dropping every
overlapping wiki match makes what remains provably disjoint.

The invariant is asserted over the 20,000 bodies J32-Y's reference test
generates from the pieces the two patterns can meet, not over the shapes that
occurred to me.

**A literal href keeps its bytes.** `decorateCandidate` skips the anchor split
for a Markdown target beginning with `[[`, so `[[Target#heading]]` is recorded
whole instead of as raw `[[Target` plus an invented anchor.

**The cost is one small slice per note.** The wiki pass walks the Markdown
matches with a single index, since both sequences ascend and neither overlaps
itself, so there is no per-link work and no per-link allocation — J32 spent a
long time getting allocations out of this file and this does not put one back.

### The consequences, each checked

| what | outcome |
|---|---|
| import rewriting | the nested wiki link is no longer rewritten; J32-D's two pinned cases updated **by decision**, with the reason in the test, and the top-level embed sharing one of those bodies still rewrites |
| the store's rows | one row per span; the literal href is `unresolved`, with no target document and no anchor |
| the preview | nothing to change: it strips the `href` of any scheme it does not know, so a literal href renders inert |
| J32-Y's reference test | it implemented the dead de-duplication it was written to preserve; it now implements this rule the slow way, scanning every claimed range per match while `Extract` walks with one index, so it still holds the scanners to the regexps |

### A library imported before this

Worth writing down, because the first guess was wrong. Those bodies read
`[label]([[document://…]])`: the old importer rewrote the inner wiki link even
though it sat inside a literal href. The new parser reads that as **one** literal
href including the brackets, which resolves to nothing — so the edge that reached
the target *from inside the parentheses* leaves the link index until the note is
reimported. The top-level link in the same note is unaffected.

A reimport restores the body the vault actually has and the rows that follow from
it, and J36-C's invariant still holds: a reimport of what this build wrote adds
no revision to anything. `internal/importers/obsidian/j37c_rows_test.go` pins the
whole sequence — the fresh import, the no-op reimport, the doctored pre-J37 body,
and the repair.

## Deferred to J41

`[outer]([inner](deep))`'s truncated span is not fixed here. The same truncation
applies to every ordinary Markdown link whose URL carries balanced parentheses,
which CommonMark allows — a Wikipedia URL, a Python docs anchor, a citation
style — so the fix would change stored rows for bodies that have nothing to do
with wiki links. It is J41, under the rule that a finding needing its own code
change gets its own item.
