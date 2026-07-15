# API Specification

This document defines the REST and MCP contract for the companion service. `api/openapi.yaml` is the machine-readable REST skeleton that Codex should keep aligned with this document.


## Implementation status after MVP Task 5

The REST persistence slice is implemented for managed Markdown documents:

- `POST /api/v1/documents` creates a document in SQLite and writes the initial revision.
- `GET /api/v1/documents/{document_id}` reads the current non-deleted document and returns an `ETag` matching `current_revision_id`.
- `PUT /api/v1/documents/{document_id}` replaces title/body when `base_revision_id` or `If-Match` matches the current revision.
- `PATCH /api/v1/documents/{document_id}` supports title changes and simple SEARCH/REPLACE edits, with optional dry run.
- `DELETE /api/v1/documents/{document_id}` soft-deletes/trashes the document when `base_revision_id` query parameter or `If-Match` matches.
- `GET /api/v1/documents/{document_id}/body` returns the Markdown body and an `ETag`.
- `GET /api/v1/documents/{document_id}/revisions` lists revision summaries.
- `GET /api/v1/documents/{document_id}/revisions/{revision_id}` returns one full revision.
- `POST /api/v1/documents/{document_id}/revisions/{revision_id}/restore` creates a new current revision from a historical revision.
- `POST /api/v1/search` searches current, non-deleted managed documents with SQLite FTS5.
- `GET /api/v1/documents/{document_id}/links` returns outgoing links and backlinks parsed from Markdown.
- `POST /api/v1/graph` returns a small document/resource graph slice for selected roots.
- import, publish, rich block indexing, links/resolve, and remote-media routes outside this slice remain placeholders until later MVP tasks; document, resource, search, link-listing, graph-slice routes, and the read-only MCP MVP adapter are live in the current slice.

## Contract goals

The API must support many hundreds of thousands of notes/resources while remaining usable by:

- the built-in React UI;
- future native clients such as C++/Qt;
- command-line utilities;
- MCP clients and LLM tools;
- import/export/publish jobs.

## REST principles

- Versioned under `/api/v1` except `/healthz` and `/mcp`.
- JSON request/response bodies except resource content streams.
- Stable opaque IDs and URI fields; do not expose sist2 row IDs as public identity.
- Optimistic concurrency for mutations through `If-Match` or request-body `base_revision_id`.
- Cursor pagination for deep navigation; offset is allowed only for shallow UI pages.
- Errors use a stable envelope.
- Every write path must be implementable as a service-layer call so REST and MCP share semantics.

## Error envelope

```json
{
  "error": {
    "code": "revision_conflict",
    "message": "Document was changed by another client.",
    "details": {
      "current_revision_id": "rev_01K..."
    }
  }
}
```

Common codes: `invalid_json`, `validation_failed`, `not_found`, `precondition_required`, `revision_conflict`, `forbidden`, `limit_too_large`, `cursor_invalid`, `unsupported_media_type`, `media_policy_blocked`, `job_failed`, `internal_error`.

## Core REST endpoints

### Status

```text
GET /healthz
GET /api/v1/status
```

`/api/v1/status` reports runtime readiness plus the config path, database driver/path/state, schema version, storage roots, capability flags, and configured search limits. `database` remains as a deprecated summary string for early UI compatibility; clients should prefer `database_info`.

### Collections

```text
GET    /api/v1/collections
POST   /api/v1/collections
GET    /api/v1/collections/{collection_id}
PATCH  /api/v1/collections/{collection_id}
```

Collections are logical namespaces. Capabilities declare whether a collection is writable, searchable, publishable, has resources, has graph data, or is backed by sist2.

### Search

```text
POST /api/v1/search
GET  /api/v1/search?q=...&collection=...&limit=...
```

Search response hits must include `source`, `id`, `uri`, `title`, `snippet`, `score`, `metadata`, and optional `resource_links`.

Default limits:

- REST default: 25.
- REST max: 100.
- MCP default: 10.
- MCP max: 50 unless explicitly configured.

Cursor tokens must be opaque and bound to query, filters, sort order, collection set, and API version.

### Documents

```text
POST   /api/v1/documents
GET    /api/v1/documents/{document_id}
PUT    /api/v1/documents/{document_id}
PATCH  /api/v1/documents/{document_id}
DELETE /api/v1/documents/{document_id}
GET    /api/v1/documents/{document_id}/body
GET    /api/v1/documents/{document_id}/revisions
GET    /api/v1/documents/{document_id}/revisions/{revision_id}
POST   /api/v1/documents/{document_id}/revisions/{revision_id}/restore
GET    /api/v1/documents/{document_id}/outline
GET    /api/v1/documents/{document_id}/blocks
```

`PUT`, `PATCH`, `DELETE`, and revision restore require optimistic concurrency through `base_revision_id` or `If-Match`. `DELETE` means trash/soft-delete in MVP: the current row is hidden from normal reads/search, FTS is refreshed, and revisions remain available. Permanent deletion is a later maintenance operation.

### Resources

