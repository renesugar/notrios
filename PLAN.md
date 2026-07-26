# Plan: v0.3 — Import, resource, and media hardening

Status: **active** (drafted 2026-07-16 from `ROADMAP.md`; supersedes the archived draft `plans/v0.2/001-import-resource-media-hardening.md`). The completed v0.2 redesign plan is archived under `plans/v0.2/`. H1–H6 are complete (`plans/v0.3/`); H7 is next.

## Goal

Extend v0.2 into a safer and more durable importer/resource pipeline and a production-quality search sidecar:

1. **Remote-media localization** through one policy engine (import-time, UI-triggered, and MCP-triggered all share it), per `SECURITY_AND_MEDIA_POLICY.md`: domain stop lists, quarantine fetches, SSRF protections, exact hashes, perceptual-hash hooks.
2. **Resource lifecycle**: dedup/reference reports, safe garbage collection with retention policy.
3. **Large-library foundations**: replace offset-backed cursors, add supporting
   indexes and measure representative 10k/100k/500k-note workloads before the
   hardened importers make those workloads routine.
4. **Importer hardening** for Joplin RAW and Obsidian: notebook-hierarchy population (deferred from v0.2 task R12), bounded-memory inventory/batches, resume/checkpoint, dry-run diffs, and loss-preserving source metadata.
5. **Recoll hardening**: batched incremental scans, periodic reconciliation, merged-result quality, extraction status in the UI.

## Working-state rule

Every task must leave the project in a working state. Update `agent/PLAN_STATUS.md`, append to `agent/ATTEMPT_LOG.jsonl` and `agent/MODEL_LOG.jsonl`, commit with git, and archive completed slices under `plans/v0.3/`. Ask the user before starting the next task.

## Tasks

### H1. Media-policy configuration and schema — COMPLETED (see `plans/v0.3/001-media-policy-config-schema.md`)

- Typed remote-media policy config (`media_policy` section): allowed/blocked/review domain patterns, max download size, fetch timeout, max redirect hops, allowed MIME types, private-network and scheme rules (block `file:`, `data:`, link-local/private addresses by default).
- Schema v7: media-policy tables per `DATABASE_SCHEMA.md` — domain rules, exact-hash blocks, perceptual-hash blocks (hooks only for now), remote-media attempts (original URL, final URL, decision, timestamps), quarantine state, resource hash records.
- `/api/v1/status` reports policy state (enabled, rule counts, quarantine dir).
- Unit tests for config parsing and migration.

Working state: service starts with (or without) a policy config and reports policy state; schema migrates v6→v7; all existing tests pass.

### H2. Remote-media scan endpoint — COMPLETED (see `plans/v0.3/002-remote-media-scan.md`)

- `GET/POST /api/v1/documents/{id}/remote-media`: parse the note body for remote `http(s)` image/media URLs and return per-URL policy decisions (allow/block/review + reason) **without downloading anything**.
- Surface the scan in the GUI: remote-media warnings in the editor's note inspector (count + per-URL decision), consistent with the preview rule — the browser never fetches as a policy signal.
- OpenAPI + `api/mcp-tools.md` updates (read-only MCP tool `scan_remote_media`).

Working state: users can inspect a note's remote media and the policy verdicts before any localization.

### H3. Quarantine download pipeline — COMPLETED (see `plans/v0.3/003-quarantine-pipeline.md`)

- Fetch allowed/review URLs into a quarantine directory (under the data dir, never the asset store) with: URL normalization, scheme/domain checks re-applied to **every redirect hop**, private-network/link-local blocking at connect time (SSRF protection), size cap enforced while streaming, timeout, MIME sniffing (`http.DetectContentType`) checked against policy, exact SHA-256 computed on the quarantined bytes.
- Record every attempt (success or refusal) in the remote-media attempts table with the policy decision.
- No admission to the content-addressed store in this task.

Working state: `internal/media` can quarantine a URL list safely; refusals are recorded and reported; nothing reaches the asset store.

### H4. Localize remote media — COMPLETED (see `plans/v0.3/004-localize-remote-media.md`)

