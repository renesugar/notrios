# Plan: v0.2 — Notrios redesign foundation

Status: active. Tasks R1–R11 are complete; next task is R12 (query-scoped export and import-with-dry-run). Ask the user before starting each task.

This plan supersedes the earlier v0.2 draft ("Import, Resource, and Media Hardening", archived at `plans/v0.2/001-import-resource-media-hardening.md`). The media-hardening items remain on the roadmap; they are re-sequenced behind the redesign items below.

## Why the redesign

The project direction changed after the v0.1 MVP:

1. The project is now named **Notrios** and will live at `https://github.com/renesugar/notrios`.
2. The service is renamed `notesd` → `notriosd` (and `notesctl` → `notriosctl`).
3. **Recoll replaces sist2** as the derived search/extraction sidecar (see `RECOLL_INTEGRATION.md`).
4. The **built-in GUI is Go/Wails** and is part of the first released version, with `-no-gui` and `-gui-only` modes (see `UI_DESIGN.md`).
5. The product model gains **notebooks (nested), tags, search notebooks** ("All notes", "Trash", "Help", user-defined query notebooks) — see `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`.
6. The schema must support **Joplin, Obsidian, Twitter/X, ChatGPT, and Claude** sources, including conversation threads (author, thread ID, reply-to, post URL).
7. The MCP/REST API must be complete enough to build a **full third-party client** (C++/Qt, Go/Wails, Rust/Tauri) — reference surfaces: joplin-mcp, obsidian-mcp-connector, obsidian-local-rest-api.
8. A **query-language adapter** (`notebook:`, `tag:`, `author:`, `since:`, `until:`, …) fronts both FTS5 and Recoll (see `SEARCH_QUERY_LANGUAGE.md`).
9. Project code must be releasable under **MIT or Apache 2.0**; GPL components (Recoll/Xapian) stay external sidecar processes and are never linked or vendored.
10. Documentation becomes a **GitHub Pages site with PageFind search**, and the same content seeds the built-in read-only "Help" notebook (see `DOCS_SITE.md`).

## Working-state rule

Every task must leave the project in a working state. Update `agent/PLAN_STATUS.md`, append to `agent/ATTEMPT_LOG.jsonl` and `agent/MODEL_LOG.jsonl`, commit with git, and archive completed slices under `plans/v0.2/`. Ask the user before starting the next task.

## Tasks

### R1. Documentation redesign and rebrand pass (docs only) — COMPLETED

