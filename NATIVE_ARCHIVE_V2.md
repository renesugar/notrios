# Native archive v2 format and identity contract

Status: format and read-only verification implemented in v0.4 P2. Streaming
export is P3; verified restore/import is P4. Archive v1 remains supported as
human-readable interchange and is not interpreted as v2.

## Purpose and boundaries

Archive v2 is the lossless, versioned snapshot container for backup, transfer,
publication handoff, and the future v0.7 full-sync bootstrap. It carries
canonical records and immutable content objects. It never carries SQLite
pages/WAL files, FTS5 rows, Recoll indexes/projections, caches, rendered sites,
quarantine files, or active import/projection jobs.

P2 exposes no REST/MCP filesystem operation and performs no canonical writes.
`internal/archivev2.VerifyDirectory` must accept the complete archive before a
future P4 restore can begin its first transaction.

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
  objects/sha256/ab/<64-lowercase-hex-sha256>
  manifest.json
```

An object path is exactly the content hash under the two-character fanout.
Objects are immutable regular files; symlinks, traversal, backslashes,
unlisted files, and extra directories are rejected. The dedicated archive
directory contains no temporary files after commit.

The writer publishes every object first and `manifest.json` last. An absent
manifest is always incomplete. The verifier does not infer completeness from
directory timestamps, object count, or a partially written manifest. P3 will
stage privately, flush and verify objects, then atomically publish the final
manifest.

## Manifest

The strict JSON manifest has these top-level fields:

- `format: "notrios-archive"`, `version: 2`;
- `commit_sha256`: SHA-256 over the canonical semantic manifest with this field
  omitted;
- `snapshot`: opaque snapshot/database/source-replica IDs, UTC creation time,
  P1 target, `sqlite_read_transaction` consistency, sorted collection IDs, and
  the complete P1 selection-manifest digest;
- `compatibility`: minimum archive reader, source and accepted schema bounds,
  and sorted required/optional capability names;
- sorted `objects`: exact hash/path/kind/MIME/byte length plus record counts;
- aggregate typed `counts`, which must equal descriptor and decoded counts.

Required v2 capabilities are `identity.database-replica.v1`,
`objects.sha256.v1`, `records.jsonl.v1`, and `revisions.complete.v1`. Unknown
required capabilities reject the archive. Unknown bounded optional
capabilities may be ignored. This reader must fall inside the declared schema
range; unsupported archive versions and schema ranges reject before objects
are admitted.

## Objects and records

Two object kinds exist:

- `blob`: exact body/resource/source-bundle bytes with a canonical MIME type;
- `records`: LF-terminated
  `application/vnd.notrios.archive-v2-records+jsonl` chunks. Each line is a
  strict `{ "type", "payload" }` envelope.

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
absolute local paths. P3 must apply every P1 link and metadata decision while
encoding records.

## Hard limits

The current verifier refuses caller limits wider than these defaults:

- manifest: 4 MiB; objects: 10,000;
- decoded records: 12,000,000 total, 10,000 per records object;
- one JSONL record: 1 MiB; one records object: 16 MiB;
- one blob: 16 GiB; archive object bytes: 4 TiB;
- archive/source relative path: 255 bytes and 8 segments;
- JSON nesting: 32; notebook nesting: 32;
- collections: 1,000; capabilities: 64 names of at most 128 bytes.

P3 may create more/smaller chunks but may not widen these limits silently.
P4 can use an indexed verification spool when large cross-reference sets make
in-memory validation inappropriate; admission semantics remain identical.

## Golden and hostile fixtures

`internal/archivev2/testdata/golden-minimal/` is a complete manifest-last
archive containing all twelve record types plus body, resource, and source
bundle objects. Tests copy and mutate it to prove rejection of missing
manifests/objects, corruption, unsupported version/schema/capability, manifest
checksum drift, inconsistent counts, traversal, symlinks, extras, invalid MIME,
unknown/duplicate JSON fields, excessive count/depth, and unsafe relative
paths. The fixture is synthetic and contains no private note data.
