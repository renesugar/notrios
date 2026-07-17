# Database Schema

This document expands the MVP SQLite schema represented by `migrations/0001_initial.sql`. SQLite is the authoritative store for managed notes, metadata, revisions, resource relationships, link graphs, media-policy decisions, import state, and outbox jobs. Recoll is the optional derived index for front-matter field search, extraction, and broad filesystem search (see `RECOLL_INTEGRATION.md`).

## Schema principles

- Keep application identity stable with opaque IDs; do not expose rowids or search-index IDs as public identities.
- Keep managed note saves transactional: document row, current revision, FTS row, links, resource references, and outbox records are updated in one SQLite transaction.
- Store binary bytes outside SQLite in a content-addressed asset directory, while SQLite tracks blob hashes, resources, references, and provenance.
- Treat FTS5 and Recoll as derived indexes. FTS5 is updated synchronously for managed documents; the Recoll projection is updated asynchronously through `index_outbox`.
- Use soft deletion for documents first. MVP Task 4 implements safe resource deletion by refusing referenced resources and removing unreferenced logical resources; richer trash/GC policy remains future work.
- Keep import and projection paths deterministic so bulk imports can be resumed and repeated idempotently.

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

### document_links and document_blocks

`document_links` records explicit links parsed from Markdown or imported source data. MVP Task 5 populates it transactionally on document create, update, restore, and soft-delete cleanup. It stores source syntax, raw target, display text, optional normalized target URI, resolved document/resource IDs, anchor type/value, context excerpt, source byte/line/column positions, relation type, and resolution status. Status values used in the MVP are `resolved`, `unresolved`, `ambiguous`, and `external`; later importers may add `invalid` and `target_deleted`.

`document_blocks` remains planned. Heading and block anchors are currently stored on link records, not as separately addressable block rows.

### media policy (schema v7, v0.3 task H1)

The media-policy tables support domain stop lists, exact-hash blocks, perceptual-hash blocks, remote media attempts, quarantine state, and resource hash records. As of schema v7:

- `media_domain_rules` — user-managed domain patterns with an `allow`/`block`/`review` action (unique, case-insensitive), complementing the `remote_media` config lists.
- `media_hash_rules` — hash-keyed rules (`algo` + `hash`), `kind` distinguishing `exact` from `perceptual`, action `block` or `review`. Exact hashes may block; perceptual hashes only ever raise review signals.
- `resource_hashes` — additional hashes per stored blob (`blob_sha256` + `algo`), the storage slot for future perceptual hashes.
- `media_policy_decisions` — one row per remote-media attempt: original/final URL, decision, reason, hashes, plus v7 quarantine-state columns (`status`, `content_type`, `size_bytes`, `quarantine_path`, `updated_at`). Populated by the H3 quarantine pipeline (`store.RecordMediaAttempt` / `ListMediaAttempts`): every fetch attempt — quarantined or refused — is recorded.

`media_hash_rules` is consulted during localization admission (H4, via `store.AddMediaHashRule`/`FindMediaHashRule`): exact-hash `block`/`review` rules stop quarantined content before it reaches the asset store. `media_domain_rules` (user-managed rules beyond the config lists) and `resource_hashes` come online with rule CRUD and the H5 hashing task.

### index_outbox

The outbox coordinates filesystem projections and Recoll indexing after the canonical SQLite transaction commits. Outbox jobs are retried and coalesced; failure does not invalidate the note save.

## MVP migration file

`migrations/0001_initial.sql` has grown with each milestone and now represents schema version 7 (v5 notebooks/tags/search notebooks, v6 source provenance, v7 media policy), applied idempotently on every startup with upgrade shims for older databases. Its original MVP portion represents schema version 4. It includes managed-document tables, Task 2 revision fields (`body_mime_type`, `message`), content-addressed blob/resource tables, document-resource reference tables, document link graph rows, link context/target URI fields, and supporting indexes. Bootstrap contains compatibility shims for older development databases before setting `PRAGMA user_version = 4`. Do not rename public tables/columns casually once tests depend on them.

## Required indexes

The migration includes indexes for common lookups:

- collections by kind;
- documents by collection, updated time, and deletion status;
- revisions by document and creation time;
- resources by collection and blob hash;
- links by source, target, relation, and unresolved status;
- resource references by document and resource;
- outbox by completion and sequence;
- media hashes by algorithm/hash.

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

The Joplin and Obsidian importers write these rows on every run (create, update, or unchanged), so re-running an import backfills provenance for previously-imported notes.

Thread/link-graph traversal stays in SQLite; the Recoll index only carries searchable copies of these fields (`RECOLL_INTEGRATION.md`).

### Deletion rules

- Soft-deleted notes are excluded from every query and appear only through the "Trash" search notebook (`ListTrash`); restore makes them visible and searchable again.
- Permanent deletion (purge) requires the note to be in the trash, removes its revisions/tags/links/FTS rows, and marks inbound links `target_deleted`. It is only permitted for notes whose source is the local database; externally-sourced trashed notes (any `document_sources` row) are refused and stay merely excluded from queries/results and exports.

## Future migrations

Future migrations should be additive where possible. Any destructive change requires a migration note in `plans/` and a backup/export instruction.
