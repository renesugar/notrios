# Plan: v0.5 — Better editing, blocks, and graph UX

Status: **active. Drafted 2026-08-05 from `ROADMAP.md` after v0.4 completed.
E1, E1a, E1b, E2, E3, E4, E5, and E6 are complete. E6a and E6b were added
2026-08-06 from what E6 measured and from a user question about math rendering;
E6a, E6b, E7, and E8 are complete; E9 requires user approval.**

v0.4 is complete and archived under `plans/v0.4/`, including a copy of its own
plan at `plans/v0.4/000-v0.4-plan.md`. Its one deferral, P6 (the `movenotes-v3`
archive compatibility bridge), moved to v0.7 slice 3 rather than into this
milestone: it is gated on the sync container, not on editing.

## Goal

v0.4 made the library portable and safe to move. v0.5 makes it better to *work
in*: addressable blocks, links that tell you while you type whether they will
resolve, a graph you can traverse and see, and bounded maintenance tools that
find and fix what has rotted.

Two constraints carry over unchanged and shape every task below:

1. **Nothing unbounded.** Every new list, traversal, and report pages through a
   keyset or an explicit bound, and every one gets a generated-scale profile.
   The 500k-note tier is the reference, not a hundred-note fixture.
2. **Note content is untrusted.** Query blocks, lint fixes, and templates are
   structured and permission-controlled; none of them becomes a way for note
   text to execute anything or to reach arbitrary filesystem paths.

## Working-state rule

Unchanged from v0.4. Complete one task at a time. Every task updates tests and
living docs, runs its relevant validation, records attempt/model status, commits
a working slice, archives it under `plans/v0.5/`, produces a verified ZIP, and
asks for approval before the next task.

## Tasks

### E1. Block anchors and block-level addressability — complete

Archived as `plans/v0.5/001-block-anchors.md`. Evidence:
`performance/v0.5-e1/`. Anchor resolution measured flat (0.3 ms p95) from
61,000 to 3,001,200 block rows; blocks roughly double the database at six blocks
per note, which is what a second index over every note's text costs.

The foundation the rest of the milestone leans on. `document_blocks` has been
"planned" since the MVP schema; heading and block anchors currently live only on
link records.

**Block identity is content-based** (decided 2026-08-05). A block's ID is
derived from its text, not from its position: moving a block within a note
keeps its ID, and editing a block's text mints a new one, so an anchor always
names exactly the text it was written against. An edit therefore breaks
anchors into that block — that is the intended trade, and links to it become
unresolved rather than silently pointing at replaced text.

Three consequences follow, all settled here rather than during implementation:

- **Duplicate text needs a disambiguator.** Two identical paragraphs in one
  note would otherwise share an ID. The hash covers the document ID, the block
  kind, the normalized text, and the occurrence index among identical blocks in
  that note, so identity stays deterministic and scoped to its note.
- **Normalization is part of the contract.** The hash is taken over text with
  line endings normalized and trailing whitespace trimmed, so re-saving a note
  through an editor that cleans whitespace does not silently break every anchor
  in it. Anything beyond that — case, punctuation, Markdown emphasis — is
  content, and changing it changes identity.
- **Authored `^markers` still win.** Obsidian-style block markers already
  arrive through the importer and already resolve as link anchors. They are
  identifiers the author chose and they survive edits to the block's text, so
  resolution tries the authored marker first and the content hash second.
  Content-based identity governs blocks Notrios names itself; it does not
  overrule a name the author wrote.

- Add schema v14 `document_blocks`: content-derived block IDs, document,
  ordinal, kind (heading/paragraph/list-item/code/table), heading level, byte
  range, authored marker when present, and the content hash, rebuilt in the same
  transaction as a note save.
- Parse blocks deterministically from canonical Markdown, reusing the existing
  extractor's conventions so a block anchor and a link anchor agree.
- Implement `GET /api/v1/documents/{id}/blocks` and block-scoped backlinks;
  extend `document://` and `notrios://` resolution to block anchors.
- Keep the rebuild proportional to one note, and profile the largest tier the
  harness generates: block rows will outnumber notes by an order of magnitude,
  so index choice and row size matter more than in any earlier table.

