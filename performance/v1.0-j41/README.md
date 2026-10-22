# v1.0 J41: a link's target ends where it closes

The Markdown scan ended a target at the first `)` in the body. J37-A found it
while probing a different question:

```
[outer]([inner](deep))   ->  one row, raw "[inner](deep", span "[outer]([inner](deep)"
```

CommonMark allows a link destination to contain "zero or more balanced pairs of
unescaped parentheses", so this is not an exotic shape — a Wikipedia URL, a
Python docs anchor and several citation styles all carry one. It was deferred out
of J37 because it has nothing to do with wiki links and changes rows in bodies
that have nothing to do with that decision.

## What was recorded before (J41-A)

| body | target | span |
|---|---|---|
| `[text](https://example.org/Foo_(bar))` | `https://example.org/Foo_(bar` | `[text](https://example.org/Foo_(bar)` |
| `[text](a(b)c)` | `a(b` | `[text](a(b)` |
| `[text](a(b(c))d)` | `a(b(c` | `[text](a(b(c)` |
| `![alt](a(b).png)` | `a(b` | `![alt](a(b)` |
| `[text](a\(b\))` | `a\(b\` | `[text](a\(b\)` |
| `[text](a\)b)` | `a\` | `[text](a\)` |
| `[text](a(b) "title")` | `a(b` | `[text](a(b)` |

Four shapes had to stay as they were, and say so in the table: an unclosed pair
still ends at the first parenthesis, an extra closing parenthesis still ends the
target, a target still may not cross a line, and an angle-bracketed destination
is still not read.

`internal/markdownlinks/j41a_target_test.go` stated each row as it was, marked
with what J41-B should make it, so the change could be seen to move those rows
and no others.

## The rule (J41-B)

`targetEnd` walks the destination once and returns the parenthesis that closes
the one the destination opened with:

- a `(` deepens, a `)` at depth zero closes, a `)` below it returns to the
  enclosing level;
- a backslash escapes the byte after it, so `\(` and `\)` are text and count
  toward nothing — which is why `a\(b\)` reads whole: the escaped pair cancels
  and the unescaped parenthesis closes;
- an escape does not carry a target across a line. A destination may not cross
  one, and a backslash does not buy one;
- **when the parentheses do not balance there is no closing one to find**, and
  the answer is the first `)` at any depth. That is what this scan has always
  returned, and a body that is wrong today should not become a different kind of
  wrong.

One pass, no allocation, as before.

### The regexp is retired as the Markdown reference, with its reason

J32-Y held each hand-written scanner to the pattern it replaced, offset for
offset, over 20,000 generated bodies. A regexp cannot express this rule: RE2 has
no recursion and the rule counts depth. So the Markdown half is held to
`referenceMarkdownSpans` instead — a second implementation written differently on
purpose, collecting a destination's unescaped parentheses and resolving the
closer from that list where the scanner decides while walking. Two
implementations of one small rule, compared over every generated body, which is
what this file has always done.

The old pattern is kept as a live cross-check for what it can still express:
where no destination carries a parenthesis or a backslash, the scanner must still
agree with it offset for offset. The wiki scan is unaffected and is still held to
its own regexp.

## What it was actually costing (J41-C)

Not a short string. **A note or attachment whose file name carries a parenthesis
never resolved at all**, because the recorded target was missing its tail — so
the link was left as prose and the attachment went unreferenced. With the target
read whole:

| link | before | after |
|---|---|---|
| `[draft](Note_(draft).md)` | target `Note_(draft`, unresolved, left as prose | resolves, rewritten to `document://…` |
| `![shot](assets/img_(1).png)` | target `assets/img_(1`, unresolved, resource unreferenced | resolves, rewritten, resource referenced |
| `[ext](https://example.org/Foo_(bar))` | target truncated, `)` left as prose | the whole URL, recorded as `external` |

Nothing else in the suite moved: no existing test depended on a truncated target.

**A library imported before J41** has these links unrewritten, because neither
target resolved — which means the stale body is the vault's own text, so the
simulation of it is exact rather than a reconstruction. A reimport is the repair,
it revises only that note, and J36-C's invariant still holds: a reimport of what
this build wrote adds no revision to anything.
`internal/importers/obsidian/j41c_parens_test.go` carries the whole sequence.

## Deferred to J42

The angle-bracketed destination `[text](<a (b) c>)` is still not read: the title
rule cuts a target at the first space, so only `<a` survives. Its **span** moved
here, because the parentheses inside it balance, which is an improvement on its
own terms — a rewrite of that span no longer strands `c>)` in the note — but the
target is still wrong, and the test says so rather than accepting it quietly.

That form is not exotic either. It is what Obsidian's own documentation
recommends for a note whose name contains a space, `[text](<Target Note>)`,
alongside `[text](Target%20Note)`. Reading it means changing what a space in a
target means, which is exactly what separates a destination from its title, so it
is J42 rather than a second change here.

## Archive

`notrios-v1.0-j41-c71acbb.zip`, built from commit `c71acbb` — the commit that
completed this item's work, with the record above in it and the ledger not yet
flipped. 26,479,358 bytes, 2,944 entries, accepted by `check_release_zip.py`.

**Built with `NOTRIOS_AGENT_USAGE_GUARD=off`, authorized by the owner for this
archive on 2026-09-26.** The seven-day bucket stood at 18.0% remaining against a
20.0% required reserve, below the line rather than on it as J37's was; the
five-hour bucket, which is the one that moves quickly, had 41.0%. The weekly
bucket resets 2026-09-29.

There are no measurement directories here. This item's evidence is its tests:
`internal/markdownlinks/j41a_target_test.go` for what a target is taken to be,
`internal/markdownlinks/j32y_scanner_test.go` for the independent reference
compared over 20,000 generated bodies and the subset the old pattern still
cross-checks, and `internal/importers/obsidian/j41c_parens_test.go` for the
import, the rows, the no-op reimport and the repair.
