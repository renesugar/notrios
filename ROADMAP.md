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
- Schema: nested notebooks with emoji icons, tags with counts, query-backed
  search notebooks (All notes first, Trash last, plus user queries), a regular
  read-only Help notebook, case-insensitive names, and trash/restore/purge.
- Schema: source provenance for Joplin, Obsidian, Twitter/X, ChatGPT, and Claude, including conversation threads (author, author ID, thread ID, reply-to, post URL).
- Query-language adapter: `notebook:`, `tag:`, `author:`, `authorid:`, `title:`, `since:`, `until:`, phrases (see `SEARCH_QUERY_LANGUAGE.md`).
- Recoll integration replacing sist2: projection + outbox, generated Recoll config, from-scratch front-matter handler, external-process adapter (see `RECOLL_INTEGRATION.md`).
- MCP/REST expansion sufficient for full third-party clients (C++/Qt, Go/Wails, Rust/Tauri).
- Twitter/X, ChatGPT, and Claude importers.
- Query-scoped export preserving notebook structure; import dry run with rename-on-import configuration file.
- Go/Wails built-in GUI: menu bar, notebooks/tags sidebar, incremental search results, Markdown editor + preview, nested notebooks, light/dark toggle with user-defined custom themes, `-no-gui` and `-gui-only` modes.
- Documentation site: `docs/` published to GitHub Pages with PageFind search; Help notebook seeded from the same content.
- GitHub release preparation for `github.com/renesugar/notrios`.

## v0.3 — Import, resource, and media hardening (complete; archived under `plans/v0.3/`)

(Deferred former v0.2 draft; see `plans/v0.2/001-import-resource-media-hardening.md`.)

- Joplin RAW and Obsidian importer hardening (larger fixtures, resume/checkpoint, dry-run diffs).
- Remote media localization; import-time and UI-triggered localization use the same policy engine.
- Domain stop list and redirect-domain checks.
- Quarantine store.
- Exact-hash deduplication.
- Local perceptual-hash database hooks for moderation and near-duplicate review.
- Resource garbage collection and retention policy.
- Keyset pagination, matching indexes, and generated 10k/100k/500k performance
  profiles before importer scale hardening.
- Recoll hardening: batched incremental scans, periodic reconciliation, FTS5/Recoll search result merging, extraction status in UI.

## v0.4 — Portable data, publishing, and stable references

- Correct Joplin RAW physical-line/title/property parsing against real exports,
  then add attachment-aware and million-note full-import performance evidence;
  synthetic dry-run profiles alone are not a throughput claim.
- Native archive v2: a versioned manifest plus immutable, hash-addressed
  objects, source-preservation bundles, checksums, capability/version bounds,
  snapshot consistency, and streaming read/write. This is the full backup and
  transfer format and deliberately becomes the container layer reused by v0.7
  synchronization; a foreign Markdown/Joplin/Obsidian export remains a lossy or
  format-limited projection.
- Publication profiles for selected notebooks/folders/tags, including recursive
  subnotebook selection, emit a privacy-reviewed scoped native-archive handoff.
- Add a documented archive-v2 compatibility fixture for a separately maintained
  `movenotes-v3/notrios2sql.py` importer. `movenotes-v3` owns portable Obsidian
  projection, Quartz generation for curated/smaller subsets, and Hugo with
  `hugo-theme-ledger` plus Bluge for large libraries. Notrios does not duplicate
  those exporters, generators, or search indexes.
- Link-to-private-note policy.
- Public resource reachability analysis.
- Dry-run publishing privacy checks.
- Optional Foam-style query/dashboard materialization remains a later
  projection through the publishing boundary.
- Stable external `notrios://` document links including the portable logical
  database identity; the OS handler resolves it to local profiles and handles
  ambiguity/stale targets.
- Share a neutral selection/link/resource/privacy planner among native archive,
  subset transfer, and publication-handoff targets.
- Extend the shared query language with uppercase `OR`, implicit `AND`, prefix
  `-negation`, parentheses, and quoted phrases. Add `category:` as an alias for
  `notebook:`; `category:"All notes"` and `notebook:"All notes"` search all
  current notes. SQLite FTS5 and Recoll must compile the same bounded expression
  tree and must not silently approximate unsupported operators.

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
- Evaluate character-level live collaboration separately. A Yjs-compatible
  Ygo library can be useful for simultaneous editing, but it must not become
  the whole-database sync format or a prerequisite for ordinary offline sync.

## v0.6 — MCP and automation expansion

- Complete MCP write-tool coverage gated by explicit scopes.
- Resource graph resources/read support.
- LLM-safe surgical edits with dry-run and revision preconditions.
- REST and MCP batch transactions for move, duplicate, trash, tag/untag, and
  stable Markdown-link copy. Requests are bounded, idempotent, support
  all-or-nothing versus best-effort modes, and return per-item outcomes.
