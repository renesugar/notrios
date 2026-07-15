# v0.2 Draft Plan — Import, Resource, and Media Hardening

Status: draft, not started. Ask the user before implementing this plan.

## Goal

Extend the v0.1 MVP into a safer and more durable importer/resource pipeline, with the same policy engine used by bulk import, UI actions, and MCP/admin workflows.

## Incremental tasks

Each task must leave the project in a working state and produce a ZIP when completed.

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

## Deferred beyond v0.2

- sist2 sidecar indexing.
- Quartz publishing.
- official MCP SDK migration unless needed immediately.
- CodeMirror 6/unified editor migration.
