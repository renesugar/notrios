# v0.2 Task R4 — Schema v6: source provenance and threads

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Scope

Provenance layer for externally-sourced notes so Twitter/X conversation threads and ChatGPT/Claude conversations can be recovered, importer runs stay idempotent, and the Trash purge rule ("only local notes can be permanently deleted") is enforceable.

## Changes

- Schema v6: `document_sources` table (document_id PK/FK, source_system, external_id, author, author_id, thread_id, reply_to, source_url, published_at, derived published_ts, metadata_json) with `(source_system, external_id)`, thread, and author indexes; `ensureSchemaV6` version bump; `PRAGMA user_version = 6`.
- Store APIs: `SetDocumentSource` (upsert; allowed for trashed notes so importers can backfill), `GetDocumentSource`, `FindDocumentBySource`, `ListThreadDocuments` (chronological by published_ts then external_id; trashed notes excluded).
- `PurgeDocument` refuses externally-sourced notes with `ErrProtected`.
- Timestamp parsing accepts ISO 8601 (RFC3339/no-zone/date-only) and digit-only epoch seconds or milliseconds.
- Joplin importer records `joplin` provenance (note ID, author, source_url, user_created_time) on create/update/unchanged; Obsidian importer records `obsidian` provenance (vault-relative path). Re-running an import backfills provenance for previously-imported notes.

## Validation

`go test ./...` (3 new store tests + extended importer fixtures), `check_required_files`, `validate-scaffold`, `mvp_smoke.sh` — all passing.
