# Context Map

This file is the codebase atlas. Update it whenever major files or directories are added.

## Root documents

- `README.md` — project overview and quick start.
- `PLAN.md` — active implementation plan (v0.2 Notrios redesign foundation).
- `SCAFFOLD_CREATION_PLAN.md` — process for creating/refining this scaffold.
- `ROADMAP.md` — product roadmap and future features.
- `AGENTS.md` — coding-agent instructions (`CLAUDE.md` points here).
- `CODING_CLIENT_HANDOFF.md` — compressed project state for any coding agent (formerly `CODEX_HANDOFF.md`).
- `SYSTEM_ARCHITECTURE.md` — architectural blueprint.
- `API_SPEC.md` — REST/MCP contract notes.
- `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md` — notebooks, tags, search notebooks, Trash/Help semantics.
- `SEARCH_QUERY_LANGUAGE.md` — user query language and backend translation.
- `RECOLL_INTEGRATION.md` — Recoll sidecar design and licensing boundary.
- `DOCS_SITE.md` — GitHub Pages documentation site (PageFind) and Help notebook.
- `CODING_STANDARDS.md` — coding style and guardrails.
- `TESTING_POLICY.md` — definition of done and testing layers.
- `ENVIRONMENT_SETUP.md` — development setup.
- `PROMPT.md` — initial agent prompt.

## Code directories

- `cmd/notriosd/` — service daemon entry point.
- `cmd/notriosctl/` — CLI/admin/import command entry point.
- `internal/importers/joplinraw/` — MVP Joplin RAW Export Directory parser/importer.
- `internal/importers/obsidian/` — MVP Obsidian vault Markdown/assets parser/importer.
- `internal/api/` — shared API request/response models.
- `internal/httpapi/` — REST HTTP adapter for status, documents, revisions, resources, links, graph slices, and staged future routes.
- `internal/store/` — SQLite-backed persistence, document CRUD, revision history, soft delete, restore, FTS5 search, resource storage, link graph persistence, and (schema v5) notebooks/tags/search-notebooks/trash operations (`sqlite_notebooks.go`).
- `internal/markdownlinks/` — conservative MVP Markdown/Obsidian/app-URI link extractor.
- `internal/version/` — version constants.
- `migrations/` — SQLite schema migrations.
- `web/` — React/Vite built-in UI scaffold.

## Agent support

- `agent/PLAN_STATUS.md` — current task and working-state notes.
- `agent/ATTEMPT_LOG.jsonl` — append-only attempt history.
- `agent/MODEL_LOG.jsonl` — model/session tracking.
- `agent/LOOP_DETECTION.md` — stalled-task detection and resolution.
- `plans/` — archived completed plans by version/milestone.
- `skills/` — agent skills using the `SKILL.md` format.
- `prompts/` — reusable agent prompts.

## API and configuration

- `api/openapi.yaml` — initial REST OpenAPI skeleton.
- `config/config.example.yaml` — example service configuration and media policy.

## Scripts

- `scripts/check_required_files.py` — verifies required scaffold files exist.
- `scripts/validate-scaffold.sh` — runs current scaffold validation checks.

## Added design docs

- `FEATURE_MATRIX.md` — milestone and ownership map.
- `UI_DESIGN.md` — web UI/editor decisions.
- `PUBLISHING_POLICY.md` — Quartz/static publishing rules.
- `VERSIONING_AND_SYNC_POLICY.md` — revisions/checkpoint/sync decisions.
- `WORKSPACE_MAINTENANCE.md` — query/lint/outline/block features.
- `SCAFFOLD_REVIEW_REPORT.md` — Step 2 repair summary.

## Step 3 additions

- `DATABASE_SCHEMA.md` explains the target SQLite schema and why search sidecars (now Recoll) remain derived indexes.
- `api/openapi.yaml` now contains the expanded REST scaffold for collections, documents, resources, revisions, links, remote media, graph, publishing, and jobs. Document, resource, graph, and read-only MCP MVP routes have live implementations; import/publish/remote-media execution remains staged.
- `api/mcp-tools.md` now defines MCP tool profiles and the implemented read-only MVP tools. `internal/httpapi/mcp.go` contains the current dependency-free adapter mounted at `/mcp`.
- `internal/api/types.go` mirrors the current REST DTO shapes.
- `internal/httpapi/server.go` has placeholder route handlers for future routes and live SQLite-backed handlers for the Step 4 document create/read/body/search slice.

