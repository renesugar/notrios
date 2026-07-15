# Feature Matrix

This matrix keeps the long conversation compressed into implementation-sized features. Use it when creating new plans from `ROADMAP.md`.

## Status legend

- **MVP** — needed for `v0.1`.
- **Soon** — likely `v0.2` to `v0.4`.
- **Later** — useful after the local product works.
- **Optional** — keep as adapter/research until a measured need appears.

## Core storage and search

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| SQLite canonical storage | MVP | companion service | Documents, resources, revisions, links, collections, import state. |
| SQLite FTS5 immediate search | MVP | companion service | First choice over Bleve when SQLite is authoritative. |
| Content-addressed blobs | MVP | companion service | Exact-hash dedupe and safe resource lifecycle. |
| Resource reference counting | MVP | companion service | Never delete shared resources accidentally. |
| Document revisions and trash | MVP | companion service | App-level undo/restore; Fossil/Git are optional checkpoint layers. |
| Cursor-ready search API | MVP | companion service | Cursor pagination required for large result sets; shallow offset may exist for UI. |
| sist2 sidecar | Soon | adapter | Derived OCR, thumbnails, archive traversal, arbitrary-file search. |
| Bleve | Optional | adapter | Add only for fuzzy/faceted/advanced search needs not met by FTS5. |
| LadybugDB | Optional | derived graph backend | Consider for advanced traversal/analytics; not primary storage. |

## Documents, links, and resources

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Stable document/resource URIs | MVP | companion service | `document://...` and `resource://...`; do not expose sist2 IDs. |
| Markdown link parsing | MVP | link parser | Standard Markdown links, app URIs, Joplin IDs, Obsidian Wikilinks, embeds. |
| Backlinks/outgoing links | MVP | graph service | Store source positions and link context when possible. |
| Resource manifest | MVP | resource service | List embedded/attached resources for a note. |
| Attachment download | MVP | REST/UI | Preview links to resources must download/open local resource content. |
| Block anchors | Soon | parser/graph | First-class block references and block-level backlinks. |
| PDF page-level resources | Later | extraction adapter | Separate page text/image/figures when needed. |
| Graph paths/centrality/community detection | Later | graph service | SQLite first; LadybugDB optional later. |

## Import and export

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Joplin RAW import | MVP | importer | Preferred over JEX for large imports; preserve original IDs. |
| Obsidian vault import | MVP | importer | Markdown + frontmatter + assets + Wikilinks/embeds. |
| Twitter/X import | Soon | importer | Preserve tweet IDs, media, replies, quote/repost relationships. |
| ChatGPT export import | Soon | importer | Preserve conversation/message roles and attachments. |
| Claude JSON import | Soon | importer | Preserve conversation/message roles and attachments. |
| Portable Markdown vault export | Soon | exporter | User-facing default export with stable frontmatter IDs. |
| Lossless application archive | Soon | exporter | Backup/restore with metadata, revisions, resources, checksums. |
| Joplin RAW export | Later | exporter | Only if exact Joplin round-trip becomes a requirement. |

## Built-in UI and editor

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| React + Vite built-in UI | MVP | web UI | Basic browser client embedded in the service. |
| `md-editor-rt` editor/preview | MVP | web UI | Initial polished editor; wrap behind an adapter. |
| Preview link interception | MVP | web UI | `document://` opens note; `resource://` opens/downloads resource. |
| Resource upload/paste | MVP | web UI + REST | Images/PDFs become local resources, not inline base64. |
| Preview sanitization | MVP | web UI | Sanitized Markdown/HTML; allow app routes/URIs carefully. |
| LeafWiki-style layout | Soon | web UI | Folder/tree, editor/preview, search, backlinks, resources, revisions. |
| CodeMirror 6 + unified migration | Later | web UI | Needed for deep editor-pane link widgets and AST/source-position behavior. |
| Native C++/Qt client | Later | separate client | Uses stable REST API; not in companion-service MVP. |

## Remote media, safety, and dedupe

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Remote image localization | Soon | media service | Import-time and UI-triggered. Server downloads; preview only detects. |
| Domain stop list | Soon | media policy | Applied before fetch and on every redirect. |
| Quarantine fetch | Soon | media service | Temporary storage until content passes policy checks. |
| Exact-hash dedupe | Soon | storage/media | SHA-256 first; optional BLAKE3 later. |
| Perceptual-hash hooks | Later | media policy | Local moderation and near-duplicate detection; ThreatExchange/PDQ-compatible path. |
| SSRF protections | Soon | media service | Block private networks, unsafe schemes, redirects. |

## MCP and automation

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| MCP read tools | MVP | MCP adapter | Search/read/list collections/links/resources. |
| MCP resources | Soon | MCP adapter | Document/resource URIs; avoid global list of huge collections. |
| MCP write tools | Later | MCP adapter | Scope-gated with revision preconditions. |
| Tool visibility profiles | Later | MCP adapter | Search-only/read-only/editor/organizer/admin profiles. |
| LLM surgical edits | Later | document service | SEARCH/REPLACE, dry-run, revision preconditions. |
| Batch transactions | Later | document service | Atomic multi-document changes and link rewrites. |

## Publishing and knowledge-base maintenance

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Quartz publish profiles | Soon | publisher | Selected notebooks/folders/tags, recursive include, private-link/resource checks. |
| Publishing dry run | Soon | publisher/UI | Show included/excluded notes/resources and privacy warnings. |
| Foam-style query blocks | Later | query service/UI | Safe embedded dashboards, not arbitrary SQL/JS. |
| Workspace lint/fix | Later | maintenance service | Broken links, stale reference definitions, remote images, unreferenced resources. |
| Outline API | Soon | document parser | Useful for long notes and LLM context selection. |
| Hierarchical tag rename | Later | maintenance service | Dry-run first; update frontmatter and inline tags. |
| Link reference definitions | Later | exporter/maintenance | Make Wikilinks more portable in Markdown exports. |

## Versioning and sync

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| SQLite revision diff/restore | MVP/Soon | document service | Preferred immediate durable undo/redo foundation. |
| go-git checkpoints | Optional | version adapter | Projection backup/history/sync, not canonical storage. |
| Fossil checkpoints | Optional | version adapter | Useful for archive/sync but not primary store. |
| Bidirectional external-vault sync | Later | sync service | Requires clear ownership/conflict policy. |
