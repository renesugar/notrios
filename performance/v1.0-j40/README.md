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

## J40-E: the import's candidates, rebuilt from provenance

`internal/importers/obsidian/provenance.go` rebuilds the import's namespace from
what the library keeps: each imported note's provenance (`document_sources`)
holds its vault path and aliases, and its title is the note's. That is
everything `buildLinkNamespace` reads. The namespace is then asked the import's
question through the import's own code. `noteMatches` was split out of
`resolveNoteID`, and the target filter that `planObsidianLinks` applied inline
became `vaultTarget`, shared by both. Nothing is a second copy of the rules, and
J39's pinned canonical digest and every importer test pass unchanged. There is
no schema change.

`j40e_provenance_test.go` imports a vault that holds every kind of note
ambiguity: file names under headings, an alias two notes claim, a name one note
has as its file name and another as its title, a folder that settles its own
`[[index]]`, and a path. For every link in every note, the rebuilt namespace
calls the link ambiguous exactly when the import warned, and names the notes the
import held for it. Dropping aliases or titles from the rebuilt namespace fails
the test. Attachment names are left to the store, which already finds them
ambiguous by file name.

A shared candidate list is handed back without copying, with the key it was
found under. In J32's collision shape, 5,000 links each name an alias that 100
notes share, so a copy per link would be 500,000 strings. The cost of copying
grows with the square of a shared name's size, since every note that shares the
name can link it.

The limit is stated wherever it applies. Provenance does not record which vault
a note came from, so two vaults imported into one collection are one namespace
to the report. A title is the note's current one.

## J40-C: the report

`internal/linkreport` sits above the store and the importer: the store cannot
import the importer, and the report needs both.

**Why a link is ambiguous.** The report gives one of two reasons:

- **The library's (`found_by: library`).** The row says `ambiguous`, and the
  candidates are what the resolver itself finds: the notes with the link's text
  as their title, or else the attachments with it as their file name. The
  resolver's lookups now take a limit and are shared, rather than copied for
  the report.
- **The import's (`found_by: import`).** The row says `unresolved`, the note has
  Obsidian provenance, and the rebuilt namespace finds several notes.

Everything else that opens nothing is only counted, with a pointer to lint
(decision 2).

**What an entry says.**

- **Each note** with an ambiguous link is named by its title, its `document://`
  URI and its stable `notrios://` URL.
- **Each link** is quoted exactly as written, sliced from the body by the row's
  own byte span, and located by line and column. Lines count from the top of the
  note as stored, including an import's provenance front matter.
- **Each candidate** is named the same way as a note. An attachment is named by
  its file name and its `resource://` URI.

**Order and caps.** Notes are in title order, links in source order, and
candidates in title order. The caps follow lint's shape (decision 8):

- `--detail-limit` caps the notes shown, and the links shown per note: 50 by
  default, 1,000 at most.
- `--candidate-limit` caps the candidates shown per link: 10 by default, 100 at
  most.
- The counts are always complete, and a cap that hides something says so.
- One digest covers every finding, so it is the same under any caps. It moves
  when a candidate is added, even if no link changed.

A shared candidate list is hashed and ordered once per name, not once per link.

**The tests** are `internal/linkreport/linkreport_test.go`:

- J40-A's library in one vault, plus notes written in Notrios, a note in Reports
  and a note in the Trash;
- the counts by reason;
- each link's text, location, reason and candidates;
- the stable URLs;
- the caps, and the digest under them;
- the digest moving when a third `Twin` is created.

Switching the import's derivation off fails them.

## J40-D: where the report is generated and read

| surface | what it carries |
|---|---|
| `notriosctl links report [--collection] [--detail-limit] [--candidate-limit] [--write-note] [--quiet]` | the whole report as JSON: links as written, titles, `document://` and `notrios://` URLs (decision 5) |
| the **Ambiguous links report** note, `doc_link_report`, in Reports | the same report as Markdown, linking each note and candidate by `document://` |
| `GET /api/v1/admin/links/report` | IDs, lines, columns, the SHA-256 of each target, the reason and kind, candidate IDs, and the counts. No titles, no link text, no URIs |
| `POST /api/v1/admin/links/report/note` | writes the note, and returns where it is with the content-free report |
| MCP | nothing. `get_lint_report` is the content-free form (decision 1) |
| Library Health | a **Reports to read** section that writes either report, says what it found and opens the note. The graph report had no control before (decision 6) |

**The note.** It is written the way the graph report is: a stable ID,
overwritten in place, read-only, in the Reports notebook.
- **Links are quoted, never live.** Each link is quoted with every ASCII
  punctuation character backslash-escaped. Markdown then shows it verbatim, and
  neither the extractor nor the preview's rule finds a link in it.
- **Titles cannot become syntax.** Titles are escaped the same way. A title
  holding a bracket is written as text beside a short `[open]` link, as the
  graph report does.
- **The note is never measured.** The read-only-notebook predicate keeps it out
  of lint, the graph report, publication handoffs and the next report.

**The tests.**

- `note_test.go` writes the note with a title built to break it:
  ``a](x) [[Twin]] <script>*b*``. Every link the extractor finds in the note is a
  `document://` or `resource://` URI the report wrote. Regenerating revises the
  same note. Neither lint's counts nor the next report's digest move. Quoting
  unescaped fails the test.
