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

## v0.2 — Import, resource, and media hardening

- Robust Joplin RAW importer with tags, notebooks, note-tag joins, resources, and original IDs.
- Obsidian importer with frontmatter, Wikilinks, embeds, headings, aliases, and block references.
- Twitter/X archive importer.
- ChatGPT conversations importer.
- Claude conversations importer.
- Remote media localization.
- Import-time and UI-triggered remote-media localization use the same policy engine.
- Domain stop list and redirect-domain checks.
- Quarantine store.
- Exact-hash deduplication.
- Local perceptual-hash database hooks for moderation and near-duplicate review.
- Resource garbage collection and retention policy.

## v0.3 — sist2 integration

- Managed filesystem projection.
- Durable indexing outbox.
- sist2 CLI or admin adapter.
- Batched incremental scans using list files.
- Periodic reconciliation.
- Search result merging between managed-note FTS5 and sist2-derived arbitrary-file search.
- OCR/thumbnail/extracted-metadata status in UI.

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

- Native C++/Qt client.
- LadybugDB derived graph backend for advanced graph traversal and analytics.
- Semantic/vector search.
- More importers.
- Multi-user deployment.
- Enterprise policy administration.


## Codex handoff status

The scaffold handoff is complete as of `SCAFFOLD_STEP6_REPORT.md`. Future roadmap planning should be driven from `ROADMAP.md`, but each active implementation cycle should create a small `PLAN.md` slice and archive it under `plans/` when complete.


## v0.1 completion note

The v0.1 MVP milestone has been implemented and release-hardened. Future active work should start from the v0.2 draft in `PLAN.md` only after user approval.
