# v1.0 J32 — import performance, measured before it is changed

J32 investigates the findings in `performance-code-review.md` (2026-09-17), and
keeps a change only if a benchmark written before it shows that it is faster.
The rule for "faster", fixed before any candidate was measured, is in PLAN.md.

This file is J32's record. J32-A built the harness and measured the baseline
below. Each later slice adds its hypothesis, declared metric, result and
disposition, and J32-M completes the table of findings.

## The harness (J32-A)

| file | what it does |
|---|---|
| `make_corpus.py` | generates the six synthetic corpora from a fixed seed, each with a manifest hash |
| `run_bench.py` | builds a harness binary at a commit; measures one case for one or two binaries |
| `run_matrix.sh` | runs every case, one after another |
| `compare.py` | applies PLAN.md's rule to a baseline and a candidate, per metric |
| `summarize.py` | prints the table below from result files |
| `cmd/notriosctl/j32_import_bench_test.go` | the measured process: the CLI's own `import`, in-process |
| `store.PreparedStatements()` | process-wide count of statements prepared, incremented at the store's only prepare site |

**One import per process.** Peak RSS is a per-process maximum, so each measured
import is a fresh process of a test binary compiled once per commit. The
process runs the CLI's `import` command, so flag parsing, configuration and the
job wrapper are all measured, and it reports:
- wall time around the command;
- bytes and objects allocated;
- the live heap after a forced GC;
- the prepared-statement count.

`/usr/bin/time` around the process supplies peak RSS and CPU. No flag or
environment variable reaches the shipped CLI: the test binary reads its
arguments from its own environment and is skipped by an ordinary `go test`.

**Isolation.** Each run has its own directory holding HOME, every XDG root and
TMPDIR, and `DBUS_SESSION_BUS_ADDRESS` is unset, so no owner configuration,
keychain or shared temp directory is touched. The corpus manifest is checked
before and after each case, so a case that read different bytes, or wrote into
its corpus, is refused.

**Scenarios.**
- `fresh`: imports into an empty library.
- `reimport`: imports the same corpus again into the library a fresh import
  made. That library is built once per binary and copied before every measured
  run, so every reimport starts from the same bytes. Nothing changes, so a
  reimport measures what an unchanged vault still costs.

**Runs.**
- One unmeasured warm-up per binary, then five measured runs.
- With two binaries, the order alternates run by run.
- Before each run the one-minute load average must be at or below 1.0, and the
  value is recorded.

The desktop's system monitor holds the load near 0.9 on this machine, so the
60 measured runs started at loads between 0.79 and 1.00. The gate costs minutes between runs while
the load decays from the previous import.

### The corpora

| shape | kind | content | why |
|---|---|---|---|
| `joplin-10k` | Joplin RAW | TestJoplinImporterProfile's shape, 10,000 notes, tags and resources | the ordinary corpus |
| `obsidian-10k` | Obsidian | TestObsidianImporterProfile's shape, 10,000 notes, aliases, links, assets | the ordinary corpus |
| `link-dense` | Obsidian | 4 notes of 1 MiB, a link every ~350 bytes, 2 of them one line | F8, §3.1A |
| `collision` | Obsidian | 5,000 `index.md` in separate folders, aliases shared by 100 notes each | F9 |
| `near-limit-obsidian` | Obsidian | 4 notes of 60 MiB among 200 small ones | F5 |
| `near-limit-joplin` | Joplin RAW | the same, as RAW items | F5 |

`link-dense` was 20 notes at first. One unmeasured import of those took 1,300 s,
with 149 MiB peak RSS, which would have made each candidate comparison several
hours long. The cost grows within a note, so four notes show it as well as
twenty (generator version 2). Five times the notes took 6.7 times as long,
so the cost also grows with the number of notes. The other five corpora are
byte-identical between the two generator versions: their manifest hashes match
the ones recorded in the baseline files.

### Reproducing

```sh
W=/path/outside/the/checkout
python3 performance/v1.0-j32/make_corpus.py "$W"
python3 performance/v1.0-j32/run_bench.py build <commit> "$W" baseline
bash performance/v1.0-j32/run_matrix.sh "$W" "$W/results/baseline" baseline
# a candidate, alternating with the baseline, on the case its slice declares:
python3 performance/v1.0-j32/run_bench.py build <candidate-commit> "$W" candidate
python3 performance/v1.0-j32/run_bench.py run "$W" "$W/out.json" link-dense/fresh baseline,candidate
python3 performance/v1.0-j32/compare.py "$W/out.json" baseline candidate wall_seconds
```

Run it with nothing else on the machine: no tests, builds or other imports.

## The baseline (J32-A)

