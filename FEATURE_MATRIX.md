# Feature Matrix

This matrix keeps the long conversation compressed into implementation-sized features. Use it when creating new plans from `ROADMAP.md`.

## Status legend

- **Implemented** — present in the repository (may still have a named
  hardening task).
- **Active** — in the current implementation plan.
- **Planned vX** — assigned to a future milestone but not implemented.
- **Optional/research** — no adoption decision until a measured need exists.

## Core storage and search

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| SQLite canonical storage | Implemented | Notrios service | Documents, resources, revisions, links, collections, import state. |
| SQLite FTS5 immediate search | Implemented | Notrios service | Always-on baseline. |
| Content-addressed blobs | Implemented | resource service | Exact-byte dedupe and H5 reference reports; H6 adds GC. |
| Resource reference reporting/GC | Implemented | resource service | H5 reports plus H6 dry-run-first retention, transactional rechecks, and sync-aware gate. |
| Document revisions and trash | Implemented | document service | v0.7 adds replicated death certificates/GC acknowledgements. |
| Scalable keyset cursors | Implemented | search service | `k2` chronological/relevance keysets plus explicit bounded `m1` sidecar snapshots; no unbounded offset ceiling. |
| Recoll sidecar (replaces sist2) | Implemented | adapter | Optional external process; bounded retry, exact reconciliation, attributed stable merges, and status/UI telemetry. |
| Notebooks/tags/search notebooks | Implemented | Notrios service | All notes/Notes/Help/Trash bootstrap and protections. |
| Query-language adapter | Implemented | search service | Bounded AST with implicit AND, uppercase `OR`, prefix negation, grouping, phrases, fields, `category:` alias/All-notes semantics, cursor binding, and live FTS5/SQL/Recoll parity. |
| Bluge generated-site search | External integration | movenotes-v3/Ledger | Measured external publication backend; not a Notrios application dependency. |
| LadybugDB | Optional/research | derived graph backend | Never primary storage. |

## Documents, links, and resources

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Stable document/resource URIs | Implemented | Notrios service | Internal `document://`/`resource://`; external `notrios://databases/{database_id}/documents/{document_id}` implemented in v0.4 P5 with strict parsing, an explicit local profile registry, `POST /api/v1/links/resolve`, and the Ubuntu/XDG protocol handler. |
| Markdown link parsing | MVP | link parser | Standard Markdown links, app URIs, Joplin IDs, Obsidian Wikilinks, embeds. |
| Backlinks/outgoing links | MVP | graph service | Store source positions and link context when possible. |
| Resource manifest | MVP | resource service | List embedded/attached resources for a note. |
| Attachment download | MVP | REST/UI | Preview links to resources must download/open local resource content. |
| Block and heading anchors | Implemented | parser/graph | Schema-v14 `document_blocks` with content-derived identity plus schema-v15 heading slugs. `GET /api/v1/documents/{id}/blocks`, per-anchor backlink counts, and anchor resolution for `document://` and `notrios://` links: precedence is authored `^marker`, then block ID, then heading slug. Heading text normalizes to the slug; a stable link carries no percent-escapes. |
| PDF page-level resources | Optional/research | extraction adapter | Separate page text/image/figures when needed. |
| Bounded graph traversal and paths | Implemented | graph service | v0.5 E4: `POST /api/v1/graph` honours `depth` (it was accepted and ignored before), `POST /api/v1/graph/path` finds a shortest path from both ends, and `GET /api/v1/graph/report` lists orphans, isolates, and in-degree hubs. Every bound is refused rather than clamped, and a stopped traversal names the ceiling it hit. SQLite only; LadybugDB remains optional research. |
| Graph centrality/community detection | Optional/research | graph service | E4 delivers degree-based hubs; clustering and centrality measures have no measured need yet. |

