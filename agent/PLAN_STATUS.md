# Plan Status

## Current phase

**v0.3 import/resource/media hardening** (`PLAN.md`, tasks H1–H10, drafted 2026-07-16 from `ROADMAP.md`). **H1 (media-policy configuration and schema v7) is complete**: typed `remote_media` config (list + byte-size parsing, safe restrictive defaults, quarantine dir), schema v7 (media_domain_rules, media_hash_rules, resource_hashes, widened media_policy_decisions; v6→v7 shim), `/api/v1/status` `media_policy` block, docs/OpenAPI updated — see `plans/v0.3/001-media-policy-config-schema.md`. **H2 (remote-media scan) is complete**: `internal/media` policy engine (static, no network/DNS), real `POST /documents/{id}/remote-media/scan` (+ `urls` override), `GET /media-policy`, `POST /media-policy/check-url`, read-only MCP `scan_remote_media`, GUI note-inspector warnings with action badges — see `plans/v0.3/002-remote-media-scan.md`. **H3 (quarantine pipeline) is complete**: `internal/media.Fetcher` with redirect-hop policy re-checks, connect-time private-address blocking (DNS-rebinding defense; env proxies ignored), streaming size caps, sniffed-MIME enforcement, SHA-256, hash-named quarantine files, and every attempt recorded via `store.RecordMediaAttempt` into `media_policy_decisions` — see `plans/v0.3/003-quarantine-pipeline.md`. Post-v0.2 work already landed on `develop`: the GUI conformance pass (four-pane workspace, splitters, read-only Help capability, cursor paging, resize equalization; commit c630411), native window-resize layout verification (`scripts/verify_layout_resize.py`; commit ccb7b2e), and the scaffold/MVP report archive move into `plans/scaffold/` and `plans/mvp/` (commit cbfe4b1).

Previous phase (complete): v0.2 Notrios redesign foundation. Tasks **R1 (documentation redesign)**, **R2 (code rename + Apache-2.0 license)**, **R3 (schema v5 — notebooks/tags/search notebooks)**, and **R4 (schema v6 — source provenance and threads)** are completed, as are **R5 (notebooks/tags/trash REST + MCP read tools)**, **R6 (query-language adapter)**, **R7 (Recoll integration)**, **R8 (full-client MCP/REST surface)**, **R9 (Twitter/X archive importer)**, **R10 (ChatGPT importer)**, and **R11 (Claude importer)**; as is **R12 (query-scoped export/import with dry-run)**; **R13 (Wails GUI shell)**, **R14 (GUI themes)**, and **R15 (Help notebook + documentation site)**; and **R16 (GitHub release preparation)** — **the v0.2 plan is complete**. Next: draft a v0.3 plan from `ROADMAP.md` with user approval; the first GitHub push is the user's step (`RELEASE_CHECKLIST.md`).

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

- R4 changes (schema v6, store layer): `document_sources` provenance table (source_system, external_id, author/author_id, thread_id/reply_to, source_url, published_at/published_ts), `SetDocumentSource`/`GetDocumentSource`/`FindDocumentBySource`/`ListThreadDocuments` store APIs, purge guard refusing externally-sourced notes, Joplin/Obsidian importers write provenance on every run (re-import backfills). See `plans/v0.2/005-schema-v6-source-provenance.md`.

- R5 changes (REST + MCP): notebook CRUD/tree routes, notebook notes listing, tags with counts, document tag add/remove, note↔notebook move, search-notebook CRUD, trash list/restore/purge; `POST /api/v1/documents` accepts `notebook_id` and documents return it; 409 `name_conflict` and 403 `forbidden` error mapping; MCP read tools `list_notebooks`/`get_notebook_tree`/`list_tags`/`list_search_notebooks`; OpenAPI + `api/mcp-tools.md` updated. See `plans/v0.2/006-notebooks-rest-mcp.md`.

- R6 changes (query language): `internal/query` parser + `internal/store/sqlite_query.go` compiler; `notebook:` (subtree, case-insensitive), `tag:`, `title:`, `author:`/`authorid:` (provenance joins), `since:`/`until:` (published_ts fallback to created_at), `is:trashed`, phrases, implicit AND; opaque query-bound cursors for incremental scrolling; `store.Search` fully routed through the adapter. See `plans/v0.2/007-query-language-adapter.md`.

- R7 changes (Recoll sidecar): transactional `index_outbox` enqueues on all document mutations; `internal/projection` (outbox drain + full sync to Markdown/front-matter files); `internal/recoll` (generated config, embedded from-scratch Apache-licensed front-matter handler, recollindex/recollq external processes, query compilation, base64 result parsing); merged sidecar search behind the existing API with FTS5 authoritative and graceful fallback; notriosd 30s sync loop when `search_sidecar.enabled`. Live pipeline verified against installed Recoll. See `plans/v0.2/008-recoll-integration.md`.
- The Recoll sidecar remains **optional**: everything (including all tests) passes without Recoll installed; `TestLiveRecollPipeline` self-skips.

