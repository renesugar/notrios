# v0.4 P5 — Stable external links and local resolution

Status: complete on 2026-08-05.

Model: Claude Opus 5 (Claude Code).

## Why

Every link Notrios had was internal. `document://default/documents/doc_x` means
nothing outside the database that minted it, so a note could not be referenced
from an email, a ticket, another application, or a second Notrios library. The
identity needed to fix that already existed — schema v12 persists a logical
`database_id` that survives moving the file, renaming the profile, and
restoring onto another machine — but nothing used it in a link.

## What it does

`notrios://databases/{database_id}/documents/{document_id}[#anchor]`.

- `internal/stablelink` parses and formats it. The parser is hand-written and
  strict rather than delegated to `net/url`: this value arrives from outside
  the application — a clicked link, a pasted string, an OS protocol handler —
  and the only safe reading is one that matches the documented shape exactly or
  fails. Rejections are typed, so "this is someone else's URI" is distinct from
  "this is a broken Notrios URI".
- `internal/profiles` is the local registry mapping a logical database ID to a
  database on this machine, written explicitly by the user. It never scans the
  filesystem and never infers a database from a path.
- The store resolves a link against the database it has open:
  `resolved`, `trashed`, `stale_target`, or `foreign_database`.
- A `notrios://` link inside a note body is a first-class link record —
  `resolved` for this database, `external` for another, `unresolved` for a
  missing note, `invalid` when malformed.
- `POST /api/v1/links/resolve` exposes resolution; `GET /api/v1/status` reports
  `database_info.database_id`.
- `notriosctl link`, `open`, `profile register|list|forget`, and
  `register-url-handler` are the desktop surface.
- The web preview routes `notrios://` anchors through the service instead of
  following them, and the UI honours the `#document=<id>` deep link the desktop
  handler produces.

## Decisions worth recording

**Ambiguity is reported, never resolved.** Two profiles can hold clones of one
logical database — a restored backup, a copied file. The registry returns every
candidate and refuses; `--profile` picks one. Picking silently could edit the
wrong copy.

**`--profile` settles ambiguity but cannot redirect.** A preferred profile must
itself hold the database the link names. Honouring `--profile work` for a link
belonging to another database would open the wrong notes under an
explicit-looking instruction.

**A foreign link is never matched against local IDs**, even when a document
with exactly that ID exists here. Document IDs are unique per database, not
globally. The response also omits title/URI/notebook for a foreign link, so it
cannot be used to probe what this database contains.

**The database ID is read from the database.** `profile register` takes no
`--database-id`: a registry entry claiming an identity its database does not
have would send links to the wrong notes.

**Exit codes are the contract.** `open` runs under a protocol handler with no
terminal: 0 opened, 1 unresolvable, 2 malformed.

**The handler prints by default.** `register-url-handler` writes nothing
without `--apply`, because it changes what happens when the user clicks such a
link anywhere on the machine. The entry claims `x-scheme-handler/notrios` and
nothing else.

**Only documents.** `notrios://.../resources/...` is rejected by name as an
unsupported route rather than ignored, so a newer link type can be added
without an older build mishandling it.

## Two things measurement or use corrected

1. **The desktop entry needed the config path.** The first version generated
   `Exec=notriosctl open --launch %u`. A desktop launch has no working
   directory or environment of the user's choosing, so that resolves the local
   service URL from built-in defaults — port 8080 — and would open the note in
   whatever service happens to be there rather than theirs.
   `register-url-handler --config` now embeds an absolute config path.
2. **The memoized database ID has to be invalidated by adoption.** Link
   resolution caches the logical database ID, because rebuilding a note's links
   consults it once per link and an import does that for every note. Restore's
   `AdoptDatabaseIdentity` changes the universe every stable link in the
   database names, so it clears the cache.

## Validation

- `internal/stablelink`: shape, anchors, scheme/authority case folding without
  folding identifiers, foreign-scheme distinction, traversal, percent-escapes,
  embedded newlines, query strings, every length bound, hostile anchors;
- `internal/profiles`: round trip and `0600` permissions, upsert/remove,
  unambiguous resolution, ambiguity carrying every candidate, the
  no-redirect rule, invalid profiles, and refusal of a corrupt registry
  (bad JSON, future version, unknown field, duplicate names, relative path);
- `internal/store`: stable URI from identity, the four resolution statuses, no
  local state leaked for a foreign link, link-graph classification of
  `notrios://` targets in note bodies, and links still resolving after the
  database file is copied to a new path;
- `internal/httpapi`: endpoint statuses, bad input, the absent replica ID in
  `/status`, and that the endpoint takes no database selector;
- `cmd/notriosctl`: desktop-entry contents, and two tests that build the real
  binary and check the exit-code contract end to end, including a
  filesystem-level database clone producing `ambiguous_database`;
- `web`: vitest over stable-link parsing/rejection, deep-link hash parsing,
  preview routing, and the wrong-database message;
- `go vet ./...`, `go test ./...`, required-file, scaffold, docs-site build,
  `npm run typecheck`/`build`/`test`, `scripts/mvp_smoke.sh`,
  `scripts/run_performance_smoke.sh`;
- live service and CLI check recorded in
  `performance/v0.4-p5/stable-link-routing-check.md`.

Browser-level verification of the deep link was not run: this machine has no
Python `playwright` module installed.

## Next task

P6, the native archive compatibility bridge to `movenotes-v3`, requires
explicit user approval.
