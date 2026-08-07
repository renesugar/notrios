# API Specification

This document defines the REST and MCP contract for the companion service. `api/openapi.yaml` is the machine-readable REST skeleton that agents should keep aligned with this document.


## Implementation status

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
- `POST /api/v1/graph` returns a bounded document/resource neighbourhood for
  selected roots, to the requested depth.
- `POST /api/v1/graph/path` returns a shortest link path between two notes.
- `GET /api/v1/graph/report` returns the read-only orphan/isolate/hub report.
- `GET /api/v1/links/suggest` returns bounded link-target autocomplete.
- `POST /api/v1/links/check` classifies the links in an unsaved buffer.
- `POST /api/v1/note-queries/run` evaluates one embedded `note-query` block.
- `POST /api/v1/selection/plan` returns the read-only selection/privacy plan.
- `POST /api/v1/links/resolve` resolves an external `notrios://` link against
  this database.
- import/publish job APIs, rich block indexing, REST profiles, batches, and sync
  remain planned. Document/resource/search/notebook/tag/trash, link-listing,
  graph-slice, remote-media scan/policy/localization, selection planning, stable
  link resolution, and the MCP adapter are live over REST; import, archive v1,
  archive v2 export/verify/restore, stable-link routing, and publication are
  local CLI operations by design.

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
- Stable opaque IDs and URI fields; do not expose search-index row IDs (Recoll/Xapian docids) as public identity.
- Optimistic concurrency for mutations through `If-Match` or request-body `base_revision_id`.
- Cursor pagination for deep navigation. `k2` tokens are query/sort-bound
  keysets: chronological traversal uses `(updated_at, id)` and reproducible
  FTS relevance uses `(score, id)`. Optional FTS5/Recoll merging uses an
  immutable `m1` snapshot capped at 1,000 hits and reports `truncated` when
  that explicit window is full. Offset is not used for unbounded traversal.
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

`/api/v1/status` reports runtime readiness plus the config path, database
driver/path/state, the stable logical `database_id`, schema version, storage
roots, capability flags, configured search limits, and the active remote-media
policy. `database_id` is what a client needs to build an external
`notrios://` link for a note it already holds; the per-copy replica ID is
deliberately not reported because it means nothing in a shared link. Its `search_sidecar` object
reports whether Recoll is configured/available/active, current state, pending
and failed projection jobs, last sync/index/reconciliation timestamps, the
last bounded reconciliation counts (missing/stale/orphaned/repaired), and a
bounded last error. SQLite/FTS5 remains available when Recoll is unavailable or
degraded. `database` remains as a deprecated summary string for early UI
compatibility; clients should prefer `database_info`.

### Collections

```text
GET    /api/v1/collections
POST   /api/v1/collections
GET    /api/v1/collections/{collection_id}
PATCH  /api/v1/collections/{collection_id}
```

Collections are logical namespaces. Capabilities declare whether a collection is writable, searchable, publishable, has resources, has graph data, or is backed by a derived search sidecar (Recoll).

### Search

```text
POST /api/v1/search
GET  /api/v1/search?q=...&collection=...&limit=...
```

Search response hits include the legacy collection `source`, `id`, `uri`,
`title`, `snippet`, `score`, and `metadata`. `sources` attributes each
deduplicated hit to `sqlite`, `fts5`, and/or `recoll`; a hit found by both FTS5
and Recoll appears once with both values. Optional Recoll merges use one
bounded immutable snapshot so order and attribution remain stable on every
cursor page.

Default limits:

- REST default: 25.
- REST max: 100.
- MCP default: 10.
- MCP max: 50 unless explicitly configured.

Cursor tokens must be opaque and bound to query, filters, sort order,
collection set, and cursor/API version. They must carry enough stable boundary
state for keyset traversal or identify a bounded result snapshot. Opaque
base64-encoding alone does not make an offset cursor scalable.

`next_cursor` is omitted at exhaustion. A cursor replayed with another query,
collection, sort, or route returns `400 cursor_invalid`. Search responses may
include `truncated: true` only for the explicit 1,000-hit optional-sidecar
snapshot window. Notebook-note and Trash list responses use the same
`documents` + `next_cursor` page shape with route-bound chronological keysets.

