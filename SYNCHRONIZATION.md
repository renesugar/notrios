# Synchronization Architecture

Status: design target for v0.7; not implemented. `PLAN.md` contains twenty-two
independently approvable slices (G0, G1, G1a, and G2-G20). No slice starts
without resolution of its blocking decisions and explicit user approval.

## Goals and non-goals

Notrios synchronization keeps independently writable, intermittently connected
SQLite replicas convergent without making a server, shared folder, Git, Recoll,
or a search index canonical. It must work through:

- authenticated REST between two Notrios services;
- immutable files in a local/shared folder;
- the same folder copied by rclone to cloud storage;
- removable media such as a USB drive;
- a disabled target (`none`) that performs no synchronization.

The first protocol is single-user/multi-device. Multi-user authorization,
character-by-character simultaneous editing, public relay transport, and BLE
mesh transport are separate future features.

The shared directory is explicitly **ephemeral**. It is an untrusted,
disposable message carrier rather than a durable source of truth. If it is
deleted, peers reconstruct advertisements, requests, and still-retained
artifacts from their canonical database, journal, and acknowledgements. A
single shared mutable manifest would make that promise false and is forbidden.

## Identities and compatibility

Never infer replica identity from a path, hostname, or copied database file.
Persist distinct random identifiers:

| Identifier | Meaning | Copy/restore behavior |
|---|---|---|
| `profile_id` | User-visible local runtime configuration and routing target | New for a separately created profile; never synchronized |
| `database_id` | Logical synchronization universe | Preserved for an in-universe restore; changed for an explicit fork |
| `replica_id` | One writable database copy/device | Always new after cloning or restoring a writable copy |
| `operation_id` | `(replica_id, monotonically increasing sequence)` | Never reused |

A handshake includes `database_id`, `replica_id`, schema version, synchronization
protocol min/max, enabled capabilities, retention horizon, blob policy, and the
sender's per-replica acknowledgement vector. A database-ID mismatch refuses by
default and offers explicit create-profile, adopt, merge-import, or fork
workflows. Schema/protocol compatibility is negotiated before changes or blobs
are accepted.

`target: none` means that no peer, folder, REST, or background transfer job is
configured. Enabling sync later starts with a full snapshot, so retaining an
unbounded transport log while sync is disabled is not required.

One installation may register several profiles and run several `notriosd`
processes at once. Each profile resolves its own database/assets/config paths,
listen address, sync target, and credential references. Profiles must refuse
path, port, or replica-ID collisions rather than guessing which process owns a
database. A profile is not the MCP tool-visibility scope and is not replicated.

## Replication model

Use a small Notrios-specific, operation-based replication core:

1. A local write transaction allocates one or more consecutive operation IDs.
2. Each committed batch receives an HLC (operations retain consecutive sequence
   IDs) for deterministic last-writer ordering without overflowing a
   per-millisecond logical counter during a huge import.
3. The database transaction writes canonical state, immutable operations, and
   an outbox checkpoint atomically.
4. Peers exchange bounded envelopes and acknowledge the highest contiguous
   sequence applied for every known replica.
5. Applying an operation is transactional and idempotent; already-seen
   operation IDs are harmless.

HLCs order concurrent values, but they do **not** prove delivery completeness.
Per-replica sequence vectors detect missing operations and drive retries,
retention, tombstone collection, and peer retirement. Clock ties use
`replica_id` and then operation sequence as deterministic comparisons. A
wall-clock jump must not move an HLC backwards.

### Canonical data types

- Scalar note/notebook/resource metadata uses per-field LWW registers. One
  device changing a title need not overwrite another device changing an icon.
- Tag membership, resource attachment, notebook membership helpers, and other
  many-to-many rows are individually addressed LWW/observed-remove set
  elements. Do not synchronize an entire tag list as one scalar.
