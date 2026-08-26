# Plan: v0.7 — Native synchronization

Status: **G0-G18 are complete through 2026-08-25. Product version remains 0.6.0 and
the canonical schema is v27. The user resolved G17b's evidence scope, exact
OpenPGP identity, and RFC-3161 provider order on 2026-08-25, then authorized
the exact signer/Secret Service workflow, TSA requests, and reserve writes and
attested offline backup/revocation readiness. G18a-G18g are not approved. The
G17b pre-push evidence gate is now mandatory; no GitHub push or
physical optical burn occurred.** The
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
existing archive-v2 container has already been verified at 382,206 notes, but
the current catch-up path wraps a loose archive in a stored ZIP and adds about
25% at only 100-500 notes. Its packed layout has not been measured end to end
through catch-up at the supplied full-corpus scale. G14a-G14e therefore reopen
the physical snapshot choice without discarding archive-v2's semantic,
compatibility, identity, and verification contract.

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

G17b adds the missing custody step to this rule. After G17b is complete, every
new release ZIP must be hashed, signed, appended to the checked-in evidence
manifest, covered by an independently timestamped signed checkpoint, verified
from a clean process, and assigned to an external immutable ISO checkpoint
before the corresponding task is called complete. No GitHub push may occur until the host-side evidence gate
confirms that every frozen artifact is covered. A push, a network timestamp
request, use or creation of a signing key, and writing physical optical media
each remain separately authorized operations; this plan does not silently
grant them.

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

**Outcome (2026-08-11).** Archived as
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

**Outcome (2026-08-11).** Archived as
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

**Outcome (2026-08-11).** Archived as
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

**Outcome (2026-08-11).** The compact NCB1 record candidate met
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

## G3. Profiles, replica identity, and multi-instance process isolation — complete

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

**Outcome (2026-08-12).** Runtime profiles now use the version-2
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

**Outcome (2026-08-12).** Schema v19 adds explicit local enrollment/snapshot
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

## G5. State vectors, missing-range planning, and idempotent admission — complete

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

**Outcome (2026-08-12).** Schema v20 persists the compatibility tuple for an
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

## G6. Deterministic metadata, membership, deletion, and tree convergence — complete

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

**Outcome (2026-08-12).** Schema v21 adds a durable non-regressing HLC to each
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

## G11. Ephemeral shared-directory protocol and peer discovery — complete

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

**Outcome (2026-08-13).** Complete, archived as
`plans/v0.7/014-ephemeral-shared-directory-protocol.md` with evidence under
`performance/v0.7-g11/`. `internal/synccarrier` holds the carrier, the round,
and the object provider G8 left for this slice; `internal/synckeys` is the
warned `0600` development secret provider; `notriosctl sync` is the first way to
run any of it. No schema change. **The illustrative layout in
`SYNCHRONIZATION.md` was wrong in three ways and is corrected there**: object
paths published plaintext content hashes, envelope names published sequence
ranges, and a separate acknowledgement class would have been a second source for
what a state vector already says. Two defects were found and fixed — a torn
artifact could never be repaired, because an existing name was taken as proof
one was there; and a fresh carrier was a standoff, because publishing waited on
an advertisement instead of starting from what the journal remembers each peer
acknowledged. Measured: the carrier layer is about one percent of an exchange,
ten quiet rounds add zero artifacts, and a carrier deleted with work in flight
recovers in two rounds. Cloud and removable-media evidence remains G12's.

## G12. Directory-carrier conformance over Google Drive and removable media — complete

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

**Outcome (2026-08-13).** Complete, archived as
`plans/v0.7/015-directory-carrier-conformance.md` with evidence under
`performance/v0.7-g12/`. **No production code changed** — the harness drives the
shipped round and adds no dependency, and no provider limitation forced a
protocol change. All eight phases passed against a Google Drive folder mounted
with `rclone mount`: convergence, no foreign writes, a substituted artifact
refused and repaired, an advertisement arriving before its envelopes, a
disconnected carrier, full carrier loss, `rclone copy --immutable` in both
directions with the destructive verbs refused in code, and a drive passed
between peers. **Two measurements now constrain later slices and are recorded in
`SYNCHRONIZATION.md`:** another device's change took 45-57 seconds to become
visible, and resolving a known name is no fresher than listing the directory.
The consequence is stated plainly — publication order is a latency optimization
on such a carrier, not a correctness mechanism, and the state vector is what
carries correctness. `docs/operations.md` gains the operator section, the safe
commands, and the never-run list.

## G13. REST security foundation, pairing, and transport policy — complete

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

**Outcome (2026-08-13).** Complete, archived as
`plans/v0.7/016-rest-security-pairing-transport-policy.md` with evidence under
`performance/v0.7-g13/`. Schema **v25** adds `sync_peer_keys` and
`sync_pairing_invitations`. `internal/syncauth` defines the peer principal — an
Ed25519 signature over the method, path, database id, replica id, timestamp,
nonce, and body hash, never a bearer token — plus the pairing proofs, the replay
cache, and the two rate limiters. Three routes landed:
`/api/v1/sync/handshake`, `/api/v1/sync/pair`, and a loopback-only redacted
`/api/v1/sync/status`. **A peer credential authorizes that surface and nothing
else**, asserted by comparing an ordinary route's answer with and without one.
**G11's clear-text development bundle is gone**: pairing is now a short-lived,
single-use code, spent in one transaction, under which the group key travels
sealed — so no artifact carries a reusable library key in displayable text. Peer
public keys moved into the database, where enrolment and revocation are
transactional and audited. The transport policy is a **startup refusal**, not a
warning: a non-loopback listener without TLS, a certificate without its key, or
unreadable TLS material makes the service exit and name the setting. Seventeen
matrix cases, three authorized and fourteen refused, none explaining itself. No
data plane (G14), no multi-user concept, no public deployment claim, and no
external security review.

## G14. REST sync data plane and resumable encrypted backup download — complete

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

**Outcome (2026-08-14).** Complete, archived as
`plans/v0.7/017-rest-sync-data-plane.md` with evidence under
`performance/v0.7-g14/`. No schema change. `internal/syncrest` implements
G11's `Carrier` over G13's signing client, so **REST/directory transcript parity
is by construction** rather than by a second implementation — asserted by
comparing what each carrier holds. Measured, REST costs +15.4% at 100 notes and
+0.07% at 500 against the same exchange through a folder: admission dominates,
so keeping the merge off the server costs nothing. `internal/syncbackup` packs,
seals in fixed authenticated frames, and extracts safely; a snapshot passes four
gates in order — declared hash, frames, container, **then archive-v2's own
verifier** — and a test asserts which layer refuses a tampered byte. Backups are
addressed by opaque id, produced only for an explicitly permitted replica, and
fetched only by the replica that asked. The working state is closed end to end:
an interrupted download resumes, verifies, restores under an explicit intent,
and then continues incrementally. **A defect in G13 was found and fixed here:**
the per-address failure budget was spent on every request rather than on
refusals, so the second data-plane round returned `429` and a legitimate peer
throttled itself out of its own library. Scheduling, retries, and backpressure
remain G15's.

## G14a. Investigation — archive scalability benchmark contract and resumable harness — complete

**Goal.** Make the format decision reproducible before another long private-
corpus run, with each phase independently resumable after interruption and no
private content entering the repository.

**Scope.** Add an evidence-only harness and aggregate schema for the three
supplied corpora: the equivalent recipe Joplin RAW and Obsidian libraries, and
the attachment-bearing Joplin RAW export. Record source item/file counts,
apparent and allocated bytes, per-directory entry histograms, resource counts,
filesystem type, tool versions, CPU, memory, and cache-state limitations. Drive
generated calibration tiers through foreign import, native full export,
verification, transport preparation/sealing, extraction/open, restore, and
post-snapshot incremental replay. Make every phase write an atomic checkpoint
and a separately valid aggregate result so a usage limit never requires
repeating completed hours of work.

Define adapters, without adding production dependencies, for these full-scale
G14b candidates: current loose archive-v2; archive-v2 `--pack`; the current
loose-directory + stored-ZIP + authenticated-frame catch-up path; packed
archive-v2 through the same catch-up path; a stopped-database file copy; the
SQLite Online Backup API; and a versioned SQLite-image bundle with external
resources/source bundles packed behind a manifest. Define comparable restic
and borg commands for both the raw source tree and the canonical Notrios state.
Never mutate the existing `/media/renes/HD2/recipedb_repo`; comparison runs use
new, explicitly named repositories.

**Boundaries.** Evidence/prototype code only: no production format, schema,
default, dependency, database encryption, or catch-up behavior changes. Do not
commit private paths, filenames, titles, bodies, hashes, databases, archives,
resource bytes, repository contents, or cloud contents. The available test
filesystems are ext4 plus the Google Drive FUSE mapping; exFAT is not currently
available, so this slice may prove bounded directory shape but must not claim
measured exFAT performance. Do not drop kernel caches, modify mounts, or delete
an existing borg/restic repository.

