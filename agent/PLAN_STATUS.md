# Plan Status

Updated: 2026-08-04

## Active milestone

v0.3 import/resource/media/large-library hardening is complete. H1–H11 are
archived under `plans/v0.3/`. The revised v0.4 plan begins with Joplin
correctness/performance prerequisites, then archive-v2, publishing handoff,
boolean search, and stable references. J1–J3, Q1, P1, P2, and P3 are complete;
P4 and all subsequent product tasks require user approval.

## 2026-08-02 follow-up review

- Compared the Go Joplin RAW importer with the real-data-tested
  `movenotes-v3/joplin2sql.py` and `notesdb.py` behavior.
- Found that Notrios' synthetic fixtures modeled `title:` metadata although
  canonical RAW stores titles on the first physical line; real imports could
  therefore use IDs as titles and retain duplicate title text in note bodies.
- Completed J1: CR/LF-only physical splitting, canonical title/body separation,
  unified ordered-property parsing, OCR control preservation, future/duplicate
  keys, delimiter whitespace, BOM support, and invalid-UTF-8 rejection.
- Completed J2: explicit per-type/malformed/unsupported inventory reporting,
  Markdown code-safe one-pass Joplin link rewriting, direct deduplicated
  resource relationships, unresolved-link counts, source-read-only CLI dry
  runs, and private-safe real-export evidence under `performance/v0.4-j2/`.
- The 1,237,553-item recipe dry run found 382,206 notes, exactly matching the
  corresponding Obsidian vault Markdown count. The 111,330-item attachment
  export planned 766 relationships over 763 resource records and reported five
  missing content files without silently dropping them.
- Completed J3: bounded canonical Store transactions with atomic checkpoints,
  an indexed temporary manifest/keyset spool, final cross-batch link pass,
  bounded notebook states, aggregate SQLite metrics, and private-safe complete
  interruption/resume/search/no-op evidence under `performance/v0.4-j3/`.
- The 1,237,553-item recipe profile imported 382,206 notes into matching
  document/revision/FTS/provenance rows and 842,813 tag relations. The full
  workflow completed in 58m11s; the 3.101 GB database was search-ready and a
  complete no-op added no revisions.
- Removed a correlated global tag count from the bounded document-membership
  query after a real 3 GB A/B measured 54.32s before and 0.42s after.
- Replaced duplicate Notrios Obsidian/Quartz/static-site work with a native
  archive-v2 handoff to a separately maintained `movenotes-v3/notrios2sql.py`
  importer. Movenotes owns Obsidian/Quartz and Hugo/Ledger+Bluge projections;
  Notrios owns selection/privacy/reachable-resource decisions.
- Completed Q1: one bounded AST for uppercase `OR`, implicit `AND`, prefix
  negation, grouping, phrases, fields, `category:` alias, and All-notes
  semantics; exact parameterized SQLite compilation, supported-shape Recoll
  compilation with explicit fallback, canonical cursor binding, recursive
  notebooks, and exact emoji handling.
- Live SQLite/Recoll parity covers grouped/negated fields, recursive category
  matching, authors, titles, and emoji. REST, MCP, search notebooks, GUI help,
  specifications, and generated scale tests all share the documented grammar.
- Completed P1: one read-only Store operation with typed recursive
  notebook/tag/query/explicit-ID selectors and reusable full-archive,
  subset-transfer, and publication-handoff privacy defaults. REST and MCP call
  the same planner with capped content-free details.
- P1 reports complete selected/excluded counts, exact reachable resources,
  internal/private/broken/external link decisions, hashed source-bundle keys,
  metadata preserve/strip decisions, warnings, and a deterministic manifest
  SHA-256 without returning bodies, bytes, source metadata JSON, or paths.
- Completed P2: schema-v12 persisted logical database and writable-copy replica
  identities; bootstrap/reopen stability and explicit replica rotation.
- Added the separate `internal/archivev2` manifest/typed-record contract,
  canonical commit digest, schema/capability negotiation, explicit
  replace/merge/fork/adopt identity decisions, and strict read-only directory
  verification before canonical writes.
- The complete synthetic golden archive covers all twelve record types and exact
  body/resource/source-bundle objects. Adversarial mutations reject absent or
  corrupt objects, traversal/symlinks/extras, unsupported compatibility,
  invalid MIME/counts/references/JSON, and bound overflows.

