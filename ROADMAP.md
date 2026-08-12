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

## v0.4 — Portable data, publishing, and stable references (complete; archived under `plans/v0.4/`)

- Correct Joplin RAW physical-line/title/property parsing against real exports,
  then add attachment-aware and million-note full-import performance evidence;
  synthetic dry-run profiles alone are not a throughput claim.
- Native archive v2: a versioned manifest plus immutable, hash-addressed
  objects with an out-of-manifest object index, source-preservation bundles,
  checksums, capability/version bounds, snapshot consistency, and bounded
  streaming read/write at real-library scale. This is the full backup and
  transfer format and deliberately becomes the container layer reused by v0.7
  synchronization; a foreign Markdown/Joplin/Obsidian export remains a lossy or
  format-limited projection.
- Publication profiles for selected notebooks/folders/tags, including recursive
  subnotebook selection, emit a privacy-reviewed scoped native-archive handoff.
- Emit a privacy-reviewed, checksum-verified subset archive that a downstream
  toolkit can consume. `movenotes-v3` owns portable Obsidian projection, Quartz
  generation for curated/smaller subsets, and Hugo with `hugo-theme-ledger` plus
  Bluge for large libraries; Notrios does not duplicate those exporters,
  generators, or search indexes. The published compatibility contract itself
  (JSON Schemas, pinned fixtures, cross-version consumer tests) moved to v0.7 —
  see below.
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

## v0.5 — Better editing and graph UX (complete, 0.5.0)

Delivered in thirteen slices, all archived under `plans/v0.5/`: addressable blocks
(E1), heading anchors in stable links (E1a), scheme-scoped anchor decoding
(E1b), workspace lint (E2), workspace fix (E3), bounded graph traversal with
shortest paths and an orphan/hub report (E4), editor link intelligence (E5), the
CodeMirror decision — stay (E6), offline-first frontend assets (E6a), HTML table
paste normalization (E6b), embedded query blocks (E7), organizer UX (E8), and
this wrap-up (E9).

Two bullets below did **not** ship and are moved to v0.6 rather than left
ambiguous:

- **Templates and task extraction** never entered `PLAN.md` as a task.
- **Graph *visualization*** — E4 delivered the traversal, path, and report data
  and named itself "visualization data"; there is no graph view in the GUI.

E6a and E6b were added mid-milestone from what E6 measured and from a user
question, and are not on the original list.

- Consider migration from `React + md-editor-rt` to `React + CodeMirror 6 + unified/remark/rehype` if deeper editor-pane behavior is needed.
- Rich link autocomplete.
- Broken-link underlines while editing.
- Block anchor database and navigation, including heading anchors: a
  `notrios://` or `document://` link addresses a section by slug and a block by
  ID or author-written marker, following Obsidian's model of naming a heading
  and a block from outside the note while keeping Notrios' rule that a stable
  link carries no percent-encoding.
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

- ~~Note templates and task extraction (moved from v0.5)~~ — done as F4.
- ~~Graph views that stay readable at scale, over the v0.5 E4
  traversal/path/report data (moved from v0.5)~~ — done as F5: a **local** graph
  around the open note, a **Top N hubs report written as a read-only note** in a
  new builtin Reports notebook, and **CSV node/edge export** for tools built for
  large graphs. Explicitly *not* a global canvas — Obsidian's degrades into an
  unreadable hairball past a few thousand notes, while its local graph stays
  useful at any size, and Notrios targets libraries far larger than that.
  Analysis that wants centrality or community detection belongs in Gephi or
  Cytoscape, which are built for it.

  F5 also carried the three surfaces that share the read-only-notebook
  predicate — the graph report, publication handoffs, and lint — and fixed a
  pre-existing defect: trashed notes' links still counted toward the graph
  report's in-degree, because soft delete deliberately keeps `document_links`
  so a restore can use them.
- ~~Notebook targeting: create into the selected notebook, and a single-note
  move control in the GUI and CLI~~ — done as F0.
- ~~Complete MCP write-tool coverage gated by explicit scopes~~ — done across
  F1, F2, F3, and F7. F7's reconciliation found six REST surfaces F3's pass had
  missed and decided each; `tag_note`/`untag_note` were added at `editor`,
  because tagging one note previously required `organizer` through `run_batch`.
  The rest are recorded as withheld with a reason in `docs/api/mcp.md`.
