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
