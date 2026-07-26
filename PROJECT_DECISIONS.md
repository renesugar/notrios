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