The live v0.4 Q1 parser supports implicit AND, uppercase `OR`, prefix `-`,
parentheses, phrases, and the documented field filters. `category:` is an exact
alias for `notebook:`; either field with `"All notes"` removes that notebook
constraint. The shared expression tree is bounded to 4,096 UTF-8 bytes, 256
tokens, and 16 parenthesis levels. Its canonical structure is part of the
cursor fingerprint. `/status` reports `search.boolean`,
`search.category_alias`, and the three parser limits.

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
GET    /api/v1/documents/{document_id}/outline      # Markdown headings with lines/anchors
GET    /api/v1/documents/{document_id}/blocks      # addressable blocks with backlink counts
POST   /api/v1/documents/{document_id}/append        # {text}; If-Match optional (retries once)
POST   /api/v1/documents/{document_id}/prepend       # {text}
GET    /api/v1/documents/{document_id}/lines?start=&end=    # 1-indexed inclusive slice
GET    /api/v1/documents/{document_id}/search-in?pattern=   # case-insensitive, line numbers + context
```

`PATCH` edits follow joplin-mcp `editNote` semantics: a search string that matches multiple locations fails unless `replace_all` is set; dry runs preview the result.

`PUT`, `PATCH`, `DELETE`, and revision restore require optimistic concurrency
through `base_revision_id` or `If-Match`. `DELETE` means trash/soft-delete: the
current row is hidden from search and from every write path, FTS is refreshed,
and revisions remain available. `GET /api/v1/documents/{document_id}` is the
one exception: it returns a trashed note with `deleted_at` set and
`editable: false`, because the Trash is a list someone reads before deciding
what to restore and a stable link resolving to `trashed` has to open something.
Every other read (MCP `read_note`, search, links, remote-media scan) still stops
at the Trash, so `is:trashed` remains a deliberate opt-in rather than a scope a
caller falls into. The Trash route currently purges local-source notes; v0.7
changes purge into a death-certificate operation whose payload collection is
retention/acknowledgement gated.

### Resources

```text
POST   /api/v1/resources
GET    /api/v1/resources/reports/reference
HEAD   /api/v1/resources/{resource_id}
GET    /api/v1/resources/{resource_id}
GET    /api/v1/resources/{resource_id}/content
DELETE /api/v1/resources/{resource_id}
GET    /api/v1/admin/gc/report
GET    /api/v1/documents/{document_id}/resources
POST   /api/v1/documents/{document_id}/resources/{resource_id}
DELETE /api/v1/documents/{document_id}/resources/{resource_id}
```

`POST /api/v1/resources` currently accepts a raw request body. `filename` and
`collection_id` are supplied as query parameters or the filename can be
inferred from `Content-Disposition`. `GET /content` streams the stored bytes
and supports `?download=1`; HTTP `Range` requests are planned in v0.6 F3, which
needs them for bounded MCP resource reads — a future hardening
item. Immediate resource deletion refuses references and requires the
object-specific confirmation below; retention-aware bulk deletion is
dry-run-first and CLI-only.

`GET /api/v1/resources/reports/reference` is the read-only H5 report. It
groups multiple logical resources that share an exact SHA-256 blob, identifies
physical blobs with no document-resource references, and reports direct
per-notebook usage for current (non-trashed) notes. References from trashed
notes still prevent a blob from being classified as unreferenced. The
`perceptual` block reports whether an embedding application installed a hook,
stored hashes, matching review rules, and near-duplicate suggestions. Those
suggestions never merge, reject, or delete content.

`GET /api/v1/admin/lint/report` is the read-only v0.5 E2 workspace lint. It
returns per-check complete counts, capped examples, and a `report_sha256` over
every finding including those the cap hid, so an unchanged library produces an
unchanged digest. Findings carry a document/resource ID, a line and column, a
stable reason code, and a SHA-256 fingerprint of the offending target — never a
note title, body excerpt, raw link target, or local path, because a broken
wikilink's raw text is frequently a private note's title. `checks=` selects a
subset and `detail_limit=` caps examples (maximum 1,000). There is deliberately
no apply endpoint. Fixing (v0.5 E3) is CLI-only for the same reason resource
garbage collection is: `notriosctl fix` is dry-run by default and applies one
note at a time against the revision its plan was computed from, writing an
ordinary revision per note. No REST surface applies a fix.

`GET /api/v1/admin/gc/report` is the read-only H6 garbage-collection plan.
It uses the configured retention windows and returns `eligible`, `retained`,
and (always empty over REST) `removed` entries with timestamps, reasons, and
the active retention gate. There is intentionally no REST apply endpoint.
`notriosctl gc` is dry-run by default; only `notriosctl gc --apply` deletes
retention-expired resources, rechecking reference state inside the deletion
transaction.

The existing immediate `DELETE /api/v1/resources/{resource_id}` remains a
single-resource administrative escape hatch. It refuses any referenced
resource and now requires
`X-Notrios-Confirmation: delete-resource:{resource_id}`. Permanent trash purge
similarly requires
`X-Notrios-Confirmation: purge-document:{document_id}`. Missing or incorrect
confirmation returns `428 confirmation_required`.

### Links and graph

```text
GET  /api/v1/documents/{document_id}/links?direction=outgoing|incoming|both
POST /api/v1/graph
POST /api/v1/graph/path
GET  /api/v1/graph/report
GET  /api/v1/links/suggest?q=...&limit=...&exclude_document_id=...
POST /api/v1/links/check
POST /api/v1/links/resolve
```

Link records preserve source syntax, raw target, normalized target URI, source position, context, anchor, relation type, and resolution status. MVP Task 5 extracts common Markdown links/images, Obsidian wikilinks/embeds, app URIs, external URLs, heading anchors, and block anchors with a conservative parser.

`GET /api/v1/documents/{document_id}/blocks` (v0.5 E1) lists a note's
addressable blocks in document order: ID, kind, heading level, authored marker,
heading slug, content hash, byte range, and incoming anchor count. Block IDs are derived
from block text, so an ID names exactly the content it was written against. The
response carries no block text — a caller that wants content reads the body,
which is already an authorized read. Blocks per note are bounded by the parser,
so this is not a paged surface.

`POST /api/v1/graph` (v0.5 E4) expands a bounded neighbourhood. `depth` is now
honoured: until E4 it was accepted, carried through two request structs, and
never read, so a caller asking for three hops received one with nothing saying
so. The ceilings are depth 5, 100 roots, 5,000 nodes, and 20,000 edges, and a
request naming a wider bound is refused with `400 validation_failed` naming the
ceiling rather than clamped — a caller who asked for 50,000 nodes and received
5,000 cannot otherwise tell a capped graph from a small one. Defaults are 250
nodes and 500 edges, which is what this document has published since the MVP.
When a ceiling stops an expansion the response sets `truncated`, names it in
`truncated_by` (`nodes` or `edges`), and reports `completed_depth` — the deepest
level expanded in full — beside `requested_depth`. Every node carries its hop
distance from the nearest root as `depth`. Nodes, edges, and depths are the
whole contract: no coordinates, no clustering, no layout.

`POST /api/v1/graph/path` (v0.5 E4) finds a shortest link path between two
notes, searching from both ends. `status` is `found`, `no_path`,
`depth_exhausted`, or `budget_exhausted`. Only `no_path` is a statement about the
library — everything reachable was searched — while the last two say the search
stopped, at `max_depth` hops or after visiting `max_visits` notes. Reporting
either of them as `no_path` would tell a user two notes are unrelated when the
traversal merely gave up. Trashed notes are neither endpoints nor waypoints, so
a missing or trashed endpoint returns `404 not_found`.

`GET /api/v1/graph/report` (v0.5 E4) is the read-only orphan, isolate, and hub
report: one ordered scan of the collection with flat memory. `isolated_count` is
notes with no resolved document link either way; `orphan_count` is notes nothing
links to, which includes the isolated ones. Hubs rank by in-degree rather than
total degree, because out-degree describes how one note was written and
in-degree describes how the library refers to it. `limit` caps the three example
lists only — counts always describe the whole collection — and sets `truncated`
when it hides an example. `elapsed_ms` is reported because this report reads the
whole library and an operator should be able to see what that cost. There is no
apply surface here, exactly as with lint and the GC report.

`GET /api/v1/links/suggest` (v0.5 E5) is the bounded autocomplete an editor
calls while someone types a link target. It answers in two passes: title-prefix
matches first, in title order from a schema-v16 index range scan, then bounded
interior-word matches from FTS5's `title` column so typing `plan` also finds
"Kitchen Plan". A suggestion is a stable ID, a title, and the canonical URI —
never a body and never a snippet, because an autocomplete dropdown is not a
place note content should arrive. `q` must be at least two characters: a single
letter matches so much of a large library that ranking it means reading the
library. `limit` defaults to 10 and is capped at 50, `exclude_document_id` drops
the note being edited, and `truncated` says more notes matched than fit. The
interior-word pass reads at most 200 candidates, so for a very common word the
matches shown are the first the index yields rather than the best; the
title-prefix pass has no such limit, which is why it runs first.

`POST /api/v1/links/check` (v0.5 E5) classifies the links in an **unsaved
buffer**. The request carries `{body, document_id?, collection_id?}` and the
response carries one entry per link: raw target, byte range, line, column,
relation type, anchor, and a resolution status. Nothing is stored and no
revision is written.

It takes the body rather than a list of targets the client extracted, because
deciding what is a link belongs to the canonical extractor: a client
reimplementation would drift and start drawing markers that disagree with the
link records a save actually writes. Statuses are the ordinary ones —
`resolved`, `unresolved`, `ambiguous`, `external`, `invalid` — plus
`stale_anchor` when the note resolves and the section or block inside it does
not. `canonical_target` is filled in when a link resolved by title or filename,
so an editor can offer the same substitution `notriosctl fix` performs, before
the note is even saved.

Anchors into the note being edited are resolved against the **submitted body**
rather than the saved block rows. While someone is typing, the buffer is the
truth about its own headings, and checking a just-typed `#new-section` against
yesterday's saved blocks would mark a correct link broken. The body is bounded
at 1 MiB and the returned list at 2,000 links; `total` and `unresolved` describe
the whole buffer even when the list was capped.

