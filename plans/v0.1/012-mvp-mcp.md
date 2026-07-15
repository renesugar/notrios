# Plan slice 012 — MVP MCP

Status: completed.

## Goal

Expose the existing read-only document/search/link/resource semantics to MCP clients without giving LLMs raw SQL, arbitrary filesystem access, or write access.

## Implemented

- Mounted `GET /mcp` metadata endpoint.
- Mounted `POST /mcp` JSON-RPC MVP endpoint.
- Implemented `initialize`, `tools/list`, and `tools/call`.
- Added read-only tools:
  - `list_collections`
  - `search_documents`
  - `get_document`
  - `get_documents`
  - `list_document_links`
  - `list_document_resources`
  - `get_document_outline`
- Added body/result clamps from MCP configuration.
- Marked returned document content as untrusted data.
- Added tests for initialize, tools/list, read-only tools, and disabled MCP behavior.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

## Notes

The implementation deliberately avoids an external MCP SDK dependency because the scaffold was built in a restricted environment. Replace the local adapter with the official SDK in a normal development environment once dependency policy is finalized.
