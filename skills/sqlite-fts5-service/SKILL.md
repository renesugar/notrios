---
name: sqlite-fts5-service
description: Implement SQLite canonical storage and FTS5 search slices safely.
---

# SQLite FTS5 Service Skill

Use this skill when the active task matches the description.

## Steps

1. Start from migrations.
2. Keep canonical tables and FTS updates in one transaction.
3. Do not expose raw SQL over REST/MCP.
4. Add tests for migration, CRUD, search, and rollback.
5. Use stable document IDs, not rowids, in public API.

## Working-state checks

- `go test ./...`
- Add fixture-based integration tests before expanding feature scope.
