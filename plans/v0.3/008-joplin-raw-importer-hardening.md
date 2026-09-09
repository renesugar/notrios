# v0.3 H8 — Joplin RAW importer hardening

Status: completed 2026-07-26

Model: GPT-5.6

## Delivered

- Restored Joplin folder items as nested Notrios notebooks with deterministic
  IDs and original names. Plain same-name notebooks merge; builtin or
  source-bound collisions stop before writes. Dry run emits path-scoped rename
  suggestions through the existing import-config workflow.
- Promoted every Joplin tag to a real Notrios tag, including unassigned tags.
  Source-derived tag IDs make source renames update the stable tag and retain
  note relationships.
- Added optional exact source capture (`--preserve-source`). Classified RAW
  item bytes are stored byte-for-byte in a separate content-addressed
  `source-bundles/sha256` namespace; the schema-v10 manifest records source
  identity, relative path, exact SHA-256/size, and original property order.
  Unknown/reordered properties and CRLF bytes are covered by tests.
- Replaced the MVP all-in-memory/per-item-query loop with one deterministic
  inventory and bounded phases: source bundle, notebooks, tags, resources,
  notes. Note bodies are discarded after inventory and reread only for the
  active batch. Large target/state reads use indexed input-scoped batch
  queries capped at 500 items.
- Added schema-v10 `import_checkpoints`, `import_item_states`, and
  `source_bundle_items`. Each completed batch persists the inventory
  fingerprint, phase/index, cumulative JSON report, and applied item
  fingerprints. A matching interrupted run resumes at the next batch; changed
  input starts a new plan while unchanged targets remain idempotent.
- Added stable resource replacement so changed Joplin attachment bytes update
  one logical resource ID without breaking note-resource references.
- Expanded dry-run output to use the real action classifier for
  note/resource/notebook/tag creates, updates, merges/skips, conflicts, source
  bundle size, and batch progress. Dry run writes no import state or content.
- Added `--batch-size`, `--preserve-source`, `--write-config`, and
  `--import-config` to `notriosctl import joplin-raw`.
- Added generated 100/10k/100k fixtures with nested folders, tags and
  resources. All tiers exercise the complete inventory/dry-run batches; 100
  and 10k additionally force a real interruption followed by resume and prove
  dry-run parity. The reusable driver is
  `scripts/run_joplin_import_profile.sh`; committed reference JSON is under
  `performance/v0.3-h8/`.

## Storage and safety decisions

- SQLite remains the canonical note/notebook/tag/resource store.
- Exact source bundles are archival inputs, not canonical notes and not
  ordinary resource blobs. Resource reports and retention GC cannot mistake
  them for unreferenced attachments.
- Source keys are normalized absolute source directories unless an embedding
  supplies an explicit key; collection ID remains part of every checkpoint
  and item-state key.
- Batches are limited to 500, matching SQLite variable bounds and preventing
  unbounded importer result sets.
- Dry run and conflict analysis occur before canonical writes. Imported notes
  in Trash are never resurrected.

## Validation

- Focused hierarchy/tag/source-bundle, schema-v10 state, dry-run parity,
  conflict/rename, resource-update, generated hundreds, and resume tests.
- Opt-in 100, 10k, and 100k generated profiles.
- Migration-copy equality and schema-version assertions.
- `go vet ./...`
- `go test ./...`
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- `cd web && npm run typecheck && npm run build && npm test`
- `bash scripts/build_docs_site.sh`
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
