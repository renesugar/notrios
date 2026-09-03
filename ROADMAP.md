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

## v0.7 — Versioning and synchronization (complete, 0.7.0)

This is the first milestone that merges independently changed canonical state,
so the replacement `PLAN.md` divides it into independently approvable items
(G0, G1, G1a, G2-G14, the blocking G14a-G14e archive-scalability sequence,
G15-G17, the blocking G17a-G17b evidence-preservation sequence, G18, the
G18a-G18g documentation-integrity/site sequence, and G19-G20)
rather than the former six
implementation groups.
The user's 2026-08-11 review resolved the G0-G17 policy decisions; no item is
approved for implementation merely by resolving its decisions.
Every completed item leaves a verified ZIP in the evidence directory and waits
for approval before the next begins. G17a-G17b established the missing
signature/timestamp/manifest/ISO custody layer; its verifier must pass before
any future GitHub push. See `VERSIONING_AND_SYNC_POLICY.md` and
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
automatic peer enrollment landed.

**G6–G11 completed 2026-08-12 and 2026-08-13**, each archived under
`plans/v0.7/` with its own evidence directory: schema v21 deterministic
metadata, membership, lifecycle, and notebook-tree convergence (G6); schema v22
note revision objects, bounded transfer deltas, and durable typed conflicts
(G7); schema v23 attachments that converge before their bytes (G8); the
encrypted, signed NAR1 artifact and canonical wire codec, with no schema change
and no dependency (G9); schema v24 snapshot catch-up and the reset state machine
(G10); and the ephemeral shared-directory carrier, its discovery, and
`notriosctl sync`, with no schema change (G11). Two replicas of one database now
converge through a folder either of them can delete.

**G12 completed 2026-08-13** and changed no production code. The shipped carrier
was run against a Google Drive folder mounted with `rclone mount` and against a
drive passed between peers; all eight conformance phases passed and no provider
limitation forced a protocol change. It measured what such a carrier costs:
another device's change took **45-57 seconds** to become visible, and resolving
a known name is no fresher than listing the directory. Publication order is
therefore a latency optimization on a cloud folder rather than a correctness
mechanism, which G15's scheduling must respect.

**G13 completed 2026-08-13.** Schema v25 adds the first authenticated surface
this project has had: a peer principal proved by an Ed25519 signature over the
request, authorizing `/api/v1/sync/...` for one database and nothing else, with
pairing reduced to a short-lived single-use code under which the group key
travels sealed. The transport policy refuses at startup rather than warning, and
ordinary note routes keep their local posture.

**G14 completed 2026-08-14** with no schema change. The REST data plane carries
G11's artifacts over that authenticated surface by implementing the same
`Carrier` interface, so REST and a shared folder are one protocol with two
couriers — measured at +0.07% overhead at 500 notes, because admission dominates
either way. A snapshot download is resumable, encrypted in authenticated frames,
and verified by archive-v2 itself before any restore. G14's small backup tiers
also exposed about **25% stored-ZIP overhead from one entry per loose archive
object**, while the existing pack fix has never been exercised end to end at
the supplied full scale.

**The blocking G14a-G14e archive-scalability sequence and G15-G17 sync control,
recovery UI, and safe retention are complete. G17a-G17b have completed the
evidence-preservation contract, historical backfill, immutable ISO reserve, and
finite outer-catalog closure. The G17b verifier now gates every future GitHub
push. G18 completed the shared-core/ABI/platform/Mermaid portability handoff;
G18a-G18g and the G18c.1 agent-workflow amendment are complete; G19 is next
but unapproved.**
The 2026-08-24 documentation-integrity amendment adds G18a-G18g after G18 and
before the compatibility/release slices; G18a-G18g are complete and G19 onward
remains approval-gated. G14a supplies the resumable, aggregate-only benchmark contract and
generated 10k/100k calibration. That evidence confirms one-file-per-object
growth, isolates stored-ZIP overhead from framing, and records a 100k
incremental-replay memory failure without changing production code. G14b's
382,206-document and attachment-bearing full-corpus matrix selected option B:
a required, same-schema SQLite image plus bounded packed external assets for
whole-library backup/catch-up, with packed semantic archive-v2 retained for
subset, merge, interchange, and fallback. The image path was 2.33x faster
locally and 1.99x faster including the distinct Google Drive copy; loose
layouts and raw Restic/Borg paths failed shape or memory gates. G14c implements
that selected representation; G14d integrates crash-safe restore, emergency
backup, physical encrypted catch-up, and post-vector replay. G14e passed 19
privacy-sanitized production phases and froze the compatible whole-library
default for G19 while retaining semantic archive-v2 as portable. The final
catch-up took 2,948.669 seconds at 210,010,112 bytes peak RSS with exact
post-vector equality. exFAT is no longer available
on the test drives, so the gate requires a bounded, non-object-per-file layout
but makes no unmeasured exFAT performance claim.

