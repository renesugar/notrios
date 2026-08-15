# Project Decisions

## Accepted baseline decisions

1. Use Go for the companion REST/MCP service initially.
2. Use React/Vite for the built-in web UI.
3. Use `md-editor-rt` initially for polished Markdown edit/preview, with a wrapper to allow migration to CodeMirror 6 later.
4. Use SQLite + FTS5 as canonical managed-note storage/search.
5. Use a derived search sidecar for extraction and broad filesystem search. (Originally sist2; superseded by Recoll in the v0.2 Notrios redesign — see `RECOLL_INTEGRATION.md`.)
6. Use Joplin RAW Export Directory as the preferred Joplin bulk import source.
7. Treat Obsidian vault export as the best default human-readable export format.
8. Support Quartz publishing profiles for public subsets.
9. Implement media stop lists, quarantine, exact hash deduplication, and perceptual-hash hooks before automatic remote-media localization.
10. Archive completed plans under `plans/v<major>.<minor>/`.
11. License project code under Apache-2.0. GPL search tools such as
    Recoll/Xapian remain external processes and are not linked or redistributed.
12. Replace offset-backed unbounded cursors with keyset/snapshot pagination
    before large importer hardening.
13. Build native archive v2 from versioned manifests and immutable
    hash-addressed objects, then reuse that container for sync snapshots.
14. Use a small Notrios record-level replication core for v0.7. Marmot,
    Cachapa, Ygo, Nostr, and bitchat are design references or later adapters,
    not initial dependencies.
15. Treat rclone/shared folders as immutable object transports, never as the
    merge or deletion algorithm.
16. Publish the external archive-v2 compatibility contract (JSON Schemas,
    pinned fixtures, cross-version consumer tests) only after the v0.7
    snapshot/change container slice is final. That slice extends the container
    the contract would pin, and unknown record types are rejected, so a reader
    integrated earlier would refuse every archive written afterwards. Decided
    2026-08-05; v0.4 P6 moved to v0.7.
17. Block anchor identity is strictly content-based: a block's ID derives from
    its text, so moving a block keeps its ID and editing its text mints a new
    one. An anchor therefore names exactly the text it was written against, and
    an edit breaks links into that block rather than silently redirecting them
    at replaced content. The hash is scoped to the document and disambiguated by
    occurrence, taken over text with line endings normalized and trailing
    whitespace trimmed. Author-written Obsidian `^markers` continue to resolve
    first, because they are names the author chose. Decided 2026-08-05 for
    v0.5 E1.
18. Workspace lint and fix stay single-note and revision-preconditioned in
    v0.5: every fix writes an ordinary revision against a precondition for one
    note, and no multi-note apply path is added. Bulk operations belong to the
    v0.6 organizer, which has its own atomic/best-effort contract. Decided
    2026-08-05 for v0.5 E2/E3.
19. **Emit slugs; accept percent-encoded anchors inside URI-schemed links
    only.** Obsidian addresses a heading by its text, percent-encoded into the
    URI (`file=Note%23Heading`). Notrios emits the slug: a `notrios://` heading
    anchor produced by this application is always `#some-heading`, and
    resolution normalizes heading text to the same slug so hand-written and
    imported anchors agree. Slugs are disambiguated by occurrence within a note.

    Percent-escapes are *read* as escapes only when the link carries one of
    Notrios' URI schemes (`notrios://`, `document://`, `resource://`). That
    scheme is the declaration — percent-encoding is defined for URIs, and RFC
    3986 already says what `%20` means there — so no new prefix syntax was
    invented to mark encoded links. In a bare Markdown target or anchor the
    bytes stay literal: `[x](#100%20off)` is text the author typed, and
    rereading it would be guessing.

    Identifiers are unaffected. `validateID` still refuses escapes outright,
    because a wrong decode there opens the *wrong note*, while a wrong decode in
    an anchor merely fails to find a section in the right one and is reported by
    lint. Decoding a link *target* would need an explicit marker that this
    asymmetry does not justify for anchors. Decided 2026-08-06 for v0.5 E1a and
    amended the same day for E1b.