Commit `bfcb372` (the harness commit), built with go1.27.0 linux/amd64. It ran
2026-09-22 from 02:42 to 10:21 UTC, on 8 CPUs, with the work directory on an
ext4 SSD (HD2). Page caches were warm; there is no passwordless sudo here to
drop them. Each cell is the median of five runs ± the range (maximum minus
minimum), which is the noise a candidate has to beat. The records are in
`baseline/`, with the work directory's path replaced by `<work>`.

| case | notes | wall s | notes/s | peak RSS MiB | allocated MiB | allocations k | statements |
|---|---:|---:|---:|---:|---:|---:|---:|
| collision/fresh | 5,500 | 93.2 ± 4.7 | 59.0 ± 2.9 | 36.5 ± 0.8 | 183.6 ± 1.3 | 2,370.6 ± 0.7 | 220,561 ± 2 |
| collision/reimport | 5,500 | 46.5 ± 1.8 | 118.3 ± 4.5 | 37.7 ± 3.1 | 165.2 ± 0.5 | 2,121.8 ± 0.2 | 108,428 ± 0 |
| obsidian-10k/fresh | 10,000 | 88.1 ± 0.6 | 113.6 ± 0.7 | 43.7 ± 0.7 | 367.8 ± 0.6 | 4,791.6 ± 0.6 | 381,524 ± 0 |
| obsidian-10k/reimport | 10,000 | 52.5 ± 1.2 | 190.4 ± 4.3 | 43.5 ± 0.6 | 311.0 ± 0.4 | 4,061.5 ± 0.4 | 211,483 ± 0 |
| joplin-10k/fresh | 10,000 | 81.3 ± 1.4 | 123.0 ± 2.1 | 33.4 ± 1.3 | 337.0 ± 0.4 | 4,226.7 ± 0.1 | 301,812 ± 2 |
| joplin-10k/reimport | 10,000 | 30.1 ± 0.5 | 331.8 ± 5.4 | 31.8 ± 0.4 | 301.6 ± 0.4 | 3,735.4 ± 0.3 | 131,768 ± 2 |
| link-dense/fresh | 4 | 193.1 ± 1.4 | 0.021 ± 0.000 | 73.0 ± 13.1 | 11,019.3 ± 0.2 | 378.8 ± 0.2 | 56,024 ± 0 |
| link-dense/reimport | 4 | 121.7 ± 0.7 | 0.033 ± 0.000 | 68.9 ± 8.9 | 10,942.9 ± 0.1 | 264.5 ± 0.1 | 28,031 ± 0 |
| near-limit-obsidian/fresh | 204 | 525.6 ± 6.2 | 0.388 ± 0.004 | 997.5 ± 112.4 | 6,939.9 ± 0.2 | 74.9 ± 0.2 | 6,051 ± 4 |
| near-limit-obsidian/reimport | 204 | 320.8 ± 7.5 | 0.636 ± 0.015 | 1,046.3 ± 1.0 | 5,527.1 ± 0.1 | 66.8 ± 0.1 | 3,176 ± 6 |
| near-limit-joplin/fresh | 204 | 396.6 ± 6.3 | 0.514 ± 0.008 | 1,445.9 ± 60.2 | 6,039.1 ± 0.1 | 49.0 ± 0.1 | 3,566 ± 2 |
| near-limit-joplin/reimport | 204 | 195.8 ± 3.9 | 1.042 ± 0.021 | 1,590.0 ± 1.5 | 4,626.9 ± 0.1 | 49.7 ± 0.1 | 1,303 ± 2 |

**Reading it.**
- **Wall-time noise** is 1–5% of the median on every case, so a candidate has
  to beat the baseline by about that much.
- **Peak RSS is noisier.** On `link-dense` and `near-limit-obsidian` fresh
  imports its range is 11–18% of the median.
- **The statement count varies by 2 to 6** between runs of some cases, even
  though the corpus is identical. That is a few statements from work whose
  count depends on timing, not on content. The rule treats the range as noise
  for counts as well, so a count win has to exceed it.
- **Four costs stand out:**
  - **Link-dense allocation.** A fresh import of four 1 MiB notes allocates
    11 GB. The review's §3.1A (replacements spliced one at a time) and F8
    (link coordinates rescanned per link) both predict a cost that grows
    within a note. J32-D and J32-E are measured on this case.
  - **No-op reimports still do most of the work.** An unchanged `link-dense`
    corpus takes 63% of its fresh-import time, and `collision` takes 50%.
    J32-I and J32-J are about this.
  - **Near-limit memory.** Four 60 MiB notes take a fresh Obsidian import to
    about 1 GB peak RSS, and a no-op Joplin reimport to 1.59 GB. J32-K (F5)
    is measured on these cases.
  - **Prepared statements.** The ordinary Obsidian corpus prepares about 38
    statements per note on a fresh import, and 21 on a no-op reimport. J32-B
    (F1) is measured by this count and by wall time.

