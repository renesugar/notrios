# v0.3 H10 — Recoll sidecar hardening

Status: completed 2026-07-26

Model: GPT-5.6

## Delivered

- Added schema-v11 durable projection retries. Failed `index_outbox` jobs
  retain bounded errors and receive persisted exponential backoff from five
  seconds to a one-hour cap. Due work drains in at most 20 200-item batches;
  a delayed failure does not spin or block later sequence numbers.
- Added exact canonical/projection reconciliation. Canonical documents,
  provenance, tags, and notebook names load in bounded batches; rendered size
  and SHA-256 detect stale files. Missing/stale notes are atomically repaired,
  orphan/temp/symlink files are removed without following them, and a second
  pass converges to zero drift.
- Replaced startup full-sync behavior with exact reconciliation, bounded
  outbox draining, and an initial index. The cancellable worker drains every
  30 seconds and reconciles every 10 minutes. Shutdown waits for the worker
  before closing SQLite.
- Hardened subprocesses with argument arrays, 30-minute index and 30-second
  query deadlines, service cancellation, bounded stdout/stderr, exact Recoll
  result counting, explicit result slices, strict base64/UTF-8/field limits,
  projection-root URL validation, deduplication, and hostile HTML/control
  snippet reduction to bounded plain text.
- Batched sidecar-result revalidation in groups of 500. Canonical and Recoll
  hits deduplicate by document ID; every hit reports `sources` (`sqlite`,
  `fts5`, and/or `recoll`). Any Recoll contribution, including duplicate
  attribution, uses the bounded immutable merged snapshot so order and sources
  stay stable on every cursor page.
- Added `/api/v1/status.search_sidecar`: configured/available/active/state,
  pending/retrying queue counts, last sync/index/reconciliation timestamps,
  repair counts, and bounded last error. Missing Recoll remains an explicit
  `unavailable` state while FTS5 continues.
- Added desktop status polling every 30 seconds, visible Recoll state/backlog/
  last sync, and accessible per-result engine badges.
- Added live installed-Recoll tests for production frontmatter/field queries,
  startup repair/status, subprocess cancellation, exact slices, hostile
  output, and missing-binary fallback.
- Added `scripts/run_recoll_hardening_profile.sh` and committed generated
  100,000-note evidence under `performance/v0.3-h10/`. The scale tier indexed
  100k real `.md` projection paths with Recoll's internal text extractor in
  122.997 seconds, returned the exact 1,001-hit cap in 2.084 seconds, purged a
  deletion and indexed its replacement in 15.035 seconds. The production
  Python frontmatter handler remains covered separately by the real live test,
  avoiding a misleading 100k interpreter-startup benchmark.

## Storage and safety decisions

- SQLite remains canonical; projection files, retry state telemetry, and the
  Recoll index are derived and reconstructible.
- Recoll/Xapian remain user-installed external GPL processes. No GPL library
  is linked, vendored, redistributed, or used as a source for project code.
- Reconciliation deletes only entries directly inside the managed `notes`
  projection directory. Unexpected directories are reported, not traversed.
- Output and error bounds are enforced while preserving graceful FTS5
  fallback; stale/foreign sidecar IDs never become canonical search hits.
- The 1,000-hit merged window remains explicit and bounded. H10 does not
  reintroduce offset traversal.

## Validation

- Focused schema-v11 upgrade/backoff/queue, failure-nonblocking, bounded drain,
  exact reconciliation/damage/convergence, hostile parser, cancellation,
  status, attribution, stable paging, unavailable fallback, and live startup
  repair tests.
- Real `TestLiveRecollPipeline` against installed Recoll 1.44.0/Xapian 1.4.22.
- Generated 100k native Recoll profile with exact bounded count, incremental
  drift convergence, and missing/stale/orphan repair evidence.
- `go vet ./...`
- `go test ./...`
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- `cd web && npm ci && npm run typecheck && npm run build && npm test`
- `make gui`
- `bash scripts/build_docs_site.sh`
- OpenAPI YAML parse and migration-copy equality.
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
