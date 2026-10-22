# v1.0 J19: the external performance review, measured

## Outcome

| review suggestion | verdict | adopted |
|---|---|---|
| pool the 32 KiB `hashFile` buffer | no effect | no |
| stream the fingerprint composition | worse (more allocations), though byte-identical | no |
| avoid `string(...)` in `splitFrontmatterBytes` | refuted: 0 allocations | no |
| preallocate `seen` maps | no effect | no |
| stream frontmatter instead of `os.ReadFile` | not applicable: the whole body is needed | no |
| whole-vault in-memory structures cost hundreds of MB | **confirmed**, but the cause was substrings keeping text alive, not maps | **yes**, as clones, in both importers |
| full-text writes after the transaction | 22% faster on a partial write path, about 4.5% of an import by inference | **no, by owner decision** |

**Owner decision on full-text placement: keep it inline (option a).** A note
stays searchable as soon as its batch commits. The measured gain was small,
and it was never measured through the importer. Both alternatives would make
notes invisible to search for longer: an indexing phase at the end of the
import, and the review's background worker.

**Carried to J20, not done here:**
- The Obsidian inventory still stores every note's struct twice, in `Notes` and
  `Files` (160 MB).
- It keeps hashes as hex strings (57 MB).
- J5's two 382,206-note corpora are re-run there for a clean baseline under
  the current importers. None of J19's memory results has been confirmed on a
  full import.

**Found here, outside the review, and not planned by this item:**
- SQL compiled on every call: 35% of import CPU
- regexp link extraction: 22%
- block extraction: half of all allocation

## J19-C: nothing else moved

- **Obsidian clone.** J17's generated 300-note vault imports to a library that
  is table by table the same as before the change. That includes:
  - the full-text rows and rowid mapping (`documents_fts`,
    `documents_fts_rowid`)
  - every item state and fingerprint, and the checkpoint
  - links, blocks and attachment references

  The fixture test also re-imports an unchanged vault and gets every item
  unchanged.
- **Joplin clone.** The whole inventory is byte-identical before and after, by
  SHA-256, on a 103,349-note and a 382,206-note export.
- **Store tests and J18.** The store is untouched, and J18's validator passes.
  The micro-level variants live only in `_test.go` files.

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

## Reading whole files with `os.ReadFile`

The review says both importers read whole items into memory "before checking
boundaries", and proposes streaming the frontmatter and breaking early.

**Verdict: not applicable as proposed.** The Obsidian importer rewrites links
across the whole body, and both importers write the whole body to the full-text
index and the revision. The body is read in full whatever happens, so breaking
early after the frontmatter would skip work that still has to be done.

What remains is copying, and the heap profile above sizes it:
- `os.readFileContents`: 2.8% of the run's allocation
- the `string(raw)` conversion in `canonicalBody`: under `canonicalBody`'s 7.1%

`readExact` hashes the bytes it has already read, with no second copy. A 1–3%
allocation share sits inside a CPU profile where allocation and GC together are
under 3%. No change here can reach the 1.5% run-to-run noise floor on import
time, so no variant is built.

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

**Where it goes.** An in-use heap profile, written by the test while the
inventory is still live (`NOTRIOS_J19_HEAP`), accounts for 804 MB:

| held by | MB | why |
|---|---|---|
| `markdownTitle` → `splitFrontmatter` | 144 | the title is a substring of a copy of the frontmatter, so it keeps that whole copy alive |
| `string(frontmatter)` for `frontmatterPropertyOrder` | 164 | property names are substrings of this copy, so it stays alive too |
| `PropertyOrder` slices | 81 | |
| `Notes` slice of `vaultFile` | 80 | every note's struct is stored here… |
| `Files` slice of `vaultFile` | 80 | …and again here |
| `buildLinkNamespace` maps | 120 | |
| hex hash strings | 47 | |
| paths, IDs, the rest | ~88 | |