## J32-B — F1, reuse the block and link insert statements: **kept**

`rebuildDocumentBlocksLocked` and `rebuildDocumentLinksLocked` called
`execPreparedLocked` once per block and once per link, and that prepares and
finalizes a statement every call. Each now prepares one statement before its
loop and rebinds it per row (`repeatedStatement` in `internal/store/sqlite.go`),
finalized on every exit. Nothing is cached across a transaction, so no
statement outlives a migration or a restore.

Declared before the change: prepared-statement count and wall time on
`obsidian-10k/fresh`, with `link-dense/fresh` as the scaling case that must not
regress. Baseline `bfcb372`, candidate `c1a7cc5`, five runs each, alternating.
Records in `j32b/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| obsidian-10k/fresh | prepared statements | 381,522 | 281,520 | 2 | −26.2% | better |
| obsidian-10k/fresh | wall s | 84.32 | 80.01 | 3.17 | −5.1% | better |
| obsidian-10k/fresh | allocations | 4,791,440 | 4,731,560 | 348 | −1.3% | better |
| link-dense/fresh | prepared statements | 56,022 | 30,576 | 0 | −45.4% | better |
| link-dense/fresh | wall s | 185.44 | 183.71 | 0.33 | −0.9% | better |

Nothing regressed beyond noise on either case: peak RSS, bytes allocated, live
heap and system time all stayed within the baseline's range.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. `j17_compare.py` between a library the baseline
binary imported and one the candidate imported reports every table identical,
including all 40,000 block rows and 30,000 link rows; the four differing tables
differ only in volatile identifier columns (`database_id`, `replica_id`,
revision `id`, `current_revision_id`, job `id`), which are random per import.

The baseline's own medians in this comparison differ from the recorded
baseline above (84.3 s against 88.1 s on `obsidian-10k/fresh`), because the
machine was in a different state hours later. That is why a candidate is
compared only with a baseline measured beside it, run by run.

## J32-C — §3.1B, build `PropertyOrder` only when source is preserved: **kept**

`readInventory` parsed each note's frontmatter property order and kept a slice
of it for the whole import, although the only reader is the source-bundle
phase, which runs only under `--preserve-source`. Joplin's scan already gated
the same field. It is now behind the `retainFiles` parameter `readInventory`
already takes, and a test pins both directions and that the inventory
fingerprint does not change either way.

Declared before the change: live heap and peak RSS on a property-rich vault.
Baseline `7e272f3` (which includes J32-B), candidate `4464a75`, five runs each,
alternating. `property-rich` is 10,000 notes with about 25 frontmatter
properties each; it was added for this slice, so it has no row in the J32-A
table. Records in `j32c/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| property-rich/fresh | peak RSS KiB | 59,248 | 40,924 | 1,204 | −30.9% | better |
| property-rich/fresh | allocations | 3,717,540 | 3,408,890 | 183 | −8.3% | better |
| property-rich/fresh | allocated bytes | 515.9 M | 493.1 M | 380 K | −4.4% | better |
| property-rich/fresh | live heap bytes | 645,080 | 643,632 | 117,360 | −0.2% | within noise |
| property-rich/fresh | wall s | 68.00 | 68.50 | 0.99 | +0.7% | within noise |
| obsidian-10k/fresh | peak RSS KiB | 44,540 | 43,184 | 1,196 | −3.0% | better |
| obsidian-10k/fresh | wall s | 80.50 | 80.50 | 0.62 | 0.0% | within noise |

**Of the two declared metrics, one was the wrong one.** Peak RSS improved by
31% on the property-rich vault, far beyond its noise, and by 3% on the ordinary
corpus. Live heap did not move, and could not have: it is read after the
command returns, when the inventory has already been released. What this change
removes is held *during* the import, which is what peak RSS measures and live
heap cannot. The slice is kept on peak RSS; live heap is recorded as unchanged
with that reason rather than treated as a failure to improve.

Wall time is unchanged on both cases, within noise. Parsing that no longer
happens is a small part of an import dominated by other work.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. The property-rich vault imported with
`--preserve-source` by each binary produces libraries whose
`source_bundle_items` table — the 10,000 rows that carry the recorded property
order — is identical; only the volatile identifier columns differ.

## J32-D — §3.1A, assemble a rewritten body in one pass: **kept**

`rewriteObsidianLinks` spliced each replacement into the body on its own
(`body[:start] + text + body[end:]`), so a note with thousands of links copied
itself thousands of times. `applyReplacements` now walks the body once into a
builder sized in advance.

