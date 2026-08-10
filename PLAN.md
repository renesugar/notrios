# Plan: v0.7 — Native synchronization

Status: **replacement draft, written 2026-08-10. Product version is 0.6.0 and
the canonical schema is v18. No v0.7 implementation item is approved.** The
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

Installation, filesystem permissions, OS credential stores, Wails v3 migration,
and Android packaging form the distinct v0.8 milestone in `ROADMAP.md`. v0.7
must nevertheless keep transport, crypto, storage, and job interfaces
UI-framework independent and must record the constraints v0.8 has to validate.

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

## G0. Investigation — threat model, terminology, and reference validation

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

**Open decisions**

- **Must all sync payloads be end-to-end encrypted? — Blocking.** Options are
  mandatory above every carrier, mandatory only for directory transport, or
  optional everywhere. **Recommended: mandatory authenticated encryption for
  every v1 protocol payload, including REST, with an explicit local development
  test mode only.** One rule avoids a library being private over a directory but
  exposed after switching targets. TLS remains required for non-loopback REST
  because payload encryption does not hide endpoints, sizes, timing, or API
  credentials.
- **Are per-replica digital signatures required? — Blocking.** AEAD with one
  shared library key detects tampering but any holder can impersonate another
  holder. **Recommended: yes—each enrolled replica signs canonical envelope,
  advertisement, request, acknowledgement, and snapshot-manifest bytes with an
  Ed25519 identity key; encryption remains separate.** Signatures give device
  attribution, replay/revocation evidence, and safe untrusted-directory
  discovery. Content hashes still verify objects; do not sign every blob twice.
- **Is SVN dump compatibility a goal? — Non-blocking.** Default and
  recommendation: no. Reuse the state-vector/change-log and base-delta ideas,
  but extend archive v2 rather than adopting a foreign repository dump grammar.

## G1. Investigation — representative divergence and revision-delta workload

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

**Open decisions**

- **What is the canonical body operation? — Blocking, resolved by this
  investigation.** Options are complete snapshots only, deltas only, or a full
  result hash/body object with an optional delta from a named parent.
  **Recommended: the third.** A delta saves transfer when its base is present;
  the complete object breaks dependency chains and makes repair possible.
- **Which three-way merge granularity and library are accepted? — Blocking,
  resolved by evidence.** Prefer the smallest maintained Apache-2.0/MIT/BSD
  implementation that preserves UTF-8 and exposes conflicts. Do not adopt a
  CRDT merely because it merges character operations; ordinary offline sync is
  revision merge, not live co-editing.
- **What counts as representative offline intervals? — Non-blocking.** Default:
  one hour, one day, one week, and thirty days, with results reported
  separately.

## G2. Investigation — envelope, resource, and constrained-device bounds

**Goal.** Determine format, compression, pack, chunk, and pending-queue bounds
before they become an immutable protocol contract.

**Scope.** Reuse archive-v2 loose/packed evidence and run generated change sets
against the large recipe corpus and attachment-bearing Joplin corpus. Compare
deterministic JSON/JSONL and one compact candidate, deterministic compression,
fixed-size chunk thresholds, whole-resource fallback, range fetch, many-small
objects, and decompression limits. Use desktop CPU/RSS/disk profiles as a proxy
only; produce an explicit real-Android validation checklist for v0.8.

**Boundaries.** No FastCDC unless fixed chunks fail a measured case. No claim
that desktop measurements prove mobile safety. No private content in evidence.

**Dependencies.** G0; G1 supplies body-delta samples.

**Working state.** A findings report under `performance/v0.7-g2/` that gives G8
and G9 numeric limits and explains every proxy limitation.

**Validation and evidence.** 100/10k/100k-operation tiers; 0-byte, small, large,
and attachment-heavy resources; compression-bomb and count-limit rejection;
time/RSS/disk/file-count/round-trip estimates; deterministic byte checks.

**Open decisions**

- **Which deterministic envelope encoding/compression is v1? — Blocking,
  resolved by the spike.** Default candidate is canonical JSON/JSONL plus a
  pinned deterministic compression profile because it is inspectable and fits
  archive v2; a compact binary encoding must show material benefit and have a
  canonicalization specification.
- **When are resources chunked? — Blocking, resolved by the spike.** Default
  candidate is whole-object transfer below a measured threshold and fixed-size
  chunks above it. Content-defined chunking remains a later optimization unless
  repeated edits to large resources justify its CPU and complexity.
- **Which Android bounds ship? — Blocking only for a mobile release.** v0.7
  records provisional limits and v0.8 must lower or confirm them on a physical
  Android device.

## G3. Profiles, replica identity, and multi-instance process isolation

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

