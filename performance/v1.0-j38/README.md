# v1.0 J38 — carrying J32's findings into the Joplin importer

J32 measured and changed the Obsidian import path. Most of what it changed lives
in shared code, so a Joplin import already has it. This item tries what does not
apply automatically, under the same rule: a change ships only if a benchmark
written before it shows it beats the baseline's own range on a declared metric,
and nothing else regresses beyond that range.

## J38-A — what a Joplin import costs now, before anything is changed

Profiles taken at `97ac9c5`, the commit that closed J32
(`near-limit-joplin-before.cpu`, `joplin-10k-before.cpu`, and the allocation
profiles beside them).

### What the shared code already delivered

| case | at the J32-A baseline | now | change |
|---|---:|---:|---|
| near-limit-joplin/fresh | 396.6 s | **52.3 s** | **−86.8%** |
| joplin-10k/fresh | 81.3 s | 77.2 s | −5.0% |

The near-limit corpus gained almost everything Obsidian's did, without a line of
Joplin-specific work: J32-B's statement reuse, J32-E's link cursor, J32-I's
re-resolving link pass, J32-N's index-seeking scope, J32-Q, S, T's block work,
J32-W1's and J32-AA's hashing, J32-X's binding, J32-Y's link matchers and
J32-Z's marker scan are all in `internal/store`, `internal/markdownlinks` and
`internal/markdownblocks`. `internal/importers/joplinraw` holds no regexp of its
own, so J32-Y and J32-Z reach it whole.

The ordinary corpus barely moved, and the profile says why.

### Where the time goes now

**joplin-10k, 77.2 s — 85% of it is SQLite:**

| work | share |
|---|---:|
| `ApplyImportDocumentBatch`'s own statements | 25.3 s, 35% |
| `setDocumentSourceLocked` | 14.4 s, 20% |
| `insertRevisionLocked` | 6.6 s, 9% |
| `COMMIT` (263 of them) | 1.9 s, 3% |
| item states, projections, blocks, full text | ~4 s, 6% |
| everything in Go | under 10% |

A Joplin import writes two rows per note — the document and its RAW source
provenance — so `setDocumentSourceLocked` is a fifth of the import on its own,
and the corpus has 20,070 files against Obsidian's 10,020.

**near-limit-joplin, 52.3 s:**

