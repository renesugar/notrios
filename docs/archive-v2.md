# Native archive v2 safety contract

> Scale review (updated 2026-08-23): export, verification, restore, and both
> existing object layouts remain supported. G14b selected a separate,
> compatible same-schema SQLite image plus bounded packed assets for physical
> whole-library backup/catch-up. G14c implements creation/verification and G14d
> integrates encrypted transport plus crash-safe restore. G14e passed 19
> aggregate full-scale phases and freezes that physical capability as the
> compatible whole-library default.
> Archive v2 remains the semantic subset, merge,
> schema-independent interchange, and fallback format.

Notrios archive v2 is the lossless backup and transfer format. Four commands
use it: `notriosctl export archive-v2` writes a snapshot,
`notriosctl compatibility archive-v2` checks a bounded capability declaration,
`notriosctl verify archive-v2` reads one back read-only, and
`notriosctl restore archive-v2` admits one into a database under an explicit
intent. All four are local filesystem commands with no REST or MCP equivalent.

## What v2 preserves

A full v2 snapshot can represent notes and every saved revision, nested and
query-backed notebooks, tags and memberships, links, provenance, logical resources and
their exact bytes, and optional exact source bundles. Search indexes, Recoll
projections, caches, generated sites, and SQLite WAL files are deliberately
excluded because they are derived or unsafe to copy as backup state.

Every database now persists two distinct random identities:

- a stable logical database ID, preserved only by an explicit in-universe
  restore;
- a replica ID for one writable copy, always rotated by supported clone/restore
  workflows.

Neither identity comes from a filename, path, profile name, or hostname.

## How verification works

A v2 archive is a directory of immutable SHA-256-addressed objects. Object
hashes, exact sizes, canonical MIME types, typed record counts, schema bounds,
required capabilities, database/snapshot identity, and the earlier
[selection/privacy plan](selection-planning.md) digest are bound by a final
manifest checksum. The manifest binds the index chunks and each chunk binds
the objects it names, so the checksum chain still reaches every byte even
though the manifest itself never grows.

`manifest.json` is written last and is the only completion marker. Verification
rejects an absent manifest, damaged or missing objects, unknown required
features, unsupported versions/schemas, count/reference inconsistencies,
unsafe paths, symlinks, extra files, excessive JSON/path/notebook depth, and
MIME disagreement before a restore is allowed to write anything.

## Restore choices are explicit

- **Replace** an existing database from a full snapshot and preserve the
  archive's logical database ID.
- **Adopt** a full snapshot into an empty target and preserve its logical ID.
- **Merge** records into an existing target while keeping the target's logical
  ID and applying conflict/import rules.
- **Fork** into an explicitly new logical database ID.

There is no implicit default and Notrios never decides by comparing local
filesystem paths. Replace/adopt/fork create a writable copy with a new replica
ID; merge is an import into the existing target replica.

## Writing an archive

```bash
# Complete database backup, verified before the command reports success.
notriosctl export archive-v2 /backups/notrios-2026-08-04

# Explicitly scoped subset transfer.
notriosctl export archive-v2 --target subset_transfer \
  --notebooks nb_research --tags shared /transfer/research
```

`full_archive` is the default and is the only mode that claims to be a complete
backup: it includes trashed notes, every saved revision, provenance, private
source metadata, and exact source bundles. `subset_transfer` requires at least
one selector, keeps provenance identity but strips private source
`metadata_json`, omits whole-library search notebooks, and records links whose
target fell outside the selection as `target_excluded` instead of leaving a
dangling reference. Any archive that is not a complete backup says so in its
warnings and in the `full_backup` field of the JSON report.

The writer publishes every object first and `manifest.json` last. If an export
is interrupted, the destination has no manifest, so verification reports it as
incomplete; re-running the same command reuses the already-published objects,
removes any object the new manifest does not list, and republishes. Pass
`--overwrite` to replace an archive that already has a manifest. Temporary
files live in a sibling `<destination>.staging` directory that is removed when
the command finishes, so the archive directory itself never holds one.