SQLCipher is an encrypted SQLite implementation, not by itself a complete
backup container for external assets, a semantic subset/merge format, or a
signed multi-file snapshot. Measure or model its encryption role separately
from physical snapshot layout. A Go port of Borg and Bluge are outside this
sequence: Bluge is a search index, not the authoritative transactional
key-to-pack index a backup repository requires.

**Dependencies.** G14, P3a/P3b/P4 evidence, and the existing importer profile
harnesses.

**Working state.** One command can run a named phase/tier, resume without
overwriting another phase, validate aggregate privacy and arithmetic, and emit
a machine-readable comparison row. Generated calibration proves all adapters
measure the same stage boundaries before private full-scale work begins.

**Validation and evidence.** Harness unit tests; interrupted/resumed phase;
atomic result publication; source-read-only checks; aggregate privacy scanner;
generated 10k/100k calibration; canonical content fingerprints rather than row
counts alone; `PRAGMA integrity_check` for database images; exact archive and
sealed-byte hashes; filesystem entry totals and maximum entries in one
directory; wall/CPU time, peak RSS, read/write bytes where available, output
bytes, compression ratio, and tool versions. Archive under
`performance/v0.7-g14a/`.

**Open decisions**

- **Benchmark acceptance policy — Non-blocking.** Default: correctness and
  recoverability are mandatory; a candidate fails if work is superlinear in
  observed items/bytes, a full stage does not finish within two hours on the
  reference machine, desktop peak RSS exceeds 512 MiB, the receiver-side
  open/verify/restore path exceeds a 256 MiB proxy bound, or transport/archive
  filesystem entries grow one-for-one with notes/objects. Among candidates
  that pass, compare end-to-end create + verify + transfer preparation + open +
  restore, and require a candidate offering the same semantics to stay within
  2× the fastest such candidate unless a recorded integrity, compatibility, or
  resume property justifies the cost. Restic/borg rows are references, not an
  excuse to compare unlike stages as one number.
- **Cache control — Non-blocking.** Default: run interleaved first and repeat
  passes and label them honestly; do not call a pass "cold" unless the harness
  actually controlled cache state.

**Outcome (2026-08-15).** Complete, archived as
`plans/v0.7/019-archive-scalability-benchmark-harness.md` with generated,
aggregate-only evidence under `performance/v0.7-g14a/`. The approved defaults
were used: every pass is labeled `interleaved-first`, no cache state is called
cold, correctness/recoverability are mandatory, and the 2-hour/512-MiB/
256-MiB/non-object-per-file gates remain unchanged for G14b. One command now
runs or resumes any named tier/phase, atomically publishes an immutable result,
validates privacy/arithmetic, and emits the 11-adapter stage map. Both 10k and
100k generated loose-path calibrations completed all nine stages. The 100k
loose archive carried 100,093 files and therefore fails the object-per-file
gate even though fanout bounded one directory at 256 entries; stored ZIP added
26,224,320 bytes (11.15%) while authenticated framing added only 6,004 bytes.
Snapshot open and restore remained below 92 MiB and content fingerprints
matched, but one post-snapshot replay reached 797,937,664 bytes peak RSS and
fails the 512 MiB desktop gate. G14a changes no production code, schema,
dependency, archive default, encryption, or catch-up behavior. G14b completed
the full-corpus selection without changing those production behaviors.

## G14b. Investigation — full-corpus baseline and physical snapshot selection — complete

**Goal.** Decide, from full-scale evidence, whether archive-v2 packing salvages
the current backup/catch-up path or whether full snapshots need a compatible
SQLite-image capability while semantic import/export remains record based.

**Scope.** Run the G14a matrix phase by phase against the supplied recipe pair
and attachment-bearing Joplin export. Import the equivalent Joplin and Obsidian
recipe sources into isolated Notrios databases and require equivalent canonical
content aggregates before using them as two source-format views of one
workload. Compare current loose/packed export, verify, restore, ZIP/seal/open,
stopped copy, Online Backup API, SQLite-image + packed-assets prototype, restic,
and borg. Test an unchanged second snapshot separately from a first snapshot so
repository deduplication is not confused with full-backup speed. Record ext4
results and Google Drive copy/visibility separately; never restore directly
into the provider mapping and call provider latency a format cost.

The outcome must choose one of: (A) keep semantic archive-v2 and make a bounded
packed representation the full-snapshot default; (B) add a required
archive-v2 SQLite-image capability for compatible full backup/catch-up while
retaining semantic records for subset, merge, and long-lived interchange; or
(C) schedule a new repository/chunk investigation because both fail. SQLCipher
or current authenticated framing may protect a chosen SQLite-image payload,
but encryption does not decide A/B/C. The report also answers why restic/borg
repository features help repeated general filesystem backups yet still pay to
recreate every source file on restore, whereas Notrios can exploit canonical
SQLite state and packed assets.

**Boundaries.** Investigation and performance-only prototypes; no production
format/default/dependency change. Do not port Borg, adopt Bluge, or treat a raw
copy of a live WAL database as a snapshot. Do not publish a universal exFAT,
mobile, or cloud-filesystem performance claim from the available ext4/FUSE
hosts.

**Dependencies.** G14a.

**Working state.** Aggregate full-corpus evidence names the selected physical
snapshot design, rejected alternatives, measured tradeoffs, compatibility and
failure model, and the exact production work G14c/G14d must perform. `PLAN.md`
is amended so G14c's blocking decision is resolved before implementation is
approved.

**Validation and evidence.** Complete/full-corpus phase records; independent
validators; semantic fingerprints across Joplin/Obsidian imports and every
restore; attachment/source-bundle fingerprints; corrupt/truncated candidate
refusal; interrupted production/resume classification; unchanged second run;
restic `check --read-data` and borg `check --verify-data` where supported;
maximum directory width; end-to-end comparison table with no combined
incomparable stage. Archive under `performance/v0.7-g14b/`.

**Open decisions**

- **Physical full-snapshot representation — Resolved 2026-08-22: option B.**
  Use a required, versioned SQLite-image plus bounded packed-assets capability
  for compatible same-schema whole-library backup and catch-up. Retain packed
  semantic archive-v2 for subset, merge, schema-independent interchange, and
  fallback recovery. The image path was 2.33x faster locally and 1.99x faster
  including the separately measured provider copy while remaining within the
  correctness, memory, time, and file-shape gates. The semantic artifact was
  78.3% smaller, so option B complements rather than replaces it. Loose layouts
  and raw Restic/Borg source-tree paths failed the approved shape or memory
  gates. The required state classification, compatibility, pack, resume, and
  restore rules are pinned in G14c and
  `performance/v0.7-g14b/SQLITE_IMAGE_CAPABILITY.md`.

**Outcome (2026-08-22).** Complete, archived as
`plans/v0.7/020-full-corpus-physical-snapshot-selection.md` with 57 validated,
aggregate-only phase rows under `performance/v0.7-g14b/`. Option B is selected
for G14c. Both semantic and physical candidates restored the exact 382,206-note
canonical aggregate; the image bundle completed the local path in 1,884.2
seconds versus 4,384.0 seconds for packed semantic archive-v2. Restic and Borg
data checks and exact restores completed for canonical and 1,237,553-file raw
inputs; repository storage was bounded, but raw traversal/restore repeatedly
failed memory gates. The investigation also records importer scale failures as
separate future performance debt. No production format, schema, dependency,
default, encryption, or catch-up behavior changed. G14c is next and requires
explicit approval.

## G14c. Implement the selected scalable native snapshot representation — complete

**Goal.** Turn G14b's selected design into a versioned, bounded production
snapshot without weakening archive-v2 verification, compatibility, or privacy.

**Scope.** Implement only the representation selected and written into this
item by G14b. If A wins, make pack creation, trailers/indexes, compression, and
resume behavior scale and remove per-entry ZIP amplification. If B wins, add a
required capability whose manifest binds a consistent SQLite snapshot and
packed external assets/source bundles, names exact schema/application bounds,
excludes or rebuilds derived/local-only state, and retains semantic archive-v2
for subset/merge/interchange. Pin any compressor with deterministic goldens,
bounded decoder memory/expansion, license review, and mobile-proxy evidence.

**Boundaries.** No raw live-WAL copy, silent format reinterpretation, SQLCipher
driver migration, general deduplicating repository, Borg port, or Bluge index.
No removal of loose/archive-v2 read compatibility. Do not change import/export
semantics merely to win a benchmark.

**Dependencies.** G14b selection.

**Working state.** New full snapshots use the selected scalable representation;
old loose and packed archives still verify/restore according to their declared
capabilities; interrupted production is incomplete or resumable by an explicit
tested rule; output directory width and receiver memory are bounded by bytes/
pack limits rather than note count.

**Validation and evidence.** Independent golden fixture; previous/current
reader matrix; deterministic output where promised; pack/container corruption,
truncation, expansion and path attacks; fault injection at publication; bounded
memory; generated 100k run; license inventory; no private content.

