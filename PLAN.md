# Plan: v0.4 — Import correctness, portable data, publishing handoff, and stable references

Status: **J1–J3, Q1, P1, P2, and P3 completed; P4 and all remaining product-feature tasks require user approval**.
Drafted 2026-07-26 and revised 2026-08-02 after comparing the Joplin importer
and publishing/search plans with the real-data-tested `movenotes-v3` pipeline.

## Goal

Build a correct large-library foundation and one safe, scalable selection and
object pipeline that can provide:

1. faithful, resumable Joplin RAW import against real exports at million-note
   scale;
2. a checksum-verified native archive v2 suitable for full backup/transfer and
   later reuse by v0.7 synchronization;
3. privacy-reviewed subset handoffs to `movenotes-v3`, which owns Obsidian,
   Quartz, Hugo/Ledger, Pagefind, and Bluge publication projections;
4. stable `notrios://` links that include logical database identity and resolve
   without guessing between local profiles.
5. Twitter/X-style boolean search (`OR`, implicit `AND`, `-negation`, grouping,
   and phrases) plus `category:` as a `notebook:` alias.

Native archive v1 remains supported as query-scoped interchange. v0.4 does not
implement record-level synchronization, mobile clients, or the v0.6 bulk/MCP
organizer.

## Review decisions (2026-08-02)

- Notrios will not duplicate `movenotes-v3`'s portable-vault, Quartz, Hugo/Ledger,
  Pagefind, or Bluge implementations. The integration boundary is native archive
  v2 plus compatibility fixtures and a separate `notrios2sql.py` importer owned
  by the MIT-licensed `movenotes-v3` repository.
- Notrios remains responsible for canonical selection, privacy decisions,
  reachable-resource analysis, metadata stripping, and a reviewed publication
  handoff. `movenotes-v3` owns format conversion and site building after that
  boundary.
- Quartz remains available for curated smaller subsets through
  `movenotes-v3`'s Obsidian output. Hugo with `hugo-theme-ledger` and Bluge is
  the already measured large-library route; v0.4 no longer contains a redundant
  search-adapter spike or Notrios-native static-site generator.
- Native archive v2 remains the restore-fidelity format. A scoped publication
  handoff is explicitly not represented as a full backup.

## Working-state rule

Complete one task at a time. Every task updates tests and living docs, runs its
relevant validation, records attempt/model status, commits a working slice,
archives it under `plans/v0.4/`, produces a verified ZIP, and asks for approval
before the next task.

## Tasks

### J1. Joplin RAW physical-line parser and canonical title/body compatibility — complete

- Parse metadata only on CR/LF physical lines so OCR control characters remain
  inside `ocr_text` values.
- Derive canonical Joplin item titles from the first source line and keep the
  title out of a note's Markdown body; retain the legacy metadata-first parser
  only as an explicit compatibility path.
- Preserve property order (including duplicate and future keys) from the same
  parse used for import, consume at most one delimiter space, accept UTF-8 BOMs,
  and reject invalid UTF-8.
- Cover CRLF, OCR controls, duplicate keys, future key spelling, whitespace,
  exact source bundles, canonical resource/folder/tag titles, and idempotence.

Working state: focused importer tests and canonical `movenotes-v3/sample`
fixtures pass; implementation evidence is archived under `plans/v0.4/001-*`.

### J2. Real-export correctness and bounded relationship planning — complete

- Validate read-only imports against `recipe_joplin` and the attachment-bearing
  Joplin archive; compare aggregate recipe results with `recipe_vault` without
  committing note data, paths, titles, databases, or resources.
- Inventory every supported Joplin item type and report unsupported/malformed
  items instead of silently dropping them; add sanitized fixtures for each
  newly discovered shape.
- Extract referenced `:/id` targets once per note instead of scanning every
  known resource for every note, while retaining unresolved-link reports and
  code-span/fence safety.
- Record elapsed time, peak RSS, item/resource counts, warnings, idempotence,
  and search/link/resource checks. Dry run must never write into the source.

Working state: aggregate results agree with the source formats, attachment
relationships are proportional to actual links, and real-data evidence contains
no private content. Evidence is archived under `performance/v0.4-j2/` and
implementation detail under `plans/v0.4/002-*`.

### J3. Million-note transactional import throughput — complete

- Replace per-document canonical transactions with a bounded store batch API
  that preserves revisions, FTS5, links, provenance, tags, resources, outbox,
  and checkpoint atomicity.
- Replace whole-inventory maps where measured memory requires it with a
  temporary indexed manifest/spool; keep deterministic fingerprints and resume.
- Profile complete imports, interruption/resume, no-op re-import, and search
  readiness on the recipe corpus; record hardware, SQLite settings, database
  size, elapsed distribution, throughput, and peak RSS.
- Keep synthetic 100/10k/100k regression tiers, but do not use dry-run-only
  evidence as a claim about full import throughput.

Working state: a complete million-note import is resumable, bounded by the
chosen batch/spool sizes, and has an evidence-backed throughput baseline.
Aggregate-only evidence is archived under `performance/v0.4-j3/` and
implementation detail under `plans/v0.4/003-*`.

### Q1. Boolean search expressions and category alias — complete

- Replace the flat parser with a bounded expression tree for uppercase `OR`,
  implicit `AND`, prefix `-`, parentheses, and quoted phrases; `AND` binds more
  tightly than `OR`, URLs and hyphenated words remain intact, and unknown
  `word:value` tokens remain searchable text.
- Compile the same tree to SQLite FTS5/SQL and Recoll, with backend parity tests,
  maximum query length/token/depth limits, and query-bound cursor fingerprints.
