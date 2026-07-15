# MVP Task 4 Report: Resource Store

## Status

Completed.

## Implemented

- Added a content-addressed resource store backed by the configured asset store directory.
- Added `store.OpenSQLiteWithAssetStore(database_path, asset_store)` and wired `notesd` to use the configured asset store.
- Implemented exact SHA-256 blob deduplication under `assets/sha256/<prefix>/<prefix>/<hash>`.
- Added live resource persistence operations:
  - create/upload resource;
  - read resource metadata;
  - stream resource content;
  - delete unreferenced logical resources;
  - list, attach, and detach document-resource references.
- Implemented REST endpoints:
  - `POST /api/v1/resources`
  - `HEAD /api/v1/resources/{resource_id}`
  - `GET /api/v1/resources/{resource_id}`
  - `GET /api/v1/resources/{resource_id}/content?download=1`
  - `DELETE /api/v1/resources/{resource_id}`
  - `GET /api/v1/documents/{document_id}/resources`
  - `POST /api/v1/documents/{document_id}/resources/{resource_id}`
  - `DELETE /api/v1/documents/{document_id}/resources/{resource_id}`
- Added safe delete behavior: referenced resources return conflict until detached.
- Updated `/api/v1/status` to advertise `resources: true` when a store is wired.
- Updated the built-in UI to upload/attach files for an opened note, insert `resource://` Markdown links, list attached resources, and provide download links.
- Advanced SQLite schema reporting to `PRAGMA user_version = 3`.

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

## Manual smoke test

A manual HTTP smoke test created a note, uploaded a text resource, attached it to the note, listed the note resources, downloaded the bytes, confirmed referenced-resource deletion fails with conflict, detached the resource, and deleted it successfully.

## Deferred

- HTTP range requests for resource content.
- Multipart upload handling; current implementation accepts raw request bodies with filename metadata in query string or `Content-Disposition`.
- Resource trash/restore and delayed blob garbage collection policy.
- Remote media localization and moderation policy enforcement.
- Perceptual hashes and malware/media hash databases.
- Automatic Markdown link parsing and graph/backlink updates.