## MVP Task 1 additions

- `internal/config/` — small dependency-free config loader for the documented YAML subset and directory bootstrap helper.
- `internal/store.StoreStatus` — database driver/path/state/schema-version reporting for status output.
- `MVP_TASK1_REPORT.md` — service persistence foundation completion report.

## MVP Task 2 additions

- `internal/store` now exposes update, soft-delete, revision list/read, and revision restore operations with optimistic concurrency.
- `internal/httpapi` implements `PUT`/`PATCH`/`DELETE` document routes plus revision list/read/restore.
- `web/src/api.ts` and `web/src/App.tsx` can save a new revision for the currently opened note.
- `MVP_TASK2_REPORT.md` records this task.

## MVP Task 4 additions

- `internal/store` now streams resource bytes into the configured asset store and deduplicates exact blobs by SHA-256.
- REST implements resource metadata/content and document-resource attach/list/detach routes.
- `web/src/App.tsx` can upload/list/download attached resources.
- `MVP_TASK4_REPORT.md` records this task.

## MVP Task 5 additions

- `internal/markdownlinks` extracts common Markdown links/images, Obsidian wikilinks/embeds, app URIs, external URLs, heading anchors, and block anchors.
- `internal/store` rebuilds `document_links` rows transactionally on document create/update/restore and clears outgoing links on soft delete.
- `internal/httpapi` implements `GET /api/v1/documents/{document_id}/links` and `POST /api/v1/graph`.
- `web/src/api.ts` and `web/src/App.tsx` can list and display outgoing links/backlinks for the opened note.
- `MVP_TASK5_REPORT.md` records this task.


## MVP Task 6 UI files

- `web/src/App.tsx`: REST-backed Markdown UI with `md-editor-rt`, preview normalization, app URI routing, resource upload, link/backlink/resource sidebars.
- `web/src/styles.css`: app layout plus editor, link-list, resource-list, and text-button styling.
- `internal/httpapi/server.go`: now serves `web/dist` for the built-in UI when the production build exists.
- `MVP_TASK6_REPORT.md`: implementation notes and deliberate limitations for the UI MVP.

## MVP Task 8 importer files

- `internal/importers/joplinraw/joplinraw.go` parses Joplin RAW item files, imports notes/resources, rewrites `:/<id>` links, and returns a JSON-serializable report.
- `internal/importers/joplinraw/joplinraw_test.go` builds a small RAW fixture and verifies import, resource attachment, link rewriting, searchability, and re-run behavior.
- `cmd/notriosctl/main.go` now includes `notriosctl import joplin-raw`.
- Store create requests support optional preferred IDs so importers can create deterministic source-derived document/resource IDs.


## MVP Task 9 importer files

- `internal/importers/obsidian/obsidian.go` scans an Obsidian-style vault, imports Markdown notes and non-Markdown assets with deterministic source-path IDs, preserves/augments frontmatter, attaches referenced local assets, and refreshes link indexes after the batch.
- `internal/importers/obsidian/obsidian_test.go` builds a small vault fixture and verifies import, resource attachment, Wikilink/embed/backlink resolution, unresolved-link preservation, searchability, and idempotent re-run behavior.
- `cmd/notriosctl/main.go` now includes `notriosctl import obsidian`.
- `store.RebuildDocumentLinks` lets batch importers refresh link resolution after all target documents/resources exist without creating extra revisions.


## Release hardening files

- `PACKAGING.md` — release ZIP contents, exclusions, and package commands.
- `SECURITY_REVIEW.md` — v0.1 security review for resource downloads, preview sanitization, MCP, and importers.
- `RELEASE_CHECKLIST.md` — pre-tag checklist for `v0.1.0-mvp`.
- `MVP_RELEASE_REPORT.md` — completed MVP capability summary and deferred features.
- `scripts/mvp_smoke.sh` — end-to-end local REST/MCP/resource smoke test.
- `scripts/run_performance_smoke.sh` — generated-dataset smoke and benchmark wrapper.
- `scripts/package_release.sh` — validates, builds UI, creates source ZIP, and verifies contents.
- `scripts/check_release_zip.py` — catches missing `web/dist`, accidental `web/node_modules`, and runtime data in ZIPs.