- Treat `category:` as an exact alias for `notebook:`. `category:"All notes"`
  and `notebook:"All notes"` remove the notebook filter and search all current
  notes; ordinary notebook matching remains recursive and case-insensitive.
- Update GUI, REST, MCP, search-notebook, docs, and generated scale tests; never
  silently approximate an operator on a backend that cannot honor it.

Working state: the documented grammar has identical result sets through live
SQLite and Recoll fixtures, including negated/grouped fields and emoji terms.
Generated scale evidence is archived under `performance/v0.4-q1/` and
implementation detail under `plans/v0.4/004-*`.

### P1. Shared selection and privacy planner — complete

- Define typed selection inputs for notebooks (recursive), tags, queries, and
  explicit bounded document IDs.
- Produce a deterministic manifest of selected notes, reachable resources,
  internal/private/broken links, source bundles, and exclusion reasons.
- Keep planning read-only and stream/batch canonical reads; no arbitrary SQL
  or filesystem paths cross REST/MCP boundaries.
- Specify reusable privacy policy and report types for full archive, subset
  transfer, and publication-handoff targets.

Working state: a dry-run planner returns deterministic bounded reports on
small fixtures and generated 100k-note data without materializing all note
bodies in memory.
Generated evidence is archived under `performance/v0.4-p1/` and implementation
detail under `plans/v0.4/005-*`.

### P2. Native archive v2 format and identity contract — complete

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
Schema v12 persists stable logical database and per-writable-copy replica IDs;
the verifier also rejects MIME/path/depth/count/reference violations.
Implementation detail is archived under `plans/v0.4/006-*`.

### P3. Native archive v2 streaming export — complete

- Export one transactionally consistent snapshot through the P1 planner into
  immutable object files and publish the manifest last.
- Stream bodies/resources/source bundles with bounded buffers; reuse exact
  content objects and report bytes/counts/warnings.
- Support full-database backup as the primary mode and explicit subset transfer
  without representing a subset as a full backup.
- Add interruption cleanup/resume or atomic staging semantics.

Working state: full and subset exports are deterministic, checksum-valid,
bounded-memory, and leave no apparently complete archive after interruption.
`notriosctl export archive-v2` reads through one SQLite read transaction,
streams every body/resource/source bundle through a 64 KiB buffer, stages
privately, publishes the manifest last, and verifies the result. Generated
100/1,000/5,000-note evidence is archived under `performance/v0.4-p3/` and
implementation detail under `plans/v0.4/007-*`.

One format bound is deliberately left open rather than widened in this slice:
one object per revision plus an inline manifest inventory caps an archive near
9,900 revisions, so archive v2 cannot yet back up a million-note library. The
exporter enforces the documented limits and fails before publishing a manifest.
See `NATIVE_ARCHIVE_V2.md` and `agent/OPEN_QUESTIONS.md`.

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

### P5. Stable external links and local resolution

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

### P6. Native archive compatibility bridge to movenotes-v3

- Publish archive-v2 JSON Schemas/golden fixtures, capability bounds, and a
  compatibility command that produces sanitized deterministic test archives.
- Coordinate the separate `movenotes-v3/notrios2sql.py` importer against those
  fixtures. Do not copy implementation code between repositories; exchange only
  documented formats, behavior, and fixtures.
- Verify revisions/provenance/source bundles needed for backup remain available
  to the importer while publication mode exposes only the selected current-note
  projection and explicitly allowed metadata.
- Add cross-version consumer tests so Notrios format additions are rejected or
  ignored according to declared capability rules.

Working state: `movenotes-v3` can consume a Notrios archive without a Notrios
Obsidian/Joplin exporter, and an unsupported archive fails before partial import.

### P7. Publication profiles and privacy-reviewed archive handoff

- Add saved publish profiles using the shared selection/privacy planner.
- Support recursive notebook/folder/tag selection, reachable public resources,
  private-note link policy, metadata stripping, and dry-run warnings.
- Emit a scoped, sanitized archive-v2 publication handoff; do not invoke
  untrusted note content as build code inside Notrios.
- Keep publish execution explicit after a reviewed plan.

Working state: curated fixtures hand off only selected reachable public content;
privacy violations are visible before generation, and `movenotes-v3` can turn
the handoff into either a Quartz vault or a Hugo/Ledger site.

### P8. v0.4 documentation and release wrap-up

- Document backup/verify/restore intent, the movenotes compatibility boundary,
  stable links, publishing privacy, and Quartz versus Hugo/Ledger operations in
  both the site and Help notebook.
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

Importer, archive, compatibility, GUI, and large-library tasks add their
specific real-format, round-trip, hostile-input, native-build, and scale gates.

## Decisions required before or during v0.4

- Approve P4 before implementing verified archive-v2 restore/import.
- Decide whether archive v2 gains an out-of-manifest object inventory so a
  full backup can exceed the current ~9,900-revision object bound.
- Restore has no default: P2 defines explicit replace/merge/fork/adopt identity
  consequences; P4 must require one after verification.
- Treat `movenotes-v3` and `hugo-theme-ledger` as optional external publishing
  tools under their own repositories and licenses; do not link or vendor them.
- OS registration details remain Ubuntu-only unless another platform is
  explicitly added and tested.

## Scope control

CodeMirror/block graph UX (v0.5), expanded MCP/batch organizer profiles (v0.6),
record-level sync and rclone transport (v0.7), authentication/multi-user
deployment, Wails v3/mobile migration, HTTP range downloads, and the official
MCP Go SDK migration remain outside v0.4 unless the roadmap is deliberately
revised.