- ~~Resource graph resources/read support~~ — done as F3: read-shaped surfaces
  became MCP tools (blocks, graph, paths, the graph report, query blocks, the
  lint report) and `read_resource` reads attachment metadata plus bounded,
  range-addressable text. HTTP `Range` landed in REST at the same time.
- ~~Job control plane for bulk work~~ — done as F6: schema-v18 job records for
  the importers and archive export, cooperative cancellation at a durable batch
  boundary, and a documented `notriosctl jobs status` exit-code contract instead
  of a scheduler. Narrower than planned in one place: jobs are started from the
  CLI only, because every kind names a filesystem path and no REST or MCP
  surface accepts one.
- ~~LLM-safe surgical edits with dry-run and revision preconditions~~ — shipped:
  `edit_note` and `PATCH /api/v1/documents/{id}` take `edits`, honour `dry_run`,
  and require `base_revision_id` for anything that writes.
- ~~REST and MCP batch transactions for move, duplicate, trash, tag/untag, and
  stable Markdown-link copy~~ — done as F1: bounded to 500 items, idempotent
  through a persisted ledger, atomic or best-effort, per-item outcomes. Stable
  Markdown-link copy is deliberately **not** an operation here: it produces text
  for a clipboard rather than changing the library.
- ~~LLM-safe SEARCH/REPLACE edits with dry-run and revision/hash
  preconditions~~ — shipped as the same `edit_note` surface above; ambiguous
  search text is refused rather than guessed at unless `replace_all` is set.
- ~~Tool visibility profiles: search-only, read-only, editor, organizer~~ —
  done as F2, cumulative, with enforcement at the call site rather than only in
  the listing. (The
  original list said five, including `administrator`. Resolved 2026-08-07: no —
  Notrios is single-user, so administrator and author are the same person, and
  destructive whole-library operations stay a deliberate act on the command
  line. A profile that cannot be selected is not a profile.)
- **MCP *watches* bulk import/export jobs; it does not start them.** Half of
  this bullet shipped as F6 and half was refused on the evidence. Every job kind
  names a filesystem path, and no REST or MCP surface accepts one — a job record
  around an operation does not change what the operation does. The second half
  of the bullet, "does not carry unbounded archive or blob bytes in model
  context", holds and is why the first half could not: bulk bytes and bulk paths
  travel the same way. Starting a job stays a CLI act.

## v0.7 — Versioning and synchronization

This is the first milestone that merges independently changed canonical state,
so the replacement `PLAN.md` divides it into **twenty-two independently
approvable items (G0, G1, G1a, and G2-G20)** rather than the former six
implementation groups.
The user's 2026-08-11 review resolved the G0-G17 policy decisions; no item is
approved for implementation merely by resolving its decisions.
Every completed item leaves a verified ZIP in the evidence directory and waits
for approval before the next begins. See `VERSIONING_AND_SYNC_POLICY.md` and
`SYNCHRONIZATION.md`.

**G0 completed 2026-08-11.** Its threat model, glossary, thirty-case control
trace, and primary-source dependency/platform/license validation are under
`performance/v0.7-g0/` and archived in `plans/v0.7/`. The review requires an
encryption-epoch advance after replica compromise/revocation and keeps
plaintext content hashes out of carrier-visible routing. These are design
requirements, not live controls.

**G1 completed 2026-08-11.** Aggregate-only evidence over 2,063,061 bodies and
deterministic divergence fixtures selected complete UTF-8 revision objects,
optional beneficial named-parent line deltas, and bounded line-first merge with
word-token conflict-region refinement. Overlap remains a typed conflict; no
merge dependency or sync runtime landed. A later review found that the probed
merge package does not solve binary delta encoding, so G1a separately evaluated
a Notrios-owned pure-Go xdelta/VCDIFF implementation and made the G7 line/word
merge Notrios-owned as well.

**G1a completed 2026-08-11.** Its investigation-only pure-Go matcher and
bounded codec produced exact deterministic results across 21 text/binary
fixtures. The recommendation is constrained RFC 3284 default-table VCDIFF,
not Subversion svndiff or a private container, only from a real named parent
and only when materially smaller than complete bytes. External xdelta3 and
open-vcdiff decoded all representative Go streams exactly. The prototype stays
under `performance/`; later G7/G8 must separately approve any production
rewrite or promotion.

