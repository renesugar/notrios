# v0.4 P1 — Shared selection and privacy planner

Status: complete (2026-08-03)

Model: GPT-5

## Scope

Define one read-only, deterministic, bounded selection/privacy planner reused
by future native archive v2, subset transfer, and publication handoff. Support
recursive notebooks, tags, canonical Q1 queries, and explicit document IDs;
report selected notes, reachable resources, link boundaries, source bundles,
metadata decisions, exclusions, and target policy without exposing SQL,
filesystem paths, or note/resource content.

## Implementation

- Added `PlanSelection` to the canonical Store contract with typed closed
  selectors and policy overrides. Selector types combine with explicit
  `any`/`all`; publication/subset plans reject an empty scope.
- Added target defaults for `full_archive`, `subset_transfer`, and
  `publication_handoff`. Publication excludes `private`, `draft`, and
  `confidential`, omits provenance/source bundles/private metadata, excludes
  Trash, and plans private/broken links as plain text unless explicitly
  overridden.
- Traversed identity/revision state, tags, resources, links, provenance, and
  source-bundle metadata in 400-ID batches under a consistent Store read lock.
  Planner SQL never selects note bodies for output and never opens resource or
  source-bundle content.
- Returned capped content-free document/resource/link/bundle/exclusion details
  plus complete counts, warnings, metadata decisions, and a SHA-256 digest over
  the complete normalized manifest. Link fingerprints bind raw link identity
  without exposing raw targets/context.
- During privacy review, confirmed importer source keys can be absolute paths.
  Planner output therefore exposes only SHA-256 fingerprints of source and
  item keys and always strips storage paths/source metadata JSON/URLs.
- Added `POST /api/v1/selection/plan`, read-only MCP `plan_selection`, status
  capabilities/limits, closed JSON schemas, REST/MCP parity and output-size
  tests, OpenAPI, contributor/user documentation, and a 12th Help/docs page.
- Replaced the obsolete scaffold Quartz-plan stub; Notrios owns the neutral
  plan while external `movenotes-v3` owns Quartz/Hugo projections.

## Limits

- 100 notebook/tag/policy selectors; 1,000 explicit IDs; 512 bytes/value.
- Q1 query bounds remain 4,096 bytes, 256 tokens, and 16 levels.
- Store planner: at most 1,000,000 selected notes.
- REST/MCP: at most 100,000 selected notes.
- REST details: at most 1,000 per array; MCP details follow configured result
  limit (default 10, maximum 50).

## Validation

- `go vet ./...`
- `go test ./...`, including live Recoll and loopback media integration
- focused deterministic/read-only/privacy/link/resource/source-bundle fixtures
- REST/MCP parity, schema, unsafe-scope, limit, and output-size tests
- `python3 scripts/check_required_files.py`
- `bash scripts/validate-scaffold.sh`
- OpenAPI YAML and evidence JSON parsing
- `cd web && npm ci && npm run typecheck && npm test -- --run && npm run build`
  (5 files, 39 tests)
- `bash scripts/build_docs_site.sh` (12 pages)
- `bash scripts/mvp_smoke.sh`
- `bash scripts/run_performance_smoke.sh`
- `bash scripts/run_large_library_profile.sh 100000 /tmp/notrios-v0.4-p1-100k.json`
- `git diff --check`

The generated full-archive plan covered 100,000 selected documents, 1,000
reachable resources, and 200,000 internal links while retaining ten visible
details. It completed in 16.239 seconds; whole-process peak RSS for dataset
seed, search, cursors, and planning was 136 MiB. Evidence is under
`performance/v0.4-p1/`.

## Next task

P2, the native archive-v2 format and identity contract, remains unstarted and
requires explicit user approval.
