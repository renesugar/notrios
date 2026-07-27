# Import, Export, and Publishing Policy

## Import

Importers are separate commands or jobs but write through the companion service layer.

Priority sources:

- Joplin RAW Export Directory.
- Obsidian vault.
- Twitter/X archive.
- ChatGPT conversations export.
- Claude conversations JSON export.
- Generic Markdown folder.

Do not write directly into search indexes. Generate canonical records and projections, then let the Recoll sidecar scan/index the projection (`RECOLL_INTEGRATION.md`).

Importers must record source provenance (source system, external IDs, author/author ID, thread ID, reply-to, URL) so Twitter/X conversation threads and ChatGPT/Claude conversations are recoverable, and note links can point back to the original posts.

## Transfer classes

Keep the following user promises distinct even where they reuse code:

1. Foreign import is ETL into canonical Notrios transactions.
2. Portable Markdown/Joplin/etc. export is an interoperability projection and
   may be format-limited.
3. Native archive is a full, versioned snapshot containing revisions,
   provenance, source-preservation bundles, blobs, hashes, and compatibility
   metadata.
4. Backup stores a native snapshot but is not an active sync peer.
5. Restore explicitly replaces, merges, adopts, or forks a database.
6. Sync repeatedly exchanges incremental native operations and
   acknowledgements.

Exports may be scoped to query results (e.g. a search notebook) instead of the entire database; notes from external sources that are marked deleted are excluded. The export preserves the notebook structure.

## Native archive v1 (implemented, task R12)

`notriosctl export archive [--query "tag:todo"] <out-dir>` writes a directory: `manifest.json`, `notebooks.json` (paths + emoji), `notes/<id>.md` (Markdown with front matter carrying title/notebook path/tags/resources), and `resources/<id>__<filename>` bytes. Exports may be query-scoped (e.g. a search notebook's contents) and preserve the notebook structure; trashed and externally-deleted notes are excluded by the query layer.

Despite the historical command name, v1 is **not a lossless database backup**:
it imports notes as new plain local notes, omits some provenance/revision/source
representation, and does not define database/replica identity. User docs must
not call it lossless or a substitute for disaster recovery.

## Native archive v2 (planned v0.4)

Use a deterministic versioned manifest and immutable SHA-256-addressed objects.
Include schema/container compatibility, a stable read-snapshot boundary,
notebooks/tags/search notebooks, all selected revisions and provenance,
source-preservation bundles, resources, checksums, and an explicit scope.
Stream records/bytes in bounded batches and publish the manifest last.

This container is also the full-snapshot bootstrap for v0.7 sync. Sync adds
incremental change envelopes and acknowledgements; it must not invent another
blob/archive layout.

## Import dry run and import configuration (implemented, task R12)

Re-importing an archive imports notes as **plain local notes**, not references to the original data source (no provenance rows; they are purgeable). `notriosctl import archive`:

- `--dry-run` reports which top-level notebook names conflict with notebooks bound to other data sources (builtin notebooks, or notebooks holding externally-sourced notes) and writes an **import configuration file** (`import-config.json`) with prefilled rename suggestions; merges into plain user notebooks are listed separately;
- the real import (`--import-config path`) validates the chosen names case-insensitively against source-bound notebooks **before writing anything** and refuses on conflict;
- missing notebook paths are created (nesting and emoji preserved) and populated from the archive; imports are idempotent.

Joplin RAW and Obsidian imports populate deterministic nested notebooks from
their source folder hierarchies. Dry-run conflict plans and path-scoped rename
configuration prevent ambiguous source-bound notebook collisions.

Large importers must inventory once, use deterministic IDs and indexed source
fingerprints, avoid one query per file, batch writes with progress/checkpoints,
and retain exact source bytes/unknown metadata when the source format supports
loss-preserving round trips. Source timestamps remain provenance; they are not
future synchronization clocks.

## Backup and restore

- The default offline backup stops writes, snapshots SQLite consistently, and
  includes every referenced asset plus a checksum manifest.
- An online backup uses SQLite's backup API or an equivalent consistent
  snapshot. Copying only the main database file while WAL writes continue is
  invalid.
- Restore first verifies hashes/versions and preserves an emergency backup of
  current state.
- Replace restore mints a new replica ID. It may retain the logical database ID
  only when rejoining the same sync universe; fork explicitly creates a new
  database ID.
- Merge restore applies records through the same validated merge/import layer.
- A backup target is a sink and never prevents sync tombstone/blob GC.

See `SYNCHRONIZATION.md` for operation ordering, acknowledgements, retention,
and full-resync behavior.

## Publishing

Publishing is privacy-sensitive. Quartz publishing profiles must select public subsets and copy only reachable public resources. Do not rely on static-site-generator private-page filters alone to protect resources.

## Export format distinction

Use an Obsidian-like portable Markdown vault as the default user-facing export
because it is human-readable, portable, and directly indexable. Use native
archive v2 for restore fidelity. Joplin RAW is the preferred Joplin import
source and only a later export target if exact Joplin round-trip is required.

## Joplin RAW importer MVP behavior

The MVP importer is implemented as `notriosctl import joplin-raw`. It imports notes and resources into the canonical store, never into any search index. It preserves selected Joplin metadata in Markdown frontmatter and rewrites internal `:/<id>` links to companion app URIs. Production hardening should add a dedicated import-job/source-object table before large migrations.