## v0.2 Notrios redesign additions (task R1)

- Git repository initialized: `main` = pre-redesign baseline, `develop` = active work.
- `CODEX_HANDOFF.md` renamed to `CODING_CLIENT_HANDOFF.md`; `CLAUDE.md` added pointing to `AGENTS.md`.
- `skills/codex-handoff/` renamed to `skills/agent-handoff/`; `prompts/start_codex_from_handoff.md` renamed to `prompts/start_agent_from_handoff.md`.
- New design docs: `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `RECOLL_INTEGRATION.md`, `DOCS_SITE.md`.
- Living design docs rebranded to Notrios and switched from sist2 to Recoll; historical reports (`SCAFFOLD_STEP*`, `MVP_TASK*`, `plans/`) intentionally keep old names as records.

## v0.2 task R3 additions (schema v5)

- `migrations/0001_initial.sql` (and the embedded copy under `internal/store/migrations/`) now creates `notebooks`, `tags`, `note_tags`, and `search_notebooks`, adds `documents.notebook_id`, and sets `PRAGMA user_version = 5`; `ensureSchemaV5` upgrades v4 databases and bootstrap backfills existing notes into the default notebook.
- `internal/store/sqlite_notebooks.go` — notebook CRUD (nested, emoji, case-insensitive sibling-unique names, cycle-safe moves, recursive delete-to-trash), tag add/remove/list with live note counts, search-notebook lifecycle with builtin protection, and trash list/restore/purge.
- `internal/store/notebooks_test.go` — coverage for bootstrap builtins, naming rules, nesting, membership/move, recursive delete, trash/restore/purge, tag counts, sidebar ordering, and the v4→v5 upgrade path.

## v0.2 task R4 additions (schema v6)

- `migrations/0001_initial.sql` (both copies) adds `document_sources` (source system, external ID, author/author_id, thread_id/reply_to, source URL, published_at + derived published_ts) and sets `PRAGMA user_version = 6`.
- `internal/store/sqlite_sources.go` — `SetDocumentSource` upsert (works for trashed notes so importers can backfill), `GetDocumentSource`, `FindDocumentBySource` (importer idempotency), `ListThreadDocuments` (chronological thread recovery, trashed notes excluded); `PurgeDocument` now refuses externally-sourced notes.
- `internal/importers/joplinraw` and `internal/importers/obsidian` record provenance rows on every import run (re-running an import backfills existing notes).
- `internal/store/sources_test.go` plus importer-test assertions cover upsert/lookup, thread ordering, purge protection, and importer provenance.

## v0.2 task R6 additions (query language)

- `internal/query/` — backend-agnostic parser for the user search language (`notebook:`, `tag:`, `title:`, `author:`, `authorid:`, `since:`, `until:`, `is:trashed`, quoted phrases, implicit AND; date-only/time-only timestamp semantics per `SEARCH_QUERY_LANGUAGE.md`).
- `internal/store/sqlite_query.go` — compiles parsed queries to FTS5 + SQL (notebook subtree expansion, tag EXISTS filters, provenance joins for author/time, LIKE fallback for trash queries) and implements opaque query-bound cursors for incremental scrolling. `store.Search` now routes every query through the adapter.

## v0.2 task R7 additions (Recoll sidecar)

- `internal/projection/` — outbox-driven Markdown+front-matter filesystem projection of managed notes (plus `FullSync` for pre-outbox databases); document mutations now enqueue `index_outbox` jobs transactionally (`internal/store/sqlite_outbox.go`).
- `internal/recoll/` — external-process Recoll sidecar: generated config (fields prefixes, `publishedts` range slot, `underscoreasletter`), the embedded from-scratch `notrios_md_handler.py` front-matter handler, `recollindex`/`recollq` invocation, query compilation, and result parsing. GPL boundary: binaries are user-installed and never linked or vendored.
- `internal/httpapi/sidecar_search.go` — merges sidecar-only hits into search results behind the existing API; FTS5 stays authoritative and sidecar failures degrade gracefully.
- `cmd/notriosd` — activates the sidecar when `search_sidecar.enabled` is true: startup full sync + index, 30s outbox drain loop, merged search.
