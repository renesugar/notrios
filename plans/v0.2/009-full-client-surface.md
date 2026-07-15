# v0.2 Task R8 — Full-client MCP/REST surface

Completed: 2026-07-15. Model: Claude Fable 5 (claude-fable-5).

## Changes

- REST: `POST /documents/{id}/append` and `/prepend` (If-Match optional; without it the server applies to the current revision and retries once on conflict), `GET /documents/{id}/lines?start=&end=` (1-indexed inclusive), `GET /documents/{id}/search-in?pattern=` (case-insensitive with line numbers + context); the `/outline` endpoint now returns real Markdown headings (previously a stub — MCP had the real extractor).
- PATCH edits: `replace_all` option added and joplin-mcp `editNote` semantics enforced — an ambiguous search (multiple matches) fails without `replace_all`.
- MCP read tools: `get_note_line_range`, `search_in_note`, `get_notebook_notes`.
- MCP write tools behind the `editor` profile (default remains read-only; tools are hidden from `tools/list` and rejected by `tools/call` otherwise): `create_note`, `update_note` (base_revision_id required), `append_to_note`, `prepend_to_note`, `edit_note` (ambiguity + dry_run), `delete_note` (base_revision_id required; trashes), `move_note_to_notebook`.
- `API_SPEC.md` gained a client-capability gap-check table (all `UI_DESIGN.md` GUI features are API-covered; deferred: HTTP ranges, links/resolve, blocks, import/export jobs); OpenAPI + `api/mcp-tools.md` updated.
- Fixed a latent smoke-test bug: `scripts/mvp_smoke.sh` killed the `go run` wrapper (leaking the real server, which then served stale data to later runs on the shared port). It now builds and runs the binary directly and refuses to start if the port is already serving.

## Validation

`go test ./...` (3 new httpapi tests: note ops, edit semantics, MCP write gating), `check_required_files`, `validate-scaffold`, hardened `mvp_smoke.sh` — all passing; no leaked server processes after runs.