**G2 completed 2026-08-11.** Aggregate-only 100/10k/100k operation and resource
evidence selected compact NCB1 operation records plus a canonical-JSON outer
manifest candidate and deterministic gzip profile. Envelopes close at 10,000
operations, 16 MiB canonical bytes, or 4 MiB compressed bytes. Resources stay
whole below 1 MiB and use 1 MiB fixed chunks above it; pending admission and
sync packs have explicit disk/count/size bounds. Android numbers remain
provisional behind the v0.8 emulator and post-1.0 physical-device gates. No
production sync code or dependency landed.

**G3 completed 2026-08-12.** Named runtime profiles now bind owner-only
per-profile configs to explicit database/replica identities, isolated absolute
paths and loopback ports, a public URL, and a safe `sync.target: none` default.
CLI create/show/list/validate/start surfaces, copied-database adopt/fork gates,
startup revalidation, active-profile UI/status, stable-link routing, and a real
two-daemon smoke are live.

**G4 completed 2026-08-12.** Schema v19 now establishes an explicit enrollment
and full-snapshot boundary, then captures every sync-relevant canonical row
mutation through one transaction-local journal seam with exact per-replica
sequences. Rollbacks retain neither canonical nor operation rows; `target:
none` before enrollment remains journal-free. The 100k import A/B recorded
23.9% elapsed and 92.8% database-byte overhead for 300,000 operations. No
transport, admission/merge engine, encryption, or UI landed.

**G5 completed 2026-08-12.** Schema v20 and the transport-neutral sync-state
core now enforce the fixed database/protocol/schema/capability handshake for
already configured fixture peers, bounded vector comparison and deterministic
missing-range planning, exact replay detection, disk-backed gap/dependency
queues, and transactional contiguous-vector/acknowledgement advancement. Three
local replicas converge after shuffled, duplicated, and delayed delivery; a
gap, rejection, restart, or injected rollback cannot claim progress. No
canonical conflict/merge rule, transport, crypto, REST/MCP/UI surface, or
automatic peer enrollment landed; G6 is next and approval-gated.

- **Evidence before contracts (G0-G2, including G1a):** threat model and
  reference validation; representative revision/delta/three-way-merge
  workloads; pure-Go binary-safe xdelta/VCDIFF feasibility and hostile-decoder
  bounds; deterministic envelope/compression, resource chunk, queue, and
  provisional mobile bounds.
- **Identity and local replication (G3-G8):** multiple named runtime profiles
  and isolated server instances; transactional operation journal; contiguous
  state vectors and gap planning; deterministic metadata/set/tree/delete
  convergence; complete note-revision objects with optional transfer deltas and
  visible three-way conflicts through Notrios-owned pure-Go implementations;
  lazy, hash-verified resource materialization, with named-parent binary deltas
  considered only if G1a/G2 evidence justifies them.
- **Secure container and catch-up (G9-G10):** archive-v2 change-envelope
  capabilities; mandatory authenticated encryption and per-replica Ed25519
  signatures; signed backup requests; encrypted snapshot/ZIP catch-up and reset
  followed by incremental replay from the snapshot vector.
- **Two carrier adapters, one protocol (G11-G15):** an ephemeral shared
  directory that can be deleted and reconstructed, with immutable per-replica
  advertisements/requests/artifacts; Google Drive/removable-media conformance
  without an rclone dependency; REST authentication/pairing/TLS/quota/audit
  foundation; resumable REST objects and backup downloads; durable jobs,
  retry/backpressure, and bounded MCP status/control.
- **User recovery, safe retention, and portability handoff (G16-G18):** pairing/profile/directory,
  encrypted-backup, lazy-resource, conflict and reset UI; explicit peer
  retirement and acknowledgement/snapshot-gated tombstone/blob GC; a platform,
  shared-core/C-ABI, and Mermaid contract handed to the distinct v0.8 milestone
  and post-1.0 Flutter client.
- **Compatibility and completion (G19-G20):** publish the archive-v2 contract
  deferred from v0.4 P6 after the sync-era container stabilizes, then run full
  multi-peer convergence, security, disaster-recovery, large-corpus, API/docs,
  and release-package reconciliation.

