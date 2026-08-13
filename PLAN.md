# Plan: v0.7 — Native synchronization

Status: **G0-G10 completed through 2026-08-13. Product version remains 0.6.0 and the
canonical schema is v24. G11 is the next item and is not approved.** The
former seven-item draft was too coarse: it mixed protocol research, canonical
write interception, merge semantics, two transports, cryptography, recovery,
UI, retention, and release validation into slices that could not be reviewed or
recovered independently.

This plan follows `SYNCHRONIZATION.md`, `VERSIONING_AND_SYNC_POLICY.md`,
`NATIVE_ARCHIVE_V2.md`, `SECURITY_REVIEW.md`, and the item-writing rules in
`AGENTS.md`. Each item below is read and approved on its own. An unresolved
choice therefore lives inside the item it affects; the register at the end is
only an index.

## Outcome and boundaries

v0.7 delivers native synchronization between independently writable Notrios
replicas through either an ephemeral shared directory or an authenticated REST
peer. It includes fast snapshot catch-up, incremental state-vector/change-log
exchange, revision-aware note merging, lazy resource materialization, encrypted
and authenticated protocol objects, conflict/recovery UI, and bounded
operations suitable for a future mobile client.

The shared directory is a disposable carrier, not a database and not an
authority. Deleting it loses no canonical note data. A peer reconstructs what
to publish by comparing signed advertisements, requests, state vectors, and its
own durable journal. `rclone` is useful only to exercise this directory over the
available Google Drive mapping; neither the protocol nor a future mobile client
depends on an `rclone` executable. `rclone sync` and `bisync` never decide
Notrios deletion or conflict outcomes.

The REST and directory adapters carry the same protocol objects. A direct REST
connection is an optimization and availability choice, not a second merge
engine.

Not in v0.7:

- character-by-character live co-editing while two people type;
- multi-user service roles or public multi-tenant hosting;
- direct Google Drive/Dropbox/etc. SDK adapters;
- an Android/iOS release or migration of the stable Wails v2 desktop shell;
- SQLCipher conversion of the live database;
- Nostr, BLE, or other opportunistic courier transports;
- using Subversion libraries or `svnadmin dump` as the wire format.

Installation, filesystem permissions, OS credential stores, a framework-neutral
Go application facade and C ABI, current-GUI Mermaid support, Wails v3
migration, and Android-emulator packaging form the distinct v0.8 milestone in
`ROADMAP.md`. v0.7 must nevertheless keep transport, crypto, storage, and job
interfaces UI-framework independent and must record the constraints v0.8 has
to validate. A post-1.0 Flutter client is an independent native client of that
shared core; it is not a Wails replacement hidden inside the sync milestone.

## Protocol shape to validate, not silently assume

The proposed v1 protocol has these layers:

1. **Canonical state** remains SQLite plus content-addressed assets.
2. **Local journal** records immutable changes transactionally with the
   canonical write, addressed by `(replica_id, sequence)`.
3. **State vector** says the highest contiguous sequence admitted for every
   enrolled replica. Gaps are explicit; a maximum alone never proves
   completeness.
4. **Change envelope** carries a bounded sequence range and dependencies.
5. **Objects** carry complete revision bodies, optional transfer deltas,
   resources/chunks, and snapshot archives. Complete content remains available
   when a delta base is absent.
6. **Authenticated encryption and signatures** protect protocol objects above
   the carrier; TLS additionally protects non-loopback REST connections.
7. **Transport adapters** publish and fetch immutable objects, requests,
   advertisements, acknowledgements, and manifests.
8. **Catch-up** requests a verified archive-v2 snapshot plus its state-vector
   boundary, restores it with an explicit intent, and then applies later
   envelopes.

Subversion is a useful model for monotonic revisions, change logs, and
base-aware deltas. Its dump format is not the proposed Notrios format: it does
not express Notrios' content-addressed resources, archive-v2 capability chain,
per-replica state vector, peer enrollment, encryption, or lazy objects. The
existing archive-v2 container has already been verified at 382,206 notes and is
the safer basis.

## Execution rule for every item

Implement one item at a time. Before it starts, resolve its blocking decisions
and obtain explicit user approval. During it, append `started` and terminal
records to `agent/ATTEMPT_LOG.jsonl`, keep the repository working, and add tests
with the change. At completion:

1. run the item-specific checks plus the repository validation appropriate to
   the change;
2. archive the completed item under `plans/v0.7/NNN-<slug>.md` with evidence
   and model information;
3. update the roadmap, handoff, status, and protocol docs where the evidence
   changed them;
4. commit the coherent slice;
5. create the release ZIP with `scripts/package_release.sh`, verify it with
   `scripts/check_release_zip.py`, and copy the verified ZIP to
   `/home/renes/evidence/notrios`;
6. report the ZIP and ask whether to proceed to the next item.

## G0. Investigation — threat model, terminology, and reference validation — complete

**Goal.** Freeze what is being protected and which ideas from the supplied
references actually fit Notrios before a schema or format is written.

**Scope.** Model a lost/hostile shared directory, replayed and reordered files,
a curious cloud provider, a compromised or retired replica, a malicious REST
client, password loss, a copied database with a duplicated replica ID, and
denial through oversized/decompression-heavy input. Specify trust roots,
enrollment, revocation, recovery, metadata leakage, and audit events. Record the
current upstream status of any proposed dependency and its license. Define
`profile`, `database`, `replica`, `peer`, `carrier`, `snapshot`, `envelope`,
`object`, `state vector`, `advertisement`, and `request` once.

**Boundaries.** No production crypto, transport, or migration code. Do not copy
reference implementations or treat hashes as authentication.

**Dependencies.** None.

**Working state.** A reviewed threat model and protocol glossary under
`performance/v0.7-g0/`, plus resolved choices that later format slices can cite.

**Validation and evidence.** Adversary/asset/trust-boundary table; dependency
license and platform matrix; misuse cases traced to planned controls; security
review updated without claiming those controls are live.

**Resolved decisions (2026-08-11)**

- **All v1 sync payloads use authenticated encryption, including over REST.**
  The only exception is an explicit local-development test mode. One rule
  avoids a library being private over a directory but exposed after switching
  targets. TLS remains required for non-loopback REST
  because payload encryption does not hide endpoints, sizes, timing, or API
  credentials.
- **Per-replica Ed25519 signatures are required.** Each enrolled replica signs
  canonical envelope, advertisement, request, acknowledgement, and
  snapshot-manifest bytes with an Ed25519 identity key; encryption remains
  separate. Signatures give device attribution, replay/revocation evidence, and
  safe untrusted-directory
  discovery. Content hashes still verify objects; do not sign every blob twice.
- **SVN dump compatibility is not a goal.** Reuse the state-vector/change-log
  and base-delta ideas, but extend archive v2 rather than adopting a foreign
  repository dump grammar.

**Completion (2026-08-11).** Archived as
`plans/v0.7/000-threat-model-terminology-reference-validation.md`. The reviewed
evidence under `performance/v0.7-g0/` defines the protocol glossary, adversary/
asset/trust-boundary model, thirty misuse cases with owning controls, metadata
leakage and audit budgets, and a primary-source dependency/license/platform
matrix. It additionally freezes two consequences later slices must preserve:
compromise revocation advances the encryption epoch for remaining peers, and
visible content addresses name ciphertext/artifact bytes rather than exposing
raw plaintext hashes. No crypto, transport, schema, or sync runtime was added.

## G1. Investigation — representative divergence and revision-delta workload — complete

**Goal.** Choose merge and delta techniques from measured Notrios bodies and
explicit concurrent-edit scenarios instead of pretending a static import can
reveal how often real users conflict.

**Scope.** Measure body/resource size and revision-chain distributions from
available safe corpora. Build a deterministic workload matrix covering
independent paragraph edits, same-line edits, title/body overlap, delete/edit,
move/rename, tag add/remove, notebook cycles, repeated offline intervals, and
large generated notes. Compare complete-body transfer with candidate line,
word, and byte delta/three-way merge approaches. Preserve aggregate-only rules
for private corpora.