Working state: a block anchor resolves to a stable position across edits that do
not touch it, block rows never outlive their document, and the 500k profile
records row counts, database growth, and save latency against the v0.3 baseline.
Heading anchors were out of scope here and are added by E1a.

### E1a. Heading anchors in stable links — complete

Archived as `plans/v0.5/003-heading-anchors.md`. Evidence:
`performance/v0.5-e1a/`. Adding heading anchors cost nothing measurable at
100,000 notes: the slug is one nullable column on an existing table and the
check rides E2's single link scan.

E1 made blocks addressable and E2 had to leave heading anchors unchecked: a
heading anchor is a slug, and block rows store a content hash rather than
heading text, so there was nothing to compare against. This closes that gap so
`notrios://` and `document://` links can name a section, not only a note or a
paragraph.

**What Obsidian does** (verified against `obsidian.md/help/links` and the
Obsidian URI help page on 2026-08-06):

- internal links name a heading by its **text**: `[[Note#Heading Text]]`, with
  nested subheading paths (`[[Note#H1#H2]]`) and a Markdown form
  `[Section](Example.md#Details)`;
- blocks use `[[Note#^block-id]]`, and block IDs are Latin letters, numbers, and
  dashes;
- the `obsidian://open` URI supports both by percent-encoding the anchor onto
  the file parameter: `file=Note%23Heading` and `file=Note%23%5EBlock`, with
  `%23` for `#`, `%5E` for `^`, `%20` for space, `%2F` for `/`.

Two claims in the source material were **not** verified and are not relied on:
that an Alt/Option-modified context menu yields "Copy obsidian URI", and that
the Advanced URI plugin exposes explicit `heading=`/`block=` parameters — that
plugin's schema page documents only the general form and `vault`.

**What Notrios adopts, and what it deliberately does not.** Notrios takes the
model — a heading is addressable by name, a block by ID, both usable in an
external URI — but not the percent-encoded heading text. P5 decided that the
stable-link parser rejects percent-encoding rather than decoding it, because an
identifier needing escapes is not one this application minted and decoding lets
two spellings name one target. A heading anchor in a `notrios://` link is
therefore a **slug**: `notrios://databases/{db}/documents/{doc}#some-heading`,
URI-safe by construction and the same convention Markdown, Joplin, and GitHub
already use for in-note section links. Resolution normalizes whatever the caller
wrote, so a `document://…#Heading Text` link written by hand or produced by the
Obsidian importer resolves to the same heading.

- Add schema v15 `document_blocks.heading_slug`, derived from heading text with
  the documented normalization and disambiguated by occurrence (`slug`,
  `slug-1`, …) so repeated headings in one note stay addressable.
- Resolve heading anchors in `notrios://` and `document://` links, reusing E1's
  precedence: an author-written `^marker` first, then a block ID, then a heading
  slug.
- Restore the `unresolved_heading_anchor` lint check E2 could not implement.
- Expose the slug through `GET /api/v1/documents/{id}/blocks`, and let
  `notriosctl link` emit an anchored stable link after checking it resolves.
- Keep the anchor charset unchanged: no percent-encoding, no spaces, no new
  parser surface.

Working state: a heading anchor written as a slug or as heading text resolves to
the same heading; a renamed heading reports `stale_anchor` rather than silently
opening the top of the note; and lint reports heading anchors that no longer
match.

### E1b. Scheme-scoped anchor decoding — complete

Archived as `plans/v0.5/005-scheme-scoped-anchor-decoding.md`.

E1a left `stablelink.Parse` accepting `#Kitchen%20Plan` and handing back an
anchor that could never resolve: the parser refused escapes in identifiers but
not in anchors, so a percent-encoded anchor was accepted and then silently
missed every heading. Rather than either rejecting it or decoding everywhere,
the scheme decides. Percent-escapes are read as escapes only inside a
`notrios://`, `document://`, or `resource://` link, because that is where
something declared itself a URI and RFC 3986 already defines what `%20` means.
A bare Markdown anchor stays literal.

This is the "unique prefix" idea using the prefix Notrios already has. A new
marker (`notrios+q:`) was considered and rejected: it would not help the case
that motivated decoding — a pasted Obsidian URI carries no Notrios prefix — and
it would add a second link spelling to the 18 non-test files that interpret a
target or an anchor.

