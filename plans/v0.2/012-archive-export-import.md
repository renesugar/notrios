# v0.2 Task R12 — Query-scoped export and import-with-dry-run

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Changes

- Native archive format (`internal/archive`): `manifest.json` (format/version/query), `notebooks.json` (paths with per-level emoji), `notes/<id>.md` (front matter: id/title/notebook path/created/updated/tags/resources), `resources/<id>__<filename>`.
- Export: query-scoped via the query language (empty = All notes; e.g. `tag:todo` exports a search notebook's contents); notebook structure preserved including intermediate levels' emojis; referenced resources copied once.
- Dry run (`notriosctl import archive --dry-run`): classifies top-level archive notebook names as creates / merges (existing plain notebooks) / **conflicts** (builtin notebooks or notebooks holding externally-sourced notes, via new `store.NotebookHasSourcedDocuments`), and writes `import-config.json` with prefilled `"Name": "Name (imported)"` rename suggestions. Dry run writes nothing to the store.
- Import: validates post-rename names case-insensitively against source-bound notebooks **before any write** (ErrNameConflict on violation); creates missing notebook paths reusing existing levels case-insensitively; resources first so `resource://` links resolve; notes upserted idempotently as **plain local notes** — no provenance rows, so re-imported notes are purgeable and are notes, not references to the original data source.
- CLI: `notriosctl export archive [--query] <out-dir>`, `notriosctl import archive [--dry-run] [--write-config] [--import-config] <archive-dir>`.

## Scope note

Joplin RAW / Obsidian notebook-hierarchy population was deferred to the v0.3 importer-hardening milestone; those importers currently target the default notebook.

## Validation

`go test ./...` (round-trip test: export → conflict dry run → refused import → renamed import → structure/tags/resources verified → no provenance → idempotent re-import; plus query-scoped export test), `check_required_files`, `validate-scaffold`, `mvp_smoke.sh` — all passing.
