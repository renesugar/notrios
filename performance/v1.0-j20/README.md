# v1.0 J20: a fresh J5 baseline, and the rest of the Obsidian inventory memory

## J20-A: the baseline under the current code

`baseline_profile.sh` is J5's `scale_profile.sh` with user and system CPU
recorded. It runs the same CLI import into a new library on the same HDD, then
the same three search probes, cold and warm. The runs:
- import the Obsidian vault first, then the Joplin RAW export, one at a time
- run with nothing else on the machine, from a clean tree at `17aa250`
- read from a warm page cache, since there is no passwordless `sudo` to drop it

**Imports.** Both completed with J5's counts: 382,206 notes each, 780 and 781
notebooks, 11,753 Joplin tags, and checkpoints `completed`.

| 382,206 notes | J5 wall | J20-A wall | J20-A user / sys | J5 peak RSS | J20-A peak RSS |
|---|---|---|---|---|---|
| Obsidian vault | 16,261.5 s (4.52 h) | **5,245.5 s (1.46 h)** | 4,881.3 s / 389.4 s | 1,720.5 MiB | **1,197.0 MiB** |
| Joplin RAW | 6,194.1 s (1.72 h) | **5,907.9 s (1.64 h)** | 5,235.6 s / 799.9 s | 2,880.7 MiB | **290.7 MiB** |

- **Obsidian: 3.1× faster and 30% less peak memory.** J17-B batched the
  commits, J18 removed the full-text scan from writes, and J19 cloned the
  inventory's substrings. This is the first full-corpus measurement of those
  changes together. It is not an attribution: they were not measured
  separately at this size.
- **Joplin: 4.6% faster and 90% less peak memory.** The memory matches J19's
  inventory measurement: 1,301 MiB of the old peak was the inventory pinning
  every note's file. The time is close to J5's, as expected, because the Joplin
  importer already batched.
- **The J5 asymmetry has reversed.** The Obsidian import is now 11% faster than
  the Joplin import of the same notes, where J5 found it 2.6× slower.
- **Provenance.**
  - The Obsidian run records commit `17aa250`. The Joplin run records `7c04d12`,
    because the J20 README was committed while it ran.
  - That commit changed only this README. Both runs used one `bin/notriosctl`,
    built before either started.
  - Each run shows zero tracked changes.

**Search, measured after each import: slower than J5.** The cause is found
below, and it is not search.

| query | J5 Obsidian | J20-A Obsidian | J5 Joplin | J20-A Joplin |
|---|---|---|---|---|
| "the" (187,518 notes) | 9.1 s | 16.4 s | 8.6 s | 14.7 s |
| "quinoa" (385) | 4.9 s | 12.4 s | 4.3 s | 10.7 s |
| "chicken stock" (9,703) | 5.2 s | 12.6 s | 4.8 s | 11.1 s |

Counts are identical. Every query is about 7 s slower, whatever it matches.

### The 7 s is paid on every open, and it is a J18 regression

**One library, two binaries.** J5's Obsidian library was copied, and each
binary searched the copy, alone on the machine:

| binary | "the" | "quinoa" | "chicken stock" |
|---|---|---|---|
| J5 (`fde1330`), library at v27 | 9.00, 9.06 s | 4.82, 5.00 s | 5.02, 5.07 s |
| current (`17aa250`), same copy after migrating to v28 | 15.84, 16.03 s | 12.09, 13.36 s | 12.20, 12.35 s |

J5's binary reproduces J5's numbers, so the machine has not changed.
`notriosctl collections list` reads almost nothing, and it takes 12.4 s with
the current binary (8.4 s user, 3.7 s system). **The cost is in opening the
library, not in search.**

**Profile.** `TestJ20OpenCost` (`internal/store/j20_open_cost_test.go`) opens
the migrated copy twice under `-cpuprofile`. Each time, opening takes 10 ms and
`Bootstrap` takes 12.1–12.2 s:

| step, re-run on every open | CPU over two opens | per open |
|---|---|---|
| `ensureSchemaV28`: rebuild the whole rowid mapping from `documents_fts` | 14.1 s | ~7 s |
| `ensureSchemaV22` → `backfillRevisionObjects` | 9.2 s | ~4.6 s |

