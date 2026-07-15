# Plan Status

## Current phase

v0.2 Notrios redesign foundation (`PLAN.md`). Task **R1 (documentation redesign and rebrand pass) completed**; next task is **R2 (code rename and license)** — ask the user before starting it.

## Current working state

- The v0.1 MVP implementation is unchanged and passing: `notesd` service, SQLite schema v4, document CRUD/revisions/FTS5 search, content-addressed resources, links/graph, React UI, read-only MCP at `/mcp`, Joplin RAW and Obsidian importers, release scripts. See `CODING_CLIENT_HANDOFF.md` for the full capability list.
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
- Code still uses old names (`notesd`, `notesctl`, `example.com/notes-companion`, `sist2_*` config keys) until R2.

## Next suggested step

Task R2: rename Go module path to `github.com/renesugar/notrios`, `cmd/notesd` → `cmd/notriosd`, `cmd/notesctl` → `cmd/notriosctl`, neutralize `sist2` config/status naming, and add the chosen license (user must pick MIT or Apache-2.0).

## Validation

Validation set for R1 (docs-only change):

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

- License choice: MIT or Apache-2.0 (user decision; blocks R2 completion and public push).
- Long-term SQLite driver choice remains open (current: local cgo/libsqlite3 adapter).
- Config parser supports only the documented example-config subset.
- The MCP MVP adapter is dependency-free; official MCP Go SDK adoption is future work.
- Importers verified on synthetic fixtures only; verify against real Joplin RAW exports and Obsidian vaults before large migrations.
- Recoll integration is design-only (R7); Wails GUI is design-only (R13/R14); docs site is design-only (R15).