`POST /api/v1/links/resolve` (v0.4 P5) answers which note an external
`notrios://` link names in the database this service has open. The request body
is `{"uri": "..."}` and nothing else: it accepts no path, profile, or database
selector, because choosing which local database answers a link is a desktop
routing decision made by the profile registry, not by an HTTP caller. The
response status is `resolved`, `trashed`, `stale_target`, `stale_anchor`, or
`foreign_database`; document fields are populated only when this database can
open the link, so a foreign link never reveals whether that ID exists locally.
A link carrying an anchor also resolves it: `#^marker`/`#^block-id` name a
block, a bare `#section-title` names a heading by slug, and heading text is
normalized to the same slug. Precedence is marker, then block ID, then heading
slug. Percent-escapes in an anchor are decoded only when the link carries a
Notrios URI scheme (`notrios://`, `document://`, `resource://`), because that is
where something declared itself a URI; a bare Markdown anchor stays literal and
identifiers never accept escapes at all. `stale_anchor` means the note is here but the block or heading is not —
the visible consequence of content-based block identity and slug-following
headings — and the note is still named so a client can offer to open it.
Malformed URIs return `400 validation_failed`.

A `notrios://` link inside a note body is a first-class link record: naming this
database and a live note it is `resolved` exactly like `document://`, naming
another database it is `external`, naming a missing note it is `unresolved`,
and malformed it is `invalid` rather than searched for as a note title.

