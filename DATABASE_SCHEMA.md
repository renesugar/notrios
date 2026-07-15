# Database Schema

This document expands the MVP SQLite schema represented by `migrations/0001_initial.sql`. SQLite is the authoritative store for managed notes, metadata, revisions, resource relationships, link graphs, media-policy decisions, import state, and outbox jobs. sist2 remains a derived index for extraction, OCR, thumbnails, and broad filesystem search.

## Schema principles

- Keep application identity stable with opaque IDs; do not expose rowids or sist2 IDs as public identities.
- Keep managed note saves transactional: document row, current revision, FTS row, links, resource references, and outbox records are updated in one SQLite transaction.
- Store binary bytes outside SQLite in a content-addressed asset directory, while SQLite tracks blob hashes, resources, references, and provenance.
- Treat FTS5 and sist2 as derived indexes. FTS5 is updated synchronously for managed documents; sist2 is updated asynchronously through `index_outbox`.
- Use soft deletion for documents first. MVP Task 4 implements safe resource deletion by refusing referenced resources and removing unreferenced logical resources; richer trash/GC policy remains future work.
- Keep import and projection paths deterministic so bulk imports can be resumed and repeated idempotently.

## Core entities

### collections

A collection is a logical namespace such as `personal-notes`, `joplin-raw-2026-07`, `obsidian-vault`, `twitter-archive`, or `research-pdfs`.

Important fields:

- `id`: stable public collection ID.
- `kind`: managed, external, imported, sist2, or projection.
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

### media policy

The media-policy tables support domain stop lists, exact-hash blocks, perceptual-hash blocks, remote media attempts, quarantine state, and resource hash records.

### index_outbox

The outbox coordinates filesystem projections and sist2 indexing after the canonical SQLite transaction commits. Outbox jobs are retried and coalesced; failure does not invalidate the note save.

## MVP migration file

`migrations/0001_initial.sql` now represents schema version 4 for this MVP branch. It includes managed-document tables, Task 2 revision fields (`body_mime_type`, `message`), content-addressed blob/resource tables, document-resource reference tables, document link graph rows, link context/target URI fields, and supporting indexes. Bootstrap contains compatibility shims for older development databases before setting `PRAGMA user_version = 4`. Codex should not rename public tables/columns casually once tests depend on them.

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

## Future migrations

Future migrations should be additive where possible. Any destructive change requires a migration note in `plans/` and a backup/export instruction.