**Open decisions**

- **Selected representation — Resolved 2026-08-22: option B.** Implement a
  required `sqlite-image+packed-assets.v1` capability as the default for
  compatible, whole-library full backup and synchronization catch-up. Create a
  consistent image with SQLite Online Backup while writes are live; a stopped,
  checkpointed close may use the same capability. Bind the image, exact schema/
  application bounds, snapshot vector/floor, database identity, and every
  external pack in a manifest. Keep semantic packed archive-v2 as the supported
  subset/merge/interchange and incompatible-schema fallback; keep all existing
  loose and packed readers.
- **Compression and pack limits — Resolved 2026-08-22.** Add no compressor or
  compression dependency in G14c. Deterministic stored external packs close
  before 256 MiB of payload or 65,536 entries; one item larger than the byte
  target occupies a separately declared oversized pack and is streamed with
  the existing resource limits. Publish each hash-verified pack through a
  private partial file, fsync, and atomic rename, with the manifest last. Resume
  only from complete verified pack boundaries; restart an interrupted SQLite
  image creation. A later compression change requires its own evidence and
  capability version.
- **Admission, migration, and local state — Resolved 2026-08-22.** Admit the
  physical path only for its exact declared schema/application compatibility
  range. An incompatible image refuses and directs the user to semantic
  archive-v2; it is never silently migrated in place. Before cutover, verify
  hashes, pack lengths, capability, SQLite integrity, vector/floor, and external
  completeness, then make an emergency snapshot. Install via durable staging,
  preserve the logical database identity, mint a new replica identity, clear
  reviewed local/transient state and private path material, and rebuild invalid
  FTS/derived projections. Signing and group-encryption private keys stay
  outside the image. G14c must review every table against the state classes in
  `performance/v0.7-g14b/SQLITE_IMAGE_CAPABILITY.md` and test interruption at
  every publication/cutover boundary. G14c tests representation publication;
  G14d owns emergency backup, installation/cutover, identity rotation, derived
  rebuild, and replay fault boundaries because G14c performs no installation.
- **Measured justification — Resolved 2026-08-22.** At 382,206 documents the
  physical local path took 1,884.2 seconds versus 4,384.0 for packed semantic
  restore (2.33x faster), wrote 6.04 GB versus 28.4 GB during recovery, stayed
  below the absolute memory gates, and restored exactly. Including the distinct
  Google Drive copy yielded 2,245.1 versus 4,457.6 seconds (1.99x). The 6.040 GB
  image is larger than the 1.309 GB semantic artifact, so semantic export stays
  first-class rather than becoming a compatibility afterthought.

**Outcome (2026-08-22).** Complete, archived as
`plans/v0.7/021-scalable-native-snapshot-representation.md`. Production
`sqlite-image+packed-assets.v1` creation uses SQLite Online Backup, securely
clears reviewed machine-local state in the copy, streams deterministic
uncompressed USTAR packs under the 256 MiB/65,536-entry limits, resumes only
verified pack boundaries, restarts interrupted database images, and publishes
the manifest last. Read-only admission checks exact schema/application
capabilities, all hashes/lengths, SQLite integrity, identity, vector/floors,
safe paths, deterministic pack structure, and database-to-pack completeness.
`notriosctl snapshot create|verify` is local-filesystem only and stops at
install-ready staging; G14d still owns encrypted catch-up, emergency backup,
atomic cutover, replica rotation, derived rebuild, and replay. Independent
golden, reader-matrix, corruption/fault, and generated 100k tests pass; the
100k image used 23,547,904 bytes peak RSS. No schema, archive-v2 compatibility,
compressor, dependency, REST/MCP surface, or current catch-up behavior changed.

## G14d. Scalable restore, emergency backup, and synchronization catch-up — complete

**Goal.** Use G14c's representation safely for full backup/restore and the G10/
G14 catch-up loop on desktop and the pre-mobile bounded core.

**Scope.** Replace the loose-object stored-ZIP catch-up production path with the
selected bounded representation; stream seal/download/open/verify without a
seekable multi-gigabyte buffer or one ZIP entry per content object. Before a
replace/reset, create and verify an emergency snapshot of current canonical
state. Enforce compatible schema/capability, explicit restore intent, database/
replica identity rotation, catch-up floor, unavailable-resource policy, and
post-snapshot incremental replay. Make interruption restart/resume rules
durable at every stage.

**Boundaries.** No automatic destructive restore, no archive bytes through MCP,
no arbitrary remote path, no background scheduler (still G15), and no claim
that a desktop proxy is a physical mobile result.

**Dependencies.** G10, G14, G14c.

**Working state.** A large snapshot can be requested, produced, resumed,
verified, restored under explicit intent, and followed by incremental sync
without materializing an object-per-note transport tree. A failed replacement
leaves a verified emergency snapshot and a typed recoverable state.

**Validation and evidence.** Multi-gigabyte range resume; crash/fault injection
at snapshot, seal, download, open, verify, emergency backup, replace and cutover;
wrong schema/capability/key; tamper/truncation; bounded memory and disk space;
directory and REST carrier parity; Android-emulator checklist update.

**Open decisions**

- None beyond G14c's resolved representation. If restore requires a materially
  different physical format from production, stop and add an investigation
  rather than hiding a second format here.

**Outcome (2026-08-22).** G14d is complete and archived under
`plans/v0.7/022-scalable-restore-catchup.md`. The selected G14c physical
representation now moves through deterministic sequential USTAR plus the
existing fixed authenticated frames over resumable REST and directory bulk
paths. Explicit local replace/adopt creates a verified emergency snapshot,
persists and resumes every blocked cutover stage, rotates the writable replica,
installs snapshot vector/floors, queues derived projections, and converges after
ordinary incremental replay. Sparse range resume beyond 3 GiB, injected failure
at every recovery boundary, wrong key/tamper/ownership, bounded memory/disk
shape, and the generated 100k restore gate pass. The approved default remained
G14c's physical format; no second restore format, schema, compressor, dependency,
automatic destructive action, MCP path, or physical-mobile claim was added.

## G14e. Full-scale archive/catch-up acceptance and format freeze — complete

**Goal.** Prove the production choice on the supplied real corpora and freeze
it before scheduling, UI, retention, or external compatibility build on it.

**Scope.** Re-run the G14b comparison with production G14c/G14d code. Cover
Joplin and Obsidian import into equivalent canonical libraries, portable native
export/verify/restore, first and unchanged backup, attachment/source-bundle
round trip, REST and directory catch-up, emergency replacement, and
post-snapshot incremental convergence. Compare the same frozen restic/borg
baseline rows and explain semantic differences. Update archive, sync, user,
operations, troubleshooting, testing, security, and compatibility documents;
feed the frozen capability into G19.

**Boundaries.** Aggregate-only committed evidence. No private archive/database,
cloud content, GitHub action, public release, or claim about untested exFAT or
physical mobile devices. Fix only defects in the approved G14c/G14d contract;
a new format returns to planning.

**Dependencies.** G14c and G14d.

**Working state.** Every full-scale workflow meets the recorded correctness,
time, memory, file-shape, interruption, and compatibility gates. The chosen
snapshot format/default is documented consistently and G15 becomes the next
approval-gated item.

**Validation and evidence.** Privacy-validated aggregate JSON; exact canonical
and attachment fingerprints; current/previous archive matrix; full first and
unchanged runs; restic/borg integrity checks; REST/directory catch-up; full
repository validation; verified release ZIP copied to the evidence directory.

**Open decisions**

- None. A failed gate reopens its owning item; it is not waived in the release
  wrap-up.

**Outcome (2026-08-23).** Production G14c/G14d code passed all 19 resumable,
privacy-sanitized full-scale phases and the frozen Restic/Borg integrity rows.
The final REST/directory catch-up restored and exactly matched 382,209 generated
private-workspace documents after post-snapshot replay in 2,948.669 seconds at
210,010,112 bytes peak RSS. Compatible whole-library backup/catch-up is frozen
as `sqlite-image+packed-assets.v1` through deterministic sequential USTAR and
NBK1; semantic archive-v2 remains the portable subset/merge/fork/interchange/
fallback contract. Four G14d scale defects were fixed without a new format:
operation-specific snapshot timeout, 16 MiB signed range ceiling, scoped
reconciliation for body-only replay, and linear per-process directory-prefix
resume. Aggregate evidence is under `performance/v0.7-g14e/`; the completion
archive is `plans/v0.7/023-full-scale-archive-catchup-acceptance.md`.

## G15. Durable sync jobs, scheduling boundaries, retries, and MCP control — complete

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

