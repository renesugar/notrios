# v0.3 H5 — Exact-hash reports and perceptual-hash hooks

Status: completed 2026-07-26

Model: GPT-5.6

## Delivered

- Added `store.ResourceReport` with:
  - exact SHA-256 duplicate groups across logical resources and collections;
  - physical blobs with no document-resource reference;
  - direct per-notebook usage for current notes, including reference/resource/
    unique-blob counts and referenced/unique byte totals;
  - perceptual hook state, matching review rules, and validated near-duplicate
    suggestions.
- Exposed the same read-only report through:
  - `GET /api/v1/resources/reports/reference`;
  - `notriosctl resources report`.
- Added `store.PerceptualHashHook` with algorithm identity, MIME support,
  compute, and near-duplicate suggestion slots.
- Wired optional hash computation into every `CreateResource` admission,
  storing one canonical value per exact blob/algorithm in `resource_hashes`.
  Exact duplicate logical resources reuse the stored value.
- Wired the admission-time perceptual policy lookup. Perceptual policy is
  review-only; `AddMediaHashRule` rejects perceptual `block` rules.
- Removed stored perceptual hashes when the final logical resource removes its
  physical blob.
- Updated the OpenAPI contract, API/schema/security/architecture/testing
  documentation, user CLI/REST/service docs, feature matrix, plan status, and
  coding handoff.

No perceptual algorithm or new dependency was added. The default service is
inert (`hook_enabled: false`) and behaves exactly as before for admissions.
Exact SHA-256 remains the only deduplication identity.

## Safety semantics

- A blob is unreferenced only when none of its logical resources has a
  document-resource reference.
- References from trashed notes remain protective even though trashed notes do
  not contribute to live per-notebook usage.
- Near-duplicate pairs must refer to blobs supplied to the hook, have a finite
  non-negative distance, and are canonicalized/deduplicated before reporting.
- Perceptual output cannot reject, merge, rewrite, hide, delete, or
  garbage-collect content. H6 owns retention and deletion.

## Validation

- `go vet ./...`
- `go test ./...`
- focused store/HTTP H5 tests
- real `notriosctl resources report` against a fresh SQLite database
- OpenAPI YAML parse
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- `cd web && npm run typecheck && npm run build && npm test` (37 tests)
- `bash scripts/build_docs_site.sh`
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`

Focused tests cover cross-collection exact duplicates, unreferenced blobs,
live notebook accounting, protective trashed-note references, inert defaults,
hash reuse per exact blob, review-rule reporting, and the prohibition on
perceptual blocking/deduplication.
