# v0.4 P4 attachment-corpus round trip — findings

Aggregate-only evidence from the attachment-bearing Joplin RAW archive
(`JoplinExport_2026_07_18`), imported with `--preserve-source`, then exported,
verified, and restored. No note content, resource bytes, or local paths are
recorded.

| Stage | Result |
|---|---|
| Import (111,330 items) | 103,349 notes, 758 resources, 111,330 source-bundle items |
| Export | 215,409 blob objects, verified |
| Verify (standalone CLI) | passed, read-only |
| Restore (adopt) | 13m 08s, 51 MiB peak RSS |
| **Aggregate round trip** | **DIFFERENT — two defects open** |

Identical across source and restored databases: documents (103,349), revisions
(103,349), document_resource_refs (761), note_tags (6,890), tags (322),
notebooks (7), document_sources (103,349), source_bundle_items (111,330),
document_links (79,409).

## Finding 1 — `full_archive` drops unreferenced resources

The source database holds **758** resources; the export carried **757**, and
the restore faithfully reproduced 757. The loss is at export, not restore.

P1 reachability starts from selected documents and follows
`document_resource_refs`, so a resource no document references is not
"reachable" and never enters the archive. For a subset transfer that is
correct. For `full_archive` it is not: an unreferenced resource still inside
its retention window is live canonical state that garbage collection has
deliberately not yet collected, and a full backup that silently drops it is not
a full backup.

Fix: a full archive must select every resource in scope, not only the
document-reachable ones.

## Finding 2 — restore registers source bundles as ordinary blobs

Source blobs went from **731** to **112,060** (731 + 111,330 bundle items,
less one duplicate), and total blob bytes from 955,882,362 to 1,161,281,906.

`DATABASE_SCHEMA.md` places exact source bundles under
`assets/source-bundles/` and states the namespace is "deliberately outside
ordinary `blobs` and resource garbage collection". Restore admits bundle
content through `AdmitRestoredBlob`, which inserts a `blobs` row — exposing
preserved source bytes to resource GC.

The same call also writes the bytes under `assets/sha256/...` while the
restored `source_bundle_items.storage_path` points at
`source-bundles/sha256/...`, so restored bundles are very likely unreadable.

Fix: restore must admit bundle content through the source-bundle namespace,
matching what the importer writes, and must not create `blobs` rows for it.

## Finding 3 — restore object lookup was quadratic (fixed)

The first restore reader resolved each object hash by scanning every index
chunk. Against ~112,000 blobs and ~1.1M index entries that stalled: the run was
still incomplete after 184 minutes and had to be killed. It looked bounded
because it held no map, but the cost was quadratic.

Building the index once into the temporary indexed spool J3 already uses
brought the same restore to **13m 08s at 51 MiB peak RSS**.

## Status

Findings 1 and 2 are open data-fidelity defects. P4 is not complete and this
round trip must be re-run after they are fixed.
