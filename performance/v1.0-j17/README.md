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

**That is a structural difference, not a measurement, and this record does not
convert it into one.** The last theory also looked obvious from the call sites.
What would settle it is a comparison of `ApplyImportDocumentBatch` against the
per-document sequence over the same documents, the way the link rebuild was
compared — which is the next thing J17-A does.

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
