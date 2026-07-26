# Coding Client Handoff

This handoff applies to any coding agent or client continuing this project (Codex, Claude, aider, swival.dev, etc. — formerly `CODEX_HANDOFF.md`). The repository is designed so an agent can continue from repository files alone.

Current phase: v0.1 and v0.2 are complete; active work is `PLAN.md` v0.3
import/resource/media/large-library hardening. H1–H6 are archived;
**H7 (large-library pagination and performance baseline) is next**. The 2026-07-26
plan review inserted H7 keyset/scale work and renumbered importer/Recoll/wrap
tasks to H8–H11. Complete one task at a time and ask before the next.

## First files to read

1. `AGENTS.md`
2. `README.md`
3. `PLAN.md`
4. `ROADMAP.md`
5. `CODING_CLIENT_HANDOFF.md`
6. `SYSTEM_ARCHITECTURE.md`
7. `API_SPEC.md`
8. `DATABASE_SCHEMA.md`
9. `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`, `SEARCH_QUERY_LANGUAGE.md`,
   `RECOLL_INTEGRATION.md`, `SYNCHRONIZATION.md`, `DOCS_SITE.md`
10. `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`, `CONTEXT_MAP.md`
11. `agent/PLAN_STATUS.md`, `agent/ATTEMPT_LOG.jsonl`, `agent/MODEL_LOG.jsonl`
12. Release/history context when needed: `plans/mvp/MVP_RELEASE_REPORT.md`, `RELEASE_CHECKLIST.md`, `SECURITY_REVIEW.md`, `PACKAGING.md`

## Current state

The completed v0.1 MVP supports:

- `notriosd` local HTTP service;
- SQLite database creation, migration bootstrap, and default managed collection bootstrap;
- document create/read/update/patch/soft-delete;
- revision list/read/restore;
- SQLite FTS5 search over current non-deleted notes;
- content-addressed resource upload, metadata, content streaming, attachment, detachment, and safe delete;
- Markdown link/backlink parsing and graph slices;
- React/Vite UI using `md-editor-rt` with document/resource link routing;
- dependency-free MCP endpoint at `/mcp` (read-only default; editor writes);
- `notriosctl import joplin-raw` and `notriosctl import obsidian`;
- generated-dataset smoke/performance tests;
- release packaging and ZIP verification scripts.

The completed v0.2 redesign added notebooks/tags/search notebooks, source
provenance and threads, query language, optional Recoll, five importers, native
archive v1 interchange, Wails v2 GUI, and docs site. v0.3 so far added the
remote-media policy/scan/quarantine/localization surfaces, exact duplicate and
unreferenced resource reports, per-notebook resource usage, and an optional
review-only perceptual hook that is inert by default. H6 added schema-v8
resource retention state, configurable local retention, dry-run-first CLI
garbage collection, a read-only REST report, explicit confirmation for
permanent REST deletion, and a future sync-aware retention gate. Archive v1 is
not a full backup; v0.4 native archive v2 is planned as the
full-snapshot/container layer reused by v0.7 sync. See `agent/PLAN_STATUS.md`.

## Validation commands

Run these before committing any task:

```bash
go vet ./... && go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build && npm test
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
```

GUI-affecting tasks also build with `make gui`; layout changes additionally run `scripts/verify_layout_resize.py` under Xvfb/Openbox (see `TESTING_POLICY.md`).

## Git workflow and state (reviewed 2026-07-26)

`main` takes reviewed merges; active work happens on `develop`. Commit each completed working-state slice; pushing to GitHub is the **user's step**.

State at review start (verify again before acting):

- `origin` is configured (`https://github.com/renesugar/notrios.git`).
- `develop` review base was `26b0925`; it has no configured upstream.
- local `main` was `265ef4e` tracking `origin/main`.
- The review commits are local only. The user explicitly prohibited a GitHub
  push for this session.

Commit-message convention: each agent ends commit messages with its own `Co-Authored-By:` trailer, and appends its model to `agent/MODEL_LOG.jsonl` at session start (see `AGENTS.md`).

## Important constraints to preserve

- SQLite is the canonical managed-note database.
- Recoll is a derived, optional, external search sidecar — never canonical storage, never linked/vendored (GPL; see `RECOLL_INTEGRATION.md`).
- Project code must remain compatible with an MIT or Apache-2.0 license.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown and downloaded resources are untrusted; preview HTML must be sanitized.
- Remote media localization must go through media policy and quarantine checks.
- Exact SHA-256 is the only deduplication identity. Perceptual hashes are
  review suggestions only and no algorithm ships by default.
- Resource garbage collection must remain dry-run first, transactionally
  recheck references on apply, and pass future synchronization acknowledgement
  policy through `store.RetentionGate`.
- Every task must leave the repo in a working state; archive completed plans under `plans/`.
- Unbounded paging uses keysets/snapshots, not hidden offsets.
- Sync must follow `SYNCHRONIZATION.md`: canonical local stores, immutable
  operations/objects, REST/folder/rclone transports, and acknowledgement-gated
  retention. `rclone sync` is not the merge algorithm.

## Local environment notes

Facts about the development machine that no other document records:

- `./data/` in the repo root holds a throwaway development database (`notes.sqlite` plus `assets/`, `quarantine/`, `projections/`, `search-index/`) created by ad-hoc service and GUI smoke runs. It is gitignored and safe to delete; the service recreates it on startup.
- `web/dist/` is gitignored; run `cd web && npm ci && npm run build` after a fresh clone (CI and `scripts/package_release.sh` build it too).
- `scripts/verify_layout_resize.py` needs Python `playwright` plus `xdotool`, `Xvfb`, and `openbox` (all installed system-wide here, but **no Python venv is committed** — create one with `python3 -m venv … && pip install playwright`; it can drive the system `google-chrome`, so no browser download is required).
- `npm test` under Node 22 prints a harmless `ExperimentalWarning: localStorage` — the real polyfill lives in `web/src/test/setup.ts` (explained in its comments).
- MCP write-tool tests enable the editor profile by setting `s.config.MCP.DefaultProfile = "editor"` directly on the server struct — a test-only shortcut (see `internal/httpapi/remote_media_test.go`).
- The GUI before/after screenshots from the v0.2 conformance pass live in `/home/renes/prompts/` (outside the repo, intentionally uncommitted — transient browser-automation output is never committed).
- All verification servers, Xvfb displays, and browser sessions from prior agent sessions are stopped; session scratchpads lived under `/tmp` and are disposable.

## Environment limitations inherited from the scaffold

The scaffold was created in a restricted container. Still-open consequences:

1. The SQLite store uses a small local cgo adapter over system `libsqlite3`; the long-term driver choice is open.
2. The MCP adapter is dependency-free; the official MCP Go SDK can replace it later without changing tool semantics.
3. Importers were tested on synthetic fixtures only — verify against real Joplin RAW exports and Obsidian vaults before large migrations; never commit private datasets.

(The formerly open "no browser testing" limitation is resolved: the GUI is browser-verified via Playwright, vitest/RTL covers the workspace, and `scripts/verify_layout_resize.py` covers native window resizing.)