- R8 changes (full-client surface): REST append/prepend/lines/search-in routes; the REST outline endpoint now returns real headings; PATCH edits enforce joplin-mcp editNote semantics (ambiguous match fails without `replace_all`); MCP read tools `get_note_line_range`/`search_in_note`/`get_notebook_notes`; MCP write tools (`create_note`, `update_note`, `append_to_note`, `prepend_to_note`, `edit_note`, `delete_note`, `move_note_to_notebook`) gated behind the `editor` MCP profile with revision preconditions on destructive ops; gap-check table added to `API_SPEC.md`. Also hardened `scripts/mvp_smoke.sh`: it now refuses a busy port and runs a built binary instead of `go run` (whose wrapper-kill leaked servers across runs). See `plans/v0.2/009-full-client-surface.md`.

- R9 changes (Twitter/X importer): `internal/importers/twitter` + `notriosctl import twitter` — parses `window.YTD` account/tweets files, recovers threads by walking in-reply-to chains among archived tweets, expands t.co URLs, imports `tweets_media` files as embedded resources, hashtags → tags, notes land in a 🐦 "Twitter" notebook, provenance rows (author, @handle, thread_id, reply_to, post URL, published) power `ListThreadDocuments` and purge protection; trashed tweets are never resurrected on re-import; dry run supported. genson-derived schemas in `testdata/schemas/`. See `plans/v0.2/010-twitter-importer.md`.

- R10/R11 changes (conversation importers): `internal/importers/chatgpt` (mapping-tree parsing, current-node main path, system/tool + abandoned-branch skipping) and `internal/importers/claude` (flat chat_messages, content-block fallback); one note per conversation with role/timestamp Markdown sections in 🤖 "ChatGPT" / ✳️ "Claude" notebooks; provenance thread = conversation ID; shared upsert semantics (unchanged detection, no resurrection of trashed notes, purge protection); `notriosctl import chatgpt|claude` with dry run; genson schemas added. See `plans/v0.2/011-conversation-importers.md`.

- R12 changes (native archive): `internal/archive` — query-scoped export (manifest, notebooks.json with paths+emoji, notes as Markdown+front matter, resource bytes), dry-run conflict analysis against source-bound notebooks with a prefilled rename `import-config.json`, validated rename-on-import that refuses conflicts before writing, notebook-path creation with nesting/emoji, idempotent plain-note imports (no provenance; purgeable); `notriosctl export archive` / `import archive`; store gains `NotebookHasSourcedDocuments`. Joplin/Obsidian notebook-hierarchy population deferred to v0.3. See `plans/v0.2/012-archive-export-import.md`.

- R13 changes (Wails GUI shell): `internal/service` extracted (shared by notriosd + notrios); `cmd/notrios` with default GUI+service, `-no-gui`, and `-gui-only -remote <url>` (reverse proxy) modes; Wails v2 asset server routes all webview requests through the service handler so the React frontend runs unmodified; menu bar (File/Edit/View/Help; Help triggers a `notebook:help` search); webview gated behind `gui desktop production webkit2_41` build tags with a helpful stub otherwise (`make gui` builds the real thing); React UI gained the sidebar layout — search notebooks ("All notes" first, "Trash" last), nested notebook tree with emoji, tag list with counts, startup "All notes" view with cursor-based "Load more". Verified live under xvfb (window + embedded service). See `plans/v0.2/013-wails-gui-shell.md`.

- R14 changes (GUI themes): `styles.css` colors tokenized into CSS custom properties; `web/src/themes.ts` (builtin Light/Dark token sets, localStorage persistence, custom-theme CRUD, per-mode theme selection, applyTheme); header 🌙/☀️ toggle + 🎨 theme settings panel (per-mode theme selects, create-from-current, per-token color editors, delete); editor/preview follows the theme base. Verified live in a browser via Playwright: toggle flips tokens, mode survives reload, a custom theme was created through the UI and selected for dark mode. See `plans/v0.2/014-gui-themes.md`.

- R15 changes (docs + Help notebook): user documentation under `docs/`; `scripts/build_docs_site.sh` builds the static site with a PageFind search index (verified live in a browser); `.github/workflows/docs.yml` deploys to GitHub Pages; `internal/helpdocs` + `notriosctl seed-help` mirror docs into the read-only Help notebook deterministically; Help notes now enforced read-only at REST/MCP layers (403). See `plans/v0.2/015-docs-site-help-notebook.md`.

- R16 changes (release prep): Go dependency licenses audited under the GUI build tags (MIT/BSD/Apache only) and npm production licenses audited (permissive + build-time MPL from lightningcss; our web package now declares Apache-2.0); CI expanded (vet/tests/scaffold with libsqlite3-dev, web typecheck/build, smoke + performance smoke, GUI compile check with Wails/WebKit tags); `RELEASE_CHECKLIST.md` rewritten with the audited state and the exact first-push sequence; README final pass. See `plans/v0.2/016-release-preparation.md`.

- 2026-07-16 documentation completeness pass: installation + troubleshooting guides, contributor-setup/packaging rewrites, accurate CLI reference, per-source import docs (Joplin RAW detailed), config/backup/API guides with live-verified examples; build hygiene (full Makefile with clean/clobber/precheck, gitignore fixes, tracked artifacts removed, release-zip exclusions); small truthfulness fixes: real `notriosctl doctor`, trashed-note re-import guards for Joplin/Obsidian, SQLite busy_timeout, version 0.2.0, web title.

## Next suggested step

Implement `PLAN.md` task **H4 (localize remote media)**; work one task at a time and ask the user before starting the next. The GitHub push itself is the user's step.

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
- Recoll integration is implemented (R7) but reconciliation/batched-scan hardening remains a v0.3 roadmap item.
