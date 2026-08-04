# Context Map

This file is the codebase atlas. Update it whenever major files or directories are added.

## Root documents

- `README.md` — project overview and quick start.
- `PLAN.md` — active v0.4 portable-data/publishing/stable-reference plan;
  J1–J3, Q1, P1, P2, and P3 are complete. P3a (archive-v2 large-library
  container revision) is next and requires user approval, followed by P4.
- `plans/scaffold/SCAFFOLD_CREATION_PLAN.md` — process for creating/refining this scaffold.
- `ROADMAP.md` — product roadmap and future features.
- `AGENTS.md` — coding-agent instructions (`CLAUDE.md` points here).
- `CODING_CLIENT_HANDOFF.md` — compressed project state for any coding agent (formerly `CODEX_HANDOFF.md`).
- `SYSTEM_ARCHITECTURE.md` — architectural blueprint.
- `API_SPEC.md` — REST/MCP contract notes.
- `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md` — notebooks, tags, search notebooks, Trash/Help semantics.
- `SEARCH_QUERY_LANGUAGE.md` — user query language and backend translation.
- `RECOLL_INTEGRATION.md` — Recoll sidecar design and licensing boundary.
- `SYNCHRONIZATION.md` — planned v0.7 database/replica identities, CRDT/change
  rules, archive/envelope format, REST/folder/rclone transports, retention,
  backup/restore relationships, library decision, and validation.
- `NATIVE_ARCHIVE_V2.md` — v2 identity, manifest-last object/record contract,
  compatibility/limits, explicit restore intent, verification rules, P3 export
  staging/resume semantics, and the open object-count bound.
- `DOCS_SITE.md` — GitHub Pages documentation site (PageFind) and Help notebook.
- `CODING_STANDARDS.md` — coding style and guardrails.
- `TESTING_POLICY.md` — definition of done and testing layers.
- `ENVIRONMENT_SETUP.md` — development setup.
- `PROMPT.md` — initial agent prompt.

## Code directories

- `cmd/notriosd/` — service daemon entry point.
- `cmd/notriosctl/` — CLI/admin/import command entry point.
- `internal/importers/joplinraw/` — hardened Joplin RAW importer: canonical
  first-line titles, CR/LF-only physical parsing (including OCR controls), one
  ordered future/duplicate-property parse, deterministic inventory, nested
  notebooks, real tags, exact optional source bundles, fingerprints, bounded
  batches, checkpoints/resume, and dry-run/config planning.
- `internal/importers/obsidian/` — hardened deterministic vault importer with
  nested notebooks, exact optional source bundles, bounded checkpoints/resume,
  dry-run/config planning, and canonical alias/embed/anchor resolution.
- `internal/api/` — shared API request/response models.
- `internal/httpapi/` — REST HTTP adapter for status, documents, revisions, resources, links, graph slices, and staged future routes.
- `internal/store/` — SQLite-backed persistence, document CRUD, revision history, soft delete, restore, FTS5 search, resource storage/reference reports (`sqlite_resource_reports.go`), retention-aware GC (`sqlite_gc.go`), importer batch/checkpoint/source-bundle state (`sqlite_imports.go`), link graph persistence, and notebooks/tags/search-notebooks/trash operations (`sqlite_notebooks.go`).
- `internal/archivev2/` — native archive-v2 manifest/record model,
  identity-intent planner, bounded directory/hash/MIME/reference verifier,
  synthetic golden/adversarial fixtures, and the P3 manifest-last streaming
  exporter (`export.go`) with its generated scale profile.
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
- `SELECTION_AND_PRIVACY_PLANNER.md` — live P1 typed selection, target privacy
  defaults, manifest/report shapes, content boundary, and scale limits.
- `VERSIONING_AND_SYNC_POLICY.md` — revision/checkpoint policy and summary of
  the synchronization invariants.
- `WORKSPACE_MAINTENANCE.md` — query/lint/outline/block features.
- `plans/scaffold/SCAFFOLD_REVIEW_REPORT.md` — Step 2 repair summary.

## Step 3 additions

