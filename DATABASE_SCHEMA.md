# Database Schema

This document expands the SQLite schema represented by
`migrations/0001_initial.sql` and the additive bootstrap upgrade shims in
`internal/store/sqlite.go`. The baseline migration creates through schema v17;
`ensureSchemaV18` adds `jobs`, and the dedicated
`migrations/0019_sync_journal.sql` adds the local replication journal while
`migrations/0020_sync_admission.sql` adds G5 peer compatibility and sequence
exhaustion protection. `store.CurrentSchemaVersion` is 25. SQLite is the
authoritative store for managed notes, metadata, revisions, resource
relationships, link graphs, media-policy decisions, import state, and jobs.
Recoll is the optional derived index for front-matter field search, extraction,
and broad filesystem search (see `RECOLL_INTEGRATION.md`).

## Schema principles

- Keep application identity stable with opaque IDs; do not expose rowids or search-index IDs as public identities.
- Keep managed note saves transactional: document row, current revision, FTS row, links, resource references, and outbox records are updated in one SQLite transaction.
- Store binary bytes outside SQLite in a content-addressed asset directory, while SQLite tracks blob hashes, resources, references, and provenance.
- Treat FTS5 and Recoll as derived indexes. FTS5 is updated synchronously for managed documents; the Recoll projection is updated asynchronously through `index_outbox`.
- Use soft deletion for documents first. Immediate resource deletion refuses
  references and requires object-specific confirmation; retention-aware
  garbage collection is dry-run first, transactionally rechecks references on
  apply, and exposes a replaceable future sync-acknowledgement gate.
- Keep import and projection paths deterministic so bulk imports can be resumed and repeated idempotently.
- Match every unbounded sort/filter cursor with a composite index and verify it
  through `EXPLAIN QUERY PLAN` plus generated scale profiles.

## Core entities

### collections

A collection is a logical namespace such as `personal-notes`, `joplin-raw-2026-07`, `obsidian-vault`, `twitter-archive`, or `research-pdfs`.

Important fields:

- `id`: stable public collection ID.
- `kind`: `managed`, `external`, `imported`, `sidecar_indexed`, or `projection`.
- `capabilities_json`: declares whether the collection supports write, resources, publishing, graph, remote media, and MCP reads.
- `settings_json`: collection-specific configuration.

### documents and document_revisions

`documents` stores current identity and state. `document_revisions` stores durable saved revisions. The current revision ID in `documents` points at the active revision.

Revision rows store body text, body MIME type, a message, and created time for MVP. Creates, updates, soft deletes, and revision restores all write a new revision row. Later versions may move large bodies to content-addressed blobs while keeping the same revision metadata shape.

### documents_fts

FTS5 indexes current managed documents. The Step 4 runnable slice uses a normal FTS5 table with `document_id` and `collection_id` as unindexed columns plus indexed `title` and `body` columns. This preserves snippets during early implementation. Later implementation may switch to external-content FTS if it simplifies synchronization.

### blobs, resources, and document_resource_refs

`blobs` are exact bytes addressed by SHA-256 or BLAKE3. `resources` are logical resources with filenames, MIME types, source metadata, and privacy policy state. `document_resource_refs` links documents to resources as `embedded`, `attachment`, `cover`, `derived`, or future relation types.

Schema v8 adds `resources.unreferenced_at` and
`resources.unreferenced_reason` plus `resources_unreferenced_idx`. A new upload
starts as `created_unattached`; attaching any document clears the retention
state; detaching the final reference records `detached`; purging the final note
reference records `purged_document`. The v7→v8 upgrade backfills old
unreferenced resources from `created_at` as `legacy_unreferenced` and clears
the fields on referenced resources. Garbage collection always rechecks the
reference table before deleting a logical resource.

### document_links and document_blocks

`document_links` records explicit links parsed from Markdown or imported source data. MVP Task 5 populates it transactionally on document create, update, restore, and soft-delete cleanup. It stores source syntax, raw target, display text, optional normalized target URI, resolved document/resource IDs, anchor type/value, context excerpt, source byte/line/column positions, relation type, and resolution status. Status values used in the MVP are `resolved`, `unresolved`, `ambiguous`, and `external`; later importers may add `invalid` and `target_deleted`.

`document_blocks` (schema v14) makes blocks separately addressable; heading and
block anchors on link records now resolve against it. See the v14 section below.

### media policy (schema v7, v0.3 task H1)

The media-policy tables support domain stop lists, exact-hash blocks, perceptual-hash blocks, remote media attempts, quarantine state, and resource hash records. As of schema v7:

