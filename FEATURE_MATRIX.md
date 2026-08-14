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
| Document revisions and trash | Implemented | document service | v0.5 E8 made deletion and restore reachable from the GUI; v0.7 adds replicated death certificates/GC acknowledgements. |
| Scalable keyset cursors | Implemented | search service | `k2` chronological/relevance keysets plus explicit bounded `m1` sidecar snapshots; no unbounded offset ceiling. |
| Recoll sidecar (replaces sist2) | Implemented | adapter | Optional external process; bounded retry, exact reconciliation, attributed stable merges, and status/UI telemetry. |
| Notebooks/tags/search notebooks | Implemented | Notrios service | All notes/Notes/Reports/Help/Trash bootstrap and protections. |
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
| Graph centrality/community detection | Exported, not implemented | graph service | v0.6 F5: `notriosctl graph export` writes CSV node and edge lists for Gephi, Cytoscape, NetworkX, and igraph, which are built for this. Notrios emits the graph rather than reimplementing a field inside a notes app. |
| Graph views | Implemented | UI + CLI | v0.6 F5, reframed away from a global canvas: a **local graph** of the notes one or two hops from the open note, a **hubs report written as a read-only note** in the builtin Reports notebook (`POST /api/v1/graph/report/note`, `notriosctl graph report --write-note`, explicit regeneration only), and **CSV export**. A ranked list reads the same at any library size; a global force-directed canvas does not. |
| Job control plane | Implemented | store + CLI + REST | v0.6 F6: schema-v18 `jobs` records one run of an importer or archive export. Records persist across a restart, the work does not — an interrupted import resumes through its own checkpoints, and a second resume mechanism would give two answers to one question. Cancellation is cooperative at a committed, checkpointed batch. `interrupted` is derived from a stale heartbeat, never stored. `notriosctl jobs status` has a documented exit-code contract (0/1/3/4/5/6) so `job-a && job-b` works without a scheduler. |
| Local replication journal | Implemented (local only) | store | v0.7 G4: schema-v19 explicit enrollment/snapshot boundary and transaction-local capture of canonical collection/document/revision/notebook/tag/membership/search-notebook/resource/provenance mutations into immutable `(replica_id, sequence)` operations. `target: none` before enrollment writes no history; rollback writes no operation. State-vector/gap/ack/pending/audit tables exist for later slices. No transport, merge, cryptography, REST, MCP, or UI surface is claimed. |
| State-vector admission core | Implemented (transport-neutral/local fixtures) | `internal/syncstate`, store | v0.7 G5/schema v20: fixed protocol/database/schema/capability compatibility, bounded vector comparison and deterministic missing ranges, disk-backed gap/dependency queues, exact duplicate/conflicting replay handling, atomic contiguous-vector and acknowledgement advancement, and three-replica model/durable convergence tests. No canonical merge/apply semantics, peer authentication, crypto, carrier, REST/MCP/UI, or background sync. |
| Starting a job over REST or MCP | Deliberately absent | — | Every job kind names a filesystem path, and no REST or MCP surface accepts one. A job record around an operation does not change what the operation does. MCP may watch; REST may watch and cancel; the CLI starts. |
| Read-only builtin notebooks | Implemented | store | v0.6 F5: `store.IsReadOnlyNotebook` replaces thirteen hard-coded `NotebookID == HelpNotebookID` comparisons and covers Help and the new Reports notebook. Deliberately **not** "is it builtin": the default Notes notebook is bootstrap-created and undeletable but its content is the user's, and a test pins that it is not read-only. |