The shared directory is disposable transport state, never canonical storage.
`rclone copy --immutable` may exercise the mapped Google Drive during tests;
`rclone sync`/`bisync` are not the merge or deletion algorithm, and mobile does
not depend on rclone. Subversion contributes state-vector/change-log and
base-delta ideas, not its dump grammar: its xdelta matcher, its svndiff format,
and RFC 3284 VCDIFF are distinct candidates that G1a compared before selecting
the constrained RFC profile recommendation. Archive v2
already supplies Notrios' verified content-addressed container.

Research outcomes:

- A small Notrios-specific Go replication library is preferred over Marmot:
  Marmot's HLC, immutable CDC segments, manifest-last publication, and
  anti-entropy are useful patterns, but its always-on SQL-cluster/2PC/CDC stack
  does not match intermittent mobile/directory/REST peers.
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

## v0.8 — Installation, configuration, shared core, and portability

This is deliberately separate from synchronization correctness. Packaging
changes which directories, credentials, ports, background work, deep links, and
file pickers an installed application may use, and those permissions must not
be smuggled into v0.7 as desktop assumptions.

- **H0 investigation — application facade and C ABI.** Audit the existing
  `internal/service`/HTTP split, validate `c-shared`/`c-archive`, SQLite/cgo,
  ownership, cancellation, threads, packaging, and Android-emulator premises,
  then freeze the minimal versioned contract described in
  `FLUTTER_GO_CLIENT.md`. This investigation comes before a bridge because the
  desktop, Android, and iOS packaging costs differ materially.
- **H1 shared core — no-GUI Notrios library.** Extract one transport-neutral
  application facade used by REST and a small `cmd/notrioslib` wrapper. Build a
  C ABI with opaque instance/stream handles, bounded serialized calls, typed
  errors, cancellation/polling, capability/version query, and explicit buffer
  ownership. Keep bulk data on bounded stream/range paths. Validate desktop
  libraries and one Android-emulator host; document unsupported platforms
  honestly. The pre-1.0 artifact is a backend/library, not a Flutter app.
- **H2 current-GUI Mermaid evidence and enablement.** Start from the measured
  fact that Notrios sets `noMermaid: true`. Pin and bundle the optional renderer
  locally, preserve CSP/sanitization and zero-CDN behavior, bound malformed and
  oversized diagrams, and test fixtures in a browser and Wails before changing
  the feature status. Keep fenced source visible on failure.
- Select self-contained application-data/config/cache locations per OS and
  migrate source-checkout defaults without losing data.
- Package the built web UI and required SQLite/runtime dependencies with the
  application; define upgrade, uninstall, and profile discovery behavior.
- Validate multiple profiles/server instances, loopback ports, URL handlers,
  shared-directory access, firewall prompts, and native file/directory pickers
  in installed Linux, Windows, and macOS builds.
- Select and test native credential-store implementations behind v0.7's secret
  interface. `zalando/go-keyring` currently documents macOS, Linux/BSD, and
  Windows only and is not the Android answer. Record `flutter_secure_storage`
  only as a post-1.0 Flutter-side candidate with platform-specific prerequisites.
- Run a separately approved Wails v3 migration spike. Wails v3 is currently
  beta for desktop; Android/iOS support is explicitly experimental. Preserve
  Wails v2 until desktop regression, dependency/license, and rollback gates
  pass.
- Before 1.0, mobile work stops at an Android emulator: compile/load the shared
  library and smoke instance lifecycle, SQLite, CRUD/search, a bounded resource
  stream, cancellation, and sync capability negotiation. This is architecture
  evidence, not a supported mobile release or a battery/background claim.
- Physical Android and all iOS client validation follow after 1.0. Record the
  scoped-storage/Storage Access Framework, sandbox/database/assets, secure
  storage, lifecycle/background transfer, notification, memory/disk/battery,
  pairing, catch-up, incremental-sync, and encrypted-backup gates now.
- Produce installable prerelease artifacts for internal evidence, not a public
  GitHub release.

## v0.9 — Release-candidate hardening

- Cross-platform upgrades and profile/data migration from source builds and
  earlier prereleases.
- Installer signing/notarization policy, SBOM and dependency/license/security
  audit, reproducible artifact metadata, rollback and disaster-recovery drills.
- Long-running directory/REST/mobile soak tests, compatibility matrix, support
  bundle/redaction, crash reporting policy, and release documentation.
