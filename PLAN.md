# Plan: v0.4 — Portable data, publishing, and stable references

Status: **proposed; implementation has not started and requires user approval**.
Drafted 2026-07-26 from `ROADMAP.md` after completing the archived v0.3 H1–H11
plan under `plans/v0.3/`.

## Goal

Build one safe, scalable selection and object pipeline that can produce:

1. a checksum-verified native archive v2 suitable for full backup/transfer and
   later reuse by v0.7 synchronization;
2. an interoperable portable Markdown vault;
3. privacy-reviewed curated Quartz sites and a measured large-library static
   publication target;
4. stable `notrios://` links that include logical database identity and resolve
   without guessing between local profiles.

Native archive v1 remains supported as query-scoped interchange. v0.4 does not
implement record-level synchronization, mobile clients, or the v0.6 bulk/MCP
organizer.

## Working-state rule

Complete one task at a time. Every task updates tests and living docs, runs its
relevant validation, records attempt/model status, commits a working slice,
archives it under `plans/v0.4/`, produces a verified ZIP, and asks for approval
before the next task.

## Tasks

### P1. Shared selection and privacy planner

- Define typed selection inputs for notebooks (recursive), tags, queries, and
  explicit bounded document IDs.
- Produce a deterministic manifest of selected notes, reachable resources,
  internal/private/broken links, source bundles, and exclusion reasons.
- Keep planning read-only and stream/batch canonical reads; no arbitrary SQL
  or filesystem paths cross REST/MCP boundaries.
- Specify reusable privacy policy and report types for archive, vault, Quartz,
  and large-site targets.

Working state: a dry-run planner returns deterministic bounded reports on
small fixtures and generated 100k-note data without materializing all note
bodies in memory.

### P2. Native archive v2 format and identity contract

- Specify a versioned manifest, logical database identity, snapshot metadata,
  capability/schema bounds, immutable SHA-256 objects, and manifest-last
  completeness rules.
- Cover notes, revisions, notebooks, tags, links, provenance, resources, and
  optional source bundles without including derived FTS5/Recoll state.
- Define replace/merge/fork/adopt restore intent; no silent database-universe
  merge.
- Add strict size/count/path/depth limits, MIME handling, and checksum
  validation fixtures.

Working state: format/golden fixtures reject corruption, traversal, unsupported
versions, missing objects, and inconsistent manifests before canonical writes.

### P3. Native archive v2 streaming export

- Export one transactionally consistent snapshot through the P1 planner into
  immutable object files and publish the manifest last.
- Stream bodies/resources/source bundles with bounded buffers; reuse exact
  content objects and report bytes/counts/warnings.
- Support full-database backup as the primary mode and explicit subset transfer
  without representing a subset as a full backup.
- Add interruption cleanup/resume or atomic staging semantics.

Working state: full and subset exports are deterministic, checksum-valid,
bounded-memory, and leave no apparently complete archive after interruption.

### P4. Native archive v2 verify and restore/import

- Add verify-only CLI/API service behavior before any mutation.
- Implement explicit replacement restore into a fresh initialized database and
  additive import with conflict reporting; preserve or mint logical database
  identity according to the chosen intent.
- Admit objects through safe bounded paths and atomic canonical transactions;
  preserve revisions/provenance/source bundles.
- Add crash/fault injection and round-trip equality tests.

Working state: a v2 archive can reconstruct the promised canonical state,
corruption causes no partial restore, and archive v1 compatibility remains.

### P5. Portable Markdown vault export

- Export stable relative paths, nested folders, front matter, resources, and
  generated link reference definitions through the shared planner.
- Rewrite document/resource links portably and report links that cannot be
  exported without revealing private targets.
- Use deterministic collision naming and a dry-run path manifest.
- Test generated large vaults without claiming that Obsidian itself can
  interactively handle the largest tier.

Working state: exported vaults are readable without Notrios, deterministic,
and preserve navigable links/resources within the selected subset.

### P6. Stable external links and local resolution

- Define `notrios://databases/{database_id}/documents/{document_id}` parsing,
  validation, length bounds, and stale-target errors.
- Add the minimum local profile/database registry needed to resolve an external
  link; prompt on ambiguity and never guess or switch by filesystem input.
- Implement OS-handler registration only after route behavior is covered by
  portable tests.
- Keep authorization/network behavior unchanged; this is local routing, not
  sync.

Working state: stable links survive local path/profile changes and malformed or
wrong-database links cannot open another profile silently.

### P7. Publishing privacy plan and Quartz profile

- Add saved publish profiles using the shared selection/privacy planner.
- Support recursive notebook/folder/tag selection, reachable public resources,
  private-note link policy, metadata stripping, and dry-run warnings.
- Generate a Quartz-compatible curated subset without invoking untrusted note
  content as build code.
- Keep publish execution explicit after a reviewed plan.

Working state: curated fixtures publish only selected reachable public content;
privacy violations are visible before generation.

### P8. Large-library search adapter spike

- Measure Bluge, an external Recoll service, and a simpler FTS-backed option
  for build time, index size, query/highlight/facet behavior, deep paging,
  licensing, maintenance, and deployment.
- Use generated 10k/100k/500k publication fixtures and bounded result APIs.
- Do not adopt or vendor a library until the evidence and maintenance owner are
  recorded.

Working state: an archived decision report selects an adapter or documents why
the milestone should retain a simpler option; no speculative dependency
remains.

### P9. Scalable archive-site publisher

- Stream page/resource generation through the shared planner.
- Use fixed/bounded navigation rather than rendering a full library tree or
  client-side index; integrate the P8 selected search adapter behind a stable
  interface.
- Add resumable/atomic output, privacy checks, accessibility, and generated
  100k/500k evidence.

Working state: the generated site has bounded build/runtime behavior and no
private or unreachable resources leak from the selected set.

### P10. v0.4 documentation and release wrap-up

- Document backup/verify/restore intent, portable vaults, stable links,
  publishing privacy, and large-site operations in both the site and Help
  notebook.
- Reconcile `FEATURE_MATRIX.md`, architecture/schema/API/security documents,
  and release checklist.
- Run the full release validation, archive the plan, draft the next plan from
  `ROADMAP.md`, and produce a verified source ZIP.

Working state: documentation matches implementation and v0.4 release checks
pass.

## Baseline validation

```bash
go vet ./...
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build && npm test
bash scripts/build_docs_site.sh
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

Archive, publisher, GUI, and large-library tasks add their specific
round-trip, hostile-input, native-build, and scale gates.

## Decisions required before or during v0.4

- Approve P1 before implementation.
- Select exact archive-v2 restore defaults only after P2 presents explicit
  replace/merge/fork/adopt behavior.
- Select the large-site search adapter only from P8 measurements; Bluge is not
  preselected.
- OS registration details remain Ubuntu-only unless another platform is
  explicitly added and tested.

## Scope control

CodeMirror/block graph UX (v0.5), expanded MCP/batch organizer profiles (v0.6),
record-level sync and rclone transport (v0.7), authentication/multi-user
deployment, Wails v3/mobile migration, HTTP range downloads, and the official
MCP Go SDK migration remain outside v0.4 unless the roadmap is deliberately
revised.