**Why steps that are guarded by version still run.**
1. `ensureSchemaV18`, like V4–V17, is unguarded. It runs on every open and ends
   with `PRAGMA user_version = 18`, rewriting a v28 library's version downward.
2. The guarded steps (V21, V22, V27, V28) then read 18 and run again.
3. The last step sets 28 again, so the file on disk always reads 28, and the
   defect cannot be seen from outside.

- **J18's V28 turned a one-time migration into a per-open cost.** Its
  `INSERT OR REPLACE … SELECT document_id, rowid FROM documents_fts` rescans the
  full-text index and rewrites 382,206 mapping rows on every command, read-only
  ones included. J18's record calls that scan "one scan, once, at migration
  time". That is wrong, and so is J18's claim that V28 costs 3 m 49 s once.
- **V22's backfill has cost ~4.6 s per open since before J5.** J5's search
  numbers already contained it, so J5's baseline was never a search
  measurement either.

The rewrite is idempotent, so the mapping stays correct. What it costs is time
and writes on every open.

## J20-B: what the remaining Obsidian candidates must preserve

These come from reading how the importer uses each `vaultFile` field, before
anything is changed.

- **`Fingerprint` (hex SHA-256 of the file) must keep its exact text wherever
  it is compared or composed.**
  - It is written as hex into the inventory fingerprint hash
    (`ItemKey + "\x00" + Fingerprint + "\x00"`), which decides whether a
    checkpoint resumes.
  - It is composed as hex into every note fingerprint (`noteFingerprint`).
  - It is compared with the hex `SHA256` of stored resources and bundle items,
    and with `sha256Hex(raw)` in `readExact`.

  Holding it as `[32]byte` is possible only if every one of those uses encodes
  it back to the identical hex. The inventory fingerprint and item-state
  comparisons prove whether that holds.
- **`FrontmatterSHA` is read once**, into a note source's `metadata_json`. It
  could be held as bytes and encoded there.
- **`AbsPath` is `root` joined with `RelPath`**, and is read only to open files.
  It could be derived instead of stored.
- **`Files` is read only by the source-bundle phase**, and when target IDs are
  copied across after source-ID lookups. Notes appear in both `Files` and
  `Notes` as full struct copies. `Files` needs the order sorted by `RelPath`,
  and a note's `TargetID` must match in both.

No candidate is adopted until `TestJ19InventoryMemory` shows held memory falling
on the full vault. Each must also pass the J17 library comparison, including
item states, and keep the inventory fingerprint identical.

## J20-B, candidate 1: build `Files` only when source is preserved

`readInventory` put every note and asset into `Files` as well as `Notes` or
`Assets`. Only the source-bundle phase reads `Files`, and `runAll` and
`firstPhase` run that phase only with `--preserve-source`, which the CLI
defaults to false. The other uses of `Files`:
- `workTotal` counts it only under `PreserveSource`
- `applyLegacySourceIDs` copies target IDs into it, which is a no-op when it is
  empty

The inventory fingerprint is still hashed from every file during the walk.
`readInventory` now takes `retainFiles`, and `newImportRun` passes
`options.PreserveSource`.

`TestJ19InventoryMemory` on the full 382,206-note vault, run once without
`Files` (the default import) and once with them (`NOTRIOS_J19_RETAIN_FILES`,
the preserve-source shape):

| | before (J19 clones) | default, no `Files` | retained, as control |
|---|---|---|---|
| live heap, inventory | 438.1 MiB | **358.1 MiB** | 438.1 MiB |
| live heap, inventory + link namespace | 560.5 MiB | **480.5 MiB** | 560.5 MiB |
| process peak RSS, not repeatable (see candidate 2) | 1,016 MiB | 900 MiB | 1,023 MiB |

**Verdict: improves, by 80 MiB (18% of the inventory), for every import that
does not preserve source.** The control reproduces 438.1 MiB exactly, so the
80 MiB is `Files` alone. That is half the ~160 MB the heap profile had put
under the two slices, because `Files` shared its strings with `Notes`. Only the
structs were a second copy. A preserve-source import holds what it held
before. The two walks took 2 m 22 s and 1 m 12 s; the first ran from a cold
page cache, and no timing is claimed.

