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
curl -s -X DELETE http://127.0.0.1:8080/api/v1/trash/$DOC       # permanent; local notes only
```

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
  -d "{\"roots\":[\"$DOC\"],\"direction\":\"both\",\"max_nodes\":20,\"max_edges\":40}" | jq
curl -s http://127.0.0.1:8080/api/v1/documents/$DOC/revisions | jq
```

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

## Placeholder endpoints (not yet functional)

Staged contracts include `GET /api/v1/jobs/{id}` and collection
creation/patching (collections are effectively fixed to `default`). Profiles,
batch organizer operations, native archive v2 jobs, and sync endpoints are
planned but not live. Remote-media scan and localization are implemented; see
[the CLI guide](../cli.md#localize) and the note inspector in the GUI.
