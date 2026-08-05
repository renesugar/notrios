# v0.4 P4 attachment-corpus round trip — findings

Aggregate-only evidence from the attachment-bearing Joplin RAW archive
(`JoplinExport_2026_07_18`), imported with `--preserve-source`, then exported,
verified, and restored. No note content, resource bytes, or local paths are
recorded.

The first round trip failed and exposed two data-fidelity defects, recorded
below as findings 1 and 2. Both are fixed; the numbers in this table are from
the re-run against the fixed code, on a rebuilt archive — finding 1 was an
export defect, so the original archive could not be reused.

| Stage | Result |
|---|---|
| Import (111,330 items) | 103,349 notes, 758 resources, 111,330 source-bundle items |
| Export | 16m 49s, 247 MiB peak RSS, 215,461 objects, verified inline |
| Verify (standalone CLI) | 4m 19s, 41 MiB peak RSS, 509,527 records, read-only |
| Restore (adopt) | 12m 12s, 75 MiB peak RSS |
| **Aggregate round trip** | **identical** |

The comparison is a 14-column SQLite aggregate over source and restored
databases, byte-identical across both (896,557 bytes each): documents
(103,349), revisions (103,349), resources (758), blobs (731),
document_resource_refs (761), note_tags (6,890), tags (322), notebooks (7),
document_sources (103,349), source_bundle_items (111,330), document_links
(79,409), total blob bytes (955,882,362), plus two content fingerprints — the
ordered concatenation of every `blobs.sha256` prefix and of every
`source_bundle_items.sha256` prefix. The two fingerprints are what make this a
content check rather than a row count: equal counts with different bytes would
not pass.

Archive composition: 215,410 blob objects (103,349 revision bodies + 111,330
source-bundle items + 731 distinct resource blobs), 51 record objects, 22 index
objects, 0 packs. The archive gained exactly one blob object over the failed
first export (215,409 → 215,410) — the previously dropped unreferenced
resource. The composition arithmetic corroborates finding 1 independently of
the resource count.

Export, standalone verify, and restore all report the same
`commit_sha256 75afa028…` and `snapshot_id snap_2ahqhtk6…`, so the verified
artifact is the one that was restored, not a re-derived equivalent.

## Finding 1 — `full_archive` dropped unreferenced resources (fixed)

The source database holds **758** resources; the first export carried **757**,
and the restore faithfully reproduced 757. The loss was at export, not restore.

P1 reachability starts from selected documents and follows
`document_resource_refs`, so a resource no document references is not
"reachable" and never entered the archive. For a subset transfer that is
correct. For `full_archive` it is not: an unreferenced resource still inside
its retention window is live canonical state that garbage collection has
deliberately not yet collected, and a full backup that silently drops it is not
a full backup.

Fixed in `loadSelectionResourcesLocked`: an unscoped full archive now selects
every resource in scope, while a scoped selection still carries only what its
notes reach. Source bundles already followed this rule; resources now match.
`TestFullArchiveCarriesUnreferencedResources` asserts both halves, so the fix
cannot regress into leaking unreferenced content into a subset transfer.

## Finding 2 — restore registered source bundles as ordinary blobs (fixed)

Source blobs went from **731** to **112,060** (731 + 111,330 bundle items, less
one duplicate), and total blob bytes from 955,882,362 to 1,161,281,906.

`DATABASE_SCHEMA.md` places exact source bundles under
`assets/source-bundles/` and states the namespace is "deliberately outside
ordinary `blobs` and resource garbage collection". Restore admitted bundle
content through `AdmitRestoredBlob`, which inserts a `blobs` row — exposing
preserved source bytes to resource GC.

The same call also wrote the bytes under `assets/sha256/...` while the restored
`source_bundle_items.storage_path` pointed at `source-bundles/sha256/...`, so
restored bundles were unreadable.

Fixed with a dedicated `AdmitRestoredSourceBundle` that writes to the
source-bundle namespace and creates no `blobs` row.
`TestRestoredSourceBundlesStayOutOfTheBlobStore` asserts both the absence of
added blobs and that a restored bundle is readable at the path it records —
the second assertion is the one that catches the unreadability, which a row
count alone would miss.

## Finding 3 — restore object lookup was quadratic (fixed)

The first restore reader resolved each object hash by scanning every index
chunk. Against ~112,000 blobs and ~1.1M index entries that stalled: the run was
still incomplete after 184 minutes and had to be killed. It looked bounded
because it held no map, but the cost was quadratic.

Building the index once into the temporary indexed spool J3 already uses
brought the same restore to 13m 08s at 51 MiB peak RSS.

## Memory note

Restore peak RSS rose from 51 MiB to 75 MiB with the finding-2 fix, which
caches resolved bundle storage paths. The cache is keyed by content hash rather
than by item, so it is bounded by distinct bundle content (~112k entries here),
not by the 111,330 items or by corpus size in bytes. It is a real cost, not a
measurement artifact, and is recorded here so a later regression in restore
memory is not mistaken for this change.

## Status

The attachment-bearing round trip passes. Remaining P4 work: crash/fault
injection, and restore coverage for the packed layout — the reader handles both
layouts but only loose is exercised at corpus scale.