Overlapping replacements keep the old splices. Two can overlap only when a
Markdown match encloses a wiki link *and* both resolve, which needs a vault
file whose name is itself a wiki link (`[[Target]].md`); a one-pass assembly
cannot reproduce what descending splices did to those bytes, and this slice
changes assembly, not meaning. The one case the old loop never decided —
two replacements starting at the same byte, which its sort ordered arbitrarily
— is now decided (widest first) and a test says so.

Pinned first, in `j32d_rewrite_test.go`: ten bodies byte for byte against the
old code (nested, adjacent, repeated, both ends, multi-byte, anchors, display
text, unresolved), a 400-link body, and `applyReplacements` held to the splice
loop itself across eight replacement sets.

Declared before the change: allocations and wall time on `link-dense/fresh`.
Baseline `100e3f8` (which includes J32-B and J32-C), candidate `bf33641`, five
runs each, alternating. Records in `j32d/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| link-dense/fresh | allocated bytes | 11.55 G | 270.4 M | 157 K | −97.7% | better |
| link-dense/fresh | wall s | 184.14 | 180.94 | 2.62 | −1.7% | better |
| link-dense/fresh | system s | 12.54 | 10.24 | 0.92 | −18.3% | better |
| link-dense/fresh | peak RSS KiB | 76,940 | 75,112 | 12,304 | −2.4% | within noise |
| obsidian-10k/fresh | allocated bytes | 384.4 M | 379.1 M | 507 K | −1.4% | better |
| obsidian-10k/fresh | wall s | 79.84 | 80.01 | 1.21 | +0.2% | within noise |

**11.55 GB becomes 270 MB**: the copying was 98% of everything the import
allocated on this corpus. Wall time falls only 1.7%, because the remaining cost
on these notes is elsewhere — the per-link coordinate rescans J32-E measures.
The ordinary corpus, where notes hold three links each, gains 1.4% of
allocations and nothing measurable in time, and regresses nowhere.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. Both corpora imported by each binary produce
identical `documents`, `document_blocks` and `document_links` — 40,000 blocks
and 30,000 links on the ordinary corpus, 12,677 links on the link-dense one —
with only volatile identifier columns differing.

## J32-E — F8, one-pass link coordinates: **kept**

`decorateCandidate` called `lineColumn`, which decodes the body from byte zero
for every candidate, so a note with thousands of links decoded itself thousands
of times and a note that is one long line was the worst case. Each extraction
pass now carries a `lineCursor` forward. Asked for an offset behind it, the
cursor starts over, so an answer never depends on the order it is asked in.
`lineColumn` stays as the reference the tests compare against.

Declared before the change: wall time on `link-dense/fresh`. Baseline
`81d44ee` (which includes J32-B through J32-D), candidate `821e82c`, five runs
each, alternating. Records in `j32e/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| link-dense/fresh | wall s | 179.98 | 32.90 | 1.42 | **−81.7%** | better |
| link-dense/fresh | user s | 170.50 | 22.47 | 1.12 | −86.8% | better |
| link-dense/fresh | notes/s | 0.022 | 0.122 | 0.000 | ×5.5 | better |
| link-dense/fresh | peak RSS KiB | 74,464 | 75,440 | 4,548 | +1.3% | within noise |
| obsidian-10k/fresh (9 runs) | wall s | 80.18 | 80.56 | 2.54 | +0.5% | within noise |
| obsidian-10k/fresh (9 runs) | system s | 5.99 | 5.97 | 0.18 | −0.3% | within noise |

**The corpus imports 5.5× faster.** With J32-D's assembly already in, what was
left on these notes was this rescanning: 170 s of user CPU becomes 22 s.

**One flagged regression did not survive a second look.** At five runs the
ordinary corpus showed system time 5.99 → 6.19 s against a range of 0.15,
which the rule counts as worse. A change that only removes user-space decoding
has no reason to spend more kernel time, so the case was measured again at nine
runs per binary: 5.99 → 5.97 s, and every other metric within noise. The
five-run range was simply narrower than the metric's own spread. Both records
are kept in `j32e/`; the nine-run one is the result.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. Both corpora imported by each binary produce
identical `document_links` — 30,000 rows on the ordinary corpus and 12,677 on
the link-dense one, which is where byte offsets, rune columns and contexts are
stored — and identical blocks and documents, with only volatile identifier
columns differing.

## J32-F — F9, collision-heavy namespace construction: **rejected, no change**

F9 said `appendUnique`'s linear scans cost on a vault where many notes share a
name. They do not, at this size:

