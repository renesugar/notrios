# Protocol glossary

These meanings are normative for v0.7 planning and implementation. A later
format specification may refine fields, but it must not silently reuse these
words for different concepts.

| Term | Normative meaning | Not this |
|---|---|---|
| **Profile** | Local, non-synchronized installation configuration that selects one database and its paths, listen/public addresses, local replica, sync target, and secret references. A profile may point at a cloned database, which is why profile ID is not a shared identity. | An MCP tool scope, a database, or an enrolled peer. |
| **Database** | One logical synchronization universe, identified by a stable `database_id`, containing canonical notes, revisions, resources, operation history, peer state, and policy. Multiple writable replicas may hold it. | A SQLite pathname, profile, carrier directory, or individual SQLite connection. |
| **Replica** | One writable copy/installation participating in a database universe, identified by a unique `replica_id` and an enrolled signing identity. It allocates its own monotonic operation sequence. A copied database must mint or explicitly adopt/fork identity before writing. | A backup, read-only carrier copy, process, or user account. |
| **Peer** | Another enrolled replica as represented in a local replica's membership state, including public keys, capabilities, status, acknowledgement vector, and retirement/revocation facts. “Peer” is relational; the local installation is itself a replica. | Any discovered directory name or unauthenticated REST client. |
| **Carrier** | A transport mechanism or location that stores or moves opaque protocol artifacts: ephemeral directory, removable drive, cloud-backed directory, or REST artifact service. It may be hostile and is never canonical. | A peer, database, merge engine, trust root, or retention authority. |
| **Snapshot** | A verified, internally consistent native archive-v2 state image bound to a database identity, schema/capability set, and state-vector boundary. It supports bootstrap, catch-up, backup, or explicit restore intent. | An acknowledgement, live database-file copy, peer, or proof that later operations were received. |
| **Envelope** | An immutable, bounded protocol artifact carrying a canonical batch of operations plus identity, ordering, dependency, version, crypto, and object-reference metadata. Its payload is authenticated-encrypted and its artifact bytes are signed by the source replica. | A mutable mailbox, raw SQL transaction, or unsigned ZIP/file. |
| **Object** | Immutable content-addressed protocol bytes referenced by an envelope or snapshot, such as a complete note body, optional delta, resource/chunk, records chunk, or pack. Admission requires type/count/size checks and exact hashing. | An authenticated principal; a content hash alone does not authorize an object. |
| **State vector** | A database-scoped map from replica ID to the highest **contiguous** applied operation sequence for that replica, qualified by retained history/retirement rules. It summarizes durable apply state and is not advanced across gaps. | A wall-clock timestamp, list of every operation, carrier inventory, or proof of authorization. |
| **Advertisement** | A signed, encrypted control payload by which a replica announces bounded available envelope/snapshot ranges, capabilities, and current state to already enrolled peers. Only minimal routing metadata is visible outside encryption. | Automatic enrollment, an unsigned directory listing, or an acknowledgement. |
| **Request** | A signed, encrypted, bounded control payload asking an enrolled peer/carrier for named missing ranges, objects, or a snapshot. Requests are idempotent and cannot name arbitrary server filesystem paths. | Authorization to join a database, a merge instruction, or proof that bytes were applied. |

## Related security terms

| Term | Meaning |
|---|---|
| **Acknowledgement** | A signed, encrypted control artifact reporting a peer's contiguous durable state vector. It is a retention input, not permission to delete immediately. |
| **Snapshot manifest** | The signed, encrypted completeness record published last for a snapshot, binding object inventory, exact hashes/sizes, database/schema/capabilities, state-vector boundary, and recipient/key epoch. |
| **Enrollment** | An explicit user-approved transition that binds a database, new replica ID, signing public key, encryption recipient/key material, capabilities, and initial status. Discovery alone never enrolls. |
| **Revocation** | A durable database-scoped fact that a signing/encryption identity is no longer accepted. Compromise revocation also advances the encryption epoch for remaining active replicas. |
| **Retirement** | An explicit, audited removal of a peer from acknowledgement/retention quorum. A retired peer returning later must be re-enrolled and normally perform snapshot catch-up. |
| **Key epoch** | A monotonically identified recipient/encryption generation. Artifacts bind their epoch; active peers receive new epoch material after membership or compromise changes according to policy. |
| **Content address** | A cryptographic digest used to name immutable exact bytes. It detects mismatches but does not establish who created the bytes or whether they are authorized. |
| **Canonical bytes** | The single deterministic serialization for an artifact type, including a domain/type/version context. Signature verification must reject alternate encodings rather than normalize attacker-controlled ambiguity after the fact. |
| **Database universe** | All states and histories intentionally sharing one `database_id`. Similar document IDs or copied SQLite paths do not authorize two universes to merge. |

## Identity invariants

- A profile ID is local; a database ID is shared; a replica ID is unique within
  the database; a signing key ID identifies one enrolled key generation.
- One running writable profile resolves to exactly one local replica identity.
- A backup/snapshot is not a replica and never blocks garbage collection.
- A carrier path or REST endpoint can serve several database universes, but
  artifacts are cryptographically bound to exactly one.
- Display names are UI labels, never protocol identity or authorization.
