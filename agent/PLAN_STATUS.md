# Plan Status

Updated: 2026-08-03

## Active milestone

v0.3 import/resource/media/large-library hardening is complete. H1–H11 are
archived under `plans/v0.3/`. The revised v0.4 plan begins with Joplin
correctness/performance prerequisites, then archive-v2, publishing handoff,
boolean search, and stable references. J1–J3 and Q1 are complete; P1 and all
subsequent product tasks require user approval.

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
- Current schema: v11.
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
- H11 local release gates pass; `docs/operations.md` is included in the
  11-page docs site and deterministic 11-note Help seed/reseed.
- Product version: 0.3.0; current schema: v11.
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
- No GitHub push is authorized for this review.

## Open implementation blockers

- Long-term SQLite driver choice (current local cgo/libsqlite3 adapter).
- Official MCP Go SDK adoption/version.
- Sync decisions listed in `SYNCHRONIZATION.md` and
  `agent/OPEN_QUESTIONS.md`; they do not block v0.4 Q1 or P1.