- Each saved body is an immutable revision object with parent revision IDs, a
  complete result hash/length, and optionally a transfer delta against a named
  parent. The complete result remains requestable so a missing base or broken
  delta chain is repairable. G1a investigates a bounded pure-Go binary-safe
  delta codec; Subversion's xdelta matcher, Subversion svndiff, and RFC 3284
  VCDIFF are distinct and no format is selected yet. Concurrent revisions with
  a common ancestor use a verified Notrios-owned pure-Go three-way merge:
  disjoint edits produce a merge revision and
  overlapping edits produce a typed conflict attached to the same document.
  Neither body is silently discarded and a conflict is not disguised as an
  ordinary duplicate note.
- Deletes create tombstone operations. A later stale snapshot cannot resurrect
  an object unless an explicit restore operation sorts after the delete.
- Purge emits a durable death certificate. Payload bytes may be collected only
  after retention and acknowledgement rules permit it; the minimum identity
  needed to reject resurrection remains.
- Notebook moves update an individually versioned parent register. After merge,
  a deterministic repair pass breaks cycles, rehomes missing-parent nodes into
  a protected Recovered notebook, and records a visible repair event. A formal
  replicated-tree move algorithm is a later upgrade if tests show that this
  conservative rule loses important intent.

Derived FTS5 tables, Recoll projections/indexes, counts, caches, rendered sites,
and search snippets never synchronize. They rebuild from canonical data.

## Change envelopes and dependency ordering

An envelope is a versioned, deterministic object containing:

- protocol/container versions and `database_id`;
- sender `replica_id`, sequence interval, HLC range, creation time;
- operation records and dependency object hashes;
- the sender acknowledgement vector and optional snapshot base;
- content length and SHA-256 for every part;
- authenticated-encryption and per-replica Ed25519 signature metadata required
  by the resolved G0 policy;
- a final envelope checksum.

Bound envelopes by both record count and encoded bytes. G2 measures and fixes
the v1 limits; a desktop proxy does not prove Android safety, so v0.8 checks an
Android emulator and the post-1.0 Flutter release gate confirms or lowers the
limits on physical devices. This is transport framing, not a merge rule.

Publish dependencies in this order:

1. immutable revision/resource blobs or blob chunks needed eagerly;
2. notebook/tag/resource identity operations;
3. note revisions and metadata;
4. membership/link/current-pointer operations;
5. the envelope manifest/commit marker.

Receivers verify length, hash, database/protocol identity, and policy before
moving bytes from quarantine. A complete transaction applies atomically. An
out-of-order operation with unavailable dependencies stays in a bounded pending
queue and is retried when dependencies arrive. Missing dependencies are
requestable by hash; they must not turn into broken canonical references.

Resource metadata may be admitted before its bytes. A permanent
`resource://` identity, content hash, MIME, length, chunk manifest, and source
availability allow a replica to show a synchronized note and lazily fetch,
resume, verify, and atomically admit the resource when it is opened or pinned.
Unavailable bytes remain a visible state; the reference is never replaced by
an empty file. Remote-media URLs still go through the existing quarantine and
SSRF policy before they become canonical resources.

### Blob and chunk policy

Whole-resource content addressing is the initial default because attachments
are usually immutable and existing Notrios blobs already deduplicate exact
bytes. Split only large objects at a measured threshold so a transfer can
resume and memory stays bounded. Fixed chunks are simpler for the first
implementation.

FastCDC/content-defined chunking is an optional later optimization for large
binaries that are repeatedly modified. It is not the change algorithm, is not
needed for Markdown bodies, and should be adopted only if representative
resource traces show enough bandwidth benefit to justify another chunk index
and garbage-collection layer.

## Transports

All transports carry the same immutable snapshots, envelopes, blobs, receipts,
and acknowledgement vectors. Transport code does not implement conflict rules.

### REST

REST is the first online transport. The conceptual surface is:

- negotiate/handshake and read peer status;
- list missing envelope ranges/object hashes;
- upload/download immutable objects with range/resume and hash verification;
- publish a manifest;
- submit/read acknowledgement vectors;
- request a full snapshot and range-download its completed encrypted ZIP/native
  archive wrapper for catch-up or reset;
- start, inspect, cancel, and retry a sync job.

Authentication, authorization, TLS expectations, quotas, request-size limits,
rate limits, cancellation, and audit records are mandatory before remote bind.