- Admission: exact-hash block check, then content-addressed store admission (dedup by construction), resource + provenance rows (original URL, final URL, retrieved timestamp, content type, hashes, decision).
- Rewrite the note's Markdown image/media links to `resource://` URIs in a **new revision** with a `base_revision_id` precondition; attach resources to the document.
- `--dry-run` mode reports what would be downloaded/rewritten without fetching.
- Entry points sharing the same engine: `POST /api/v1/documents/{id}/remote-media/localize` (REST), `notriosctl localize <document-id>` (CLI), `localize_remote_media` (MCP, editor scope, revision precondition), and an importer flag (`--localize-media`) for import-time localization.
- GUI: localize action from the note inspector with results (localized/skipped/blocked).

Working state: a note with remote images can be safely converted to local resources from UI, CLI, REST, or MCP; blocked domains stay blocked; dry run never writes.

### H5. Exact-hash dedup reports and perceptual-hash hooks — COMPLETED (see `plans/v0.3/005-exact-hash-reports-perceptual-hooks.md`)

- Resource/blob reference reports: duplicates by exact hash across collections, unreferenced blobs, per-notebook resource usage (REST + `notriosctl resources report`).
- Perceptual-hash **hook** interface (pluggable; no algorithm shipped yet): compute-and-store slot on admission, policy-check slot, and a near-duplicate review report that only ever *suggests* (perceptual matches are moderation/similarity signals — never silent dedup, per `SECURITY_AND_MEDIA_POLICY.md`).
- Document the hook contract in `SECURITY_AND_MEDIA_POLICY.md`.

Working state: users can inspect duplicate and unreferenced resources; perceptual hooks are wired but inert by default.

### H6. Resource garbage collection and retention — COMPLETED (see `plans/v0.3/006-resource-garbage-collection-retention.md`)

- Retention policy config: how long unreferenced blobs and resources of purged notes are kept.
- `notriosctl gc --dry-run` (default) reports exactly what would be removed and why; `--apply` deletes only unreferenced, retention-expired blobs/resources. Referenced resources are never eligible.
- REST admin report endpoint for the same data; no destructive REST endpoint without an explicit confirmation token.
- Tests covering reference counting edge cases (multi-document attachment, trash, purge).
- Keep deletion mechanics behind a sync-aware retention interface: v0.3 has
  only the local replica, while v0.7 will also require peer acknowledgement
  watermarks before blobs or tombstones become eligible.

Working state: GC never removes referenced data; dry run is the default everywhere; deletion requires an explicit flag.

### H7. Large-library pagination and performance baseline

- Replace `q1:<offset>` cursors and `LIMIT ... OFFSET` for chronological
  document/notebook/search-notebook traversal with query-bound keyset cursors.
  The stable order is `(updated_at DESC, id DESC)` with matching composite
  indexes. Cursors are opaque, versioned, and rejected when replayed against a
  different query or sort.
- Design relevance pagination separately: use a stable `(score, id)` boundary
  when FTS5 can reproduce it; otherwise use a bounded, generation-labelled
  search snapshot. Never silently present an offset token as a keyset cursor.
- Make FTS5/Recoll merged paging stable on every page, not only page one.
- Add generated 10k, 100k, and 500k note/resource profiles. Measure first page,
  90th-percentile-deep traversal, a selective text query, notebook/tag filters,
  and importer/export streaming. Record query plans, elapsed time, and peak RSS.
- Performance gates: ordinary first/next local pages target p95 under 100 ms on
  the recorded reference machine; memory and DOM rows stay proportional to
  page/batch size. Treat regressions as evidence, not as a machine-independent
  absolute promise.
- Remove the current 100,000-offset ceiling only after all unbounded routes have
  a scalable cursor or an explicitly documented bounded-result contract.

Working state: users can traverse a collection with hundreds of thousands of
notes without linear offset work or a 100,000-result dead end; the benchmark is
reproducible and reports its environment.

### H8. Joplin RAW importer hardening

