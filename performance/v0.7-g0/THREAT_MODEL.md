# Synchronization threat model

## Overview

Notrios is a single-user, local-first notes system. Each writable installation
keeps a canonical SQLite database and asset store; synchronization exchanges
immutable operations and content objects between explicitly enrolled replicas.
Ephemeral directories, removable drives, cloud-backed folders, and REST are
carriers only. They are assumed able to observe routing metadata and to delete,
delay, duplicate, reorder, rename, truncate, or inject artifacts.

The core security promise planned for protocol v1 is:

> A replica changes canonical state only after a bounded artifact is associated
> with the intended database/protocol, attributed to a currently acceptable
> enrolled replica, authenticated and decrypted under an acceptable key epoch,
> replay/dependency checked, content verified, and applied by deterministic
> merge rules in one canonical transaction.

G0 freezes that promise and the supporting controls. It does not implement
them. Current v0.6 REST has no general authentication and remains approved only
for loopback/local use. Future sync-route authentication and TLS requirements
do not make today's listener suitable for public deployment.

## Threat Model, Trust Boundaries, and Assumptions

### Security objectives

1. **Universe isolation:** data from one database cannot be applied to another
   without an explicit restore/adopt/fork decision.
2. **Confidentiality:** carriers, curious providers, and unauthenticated REST
   clients cannot read sync payloads; visible metadata is minimized and
   documented.
3. **Artifact integrity and attribution:** accepted control artifacts and
   envelopes are bound to canonical bytes and a currently acceptable enrolled
   replica; objects match their exact declared bytes.
4. **Replay and ordering safety:** duplication, replay, reordering, gaps, stale
   key epochs, and retired identities cannot move applied state backward or
   bypass dependencies.
5. **Convergence without silent loss:** valid eventually delivered operations
   converge deterministically, while meaningful conflicts and repairs remain
   recoverable and visible.
6. **Bounded work:** hostile counts, sizes, nesting, compression ratios,
   dependencies, retries, requests, and concurrent transfers cannot consume
   unbounded memory, disk, CPU, connections, or time.
7. **Recoverability:** deletion/withholding by a carrier, local corruption,
   password errors, and replica loss have explicit recovery paths that never
   silently weaken authentication.
8. **Secret containment:** private signing keys, recipient/content keys,
   passwords, REST credentials, and pairing secrets do not enter carrier
   plaintext, manifests, process arguments, logs, job records, or repository
   evidence.
9. **Auditability:** security-relevant membership, rejection, recovery, and
   destructive decisions have stable, content-redacted audit events.

### Assets and consequences

| Asset | Required property | Consequence if lost |
|---|---|---|
| Canonical notes, metadata, revisions, tasks, links, and notebook structure | Confidentiality, integrity, recoverability | Private disclosure, silent edit/deletion, misleading merged state |
| Resource and source-bundle bytes | Confidentiality, exact integrity, availability | Private attachment disclosure, substituted content, broken notes |
| Operation history, tombstones, conflict revisions, and state vectors | Integrity, monotonicity, bounded retention | Replay acceptance, resurrection, premature GC, non-convergence |
| Database/replica/key identities and enrollment state | Uniqueness, authenticity, durability | Cross-database merge, clone collision, impersonation, stale-key acceptance |
| Signing private keys | Confidentiality and controlled use | Attacker can attribute valid artifacts to the replica until revocation |
| Encryption recipient/content keys and key epochs | Confidentiality, freshness | Carrier or revoked device can read protected payloads; future secrecy fails |
| Passwords and KDF parameters for encrypted snapshots | Password secrecy; parameter integrity and portability | Offline guessing, unrecoverable backup, resource-exhaustion attack |
| REST/pairing credentials | Confidentiality, route/database scope, expiry | Unauthorized transfer, enumeration, quota abuse, enrollment attempt |
| Audit and recovery records | Integrity, ordering, privacy | Compromise hidden, wrong recovery choice, content/path leakage in logs |
| Service availability and local disk budget | Bounded consumption, graceful failure | Foreground editing blocked, disk filled, repeated crash loop |