**G15 completed 2026-08-24** with schema v26. Explicit operations now enter a
closed per-target durable outbox with atomic leases, phase checkpoints,
heartbeats, cancellation, byte budgets, and bounded jittered retry. Local REST
and CLI expose safe control; MCP sync visibility/control is separately disabled
by default and can never reach target locations, credentials, keys,
backup/restore, enrollment/retirement, purge, reset, or another actor's
catch-up. The worker drains work but never schedules periodic sync.

**G16 completed 2026-08-24** without a schema change. The responsive local
Sync Center exposes active-profile identity, native directory selection,
explicit REST pairing and complete-snapshot permission, durable job states,
lazy attachment fetch/pin, three-way conflict resolution, repair reports,
verified catch-up staging, and NPB1 password backups. Passwords are cleared and
never persisted; restore/reset remain review-only. A real two-daemon test and
desktop/mobile Playwright sweep passed. No mobile build is claimed.

**G17 completed 2026-08-24** with schema v27. Signed peer-retirement decisions
travel through the ordinary log and prevent stale credential re-enrollment.
The 90-day configurable floor now takes the minimum of age, a retained physical
snapshot re-verified at the destructive boundary, and every active peer's
acknowledgement. Compaction retains checkpoint/death/revision-order identity;
below-floor peers receive a typed snapshot catch-up requirement. Resource GC
uses the same conservative gate. The Sync Center reviews horizons and
retirement consequences without receiving filesystem paths or exposing an
HTTP apply route. Generated one-full-corpus-churn evidence measured about 162
MiB for 382,206 representative operations, so the resolved default remains.

**G17a-G17b are a complete pre-push evidence-preservation sequence.** G17a
freezes the curated external evidence inventory, corrects the proposed
claims about hashes, OpenPGP signatures, RFC 3161 time evidence, Git history,
custody, and legal scope, and selects a canonical chained manifest plus
deterministic CD-sized ISO contract. It found 78 top-level files but also three recursive private
benchmark workspaces, so recursive all-file inclusion is rejected by default.
It recommends per-artifact signatures plus one timestamped signed batch
checkpoint. The user selected curated top-level scope, exact Ed25519 evidence
subkey `4ABE…4005`, DigiCert primary, and Sectigo fallback on 2026-08-25;
operational key/network/reserve permission and offline backup/revocation
attestation were supplied on 2026-08-25; the generated DigiCert pilot passed.
G17b backfilled every approved original ZIP and screenshot without rewriting it,
checked the canonical manifest and ISO catalog into Git, placed immutable
numbered ISO images and their verification sets only under
`/media/renes/SEAGATE2TB/notrios-evidence/`, and adds a host-side pre-push gate.
Backfilled seals say they are retroactive; a current timestamp never becomes a
historical completion time. ISO creation does not authorize GitHub push or
physical CD-R burning, which remain separate operations.

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
- **Secure container and catch-up (G9-G10, G14a-G14e):** archive-v2 change-envelope
  capabilities; mandatory authenticated encryption and per-replica Ed25519
  signatures; signed backup requests; encrypted physical snapshot catch-up
  through deterministic sequential USTAR and NBK1, plus reset
  followed by incremental replay from the snapshot vector; then a blocking,
  full-corpus physical-format benchmark, implementation, catch-up integration,
  and acceptance freeze before scheduling or UI work.
- **Two carrier adapters, one protocol (G11-G15):** an ephemeral shared
  directory that can be deleted and reconstructed, with immutable per-replica
  advertisements/requests/artifacts; Google Drive/removable-media conformance
  without an rclone dependency; REST authentication/pairing/TLS/quota/audit
  foundation; resumable REST objects and backup downloads; durable jobs,
  retry/backpressure, and bounded MCP status/control.