- `media_domain_rules` — user-managed domain patterns with an `allow`/`block`/`review` action (unique, case-insensitive), complementing the `remote_media` config lists.
- `media_hash_rules` — hash-keyed rules (`algo` + `hash`), `kind` distinguishing `exact` from `perceptual`, action `block` or `review`. Exact hashes may block; perceptual hashes only ever raise review signals.
- `resource_hashes` — additional hashes per stored blob (`blob_sha256` + `algo`). H5 populates this slot when an optional perceptual hook is installed; no algorithm ships by default. Rows are removed when the physical blob is removed.
- `media_policy_decisions` — one row per remote-media attempt: original/final URL, decision, reason, hashes, plus v7 quarantine-state columns (`status`, `content_type`, `size_bytes`, `quarantine_path`, `updated_at`). Populated by the H3 quarantine pipeline (`store.RecordMediaAttempt` / `ListMediaAttempts`): every fetch attempt — quarantined or refused — is recorded.

`media_hash_rules` is consulted during localization admission (H4, via `store.AddMediaHashRule`/`FindMediaHashRule`): exact-hash `block`/`review` rules stop quarantined content before it reaches the asset store. H5 wires `resource_hashes` into every `CreateResource` admission when a hook is installed and checks matching perceptual `review` rules without blocking admission. `media_domain_rules` user-managed CRUD remains future work beyond the config lists.

### index_outbox

The outbox coordinates filesystem projections and Recoll indexing after the
canonical SQLite transaction commits. Schema v11 adds `next_attempt_at` and an
index over `(completed_at, next_attempt_at, sequence)`. Failed jobs retain a
bounded error, increment `attempt_count`, and receive durable exponential
backoff (5 seconds through a one-hour cap); later due jobs continue in bounded
batches, so one failure cannot spin or block the queue. Failure never
invalidates the canonical note save.

## MVP migration file

`migrations/0001_initial.sql` has grown with each milestone and now creates
through schema version **16** (v5 notebooks/tags/search notebooks, v6 source
provenance, v7 media policy, v8 resource retention, v9 scalable keyset indexes,
v10 resumable import state/source bundles, v11 projection retry scheduling, v12
logical database/replica identity, v13 restore state, v14 note blocks, v15
heading slugs, v16 title/filename indexes), applied idempotently on every startup with upgrade shims for
older databases.
Its original MVP portion represents schema version 4. Do not rename public
tables/columns casually once tests depend on them.

## Current and required indexes

Schema v9 adds and query-plan tests the primary chronological index:

```sql
(collection_id, deleted_at, updated_at DESC, id DESC)
```

It also adds `(notebook_id, deleted_at, updated_at DESC, id DESC)`, a
deleted-at/id partial Trash index, and `(tag_id, document_id)` for tag
membership. `internal/store/pagination_schema_test.go` verifies both migration
version and planner use. Chronological and relevance traversal use row-value
keysets; optional merged sidecar results use a bounded in-memory immutable
snapshot rather than a numeric offset inside an opaque token.

## Schema v10 — importer checkpoints and exact source bundles

- `import_checkpoints` stores one phase/index/report row per
  `(source_system, source_key, collection_id)`. The inventory fingerprint
  prevents a changed source tree from replaying a stale checkpoint.
- `import_item_states` stores each successfully applied input fingerprint,
  target ID, and action. Its source/type index supports bounded input-scoped
  batch lookups; importers do not scan the canonical document tables.
- `source_bundle_items` is the manifest for optional exact source capture:
  external ID, relative path, SHA-256, byte size, property-order JSON, and a
  safe relative asset path. Exact bytes live under
  `assets/source-bundles/sha256/`; this namespace is deliberately outside
  ordinary `blobs` and resource garbage collection.

H8 Joplin RAW and H9 Obsidian both use these generic tables. Obsidian item
keys are vault-relative file/folder paths; complete Markdown files preserve
their exact frontmatter and line endings, while non-Markdown bundle items
preserve original bytes. The importer checkpoint fingerprint also binds
source-preservation and folder-rename choices so incompatible resumptions
restart from the first phase.

## Schema v11 — projection retry scheduling

- `index_outbox.next_attempt_at` persists retry eligibility across restarts.
- `index_outbox_pending_idx` supports due-job traversal by completion/retry
  state and sequence.
- `/api/v1/status.search_sidecar` exposes pending/retrying counts while exact
  projection reconciliation remains a derived-filesystem operation.

## Schema v12 — archive and synchronization identity

