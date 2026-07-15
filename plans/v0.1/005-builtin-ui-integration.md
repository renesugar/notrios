# Plan 005 — Built-in UI integration

## Goal

Wire the React/Vite scaffold to the live Step 4 REST persistence slice so a browser user can create, search, and open managed Markdown notes.

## Status

Completed in scaffold Step 5.

## Implementation summary

- Added typed REST client functions in `web/src/api.ts`.
- Replaced the placeholder UI with create-note, search-results, and opened-note panels.
- Added safe rendering for generated FTS `<mark>` snippets without arbitrary HTML injection.
- Added `web/README.md` and `SCAFFOLD_STEP5_REPORT.md`.
- Updated scaffold status and planning files.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm run typecheck
cd web && npm run build
```

## Follow-up tasks

- Replace textarea with `md-editor-rt` and app-controlled preview component.
- Add document update/autosave and revision conflict handling.
- Add resource upload/download UI.
- Add `document://` and `resource://` link interception.
- Decide when to embed built web assets into `notesd`.
