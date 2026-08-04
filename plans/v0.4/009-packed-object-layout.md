# v0.4 P3b — Packed object layout

Status: complete on 2026-08-04.

Model: Claude Opus 5 (Claude Code).

## Why

P3a made a real library archivable and then measured what it cost: the
382,206-note corpus wrote 382,407 loose objects in 48m26s, about 131 objects
per second for 1.14 GB. The hypothesis was that one
`create + write + fsync + rename` per object dominated, and that v0.7 sync
would additionally pay one transport round trip per object over REST and
folder/rclone.

This slice resolves `agent/OPEN_QUESTIONS.md` question 18 by building the
packed layout and measuring it against the loose one on the same corpus.

## Format

- `pack` joins the discriminated `location` union P3a introduced. A packed
  entry carries `pack_sha256`, `offset`, and `length` instead of a path; its
  own SHA-256 still identifies it, so placement cannot launder content.
- Pack containers are `kind: "pack"` index entries stored under the ordinary
  fanout. Packed entries reference their pack through the same spool join that
  checks every other reference, so a packed object naming a pack the archive
  does not carry fails like any dangling reference.
- Each pack ends with a self-describing trailer — a JSONL listing of contents
  and offsets, plus a fixed footer holding the trailer bounds and a magic — so
  a pack is verifiable without the archive index.
- `objects.pack.v1` is optional, but an archive that uses packs must declare it
  as required. A reader without pack support then rejects rather than misreads.
- Verification caches a bounded number of pack handles and reads each packed
  object as a section of its pack, hashing it against its index entry.

## Measured result

| | Loose | Packed |
|---|---|---|
| Files | 382,447 | **46** |
| Export + verify | 48m 26s | 37m 28s |
| Peak RSS | 298 MiB | 279 MiB |
| Disk | 1.322 GB | 1.466 GB |

Record counts and the bound selection digest are identical under both layouts.

**The speedup is 1.29×, not the order of magnitude the fsync hypothesis
predicted.** Reading and hashing 382,206 revision bodies out of SQLite
dominates, and both layouts pay it equally. The result worth having is the
8,314× file-count collapse, which is what the planned REST and folder/rclone
sync transports need — 46 files instead of 382,447 is a transport question,
not a local-speed one.

Packing costs about 11% more disk: a packed writer cannot use the published
object tree as its deduplication index, so duplicate content is written and
only collapses when index entries are published.

## Consequence

Loose stays the default and `--pack` is opt-in. Loose deduplicates through the
filesystem, resumes an interrupted export by reusing published objects, and
uses less disk. An interrupted packed export restarts; packs are
self-describing, so trailer-based resume remains possible later without a
format change.

The plan and docs state the transport rationale rather than claiming a local
speed benefit the measurement does not support.

## A defect the A/B found

The packed run reported 2.42 GB of archive bytes against 1.47 GB on disk:
`totals.Bytes` summed every index entry, counting a packed object once for
itself and again inside its pack. Unfixed it would have halved the effective
`MaxTotalBytes` bound for packed archives. Storage bytes are now counted once,
with a test comparing the manifest total against real on-disk size.

## Validation

- focused pack fixtures: packed export verifies and collapses files, capability
  declaration and its enforcement, packed-object corruption, self-describing
  trailers, storage-byte accounting;
- the complete P3/P3a suites unchanged, with packs off by default;
- real 382,206-note corpus A/B under `performance/v0.4-p3b/`;
- `go vet ./...`, `go test ./...`, required-file, scaffold, and docs checks.

## Next task

P4, native archive-v2 verify and restore/import, requires explicit user
approval. It must read both layouts and reuse the spooled verification.