`database_identity` has exactly one row. `database_id` is the stable logical
database/synchronization universe; `replica_id` is one writable SQLite copy.
Both are random opaque IDs and are never derived from the database path,
profile, hostname, or asset directory. Bootstrap and reopen preserve both.
Supported clone/restore workflows preserve or replace `database_id` only under
an explicit archive intent and always rotate `replica_id` before the restored
copy becomes writable. Profile routing identity remains separate and planned.

## Schema v13 — restore-in-progress marker

`restore_state` has at most one row. An archive-v2 restore commits many bounded
transactions, so an interruption leaves committed rows behind; this row is what
stops that partial library from being mistaken for a complete one. It records
the archive's `snapshot_id`, `commit_sha256`, and the chosen `intent`, is
written before the first canonical row, and is cleared after the last one
including identity adoption.

A library carrying the marker is neither empty nor complete. `adopt`, `merge`,
and `fork` refuse it; only `replace` recovers it, and the recovered library is
identical to a clean restore. Nothing else in the service writes this table.

## Schema v14 — note blocks

`document_blocks` holds one row per addressable region of a note: heading,
paragraph, list item, fenced code block, or table. Rows are derived state, like
FTS5 rows and links, and are rebuilt from the body inside the same transaction
as the save that produced them. They never outlive their document.

**Block identity is content-based** (`PROJECT_DECISIONS.md` 17). `id` derives
from a SHA-256 over the document ID, the block kind, the normalized text, and
the occurrence index among identical blocks in that note. Moving a block within
a note keeps its ID; editing its text mints a new one, so an anchor always names
exactly the text it was written against and a rewritten block breaks links into
it rather than silently redirecting them. Normalization is limited and
deliberate: line endings are normalized and trailing whitespace is trimmed, so
an editor that cleans whitespace on save does not break every anchor in the
note. Document scope keeps identity local — the same sentence in two notes is
two blocks, and a block key never lets one note's content be recognized in
another.

`marker` stores an author-written Obsidian-style `^marker` when the block ends
with one. Those are names the author chose and they survive edits to the block's
text, so anchor resolution tries the marker first and the derived ID second.

`heading_slug` (schema v15) is the URI-safe name a `#section-title` anchor
resolves against, derived from heading text: lowercase, spaces to hyphens,
anything that is not a letter, digit, hyphen, or underscore dropped, and
repeated headings disambiguated by occurrence (`notes`, `notes-1`). Only heading
blocks have one. It exists because block rows deliberately store a content hash
rather than heading text, which left a heading anchor with nothing to compare
against. Resolution normalizes whatever a link wrote, so `#Install & Setup` and
`#install-setup` reach the same heading; precedence is marker, then block ID,
then slug.

Indexes: `(document_id, ordinal)` for listing a note's blocks in order, a
partial `(document_id, marker)` index for authored-anchor lookups, and a partial
`(document_id, heading_slug)` index for heading anchors. A database
upgraded to v14 has no rows for notes nobody has edited since;
`RebuildDocumentBlocks` fills them in without writing a revision.

Sizing measured on the generated profiles: at six blocks per note the table
roughly doubles the database (114 MB to 245 MB at 100,000 notes), while anchor
resolution stays at 0.2–0.5 ms from 61,000 to 601,000 block rows.

## Schema v17 — the batch idempotency ledger

`batch_operations` (v0.6 F1) records which batch request keys have already run,
and what they returned.

| Column | Meaning |
|---|---|
| `request_key` | the caller's idempotency key (primary key) |
| `operation`, `mode` | what ran, for readability in the table |
| `response` | the first run's report, verbatim |
| `request_sha256` | a fingerprint of the arguments the key was used for |
| `created_at` | when it ran |

Two decisions are embedded here. **The ledger is on disk rather than in memory**
because a batch is retried exactly when something went wrong — a dropped
connection, a client restart — and an in-memory map forgets precisely then.

**The response is stored, not recomputed.** A replay returns the first run's
outcomes verbatim; re-deriving them would describe a library that has since
moved on, which is a different answer to the same question. `request_sha256`
exists so a key reused with different arguments can be refused rather than
answered with an unrelated result.


## Schema v16 — title and filename indexes

Two indexes, added for v0.5 E5:

```sql
CREATE INDEX documents_title_idx  ON documents(collection_id, deleted_at, title COLLATE NOCASE, id);
CREATE INDEX resources_filename_idx ON resources(collection_id, filename COLLATE NOCASE, id);
```