### Adversaries and capabilities

| Adversary / failure | Can do | Cannot be assumed to do |
|---|---|---|
| Lost or hostile shared directory / USB | Read visible names/headers; delete, replace, replay, reorder, duplicate, truncate, rename, withhold, or partially copy files | Preserve atomicity, timestamps, ordering, uniqueness, confidentiality, or availability |
| Curious cloud/provider/operator | Observe account, endpoints, timing, frequency, sizes, visible routing and ciphertext equality; retain old bytes | Read properly encrypted payloads or forge signatures without keys |
| Unauthenticated/malicious REST client | Probe routes, send malformed/large/replayed requests, race transfers, consume quota, lie about capabilities | Gain sync authorization merely by reaching the service or presenting a carrier object ID |
| Compromised active replica | Read its available plaintext/keys; create correctly signed and encrypted operations; delete local state; leak content | Be distinguished cryptographically from the legitimate user while its key remains active |
| Revoked or retired replica | Retain every key and artifact it possessed; replay old valid artifacts; request old data | Learn later key epochs or regain quorum status without explicit re-enrollment |
| Copied database / duplicate replica ID | Produce two writers with identical historical identity and sequence space | Safely allocate new operations without clone detection and an adopt/fork decision |
| Offline password guesser | Copy encrypted snapshot and try arbitrary passwords/KDF implementations | Be rate-limited by a remote service once the archive is obtained |
| Resource-exhaustion attacker | Craft high counts, deep dependencies, huge declared sizes, compression bombs, slow/range abuse, hash collisions attempts | Force parsing/decompression/application before fixed admission gates |
| Local unprivileged process/user | Read world-readable files, process arguments, logs, or insecure temp files; race writable carrier paths | Bypass correctly configured OS account/keyring protections |
| Accidental operator/environment failure | Lose password/device, restore stale backup, copy live SQLite file, misconfigure target, interrupt writes | Follow warnings, eject media safely, or preserve a complete carrier |

### Trust boundaries

| Boundary | Less-trusted side | Trust decision / validation required |
|---|---|---|
| User/UI → enrollment and recovery controller | QR/file/link input, display names, requested restore intent | Explicit confirmation, database/key fingerprint display, one-use/expiry, no secret in URL/history, clear destructive effect |
| Secret store → sync crypto | OS vault/provider and retrieved opaque secret bytes | Injected provider interface, correct profile/replica/key binding, locked permissions, no silent plaintext fallback |
| Canonical store → replication journal | Existing mutation paths and imported untrusted content | Journal allocation in the same transaction, stable IDs, deterministic operation projection, no raw SQL from protocol input |
| Replica → carrier | Local canonical artifact bytes | Encrypt all payloads, bind minimal visible header as AEAD associated data, sign canonical outer artifact, publish immutable bytes/manifest last |
| Carrier → replica staging | Arbitrary names and bytes | Ignore unknown/temp entries; lexical safe path handling; count/size/type/version/database/key/signature/replay gates before decrypt/decompress; quarantine until verified |
| Decrypted artifact → decoder/decompressor | Authenticated but possibly malicious enrolled-replica input | Compressed and expanded limits, nesting/count limits, strict canonical encoding, no executable content, bounded dependencies |
| Replication apply → canonical SQLite/assets | Validated operations and objects | One deterministic transaction, optimistic/invariant checks, object hash/MIME checks, atomic blob publication, auditable conflicts/repairs |
| REST client → sync artifact service | Network requests, credentials, opaque artifact IDs, ranges | Non-loopback TLS, route/database-scoped auth, pairing state, quotas/rate/concurrency/size limits, no arbitrary paths, generic errors |
| Sync/core → current ordinary REST/MCP | Sync-specific authority | Separate routes/credentials; sync credentials authorize no ordinary note API; current MCP scope remains a guardrail, not authentication |
| Archive/snapshot → restore | Encrypted and untrusted container | Verify entire bounded manifest/object chain, authenticate/decrypt, explicit replace/merge/adopt/fork intent, emergency backup, mint identity when required |
| Audit/logging → operators/support | Potentially attacker-controlled errors and identifiers | Stable event codes and redacted/truncated IDs; omit note text, secrets, tokens, URLs with credentials, and local paths from remote/MCP views |