### Embedded query blocks — implemented (v0.5 E7)

```text
POST /api/v1/note-queries/run
```

Evaluates one fenced ```` ```note-query ```` block. The request is
`{block, collection_id?}` — the block's text and nothing else: no SQL, no
filesystem path, no output target. The block is `key: value` lines using
`query` (required, the Q1 expression language), `fields`, `sort`, and `limit`;
an unknown key is an error that names the keys that work.

Parsing happens here rather than in the client so a block reaches the same Q1
parser as every other search surface and can express nothing its author could
not type into the search box. Bounds: 4 KiB of block text, 32 lines, 100 rows
(10 by default). Fields are opt-in from `title`, `notebook`, `tags`, `updated`,
and `snippet`, with `title` always present. `sort` is `updated` or `relevance`
— the two orders a keyset reproduces.

**A malformed block returns `200` with `error` set, not a `4xx`.** The note
containing it still has to render, so the block shows the message and the note
around it is unaffected. `truncated` reports that more notes match than the
block's limit showed.

A publication handoff carries the block's *text*, never a materialized result:
a rendered result would leak what the query matched at export time and would go
stale immediately.

### Remote media (implemented, v0.3 tasks H2–H4)

```text
POST /api/v1/documents/{document_id}/remote-media/scan       # per-URL policy decisions, no downloads
POST /api/v1/documents/{document_id}/remote-media/localize   # quarantine fetch → hash check → admission → rewrite
GET  /api/v1/media-policy                                    # active policy report
POST /api/v1/media-policy/check-url                          # evaluate explicit URLs
```

Localize requires `base_revision_id` (or `If-Match`) for non-dry runs and rewrites the note in a new revision; `dry_run` reports decisions without fetching a byte; `allow_review` opts review-listed URLs in; blocked URLs and exact-hash-blocked content are never admitted. Read-only notes (Help, Trash) return 403. The editor-profile MCP tool `localize_remote_media` exposes the same engine, as do `notriosctl localize` and the importers' `--localize-media` flag (Joplin RAW, Obsidian).

The scan evaluates every remote image/media URL in the stored body (Markdown images/embeds, media-extension links, HTML `<img>` tags) against the `remote_media` policy and returns `{url, media_class, action, reason, line}` decisions plus counts; an optional request body with `urls` evaluates that explicit list instead (e.g. unsaved editor drafts). Scanning is purely static — no downloads and no DNS resolution; address checks cover literals, and resolved addresses are re-checked at fetch time by the quarantine pipeline (H3). The read-only MCP tool `scan_remote_media` exposes the same scan.

Remote media localization never reads browser preview caches. The implemented
server path downloads into quarantine, applies URL/domain policy, size/MIME
checks and exact-hash policy, deduplicates by content hash, stores approved
resources, and rewrites Markdown in a new revision. H5 wires the perceptual
admission/policy/report hook, but Notrios ships no algorithm; the hook remains
inert unless an embedding application explicitly installs one.

### Selection/privacy planning — implemented

```text
POST /api/v1/selection/plan
```

The read-only planner accepts a typed target (`full_archive`,
`subset_transfer`, or `publication_handoff`), recursive notebook IDs, tags, a
Q1 query, and bounded explicit document IDs. It is the review step of the v0.4
P7 publication workflow: the digest it returns is what `notriosctl publish run`
requires before writing anything. It returns content-free document
and resource manifests, link decisions, source-bundle policy results,
exclusions, metadata stripping decisions, warnings, complete counts, and a
deterministic SHA-256 manifest digest. Detail arrays are capped at 1,000 over
REST and selected documents at 100,000. The endpoint accepts neither SQL nor
filesystem/output paths and never returns note bodies, resource bytes, source
metadata JSON, or local storage paths. See
`SELECTION_AND_PRIVACY_PLANNER.md`.

### Import/export/jobs (staged contract — import/export run through `notriosctl` today; jobs return stubs)

```text
POST /api/v1/import-jobs
GET  /api/v1/import-jobs/{job_id}
POST /api/v1/export-jobs
GET  /api/v1/export-jobs/{job_id}
GET  /api/v1/jobs/{job_id}
```

Native archive v1 exists only through `notriosctl` and is not a lossless backup.
Native archive v2 defines a strict versioned snapshot/manifest/object contract,
schema-v12 logical database/replica identity, bounded typed records,
manifest-last completeness, and read-only verification. It later becomes the
v0.7 full-sync bootstrap. Its streaming export (P3/P3a/P3b) and verify/restore
(P4) are deliberately CLI-only: no REST or MCP surface accepts an archive path,
streams archive bytes, or restores a database. See `NATIVE_ARCHIVE_V2.md`.

### Profiles, batches, external links, and sync (planned)

```text
GET    /api/v1/profiles
GET    /api/v1/profiles/{profile_id}
POST   /api/v1/batches                         # bounded organizer transaction
GET    /api/v1/batches/{job_id}
GET    /api/v1/sync/status
POST   /api/v1/sync/jobs                       # start pull/push/full-resync
GET    /api/v1/sync/jobs/{job_id}
DELETE /api/v1/sync/jobs/{job_id}              # cancel
POST   /api/v1/sync/handshake
GET    /api/v1/sync/objects/{sha256}           # resumable/range data plane
PUT    /api/v1/sync/objects/{sha256}
POST   /api/v1/sync/manifests
POST   /api/v1/sync/acknowledgements
```

`POST /api/v1/batch` implements the bounded batch contract (v0.6 F1): `move`,
`add_tags`, `remove_tags`, `trash`, `restore`, and `duplicate` over an explicit
list of note IDs, in `best_effort` (default) or `atomic` mode. Every requested
item gets an outcome in both modes — `applied`, `skipped`, `failed`, or
`rolled_back` — so a caller always knows which half happened. A request that ran
is `200` even when every item failed; `4xx` is reserved for a malformed call.
`trash` requires `base_revision_id` per item. `request_key` gives exactly-once
semantics through a persisted ledger (schema v17 `batch_operations`), replaying
the first run's outcomes verbatim; a key reused with different arguments is
refused. Bounded at `store.MaxBatchItems` (500), refused rather than truncated.

Historical note — the original contract sketch:

Batch operations cover bounded sets of IDs for move, duplicate, trash,
tag/untag, and stable Markdown-link formatting. A request specifies
all-or-nothing versus best-effort, an idempotency key, dry-run where meaningful,
and revision preconditions for destructive edits; the response has per-item
outcomes. Copying link text is not treated as exporting note contents.

External desktop links use
`notrios://databases/{database_id}/documents/{document_id}` and are implemented
in v0.4 P5 (`POST /api/v1/links/resolve` plus the `notriosctl link`, `open`,
`profile`, and `register-url-handler` commands). The handler maps the portable
database ID to a local profile through the explicit registry, reports every
candidate on ambiguity, and never guesses across database IDs. The `profiles`
REST routes below remain planned; the registry is local desktop configuration
today, not an HTTP surface.