**Boundaries.** Do not claim a frequency of human concurrency from imported
data; the corpora do not contain multi-device editing history. No production
merge engine or new dependency.

**Dependencies.** G0 terminology.

**Working state.** Reproducible fixtures and a findings report under
`performance/v0.7-g1/` selecting the body operation and conflict model for G7.

**Validation and evidence.** Delta ratio and CPU/RSS distributions; clean-merge
and conflict classifications; Unicode, Markdown, very-long-line, binary-looking
text, missing-base, and malicious-patch cases; exact reconstruction hashes.

**Resolved decisions (2026-08-11)**

- **The canonical representation is a full result hash/body object with an
  optional delta from a named parent.** A delta saves transfer when its base is
  present; the complete object breaks dependency chains and makes repair
  possible.
- **The investigation selects merge granularity and implementation by
  evidence.** It must choose the smallest maintained Apache-2.0/MIT/BSD
  implementation that preserves UTF-8 and exposes conflicts. Do not adopt a
  CRDT merely because it merges character operations; ordinary offline sync is
  revision merge, not live co-editing.
- **The representative offline intervals are one hour, one day, one week, and
  thirty days**, with results reported separately.

**Completion (2026-08-11).** Archived as
`plans/v0.7/001-representative-divergence-revision-delta-workload.md`. Evidence
under `performance/v0.7-g1/` profiles 2,063,061 aggregate-only bodies, runs 112
delta measurements and eighteen merge classifications, checks a 1 MiB
generated body, and rejects seven malformed/untrusted patch cases. G7 is to use
complete UTF-8 result objects plus optional beneficial named-parent line
deltas, with line-first three-way merge and bounded Unicode-aware word-token
refinement of conflict regions. Same-token/delete-edit overlap becomes a typed
durable conflict. G1's measured merge behavior and transfer-ratio conclusions
remain valid. Its recommendation to consider `github.com/epiclabs-io/diff3`
as the G7 implementation was superseded by the 2026-08-11 G1a plan amendment:
G7 is to own its bounded pure-Go line/word merge implementation, and G1a first
investigates a separate pure-Go binary-safe delta codec. G1 added no dependency
or runtime code.

## G1a. Investigation — pure-Go xdelta/VCDIFF feasibility and dependency removal — complete

**Goal.** Determine whether a Notrios-owned pure-Go implementation can produce
and apply deterministic, bounded transfer deltas for arbitrary text or binary
bytes, while keeping the separate conflict-aware three-way merge implementation
free of an external runtime dependency.

**Scope.** Build an investigation-only prototype under
`performance/v0.7-g1a/`. Specify the behavior independently, then compare three
distinct things rather than conflating them: Subversion's rolling-checksum
xdelta matcher and `COPY`/new-data operation stream, Subversion's separate
`svndiff` serialization, and RFC 3284 VCDIFF. Evaluate source and target
windows, rolling checksums, `ADD`/`COPY`/`RUN`, address modes, deterministic
output, streaming `io.Reader`/`io.Writer` APIs, and source-versus-target copy
semantics. Exercise the G1 text fixtures plus generated repetitive, random,
sparse-edit, and attachment-like binary fixtures. Compare complete bytes, the
G1 line delta, the pure-Go prototype, and external xdelta3/open-vcdiff/
Subversion tools only as test oracles where their formats overlap. Record exact
reconstruction, compression ratio, CPU, RSS, allocations, and format/license/
provenance evidence.

The decoder/parser investigation must define limits for source, window, input,
output, instruction, address, and varint counts; integer overflow; expansion
ratio; delta-chain depth; memory; CPU/cancellation; corrupt/truncated streams;
overlap; unsupported custom code tables; and secondary compressors. It must
also determine whether optional binary resource deltas are worthwhile only
when an immutable resource has a known named parent; ordinary content-addressed
resources without that relationship continue to use complete or chunked
transfer.

**Boundaries.** This is evidence and prototype code, not a production codec or
protocol-format commitment. Do not add a project `go.mod` dependency, cgo,
vendored C/C++, or code derived from GPL implementations. Do not treat xdelta
as a three-way merge algorithm, and do not infer a parent relationship between
unrelated content-addressed resources. Desktop results do not prove mobile
safety.

**Dependencies.** G0 security/provenance rules and G1 workload/merge behavior.

**Working state.** A reproducible prototype, golden/corrupt vectors, benchmark
results, interoperability matrix, license/provenance report, and recommendation
under `performance/v0.7-g1a/`. Production promotion requires a later approved
G7 or G8 slice.

**Validation and evidence.** Exact round trips for empty, identical, inserted,
deleted, repeated, random, sparse-edited, UTF-8, and binary inputs; deterministic
bytes; property/fuzz tests; every declared decoder limit and malformed-input
refusal; RFC 3284 vectors and external-oracle comparison where compatible;
CPU/RSS/allocation/ratio distributions; and an explicit statement of whether
the prototype emits VCDIFF, Subversion svndiff, or only a Notrios operation
format.

**Resolved direction (2026-08-11)**

- **Production candidates must be pure Go and Notrios-owned.** External C/C++,
  cgo wrappers, and Go merge/delta packages may be research or test oracles but
  are not runtime dependencies.
- **Binary delta and three-way merge stay separate.** G1's bounded line-first,
  word-region conflict behavior remains G7's contract; an xdelta/VCDIFF codec
  only optimizes transport from a named base.

**Resolved decisions (2026-08-11)**

- **Which interoperability target should Notrios adopt? — Resolved.** Recommend
  a constrained RFC 3284 default-code-table profile over the private `NXD1`
  comparison container. The measured private saving was normally ten bytes,
  while both pinned external oracles decoded every Go-emitted VCDIFF fixture
  exactly. This is Subversion-style matcher compatibility, not svndiff byte
  compatibility, and it is still a recommendation rather than a live wire
  format.
- **How should implementation provenance be handled? — Resolved.** Keep the
  spec-first independent Go implementation model. Apache Subversion is an
  attributed behavioral reference; no direct translation is claimed and no
  GPL source was used. Pinned Apache-2.0 C/C++ implementations remain external
  test oracles only.
- **May the prototype move into production? — Resolved.** No. It remains under
  `performance/v0.7-g1a/`; a separately approved G7/G8 slice must review and
  either rewrite or deliberately promote it using G2's completed numeric bounds.

**Completion (2026-08-11).** Archived as
`plans/v0.7/003-pure-go-xdelta-vcdiff-feasibility.md`. The pure-Go prototype
under `performance/v0.7-g1a/` produced exact deterministic round trips across
14 text and seven generated binary fixtures. VCDIFF was beneficial in 19 of 21
cases and smaller than G1's JSON line delta in all 14 comparisons; empty and
unrelated random bytes prove the complete-object size gate is still required.
Both xdelta3 and open-vcdiff decoded all three representative Go-emitted
fixtures exactly. The bounded decoder intentionally accepts only a strict
profile, rejects unsupported RFC features and hostile/over-limit inputs, and
is not a general VCDIFF decoder. G1a added no production codec, dependency,
schema, or mobile-safety claim.

## G2. Investigation — envelope, resource, and constrained-device bounds — complete

**Goal.** Determine format, compression, pack, chunk, and pending-queue bounds
before they become an immutable protocol contract.

**Scope.** Reuse archive-v2 loose/packed evidence and run generated change sets
against the large recipe corpus and attachment-bearing Joplin corpus. Compare
deterministic JSON/JSONL and one compact candidate, deterministic compression,
fixed-size chunk thresholds, whole-resource fallback, range fetch, many-small
objects, and decompression limits. Use desktop CPU/RSS/disk profiles as a proxy
only; produce an explicit Android-emulator checklist for v0.8 and a separate
post-1.0 physical-device checklist.

**Boundaries.** No FastCDC unless fixed chunks fail a measured case. No claim
that desktop measurements prove mobile safety. No private content in evidence.