- **Evidence preservation before external publication (G17a-G17b):** frozen
  curated inventory plus explicit private-workspace exclusion; versioned
  chained manifest; per-artifact detached signatures bound by one signed
  checkpoint and independently verified RFC 3161 token over that checkpoint;
  explicit retroactive/contemporaneous time semantics; deterministic,
  immutable numbered ISO 9660 reserves on the designated external disk; clean
  offline restore and a strict host-side pre-push coverage gate. Hashes,
  signatures, timestamps, Git, and ISO media are described as corroborating
  controls, not automatic legal proof.
- **User recovery, safe retention, and portability handoff (G16-G18):** pairing/profile/directory,
  encrypted-backup, lazy-resource, conflict and reset UI; explicit peer
  retirement and acknowledgement/snapshot-gated tombstone/blob GC; a platform,
  shared-core/C-ABI, and Mermaid contract handed to the distinct v0.8 milestone
  and post-1.0 Flutter client.
- **Documentation integrity and site migration (G18a-G18g):** investigate and
  then implement source-adjacent user/API doc anchors; honest executed,
  generated, claimed, and unverified grades; result-bearing CLI/config/REST/MCP
  examples; browser-executed GUI journeys with action-length evidence;
  deterministic generated-subset freshness; calibrated advisory contradiction/
  actionability review; and a pinned, offline Hugo/Ledger+Pagefind GitHub Pages
  build that preserves the same Markdown Help source and public links.
- **Compatibility and completion (G19-G20):** publish the archive-v2 contract
  deferred from v0.4 P6 through the new documentation checks after the sync-era
  container stabilizes, then run full
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

**Completion (2026-08-31).** G0-G20 are implemented and archived under
`plans/v0.7/`. The milestone ships protocol-1.0 encrypted/signed record sync,
directory and REST carriers, physical catch-up and semantic backup, durable
jobs, Sync Center, retention/retirement/repair, evidence custody, portability
and documentation contracts, the archive-v2 consumer bridge, and full-system
release acceptance at product 0.7.0/schema v27. G20 additionally centralizes
remote route/TLS admission, bounds browser/JSON/resource bodies, and roots and
bounds untrusted carrier/legacy-archive access. Publishing/tagging remains an
owner action; v0.8 is planned but unstarted.

## v0.8 — Installation, configuration, shared core, and portability

This is deliberately separate from synchronization correctness. Packaging
changes which directories, credentials, ports, background work, deep links, and
file pickers an installed application may use, and those permissions must not
be smuggled into v0.7 as desktop assumptions.

- **Shared core and bounded GUI enablement.** Complete H0's
  facade/SQLite/C-ABI investigation, H1's transport-neutral library, and the
  separately gated H2a/H2 offline Mermaid work. The Android acceptance target
  remains an emulator/backend result, not a mobile product.
- **Installed-path and migration contract.** Centralize immutable assets and
  native config/data/state/cache/runtime paths. Linux follows the XDG base
  directory contract, including absolute override validation; Windows/macOS use
  native equivalents. Preserve explicit paths, require explicit portable mode,
  migrate checkout-relative data only through reviewed backup/rollback, and
  account for profiles whose data lives outside standard roots.
- **End-user-location Make lifecycle.** Add `make install`, `make uninstall`,
  and `make purge` without changing development-only `clean`/`clobber`.
  Support GNU install variables and `DESTDIR`, isolated XDG overrides, exact
  ownership manifests, `DRYRUN=1`, headless `FORCE=1`, and deliberately
  destructive `NO_BACKUP=1`. Uninstall leaves every user-owned file; purge
  backs up and verifies config/data/state before bounded deletion unless the
  user explicitly chooses the deep-warning no-backup path.
- **Ubuntu-priority native installer.** Investigate and pin a minimal
  Apache-2.0/MIT-compatible packaging toolchain, then create and natively test
  an internal Ubuntu package containing the GUI, daemon, CLI, immutable web
  assets, desktop metadata, notices, and exact runtime dependencies. A user
  must be able to install and run it without source, Go, Node, Wails, compiler,
  or development headers.
