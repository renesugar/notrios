# Scaffold Creation Plan

This plan governs the process of creating and iterating the project scaffold itself. It is separate from `PLAN.md`, which governs implementation of the product MVP.

## Working-state rule

After each step, the repository must be coherent enough that Codex or a human can resume without relying on chat history. At minimum:

- Required documents exist.
- Active plan and status files are updated.
- Validation commands either pass or document why they cannot yet pass.
- Any uncertainty is written as a decision item or open question.
- A ZIP snapshot can be produced.

## Steps

### Step 1 — Baseline repo and documentation scaffold

Status: done in Step 1 ZIP

Create the initial repository skeleton, root planning docs, agent instructions, plan-history convention, scaffold status files, code stubs, API placeholders, prompt templates, and starter skills.

### Step 2 — Review and repair scaffold content

Status: done in Step 2 ZIP

Review all documents against the copied conversation context and user corrections. Tighten terminology, remove contradictions, and add missing design decisions.

### Step 3 — Expand API and schema detail

Status: done in Step 3 ZIP

Turn the initial API and database migration into a more precise MVP contract: document CRUD, resources, revisions, link graph, search, media policy, and publish-plan endpoints.

Changed files include `API_SPEC.md`, `api/openapi.yaml`, `api/mcp-tools.md`, `DATABASE_SCHEMA.md`, `migrations/0001_initial.sql`, `internal/api/types.go`, and stub HTTP handlers/tests.

### Step 4 — Add first runnable persistence slice

Status: completed

Added SQLite initialization, embedded migrations, default collection bootstrap, document create/read/body endpoints, FTS5 search, and tests. See `SCAFFOLD_STEP4_REPORT.md` and `plans/v0.1/004-runnable-persistence-slice.md`.

### Step 5 — Add built-in UI scaffold integration

Status: completed

The React UI reads `/api/v1/status`, creates persisted notes, searches through `/api/v1/search`, and opens selected documents. See `SCAFFOLD_STEP5_REPORT.md` and `plans/v0.1/005-builtin-ui-integration.md`.

### Step 6 — Prepare Codex handoff

Status: completed

`PROMPT.md`, `AGENTS.md`, `PLAN.md`, `ROADMAP.md`, skills, handoff notes, and container-limit notes were aligned. See `SCAFFOLD_STEP6_REPORT.md` and `plans/v0.1/006-codex-handoff.md`.

## MVP implementation snapshots

After scaffold creation, continue archiving MVP slices in the same plan-history convention.

### MVP Task 1 — Service persistence foundation

Status: completed

Configuration loading, storage directory creation, schema-version reporting, and enriched `/api/v1/status` have been implemented. See `MVP_TASK1_REPORT.md` and `plans/v0.1/007-mvp-service-persistence-foundation.md`.

## Plan archive convention

Implemented plans should be archived under:

```text
plans/v<major>.<minor>/<NNN>-<milestone-or-task-slug>.md
```

Examples:

```text
plans/v0.1/001-scaffold-baseline.md
plans/v0.1/002-sqlite-migration-slice.md
plans/v0.2/001-joplin-raw-importer.md
```

Each archived plan must include:

- original goal;
- final status;
- implementation summary;
- validation evidence;
- model(s) used;
- follow-up tasks.

### MVP Task 2 — Document CRUD and revisions

Status: completed

Document update, patch, soft-delete/trash, revision listing, revision read, revision restore, optimistic concurrency, and UI save-revision integration have been implemented. See `MVP_TASK2_REPORT.md` and `plans/v0.1/008-mvp-document-crud-revisions.md`.

### MVP Task 4 — Resource store

Status: completed

Streaming resource upload/download, SHA-256 content-addressed blobs, resource metadata, document-resource attach/list/detach, safe referenced-resource deletion, and minimal UI upload/download support have been implemented. See `MVP_TASK4_REPORT.md` and `plans/v0.1/009-mvp-resource-store.md`.


### MVP Task 5 — Markdown link parser and graph

Status: completed

The conservative Markdown/Obsidian/app-URI parser, transactional `document_links` persistence, outgoing/backlink endpoint, graph endpoint, schema version 4, tests, and UI link/backlink display have been implemented. See `MVP_TASK5_REPORT.md` and `plans/v0.1/010-mvp-markdown-link-parser-graph.md`.

### MVP Task 6 — Built-in React UI MVP

Status: completed

`md-editor-rt` editing/preview, app-owned `document://` preview navigation, `resource://` preview downloads, editor image upload to local resources, and `web/dist` static serving from `notesd` have been implemented. See `MVP_TASK6_REPORT.md` and `plans/v0.1/011-mvp-builtin-react-ui.md`.

### MVP Task 7 — MCP MVP

Status: completed

Added the first read-only MCP server surface around the existing shared service semantics. The current dependency-free JSON-RPC adapter exposes `initialize`, `tools/list`, and `tools/call` at `/mcp` with `list_collections`, `search_documents`, `get_document`, `get_documents`, `list_document_links`, `list_document_resources`, and `get_document_outline`.

### MVP Task 8 — Joplin RAW importer MVP

Status: completed

Added a tolerant Joplin RAW importer and `notesctl import joplin-raw`. The importer parses notes/folders/resources/tags/note-tag joins, imports notes/resources with deterministic source-derived IDs, rewrites `:/<id>` links, preserves selected metadata in frontmatter, attaches referenced resources, and verifies searchability with a fixture test. See `MVP_TASK8_REPORT.md` and `plans/v0.1/013-mvp-joplin-raw-importer.md`.


### MVP Task 9 — Obsidian importer MVP

Status: completed

Added a conservative Obsidian vault importer and `notesctl import obsidian`. The importer scans Markdown notes and local assets, imports deterministic source-path IDs, preserves and augments frontmatter with source metadata, attaches referenced local assets, preserves Wikilinks/embeds/unresolved links, and refreshes link indexes after all notes/resources exist. See `MVP_TASK9_REPORT.md` and `plans/v0.1/014-mvp-obsidian-importer.md`.


## MVP Task 10 completion

MVP release hardening is complete. The v0.1 plan has been archived, release packaging/check scripts have been added, and the next draft plan has been generated from `ROADMAP.md` v0.2. Ask the user before starting the v0.2 plan.