They exist because resolving a link by title ran `lower(title) = lower(?)`,
which no index can serve. Every link that did not already name a URI therefore
cost a full scan of the document table — once per link, on every save and every
lint pass. The comparison moved to the NOCASE collation, which is the same
comparison: SQLite's built-in `lower()` folds ASCII only, exactly as NOCASE
does. The lookup is now an index probe.

The collation does a second job. It makes `title LIKE 'prefix%'` a range scan
rather than a table scan, which is what bounds the E5 suggestion endpoint: the
scan starts at the first matching title, walks in title order, and stops one row
past the page. Including `id` in the index makes `ORDER BY title COLLATE NOCASE,
id` exactly index order, so no temporary B-tree is built for the tie-break.

Both are pure additions. No row changes and no data migration; an upgraded
database gets the indexes on the next startup.

## Schema v18 — the job control plane

`jobs` (v0.6 F6) records one run of a long operation: an importer, archive
export, snapshot, or (through G15's closed extension) sync operation.

| Column | Meaning |
|---|---|
| `id`, `kind` | the record and which operation it was |
| `status` | `queued`, `running`, `succeeded`, `failed`, or `cancelled` |
| `parameters` | JSON list of `{name, value, path}`, enough to reproduce the run |
| `phase`, `processed`, `total` | the last progress report |
| `summary`, `error` | the bounded result, and why it stopped |
| `cancel_requested` | the cooperative stop flag |
| `created_at`, `started_at`, `finished_at`, `heartbeat_at` | when |

Four decisions are embedded here.

**There is no `interrupted` status**, even though callers see one. A process
that dies cannot write its own epitaph, so a job left `running` with a heartbeat
older than two minutes is *derived* as interrupted at read time. A sweeper that
wrote it would have to decide another process is dead, and two Notrios processes
against one database — `notriosd` serving the GUI while `notriosctl` imports —
would take turns declaring each other's work over.

**Ordinary records persist across a restart; their work does not.** Imports
resume through their own `import_checkpoints`. G15's sync-only companion table
is the narrow exception: it preserves phase/count checkpoints and replans from
canonical vectors and verified resource chunks.

**There is no priority, no dependency, and no queue column**, because none of
those is a job record — they are a scheduler, which this deliberately is not.
Sequencing lives in the caller's shell, which is what the `jobs status` exit
codes are for.

**Parameters are stored; the command is rendered.** Storing raw argv would have
captured local paths and any secret that happened to be on the command line, and
a stored string cannot improve when a flag is renamed. The `path` flag on each
parameter is what lets `notriosctl jobs show` print a path locally while REST and
MCP return no parameters at all.

Listing orders by `rowid`, not `created_at`: `CURRENT_TIMESTAMP` has one-second
resolution, so jobs started together share a timestamp and a tiebreak on the
random job ID would look like chronology without being it.

## Schema v26 — durable sync job outbox

G15 adds `sync_jobs`, keyed one-to-one to a closed sync kind in `jobs`. It is a
durable outbox, not a general scheduler: there are no commands, raw argv,
priorities, dependencies, DAG edges, or cron fields.

| Column | Meaning |
|---|---|
| `actor` | provenance: `cli`, `rest`, `mcp`, or `service` |
| `target_id` | opaque SHA-256-derived target identity; never a path, URL, credential reference, or key |
| `attempt`, `max_attempts`, `next_attempt_at`, `retry_code` | bounded exponential retry state (`offline`, `quota`, `temporary`, or `byte_budget`) |
| `byte_budget`, `bytes_used` | per-attempt encrypted artifact allowance |
| `checkpoint` | at most 4 KiB of content-free phase/count metadata; canonical vectors and verified chunks remain the resume authority |
| `lease_owner` | process-local worker token, never exposed over REST/MCP |

Claiming holds `BEGIN IMMEDIATE`, recovers stale heartbeats, and refuses a
second running row for the same target. Checkpoint, retry/reset, and settlement
update the base job, sync extension, and `sync_job_audit` event in one
transaction. `sync_job_audit` stores bounded event codes/counts only.

## Schema v19 — local replication journal

G4 adds local replication durability without adding a transport or remote
admission path:

- `sync_replicas` records local/peer identities and active/retired/revoked
  lifecycle state.
- `sync_snapshot_boundaries` records the explicit full-snapshot floor at which
  a replica begins journaling. `sync_local_journal` is the one local monotonic
  allocator and points at that boundary.
- `sync_operations` stores immutable `(replica_id, sequence)` operations;
  `sync_operation_dependencies` reserves their causal edges.
- `sync_state_vectors` stores contiguous positions and `sync_state_gaps`
  stores explicit missing ranges. `sync_peer_acknowledgements` is separate from
  both because receipt and peer acknowledgement are different claims.
- `sync_pending_admissions` is disk-backed and rejects an individual encoded
  operation over 1 MiB. G5 enforces aggregate count/byte admission limits.
- `sync_audit_events` records enrollment and identity-retirement events now and
  reserves the bounded operational audit surface for later slices.

Before enrollment, `sync_local_journal` has no row and every canonical capture
trigger is inert. Enrollment writes a sequence-zero boundary and the local
vector. Afterward, triggers on sync-relevant canonical tables write into
`sync_journal_capture`; its single trigger increments the allocator, inserts
the immutable operation, advances the local contiguous vector, and deletes the
transient row. These statements execute inside the canonical caller's existing
transaction, so rollback removes both the data mutation and operation.

The exact G4 classification is:

| Record type | Class | Captured mutations |
|---|---|---|
| collection | field register | create, update, delete |
| document | field register | create, update, trash, restore, purge |
| revision | immutable record | create only |
| notebook | field register/tree node | create, update, delete |
| tag | field register | create, update, delete |
| document-tag | membership element | add, remove |
| search notebook | field register | create, update, delete |
| resource | immutable content reference plus metadata registers | create, update, delete |
| document-resource | membership element | add, metadata update, remove |
| document source/provenance | field register | create, update, delete |
| exact source-bundle item | immutable content reference plus metadata registers | create, update, delete |

FTS5 rows, parsed links and blocks, projections/Recoll, reports, task
extraction, jobs, batch ledgers, importer checkpoints/item state, local
source-bundle storage paths, media fetch attempts/policy rules, and search
snippets are derived or local operational state and deliberately have no
capture trigger. Resource
operations name the canonical blob hash; physical blob placement remains local
and G8 owns transfer/materialization.

A profile with `target: none` does not accumulate pre-enrollment history. A
non-none target establishes the boundary at service startup but starts no
transport. Rotating/adopting identity retires and disconnects the previous
allocator; the new replica must explicitly enroll from a new snapshot boundary.

## Schema v20 — state-vector admission compatibility

G5 adds `sync_peer_compatibility`, keyed by an explicitly configured peer
replica. It persists the exact protocol major/minor range, schema and compatible
schema range, and sorted required/optional capability arrays used to approve
local admission fixtures. A successful handshake never inserts this row; the
caller must configure the already known peer explicitly. Cryptographic
proof-of-possession enrollment and key material do not belong in this table and
remain G9/G13 work.

The G4 tables now have live bounded semantics:

- `sync_pending_admissions` holds strict normalized operation bytes while a
  source sequence is out of order or a named operation dependency is absent.
  One operation remains capped at 1 MiB; each source is capped at 10,000 rows
  and 64 MiB, and one admission call at 10,000 rows/16 MiB.
- `sync_state_vectors` advances only through the next contiguous, dependency-
  complete sequence in the same transaction that moves it into
  `sync_operations`. `sync_state_gaps` is rebuilt from durable pending positions
  in that transaction. Neither pending nor rejected input is progress.
- `sync_operation_dependencies` is populated at admission and remains part of
  the immutable operation set. Exact operation replay is inert; a sequence or
  operation ID reused for different normalized bytes is a conflict.
- `sync_peer_acknowledgements` accepts only monotonic positions no higher than
  the local durable vector and only for known replicas in the same database.

`sync_journal_sequence_exhaustion` aborts the canonical statement before the
local allocator can exceed `9223372036854775806`; SQLite must never silently
promote an exhausted sequence integer. The G5-only `sync_noop/sync.noop` record
pair exists solely for local convergence fixtures. It is not produced by a
canonical-table trigger and is not a future-extension escape hatch: every
other unknown record/kind pair is refused.

## Schema v5/v6 — Notrios redesign (tasks R3 and R4 implemented)

Schema v5 (notebooks) and v6 (source provenance) are live in `migrations/0001_initial.sql` (with an `ensureSchemaV5` upgrade shim for v4 databases that adds `documents.notebook_id` and backfills existing rows into the default notebook). It adds the note-taking data model on top of the existing document tables:

### notebooks

- Nested notebooks via `parent_id`; stable opaque IDs; `builtin` flag; `position` for manual ordering.
- Optional `icon_emoji` displayed before the name in sidebars.
- Names are case-insensitively unique among siblings (`NOCASE` unique expression index); violations surface as name-conflict errors.
- Every managed note belongs to exactly one notebook (`documents.notebook_id`); bootstrap creates the default "Notes" notebook (`nb_notes`, undeletable) and the builtin read-only "Help" notebook (`nb_help`).
- Deleting a notebook trashes its notes recursively and re-homes them to the default notebook; the notebook rows are removed.

### tags and note_tags

- First-class tag rows with join table; per-tag note counts are derivable for the sidebar.
- Importers map source tags here (Joplin note-tag joins, frontmatter tags).

### search_notebooks

- Name (case-insensitively unique), optional emoji, query string (in the `SEARCH_QUERY_LANGUAGE.md` syntax), `builtin` flag, and `sort_anchor` (`first`/`normal`/`last`).
- Builtin rows bootstrapped: "All notes" (`snb_all_notes`, first, undeletable, empty query) and "Trash" (`snb_trash`, last, undeletable, reserved query `is:trashed`).
- Deleting a user search notebook deletes only the row, never notes.

### document_sources (schema v6)

One provenance row per externally-sourced document (Joplin, Obsidian, Twitter/X, ChatGPT, Claude); purely local notes have no row:

- `source_system` (normalized lowercase), `external_id`, `source_url`; indexed by `(source_system, external_id)` for idempotent importer lookups (`FindDocumentBySource`);
- `author` (display name) and `author_id` (canonical account identity, kept separate so same-named people are never conflated);
- `thread_id` and `reply_to` (source-native external IDs) so Twitter/X conversation threads and ChatGPT/Claude conversations can be recovered in chronological order (`ListThreadDocuments`; trashed notes are excluded); links may point at the original posts;
- `published_at` (ISO 8601 as provided; digit-only epoch seconds/milliseconds also accepted) with derived `published_ts` UTC Unix seconds for range queries;
- `metadata_json` for source-specific extras.

The Joplin and Obsidian importers create/update these rows with canonical
changes and backfill missing provenance. A fingerprint- and source-mapping-
verified Joplin no-op skips identical provenance/state rewrites while advancing
the checkpoint atomically.

Thread/link-graph traversal stays in SQLite; the Recoll index only carries searchable copies of these fields (`RECOLL_INTEGRATION.md`).

### Deletion rules

- Soft-deleted notes are excluded from every query and appear only through the "Trash" search notebook (`ListTrash`); restore makes them visible and searchable again.
- Permanent deletion (purge) currently requires the note to be in the trash,
  removes its revisions/tags/links/FTS rows, and marks inbound links
  `target_deleted`. It is only permitted for notes whose source is the local
  database; externally-sourced trashed notes are refused. With sync, purge must
  instead leave a replicated death certificate and defer payload/blob
  collection until retention plus peer acknowledgements make it safe.

## Future migrations

## Schema-v25/v26 physical image policy

G14c adds no table or migration. It originally copied schema v25 with SQLite Online Backup
and securely clears machine-local resumptions from the private image before
publication. The exhaustive retained/cleared/rebuildable classification is in
`performance/v0.7-g14c/STATE_REVIEW.md`. Admission requires the image's
`PRAGMA user_version`, database identity, local-observer vector, catch-up
floors, and cleared-table assertions to match the manifest exactly. No in-place
migration of a physical image is allowed; an incompatible image uses semantic
archive-v2 instead.

G14d also adds no table or migration. Physical activation occurs only on the
private staged same-schema copy (v26 after G15). It mints a fresh local replica/allocator,
retains the snapshot source as a peer, installs the authenticated snapshot
vector and catch-up floors, clears copied peer acknowledgements, and enqueues
every document in `index_outbox` for external projection rebuild. An adjacent
owner-only JSON restore plan and startup-blocker file make filesystem cutover
roll-forward resumable; they are deliberately outside canonical SQLite state.

Future migrations should be additive where possible. Any destructive change requires a migration note in `plans/` and a backup/export instruction.

Later v0.7 sync schema is described semantically in `SYNCHRONIZATION.md` and
split across `PLAN.md` G6-G17: HLC field/register state, peer key enrollment and
revocation, revision parents/delta references, lazy-resource availability,
tombstones/death certificates, conflicts, jobs, snapshot floors, and retention
watermarks. G2 recommends bounded compact NCB1 operation records with per-kind
canonical-JSON payloads, but G9 must promote or replace that evidence format
deliberately; schema-v20 `payload_json` is local journal/admission state, not the wire
codec. Derived FTS/Recoll data remains excluded.

## Schema v21 — deterministic metadata convergence

G6 adds `hlc_wall_ms` and `hlc_logical` to immutable operations plus a durable
`sync_hlc_clock` that cannot move backward. `sync_field_registers`,
`sync_lifecycle_registers`, and `sync_membership_registers` materialize the
protocol-order winners; `sync_death_certificates` preserves structurally signed
purge identity; and `sync_repair_events` is the deterministic visible report
for notebook cycles/orphans, missing document homes, and uniqueness repairs.

The sequence-zero baseline tables represent canonical state at enrollment, so
pre-enrollment data does not become synthetic history. Every admitted metadata
set is folded from that baseline and applied under `sync_apply_guard` in the
same SQLite transaction as operation/vector advancement. The guard prevents an
applied remote projection from being journaled as a new local mutation. Local
purge is refused while enrolled until G9 supplies real signing and verification;
payload and blob collection remains deferred to G17.

## Schema v22 — note revision objects, transfer deltas, and body conflicts

G7 makes a revision an object rather than a row of text. `document_revisions`
gains `content_sha256`, `content_length`, and `parent_revision_ids`, so every
revision names its exact content and its place in the note's history. The v22
upgrade backfills all three for existing revisions, synthesizing the parent
chain from each document's own revision order — which is the honest reading of a
pre-v22 history, because it had no branching. The tiebreak within one
`created_at` second is `rowid`, not `id`: revision ids are random, and ordering
by one would give a note's history an arbitrary direction.

`sync_capture_revisions_insert` now emits a named `revision.create` operation
and **refuses** an enrolled insert whose content hash is missing or malformed. A
revision without one could not be verified after transfer, could not be a delta
base, and could not be found as a merge ancestor.

Three tables support the transfer and the disagreement:

- `sync_revision_deltas` — an optional named-base delta in the constrained
  RFC 3284 VCDIFF profile, stored base64 with both endpoint hashes. It is a
  transfer optimization and never canonical state; the complete body stays in
  `document_revisions`, so a receiver that lacks the base can still obtain it.
- `sync_revision_pending_bodies` — a revision this replica knows exists but
  whose bytes it does not hold, with the reason: `oversize` (larger than an
  operation payload, awaiting the G8/G9 object path), `missing_base` (a delta
  named a base this replica lacks), or `unverified` (a refusal, and terminal).
- `sync_document_conflicts` — a durable typed conflict attached to the document
  and its revision graph, never a second note. Its two revisions are stored in
  sorted order rather than as "mine" and "theirs", because each replica calls a
  different one local and a conflict with two identities would be reported twice
  and resolved once.

`sync_revision_transfer` is transient: Go writes one row immediately before
inserting a revision when it has measured an exact inline payload or generated a
beneficial delta, and the capture trigger consumes and deletes it. With no row
the trigger inlines a body of at most 65,536 bytes, so a code path that predates
G7 still journals a usable object.

A document's `current_revision_id` is **not** a last-writer-wins field. It is
derived from the revision graph: the single head, the merge that reduced two
heads to one, or — while a conflict stands — the newer of the two heads by the
same `(wall, logical, replica, sequence)` order G6 uses, so a conflicted
document does not additionally disagree about which side it is displaying.

## Schema v23 — attachment metadata that arrives before its bytes

G8 lets a blob row exist without a file. That is what makes lazy
materialization possible: a note can reference an attachment whose identity,
size, and type this replica knows but has not downloaded. `blobs.availability`
is `local` or `unavailable`, and a trigger refuses in **both directions** any
row whose availability contradicts whether it has a storage path — a CHECK
constraint cannot be added to an existing table, and a rule that only applied to
databases created after v23 would be the rule least likely to hold.

- `sync_blob_manifests` — an object's byte length, content type, chunk count,
  chunk size, and the manifest's own digest. That digest is what travels inside
  a bounded operation payload: 16,384 chunk hashes would not fit in one, and a
  digest lets the manifest be fetched by another route and still be verified on
  arrival. `complete` distinguishes a manifest built from a local file from a
  shell that only records the shape.
- `sync_blob_chunks` — one row per transfer segment with its hash, length, and
  whether it has been fetched. This is what makes a resumed download resume.
- `sync_blob_sources` — which peers advertised the object. Advertisements are
  kept: the answer to "nobody has it right now" is to ask again later, not to
  forget who used to.
- `sync_blob_materialization` — this replica's own intent, separate from what
  the protocol says exists: `pinned`, `requested`, attempt count, staged bytes,
  and the reason the last pass produced no bytes.

The resource capture trigger now names the blob's length, content type, chunk
count, and manifest digest alongside the hash it already carried, so a receiver
can decide whether to fetch and how to verify. Go maintains
`sync_blob_manifests` for every local blob, so the trigger reads rather than
computes.

Partially fetched objects live in `<asset root>/staging/<object hash>/<ordinal>`
— inside the asset root but deliberately outside the content-addressed tree, so
a half-downloaded object is never reachable as a blob.

Two consequences elsewhere. Garbage collection removes an unmaterialized blob's
transfer state and staged chunks with it, so a later admission of the same bytes
does not believe it has already fetched them. And an archive-v2 export refuses,
naming the object, rather than producing a container that silently omits bytes
it claims to hold.

## Schema v24 — snapshot catch-up and the reset state machine

G10 makes catch-up durable. `sync_catchup_sessions` records where a large,
slow, interruptible process has got to: what was asked, who answered, the
snapshot's recorded state vector, how many archive bytes have arrived, the
explicit restore intent, and why a session stopped. `sync_catchup_permissions`
is separate from enrollment on purpose — a peer may exchange operations and
still not be allowed to answer a backup request, because answering means
handing over a complete copy of the library.

`sync_catchup_floors` is the table that makes catch-up work at all. G5 admits an
operation only when the row for the sequence before it is present, and a replica
built from a snapshot does not have those rows and correctly should not: the
snapshot *is* that history, already folded into canonical state. A floor records,
per replica, the sequence a restored snapshot accounts for. Admission consults it
in exactly two places — the missing-predecessor check, and the below-the-vector
check that reports an already-contained operation as an inert duplicate rather
than a conflict. It is the only thing permitted to stand in for a missing
predecessor; a replica without a snapshot still leaves the same operations
pending rather than admitting a history with a hole in it.

Cutover installs the snapshot's vector as this replica's own observation and
writes no peer acknowledgement: this replica holding a copy says nothing about
what any peer has durably admitted, so a backup must never hold back collection.

## Schema v25 — peer keys and pairing invitations

G13 moves two things into the database, and both are about who may talk to this
replica.

`sync_peer_keys` holds each enrolled peer's **public** signing key, its replica,
its status, and when it was enrolled or revoked. Nothing secret moved: these are
public keys. What was gained is that enrolling and revoking are transactional,
auditable, and visible to every process at once — a key file read by a daemon
and rewritten by a CLI is a race with a security outcome. A revoked key is
reported to the verifier as unknown and cannot be re-enrolled; minting a new one
is the deliberate act.

`sync_pairing_invitations` makes a pairing code single-use across restarts. The
code itself is never stored — only its SHA-256, so a stolen database yields no
usable invitation — and the row is moved to `consumed` in the same statement
that checks it is open and unexpired. Two peers racing one code therefore
produce one pairing and one refusal rather than two pairings, and an expired or
consumed row is evidence rather than a credential.

The private half of this — this replica's signing key and the library group key
per epoch — stays outside the database in the warned `0600` key file, because a
database copied to another machine must not carry the keys that decrypt its
traffic.

## G16 UI projection — no schema change

G16 remains at schema v26. Sync Center state is a bounded projection of the
existing journal/vector, peer-key, catch-up-permission, job/audit, conflict,
blob/materialization, and repair rows. Pin/request intent uses the existing
resource materialization table; conflict resolution adds an ordinary immutable
revision whose two parents are the conflicting heads; snapshot permission uses
`sync_catchup_permissions`; no UI-only canonical table or stored password was
added. NPB1 password-backup headers and private verification staging are files,
not database records.

## Schema v27 — peer retirement and safe collection floors

G17 adds `sync_peer_retirements` for signed ordinary-log retirement identity;
`sync_verified_snapshots` plus vectors for local verification evidence;
`sync_retention_floors` for monotonic collected history; and
`sync_tombstone_payloads` for permanent-delete payload still awaiting the safe
floor. `sync_retained_revision_orders` preserves deterministic merge ordering
after creation operations are compacted, while
`sync_metadata_baseline_deaths` keeps the minimum permanent-death identity in
the canonical checkpoint after payload/log collection.

Snapshot verification records are local operational evidence and contain no
path or bytes. Published physical images clear those records and peer
acknowledgements, but preserve canonical collection floors, retirements,
revision order, and death identity. A repair snapshot is usable only when its
vector covers every existing retention floor. Destructive CLI entry points
re-verify the actual retained directory and database identity at invocation;
the stored record alone never proves the directory still exists.