Export is a local command on purpose: no REST or MCP endpoint accepts an output
path or streams archive bytes.

## Size

Each saved revision, resource, and source bundle becomes one immutable object.
The manifest does not list them: it names checksummed **index chunks** that do,
so the manifest stays a few kilobytes whether an archive holds a hundred notes
or a million. An archive may hold up to 8,000,000 objects.

Objects live under a two-level fanout (`objects/sha256/ab/cd/<hash>`) so a
large archive does not pile millions of files into a few directories. Export
and verification both stream: neither keeps a table of objects or records in
memory, so peak memory tracks the library's notebook and collection counts
rather than its note count.

## Packing

`--pack` concatenates objects into a few large pack files instead of writing
one file per object. On a real 382,206-note library that turns 382,447 files
into 46, and the export runs about 1.29× faster.

Loose storage is still the default, because it deduplicates and resumes through
the object tree and uses about 11% less disk. Reach for `--pack` when the
archive will be *moved* — copied to a remote, synchronized, or shipped — where
the number of files matters far more than the number of bytes. An interrupted
packed export restarts rather than resuming.

## Checking consumer compatibility

Use `notriosctl compatibility archive-v2 /backups/notrios-2026-08-05` for
declaration-only admission with the current reader. Add `--reader
previous-loose-v2` before the path to reproduce the frozen reader matrix
without claiming an old binary ran.

The command emits a machine-readable accept/refuse decision. It reads only the
bounded manifest: it never opens index/object bytes or a physical SQLite image.
Exit 0 means the selected profile supports the declaration and that full
verification is still required; refusal exits 1 with a stable reason code.
Unknown required capabilities refuse, while unknown bounded optional
capabilities may be ignored.

The published [archive-v2 contract](contracts/archive-v2/contract.json),
[manifest JSON Schema](contracts/archive-v2/schemas/manifest.schema.json), and
[sanitized deterministic fixture matrix](contracts/archive-v2/fixtures/fixture-matrix.json) are the
consumer-facing reference. Loose schema-12, packed schema-12, and packed
schema-27 sync-era fixtures are complete and independently generated. The
physical refusal fixture contains a real manifest shape with invented hashes
but no `notes.sqlite` or pack bytes. A portable consumer must identify
`notrios-sqlite-image` v1, refuse it, and report its `notrios-archive-v2`
semantic fallback rather than treating it as archive-v3.

## Reading an archive back

```bash
# Read-only: report what the archive contains and prove it is intact.
notriosctl verify archive-v2 /backups/notrios-2026-08-05

# Restore into a database. The intent is required.
notriosctl restore archive-v2 --intent adopt --db ./restored/notes.sqlite \
  /backups/notrios-2026-08-05
```

`verify` never writes. `restore` finishes verifying the whole archive before it
writes its first row, so a damaged archive cannot leave a half-restored library
behind. It reads both object layouts, re-hashes every note body, resource, and
source bundle as it uses it, and re-sniffs attachment types through the ordinary
resource admission path instead of trusting what the archive claims.

Pick the intent deliberately — Notrios never guesses one, and never infers it
from a filesystem path:

| Intent | Target | Result |
|---|---|---|
| `adopt` | empty database | the archive's logical database ID, a new replica ID |
| `replace` | existing database | contents replaced; archive's database ID, new replica ID |
| `merge` | existing database | records imported; the target keeps its own identity |
| `fork` | any | a new logical database ID you supply with `--new-database-id` |

If a restore is interrupted — power loss, a full disk, Ctrl-C — the library keeps
a durable marker recording which snapshot it was restoring. That library is not
a valid database: `adopt`, `merge`, and `fork` refuse it, and only
`--intent replace` recovers it, producing the same library a clean restore
would have.

Use archive v1 commands for query-scoped plain-note interchange.
