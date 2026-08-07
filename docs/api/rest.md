# REST API

Everything the GUI does goes through the REST API; a third-party client can implement the entire feature set with it. The machine-readable schema lives in `api/openapi.yaml` in the repository; this page is the operational guide.

## Getting started

Start the service (`make build && ./bin/notriosd -config config/config.example.yaml`, or `go run ./cmd/notriosd ...`). The base URL is `http://<server.listen_addr>` — `http://127.0.0.1:8080` by default. There is no authentication; see [network exposure](../service.md#network-exposure).

```sh
curl http://127.0.0.1:8080/healthz              # -> ok
curl http://127.0.0.1:8080/api/v1/status | jq   # paths, schema version, capabilities
```

All request/response bodies are JSON except resource content streams. Errors use a stable envelope:

```json
{"error": {"code": "revision_conflict", "message": "operation conflicts with the current resource state"}}
```

Common codes: `validation_failed`/`cursor_invalid` (400), `not_found` (404), `precondition_required` (428), `revision_conflict`/`conflict` (409), `name_conflict` (409), `forbidden` (403), `web_ui_not_built` (404 on `/` without built assets).

## Notes: create, read, edit

```sh
# Create (notebook_id optional; defaults to the "Notes" notebook)
curl -s -X POST http://127.0.0.1:8080/api/v1/documents \
  -H 'Content-Type: application/json' \
  -d '{"title":"Meeting notes","body":"# Agenda\n\n- apples\n"}' | jq
```

The response carries `id`, `uri`, `notebook_id`, and — critically — `current_revision_id`. Reads return the same document plus an `ETag` header equal to the current revision:

```sh
curl -s http://127.0.0.1:8080/api/v1/documents/$DOC | jq
curl -s http://127.0.0.1:8080/api/v1/documents/$DOC/body     # raw Markdown
```

### Optimistic concurrency

Every mutation (`PUT`, `PATCH`, `DELETE`, revision restore) requires the revision you based your change on, either as `base_revision_id` in the body or an `If-Match` header. A stale revision gets `409 revision_conflict`; omitting it gets `428 precondition_required`.

```sh
REV=$(curl -s http://127.0.0.1:8080/api/v1/documents/$DOC | jq -r .current_revision_id)

curl -s -X PUT http://127.0.0.1:8080/api/v1/documents/$DOC \
  -H 'Content-Type: application/json' \
  -d "{\"title\":\"Meeting notes v2\",\"body\":\"updated\",\"base_revision_id\":\"$REV\"}" | jq

# Or with If-Match:
curl -s -X DELETE http://127.0.0.1:8080/api/v1/documents/$DOC \
  -H "If-Match: \"$REV\""            # 204: moved to Trash
```

### Surgical edits and note operations

```sh
# String-replace edit; ambiguous matches fail unless replace_all; dry_run previews
curl -s -X PATCH http://127.0.0.1:8080/api/v1/documents/$DOC \
  -H 'Content-Type: application/json' \
  -d "{\"base_revision_id\":\"$REV\",\"edits\":[{\"search\":\"apples\",\"replace\":\"oranges\"}],\"dry_run\":true}" | jq

curl -s -X POST http://127.0.0.1:8080/api/v1/documents/$DOC/append \
  -H 'Content-Type: application/json' -d '{"text":"- follow-up item"}' | jq
# also: /prepend, GET /lines?start=1&end=20, GET /search-in?pattern=agenda, GET /outline
```

`append`/`prepend` work without a precondition (the server applies them to the current revision and retries once on a concurrent write); pass `If-Match` for strict behavior.

## Search and pagination

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"(category:\"Work\" OR tag:todo) -tag:private","limit":25}' | jq
```

The `query` string accepts the full [query language](../query-language.md):
implicit AND, uppercase OR, prefix negation, grouping, phrases, typed fields,
and the `category:`/`notebook:` alias. The empty query is "All notes". Queries
are limited to 4,096 UTF-8 bytes, 256 tokens, and 16 parenthesis levels. When
more results exist, the response includes `next_cursor` — send it back
unchanged to fetch the next page. Cursors are opaque and bound to the canonical
expression; reusing one with a different expression returns `400`.

Chronological results use `(updated_at, id)` keysets and reproducible FTS5
relevance uses `(score, id)` keysets. If optional Recoll adds results, paging
uses a stable ten-minute snapshot with an explicit 1,000-hit bound;
`"truncated": true` reports when that window is full. Clients must keep tokens
opaque and restart a search after `cursor_invalid` (including an expired merged
snapshot). GET `/api/v1/search?q=...&limit=...` is a live equivalent for the
simple query/collection shape.

## Notebooks, tags, search notebooks, trash

```sh
curl -s http://127.0.0.1:8080/api/v1/notebooks/tree | jq        # nested sidebar tree
curl -s -X POST http://127.0.0.1:8080/api/v1/notebooks \
  -H 'Content-Type: application/json' \
  -d '{"name":"Work","icon_emoji":"💼"}' | jq