**Open decisions**

- **Where does profile configuration live? — Blocking.** Options are one
  registry containing all settings or a small registry pointing to one config
  file per profile. **Recommended: a small registry plus per-profile config.**
  It gives each server an explicit file, keeps process arguments free of
  secrets, and lets packaging relocate the config root in v0.8.
- **May an unpaired copied database synchronize? — Blocking.** Recommended: no.
  It must explicitly adopt the database universe while minting a new replica
  ID, or fork to a new database ID. A duplicated replica ID is always refused.

## G4. Replication schema and transactionally complete local journal

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

**Validation and evidence.** Migration upgrade/fresh-baseline parity; table-
driven coverage over every Store mutation; rollback/crash injection; batch and
import atomicity; journal-disabled baseline; 100k import/write overhead profile.

**Open decisions**

- **Which canonical rows are field registers versus indivisible records? —
  Blocking.** Recommended starting rule: user-editable scalar metadata is a
  field register; immutable revisions/operations/resources are indivisible;
  memberships are their own elements; derived rows are absent. The exact table
  belongs in the migration report and must be approved before implementation.
- **When does journaling begin? — Non-blocking.** Default: on explicit sync
  enrollment, paired with a full snapshot boundary. Target `none` before
  enrollment does not accumulate transport history.

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

**Open decisions**

- **What compatibility mismatches are accepted? — Blocking.** Recommended:
  database ID must match; protocol major must match; required capabilities must
  be understood; schema may differ only inside an explicit protocol
  compatibility range. Unknown required records are refused, never skipped.

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

**Open decisions**

- **Concurrent membership add/remove policy — Blocking.** Options are add-wins,
  remove-wins, or LWW per membership element. **Recommended: LWW per element
  using the protocol order**, because user intent has an order without keeping
  an unbounded observed-remove dot set; G1 must show whether that loses a
  material scenario.
- **Notebook-cycle repair — Blocking.** Recommended: retain the winning parent
  assignment by protocol order and reparent each losing/cycle-forming node to
  the nearest valid ancestor or root, with an explicit repair record. The exact
  algorithm must be deterministic from the operation set, not arrival order.
- **Purge propagation — Blocking.** Recommended: a signed death certificate
  outlives the document until every active peer acknowledges it; restore after
  purge creates a new document identity rather than resurrecting erased state.

## G7. Note revision objects, transfer deltas, three-way merge, and conflicts

**Goal.** Synchronize note bodies without silently overwriting concurrent edits
and without making every small edit transfer the entire body.

**Scope.** Represent each immutable note revision with parent revision IDs,
complete result hash/length, optional delta object and named base, authoring
replica/sequence, and merge parents. Fetch or reconstruct the complete body,
verify its hash, find a common ancestor, perform the G1-selected three-way
merge, and create an ordinary merge revision. Non-overlapping edits merge;
overlap creates a durable visible conflict holding both inputs and base. Never
apply an unverified patch to canonical state.

**Boundaries.** Not live collaboration. No hidden conflict-note duplication.
Delta chains must be bounded and never be the sole recovery representation.

**Dependencies.** G1 decision, G5 admission, G6 document metadata.

**Working state.** Independent edits to different regions converge to one
verified merge revision; overlapping edits preserve both variants and never
pretend to converge until the conflict is resolved by another revision.

**Validation and evidence.** Unicode/Markdown/long-line fixtures; clean and
conflicting merges in every delivery order; missing/corrupt/wrong-base delta;
bounded chain fallback; delete/edit and restore/edit; transfer-byte comparison
against full snapshots.

**Open decisions**

- **Where is unresolved conflict content stored? — Blocking.** Recommended: a
  typed conflict record attached to the same document and its revision graph,
  not a second ordinary note. The UI can show base/local/remote without
  inventing a notebook/title or polluting search and publication.
- **May an automatic clean merge be emitted by more than one replica? —
  Blocking.** Recommended: yes, if the merge revision ID is content- and
  parent-derived so equivalent merges deduplicate; otherwise elect one emitter
  deterministically.

## G8. Resource metadata, lazy materialization, chunks, and integrity

**Goal.** Converge attachment references immediately while downloading bytes
only when policy or user action needs them.

**Scope.** Sync immutable resource metadata, permanent `resource://` identity,
content hash, MIME, length, chunk manifest where applicable, availability
state, and source peers. A note may reference an admitted but nonmaterialized
resource. Opening/downloading/pinning requests missing objects, verifies hashes
and MIME through the existing admission path, and atomically marks them local.
Support eager/pinned/lazy policy and bounded concurrent fetches.