| work | share |
|---|---:|
| SQLite | 45.6% |
| SHA-256 (2.0 passes over the content, against Obsidian's 4.0) | 23.8% |
| `memmove` — copies | 7.8% |
| `rewriteJoplinLinkLine` | 5.9% |
| `splitPhysicalLines` | 4.7% |
| block parsing | 4.9% |

Its allocations are 3.13 GB for 252 MB of notes — about twelve passes, against
Obsidian's seven — spread over `parseItemBytes` (780 MB), `buildDocumentBody`
(480 MB), two file reads (480 MB), string conversions (720 MB), the block line
table (250 MB) and one body read back out of SQLite (240 MB).

### What this says about the slices that follow

No 10× win is available here: nothing in a Joplin import is repeating a whole
parse the way Obsidian's final pass was, and the two corpora are dominated by
SQLite (85% and 45.6%) and hashing — the costs J32 ended at, governed by the
owner's decisions on inline full-text indexing and on never relaxing
`synchronous`.

So the remaining slices are the modest, well-understood ports, and they are
expected to be modest:
- **J38-B**, existence-only reads: one body-sized pass, 240 MB on the near-limit
  corpus. On Obsidian the same change was worth −14.2% of allocated bytes and
  −9.9% of system time.
- **J38-C**, byte-bounded batches: peak RSS is 1.39 GB here, higher than
  Obsidian's was before J32-K took a twelve-large-note corpus from 1.88 GB to
  917 MB. This is the largest expected win of the item.
- **J38-D**, a reused item buffer: 480 MB of the 3.13 GB, two reads per item.
- **J38-E**, what the profile adds to that list: `buildDocumentBody`'s assembly
  (480 MB, the shape J32-U fixed on the other side), `parseItemBytes`' copies
  (780 MB), and `rewriteJoplinLinkLine` with `splitPhysicalLines` (10.6% of the
  near-limit import's CPU, and Joplin's only sizeable Go-side cost).

## J38-C — byte-bounded Joplin batches: **kept, with its cost stated**

Joplin windowed its notes phase by item count alone — a hundred by default, five
hundred at most — while the phase keeps every canonical body of a batch until it
is written. A window now ends at the count or once it holds 64 MiB of items, the
size one RAW item may already reach, so a large item becomes a batch of its own
and nothing legal is refused. The sizes come from the inventory the manifest
already holds, so no file is read to decide a window.

Only the notes phase takes the bound. Two other loops walk the same inventory
with identical-looking code, and both collect ids and carry no bodies. Joplin
already writes its checkpoint inside the batch, so J32-J's duplicate write does
not exist here.

**The corpus had to be built for it,** as on the Obsidian side:
`near-limit-joplin` holds four large items among two hundred small ones, so a
hundred-item batch happened to hold only those four. `many-large-joplin` is
twelve RAW items of 60 MiB and nothing else.

Declared: peak RSS. Baseline `97ac9c5`, candidate `fb10ed1`, five runs each,
alternating. Records in `j38c/`.

| case | peak RSS before | after | baseline range | change | wall s |
|---|---:|---:|---:|---:|---:|
| many-large-joplin/fresh | 3.38 G | **1.15 G** | 123 M | **−66.0%** | unchanged |
| near-limit-joplin/fresh | 1.39 G | 1.20 G | 171 M | −13.8% | unchanged |
| joplin-10k/fresh | 34.0 M | 34.0 M | 0.9 M | within noise | unchanged |

**A larger win than the Obsidian side's 51%, and a larger cost with it.** On
`many-large-joplin`: commits 5 → 10, prepared statements 270 → 335 (+24.1%),
allocations 5,656 → 6,991 (+23.6%), user CPU +0.63% — every one past its range,
and every one the direct consequence of more batches, because bounding a batch
means making more of them. Peak RSS falls by 2.2 GB for that. Wall time is
unchanged on all three corpora, so nothing is being traded for time.

This is the same trade the owner accepted for J32-K, in the same direction, with
a bigger memory gain and a proportionally bigger bookkeeping cost. Joplin's
batches held more per item because the phase keeps the parsed RAW item as well as
the canonical body.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh` passes
with no temp entry left. A test states the window rule — small items all fit, one
item at or past the budget stands alone, the budget closes a window early, a
small item before a large one keeps them together — and that the budget is the
size limit one item may already reach. Both corpora imported by each binary
produce identical `document_blocks`, `document_links`, `document_sources` — the
RAW provenance rows — and `documents`.

Two of the import report's 44 fields change, and both count the thing that
changed: `batches_completed` 4 → 9 and `canonical_document_batches` 1 → 6 on
`many-large-joplin`.

## J38-E — Joplin's own scans, by runs instead of bytes: **kept**

The two costs J38-A named as Joplin's only sizeable Go-side work.
`rewriteJoplinLinkLine` walked each body one byte at a time, writing every
character individually, at 5.9% of a near-limit import; `splitPhysicalLines`
tested every byte for a line ending, at 4.7%. Both now jump to the next byte
that could matter — a backtick inside inline code, a backtick or the colon of a
`:/` link outside it, or the next line ending — and copy the run between in one
write.

| micro-benchmark | before | after |
|---|---:|---:|
| the rewriter, a paragraph with one link | 19.5 µs, 16 allocations | **10.1 µs, 9 allocations** |
| the splitter, a 20,000-line body | 5.33 ms | **4.12 ms** |

Declared: wall time on `near-limit-joplin/fresh`. Baseline `fb10ed1`, candidate
`d1ee668`, five runs each, alternating. Records in `j38e/`.

| case | metric | baseline | candidate | baseline range | change |
|---|---|---:|---:|---:|---:|
| near-limit-joplin/fresh | wall s | 50.38 | **47.94** | 0.60 | **−4.8%** |
| near-limit-joplin/fresh | user s | 35.40 | 32.99 | 0.65 | −6.8% |
| joplin-10k/fresh | wall s | 67.81 | 68.37 | 1.44 | within noise |

J38-A predicted about 5% on the near-limit corpus — roughly half of the 10.6% the
two functions cost — and nothing on `joplin-10k`, which is 85% SQLite. That is
what happened, which is the first time in either item that a prediction made
before the work matched the measurement this closely.

**Correctness.** The full `go test` through `scripts/check_temp_leaks.sh` passes
with no temp entry left. The byte-at-a-time versions are kept in the tests as the
reference and compared over **20,024 lines** — links resolved to a note and to a
resource, unresolved ones, escaped colons, inline code of every backtick run
length, code that never closes, a colon with nothing after it, multi-byte text —
with the **inline-code state carried out of each line** compared as well, since
that state machine is what a run-skipping rewriter could most easily break. Both
corpora imported by each binary produce identical `document_links`,
`document_sources` and bodies.
