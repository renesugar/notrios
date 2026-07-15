# MVP Task 8 Report — Joplin RAW Importer MVP

## Status

Completed.

## Implemented

- Added `internal/importers/joplinraw`.
- Added tolerant Joplin RAW item parsing for both body-first note files with trailing metadata and metadata-first fixture files.
- Recognizes Joplin item types used by the MVP:
  - `type_: 1` notes.
  - `type_: 2` notebooks/folders.
  - `type_: 4` resources.
  - `type_: 5` tags.
  - `type_: 6` note-tag joins.
- Imports notes as managed Markdown documents with deterministic IDs of the form `doc_joplin_<joplin-id>`.
- Imports resources as logical content-addressed resources with deterministic IDs of the form `res_joplin_<joplin-id>`.
- Rewrites Joplin internal `:/<id>` note/resource links to companion `document://` and `resource://` URIs.
- Adds source metadata in Markdown frontmatter, including original Joplin IDs, parent IDs, notebook path, tags, selected timestamps, and source URL when present.
- Attaches resources referenced by imported note bodies to the corresponding managed document.
- Added `notesctl import joplin-raw` with `--config`, `--db`, `--asset-store`, `--collection`, and `--dry-run` options.
- Added importer tests with a small RAW fixture and idempotent re-run behavior.
- Added optional preferred IDs to store create requests so importers can keep deterministic external-source mappings.

## Validation

Manual smoke test also passed with `go run ./cmd/notesctl import joplin-raw --db <tmp>/notes.sqlite --asset-store <tmp>/assets <tmp>/raw`, which imported one body-first RAW note and printed a JSON report.

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci
cd web && npm run typecheck
cd web && npm run build
```

## Limitations

- The parser is intentionally tolerant and MVP-level. Codex should verify against real Joplin RAW exports from multiple Joplin versions before relying on it for production migrations.
- Import metadata is preserved in Markdown frontmatter, not yet in a dedicated source-object metadata table.
- Import is idempotent for deterministic document/resource IDs and unchanged bodies, but it does not yet implement a full resumable import job table.
- Resource content lookup supports common RAW layouts such as `resources/<id>.<ext>`, `.resource/<id>`, and root-level `<id>`, but real-world exports may require additional paths.
- The importer does not yet preserve todos, geolocation, conflict state, encryption metadata, or full Joplin application data beyond selected frontmatter fields.