The sync surface is conceptual until v0.7. MCP may start/cancel/status a job and
list bounded conflicts, but bulk envelopes/blobs use REST or immutable folder
objects. Full algorithms and compatibility rules are in `SYNCHRONIZATION.md`.

### Notebooks, tags, and search notebooks (implemented — plan tasks R3/R5)

```text
GET    /api/v1/notebooks                      # nested tree; emoji icons; "All notes" first, "Trash" last
POST   /api/v1/notebooks
GET    /api/v1/notebooks/{notebook_id}
PATCH  /api/v1/notebooks/{notebook_id}        # rename (case-insensitive uniqueness), move, emoji
DELETE /api/v1/notebooks/{notebook_id}        # refuses builtin search notebooks
GET    /api/v1/notebooks/{notebook_id}/notes  # cursor-paged
GET    /api/v1/notebooks/{notebook_id}/deletion-preview  # what deletion would do (v0.5 E8)
GET    /api/v1/tags                           # with note counts
POST   /api/v1/tags/rename                    # hierarchical rename; dry run by default (v0.5 E8)
POST   /api/v1/batch                          # bounded organizer transaction over an explicit note list (v0.6 F1)
POST   /api/v1/documents/{document_id}/tags/{tag}
DELETE /api/v1/documents/{document_id}/tags/{tag}
POST   /api/v1/documents/{document_id}/notebook   # move note to notebook
GET    /api/v1/trash                              # list soft-deleted notes
POST   /api/v1/trash/{document_id}/restore        # undelete
DELETE /api/v1/trash/{document_id}                # permanent delete, local-source notes only
GET    /api/v1/notebooks/tree                     # nested tree in sidebar order
GET    /api/v1/search-notebooks
POST   /api/v1/search-notebooks
DELETE /api/v1/search-notebooks/{id}              # refuses builtin rows
```