MCP may plan/start ordinary incremental sync, request bounded resource fetch,
and inspect status/conflicts at an explicit sync scope. It returns job IDs and
summaries, not envelopes, keys, backups, or arbitrary blob bytes in an LLM
context. Enrollment, peer retirement, purge, backup export/restore, destructive
recovery, and cancelling another actor's catch-up remain outside MCP.
REST/object storage remains the data plane.

### Shared folder and rclone

The folder layout uses unique immutable names, for example:

```text
notrios-sync/v1/<database-id>/
  replicas/<replica-id>/advertisements/<generation>-<hash>.bin
  replicas/<replica-id>/requests/<generation>-<hash>.bin
  replicas/<replica-id>/envelopes/<first>-<last>-<hash>.bin
  replicas/<replica-id>/acknowledgements/<generation>-<hash>.bin
  replicas/<replica-id>/snapshots/<snapshot-id>/<artifact>
  objects/sha256/ab/cd/<hash>
  staging/<replica-id>/<private-temporary-name>
```

Writers create a private temporary file, flush it, verify its hash, atomically
rename it to the immutable name, then publish the manifest last. Readers ignore
temporary, unknown, incomplete, or hash-invalid files.

Each replica writes only its own namespace. Discovery comes from signed,
immutable advertisements; missing ranges/objects and snapshot catch-up use
signed requests. No peer overwrites another peer's acknowledgement or deletes
another peer's only copy. Correctness must work by explicit scan/poll/manual
sync; filesystem watchers improve latency only.

rclone is a carrier, not the synchronization algorithm. Use non-destructive
immutable copying, conceptually `rclone copy --immutable` (and `--no-traverse`
when a small set of new objects is copied into a very large destination).
Never use `rclone sync` or bisync to decide Notrios deletions: mirrored
filesystem deletion can discard another replica's only envelope, and
modification-time/size comparison cannot represent CRDT conflict semantics.
Application deletes travel as operations.

The same format works when both services watch one carrier directory or when a
user copies the carrier to a USB drive. Multiple devices share the
database-scoped root but write disjoint replica namespaces; content-addressed
objects are immutable and identical names must have identical verified bytes.

The available `/home/renes/GoogleDrive` mapping and `rclone copy --immutable`
are conformance-test carriers only. Notrios does not wrap rclone as its sync
engine, require its config, or carry that dependency to mobile.

## Import, export, backup, restore, and sync

These workflows share storage/container primitives but have different
semantics:

| Workflow | Meaning |
|---|---|
| Foreign import | ETL from Joplin/Obsidian/etc. into canonical transactions |
| Foreign export/publish | A projection that may be format-limited or lossy |
| Native snapshot export | Full versioned state plus revisions, provenance, blobs, hashes, and compatibility metadata |
| Backup | A native snapshot stored as a non-participating sink; it does not block peer GC |
| Restore-replace | Verify, preserve an emergency backup, replace state, mint a new replica ID |
| Restore-merge | Feed snapshot records through the replication/import merge rules |
| Sync | Repeated bidirectional exchange of incremental operations and acknowledgements |

### Snapshot catch-up and reset

A blank, far-behind, repaired, or user-reset replica may publish a signed
snapshot request. An enrolled peer permitted to act as a snapshot source
creates a consistent archive-v2 snapshot, binds it to a state-vector boundary,
encrypts it for the requesting peer (or through the separately reviewed
password-wrapping mode), and publishes the complete manifest last. The
requester resumes transfer, verifies and decrypts before any canonical write,
chooses an explicit restore intent, then requests only operations after the
snapshot vector.

The same state machine is carried through the directory and REST. REST exposes
an opaque authorized artifact ID with range download; it never accepts an
arbitrary server path. ZIP can be a user-facing transport wrapper, but the
archive-v2 checksum/capability chain and encrypted payload define correctness.
The UI prompts for a password without placing it in a manifest, process
argument, job record, log, or command history and offers retry/cancel on a wrong
password. A snapshot/backup is a sink, not a peer acknowledgement.

The v0.4 P2 native archive-v2 identity/manifest/object verifier is the
full-snapshot format foundation. P3/P3a/P3b add streaming export under a loose
or packed object layout and P4 adds verification and restore under explicit
intent; v0.7 adds operation logs, acknowledgement vectors, and transports
without inventing a second blob/manifest format. The packed layout exists
because loose objects cost one transport round trip each: the 382,206-note
corpus is 382,447 files loose and 46 packed.

