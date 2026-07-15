# Plan Slice 009: MVP Resource Store

## Goal

Implement PLAN.md task 4 while preserving a working repository state: users should be able to upload a resource, store bytes content-addressed by exact hash, attach it to a document, list it, download it, and safely detach/delete it.

## Completed work

- Store-level resource model and operations added.
- Raw streaming upload implemented through REST.
- Content-addressed SHA-256 asset layout implemented.
- `blobs`, `resources`, and `document_resource_refs` are used by live code.
- Resource metadata and content endpoints implemented.
- Document-resource attach/list/detach implemented.
- Safe delete conflict implemented for referenced resources.
- UI can upload and attach files to the opened document.
- Tests and documentation updated.

## Working state after this slice

- `go test ./...` passes.
- Web typecheck/build pass after `npm ci`.
- Service can create/search/update notes and upload/list/download resources.
- Referenced resources cannot be deleted until detached.

## Next recommended slice

PLAN.md task 5: Markdown link parser and graph/backlinks.