curl -s -X POST http://127.0.0.1:8080/api/v1/documents/$DOC/notebook \
  -H 'Content-Type: application/json' -d "{\"notebook_id\":\"$NB\"}" | jq

curl -s http://127.0.0.1:8080/api/v1/tags | jq                  # with note counts
curl -s -X POST http://127.0.0.1:8080/api/v1/documents/$DOC/tags/todo | jq
curl -s -X DELETE http://127.0.0.1:8080/api/v1/documents/$DOC/tags/todo   # 204

curl -s http://127.0.0.1:8080/api/v1/search-notebooks | jq      # "All notes" first, "Trash" last
curl -s -X POST http://127.0.0.1:8080/api/v1/search-notebooks \
  -H 'Content-Type: application/json' -d '{"name":"TODO","query":"tag:todo","icon_emoji":"✅"}' | jq

curl -s http://127.0.0.1:8080/api/v1/trash | jq
curl -s -X POST http://127.0.0.1:8080/api/v1/trash/$DOC/restore | jq
curl -s -X DELETE http://127.0.0.1:8080/api/v1/trash/$DOC \
  -H "X-Notrios-Confirmation: purge-document:$DOC"            # permanent; local notes only
```

A trashed note stays readable through `GET /api/v1/documents/{id}`, which
returns it with `deleted_at` set and `editable: false`. It has to be readable to
be recoverable — the Trash is a list you look at before deciding what to
restore. Every *other* read stops at the Trash: it is absent from search, from
link listings, and from MCP, so `is:trashed` remains something you ask for
rather than something you fall into.

### Previewing a notebook deletion

Deleting a notebook does not delete its notes: they move to the Trash and are
re-homed to the default notebook so a later restore has a destination. Ask what
would happen first:

```sh
curl -s http://127.0.0.1:8080/api/v1/notebooks/$NB/deletion-preview | jq
```

```json
{"notebook_id":"nb_...","name":"Work","notebooks":3,
 "descendant_names":["Reports","Drafts"],"truncated":false,
 "notes":12,"trashed_notes":1,"rehome_notebook_id":"nb_notes","deletable":true}
```

The route is read-only. A protected notebook answers `200` with
`deletable: false` and a `reason` instead of an error.

### Renaming a tag hierarchy

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/tags/rename \
  -H 'Content-Type: application/json' \
  -d '{"from":"project","to":"work","include_children":true}' | jq
```

**`dry_run` defaults to `true`.** The request above changes nothing; only
`"dry_run": false` writes. The report is not a prediction — the service runs the
rename inside a transaction and rolls it back — so a dry run and an apply cannot
disagree.

```json
{"from":"project","to":"work","include_children":true,"dry_run":true,
 "changes":[
   {"tag_id":"tag_...","from":"project","to":"work","action":"rename","notes":12,"notes_gained":12},
   {"tag_id":"tag_...","from":"project/alpha","to":"work/alpha","action":"merge",
    "merged_into_tag_id":"tag_...","notes":5,"notes_gained":2}],
 "notes":15,
 "warnings":["1 saved search(es) mention \"project\" and are not rewritten: Project work"]}
```

