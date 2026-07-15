# Plan Slice 014 — MVP Obsidian Importer

## Goal

Implement PLAN.md task 9: import a small Obsidian-style Markdown vault with local assets into the managed companion store while preserving recognizable Markdown source and making graph/backlinks work.

## Final status

Completed.

## Implementation summary

- Added `internal/importers/obsidian`.
- Added `notesctl import obsidian`.
- Scans Markdown files and non-Markdown local assets.
- Skips `.obsidian`, VCS directories, dependency directories, and hidden files.
- Imports documents/resources with deterministic vault-path-derived IDs.
- Preserves original Markdown/frontmatter and adds source metadata fields.
- Attaches referenced local assets as document-resource references.
- Preserves Wikilinks, embeds, headings, block refs, and unresolved links in source Markdown.
- Added `store.RebuildDocumentLinks` so importers can refresh graph resolution after a batch import.
- Added fixture tests for import, idempotency, resources, graph, backlinks, unresolved links, and dry run.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

## Model

GPT-5.5 Thinking.

## Follow-up tasks

- Test against real Obsidian vaults with duplicate note titles, duplicate asset filenames, unusual YAML, plugin syntax, and large asset folders.
- Add source-object/import-job tables for resumability and auditability.
- Add richer Obsidian resolver support for aliases, path-qualified links, and extensionless asset references.
- Decide whether import should optionally rewrite local asset links to `resource://` URIs or keep source Markdown fully preserved.
