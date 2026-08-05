# v0.4 P4 — Native archive v2 verify and restore/import

Status: complete on 2026-08-05.

Model: Claude Opus 5 (Claude Code).

## Why

P3 through P3b made a real library archivable and measured what it cost. An
archive nothing can read back is not a backup, so this slice adds the
verify-only service surface and the restore path, and proves them against the
one corpus that carries resources and exact source bundles:
`JoplinExport_2026_07_18`, 111,330 items imported with `--preserve-source` into
103,349 notes, 758 resources, and 111,330 source-bundle items. Neither recipe
corpus has attachments, so P3a and P3b exercised the blob path at scale with
note bodies only.

## What it does

- `verify archive-v2` reads an archive read-only and reports what it contains,
  reusing P3a's spooled verification rather than holding record sets in memory.
- `restore archive-v2` verifies completely before the first canonical write,
  then applies containers, content, and relations in dependency order through
  bounded batches. Intent is mandatory: `replace`, `adopt`, `merge`, `fork`.
- Both object layouts are read. Blob and source-bundle bytes are re-hashed at
  the point of use, and blob admission re-sniffs MIME rather than trusting the
  archive.
- A durable `restore_state` marker (schema v13) is written before the first
  canonical row and cleared after the last, including identity adoption.

## Results on the attachment corpus

| Stage | Loose | Packed |
|---|---|---|
| Export | 15m 48s, 247 MiB | 8m 22s, 249 MiB |
| Verify | 4m 19s, 41 MiB | 3m 46s, 74 MiB |
| Restore (adopt) | 13m 05s, 43 MiB | 12m 40s, 127 MiB |
| Files | 215,484 | 30 |
| Archive bytes | 1,645,978,902 | 1,683,507,465 |

The packed-restored database is byte-identical to the source *and* to the
loose-restored database under a 14-column aggregate that includes ordered
content fingerprints over `blobs.sha256` and `source_bundle_items.sha256`.
Equal counts over different bytes cannot pass it.

## Defects this slice found

Seven, six of them only reachable at corpus scale or under fault injection.

1. **Quadratic restore lookup.** Resolving each object hash scanned every index
   chunk. The first restore was still incomplete after 184 minutes. It looked
   bounded because it held no map. Building the index once into the temporary
   indexed spool brought it to 13m 08s at 51 MiB.
2. **`full_archive` dropped unreferenced resources** (758 carried as 757).
   Reachability from documents is right for a subset transfer and wrong for a
   backup: a resource retention has deliberately not collected is live state.
3. **Restore registered source bundles as ordinary blobs** (731 rows became
   112,060), exposing preserved source bytes to resource GC and writing them to
   a path the bundle rows did not name, so restored bundles were unreadable.
4. **The pack handle cache closed packs it was still reading.** Restore holds a
   reader into one pack while opening the blobs its records name; eviction by
   arbitrary map choice could close the pack mid-scan once an archive held more
   packs than the cache. Reported as `unexpected end of JSON input`, because
   `bufio.Scanner` emits its leftover buffer as a final token on read failure —
   blaming the archive for an I/O fault.
5. **An interrupted restore that no intent could recover.** Containers and
   resources are written before the first document, so the target still counted
   zero documents and `replace` — the only recovery intent — was refused.
6. **Restore kept the target's builtin container rows** instead of the
   archive's, so two restores of one archive differed by their bootstrap
   timestamps.
7. **The reader treated the exporting database's schema version as a ceiling**,
   so bumping the schema to 13 made every archive already on disk unreadable,
   golden fixture included.

## Deduplication

The loose layout collapses duplicates for free because identical objects
address the same path; a pack writer only learns an object's hash after
streaming it, so packed archives carried duplicates loose archives did not.
Export now checks a hash the store already records — `resources.blob_sha256`,
`source_bundle_items.sha256`, or a note body hashed in memory — before opening
content. The membership index is the bounded temporary spool, not an in-memory
set. Restore's `admitted` and `bundlePaths` maps moved to the same spool.

Measurement corrected the design twice. The lookup is a net loss on the loose
layout, which needs no help, so it is gated to packed exports. And the restore
memory saving is invisible under packs, where peak RSS is set by reading pack
trailers whole; on loose it is 76,432 KB to 44,012 KB.

## `omitempty` on a struct value

`record_counts` carried `json:"...,omitempty"`, which Go silently ignores for a
struct, so every blob entry declared twelve zeroes — in index entries under
both layouts, not only pack trailers. Making it a pointer saves 44 MB loose and
88 MB packed, and because a pack trailer is read whole it cut packed verify
peak RSS 41% and runtime 31%. The packed layout's disadvantage against loose
was substantially self-inflicted.

Six struct-typed fields shared the tag. The four in `internal/api/types.go` and
`internal/store/store.go` are always populated, so honouring the tag would
delete keys from responses clients already receive; those tags were dropped
instead.

## Compatibility

The same mistake was made twice in one day: the schema-version ceiling, and
then a `RecordCounts != nil` check that rejected every archive written before
the field became a pointer. Both would have made existing backups unreadable
for a change that altered nothing about what the archive means.

Readers now accept an explicit all-zero `record_counts` as equivalent to
absence, and incompatibility is expressed with a required capability or a
format version rather than a schema number. A regression test rewrites a real
archive into the older spelling and requires it to verify.

## Validation

- restore fixtures: canonical round trip, resource bytes with relations,
  pre-write verification refusal, mandatory intent, unreferenced resources,
  bundles staying out of the blob store;
- packed-layout restore: layout equivalence by re-exported object set, resource
  and bundle bytes out of pack slices, corruption refusal, the handle-cache
  contract, and an archive spanning more packs than the cache holds — which
  fails 5 times out of 5 without its fix;
- crash/fault injection at each of the six stages that commit canonical state,
  plus recovery by `replace` producing a library identical to a clean restore;
- objects that change between verification and use;
- archives written before optional record counts;
- attachment-corpus evidence under `performance/v0.4-p4/`;
- `go vet ./...`, `go test ./...`, required-file, scaffold, and docs checks.

## Next task

P5, stable external links and local resolution, requires explicit user
approval.