- **Windows and macOS installers are deferred to post-v1.0.** The H6a
  investigation established that no toolchain can be selected for either
  platform from a Linux development machine: compilation is not acceptance, and
  each claimed candidate must install, launch, use native paths, upgrade,
  remove, reinstall, preserve data and clean up **on its own OS**. That needs
  Windows and Apple hardware, or hosted runners standing in for them, and
  neither is available. Postponing explicitly is what the original entry asked
  for when the gate could not pass; v0.8 ships Ubuntu only and claims nothing
  else.
- **Installed integration and credentials.** Exercise multiple profiles,
  loopback ports, URL handlers, shared directories, firewall behavior, native
  pickers, upgrade/removal, and lifecycle targets. Select native credential
  providers per supported desktop OS; installed mode never silently falls back
  to plaintext and purge backups never contain native-store secret bytes.
- **Framework/mobile boundaries.** Run Wails v3 only as a separately approved
  spike after the Wails v2 installer baseline. Before 1.0, mobile work stops at
  one Android-emulator shared-core acceptance; physical Android and all iOS
  client validation remain post-1.0.
- **Delayed GitHub integration.** Keep all local implementation and validation
  on `develop`; delay the first push until native runners are actually needed.
  Re-audit `main`, merge it into `develop` only if required, run the evidence
  pre-push gate, push `develop`, and open a `develop`-to-`main` PR. Merge only
  after final review/authorization, then bring the merged result back into
  `develop` and verify no content divergence.
- **Internal artifacts only.** v0.8 may retain the verified unsigned Ubuntu
  prerelease installer as explicitly internal evidence; it is the only platform
  v0.8 packages at all. It does not create a tag,
  GitHub Release, public installer, signing/notarization claim, app-store
  upload, or unsupported-platform claim.

## v0.9 — Release-candidate hardening

- Promote the v0.8 Ubuntu installer through clean native environment matrices.
  Windows and macOS are deferred to post-v1.0 with the hardware they need, so
  there is no feasible candidate to promote here; v1.0 already provides for
  marking them postponed rather than shipping unexecuted build output. Rehearse fresh install, source-
  layout migration, upgrade across prereleases, downgrade refusal/rollback,
  remove/reinstall, profile discovery, and no-source/no-development-toolchain
  runtime operation.
- Harden `install`/`uninstall`/`purge` and package-manager interoperability:
  backup capacity and corruption faults, restore drills, process/mount/symlink
  races, external profile roots, modified installed artifacts, unattended
  execution, and proof that uninstall never deletes user data.
- Resolve production signing/notarization and timestamping policy per supported
  platform. Keep certificates, tokens, and passphrases out of pull-request
  jobs, logs, artifacts, backups, and the repository; document what remains
  blocked when an owner account or native trust service is unavailable.
- Generate and verify checksums, SBOM, dependency/license/security reports,
  provenance/attestations, reproducible metadata, installer inventories, and
  pinned least-privilege GitHub workflows. Exercise a non-public release
  candidate/draft flow without making v0.9 an end-user GitHub release.
- Run long-lived installed directory/REST/emulator soak tests, native
  integration and cleanup matrices, support-bundle redaction, crash-reporting
  policy, and disaster-recovery drills. Freeze the exact desktop support matrix;
  a postponed platform remains absent from release claims.
- Freeze REST, MCP, archive, sync, configuration, installer, Make lifecycle,
  and shared C ABI compatibility candidates for 1.0. Run ABI ownership/leak/
  double-free, wrong-handle, concurrent-shutdown, cancellation, and stream-
  limit tests.
- Write release-grade installation, upgrade, rollback, backup/restore,
  uninstall, purge, troubleshooting, and artifact-verification documentation
  for a user who has neither the repository nor a development environment.

## v1.0 — Feature-complete local product

- Stable REST API.
- Stable MCP tool/resource schemas.
- Large-scale performance tests with hundreds of thousands of documents/resources.
- Publish a user-authorized GitHub Release from reviewed `main` so an end user
  can download, verify, install, launch, upgrade, uninstall/reinstall, and
  restore Notrios without source code, Go, Node/npm, Wails, a compiler, or
  development headers.
- Ship a signed, checksummed, SBOM/provenance-bearing Ubuntu installer as the
  minimum supported desktop artifact. Ship Windows and/or macOS installers only
  if their v0.8-v0.9 native build/install/runtime, signing/notarization,
  upgrade/removal, data-preservation, and cleanup gates pass; otherwise mark
  them postponed and do not present their build outputs as supported downloads.
