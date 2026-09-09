# Native archive v2 format and identity contract

Status: format and read-only verification implemented in v0.4 P2; streaming
export implemented in v0.4 P3; the large-library container revision (object
index, two-level fanout, bounded writer and verifier) implemented in v0.4 P3a;
the optional packed object layout in v0.4 P3b; verify-only and restore/import
in v0.4 P4; and the publication projection in v0.4 P7. Archive v1 remains
supported as human-readable interchange and is not interpreted as v2.

Scalability status (2026-08-23): the semantic/verification contract remains
implemented. G14c implements the separate physical full-snapshot representation
selected by G14b, and G14d carries it through deterministic sequential USTAR
and authenticated NBK1 frames for REST/directory catch-up and crash-safe
restore. This replaced G14's loose-object stored-ZIP producer without changing
archive-v2. G14e passed the production full-scale acceptance and freezes
`sqlite-image+packed-assets.v1` as the compatible whole-library default.
Archive-v2 remains the portable subset/merge/fork/interchange/fallback contract;
G19 publishes that external contract after the sync-era format freeze.

G14a's generated 100k calibration confirmed the physical issue while preserving
the semantic contract: loose export produced 100,093 files; stored ZIP added
11.15%, while authenticated framing added only 6,004 bytes; open/restore stayed
below 92 MiB and the canonical content fingerprint matched. G14b's full-corpus
matrix selected a separate required `sqlite-image+packed-assets.v1` capability
for compatible same-schema whole-library backup/catch-up. G14c implements that
capability as `notriosctl snapshot create|verify`: a sanitized SQLite Online
Backup image, deterministic bounded uncompressed USTAR packs, and a strict
manifest-last verifier. Archive v2 remains
the record-level subset, merge, schema-independent interchange, and fallback
format; the physical image capability does not reinterpret or remove it.

## Purpose and boundaries

Archive v2 is the lossless, versioned snapshot container for backup, transfer,
publication handoff, and the future v0.7 full-sync bootstrap. It carries
canonical records and immutable content objects. It never carries SQLite
pages/WAL files, FTS5 rows, Recoll indexes/projections, caches, rendered sites,
quarantine files, or active import/projection jobs.

The physical `notrios-sqlite-image` v1 format is complementary, not archive-v3
and not a reinterpretation of this manifest. It is admitted only at exact
schema/application compatibility for whole-library replacement/catch-up. An
incompatible reader must refuse it and direct the user to archive-v2. Subset,
merge, publication, and schema-independent interchange remain archive-v2
operations.

## Published consumer contract and compatibility preflight

The machine-readable external contract lives under `contracts/archive-v2/` and
is copied to `/contracts/archive-v2/` by the documentation-site build. It
contains strict Draft 2020-12 schemas for manifests, index entries, all twelve
record payloads, pack trailers, and physical-manifest identification; a
capability/limit registry; and sanitized deterministic loose, packed, and
schema-27 sync-era goldens. The three positive goldens are independently
generated rather than exported and must pass the production verifier.

Run declaration-level admission before opening archive objects:

```sh
notriosctl compatibility archive-v2 --reader current-v2 ./archive
notriosctl compatibility archive-v2 --reader previous-loose-v2 ./archive
```

The command emits JSON and exits 0 only when the selected frozen reader profile
supports the declared archive format, schema minimum, and every required
capability. A refusal is also emitted as JSON and exits 1. Unknown required
capabilities refuse; unknown bounded optional capabilities do not. The
historical `previous-loose-v2` profile is a frozen capability set at commit
`5ae93df8e14880a6f83cf20014bddcb4e9079f1a`, not a claim that an old binary was
executed.

Compatibility acceptance is not integrity verification. It deliberately opens
no index, pack, blob, record, or SQLite image. A consumer must still run its
complete archive-v2 verifier (or `notriosctl verify archive-v2`) before import
or restore. JSON Schema likewise proves structure, not duplicate-key safety,
global ordering, hash chains, path safety, cross-record references, totals, or
filesystem completeness.