Name conflicts return `409 name_conflict`; builtin protection (Help/default notebooks, "All notes"/"Trash" search notebooks, Help note moves, purging externally-sourced notes) returns `403 forbidden`. Permanent local-note purge also requires the object-specific confirmation header documented above. `POST /api/v1/documents` accepts `notebook_id` (defaults to the "Notes" notebook), and document responses include `notebook_id`.

`POST /api/v1/tags/rename` renames a tag and, with `include_children`, every
tag under `<from>/`. Hierarchy is matched by path segment, so `projects` is not
a child of `project`. **`dry_run` defaults to `true`**: a request that omits the
field reports and changes nothing, and only `"dry_run": false` writes. The
report is not a prediction — the service runs the rename inside a transaction
and rolls it back for a dry run, so a dry run and an apply cannot disagree.
Renaming onto an existing name is a `merge`, reported per tag with `notes` (how
many notes carried the source) and `notes_gained` (how many actually change).
Saved searches mentioning the old name are reported in `warnings` and never
rewritten, because a saved search is text the user wrote. Tags are relational,
so a rename never edits a note body. The ceiling is
`store.MaxTagRenameTags` (500) tags per rename, refused rather than truncated.

`GET /api/v1/notebooks/{notebook_id}/deletion-preview` reports what deleting a
notebook would do: the notebooks removed, the notes that would move to the
Trash, the already-trashed notes affected, and `rehome_notebook_id` — the
notebook the whole subtree is re-assigned to so a later restore has a
destination. A protected notebook answers `200` with `deletable: false` and a
`reason` rather than an error.

Search notebooks are notebook rows with a `query` (see `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`); deleting one never deletes notes. `notebook:` and other query operators are defined in `SEARCH_QUERY_LANGUAGE.md`; search endpoints accept the user query language and must support cursor-based incremental results so clients can lazily populate large views like "All notes".

### Full-client goal (gap check after task R8)

The REST + MCP surface must be sufficient to build a full-featured third-party note-taking client (native C++/Qt, Go/Wails, Rust/Tauri). Parity check against the joplin-mcp tool list and the `UI_DESIGN.md` GUI features:

| Client capability | Covered by |
|---|---|
| sidebar notebooks tree with emoji, "All notes" first / "Trash" last | `/notebooks/tree`, `/search-notebooks` |
| tag list with counts | `/tags` |
| incremental search & search notebooks | `/search` with query language + cursors |
| open/edit/save with conflict detection | documents CRUD + revisions + ETags |
| read note slices, in-note find, table of contents | `/lines`, `/search-in`, `/outline` |
| append/prepend/string-replace edits | `/append`, `/prepend`, PATCH edits |
| move note, tag/untag, trash/restore/purge | `/documents/{id}/notebook`, tags routes, trash routes |
| hierarchical tag rename with a dry run | `/tags/rename` |
| confirm a notebook deletion with real counts | `/notebooks/{id}/deletion-preview` |
| resources/attachments | resources routes |
| links/backlinks/graph | links + graph routes |