- `DATABASE_SCHEMA.md` explains the target SQLite schema and why search sidecars (now Recoll) remain derived indexes.
- `api/openapi.yaml` contains the REST contract for collections, documents,
  resources, revisions, links, remote media, graph, selection planning, and jobs.
  Remote-media scan/localization and selection/privacy dry runs are live; publish/import job execution,
  profiles/batches/sync remain staged.
- `api/mcp-tools.md` now defines MCP tool profiles and the implemented read-only MVP tools. `internal/httpapi/mcp.go` contains the current dependency-free adapter mounted at `/mcp`.
- `internal/api/types.go` mirrors the current REST DTO shapes.
- `internal/httpapi/server.go` and `selection.go` expose live SQLite-backed
  document/search and shared selection-planning routes; future job routes stay
  staged.

## MVP Task 1 additions

- `internal/config/` — small dependency-free config loader for the documented YAML subset and directory bootstrap helper.
- `internal/store.StoreStatus` — database driver/path/state/schema-version reporting for status output.
- `plans/mvp/MVP_TASK1_REPORT.md` — service persistence foundation completion report.

## MVP Task 2 additions

- `internal/store` now exposes update, soft-delete, revision list/read, and revision restore operations with optimistic concurrency.
- `internal/httpapi` implements `PUT`/`PATCH`/`DELETE` document routes plus revision list/read/restore.
- `web/src/api.ts` and `web/src/App.tsx` can save a new revision for the currently opened note.
- `plans/mvp/MVP_TASK2_REPORT.md` records this task.

## MVP Task 4 additions

- `internal/store` now streams resource bytes into the configured asset store and deduplicates exact blobs by SHA-256.
- REST implements resource metadata/content and document-resource attach/list/detach routes.
- `web/src/App.tsx` can upload/list/download attached resources.
- H5 adds read-only exact-duplicate/unreferenced/per-notebook usage reports
  through Store, REST, and `notriosctl`, plus an optional
  `PerceptualHashHook`; no perceptual algorithm is installed by default.
- H6 adds schema-v8 unreferenced timestamps/reasons, retention config,
  `RetentionGate`, dry-run/apply GC in `notriosctl`, and a read-only REST GC
  report. Permanent REST deletion requires object-specific confirmation.
- `plans/mvp/MVP_TASK4_REPORT.md` records this task.

## MVP Task 5 additions

- `internal/markdownlinks` extracts common Markdown links/images, Obsidian wikilinks/embeds, app URIs, external URLs, heading anchors, and block anchors.
- `internal/store` rebuilds `document_links` rows transactionally on document create/update/restore and clears outgoing links on soft delete.
- `internal/httpapi` implements `GET /api/v1/documents/{document_id}/links` and `POST /api/v1/graph`.
- `web/src/api.ts` and `web/src/App.tsx` can list and display outgoing links/backlinks for the opened note.
- `plans/mvp/MVP_TASK5_REPORT.md` records this task.


## MVP Task 6 UI files

- `web/src/App.tsx`: REST-backed Markdown UI with `md-editor-rt`, preview normalization, app URI routing, resource upload, link/backlink/resource sidebars.
- `web/src/styles.css`: app layout plus editor, link-list, resource-list, and text-button styling.
- `internal/httpapi/server.go`: now serves `web/dist` for the built-in UI when the production build exists.
- `plans/mvp/MVP_TASK6_REPORT.md`: implementation notes and deliberate limitations for the UI MVP.

## MVP Task 8 importer files

- `internal/importers/joplinraw/joplinraw.go` owns RAW parsing/body/link helpers and the JSON report contract; `scalable.go` owns deterministic inventory, conflict planning, bounded phases, fingerprints, exact bundles, and resume.
- `internal/importers/joplinraw/joplinraw_test.go` covers hierarchy, real tags and renames, exact RAW bytes/property order, dry-run parity, resource refresh, conflicts, bounded batches, and resume; `profile_test.go` drives generated 100/10k/100k profiles.
- `cmd/notriosctl/main.go` now includes `notriosctl import joplin-raw`.
- Store create requests support optional preferred IDs so importers can create deterministic source-derived document/resource IDs.


