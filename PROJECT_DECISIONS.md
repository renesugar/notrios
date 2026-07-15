# Project Decisions

## Accepted baseline decisions

1. Use Go for the companion REST/MCP service initially.
2. Use React/Vite for the built-in web UI.
3. Use `md-editor-rt` initially for polished Markdown edit/preview, with a wrapper to allow migration to CodeMirror 6 later.
4. Use SQLite + FTS5 as canonical managed-note storage/search.
5. Use sist2 as a derived sidecar for OCR, thumbnails, archive traversal, and broad filesystem search.
6. Use Joplin RAW Export Directory as the preferred Joplin bulk import source.
7. Treat Obsidian vault export as the best default human-readable export format.
8. Support Quartz publishing profiles for public subsets.
9. Implement media stop lists, quarantine, exact hash deduplication, and perceptual-hash hooks before automatic remote-media localization.
10. Archive completed plans under `plans/v<major>.<minor>/`.

## Deferred decisions

- Final license.
- Exact Go SQLite driver.
- Exact MCP Go SDK version.
- Whether to add go-git, Fossil, or neither for versioned projections.
- Whether to add LadybugDB as a derived graph backend.
- Whether to add Bleve for fuzzy/faceted search beyond FTS5.
- Whether to publish a native C++/Qt client in this repository or a separate repository.


## Release hardening decision

Completed release ZIPs must include `web/dist/` but exclude `web/node_modules/`, runtime `data/`, SQLite database files, and `.git/`. Use `scripts/package_release.sh` and `scripts/check_release_zip.py` to avoid repeating the Task 9 packaging omission.