Remaining known gaps: import/export job APIs (v0.6 F6), REST routes for the
local database registry and publication profiles, and sync (v0.7). HTTP `Range`
on resource content moved from this list into v0.6 F3. Bulk organizer
operations left it when v0.6 F1 shipped `POST /api/v1/batch`.

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
- `list_notebooks`
- `get_notebook_tree`
- `list_tags`
- `list_search_notebooks`
- `get_note_line_range`
- `search_in_note`
- `get_notebook_notes`

Write tools implemented in task R8, exposed only when the MCP profile is `editor` (default profile is read-only): `create_note`, `update_note` (requires `base_revision_id`), `append_to_note`, `prepend_to_note`, `edit_note` (fails on ambiguous matches without `replace_all`; supports `dry_run`), `delete_note` (requires `base_revision_id`; moves to Trash), `move_note_to_notebook`.

## MCP tools planned later

Tools implemented after R8 include `localize_remote_media` (editor profile;
revision precondition). Tools still planned later:

- `upload_resource`
- `get_document_graph`
- `lint_workspace`
- `fix_workspace_issues`
- `create_import_job`
- `run_batch` / `get_batch_status` (organizer profile)
- `plan_sync`, `start_sync`, `get_sync_status`, `list_sync_conflicts` (scoped
  administration; bounded output only)

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
- No direct search-index (Recoll/Xapian) mutation through the public API.
- Full document retrieval requires explicit IDs.
- Write tools require scope and revision preconditions.
- Remote media tools require media policy checks and SSRF protections.
- Imported documents are untrusted content; preview HTML must be sanitized.

## UI-specific navigation semantics

Preview-rendered links must be routed by the web UI and backed by REST:

- `document://{collection}/documents/{document_id}` opens the note in the UI.
- `resource://{collection}/resources/{resource_id}` opens/downloads the resource via `/content`.
- Remote `http(s)` image links are detected and can trigger localization, but preview loading alone must not modify the note.
- `notrios://databases/.../documents/...` links enter through the validated OS
  protocol handler and resolve to the internal `document://` route only after
  local profile/database identity checks. In the preview they are routed rather
  than followed: the client parses the shape, asks
  `POST /api/v1/links/resolve`, opens the note on `resolved`/`trashed`, and
  reports `foreign_database`/`stale_target` instead of opening a local note
  that happens to share the ID.

## API versioning

Breaking changes require a new API version or a compatibility shim. Additive fields are allowed. Clients must ignore unknown fields. The OpenAPI file should be updated in the same change as any REST contract modification.

## CLI import surface

The CLI binary is `notriosctl` (renamed from `notesctl` in plan task R2). Twitter/X, ChatGPT, and Claude importers are added in plan tasks R9–R11.

`notriosctl import joplin-raw [--config path] [--db path] [--asset-store path] [--collection id] [--batch-size 100] [--preserve-source] [--dry-run] [--write-config path] [--import-config path] <raw-export-dir>` inventories a Joplin RAW Export Directory, restores nested notebooks and real tags, and imports it into the canonical SQLite/resource store. Dry run uses the same action planner and reports creates/updates/skips without writing import state or source content; it writes a rename-on-conflict configuration only when `--write-config path` is explicitly supplied. Real imports checkpoint each bounded batch and resume only when the source inventory fingerprint still matches. `--preserve-source` additionally captures exact RAW item bytes and property order in the source-bundle store. The JSON report includes source/checkpoint identity, resume/batch progress, per-type inventory and malformed/unsupported counts, unresolved links, missing resource content, and note/resource/notebook/tag create-update-skip totals.

Joplin parsing uses CR/LF physical endings only, derives item titles from the
first RAW source line, removes that title line from canonical note Markdown,
preserves duplicate/future property order for source bundles, accepts UTF-8
BOMs, and rejects invalid UTF-8. J2's one-pass Markdown-aware link planner
skips escaped/code targets, reports unresolved targets, and returns direct
deduplicated resource relationships. J3 uses an indexed temporary manifest and
bounded canonical transactions whose document/revision/FTS5/link/provenance/
tag/resource/outbox/item-state changes commit with the matching checkpoint. A
final bounded link pass resolves later-batch targets. Complete private-safe
interruption/resume, search, and revision-stable no-op evidence satisfies the
v0.4 J3 gate.