**Dependencies.** G0; G1 supplies body-delta samples; G1a supplies measured
binary-safe delta candidates and decoder-bound requirements.

**Working state.** A findings report under `performance/v0.7-g2/` that gives G8
and G9 numeric limits and explains every proxy limitation.

**Validation and evidence.** 100/10k/100k-operation tiers; 0-byte, small, large,
and attachment-heavy resources; compression-bomb and count-limit rejection;
time/RSS/disk/file-count/round-trip estimates; deterministic byte checks.

**Resolved decisions (2026-08-11; measured outcome recorded at completion)**

- **The spike selects the v1 encoding/compression using the approved default
  candidate:** canonical JSON/JSONL plus a pinned deterministic compression
  profile because it is inspectable and fits
  archive v2; a compact binary encoding must show material benefit and have a
  canonicalization specification.
- **The spike selects the numeric chunk threshold using the approved default:**
  whole-object transfer below a measured threshold and fixed-size
  chunks above it. Content-defined chunking remains a later optimization unless
  repeated edits to large resources justify its CPU and complexity.
- **Android bounds do not become a v0.7 support claim.** v0.7 records
  provisional limits; pre-1.0 validation may use an Android emulator only.
  Physical-device confirmation belongs to the post-1.0 Flutter client release
  gate. The v0.8 shared-core/FFI work prevents Wails-mobile maturity from being
  the only route to a mobile client.

**Completion evidence (2026-08-11).** The compact NCB1 record candidate met
the recorded exception gate: at 10,000 generated operations it was 51.9%
smaller raw and 16.3% smaller after deterministic gzip, decoded materially
faster, and allocated about one-fifth as much as canonical JSONL on the desktop
proxy. G9 therefore receives NCB1 plus a canonical-JSON outer manifest as the
implementation candidate, with gzip level 6/fixed headers and cross-toolchain
goldens still required. Envelopes close at 10,000 operations, 16 MiB canonical
bytes, or 4 MiB compressed bytes; pending admission is bounded per peer at
10,000 operations/64 MiB disk-backed bytes. Resources stay whole below 1 MiB
and use 1 MiB fixed chunks above it, with at most 16,384 chunks under the
existing 16 GiB resource ceiling. Sync packs provisionally target 64 MiB/4,096
objects/4 MiB trailers. FastCDC remains deferred. The v0.8 emulator and
post-1.0 physical-device checklists may only retain or lower these provisional
mobile bounds. No production sync codec, schema, dependency, or runtime landed.

## G3. Profiles, replica identity, and multi-instance process isolation

**Status: complete 2026-08-12.** Archived as
`plans/v0.7/005-profiles-replica-identity-multi-instance-isolation.md`.

**Goal.** Make one installation manage multiple databases explicitly and allow
two or more server processes on one machine without sharing paths, ports,
replica identities, or secrets accidentally.

**Scope.** Extend the existing logical-database registry into named runtime
profiles. A profile resolves database/assets/config paths, listen address,
public URL, sync target `none|directory|rest`, credential references, and its
one local replica identity. Add list/show/create/validate/start-oriented CLI
surfaces without embedding secrets in process arguments. Detect duplicate
database paths, duplicate replica IDs, port collisions, stale registrations,
and copied databases. Keep `notrios://` ambiguity explicit.

**Boundaries.** A profile is local configuration, not a synchronized record and
not an MCP tool scope. Do not add filesystem scanning or automatic access to an
arbitrary path. Do not build a process supervisor.

**Dependencies.** G0 identity terms and threat model.

**Working state.** Two independently configured `notriosd` processes can run on
different loopback ports against different databases, and the CLI/UI can name
which profile they address. Sync target `none` remains a first-class safe
default.

**Validation and evidence.** Fresh/adopted/forked/copied database fixtures;
parallel-process smoke test; path/port/replica collision refusals; configuration
redaction and `0600` file checks; stable-link routing across profiles.

**Resolved decisions (2026-08-11)**

- **Profile configuration uses a small registry plus one config file per
  profile.**
  It gives each server an explicit file, keeps process arguments free of
  secrets, and lets packaging relocate the config root in v0.8.
- **An unpaired copied database may not synchronize.**
  It must explicitly adopt the database universe while minting a new replica
  ID, or fork to a new database ID. A duplicated replica ID is always refused.
  Moving selected notebooks between unrelated database universes uses an
  explicit export followed by import, not synchronization.

**Completion evidence (2026-08-12).** Runtime profiles now use the version-2
local registry plus one generated `0600` config, bind a random local profile ID
and the database/replica identity, resolve isolated absolute runtime paths and
loopback ports, default sync to `none`, redact credential references, and
validate again during service startup. The CLI exposes
`create|show|list|validate|start`; start passes only the config path and remains
a foreground launcher. Raw copies are refused until explicit adopt/fork,
whereas valid replicas of one logical database remain explicit stable-link
ambiguity. A real integration fixture starts two profile-bound `notriosd`
processes simultaneously and verifies separate status/routing. No schema,
journal, sync transport, MCP profile surface, supervisor, or dependency landed.

## G4. Replication schema and transactionally complete local journal — complete

**Goal.** Record every sync-relevant canonical mutation exactly once, in the
same transaction as the mutation, without changing behavior when sync is off.

**Scope.** Add the next schema migration for enrolled replicas, local monotonic
sequence allocation, operations, operation dependencies, state-vector/gap
tracking, peer acknowledgements, pending admissions, and audit events. Define
the record/field vocabulary for documents, revisions, notebooks, tags,
memberships, links/provenance where canonical, resources, trash/restore/purge,
and batch/import/restore effects. Route all Store write paths through one
journal seam. Establish a snapshot boundary when a profile first enables sync;
do not retain an unbounded pre-enrollment log under target `none`.

**Boundaries.** No transport, merge, encryption, or UI. FTS5, Recoll,
projections, reports, task extraction, and search snippets never become sync
records. No raw SQL surface.

**Dependencies.** G3; G1/G2 findings shape operation fields but do not require
the final envelope codec.

**Working state.** After enrollment, every committed canonical change has one
durable `(replica_id, sequence)` operation and every rollback has none. Existing
non-sync behavior and import throughput remain within recorded bounds.

**Completed 2026-08-12.** Schema v19 adds explicit local enrollment/snapshot
boundaries, replicas, one monotonic local allocator, immutable operations and
dependencies, state vectors/gaps, peer acknowledgements, pending admissions,
and audit events. Canonical-table triggers feed one transient capture seam;
sequence allocation, operation insertion, and local-vector advancement occur
inside the caller's existing transaction. `target: none` remains inert before
enrollment. A non-none profile target establishes the boundary at startup but
starts no transport. Identity rotation retires the old allocator and requires
a new boundary. The 100k import A/B recorded 300,000 exact operations, 23.9%
elapsed overhead, and 92.8% database-byte overhead; no-journal import retained
zero operations. Evidence and the exact classification table are archived in
`plans/v0.7/007-replication-schema-local-journal.md` and
`performance/v0.7-g4/`.

**Validation and evidence.** Migration upgrade/fresh-baseline parity; table-
driven coverage over every Store mutation; rollback/crash injection; batch and
import atomicity; journal-disabled baseline; 100k import/write overhead profile.

**Resolved decisions (2026-08-11)**

- **The record-classification rule is approved:** user-editable scalar metadata
  is a field register; immutable revisions/operations/resources are indivisible;
  memberships are their own elements; derived rows are absent. The exact table
  belongs in the G4 implementation table. A deviation requires an item-local
  plan amendment before code, not an after-the-fact justification.
- **Journaling begins on explicit sync enrollment**, paired with a full snapshot
  boundary. Target `none` before enrollment does not accumulate transport
  history.

## G5. State vectors, missing-range planning, and idempotent admission

**Goal.** Exchange immutable operations repeatedly, out of order, and with
gaps, while proving what each replica has and still needs.

