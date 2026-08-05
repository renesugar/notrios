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

## Packed layout — the same corpus stored the other way

P4 requires the corpus to restore under both object layouts. The same
`attach.sqlite` was exported packed, verified, and restored into a third
database.

| Stage | Loose | Packed |
|---|---|---|
| Export | 16m 49s, 247 MiB | **10m 11s**, 283 MiB |
| Verify (standalone CLI) | 4m 19s, 41 MiB | 5m 26s, 124 MiB |
| Restore (adopt) | 12m 12s, 75 MiB | 12m 40s, 127 MiB |
| Files on disk | 215,484 | **30** |
| Space consumed (`du`) | 2.5 GB | **1.7 GB** |
| Object bytes (apparent) | 1,587,759,142 | 1,717,940,079 |

Two comparisons pass: the packed-restored database is byte-identical to the
source under the same 14-column aggregate, **and** it is byte-identical to the
loose-restored database. Layout is therefore invisible in the restored library
— which is the property that matters, and which row counts alone would not
establish.

Packed export is 40% faster than loose: creating 215,410 files costs more than
writing seven. The file count collapses 7,183× (30 files = 7 packs + 22 index
chunks + the manifest), and block-rounding waste on 215k small files makes the
packed archive occupy 0.8 GB less despite carrying more bytes.

Verify and restore both cost more memory under packs (124 and 127 MiB, versus
41 and 75 MiB). A pack trailer is read whole, so it scales with objects per
pack rather than with the archive; it stays bounded, but the packed layout is
not the cheaper one to read.

### Packed archives carry 8.2% more object bytes, fully accounted

The 130,180,937-byte difference decomposes exactly, with 224 bytes left over
for the seven pack footers:

| Component | Bytes |
|---|---|
| Pack trailers | 78,305,373 |
| — of which zero `record_counts` on blob entries | 44,164,585 |
| Duplicate object copies inside packs | 51,875,340 |
| Pack footers (7 × 32) | 224 |

Both components are avoidable, and neither is a correctness problem. They are
recorded rather than fixed here because both change the pack format, which
would invalidate the archive this evidence was measured against; the work
belongs with a format revision, not with restore coverage.

**Blob trailer entries carry a fully expanded `record_counts` of twelve
zeroes.** An object holding bytes has no records to count. This is 44 MB — 34%
of all trailer bytes, 2.6% of the archive — spent describing nothing.

**Packs do not deduplicate.** *(fixed — see below.)* The loose layout collapses
duplicates for free because two identical objects address the same path. A pack
writer streams bytes and only learns the object's hash once it has written
them, so the pack physically carried 27 duplicate copies of 24 hashes — 51.9 MB
here. The index still held one entry per hash, so restores were correct and the
duplicates were pure waste rather than a fidelity risk.

## Deduplication, measured

Export now checks a hash the store already records — `resources.blob_sha256`,
`source_bundle_items.sha256`, or a note body hashed in memory — *before* opening
content, so a duplicate costs an indexed lookup instead of a read and a write.
The membership index is the bounded temporary spool J3 and the restore object
index already use, not an in-memory set. Restore's `admitted` and `bundlePaths`
maps moved to the same spool.

| Measurement | Before | After |
|---|---|---|
| Packed archive bytes | 1,823,703,072 | **1,771,827,000** |
| Packed export | 10m 11s | **9m 26s** |
| Loose archive bytes | 1,690,137,952 | 1,690,137,952 |
| Loose export | 16m 49s | 16m 50s |
| Loose restore peak RSS | 76,432 KB | **44,012 KB** |
| Loose restore | 12m 12s | 13m 05s |
| Packed round trip | — | identical |

The 51,876,072 bytes saved match the predicted duplicate total (51,875,340) to
within 732 bytes, and packed export got *faster*: skipping 52 MB of writes more
than pays for 215,410 lookups.

Two results contradicted the reasoning that motivated the change, and are
recorded because the corrected design came from them.

**The lookup is a loss on the loose layout.** Enabling it everywhere cost loose
export 7.6% — 16m 49s to 18m 06s — to find 27 duplicates among 215,410 objects,
on the one layout that already deduplicates for free. The check is now gated to
packed exports, and loose export measures 16m 50s against a 16m 49s baseline:
the regression is gone rather than assumed gone.

**The memory win is real but was invisible where it was first measured.**
Packed restore peak RSS did not move (130,528 KB to 130,680 KB), because there
the peak is set by reading pack trailers whole, not by the maps. On the loose
layout, where nothing else dominates, peak RSS fell 42%, from 76,432 KB to
44,012 KB — below even the 51,632 KB measured before source bundles were
admitted correctly.

The cost is 7% more restore time (12m 12s to 13m 05s) for memory that no longer
grows with the library. At this scale that trade is marginal; at ten million
items the maps would cost gigabytes while the spool stays flat, which is the
failure mode the spooled writer and verifier exist to prevent.

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

## Finding 4 — packed restore closed packs it was still reading (fixed)

Restore reads a record object and, for each record inside it, opens the blob
that record names. Those blobs live in other packs. The pack handle cache
evicted by arbitrary map choice, so once an archive held more packs than the
16-handle cache, it could close the record object's own pack while its reader
was mid-scan.

This is the defect loose-only coverage could not reach: it needs more packs
than the cache holds *and* a record object too large to be consumed in one
buffered read. No small fixture reaches that state.

Handles are now reference counted. Eviction takes the least recently used idle
handle and never one a reader holds; if every cached handle is borrowed the
cache exceeds its bound rather than break a live reader, which is safe because
the excess is bounded by concurrent readers rather than by archive size.

A second defect sat underneath it. `bufio.Scanner` emits whatever is left in
its buffer as a final token when the underlying read fails, so the closed-handle
error arrived disguised as `unexpected end of JSON input` — blaming the archive
for an I/O fault and pointing any investigation at the export path. Read errors
now report as read errors.

The regression test fails 5 times out of 5 without the fix.

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

The attachment-bearing round trip passes under both object layouts, and the two
restored libraries are byte-identical to each other and to the source.

Remaining P4 work: crash/fault injection.

Deferred to a pack format revision, not blocking P4: the zero `record_counts`
on blob trailer entries, and pack-internal deduplication.
