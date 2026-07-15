# Plan Status

## Current phase

v0.1 MVP completed; v0.2 draft plan prepared but not started.

## Current working state

- Step 1 through Step 6 scaffold content remains intact.
- MVP Tasks 1, 2, 4, 5, 6, 7, 8, 9, and 10 are complete.
- SQLite migrations are wired into `internal/store` and report schema version `4` through `PRAGMA user_version`.
- `notesd` loads configuration, creates storage directories, opens SQLite, bootstraps the default collection, and starts REST and MCP handlers.
- REST can create, read, update, patch, soft-delete, list revisions, read revisions, restore revisions, return document bodies, and search managed Markdown documents through FTS5.
- REST can upload resources, return resource metadata, stream resource content, attach/list/detach document-resource references, and safely refuse deletion of referenced resources.
- Resource bytes are stored in the configured asset store under SHA-256 content-addressed paths; exact duplicate bytes reuse the same blob.
- Resource content responses set `X-Content-Type-Options: nosniff` and safe `Content-Disposition` headers.
- REST parses and stores Markdown/document/resource/external links, exposes outgoing links and backlinks, and returns a small graph slice for selected roots.
- `/api/v1/status` reports document, resource, search, link, and MCP capability flags as implemented when configured and store-backed.
- The React/Vite UI can read status, create a note, search notes, open a note, save an opened note as a new revision, upload/attach a resource to the opened note, insert a `resource://` Markdown link, list attached resources, download them, and display outgoing links/backlinks for the opened note. It uses `md-editor-rt`, rendered-preview navigation for `document://` links, rendered-preview downloads for `resource://` links, and editor image upload to local resources.
- MCP MVP is mounted at `/mcp` with a dependency-free JSON-RPC adapter. Implemented methods: `initialize`, `tools/list`, and `tools/call`. Implemented read-only tools: `list_collections`, `search_documents`, `get_document`, `get_documents`, `list_document_links`, `list_document_resources`, and `get_document_outline`.
- `notesctl import joplin-raw` imports a small Joplin RAW Export Directory into managed notes/resources, rewrites `:/<id>` note/resource links, attaches referenced resources, preserves selected Joplin metadata in frontmatter, and prints a JSON report.
- `notesctl import obsidian` imports a small Obsidian-style vault into managed notes/resources, preserves/augments Markdown frontmatter with source path metadata, attaches referenced local assets, preserves Wikilinks/embeds/unresolved links, and refreshes graph links after the batch.
- Release-hardening scripts exist for smoke testing, generated-dataset performance smoke testing, release packaging, and release ZIP verification.
- `PLAN.md` now contains a v0.2 draft generated from `ROADMAP.md`; do not begin it until the user approves.

## Next suggested step

Review the MVP ZIP, choose a license/module path/repository name, and then decide whether to start the v0.2 plan in `PLAN.md`.

## Validation

Expected validation set after MVP Task 10:

```bash
go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
cd web && npm ci && npm run typecheck && npm run build
bash scripts/mvp_smoke.sh
bash scripts/run_performance_smoke.sh
bash scripts/package_release.sh /tmp/notes-companion-v0.1.0-mvp-check.zip
python3 scripts/check_release_zip.py /tmp/notes-companion-v0.1.0-mvp-check.zip
```

## Open blockers

- Final license has not been selected.
- Repository name and Go module path are still placeholders.
- Long-term SQLite driver choice remains open. The current adapter uses cgo/libsqlite3 because external Go module downloads were avoided during scaffold creation.
- Current config parser supports only the documented example-config subset; Codex may replace it with a pinned YAML/TOML parser in a fuller environment.
- The MCP MVP adapter is intentionally dependency-free. Codex should replace it with the official MCP Go SDK in a normal environment if target MCP clients require full transport/session behavior.
- The Joplin RAW importer has only a synthetic fixture. Codex should verify it against real Joplin RAW exports before large migrations.
- The Obsidian importer has only a synthetic fixture. Codex should verify it against real vaults with duplicate note titles, duplicate asset filenames, unusual YAML, canvases, and plugin syntax before large migrations.
- Joplin/Obsidian source metadata is currently preserved in Markdown frontmatter rather than a dedicated import-object/job table.
- HTTP range requests, multipart upload parsing, remote-media localization, perceptual hashes, malware/media hash databases, sist2, and Quartz execution are documented but not implemented.
- The Markdown link parser is conservative and regex-based; later CodeMirror/unified/remark work can replace it behind the existing API/store contract.
- The `md-editor-rt` bundle is large and should be reviewed for code splitting or reduced toolbar/highlight configuration before production packaging.
- Preview app-link routing works in rendered preview, but source-editor Ctrl-click navigation remains a later CodeMirror/unified-level feature.