- Rename the project to Notrios in all living design docs; historical reports (`SCAFFOLD_STEP*`, `MVP_TASK*`, `plans/`) are left as records.
- Rename `CODEX_HANDOFF.md` → `CODING_CLIENT_HANDOFF.md`; generalize it to all coding agents (codex, claude, aider, etc.).
- Add `CLAUDE.md` pointing to `AGENTS.md`; generalize `AGENTS.md`.
- Replace sist2 with Recoll in living design docs; add `RECOLL_INTEGRATION.md`.
- Add `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `DOCS_SITE.md`.
- Update `ROADMAP.md` (Wails GUI moves into the first released version; Recoll milestone replaces sist2 milestone).
- Update `README.md`, `SYSTEM_ARCHITECTURE.md`, `API_SPEC.md`, `DATABASE_SCHEMA.md`, `UI_DESIGN.md`, `FEATURE_MATRIX.md`, `IMPORT_EXPORT_POLICY.md`, `CONTEXT_MAP.md`, `PROMPT.md`, prompts/skills, and `scripts/check_required_files.py`.
- Record license direction in `LICENSE_PENDING.md` (resolved in R2: Apache-2.0).

Working state: all validation scripts pass; docs consistently describe the Notrios design; code still uses old names (renamed in R2).

### R2. Code rename and license — COMPLETED (license: Apache-2.0)

- Go module path `example.com/notes-companion` → `github.com/renesugar/notrios`.
- `cmd/notesd` → `cmd/notriosd`; `cmd/notesctl` → `cmd/notriosctl`.
- Rename `sist2`-derived config keys/fields (`sist2_index_dir` → `search_index_dir`, capability flags, status output, `web/src/api.ts` types) to search-sidecar-neutral names.
- Replace `LICENSE_PENDING.md` with the chosen license (user selected **Apache-2.0**).
- Update scripts, Makefile/Taskfile, CI, packaging, and web references.

Working state: `go test ./...`, scaffold checks, web typecheck/build, and smoke scripts pass under the new names.

### R3. Schema v5 — notebooks, tags, and search notebooks — COMPLETED

- `notebooks` table: nested (parent ID), stable IDs, optional emoji icon, position/sort metadata.
- Notebook names case-insensitively unique among siblings; enforce at store layer (`NOCASE` unique index).
- `tags` and `note_tags` tables (with counts derivable per tag).
- Document→notebook membership (every managed note lives in exactly one notebook; default "Notes" notebook bootstrap).
- `search_notebooks` table: name, emoji, query string, `builtin` flag, fixed sort anchors. Bootstrap builtin rows: "All notes" (first, undeletable), "Trash" (last, undeletable), "Help" (undeletable, read-only content).
- Trash semantics: soft-deleted notes appear only via the Trash search notebook and are excluded from all other queries; undelete restores visibility; permanent delete allowed only for locally-stored notes.
- Migration + store/API tests.

Working state: schema migrates from v4; existing CRUD/search/import tests pass with default notebook membership.

### R4. Schema v5 — source provenance and threads — COMPLETED (as schema v6)

- Source-object tables recording `source_system` (joplin, obsidian, twitter, chatgpt, claude, local), external IDs, author display name, canonical author ID, `thread_id`, `reply_to`, source URL, and timestamps — sufficient to recover Twitter/X conversation threads and ChatGPT/Claude conversations.
- Notes from external sources marked deleted are excluded from queries but never permanently deleted (only local notes can be purged from Trash).
- Backfill provenance from the frontmatter the Joplin/Obsidian importers already write.

Working state: importers record provenance rows; thread queries work in store tests.

### R5. Notebooks/tags/trash REST + MCP API — COMPLETED (write MCP tools deferred to R8)

- REST: notebook CRUD (create/rename/move/delete, emoji, nested tree), tag list with counts, note↔notebook move, note↔tag assignment, trash list/undelete/purge, search-notebook CRUD with builtin protection.
- MCP: read tools (`list_notebooks`, `get_notebook_tree`, `list_tags`), then scope-gated write tools.
- `notebook:"name"` and `tag:"name"` filters in search (case-insensitive notebook match).
- OpenAPI + `api/mcp-tools.md` updates.

Working state: a client can reproduce the sidebar (notebooks tree + tag counts) and trash flows purely via REST.

### R6. Query-language adapter — COMPLETED

- Parser for `notebook:`, `tag:`, `author:`, `authorid:`, `title:`, `since:`, `until:`, quoted phrases, implicit AND.
- ISO 8601 timestamps normalized to UTC; date-only `until:` means end-of-day; document time-only semantics.
- Compile to FTS5 + SQL filters now; keep a Recoll compilation target for R7 (`publishedts:lower..upper` ranges).
- Incremental/cursor search results for scrolling UIs (extend existing cursor rules in `API_SPEC.md`).

Working state: search notebooks (including "All notes" and user query notebooks like `tag:todo`) execute through the adapter.

### R7. Recoll integration (replaces sist2) — COMPLETED

- Managed filesystem projection + outbox reuse (as designed for sist2).
- Generate Recoll config (`fields` file with `tag`, `authorid`, `publishedts`, `noteid`, etc.; `underscoreasletter` guidance).
- Write a **from-scratch** Markdown front-matter input handler (full YAML/TOML) emitting the field mapping in `RECOLL_INTEGRATION.md`; do not derive from Recoll's GPL `rclmd.py`.
- Adapter invokes `recollindex`/`recollq` (or the Recoll Python API in a helper subprocess) — external process only, keeping MIT/Apache licensing clean.
- Merge Recoll hits with FTS5 hits behind the existing search API; Recoll absence degrades gracefully to FTS5.

Working state: with Recoll installed, field queries (`tag:`, `author:`, `publishedts` ranges) work over the projection; without it, FTS5 search still works.

### R8. Full-client MCP/REST surface — COMPLETED

- Note ops: append/prepend, server-side string replace (extend existing PATCH), line-range read, in-note search, sections/outline (parity with joplin-mcp tool list).
- Notebook trees with notes, scoped trees, all-notes tree.
- MCP write tools gated by scopes and revision preconditions.
- Verify a third-party client (native C++/Qt, Go/Wails, Rust/Tauri) could implement the full GUI feature list from `UI_DESIGN.md` using only this API; document gaps and close them.

### R9. Twitter/X archive importer — COMPLETED

- Parse Twitter/X archive exports (reference: doggy8088/x-archive-parser); recover conversation threads (author, author ID, thread ID, reply-to, post URLs), media resources, and a "Twitter" notebook.
- Derive JSON schemas from sample data with genson (`uvx genson`) into `testdata/schemas/`; synthetic fixtures only — no private data in the repo.

### R10. ChatGPT conversations importer — COMPLETED

- Parse ChatGPT exports (references: temnoon/openai_export_parser, slyubarskiy/chatgpt-conversation-extractor); one note per conversation or per message with thread provenance; synthetic fixtures.

### R11. Claude conversations importer — COMPLETED

- Parse Claude export JSON with the same provenance model; synthetic fixtures.

### R12. Query-scoped export and import-with-dry-run

- Export query results (or whole notebooks) preserving notebook structure; exported external-source notes become plain notes on re-import.
- Import dry run reports notebook-name conflicts and writes an import configuration file for rename-on-import; import validates that config against existing source-bound notebooks before writing.
- Support Joplin RAW, Obsidian vault, and the native archive format as import sources for notebook creation/population.

### R13. Wails GUI shell

- New `notrios` executable: Go/Wails GUI embedding the service. Modes: default (GUI + local service), `-no-gui` (headless service, same behavior as `notriosd`), `-gui-only` (GUI as pure REST client, optionally against a remote service URL).
- Layout per `UI_DESIGN.md`: menu bar (File/Edit/View/Help); left sidebar with notebooks tree ("All notes" first, "Trash" last, emoji icons, nested notebooks) and tag list with counts below; search box + incremental result list; Markdown editor pane; Markdown preview pane.
- Startup view: "All notes" search notebook with cursor-based incremental loading.

### R14. GUI themes

- Light/dark toggle on the main window.
- User-defined custom themes, selectable as the active light and dark theme.

### R15. Help notebook and documentation site

- Markdown user/reference docs for the service and built-in client under `docs/`.
- GitHub Pages site built from `docs/` with PageFind search.
- Seed the protected read-only "Help" notebook from the same content for offline use; `notebook:help` searches work.

### R16. GitHub release preparation

- Verify licenses of all dependencies are MIT/Apache-2.0 compatible.
- CI, branch protection notes, and the push sequence:

```bash
git remote add origin https://github.com/renesugar/notrios.git
git branch -M main
git push -u origin main
```

## Validation

Until a task adds more specific checks:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

## Scope control

Remote-media localization, quarantine, perceptual hashes, resource GC (the superseded v0.2 draft), Quartz publishing, sync, and CodeMirror 6 migration remain roadmap items after this plan unless the user changes priorities.
