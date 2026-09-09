# Plan and Roadmap Review — 2026-07

Status: completed documentation/planning slice. No future feature described
here was implemented by this review.

## Scope

Reviewed the active plan, roadmap, architecture, API/schema, user docs,
historical plans/reports/prompts/skills, attempt/model logs, and current code
paths requested by `/home/renes/prompts/notrios_codex_reviewplan.md`. The review
focused on consistency, shared implementations, large-library performance,
publication/export, backup/restore, profiles, bulk operations, and v0.7
synchronization.

## Repository findings

1. `internal/store/sqlite_query.go` encoded `q1:<offset>:<checksum>`, used
   `LIMIT ... OFFSET`, and refused offsets above 100,000. The public docs called
   these cursors opaque/cursor-based without disclosing the linear offset or
   hard traversal ceiling.
2. All Notes sorts by `(updated_at DESC, id DESC)`, but the migration did not
   contain the claimed matching documents composite index.
3. Recoll-only hits were merged only on the first page, so paging semantics
   changed after the first response.
4. API and REST user docs still described implemented H4 localization as a
   stub; the MCP planned list also duplicated the implemented tool.
5. `FEATURE_MATRIX.md`, `CONTEXT_MAP.md`, `SECURITY_REVIEW.md`, CLI schema
   example, and project-decision/license text retained v0.1/v0.2-era status.
6. Native archive v1 was sometimes described as lossless although re-import
   creates plain notes and omits revision/provenance/database identity.
7. Fresh database bootstrap already creates the intended four entries: All
   notes and Trash search notebooks, Notes and read-only Help regular
   notebooks. This needed an explicit contract/test requirement rather than a
   redesign.

## Pagination evidence and decision

SQLite's official row-value documentation states that OFFSET work is
proportional to the offset because SQLite computes `LIMIT x+y` and discards the
first `y` rows:

- <https://www.sqlite.org/rowvalue.html#scrolling_window_queries>

A local probe created 1,000,000 rows with a covering
`(collection_id, deleted_at, updated_at DESC, id DESC)` index. One warm-ish
single-run sample measured:

| Offset/result depth | OFFSET elapsed | Keyset elapsed |
|---:|---:|---:|
| 0 | <0.01 s | <0.01 s |
| 10,000 | 0.01 s | <0.01 s |
| 50,000 | 0.02 s | <0.01 s |
| 100,000 | 0.03 s | <0.01 s |
| 200,000 | 0.06 s | <0.01 s |
| 500,000 | 0.13 s | <0.01 s |
| 900,000 | 0.43 s | <0.01 s |

This ideal covering-index probe is not a universal threshold; joins, FTS
ranking, cache, storage, and device speed move the crossover. The design rule
is therefore keyset/snapshot paging for every unbounded collection, not “use
offset until the database contains N notes.” H7 was inserted before importer
hardening with 10k/100k/500k profiles and a recorded-machine p95 target.

## Reference implementation review

Exact commits are the reviewed local clones; future work must re-check upstream
before adopting a dependency.

### movenotes-v3

Reviewed the complete README plus performance/static-site implementation
relevant to v0.3/v0.4. Useful behaviors:

- exact RAW/frontmatter/path/non-Markdown source preservation in optional
  bundles;
- deterministic one-pass inventory, source fingerprints, adaptive preload
  versus indexed input-scoped lookup, no per-file query, bounded batches and
  progress;
- streamed site metadata/index input, fixed navigation instead of a complete
  note tree, bucketed exact tags, and distinct small/static versus
  large/server-search publishing profiles;
- representative recorded import/site performance rather than unqualified
  scale claims.

Notrios adopted these as requirements, not code. v0.3 importer tasks now
preserve exact source representations and measure bounded inventory/batches;
v0.4 now has a scalable archive-site profile beside Quartz.

### Bluge

- Reviewed commit `574141970051...` (last upstream commit observed
  2022-07-04), Apache-2.0.
- Capabilities include BM25, phrase/prefix/fuzzy/range/geo queries, custom
  sorts, highlighting, facets/aggregations, and `TopNSearch.After` search-after.
- It exceeds current FTS5 plans in fuzzy search, typed queries, highlights,
  facets, and search-after, but adds another derived index and its maintenance
  activity is a material risk.

Decision: do not replace canonical FTS5 or optional Recoll. Evaluate Bluge only
behind the v0.4 generated-site search adapter against SQLite FTS and an external
Recoll service, with a maintenance/security/performance plan.