- Keep end-user configuration/data/state in documented native locations;
  preserve it on ordinary uninstall; provide exact dry-run and verified-backup
  purge behavior; and publish migration, rollback, disaster-recovery, and
  artifact-authenticity instructions.
- Versioned no-GUI Notrios library/header artifacts for the pre-1.0 supported
  platform matrix, with lifecycle, ownership, threading, error, stream, and
  compatibility examples. An Android-emulator result is not labelled physical
  Android support; unsupported iOS artifacts are not implied.
- Backup/export/restore/sync compatibility and disaster-recovery validation.
- Security review for remote media and MCP.
- Usable installation and support documentation on the documentation site and
  in the GitHub Release, with exact supported OS/architecture/runtime rows.
- Create the user-authorized release for `github.com/renesugar/notrios` only
  after `develop`/`main` synchronization, required CI, installer readback, and
  pre-push evidence gates pass. Verify the version tag, release notes,
  installers, checksums, signatures, SBOM/provenance, upgrade/rollback
  instructions, and downloaded bytes after publication.
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

## Post-v1.0 — Headless and remote-server credential storage

Moved here from v0.8 (H9) because Notrios is initially an end-user application
driven by a UI, and a remote server deployment is a future goal. This is a
scheduling decision, not a change of intent.

- A server or SSH install has no Secret Service, no unlocked keychain and no
  pinentry, so it cannot use the desktop credential provider. Measured during
  H9: `gopass` generates a passphrase-protected `age` identity and then fails
  with `pinentry: gnome3.isatty` when there is no tty, and the working headless
  path costs a plaintext private key at `0600` readable by the service account.
- Four decisions were recorded and deferred with this work: whether headless is
  a provider tier or a provider parameter, whether machine-scope or root-scope
  storage is admissible, which Windows logon types count as supported, and
  whether the protection guarantee is stated per platform. Headless Windows
  under a service logon may be *stronger* than headless Linux, so one blanket
  statement will not do.
- `99designs/keyring` was evaluated for this and is the likely starting point:
  it carries `keyctl`, `pass` and `file` backends and can address a named macOS
  keychain, which `zalando/go-keyring` cannot. Porting it to `godbus/dbus/v5`
  was measured as a pure import rewrite with no code changes, and the tree is
  1,708 non-comment lines. Its `Open` walks a backend order and continues past
  failures, which must be replaced with fail-closed selection before adoption.
- What v0.8 ships instead is the refusal, not silence: `doctor` reports whether
  a native store is reachable and enrolment refuses with that reason. The
  condition is reachable on a desktop install through `loginctl enable-linger`
  or an SSH login, so the refusal is not hypothetical.

## Post-v1.0 — Windows and macOS installers

Moved here from v0.8 (H7) because it cannot be implemented until hardware is
available. This is a scheduling decision rather than a change of intent: the
requirements below are the ones H7 already carried.

- Native GitHub-hosted runners, or physical Windows and Apple hardware, are a
  prerequisite rather than an optimisation. H6a measured why: `CGO_ENABLED=0`
  does not build this project, so every target needs its own C toolchain, and a
  cross-built artifact that has never run is not evidence that the platform
  works.
- Windows: a Wails v2 or NSIS installer that installs, launches, resolves the
  native per-OS roots from H3, upgrades, removes, reinstalls and preserves the
  user's library. `makensis` exists on Linux and can produce an installer that
  nothing here can execute; that artifact is not acceptance.
- macOS: an `.app` bundle in a native distribution container, with the same
  install/launch/upgrade/remove/preserve gate. Nothing about macOS can be
  established without Apple hardware, and emulating it locally is out of scope.
- Unsigned artifacts stay clearly labelled internal candidates until each
  platform passes natively. Signing and notarisation are separate decisions with
  their own secret-handling boundaries.
- The four-level claim ladder from H6a applies unchanged: generated,
  structurally inspected, natively installed and executed, supported. Neither
  platform may be described above the level its evidence reaches.

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

`PLAN.md` now holds the active v0.8 plan derived from this roadmap. H0 is
complete; H1 is the next separately approval-gated item.
