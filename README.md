# Notrios

Notrios (formerly "Notes Companion") is a local-first note-taking, search, import, and publishing system for very large Markdown and document collections.
It combines a Go REST/MCP service (`notriosd`), a built-in GUI, SQLite/FTS5-backed canonical storage, content-addressed resources, optional Recoll-derived search/extraction, and support for third-party native clients (C++/Qt, Go/Wails, Rust/Tauri) over the same API.

The v0.1 MVP is complete; the project is now in the **Notrios redesign** phase — see [`PLAN.md`](PLAN.md). The repository is structured so a coding agent can resume safely after usage limits or model changes.

## Technology choices

- **Service:** Go, standard library first, SQLite/FTS5 canonical storage. Binaries: `notriosd` (service) and `notriosctl` (CLI); Go module `github.com/renesugar/notrios`.
- **MCP:** read-only JSON-RPC MCP MVP adapter is mounted at `/mcp`; replace with the official Go SDK once dependency policy/tooling is settled. The MCP/REST surface is being expanded so a full note-taking client can be built on it alone.
- **Built-in GUI:** Go + Wails (`cmd/notrios`, built with `make gui`) with `-no-gui` (headless service) and `-gui-only` (pure REST client, optionally against a remote service via `-remote`) modes; the React + Vite frontend (`md-editor-rt`, notebooks/tags sidebar, incremental "All notes") runs inside the webview and in the browser. See `UI_DESIGN.md`.
- **Search:** SQLite FTS5 for managed notes; Recoll as an optional derived sidecar for field/front-matter search, OCR-style extraction, and arbitrary files (see `RECOLL_INTEGRATION.md` and `SEARCH_QUERY_LANGUAGE.md`).
- **Data model:** nested notebooks with emoji icons, tags, and query-backed "search notebooks" ("All notes", "Trash", "Help") — see `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md`.
- **Import sources:** Joplin RAW, Obsidian vaults, Twitter/X archives (thread-preserving), ChatGPT exports, Claude exports.
- **Resource store:** content-addressed assets with exact hashes and later perceptual hashes.
- **Publishing/docs:** Quartz-compatible curated subset publishing planned; project documentation ships as a GitHub Pages site with PageFind search (`DOCS_SITE.md`).

## Quick start

```bash
# Go service and CLI stubs
go test ./...
go run ./cmd/notriosd -config config/config.example.yaml
# Optional overrides still work:
# go run ./cmd/notriosd -addr 127.0.0.1:8080 -db data/notes.sqlite

# In another terminal
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/v1/status

# MCP MVP metadata and tool listing
curl http://127.0.0.1:8080/mcp
curl -X POST http://127.0.0.1:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Create, update, and search a persisted Markdown note
curl -X POST http://127.0.0.1:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{"title":"Hello","body":"SQLite FTS5 is wired."}'

curl -X POST http://127.0.0.1:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query":"SQLite FTS5","limit":10}'

# After creating notes with Markdown links, inspect outgoing links/backlinks:
# curl http://127.0.0.1:8080/api/v1/documents/<doc_id>/links?direction=both
# curl -X POST http://127.0.0.1:8080/api/v1/graph \
#   -H "Content-Type: application/json" \
#   -d '{"roots":["<doc_id>"],"direction":"both","max_nodes":20,"max_edges":40}'

# To update a note, send the current revision from the create/read response:
# curl -X PUT http://127.0.0.1:8080/api/v1/documents/<doc_id> \
#   -H "Content-Type: application/json" \
#   -d '{"title":"Hello v2","body":"Updated body","base_revision_id":"<current_revision_id>"}'

go run ./cmd/notriosctl doctor
```

Frontend dependencies are declared and can be installed with npm:

```bash
cd web
npm install
npm run dev
```


## Configuration

`notriosd` now loads runtime configuration from `-config <path>`. When no path is supplied, it loads `config/config.example.yaml` from a source checkout if that file exists; otherwise it uses compiled local-development defaults. The `-addr` and `-db` flags remain available as explicit overrides.

On startup, the service creates the configured data directory, SQLite database parent directory, asset store, projection directory, and search-sidecar index directory (`search_sidecar.index_dir`). `/api/v1/status` reports the active storage roots, database state, schema version, capability flags, and search limits. The current schema version is 4 after the Markdown link parser and graph task; the Joplin RAW and Obsidian importers reuse this schema.

## Project status

- v0.1 MVP: complete (see `MVP_RELEASE_REPORT.md`).
- Current phase: **v0.2 Notrios redesign foundation** — active plan in [`PLAN.md`](PLAN.md); tasks R1 (documentation redesign) and R2 (code rename + Apache-2.0 license) done.
- Scaffold creation plan: [`SCAFFOLD_CREATION_PLAN.md`](SCAFFOLD_CREATION_PLAN.md).
- Agent progress/attempt tracking: [`agent/PLAN_STATUS.md`](agent/PLAN_STATUS.md), [`agent/ATTEMPT_LOG.jsonl`](agent/ATTEMPT_LOG.jsonl), and [`agent/MODEL_LOG.jsonl`](agent/MODEL_LOG.jsonl).

