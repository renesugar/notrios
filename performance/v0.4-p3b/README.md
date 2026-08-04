# v0.4 P3b packed object layout — A/B evidence

Both layouts exporting and verifying the same real 382,206-note Joplin corpus
on the same machine (Intel i5-9300H, Go 1.26.5, linux/amd64, NVMe). Aggregate
only; no note content, paths, databases, or archives are committed.

| | Loose (`fanout`) | Packed (`pack`) |
|---|---|---|
| **Files on disk** | **382,447** | **46** |
| Objects indexed | 382,407 | 382,413 (6 packs) |
| Export + verify | 48m 26s | **37m 28s** |
| Peak RSS | 298 MiB | 279 MiB |
| Disk used | 1.322 GB | 1.466 GB |
| `verified` | true | true |

Record counts and the bound selection digest are identical under both layouts.

## What the measurement actually says

**Packing is a 1.29× speedup, not an order of magnitude.** Going in, the
hypothesis was that one `create + write + fsync + rename` per object dominated,
so collapsing 382k of them into 6 would transform throughput. It did not. Most
of the remaining time is reading 382,206 revision bodies out of SQLite and
hashing them, which both layouts pay equally.

**The real win is the file count: 8,314× fewer files.** That is what matters
for v0.7 synchronization, where the container is carried over REST and
folder/rclone transports and every object costs a round trip. Shipping 46
files instead of 382,447 is the difference between a practical and an
impractical transport, independent of local speed.

**Packing costs about 11% more disk here.** A packed writer cannot use the
published object tree as its deduplication index, so duplicate content is
written into packs and only collapses when index entries are published. Pack
trailers add a little more.

## Consequence: loose stays the default

`--pack` is opt-in. Loose remains the default because it deduplicates through
the filesystem, resumes an interrupted export by reusing published objects, and
uses less disk. Packs are the right choice when an archive will be *moved* —
which is exactly the v0.7 sync case — and the plan records that as the reason
the layout exists rather than claiming a local speed benefit it does not have.

An interrupted packed export restarts rather than resumes. Packs are
self-describing (each carries a trailer listing its contents and offsets), so
trailer-based resume remains possible later without a format change.

## A defect this A/B found

The packed run reported 2.42 GB of archive bytes against 1.47 GB on disk:
`totals.Bytes` summed every index entry, counting a packed object once for
itself and again inside its pack. Left alone it would have halved the effective
`MaxTotalBytes` bound for packed archives. Storage bytes are now counted once,
with a test that compares the manifest total against real on-disk size.

The packed measurement above predates that fix, so its manifest byte field is
not comparable; `disk_bytes` is measured with `du`.

## Reproducing

```bash
notriosctl export archive-v2 --db <db> --asset-store <assets> <out-dir>
notriosctl export archive-v2 --db <db> --asset-store <assets> --pack <out-dir>
```