**About 390 MB of it is substrings keeping their source strings alive.** A
title of a few dozen bytes holds the frontmatter copy it was cut from, and the
inventory lives for the whole import. That is not the "native Go maps" the
review blamed. The maps (the link namespace) hold 120 MB.

**Candidate: clone the substrings the inventory keeps.** `readInventory` now
stores `strings.Clone` of the title, and of each alias and property name, so
none of them pins the frontmatter copy it came from. Values are unchanged, so
fingerprints are unchanged: they hash file bytes, not these fields. Same vault,
same test:

| | before | cloned |
|---|---|---|
| live heap, inventory | 672.4 MiB | **438.1 MiB** (−35%) |
| live heap, inventory + link namespace | 794.8 MiB | **560.5 MiB** (−29%) |
| process peak RSS | 1,279 MiB | 1,016 MiB |

**Verdict: improves.** The substrings accounted for about 390 MB. Cloning frees
234 MiB, because the clones themselves cost 54.5 MB and the property-name
slices remain. What is left is mostly structure:
- `Notes` and `Files` each hold a full copy of every note's struct (160 MB
  between them)
- hex hash strings (57 MB)
- the link namespace (116 MB)

Those are the next candidates, each to be measured on its own. `Files` is read
only by the source-bundle phase and to copy target IDs across, so it need not
hold a second full struct per note. That still has to be measured before it
counts.

**Proven not to change the import.** J17's generated 300-note vault imports to
the same library with and without the clones:
- 1,055 links, 170 attachment references and 3,129 blocks
- the checkpoint, and all 329 item states and their fingerprints

`j17_compare.py` finds only the three random-identifier columns that differ
between any two libraries. The Obsidian importer tests pass.

### The same defect in the Joplin importer, and much larger

The Joplin RAW importer does not hold notes in its inventory, only a map of
note IDs. `parseInventoryItemBytes` converts each file with `string(raw)` and
cuts every field, the ID included, from that text. The ID then becomes a key
in `NoteIDs`. **Each key kept its note's entire file alive.**
`TestJ19InventoryMemory` in `internal/importers/joplinraw`, on the full
382,206-note recipe export:

| | before | cloned |
|---|---|---|
| live heap held by the inventory | **1,301.4 MiB** (3,571 bytes per note) | **51.9 MiB** (142 bytes per note) |
| process peak RSS | 2,295 MiB | 258 MiB |

Before, 97% of the held heap sat under `parseInventoryItemBytes`. After, the
largest holder is the note-ID map's own values, at 20 MB. **Verdict: improves,
by 96%.** J5 recorded a 2,881 MiB peak for the full Joplin import, and this
inventory is probably most of it. That is inferred: the full import has not
been re-run.

The Joplin inventory reads took 10 m 30 s and 9 m 45 s. Both ran while the
Obsidian clone measurement shared the disk, so no timing claim is made.

**Proven to leave every inventory value identical.** `TestJ19InventoryMemory`
with `NOTRIOS_J19_DIGEST` writes a SHA-256 of the whole inventory, serialised as
JSON with sorted map keys. It ran on the importer before the change (a worktree
at `111a53c`) and after, on two real exports:

| export | digest, before and after | live heap before → after |
|---|---|---|
| personal, 103,349 notes, 763 resources | `a9d03f8029ee6031aab676038a32ac26c3e762526761380e264c2b494b6fde08` (9,149,711 bytes of JSON) | 216.5 → **14.6 MiB** |
| recipe, 382,206 notes, 11,753 tags | `e0d23c26ad80bf377f9c956dc86dd6d4a9b68b01202d39b31719026d70ca5316` (32,148,136 bytes of JSON) | 1,301.4 → **51.9 MiB** |

The change touches only the values `parseInventoryItemBytes` returns. So
identical inventories mean every later phase reads identical input. The
personal export is recorded here only as these totals. The Joplin importer
tests pass.

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
