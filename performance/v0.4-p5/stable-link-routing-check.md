# v0.4 P5 — stable external link routing check

Date: 2026-08-05. Aggregate-only: no note content, no private paths, no
databases. The identifiers below come from a throwaway database created for
this check and destroyed afterwards.

P5 is a correctness slice, not a performance slice: resolution is one indexed
lookup in a database this machine already has open, so there is no scale
measurement to record. What follows is the live end-to-end behaviour check that
unit tests alone cannot show — a real `notriosd`, a real HTTP client, and the
real CLI binary.

## Service surface

`notriosd` on a temporary database; note created over REST.

| Request | Result |
|---|---|
| `GET /api/v1/status` | `database_info.database_id` present; replica ID absent |
| `POST /api/v1/links/resolve` with this database's link | `resolved`, with `document_uri`, `title`, `notebook_id` |
| same link with a foreign `database_id`, identical document ID | `foreign_database`; no `document_uri`, `title`, or `notebook_id` |
| malformed link (`/documents/doc/extra`) | `400` |

The foreign case is the one that matters: the document ID in that link exists
in the queried database, and the service still refuses to answer with it.
Document IDs are unique per database, not globally.

## CLI surface

| Command | Exit | Result |
|---|---:|---|
| `link <document-id>` | 0 | `notrios://databases/db_…/documents/doc_…` |
| `profile register --name live` | 0 | database ID read from the database, not from a flag |
| `open <uri>` (registry only, no `--db`) | 0 | `resolved` through profile `live` |
| `open <uri>` for an unregistered database | 1 | `unregistered_database`; no fallback to the only local database |
| `open <uri>` for a missing note | 1 | `stale_target` |
| `open notrios://…/resources/res_x` | 2 | `unsupported notrios link route` |
| `open --config … <uri>#heading-two` | 0 | `local_url` = the configured service, anchor preserved |
| `register-url-handler` (no `--apply`) | 0 | prints the entry; writes nothing |

The registry file was created `0600`.

## Ambiguity

Covered by `TestClonedDatabasesAreReportedAsAmbiguousRatherThanPicked`, which
copies a database at the filesystem level — the case the identity contract says
cannot announce itself as a clone — registers both copies, and requires
`ambiguous_database` with both candidates listed and exit 1. Naming one with
`--profile` then resolves it. A separate registry test proves `--profile`
cannot redirect a link into a database that profile does not hold.

## Not covered this session

Browser-level verification of the `#document=<id>` deep link was not run: this
machine has no Python `playwright` module installed (the repository documents
creating a venv for `scripts/verify_layout_resize.py`). The client-side pieces
are covered by vitest — stable-link parsing and rejection, deep-link hash
parsing, preview routing of `notrios://` anchors, and the wrong-database
message — plus `npm run typecheck` and the production build.