`notrios-sqlite-image` v1 always produces a portable-consumer refusal with the
declared `notrios-archive-v2` fallback. It is not an archive-v2 capability and
must never be opened as archive-v3. G9's NCB1/NEV1/NAR1 protocol-1.0 wire
formats are separate again; the schema-27 golden proves the sync era did not
add sync-wire records to archive-v2. Production NAR1 sealing uses fresh entropy,
so only G9's independent NCB1/NEV1 vectors are published as deterministic.

Export, verification, and restore are local CLI operations. No REST or MCP
surface accepts an archive path, streams archive bytes, or performs canonical
archive writes. `internal/archivev2.VerifyDirectory` must accept the complete
archive before `internal/archivev2.Restore` begins its first transaction.

## Identity

Schema v12 persists one `database_identity` row:

- `database_id` identifies the logical database/synchronization universe. It
  is random and stable; it is never derived from a path, profile, hostname, or
  SQLite filename.
- `replica_id` identifies one writable copy. A clone or restored writable copy
  must explicitly rotate it even when it preserves `database_id`.
- `profile_id` remains a future local routing identity and is not archived.

Raw filesystem copies cannot announce that they are clones. Supported
replace/adopt/fork and clone workflows must call the identity rotation
operation before writes. Merge is an import into the existing writable replica
and does not impersonate a new copy.

Restore intent is mandatory:

| Intent | Target rule | Resulting database universe |
|---|---|---|
| `replace` | Existing target; full archive only | Archive `database_id`; new replica ID |
| `adopt` | Empty target; full archive only | Archive `database_id`; new replica ID |
| `merge` | Existing target | Target `database_id`; foreign records use conflict/import rules; existing replica ID remains |
| `fork` | Explicit new ID distinct from archive/target | New database universe and new replica ID |

There is no default and no path-based inference. `PlanRestoreIdentity` rejects
ambiguous or inconsistent choices without mutating either side.

## Directory and completion contract

```text
archive/
  objects/sha256/ab/cd/<64-lowercase-hex-sha256>
  manifest.json
```

An object path is exactly the content hash under a two-level fanout. One level
would put every object of a million-note archive into 256 directories holding
tens of thousands of entries each; two levels keeps them near 25 and matches
the layout the canonical asset store already uses. A path's lexical order is
therefore its hash's, which is what lets the writer prune and the verifier
count by merging sorted streams instead of holding a path set.

Objects are immutable regular files; symlinks, traversal, backslashes,
unlisted files, and extra directories are rejected. The dedicated archive
directory contains no temporary files after commit.

The writer publishes every object first and `manifest.json` last. An absent
manifest is always incomplete. The verifier does not infer completeness from
directory timestamps, object count, or a partially written manifest.

P3 (`internal/archivev2.Export`) implements that contract: object and manifest
bytes are staged in a sibling `<destination>.staging` directory, flushed and
fsynced, then renamed into place, so the archive directory never contains a
temporary file. The writer refuses a destination holding anything other than
`objects/` and `manifest.json`, and it removes an existing manifest before
rewriting objects so a partially rewritten archive cannot claim completeness.

Because objects are content-addressed and immutable, resuming is the ordinary
path: re-running an interrupted export reuses every already-published object of
the right size, prunes objects the new manifest does not list, republishes the
manifest, and then verifies the result. A published archive that fails its own
verification has its manifest removed rather than being reported as complete.

## Manifest and object index

The strict JSON manifest has these top-level fields:

- `format: "notrios-archive"`, `version: 2`;
- `commit_sha256`: SHA-256 over the canonical semantic manifest with this field
  omitted;
- `snapshot`: opaque snapshot/database/source-replica IDs, UTC creation time,
  P1 target, `sqlite_read_transaction` consistency, sorted collection IDs, and
  the complete P1 selection-manifest digest;
- `compatibility`: minimum archive reader, source and accepted schema bounds,
  and sorted required/optional capability names;
- `index`: the index chunks, in entry order, each with exact
  hash/size/entry-count/MIME/location;
- `totals`: aggregate object count, byte count, record-chunk count, and blob
  count, so a reader can bound its work before opening a chunk;