## Repository setup

The repository is a local git repo: `main` holds the pre-redesign baseline; development happens on `develop`. The eventual public home is `https://github.com/renesugar/notrios`:

```bash
git remote add origin https://github.com/renesugar/notrios.git
git branch -M main
git push -u origin main
```

Configure branch protection so `main` accepts only reviewed merges from `develop` or release branches.

## License

Notrios is licensed under the [Apache License 2.0](LICENSE). All code and dependencies must remain Apache-2.0 compatible; GPL tools (Recoll, Xapian) are only ever invoked as external user-installed processes.

## Notrios redesign design documents

- `NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md` — notebooks, tags, search notebooks, Trash/Help semantics.
- `SEARCH_QUERY_LANGUAGE.md` — user query language and backend translation rules.
- `RECOLL_INTEGRATION.md` — Recoll sidecar design (replaces sist2) and licensing boundary.
- `DOCS_SITE.md` — GitHub Pages documentation site with PageFind and the Help notebook.
- `CODING_CLIENT_HANDOFF.md` — agent handoff (formerly `CODEX_HANDOFF.md`).

## Design documents added during scaffold review

- `FEATURE_MATRIX.md` — feature inventory, milestone placement, and ownership.
- `UI_DESIGN.md` — built-in React UI, `md-editor-rt`, preview links, resources, and future editor migration.
- `PUBLISHING_POLICY.md` — Quartz publishing profiles and privacy-safe subset export.
- `VERSIONING_AND_SYNC_POLICY.md` — SQLite revisions first, optional go-git/Fossil checkpoint adapters.
- `WORKSPACE_MAINTENANCE.md` — Foam-style query blocks, lint/fix, outlines, block anchors, and tag maintenance.
- `SCAFFOLD_REVIEW_REPORT.md` — Step 2 review summary.

- `SCAFFOLD_STEP4_REPORT.md` — Step 4 runnable persistence slice summary.
- `SCAFFOLD_STEP5_REPORT.md` — Step 5 UI integration summary.
- `SCAFFOLD_STEP6_REPORT.md` — Step 6 Codex handoff summary.
- `MVP_TASK1_REPORT.md` — configuration, directory creation, and enriched status endpoint summary.
- `MVP_TASK2_REPORT.md` — document update/delete/revision/restore implementation summary.

- `MVP_TASK4_REPORT.md` — content-addressed resource store implementation summary.
- `MVP_TASK5_REPORT.md` — Markdown link parser, backlinks, graph endpoint, and UI link display summary.
- `MVP_TASK6_REPORT.md` — `md-editor-rt` editor/preview and preview app-link routing summary.
- `MVP_TASK7_REPORT.md` — read-only MCP MVP adapter summary.
- `MVP_TASK8_REPORT.md` — Joplin RAW importer MVP summary.
- `MVP_TASK9_REPORT.md` — Obsidian vault importer MVP summary.


## MVP Task 6 UI status

The browser UI now uses `md-editor-rt` for Markdown editing and preview. Rendered preview links using `document://.../documents/{id}` open the target note inside the UI; rendered `resource://.../resources/{id}` links download through the REST resource endpoint. Local resource images are rewritten to REST content URLs in preview.

## MVP Task 8 Joplin RAW import status

`notriosctl` now supports a first Joplin RAW importer:

```bash
go run ./cmd/notriosctl import joplin-raw \
  --db ./data/notes.sqlite \
  --asset-store ./data/assets \
  --collection default \
  /path/to/joplin-raw-export
```

The importer parses Joplin notes, notebooks, tags, note-tag joins, and resources; stores imported notes/resources with deterministic source-derived IDs; rewrites `:/<id>` links to `document://` and `resource://` URIs; adds selected Joplin metadata to Markdown frontmatter; attaches referenced resources; and prints a JSON import report. Use `--dry-run` to inspect a source directory without writing.

## MVP Task 9 Obsidian import status

`notriosctl` now supports a first Obsidian vault importer:

```bash
go run ./cmd/notriosctl import obsidian \
  --db ./data/notes.sqlite \
  --asset-store ./data/assets \
  --collection default \
  /path/to/obsidian-vault
```

The importer scans Markdown files and non-Markdown assets, skips `.obsidian` and common VCS/dependency folders, imports notes/resources with deterministic vault-path-derived IDs, augments/preserves frontmatter with `source_system: obsidian` and `obsidian_path`, attaches referenced local assets, preserves Wikilinks/embeds/unresolved links in Markdown, and refreshes link rows after the batch so graph/backlinks work for notes imported in any order. Use `--dry-run` to inspect a vault without writing.


## v0.1 MVP release hardening

The MVP release-hardening task added repeatable checks and packaging helpers:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notrios-v0.1.0-mvp.zip
python3 scripts/check_release_zip.py /tmp/notrios-v0.1.0-mvp.zip
```

See `PACKAGING.md`, `SECURITY_REVIEW.md`, `RELEASE_CHECKLIST.md`, and `MVP_RELEASE_REPORT.md` before tagging a repository release. `PLAN.md` now contains the draft v0.2 plan and should not be started until the user approves.
