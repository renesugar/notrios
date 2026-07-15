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

## Import dry run and import configuration

Re-importing an exported archive (Joplin RAW directory, Obsidian vault, or the native archive format) imports notes as plain notes, not references to the original data source. The import command supports:

- a **dry run** that reports which notebook names conflict with notebooks bound to other data sources;
- the dry run can emit an **import configuration file** allowing notebooks to be renamed on import;
- the real import validates that configuration, checking the chosen names don't conflict with notebooks used for other data sources (case-insensitively);
- notebooks that don't exist are created and populated from the archive.

## Publishing

Publishing is privacy-sensitive. Quartz publishing profiles must select public subsets and copy only reachable public resources. Do not rely on static-site-generator private-page filters alone to protect resources.

## Export format distinction

Use an Obsidian-like portable Markdown vault as the default user-facing export because it is human-readable, portable, and directly indexable. Use a lossless structured application archive for restore fidelity. Joplin RAW is the preferred Joplin import source and only a later export target if exact Joplin round-trip is required.

## Joplin RAW importer MVP behavior

The MVP importer is implemented as `notriosctl import joplin-raw`. It imports notes and resources into the canonical store, never into any search index. It preserves selected Joplin metadata in Markdown frontmatter and rewrites internal `:/<id>` links to companion app URIs. Production hardening should add a dedicated import-job/source-object table before large migrations.