- aggregate typed `counts`, which must equal decoded record counts.

**The manifest does not list objects.** Until v0.4 P3a it did, at roughly 645
bytes per descriptor inside a 4 MiB manifest, which capped an archive near
6,500 objects — about 6,400 single-revision notes — and made a real library
unarchivable. The inventory now lives in `index` chunks: LF-terminated
`application/vnd.notrios.archive-v2-index+jsonl` objects whose entries are
globally sorted by object hash across chunks in manifest order.

The checksum chain is unbroken: the manifest commit digest binds each chunk's
hash, and each chunk binds the hash of every object it names. Tampering with a
chunk breaks its own hash; rehashing the chunk breaks the manifest.

Each index entry carries `sha256`, `kind`, `media_type`, `size_bytes`, optional
record counts for record chunks, and a discriminated `location`. Two layouts
exist: `fanout` (one object, one file) is the default, and `pack` is the
opt-in P3b layout described below. `record_counts` is a pointer and is absent
on blob entries; readers treat an absent and an explicitly all-zero value as
identical, because archives written before the field became optional spell it
the second way.

The layout is named rather than assumed because measurement showed one file
per object is the wrong long-term storage shape: a full backup of a real
382,206-note corpus wrote 382,407 loose objects in 48m26s — about 131 objects
per second for 1.14 GB — since throughput tracks one
`create + write + fsync + rename` per object rather than bytes, and v0.7 sync
would pay one transport round trip per object over REST or an ephemeral
directory carrier.
P3b added the `pack` layout behind the optional `objects.pack.v1` capability,
which was possible precisely because `location` is discriminated rather than
assumed.

A packed entry carries `pack_sha256`, `offset`, and `length` instead of a path;
its own SHA-256 still identifies it, so placement can never launder content.
Pack containers are `kind: "pack"` entries stored under the ordinary fanout,
and each ends with a self-describing trailer listing its contents and offsets,
so a pack is verifiable without the archive index. An archive that stores
objects in packs must declare `objects.pack.v1` as required, so a reader
without pack support rejects rather than misreads it.

Measurement decided the default. On the real 382,206-note corpus packing cut
382,447 files to 46 and 48m26s to 37m28s — 1.29×, well short of the order of
magnitude the fsync hypothesis predicted, because reading and hashing every
revision body dominates and both layouts pay it. Packing also costs about 11%
more disk, because a packed writer cannot use the object tree as its
deduplication index. Loose therefore stays the default and `--pack` is opt-in;
the file-count collapse is what the planned REST and ephemeral-directory sync
transports need. An interrupted packed export restarts rather than resumes.
See `performance/v0.4-p3b/`.

G2 does not change this snapshot format or its JSONL records. Its incremental
sync candidate uses compact NCB1 operation records behind a future G9
capability, while reusing archive-v2 object identity and pack structure. For
sync carriers G2 provisionally targets smaller 64 MiB/4,096-object packs and a
4 MiB trailer ceiling because the current reader loads the trailer whole;
ordinary desktop archive exports retain their existing 256 MiB/65,536-object
targets. G9 must preserve explicit capability rejection and loose-layout
compatibility.

Required v2 capabilities are `identity.database-replica.v1`,
`objects.index.v1`, `objects.sha256.v1`, `records.jsonl.v1`, and
`revisions.complete.v1`. Unknown required capabilities reject the archive.
Unknown bounded optional capabilities may be ignored. Unsupported archive
versions reject before objects are admitted.

A reader older than `minimum_schema_version` rejects the archive: it cannot
interpret what the archive contains. A reader **newer** than
`maximum_schema_version` does not. `maximum_schema_version` records the newest
schema known to read the archive *when it was written*, and an export cannot
know which schemas will exist later; treating it as a ceiling made every schema
bump retroactively invalidate every archive already on disk, including backups
that were still perfectly readable. Genuine incompatibility is expressed with a
required capability or a format version — both of which a reader does refuse —
rather than by the source schema number.

## Objects and records

Three object kinds exist:

