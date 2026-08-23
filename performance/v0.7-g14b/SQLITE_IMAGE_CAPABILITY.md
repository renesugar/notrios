# Selected SQLite-image capability contract

G14b selected option B on 2026-08-22. This remains a prototype contract, not a
production format: G14c owns implementation and table-by-table review. The
required capability name is `sqlite-image+packed-assets.v1`; it is the default
only for compatible same-schema whole-library full backup/catch-up.

## What the capability would contain

- A consistent SQLite image produced only after a stopped/checkpointed close or
  with SQLite's Online Backup API. A live `.sqlite` copy without its WAL is
  never admitted.
- A versioned manifest binding the exact database SHA-256, application/schema
  compatibility bounds, database identity, snapshot vector/floor, and the
  exact hashes, sizes, counts, and pack locations of resources and preserved
  source bundles.
- Bounded external-object packs. A directory containing one restored file per
  attachment/source item is not the selected transport representation. Stored,
  deterministic packs close before 256 MiB payload or 65,536 entries. One item
  larger than the byte target occupies its own declared oversized pack and is
  streamed within the existing per-resource limits.
- The existing NBK1 authenticated framing and peer/password key wrapping at the
  transport boundary. SQLCipher could protect the image at rest, but it does
  not replace the multi-file manifest, signatures, framing, or compatibility
  checks.
- A required archive-v2 capability declaration plus a semantic archive-v2
  fallback. Record-based export remains the path for subset, merge, portable
  interchange, and a schema transition outside the admitted image range.

G14c adds no new compression dependency. Each pack publishes through a private
partial file, fsync, complete hash/length verification, and atomic rename; the
manifest publishes last. Resume starts only after the last complete verified
pack. An interrupted SQLite image creation restarts rather than claiming page-
level resume. A future compressor requires a new capability version plus
determinism, expansion, decoder-memory, license, and mobile-proxy evidence.

## State classes

The image may preserve canonical content and admitted sync history: documents,
revisions, notebooks, tags, resources and references, provenance/source
bundles, immutable operations and dependencies, state vectors/floors,
convergence registers/conflicts, peer public keys, and database-wide policy.
The database ID is preserved; the installed writable copy always mints a new
replica ID and retires/disconnects the copied local allocator.

The G14b prototype clears these definitely local/transient tables before
binding the image: jobs, batch ledgers, restore markers, pairing invitations,
catch-up sessions, revision-transfer seams, apply/capture guards, and pending
admission bytes. G14c must review that list table by table and decide how to
handle local blob sources/materialization attempts, unfinished importer state,
peer acknowledgements, and staged chunks without losing canonical history or
carrying machine-local paths.

FTS/Recoll projections, parsed links/blocks, snippets, and the index outbox are
derived/local indexing state. An exact-schema image may carry verified SQLite
derived tables for fast recovery, but G14c must provide a bounded rebuild and
must select it whenever the capability or migration invalidates them. Recoll's
external index is never part of the authoritative image.

The replica signing private key and group encryption keys stay in the existing
owner-only secret file and never enter the database image. Copying a database
must not copy the credentials that let its source replica impersonate itself.

## Admission and cutover order

1. Authenticate/decrypt the complete outer artifact into private staging.
2. Verify the manifest, database hash, every external pack hash/length, and the
   required capability chain before opening the image for writes.
3. Run `PRAGMA integrity_check` and enforce the exact declared application/
   schema compatibility range.
4. Verify the snapshot vector/floor and semantic content aggregate; refuse a
   missing external object even when SQLite itself is valid.
5. Stop target writes and produce a separately verified emergency snapshot.
6. Install database and external objects through fsync plus atomic rename under
   a durable replacement marker. Never restore directly into a cloud/FUSE
   provider mapping.
7. Clear reviewed local/transient state, rotate replica identity, establish the
   new snapshot boundary, and rebuild invalid derived state.
8. Reopen the service only after all of the above succeed; incremental replay
   begins strictly after the manifest's snapshot vector.

An interrupted stage preserves the old target and either resumes verified
staging or restarts. A corrupt/truncated artifact, wrong key, incompatible
schema, hash mismatch, failed integrity check, missing external object, or
failed identity rotation never exposes a partially installed library.

## Why this is not Restic/Borg or raw SQLite

Restic and Borg supply excellent content-defined chunking, encryption,
repository indexes, deduplication, retention, and repair/check tooling for
general filesystem history. They cannot know that Notrios can restore one
transactional canonical image, rebuild indexes, rotate a replica, and then
replay operations. A raw filesystem restore must recreate every source file
and directory because those tools cannot substitute application semantics.

Conversely, SQLite alone knows nothing about external resource/source-bundle
bytes, subset/merge semantics, archive capability negotiation, identity
rotation, authenticated transport, or emergency cutover. Option B is viable
only as the complete capability above.
