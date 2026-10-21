# v1.0 J40: show ambiguous links, and report them with the notes they might mean

The plan is in `PLAN.md` under J40, with the owner's decisions of 2026-10-18.

## J40-A: what is recorded today (2026-10-21)

The tests are `internal/importers/obsidian/j40a_ambiguity_test.go` for the
library and, until J40-B retired it, `web/src/__tests__/j40a-preview.test.tsx`
for the preview. Each case
holds one link and records, for that link, what the note keeps, the stored row,
the editor's buffer check (`POST /api/v1/links/check`), lint's `ambiguous_link`
and `broken_document_link` counts, and whether the importer warned.

| how the link became ambiguous | note keeps | row | editor | lint | importer |
|---|---|---|---|---|---|
| two notes titled `Shared`, written in Notrios | `[[Shared]]` | ambiguous | ambiguous | `ambiguous_link` 1 | — |
| `a/Foo.md` and `b/Foo.md`, titled by their file names | `[[Foo]]` | ambiguous | ambiguous | `ambiguous_link` 1 | warned |
| `a/index.md` and `b/index.md`, titled by headings (J36-E; J32's collision corpus) | `[[index]]` | **unresolved** | **unresolved** | `broken_document_link` 1 | warned |
| two notes claim the alias `Shared` | `[[Shared]]` | **unresolved** | **unresolved** | `broken_document_link` 1 | warned |
| `a/pic.png` and `b/pic.png` | `![[pic.png]]` | ambiguous | ambiguous | `ambiguous_link` 1 | warned |

**The expected surprise is confirmed, and is narrower than the plan expected.**
The importer decides ambiguity by file name, and then by alias. It leaves an
ambiguous link as written (J36-E). The store re-resolves that text the only way
it can, by note title, so the row depends on how the notes that share the name
were titled:

- **Titled by their file names:** they share the title too, and the row says
  `ambiguous`.
- **Titled by a heading or by front matter,** as every note in J32's collision
  corpus is: no note has the title, and the row says `unresolved`.
- **An alias:** the store never resolves aliases, so an alias two notes claim is
  `unresolved`.
- **An attachment name:** a resource keeps its file name, so it is `ambiguous`
  to both the importer and the store.

A report built only from rows would therefore miss the vault-name case whenever
titles come from headings, and the alias case always. Those are the cases that
put this item on the plan.

**The editor and lint agree with the row.** The buffer check runs the store's
extractor and resolver, so it can say nothing the row does not. Lint reads the
rows. A link the importer found ambiguous, but the store calls `unresolved`, is
counted by lint as a broken document link, not as an ambiguous one.

**Library Health shows lint's counts and nothing else.** It fetches the capped
findings but renders only each check's count (`LibraryHealth.tsx`, the
`health-checks` list). So it can say "1 ambiguous link" or "1 broken document
link", but never which link or which note. When a check is truncated, it adds
"showing the first N", where N is the number of findings it fetched, though it
shows none of them. No test pins either behaviour.

**The preview says nothing about any of them.** The renderer has no wiki-link
rule, so `[[index]]` is printed as prose, brackets included. A Markdown link
whose target is not a scheme the preview routes keeps its `<a>` element and
loses its `href`. It then has the same class as a working link and no title. The
preview renders the body alone and never consults the service.

**The editor's check runs only for an editable note.** `useBufferLinks` is
enabled by `editable`, so a note in a read-only notebook, Help or Reports, is
never checked. J40-B's marking is planned to come from that check. For a
read-only note there would then be nothing to mark from, unless the preview
gains a check of its own. J40-B's design has to say which.

### What the library keeps that J40-E can use

The plan says that deriving the candidates later "needs the source vault, which
the library does not keep". That is not quite so. Every Obsidian import writes
each note's provenance (`noteSource`, into `document_sources`). Its
`external_id` is the note's vault path, and its `metadata_json` holds
`relative_path` and `aliases`. That is what the importer's namespace was built
from: names come from paths, and aliases from front matter. Provenance is
written on every import, including for a skipped note (J39), and it travels by
sync. An attachment keeps its file name on the resource.

What provenance does not keep is **which vault** a note came from. Two vaults
imported into one collection would be one namespace to a derivation, where the
importer saw two. Item states (`import_item_states`) do record the vault, by
source key, but they are local, and a library reached by sync has none.

### What the owner decided on this evidence (2026-10-21)

The plan records these as decisions 9 to 11.

- **J40-E derives candidates from provenance,** and records nothing new at
  import. There is no schema change, and the one-namespace-per-collection limit
  is stated where it applies.
- **A read-only note is not marked.** The preview makes no request of its own.
- **J40-B renders wiki links, and J47 renders embeds.** A probe for this
  decision found that the preview shows every link an import resolved as raw
  text: `[[document://default/documents/doc_x|shown]]` renders as exactly those
  characters inside a `<p>`, and so does `![[resource://…]]`.