- `blob`: exact body/resource/source-bundle bytes with a canonical MIME type;
- `records`: LF-terminated
  `application/vnd.notrios.archive-v2-records+jsonl` chunks. Each line is a
  strict `{ "type", "payload" }` envelope.
- `index`: LF-terminated
  `application/vnd.notrios.archive-v2-index+jsonl` chunks listing every `blob`
  and `records` object. Index chunks are named by the manifest rather than by
  the index, so the inventory never lists itself.

Record types cover:

| Type | Canonical content |
|---|---|
| `collection` | Collection identity, capabilities, settings |
| `notebook` | Parent, name/icon/order/builtin state |
| `search_notebook` | Query-backed notebook identity/query/order/builtin state |
| `tag` | Stable tag identity and name |
| `document` | Collection/notebook/current-revision/deletion/timestamps |
| `revision` | Title/metadata/message plus immutable body object reference |
| `document_tag` | Note/tag membership |
| `resource` | Logical metadata plus immutable blob reference |
| `document_resource` | Relation, ordinal, anchor metadata |
| `link` | Full canonical link/resolution record permitted by P1 policy |
| `provenance` | Source/thread/author/time metadata permitted by P1 policy |
| `source_bundle` | Hashed source/item keys, safe relative path, property order, and immutable exact bytes |

The verifier rejects duplicate record identities; missing collections,
notebooks, current revisions, tags, documents, resources, or blob objects;
notebook cycles/depth overflow; MIME/size disagreement; and unreferenced blob
descriptors. FTS5, Recoll, and outbox rows are rebuilt after future restore.

## MIME and privacy rules

MIME values are parsed and canonicalized. Revision `body_mime_type` and
resource `mime_type` must agree with their semantic blob references. One exact
byte object may be reused by records with different semantic MIME types, so an
object descriptor's MIME is a bounded storage hint rather than part of
content identity. No extension or browser result overrides record metadata;
future restore still sniffs and applies resource admission policy before making
bytes available.

The manifest binds the P1 plan digest. Full archives preserve permitted
canonical history/provenance/source bundles. Subset and publication archives
remain explicitly scoped and must never claim to be full backups. Source and
item keys are hashes; source-bundle paths are bounded relative paths, never
absolute local paths.

P3 applies every P1 link and metadata decision while encoding records:

- `full_archive` is the only mode reported as `full_backup`, and only when no
  selector narrowed it and Trash is included. It keeps complete revision
  history, provenance including private `metadata_json`, exact source bundles,
  every notebook/tag, and the builtin search notebooks.
- `subset_transfer` requires a selector, keeps provenance identity but replaces
  private source `metadata_json` with `{}`, exports only notebooks reachable
  from selected notes plus their ancestors and only tags those notes use, and
  omits query-backed search notebooks because their queries describe the whole
  library.
- A link whose target document or resource fell outside the archived selection
  is recorded with the target cleared and `resolution_status: target_excluded`,
  so no record ever names an object the archive does not contain. The count is
  reported.
- `publication_handoff` (P7) is the projection target. It publishes current
  revisions only, no trashed notes, no provenance, no exact source bundles, and
  no search notebooks, and it blanks revision `metadata_json`.
- `plain_text` and `redact` link actions rewrite note bodies. They are available
  to scoped targets and refused for `full_archive`, because a backup must
  reproduce canonical bytes. When they apply, the link records for the rewritten
  links are omitted — a record names the withheld target and quotes the
  surrounding text — and retained links keep offsets shifted onto the published
  body.

Export, verification, restore, and publication are local filesystem operations.
No REST or MCP surface accepts an output path or streams archive bytes; the
read-only `POST /api/v1/selection/plan` review is the only network-visible part
of the publication workflow.

## Restore (P4)

`internal/archivev2.Restore` is the only writer of an archive into a canonical
store, and `notriosctl restore archive-v2 --intent replace|adopt|merge|fork`
is its only surface. Its contract:

- verification completes in full before the first canonical write, so a corrupt
  or inconsistent archive can never leave a partial library;
- both object layouts are read, and the object index is loaded once into the
  temporary indexed spool so a lookup is an indexed query rather than a scan of
  every index chunk;
