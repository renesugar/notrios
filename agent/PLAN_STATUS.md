# Plan Status

Updated: 2026-07-26

## Active milestone

`PLAN.md` is v0.3 import/resource/media/large-library hardening, now H1–H11.
H1–H7 are complete and archived under `plans/v0.3/`; H8 is the next
implementation task and still requires the normal user approval before coding.

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
- H8/H9: Joplin RAW/Obsidian hierarchy, exact source preservation, bounded
  batches/fingerprints/checkpoints, and large fixtures.
- H10: Recoll reconciliation, bounded batches, stable merge quality, UI status.
- H11: v0.3 documentation/release wrap-up.

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
- v0.3 H1–H7 are under `plans/v0.3/001`–`007`.
- GUI conformance/native-resize verification and the scaffold/MVP report
  archive moves are committed before the current review.

The authoritative capability summary is `CODING_CLIENT_HANDOFF.md`; historical
attempt detail remains append-only in `agent/ATTEMPT_LOG.jsonl`.

## Current working-state facts

- Branch: `develop`.
- Review base before this session: `26b0925`.
- Project license: Apache-2.0.
- Canonical store: SQLite plus content-addressed assets; FTS5/Recoll are derived.
- Current schema: v9.
- Unbounded local traversal uses `(updated_at, id)` or `(score, id)` keysets;
  notebook and Trash pages are route-bound. Optional Recoll merge pages use a
  ten-minute, 1,000-hit immutable snapshot and report truncation explicitly.
- H7 generated profile evidence is under `performance/v0.3-h7/`.
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
- Real private Joplin/Obsidian corpus verification (never commit private data).
- Sync decisions listed in `SYNCHRONIZATION.md` and
  `agent/OPEN_QUESTIONS.md`; they do not block H8.
