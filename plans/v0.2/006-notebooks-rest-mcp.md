# v0.2 Task R5 — Notebooks/tags/trash REST + MCP read tools

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Changes

- REST (`internal/httpapi/notebooks.go` + routes): `GET/POST /api/v1/notebooks`, `GET /api/v1/notebooks/tree` (nested, sidebar order), `GET/PATCH/DELETE /api/v1/notebooks/{id}` (delete moves notes to Trash), `GET /api/v1/notebooks/{id}/notes`, `GET /api/v1/tags`, `GET /api/v1/documents/{id}/tags`, `POST/DELETE /api/v1/documents/{id}/tags/{tag}`, `POST /api/v1/documents/{id}/notebook`, `GET/POST /api/v1/search-notebooks`, `DELETE /api/v1/search-notebooks/{id}`, `GET /api/v1/trash`, `POST /api/v1/trash/{id}/restore`, `DELETE /api/v1/trash/{id}`.
- Error mapping: `ErrNameConflict` → 409 `name_conflict`; `ErrProtected` → 403 `forbidden`.
- `POST /api/v1/documents` accepts `notebook_id`; document DTO gains `notebook_id` and `deleted_at`.
- Store: added `ListNotebookDocuments` (direct membership, newest first).
- MCP read tools: `list_notebooks`, `get_notebook_tree`, `list_tags`, `list_search_notebooks`; write tools stay in R8 (scope-gated).
- Docs: `API_SPEC.md` section marked implemented, OpenAPI paths added, `api/mcp-tools.md` tool sections added.
- Tests: `internal/httpapi/notebooks_test.go` — notebook lifecycle over REST (nesting, 409 case-insensitive conflicts, 403 builtin protection, delete-to-trash), tags + move flows, search-notebook order/protection, trash restore/purge round trip, MCP tools/list + calls.

## Validation

`go test ./...`, `check_required_files`, `validate-scaffold`, `mvp_smoke.sh`, web typecheck+build — all passing.