## Import and export

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Joplin RAW import | Implemented | importer | J1 physical-line/canonical-title parser plus J2/J3 real-export relationship and million-note full-write evidence. |
| Obsidian vault import | Implemented | importer | Nested hierarchy, bounded batches, fingerprints/checkpoints, dry-run diffs, rename config, exact source bundle, and canonical links/anchors. |
| Twitter/X import | Implemented | importer | Provenance/thread/media import. |
| ChatGPT export import | Implemented | importer | Conversation provenance. |
| Claude JSON import | Implemented | importer | Conversation provenance. |
| Native archive v1 | Implemented | exporter | Query-scoped plain-note interchange; not lossless backup. |
| Shared selection/privacy planner | Implemented | archive/publishing service | Read-only typed recursive notebook/tag/query/ID selection, target policies, reachable resources, link/privacy/source-bundle/metadata decisions, bounded REST/MCP details, and deterministic digest. |
| Portable Markdown vault export | Planned v0.7 G19 bridge | movenotes-v3 | `notrios2sql.py` is intended to consume archive v2; movenotes owns Obsidian projection. The compatibility bridge waits for G9's sync-era container, and the importer was absent at the last check. |
| Native archive v2/backup | Implemented (export, verify, restore) | archive service | Schema-v12 database/replica identity, manifest-last SHA-256 objects, typed bounded records, explicit restore intent, and strict verification. `notriosctl export archive-v2` writes full-backup and explicitly scoped subset snapshots from one read transaction, stages privately, publishes the manifest last, resumes over published objects, and self-verifies. P3a moved the object inventory into checksummed index chunks under a two-level fanout and made writer and verifier stream through external-sorted spools, so an archive holds up to 8,000,000 objects and the manifest no longer grows with the library. P3b added the opt-in packed layout behind `objects.pack.v1`. P4 added `verify archive-v2` and `restore archive-v2`, proven on the attachment-bearing corpus under both layouts. |
| Joplin RAW export | Optional/research | exporter | Only for measured exact round-trip need. |

