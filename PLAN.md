# Plan: v0.5 — Better editing, blocks, and graph UX

Status: **drafted 2026-08-05 from `ROADMAP.md` after v0.4 completed. E1 is
complete; E2–E9 require user approval.**

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

### E2. Workspace lint

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

### E3. Workspace fix

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

### E4. Graph traversal, paths, and visualization data

- Extend the graph slice into bounded traversal: neighbors at depth N, shortest
  path between two notes, and orphan/hub reports, all with explicit node and
  edge ceilings and a documented refusal when a request would exceed them.
- Keep it in SQLite. LadybugDB stays a research option and does not become a
  dependency for this milestone.
- Return data a client can render; do not put layout in the service.
- Profile against the 1M-link tier the v0.3 harness already generates.

Working state: every traversal is bounded, a refused request says which ceiling
it hit, and the 1M-link profile records latency and peak RSS.

### E5. Editor-pane link intelligence

- Rich link autocomplete: a bounded prefix/title search endpoint the editor
  calls while typing, returning stable IDs and titles, never whole bodies.
- Broken-link markers while editing: the client asks whether the targets in the
  current buffer resolve, in one bounded batch, without saving.
- Both surfaces are read-only and bounded; neither writes a revision.

Working state: autocomplete and markers work on the 500k library without a
per-keystroke whole-library query, and an offline client degrades to no markers
rather than to errors.

### E6. CodeMirror 6 migration decision

Explicitly a decision task, not an implementation one. `md-editor-rt` sits
behind an application-owned adapter precisely so this stays a measured choice.

- Establish what E5 could not do inside the current editor: source positions,
  in-editor Ctrl-click, inline widgets, AST-safe edits.
- Prototype the same two features on CodeMirror 6 + unified/remark/rehype behind
  the existing adapter, and measure bundle size, first paint, typing latency on
  a large note, and behaviour inside the Wails webview.
- Recommend migrate or stay, with the measurement, and record it in
  `PROJECT_DECISIONS.md`. A migration, if chosen, is its own approved slice.

Working state: the recommendation is backed by numbers from both editors on the
same notes, not by preference.

### E7. Embedded query blocks

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

### E8. Organizer UX: trash-first delete, restore, and hierarchical tag rename

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
- E1 is complete; the remaining tasks are not approved.
- E6 is a decision task. Approving E6 does not approve a CodeMirror migration.

## Scope control

Batch/organizer transactions and expanded MCP profiles (v0.6), record-level sync
and transports (v0.7), the deferred `movenotes-v3` compatibility bridge (v0.7
slice 3), authentication and multi-user deployment, Wails v3/mobile migration,
HTTP range downloads, and the official MCP Go SDK migration all remain outside
v0.5 unless the roadmap is deliberately revised.
