# Plan Status

## Current phase

v0.2 Notrios redesign foundation (`PLAN.md`). Tasks **R1 (documentation redesign)**, **R2 (code rename + Apache-2.0 license)**, and **R3 (schema v5 — notebooks, tags, search notebooks)** are completed; next task is **R4 (schema v5 — source provenance and threads)** — ask the user before starting it.

## Current working state

- The v0.1 MVP implementation is functionally unchanged and passing: `notriosd` service (renamed from `notesd` in R2), SQLite schema v4, document CRUD/revisions/FTS5 search, content-addressed resources, links/graph, React UI, read-only MCP at `/mcp`, Joplin RAW and Obsidian importers, release scripts. See `CODING_CLIENT_HANDOFF.md` for the full capability list.
- The repository is now a git repo: `main` holds the pre-redesign baseline commit; active work is on `develop`.
- R1 changes (docs only):
  - Project renamed to **Notrios** in all living design docs; target repo `github.com/renesugar/notrios`; binaries to be renamed `notriosd`/`notriosctl` in R2.
  - `CODEX_HANDOFF.md` → `CODING_CLIENT_HANDOFF.md` (generalized to all coding agents); `CLAUDE.md` added pointing to `AGENTS.md`; `skills/codex-handoff` → `skills/agent-handoff`; `prompts/start_codex_from_handoff.md` → `prompts/start_agent_from_handoff.md`.
  - **Recoll replaces sist2** in the design: new `RECOLL_INTEGRATION.md` (field mapping, from-scratch front-matter handler, external-process/GPL licensing boundary), `SEARCH_QUERY_LANGUAGE.md` (query operators, timestamp semantics, FTS5/Recoll translation).
  - New product model docs: `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md` (nested notebooks, emoji icons, tags with counts, "All notes"/"Trash"/"Help" search notebooks, case-insensitive names, trash/purge rules) and `DOCS_SITE.md` (GitHub Pages + PageFind + Help notebook).
  - `PLAN.md` replaced with the R1–R16 redesign plan; the former v0.2 media-hardening draft moved to the v0.3 milestone in `ROADMAP.md` (archived copy at `plans/v0.2/001-import-resource-media-hardening.md`).
  - `API_SPEC.md` gained planned notebook/tag/trash routes, the full-client goal, and the expanded MCP tool list; `DATABASE_SCHEMA.md` gained the planned schema v5 section (notebooks, tags, search notebooks, source provenance/threads, deletion rules).
  - `LICENSE_PENDING.md` narrowed to MIT or Apache-2.0 (user decision pending).
  - `scripts/check_required_files.py` updated for renamed/new docs.
- Historical reports (`SCAFFOLD_STEP*`, `MVP_TASK*`, `plans/`, log files) intentionally keep old names as records.
- R2 changes (code rename + license):
  - Go module path is `github.com/renesugar/notrios`; binaries are `cmd/notriosd` and `cmd/notriosctl`.
  - Config section `sist2` → `search_sidecar` (keys `enabled`, `binary` [default `recollindex`], `index_dir` [default `./data/search-index`]); Go types `Sist2Config` → `SearchSidecarConfig`; status JSON `sist2_index_dir` → `search_sidecar_index_dir`; capability flag `sist2` → `search_sidecar`; OpenAPI collection kind `sist2` → `sidecar_indexed`.
  - `LICENSE` is Apache-2.0 (user decision); `LICENSE_PENDING.md` removed; `LICENSE` added to required files.
  - Makefile, Taskfile, smoke/packaging scripts, web package name (`notrios-web`), and MCP server name (`notrios`) updated.

- R3 changes (schema v5, store layer): notebooks/tags/note_tags/search_notebooks tables, `documents.notebook_id` (backfilled, defaults to `nb_notes`), builtin rows ("Notes", read-only "Help", "All notes" first, "Trash" last with reserved query `is:trashed`), notebook CRUD with case-insensitive sibling-unique names and recursive delete-to-trash, tag counts, search-notebook lifecycle, trash list/restore/purge, v4→v5 upgrade shim, 9 new store tests. See `plans/v0.2/004-schema-v5-notebooks.md`. REST/MCP exposure is task R5.

## Next suggested step

Task R4: schema v5 source provenance and threads — source-object tables (source_system, external IDs, author/author_id, thread_id/reply_to, URL, timestamps), purge guard for externally-sourced notes, provenance backfill from importer frontmatter, thread-order store queries + tests.

## Validation

Validation set used for R1/R2:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
```

Full set (unchanged) for code tasks:

```bash
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

## Open blockers

- Long-term SQLite driver choice remains open (current: local cgo/libsqlite3 adapter).
- Config parser supports only the documented example-config subset.
- The MCP MVP adapter is dependency-free; official MCP Go SDK adoption is future work.
- Importers verified on synthetic fixtures only; verify against real Joplin RAW exports and Obsidian vaults before large migrations.
- Recoll integration is design-only (R7); Wails GUI is design-only (R13/R14); docs site is design-only (R15).
