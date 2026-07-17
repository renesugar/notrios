# Roadmap

This roadmap is a feature inventory and planning source. `PLAN.md` should contain only the active implementation plan.

## v0.1 — Minimum viable product

Goal: usable local note-taking/search app with REST/MCP and a built-in UI.

- Go service and CLI stubs mature into a real local daemon.
- SQLite canonical storage with documents, revisions, resources, links, and FTS5.
- Content-addressed resources and safe download endpoints.
- React built-in UI using `md-editor-rt` initially.
- Built-in UI follows a LeafWiki-like layout: tree/search, editor/preview, links/resources/revisions panels.
- Preview link interception for `document://` and `resource://`.
- Basic document CRUD and search.
- Markdown link/backlink parsing.
- MCP read tools.
- Minimal Joplin RAW and Obsidian importers.
- Preview links open internal notes and downloadable resources.
- Test fixtures and CI.

## v0.2 — Notrios redesign foundation (complete; archived under `plans/v0.2/`)

The built-in Go/Wails GUI is part of the first released version, so it lives here rather than in a later milestone.

- Rebrand to Notrios: `notesd` → `notriosd`, `notesctl` → `notriosctl`, module path `github.com/renesugar/notrios`, MIT/Apache-2.0 license selection.
- Schema: nested notebooks with emoji icons, tags with counts, query-backed search notebooks ("All notes" first / "Trash" last / read-only "Help", user query notebooks), case-insensitive notebook names, trash/undelete/purge semantics.
- Schema: source provenance for Joplin, Obsidian, Twitter/X, ChatGPT, and Claude, including conversation threads (author, author ID, thread ID, reply-to, post URL).
- Query-language adapter: `notebook:`, `tag:`, `author:`, `authorid:`, `title:`, `since:`, `until:`, phrases (see `SEARCH_QUERY_LANGUAGE.md`).
- Recoll integration replacing sist2: projection + outbox, generated Recoll config, from-scratch front-matter handler, external-process adapter (see `RECOLL_INTEGRATION.md`).
- MCP/REST expansion sufficient for full third-party clients (C++/Qt, Go/Wails, Rust/Tauri).
- Twitter/X, ChatGPT, and Claude importers.
- Query-scoped export preserving notebook structure; import dry run with rename-on-import configuration file.
- Go/Wails built-in GUI: menu bar, notebooks/tags sidebar, incremental search results, Markdown editor + preview, nested notebooks, light/dark toggle with user-defined custom themes, `-no-gui` and `-gui-only` modes.
- Documentation site: `docs/` published to GitHub Pages with PageFind search; Help notebook seeded from the same content.
- GitHub release preparation for `github.com/renesugar/notrios`.

## v0.3 — Import, resource, and media hardening (active; see `PLAN.md`)

(Deferred former v0.2 draft; see `plans/v0.2/001-import-resource-media-hardening.md`.)

- Joplin RAW and Obsidian importer hardening (larger fixtures, resume/checkpoint, dry-run diffs).
- Remote media localization; import-time and UI-triggered localization use the same policy engine.
- Domain stop list and redirect-domain checks.
- Quarantine store.
- Exact-hash deduplication.
- Local perceptual-hash database hooks for moderation and near-duplicate review.
- Resource garbage collection and retention policy.
- Recoll hardening: batched incremental scans, periodic reconciliation, FTS5/Recoll search result merging, extraction status in UI.

## v0.4 — Publishing and export

- Portable Markdown vault export.
- Lossless application archive export.
- Quartz publish profiles for selected notebooks/folders/tags, including recursive subfolder/subnotebook selection.
- Link-to-private-note policy.
- Public resource reachability analysis.
- Dry-run publishing privacy checks.
- Optional Foam-style query/dashboard export.
- Link reference definition generation for portable Markdown publishing.

## v0.5 — Better editing and graph UX

- Consider migration from `React + md-editor-rt` to `React + CodeMirror 6 + unified/remark/rehype` if deeper editor-pane behavior is needed.
- Rich link autocomplete.
- Broken-link underlines while editing.
- Block anchor database and navigation.
- Graph visualization and path finding.
- Embedded query blocks inside notes.
- Outline API and block-level addressability.
- Trash-first delete/restore UX.
- Workspace lint/fix.
- Templates and task extraction.

## v0.6 — MCP and automation expansion

- MCP write tools gated by explicit scopes.
- Resource graph resources/read support.
- LLM-safe surgical edits with dry-run and revision preconditions.
- Batch transactions.
- LLM-safe SEARCH/REPLACE edits with dry-run and revision/hash preconditions.
- Tool visibility profiles: search-only, read-only, editor, organizer, administrator.

## v0.7 — Versioning and synchronization

- SQLite saved-revision diff/restore UX.
- Optional go-git projection checkpointing.
- Optional Fossil export/checkpoint support.
- Sync status and conflict workflows.
- Git-managed Obsidian vault import/checkpoint support as an optional adapter.
- External Obsidian-vault bidirectional sync policy.

## v1.0 — Feature-complete local product

- Stable REST API.
- Stable MCP tool/resource schemas.
- Large-scale performance tests with hundreds of thousands of documents/resources.
- Installer/package story.
- Backup/export/restore validation.
- Security review for remote media and MCP.
- Usable documentation for Gitea/GitHub public release.

## Future candidates

- Native third-party clients over the public REST/MCP API (C++/Qt, Rust/Tauri, additional Go/Wails clients).
- LadybugDB derived graph backend for advanced graph traversal and analytics.
- Semantic/vector search.
- More importers.
- Multi-user deployment.
- Enterprise policy administration.


## Agent handoff status

The scaffold handoff is complete; see `CODING_CLIENT_HANDOFF.md`. Future roadmap planning should be driven from `ROADMAP.md`, but each active implementation cycle should create a small `PLAN.md` slice and archive it under `plans/` when complete.


## v0.1 completion note

The v0.1 MVP milestone has been implemented and release-hardened. Active work follows the v0.2 Notrios redesign plan in `PLAN.md`; the former v0.2 media-hardening draft moved to the v0.3 milestone above.
