# MVP Task 2 Report — Document CRUD and Revisions

## Goal

Implement the next MVP slice from `PLAN.md`: document update/delete routes, optimistic concurrency, durable revision history, and soft-delete/trash behavior while keeping the repository in a working state.

## Implemented

- Added store errors for `ErrConflict` and `ErrPreconditionRequired`.
- Expanded `store.Store` with:
  - `UpdateDocument`
  - `DeleteDocument`
  - `ListDocumentRevisions`
  - `GetDocumentRevision`
  - `RestoreDocumentRevision`
- Added revision metadata fields to the schema:
  - `body_mime_type`
  - `message`
- Bumped schema reporting to `PRAGMA user_version = 2`.
- Added a bootstrap compatibility shim so older schema-version-1 development databases can receive the new revision columns.
- Implemented `PUT /api/v1/documents/{document_id}` with `base_revision_id` or `If-Match` optimistic concurrency.
- Implemented `PATCH /api/v1/documents/{document_id}` with title changes, simple SEARCH/REPLACE edits, and dry-run mode.
- Implemented `DELETE /api/v1/documents/{document_id}` as a soft delete/trash operation.
- Implemented revision list/read/restore routes.
- Updated FTS5 transactionally on create, update, delete, and restore.
- Added ETag headers for document reads, body reads, updates, and restores.
- Updated the React UI so an opened note can be saved as a new revision.

## Working state

A developer can now use curl or the browser UI to create a note, update it with a revision precondition, inspect revision history, soft-delete it, and restore a prior revision. Deleted notes disappear from normal reads and search results but remain in revision history.

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

Manual smoke test also passed: started `notesd` from a temporary config, created a note, updated it with `base_revision_id`, listed revisions, soft-deleted it with `If-Match`, and confirmed it no longer appeared in search.

## Notes and limitations

- The cgo/libsqlite3 wrapper remains intentionally narrow. Codex may replace it with a pinned SQLite driver in a less restricted environment.
- `PATCH` currently supports simple exact SEARCH/REPLACE edits only. Fuzzy matching and AST-aware edits are deferred.
- Revision listing omits bodies by design; `GET /revisions/{revision_id}` returns the body.
- Trash listing and permanent deletion are deferred to workspace maintenance/resource lifecycle tasks.
