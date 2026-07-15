# v0.2 Task R3 — Schema v5: notebooks, tags, and search notebooks

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Scope

Store-layer slice of the Notrios notebook model (`NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`). REST/MCP exposure follows in task R5; the query language in R6.

## Changes

- Schema v5 (`migrations/0001_initial.sql`, embedded copy, `ensureSchemaV5` shim for v4 DBs): `notebooks` (nested, `icon_emoji`, `builtin`, `position`, case-insensitive sibling-unique names via `NOCASE` expression index), `tags` + `note_tags`, `search_notebooks` (query, `builtin`, `sort_anchor`), `documents.notebook_id` + index, backfill to default notebook.
- Bootstrap seeds: "Notes" (`nb_notes`, default, undeletable), "Help" (`nb_help`, builtin regular notebook, read-only content), search notebooks "All notes" (`snb_all_notes`, first, empty query) and "Trash" (`snb_trash`, last, reserved query `is:trashed`).
- Store API: notebook create/get/list/update/delete (builtin/default protection, cycle-safe moves, recursive delete moves notes to Trash and re-homes them to the default notebook), `MoveDocumentToNotebook` (Help protected), tag add/remove/list with live counts (orphan tags removed), search-notebook create/list/delete (builtin protection, sidebar ordering first→normal→last), `ListTrash`, `RestoreDocument` (FTS + links restored), `PurgeDocument` (trash-only; local-source-only guard lands with R4 provenance).
- `CreateDocumentRequest.NotebookID` (defaults to the "Notes" notebook); `Document.NotebookID` returned everywhere.
- Design docs aligned: Help clarified as a builtin regular notebook; reserved `is:trashed`/empty-query operators documented; `DATABASE_SCHEMA.md` v5 section marked implemented.

## Validation

`go test ./...` (including 9 new store tests and a v4→v5 upgrade test), `python3 scripts/check_required_files.py`, `bash scripts/validate-scaffold.sh`, `bash scripts/mvp_smoke.sh` — all passing.
