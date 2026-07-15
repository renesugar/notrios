# Plan: v0.2 Import, Resource, and Media Hardening

Status: draft, not started. The v0.1 MVP plan is complete and archived under `plans/v0.1/`. Ask the user before starting this plan.

This plan was generated from the `ROADMAP.md` v0.2 milestone after MVP Task 10 completed.

## v0.1 completed baseline

The completed MVP provides:

- local `notesd` service with REST API and built-in UI serving;
- SQLite canonical storage for collections, documents, revisions, resources, links, and FTS5 search;
- content-addressed resource storage and safe local download endpoint;
- Markdown editor/preview UI using `md-editor-rt`;
- preview routing for `document://` and `resource://` links;
- outgoing-link/backlink parsing and graph slice API;
- read-only MCP MVP adapter;
- Joplin RAW importer MVP;
- Obsidian importer MVP;
- release smoke, generated-dataset, and package verification scripts.

See `MVP_RELEASE_REPORT.md`, `RELEASE_CHECKLIST.md`, and `plans/v0.1/MVP_COMPLETION_SUMMARY.md`.

## v0.2 goal

Harden imports, resources, and remote-media handling so imported/clipped notes can become durable local archives without turning the companion service into an uncontrolled downloader.

## Working-state rule

Every task must leave the project in a working state. Update `agent/PLAN_STATUS.md`, append to `agent/ATTEMPT_LOG.jsonl`, append to `agent/MODEL_LOG.jsonl`, and archive completed task slices under `plans/v0.2/`.

## Incremental tasks

### 1. Configuration and policy schema

- Add typed remote-media/resource policy config.
- Parse domain allow/block/review lists.
- Add policy status to `/api/v1/status`.
- Add unit tests for policy parsing.

Working state: service starts with policy config and reports policy state.

### 2. Remote-media scan endpoint

- Implement Markdown scan for remote image/media URLs.
- Return per-URL policy decisions without downloading bytes.
- Surface remote-media warnings in the UI.

Working state: users can inspect remote media in a note before localization.

### 3. Quarantine download pipeline

- Download allowed/review media into temporary quarantine.
- Enforce timeout, redirect, scheme, domain, private-network, MIME, and size policies.
- Compute exact hashes before admission.

Working state: remote media can be fetched safely without entering the final asset store until checks pass.

### 4. Localize remote images

- Store accepted media as content-addressed resources.
- Rewrite Markdown image links to `resource://` URIs.
- Preserve original URL/provenance metadata.
- Support dry-run and optimistic concurrency.

Working state: a note with remote images can be converted to local resources.

### 5. Resource deduplication and retention

- Add resource/blob reference reports.
- Add safe garbage-collection dry run.
- Add retention policy for deleted/unreferenced resources.

Working state: users can inspect duplicate/unreferenced resources without deleting data unexpectedly.

### 6. Joplin RAW importer hardening

- Improve notebook/tag preservation.
- Add larger fixture coverage.
- Add import resume/checkpoint report.
- Add dry-run diff summary.

Working state: importer remains idempotent and gives useful large-import status.

### 7. Obsidian importer hardening

- Improve aliases, frontmatter, embeds, block references, and path handling.
- Add import resume/checkpoint report.
- Add dry-run diff summary.

Working state: importer remains idempotent and graph-preserving for richer vaults.

### 8. Additional importers planning slice

- Create implementation plans for Twitter/X, ChatGPT, and Claude importers.
- Add fixture format descriptions.
- Do not implement all three unless explicitly approved.

Working state: next importers are specified enough for small implementation tasks.

## Validation

Use the v0.1 release-hardening validation set until a task adds more specific checks:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

## Scope control

Remote-media localization, content-hash policy, and importer hardening are in scope for v0.2. sist2, Quartz publishing, sync, advanced graph UX, and CodeMirror 6 migration remain later roadmap items unless the user explicitly changes priorities.
