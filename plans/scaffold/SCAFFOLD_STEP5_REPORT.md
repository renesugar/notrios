# Scaffold Step 5 Report — Built-in UI Integration

## Goal

Connect the React/Vite scaffold to the live REST persistence slice so a developer can create, search, and open managed Markdown notes through the browser UI.

## Implemented

- Expanded `web/src/api.ts` with typed client functions for:
  - `GET /api/v1/status`
  - `POST /api/v1/documents`
  - `GET /api/v1/documents/{document_id}`
  - `GET /api/v1/documents/{document_id}/body`
  - `POST /api/v1/search`
- Replaced the placeholder UI with a two-column workflow:
  - create-note textarea placeholder;
  - FTS search panel;
  - clickable search results;
  - opened-note detail panel.
- Added safe rendering of SQLite FTS snippets by parsing only the generated `<mark>` tags instead of injecting arbitrary HTML.
- Updated Vite/TypeScript configuration and added React type dependencies so `npm run typecheck` and `npm run build` pass.
- Preserved `md-editor-rt` as a declared dependency but did not wire it yet. The Step 5 editor remains a textarea so the UI integration slice stays small and working.

## Validation

Executed successfully:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm run typecheck
cd web && npm run build
```

Manual smoke workflow:

1. Start the service:

   ```bash
   go run ./cmd/notesd -addr 127.0.0.1:8080 -db data/notes.sqlite
   ```

2. Start the web UI:

   ```bash
   cd web
   npm install
   npm run dev
   ```

3. Open the Vite URL, create a note, search for a term in it, and click the result to open it.

## Deferred

- `md-editor-rt` editor/preview integration.
- `document://` and `resource://` preview link interception.
- Resource upload/download UI.
- Autosave, update, optimistic concurrency, and conflict UI.
- Backlinks/resources sidebars.
- Embedded static UI served from `notesd`.
