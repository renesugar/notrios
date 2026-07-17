# Plan: v0.3 — Import, resource, and media hardening

Status: **active** (drafted 2026-07-16 from `ROADMAP.md`; supersedes the archived draft `plans/v0.2/001-import-resource-media-hardening.md`). The completed v0.2 redesign plan is archived under `plans/v0.2/`. H1–H4 are complete (`plans/v0.3/`); H5 is next.

## Goal

Extend v0.2 into a safer and more durable importer/resource pipeline and a production-quality search sidecar:

1. **Remote-media localization** through one policy engine (import-time, UI-triggered, and MCP-triggered all share it), per `SECURITY_AND_MEDIA_POLICY.md`: domain stop lists, quarantine fetches, SSRF protections, exact hashes, perceptual-hash hooks.
2. **Resource lifecycle**: dedup/reference reports, safe garbage collection with retention policy.
3. **Importer hardening** for Joplin RAW and Obsidian: notebook-hierarchy population (deferred from v0.2 task R12), larger fixtures, resume/checkpoint, dry-run diffs.
4. **Recoll hardening**: batched incremental scans, periodic reconciliation, merged-result quality, extraction status in the UI.

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

### H5. Exact-hash dedup reports and perceptual-hash hooks

- Resource/blob reference reports: duplicates by exact hash across collections, unreferenced blobs, per-notebook resource usage (REST + `notriosctl resources report`).
- Perceptual-hash **hook** interface (pluggable; no algorithm shipped yet): compute-and-store slot on admission, policy-check slot, and a near-duplicate review report that only ever *suggests* (perceptual matches are moderation/similarity signals — never silent dedup, per `SECURITY_AND_MEDIA_POLICY.md`).
- Document the hook contract in `SECURITY_AND_MEDIA_POLICY.md`.

Working state: users can inspect duplicate and unreferenced resources; perceptual hooks are wired but inert by default.

### H6. Resource garbage collection and retention

- Retention policy config: how long unreferenced blobs and resources of purged notes are kept.
- `notriosctl gc --dry-run` (default) reports exactly what would be removed and why; `--apply` deletes only unreferenced, retention-expired blobs/resources. Referenced resources are never eligible.
- REST admin report endpoint for the same data; no destructive REST endpoint without an explicit confirmation token.
- Tests covering reference counting edge cases (multi-document attachment, trash, purge).

Working state: GC never removes referenced data; dry run is the default everywhere; deletion requires an explicit flag.

### H7. Joplin RAW importer hardening

- Populate the notebook hierarchy from Joplin folder items (deferred from v0.2 R12): nested notebooks with original names, rename-on-conflict via the existing import-config mechanism.
- Preserve Joplin tags as Notrios tags.
- Resume/checkpoint report: per-item status persisted so a re-run after interruption skips completed work and reports progress.
- `--dry-run` diff summary: notes/resources/notebooks/tags that would be created, updated, or skipped.
- Larger synthetic fixtures (hundreds of notes, nested folders, tags, resources).

Working state: importer remains idempotent; a large interrupted import resumes cleanly; dry run matches the subsequent real run.

### H8. Obsidian importer hardening

- Populate notebooks from the vault folder hierarchy (same conflict/rename mechanism as H7).
- Improve aliases, frontmatter, embeds (`![[...]]`), block references, and relative-path resolution.
- Resume/checkpoint report and `--dry-run` diff summary (same shape as H7).
- Larger synthetic vault fixtures.

Working state: importer remains idempotent and graph-preserving for richer vaults; hierarchy and links survive round trips.

### H9. Recoll sidecar hardening

- Batched incremental scans: drain the outbox in bounded batches with backoff instead of unbounded single passes.
- Periodic reconciliation: compare the projection directory against canonical rows (missing/stale/orphaned files), repair, and report; expose last-reconciliation status via `/api/v1/status`.
- Merged-result quality: dedupe FTS5/Recoll hits by document, stable ordering, and per-hit source attribution in the search response.
- Extraction/index status in the GUI (sidecar enabled, last sync, backlog size).

Working state: with Recoll installed the index converges after crashes/manual file damage; without Recoll everything still passes.

### H10. v0.3 wrap-up: docs, feature matrix, release checklist

- User docs for media localization, resource reports/GC, importer resume/diff, and sidecar status (docs site + Help notebook reseed).
- Update `FEATURE_MATRIX.md` rows (remote localization, dedupe, GC: Soon → Implemented), `DATABASE_SCHEMA.md` (schema v7 as implemented), `API_SPEC.md`.
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

- `agent/OPEN_QUESTIONS.md` #3: notes of a deleted notebook go to Trash (current spec) — confirm before H7/H8 change notebook handling.
- `agent/OPEN_QUESTIONS.md` #4: FTS5 stays the always-on baseline with Recoll optional (current design assumed by H9).
- Which perceptual-hash algorithm (pHash/dHash/blockhash) to ship first — H5 only lands the hooks.

## Scope control

Quartz publishing and portable-vault export (v0.4), editor/graph UX (v0.5), MCP scope profiles beyond what exists (v0.6), sync (v0.7), HTTP range requests for resource content, and the official MCP Go SDK migration stay on the roadmap unless the user changes priorities.
