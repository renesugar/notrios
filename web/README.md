# Notes Companion Web UI

This is the built-in browser UI scaffold for the companion service.

## Development

Start the Go service first:

```bash
go run ./cmd/notesd -addr 127.0.0.1:8080 -db data/notes.sqlite
```

Then start Vite:

```bash
cd web
npm install
npm run dev
```

The Vite dev server proxies `/api` and `/healthz` to `127.0.0.1:8080`.

## Current scope

The UI can:

- read service status;
- create a Markdown note through `POST /api/v1/documents`;
- search persisted notes through `POST /api/v1/search`;
- open a selected note through `GET /api/v1/documents/{document_id}`;
- save a new revision for the opened note through `PUT /api/v1/documents/{document_id}` using `base_revision_id` optimistic concurrency.

The editor now uses `md-editor-rt`.

Preview behavior:

- `document://.../documents/{id}` links are intercepted and opened inside the UI.
- `resource://.../resources/{id}` links are intercepted and downloaded through the REST resource-content endpoint.
- `resource://` image sources are rewritten to REST content URLs so local resource images can render in preview.

Development note: the current sanitizer and preview link router are an MVP implementation. Before a production release, add explicit browser tests and review the sanitizer allowlist.
