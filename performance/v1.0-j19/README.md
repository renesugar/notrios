# v1.0 J19-A: the external performance review, measured

The review (`openrouter/google/gemini-3.1-pro-preview` through `opencode`) is
treated as a list of hypotheses. Each one gets a measurement and a verdict:
**improves**, **no effect**, or **worse**. Nothing is adopted because it reads
well.

## Where time and memory actually go, before testing any suggestion

A 10,000-note subset of the recipe vault, imported with the J17-B importer into
a library on J5's HDD, with CPU and heap profiles.

**CPU**, as cumulative share of samples:

| where | share |
|---|---|
| inside SQLite (`runtime.cgocall`) | 67% |
| `ApplyImportDocumentBatch` | 60% |
| `rebuildDocumentLinksLocked` (links and blocks) | 50% |
| `sqlite3_prepare_v2`, compiling SQL on every call | 35% |
| regexp backtracking, mostly `markdownlinks.Extract` | 22% |
| `setDocumentSourceLocked` | 12% |
| `markdownblocks.Extract` | 9% |

**Allocation.** The run allocated 1.07 GB in total:

| where | share |
|---|---|
| `markdownblocks.Extract` | 51% |
| `readInventory` | 13% |
| `splitFrontmatter` (the string version, not the bytes version the review named) | 7% |
| `noteFingerprint` | 2% |

Peak RSS was 73 MiB.

**The review names none of the three largest costs.** Those are SQL compiled on
every call, regexp link extraction, and block extraction's allocations. Every
review suggestion targets something that takes at most a few percent. That caps
what any of them can gain, whatever its benchmark shows.

## Verdicts on the micro-level suggestions

The benchmarks are `internal/importers/obsidian/j19_bench_test.go` and
`internal/importers/joplinraw/j19_bench_test.go`. Each ran three times, with the
variant beside the current code.

| suggestion | measured | verdict |
|---|---|---|
| Pool the 32 KiB buffer in `hashFile` | 6 allocations, 312 B/op either way. The buffer never reaches the heap, and time is within run noise. | **no effect** |
| Stream the fingerprint composition instead of concatenating | Time equal. 9 allocations against 6, 2,576 B/op against 2,448. Byte-identical on every input tested (`TestJ19StreamedFingerprintIsByteIdentical`). | **worse** (allocations) |
| `string(...)` conversions in `splitFrontmatterBytes` allocate | **0 allocations**, 0 B/op. The compiler does not allocate for a conversion used only in a comparison. | **claim refuted** |
| Preallocate `seen` maps | Identical allocations at folder depths 1, 4 and 12. Time within noise. | **no effect** |

## How much one run differs from the next, before comparing anything

The profiled run above took 13.8 ms/note, and J17-B had recorded 19.7 for the
same importer. Both had other CPU work on the machine. So before any
comparison, the same 10,000-note import ran five times, one process at a time,
with nothing else running:

| run | importer | ms/note | wall | user | sys |
|---|---|---|---|---|---|
| new1 | current | 13.2 | 133.0 s | 124.3 s | 12.4 s |
| old1 | before J17-B | 27.1 | 271.8 s | 147.4 s | 41.5 s |
| new2 | current | 13.1 | 131.7 s | 123.3 s | 12.1 s |
| old2 | before J17-B | 31.1 | 311.4 s | 152.5 s | 42.9 s |
| new3 | current | 13.3 | 135.7 s | 125.8 s | 12.6 s |

- **Current importer: 13.1–13.3 ms/note, within 1.5% across three runs.** A
  difference smaller than that on this machine is not a difference.
- **J17-B's 1.46× was contaminated.** Measured clean, it is about 2.2×, and
  user CPU falls by about 25 s. The J17 and J5 records now carry that
  correction beside the original claim.
- **Shared-machine timings are not evidence.** The 19.7 and 13.8 differed by
  43% on identical code because other work ran beside them. Every comparison in
  this item runs alone.
- **Warm page cache.** The harness has no passwordless `sudo`, so it cannot
  drop the page cache, and every run reads the vault from a warm cache. That is
  the same condition for each run, and it excludes cold-read cost.

## Whole-vault maps: the one review claim that points at a real cost

`TestJ19InventoryMemory` builds what an Obsidian import holds for its whole run:
the inventory and the link namespace. It builds them from the full 382,206-note
vault and reports live heap after a full GC, so transient allocation is not
counted as memory held.

```
382206 files, 382206 notes, 0 assets, 780 folders; inventory walked in 2m32.109s
live heap held by the inventory:      672.4 MiB (1845 bytes per file)
live heap held by the link namespace: 122.4 MiB (336 bytes per note)
together: 794.8 MiB                   process peak RSS 1,279 MiB
```

**Verdict: confirmed.** The review said "hundreds of megabytes", and it is
795 MiB. J5's Obsidian import peaked at 1,721 MiB, so these two structures are
most of it.

1,845 bytes per file is also far more than a path, a title and two hashes
should need. The breakdown comes next, from a heap profile written while the
inventory is still live.

**The review's remedy has not been measured, and is not the only one.** The
review proposes transient SQLite tables and sorted merges in place of the maps.
That is a redesign of how links resolve. Smaller candidates come first, sized
by the breakdown: fields stored twice, hashes held as hex strings, and paths
that can be derived. None of this is adopted until measured.

## Full-text writes inside the transaction, or after it

`TestJ19FTSInsideVersusAfterTransaction` writes 10,000 real note bodies
(14.9 MiB) in batches of 100 into a library on the HDD. Each batch writes the
document, revision and projection job. `inline` also writes the full-text row
inside each batch, as the importer does today. `after` indexes every note in
its own batches of 100 once the writes finish. Both are timed until the last
note is searchable, and each mode ran twice, alternating.

| mode | run 1 | run 2 | ms/note |
|---|---|---|---|
| inline | 27.664 s | 26.990 s | 2.70–2.77 |
| after | 21.496 s | 21.525 s | 2.15 |

**On this write path, indexing afterwards is about 22% faster, and each mode
repeats.** It is not yet an import improvement:

- **The path excludes most of an import's work.** It leaves out links, blocks
  and sources. The ~0.6 ms/note saved is about 4.5% of the 13.2 ms/note a real
  import takes. That size is inferred, not measured, and it has to be measured
  through the importer before it is claimed.
- **The review's form of the change makes notes unsearchable for a while.** The
  review proposes an asynchronous background worker. A notes phase that indexes
  synchronously, before the import reports complete, would keep "import
  finished" meaning "searchable". It still changes what a note mid-import looks
  like, and it needs checkpoint handling for the index phase.

Both forms are product decisions. This goes to the owner with the numbers, and
nothing is merged.

## Still being measured


- **Full-text writes inside the transaction versus a sweep after it**, timed
  until the last note is searchable
  (`internal/store/j19_fts_cost_test.go`).
- **Memory held by the inventory and link namespace on the full 382,206-note
  vault** (`internal/importers/obsidian/j19_inventory_memory_test.go`).