`notriosctl import obsidian [--config path] [--db path] [--asset-store path] [--collection id] [--batch-size 100] [--preserve-source] [--dry-run] [--write-config path] [--import-config path] <vault-dir>` inventories an Obsidian vault, restores its folder hierarchy as nested notebooks, resolves aliases/relative paths/embeds/heading and block references to stable canonical URIs, imports or refreshes local assets, and rebuilds links after all targets exist. Dry run writes a path-scoped rename configuration (default `<vault>/.notrios/import-config.json`) and performs no canonical, checkpoint, or source-bundle writes. Real imports fingerprint and checkpoint bounded batches; `--preserve-source` stores exact Markdown/frontmatter and non-Markdown file bytes with relative paths.

`notriosctl import twitter [--config path] [--db path] [--asset-store path] [--collection id] [--notebook Twitter] [--dry-run] <extracted-archive-dir>` imports an extracted Twitter/X archive: tweets become Markdown notes in a "Twitter" notebook with expanded URLs and embedded media resources; hashtags become tags; provenance rows record author/@handle/thread/reply-to/post URL so conversation threads are recoverable; trashed tweets are never resurrected by re-imports.

`notriosctl import chatgpt [--options] <conversations.json|export-dir>` and `notriosctl import claude [--options] <conversations.json|export-dir>` import conversation exports: each conversation becomes one Markdown note (messages as role/timestamp sections; ChatGPT follows the current-node main path, skipping system/tool messages and abandoned branches) in a "ChatGPT"/"Claude" notebook, with provenance rows using the conversation ID as the thread ID. The same trashed-note and idempotency rules apply. It preserves Markdown/frontmatter, records source paths in frontmatter, imports local assets as content-addressed resources, attaches referenced assets, and refreshes graph links after the batch.

The commands are intentionally separate from REST/MCP today; later import jobs
expose a bounded HTTP/control-plane API while archive/source bytes stream
outside MCP context.

## CLI archive surface

Archive v1 (`export archive`, `import archive`) is query-scoped plain-note
interchange. Archive v2 is the lossless snapshot format and has three commands,
all local filesystem operations with no REST/MCP equivalent:

```text
notriosctl export archive-v2 [--target full_archive|subset_transfer]
    [--notebooks ids] [--tags names] [--query "…"] [--documents ids]
    [--match any|all] [--max-documents N] [--records-per-object N]
    [--pack] [--pack-bytes N] [--overwrite] [--no-verify] <out-dir>
notriosctl verify archive-v2 <archive-dir>
notriosctl restore archive-v2 --intent replace|adopt|merge|fork
    [--new-database-id id] [--batch-size N] <archive-dir>
```

Stable-link routing is likewise CLI/desktop-local:

```text
notriosctl link [--db path] <document-id>
notriosctl open [--registry path] [--profile name] [--db path] [--launch] <notrios-uri>
notriosctl profile register --name <profile> [--db path] [--registry path]
notriosctl profile list|forget [--registry path]
notriosctl register-url-handler [--apply] [--binary path] [--dir path]
```

Publication is likewise CLI-only:

```text
notriosctl publish profile save|list|delete [--profiles path] …
notriosctl publish plan --profile <profile> [--detail-limit N]
notriosctl publish run --profile <profile> --reviewed-plan <sha256> <out-dir>
```

`publish plan` is a read-only review; `publish run` re-plans and refuses unless
the result still matches the reviewed digest. No REST or MCP surface produces a
handoff or accepts an output path — `POST /api/v1/selection/plan` with target
`publication_handoff` is the network-visible review and nothing more.

`open` exits 0 when the link named a note this machine can open, 1 when it
could not be resolved (unregistered database, several candidate profiles, or a
missing note), and 2 when the link was malformed — an OS protocol handler runs
it without a terminal, so the codes are part of the contract.
`register-url-handler` prints the Ubuntu/XDG desktop entry and installs it only
with `--apply`.

`export` streams one SQLite read-transaction snapshot through the P1 planner,
publishes `manifest.json` last, and verifies the result before reporting
success. `--pack` selects the optional `objects.pack.v1` layout, which collapses
file count at the cost of ~11% more disk and a restart-rather-than-resume
interruption. `verify` is read-only. `restore` requires an intent — there is no
default and no path-based inference — completes verification before its first
canonical write, and leaves a durable `restore_state` marker if it is
interrupted, which only `--intent replace` can recover.


## Release-hardening note

Resource content responses set `X-Content-Type-Options: nosniff` and generate
`Content-Disposition` with sanitized filenames. Remote-media download policy is
implemented through v0.3 H4; generic upload limits, HTTP range support, and
perceptual hooks remain separate hardening items.