- H1: media-policy configuration and schema v7.
- H2: static remote-media scan, policy API/MCP, GUI decisions.
- H3: quarantine fetch with redirect/connect-time SSRF controls, size/MIME/hash
  checks, and attempt records.
- H4: shared localization engine exposed through REST/CLI/MCP/GUI/importer flag.
- H5: exact-hash/unreferenced/per-notebook reports through Store, REST, and
  CLI; pluggable perceptual admission/policy/report hook, inert by default.
- H6: schema-v8 retention state, dry-run-first local GC, read-only REST report,
  explicit permanent-delete confirmation, and a future sync-aware gate.
- H7: schema-v9 keyset indexes, query/sort-bound `k2` cursors, bounded stable
  merged-sidecar snapshots, live GET search, and reproducible
  10k/100k/500k performance profiles.
- H8: schema-v10 importer checkpoints/item fingerprints/exact source-bundle
  manifests; Joplin nested notebooks, stable real tags, bounded phases,
  dry-run/config parity, resource refresh, resume, and 100/10k/100k fixtures.
- H9: Obsidian nested vault hierarchy, conflict/config planning, exact
  Markdown/frontmatter/non-Markdown source bundles, bounded
  batches/fingerprints/checkpoints, canonical alias/relative/embed/anchor
  resolution, stable resource refresh, and 100/10k/100k/500k fixtures.
- H10: schema-v11 Recoll retry/backoff, exact reconciliation, hardened
  processes, stable attributed merges, status/UI, and 100k native evidence.
- H11: v0.3 documentation/release wrap-up, Help reseed, version 0.3.0,
  release-candidate validation, and verified packaging.

## 2026-08-04 session (Claude Code, Opus 5)

- Completed P3: `notriosctl export archive-v2` streams one SQLite
  read-transaction snapshot through the P1 planner into immutable SHA-256
  objects and publishes `manifest.json` last.
- Added a store-internal `ExportReader` contract (`internal/store/export.go`,
  `sqlite_export.go`) with bounded identity-scoped reads, visitor-streamed
  revisions/links, and blob/source-bundle content streams. It is deliberately
  separate from `Store` so no REST or MCP adapter can reach complete identity
  sets, raw source-bundle keys, or blob content.
- `ResolveSelection` reuses `planSelectionLocked`, so the archive binds exactly
  the `manifest_sha256` a dry-run `PlanSelection` reports for the same request.
- full_archive keeps trashed notes, complete revision history, provenance with
  private `metadata_json`, source bundles, all notebooks/tags, and builtin
  search notebooks. subset_transfer requires a selector, blanks private source
  metadata, exports only reachable notebooks plus ancestors and used tags, and
  omits whole-library search notebooks. Links to excluded targets are recorded
  as `target_excluded` with the target cleared, never as dangling references.
- publication_handoff and the `plain_text`/`redact` link actions are refused:
  both rewrite note content and belong to P7.
- Interruption semantics: objects and the manifest are staged in a sibling
  `<destination>.staging` directory and renamed in; an interrupted run leaves no
  manifest, a resumed run reuses published objects and prunes unlisted ones, and
  an archive that fails its own post-publication verification has its manifest
  removed.
- Generated 100/1,000/5,000-note evidence is under `performance/v0.4-p3/`:
  5,000 notes exported in 3.210 s, verified in 5.130 s, resumed with zero bytes
  rewritten, and used 46 MiB whole-process peak RSS.
- Recorded an open format bound rather than widening limits silently: one object
  per revision plus an inline manifest inventory caps an archive near 9,900
  revisions, so archive v2 cannot yet back up the million-note libraries J3
  imports. See `agent/OPEN_QUESTIONS.md` question 17.

## 2026-07-26 plan/roadmap review

The review initiated by `/home/renes/prompts/notrios_codex_reviewplan.md`
updated the planning model without implementing future product features:

- confirmed the current `q1` cursor is offset-backed, SQL uses OFFSET, and
  navigation has a 100,000-offset ceiling; H7 now fixes this before importer
  scale hardening;
- added `SYNCHRONIZATION.md` for v0.7 identities, operation IDs/HLC/ack vectors,
  merge rules, conflicts, immutable archive/envelope objects, REST and
  folder/rclone transports, GC/retention, and convergence validation;
- made native archive v2 (v0.4) the full-snapshot/container layer reused by
  sync, while documenting archive v1 as non-lossless interchange;
