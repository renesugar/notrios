# Plan Slice 013 — MVP Joplin RAW Importer

## Goal

Implement the first useful Joplin RAW Export Directory importer that can import a small fixture into managed notes/resources, rewrite Joplin internal links, and leave imported notes searchable.

## Status

Completed.

## Implementation summary

- Added `internal/importers/joplinraw` with tolerant body-first and metadata-first RAW parsing.
- Added deterministic imported object IDs using `doc_joplin_<id>` and `res_joplin_<id>`.
- Imported Joplin notes, folders, tags, note-tag joins, and resources.
- Rewrote `:/<id>` links to `document://` or `resource://` URIs.
- Preserved selected Joplin source metadata in frontmatter.
- Attached referenced resources to imported documents.
- Added `notesctl import joplin-raw` with config/database/asset-store/collection/dry-run flags.
- Added importer unit/integration tests and fixture generation.

## Validation

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

## Model used

GPT-5.5 Thinking.

## Follow-up tasks

- Verify parser against real Joplin RAW exports.
- Add dedicated source object/import job tables.
- Add robust resource path discovery for all Joplin export layouts.
- Preserve more Joplin metadata and todo fields.
- Add import progress reporting and cancellation for large exports.