## J40-B: the preview renders wiki links, and marks links that will not open

The rendering was written into `UI_DESIGN.md` ("Wiki links, and links that will
not open") and committed, in `9b63ff1`, before any of it was built. The user
documentation is in `docs/gui.md`, under the heading of the same name.

**How it is built.**

- **The renderer rule.** `web/src/preview-links.ts` adds a markdown-it rule. It
  matches what the extractor matches, `(!?)\[\[([^\]\n]+)\]\]`, and splits the
  text at the first pipe the same way.
  - A `[[…]]` naming a `document://`, `resource://` or `notrios://` URI becomes
    an `<a href>`, which the existing sanitiser routes like the Markdown link to
    the same URI.
  - A `[[Name]]` becomes an `<a>` with no destination.
  - A `![[…]]` becomes a `<span>` holding the characters as written.
  - markdown-it's own link rule is wrapped. Every element either rule emits
    records the link's exact source text in `data-link-source`.
- **The marks.** After the HTML is in the page, `applyLinkMarks` looks each
  element's source text up in the editor's last buffer check. The check's byte
  offsets are converted into the checked body, and the text is compared whole.
  - A broken status marks the element. Its destination and any title the author
    wrote are kept aside, and its hover text is set from the editor's wording.
  - A name link the check resolved is routed to the URI the check found.
  - Every pass first undoes the last one. A link the check has not seen is left
    alone.
- **Where the check comes from.** EditorPane hands each check up through
  `onLinksChecked`. App passes it to the preview only for an editable note, and
  clears it whenever another note is opened, because the same text can have a
  different status in another note: an anchor-only link, for one.

**What the tests show.** These are `web/src/__tests__/j40b-preview-links.test.tsx`
(12 tests) and `j40b-editor-check.test.tsx` (2 tests).

- **A URI wiki link** renders and opens like its Markdown form. A working embed
  stays text.
- **A wiki link inside a Markdown link's text** stays one Markdown link (J37).
  The rule declines when markdown-it measures a link's text in silent mode.
- **Note text never becomes markup.** A wiki link carrying `"><img onerror>` and
  `<script>` renders those characters as text.
- **Each status gets its own class and hover text.**
  - `ambiguous` and `unresolved` links are marked distinctly.
  - A written-out URI the check calls broken loses its click target.
  - Code and plain words are never marked, even when the check classified the
    same text inside a code span. The extractor does classify links in code, and
    the preview has no element there to mark.
- **With no check,** as for a read-only note, nothing is marked.
- **A newer check that disagrees** gives back what a mark took. An author's
  title survives being marked and unmarked. The undo is also pinned directly on
  `applyLinkMarks`, because in the pane a newer check usually arrives with a
  freshly rendered DOM.
- **Byte offsets after multi-byte text** place the mark correctly.
- **The editor** hands its check up for an editable note. For a read-only note
  it hands up only nothing, and asks the service nothing.

**The tests were mutated to show they can fail.**

| mutation | result |
|---|---|
| `applyLinkMarks` does nothing after its undo | 7 tests fail |
| the rule consumes in silent mode | the J37 test fails |
| the undo restores no stashed attribute | the direct undo test fails, after it was added for this reason |

The J40-A preview test recorded the preview as it was before this slice: a wiki
link printed as prose. J40-B changes exactly that, so the test was retired in
this slice. Its observation is in the J40-A table above.

**What it costs.** The built UI grows by 3,455 bytes in total:

| file | before | after |
|---|---:|---:|
| entry chunk `index-*.js` | 1,302.20 kB (gzip 429.55 kB) | 1,305.29 kB (gzip 430.61 kB) |
| `index-*.css` | 130.80 kB (gzip 27.42 kB) | 131.17 kB (gzip 27.49 kB) |

Every other chunk is unchanged in size, and only renamed because the entry
chunk's hash moved. Opening or editing a note makes no new request. The marks
reuse the check the editor already makes.

**What the GUI crawl found.** The new file moved H15's interface signature, which
lists every interface file even when it holds no control. So the control
inventory was crawled again. The crawl ran with HOME and XDG in a work directory
on HD2 and DBUS unset, and its replica uses the `development-file` credential
store. Two runs agreed exactly. It still finds 89 distinct controls.

To separate this slice from anything older, the commit before the code
(`9b63ff1`) was crawled the same way. Against that, J40-B changes one thing. The
new-note template's example link,
`[Related note](document://default/documents/<document-id>)`, names no note. In
the 12 states where the editor checks it, the preview now marks it unresolved
and removes its destination. Each of those states therefore counts one fewer
link, and `shape:a|||` falls from 14 instances to 2. This is the marking working
as designed, and it is visible: a new note's example link now shows the wavy
underline until it is replaced.

The committed inventory differed from the base crawl in other ways too: more
search cards (328 to 343) and different sample labels. That drift predates this
item. The signature only moves when a control's markup does, and nothing had
asked for a re-crawl since H29.
