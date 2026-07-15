# Plan 003 — API and Schema Detail

## Goal

Expand the scaffold REST/MCP API and SQLite schema detail so Codex can implement the first persistence slice against a stable contract.

## Status

Completed in Step 3 ZIP.

## Implementation summary

- Rewrote `API_SPEC.md` with detailed REST/MCP principles, endpoints, security rules, and UI navigation semantics.
- Expanded `api/openapi.yaml` with document CRUD, resources, revisions, links, outline, graph, remote media, Quartz publish planning, and job schemas.
- Added `DATABASE_SCHEMA.md` explaining the authoritative SQLite schema and derived sist2 boundary.
- Expanded `migrations/0001_initial.sql` with collections, documents, revisions, FTS mapping, blobs/resources, links, blocks, imports, media policy, outbox, publish profiles, and jobs.
- Expanded `internal/api/types.go` to mirror API DTOs.
- Added compile-tested placeholder handlers for the expanded route surface.
- Added route and validation tests.

## Validation evidence

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
```

## Model used

- GPT-5.5 Thinking

## Follow-up tasks

- Step 4: implement the first SQLite-backed persistence slice.
- Keep OpenAPI, DTO structs, and API docs synchronized as endpoints become real.
