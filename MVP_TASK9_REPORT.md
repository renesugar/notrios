# MVP Task 9 Report — Obsidian Importer MVP

## Status

Completed.

## Goal

Implement the minimum Obsidian vault importer needed for the MVP: import a folder of Markdown notes and local assets, preserve source paths/frontmatter/Wikilinks/embeds/unresolved links, and ensure graph/backlinks work for a small vault fixture.

## Implemented

- Added `internal/importers/obsidian`.
- Added `notesctl import obsidian` with:
  - `--config`
  - `--db`
  - `--asset-store`
  - `--collection`
  - `--dry-run`
- Scans Markdown files with `.md` and `.markdown` extensions.
- Imports non-Markdown local assets as logical resources backed by SHA-256 content-addressed blobs.
- Skips `.obsidian`, VCS directories, `node_modules`, and hidden files/directories that should not become notes/resources in the MVP.
- Uses deterministic vault-path-derived IDs:
  - `doc_obsidian_<safe-vault-path>` for documents
  - `res_obsidian_<safe-vault-path>` for resources
- Preserves source Markdown and original frontmatter while adding:
  - `source_system: obsidian`
  - `obsidian_path`
  - `obsidian_folder` when applicable
- Selects document title from frontmatter `title`, first level-one heading, or filename fallback.
- Preserves Obsidian Wikilinks and embeds in the Markdown source.
- Attaches referenced local assets found through Markdown image/file links and Obsidian embeds.
- Preserves unresolved links for later graph/lint workflows.
- Added `store.RebuildDocumentLinks` so batch importers can refresh link resolution after all notes/resources have been created.
- Added tests for:
  - note/resource import
  - frontmatter preservation and augmentation
  - asset attachment
  - Wikilink and embed resolution
  - backlinks
  - unresolved-link preservation
  - searchability
  - idempotent re-run behavior
  - dry-run behavior

## Deliberate limitations

- The importer does not yet maintain a dedicated import-job/source-object table.
- Path and alias resolution are still limited by the MVP Markdown link parser.
- Duplicate note titles or duplicate asset filenames may produce ambiguous links until a richer resolver is implemented.
- Obsidian Canvas, plugin-specific frontmatter, Dataview queries, Tasks syntax, and unusual YAML structures are preserved as text but not interpreted.
- Imported local image links are not rewritten to `resource://` URIs in this slice; the graph/resource references are available for later UI/export rewrites.
- No real private Obsidian vault was available in this container; only synthetic fixtures were tested.

## Validation

Executed successfully:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

Manual CLI smoke testing was also performed with a temporary vault fixture.

## Next suggested task

Proceed to MVP Task 10: release hardening, generated-dataset smoke/performance tests, packaging notes, and security review of resource download and preview sanitization.
