# Plan 004 — Runnable Persistence Slice

## Status

Completed.

## Goal

Leave the scaffold in a working state where the service can persist a managed Markdown document in SQLite, read it back, and find it through FTS5-backed REST search.

## Implemented

- Added `internal/store` with a narrow SQLite store interface and cgo-backed implementation.
- Embedded and applied `migrations/0001_initial.sql`.
- Bootstrapped a default managed collection.
- Wired `notesd` to open `data/notes.sqlite` by default.
- Wired REST handlers for create/read/body/search when a store is available.
- Preserved placeholder behavior for routes outside the Step 4 slice.
- Added store unit tests and HTTP integration tests.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
```

## Next plan

Step 5 should connect the built-in React UI to these endpoints and provide a tiny create/search/read browser workflow.
