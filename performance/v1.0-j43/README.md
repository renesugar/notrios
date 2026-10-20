# v1.0 J43: a title is the author's text, and it comes back

J42-C found this while making bracketed destinations keep their spaces. A
Markdown link's replacement text was built from the display text and the
resolved URI alone:

```go
replacementText = prefix + "[" + candidate.DisplayText + "](" + uri + ")"
```

So `[a](Target.md "A title")` came back as `[a](document://…)`, and the title was
gone from the note. That had always been true, and J42 did not cause it: a title
may follow any destination. But J42 made it easier to meet, because the bracketed
form is where a title most often appears. A title is the author's text, so losing
it on import is the same class of harm as losing a link. The note comes back from
a round trip saying less than it said.

## What was lost (J43-A)

Before J43-B, the Obsidian rewrite dropped every one of these:

| body | rewritten before J43 | rewritten now |
|---|---|---|
| `[shown](Target.md "A title")` | `[shown](uri)` | `[shown](uri "A title")` |
| `![shown](Target.md "A title")` | `![shown](uri)` | `![shown](uri "A title")` |
| `[shown](<Target.md> "A title")` | `[shown](uri)` | `[shown](uri "A title")` |
| `[shown](Target.md 'A title')` | `[shown](uri)` | `[shown](uri 'A title')` |
| `[shown](Target.md (A title))` | `[shown](uri)` | `[shown](uri (A title))` |
| `[shown](Target.md#Heading "A title")` | `[shown](uri#Heading)` | `[shown](uri#Heading "A title")` |

Two rows were already right, and are pinned so they stay that way:
- a wiki link's display text, `[[Target|shown text]]`, which is not a title;
- a link with no title, which is written byte for byte as before.

**Two other places were read for the same defect.**

- **The Joplin importer does not have it.** It substitutes the `:/id` token where
  it stands and copies everything around it unchanged, so a title is never seen
  and never lost. The Obsidian rewriter rebuilds the link from its parts, which is
  why only that one dropped anything. `TestJ43AJoplinKeepsTitles` pins it.
- **`notriosctl fix` did have it.** `renderMarkdownLink` rebuilt a link from a
  stored row whose `RawTarget` is the destination alone. So repairing a
  non-canonical target, or adding missing alt text, cost the author their title.
  A repair tool losing text is worse than an importer losing it, so it was fixed
  in this item rather than deferred again.

## The fix (J43-B)

`markdownDestination` now returns the destination and, beside it, the bytes that
followed it: the whitespace and whichever of the three delimiters the author
used. `Candidate.TitleSuffix` carries them, and both rewriters put them back
after the URI and any anchor. The bytes are copied, never parsed or normalised.
That is how one field covers `"…"`, `'…'` and `(…)` without knowing the
difference: whatever the author wrote between the delimiters is what comes back.

`notriosctl fix` has no candidate to read, only a stored row. But it holds the
link's own bytes, so `markdownlinks.TitleSuffix` recovers the title from those.
No column was added and the schema is unchanged. `TestFixKeepsATitle` repairs a
double-quoted title, a single-quoted one, and an image title while adding alt
text, and keeps all three.

## The consequences (J43-C)

- **Stored rows do not change.** A row records where a link points, and a title
  is not part of that. The test asserts it, so a title cannot leak into a target.
- **The preview needs nothing.** A title becomes the `title` attribute that the
  Markdown renderer writes, escaped as an attribute value. The preview already
  strips every `on*` and `style` attribute from note-controlled HTML. A note
  authored in Notrios could always carry a title, so this is not a new kind of
  content reaching the preview, only one that imported notes stopped losing.
- **A library imported before J43 is repaired by a reimport.**
  `TestJ43CTitlesSurviveAnImport` builds such a library exactly: the
  titles are removed from the body, because those bytes are what the old
  importer wrote. A reimport revises that note once and puts both titles back,
  and the note that never lost anything gains no revision. J36-C's invariant
  holds for a library written by this build: a reimport that changes nothing
  adds nothing.

J42-C's test had asserted the old behaviour and flagged it as recorded rather
than endorsed. It now records the repair.

## What this does not prove

- **Three places rewrite a Markdown link's destination, and all three were
  read:** the Obsidian and Joplin importers and `notriosctl fix`. That list
  comes from searching the code for link text built from parts and for every
  link-rewriting function, during this record. The same search found a fourth
  place that changes links: publication, in `internal/archivev2/publish.go`. It
  replaces a link to a withheld or unresolved note with the link's display text,
  or a placeholder. That drops a title as well, but it removes the link on
  purpose rather than rewriting where it points, and a title could name the note
  that was held back. It is left as it is. A future rewriter that builds a link
  from its parts would need the same care. The guard against it is the pattern
  J43-A found, not a test.
- **A title is copied verbatim, including any oddity in it.** That is the rule,
  not a gap: normalising it would be rewriting the author's prose.
- **No library outside the tests was reimported to measure the repair.** The
  owner's own Obsidian libraries are private and stay out of the repository and
  the evidence. The repair is shown on a library built to match what the old
  importer wrote.

There are no measurement directories here. This item's evidence is its tests:
`internal/importers/obsidian/j43a_title_test.go` for each title form,
`internal/importers/joplinraw/j43a_title_test.go` for the importer that never
had the defect, `internal/store/fix_test.go` (`TestFixKeepsATitle`) for the
repair tool, and `internal/importers/obsidian/j43c_reimport_test.go` for the
rows, the no-op reimport and the repair.

## Archive

`notrios-v1.0-j43-666b705.zip`, built from commit `666b705`, the commit that
completed this item's record, with the ledger not yet flipped. 26,613,074 bytes,
2,979 entries, accepted by `check_release_zip.py`. No usage override was needed:
the seven-day bucket had 92% remaining and the five-hour bucket had 75%.

The work itself is in `f4de470`, from 2026-09-26. The record and archive waited
on J44 and J45, which J44's discovery during a test run put first, so this
archive also carries both of them. That changes nothing J43 claims: its tests
pass unchanged at this commit.