- records are applied in dependency order — containers, then content, then
  relations — in bounded transactions. Records objects are a small fraction of
  an archive's bytes, so more than one pass over them is cheap next to reading
  blobs once;
- every blob, revision body, and source-bundle item is re-hashed at the point of
  use. Verification is a separate pass over the same files, and bit rot, a
  concurrent writer, or a network filesystem can change an object between the
  two;
- blob admission re-sniffs MIME and applies resource admission policy rather
  than trusting archive metadata;
- exact source bundles are admitted through the source-bundle namespace, never
  as ordinary blobs, so they stay outside `blobs` and resource garbage
  collection and are readable at the path their records name;
- a durable `restore_state` row (schema v13) is written before the first
  canonical row and cleared after the last, including identity adoption. A
  library carrying a marker is not empty and not complete: `adopt`, `merge`,
  and `fork` refuse it and only `replace` recovers it.

The restore path holds no state proportional to the archive. Its dedup and
bundle-path maps use the same bounded spool as the writer and verifier.

## Hard limits

P3a re-derived these from the largest library this build is expected to
archive — 1,000,000 notes with saved revisions, reachable resources, and one
exact source-bundle item per imported source item — rather than from the
original small-archive defaults. The verifier refuses caller limits wider than
them:

- manifest: 4 MiB (it now holds only index descriptors);
- objects: 8,000,000; index chunks: 1,000 of at most 10,000 entries each;
- one index entry: 4 KiB; one index chunk: 64 MiB;
- decoded records: 64,000,000 total, 10,000 per records object;
- one JSONL record: 1 MiB; one records object: 16 MiB;
- one blob: 16 GiB; archive object bytes: 4 TiB;
- archive/source relative path: 255 bytes and 8 segments;
- JSON nesting: 32; notebook nesting: 32;
- collections: 1,000; notebooks: 1,000,000;
- capabilities: 64 names of at most 128 bytes.

P3 may create more/smaller chunks but may not widen these limits silently.
P4 reuses the same indexed verification spool rather than reintroducing
in-memory record sets; admission semantics are identical for export,
verification, and restore.

### Bounded writer and verifier

Neither side keeps state proportional to the archive.

The writer needs no object table: the published object tree *is* the
deduplication index, since an object exists exactly when its
content-addressed file does. Index entries stream into a 256-bucket
external-sort spool keyed by the object hash — the same first byte that
selects the fanout directory — so replaying buckets in order yields globally
sorted entries while holding one bucket. Duplicate hashes collapse during that
replay, and pruning merges the sorted expected-path list against the sorted
object tree.

The verifier keeps only the two genuinely bounded sets: collections
(`MaxCollections`) and the notebook tree (`MaxNotebooks`, needed for cycle and
depth checks). Every record identity and object hash goes to declaration and
reference spools that are merge-joined per bucket. Composite keys fold
consistency checks into that join: a revision key carries its document ID, and
a blob key carries its exact byte length, so a wrong owner or a wrong size
simply fails to find a declaration. The join reports references with no
declaration, duplicate declarations, and blob objects nothing references.

"No extra files" is proven by counting rather than by a path set: every
declared object is opened during the index scan, so an equal file count leaves
no room for an unlisted one.

## Golden and hostile fixtures

`internal/archivev2/testdata/golden-minimal/` is a complete manifest-last
archive containing all twelve record types plus body, resource, and source
bundle objects. It is built by a generator that does not use the exporter, so
a writer bug cannot become the expected result, and a test asserts the
committed fixture equals what that generator produces byte for byte
(`NOTRIOS_UPDATE_GOLDEN=1` rewrites it after a deliberate format change).

Tests copy and mutate it to prove rejection of missing manifests/objects,
corruption of an object or of an index chunk, unsupported
version/schema/capability, manifest checksum drift, inconsistent counts and
object totals, index entry-count drift, traversal, symlinks, extras, invalid
MIME, unknown/duplicate JSON fields, excessive count/depth, and unsafe
relative paths. The fixture is synthetic and contains no private note data.
