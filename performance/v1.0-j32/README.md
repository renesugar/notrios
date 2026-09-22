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

## Found along the way

- **J36, Obsidian partial-path links.** A link such as `[[topic-00001/index]]`
  is reported as ambiguous, and left unresolved, although exactly one file ends
  with that path. The cause is that `resolveNoteID` and `resolveAssetID` have
  no suffix step. Under the J8 rule this is a plan item of its own, not a J32
  change. The `collision` corpus exercises it, so whichever of J32-F and J36
  lands second is measured against a baseline that includes the other.
