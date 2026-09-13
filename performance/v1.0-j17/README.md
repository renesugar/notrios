# v1.0 J17-A — measuring before fixing, and killing the first theory

```sh
NOTRIOS_J17_LIBRARY=/path/to/notes.sqlite \
  go test ./internal/store -run TestJ17LinkRebuildTransactionCost -count=1 -v
```

## The theory J5 published, and what happened to it

J5 found the Obsidian importer 2.6× slower than the Joplin importer on the same
382,206 notes, and found a call-site difference that looked like the reason: the
Obsidian importer rebuilds links once per document (382,206 transactions) where
the Joplin importer batches them (3,823). Both call the same inner function, so
only the transaction boundary differs, and J5's record reasoned that ~378,000
extra commits across a 10,068-second gap implied ~27 ms per commit.

**Measured on 2,000 documents in a copy of J5's own library:**

```
per-document: 19.095s total, 9.547ms per document (2000 transactions)
batched:      12.639s total, 6.320ms per document (4 transactions)
ratio: 1.51x
```

Collapsing 500 commits into one buys **34%**. A commit is worth ~3.2 ms of the
9.5 ms, not 27 ms. Most of the cost is the rebuild itself, and even batched,
382,206 documents is ~40 minutes of work neither importer avoids.

**So the theory is wrong, and it was wrong in a specific way worth naming:** the
27 ms figure was obtained by dividing the observed gap by the observed call
difference, which assumes the conclusion. It is arithmetic that can only agree
with itself. The experiment that could disagree took 138 seconds.

This is why J17-A exists as a slice before J17-B. A 4.5-hour re-import and a
code change would have "confirmed" an improvement of 1.51× on one phase and
been written up as the fix for a 2.6× gap.

## Where the gap more plausibly lives — structural, not yet measured

The link rebuild is one per-document call among several. Comparing what each
importer asks the store to do:

| | Obsidian | Joplin |
|---|---|---|
| document write | `CreateDocument` / `UpdateDocument`, plus `SetDocumentSource` and `MoveDocumentToNotebook` — **per document** | `ApplyImportDocumentBatch` — **per batch** |
| link rebuild | `RebuildDocumentLinks` — per document | `RebuildImportDocumentLinksBatch` — per batch |

`CreateDocument` and `UpdateDocument` each open their own `BEGIN IMMEDIATE`, so
a note costs at least two explicit transactions on the Obsidian path and a
fraction of one on the Joplin path.

**That was a structural difference, and it was measured too. It is also wrong.**

```
per-document: 13m14.552s total, 1.986379s each (400 documents, 400 transactions)
batched:      13m18.590s total, 1.996476s each (400 documents,   1 transaction)
ratio: 0.99x
```

400 transactions against 1 makes **no difference at all** — the batched path was
marginally slower, which at this margin is noise. So transaction count is not
what makes the Obsidian importer slow, and two independent measurements now say
so: 34% on the link rebuild, 0% on the document write.

## What the second failure revealed

Both runs had been reporting the same thing without being asked: **a single
document write against this 382,206-note library costs about two seconds**,
whichever path it takes. That is only worth noticing once batching is ruled out,
because while a batching theory is alive the per-unit number looks like
something batching will fix.

It is not. `documents_fts` declares `document_id UNINDEXED`, so
`DELETE FROM documents_fts WHERE document_id = ?` scans the entire full-text
index on every write — 2,068 ms of the 2,087 ms. That is **J18**, it is a
product problem rather than an importer one, and it is the largest thing either
of these measurements found.

## Two more hypotheses, two more rejections

**Directory concentration.** The vault holds 118,866 of its 382,206 files in one
directory; the Joplin export is flat, so nothing it does can depend on directory
size. Twenty thousand notes drawn entirely from that directory against twenty
thousand spread across 110 others:

| subset | directories | ms/note |
|---|---|---|
| deep | 4 | 39.3 |
| wide | 110 | 36.8 |

6.7% apart. Not the cause.

**Superlinear growth — which I claimed and then had to withdraw.** Two points
suggested the per-note cost was rising with corpus size: 28.5 ms at 10,000 notes
against 42.5 ms at 382,206. Four points show a curve that saturates:

| notes | ms/note |
|---|---|
| 10,000 | 28.5 |
| 20,000 | ~38 |
| 40,000 | 39.5 |
| 382,206 | 42.5 |

From 40,000 to 382,206 — 9.5× the notes — the per-note cost rises 7.6%. That is
a warm-up that flattens, not growth. **The claim was made on two points and a
slope, and it was wrong**; it is recorded here rather than deleted because the
reasoning that produced it is the same reasoning that produced the first three
theories.

## Where the gap stands, precisely

**Established:** the Obsidian importer costs ~40 ms per note and the Joplin
importer ~16 ms, both stable across corpus size. The difference is ~24 ms of
per-note work, and it is a **constant factor**, not a scaling defect.

**Rejected, each by measurement rather than by argument:**

| hypothesis | measured | verdict |
|---|---|---|
| link-rebuild batching | 1.51× | too small |
| document-write batching | 0.99× | nothing |
| transaction count in general | — | falsified by the two above |
| directory concentration | 1.07× | nothing |
| the J18 full-text scan | not executed on create | not this |

**Unresolved:** what the ~24 ms is.

## Why the next step is a profiler and not a sixth hypothesis

Five call-site guesses have now been tested and rejected. Each was plausible
from reading the code, each cost a measurement, and the method has an obvious
flaw: it can only find causes I happen to think of, and it has been wrong every
time. "Where does the per-note time go" is the question a CPU profile answers
directly, and it does not require guessing first.

That is J17-A's remaining work, and the item says so rather than proposing
another call site.

## Three harness bugs on the way to the number

All mine, all recorded because each cost a run and none was a product defect.

- **`Search` clamps a page to `max_limit`** — 100 by default. Asking for 2,000
  in one call returns 100 silently, and the test failed with "only 100
  documents available". The clamp is correct; the test had assumed otherwise and
  now pages with the cursor.
- **`RebuildImportDocumentLinksBatch` requires a checkpoint**, with a source
  system, key, inventory fingerprint, phase and status. That is not overhead to
  work around: the checkpoint is what lets the Joplin import resume, and it
  means the batch path wins while carrying weight the per-document path does
  not.
- The first attempt asked for more documents than a page returns, which is the
  first bug wearing different clothes.