## MVP Task 9 importer files

- `internal/importers/obsidian/obsidian.go` owns deterministic one-pass vault
  inventory, nested-folder conflict planning, bounded fingerprint/checkpoint
  phases, exact source bundles, canonical alias/relative/embed/anchor
  resolution, resource refresh, and link rebuild.
- `internal/importers/obsidian/obsidian_test.go` covers exact Markdown/binary
  recovery, hierarchy, conflicts/renames, dry-run parity, interruption/resume,
  resource refresh, richer graph edges, Trash, search, and idempotence;
  `profile_test.go` drives generated 100/10k/100k/500k tiers.
- `cmd/notriosctl/main.go` exposes Obsidian batch, source preservation,
  dry-run config, resume progress, and media-localization flags.
- `store.RebuildDocumentLinks` lets batch importers refresh link resolution after all target documents/resources exist without creating extra revisions.


## Release hardening files

- `PACKAGING.md` — release ZIP contents, exclusions, and package commands.
- `SECURITY_REVIEW.md` — v0.1 security review for resource downloads, preview sanitization, MCP, and importers.
- `RELEASE_CHECKLIST.md` — pre-tag checklist for `v0.1.0-mvp`.
- `plans/mvp/MVP_RELEASE_REPORT.md` — completed MVP capability summary and deferred features.
- `scripts/mvp_smoke.sh` — end-to-end local REST/MCP/resource smoke test.
- `scripts/run_performance_smoke.sh` — generated-dataset smoke and benchmark wrapper.
- `scripts/run_large_library_profile.sh` — reproducible H7
  10k/100k/500k keyset/search/resource profile driver.
- `scripts/run_joplin_import_profile.sh` — reproducible H8 generated
  100/10k/100k dry-run plus interrupted/resumed import profile.
- `scripts/run_obsidian_import_profile.sh` — reproducible H9 generated
  100/10k/100k/500k dry-run plus interrupted/resumed vault profile.
- `scripts/run_archive_export_profile.sh` — reproducible P3 generated
  100/1,000/5,000-note archive-v2 export, verify, resume, and subset profile.
- `performance/v0.4-p3/` — committed aggregate archive-v2 export timings,
  object/byte counts, peak RSS, and object-budget usage.
- `performance/v0.3-h7/` — committed environment, query-plan, latency, size,
  and peak-RSS evidence from the H7 scale runs.
- `performance/v0.3-h8/` — committed Joplin import duration, batch/resume, and
  memory evidence for the H8 synthetic profiles.
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

- `internal/query/` — bounded backend-neutral expression parser for implicit
  AND, uppercase OR, prefix negation, parentheses, phrases, `category:`/
  `notebook:`, tags, provenance/time fields, literal unknown-colon fallback,
  URLs/hyphens/emoji, and Trash scope.
- `internal/store/sqlite_query.go` — compiles positive text trees to FTS5 with
  relevance keysets and mixed/negated trees to exact parameterized SQL
  predicates with chronological keysets; recursive case-insensitive notebook
  expansion, All-notes alias semantics, emoji fallback, and canonical AST
  cursor binding are shared by every search surface.

## v0.2 task R7 additions (Recoll sidecar)

- `internal/projection/` — outbox-driven Markdown+front-matter filesystem
  projection with bounded draining, durable retry scheduling, exact
  missing/stale/orphan reconciliation, and atomic repair writes.
- `internal/recoll/` — external-process Recoll sidecar: generated config (fields prefixes, `publishedts` range slot, `underscoreasletter`), the embedded from-scratch `notrios_md_handler.py` front-matter handler, `recollindex`/`recollq` invocation, query compilation, and result parsing. GPL boundary: binaries are user-installed and never linked or vendored.
- `internal/httpapi/sidecar_search.go` — merges sidecar-only hits into search results behind the existing API; FTS5 stays authoritative and sidecar failures degrade gracefully.
- `internal/service/` — activates optional Recoll: startup reconciliation and
  index, 30-second bounded drain, 10-minute reconciliation, runtime status,
  cancellation, and stable deduplicated merged search.