- LLM-safe SEARCH/REPLACE edits with dry-run and revision/hash preconditions.
- Tool visibility profiles: search-only, read-only, editor, organizer, administrator.
- MCP starts/statuses bulk export/import/sync jobs but does not carry unbounded
  archive or blob bytes in model context; REST/object transfer remains the data
  plane.

## v0.7 — Versioning and synchronization

This is a multi-slice milestone; each slice gets a separate active plan and
user approval. See `VERSIONING_AND_SYNC_POLICY.md`.

1. **Profiles and identities** — explicit profiles, logical database UUID,
   replica/device UUID, schema/protocol compatibility, bootstrap notebooks,
   validated `notrios://` routing, and a `sync: none` target.
2. **Replication core** — immutable operations identified by
   `(replica_id, sequence)`, hybrid logical clocks for deterministic conflict
   ordering, per-replica acknowledgement vectors for completeness and GC,
   idempotent apply, record/field LWW registers, set membership tombstones,
   body-snapshot conflict copies, and deterministic notebook-tree repair.
3. **Native snapshot/change container** — reuse archive v2 manifests and object
   storage for full snapshots and bounded change envelopes; blobs publish
   before references and manifests publish last.
4. **REST transport and folder/rclone transport** — the same protocol over
   authenticated REST and immutable shared-folder objects. `rclone copy
   --immutable` is a carrier; `rclone sync`/bisync are not the merge algorithm.
   Same-machine folders, removable drives, and cloud remotes all use the same
   inbox/outbox layout.
5. **Operations and recovery** — durable outbox, retries/backpressure,
   peer retirement, tombstone/blob GC watermarks, full-resync after retention
   horizon, conflict UI, replace/merge/adopt/fork restore, and fault-injection
   convergence tests.

Research outcomes:

- A small Notrios-specific Go replication library is preferred over Marmot:
  Marmot's HLC, immutable CDC segments, manifest-last publication, and
  anti-entropy are useful patterns, but its always-on SQL-cluster/2PC/CDC stack
  does not match intermittent mobile/folder/rclone peers.
- Cachapa's record-level HLC/LWW approach is the closest conceptual reference,
  but the Dart packages are not adopted or ported wholesale. Notrios also needs
  per-replica sequence vectors, tree invariants, immutable resources, revision
  conflicts, retention acknowledgements, and Go/mobile test fixtures.
- Yjs-compatible Ygo is reserved for optional live co-editing: per-character
  CRDT history is too costly and semantically mismatched for whole-database
  import/export/sync. Re-evaluate maintained Go implementations when that
  separate feature is planned.
- Nostr and bitchat-inspired transports are post-v1 research. Their signed
  envelopes, outboxes, dedupe IDs, TTLs, acknowledgement, and opportunistic
  courier patterns are useful; public relay metadata/retention and BLE
  bandwidth/platform limits make them inferior to REST plus rclone for the
  first supported sync transports.
- Optional go-git/Fossil/Obsidian adapters remain projections/checkpoints, not
  the canonical merge protocol.

## v1.0 — Feature-complete local product

- Stable REST API.
- Stable MCP tool/resource schemas.
- Large-scale performance tests with hundreds of thousands of documents/resources.
- Installer/package story.
- Backup/export/restore/sync compatibility and disaster-recovery validation.
- Security review for remote media and MCP.
- Usable documentation for Gitea/GitHub public release.
- Desktop remains on stable Wails v2 until a separately approved Wails v3
  migration spike passes desktop regression and real Android tests. Wails v3
  currently offers a shared desktop/iOS/Android codebase, but v3 and mobile are
  pre-release/experimental and Android/iOS impose mobile storage, lifecycle,
  background, and file-dialog constraints.

## Future candidates

- Native third-party clients over the public REST/MCP API (C++/Qt, Rust/Tauri, additional Go/Wails clients).
- LadybugDB derived graph backend for advanced graph traversal and analytics.
- Semantic/vector search.
- More importers.
- Multi-user deployment.
- Enterprise policy administration.
- Encrypted Nostr relay and BLE/opportunistic-courier sync transports after the
  core protocol, threat model, and constrained-device benchmarks are stable.


## Agent handoff status

The scaffold handoff is complete; see `CODING_CLIENT_HANDOFF.md`. Future roadmap planning should be driven from `ROADMAP.md`, but each active implementation cycle should create a small `PLAN.md` slice and archive it under `plans/` when complete.


## v0.1 completion note

The v0.1 MVP, v0.2 redesign, and v0.3 hardening milestones are implemented and
archived. v0.4 J1–J3 and Q1 are implemented; P1 and the remaining v0.4 product
tasks in `PLAN.md` require approval.
