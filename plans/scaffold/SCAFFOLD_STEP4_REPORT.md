# Scaffold Step 4 Report — First Runnable Persistence Slice

## Completed

Step 4 wires the service to a narrow SQLite-backed persistence path while preserving the expanded API placeholders from Step 3.

Implemented slice:

- cgo-backed local SQLite adapter in `internal/store`.
- Embedded `migrations/0001_initial.sql` for service bootstrap.
- Default managed collection bootstrap.
- Document create/read through `POST /api/v1/documents` and `GET /api/v1/documents/{document_id}`.
- Markdown body retrieval through `GET /api/v1/documents/{document_id}/body`.
- Managed-note search through `POST /api/v1/search` using SQLite FTS5.
- `notesd -db <path>` flag for persistent local databases.
- Store and HTTP integration tests.

## Intentional limitations

- Document update/delete still use placeholders; planned for a later MVP task.
- Resources/assets remain placeholders; planned after document update semantics are stable.
- Cursor pagination is not implemented yet; current search returns up to the requested limit.
- MCP remains a documented contract only; REST proves the shared persistence slice first.
- The local SQLite adapter uses cgo/libsqlite3 because external Go module downloads were unavailable in this scaffold environment. `ENVIRONMENT_SETUP.md` now calls out `libsqlite3-dev`.

## Working-state definition after this step

A developer can run the service, create a Markdown note, read it back, and find it via FTS5 search. This is the first end-to-end application behavior beyond route placeholders.

## Validation

Executed successfully:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh

# Manual smoke check also passed with go run ./cmd/notesd, curl document create, and curl search.
```
