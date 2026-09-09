# v0.5 E6 — CodeMirror 6 migration decision

Status: complete on 2026-08-06. **Recommendation: stay.**

Model: Claude Opus 5 (Claude Code).

Recorded as `PROJECT_DECISIONS.md` 20. Evidence: `performance/v0.5-e6/`.

## What the task was, and what it turned out to be

Weigh migrating from `md-editor-rt` to `CodeMirror 6 + unified/remark/rehype`,
prototype the E5 features on CodeMirror, measure, recommend.

The premise was false. **`md-editor-rt` 6.5.3 is CodeMirror 6.** It depends on
`@codemirror/{view,state,autocomplete,commands,language,search}` 6.x, and it
exposes them:

| Capability | How `md-editor-rt` provides it |
|---|---|
| autocomplete | `completions` prop takes `CompletionSource`s straight into `@codemirror/autocomplete` |
| arbitrary extensions | `config({ codeMirrorExtensions })` |
| the editor itself | `getEditorView(): EditorView` |
| DOM events with positions | `domEventHandlers`, CodeMirror's own handler map |

So the question "should we migrate to CodeMirror to get source positions and
inline widgets" has no content: we are on CodeMirror and those are available.

## The correction this forced

E5 recorded that the editor "exposes `insert` at the caret but nothing about
where the caret is" and "accepts no inline widgets", and offered that as E6's
measured input. It was wrong. It came from reading part of the editor's exposed
interface — `insert`, `focus` — and not the rest of it, where
`getEditorView`, `domEventHandlers`, and `getSelectedText` sit.

The claim had reached `UI_DESIGN.md`, `PLAN.md`, `CODING_CLIENT_HANDOFF.md`,
`agent/PLAN_STATUS.md`, `FEATURE_MATRIX.md`, and the archived E5 slice. Every
one has been corrected. A wrong fact about what a tool can do is worse than an
open question about it, because nobody goes back to check a settled one.

## What was built, and why it was kept

A decision task needs the prototype to be real or the numbers mean nothing. The
prototype turned out to run on the editor already shipping, so throwing it away
would have discarded working features to preserve a category:

- **`[[` autocomplete inside the editor.** Typing `[[kit` opens CodeMirror's own
  completion popup against `GET /api/v1/links/suggest`. It inserts a canonical
  `[Title](document://…)` link, replacing the trigger — the wikilink spelling is
  the muscle memory, the canonical URI is what survives a rename.
- **Wavy underlines on broken links.** A `StateField` holding a `DecorationSet`,
  refreshed whenever `POST /api/v1/links/check` returns, and **mapped through
  document changes** so an underline follows its text while typing continues
  rather than sitting at a stale offset.
- **Ctrl-click to open a link's target**, through `posAtCoords` — the exact
  source-position access E5 reported as unavailable.

No editor was replaced. This is not the migration, and the migration remains
unapproved.

## Decisions worth recording

**Byte offsets are not editor positions.** The service locates links by UTF-8
byte offset, because that is what Go measures; CodeMirror counts UTF-16 units,
because that is what a JavaScript string is. They agree on ASCII and diverge on
the first accent. Underlining with an unconverted offset marks the wrong text,
and marks it further off the further into the note the link sits.
`web/src/editor-offsets.ts` is the conversion, resolving all offsets in one walk
of the text, and it **drops** any offset landing inside a character rather than
rounding: a missing underline is a smaller error than one in the wrong place.

**A stale check marks nothing.** `applyBrokenLinks` compares the document
against the exact body the service classified and refuses when they differ —
the same rule the publication rewriter and `notriosctl fix` follow for a moved
span.

**The list beside the text stayed.** E5 shipped a located list of broken links
because it believed underlines were impossible. Underlines are possible now, and
the list stayed anyway: an underline says "something here is wrong" only where
you happen to be looking, while the list says how many, where, and why — for a
note longer than the screen, which is most of them.

**Declared dependencies must not duplicate CodeMirror.** Importing
`@codemirror/*` as a transitive dependency works but is fragile; declaring it
risks npm resolving a second copy, and two copies of `@codemirror/state` break
CodeMirror at runtime. They were pinned to the versions already installed, and
the check is empirical: one copy of each on disk, and a byte-identical build.

## The measurement

Real headless Chrome, a 206,549-character note, three runs per arm.

| Arm | keystroke p50 | keystroke p95 | eager bundle (gzip) |
|---|---:|---:|---:|
| before (E5) | 30.9 ms | 71.4 ms | 259.64 kB |
| after (E6) | 30.0 ms | 67.4 ms | 260.94 kB |

**1.3 kB gzipped and nothing measurable in typing latency.** The reason to say
"nothing measurable" rather than "faster" is that first paint alone ranged
160–528 ms across three identical *before* runs — the within-arm spread is
larger than the between-arm difference.

What migrating would still buy: removing `@codemirror/language-data`, which
contributes **113 lazy chunks, 1.32 MB raw / 480 kB gzipped** — 61% of the
distribution by size and 97% by file count — for syntax modes a note editor
rarely meets. They load only when a fenced block names their language, so they
cost distribution size rather than first paint. Not enough against
re-implementing the preview, sanitizer, toolbar, upload, and theming.

## What was not measured

**The Wails webview.** The task asked for it. WebKitGTK does not speak the
DevTools Protocol the harness uses and no WebKit remote-inspection tooling is
installed here. `make gui` builds with the extensions and the webview loads the
identical bundle through the service handler, so the code path is the one
measured — but the engine is not, and no number here describes it.

**`unified`/`remark`/`rehype`.** They render the *preview*, and the preview was
never what E5 could not do. Recorded separately: `remark-gfm`, `remark-math`,
`rehype-katex`, and `rehype-sanitize` are declared in `web/package.json` and
imported nowhere.

## Validation

`go vet ./...`, `go test ./...`, required-files, plan-loop, scaffold validation,
OpenAPI parse, migration-copy equality, web typecheck/tests/build, `make gui`,
docs-site build, Help reseed, REST/MCP smoke, performance smoke, and three runs
per arm of `scripts/run_editor_profile.sh`.