- added scalable publication planning and a Bluge/Recoll/FTS adapter spike;
- kept Marmot, Cachapa, Ygo, Nostr, and bitchat as bounded design references,
  not dependencies; REST plus immutable rclone/folder transport comes first;
- documented Wails v3/mobile as a later migration spike because v3/mobile are
  currently pre-release/experimental;
- reconciled stale API/security/feature/docs claims after H4.

See the archived review under `plans/v0.3/000-plan-roadmap-review-2026-07.md`
when this session completes.

## Completed milestones

- v0.1 MVP is archived under `plans/mvp/`.
- v0.2 redesign R1–R16 is complete under `plans/v0.2/`.
- v0.3 H1–H11 are under `plans/v0.3/001`–`011`.
- GUI conformance/native-resize verification and the scaffold/MVP report
  archive moves are committed before the current review.

The authoritative capability summary is `CODING_CLIENT_HANDOFF.md`; historical
attempt detail remains append-only in `agent/ATTEMPT_LOG.jsonl`.

## Current working-state facts

- Branch: `develop`.
- Review base before this session: `26b0925`.
- Project license: Apache-2.0.
- Canonical store: SQLite plus content-addressed assets; FTS5/Recoll are derived.
- Current schema: v12.
- Unbounded local traversal uses `(updated_at, id)` or `(score, id)` keysets;
  notebook and Trash pages are route-bound. Optional Recoll merge pages use a
  ten-minute, 1,000-hit immutable snapshot and report truncation explicitly.
- H7 generated profile evidence is under `performance/v0.3-h7/`.
- H8 generated Joplin dry-run/interruption/resume evidence is under
  `performance/v0.3-h8/`.
- H9 generated Obsidian dry-run/interruption/resume and scale evidence is under
  `performance/v0.3-h9/`.
- H10 native Recoll 100k index/query/drift/reconciliation evidence is under
  `performance/v0.3-h10/`.
- J2 private-safe real-export planner evidence is under
  `performance/v0.4-j2/`.
- J3 synthetic and private-safe full transactional evidence is under
  `performance/v0.4-j3/`.
- Q1 generated boolean/category scale evidence is under
  `performance/v0.4-q1/`; ordinary first/next/deep pages remain below the
  recorded 100 ms target, while the nonselective negation cost is reported
  separately.
- P1 generated 100k selection evidence is under `performance/v0.4-p1/`: the
  full plan covered 100,000 documents, 1,000 resources, and 200,000 links in
  16.239 seconds; whole-process peak RSS was 136 MiB.
- P2 synthetic golden/adversarial evidence lives with the verifier under
  `internal/archivev2/testdata/`; no content-bearing private corpus is needed
  for the format-contract slice.
- H11 local release gates pass; `docs/operations.md` is included in the
  docs site; P1/P2 expand it from 11 to 13 pages/Help notes with selection and
  archive-v2 safety guides.
- Product version: 0.3.0; current schema: v12.
- Resource reference report:
  `GET /api/v1/resources/reports/reference` and
  `notriosctl resources report`.
- Resource GC: `GET /api/v1/admin/gc/report` is read-only;
  `notriosctl gc` defaults to dry run and requires `--apply` to remove
  retention-expired, currently unreferenced resources. The local retention
  gate is replaceable with future peer-acknowledgement policy.
- Perceptual hashing: hook contract is wired, but no algorithm ships or is
  installed by default; all output is review-only.
- Fresh database builtins: All notes, Notes, Help, Trash.
- MCP default profile is read-only; editor-profile write tools are implemented.
- Native archive v1 is query-scoped interchange, not full backup.
- Native archive v2 streaming export is live as `notriosctl export archive-v2`;
  it is a local CLI operation with no REST/MCP output-path surface, and restore
  is still P4, so a v2 archive cannot yet be read back.
- P3 generated 100/1,000/5,000-note export evidence is under
  `performance/v0.4-p3/`. One archive currently holds at most ~9,900 revisions
  because each revision is one object and the manifest lists objects inline.
- No GitHub push is authorized for this review.

## Open implementation blockers

- Long-term SQLite driver choice (current local cgo/libsqlite3 adapter).
- Official MCP Go SDK adoption/version.
- Sync decisions listed in `SYNCHRONIZATION.md` and
  `agent/OPEN_QUESTIONS.md`; they do not block v0.4 P3.