**Scope.** Implement vector comparison, missing contiguous range planning,
duplicate/replay detection, dependency queues, database/protocol/schema/capability
handshake, and transactional admission. A vector entry means the highest
contiguous admitted sequence; later received sequences remain explicit gaps.
Apply and acknowledgement advance only after canonical admission commits.

**Boundaries.** In-memory/local fixtures only; no record conflict semantics
beyond admission and no network/directory adapter.

**Dependencies.** G4.

**Working state.** Three local replicas can exchange shuffled, duplicated,
dropped-then-delivered no-op operations and converge on identical vectors and
operation sets without falsely closing a gap.

**Validation and evidence.** Property/model tests over at least three replicas;
sequence exhaustion and skew cases; database/protocol/capability mismatch;
bounded pending dependencies; restart and crash boundaries.

**Resolved decisions (2026-08-11)**

- **Compatibility rules are fixed:** database ID and protocol major must match;
  required capabilities must be understood; schema may differ only inside an
  explicit protocol compatibility range. Unknown required records are refused,
  never skipped.

**Implementation defaults recorded before G5 code (2026-08-12; non-blocking).**

- The internal admission contract starts at protocol `1.0`. Schema v20 is
  compatible with v19-v20 peers that explicitly advertise that same range;
  widening either range requires an explicit compatibility-table change.
- A handshake validates an already configured peer and never enrolls one.
  G5 exposes only an internal explicit local configuration seam for fixtures;
  cryptographic/user-approved enrollment remains owned by G9/G13.
- State-vector and missing-plan inputs are bounded at 1,024 replicas/ranges and
  10,000 requested sequences per plan. An inbound sequence may be at most
  10,000 positions ahead of its durable contiguous vector. These limits make
  sparse-range work bounded; snapshot catch-up can replace repeated windows.
- G2's approved pending ceiling applies unchanged: 10,000 operations and 64
  MiB of encoded bytes per source replica, with 64 dependencies, 512 KiB of
  payload, and 1 MiB per encoded operation. Overflow refuses the transaction;
  it never evicts an arbitrary dependency.

**Completed 2026-08-12.** Schema v20 persists the compatibility tuple for an
explicitly configured admission peer and adds a hard local sequence-exhaustion
guard. The transport-neutral `internal/syncstate` core validates protocol 1.0,
database/schema/capability compatibility, compares bounded vectors, and emits
deterministic bounded missing ranges. SQLite admission normalizes a strict
internal operation representation, refuses unknown record/kind pairs and
conflicting replays, queues gaps/dependencies on disk, and moves operations,
dependencies, vector positions, explicit gaps, and the returned acknowledgement
in one transaction. A gap never advances the vector; exact duplicates are
counted without another write. Three real local replicas converge after
shuffle, duplication, and drop-then-deliver schedules. Restart and injected
pre-commit rollback retain the original pending/vector boundary. G5 adds no
network/directory adapter, record merge/application semantics, cryptography,
REST/MCP/UI surface, or background synchronization. Evidence is archived in
`plans/v0.7/008-state-vectors-missing-range-idempotent-admission.md` and
`performance/v0.7-g5/`.

## G6. Deterministic metadata, membership, deletion, and tree convergence

**Goal.** Make non-body canonical records converge before note text and blobs
add their own dependency graphs.

**Scope.** Implement HLC ordering with replica/sequence tie-breakers, field
register application, tag/membership semantics, trash/restore/delete
certificates, notebook moves/renames, and deterministic cycle/orphan repair.
Every repair is a visible audit event.

**Boundaries.** No body merge or resource bytes. Permanent purge remains gated
and must not be inferred from absence.

**Dependencies.** G5 and G1 workload fixtures.

**Working state.** All replicas given the same metadata operations converge on
byte-equivalent canonical rows and the same repair report under every tested
delivery order.

**Validation and evidence.** Exhaustive small-state and randomized tests for
concurrent update/delete/restore, add/remove, notebook move/cycle, clock skew,
and replay; ordinary Store invariants after every admitted transaction.

**Resolved decisions (2026-08-11)**

- **Concurrent membership add/remove uses LWW per element using the protocol
  order**, because user intent has an order without keeping an unbounded
  observed-remove dot set; G1 must show whether that loses a
  material scenario.
- **Notebook-cycle repair retains the winning parent assignment by protocol
  order** and reparents each losing/cycle-forming node to the nearest valid
  ancestor or root, with an explicit repair record. The exact
  algorithm must be deterministic from the operation set, not arrival order.
- **Purge propagates as a signed death certificate** that outlives the document
  until every active peer acknowledges it; restore after purge creates a new
  document identity rather than resurrecting erased state.

**Implementation defaults recorded before G6 code (2026-08-12;
non-blocking).**

- Update operations carry only fields changed by that canonical statement;
  create operations carry the complete initial scalar record. This is the
  default required for per-field registers: a body-pointer or lifecycle write
  cannot accidentally refresh a title register it did not edit.
- Existing case-insensitive uniqueness constraints remain canonical. If
  concurrent notebook, tag, or search-notebook names collide, the protocol-
  order winner retains the requested name and each loser receives a stable
  short ID suffix. The repair is deterministic and audited instead of making
  admission depend on delivery order. Protected built-in names remain reserved;
  a colliding mutable record receives the suffix.
- Schema v21 stores non-empty death-certificate signer/signature bytes and
  permanently rejects same-ID restoration once such a certificate is
  admitted. G9 still owns key enrollment and cryptographic verification, so
  G6 exposes only an internal trusted-fixture seam for a structurally signed
  certificate and refuses local purge while synchronization is enrolled.
  Absence never implies purge, and payload/blob collection remains deferred to
  G17 acknowledgement and retention policy.

**Completed 2026-08-12.** Schema v21 adds a durable non-regressing HLC to each
operation, sparse per-field and lifecycle registers, LWW document-tag elements,
structural death certificates, and deterministic current repair records. G5
admission now folds the complete post-boundary G6 operation set and applies its
projection in the same transaction as vector advancement; an apply guard keeps
those canonical writes from echoing into a second local operation. Notebook
parent edges are considered by descending protocol order, invalid edges go to
the nearest valid root/Recovered home, and case-insensitive name collisions use
a stable ID suffix. Exhaustive 40,320-order, 250-seed randomized, two-SQLite-
replica, restart/upgrade, replay, clock, lifecycle, membership, and purge-gate
tests pass. Body revisions, resource bytes, carrier/crypto, public surfaces,
and retention collection remain outside G6. Evidence is archived in
`plans/v0.7/009-metadata-membership-tree-convergence.md` and
`performance/v0.7-g6/`.

## G7. Note revision objects, transfer deltas, three-way merge, and conflicts — complete

**Goal.** Synchronize note bodies without silently overwriting concurrent edits
and without making every small edit transfer the entire body.

**Scope.** Represent each immutable note revision with parent revision IDs,
complete result hash/length, optional delta object and named base, authoring
replica/sequence, and merge parents. Fetch or reconstruct the complete body,
verify its hash, find a common ancestor, perform the G1-selected three-way
merge through a bounded Notrios-owned pure-Go line/word implementation, and
create an ordinary merge revision. Non-overlapping edits merge;
overlap creates a durable visible conflict holding both inputs and base. Never
apply an unverified patch to canonical state.

**Boundaries.** Not live collaboration. No hidden conflict-note duplication.
Delta chains must be bounded and never be the sole recovery representation.

**Dependencies.** G1 merge behavior, G1a delta/provenance recommendation, G5
admission, and G6 document metadata.

**Working state.** Independent edits to different regions converge to one
verified merge revision; overlapping edits preserve both variants and never
pretend to converge until the conflict is resolved by another revision.

**Validation and evidence.** Unicode/Markdown/long-line fixtures; clean and
conflicting merges in every delivery order; missing/corrupt/wrong-base delta;
bounded chain fallback; delete/edit and restore/edit; transfer-byte comparison
against full snapshots.

