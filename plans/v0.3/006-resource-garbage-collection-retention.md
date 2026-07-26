# v0.3 H6 — Resource garbage collection and retention

Status: completed 2026-07-26

Model: GPT-5.6

## Delivered

- Added schema v8 resource-lifecycle state:
  - `unreferenced_at` starts or restarts when a logical resource loses its
    final document reference;
  - `unreferenced_reason` distinguishes new unattached resources, explicit
    detach, permanent note purge, and legacy migration;
  - attaching a resource clears both fields, and references from Trash remain
    protective.
- Added configurable local retention windows:
  - `retention.unreferenced_resource_days` defaults to 30 days;
  - `retention.purged_resource_days` defaults to 90 days.
- Added `store.GarbageCollect` with a shared plan/apply report and
  `store.RetentionGate`. The v0.3 local gate uses elapsed retention; a future
  sync gate can additionally require peer acknowledgement watermarks.
- Added `notriosctl gc`, which is a dry run unless `--apply` is explicitly
  supplied. Conflicting `--dry-run --apply` flags are rejected.
- Added the read-only `GET /api/v1/admin/gc/report` REST route. There is no REST
  apply route.
- Required object-specific `X-Notrios-Confirmation` values for immediate
  resource deletion and permanent Trash purge.
- Updated status capabilities/limits, OpenAPI, config examples, API/schema/
  security/architecture/testing docs, user CLI/REST/service docs, feature
  matrix, plan status, context map, and coding handoff.

## Safety semantics

- Any logical resource with at least one document reference is absent from the
  candidate set, including resources referenced only by trashed notes.
- Planning evaluates retention first and then the supplied retention gate.
- Apply acquires a write transaction and rechecks the reference count,
  unreferenced timestamp/reason, and blob identity before deleting each
  candidate. A resource attached between planning and apply is retained.
- A physical blob and its derived perceptual hashes are removed only after its
  final logical resource is removed. Exact SHA-256 remains the sole blob
  identity.
- Filesystem deletion is constrained to relative paths under the configured
  asset root. Failures are reported as warnings rather than broadening the
  deletion target.
- Existing v7 unreferenced resources begin retention at their recorded
  creation time; referenced resources migrate with no unreferenced state.

## Validation

- `go vet ./...`
- `go test ./...`
- focused config, store GC/schema-upgrade, and HTTP confirmation/report tests
- real `notriosctl gc --dry-run` against a fresh SQLite database
- OpenAPI YAML parse and migration-copy equality
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- `cd web && npm run typecheck && npm run build && npm test` (37 tests)
- `bash scripts/build_docs_site.sh`
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`

Focused tests cover dry-run/apply behavior, retention boundaries, multi-note
references, Trash protection, longer post-purge retention, shared physical
blobs, future sync-gate denial, transaction-time reference rechecks, and v7→v8
backfill.
