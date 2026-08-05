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

## Deferred decisions

- Exact Go SQLite driver.
- Exact MCP Go SDK version.
- Whether to add go-git, Fossil, or neither for versioned projections.
- Whether to add LadybugDB as a derived graph backend.
- Which backend, if any, to use for scalable generated-site search after the
  Bluge/Recoll/FTS adapter spike.
- Exact sync envelope encoding, encryption policy, retention horizon, blob
  chunk threshold, and per-set conflict policy (see `SYNCHRONIZATION.md`).
- Whether optional live co-editing merits a Yjs-compatible Go dependency.
- When Wails v3/mobile has sufficient stability and native evidence to replace
  the stable Wails v2 shell.
- Whether to publish a native C++/Qt client in this repository or a separate repository.


## Release hardening decision

Completed release ZIPs must include `web/dist/` but exclude `web/node_modules/`, runtime `data/`, SQLite database files, and `.git/`. Use `scripts/package_release.sh` and `scripts/check_release_zip.py` to avoid repeating the Task 9 packaging omission.
