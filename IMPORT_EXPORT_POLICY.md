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

Do not write directly into sist2 indexes. Generate canonical records and projections, then ask sist2 to scan/index the projection.

## Export

Two export classes are required:

1. Portable Markdown vault export for interoperability.
2. Lossless archive export for restoration.

## Publishing

Publishing is privacy-sensitive. Quartz publishing profiles must select public subsets and copy only reachable public resources. Do not rely on static-site-generator private-page filters alone to protect resources.

## Export format distinction

Use an Obsidian-like portable Markdown vault as the default user-facing export because it is human-readable, portable, and directly indexable. Use a lossless structured application archive for restore fidelity. Joplin RAW is the preferred Joplin import source and only a later export target if exact Joplin round-trip is required.

## Joplin RAW importer MVP behavior

The MVP importer is implemented as `notesctl import joplin-raw`. It imports notes and resources into the canonical store, not into sist2. It preserves selected Joplin metadata in Markdown frontmatter and rewrites internal `:/<id>` links to companion app URIs. Production hardening should add a dedicated import-job/source-object table before large migrations.