- a CPU profile of a fresh collision import
  (`j32n/collision-fresh-before.pprof`) puts `buildLinkNamespace` and
  `appendUnique` below the profiler's 0.38 s threshold, under 0.5% of 77 s of
  samples;
- `BenchmarkBuildLinkNamespaceCollision` builds the namespace for the corpus's
  shape (5,000 notes named `index.md`, 50 aliases shared by 100 notes each) in
  **213 ms**, against **223 ms** for the same 5,000 notes with no shared alias.
  Colliding names make it slightly cheaper: fewer distinct map keys offset the
  scans.

A set would replace 0.2% of an import with the memory the slice was written to
avoid, so nothing ships. The benchmark stays, as the evidence and as the thing
that would notice if this ever became a cost.

## J32-N — the collection scope defeated the title index: **kept**

Found while measuring J32-F. Every link resolved by title ran

```sql
collection_id = COALESCE(NULLIF(?, ''), collection_id)
```

which is not a constant SQLite can seek with, so the plan was
`SCAN documents USING COVERING INDEX documents_title_idx` plus a temp B-tree
for the ordering — a full index scan per link, and one that grows with the
library rather than with the import. It was 46% of a fresh collision import,
35 s of 77 s of samples, at 5,500 notes.

`CollectionScopeSQLFor` writes `collection_id = ?` when the caller has a
collection and keeps today's predicate when it does not, so an empty collection
still means every collection and the bound parameter is unchanged. Both
link-resolution lookups use it: documents by title, and resources by filename.

Declared before the change: wall time on `collision/fresh`. Baseline `15ee070`,
candidate `6bb843f`, five runs each, alternating. Records in `j32n/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| collision/fresh | wall s | 87.20 | 52.43 | 2.81 | −39.9% | better |
| collision/fresh | user s | 73.56 | 38.43 | 1.63 | −47.8% | better |
| collision/fresh | notes/s | 63.1 | 104.9 | 2.05 | +66.3% | better |
| obsidian-10k/fresh | wall s | 80.15 | 80.02 | 1.45 | −0.2% | within noise |
| obsidian-10k/fresh | user s | 76.36 | 75.30 | 0.83 | −1.4% | better |

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. Both corpora imported by each binary produce
identical `document_links` — 16,000 rows on the collision corpus and 30,000 on
the ordinary one, including every row's resolution status and target — and the
importer reports the same warnings, all 101 ambiguity warnings included. A
scope test pins the rendered predicate for a named collection, an empty one and
a blank one, with and without a table alias.

## J32-G — §3.1C, hash Markdown from the bytes already read: **reverted**

`readInventory` streams each file to a hash and then reads it again to take its
title and aliases, so every Markdown note is read twice — 240 MB of second
reads for the four notes of the near-limit corpus. The candidate read a note
once, bounded one byte past the 64 MiB limit, and hashed those bytes.

Declared before the change: wall time on `near-limit-obsidian/fresh`, where the
second read is largest. Baseline `948260f`, five runs each, alternating.
Records in `j32g/`.

| attempt | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| `io.ReadAll` (`d7a7cf4`) | wall s | 498.66 | 500.93 | 4.21 | +0.5% | within noise |
| `io.ReadAll` | allocated bytes | 7.28 G | 7.54 G | 215 K | +3.6% | worse |
| stat-sized buffer (`82ab0af`) | wall s | 495.87 | 497.60 | 6.20 | +0.3% | within noise |
| stat-sized buffer | allocated bytes | 7.28 G | 7.78 G | 146 K | +6.9% | worse |
| ordinary corpus | wall s | 80.01 | 80.27 | 0.46 | +0.3% | within noise |
| ordinary corpus | allocated bytes | 379.2 M | 375.0 M | 336 K | −1.1% | better |

**Why it does not pay.** The second read comes from the page cache, which is
warm, so removing it saves almost nothing. Meanwhile holding a 60 MiB note in
one buffer allocates more than the streaming hash's 32 KiB buffer plus
`os.ReadFile`'s stat-sized one: `io.ReadAll` grows by doubling, and
`bytes.Buffer.ReadFrom` grows past a capacity hint as well, so the second
attempt was worse than the first. Both are kept in `j32g/`.

**What is not shown.** Caches cannot be dropped on this machine (no
passwordless sudo, as J20 recorded), so the cold-cache case — where halving the
bytes read from disk would matter — was not measured. That is a limit of the
measurement, not evidence either way, and it is why the finding is recorded as
reverted here rather than closed as wrong.

The ordinary corpus did gain 1.1% of allocations, which is not the declared
metric and is not worth a second way to read a file.

## J32-Q — a block's text, copied three times: **kept**

The allocation profile of a fresh near-limit import
(`j32r/near-limit-before.allocs`) put 3.31 GB of 6.95 GB in block extraction.
It was three copies, not one:

- `identify` joined its five parts with `strings.Join` and converted the result
  with `[]byte(...)`: two whole copies of a block's text per block;
- `normalize` split the text into lines and joined them back even when no line
  had trailing whitespace and there was no carriage return;
- each block's text was collected as a `[]string` of its lines and joined.

Now: a block's text is written once into a builder grown from the block's own
extent, or is the line itself when the block has one line; `identify` appends
into a scratch buffer the next block reuses; `normalize` returns what it was
given when nothing would change.

**Two attempts measured worse and were replaced, not kept.**

| attempt | one large note | many small blocks |
|---|---|---|
| before the slice | 175.5 MB, 121 allocs | 56.3 MB, 147,949 allocs |
| builder grown by doubling | 206.1 MB, 140 allocs | — |
| streaming `sha256.New` per block | — | 55.8 MB, 214,604 allocs |
| what shipped | **103.4 MB, 86 allocs** | **54.7 MB, 111,286 allocs** |

A builder left to double allocates about twice a block's size, which is worse
than the join it replaced; and `sha256.New` returns an interface, so handing it
the digest array makes that array escape once per block. Both are why the
shipped version hashes through a reused buffer instead of streaming.

Declared before the change: allocated bytes on `near-limit-obsidian/fresh`.
Baseline `e8469f8`, candidate `994894e`, five runs each, alternating. Records
in `j32q/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| near-limit-obsidian/fresh | allocated bytes | 7.28 G | 5.85 G | 272 K | −19.6% | better |
| near-limit-obsidian/fresh | allocations | 73,984 | 70,477 | 245 | −4.7% | better |
| near-limit-obsidian/fresh | wall s | 520.97 | 508.68 | 32.38 | −2.4% | within noise |
| link-dense/fresh | allocated bytes | 269.6 M | 254.7 M | 216 K | −5.6% | better |
| obsidian-10k/fresh | allocated bytes | 379.0 M | 348.8 M | 589 K | −8.0% | better |
| obsidian-10k/fresh | allocations | 4,661,580 | 4,401,560 | 340 | −5.6% | better |

