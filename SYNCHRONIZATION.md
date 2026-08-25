# Synchronization Architecture

Status: design target for v0.7; G3-G5 implement isolated profiles, the local
journal, and transport-neutral bounded admission, but no carrier or merge
engine. `PLAN.md` contains twenty-two
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

G3 implemented that local lifecycle in 2026-08-12: registry/config/database
binding, collision and staleness validation, explicit copied-database
adopt/fork, active-profile status, and a two-process smoke are live. Sync target
fields initially remained configuration only. G4/G5 have since added local
journal enrollment and transport-neutral admission, while keys, containers,
carriers, and background transfer remain unimplemented.

## Replication model

Use a small Notrios-specific, operation-based replication core:

G4 implements the local durability foundation of this model in schema v19.
Explicit enrollment records a full-snapshot boundary, and transaction-local
capture produces monotonic immutable operations for canonical rows. Derived
indexes/projections are excluded. The dependency/gap/ack/pending tables exist,
G5 implements bounded remote-operation admission and gap/dependency planning;
G6 applies deterministic metadata/register/tree convergence in schema v21 for
explicitly configured local fixture peers. G7 adds note-body convergence in
schema v22: immutable revision objects with named parents and exact content
hashes, optional named-base transfer deltas, a bounded line-first three-way
merge with word-region refinement, and durable typed conflicts. Resources,
transport, authenticated enrollment, and cryptographic framing remain later
work.

A revision travels as its identity plus either its complete body inline or a
delta against a named parent. A delta is a transfer optimization and never
canonical state: reconstruction verifies the base hash, decodes, and verifies
the exact result hash and length before any canonical write, and every refusal
falls back to obtaining the complete object rather than patching best effort.
Non-overlapping edits become an ordinary merge revision whose identity is
derived from the document, its sorted parents, and the merged bytes, so two
replicas computing the same merge produce one revision rather than two.
Overlapping edits become a conflict attached to the document and its revision
graph, holding the base and both inputs, and never a second note.

G9 turns all of that into artifacts. `internal/syncwire` defines one canonical
byte representation per logical envelope — minimal varints, sorted vectors and
dependencies, pinned gzip — so two replicas that agree on the content agree on
its hash and its signature. Each artifact is sealed with AES-256-GCM under a key
derived per artifact by HKDF-SHA256 from a fresh random salt, then signed with
Ed25519 over domain-separated canonical outer bytes. The visible header carries
only the routing tuple G0 permitted: protocol, key and signer identifiers, the
artifact kind, a bounded length, and a keyed blind of a content address where a
carrier needs a name. Record ids, titles, MIME types, filenames, and state
vectors are inside the ciphertext. The header is both the HKDF salt and the AEAD
associated data, and the signature is checked before decryption. An encryption
epoch advances when a replica is revoked; retiring an epoch is a separate act
from advancing it, so a library keeps reading its own history. Every primitive
is from the Go standard library, so G9 adds no dependency.

G10 adds snapshot catch-up in schema v24. A blank, far-behind, repaired, or
reset replica signs a backup request; only an active enrolled peer it has
*explicitly permitted* as a snapshot source may answer; and among several
answers it takes one compatible verified offer rather than merging them. The
snapshot's manifest records the state vector it was taken at, so after restoring
under an explicit `adopt|replace|merge|fork` intent the replica asks only for
later envelopes. Cutover also records a catch-up floor per replica, because a
library built from a snapshot has canonical state without the operations that
produced it and would otherwise look like a history with a hole in it. A backup
is not a peer acknowledgement and holds nothing back from collection.

G8 adds attachment convergence in schema v23. A resource's identity, length,
content type, and transfer shape travel as ordinary metadata, so a note that
references an attachment is usable before the attachment is. Bytes are fetched
separately through a transport-neutral object provider: whole below G2's one
mebibyte threshold, 1 MiB chunks above it, resumed from verified segments, and
installed only after the manifest digest, every chunk hash, the whole-object
hash, the byte length, and the sniffed content type all agree. When no peer has
the bytes, the resource keeps its metadata and a visible unavailable state
indefinitely; nothing drops the reference or substitutes an empty file.