**Boundaries.** A remote-media URL is not a synced resource until ordinary
quarantine policy admits it. No direct carrier path leaks through REST/MCP/UI.
No placeholder bytes in the canonical asset store.

**Dependencies.** G2 bounds, G5 admission, G7 revision references.

**Working state.** A replica can read/search a synchronized note before its
large attachment is local, request it later, verify it, deduplicate it, and
survive all advertised sources being temporarily offline.

**Validation and evidence.** Whole/chunked object fixtures; range/resume;
missing/corrupt/MIME-mismatch sources; shared-blob dedupe; pin/unpin; restart;
bounded parallelism and disk limits; no accidental remote-media fetch.

**Open decisions**

- **Default materialization policy — Non-blocking.** Default: metadata and
  small inline-view resources are eligible for eager fetch within a byte
  budget; larger resources are lazy unless pinned. The G2 report supplies the
  threshold.
- **What happens when no peer currently has bytes? — Non-blocking.** Default:
  keep the resource metadata and a visible `unavailable` state indefinitely;
  never drop the reference or substitute an empty file.

## G9. Deterministic envelope/container codec, encryption, and signatures

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

**Dependencies.** G0 crypto decisions, G2 codec bounds, G5 operation ranges,
G7/G8 objects.

**Working state.** The same logical envelope produces the same canonical
plaintext bytes, decrypts only for an enrolled key, verifies its sender, rejects
replay/tampering before canonical admission, and streams within fixed limits.

**Validation and evidence.** Independent golden fixture generator; known-answer
crypto tests; tampered header/ciphertext/signature/object; wrong/revoked key;
nonce uniqueness; decompression bombs; deterministic pre-encryption bytes;
license inventory.

**Open decisions**

- **Which algorithms and Go libraries implement the G0 policy? — Blocking.**
  Prefer standard-library Ed25519 and a maintained BSD/MIT/Apache authenticated-
  encryption/KDF implementation with mobile-compatible builds. Record exact
  versions and parameters only after G0/G2; do not assume `go-keyring` covers
  Android—it currently documents macOS, Linux/BSD, and Windows only.
- **What metadata remains visible? — Blocking.** Recommended: expose only the
  minimum routing tuple (protocol/database/key IDs, artifact kind, bounded
  length, content address where needed); encrypt state vectors, record IDs,
  titles, MIME, and filenames.

## G10. Snapshot catch-up and reset state machine

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

**Open decisions**

- **Who may answer a backup request? — Blocking.** Recommended: only an active
  enrolled peer explicitly permitted as a snapshot source; if several answer,
  the requester chooses a compatible verified response rather than merging ZIPs.
- **How are password-encrypted portable backups related to enrolled-peer
  encryption? — Blocking.** Recommended: one archive payload format with two
  key-wrapping modes—peer-recipient keys for ordinary catch-up and a
  memory-hard password KDF for portable/cloud backup. The password is entered
  at decrypt/restore time and never stored in the archive or command history.

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

**Open decisions**

- **Who may initialize a missing directory? — Non-blocking.** Default: any
  enrolled profile may create the versioned/database-scoped skeleton and its own
  advertisement; no peer owns the carrier.
- **When may carrier artifacts be removed? — Blocking.** Recommended: only the
  writer removes its own expired artifacts after durable acknowledgement by all
  active peers, and correctness must survive no cleanup at all. Carrier cleanup
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

**Open decisions**

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

**Open decisions**

- **Does sync authentication authorize any ordinary REST route? — Blocking.**
  Recommended: no. A peer credential reaches only `/api/v1/sync/...` and the
  backup object it was explicitly granted; ordinary note APIs retain their
  existing local posture until a separate multi-user authorization milestone.
- **How is first pairing bootstrapped? — Blocking.** Recommended: a short-lived,
  one-use pairing bundle transferred by QR/file/manual code, containing no
  reusable library decryption key in displayable text. Exact UX waits for G18.

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

**Open decisions**

- **ZIP versus native outer container — Blocking.** Recommended: ZIP may be the
  user-facing/download wrapper, but archive-v2 manifest/object verification and
  encryption define correctness. Do not make ZIP central-directory parsing the
  trust boundary or require a seekable multi-gigabyte buffer.

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

**Open decisions**

- **Which sync actions may MCP initiate or cancel? — Blocking.** Recommended:
  MCP may plan, start ordinary incremental sync, request bounded resource fetch,
  and inspect status/conflicts at an explicit sync scope; it may not enroll a
  peer, reveal keys, request/export a backup, retire a peer, purge, or apply a
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

**Open decisions**

- **Where are long-lived secrets stored in v0.7 source builds? — Blocking.**
  Recommended: define an injectable secret-store interface; use an explicit
  locked-file development provider only with warnings and `0600`; v0.8 selects
  and validates native desktop/Android stores. Do not claim the desktop-only
  `zalando/go-keyring` solves mobile storage.
