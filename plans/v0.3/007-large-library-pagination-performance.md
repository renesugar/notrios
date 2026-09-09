# v0.3 H7 — Large-library pagination and performance baseline

Status: completed 2026-07-26

Model: GPT-5.6

## Delivered

- Added schema v9 indexes for chronological collection, notebook, Trash, and
  tag-membership traversal. Migration and query-plan tests verify the expected
  covering indexes.
- Replaced SQLite offset paging and the 100,000-result ceiling with opaque,
  query/sort-bound `k2` cursors:
  - chronological pages use `(updated_at DESC, id DESC)`;
  - reproducible FTS5 relevance pages use `(bm25 score ASC, id ASC)`;
  - notebook and Trash cursors are bound to the exact route/container.
- Made `GET /api/v1/search` a live search surface and updated REST, MCP, API
  types, OpenAPI, and status metadata for the new page contracts.
- Added stable optional-sidecar paging. When Recoll contributes sidecar-only
  hits, FTS5-first merged order is frozen in an immutable `m1` snapshot:
  1,000-hit cap, ten-minute TTL, at most 32 cached snapshots, explicit
  `truncated`, and `cursor_invalid` on expiry or replay mismatch. Canonical-only
  results retain live keyset cursors.
- Added an opt-in synthetic profile covering 10k, 100k, and 500k notes, two
  links per note, tags/notebooks, logical resources, and exact-byte blob
  deduplication. The profile records environment, query plans, p50/p95/max
  latency, database size, peak RSS, batch seed time, and full cursor-stream
  traversal time.
- Added `scripts/run_large_library_profile.sh`, integrated the 10k gate into
  `scripts/run_performance_smoke.sh`, and committed all three reference JSON
  profiles under `performance/v0.3-h7/`.

## Cursor and snapshot semantics

- Clients treat cursors as opaque and return them unchanged.
- A cursor is bound to its route, normalized collection, query parse, paging
  mode, and stable sort. Cross-query, cross-sort, cross-notebook, or
  notebook-to-Trash replay fails with `400 cursor_invalid`.
- Cursor SQL uses row-value keyset predicates for chronological traversal and a
  score/ID boundary over a ranked FTS CTE. No SQLite query uses `OFFSET`.
- Optional Recoll is still derived and non-canonical. Every sidecar document is
  revalidated in SQLite before entering the bounded snapshot; failure falls
  back to canonical FTS5 paging.

## Performance evidence

The committed profiles are reproducible evidence for the recorded reference
machine, not hardware-independent service-level guarantees. Every 10k/100k/500k
run met the ordinary first/next/90%-deep page p95 target of 100 ms. See
`performance/v0.3-h7/README.md` and the individual JSON files for precise
latencies, sizes, peak RSS, environment, and `EXPLAIN QUERY PLAN` output.

## Validation

- focused schema/query-plan, chronological/relevance pagination,
  notebook/Trash, live GET search, and stable merged-snapshot tests
- opt-in 10k, 100k, and 500k profile runs with committed JSON evidence
- OpenAPI YAML parse and migration-copy equality
- `go vet ./...`
- `go test ./...`
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- `cd web && npm run typecheck && npm run build && npm test`
- `bash scripts/build_docs_site.sh`
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