**Resolved decisions (2026-08-11)**

- **Unresolved content lives in a typed conflict record attached to the same
  document and its revision graph**, not a second ordinary note. The UI can
  show base/local/remote without inventing a notebook/title or polluting search
  and publication.
- **More than one replica may emit an automatic clean merge** if the merge
  revision ID is content- and parent-derived so equivalent merges deduplicate;
  otherwise elect one emitter deterministically.

**Outcome (2026-08-12).** Schema v22 gives every revision an exact content hash,
byte length, and parent list, and the capture trigger refuses an enrolled
revision without one. `internal/syncdelta` is the reviewed promotion of the G1a
VCDIFF prototype with production bounds and a benefit gate; `internal/syncbody`
implements the bounded line-first merge with single-line word refinement plus
the revision DAG. Admission reconstructs and verifies a body before any
canonical write, merges concurrent edits into a derived-identity merge revision,
and records overlapping ones as a durable typed conflict on the same document.
`current_revision_id` is derived from the graph. Real two-replica evidence
transferred 55.37%, 27.92%, 16.49%, and 12.06% of the complete-body
counterfactual at G1's four offline intervals with every document converging.
Resource bytes, carrier, cryptography, catch-up, transport, and any REST/MCP/UI
surface remain outside G7. Evidence is archived in
`plans/v0.7/010-note-revision-objects-deltas-merge-conflicts.md` and
`performance/v0.7-g7/`.

## G8. Resource metadata, lazy materialization, chunks, and integrity — complete

**Goal.** Converge attachment references immediately while downloading bytes
only when policy or user action needs them.

**Scope.** Sync immutable resource metadata, permanent `resource://` identity,
content hash, MIME, length, chunk manifest where applicable, availability
state, and source peers. A note may reference an admitted but nonmaterialized
resource. Opening/downloading/pinning requests missing objects, verifies hashes
and MIME through the existing admission path, and atomically marks them local.
Support eager/pinned/lazy policy and bounded concurrent fetches. If G1a and G2
show a material benefit, permit a bounded delta only when a resource version
explicitly names an immutable parent; resources without that relationship use
complete or chunked transfer.

**Boundaries.** A remote-media URL is not a synced resource until ordinary
quarantine policy admits it. No direct carrier path leaks through REST/MCP/UI.
No placeholder bytes in the canonical asset store.

**Dependencies.** G1a binary-delta recommendation, G2 bounds, G5 admission, and
G7 revision references.

**Working state.** A replica can read/search a synchronized note before its
large attachment is local, request it later, verify it, deduplicate it, and
survive all advertised sources being temporarily offline.

**Validation and evidence.** Whole/chunked object fixtures; range/resume;
missing/corrupt/MIME-mismatch sources; shared-blob dedupe; pin/unpin; restart;
bounded parallelism and disk limits; no accidental remote-media fetch.

**Resolved decisions (2026-08-11)**

- **The default materialization policy admits metadata immediately and makes
  small inline-view resources eligible for eager fetch** within a byte
  budget; larger resources are lazy unless pinned. The G2 report supplies the
  threshold.
- **When no peer currently has bytes, keep the resource metadata and a visible
  `unavailable` state indefinitely;** never drop the reference or substitute an
  empty file.

**Outcome (2026-08-13).** Schema v23 lets a blob row exist without a file, with
a structural trigger refusing any row whose availability contradicts whether it
has bytes, so a note can reference an attachment this replica has not
downloaded. `internal/syncassets` owns G2's chunk plan, manifests with a
content-addressed digest, and the eager/pinned/lazy policy whose threshold is
G2's one mebibyte in both roles. Materialization fetches through a
transport-neutral `ObjectProvider`, stages verified chunks, resumes from what it
holds, and installs only after the manifest digest, every chunk hash, the
whole-object hash, the length, and the sniffed type agree. Admission time is
flat across a 256× attachment-size range and every object reconstructs exactly.
Resource deltas were considered and not implemented: G1a's benefit case needs a
named immutable parent, which Notrios resources do not have. An archive export
now refuses rather than omitting unmaterialized bytes. Evidence is archived in
`plans/v0.7/011-resource-metadata-lazy-materialization-chunks-integrity.md` and
`performance/v0.7-g8/`.

## G9. Deterministic envelope/container codec, encryption, and signatures — complete

**Goal.** Turn admitted operation ranges and objects into bounded, deterministic,
encrypted, signed protocol artifacts reusable by every transport.

**Scope.** Extend archive-v2 capabilities for change envelopes, state vectors,
dependencies, object/chunk indexes, and snapshot boundaries. Define canonical
bytes, compression profile, nonce/key derivation, associated data, signature
coverage, key IDs, version negotiation, limits, quarantine, manifest-last
publication, and streaming verification. Implement key interfaces with test
providers; production secret-store integration belongs to v0.8.

**Boundaries.** Do not invent crypto primitives, put passwords in manifests, or
make signatures substitute for encryption. Do not expose decrypted bulk bytes
through MCP.

**Dependencies.** G0 crypto decisions, G1a delta-format decision if adopted,
G2 codec bounds, G5 operation ranges, and G7/G8 objects.

**Working state.** The same logical envelope produces the same canonical
plaintext bytes, decrypts only for an enrolled key, verifies its sender, rejects
replay/tampering before canonical admission, and streams within fixed limits.

**Validation and evidence.** Independent golden fixture generator; known-answer
crypto tests; tampered header/ciphertext/signature/object; wrong/revoked key;
nonce uniqueness; decompression bombs; deterministic pre-encryption bytes;
license inventory.

**Resolved decisions (2026-08-11)**

- **The algorithm/library selection rule is approved.** Use standard-library
  Ed25519 and a maintained BSD/MIT/Apache authenticated-encryption/KDF
  implementation with mobile-compatible builds. Exact versions and parameters
  are a recorded output after G0/G2; do not assume `go-keyring` covers
  Android—it currently documents macOS, Linux/BSD, and Windows only.
- **Only the minimum routing tuple remains visible:** protocol/database/key IDs,
  artifact kind, bounded length, and content address where needed. Encrypt
  state vectors, record IDs, titles, MIME, and filenames.

**Outcome (2026-08-13).** `internal/syncwire` defines one canonical byte
representation per logical envelope and seals it as an NAR1 artifact:
AES-256-GCM under a per-artifact HKDF-SHA256 key, signed with Ed25519 over
domain-separated outer bytes, with the visible header serving as both the
derivation salt and the associated data. Routing names are keyed blinds, never
plaintext content hashes. Epoch advance and epoch retirement are separate acts,
so revocation does not cost a library its own history. Every primitive comes
from the Go standard library, so the license inventory is one line and no
dependency was added; no schema changed. Goldens are produced by an independent
Python implementation of `FORMAT.md` and compared byte for byte, alongside RFC
8032, NIST GCM, and RFC 5869 known-answer vectors. The canonical encoding is
59.4-59.7% smaller than the journal's JSON and 28-32% smaller compressed, with
constant 181-183 byte crypto overhead. G2's fixed-width identifier assumption
did not survive production identifiers and was replaced with length prefixes.
Signing for death certificates is supplied; wiring the enrolled-purge path to it
stays with G17's retention work. Evidence is archived in
`plans/v0.7/012-envelope-codec-encryption-signatures.md` and
`performance/v0.7-g9/`.

## G10. Snapshot catch-up and reset state machine — complete

**Goal.** Bring a new, far-behind, repaired, or user-reset replica to a known
state quickly through existing verified backup/restore machinery.

**Scope.** Define signed backup requests, responder selection, consent/policy,
archive-v2 snapshot generation, encrypted ZIP/container publication, vector and
journal cutover, resume, expiry, cancellation, and explicit
`adopt|replace|merge|fork` restore intent. Snapshot manifest records its state
vector; after restore the peer requests only later envelopes. A source must not
checkpoint live SQLite by copying a WAL-active main file.

**Boundaries.** A backup is not a peer acknowledgement and does not hold back
GC. No automatic destructive restore. No path-taking REST/MCP request.