## v0.2 task R9 additions (Twitter/X importer)

- `internal/importers/twitter/` — extracted-archive parser (`window.YTD` wrappers, tweets.js/tweet.js, account.js, tweets_media), in-reply-to thread recovery, t.co URL expansion, media-as-resources, hashtag tags, "Twitter" notebook, provenance rows, trashed-note non-resurrection, dry run.
- `cmd/notriosctl` — `import twitter` subcommand.
- `testdata/schemas/` — genson-derived JSON Schemas for the Twitter archive formats (synthetic samples only).

## v0.2 tasks R10/R11 additions (conversation importers)

- `internal/importers/chatgpt/` — ChatGPT `conversations.json` importer (mapping tree, current-node main path, system/tool skip).
- `internal/importers/claude/` — Claude `conversations.json` importer (flat chat_messages, content blocks).
- `cmd/notriosctl` — `import chatgpt` and `import claude` subcommands.
- `testdata/schemas/` — genson-derived schemas for both export formats.

## v0.2 task R12 additions (native archive)

- `internal/archive/` — query-scoped export (notebook paths + emoji preserved, tags, resource bytes), dry-run conflict analysis with rename-suggestion `import-config.json`, validated rename-on-import (refuses names colliding with source-bound notebooks before writing), idempotent plain-note import.
- `cmd/notriosctl` — `export archive` and `import archive` subcommands.

## v0.2 task R13 additions (Wails GUI shell)

- `internal/service/` — shared startup (directories, store, HTTP handler, Recoll sidecar loop) used by `notriosd` and `notrios`.
- `cmd/notrios/` — GUI executable: default (GUI + local service), `-no-gui`, `-gui-only -remote <url>`; `gui_wails.go` (build-tagged Wails app whose asset server routes all webview requests through the service handler or a remote reverse proxy) and `gui_stub.go` (helpful error without the tags). Build with `make gui`.
- `web/src/App.tsx` + `api.ts` + `styles.css` — Notrios sidebar layout: search notebooks first/last anchoring, nested notebook tree with emoji, tags with counts, startup "All notes" with cursor-based Load more, Help-menu event hook.

## v0.2 task R14 additions (GUI themes)

- `web/src/themes.ts` — theme token sets (builtin Light/Dark), custom-theme persistence, per-mode theme selection, applyTheme.
- `web/src/styles.css` — colors tokenized into CSS custom properties; theme-panel styles.
- `web/src/App.tsx` — header light/dark toggle and theme settings panel (create/edit/delete custom themes, assign per mode).

## v0.2 task R15 additions (docs + Help notebook)

- `docs/` — user documentation (published to GitHub Pages and seeded into the Help notebook).
- `scripts/build_docs_site.sh` — Markdown → HTML with a shared template plus a PageFind static search index.
- `.github/workflows/docs.yml` — GitHub Pages deployment for the docs site.
- `internal/helpdocs/` + `notriosctl seed-help` — deterministic Help-notebook seeding (create/update/remove).
- Help notes are read-only at the REST/MCP layers (`guardHelpNote`).

## Documentation completeness pass (2026-07-16)

- New user docs: `docs/installation.md` (build/install guide),
  `docs/troubleshooting.md`, and `docs/operations.md` (remote media,
  resources/GC, resumable imports, Recoll status); all pages are in the site
  nav and deterministic Help seed.
- Rewritten: `ENVIRONMENT_SETUP.md` (contributor guide incl. cleanup/precheck workflow), `PACKAGING.md`, `docs/cli.md` (full subcommand reference), `docs/service.md` (config reference + backup/restore), expanded `docs/import-export.md` (per-source workflows incl. the detailed Joplin RAW procedure) and API guides (curl examples, placeholder-endpoint labeling).
- `notriosctl doctor` performs real diagnostics; importers gained trashed-note re-import guards (Joplin/Obsidian); SQLite opens with a 5s busy timeout; Makefile has full build/clean/precheck targets.