The live G8 compatibility tuple is protocol 1.0 with schema compatibility 23
and required capabilities `sync.dependencies.v1`, `sync.metadata-lww.v1`,
`sync.operations.v1`, and `sync.state-vectors.v1`. Database ID and protocol
major must match, ranges must intersect, every required capability must be
understood, and unknown required record/kind pairs reject. A handshake never
enrolls a peer. Vectors and plans are capped at 1,024 entries/ranges and 10,000
requested sequences; pending work stays disk-backed under 10,000-operation/64
MiB per-peer bounds and a 10,000-position sparse-sequence window.

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
  delta chain is repairable. G1a recommends a bounded pure-Go Subversion-style
  matcher with a constrained RFC 3284 default-table VCDIFF transfer profile;
  Subversion svndiff remains distinct and is not selected. Production adoption
  remains gated on G2's completed numeric bounds and a later G7/G8 approval.
  Concurrent revisions with
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

Close a v1 candidate envelope at the first of 10,000 operation records, 16 MiB
canonical record bytes, or 4 MiB compressed record bytes. One record is at most
1 MiB, its kind payload at most 512 KiB, and it names at most 64 dependencies.
G2 recommends compact canonical NCB1 operation records, a canonical-JSON outer
manifest, and deterministic gzip level 6 with fixed headers; G9 must promote or
replace that evidence format and pin cross-toolchain goldens. A desktop proxy
does not prove Android safety, so v0.8 confirms or lowers the limits on an
Android emulator and the post-1.0 Flutter release gate does so on physical
devices. This is transport framing, not a merge rule.

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
bytes. G2 keeps resources below 1 MiB whole and splits resources at or above
that threshold into 1 MiB fixed chunks, up to 16,384 chunks under archive-v2's
existing 16 GiB resource ceiling. A 64 KiB range on a chunked resource may
transfer one aligned 1 MiB chunk; every chunk is verified and the resource is
admitted atomically only when complete.

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

G15 implements this as a schema-v26 sync-only extension of the existing job
record. An explicit CLI, local REST, or MCP control call queues one closed kind;
the daemon drains queued/due rows but never creates work on a timer. One target
may have one running job, while different targets can progress concurrently.
Each attempt has a byte allowance and durable `plan`, `pull`,
`resource_serve`, `push`, `advertise`, and optional `resource_fetch`
checkpoints. A crash preserves the last checkpoint and replans from canonical
vectors/verified chunks. Offline, quota, temporary, and budget failures use
exponential backoff with stable ±20% jitter, capped at 15 minutes. MCP sync
authority is separately configured as `disabled|status|control` and never
reaches catch-up/restore preparation, enrollment, keys, backup/restore,
retirement, purge, or reset.

### Shared folder and rclone

The folder layout uses unique immutable names. As implemented in G11
(`internal/synccarrier`):

```text
notrios-sync/v1/<blinded-database>/
  replicas/<blinded-replica>/advertisements/<blinded-name>.nar
  replicas/<blinded-replica>/requests/<blinded-name>.nar
  replicas/<blinded-replica>/envelopes/<blinded-name>.nar
  replicas/<blinded-replica>/objects/<ab>/<blinded-name>.nar
  replicas/<blinded-replica>/snapshots/<content-hash>.nar
  replicas/<blinded-replica>/staging/<private-temporary-name>.tmp
```

Every path segment below the layout version is a keyed HMAC blind under the
group key (`syncwire.CarrierName`), and the drafted layout above it was
corrected in three places when it met G0's metadata budget:

- **an `objects/sha256/ab/cd/<hash>` tree publishes plaintext content hashes**,
  which is exactly what the budget forbids: anyone holding the same file could
  confirm the library holds it. Object addresses are keyed blinds.
- **`<first>-<last>` in an envelope name publishes a sequence range**, and
  request ranges and acknowledgement positions are on the encrypted side of the
  budget. An artifact's name is a blind of what it logically covers, so the name
  is stable — republishing the same thing is a no-op — without being readable.
- **`acknowledgements/` is not a separate class.** A replica's contiguous state
  vector *is* what it has durably admitted from every peer, so the advertisement
  carries both. Two artifacts for one fact can disagree; one cannot.

A name is stable rather than derived from sealed bytes because every seal draws
a fresh salt: byte-named artifacts would leave a new file per round on a shared
drive forever. Writers create a private temporary file, flush it, rename it, and
publish the advertisement last, so everything an advertised vector implies is
already readable. Where a filesystem refuses to rename, the writer falls back to
writing in place — correctness does not depend on atomic rename, because
artifacts are authenticated and a torn one fails to open.

