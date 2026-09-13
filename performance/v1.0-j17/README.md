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

## What J17 is left holding

The importer gap J5 measured — 4.52 h against 1.72 h on the same notes — is
**still unexplained**. J18 does not explain it: during a fresh import every
document is a create, so the `DELETE` never runs and no scan is paid, which is
why the Joplin import averaged ~16 ms per note. Both of J17's own theories are
dead. What remains is to look outside the store calls entirely — inventory
scanning, resource handling, or something in the Obsidian importer's own work.

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