### Assumptions and non-goals

- The local operating-system account and device secret store are a trust root.
  A fully compromised OS can read plaintext and use active keys; sync crypto
  cannot protect against that while the application is running.
- v0.7 does not encrypt the canonical SQLite database or asset store at rest.
  SQLCipher is a separate future decision.
- v0.7 is one user with several devices. It does not provide multi-user roles,
  per-field access control, or non-repudiation against the database owner.
- Ed25519 provides protocol attribution, not proof that a human intentionally
  approved each operation and not secrecy.
- AEAD provides confidentiality/integrity to key holders, not freshness,
  authorization, or availability.
- SHA-256 content addresses provide exact-byte integrity/deduplication, not
  writer authentication or authorization.
- The carrier may permanently lose all artifacts. Availability requires another
  replica or verified backup; no protocol can recover the last remaining copy.
- Metadata confidentiality is best effort within the documented visible tuple.
  Traffic analysis, account identity, timing, volume, and endpoint observation
  remain possible without an anonymity/padding system, which is out of scope.
- Historical plaintext or keys already obtained by a compromised replica cannot
  be revoked retroactively. Rotation protects only artifacts under later
  epochs; re-encrypting all history is a separate expensive operation.
- No SVN dump compatibility, rclone merge/deletion semantics, Nostr/BLE
  transport, or live character-CRDT editing is part of protocol v1.

### Trust roots, enrollment, revocation, and recovery

The database membership record is the trust root for remote replica identities.
An enrollment transaction binds `database_id`, new `replica_id`, Ed25519 public
key and key ID, encryption recipient/key material, approved capabilities,
creation epoch, and active status. It is user-approved through a one-use,
expiring pairing flow that displays database and key fingerprints. Merely
finding an advertisement, directory name, copied database, or REST endpoint
never enrolls a peer. G4 owns the exact exchange and proof-of-possession flow.

Private signing keys and recipient secrets live behind an injected platform
secret-store interface and are referenced by local profile configuration; they
are not synchronized as ordinary database content. Signing and encryption keys
are independent so either role can rotate without changing the other.

Revocation is a durable signed/membership operation. Receivers reject artifacts
whose key was not active for the artifact's acceptable protocol position.
Compromise revocation also creates a new encryption key/recipient epoch and
distributes it only to remaining active peers. The exact group/per-recipient
construction belongs to G9, but it must provide this property. Retirement also
removes the peer from future acknowledgement quorum after the durable retirement
floor is retained. Returning retired peers re-enroll and snapshot-catch-up;
they do not reuse old identity or demand collected history.

Recovery never bypasses authentication:

- lost carrier: reconstruct advertisements/envelopes from canonical journals or
  catch up from an active peer/snapshot;
- lost replica: restore a verified snapshot or enroll a fresh replica and catch
  up, minting a new identity;
- compromised replica: revoke, advance encryption epoch, preserve audit and
  recovery evidence, inspect operations attributed to it, and restore/merge as
  needed;
- lost snapshot password: no backdoor or escrow is implied; explain that
  recovery is impossible unless another replica, backup, or recorded recovery
  method exists;
- copied database: refuse new writes under duplicate identity until the user
  explicitly chooses adopt/fork/replace and a safe replica identity plan;
- far-behind peer: verified snapshot to a named state-vector boundary, then
  incremental operations after that boundary.

### Artifact authentication and admission order

G9 must specify exact bytes, but the order and coverage objectives are frozen:

1. Parse only a small fixed/versioned outer header with hard byte limits.
2. Require artifact type/domain, protocol version, database ID, source replica
   and signing-key ID, encryption epoch, artifact ID, declared ciphertext size,
   and nonce/algorithm identifiers. Reject unsupported/unknown critical fields.