### recollwebui-go

Reviewed local `develop` commit
`d17d2fc9cbbe99792ab1f3245307de8e7abfd81f` (2026-07-23).
Reusable behavioral lessons:

- shell-free Recoll arguments and strict bounded base64 fields;
- exact-count/query-drift checks across page/export subprocesses;
- bounded streaming export batches and atomic private replacement;
- hostile snippets converted to bounded plain text;
- cancellation, stale-result revalidation, opaque identity, accessible bounded
  DOM, and native Recoll/Wails/AT-SPI evidence.

Its reviewed 100,000-result native profile returned 20 headless rows in
2.43/2.59 seconds with ~40 MiB process max RSS for first/second pages, while
native Wails process-tree evidence remained bounded by the visible page. Those
measurements are reference-machine evidence, not Notrios targets. The
page-number/absolute-offset model is explicitly not reused.

### Marmot

- Reviewed commit `c8ee91a825257f56dd5613f2f75a76907d2d1ecc`
  (2026-05-06), MIT.
- Marmot v2 provides leaderless full SQLite replication, gossip/anti-entropy,
  HLC/LWW, immutable CDC segments, distributed transactions, a SQL proxy, and
  server/edge cluster operations.
- Useful patterns: HLC ordering, append-only CDC segments, atomic
  manifest-last segment publication, anti-entropy summaries, and retention
  tied to replica progress.
- Poor fit: it changes the database write/operation model, assumes an
  operational cluster, does not solve Notrios profiles/folder/rclone/selective
  blobs, and brings a large NATS/gRPC/Pebble/Vitess/Kafka/metrics dependency
  surface.

Decision: reference patterns only; build a much smaller transport-neutral
record replication package.

### Cachapa CRDT/sqlite_crdt/crdt_sync

- Reviewed `crdt` commit `a361364...` (2024-11-02),
  `sqlite_crdt` `595d610...` (2025-10-27), and `crdt_sync` `5afa277...`
  (2024-11-02), all Apache-2.0 Dart packages.
- The design demonstrates a small HLC/LWW record abstraction, SQLite storage,
  delta changesets, and WebSocket reconnect/resume.
- It does not directly supply Go code or Notrios's per-field merges, notebook
  tree invariants, immutable resource dependencies, concurrent body
  revisions, per-replica sequence/ack vectors, or acknowledgement-gated GC.

Decision: closest conceptual reference, but do not port/adopt wholesale.

### Ygo implementations

- `github.com/reearth/ygo`, reviewed
  `a9cc4d42e001990d2f9500efcc722d2f8c90ff77` (2026-07-24), MIT, Go 1.23.
- `github.com/Deln0r/ygo`, reviewed
  `d298b1f88ef8f2a5505de8a7433501964931677f` (2026-07-23), MIT, Go 1.22.
- Both claim active pure-Go Yjs-compatible document CRDTs, state-vector sync,
  persistence/server/mobile paths, and extensive interoperability fixtures.

Decision: promising candidates for a separately scoped simultaneous-editing
feature. Per-character Yjs state is the wrong unit for relational whole-library
sync, bulk import, immutable attachments, and notebook/tree operations. Recheck
maintenance, conformance, security, history-size/GC, and real mobile results
when live co-editing is actually planned.

### Wails v3/mobile

Official v3 documentation says desktop/iOS/Android can reuse one `main.go` and
frontend, but v3 remains pre-release and mobile support experimental. Android
uses a native WebView/NDK-built Go library; iOS requires macOS/Xcode. Mobile
also lacks desktop window/menu behavior and save-file dialogs.

Decision: keep the stable Wails v2 desktop shell. Design sync/UI-independent
packages now, then run a separately approved v3 migration/Android real-device
spike with desktop regression, lifecycle/background, storage/picker, responsive
layout, and resource measurements.

References:

- <https://v3.wails.io/guides/mobile/>
- <https://v3.wails.io/faq/>

## Synchronization decision

The design is recorded in `SYNCHRONIZATION.md`. Core choices:

- distinguish profile, logical database, writable replica, and operation IDs;
- immutable operation IDs `(replica_id, sequence)` for completeness/dedupe;
- HLC only for deterministic conflict order; per-replica acknowledgement
  vectors for missing-change detection and GC;
- per-field LWW registers, per-membership set elements, immutable body revisions
  with visible concurrent conflicts, tombstones/death certificates, and
  deterministic notebook-cycle recovery;