- Decode `%XX` in anchors reached through a URI-schemed link: stable-link
  resolution, the lint heading check, and backlink counting.
- Leave link *targets* literal. A wrong decode there opens the wrong note; a
  wrong decode in an anchor lands in the right note and lint reports it. That
  asymmetry is why targets would need an explicit marker and anchors do not.
- Leave invalid escapes (`%zz`, a trailing `%`) exactly as written: a heading
  with a literal percent sign is likelier than a typo in an escape, and deleting
  bytes would break a link invisibly. `+` is not a space.
- Keep `Parse` returning the anchor as written, so a link round-trips byte for
  byte and decoding stays a resolution-time reading.

Working state: `notrios://…#Install%20%26%20Setup` and `#install-setup` reach the
same heading, the same bytes in a bare Markdown anchor do not, a heading
containing a real percent sign still resolves, and identifiers still refuse
escapes.

### E2. Workspace lint — complete

Archived as `plans/v0.5/002-workspace-lint.md`. Ten checks through
`notriosctl lint` and `GET /api/v1/admin/lint/report`, with no apply surface.
Findings locate problems without quoting them; the detail cap hides examples
while counts and `report_sha256` describe the whole library. Heading anchors are
deliberately unchecked — a heading anchor is a slug and block rows store a hash,
so there is nothing to compare against; that needs a stored slug and its own
slice.

Read-only first, exactly as resource GC was. A report that cannot mutate is also
a report that is safe to run on a library nobody has backed up yet.

- Detect the checks `WORKSPACE_MAINTENANCE.md` lists: broken document/resource
  links, ambiguous wikilinks, unresolved block references, duplicate IDs,
  missing titles, unlocalized remote images, unreferenced resources, missing
  alt text, and projection/index drift.
- One bounded Store operation with typed findings, stable reason codes, capped
  detail arrays, and complete counts — the shape P1 established.
- Expose it through `notriosctl lint` and a read-only REST report. No fix
  actions in this slice.
- Profile the 500k tier: a lint pass is a whole-library read and must stay
  bounded in memory and explicit about its cost.

Working state: findings are deterministic, content-free at the API boundary, and
reproducible; nothing about the library changes.

### E3. Workspace fix — complete

Archived as `plans/v0.5/004-workspace-fix.md`. `notriosctl fix` is dry-run by
default, repairs one note at a time against the revision its plan was computed
from, and writes an ordinary revision per fix. Non-canonical link targets are
repaired by default; alt text and remote-media localization are opt-in — the
first because a filename is not a description, the second because it reaches the
network and runs through the full media policy.

Stale link reference definitions are **not** fixed, because they are not
detected: reference definitions are outside the link extractor, so E2 has no
check for them, and a repair for something lint cannot find would be fixing
blind. The check and its fix belong together in a later slice.

- Implement the apply half for the mechanically safe subset only: unlocalized
  remote images (through the existing media policy), missing alt text,
  normalizable link syntax, and stale link reference definitions.
- Dry run is the default and prints the exact edit; apply requires explicit
  confirmation and a revision precondition per note, so a concurrent edit fails
  rather than being overwritten.
- Every fix writes an ordinary revision. There is no silent rewrite path, and
  nothing bypasses the media policy or the link resolver.
- Findings the tool cannot fix safely stay reported and unfixed rather than
  guessed at.

Working state: a fix run is reproducible, reversible through revision history,
and refuses when the note changed under it.

### E4. Graph traversal, paths, and visualization data — complete

Archived as `plans/v0.5/006-graph-traversal.md`. Evidence:
`performance/v0.5-e4/`. The bounded operations measured flat across a fifty-fold
library (depth-5 expansion 18.6 → 19.2 ms p95 from 10k to 500k notes, 1,000,200
links) and peak RSS was unchanged; the whole-library report is linear at
0.21/3.92/21.6 s, the same shape as lint.