3. Verify the Ed25519 signature over domain-separated canonical outer artifact
   bytes, including visible routing metadata and ciphertext commitment. Reject
   inactive/revoked/retired keys according to recorded membership position.
4. Check replay/duplicate/stale epoch and resource budgets before expensive
   work. Duplicate exact artifacts are idempotent; conflicting bytes for one ID
   are quarantined and audited.
5. AEAD-open with the visible header bound as associated data. There is no
   plaintext fallback or algorithm negotiation chosen by the sender.
6. Enforce compressed-byte, expanded-byte, ratio, object-count, nesting, and
   dependency limits while decoding/decompressing; never allocate from an
   untrusted declared size without a bound.
7. Verify inner canonical type/database/replica/sequence/dependency metadata and
   exact object hashes/MIME before admission.
8. Apply only contiguous/dependency-ready operations in a canonical transaction;
   quarantine bounded pending gaps and request missing data without advancing
   the state vector.

Advertisements, requests, acknowledgements, envelopes, and snapshot manifests
are payload-encrypted and replica-signed. Bulk object/blob bytes are protected
by AEAD/container integrity and exact hashes, then referenced from one of those
signed artifacts; they are not detached-signed a second time merely because
they are large.

### Metadata leakage budget

The G9 format may expose only fields needed to route opaque bytes before
decryption. The preferred visible tuple is protocol magic/major version,
database routing ID or blinded equivalent, artifact class, opaque artifact or
ciphertext content address, source key ID where pre-decryption signature lookup
requires it, encryption epoch/recipient routing, ciphertext length, nonce, and
signature. Exact necessity must be justified field by field.

| Observer can still learn | Must remain encrypted |
|---|---|
| Provider account and endpoint; access times/frequency; ciphertext and aggregate sizes; number of artifacts; coarse source/key/epoch linkage needed for routing; ciphertext equality where content addressed | Note/resource plaintext, titles, tags, notebooks, authors/provenance, plaintext hashes, filenames/MIME where avoidable, full replica state vectors, operation kinds/targets, conflict and deletion facts, peer display names, request ranges, acknowledgement positions |

Padding, cover traffic, anonymity, ORAM, and hiding whether two accesses use the
same cloud account are out of scope. The UI/security documentation must say so.

### Audit events

Planned audit records use stable event codes, database-local monotonic order,
timestamp as observation metadata, actor replica/key IDs, result, and bounded
reason codes. They never store secret/key/password/token bytes or note/resource
content. Remote/MCP views omit local paths and attacker-supplied free text.

Required event families:

- enrollment requested/approved/refused/expired and proof-of-possession failure;
- signing or encryption key creation/rotation, key-epoch advance, revocation,
  retirement, and re-enrollment;
- artifact accepted, duplicate-idempotent, quarantined, or rejected for
  database/version/type/key/signature/AEAD/replay/gap/hash/size/count/ratio;
- clone/duplicate-replica detection and adopt/fork/replace choice;
- snapshot requested/authorized/published/downloaded/verified/decrypted,
  password failure without password data, restore intent and result;
- peer acknowledgement/retention-floor changes and dry-run/apply GC decisions;
- sync target/config changes, plaintext development mode enabled/refused, REST
  auth/quota/rate-limit failures, and repeated carrier corruption/withholding;
- conflict creation/resolution and structural repair attribution.

High-volume success events may be aggregated by bounded range, but security
failures and membership/recovery decisions remain individually attributable.

## Attack Surface, Mitigations, and Attacker Stories

The complete control trace is in `MISUSE_CASES.md`. The highest-risk stories
are summarized here.

1. **Compromised active device sends a valid mass deletion.** Signatures pass
   because the key is legitimately enrolled. Planned mitigation is rapid
   revocation plus key-epoch advance, attributed operation history, retained
   tombstones/revisions/conflicts, acknowledgement-gated GC, dry-run destructive
   maintenance, and explicit restore/merge. The protocol cannot infer the human
   did not intend the deletion.
