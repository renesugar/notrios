# Feature Matrix

This matrix keeps the long conversation compressed into implementation-sized features. Use it when creating new plans from `ROADMAP.md`.

## Status legend

- **Implemented** — present in the repository (may still have a named
  hardening task).
- **Active** — in the current v0.3 plan.
- **Planned vX** — assigned to a future milestone but not implemented.
- **Optional/research** — no adoption decision until a measured need exists.

## Core storage and search

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| SQLite canonical storage | Implemented | Notrios service | Documents, resources, revisions, links, collections, import state. |
| SQLite FTS5 immediate search | Implemented | Notrios service | Always-on baseline. |
| Content-addressed blobs | Implemented | resource service | Exact-byte dedupe; H5/H6 add reports/GC. |
| Resource reference reporting/GC | Active | resource service | Dry-run first; sync-aware retention interface. |
| Document revisions and trash | Implemented | document service | v0.7 adds replicated death certificates/GC acknowledgements. |
| Scalable keyset cursors | Active | search service | Current opaque `q1` cursor is offset-backed and capped at 100k; H7 replaces it. |
| Recoll sidecar (replaces sist2) | Implemented/Active | adapter | Optional external process; H10 reconciliation/paging hardening. |
| Notebooks/tags/search notebooks | Implemented | Notrios service | All notes/Notes/Help/Trash bootstrap and protections. |
| Query-language adapter | Implemented | search service | FTS5 + optional Recoll compilation. |
| Bluge generated-site search | Optional/research | publishing adapter | Apache-2.0/capable but inactive upstream; v0.4 spike only. |
| LadybugDB | Optional/research | derived graph backend | Never primary storage. |

## Documents, links, and resources

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Stable document/resource URIs | Implemented | Notrios service | Internal `document://`/`resource://`; external `notrios://` planned v0.4. |
| Markdown link parsing | MVP | link parser | Standard Markdown links, app URIs, Joplin IDs, Obsidian Wikilinks, embeds. |
| Backlinks/outgoing links | MVP | graph service | Store source positions and link context when possible. |
| Resource manifest | MVP | resource service | List embedded/attached resources for a note. |
| Attachment download | MVP | REST/UI | Preview links to resources must download/open local resource content. |
| Block anchors | Planned v0.5 | parser/graph | First-class block references and block-level backlinks. |
| PDF page-level resources | Optional/research | extraction adapter | Separate page text/image/figures when needed. |
| Graph paths/centrality/community detection | Planned v0.5 | graph service | SQLite first; LadybugDB optional later. |

## Import and export

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Joplin RAW import | Implemented/Active | importer | H8 adds hierarchy, scale, checkpoints, exact source bundle. |
| Obsidian vault import | Implemented/Active | importer | H9 adds hierarchy, scale, checkpoints, exact source bundle. |
| Twitter/X import | Implemented | importer | Provenance/thread/media import. |
| ChatGPT export import | Implemented | importer | Conversation provenance. |
| Claude JSON import | Implemented | importer | Conversation provenance. |
| Native archive v1 | Implemented | exporter | Query-scoped plain-note interchange; not lossless backup. |
| Portable Markdown vault export | Planned v0.4 | exporter | User-facing interoperable export. |
| Native archive v2/backup | Planned v0.4 | archive service | Full snapshot/object manifest reused by sync. |
| Joplin RAW export | Optional/research | exporter | Only for measured exact round-trip need. |

## Built-in UI and editor

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| React + Vite built-in UI | MVP | web UI | Basic browser client embedded in the service; becomes the Wails webview frontend. |
| Go/Wails v2 built-in GUI (`-no-gui`/`-gui-only`, themes) | Implemented | GUI | Desktop release shell. |
| `md-editor-rt` editor/preview | MVP | web UI | Initial polished editor; wrap behind an adapter. |
| Preview link interception | MVP | web UI | `document://` opens note; `resource://` opens/downloads resource. |
| Resource upload/paste | MVP | web UI + REST | Images/PDFs become local resources, not inline base64. |
| Preview sanitization | MVP | web UI | Sanitized Markdown/HTML; allow app routes/URIs carefully. |
| Four-pane layout | Implemented | web UI | Accessible splitters and independent scrolling. |
| Wails v3/mobile migration | Optional/research | GUI | Pre-release/experimental; real Android + desktop parity gate. |
| CodeMirror 6 + unified migration | Planned v0.5 | web UI | For deeper AST/source-position behavior. |
| Third-party native clients | Optional/research | separate client | Use stable REST/MCP. |

## Remote media, safety, and dedupe

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Remote image localization | Implemented | media service | REST/CLI/MCP/UI/import flag share one engine. |
| Domain stop list | Implemented | media policy | Applied before fetch and every redirect. |
| Quarantine fetch | Implemented | media service | SSRF/size/MIME/hash checks. |
| Exact-hash reports | Active | storage/media | SHA-256 report in H5. |
| Perceptual-hash hooks | Active | media policy | Inert/suggest-only hooks in H5. |
| SSRF protections | Implemented | media service | Connect-time address policy and redirect checks. |

## MCP and automation

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| MCP read tools | Implemented | MCP adapter | Bounded search/read/list tools. |
| MCP editor write tools | Implemented | MCP adapter | Profile-gated with revision preconditions. |
| MCP resources | Planned v0.6 | MCP adapter | Avoid global listing of huge collections. |
| Tool visibility profiles | Planned v0.6 | MCP adapter | Search/read/editor/organizer/admin. |
| LLM surgical edits | Implemented | document service | SEARCH/REPLACE, dry-run, revision preconditions. |
| Batch organizer transactions | Planned v0.6 | document service | Bounded atomic/best-effort move/duplicate/trash/tag/link. |
| Sync MCP control plane | Planned v0.7 | MCP adapter | Jobs/status/conflicts only; no bulk bytes in context. |

## Publishing and knowledge-base maintenance

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Quartz publish profiles | Planned v0.4 | publisher | Curated subsets with privacy planner. |
| Scalable archive-site profile | Planned v0.4 | publisher | Streamed generation/fixed nav/server search adapter. |
| Publishing dry run | Planned v0.4 | publisher/UI | Included/excluded/resources/private links. |
| Foam-style query blocks | Planned v0.5 | query service/UI | No arbitrary SQL/JS. |
| Workspace lint/fix | Planned v0.5 | maintenance service | Broken links/media/resources. |
| Outline API | Implemented | document parser | Headings/line anchors. |
| Hierarchical tag rename | Planned v0.5 | maintenance service | Dry-run first. |
| Link reference definitions | Planned v0.4 | exporter/maintenance | Portable Markdown links. |

## Versioning and sync

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| SQLite revision restore | Implemented | document service | Restore creates a new revision. |
| Native record-level sync | Planned v0.7 | sync service | Operation IDs + HLC + ack vectors; see `SYNCHRONIZATION.md`. |
| REST/folder/rclone transports | Planned v0.7 | sync service | One immutable object/envelope protocol; target `none` supported. |
| Backup/restore replace/merge/fork | Planned v0.4/v0.7 | archive/sync | Verified snapshot identity semantics. |
| Yjs-compatible live co-editing | Optional/research | editor service | Separate from database sync. |
| go-git/Fossil checkpoints | Optional/research | version adapter | Projection history, never canonical sync. |
| Bidirectional external-vault sync | Optional/research | sync adapter | Explicit ownership/conflict policy. |
| Nostr/BLE courier transports | Optional/research | transport adapter | Post-v1; REST+rclone first. |