**Dependencies.** G3 identities, G4 journal boundary, G9 secure container, live
archive-v2 export/verify/restore.

**Working state.** A blank or reset replica requests, resumes, verifies,
decrypts, explicitly restores, receives a fresh replica ID where required, then
applies post-snapshot changes to converge.

**Validation and evidence.** New-device and reset flows; interrupted export,
transfer, decrypt, verify, restore, and post-snapshot replay; stale/competing
responses; wrong password/key; source changes during snapshot; large-corpus
aggregate profile.

**Resolved decisions (2026-08-11)**

- **Only an active enrolled peer explicitly permitted as a snapshot source may
  answer a backup request;** if several answer, the requester chooses a
  compatible verified response rather than merging ZIPs.
- **Portable backups and peer catch-up use one archive payload format with two
  key-wrapping modes**—peer-recipient keys for ordinary catch-up and a
  memory-hard password KDF for portable/cloud backup. The password is entered
  at decrypt/restore time and never stored in the archive or command history.

**Outcome (2026-08-13).** `internal/synccatchup` signs requests and responses,
enforces that only an active enrolled peer explicitly permitted as a snapshot
source may answer, selects one compatible verified offer among competitors
rather than merging, and defines the interruptible state machine as an explicit
transition table. Schema v24 makes sessions, permissions, and catch-up floors
durable. Cutover installs the snapshot's vector, requires an explicit restore
intent, and writes no peer acknowledgement. The evidence run found a real
defect: a replica built from a snapshot has no predecessor operation rows, so
G5 refused the first operation after it; the catch-up floor is the only thing
permitted to stand in for a missing predecessor, and a replica without one still
leaves such operations pending. Measured 90.91% of operations avoided and
90.18-93.87% of time saved at 200 and 1,000 notes. Argon2id password wrapping
promotes `golang.org/x/crypto` from an indirect to a direct dependency at the
same version. Evidence is archived in
`plans/v0.7/013-snapshot-catchup-reset-state-machine.md` and
`performance/v0.7-g10/`.

## G11. Ephemeral shared-directory protocol and peer discovery

**Goal.** Synchronize without a direct connection through a disposable folder
that peers can recreate and inspect safely.

**Scope.** Implement a database-scoped layout with per-replica immutable
advertisements, requests, envelopes, objects, acknowledgements, snapshots, and
temporary staging. Writers use private temp + fsync + atomic rename where the
filesystem supports it and publish manifests last. Readers tolerate partial,
duplicate, reordered, stale, and missing files. Each peer writes only its own
namespace; no shared mutable `manifest.json`. Deleting the directory causes
republication from durable local state. UI-independent discovery reports
candidate peers but pairing remains explicit.

**Boundaries.** No filesystem watcher as the correctness mechanism; polling or
manual sync must work. Do not delete another peer's files. Do not trust locks,
mtime, filenames, or cloud-provider atomicity. No rclone dependency.

**Dependencies.** G5 planner, G9 artifacts, G10 snapshot messages.

**Working state.** Two or more local server instances converge through a newly
created directory; after the directory is deleted they recreate enough state
to discover missing work and converge again.

**Validation and evidence.** Same-machine, removable-directory simulation, and
delete/recreate tests; torn writes; no atomic rename; stale cache; case
insensitivity; reordered listings; two writers; bounded scan time/file count.

**Resolved decisions (2026-08-11)**

- **Any enrolled profile may initialize a missing directory** by creating the
  versioned/database-scoped skeleton and its own advertisement; no peer owns
  the carrier.
- **Only the writer removes its own expired artifacts after durable
  acknowledgement by all active peers**, and correctness must survive no
  cleanup at all. Carrier cleanup
  is not canonical GC.

## G12. Directory-carrier conformance over Google Drive and removable media

**Goal.** Prove the shared-directory adapter against the available mapped cloud
folder and a USB-like handoff without making either provider part of the engine.

**Scope.** Run the G11 transcript through `/home/renes/GoogleDrive` using
non-destructive immutable copy behavior where rclone is useful, plus staged
mount/unmount simulations representing a drive passed between peers. Record
eventual-listing delay, filename behavior, partial upload behavior, throughput,
and recovery. Add operator documentation and safe commands.

**Boundaries.** No direct cloud API/OAuth integration and no mobile rclone.
Never run `rclone sync`, `bisync`, `move`, `delete`, or `purge` against protocol
state. Do not commit cloud contents or private notes.

**Dependencies.** G11.

**Working state.** Desktop replicas converge through the mapped Google Drive
and through a directory that is alternately available to each peer; failures
produce bounded retry/status, not divergence.

**Validation and evidence.** Sanitized transcript and aggregate performance;
disconnect/reconnect; delayed listing; duplicate/conflicting immutable name;
full carrier loss and reconstruction; no-source-write checks.

**Resolved decisions (2026-08-11)**

- None. This is evidence for the provider-neutral adapter. A surprising
  provider limitation that changes the protocol must reopen G11 rather than be
  hidden in an rclone-specific workaround.

## G13. REST security foundation, pairing, and transport policy

**Goal.** Satisfy the security prerequisites that currently prohibit exposing
Notrios beyond loopback before adding sync endpoints.

**Scope.** Add authenticated peer principals, pairing/enrollment proof,
authorization limited to one database and sync capability, TLS configuration
and guidance, safe CORS/CSRF posture, quotas/rate limits, request/body/time
bounds, audit logs, key rotation/revocation hooks, and redacted status. Preserve
the default loopback-only local product.

**Boundaries.** Not general multi-user accounts or roles. Existing note REST
routes do not become remotely authorized merely because sync authentication
exists. No public deployment claim without an external security review.

**Dependencies.** G0 threat model, G3 profiles, G9 replica keys.

**Working state.** An enrolled peer can authenticate to a database-scoped sync
test endpoint; anonymous, wrong-database, revoked, replayed, cross-origin, and
over-quota requests are refused and audited.

**Validation and evidence.** Authentication/authorization matrix; TLS and
loopback/plaintext modes; CSRF/CORS tests; brute-force/rate/size/time limits;
credential redaction; audit review; security scan and manual threat-model
reconciliation.

**Resolved decisions (2026-08-11)**

- **Sync authentication authorizes no ordinary REST route.** A peer credential
  reaches only `/api/v1/sync/...` and the backup object it was explicitly
  granted; ordinary note APIs retain their existing local posture until a
  separate multi-user authorization milestone.
- **First pairing uses a short-lived, one-use pairing bundle transferred by
  QR/file/manual code**, containing no reusable library decryption key in
  displayable text. Exact UX waits for G18.

## G14. REST sync data plane and resumable encrypted backup download

**Goal.** Carry the same artifacts as G11 over a direct authenticated API,
including catch-up ZIP/container download.

**Scope.** Add handshake/vector exchange, missing-range plan, immutable object
HEAD/GET/PUT, envelope/ack/request publication, range/resume, snapshot request
and status, and bounded download of a completed encrypted backup artifact.
Downloads use opaque IDs and authorization, not server filesystem paths.
Clients verify protocol hashes/signatures/encryption after transport checks.

**Boundaries.** No arbitrary path parameters, directory listing, decrypted ZIP
streaming to MCP, or remote archive import request. REST never becomes the merge
implementation.

**Dependencies.** G10 and G13; replay the G11 protocol transcript.

**Working state.** Two replicas converge over REST and a new replica can resume
an interrupted encrypted snapshot download, verify/restore it, and continue
incrementally.

**Validation and evidence.** REST/directory golden transcript parity; range
`206/416`; interrupted/resumed transfer; ETag/content hash; stale/expired
artifact; auth/database scope; quota/backpressure; large snapshot streaming
with bounded memory.

**Resolved decisions (2026-08-11)**

- **ZIP may be the user-facing/download wrapper, but archive-v2 manifest/object
  verification and encryption define correctness.** Do not make ZIP
  central-directory parsing the trust boundary or require a seekable
  multi-gigabyte buffer.

