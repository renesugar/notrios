# MVP Task 1 Report — Service Persistence Foundation

## Goal

Complete `PLAN.md` task 1 by making the service start from configuration, create its local storage roots, apply migrations, and report database/storage/schema state through `/api/v1/status`.

## Implementation summary

- Added `internal/config` with a dependency-free parser for the documented `config/config.example.yaml` subset.
- Added config defaults and `-config` support in `cmd/notesd`.
- Preserved `-addr` and `-db` as explicit runtime overrides.
- Added startup directory creation for:
  - data directory;
  - SQLite database parent directory;
  - asset store;
  - projection directory;
  - sist2 index directory.
- Added `StoreStatus` to the persistence contract.
- Added SQLite schema-version reporting through `PRAGMA user_version`.
- Updated both migration copies to set `PRAGMA user_version = 1`.
- Expanded `/api/v1/status` with:
  - config path;
  - database driver/path/state/schema version;
  - storage roots;
  - capability flags;
  - search limits.
- Updated OpenAPI, API documentation, README, environment setup, context map, plan status, and plan archive.
- Updated the web UI status type to tolerate the richer status response.

## Working state

The repository remains runnable after this task:

- `notesd` starts from `config/config.example.yaml`.
- missing data/resource/projection/index directories are created automatically.
- SQLite opens and migrates.
- `/api/v1/status` reports schema version `1` for a freshly bootstrapped database.
- existing document create/read/body/search behavior remains passing.
- the React UI still typechecks and builds.

## Validation evidence

Executed successfully:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

Manual smoke check performed:

```bash
go run ./cmd/notesd -config config/config.example.yaml
curl http://127.0.0.1:8080/api/v1/status
```

Expected status fields include `database_info.schema_version`, `storage.asset_store`, `storage.projection_dir`, and `capabilities.search.fts5`.

## Limitations and follow-up

- The config parser intentionally supports only the subset used by the current example config. Codex may replace it with a pinned YAML/TOML library once dependency policy is settled.
- Remote-media policy fields are still documented but not yet enforced by code.
- MCP, resources, Quartz, and sist2 remain planned integrations.
- The current cgo/libsqlite3 adapter remains temporary until the project chooses the long-term SQLite driver.

## Next recommended task

Proceed to `PLAN.md` task 2: **Document CRUD and revisions**.

Recommended narrow slice:

1. Implement `PUT` or `PATCH /api/v1/documents/{document_id}` for whole-body updates.
2. Require `base_revision_id` for optimistic concurrency.
3. Write a new `document_revisions` row for each update.
4. Update `documents.current_revision_id`, `documents.updated_at`, and `documents_fts` transactionally.
5. Add tests for successful update, stale revision conflict, and search reflecting the latest body.

## Superseded by MVP Task 2

MVP Task 2 bumped the current development schema to `PRAGMA user_version = 2` to add revision `body_mime_type` and `message` fields. This report is retained as the historical Task 1 completion record.
