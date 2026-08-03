# Native archive v2 safety contract

Notrios archive v2 is the future lossless backup, transfer, and publication
handoff format. The v0.4 P2 foundation now defines and verifies that format;
the current CLI still exports/imports archive v1 until the P3 writer and P4
verified restore tasks are implemented.

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
manifest checksum.

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

For now, use archive v1 commands only for query-scoped plain-note interchange.
P3 will add streaming v2 export; P4 will add verify-only and restore commands.