- Populate the notebook hierarchy from Joplin folder items (deferred from v0.2 R12): nested notebooks with original names, rename-on-conflict via the existing import-config mechanism.
- Preserve Joplin tags as Notrios tags.
- Preserve exact RAW bytes, property order, and unknown properties in an
  optional source bundle so a native/archive round trip need not discard
  information that is not part of the canonical note model.
- Build one deterministic source inventory; parse parent folders once; preload
  small collections and use indexed, input-scoped lookups for large ones.
  Perform bounded batches rather than per-file database queries.
- Persist source fingerprints and a resume/checkpoint report so unchanged
  items are skipped, interrupted batches resume, and progress is observable.
- `--dry-run` diff summary: notes/resources/notebooks/tags that would be created, updated, or skipped.
- Larger synthetic fixtures (hundreds through 100k generated notes, nested
  folders, tags, resources) and a fixture with unknown/reordered RAW properties.

Working state: importer remains idempotent; a large interrupted import resumes cleanly; dry run matches the subsequent real run.

### H9. Obsidian importer hardening

- Populate notebooks from the vault folder hierarchy (same conflict/rename mechanism as H8).
- Improve aliases, frontmatter, embeds (`![[...]]`), block references, and relative-path resolution.
- Preserve the original Markdown bytes, frontmatter bytes, relative path, and
  non-Markdown file bytes in the optional source bundle; canonical parsing is
  additional metadata, not a replacement for the source representation.
- Use one-pass discovery, deterministic path IDs, indexed fingerprints, and
  bounded batches. Resume/checkpoint and `--dry-run` use the same report shape
  as H8.
- Larger synthetic vault fixtures through the H7 scale profiles.

Working state: importer remains idempotent and graph-preserving for richer vaults; hierarchy and links survive round trips.

### H10. Recoll sidecar hardening

- Batched incremental scans: drain the outbox in bounded batches with backoff instead of unbounded single passes.
- Periodic reconciliation: compare the projection directory against canonical rows (missing/stale/orphaned files), repair, and report; expose last-reconciliation status via `/api/v1/status`.
- Merged-result quality: dedupe FTS5/Recoll hits by document, stable cursor
  ordering across all pages, and per-hit source attribution in the response.
- Extraction/index status in the GUI (sidecar enabled, last sync, backlog size).
- Reuse the bounded process, hostile-content, exact-count drift, export
  streaming, accessibility, and native-evidence patterns established by
  `recollwebui-go`; do not copy its offset paging or product-specific code.

Working state: with Recoll installed the index converges after crashes/manual file damage; without Recoll everything still passes.

### H11. v0.3 wrap-up: docs, feature matrix, release checklist

- User docs for media localization, resource reports/GC, importer resume/diff, and sidecar status (docs site + Help notebook reseed).
- Reconcile `FEATURE_MATRIX.md` active/implemented rows for H5–H10,
  `DATABASE_SCHEMA.md`, and `API_SPEC.md`.
- `RELEASE_CHECKLIST.md` v0.3.0 section; full validation + packaging run.

Working state: documentation matches implementation; release checks pass.

## Validation

Until a task adds more specific checks:

```bash
go vet ./... && go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm run typecheck && npm run build && npm test
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

GUI-affecting tasks also build with `make gui` and, for layout changes, run `scripts/verify_layout_resize.py` under Xvfb/Openbox (see `TESTING_POLICY.md`).

## Open questions (carried into v0.3)

- `agent/OPEN_QUESTIONS.md` #3: notes of a deleted notebook go to Trash (current spec) — confirm before H8/H9 change notebook handling.
- `agent/OPEN_QUESTIONS.md` #4: FTS5 stays the always-on baseline with Recoll optional (current design assumed by H10).
- Which perceptual-hash algorithm (pHash/dHash/blockhash), if any, should
  eventually occupy the implemented H5 hook? Core ships none.

## Scope control

Publishing/native archive v2 and portable-vault export (v0.4), editor/graph UX
(v0.5), MCP scope profiles and bulk organizer operations (v0.6), sync (v0.7),
HTTP range requests for resource content, Wails v3/mobile migration, and the
official MCP Go SDK migration stay on the roadmap unless the user changes
priorities. Bluge and Yjs-compatible Ygo are research options, not v0.3
dependencies.