## Built-in UI and editor

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| React + Vite built-in UI | MVP | web UI | Basic browser client embedded in the service; becomes the Wails webview frontend. |
| Go/Wails v2 built-in GUI (`-no-gui`/`-gui-only`, themes) | Implemented | GUI | Desktop release shell. |
| Mermaid diagrams in current GUI | Planned v0.8 | GUI | `md-editor-rt` is capable, but Notrios currently sets `noMermaid: true` after the offline-assets hardening. Requires a pinned local renderer plus CSP, sanitization, size/error, browser, and Wails evidence before enablement. |
| `md-editor-rt` editor/preview | MVP | web UI | Initial polished editor; wrap behind an adapter. |
| Preview link interception | MVP | web UI | `document://` opens note; `resource://` opens/downloads resource. |
| Resource upload/paste | MVP | web UI + REST | Images/PDFs become local resources, not inline base64. |
| Preview sanitization | MVP | web UI | Notrios' own `normalizePreviewHTML` (DOMParser) passed as md-editor-rt's `sanitize` prop, over that library's built-in `xss`. Verified 2026-08-06 to strip `<script>`, `on*` handlers, and inline styles. `rehype-sanitize` is declared in `package.json` but unused and unusable — rendering is markdown-it, not unified. |
| Offline frontend assets | Implemented | web UI + service | v0.5 E6a: KaTeX, highlight.js, and cropper are bundled; echarts and prettier are off; `handleWebApp` serves a Content-Security-Policy with `script-src 'self'`. Third-party requests went 13 → 0 and 623 kB → 0, at 151 kB gzipped and ~400 ms of first contentful paint. `scripts/run_offline_assets_check.sh` fails if a remote asset returns, and was verified to fail on the pre-fix commit. |
| Math rendering (KaTeX) | Implemented | web UI | `$…$` and `$$…$$` render, offline included since E6a. FTS indexes the LaTeX source, not the rendered output — searching `mc^2` finds the note. |
| HTML paste to Markdown | Implemented | web UI | v0.5 E6b: a pasted HTML table becomes a Markdown pipe table, so blocks, link extraction, and portable export can see into it. Refuses merged cells, ragged rows, nested blocks, multi-line cells, and pastes that merely contain a table — every refusal falls through to the ordinary paste, so nothing pasted is lost. Parsing is inert `DOMParser`; no HTML is re-emitted. |
| Four-pane layout | Implemented | web UI | Accessible splitters and independent scrolling. |
| Wails v3/mobile migration | Planned v0.8 spike | GUI | Optional route, not the only mobile plan. Requires Wails v2 desktop parity and rollback; pre-1.0 mobile evidence is emulator-only. |
| Framework-neutral Go application facade | Planned v0.8 | core | Shared semantics for REST, Wails, and the no-GUI C ABI; transport adapters do not own business rules. |
| Versioned no-GUI Go C ABI/shared library | Planned pre-1.0 (v0.8-v1.0) | core/packaging | Opaque handles, bounded serialized calls, typed errors, cancellation/polling, streams, and explicit memory ownership. Android emulator only before 1.0. |
| Flutter/Go universal native client | Planned post-1.0 | GUI | Android/iOS/Linux/macOS/Windows over Dart FFI. `flutter_smooth_markdown` and secure storage are candidates pending evidence; Flutter Web uses REST or a separate Wasm adapter. |
| Editor link intelligence | Implemented | web UI + service | v0.5 E5: `GET /api/v1/links/suggest` (bounded title autocomplete, IDs and titles only) and `POST /api/v1/links/check` (unsaved-buffer link resolution through the canonical extractor). E6 added the in-editor half: `[[` autocomplete, wavy underlines on broken links that map through edits, and Ctrl-click to open a target. Everything degrades to nothing when the service is unreachable. |
| CodeMirror 6 + unified migration | Declined | web UI | v0.5 E6: `md-editor-rt` **is** CodeMirror 6 and exposes it, so the capabilities the migration was for cost 1.3 kB gzipped through its existing hooks. The remaining argument — dropping `@codemirror/language-data`'s 113 lazy chunks — does not justify re-implementing preview, sanitizer, toolbar, upload, and theming. `PROJECT_DECISIONS.md` 20. |
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
| MCP editor write tools | Implemented | MCP adapter | Tool-scope-gated with revision preconditions. |
| MCP protocol resources | Deliberately absent | MCP adapter | Notrios uses bounded tools and returned `document://`/`resource://` links; global `resources/list` would enumerate huge collections. |
| MCP read coverage | Implemented | MCP adapter | v0.6 F3 decided every withheld REST surface rather than inheriting the list. Added under `read-only`: `get_document_blocks`, `get_graph`, `find_graph_path`, `get_graph_report`, `run_note_query`, `get_lint_report`, `read_resource`. Still off, each with a recorded reason: fix, tag rename, notebook deletion, GC, archive operations, publication, purge — anything that writes outside the note model, deletes permanently, or acts on the whole library at once. |
| MCP resource reads | Implemented | MCP adapter | v0.6 F3: `read_resource` returns metadata plus a `resource://` URI by default; bytes only when asked for, only for text-like MIME types, and only within `mcp.max_document_bytes`. `offset`/`length` give a range read, and a slice ending mid-character is trimmed so the text stays valid UTF-8. Binary resources are described, never transcribed. |
| HTTP range requests for resource content | Implemented | resource service | v0.6 F3: `206` with `Content-Range`, `416` naming the real size when unsatisfiable, composing with `?download=1`. Implemented by delegating to `http.ServeContent` rather than reimplementing the RFC. |
| Tool visibility scopes | Implemented | MCP adapter | v0.6 F2: four cumulative scopes — `search-only`, `read-only` (default), `editor`, `organizer` — enforced **at the call site**, not only by filtering `tools/list`; before F2 a hidden read tool answered when called directly. One table drives both the listing and the check, and a test walks every registered tool against every scope so a tool cannot ship unclassified. No `administrator` scope: whole-library destructive operations are unreachable over MCP at any scope, so it would name an empty set. `mcp.default_profile` renamed to `mcp.default_scope`, old key kept as a deprecated alias with the narrower winning on conflict. A scope is a guardrail on one's own agent, not authorization. |
| LLM surgical edits | Implemented | document service | SEARCH/REPLACE, dry-run, revision preconditions. |
| Batch organizer transactions | Implemented | document service | v0.6 F1: `POST /api/v1/batch` over an explicit note list — move, add_tags, remove_tags, trash, restore, duplicate — in `atomic` or `best_effort` mode. Every requested item gets an outcome in both modes (applied / skipped / failed / rolled_back), so a caller always knows which half happened. `trash` is revision-preconditioned per item. `request_key` gives exactly-once through a persisted ledger (schema v17), replaying the first run's outcomes verbatim; a reused key with different arguments is refused. Bounded at 500 items, refused rather than truncated. Stable Markdown-link copy is not an operation here — it produces text for a clipboard rather than changing the library. |
| Sync MCP control plane | Planned v0.7 G15 | MCP adapter | Resolved policy: plan/start ordinary incremental sync, bounded resource fetch, and status/conflict inspection at sync scope; no keys, backups, restore, enrollment, retirement, purge, or bulk bytes in context. |