```text
POST   /api/v1/resources
HEAD   /api/v1/resources/{resource_id}
GET    /api/v1/resources/{resource_id}
GET    /api/v1/resources/{resource_id}/content
DELETE /api/v1/resources/{resource_id}
GET    /api/v1/documents/{document_id}/resources
POST   /api/v1/documents/{document_id}/resources/{resource_id}
DELETE /api/v1/documents/{document_id}/resources/{resource_id}
```

`POST /api/v1/resources` currently accepts a raw request body. `filename` and `collection_id` are supplied as query parameters or the filename can be inferred from `Content-Disposition`. `GET /content` streams the stored bytes and supports `?download=1`; HTTP range requests remain a future hardening item. Resource deletion refuses referenced resources unless an administrative force/GC policy is added later.

### Links and graph

```text
GET  /api/v1/documents/{document_id}/links?direction=outgoing|incoming|both
POST /api/v1/graph
POST /api/v1/links/resolve
```

Link records preserve source syntax, raw target, normalized target URI, source position, context, anchor, relation type, and resolution status. MVP Task 5 extracts common Markdown links/images, Obsidian wikilinks/embeds, app URIs, external URLs, heading anchors, and block anchors with a conservative parser. `POST /api/v1/links/resolve` remains a future endpoint.

### Remote media

```text
POST /api/v1/documents/{document_id}/remote-media/scan
POST /api/v1/documents/{document_id}/remote-media/localize
GET  /api/v1/media-policy
POST /api/v1/media-policy/check-url
```

Remote media localization must never be implemented by reading browser preview caches. The server downloads into quarantine, applies URL/domain policy, size/MIME checks, exact/perceptual hash checks, deduplicates by content hash, stores approved resources, and rewrites Markdown in a new revision.

### Import/export/publish

```text
POST /api/v1/import-jobs
GET  /api/v1/import-jobs/{job_id}
POST /api/v1/export-jobs
GET  /api/v1/export-jobs/{job_id}
POST /api/v1/publish/quartz/plan
POST /api/v1/publish/quartz/run
GET  /api/v1/jobs/{job_id}
```

Quartz publish planning must be privacy-aware: it selects a subset, rewrites links, copies only reachable public resources, applies media policy, strips private metadata, and reports warnings before building.

## MCP MVP endpoint

The current MVP mounts a dependency-free JSON-RPC MCP adapter at `/mcp`. It supports `initialize`, `tools/list`, and `tools/call`. This adapter should be treated as the working contract slice and may be replaced by the official MCP Go SDK later without changing tool semantics.

Implemented read-only tools:

- `list_collections`
- `search_documents`
- `get_document`
- `get_documents`
- `list_document_links`
- `list_document_resources`
- `get_document_outline`

## MCP tools planned later

- `create_document`
- `update_document`
- `upload_resource`
- `localize_remote_media`
- `get_document_graph`
- `lint_workspace`
- `fix_workspace_issues`
- `publish_quartz_plan`
- `create_import_job`

## MCP resources

Resource URI templates:

```text
document://{collection}/documents/{document_id}
document://{collection}/documents/{document_id}/body
document://{collection}/documents/{document_id}/manifest
resource://{collection}/resources/{resource_id}
resource://{collection}/resources/{resource_id}/content
resource://{collection}/resources/{resource_id}/derived/{variant}
```

For large collections, do not list every document through `resources/list`; return resource links from search, graph, and list tools.

## Security rules

- No raw SQL MCP tool in normal profiles.
- No arbitrary filesystem path tool.
- No direct sist2 database mutation through public API.
- Full document retrieval requires explicit IDs.
- Write tools require scope and revision preconditions.
- Remote media tools require media policy checks and SSRF protections.
- Imported documents are untrusted content; preview HTML must be sanitized.

## UI-specific navigation semantics

Preview-rendered links must be routed by the web UI and backed by REST:

- `document://{collection}/documents/{document_id}` opens the note in the UI.
- `resource://{collection}/resources/{resource_id}` opens/downloads the resource via `/content`.
- Remote `http(s)` image links are detected and can trigger localization, but preview loading alone must not modify the note.

## API versioning

Breaking changes require a new API version or a compatibility shim. Additive fields are allowed. Clients must ignore unknown fields. The OpenAPI file should be updated in the same change as any REST contract modification.

## CLI import surface

`notesctl import joplin-raw [--config path] [--db path] [--asset-store path] [--collection id] [--dry-run] <raw-export-dir>` imports a Joplin RAW Export Directory into the canonical SQLite/resource store and prints a JSON report.

`notesctl import obsidian [--config path] [--db path] [--asset-store path] [--collection id] [--dry-run] <vault-dir>` imports an Obsidian-style Markdown vault. It preserves Markdown/frontmatter, records source paths in frontmatter, imports local assets as content-addressed resources, attaches referenced assets, and refreshes graph links after the batch.

The commands are intentionally separate from REST/MCP in the MVP; later import jobs may expose an HTTP job API.


## Release-hardening note

Resource content responses now set `X-Content-Type-Options: nosniff` and generate `Content-Disposition` with sanitized filenames. This is an MVP local-service hardening measure; full upload-size and media-policy enforcement is deferred to v0.2.