- **May the password be remembered? — Non-blocking.** Default: no. An opt-in
  native credential-store action can be added only after v0.8 validates the
  platform provider and labels the recovery consequences.

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

**Open decisions**

- **Default retention horizon — Blocking.** Recommended initial default: 90
  days plus at least one verified snapshot floor, configurable per profile, with
  warnings well before a peer crosses it. Evidence must report disk cost at the
  real-corpus scale before accepting this value.
- **Does retirement require every peer online? — Blocking.** Recommended: no;
  the local owner can sign a retirement decision, but other peers learn it
  through the ordinary log and refuse that replica thereafter. The UI must show
  peers that have not yet acknowledged the retirement.

## G18. Installation/mobile portability contract handoff

**Goal.** End v0.7 with an explicit, testable contract for the separate v0.8
installation/configuration/mobile milestone.

**Scope.** Inventory every runtime path, port, loopback/network permission,
shared-directory capability, file-picker need, credential-store operation,
background/lifecycle assumption, cgo/SQLite requirement, notification, and
deep-link association introduced by sync. Map them to Linux packages,
Windows/macOS sandboxes, Wails v3 desktop beta, and experimental Android/iOS.
Create build tags/interfaces where a desktop-only import would otherwise poison
mobile compilation, but do not migrate the shell.

**Boundaries.** No installer, APK, Wails v3 migration, Play Store claim, or
physical-device performance claim.

**Dependencies.** G3, G9, G11, G13, G16.

**Working state.** v0.8 can start from a finite permission/platform matrix and
compile-time seams rather than rediscovering hidden desktop assumptions.

**Validation and evidence.** Dependency/build-tag audit; headless and Wails v2
desktop regression; Android cross-compile feasibility report where tooling
allows; documented physical-device gates; no unsupported mobile library in the
shared core.

**Open decisions**

- None for v0.7. Wails v3 is currently beta for desktop while Android/iOS are
  explicitly experimental. v0.8 decides migration only after its own approved
  spike and real-device gate.

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
| Mandatory E2EE scope | G0 | **Open, blocking** |
| Per-replica signatures | G0 | **Open, blocking** |
| SVN dump compatibility | G0 | Open, non-blocking; default no |
| Canonical full-body/delta representation | G1 | **Open, blocking; investigation** |
| Three-way merge granularity/library | G1 | **Open, blocking; investigation** |
| Offline intervals measured | G1 | Open, non-blocking; four defaults |
| Envelope encoding/compression | G2 | **Open, blocking; investigation** |
| Resource chunk threshold | G2 | **Open, blocking; investigation** |
| Android bounds | G2 | Deferred gate for v0.8 physical device |
| Profile config layout | G3 | **Open, blocking** |
| Copied database enrollment | G3 | **Open, blocking** |
| Field-register/record map | G4 | **Open, blocking** |
| Journal start boundary | G4 | Open, non-blocking; enrollment default |
| Compatibility mismatches | G5 | **Open, blocking** |
| Membership add/remove rule | G6 | **Open, blocking** |
| Notebook-cycle repair | G6 | **Open, blocking** |
| Purge propagation | G6 | **Open, blocking** |
| Conflict representation | G7 | **Open, blocking** |
| Automatic merge dedup/emitter | G7 | **Open, blocking** |
| Lazy-resource defaults/unavailable bytes | G8 | Open, non-blocking defaults |
| Crypto algorithms/libraries | G9 | **Open, blocking after G0/G2** |
| Visible protocol metadata | G9 | **Open, blocking** |
| Snapshot responder policy | G10 | **Open, blocking** |
| Peer-key versus password backup wrapping | G10 | **Open, blocking** |
| Directory initialization | G11 | Open, non-blocking; any enrolled peer |
| Carrier artifact cleanup | G11 | **Open, blocking** |
| Sync credential reach into ordinary REST | G13 | **Open, blocking** |
| Pairing bootstrap | G13 | **Open, blocking** |
| ZIP wrapper contract | G14 | **Open, blocking** |
| MCP sync controls | G15 | **Open, blocking** |
| v0.7 secret-store provider | G16 | **Open, blocking** |
| Remember backup password | G16 | Open, non-blocking; default no |
| Retention horizon | G17 | **Open, blocking** |
| Offline peer retirement | G17 | **Open, blocking** |
| Release version/schema bookkeeping | G20 | Open, non-blocking until wrap-up |

The first implementable item is G0, but it is not approved by this draft. The
user reviews the decisions first; implementation begins only after an explicit
instruction naming the item.