**Outcome (2026-08-24).** Schema v26 extends the F6 job record with a closed,
transactional sync outbox and content-free audit trail. Explicit CLI, local
REST, or MCP control queues work; the daemon only drains queued/due rows and
never creates a cadence. Atomic per-target leases, durable phase checkpoints,
heartbeats, safe cancellation, byte allowances, stale-worker recovery, and
deterministic exponential jittered retry passed restart/offline/quota/two-target
fixtures. The non-blocking permission default is `mcp.sync_scope=disabled`;
`status` exposes bounded status/conflicts and `control` adds only path-free
incremental/resource operations plus retry/cancel of the MCP actor's own jobs.
Catch-up/restore-prep records remain local-only; keys, backup/restore,
enrollment, retirement, purge, reset, and bulk bytes remain outside MCP. The
implementation and validation evidence are archived at
`plans/v0.7/024-durable-sync-jobs.md`.

## G16. Sync, pairing, catch-up, encrypted-backup, and conflict UI — complete

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

**Outcome (2026-08-24).** The responsive loopback/native Sync Center now covers
active-profile identity, none/directory/REST setup with a native-only chooser,
explicit pairing and separate snapshot permission, durable operational state,
lazy-resource intent, three-way conflict resolution, repair visibility,
verified catch-up staging, reset review, and NPB1 password backups. The warned
owner-only provider remains injectable; passwords are cleared and never stored;
restore remains non-applying. A real two-daemon flow and desktop/mobile
Playwright sweep passed. No schema or mobile-build claim was added. Archived as
`plans/v0.7/025-sync-recovery-ui.md` with evidence under
`performance/v0.7-g16/`.

## G17. Peer retirement, retention horizon, tombstone/resource GC, and repair — complete

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

**Outcome (2026-08-24).** Schema v27 now keeps signed retirement decisions,
verified-snapshot vectors, monotonic collection floors, retained revision
orders, and compact death-certificate baselines. Operation/tombstone and
resource collection require age plus a currently re-verified physical snapshot
plus every active peer acknowledgement; stale digests are refused and below-
floor peers receive an explicit non-automatic snapshot catch-up requirement.
The Sync Center provides path-free retention and retirement review; destructive
retention remains local CLI-only with exact digest confirmation and immediate
snapshot re-verification. Full-corpus-scale evidence keeps the resolved 90-day
default. Archived as `plans/v0.7/026-peer-retention-gc-repair.md` with evidence
under `performance/v0.7-g17/`.

## G17a. Investigation — evidence provenance, sealing, and optical reserve contract — complete

**Goal.** Turn the accumulated release ZIPs and screenshots into a finite,
truthfully described preservation set before any repository push, without
retroactively claiming contemporaneous authorship or completion times that were
never recorded cryptographically.

**Scope.** Freeze a read-only inventory of every top-level regular file present
in `/home/renes/evidence/notrios` at the investigation boundary, including
ZIPs, PNGs, and any legacy or plan-only bundle; separately inspect directory
scope without publishing private recursive contents; record counts, byte totals, file type,
ZIP structural validation, filesystem modification time as weak source metadata,
and the strongest repository commit/task association that can be proved from
archive contents, Git history, archived plans, and attempt logs. An unknown or
ambiguous association stays unknown or lists candidates. Never infer a commit
from a filename alone.

Define a versioned canonical evidence schema and threat model that keeps four
claims separate:

1. SHA-256 and archive validation detect byte changes relative to a recorded
   digest;
2. an OpenPGP detached signature proves that the selected key signed exact
   bytes, while identity depends on the separately documented key-trust process;
3. a verified RFC 3161 response proves that the timestamped datum existed no
   later than the token time under the selected TSA policy and trust chain; and
4. inventory, transfer, ISO creation, storage, and later burn/read-back records
   document custody but do not by themselves decide legal admissibility.

Choose the exact signed and timestamped datum. The selected scalable construction
is a detached signature over each original artifact, hashes of every artifact
and signature in the canonical manifest, a detached signature over its content
checkpoint, and one RFC 3161 token over that checkpoint signature. The token
therefore commits the complete batch of exact artifact/signature bytes by the
TSA time. Preserve the request, response, signer public
key/fingerprint, TSA leaf/intermediate/root certificates as licensing and
redistribution allow, policy OID, verification command, tool versions, and
retrieval facts. Specify explicit `captured_at`, independently sourced
`step_completed_at`, `sealed_at`, and `retroactive` fields; backfilled artifacts
always use `retroactive: true`, and a 2026 seal must never be described as a
2025 or earlier completion timestamp.

Design an append-only canonical JSONL content manifest whose entry digest chains
canonical entry bytes, not just artifact ZIP hashes. It must use repository-
relative logical names rather than machine-specific absolute paths, record every
artifact/signature/request/response/support-file digest and size, represent
unknown values explicitly, and support signed checkpoints. Design a two-level
ISO catalog: the content manifest included inside an image covers every payload
byte, while a checked-in outer catalog records the completed ISO's SHA-256 and
its content-manifest/checkpoint digest. An ISO cannot contain its own final hash.

Measure the current set against a conservative CD-R volume budget using the
actual ISO builder's print-size result; define stable sorting, volume IDs,
timestamps, permissions, Rock Ridge/Joliet compatibility, tool/version capture,
and a deterministic rebuild test. Test the contract only with generated files
under `/tmp`. Specify volume rollover and immutable checkpoint rules: an issued
ISO is never silently regenerated; later evidence goes into the next numbered
volume even when an earlier image has free space. Define a restore/verifier and
future burn/read-back runbook, including full-image and per-file verification,
but do not burn media.

**Boundaries.** Investigation, generated prototypes, and aggregate inventory
only. Do not modify the evidence directory, create the production ISO, contact
a TSA, create/use a production secret key, push Git, burn optical media, claim WORM storage,
or offer legal advice. Git history is useful corroboration but is not an
immutable public ledger. The ISO is a preservation container, not automatic
proof of authorship, independent creation, custody, or admissibility.

**Dependencies.** G17 completion; existing release-packaging script and archives;
Git/attempt/plan history; locally available OpenSSL, GnuPG, and xorriso-compatible
tools.

**Working state.** `performance/v0.7-g17a/` contains only privacy-safe aggregate
inventory, generated prototype evidence, the schema/claim vocabulary, provider
and trust-chain assessment, deterministic ISO/capacity findings, commit-mapping
rules, and one recommended implementation contract. `EVIDENCE_PRESERVATION.md`
is the human-readable verification and custody specification. No digest of an
artifact absent from the frozen set is presented as if it had been observed.

**Validation and evidence.** Generated good/tampered/swapped/truncated artifact,
signature, timestamp, chain, and manifest fixtures; explicit OpenSSL verification
with the selected CA file and any required untrusted intermediates; detached
signature verification with the data filename supplied; canonical JSONL and
entry-chain mutation tests; ISO clean rebuild byte comparison, print-size/capacity
gate, extraction/read-back, long-name/Unicode/permission checks; corrupt/missing/
extra source refusal; privacy scan; and no network or evidence-directory write.

**Resolved decisions (2026-08-25)**

- **OpenPGP identity — resolved.** Use the dedicated existing UID `Rene Sugar
  (Evidence Identity) <rene.sugar@gmail.com>`, primary fingerprint
  `AEE5F82F2C216D6D15992C8DC96A1C6039BC8098`, and exact Ed25519 signing-subkey
  fingerprint `4ABEB98AF99C8321931BCF282C6A8A4568264005`. A read-only colon listing
  shows the certification primary secret offline (`sec#`) and the signing
  subkey usable (`ssb`); the subkey expires `2027-08-25T18:32:16Z`. Production
  commands must force the subkey by full fingerprint plus `!`, never select by
  email or short key ID. The user attests that the operational secret-subkey
  export and passphrase are stored as separate Secret Service items selected by
  `service=gpg_evidence,type=subkey_secret|passphrase`, and that a clearsign
  test succeeded. This decision update did not retrieve either secret. G17a's
  statement that no production key existed during its capture remains historical.
- **RFC 3161 authority/policy — resolved provider order and acceptance rule.**
  Use DigiCert `http://timestamp.digicert.com` first and Sectigo
  `http://timestamp.sectigo.com` as fallback. Accept no policy or certificate
  merely because it came from that host: an authorized generated pilot must
  capture the actual policy OID and responder chain, match nonce and SHA-256
  imprint, require timestamping EKU and validity at `genTime`, validate through
  explicit pinned CA/intermediate inputs, and preserve the verified response.
  A passing DigiCert pilot fixes its observed policy/chain for production; if it
  fails, apply the same gate to Sectigo. If neither passes, stop and ask rather
  than silently waive timestamping. This plan update contacted no TSA endpoint.
- **Filesystem scope — resolved as option A.** Preserve only curated top-level
  handoff ZIPs/screenshots present at the G17b freeze, including the G17a ZIP and
  any verified decision-update ZIP created before G17b. Exclude all three
  recursive G14 benchmark workspaces. Any other drift is refused until named
  and approved; workspace preservation remains separate privacy-reviewed work.

**Reference assessment (2026-08-25).** The supplied descriptions are broadly
accurate, with scope qualifications:

