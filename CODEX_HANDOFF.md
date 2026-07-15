# Codex Handoff

This repository is ready for Codex to continue from repository files alone. The v0.1 MVP plan is complete, release-hardening has been added, and `PLAN.md` now contains the next v0.2 draft plan. Do not begin the v0.2 plan until the user confirms.

## First files to read

1. `AGENTS.md`
2. `README.md`
3. `PLAN.md`
4. `ROADMAP.md`
5. `CODEX_HANDOFF.md`
6. `MVP_RELEASE_REPORT.md`
7. `RELEASE_CHECKLIST.md`
8. `SECURITY_REVIEW.md`
9. `PACKAGING.md`
10. `SYSTEM_ARCHITECTURE.md`
11. `API_SPEC.md`
12. `DATABASE_SCHEMA.md`
13. `TESTING_POLICY.md`
14. `ENVIRONMENT_SETUP.md`
15. `CONTEXT_MAP.md`
16. `agent/PLAN_STATUS.md`
17. `agent/ATTEMPT_LOG.jsonl`
18. `agent/MODEL_LOG.jsonl`

## Current state

The completed v0.1 MVP supports:

- `notesd` local HTTP service;
- SQLite database creation, migration bootstrap, and default managed collection bootstrap;
- document create/read/update/patch/soft-delete;
- revision list/read/restore;
- SQLite FTS5 search over current non-deleted notes;
- content-addressed resource upload, metadata, content streaming, attachment, detachment, and safe delete;
- Markdown link/backlink parsing and graph slices;
- React/Vite UI using `md-editor-rt` with document/resource link routing;
- read-only MCP MVP endpoint at `/mcp`;
- `notesctl import joplin-raw`;
- `notesctl import obsidian`;
- generated-dataset smoke/performance tests;
- release packaging and ZIP verification scripts.

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

To build and verify a release ZIP:

```bash
bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp.zip
python3 scripts/check_release_zip.py /tmp/notes-companion-v0.1.0-mvp.zip
```

## Git setup checklist

Use a development branch until the user approves merging to `main`:

```bash
git init
git add .
git commit -m "Initial Notes Companion MVP"
git branch -M main
git checkout -b develop
```

Before pushing:

1. Change the Go module path from `example.com/notes-companion` if the repository name is known.
2. Select a license and replace `LICENSE_PENDING.md` with the chosen license file.
3. Confirm CI can install Go, Node, npm dependencies, SQLite development headers, and any future browser-test dependencies.
4. Push `develop` first.
5. Protect `main` and require CI before merges.

## Important constraints to preserve

- SQLite is the canonical managed-note database.
- sist2 is a derived extraction/search sidecar, not canonical storage.
- MCP must not expose raw SQL or arbitrary filesystem operations.
- Imported Markdown and downloaded resources are untrusted.
- Remote media localization must go through media policy and quarantine checks.
- Preview HTML must be sanitized.
- Every task must leave the repo in a working state.
- Completed plans must be archived under `plans/`.

## Container/scaffold limitations that affected choices

This scaffold was created in a restricted container, not a full developer workstation. The following limitations affected implementation choices:

1. **No reliable external network for code tooling.** Go module downloads and package discovery were avoided where possible. The SQLite store therefore uses a small local cgo adapter over system `libsqlite3` instead of immediately choosing `modernc.org/sqlite` or `mattn/go-sqlite3`.
2. **No real Gitea/GitHub remote.** Git branch protection, CI settings, GitHub Actions secrets, and actual repository metadata could only be documented, not verified against a remote.
3. **No real user export datasets.** Joplin RAW, Obsidian, Twitter/X, ChatGPT, Claude, image/PDF, and large-scale performance fixtures are described but not included. Do not commit private datasets.
4. **No real sist2/Quartz integration was installed or exercised.** The official MCP Go SDK was not installed or exercised in this restricted scaffold environment.
5. **No browser automation stack was installed.** The UI was typechecked and built, but not tested with Playwright or an actual browser flow.
6. **No final license or production module path was selected.** These remain explicit open questions.

## Things Codex may want to change early in a less restricted environment

1. Decide and pin the long-term SQLite driver.
2. Replace `example.com/notes-companion` with the actual module path before public commits.
3. Select and add the project license.
4. Replace the local dependency-free MCP MVP adapter with the official MCP Go SDK once dependency access/policy is available, preserving the read-only tool names and safety boundaries.
5. Run `npm ci`, `npm audit`, and confirm `md-editor-rt` compatibility in the target Node environment.
6. Add Playwright or another UI smoke-test stack with `xvfb` for CI.
7. Install and test sist2 and Quartz in an integration environment rather than relying on documentation alone.
8. Add real, private test runs against Joplin RAW exports and Obsidian vaults without committing those datasets.
9. Review generated OpenAPI and Go API types together, then decide whether to introduce code generation.
10. Decide whether config parsing should use a dependency such as `gopkg.in/yaml.v3`, a tiny local YAML subset, or JSON/TOML instead.