A publisher republishes when its own copy is *unreadable*, not merely absent: a
half-copied artifact keeps its name, and treating the name as proof would strand
the peer waiting for it.

Each replica writes only its own namespace, including its objects and staging.
Discovery comes from signed, immutable advertisements and reports candidates it
cannot verify by signing key alone; pairing stays an explicit act. A round
begins from what the journal remembers about each enrolled peer, so a new or
deleted folder still receives exactly the missing work rather than waiting to be
greeted. No peer overwrites another peer's artifact or deletes another peer's
only copy, and correctness survives no cleanup at all. Correctness works by
explicit scan/poll/manual sync; filesystem watchers improve latency only.

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

A mapped cloud folder and `rclone copy --immutable` are conformance-test
carriers only. Notrios does not wrap rclone as its sync engine, require its
config, or carry that dependency to mobile.

G12 ran the shipped protocol against Google Drive through `rclone mount` and
measured what such a carrier costs (`performance/v0.7-g12/`). Two results
change how the rest of the milestone should be planned:

- **A change published by another device took 45–57 seconds to become
  visible.** That is the provider's change-notification cadence, not a protocol
  cost, and it is the floor on how fresh any folder-carried exchange can be.
  Scheduling work should treat a few minutes as a sensible interval and an
  explicit sync after real work as better than any interval.
- **Resolving a known name is no fresher than listing the directory** — the two
  differ by a millisecond or two. A stale listing cannot be worked around by
  fetching an object whose address is already known.

Consequently **publication order is a latency optimization on such a carrier,
not a correctness mechanism**: a reader may see an advertisement before the
envelopes it implies, and must admit nothing, claim no progress, and converge on
a later round. That is asserted directly rather than assumed. The same run
confirmed that no peer writes outside its own namespace even while the
provider's listing lags, that a substituted artifact is refused and republished
by its owner while a peer still needs it, and that a carrier deleted in full
costs only republication.

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
creates a consistent whole-library snapshot, binds it to a state-vector boundary,
encrypts it for the requesting peer (or through the separately reviewed
password-wrapping mode), and publishes the complete manifest last. The
requester resumes transfer, verifies and decrypts before any canonical write,
chooses an explicit restore intent, then requests only operations after the
snapshot vector.

The same protocol is carried through the directory and REST. REST exposes
an opaque authorized artifact ID with range download; it never accepts an
arbitrary server path. G14d uses a deterministic sequential USTAR wrapper and
fixed 1 MiB authenticated-encryption frames; the physical manifest/database/
pack verifier, not the wrapper, defines correctness. Directory bulk transfer
publishes and resumes the same sealed bytes without the ordinary small-artifact
in-memory limit.
The UI prompts for a password without placing it in a manifest, process
argument, job record, log, or command history and offers retry/cancel on a wrong
password. A snapshot/backup is a sink, not a peer acknowledgement.

The v0.4 P2 native archive-v2 identity/manifest/object verifier is the
full-snapshot format foundation. P3/P3a/P3b add streaming export under a loose
or packed object layout and P4 adds verification and restore under explicit
intent; v0.7 adds operation logs, acknowledgement vectors, and transports
without inventing a second blob/manifest format. The packed layout exists
because loose objects cost one transport round trip each: the 382,206-note
corpus is 382,447 files loose and 46 packed. G2 provisionally targets sync
carrier packs at 64 MiB/4,096 contained objects with a 4 MiB trailer ceiling,
below the desktop archive writer's 256 MiB/65,536-object targets because the
current reader loads a trailer whole. G9 must preserve loose compatibility and
bound or stream trailer reads.

