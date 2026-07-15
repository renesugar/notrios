# Coding Client Handoff

This handoff applies to any coding agent or client continuing this project (Codex, Claude, aider, swival.dev, etc. — formerly `CODEX_HANDOFF.md`). The repository is designed so an agent can continue from repository files alone.

Current phase: the v0.1 MVP is complete and the project is being redesigned as **Notrios** — see `PLAN.md` ("v0.2 — Notrios redesign foundation"). Complete one plan task at a time, leave the repo in a working state, and ask the user before starting the next task.

## First files to read

1. `AGENTS.md`
2. `README.md`
3. `PLAN.md`
4. `ROADMAP.md`
5. `CODING_CLIENT_HANDOFF.md`
6. `SYSTEM_ARCHITECTURE.md`
7. `API_SPEC.md`
8. `DATABASE_SCHEMA.md`
9. `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`, `RECOLL_INTEGRATION.md`, `DOCS_SITE.md`
10. `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`, `CONTEXT_MAP.md`
11. `agent/PLAN_STATUS.md`, `agent/ATTEMPT_LOG.jsonl`, `agent/MODEL_LOG.jsonl`
12. Release/history context when needed: `MVP_RELEASE_REPORT.md`, `RELEASE_CHECKLIST.md`, `SECURITY_REVIEW.md`, `PACKAGING.md`

## Current state

The completed v0.1 MVP supports:

- `notesd` local HTTP service (rename to `notriosd` is plan task R2);
- SQLite database creation, migration bootstrap, and default managed collection bootstrap;
- document create/read/update/patch/soft-delete;
- revision list/read/restore;
- SQLite FTS5 search over current non-deleted notes;
- content-addressed resource upload, metadata, content streaming, attachment, detachment, and safe delete;
- Markdown link/backlink parsing and graph slices;
- React/Vite UI using `md-editor-rt` with document/resource link routing;
- read-only MCP MVP endpoint at `/mcp`;
- `notesctl import joplin-raw` and `notesctl import obsidian`;
- generated-dataset smoke/performance tests;
- release packaging and ZIP verification scripts.

The redesign (naming, Recoll, notebooks/search notebooks, Wails GUI, importers, docs site) is specified in `PLAN.md` and the design docs listed above.

## Validation commands

Run these before committing any task:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

## Git workflow

The repository is a local git repo: `main` holds the baseline, active work happens on `develop`. Commit each completed working-state slice. When ready for the public repository:

```bash
git remote add origin https://github.com/renesugar/notrios.git
git branch -M main
git push -u origin main
```

Before pushing: finish plan task R2 (module path `github.com/renesugar/notrios`, binary renames), select the license (MIT or Apache-2.0), and confirm CI installs Go, Node, npm dependencies, and SQLite development headers.

## Important constraints to preserve

- SQLite is the canonical managed-note database.
- Recoll is a derived, optional, external search sidecar — never canonical storage, never linked/vendored (GPL; see `RECOLL_INTEGRATION.md`).
- Project code must remain compatible with an MIT or Apache-2.0 license.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown and downloaded resources are untrusted; preview HTML must be sanitized.
- Remote media localization must go through media policy and quarantine checks.
- Every task must leave the repo in a working state; archive completed plans under `plans/`.

## Environment limitations inherited from the scaffold

The scaffold was created in a restricted container. Still-open consequences:

1. The SQLite store uses a small local cgo adapter over system `libsqlite3`; the long-term driver choice is open.
2. The Go module path is still `example.com/notes-companion` (fixed in task R2).
3. No final license file yet (`LICENSE_PENDING.md`; user decision between MIT and Apache-2.0).
4. The MCP adapter is dependency-free; the official MCP Go SDK can replace it later without changing tool semantics.
5. The UI was typechecked/built but not browser-tested (no Playwright yet).
6. Importers were tested on synthetic fixtures only — verify against real Joplin RAW exports and Obsidian vaults before large migrations; never commit private datasets.
