# Synchronization Architecture

Status: design target for v0.7; not implemented. Each implementation slice
requires a new active plan and user approval.

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

## Identities and compatibility

Never infer replica identity from a path, hostname, or copied database file.
Persist distinct random identifiers:

| Identifier | Meaning | Copy/restore behavior |
|---|---|---|
| `profile_id` | User-visible local configuration and routing target | New for a separately created profile |
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
- Each saved body is an immutable revision/blob. The current-body pointer is an
  LWW register, but concurrent body revisions are retained and surfaced as a
  conflict copy instead of silently discarding either body.
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
- optional encryption/signature metadata;
- a final envelope checksum.

Bound envelopes by both record count and encoded bytes. The first implementation
should start with measured defaults in the 4–16 MiB range and adapt downward on
mobile/slow links. This is transport framing, not a CRDT rule.

Publish dependencies in this order:

1. immutable resource blobs or blob chunks;
2. notebook/tag/resource identity operations;
3. note revisions and metadata;
4. membership/link/current-pointer operations;
5. the envelope manifest/commit marker.

Receivers verify length, hash, database/protocol identity, and policy before
moving bytes from quarantine. A complete transaction applies atomically. An
out-of-order operation with unavailable dependencies stays in a bounded pending
queue and is retried when dependencies arrive. Missing dependencies are
requestable by hash; they must not turn into broken canonical references.

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
- request or stream a full snapshot;
- start, inspect, cancel, and retry a sync job.

Authentication, authorization, TLS expectations, quotas, request-size limits,
rate limits, cancellation, and audit records are mandatory before remote bind.

MCP exposes bounded administration—plan/start/cancel/status/conflicts—over the
same service layer. It returns job IDs and summaries, not multi-megabyte
envelopes or arbitrary blob bytes in an LLM context. MCP is therefore a control
plane, while REST/object storage is the data plane.

### Shared folder and rclone

The folder layout uses unique immutable names, for example:

```text
notrios-sync/v1/<database-id>/
  objects/sha256/ab/<hash>
  envelopes/<replica-id>/<first>-<last>-<hash>.bin
  manifests/<replica-id>/<first>-<last>-<hash>.json
  acknowledgements/<replica-id>/<generation>-<hash>.json
  snapshots/<snapshot-id>/manifest.json
```

Writers create a private temporary file, flush it, verify its hash, atomically
rename it to the immutable name, then publish the manifest last. Readers ignore
temporary, unknown, incomplete, or hash-invalid files.

rclone is a carrier, not the synchronization algorithm. Use non-destructive
immutable copying, conceptually `rclone copy --immutable` (and `--no-traverse`
when a small set of new objects is copied into a very large destination).
Never use `rclone sync` or bisync to decide Notrios deletions: mirrored
filesystem deletion can discard another replica's only envelope, and
modification-time/size comparison cannot represent CRDT conflict semantics.
Application deletes travel as operations.

The same format works when both services watch one local directory or when a
user copies the directory to a USB drive. Multiple devices may write the same
namespace because no committed object name is overwritten.

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
- REST and folder/rclone conformance against identical golden transcripts.
- Same-machine, removable-drive interruption, slow link, corrupt/truncated
  object, clock-skew, schema mismatch, and device-clone tests.
- Full snapshot bootstrap and forced full resync after retention expiry.
- Hundreds-of-thousands-of-notes profiles with bounded envelopes, foreground
  responsiveness, peak RSS, bytes transferred, and convergence time recorded.

## Questions to resolve before implementation

1. Is metadata merge per field for every type, or do some small records use one
   LWW register?
2. Do tag/set concurrent add-versus-remove conflicts use add-wins,
   remove-wins, or an explicit LWW membership clock? The initial proposal is
   LWW per membership row because it matches visible user intent and supports a
   deterministic restore.
3. How long is the default offline retention horizon, and how is peer
   retirement confirmed?
4. Is transport encryption always end-to-end above rclone/REST, optional for
   already encrypted private stores, or deferred? Integrity hashes alone do not
   provide confidentiality or writer authentication.
5. Which deterministic encoding and compression become protocol v1?
6. What resource size triggers fixed chunking, and what measurements would
   justify FastCDC later?
7. Does a notebook cycle repair always choose the lowest operation ID as the
   losing move, and how is the Recovered path presented?
8. What maximum envelope/blob/pending sizes are safe on the first real Android
   target?

External relay and BLE transport questions are intentionally deferred until the
REST/folder protocol, threat model, and retention behavior are stable.