- archive v2 immutable objects/manifests as full-snapshot bootstrap; bounded
  manifests/envelopes publish after dependencies;
- whole-blob/fixed chunks first; FastCDC only if modified-large-binary traces
  justify its index/GC complexity;
- REST is the online data plane; MCP controls bounded jobs/status/conflicts;
- shared folder/removable media/rclone carry the same immutable object format;
  use copy/immutable behavior, never mirrored deletion as the merge algorithm;
- target `none` performs no sync, and enabling later starts with a snapshot;
- backups are non-participating sinks; restore explicitly replaces, merges,
  adopts, or forks; peer retirement and offline retention are explicit.

Official rclone docs confirm that `sync` deletes destination files, while
`copy --immutable` can refuse modified existing files:

- <https://rclone.org/commands/rclone_sync/>
- <https://rclone.org/commands/rclone_copy/>

## Nostr and bitchat transport review

Nostr's signed immutable events and relay model could carry encrypted small
change-envelope pointers, but public-relay metadata, retention/availability,
deletion semantics, writer-key recovery, quotas, and separate attachment
storage make it a poor first transport. NIP-96 is currently marked
unrecommended/replaced by Blossom APIs, so it must not be cited as the default
current file API:

- <https://github.com/nostr-protocol/nips>

The current bitchat design contributes durable sender outboxes, unique IDs,
dedupe, acknowledgements/retries, TTL/hop/copy budgets, opportunistic couriers,
and encrypted store-and-forward patterns. BLE bandwidth, background/platform
limits, peer churn, and additional abuse/key-management surface make it
post-v1 research:

- <https://github.com/permissionlesstech/bitchat>
- <https://github.com/permissionlesstech/bitchat/blob/main/WHITEPAPER.md>

REST plus immutable rclone/folder transport therefore remains first.

## Import/export/backup/bulk/profile decisions

- Foreign import/export remains ETL/projection. Native snapshot/backup/sync
  share objects/manifests but not user semantics.
- Native archive v1 remains supported interchange and is documented as
  non-lossless. v0.4 native archive v2 includes revisions/provenance/source
  bundles/blobs/checksums/version bounds and becomes sync snapshot bootstrap.
- Imports inventory once, use deterministic IDs and fingerprints, batch
  transactions, publish blobs before references, and expose checkpoints.
- Exports use stable read snapshots. Online database backup uses SQLite's
  backup API; a complete backup includes assets.
- Profiles/database/replica identities precede sync. Fresh profiles bootstrap
  All notes, Notes, Help, and Trash.
- v0.6 gets bounded idempotent batch move/duplicate/trash/tag/untag/link
  operations with atomic/best-effort modes and per-item results.
- Stable external links include the portable database identity:
  `notrios://databases/.../documents/...`; the OS handler maps that identity to
  local profiles and prompts if multiple profiles point at clones.

## Files changed by the review

Living plan/architecture/policy/API/schema/testing/security/user/handoff files
were updated, `SYNCHRONIZATION.md` was added, and the required-file checker now
tracks it. Historical completed-plan content was not rewritten.

## Validation evidence

- `python3 scripts/check_required_files.py`: 64 required files present.
- `bash scripts/validate-scaffold.sh`: pass, including all Go packages and the
  installed live Recoll pipeline. The first sandboxed attempt could not bind
  loopback/write Recoll runtime state; the unrestricted rerun passed.
- `go vet ./...`: pass.
- `go test ./...`: pass through the scaffold validator.
- `web`: TypeScript typecheck, Vite production build, and 37 Vitest tests pass.
- `bash scripts/build_docs_site.sh`: 10 pages built; PageFind indexed all 10
  pages (1,626 words).
- `bash scripts/mvp_smoke.sh`: pass.
- `bash scripts/run_performance_smoke.sh`: pass
  (`BenchmarkGeneratedDatasetSearch-8`, 3.340 ms/op, 14,052 B/op, 194
  allocations/op on the recorded Intel i5-9300H run).
- Release-package preflight: `/tmp/notrios-plan-review-preflight.zip`, verified
  by `scripts/check_release_zip.py` (424 entries, 1,297,072 bytes,
  SHA-256 `c712bef0f9e90e9267381e4a5060bd1690afa0c821c3705cd2f0589e26155262`).

Model used: GPT-5.6 (Codex). No subagents. No GitHub push.
