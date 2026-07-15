# MVP Task 7 Report — MCP MVP

Status: completed.

## Summary

Task 7 added a read-only MCP MVP surface over the same store/service semantics used by the REST API. The implementation is intentionally dependency-free in this restricted scaffold environment: it mounts a small JSON-RPC HTTP adapter at `/mcp` instead of pulling the official MCP Go SDK. This keeps the repo working without external Go module downloads while preserving the intended tool names, inputs, output limits, and safety boundaries.

Codex should consider replacing this adapter with the official MCP Go SDK once the project is in a normal development environment and the long-term dependency policy is settled.

## Implemented endpoint

- `GET /mcp` returns endpoint/tool metadata when MCP is enabled.
- `POST /mcp` accepts a small JSON-RPC-style MVP MCP surface.

Supported JSON-RPC methods:

- `initialize`
- `tools/list`
- `tools/call`

## Implemented read-only tools

- `list_collections`
- `search_documents`
- `get_document`
- `get_documents`
- `list_document_links`
- `list_document_resources`
- `get_document_outline`

## Safety controls

- No raw SQL tool.
- No arbitrary filesystem access.
- No MCP write tools yet.
- Tool result limits are clamped by `mcp.max_results`.
- Document bodies are clamped by `mcp.max_document_bytes`.
- Retrieved document bodies include `untrusted_data` and `body_truncated` metadata.
- `get_documents` is capped at five documents per call in the MVP.
- MCP can be disabled by configuration.

## Validation

Executed successfully:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

## Deferred

- Replace the local JSON-RPC adapter with the official MCP Go SDK.
- Add streamable HTTP/SSE/session behavior if required by target MCP clients.
- Add MCP resource listing/templates when the SDK integration is selected.
- Add write tools only after scopes, revision preconditions, and confirmation policy are implemented.
- Add MCP client integration tests against real clients.