## G15. Durable sync jobs, scheduling boundaries, retries, and MCP control

**Goal.** Make long sync/catch-up/resource operations observable, cancellable,
restart-safe, and bounded without turning Notrios into a workflow scheduler.

**Scope.** Extend schema-v18 jobs for sync plan/push/pull/catch-up/restore-prep
and resource-fetch phases, durable outbox, exponential backoff with jitter,
per-target concurrency/byte budgets, heartbeats, cooperative cancellation,
retry/reset, and structured audit/status. Reuse local checkpoints; do not store
raw argv or secrets. REST and MCP expose bounded status/conflict summaries and
safe control only where G13 authorization and MCP scopes permit it.

**Boundaries.** No general DAG, cron, background mobile service promise, bulk
bytes in MCP context, or path-taking MCP tool. Destructive restore still needs
local explicit confirmation.

**Dependencies.** G11/G14 transports and existing job plane.

**Working state.** A sync interrupted at every durable boundary resumes or
replans without double apply; cancellation stops at a safe checkpoint; a user
can see what is waiting and why.

**Validation and evidence.** Restart/crash/cancel/retry fixtures; offline and
quota backoff; two targets; stale heartbeat; status exit codes; REST/MCP size
and scope tests; secrets absent from job records.

**Resolved decisions (2026-08-11)**

- **MCP may plan and start ordinary incremental sync, request bounded resource
  fetch, and inspect status/conflicts at an explicit sync scope;** it may not
  enroll a peer, reveal keys, request/export a backup, retire a peer, purge, or
  apply a
  restore. Cancelling another actor's catch-up remains local UI/CLI/REST with
  authorization, not MCP.

## G16. Sync, pairing, catch-up, encrypted-backup, and conflict UI

**Goal.** Give the user a comprehensible path through setup, normal operation,
missing resources, conflicts, password-protected backups, reset, and repair.

**Scope.** Add profile selection and clear active-profile identity; sync target
`none|directory|rest`; shared-directory chooser; peer discovery and explicit
pairing; progress/pending/error/offline/behind/retired states; lazy-resource
download/pin; conflict base/local/remote comparison and resolution; notebook
repair reports; catch-up/reset request; encrypted backup password prompt with
show/retry/cancel and no persistence by default; destructive restore review.
Design responsive/mobile variants without claiming a mobile build.

**Boundaries.** The UI never invents merge/retention decisions, exposes keys or
paths to the web layer unnecessarily, or silently restores/retires/purges.

**Dependencies.** G3, G7/G8, G10/G11/G14/G15.

**Working state.** A non-expert can configure two local profiles, choose a
shared directory or REST peer, pair, catch up, sync, fetch an attachment,
resolve a conflict, and recover from a wrong backup password with explicit
state at each step.

**Validation and evidence.** React unit flows; real browser flow; keyboard and
screen-reader labels; narrow/touch layout; password redaction and clipboard
tests; two-process end-to-end demo; cancel and retry; screenshots contain no
private corpus.

**Resolved decisions (2026-08-11)**

- **v0.7 defines an injectable secret-store interface and uses an explicit
  locked-file development provider only with warnings and `0600`.** v0.8 selects
  and validates native desktop/Android stores. Do not claim the desktop-only
  `zalando/go-keyring` solves mobile storage.
- **The backup password is not remembered.** An opt-in native credential-store
  action can be added only after v0.8 validates the platform provider and labels
  the recovery consequences.

## G17. Peer retirement, retention horizon, tombstone/resource GC, and repair

**Goal.** Reclaim history and blobs without making an offline peer diverge or
allowing a stale device to resurrect deleted state.

**Scope.** Track active/retired/revoked peers, acknowledgement watermarks,
operation/checkpoint retention, death certificates, resource reachability,
snapshot floor, full-resync-required state, retirement preview/confirmation,
and repair from retained snapshot plus newer log. A backup sink never counts as
an active peer. Integrate the existing replaceable retention gate.

**Boundaries.** Time alone never proves safe deletion. Carrier cleanup is not
canonical GC. A retired peer cannot silently rejoin with old credentials.

**Dependencies.** G6 delete semantics, G8 resources, G10 snapshots, G15 status.

**Working state.** GC removes only state below the safe floor acknowledged by
all active peers; an offline peer within the horizon resumes incrementally; a
peer below the floor receives an explicit catch-up requirement; a retired peer
must re-enroll/reset.

**Validation and evidence.** Multi-peer watermark matrices; phone-in-drawer
simulation; retirement/revocation races; backup target excluded; resource still
needed by one peer; tombstone resurrection attack; dry-run/apply parity.

**Resolved decisions (2026-08-11)**

- **The initial default retention horizon is 90 days plus at least one verified
  snapshot floor**, configurable per profile, with warnings well before a peer
  crosses it. Evidence must report disk cost at real-corpus scale; if that
  invalidates the default, G17 reopens the value through a plan amendment.
- **Retirement does not require every peer online.** The local owner can sign a
  retirement decision, but other peers learn it through the ordinary log and
  refuse that replica thereafter. The UI must show peers that have not yet
  acknowledged the retirement.

## G18. Shared-core, FFI, Mermaid, and installation/mobile portability handoff

**Goal.** End v0.7 with an explicit, testable contract for the separate v0.8
installation/configuration/shared-core milestone and the independent post-1.0
Flutter client, so mobile delivery does not depend exclusively on Wails.

**Scope.** Inventory every runtime path, port, loopback/network permission,
shared-directory capability, file-picker need, credential-store operation,
background/lifecycle assumption, cgo/SQLite requirement, notification, and
deep-link association introduced by sync. Map them to Linux, Windows, macOS,
Android-emulator, and deferred iOS/physical-device gates. Audit `internal/service`
and HTTP handlers into a framework-neutral application facade shared by
`notriosd`, Wails, and a future `cmd/notrioslib` C ABI wrapper. Specify a small
versioned ABI: instance open/close, capability/version query, bounded serialized
request/response calls, typed errors, cancellation, polling/event delivery,
range/chunk stream handles, and an explicit result-buffer release or
caller-owned-buffer contract. Use opaque handles rather than exposing Go
pointers or Store objects. Map existing REST request/response schemas where
they are transport-neutral; name each HTTP-only concept that needs an ABI
status/error/stream equivalent. Audit native packaging, cgo pointer/thread
rules, SQLite linkage, secret-store injection, and lifecycle ownership.

Also record the current GUI Markdown baseline. Source inspection on 2026-08-11
found that `md-editor-rt` contains optional Mermaid support but Notrios
deliberately sets `noMermaid: true` because E6a disabled every unbundled
runtime/CDN extension. G18 therefore writes the exact offline bundle,
sanitization, CSP, malformed/oversized-diagram, browser, and Wails evidence that
v0.8 must require before claiming Mermaid support; a package capability is not
an application test.

**Boundaries.** No installer, Flutter app, production C ABI, APK, Wails v3
migration, Mermaid enablement, Play Store claim, or physical-device performance
claim. Do not implement an FFI bridge by routing through a loopback socket, by
exposing one C symbol per REST endpoint, or by sharing live Go/Dart pointers.
Flutter Web cannot load the native C ABI through `dart:ffi`; any future web
client continues over REST or needs a separately designed Go-Wasm/JS adapter.

**Dependencies.** G3, G9, G11, G13, G16.

**Design reference.** `FLUTTER_GO_CLIENT.md` records the verified upstream
facts and proposed ABI shape; this item remains self-contained if that document
changes later.

**Working state.** v0.8 can start from a finite permission/platform matrix, a
versioned application-facade/ABI proposal, and compile-time seams rather than
rediscovering hidden desktop assumptions. It can also distinguish the Mermaid
feature present in an upstream package from the feature currently disabled in
Notrios.