Wall time moves within noise everywhere: this is allocation, and the import is
bound by other work. Nothing regressed on any case.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. Reference implementations of `identify` and
`normalize` — the code this replaced — are kept in the tests and compared
against the new code across sixteen texts and five block kinds. Both corpora
imported by each binary produce identical `document_blocks`, 40,000 rows and
54, with every ID, hash, slug, ordinal and offset equal, and identical
`document_links`.

## J32-S — eleven allocations per block: **kept**

`MaxBlocksPerDocument` caps a note at 10,000 blocks, so the many-blocks
benchmark's 111,000 allocations were about eleven per block: a regexp replace
to strip each list marker (and a `sync.Pool` entry per call), the occurrence
built as a string only to be hashed, the hash and the ID allocated twice each,
and `Slugify` lowercasing a whole heading into a copy before collecting runes
into a slice to convert once more.

Now the list marker, heading and table-row shapes are matched by hand, the
occurrence is appended as digits, the hash and ID are written into stack arrays
and converted once each — two strings are stored, so two allocations are the
floor — and `Slugify` writes bytes into a buffer sized from its input.

**The regexps stay in the package** as the reference the tests hold the
matchers to, across 4,050 lines covering every marker shape, each whitespace
byte, both digit terminators and the near-misses (`- `, `1.`, `1 item`,
`--`, `#######`, `|`, `x|`). What is recognised cannot drift without the tests
saying so.

| shape | before J32-Q | after J32-Q | after J32-S |
|---|---|---|---|
| one large note | 175.5 MB, 121 allocs | 103.4 MB, 86 allocs | 103.4 MB, 80 allocs |
| many small blocks | 56.3 MB, 147,949 allocs, 324 ms | 54.7 MB, 111,286 allocs, 274 ms | **52.9 MB, 36,843 allocs, 196 ms** |