## Import and export

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Joplin RAW import | Implemented + active hardening | importer | J1 physical-line/canonical-title parser complete; J2/J3 add real-export relationship and million-note full-write evidence. |
| Obsidian vault import | Implemented | importer | Nested hierarchy, bounded batches, fingerprints/checkpoints, dry-run diffs, rename config, exact source bundle, and canonical links/anchors. |
| Twitter/X import | Implemented | importer | Provenance/thread/media import. |
| ChatGPT export import | Implemented | importer | Conversation provenance. |
| Claude JSON import | Implemented | importer | Conversation provenance. |
| Native archive v1 | Implemented | exporter | Query-scoped plain-note interchange; not lossless backup. |
| Shared selection/privacy planner | Implemented | archive/publishing service | Read-only typed recursive notebook/tag/query/ID selection, target policies, reachable resources, link/privacy/source-bundle/metadata decisions, bounded REST/MCP details, and deterministic digest. |
| Portable Markdown vault export | Planned v0.7 | movenotes-v3 | `notrios2sql.py` consumes archive v2; movenotes owns Obsidian projection. The compatibility bridge moved from v0.4 P6 to v0.7 because slice 3 extends the container it would pin, and the importer does not exist yet. |
| Native archive v2/backup | Implemented (export, verify, restore) | archive service | Schema-v12 database/replica identity, manifest-last SHA-256 objects, typed bounded records, explicit restore intent, and strict verification. `notriosctl export archive-v2` writes full-backup and explicitly scoped subset snapshots from one read transaction, stages privately, publishes the manifest last, resumes over published objects, and self-verifies. P3a moved the object inventory into checksummed index chunks under a two-level fanout and made writer and verifier stream through external-sorted spools, so an archive holds up to 8,000,000 objects and the manifest no longer grows with the library. P3b added the opt-in packed layout behind `objects.pack.v1`. P4 added `verify archive-v2` and `restore archive-v2`, proven on the attachment-bearing corpus under both layouts. |
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
| Editor link intelligence | Implemented | web UI + service | v0.5 E5: `GET /api/v1/links/suggest` (bounded title autocomplete, IDs and titles only) and `POST /api/v1/links/check` (unsaved-buffer link resolution through the canonical extractor). The client shows a link picker that inserts a canonical URI at the caret and a located list of links that will not open; both degrade to nothing when the service is unreachable. In-editor underlines need source positions `md-editor-rt` does not expose — that is E6's input. |
| CodeMirror 6 + unified migration | Planned v0.5 | web UI | For deeper AST/source-position behavior. E5 established the concrete gap: in-editor marker placement and caret position. |
| Third-party native clients | Optional/research | separate client | Use stable REST/MCP. |

## Remote media, safety, and dedupe

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Remote image localization | Implemented | media service | REST/CLI/MCP/UI/import flag share one engine. |
| Domain stop list | Implemented | media policy | Applied before fetch and every redirect. |
| Quarantine fetch | Implemented | media service | SSRF/size/MIME/hash checks. |
| Exact-hash reports | Implemented | storage/media | SHA-256 duplicates, unreferenced blobs, and notebook usage via REST/CLI. |
| Perceptual-hash hooks | Implemented | media policy | Pluggable admission/policy/report contract; no algorithm ships; suggestions only. |
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
| Publication profiles | Implemented | planner/archive | Saved profiles (`notriosctl publish`), review-then-publish gated on the plan digest, current-note projection with link rewriting and dropped link records. Consuming the handoff downstream is the v0.7 compatibility bridge. |
| Quartz and scalable archive site | External integration | movenotes-v3/Ledger | Obsidian/Quartz for subsets; Hugo/Ledger+Bluge for large libraries. |
| Publishing dry run | Implemented | planner/UI | `publish plan` and `POST /api/v1/selection/plan` report included/excluded notes, resources, link decisions, metadata stripping, and warnings before anything is written. |
| Foam-style query blocks | Planned v0.5 | query service/UI | No arbitrary SQL/JS. |
| Workspace lint | Implemented | maintenance service | Eleven read-only checks through `notriosctl lint` and `GET /api/v1/admin/lint/report`; complete counts with capped examples, a library-wide digest, and content-free findings. |
| Workspace fix | Implemented | maintenance service | `notriosctl fix`, dry-run first, single-note and revision-preconditioned, every fix an ordinary revision. Repairs non-canonical link targets by default; alt text and remote-media localization are opt-in. Everything else stays reported. |
| Outline API | Implemented | document parser | Headings/line anchors; v0.5 E1 adds the block model underneath. |
| Hierarchical tag rename | Planned v0.5 | maintenance service | Dry-run first. |
| Link reference definitions | External integration | movenotes-v3 | Generated in downstream portable Markdown projection. |

## Versioning and sync

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| SQLite revision restore | Implemented | document service | Restore creates a new revision. |
| Native record-level sync | Planned v0.7 | sync service | Operation IDs + HLC + ack vectors; see `SYNCHRONIZATION.md`. |
| REST/folder/rclone transports | Planned v0.7 | sync service | One immutable object/envelope protocol; target `none` supported. |
| Backup/restore replace/merge/fork/adopt | Implemented | archive/sync | No default; verification completes before writes; replace/adopt/fork rotate replica identity while in-place merge retains the target replica. A schema-v13 `restore_state` marker makes an interrupted restore visible, and only `replace` recovers it. |
| Yjs-compatible live co-editing | Optional/research | editor service | Separate from database sync. |
| go-git/Fossil checkpoints | Optional/research | version adapter | Projection history, never canonical sync. |
| Bidirectional external-vault sync | Optional/research | sync adapter | Explicit ownership/conflict policy. |
| Nostr/BLE courier transports | Optional/research | transport adapter | Post-v1; REST+rclone first. |