Imports execute in bounded transactions under one import-job identity. They may
allocate an HLC plus consecutive operations for each committed batch; source
timestamps are metadata, never the synchronization clock. Blobs publish before
note references. A batch checkpoint/manifest prevents peers from mistaking a
partially imported source inventory for a complete import. Sync need not stop
globally, but backpressure must keep foreground editing responsive.

Exports use a stable SQLite read snapshot. Restore and database replacement stop
writes; online backup uses the SQLite backup API or an equivalent consistent
snapshot. Copying only the live `.sqlite` file while WAL writes are active is
not a valid backup, and a complete backup must also include referenced asset
bytes.

## Retention, GC, and offline peers

An active peer's acknowledgement vector is a safety input. Do not collect an
operation, tombstone, death certificate, conflict revision, or referenced blob
until:

- the configured retention interval has elapsed;
- every non-retired peer has acknowledged the required operation range;
- no retained snapshot/reference depends on the payload;
- the user-visible dry run explains the decision.

Peer retirement is explicit and audited. A replica returning after its allowed
offline/retention horizon performs a full snapshot resync; it cannot demand
history that was legitimately collected. Backups are sinks, not peers, and
therefore never hold the acknowledgement watermark open.

## Conflict and recovery UX

Normal scalar conflicts resolve deterministically and remain auditable. The UI
needs explicit views for:

- concurrent body revisions and their source replicas/timestamps;
- notebook-tree repairs and recovered items;
- rejected database/schema/protocol identities;
- pending/missing/corrupt objects;
- peers behind the retention horizon;
- restore replace/merge/adopt/fork choices;
- retryable versus permanent transport errors.

The setup and recovery UI also names the active local profile, offers
`none|directory|rest`, lets the user choose the shared directory, requires
explicit peer pairing, shows lazy-resource availability, supports catch-up/reset
requests, and handles password-encrypted backups. It does not silently retain a
password or apply a destructive restore.

“Last writer wins” is not a sufficient user explanation. Show both candidate
values when a meaningful body or structural edit lost the current-pointer
comparison.

## Library decision

The replication core should be a separate, small Go package (either internal
first or a separately versioned library after its API stabilizes) with:

- no dependency on the Notrios HTTP, Wails, MCP, Recoll, or UI packages;
- deterministic encoders and cross-version golden fixtures;
- HLC and operation-ID allocation;
- per-field register/set/tree merge rules;
- sequence-vector diff and acknowledgement logic;
- envelope/snapshot manifests and hash verification;
- bounded pending dependencies and retention calculations;
- property/fuzz tests that shuffle, duplicate, drop, replay, and partition
  operations and still prove convergence after eventual delivery;
- storage interfaces so SQLite and in-memory model tests share semantics;
- transport-neutral APIs and no unrestricted filesystem access.

Cryptographic identity, encoding, and secret storage remain separate
interfaces. The resolved security shape is authenticated encryption plus a
per-replica Ed25519 signature over canonical control/envelope bytes: AEAD
protects confidentiality and integrity for holders of the library key, while a
signature attributes an artifact to an enrolled/revocable replica. This is a
selected design requirement, not an implemented guarantee. The desktop-oriented
`zalando/go-keyring` cannot be treated as the mobile abstraction: its own
platform list is macOS, Linux/BSD, and Windows. v0.8 must validate native
desktop and Android secret stores behind the interface.

G0's reviewed threat model and normative vocabulary are under
`performance/v0.7-g0/`. Discovery never enrolls a peer. Enrollment explicitly
binds database, replica, signing key, encryption recipient/key epoch,
capabilities, and status. A compromised replica's revocation must also advance
the encryption epoch for remaining active peers; disabling only its signing
key would not stop it reading future traffic with encryption material it kept.
Historical plaintext already obtained cannot be revoked.