20. **Stay on `md-editor-rt`. There is no CodeMirror migration to make,
    because `md-editor-rt` is CodeMirror.** v0.5 E6 was scheduled to weigh
    migrating from `md-editor-rt` to `CodeMirror 6 + unified/remark/rehype`.
    The premise was false. `md-editor-rt` 6.5.3 depends on
    `@codemirror/{view,state,autocomplete,commands,language,search}` 6.x and
    exposes them: `completions` takes `CompletionSource`s straight into
    `@codemirror/autocomplete`, `config({ codeMirrorExtensions })` accepts
    arbitrary extensions, `getEditorView()` returns the `EditorView` itself, and
    `domEventHandlers` is CodeMirror's own handler map.

    All three capabilities E5 reported as unreachable — caret position, inline
    decorations, in-editor Ctrl-click — were reachable through that public API.
    E6 implemented them rather than only arguing about them: `[[` autocomplete
    inside the editor, wavy underlines on broken links that move with their
    text, and Ctrl-click to open a link's target.

    Measured on a 206,549-character note in real headless Chrome, three runs per
    arm:

    | | Bundle (eager, gzip) | Keystroke p50 | Keystroke p95 |
    |---|---:|---:|---:|
    | before | 259.64 kB | 30.9 ms | 71.4 ms |
    | after | 260.94 kB | 30.0 ms | 67.4 ms |

    The three features cost **1.3 kB gzipped** and nothing measurable in typing
    latency, because the library was already in the bundle. Declaring
    `@codemirror/*` as direct dependencies pinned to the versions already
    installed produced a byte-identical build and a single copy of each package,
    which is the outcome that matters: two copies of `@codemirror/state` break
    CodeMirror at runtime.

    Migrating would therefore buy nothing in capability. What it would buy is
    the removal of md-editor-rt's other cargo — `markdown-it` and its plugins,
    `xss`, `lucide-react`, `medium-zoom`, and above all
    `@codemirror/language-data`, which contributes **113 lazy chunks totalling
    1.32 MB raw / 480 kB gzipped** of syntax modes for languages a note editor
    rarely meets. Those are lazily loaded, so they cost distribution size rather
    than first paint. And it would cost re-implementing the preview renderer,
    the sanitizer, the toolbar, image upload, and theming — all of which work
    today.

    The adapter stays worth keeping: this decision was cheap to reach precisely
    because the editor sits behind one. Revisit only if distribution size becomes
    a real constraint, and try trimming `@codemirror/language-data` first.
    Decided 2026-08-06 for v0.5 E6.

    A correction belongs with this decision. E5 recorded that the editor
    "exposes no caret position and accepts no inline widgets" and that claim
    reached `UI_DESIGN.md`, `PLAN.md`, `CODING_CLIENT_HANDOFF.md`, and the
    archived slice. It came from reading part of the editor's exposed interface
    and not the rest of it. It was wrong, it was the premise E6 inherited, and
    every document carrying it has been corrected.

## Deferred decisions

- Exact Go SQLite driver.
- Exact MCP Go SDK version.
- Whether to add go-git, Fossil, or neither for versioned projections.
- Whether to add LadybugDB as a derived graph backend.
- Which backend, if any, to use for scalable generated-site search after the
  Bluge/Recoll/FTS adapter spike.
- Exact sync envelope encoding, encryption/signature policy, revision-delta
  merge, retention horizon, resource chunk threshold, and per-set conflict
  policy (see the owning G0-G17 items in `PLAN.md`).
- Physical full-snapshot representation after the G14 scale gap: G14b must
  choose packed semantic archive-v2, a compatible SQLite-image capability with
  packed external assets and semantic fallback, or a further repository
  investigation from full-corpus evidence. G14c and all later sync work are
  blocked on that recorded choice.
- Whether optional live co-editing merits a Yjs-compatible Go dependency.
- When Wails v3/mobile has sufficient stability and native evidence to replace
  the stable Wails v2 shell.
- Whether to publish a native C++/Qt client in this repository or a separate repository.


## Release hardening decision

Completed release ZIPs must include `web/dist/` but exclude `web/node_modules/`, runtime `data/`, SQLite database files, and `.git/`. Use `scripts/package_release.sh` and `scripts/check_release_zip.py` to avoid repeating the Task 9 packaging omission.