That physical-format conclusion was reopened before G15. G14's original
catch-up path exported loose archive-v2, wrapped each object as a stored ZIP entry,
then seals the stream; ZIP metadata added about 25% at 100 and 500 notes, while
the authentication frames add only 28 bytes per MiB. G14a-G14e now block later
sync work. G14a completed the resumable aggregate-only harness and its generated
10k/100k calibration: the loose archive produced 100,093 files at 100k, stored
ZIP added 11.15%, authenticated framing added only 6,004 bytes, and one
incremental replay exceeded the 512 MiB desktop gate. G14b then compared the
complete candidates on the supplied real corpora and selected a required,
same-schema SQLite image plus bounded packed external assets for whole-library
backup/catch-up. Packed semantic archive-v2 remains the subset, merge,
interchange, and fallback path. G14c implements local creation and complete
read-only admission for `sqlite-image+packed-assets.v1`. G14d makes it the
encrypted catch-up artifact, creates a verified emergency physical snapshot,
durably rolls forward an explicit replace/adopt cutover, rotates the replica
allocator, installs vector/floors, queues the external derived index, and proves
ordinary post-vector replay. Packed semantic archive-v2 remains the merge,
fork, portable, incompatible-schema, and subset path. G14e freezes the physical
capability as the compatible whole-library default only after exact full-scale
canonical/resource/source-bundle round trips, current/previous semantic reader
coverage, REST/directory resume, emergency replacement, post-vector replay, and
the frozen Restic/Borg integrity rows pass. A raw
live-WAL copy is never a candidate. SQLCipher may encrypt a database image but does not by itself bind
external assets or provide semantic subset/merge, capability verification,
signatures, or replica-identity handling.

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

G17 implements this policy with a configurable 90-day history interval and a
30-day warning window. The eligible sequence for each replica is the minimum
of age, current vector, a currently usable verified snapshot, and every active
peer acknowledgement; existing floors never move backward. Destructive CLI
commands re-verify the retained physical snapshot and database identity at the
boundary and require the exact dry-run digest. A verified snapshot that falls
below any prior collection floor is not a repair source. The repair plan names
the covering snapshot plus newer operations and is never installed
automatically.

Key revocation only ends credential trust and deliberately leaves that replica
holding the watermark open. Retirement is a separate, signed `replica.retire`
operation with explicit owner confirmation. It travels through the ordinary
log without requiring all peers online, revokes old credentials, reports which
active peers have not acknowledged the decision, and requires the retired
device to reset and pair as a new replica. Signed permanent purge retains the
document payload until safe collection, then keeps death-certificate identity
in the checkpoint so replay cannot resurrect it.

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

G16 implements the setup and recovery UI as a responsive loopback/native-only
Sync Center. It names the active profile/database/replica, offers
`none|directory|rest`, uses the native desktop directory chooser when available,
requires explicit peer pairing, separately grants complete-snapshot permission,
shows durable pending/running/offline/failed/behind/retired state, and drives
lazy-resource fetch/pin. Conflict review shows common base, current, and other
bodies and writes an explicit two-parent resolution revision. Catch-up verifies
into private staging; reset and restore remain blocked on destructive review.
NPB1 password backups reuse the verified physical NBK1 payload, wrap a fresh
payload key with Argon2id, clear password fields after each attempt, and never
persist or copy the password. This is a responsive/mobile design, not a mobile
build claim.

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
the item it affects. The user's 2026-08-11 review resolved G0-G17 sync policy;
the later G17a evidence-preservation investigation owns the still-open signing
fingerprint and RFC 3161 authority/policy choices before G17b or any GitHub
push. No key operation, timestamp request, push, or physical burn is implied.
The resolved synchronization decisions include:

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
durable conflict. G1a completed its separate pure-Go binary-delta investigation
under `performance/v0.7-g1a/`: 21 exact deterministic fixture round trips and
external decoding evidence support a constrained RFC 3284 default-table
profile from a real named parent, with complete-object fallback. Its strict
decoder limits and rejects unsupported features; it is not live sync code and
does not replace the G7 line/word merge.

G2 completed its aggregate-only bounds investigation under
`performance/v0.7-g2/`. At 10,000 generated operations, NCB1 was 51.9% smaller
raw and 16.3% smaller after deterministic gzip than canonical JSONL, with lower
desktop decode time and allocation. The numeric envelope/resource/pack bounds
above and a per-peer pending ceiling of 10,000 operations/64 MiB disk-backed
encoded bytes are inputs to G5/G8/G9, not current runtime controls. FastCDC is
deferred because static corpora contain no repeated binary-edit trace. Android
limits remain provisional through the v0.8 emulator pass and require post-1.0
physical-device confirmation.

G18 also records a framework-neutral Go application facade and pre-1.0 C ABI
handoff. That ABI adds lifecycle, ownership, typed-error, cancellation/event,
capability, and bounded-stream contracts around the same application semantics;
it does not add ordinary REST routes or put HTTP inside the shared library. See
`FLUTTER_GO_CLIENT.md`.

External relay and BLE transport questions are intentionally deferred until the
REST/directory protocol, threat model, and retention behavior are stable.