Declared before the change: allocations on `obsidian-10k/fresh`, where a note
has few blocks and the per-block cost is the whole cost. Baseline `9ade826`,
candidate `0377418`, five runs each, alternating. Records in `j32s/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| obsidian-10k/fresh | allocations | 4,401,480 | 4,081,300 | 347 | −7.3% | better |
| obsidian-10k/fresh | allocated bytes | 348.6 M | 337.2 M | 116 K | −3.3% | better |
| obsidian-10k/fresh | wall s | 82.49 | 79.96 | 12.12 | −3.1% | within noise |
| near-limit-obsidian/fresh | allocations | 70,470 | 65,358 | 146 | −7.3% | better |
| near-limit-obsidian/fresh | wall s | 502.87 | 501.16 | 8.11 | −0.3% | within noise |

Wall time stays within noise, and the ordinary corpus's range was unusually
wide this run (12.1 s against the 0.5–2.8 s of earlier runs), which is its own
reminder that a 3% wall-time move here would not have been a result.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. Both corpora imported by each binary produce
identical `document_blocks` — 40,000 rows and 16,500 — with every ID, hash,
slug, ordinal and offset equal.

## J32-T — block storage reused across notes: **kept, after a capped second attempt**

`Extract` allocated a block slice, a line table and two maps for every note,
and an import parses every note in a library. An `Extractor` keeps them; the
store holds one and resets it after each note is written. `Extract` remains for
callers that keep blocks around, since the reused buffers are valid only until
the next call.

**The first attempt improved allocations and regressed live heap by 77 MB.**
`Reset` cleared what the buffers referred to and kept their capacity — which is
the point of reuse — but a 60 MiB note's line table is about four million
entries, so after one such note the process held 77 MB for good:

| attempt | case | allocated bytes | live heap | verdict |
|---|---|---:|---:|---|
| uncapped (`e2d08ad`) | near-limit | 5.85 G → 4.95 G (−15.4%) | 639 KB → **77.0 MB** | worse |
| capped (`eb9cd33`) | near-limit | 5.849 G → 5.848 G (−0.02%) | 644 KB → 641 KB | within noise |

`Reset` now gives back a line table, block slice or scratch buffer past its cap
(65,536 lines, 4,096 blocks, 1 MiB). **The near-limit byte gain went with it**,
and that is the honest reading: on that corpus the 15% came from retaining the
table, which is the thing that cannot be kept. What remains there is the
allocation count.

Declared before the change: allocated bytes and allocations. Baseline
`370cd02`, candidate `eb9cd33`, five runs each, alternating. Both records are
in `j32t/`, the first attempt's under `-uncapped`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| obsidian-10k/fresh | allocated bytes | 337.0 M | 283.8 M | 572 K | −15.8% | better |
| obsidian-10k/fresh | allocations | 4,081,170 | 3,800,920 | 229 | −6.9% | better |
| obsidian-10k/fresh | live heap bytes | 1.155 M | 1.152 M | 14 K | −0.3% | within noise |
| near-limit-obsidian/fresh | allocations | 65,323 | 60,106 | 76 | −8.0% | better |
| near-limit-obsidian/fresh | peak RSS KiB | 1,140,580 | 1,088,960 | 123,176 | −4.5% | within noise |

Across 200 notes the parser's own benchmark goes from 15.5 MB and 76,246
allocations to 2.6 MB and 66,210, and from 137 ms to 113 ms.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. A reuse test runs notes of decreasing block
count through one `Extractor` and compares every block with what a fresh
`Extract` returns; a reset test proves the buffers refer to nothing afterwards;
a third proves a note past the caps hands its buffers back while an ordinary
one still reuses. Both corpora imported by each binary produce identical
`document_blocks`, 40,000 rows and 616.

## J32-U — is an edit script the right model? **Yes, and it was already half-built**

The owner's question: import *compiles* a note, so should everything done to the
body before writing — canonicalizing newlines, rewriting link ranges,
prepending frontmatter, trimming — be collected as edits against the original
bytes and applied once, the way a text editor's rope handles edits?

**The answer has two halves.**

*A rope, no.* A rope pays for itself when a program makes many small edits to
one large text and keeps it live between them. An import makes one pass and
writes the result once, and J32-D already turned that pass into a single
assembly. The remaining copies were not edits but *re-materializations* — the
same bytes converted between `[]byte` and `string`, split and rejoined. A rope
would add a structure over the text without removing one conversion, and would
have to flatten itself for the database write. Go already gives the operation a
rope would provide: a string slice shares its bytes and copies nothing.

*An edit list, yes — and J32-D had already made one.* `rewriteObsidianLinks`
produced a list of replacements and applied it in one pass, then
`augmentFrontmatter` copied the result again to rebuild the frontmatter around
it. So the edit script was there; it just stopped short of the last stage.
`planObsidianLinks` now returns the edits, and `canonicalBodyOnce` writes the
frontmatter and the edited body together, locating the frontmatter by offset
instead of copying it out. The staged path remains for what the one pass
declines: a rewrite straddling the frontmatter delimiters, or overlapping
replacements.

**No separate intermediate representation was needed**, and that is the
result: a general edit DSL would have been a new IR on the import path, and the
measurement below was obtained by extending the list the code already had.

Declared: allocated bytes. Baseline `bae3c0b`, candidate `09e2cbe`, five runs
each, alternating; the near-limit case re-measured at nine. Records in `j32u/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| obsidian-10k/fresh | allocated bytes | 283.7 M | 268.8 M | 379 K | −5.2% | better |
| obsidian-10k/fresh | allocations | 3,801,040 | 3,720,910 | 309 | −2.1% | better |
| obsidian-10k/fresh | user s | 74.10 | 73.58 | 0.23 | −0.7% | better |
| near-limit-obsidian/fresh (9 runs) | allocated bytes | 5.85 G | 5.09 G | 237 K | −12.9% | better |
| near-limit-obsidian/fresh (9 runs) | live heap bytes | 641,344 | 641,560 | 1,952 | +0.03% | within noise |