**Proven not to change the default import.** J17's generated 300-note vault
imports to a library table-by-table identical to the post-clone library, apart
from the three random-identifier columns. The checkpoint matches, so the
inventory fingerprint is unchanged, and so do all 329 item states. All
Obsidian importer tests pass, including `TestH9HierarchyRichLinksAndExactSourceBundle`,
which imports with source preserved and so exercises the retained path.

**What remains live**, from the in-use heap profile of the default run
(482 MB):

| holder | MB |
|---|---|
| `readInventory` structs and paths | 108 |
| `bytealg.MakeNoZero` (string building: paths, IDs) | 91 |
| `frontmatterPropertyOrder` slices | 83 |
| link namespace | 128 |
| clones (titles, aliases, property names) | 52 |
| hex hash strings | 45 |

## J20-B, candidate 2: derive the absolute path

`vaultFile.AbsPath` held each file's absolute path for the whole import. It is
read only to open files, in the source-bundle and resource phases and in
`readExact`, and it equals the vault root joined with `RelPath`. The importer
now derives it with `run.absPath(item)`.

**How much one measurement varies, first.** Candidate 1's committed binary was
run again on the full vault. Its inventory held **358.1 MiB, the same as the
first run to the tenth of a MiB**. Live heap after two forced GCs is
repeatable here, so a difference of a few MiB is real. Peak RSS is not
repeatable: 900 MiB and then 822 MiB for the same binary. No candidate claims
an RSS change.

| inventory live heap, 382,206-note vault | MiB | vs candidate 1 |
|---|---|---|
| candidate 1 (committed), two runs | 358.1, 358.1 | — |
| derive `AbsPath` only | 368.3 | **+10.2, worse** |
| derive `AbsPath`, and clone `RelPath` | **352.7** | **−5.4** |

**Deriving the path alone made the inventory larger, and the reason is not
known.** Removing a field should not add memory. The heap profile samples
allocations, so it attributes MB only approximately and cannot place 10 MiB.
This is recorded as measured, not explained. `filepath.Rel` returns a slice of
the absolute path, so `RelPath` was keeping that whole path alive either way.
That is why removing `AbsPath` alone could not free the path bytes.

**Cloning `RelPath` as well lowers the held heap by 5.4 MiB (1.5%). Verdict:
improves, by a small amount, and adopted on that measurement.** Candidate 1's
repeat run shows the difference is not noise. The Obsidian importer tests pass,
and J17's generated vault imports to a library identical to candidate 1's,
apart from the three random-identifier columns.

This candidate was estimated at tens of MB and measured at 5.4 MiB. The
estimate is not repeated anywhere as a result.

## J20-B, candidate 3: derive each file's item-state key

`vaultFile.ItemKey` held `"file:" + RelPath`, a second copy of every path, for
the whole import. It is read in these places:
- the inventory fingerprint hash
- the source-bundle and resource phases
- the notes phase
- `itemState`

It is now built on demand by `vaultFile.itemKey()`, with the same composition
the stored item states were written with. Folders keep their stored key.

| inventory live heap, 382,206-note vault | MiB |
|---|---|
| before (candidate 2 adopted) | 352.7 |
| `ItemKey` derived on demand | **316.7** |

**Verdict: improves, by 36.0 MiB (10% of the inventory).** Live heap repeats
exactly on this measurement (candidate 2's variance run), so the difference is
real. Item keys are proven unchanged: J17's generated 300-note vault imports to
a library identical to candidate 2's, with the same checkpoint (so the same
inventory fingerprint) and all 329 item states, apart from the three
random-identifier columns. The Obsidian importer tests pass, including resume
and preserve-source.

**What remains**, from the in-use heap profile (465 MB, sampled):

| holder | MB |
|---|---|
| link namespace | 143 |
| `readInventory` structs | 81 |
| `frontmatterPropertyOrder` slices | 81 |
| clones (titles, aliases, property names, paths) | 73 |
| string building (IDs, notebook paths) | 58 |
| hex hash strings | 53 |

## J20-B, candidate 4: hold the file hashes as bytes