## Publishing and knowledge-base maintenance

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| Publication profiles | Implemented | planner/archive | Saved profiles (`notriosctl publish`), review-then-publish gated on the plan digest, current-note projection with link rewriting and dropped link records. Consuming the handoff downstream is the v0.7 compatibility bridge. |
| Quartz and scalable archive site | External integration | movenotes-v3/Ledger | Obsidian/Quartz for subsets; Hugo/Ledger+Bluge for large libraries. |
| Publishing dry run | Implemented | planner/UI | `publish plan` and `POST /api/v1/selection/plan` report included/excluded notes, resources, link decisions, metadata stripping, and warnings before anything is written. |
| Foam-style query blocks | Implemented | query service/UI | v0.5 E7: a fenced ```note-query block declares a Q1 query plus typed fields, sort, and limit, parsed server-side through `POST /api/v1/note-queries/run`. No SQL and no JavaScript; a block can express nothing its author could not type into the search box. Bounded at 100 rows with visible truncation; a malformed block renders its error inside the block rather than failing the note; a publication carries the block's text, never a result. |
| Workspace lint | Implemented | maintenance service | Eleven read-only checks through `notriosctl lint` and `GET /api/v1/admin/lint/report`; complete counts with capped examples, a library-wide digest, and content-free findings. |
| Workspace fix | Implemented | maintenance service | `notriosctl fix`, dry-run first, single-note and revision-preconditioned, every fix an ordinary revision. Repairs non-canonical link targets by default; alt text and remote-media localization are opt-in. Everything else stays reported. |
| Outline API | Implemented | document parser | Headings/line anchors; v0.5 E1 adds the block model underneath. |
| Hierarchical tag rename | Implemented | maintenance service | v0.5 E8: `store.RenameTag`, `POST /api/v1/tags/rename`, `notriosctl tags rename`. Dry run defaults to true, and it is a rolled-back apply rather than a prediction — the real statements run inside a transaction. Hierarchy matches by path segment (`projects` is not a child of `project`); renaming onto an existing name merges and reports it; saved searches mentioning the old name are warned about, never rewritten. Bounded at 500 tags, refused rather than truncated. No GUI surface. |
| Move a note between notebooks | Implemented | document service / UI | v0.6 F0 closed the client gap: `notriosctl notes move` (a name is refused when it matches more than one notebook) and a notebook control in the editor's own toolbar that shows where the open note lives and files it elsewhere on selection. Store, REST, and MCP already had it. Multi-note move is F1. |
| Create a note in the selected notebook | Implemented | UI | v0.6 F0: the sidebar selection is the creation target and the search pane's **New note** names it. Selection is tracked by notebook ID, never the row's `notebook:"<name>"` query — names are unique only among siblings. "All notes", saved searches, and Help fall back to the default "Notes" notebook, which is correct for a view rather than a place. |
| Trash-first delete/restore in the GUI | Implemented | UI | v0.5 E8: Move to Trash, Restore, and Delete forever in the editor pane, with a badge that distinguishes a trashed note from a permanently read-only one. Notebook deletion confirms with `GET /api/v1/notebooks/{id}/deletion-preview` — the service's counts and its re-homing rule — rather than a generic confirmation. |
| Note templates | Implemented | document service | v0.6 F4: a template is an ordinary note carrying a ```note-template block declaring `prompt:` lines. Placeholders are `{{name}}` over a **closed** vocabulary — declared prompts plus `date`, `time`, `datetime`, `title`, `notebook`. Substitution is replacement, never evaluation: no arithmetic, no conditionals, no filesystem reach, and supplied values are inserted once and never re-scanned. An unknown placeholder is an error on the template, reported at listing time rather than at use. |
| Task extraction | Implemented | document service | v0.6 F4: checkbox list items computed on read from note bodies — `document_blocks` stores a hash and offsets but not text, so the body is read either way, and no table exists until a query needs one a scan cannot serve. Identity is the block model's, so a task survives edits around it; ticking it changes the derived ID, and an author-written `^marker` is the address that survives completion. Counts are complete even when rows are capped. |
| Graph visualization (a graph *view*) | Delivered differently in v0.6 F5 | UI | v0.5 E4 shipped the traversal, shortest-path, and orphan/hub **data**. F5 supplied the view, but not as a canvas: a local neighbourhood list, a hubs report as a note, and CSV export for tools built for large graphs. A global canvas is explicitly out — it stops being readable at Notrios' target scale, which is the scale the feature was for. |
| Link reference definitions | External integration | movenotes-v3 | Generated in downstream portable Markdown projection. |