- `internal/httpapi/link_report_test.go` marks every piece of note text and finds
  none of the markers in the REST response. It checks that writing is a POST,
  that the note is not editable and an edit is refused, and that MCP has no such
  tool.
- `cmd/notriosctl/links_test.go` runs the built command, checking the quoted
  link, the stable URLs, the caps, the refusals of bad limits, and
  `--write-note --quiet`.
- `j40d-library-health-reports.test.tsx` checks the GUI:
  - opening the screen writes nothing;
  - each report is written on request, and the screen says what it found;
  - the note opens;
  - a failure shows as a failure.

**What else moved with the new surface.** Each record that counts surfaces was
updated, and says why:
- docgen's registry counts: 93 CLI forms, 111 REST operations;
- G18's route totals;
- G18f's hashes of the regenerated pages;
- the G18a inventory;
- FEATURES.json, under "Keep a library healthy";
- the atlas entry for `internal/linkreport`;
- J4's surface review, which finds both routes described, documented and
  tested;
- J13's example classification;
- I8's freeze.

The freeze change is additive: one command and two routes, with nothing
removed, so it is compatible with every client.

The store's SQL for the report lives in `sqlite_lint_links.go` beside lint's own
link scan, rather than in a new file that calls the C API. G18 pins the number
of such files, and no item since has added one.

**Recorded under J8, not fixed here:** H15's crawl never opens Library Health. Its
controls are not in the inventory: the repair buttons were not, and the new
report buttons are not. The crawl found the same 89 controls after this slice.

### What the report costs (J40-C)

Measured on J32's collision shape, the worst case for this report. That shape is
5,500 notes in 5,050 folders. Every `index.md` claims an alias shared by 100
notes and links it by name, and every note also links a path that resolves
nowhere.

The run used `measure_collision.sh` at `de29307`, with HOME and XDG in a work
directory on HD2. It imported the corpus once, then ran the report five times,
alternating with lint and the graph report, which are the other whole-library
reads. The results are in `collision-cost.json`. Firefox was running throughout,
with a load average of 4 to 6, so the absolute times are this machine under that
load. The neighbouring reports ran under the same load.

| run on the collision library | wall time, median of 5 (range) | peak RSS |
|---|---:|---:|
| `notriosctl links report` | 0.92 s (0.83–0.94) | 36.7 MB |
| `notriosctl lint --quiet` | 0.55 s (0.51–0.67) | 22.6 MB |
| `notriosctl graph report` | 0.18 s (0.18–0.19) | 20.1 MB |
| `notriosctl links report --write-note` | 0.96 s | 37.4 MB |

**What it found.** The report found 5,000 ambiguous links in 5,000 notes. Every
one is an alias the import found 100 notes for (`found_by: import`). It counted
5,000 other links that open nothing: the corpus's path links, which name no
file. This matches J39, where 5,000 of the 5,500 notes warned. All five runs gave
the same digest.

**The note.** Capped at 50 notes and 10 candidates per link, the Reports note is
54,867 bytes.

**Why it costs a little more than lint.** The report reads what lint reads, and
also rebuilds the import's namespace from 5,500 provenance rows. Each shared
candidate list is ordered and hashed once per name rather than once per link. A
list copied per link would have been 500,000 strings here.

It is a maintenance cost. It runs only when asked, and nothing runs it on a
timer or on save.

## J40-F: one section that lists every report

`docs/operations.md` now opens with **Reports**, a table of every report Notrios
produces. Each row says:

- what the report answers;
- the command, REST route, MCP tool and place in the interface for it, or
  "none";
- what it will quote from notes;
- what it costs, with the timings measured in J40-C for the reports that read
  every link.

Each row links to the section that explains the report. The table covers
lint, ambiguous links, the link graph, mechanical repairs, garbage collection,
resource references, the import report, the installation report and the
installation check. A new section, **Finding ambiguous links**, explains this
item's report. It also says plainly what the report quotes and why REST and MCP
do not carry it.

**The check.** `internal/docfeatures/reports_test.go` has two halves:

- **Every report is listed.** A command, REST route or MCP tool whose name says
  "report" must appear in the section, written as a reader would type it, such
  as `notriosctl paths --report`. Ten such surfaces exist today.
- **Every name is real.** Every command, route and `_report` tool the section
  names must exist, so a row cannot outlive the report it describes.

Reports with other names, such as lint, fix, gc and doctor, are listed by choice,
and the second half keeps those rows honest too. Three mutations are each
caught: dropping a listed route, naming a command that does not exist, and
naming a route that does not exist.

**The example runs.** The new section's recipe is executed by docexec. The
seeded library has no ambiguous link, so the adapter gives it one: two notes
titled `Docexec twin`, and a note linking `[[Docexec twin]]`. It then checks:

- the link is quoted with both candidates;
- `--candidate-limit 3` holds;
- `--write-note --quiet` prints nothing and writes the note into Reports;
- the seeded note is untouched.

Expecting the wrong text fails it. `links report` therefore joins the commands
with an executed published example. J13's count of commands without one falls
from 61 to 60.

**What moved with it:**
- docaudit: 290 sections, 173 executables, 79 executed, 363 unverified,
  denominator 493;
- G18a's denominator: 290;
- G18d's report: 173 entries, 71 executed;
- the example decisions;
- J13's classification;
- J4's review.

J9 checks this section; it does not create it (decision 7).