**Validation and evidence.** Dependency/build-tag audit; headless and Wails v2
desktop regression; facade-to-REST semantic mapping; ABI ownership/error/
cancel/stream test design; Android-emulator cross-compile feasibility report
where tooling allows; documented post-1.0 physical-device and iOS gates; no
unsupported mobile or Wails import in the shared core; Mermaid fixture and
offline/security test design.

**Resolved decisions (2026-08-11)**

- A no-GUI Go library and stable C ABI are pre-1.0 deliverables, but their
  design/build belongs to v0.8 after this evidence handoff; G18 does not smuggle
  an ABI implementation into synchronization.
- The ABI reuses application semantics and serialized schema types from REST,
  but adds non-HTTP lifecycle, ownership, cancellation, event, capability,
  typed-error, and bounded-stream contracts. Those are ABI concerns, not new
  ordinary REST routes.
- Pre-1.0 mobile evidence is Android-emulator-only. Physical Android/iOS
  support is a post-1.0 Flutter release gate.
- Wails mobile remains an option if it matures, not the only mobile roadmap.
  Flutter native targets use Dart FFI; Flutter Web is outside that ABI.
- `flutter_smooth_markdown` is a candidate, not selected merely from its
  feature list. Its Markdown fidelity, editor behavior, Mermaid subset,
  sanitization, accessibility, performance, maintenance, and BSD-3-Clause
  dependency tree must pass a post-1.0 spike before adoption.

## G19. Archive-v2 compatibility bridge

**Goal.** Publish the external archive contract deferred from v0.4 only after
the sync-era container capabilities are stable.

**Scope.** Publish JSON Schemas, capability/version bounds, sanitized
deterministic loose/packed/sync-era golden fixtures, and a compatibility command.
Coordinate `movenotes-v3/notrios2sql.py` against verified fixtures and add
cross-version consumer tests if that external importer now exists.

**Boundaries.** Gated on G9, not on transport completion. Do not claim an
external consumer that does not exist; a missing consumer yields producer-side
contract evidence, not invented coordination.

**Dependencies.** G9; archive v2 P2–P4.

**Working state.** A consumer can validate which Notrios archive/container
capabilities it supports and safely refuse the rest.

**Validation and evidence.** Schema validation; independent fixtures;
current/previous reader matrix; unknown required capability refusal; external
consumer test or explicit evidence that it remains absent.

**Open decisions**

- None unless the external importer has selected an incompatible contract; in
  that case add an investigation item rather than bending the format silently.

## G20. Full convergence, disaster recovery, security, and release wrap-up

**Goal.** Reconcile every v0.7 promise against source and prove the milestone as
one system before changing the product version.

**Scope.** Run three-plus-peer randomized convergence, two-process directory
and REST end-to-end tests, carrier deletion/recreation, catch-up/reset, lazy
resources, conflict resolution, retention/full-resync, security abuse cases,
large-corpus performance, documentation/API/OpenAPI/MCP parity, dependency
licenses, clean-install upgrade, and release packaging. Fix only defects needed
to satisfy already approved v0.7 contracts; new features return to planning.

**Boundaries.** No GitHub push, tag, or public release without separate user
authorization. No v0.8 installer/mobile work.

**Dependencies.** G3–G19 as applicable.

**Working state.** Product/version/schema/docs agree; all supported peers
converge or report a typed recoverable state; source ZIP verifies and is copied
to the evidence directory; the active plan can be archived and the next plan is
created from the roadmap only after user review.

**Validation and evidence.** Full repository checks and smoke tests; mutation
or fault-injection evidence for merge/admission/GC boundaries; 382,206-note
aggregate snapshot/incremental profile; attachment-bearing lazy-fetch profile;
security review; upgrade/rollback notes; verified ZIP checksum.

**Open decisions**

- **What is the v0.7 version/schema release number? — Non-blocking until this
  item.** Default: increment schema once per required migration but ship one
  product version `0.7.0`; the completion report records every schema step.

## Decisions register

This table is an index only. The owning item's text contains the options,
recommendation, blocking status, and consequence.

| Decision | Owner | Status |
|---|---|---|
| Mandatory E2EE scope | G0 | Resolved: every v1 payload; local-dev test exception only |
| Per-replica signatures | G0 | Resolved: Ed25519 over named canonical artifacts |
| SVN dump compatibility | G0 | Resolved: no |
| Canonical full-body/delta representation | G1 | Resolved: full object plus optional named-parent delta |
| Three-way merge granularity/ownership | G1/G1a | Resolved: bounded line-first plus word-region refinement in Notrios-owned pure Go; external candidate superseded |
| Binary-delta algorithm/format | G1a | Resolved: recommend constrained RFC 3284 default-table VCDIFF; not svndiff or NXD1 |
| Delta implementation provenance | G1a | Resolved: independent spec-first Go; attributed Apache behavioral reference; no GPL-derived code |
| Investigation prototype promotion | G1a | Resolved: performance-only; later G7/G8 must approve rewrite or promotion after G2 |
| Offline intervals measured | G1 | Resolved: 1 hour/day/week/30 days |
| Envelope encoding/compression | G2 | Resolved: NCB1 compact records plus canonical-JSON outer candidate and pinned deterministic gzip; G9 goldens required |
| Resource chunk threshold | G2 | Resolved: whole below 1 MiB; 1 MiB fixed chunks above; FastCDC deferred |
| Android bounds | G2/G18 | Emulator-only before 1.0; physical-device gate after 1.0 |
| Profile config layout | G3 | Resolved: registry plus per-profile config |
| Copied database enrollment | G3 | Resolved: refuse until adopt/fork; transfer via export/import |
| Field-register/record map | G4 | Resolved classification rule; G4 records exact table before code |
| Journal start boundary | G4 | Resolved: explicit enrollment/snapshot boundary |
| Compatibility mismatches | G5 | Resolved: exact database/protocol/capability rules in item |
| Membership add/remove rule | G6 | Resolved: LWW per element by protocol order |
| Notebook-cycle repair | G6 | Resolved: deterministic winning-parent repair |
| Purge propagation | G6 | Resolved: signed death certificate and new identity on restore |
| Conflict representation | G7 | Resolved: typed same-document conflict record |
| Automatic merge dedup/emitter | G7 | Resolved: multiple emitters with derived deduplicating ID |
| Lazy-resource defaults/unavailable bytes | G8 | Resolved: bounded eager/lazy default; preserve unavailable |
| Crypto algorithms/libraries | G9 | Resolved selection rule; exact versions/parameters follow G0/G2 |
| Visible protocol metadata | G9 | Resolved: minimum routing tuple only |
| Snapshot responder policy | G10 | Resolved: explicitly permitted active enrolled peers |
| Peer-key versus password backup wrapping | G10 | Resolved: one payload, two wrapping modes |
| Directory initialization | G11 | Resolved: any enrolled profile |
| Carrier artifact cleanup | G11 | Resolved: writer-owned and acknowledgement-gated |
| Sync credential reach into ordinary REST | G13 | Resolved: none |
| Pairing bootstrap | G13 | Resolved: short-lived one-use bundle |
| ZIP wrapper contract | G14 | Resolved: UX wrapper, not trust boundary |
| MCP sync controls | G15 | Resolved: bounded incremental controls only |
| v0.7 secret-store provider | G16 | Resolved: interface plus warned `0600` development provider |
| Remember backup password | G16 | Resolved: no |
| Retention horizon | G17 | Resolved: 90 days plus verified snapshot floor |
| Offline peer retirement | G17 | Resolved: no all-peers-online requirement |
| Shared-core/FFI and Flutter boundary | G18 | Resolved: pre-1.0 ABI; post-1.0 client; no Web FFI |
| Current-GUI Mermaid baseline | G18 | Resolved fact: upstream-capable but disabled pending offline/security evidence |
| Release version/schema bookkeeping | G20 | Open, non-blocking until wrap-up |

G0-G10 are complete. G11 is the next implementable item, but is not approved.
Implementation begins only after an explicit instruction naming G11; completing
it still stops for a verified ZIP and approval before G12.