A document reconciliation before this slice (see "Document reconciliation
2026-08-06" below) found the concrete defect E4 had to fix first: **`depth` was
accepted and silently ignored.** `POST /api/v1/graph` declared `depth` in
`api/openapi.yaml` (minimum 0, maximum 5, default 1) and carried it through
`internal/api.GraphRequest` into `store.GraphRequest`, and `store.Graph` never
read the field — every graph slice was the roots' immediate neighbours. The same
schema declared `max_nodes: 250` and `max_edges: 500` while the store defaults
were 100 and 200. A request asking for depth 3 got depth 1 with no warning,
which is worse than a refusal: the caller cannot tell a shallow graph from a
small one.

- Extend the graph slice into bounded traversal: neighbors at depth N, shortest
  path between two notes, and orphan/hub reports, all with explicit node and
  edge ceilings and a documented refusal when a request would exceed them.
- Honour `depth` or reject it. Reconcile the OpenAPI bounds and defaults with
  the store's in the same change, so the declared contract and the implemented
  one are the same contract.
- Keep it in SQLite. LadybugDB stays a research option and does not become a
  dependency for this milestone.
- Return data a client can render; do not put layout in the service.
- Profile against the 1M-link tier the v0.3 harness already generates.

Working state: every traversal is bounded, a refused request says which ceiling
it hit, and the 1M-link profile records latency and peak RSS.

Three decisions came out of the implementation. A request naming a bound wider
than a ceiling is refused rather than clamped, because clamping reproduces the
original defect in another form. A traversal stopped by a ceiling reports
`truncated_by` and `completed_depth`, so a partial neighbourhood is never read as
a complete one. And a shortest-path search separates `no_path` — a proof that
everything reachable was searched — from `depth_exhausted` and
`budget_exhausted`, which only say the search stopped.

### E5. Editor-pane link intelligence — complete

Archived as `plans/v0.5/007-editor-link-intelligence.md`. Evidence:
`performance/v0.5-e5/`.

- Rich link autocomplete: a bounded prefix/title search endpoint the editor
  calls while typing, returning stable IDs and titles, never whole bodies.
- Broken-link markers while editing: the client asks whether the targets in the
  current buffer resolve, in one bounded batch, without saving.
- Both surfaces are read-only and bounded; neither writes a revision.

Working state: autocomplete and markers work on the 500k library without a
per-keystroke whole-library query, and an offline client degrades to no markers
rather than to errors.

Four things came out of the implementation. The buffer check takes the **body**
rather than a client-extracted target list, because deciding what is a link
belongs to the canonical extractor and a second implementation would draw
markers that disagree with what a save records. Anchors into the note being
edited resolve against the **submitted body**, since while someone types the
buffer is the truth about its own headings. Schema **v16** replaced
`lower(title) = lower(?)` with a NOCASE index — the same comparison, but the old
one made every title-resolved link a full scan of the document table on every
save and every lint pass. The slice also produced a claim about the editor that **E6 found to be wrong**:
E5 recorded that `md-editor-rt` gives no caret position and no inline widgets.
It gives both. See E6 and `PROJECT_DECISIONS.md` 20.

### E6. CodeMirror 6 migration decision — complete

Archived as `plans/v0.5/008-codemirror-decision.md`. Evidence:
`performance/v0.5-e6/`. **Recommendation: stay.** Recorded as
`PROJECT_DECISIONS.md` 20.

Explicitly a decision task, not an implementation one. `md-editor-rt` sits
behind an application-owned adapter precisely so this stays a measured choice.

**The task's premise was false, and finding that out was the task.**
`md-editor-rt` 6.5.3 *is* CodeMirror 6: it depends on
`@codemirror/{view,state,autocomplete,commands,language,search}` 6.x and exposes
them through `completions`, `config({ codeMirrorExtensions })`,
`getEditorView()`, and `domEventHandlers`. E5's claim that the editor gives no
caret position and no inline widgets was wrong; both are available.

- Establish what E5 could not do inside the current editor: source positions,
  in-editor Ctrl-click, inline widgets, AST-safe edits.
- Prototype the same two features on CodeMirror 6 + unified/remark/rehype behind
  the existing adapter, and measure bundle size, first paint, typing latency on
  a large note, and behaviour inside the Wails webview.
- Recommend migrate or stay, with the measurement, and record it in
  `PROJECT_DECISIONS.md`. A migration, if chosen, is its own approved slice.

Working state: the recommendation is backed by numbers from both editors on the
same notes, not by preference.

Because the prototype had to be real to be evidence, and because it turned out
to run on the editor already shipping, it was kept: `[[` autocomplete inside the
editor, wavy underlines on broken links that move with their text, and Ctrl-click
to open a target. Cost: **1.3 kB gzipped and nothing measurable in typing
latency** over three runs per arm on a 206,549-character note in real headless
Chrome. That is not a migration — no editor was replaced — and the four
capabilities the task listed are now demonstrated rather than argued about.

What migrating would still buy is removing `@codemirror/language-data`, which
md-editor-rt pulls in for code-block highlighting and which contributes 113 lazy
chunks totalling 1.32 MB raw / 480 kB gzipped. They load only when a fenced block
names their language, so they cost distribution size rather than first paint —
not enough to justify re-implementing the preview, sanitizer, toolbar, upload,
and theming that work today.

### E6a. Offline-first frontend assets — complete

Archived as `plans/v0.5/009-offline-frontend-assets.md`. Evidence:
`performance/v0.5-e6a/`. Third-party requests 13 → 0 and 623 kB → 0; the eager
bundle grew 151 kB gzipped and first contentful paint regressed ~400 ms, which
is reported rather than buried. `scripts/run_offline_assets_check.sh` was
verified to fail on the pre-fix commit and pass on the fixed tree.


**A defect found while measuring E6, not a feature.** Notrios binds to loopback
by default, quarantines remote *images* behind a media policy, and refuses to
fetch a byte during a static media scan. Meanwhile the frontend loads remote
**executable JavaScript** from a third-party CDN, unconditionally, on every
launch — including inside the Wails desktop app.

Measured: opening one note issues thirteen requests to `unpkg.com` — six
external scripts and three stylesheets, plus three KaTeX fonts.

| Loaded from unpkg.com | Used by Notrios? |
|---|---|
| `katex.min.js` + css + 3 fonts | yes — `$…$` and `$$…$$` render through it |
| `highlight.min.js` + theme css | yes — every fenced code block |
| `echarts.min.js` | no |
| `cropper.min.js` + css | only the image-crop dialog |
| `prettier/standalone.js` + markdown plugin | no |

With the CDN blocked and a cold cache, `window.katex` is `undefined`, no
`.katex` element is produced, and a formula silently renders as its raw LaTeX
source. Code blocks lose highlighting the same way. No error, no placeholder.

`md-editor-rt` does not bundle these: its default config points at
`https://unpkg.com/...` and injects `<script>`/`<link>` tags at runtime. It
accepts local instances instead, which is the fix.

- Supply local `katex` and `highlight.js` instances through
  `config({ editorExtensions: … })`, and declare both as direct dependencies
  pinned to the installed versions — with the same duplicate check E6 used,
  because a second copy of a library is a runtime failure rather than a size
  regression.
- Turn off what Notrios does not use: `noEcharts`, `noPrettier` (`noMermaid` is
  already off). Decide cropper deliberately — localize it or drop the crop
  dialog; do not leave it fetching.
- Remove `remark-gfm`, `remark-math`, `rehype-katex`, and `rehype-sanitize` from
  `web/package.json`. They are declared and imported nowhere, and they cannot be
  plugged into `md-editor-rt`, which renders through markdown-it. Preview
  sanitization is already Notrios' own `normalizePreviewHTML` passed as the
  `sanitize` prop.
- Add the regression guard that makes this stay fixed: a test asserting **zero
  external requests** on a note load, and a `Content-Security-Policy` from the
  service restricting `script-src`/`style-src`/`font-src` to `'self'`. A test
  catches a regression; the header prevents one.
- Record the bundle cost honestly — KaTeX and highlight.js move roughly 300 kB
  of JavaScript plus fonts into the distribution, against removing every
  external request and making math and highlighting work offline.

Working state: a note containing `$E = mc^2$` and a fenced code block renders
identically with the network unplugged, the page issues no third-party request,
and the CSP would reject one if a future change tried.

### E6b. Paste normalization: HTML tables to Markdown — complete

Archived as `plans/v0.5/010-html-table-paste.md`. The converter refuses far more
than it converts — merged cells, ragged rows, nested blocks, multi-line cells, a
paste that merely contains a table — and every refusal falls through to the
ordinary paste, so nothing a user pastes can be lost. Verified end to end in
real headless Chrome with a genuine `ClipboardEvent`.


Pasting an HTML table into the editor works today in the sense that the preview
renders it — raw HTML is enabled and Notrios' sanitizer strips scripts, event
handlers, and inline styles. What lands in the **note source** is still raw
HTML, and that is where the cost sits:

- `internal/markdownblocks` sees the table as paragraphs, so nothing in it is
  block-addressable;
- `internal/markdownlinks` does not extract `<a href>` inside HTML, so those
  links are invisible to the graph, to lint, and to `notriosctl fix`;
- a publication handoff carries the raw HTML downstream to `movenotes-v3`,
  Obsidian, and Quartz, which is the least portable thing a note can contain.

Converting on paste puts the content in the format the rest of the system
already understands.

- Handle `paste` on the editor through the CodeMirror `domEventHandlers` E6
  established. When the clipboard offers `text/html` containing a table,
  convert it to a Markdown pipe table.
- Convert **simple tables only**: rectangular, no `colspan`/`rowspan`, no nested
  tables, no block elements inside cells. Anything else pastes unchanged —
  a mangled table is worse than an HTML one, and refusing is the same rule E3's
  fix and P7's rewriter follow for a span they cannot place.
- Parse to text; never re-emit HTML and never execute anything. Escape `|` in
  cell content and preserve inline links as Markdown.
- Keep it client-side and small. A server-side HTML-to-Markdown converter for
  importers is a different feature and is not in scope.

Working state: pasting a simple HTML table produces a Markdown table whose
blocks and links the rest of the system can see; a complex table pastes exactly
as it arrived; and no pasted markup is executed at any point.

### E7. Embedded query blocks — complete

Archived as `plans/v0.5/011-embedded-query-blocks.md`. A fenced ```note-query
block declares a Q1 query plus typed fields, sort, and limit; the service parses
it with the same parser every search surface uses, so a block can express
nothing its author could not type into the search box. A malformed block is a
200 carrying `error`, because the note has to render. E7 also gave
`SearchRequest` an explicit `Sort`: the order used to be implied by the query's
shape, so `sort: updated` over a text query would silently have got relevance.


- Render a fenced `note-query` block from the Q1 expression language plus a
  bounded, typed selection of fields, sorts, and limits. No SQL, no JavaScript,
  no filesystem reach.
- Evaluate at render time with a hard result cap and a visible truncation
  marker; a query block never blocks note loading.
- Keep the block declarative and inert in export: a publication handoff carries
  the block's text, not a materialized result, unless a profile explicitly asks
  for materialization.

Working state: a malformed or hostile query block renders an error inside the
note rather than failing the note, and no query block can read anything the
user's own search cannot.

### E8. Organizer UX: trash-first delete, restore, and hierarchical tag rename — complete

Archived as `plans/v0.5/012-organizer-ux.md`. The GUI reaches the store's
trash-first rule directly: Move to Trash, Restore, and Delete forever, with a
badge that distinguishes a trashed note from a permanently read-only one. A
notebook deletion confirms with `GET /api/v1/notebooks/{id}/deletion-preview` —
the service's own counts and the re-homing rule — rather than a generic "are you
sure". `POST /api/v1/tags/rename`, `store.RenameTag`, and `notriosctl tags
rename` give hierarchical rename a dry run that runs the real statements inside
a transaction and rolls them back, so a dry run and an apply cannot disagree.

- Make trash-first deletion and restore first-class in the GUI, including the
  notebook-deletion rehoming rule the store already implements.
- Add hierarchical tag rename with dry run, including child tags, through the
  Store/REST/CLI with counts before apply.
- Keep every bulk-shaped operation out of scope: batch organizer transactions
  are v0.6 and must not be pre-empted here.

Working state: a user can see and undo deletions without the CLI, and a tag
rename reports exactly what it will touch before touching it.

### E9. v0.5 documentation and release wrap-up

- Document blocks, lint/fix, graph traversal, editor behaviour, and query blocks
  in both the site and the Help notebook.
- Reconcile `FEATURE_MATRIX.md`, architecture/schema/API/security documents, and
  the release checklist.
- Run the full release validation, archive the plan, draft the next plan from
  `ROADMAP.md`, and produce a verified source ZIP.

Working state: documentation matches implementation and v0.5 release checks
pass.

## Document reconciliation 2026-08-06

Every living document was compared with the source tree before E4 started. Six
findings, five of them stale claims and one a real gap between a declared API
and its implementation:

1. `README.md`, `ROADMAP.md`, and `CONTEXT_MAP.md` all called v0.5 "unstarted"
   after five of its slices had shipped and been archived under `plans/v0.5/`.
   Corrected to "in progress"/"active" with the completed slices named.
2. `DATABASE_SCHEMA.md` said `migrations/0001_initial.sql` "creates through
   schema version **14**" while the same sentence's parenthetical already listed
   v15 and both migration copies end at `PRAGMA user_version = 15`. Corrected.
3. `agent/PLAN_STATUS.md` said "the schema is v13" in its active-milestone
   section, contradicting both its own working-state facts and
   `store.CurrentSchemaVersion = 15`. Corrected, with the v14/v15 provenance.
4. `README.md` listed committed evidence only through `performance/v0.4-p1/`
   and `performance/v0.4-p4/`; `v0.4-p5/`, `v0.4-p7/`, `v0.4-q1/`, and the three
   `v0.5-*` directories also exist. Corrected.
5. `POST /api/v1/graph` accepts `depth` and never uses it, and its OpenAPI
   node/edge defaults disagree with the store's. Recorded in E4 above rather
   than patched in isolation, because honouring `depth` *is* E4.

Nothing else disagreed. The REST route table, the eleven lint check names, the
MCP tool list, the blocks response fields, and the archive-v2 CLI surface all
match their documents, and `api/openapi.yaml` already carries the E1 blocks and
E2 lint routes.

## Baseline validation

```bash
go vet ./...
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build && npm test
bash scripts/build_docs_site.sh
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

Block, lint, graph, and editor tasks add their own generated-scale profiles;
GUI-affecting tasks also build with `make gui`, and layout changes run
`scripts/verify_layout_resize.py` under Xvfb/Openbox.

## Decisions required before or during v0.5

- **Resolved 2026-08-05: block identity is strictly content-based.** An anchor
  survives a block moving and breaks when the block's text changes. See E1 for
  the three consequences that follow, and `PROJECT_DECISIONS.md` decision 17.
- **Resolved 2026-08-05: lint and fix stay single-note and
  revision-preconditioned.** Every fix writes an ordinary revision against a
  precondition for one note; anything bulk belongs to the v0.6 organizer, and
  E3 may not grow a multi-note apply path.
- E1 through E6, E6a, E6b, E7, and E8 are complete; E9 is not approved.
- **Resolved 2026-08-06 by E8: a dry run is a rolled-back apply, not a
  prediction.** `RenameTag` runs its real statements inside a transaction and
  rolls back when `dry_run` is set, so the report comes from the code that does
  the work. Cascading tag merges are exactly where a separate predictor and
  applier drift apart.
- **Resolved 2026-08-06 by E6a: the frontend is offline-capable now.** KaTeX,
  highlight.js, and cropper are bundled, echarts and prettier are off, and a
  Content-Security-Policy from the service refuses third-party script. The cost
  is 151 kB gzipped and ~400 ms of first contentful paint, both measured and
  recorded.
- **Resolved 2026-08-06: heading anchors in stable links use a slug, not
  percent-encoded heading text.** Obsidian percent-encodes the heading name into
  its URI; Notrios keeps P5's refusal to decode percent-escapes, so the URI form
  is the slug and resolution normalizes heading text to it. See E1a and
  `PROJECT_DECISIONS.md` 19.
- **Resolved 2026-08-06: stay on `md-editor-rt`.** E6 found that it *is*
  CodeMirror 6 and exposes it, so the migration it was weighing does not exist
  as a capability question. See `PROJECT_DECISIONS.md` 20.

## Scope control

Batch/organizer transactions and expanded MCP profiles (v0.6), record-level sync
and transports (v0.7), the deferred `movenotes-v3` compatibility bridge (v0.7
slice 3), authentication and multi-user deployment, Wails v3/mobile migration,
HTTP range downloads, and the official MCP Go SDK migration all remain outside
v0.5 unless the roadmap is deliberately revised.