2. **Carrier replays a pre-revocation envelope after local compaction.** The
   receiver binds database/type/key/epoch, retains replica sequence and
   retirement/replay floors beyond payload GC, treats exact duplicates as
   idempotent, and refuses conflicting/stale sequence identities.
3. **Provider replaces ciphertext routing metadata.** The signature covers the
   canonical outer artifact and AEAD associated data binds the same visible
   header, so changing database/source/type/epoch/size/nonce fails before apply.
4. **Copied SQLite database writes as the same replica from two machines.** G3
   detects profile/path/replica collisions and G5's sequence allocation refuses
   ambiguous continuation. The user selects adopt/fork and a new replica ID;
   “last writer wins” must not hide the collision.
5. **Small compressed artifact expands until the service is killed.** Admission
   checks compressed bytes first, then streams through hard expanded-byte,
   ratio, object/count/depth/time limits into quarantine. Decompression never
   precedes signature/AEAD verification where the format permits.
6. **Malicious REST caller enumerates or downloads artifacts.** Sync routes use
   database/route-scoped credentials, opaque IDs, non-loopback TLS, pairing
   state, constant/generic existence errors, range and concurrency limits, and
   no arbitrary path. Payload encryption remains mandatory even after REST auth.
7. **User loses the only backup password.** The UI warns before creation,
   verifies a test decrypt/manifest path, offers safe recovery-copy guidance,
   and stores no password. There is deliberately no recovery bypass.
8. **Hostile carrier deletes everything.** Local canonical state and journals
   remain authoritative; artifacts are re-advertised/reconstructed. If all
   canonical replicas and backups are also gone, availability is unrecoverable.

Residual systemic risks are active-replica compromise, local OS compromise,
traffic analysis, permanent loss of every canonical/backup copy, weak user
passwords against offline attack, and denial by a carrier that withholds all
traffic. Later slices must document them rather than claim cryptography solves
them.

## Severity Calibration

### Critical

- Unauthenticated or cross-database remote operations can modify canonical
  state at scale with ordinary/default configuration.
- Sync protocol or restore path permits arbitrary code execution, arbitrary
  filesystem write/read, or extraction outside controlled roots.
- Private signing/encryption keys or backup passwords are exposed in carrier
  plaintext, remote responses, logs, process arguments, or release artifacts.
- A carrier/provider without enrolled keys can decrypt a complete database or
  forge accepted operations without local compromise.

### High

- A hostile carrier can cause silent accepted note/resource substitution,
  rollback, resurrection, cross-universe merge, or premature irreversible GC.
- Revocation leaves a compromised replica able to decrypt or authenticate
  future-epoch traffic without re-compromise.
- Default-bounded input can reliably exhaust disk/memory/CPU or corrupt the
  canonical database before verification.
- A remotely reachable sync route lacks effective authentication, database
  scope, or TLS enforcement for non-loopback use.

### Medium

- A bounded malicious artifact causes repeatable service interruption but not
  canonical corruption or unbounded resource loss.
- More metadata than the documented routing budget leaks to a carrier, such as
  plaintext hashes, operation kinds, full state vectors, filenames, or peer
  display names.
- Audit/recovery gaps materially delay detection or make an attributed conflict
  difficult to recover, while normal revisions/backups remain intact.
- Explicit development plaintext mode can be accidentally enabled on loopback
  production data but cannot bind non-loopback or interoperate with production.

### Low

- Security/status messaging is misleading or omits a safe recovery action
  without itself weakening enforcement.
- Non-sensitive coarse metadata or bounded diagnostic detail exceeds the
  intended redaction but does not reveal content, secrets, paths, or tokens.
- Defense-in-depth validation is missing where an earlier independent control
  reliably rejects the same input and no practical bypass is shown.

Severity assumes the planned v1 deployment: a single user, explicitly enrolled
devices, local canonical storage, and carriers/REST treated as hostile. A valid
active peer intentionally changing its own user's data is a trust-model event,
not automatically a protocol vulnerability; bypassing revocation, audit,
retention, or recovery controls can still make the resulting finding High.