At five runs the near-limit live heap was flagged worse — 641,080 → 643,384
against a range of 1,512 — so the case was re-measured at nine runs per binary:
a 216-byte difference against a 1,952-byte range. The narrow range was the
artefact, as in J32-E. Both records are kept.

**What it means for the two slices it was measured against.**
- **J32-P (assemble the canonical body once) is withdrawn**: this *is* that
  change, arrived at from the other direction.
- **J32-O stands, with its target now exact.** The profile taken after this
  change (`j32u/near-limit-after.allocs`) still shows 480 MB in
  `splitFrontmatter`, all of it from `markdownTitle`, which converts a whole
  note to `[]byte` and back to find its frontmatter while building the
  inventory. `frontmatterRegion` already answers that by offset.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. The one-pass assembly is held to the staged
path byte for byte across eight note shapes, 3,000 randomly assembled notes and
three note paths, and `frontmatterRegion` is held to the splitter that returns
the bytes. Both corpora imported by each binary produce identical `documents`
and `document_revisions` apart from the volatile identifier columns, so every
canonical body is unchanged.

**Where the 6.95 GB stands.** After J32-Q, S, T and U a fresh near-limit import
allocates **4.85 GB**, down 30%. What is left: the body read back out of SQLite
(721 MB, J32-R), string conversions (720 MB), the line table (497 MB),
`markdownTitle`'s frontmatter split (480 MB, J32-O), block identity (480 MB),
and the note read twice (480 MB, which J32-G measured and did not pay for).

## J32-O — read a note's title without copying the note: **kept**

J32-U's profile put 480 MB of a 4.85 GB near-limit import in `splitFrontmatter`,
all of it from `markdownTitle`. That function copied a whole note twice to
inspect a handful of its bytes:

- `splitFrontmatter` converted the body to `[]byte`, split it, and converted
  both halves back;
- `strings.Split(body, "\n")` built a slice of every line — about four million
  for a 60 MiB note — to find the first `# ` heading.

Both are now slices of the body the caller already holds: the frontmatter is
located by offset with `frontmatterRegion` (added by J32-U), and the lines are
walked one at a time with `strings.Cut`.

On a 7.5 MB note the function goes from **18.19 MB and 5 allocations to 32
bytes and 1**, and runs 2.9× faster.

Declared: allocated bytes on `near-limit-obsidian/fresh`. Baseline `09e2cbe`,
candidate `11faa99`, five runs each, alternating. Records in `j32o/`.

| case | metric | baseline | candidate | baseline range | change | verdict |
|---|---|---:|---:|---:|---:|---|
| near-limit-obsidian/fresh | allocated bytes | 5.09 G | 4.56 G | 76 K | −10.5% | better |
| near-limit-obsidian/fresh | allocations | 58,719 | 58,037 | 99 | −1.2% | better |
| near-limit-obsidian/fresh | wall s | 502.95 | 496.51 | 13.66 | −1.3% | within noise |
| obsidian-10k/fresh | allocated bytes | 268.6 M | 262.8 M | 353 K | −2.1% | better |
| obsidian-10k/fresh | allocations | 3,720,870 | 3,680,800 | 365 | −1.1% | better |

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh`
passes with no temp entry left. A note is addressed by its title, so the old
reading is kept in the tests and compared against the new one across twenty
shapes, 3,000 randomly assembled bodies and four note paths — including
unclosed frontmatter, CRLF, an empty title key, `#` without a space, and an
indented heading. Both corpora imported by each binary produce identical
`documents` and `document_links` apart from the volatile identifier columns.

## Found along the way

- **J36, Obsidian partial-path links.** A link such as `[[topic-00001/index]]`
  is reported as ambiguous, and left unresolved, although exactly one file ends
  with that path. The cause is that `resolveNoteID` and `resolveAssetID` have
  no suffix step. Under the J8 rule this is a plan item of its own, not a J32
  change. The `collision` corpus exercises it, so whichever of J32-F and J36
  lands second is measured against a baseline that includes the other.