- Freeze REST, MCP, archive, sync, configuration, installer, and shared C ABI
  compatibility candidates for 1.0. Run ABI ownership/leak/double-free,
  wrong-handle, concurrent-shutdown, cancellation, and stream-limit tests.

## v1.0 — Feature-complete local product

- Stable REST API.
- Stable MCP tool/resource schemas.
- Large-scale performance tests with hundreds of thousands of documents/resources.
- Installable signed artifacts for supported desktop platforms, with an
  explicitly documented mobile support level.
- Versioned no-GUI Notrios library/header artifacts for the pre-1.0 supported
  platform matrix, with lifecycle, ownership, threading, error, stream, and
  compatibility examples. An Android-emulator result is not labelled physical
  Android support; unsupported iOS artifacts are not implied.
- Backup/export/restore/sync compatibility and disaster-recovery validation.
- Security review for remote media and MCP.
- Usable documentation for Gitea/GitHub public release.
- Create a user-authorized GitHub release for `github.com/renesugar/notrios`:
  version tag, checksums, signatures, SBOM/provenance, release notes, upgrade and
  rollback instructions, installable artifacts, and readback verification.
- Desktop remains on stable Wails v2 until a separately approved Wails v3
  migration spike passes desktop regression; any physical Android claim waits
  for the post-1.0 device gate. Wails v3
  currently offers a shared desktop/iOS/Android codebase; v3 desktop is beta
  and mobile remains experimental, with Android/iOS storage, lifecycle,
  background, credential, and file-dialog constraints.

## Post-v1.0 — Flutter/Go universal native client

- Start with a separately approved dependency and architecture spike. Evaluate
  `flutter_smooth_markdown` for Markdown round-trip fidelity, Notrios links and
  resources, its Mermaid grammar, sanitization, accessibility, large-note
  performance, maintenance, and BSD-3-Clause-compatible dependency tree; its
  advertised feature list is not acceptance evidence.
- Build one Flutter UI for phones, tablets, laptops, and desktops over the
  versioned Notrios C ABI on Android, iOS, Linux, macOS, and Windows. Keep
  profile, job, stream, sync, and secret-store lifecycle explicit.
- Validate physical Android first, then iOS when Apple/Xcode hardware is
  available, then the desktop targets. Responsive layout, touch/keyboard,
  accessibility, lifecycle/background behavior, file pickers, secure storage,
  deep links, pairing, catch-up, sync, conflict resolution, and encrypted
  backup are release gates.
- Flutter Web is not covered by `dart:ffi`. A web build continues over REST or
  requires its own approved Go-Wasm/JavaScript-interoperability investigation.
- Wails mobile remains an alternative if it reaches production quality; the
  two clients share the core contracts rather than making either UI canonical.

## Future candidates

- **Multi-user roles over a shared service** (administrator, author, reviewer),
  for several people using GUI-only instances against one remote Notrios. This
  would go further than either reference implementation: Joplin offers only
  read-only versus read-write per shared notebook, and Obsidian states plainly
  that fine-grained permissions are not supported — its collaborators all get
  the owner's rights. It needs an `author` concept first, which Notrios has no
  field for today; see `agent/OPEN_QUESTIONS.md`.
- Native third-party clients over the public REST/MCP API or the versioned C ABI
  (C++/Qt, Rust/Tauri, additional Go/Wails clients).
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
archived, and so is v0.4: J1–J3, Q1, P1, P2, P3, P3a, P3b, P4, P5, P7, and P8
are under `plans/v0.4/`, with P6 deferred to v0.7 G19 after G9 stabilizes the
sync-era container. v0.5 is complete and
archived under `plans/v0.5/`, including a copy of its own plan at
`plans/v0.5/000-v0.5-plan.md`. Two v0.5 roadmap bullets did not ship and moved
to v0.6: note templates with task extraction, and a graph *view* in the GUI (E4
delivered the traversal, path, and report data it is built on).

**v0.6 is complete and archived under `plans/v0.6/`** (F0–F7), product version
0.6.0, schema v18. Every v0.6 bullet above is reconciled against the code. One
shipped **half**: MCP watches bulk jobs but does not start them, because every
job kind names a filesystem path. That is recorded in the bullet rather than the
bullet being marked done, and it does not move to v0.7 — it is a decision, not
an omission.

`PLAN.md` now holds the v0.7 plan.