Hierarchy is matched segment by segment, so `projects` is not a child of
`project`. `action: "merge"` means the destination name already existed;
`notes_gained` is how many notes actually change, which is smaller than `notes`
when some already carried both tags. A rename never edits note bodies and never
rewrites a saved search. Bounded at 500 tags; over that it returns `400` rather
than renaming half a hierarchy. See
[renaming a tag hierarchy](../operations.md#renaming-a-tag-hierarchy).

`GET /api/v1/notebooks/{id}/notes` and `GET /api/v1/trash` accept `limit` and
`cursor` and return `{documents, next_cursor}`. Tokens are route-bound and
cannot be replayed between notebook and Trash listings.

Notebook and search-notebook names are case-insensitive; collisions return `409 name_conflict`. Protection rules return `403 forbidden`: deleting builtins ("All notes", "Trash", the Help notebook, the default "Notes" notebook), renaming/moving builtins, editing Help notes, moving notes into or out of Help, and purging externally-imported notes.

## Resources (attachments)

```sh
# Upload raw bytes; filename via query parameter (or Content-Disposition)
curl -s -X POST 'http://127.0.0.1:8080/api/v1/resources?filename=chart.png' \
  -H 'Content-Type: image/png' --data-binary @chart.png | jq    # -> id, uri, sha256

# Attach to a note, list, download
curl -s -X POST http://127.0.0.1:8080/api/v1/documents/$DOC/resources/$RES \
  -H 'Content-Type: application/json' -d '{"relation_type":"embedded"}' | jq
curl -s http://127.0.0.1:8080/api/v1/documents/$DOC/resources | jq
curl -sOJ 'http://127.0.0.1:8080/api/v1/resources/'$RES'/content?download=1'

# Read-only exact duplicate, unreferenced blob, notebook usage, and optional
# perceptual review report
curl -s http://127.0.0.1:8080/api/v1/resources/reports/reference | jq

# Retention-aware GC plan (always read-only over REST)
curl -s http://127.0.0.1:8080/api/v1/admin/gc/report | jq
```

Bytes are stored content-addressed (identical uploads share storage). Reference the resource in Markdown as `![chart](resource://default/resources/$RES)` — the UI renders and downloads through the same endpoints. Deleting a resource that notes still reference is refused. Perceptual report entries are suggestions only; no perceptual algorithm ships by default.

Immediate deletion of one unreferenced resource requires:

```sh
curl -s -X DELETE http://127.0.0.1:8080/api/v1/resources/$RES \
  -H "X-Notrios-Confirmation: delete-resource:$RES"
```

Permanent purge of a local note in Trash similarly requires
`X-Notrios-Confirmation: purge-document:$DOC`. Missing or incorrect
confirmation returns HTTP 428. Garbage-collection apply is intentionally CLI
only (`notriosctl gc --apply`).

## Links, graph, revisions

```sh
curl -s "http://127.0.0.1:8080/api/v1/documents/$DOC/links?direction=both" | jq
curl -s -X POST http://127.0.0.1:8080/api/v1/graph \
  -H 'Content-Type: application/json' \
  -d "{\"roots\":[\"$DOC\"],\"direction\":\"both\",\"depth\":2,\"max_nodes\":200,\"max_edges\":400}" | jq
curl -s http://127.0.0.1:8080/api/v1/documents/$DOC/revisions | jq
```

`depth` is how many link hops to follow (maximum 5; the default is 1). Every
node comes back with its hop distance from the nearest root. The ceilings are
100 roots, 5,000 nodes, and 20,000 edges; asking for more returns HTTP 400
naming the ceiling rather than quietly giving you less. If a ceiling stops the
expansion the response says so:

```json
{"truncated": true, "truncated_by": "nodes", "requested_depth": 4, "completed_depth": 2}
```

`completed_depth` is the deepest level that was expanded in full, so you can
tell a partial neighbourhood from a small one.

### Shortest path between two notes

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/graph/path \
  -H 'Content-Type: application/json' \
  -d "{\"from\":\"$DOC\",\"to\":\"$OTHER\",\"direction\":\"both\"}" | jq
```

`direction: "both"` ignores which way each link points, which is usually what
"how are these two notes related" means; `"outgoing"` follows links as written.
`status` is one of:

| Status | Meaning |
|---|---|
| `found` | `nodes` runs from `from` to `to` and `length` is the hop count |
| `no_path` | everything reachable was searched; these notes are not connected |
| `depth_exhausted` | a path may exist but it is longer than `max_depth` (default 6, maximum 10) |
| `budget_exhausted` | the search stopped after `max_visits` notes and proved nothing |

Only `no_path` says something about your library. The other two say the search
gave up, and raising `max_depth` or `max_visits` may change the answer.

### Link autocomplete while typing

```sh
curl -s "http://127.0.0.1:8080/api/v1/links/suggest?q=kit&limit=10" | jq
```

Title-prefix matches come first, in title order; then interior-word matches, so
`plan` also finds "Kitchen Plan". Each suggestion carries a stable ID, the
title, and the canonical `document://` URI to insert — never a body or a
snippet. The query must be at least two characters. `exclude_document_id` drops
the note being edited so it is not offered as its own target.

### Checking the links in an unsaved note

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/links/check \
  -H 'Content-Type: application/json' \
  -d "{\"document_id\":\"$DOC\",\"body\":\"[a](Kitchen)\n\n[b](document://default/documents/gone)\n\"}" | jq
```

This is the call an editor makes to mark broken links before anything is saved.
It stores nothing and writes no revision.

You send the body rather than a list of targets, because deciding what counts as
a link is the service's canonical parser's job — a second implementation in the
client would drift and start marking links a save would record differently.

Each entry is located (`line`, `column`, `start_byte`, `end_byte`) and carries a
status: `resolved`, `unresolved`, `ambiguous`, `external`, `invalid`, or
`stale_anchor` when the note exists and the section or block inside it does not.
A link that resolved by title also carries `canonical_target` — the URI it
already points at — which is the same repair `notriosctl fix` applies, offered
before you save.

Anchors into the note you are editing are checked against the body you sent, not
against the last saved version, so a heading you just typed resolves immediately.

### Orphan, isolate, and hub report

```sh
curl -s "http://127.0.0.1:8080/api/v1/graph/report?limit=20" | jq
```

Read-only, and one pass over the collection. `orphan_count` is notes nothing
links to; `isolated_count` is the subset that also links to nothing. `hubs`
ranks by in-degree — how many notes point at it — because that is the property
the rest of the library decides, not the note's own author. `limit` caps the
example lists only; the counts always describe the whole collection.

A note nobody links to is not an error, which is why this is a report and not a
lint check. Lint's `unreferenced_resource` answers the neighbouring question
about attachments.

## Selection and privacy dry runs

`POST /api/v1/selection/plan` is the shared read-only planner for future full
archives, subset transfers, and publication handoffs. It accepts typed
notebook/tag/query/explicit-ID selectors and policy overrides; no SQL or paths.

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/selection/plan \
  -H 'Content-Type: application/json' \
  -d '{"target":"publication_handoff","selection":{"tags":["publish"]}}' | jq
```

The response contains stable content-free IDs/hashes, counts, link/privacy and
metadata decisions, exclusions, warnings, and `manifest_sha256`. Detail arrays
are bounded independently; `truncated` never means the counts/digest are
partial. See [selection planning](../selection-planning.md).

## Workspace lint report

```sh
curl -s 'http://127.0.0.1:8080/api/v1/admin/lint/report?detail_limit=20' | jq
```

Read-only, like the garbage-collection report, and with no apply endpoint.
Returns per-check complete counts, capped examples, and `report_sha256` over
every finding. Findings locate problems (document ID, line, column, reason code,
target fingerprint) without quoting note content. `checks=` selects a subset.
See [maintenance](../operations.md#finding-what-has-rotted-workspace-lint).

## Listing a note's blocks

```sh
curl -s http://127.0.0.1:8080/api/v1/documents/doc_01H.../blocks | jq
```

Returns each addressable block in document order with its ID, kind, heading
level, author-written marker if any, content hash, byte range, and how many
links point at it. Block IDs derive from block text, so an ID names exactly the
content it was written against. The response contains no block text — read the
note body for that. See [stable links](../stable-links.md#linking-to-a-block-not-just-a-note).

## Running a note's query block

A fenced ```` ```note-query ```` block in a note is evaluated by the service, not
by the client:

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/note-queries/run \
  -H 'Content-Type: application/json' \
  -d '{"block":"query: tag:todo -tag:done\nfields: notebook, updated\nsort: updated\nlimit: 20"}' | jq
```

```json
{"spec":{"query":"tag:todo -tag:done","fields":["title","notebook","updated"],
         "sort":"updated","limit":20},
 "rows":[{"document_id":"doc_...","uri":"document://default/documents/doc_...",
          "title":"Renew passport","notebook":"Admin","updated_at":"2026-08-01T09:00:00Z"}],
 "truncated":false}
```

The block is parsed with the same query parser every other search surface uses,
so a block can express nothing its author could not type into the search box —
there is no SQL, no scripting, and no filesystem reach. `limit` caps at 100 and
`truncated` says when more matched.

A malformed block returns **`200` with an `error` string**, not a `4xx`. The
note it lives in still has to render; only the block should show a problem.
Reserve failures for real transport or storage faults.

## Resolving a stable link

```sh
curl -s -X POST http://127.0.0.1:8080/api/v1/links/resolve \
  -H 'Content-Type: application/json' \
  -d '{"uri":"notrios://databases/db_qz.../documents/doc_01H..."}' | jq
```

Answers which note the link names in the database this service has open. The
request takes a URI and nothing else — no path, no profile, no database
selector — because choosing which local database answers a link is a desktop
routing decision, not something an HTTP caller makes. `status` is `resolved`,
`trashed`, `stale_target`, or `foreign_database`; the document fields are
present only when this database can actually open the link. `GET
/api/v1/status` reports `database_info.database_id` so a client can build stable
links itself. See [stable links](../stable-links.md).

## Placeholder endpoints (not yet functional)

Staged contracts include `GET /api/v1/jobs/{id}` and collection
creation/patching (collections are effectively fixed to `default`). Profiles,
batch organizer operations, native archive v2 jobs, and sync endpoints are
planned but not live; archive v2 export/verify/restore are CLI commands by
design. Remote-media scan and localization are implemented; see
[the CLI guide](../cli.md#localize) and the note inspector in the GUI.