`vaultFile.Fingerprint` and `FrontmatterSHA` were 64-character hex strings, two
per note for the whole import. They are now `[sha256.Size]byte`. Every place
that compares or composes them calls `fingerprintHex()` or
`frontmatterSHAHex()`, which return exactly the lowercase hex they held before:
- the inventory fingerprint hash
- the bundle, resource and rehash comparisons
- item states
- `noteFingerprint`
- `readExact`
- the source's `frontmatter_sha256` metadata

| inventory live heap, 382,206-note vault | MiB |
|---|---|
| before (candidate 3 adopted) | 316.7 |
| hashes held as bytes | **269.2** |

**Verdict: improves, by 47.5 MiB (15% of the inventory).** Every stored hash
string is proven byte-identical. J17's generated vault imports to a library
identical to candidate 3's, including:
- `document_sources` (with each note's `frontmatter_sha256`)
- all 329 `import_item_states` fingerprints
- the checkpoint's inventory fingerprint

Only the three random-identifier columns differ. The Obsidian importer tests
pass, including J19's fingerprint identity test.

## J20-B: where the Obsidian inventory ended

| step | inventory live heap | change |
|---|---|---|
| J19 (substring clones) | 438.1 MiB | |
| 1: keep `Files` only when preserving source | 358.1 MiB | −80.0 |
| 2: derive `AbsPath`, clone `RelPath` | 352.7 MiB | −5.4 |
| 3: derive `ItemKey` | 316.7 MiB | −36.0 |
| 4: hashes as bytes | **269.2 MiB** | −47.5 |

**−168.9 MiB (−39%) from J19's result**, and −403 MiB from the 672.4 MiB the
inventory held before J19. With the link namespace, 391.6 MiB is held against
560.5 MiB at J19. One step was measured worse and not adopted: deriving
`AbsPath` alone, +10.2 MiB, unexplained.

Each step was measured on the full vault with a live-heap figure that repeats
exactly. Each proved the import unchanged with the J17 library comparison. No
subset result is used. Peak RSS varies too much between identical runs to
support a claim; J20-C measures the whole import instead.

**Not done, and left as found:**
- the link namespace (114 MB)
- `frontmatterPropertyOrder`'s slices (72 MB), which the source bundle needs
  only with `--preserve-source`
- per-note titles and IDs

Each would be a separate measured candidate.

## J20-C: the full Obsidian import again, after J20-B

`baseline_profile.sh` ran with the same corpus, HDD and conditions as J20-A:
one process, a warm page cache, and a clean tree at `06fbda9`, which includes
J20-B's four changes and J21's open fix. The import completed with J20-A's
counts: 382,206 notes, 780 notebooks, 382,206 link indexes, 7,654 batches, and
checkpoint `completed`.

| Obsidian vault, 382,206 notes | J5 | J20-A | J20-C |
|---|---|---|---|
| wall | 16,261.5 s | 5,245.5 s | 5,278.2 s |
| user / sys | not recorded | 4,881.3 s / 389.4 s | 4,933.4 s / 382.9 s |
| peak RSS | 1,720.5 MiB | 1,197.0 MiB | **842.6 MiB** |
| search "the" (187,518) | 9.1 s | 16.4 s | **3.9 s** |
| search "quinoa" (385) | 4.9 s | 12.4 s | **0.07 s** |
| search "chicken stock" (9,703) | 5.2 s | 12.6 s | **0.35 s** |

- **Import time is unchanged:** +0.6% on one run each, inside the run-to-run
  noise J19 measured. J20-B was a memory change, and no time effect is claimed.
- **Peak RSS is 354 MiB lower than J20-A (−30%) and 878 MiB lower than J5
  (−51%).** Each import ran once, and peak RSS is not exactly repeatable: two
  runs of one inventory test gave 900 and 822 MiB. The drop here is more than
  four times that spread. J20-B measured 169 MiB less held by the inventory,
  so the rest of the difference is not attributed to anything.
- **Search reflects J21's fix**, which is in this tree. The J21 record measures
  it on its own.

Against J5 at 382,206 notes, the Obsidian import is **3.1× faster with 51% less
peak memory**, and every search probe is faster.
