# v0.3 H9 — Obsidian importer hardening

Status: completed 2026-07-26

Model: GPT-5.6

## Delivered

- Restored every discovered vault folder as a deterministic nested Notrios
  notebook. Plain same-name siblings merge; builtin or source-bound collisions
  stop before writes. Dry run emits path-scoped ` (Obsidian)` rename
  suggestions through the import-config workflow.
- Replaced title-only graph behavior with importer-owned canonical resolution.
  Note-relative and vault-root paths, filenames, frontmatter aliases, Markdown
  links, Wikilinks, document/resource embeds, heading anchors, and block
  references resolve to stable `document://` or `resource://` URIs. Ambiguous
  and missing targets remain unchanged and are reported.
- Added optional exact source capture with `--preserve-source`. Complete
  Markdown bytes retain original frontmatter, CRLF/LF choices, unknown fields,
  and relative paths; every discovered non-Markdown file is captured
  byte-for-byte in the schema-v10 source-bundle namespace. Canonical parsing
  and provenance metadata are additional representations, never replacements.
- Built one deterministic, symlink-refusing vault inventory. Note bodies are
  discarded after discovery and reread only inside the active batch; assets
  are hashed with streaming I/O. Deterministic path IDs handle normalized path
  collisions conservatively.
- Split writes into bounded source-bundle, notebook, resource, note, and final
  link-rebuild phases (1–500 items). Input-scoped indexed lookups replace
  per-file target/state queries.
- Persisted exact file fingerprints, applied item states, inventory/options/
  rename-bound checkpoints, phase/index, and cumulative reports. Interrupted
  imports resume after the last durable batch; changed vaults or incompatible
  source/config choices restart safely while retaining idempotence.
- Added stable resource replacement: changed local asset bytes update the same
  logical deterministic resource ID without breaking document attachments.
- Expanded `notriosctl import obsidian` with `--batch-size`,
  `--preserve-source`, `--write-config`, and `--import-config`, plus durable
  progress output. Dry run uses the real action classifiers and writes no
  canonical row, checkpoint, or bundle object.
- Added focused hierarchy/rich-graph/source-byte/config/parity/resume/resource/
  Trash/idempotence tests and generated 100/10k/100k/500k vault profiles. The
  100 and 10k tiers force interruption/resume real writes; 100k and 500k run
  the complete bounded inventory/dry-run path. Evidence is under
  `performance/v0.3-h9/`.

## Storage and safety decisions

- SQLite remains canonical. Exact source bundles are archival inputs outside
  ordinary attachment blobs and retention GC.
- The default source key is the normalized absolute vault path; collection ID
  remains part of every checkpoint/item key.
- `.obsidian`, `.notrios`, VCS, Trash, and dependency directories are skipped.
  Symlinks are rejected rather than followed outside the selected vault.
- Frontmatter `tags:` remain source/canonical metadata and do not yet become
  sidebar tags; H9 does not silently expand that product behavior.
- Imported notes already in Trash are never resurrected.

## Validation

- Focused exact CRLF Markdown/frontmatter fingerprint and binary source-bundle
  recovery; nested notebook and collision rename; alias/relative/embed/heading/
  block graph; dry-run parity; stable resource update; interruption/resume;
  Trash/idempotence tests.
- Generated profiles: 100 and 10k dry-run/interrupted/resumed real imports;
  100k and 500k inventory/dry-run tiers.
- `go vet ./...`
- `go test ./...`
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- `cd web && npm ci && npm run typecheck && npm run build && npm test`
- `bash scripts/build_docs_site.sh`
- OpenAPI YAML parse and migration-copy equality.
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
