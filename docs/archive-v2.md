# Native archive v2 safety contract

Notrios archive v2 is the lossless backup and transfer format. v0.4 P2 defines
and verifies the format; P3 adds the streaming writer
(`notriosctl export archive-v2`). Verified restore is P4, so today a v2 archive
is a checksum-verified snapshot you keep, not something Notrios can read back
yet. Keep an archive v1 export or a database copy until P4 lands.

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

Use archive v1 commands for query-scoped plain-note interchange. P4 will add
verify-only and restore commands.
