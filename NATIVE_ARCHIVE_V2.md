# Native archive v2 format and identity contract

Status: format and read-only verification implemented in v0.4 P2; streaming
export implemented in v0.4 P3. Verified restore/import is P4. Archive v1
remains supported as human-readable interchange and is not interpreted as v2.

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
- `plain_text` and `redact` link actions rewrite note bodies and are refused by
  export; they belong to the publication projection. `publication_handoff` is
  refused for the same reason and arrives with P7.

Export is a local filesystem operation. No REST or MCP surface accepts an
output path or streams archive bytes.

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

### Open format bound: object count versus full-database backup

Every saved revision, resource, and source bundle is one immutable object, and
the manifest lists every object inline. The 10,000-object and 4 MiB-manifest
bounds therefore cap one archive at roughly 9,900 revisions plus attachments —
measured at `performance/v0.4-p3/`, where 5,000 notes consumed 50.5 % of the
object budget. That is well below the million-note libraries J3 imports, so
archive v2 cannot yet back up a large library.

This is a format decision, not an exporter defect, and it is deliberately left
open rather than silently widened: raising `MaxObjects` alone does not work
because the inline object inventory would exceed the manifest bound. A future
revision needs the inventory to move into its own `records`-style object with
its own checksum. The exporter enforces the documented bounds today and fails
with an explicit object-budget error before publishing any manifest, so an
over-budget archive can never appear complete. See `agent/OPEN_QUESTIONS.md`.

## Golden and hostile fixtures

`internal/archivev2/testdata/golden-minimal/` is a complete manifest-last
archive containing all twelve record types plus body, resource, and source
bundle objects. Tests copy and mutate it to prove rejection of missing
manifests/objects, corruption, unsupported version/schema/capability, manifest
checksum drift, inconsistent counts, traversal, symlinks, extras, invalid MIME,
unknown/duplicate JSON fields, excessive count/depth, and unsafe relative
paths. The fixture is synthetic and contains no private note data.