The G9 outer format must sign domain-separated canonical artifact bytes,
including visible routing and a ciphertext commitment, and bind that visible
header as AEAD associated data. Signature, AEAD, content hash, and replay state
are independent controls. Plaintext body/resource hashes, names, operation
kinds, state vectors, request ranges, and acknowledgement positions remain
inside encryption. A carrier-visible content address, where needed, addresses
ciphertext/artifact bytes rather than exposing a raw plaintext hash. These too
are selected requirements, not current controls.

Marmot is not adopted: it targets an always-on distributed SQLite server with
gossip, SQL proxying, distributed transactions, and a large operational
dependency set. Its HLC, immutable CDC segment, manifest-last, and anti-entropy
patterns are useful references.

Cachapa's Apache-2.0 Dart CRDT/sqlite_crdt/crdt_sync projects demonstrate a
compact record-level HLC/LWW model and WebSocket delta exchange. They do not
cover Notrios's per-field merge, tree invariants, content-addressed resources,
revision conflicts, acknowledgement-based GC, or Go/mobile integration, so
they are references rather than porting sources.

The reviewed Go Ygo implementations are MIT-licensed, active, pure-Go
Yjs-compatible CRDTs with state-vector synchronization and mobile work. They
solve collaborative document editing, not relational database replication.
If live co-editing becomes a requirement, choose one through a separate
maintenance, interoperability, security, size-growth, and mobile benchmark.

## Required validation

- Model/property tests over every operation ordering, duplication, replay, and
  partition pattern in the bounded generated state space.
- At least three replicas with concurrent scalar, set, body, delete/restore,
  purge, and notebook-move operations.
- Crash injection before/after every object, manifest, transaction, and
  acknowledgement boundary.
- REST and ephemeral-directory conformance against identical golden transcripts.
- Same-machine, removable-drive interruption, slow link, corrupt/truncated
  object, clock-skew, schema mismatch, and device-clone tests.
- Full snapshot bootstrap and forced full resync after retention expiry.
- Shared-directory deletion/recreation, signed discovery and snapshot request,
  lazy resource materialization, encrypted backup password failure, and
  multi-profile/two-process port/path isolation.
- Hundreds-of-thousands-of-notes profiles with bounded envelopes, foreground
  responsiveness, peak RSS, bytes transferred, and convergence time recorded.

## Resolved policy and investigation outputs

`PLAN.md` is the authoritative home because each decision must be visible in
the item it affects. The user's 2026-08-11 review resolved G0-G17 policy,
including:

1. mandatory end-to-end encryption and per-replica digital signatures;
2. complete-body plus optional delta representation and three-way merge;
3. deterministic envelope encoding/compression and resource chunk bounds;
4. profile layout, copied-database enrollment, and compatibility refusal;
5. field-register, membership, notebook-cycle, purge, and conflict rules;
6. snapshot responder/password wrapping and ephemeral-carrier cleanup;
7. REST pairing/authorization, ZIP wrapper, and MCP control boundaries;
8. secret-store behavior, retention horizon, and offline-peer retirement.

G1 completed its investigation under `performance/v0.7-g1/`: complete UTF-8
result objects remain canonical; optional named-parent line deltas are retained
only when bounded generation, exact reconstruction, and a material size gate
succeed; and merge is bounded line-first with Unicode-aware word-token
refinement of conflict regions. Same-token/delete-edit overlap remains a typed
durable conflict. G1a is now the next unapproved investigation: it evaluates a
pure-Go xdelta/VCDIFF codec for arbitrary bytes and hostile-decoder bounds while
keeping the separate G7 line/word merge implementation Notrios-owned. G2 remains
an investigation: the review selected its decision rule and default candidate,
not an unmeasured implementation. Android limits remain
provisional through the v0.8 emulator pass and require post-1.0 physical-device
confirmation.

G18 also records a framework-neutral Go application facade and pre-1.0 C ABI
handoff. That ABI adds lifecycle, ownership, typed-error, cancellation/event,
capability, and bounded-stream contracts around the same application semantics;
it does not add ordinary REST routes or put HTTP inside the shared library. See
`FLUTTER_GO_CLIENT.md`.

External relay and BLE transport questions are intentionally deferred until the
REST/directory protocol, threat model, and retention behavior are stable.