- Notary Project's Apache-2.0 `tspclient-go` implements RFC 3161/RFC 5816
  request, transport, response, and verification support
  (<https://github.com/notaryproject/tspclient-go>). It is a compatible future
  implementation candidate, not an approved new dependency for G17b; the
  selected baseline remains the already exercised GnuPG/OpenSSL subprocess
  boundary unless a focused dependency/pinning review justifies a change.
- Trail of Bits' Apache-2.0 `rfc3161-client` is a Python API backed by Rust/PyO3
  for constructing and verifying RFC 3161 objects; network transport is separate
  (<https://pypi.org/project/rfc3161-client/>). Older releases through 1.0.2
  were yanked for CVE-2025-52556. Do not add or pin it implicitly merely because
  it is referenced.
- Notary Project Notation is an OCI/blob signature CLI and now supports RFC 3161
  timestamps (<https://github.com/notaryproject/notation>). Its signature and
  trust-policy model is not the selected detached-OpenPGP evidence format, so it
  is not a drop-in verifier for this contract and is not selected for G17b.

**Outcome (2026-08-24).** G17a froze 78 curated top-level artifacts totaling
270,506,844 bytes: 74 CRC/path-valid ZIPs and four chunk-CRC-valid PNGs. All 73
current-shape release ZIPs map uniquely to 73 distinct commits through six
embedded source anchors; legacy `notrios.zip` remains provenance-unknown. The
curated sources print as a 270,962,688-byte ISO, 39.75% of the conservative
650 MiB project budget. Generated-only fixtures proved canonical entry-chain
mutation refusal, detached-signature verification, RFC 3161 nonce/imprint/
policy/explicit-chain verification and wrong-data/wrong-CA refusal, and two
byte-identical xorriso builds with full extraction hashes. The selected default
is per-artifact signatures bound by one signed/timestamped batch checkpoint,
not one TSA call per artifact. `EVIDENCE_PRESERVATION.md` records the contract;
aggregate evidence is under `performance/v0.7-g17a/`; archived as
`plans/v0.7/028-evidence-preservation-contract.md`. No production artifact,
evidence/reserve directory, signing identity, TSA, ISO, remote, or physical
medium changed; the generated one-day key and local fixture TSA existed only in
the disposable `/tmp` self-test.

## G17b. Backfill evidence seals, checked-in manifest, and immutable ISO reserve — complete

**Goal.** Apply G17a's approved contract to every frozen historical artifact,
store verifiable manifests and tooling in Git, reserve burn-ready ISO images on
`/media/renes/SEAGATE2TB`, and make complete coverage a mandatory gate before
the next GitHub push.

**Resolved inputs (2026-08-25).** The source is curated top-level handoffs at
the G17b freeze, including verified G17a and pre-G17b decision ZIPs; all three
recursive G14 workspaces are excluded. The signer is exact Ed25519 subkey
`4ABEB98AF99C8321931BCF282C6A8A4568264005` under primary
`AEE5F82F2C216D6D15992C8DC96A1C6039BC8098`; automated GnuPG selection appends
`!`. DigiCert `http://timestamp.digicert.com` is the primary RFC 3161 endpoint
and Sectigo `http://timestamp.sectigo.com` is fallback. The generated pilot,
not the hostname alone, fixes the accepted policy OID and responder chain. No
new RFC 3161/Notation dependency is selected.

**Scope.** Re-freeze the source inventory, reconcile only the named verified
post-G17a appends, and refuse all other drift. Validate every ZIP with the
repository release checker when compatible
and with complete ZIP CRC/path checks otherwise; validate image decodability;
hash original bytes without rewriting metadata. Resolve task/commit provenance
only from reproducible evidence and record uncertainty. Generate each artifact's
detached OpenPGP signature and verification record. Hash every artifact and
signature into the canonical manifest, sign the content checkpoint with the
exact resolved subkey, and obtain one RFC 3161 request/response over that
checkpoint signature using the first selected provider whose generated pilot
passes the strict acceptance gate. Optional per-artifact timestamp tokens
require a separate standalone-extraction profile and are not the default. Keep the private key
outside the repository, logs, ISO, and process arguments.

Add the versioned manifest schema, canonicalizer, append/checkpoint tool,
read-only verifier, committed public key, permitted TSA trust/policy material,
custody template, burn/read-back runbook, and tests under a clearly documented
repository evidence directory. Check in the canonical historical manifest and
signed content checkpoint, including byte/size/hash coverage of signatures,
requests, responses, and support material. Never check in the release ZIPs,
screenshots, ISO bytes, a secret key, passphrase, private pathname, or a token
that has not independently verified.

Build numbered deterministic ISO 9660 images from private staging directly on
`/media/renes/SEAGATE2TB/notrios-evidence/`, partitioned below G17a's measured
CD-R budget. Each image contains original artifacts, their detached signatures
and timestamp material, the content manifest/checkpoint, public verification
materials, verifier source, and plain-text restore/custody instructions. Write
an adjacent external checksum/signature/timestamp set for each ISO. After an
image is byte-final, append its SHA-256, byte size, volume ID, content-checkpoint
digest, creation tool/options/version, and external reserve location identifier
to the checked-in outer ISO catalog; do not attempt a self-hash inside the ISO.
Commit the content checkpoint before imaging and the outer catalog afterward,
then verify both commits remain ancestors of the final pre-push HEAD. Define the
catalog-only closure commit as an explicit trust boundary: it does not trigger a
new release ZIP of itself, because requiring that ZIP to appear in the catalog
would recurse forever. The next ordinary content checkpoint covers the prior
closure commit. The verifier reports this one expected outer boundary instead
of calling the current ISO self-contained with respect to its own catalog.

Add a host-side pre-push evidence command that fails closed on an unmanifested
artifact, source drift, invalid entry chain/checkpoint, missing or invalid
artifact signature or checkpoint timestamp, unassigned artifact, absent external ISO, ISO hash/volume
mismatch, failed ISO extraction/content verification, or catalog commit not in
HEAD. CI validates schemas, canonicalization, fixtures, and tracked support
files without pretending it can see the external reserve. Document that the
current `develop` branch has no upstream and that no remote write is part of
this item.

Amend the per-item release workflow so each later ZIP is sealed immediately
after verification, with `retroactive: false` only when capture is genuinely
contemporaneous. Create a new immutable ISO volume checkpoint before each future
GitHub push and at milestone close; do not rebuild historical volume IDs.
Physical CD-R burning remains a later owner-authorized custody operation from
these exact ISO bytes. The runbook uses a drive/media-supported speed rather
than assuming 4× is universally safer, closes the session where supported, and
requires full read-back plus per-file verification for every copy.

**Boundaries.** No GitHub push, tag, release, remote upload, or physical-media
write. No fabricated old signature/time/custodian event and no claim that
hashes, signatures, timestamps, Git, or an ISO alone prove authorship,
independent creation, clean-room status, or admissibility. Do not mutate the
original artifact bytes. Do not depend on a GUI verifier, mounted ISO, network,
secret key, or the original repository for ordinary offline verification.

**Dependencies.** Completed and user-approved G17a contract and the three
resolved design decisions above; explicit permission for external evidence/
reserve writes, exact signing-subkey use and Secret Service passphrase retrieval,
and network requests to the selected TSA endpoints. Before the first production
signature, the owner must attest that a restorable full primary-key backup and
revocation certificate exist offline; do not inspect or record secret locations.

**Working state.** Every frozen evidence file has an exact checked-in manifest
entry, validated signature, signed/timestamped checkpoint coverage, and immutable volume assignment;
every numbered ISO and adjacent verification set exists on the designated
SEAGATE reserve and passes a clean offline restore. The pre-push command reports
complete coverage. G18 remains unstarted and no remote state has changed.

**Validation and evidence.** Full source-to-manifest-to-signature-to-RFC3161-to-
ISO-to-outer-catalog verification in a clean temporary environment; independent
recomputation of hashes and entry chain; key fingerprint/trust-material pinning;
TSA nonce, imprint, time, policy, signer purpose, and chain validation; bad-key,
bad-CA, wrong-data, stale-query, tamper, missing/extra/swap, unavailable-reserve,
and catalog/HEAD mutation tests; two deterministic builds with identical inputs/
toolchain; ISO size budget and no-overflow proof; extraction and full hash walk;
repository secret/private-path/privacy scan; standard repository validation;
verified release ZIP built from G17b's content checkpoint and sealed into the
first applicable volume; one documented catalog-only closure commit afterward,
with no recursive release-ZIP requirement.

**Open decisions**

- **May G17b perform its operational actions? — Resolved 2026-08-25.** The user
  explicitly authorized use of signing subkey
  `4ABEB98AF99C8321931BCF282C6A8A4568264005`, non-logging Secret Service
  passphrase retrieval, generated and production RFC 3161 requests to the
  selected DigiCert/Sectigo endpoints under the acceptance gate above, and
  writes below `/media/renes/SEAGATE2TB/notrios-evidence/`. It still does not
  authorize a GitHub push or physical burn.
- **Is offline recovery/revocation ready? — Resolved 2026-08-25.** The owner
  attested that the full primary-key backup and revocation certificate are
  restorable offline. Their paths, media identifiers, and secret bytes are not
  recorded.
- **Unexpected TSA pilot result — Resolved for this run.** The generated DigiCert
  pilot passed the nonce, SHA-256 imprint, policy `2.16.840.1.114412.7.1`,
  critical timestamping EKU, `genTime` validity, and explicit pinned-chain gates.
  Sectigo was not contacted. Local wall-clock time and implicit CA stores remain
  forbidden fallbacks.

**Outcome (2026-08-25).** The authorized production run froze 81 curated
top-level artifacts (284,012,518 original bytes), preserving the 78-file G17a
base plus the verified G17a, decision, and G17b release ZIP appends while
excluding all three recursive private workspaces. It created 81 exact-subkey
detached signatures and a 162-entry canonical chain; 76 ZIPs map uniquely to
Git commits, four screenshots are not applicable, and legacy `notrios.zip`
remains honestly unresolved. Content commit
`538b74d9976522f01dea54dfe9dd5d1b38055ca0` anchors manifest SHA-256
`683c10aaf2ee114306c799d431d3f33e202d77356d47ee0e237f56a5ab505bac`
and checkpoint SHA-256
`ed88bbdf9a5875d0a86547f5560101e99cbc9c8256764f399fa785286895a298`.
DigiCert policy `2.16.840.1.114412.7.1` passed the generated pilot and every
production verification with the explicit pinned root/responder; Sectigo was
not used. Two clean 139,127-block builds were byte-identical. The issued
284,932,096-byte `NTR-EV-0001` has SHA-256
`f4df1e047e3f372efdf5ab3d3a89089f2413c1e91243afaff01012054161258f`,
its adjacent signature/timestamp/checksum set is on the designated reserve,
and extracted offline verification covers all 179 staged files. The corrected
signed/timestamped outer catalog has SHA-256
`b47f9a7d1879459ee7b0c269aafe852c577e514fadf3369ba6137f5b94a0b1c0`;
the verified first catalog seal was preserved as superseded after its
ISO-signature `genTime` field was found mislabeled as ISO creation time. The
14-case refusal matrix passes, and the host-side pre-push gate verifies source,
tracked checkpoint, reserve, and commit ancestry. The catalog-only closure does
not recursively require another release ZIP. No original artifact, secret,
remote, GitHub branch, or physical medium was modified. Archive:
`plans/v0.7/030-evidence-seals-iso-reserve.md`.

## G18. Shared-core, FFI, Mermaid, and installation/mobile portability handoff — complete

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

**v0.8 Android SQLite investigation input (added 2026-08-26).** This follow-up
does not reopen G18 or authorize a dependency/Android build. It corrects the
premise for v0.8 H0: Android framework SQLite is a Java/Kotlin API, not a public
NDK `sqlite3.h`/`libsqlite3` C development contract. Jetpack's
`BundledSQLiteDriver` embeds SQLite for Kotlin/Room but does not satisfy the
current Go cgo adapter. Notrios canonical persistence is owned by the Go core
across 43 C-importing store files and requires FULLMUTEX/WAL, FTS5, and JSON.
H0's recommended investigation default is therefore one checksum-pinned
upstream SQLite amalgamation compiled into the Go shared library.

**Open decisions for v0.8 H0.** These do not block completed G18, but all are
blocking for H1's Android build. H0 must measure and recommend before H1 starts.

- **Database owner/package.** Choose the pinned amalgamation in the Go core
  (recommended because it preserves one transport-neutral canonical store), or
  explicitly approve a Kotlin/Room ownership redesign and new bridge. Android
  framework SQLite and an implicit dual-engine file share are not candidates.
- **Version and features.** Pin exact upstream source/checksum, security-update
  cadence, and compile options. FTS5 and JSON probes are required; any optional
  extension must be justified by source use and size/security evidence.
- **Platform contract.** Select minSdk and packaged ABIs, including the actual
  emulator ABI. Measure per-ABI size/RSS and reject an architecture inferred
  only from the host or an unconfigured emulator.
- **Native symbols/concurrency.** Freeze static/shared linkage, symbol
  visibility or prefixing, one-SQLite-engine-per-process, FULLMUTEX plus Go
  mutex behavior, and refusal of concurrent canonical-file ownership.
- **Storage/compatibility.** Freeze sandbox database/assets and backup/no-backup
  locations, WAL checkpoint/crash behavior, and desktop/Android database image
  compatibility. Round-trip a checkpointed file in both directions and run
  integrity, snapshot/restore, CRUD/search, and sync tests.

The same follow-up confirms the current editor behavior: md-editor-rt 6.5.3 and
CodeMirror search 6.7.1 expose case, regexp, by-word, navigation, single replace,
and replace-all controls in the Ctrl/Cmd+F panel. Rendered desktop/narrow QA and
persistence pass without console warnings. Ctrl/Cmd+H is not a default binding.

**Outcome (2026-08-25).** G18 froze a machine-checked 19-capability platform/
permission matrix and audited all 109 normalized non-HEAD API operations. The
current reusable seams are `store.Store`, injected secret/sidecar providers,
contexts, jobs, and bounded readers; the audit truthfully finds that
`internal/service` still owns HTTP and handler orchestration is not yet a
framework-neutral facade. The ABI-major-1 proposal has 12 version/lifecycle/
dispatch/cancel/poll/event/stream/release symbols, generation-bearing opaque
64-bit handles, a closed typed-error set, caller-borrowed inputs, explicitly
released immutable outputs, 1 MiB JSON/stream-read bounds, and no callbacks
from arbitrary Go threads. HTTP status/ETag/Range/cancellation translate to
typed status/revision/stream/call fields while TLS, CSP, peer auth/rate limits,
and web assets stay adapters. Linux `c-shared`/`c-archive` feasibility probes
built against host SQLite but exported no ABI/header; Android/arm64 reached the
installed NDK compiler and failed honestly at the missing Android-target
`sqlite3.h`. Debian's host header/x86-64 library are present but forcing the
host include root produces incompatible glibc/Android headers. A 2026-08-26
follow-up records that Flutter Doctor now passes every check after Clang PATH
correction; Swiftly has no selected toolchain, and there is still no AVD,
connected Android device, or Flutter/Android build claim. It also records the
Android SQLite H0 decisions and rendered editor-search result above. Mermaid
remains disabled; the v0.8 gate now
has exact source/node/edge/label/time/heap bounds plus browser/Wails/offline/
CSP/sanitization/accessibility fixtures and safe source fallback. No production
code, schema, dependency, installer, ABI, Flutter artifact, Mermaid setting,
remote, or physical device changed. Archive:
`plans/v0.7/031-shared-core-ffi-portability-handoff.md`.

## G18a. Investigation — documentation anchors, truth grades, and review calibration

**Goal.** Adapt the proven Borge doc-anchor method to Notrios before moving
prose or adding a generator, so source-adjacent documentation becomes
auditable without pretending that all prose can be proved by a test.

**Scope.** Inventory the 15 published/Help-notebook Markdown pages, CLI help,
configuration reference, OpenAPI operations, MCP tools/resources, and GUI user
journeys. Map each actionable claim to the declaration that implements it and,
where the behavior is not local, its direct same-package callees. Verify
Go `doc/comment` directive behavior against the project toolchain and define the
Notrios directive grammar corresponding to `doc`, `help`, `enumerates`, and
`claim`. Determine the source-symbol anchoring rule for Go and TS/TSX without
accepting a free-form path/line registry that can silently point nowhere.

Define four honest grades per topic: **executed** (an example or journey is run
and its result checked), **generated** (a finite list comes from the same
registry as code), **claimed** (prose names an executable behavioral check),
and **unverified** (permitted but counted). Separate user, API, and maintainer
audiences; rationale stays unmarked. Build a small labelled contradiction
calibration set from real history or controlled mutations before selecting any
semantic checker. The known current drift where `docs/service.md` names schema
v20 while the canonical store is v27 belongs in the baseline rather than being
silently corrected before the audit can observe it.

**Boundaries.** Investigation and fixtures only. Do not move the manual, emit
generated docs, call an external model, or make a non-deterministic result fail
CI. Semantic similarity/cosine distance is explicitly not an agreement test:
negated false prose can remain highly similar to the correct explanation.

**Dependencies.** Existing docs/Help pipeline, OpenAPI/MCP parity checks, G16
GUI, and the current Go/TypeScript build toolchains.

**Working state.** `performance/v0.7-g18a/` contains the inventory, proposed
directive grammar, source-symbol rules, per-topic grade baseline, labelled
supported/contradicted/not-determinable cases, and a recommendation that makes
G18c-G18f finite.

**Validation and evidence.** Parser fixtures for ordinary/directive comments,
duplicate/dangling symbols, mixed audiences, direct-callee scope, and TS/TSX
anchors; grade totals reconcile to every topic/section; calibration includes
negation and rationale; no user data, private note content, or generated model
claim is committed.

**Open decisions**

- None. This slice exists to measure the uncertain anchoring and calibration
  premises. Any materially different viable approaches become separate plan
  items rather than a fork inside G18c.

## G18b. Investigation — Hugo/Ledger migration and reproducible site contract

**Goal.** Prove how the existing static documentation and offline Help source
can use `hugo-theme-ledger` without losing stable URLs, Pagefind search,
offline assets, or reproducible release packaging.

**Scope.** Evaluate the local Apache-2.0 theme copy at
`/home/renes/projects/hugo-theme-ledger` (clean `develop` at observed commit
`f9d28ea297427890ecffa31fa74caa9ee385d9f5`) against Notrios' 15-page hierarchy,
GitHub Pages `/notrios/` base path, raw Markdown Help seeding, internal anchors,
Pagefind indexing, light/dark/high-contrast behavior, and current CI. Measure a
prototype with Hugo Extended 0.146 or newer (the local environment currently
has 0.164.0), `googleFonts=false`, the static Pagefind backend, and no Bluge
server. Compare vendored pinned theme source, a Git submodule, and a network-
resolved Hugo module for clean clone, source ZIP, license/provenance, update,
and offline-build behavior. Inventory URL and content-adapter changes rather
than assuming a notes-oriented theme is already a documentation information
architecture.

**Boundaries.** No production site/workflow switch and no changes to the Help
notebook. Do not copy `node_modules`, the reference search-server binary, or a
floating theme checkout. This task does not change the separate Notrios note-
publication/Hugo pipeline or select Bluge for the small documentation site.

**Dependencies.** Current `scripts/build_docs_site.sh`, `.github/workflows/docs.yml`,
`DOCS_SITE.md`, release-ZIP rules, and the theme's README/AGENTS/performance
constraints.

**Working state.** `performance/v0.7-g18b/` contains a pinned prototype,
route/link/search diff, asset/request inventory, license bill, build-time/size
baseline, Help-seed byte comparison, and a single recommended integration for
G18g.

**Validation and evidence.** Clean local and CI-shaped builds; all current
public page/fragment links resolve or have explicit redirects; Pagefind finds a
known term and excludes site furniture; no third-party runtime request; the
source ZIP contains everything needed except documented build tools; keyboard,
mobile, and contrast smoke designs are executable.

**Open decisions**

- **How is the theme pinned? — Non-blocking.** Options are a vendored source
  snapshot, Git submodule, or Hugo module. Default/recommendation: vendor the
  minimal upstream source plus LICENSE and commit provenance. It keeps release
  ZIPs and offline/clean builds self-contained; approving G18b approves this
  default unless the evidence shows an unacceptable maintenance cost.
- **Does the public URL shape change? — Non-blocking.** Default: preserve the
  existing `.html` and fragment URLs, adding generated redirects only where
  Hugo cannot emit an exact equivalent. A visual theme migration does not
  justify breaking saved documentation links.
- **Which Ledger search backend is used? — Non-blocking.** Default: Pagefind.
  The documentation corpus is small and static; adding the Bluge service would
  create deployment and operations scope with no measured benefit.

## G18c. Documentation anchor audit and executable-claim registry

**Goal.** Make documentation drift visible before generating or relocating
large amounts of prose.

**Scope.** Implement a repository-only `docaudit` tool/library using G18a's
selected Go and TS/TSX source-symbol rules. Recognize the frozen
`notrios:doc`, `notrios:help`, `notrios:enumerates`, and `notrios:claim`
directives; bind fragments to existing Markdown page/section templates; and
maintain a typed registry from claim IDs to tests. Report executed, generated,
claimed, and unverified counts per topic. Fail deterministic checks on unknown
audiences, duplicate fragment/claim IDs, missing topics or declarations,
dangling claims, orphan registered checks, mixed user/API blocks, or an
unaccounted executable example/journey.

Start with the highest-risk finite surfaces: product/schema/version claims,
configuration keys/defaults, CLI command/flag lists, REST operation IDs, MCP
tool/resource names/scopes, destructive confirmations, and GUI controls that
authorize destructive or recovery actions. Existing prose remains authoritative
until a later generation slice moves a fragment.

**Boundaries.** Read-only audit and a small first anchor set. No bulk comment
migration, rendered-doc generation, semantic/model verdict, or behavior change.
An anchor does not prove truth by proximity; only its grade states what backs it.

**Dependencies.** G18a.

**Working state.** One deterministic command and one test expose the coverage
and every broken edge in the documentation/source/claim graph. The initial
unverified share is recorded, not hidden behind a pass/fail total.

**Validation and evidence.** Golden and mutation fixtures for every dangling,
duplicate, orphan, audience, and source-symbol case; the audit finds the G18a
known-drift fixture before its correction and stays green after the generated
or claimed source replaces it; deletion of either an anchor or registered
check fails in the expected direction.

**Open decisions**

- None. G18a freezes the grammar and symbol mechanism before this item may be
  approved.

## G18d. Executed CLI, configuration, REST, and MCP documentation examples

**Goal.** Ensure copyable non-GUI examples both run and do the specific thing
the surrounding prose promises.

**Scope.** Extract every explicit `notriosctl`, `notriosd`, configuration,
`curl`/REST, and MCP request example from published/Help documentation into a
bidirectional manifest. Each entry names its document fragment, fixture,
substitutions, expected exit/HTTP/tool status, and a semantic postcondition:
created rows, selected notes, archive contents, unchanged state for a dry run,
redaction, or typed refusal. Build isolated scratch profiles/databases and a
loopback service/MCP harness with real non-vacuous data. Destructive examples
run only inside their fresh fixture or carry an explicit, reviewed unrun reason.
Configuration fragments must parse and demonstrate precedence/default effects;
OpenAPI/MCP request and response examples must also validate against their
published schemas.

**Boundaries.** No private corpus, user path, network service, or shared test
state. Exit code/HTTP status alone is not a sufficient assertion. Do not make a
dangerous example harmless by testing a different command than users copy.
When an example exposes a product defect, first pin the contradiction and then
fix only behavior already required by an approved contract; otherwise record a
new plan item rather than silently changing semantics.

**Dependencies.** G18c and existing CLI/REST/MCP test seams.

**Working state.** Every executable non-GUI example has a result-bearing test
or an explicit reason it cannot run, in both directions: no undocumented test
entry and no untracked example. Every documentation topic that teaches a
command/API action carries at least one executed example.

**Validation and evidence.** Mutation checks break an example and separately
break its implementation; both fail. Fixtures include empty/absent input,
options before/after positionals where supported, no-match read versus write,
dry-run/apply parity, config precedence, REST confirmation/redaction, MCP scope,
and rollback/cleanup. Report per-topic executed coverage and runtime.

**Open decisions**

- None. Examples unsafe to execute are visible manifest entries with reasons,
  not a decision to omit them silently.

## G18e. Executed GUI user journeys and action-length baseline

**Goal.** Test the menu clicks, buttons, checkboxes, fields, dialogs, and
keyboard actions that the GUI documentation tells a user to perform, while
measuring—not guessing—the interaction cost of each task.

**Scope.** Turn each documented GUI procedure into a structured journey linked
through G18a's selected source-symbol mechanism to its React component/action
handler and any owning Go handler. Run the journeys in a deterministic seeded
profile through the browser harness at desktop and narrow/mobile viewports.
Assert the visible result and canonical/API postcondition for create/edit/move/
trash/restore, import/export handoff, settings/theme, links, resources, Sync
Center pairing/job/conflict/backup/retirement review, and every documented
destructive confirmation. Record click/keypress count, typed-field count,
branch/decision count, modal depth, and recovery steps per user goal.

**Boundaries.** Journey length is an ease-of-use signal, not a universal score
or automatic redesign trigger. Setup/teardown and accessibility navigation are
reported separately from the user's task. No screenshot-only assertion,
sleep-based success, automatic destructive restore, real peer/cloud account,
or unsupported mobile claim.

**Dependencies.** G18a, G18c, G16, and the existing browser test stack.

**Working state.** Every GUI procedure is executed or explicitly marked with a
reason and owner; its source anchor and behavioral postcondition are auditable.
The baseline identifies unusually long or branching flows for human review
without changing product scope inside a documentation task.

**Validation and evidence.** Browser-plugin-first policy with recorded
Playwright fallback; desktop/mobile layout, keyboard/Escape/focus, 44 px mobile
targets, console/CSP, and no-vacuity checks; mutation of a control label/action
or expected postcondition fails; step-count report is stable after excluding
fixture mechanics.

**Open decisions**

- None. Any flow selected for redesign from the measured baseline becomes its
  own approved feature item.

## G18f. Generated documentation subsets, freshness, and advisory prose review

**Goal.** Make anchored user/API fragments and finite lists impossible to
silently drift, then use calibrated semantic review to reduce—not disguise—the
remaining unverified prose.

**Scope.** Implement `docgen --user` and `docgen --api` over explicit per-page/
section templates, never source-order concatenation. Generate finite lists
from the same registries used by configuration, CLI, REST/OpenAPI, MCP, and GUI
code. Keep user-facing and API paragraphs as separate anchored comment blocks;
maintainer rationale remains ordinary source commentary. Commit generated
Markdown fragments consumed by the site and Help seeder, and add a
`TestDocsAreCurrent` in-memory regeneration diff.

Add an advisory `doccheck` pass for `notrios:doc user` blocks only. First
explain the anchored declaration plus bounded direct same-package callees with
the prose withheld; then classify the independent explanation against the
claim as supported, contradicted, or not determinable. Run the G18a labelled
calibration set before accepting a report. Do not use embedding similarity as
the verdict. Separately, ask whether each topic is actionable by generating a
command, configuration, API request, or GUI journey from the prose alone and
running it through G18d/G18e's fixture. Emit a human/co-author triage report
containing the claim, anchor, blind explanation, verdict/attempt, and evidence
hash; never rewrite prose automatically.

**Boundaries.** Deterministic generation/freshness is a CI gate; model-based
contradiction/actionability is advisory and never blocks a build. No external
model receives source without separate approval, and no note/database content
is ever input. A model cannot register its own claim test or mark rationale as
proved. Cross-package behavior beyond the bounded source view remains not
determinable and should move to a better anchor or explicit claim test.

**Dependencies.** G18a, G18c, G18d, and G18e.

**Working state.** User and API subsets have a reviewable template-defined
shape; edits without regeneration fail; coverage reports show all four grades;
semantic and actionability reports surface calibrated disagreements without
making probabilistic output a release oracle.

**Validation and evidence.** Generated-output goldens and freshness mutation;
enumeration parity for config/CLI/REST/MCP/GUI; stable fragment ordering;
calibration confusion matrix with the negation cases separated; manual review
disposition for every contradicted verdict; repeated advisory runs record model,
prompt, source hash, variance, and cost without committing secrets.

**Open decisions**

- **Where may semantic review run? — Non-blocking.** Default: an explicit
  maintainer command with recorded inputs/outputs, never ordinary CI. A local
  model may run without network; any hosted-model source upload needs separate
  approval. Approving this item does not approve an external service,
  dependency, or recurring cost.

## G18g. Migrate the documentation site to pinned Hugo/Ledger

**Goal.** Replace the bespoke Marked template with the selected pinned
`hugo-theme-ledger` integration while retaining one Markdown source for the
public site and offline Help notebook.

**Scope.** Apply G18b's selected integration, including upstream commit and
Apache-2.0 provenance, a pinned Hugo Extended/Node/Pagefind build, Hugo config,
content adapter or staging step, Notrios navigation/information architecture,
and GitHub Pages workflow. Preserve current public routes/fragments or ship
tested redirects; preserve raw `docs/` Markdown and deterministic Help IDs.
Configure the static Pagefind backend, project base path, site-furniture
exclusion, newest/appropriate documentation ordering, and local fonts/system
fallback with `googleFonts=false`. Keep the theme's cached-sidebar, bounded
pagination, URL-helper, and search-index invariants intact; any Notrios override
is minimal, named, and browser-tested.

**Boundaries.** No Bluge service, remote font/CDN/runtime asset, generated user
note site, movenotes projection change, or theme `node_modules`/binary vendor.
Do not expose design/contributor documents through the user site or seed them
into Help. The local theme checkout is reference input, not an implicit runtime
dependency.

**Dependencies.** G18b and G18f.

**Working state.** `scripts/build_docs_site.sh` produces the complete Ledger-
themed `_site` reproducibly from a clean release ZIP; GitHub Pages uses the same
command; `seed-help` still mirrors the same user Markdown into protected Help.

**Validation and evidence.** Pinned clean build with no network after declared
tool installation; release-ZIP rebuild; internal/external-link and fragment
check; route/redirect manifest; Pagefind known-term, exclusion, and result-link
tests under `/notrios/`; zero third-party requests/CSP violations; semantic
heading/code/table rendering; desktop/mobile/keyboard/screen-reader/contrast
browser sweep; Help reseed idempotence and byte/content equivalence; build time
and output size recorded against G18b.

**Open decisions**

- None. G18b resolves the pinning, URL, and search choices before this
  implementation slice may be approved.

## G19. Archive-v2 compatibility bridge

**Goal.** Publish the external archive contract deferred from v0.4 only after
the sync-era container capabilities are stable.

**Scope.** Publish JSON Schemas, capability/version bounds, sanitized
deterministic loose/packed/sync-era golden fixtures, and a compatibility command.
Carry G14e's freeze explicitly: archive-v2 is the portable semantic consumer
contract, while `sqlite-image+packed-assets.v1` is a same-schema whole-library
capability that an incompatible or portable consumer must identify and refuse,
not reinterpret as archive-v3. Include a sanitized physical-manifest refusal
golden without publishing a private or full database image.
Coordinate `movenotes-v3/notrios2sql.py` against verified fixtures and add
cross-version consumer tests if that external importer now exists.

**Boundaries.** Gated on G9, not on transport completion. Do not claim an
external consumer that does not exist; a missing consumer yields producer-side
contract evidence, not invented coordination.

**Dependencies.** G9, G18a-G18g documentation integrity/site pipeline; archive
v2 P2–P4.

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

**Dependencies.** G3–G19 as applicable, including G18a-G18g.

**Working state.** Product/version/schema/docs agree; all supported peers
converge or report a typed recoverable state; source ZIP verifies and is copied
to the evidence directory; the active plan can be archived and the next plan is
created from the roadmap only after user review.

**Validation and evidence.** Full repository checks and smoke tests; doc-anchor
audit, generated-doc freshness, all executable CLI/config/API/GUI examples,
Hugo/Ledger clean build/search/link/accessibility checks, and disposition of
advisory contradiction/actionability reports; mutation
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
| Sync credential reach into ordinary REST | G13 | Resolved: none — implemented and asserted |
| Pairing bootstrap | G13 | Resolved: short-lived one-use bundle |
| Snapshot wrapper contract | G14/G14d/G14e | Resolved: deterministic sequential USTAR is transport only; the physical manifest/verifier is the trust boundary |
| Archive benchmark acceptance policy | G14a | Resolved: approved defaults exercised and retained for G14b |
| Physical full-snapshot representation | G14b/G14c | Resolved: option B, same-schema SQLite image + bounded packed assets with semantic archive-v2 retained |
| MCP sync controls | G15 | Resolved: bounded incremental controls only |
| v0.7 secret-store provider | G16 | Resolved: interface plus warned `0600` development provider |
| Remember backup password | G16 | Resolved: no |
| Retention horizon | G17 | Resolved: 90 days plus verified snapshot floor |
| Offline peer retirement | G17 | Resolved: no all-peers-online requirement |
| Evidence filesystem scope | G17a | Resolved 2026-08-25: curated top-level handoffs at G17b freeze; all recursive private workspaces excluded |
| Evidence OpenPGP signing identity | G17a | Resolved 2026-08-25: exact full primary/subkey fingerprints are in the owning item; abbreviated IDs must not select keys |
| Evidence RFC 3161 authority/policy | G17a | Resolved 2026-08-25: DigiCert primary, Sectigo fallback; accept only the explicit policy/chain that passes the generated pilot gate |
| G17b operational authorization | G17b | Resolved 2026-08-25: exact signer/credential use, selected TSA requests, and external reserve writes authorized; no push or burn |
| Evidence key offline recovery/revocation | G17b | Resolved 2026-08-25: owner attested restorable offline primary backup and revocation certificate; no secret paths/bytes recorded |
| Shared-core/FFI and Flutter boundary | G18 | Resolved: pre-1.0 ABI; post-1.0 client; no Web FFI |
| Current-GUI Mermaid baseline | G18 | Resolved fact: upstream-capable but disabled pending offline/security evidence |
| Cross-language documentation anchor/calibration mechanism | G18a | Investigation; blocking G18c-G18f until one finite mechanism and labelled set are selected |
| Ledger theme pin/distribution | G18b | Open, non-blocking default: minimal vendored source snapshot with license and upstream commit |
| Documentation public URL shape | G18b | Open, non-blocking default: preserve `.html`/fragment links or add tested redirects |
| Documentation search backend | G18b | Open, non-blocking default: static Pagefind; no Bluge service |
| Hosted semantic-review execution | G18f | Open, non-blocking default: maintainer-only recorded command; hosted source upload needs separate approval |
| Release version/schema bookkeeping | G20 | Open, non-blocking until wrap-up |

G0-G18 are complete and the production physical restore/catch-up, durable
sync-job, local recovery UI, safe-retention, and evidence-preservation design
contracts are frozen. The G17b host-side evidence gate is mandatory before any
future GitHub push. G18a-G18g are unapproved and G18a is next.
Implementation begins only after an explicit instruction naming
the item to start and, where stated, authorizing its blocking operations.
