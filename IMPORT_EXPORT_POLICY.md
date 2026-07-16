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

## Export

Two export classes are required:

1. Portable Markdown vault export for interoperability.
2. Lossless archive export for restoration.

Exports may be scoped to query results (e.g. a search notebook) instead of the entire database; notes from external sources that are marked deleted are excluded. The export preserves the notebook structure.

## Native archive format (implemented, task R12)

`notriosctl export archive [--query "tag:todo"] <out-dir>` writes a directory: `manifest.json`, `notebooks.json` (paths + emoji), `notes/<id>.md` (Markdown with front matter carrying title/notebook path/tags/resources), and `resources/<id>__<filename>` bytes. Exports may be query-scoped (e.g. a search notebook's contents) and preserve the notebook structure; trashed and externally-deleted notes are excluded by the query layer.

## Import dry run and import configuration (implemented, task R12)

Re-importing an archive imports notes as **plain local notes**, not references to the original data source (no provenance rows; they are purgeable). `notriosctl import archive`:

- `--dry-run` reports which top-level notebook names conflict with notebooks bound to other data sources (builtin notebooks, or notebooks holding externally-sourced notes) and writes an **import configuration file** (`import-config.json`) with prefilled rename suggestions; merges into plain user notebooks are listed separately;
- the real import (`--import-config path`) validates the chosen names case-insensitively against source-bound notebooks **before writing anything** and refuses on conflict;
- missing notebook paths are created (nesting and emoji preserved) and populated from the archive; imports are idempotent.

Joplin RAW and Obsidian imports currently place notes in the default notebook; populating notebook hierarchies from those sources is deferred to the v0.3 importer-hardening milestone.

## Publishing

Publishing is privacy-sensitive. Quartz publishing profiles must select public subsets and copy only reachable public resources. Do not rely on static-site-generator private-page filters alone to protect resources.

## Export format distinction

Use an Obsidian-like portable Markdown vault as the default user-facing export because it is human-readable, portable, and directly indexable. Use a lossless structured application archive for restore fidelity. Joplin RAW is the preferred Joplin import source and only a later export target if exact Joplin round-trip is required.

## Joplin RAW importer MVP behavior

The MVP importer is implemented as `notriosctl import joplin-raw`. It imports notes and resources into the canonical store, never into any search index. It preserves selected Joplin metadata in Markdown frontmatter and rewrites internal `:/<id>` links to companion app URIs. Production hardening should add a dedicated import-job/source-object table before large migrations.