## Versioning and sync

| Feature | Status | Primary owner | Notes |
|---|---:|---|---|
| SQLite revision restore | Implemented | document service | Restore creates a new revision. |
| Named runtime profiles and multi-instance isolation | Implemented v0.7 G3 | config/service/CLI | Version-2 local registry plus one `0600` config per runtime profile; explicit profile/database/replica binding, isolated paths and loopback ports, safe `none` target, copied-database adopt/fork gate, startup validation, active-profile status, and no REST/MCP management surface. |
| Native record-level sync | Planned v0.7 | sync service | Transactional change log, contiguous state vectors, Notrios-owned pure-Go revision merge and optional named-parent constrained-VCDIFF transfer delta (G1a evidence; production still gated), G2's bounded NCB1 envelope candidate and 1 MiB resource chunks (also production-gated), lazy resources, acknowledgement/retention; see `PLAN.md` G0, G1, G1a, and G2-G20. |
| Ephemeral shared-directory transport | Implemented v0.7 G11, conformance-tested G12 | `internal/synccarrier`, CLI | Two replicas exchange encrypted signed artifacts through a disposable folder and converge; `notriosctl sync init/bundle/pair/status/discover/once` runs it. Every path segment below `notrios-sync/v1/` is a keyed blind, each replica writes only its own namespace, artifacts are named by what they logically are, and a publisher repairs its own unreadable copy. Discovery reports candidates and never enrols. G12 ran it against a mounted Google Drive folder and a drive passed between peers: all eight phases passed, and the measured 45-57 second visibility delay makes publication order a latency optimization rather than a correctness mechanism. No watcher or scheduler (G15), no snapshot transfer (G14). |
| REST peer authentication | Implemented v0.7 G13 | `internal/syncauth`, httpapi, store | Schema v25: a peer principal is one enrolled replica of one database, proved by an Ed25519 signature over the method, path, database id, replica id, timestamp, nonce, and body hash — no bearer token, no session, no user. It authorizes `/api/v1/sync/...` for that database and nothing else, asserted by comparing an ordinary route's answer with and without it. Pairing is a short-lived single-use code under which the group key travels sealed; the transport policy refuses startup rather than warning; refusals are uniform and audited under a closed vocabulary. |
| REST sync data plane | Planned v0.7 | sync service | The same object/envelope protocol over the authenticated surface above; G14 owns envelopes, objects, and resumable encrypted backup download. `rclone` remains a conformance carrier only, and target `none` stays the default. |
| Sync key material | Development provider v0.7 G11, narrowed G13 | `internal/synckeys` + store | A `0600` JSON file holding **secrets only** — one group key per epoch and this replica's Ed25519 signing key — refusing to open if other users can read it. Which peers are trusted is schema-v25 database state, so enrolling and revoking are transactional and audited. Explicitly not an OS keychain: v0.8 selects the platform store. |
| Snapshot catch-up/reset | Planned v0.7 | archive/sync | Signed request, encrypted archive-v2 snapshot/ZIP, explicit restore intent, then incremental replay from its state-vector boundary. |
| Install/config/mobile portability | Planned v0.8 | packaging/GUI | Installed-path/permission/credential-store work, shared-core/C-ABI build, Android-emulator smoke, and separately gated Wails v3 spike. Physical mobile release gates are post-1.0. |
| Backup/restore replace/merge/fork/adopt | Implemented | archive/sync | No default; verification completes before writes; replace/adopt/fork rotate replica identity while in-place merge retains the target replica. A schema-v13 `restore_state` marker makes an interrupted restore visible, and only `replace` recovers it. |
| Yjs-compatible live co-editing | Optional/research | editor service | Separate from database sync. |
| go-git/Fossil checkpoints | Optional/research | version adapter | Projection history, never canonical sync. |
| Bidirectional external-vault sync | Optional/research | sync adapter | Explicit ownership/conflict policy. |
| Nostr/BLE courier transports | Optional/research | transport adapter | Post-v1; REST+rclone first. |
