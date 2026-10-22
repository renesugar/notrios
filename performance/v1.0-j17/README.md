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

## The profile

```sh
NOTRIOS_J17_VAULT=/path/to/vault NOTRIOS_J17_DB=/path/on/the/disk/under/test.sqlite \
  go test ./internal/importers/obsidian -run TestJ17ImportRealVaultOnDisk \
  -count=1 -v -cpuprofile cpu.out -timeout 60m
```

A 10,000-note subset of the recipe vault, imported into a library on the same
HDD J5 used: **28.7 ms per note**, 298.9 s wall against 161.8 s user and 43.0 s
system. So about 69% of the wall clock is CPU and about 31% is waiting. 187.8 s of
samples:

| where | cumulative | what it is |
|---|---|---|
| `runtime.cgocall` (flat) | 70.4% | time inside SQLite |
| `CreateDocument` | 48.0% | the document write, its own transaction |
| `rebuildDocumentLinksLocked` | 38.8% | links **and** blocks |
| `prepareLocked` → `sqlite3_prepare_v2` | 30.7% | compiling SQL text |
| `rebuildDocumentBlocksLocked` | 29.4% | one `DELETE` plus one `INSERT` per block |
| `processLinkRebuild` → `RebuildDocumentLinks` | 27.5% | the second link phase |
| `sqlite3_exec` via `execLocked` | 22.3% | `BEGIN`/`COMMIT`, 74% of it from `CreateDocument` |
| regexp backtracking | 17.7% | 13.7% from `markdownlinks.Extract` |
| `markdownblocks.Extract` | 7.5% | block parsing |

Three facts come out of it, each read from the call graph rather than guessed.

1. **Every statement is compiled on every call.** `execPreparedLocked` calls
   `sqlite3_prepare_v2`, steps once, and finalizes. Nothing is cached. Compiling
   SQL takes 30.7% of the importer's CPU, and 91% of that comes from
   `execPreparedLocked`. The block rebuild issues one `INSERT` per block and
   compiles it again each time.
2. **Links and blocks are rebuilt twice per note.** The first rebuild runs inside
   `CreateDocument` (33.95 s). The second runs in the importer's link phase
   (38.93 s). The first cannot resolve links to notes that have not been imported
   yet, which is why the second exists, so the first one's output is discarded.
   **This is not the asymmetry.** `ApplyImportDocumentBatch` also rebuilds links
   for changed bodies, and the Joplin importer's
   `RebuildImportDocumentLinksBatch` rebuilds them again. Both importers pay for
   it twice.
3. **Commits happen per note, not per batch.** For each created note, the
   Obsidian path runs:
   - `CreateDocument`: one `BEGIN IMMEDIATE`…`COMMIT`
   - `SetDocumentSource`: no explicit transaction, so it autocommits
   - `AttachDocumentResource`: autocommits, once per attachment
   - `RebuildDocumentLinks`: one `BEGIN IMMEDIATE`…`COMMIT`

   That is at least three durable commits per note. The Joplin path makes two per
   100 notes, one for the document batch and one for the link batch.

### A rejection this reopens

The table above lists "document-write batching, 0.99×" as a rejection. That
measurement ran against the 382,206-note library, where J18's full-text scan
made **every** write cost about 2 s. A commit worth a few milliseconds cannot
show up beside a 2,000 ms scan. The experiment could not have found commit cost,
so it could not rule commit cost out, and "transaction count is falsified" went
further than the evidence. J18 has since removed the scan, so the question can
be asked again on a library where it is answerable.

### Asked again: the same import with the library on tmpfs

The same binary, vault and 10,000 notes, with only the library moved from the
HDD to tmpfs, where a commit's sync to disk costs nothing:

| library on | wall | user | sys | ms/note |
|---|---|---|---|---|
| HDD | 298.9 s | 161.8 s | 43.0 s | 28.7 |
| tmpfs | 167.0 s | 158.9 s | 14.0 s | 16.7 |

User CPU is unchanged (−1.8%), so the computation does not depend on where the
library lives. **Everything that moved is waiting on the disk: 132 s, or 12 ms
per note, 42% of the HDD run.** System time falls by 29 s because it includes
the sync calls. At least three commits per note on a disk that syncs each one
fits that cost, which makes commit count a strong lead again. It is not yet
proven to be the cause: this run moved the whole library off the disk, not just
the commits. Proving it takes the same import on the HDD with the commits
batched.

**Not claimed:** that this closes the gap with the Joplin importer. The
~16 ms/note Joplin figure comes from all 382,206 notes on the HDD. This is a
10,000-note subset on tmpfs, and the Obsidian importer's per-note cost rises
between 10,000 and 40,000 notes (table above). Only a comparison on the same
disk and the same corpus size can say how much of the gap this is.

## J17-B — the commits batched, measured on the disk that was slow

The note phase now builds one `ImportDocumentMutation` per note and applies the
batch with `ApplyImportDocumentBatch`. The document, its source, its attachment
references, its item state and the checkpoint all land in one transaction. The
link phase calls `RebuildImportDocumentLinksBatch`. These are the store methods
the Joplin importer already uses. `eachBatch` still saves its checkpoint after
each batch, one commit per 100 notes, so the report counters stay exact.

The same binary harness, vault and 10,000 notes, on J5's HDD:

| importer | wall | user | sys | ms/note |
|---|---|---|---|---|
| per-note commits (before) | 298.9 s | 161.8 s | 43.0 s | 28.7 |
| batched commits (after) | 198.1 s | 174.8 s | 17.3 s | 19.7 |
| per-note commits, library on tmpfs | 167.0 s | 158.9 s | 14.0 s | 16.7 |

**1.46× on 10,000 notes on the HDD.** The lead now has a direct measurement
behind it: moving only the commits recovers 101 s of the 132 s that moving the
whole library recovered. System time falls to within 3 s of the tmpfs run.
User CPU rose by 13 s (8%). That rise has not been explained and is not claimed
to be noise. It is small against the 101 s gained, and the profile attributes
none of it yet.

**Claimed at the size measured, and no larger.** This is 10,000 notes. The
382,206-note import, and the gap against the Joplin importer on that corpus,
have not been re-run. The per-note cost of the old importer rose between 10,000
and 40,000 notes, so the 10k ratio is not extrapolated to J5's 4.52 hours.

### Re-measured clean: the 1.46× was wrong, and so was the CPU rise

J19-A found that two timings of the same importer disagreed: 19.7 ms/note here
and 13.8 there. **Both had shared the machine with other work.** The 19.7 ran
alongside the 300-note equivalence imports, `go vet` and the package tests. The
old importer's 28.7 ran alone. So the 1.46× compared a contended run against a
clean one, and the "user CPU rose 13 s" was that contention, not the change.

Re-run in J19-A: one process at a time, same binaries, vault and HDD, with
nothing else running:

| importer | runs | ms/note | user | sys |
|---|---|---|---|---|
| per-note commits (before J17-B) | 2 | 27.1, 31.1 | 147.4 s, 152.5 s | 41.5 s, 42.9 s |
| batched commits (J17-B) | 3 | 13.2, 13.1, 13.3 | 124.3 s, 123.3 s, 125.8 s | 12.4 s, 12.1 s, 12.6 s |

**J17-B is about 2.2× on 10,000 notes (29.1 against 13.2 ms/note means), not
1.46×. User CPU falls by about 25 s, not rises.** The batched importer repeats
within 1.5%. The per-note importer varies by 15% between its two runs, which
fits a cost that is mostly waiting on the disk.

Warm page cache throughout: no passwordless `sudo`, so no cache drop between
runs. The table above is left as it was measured, with this correction beside
it. The 382,206-note import still has not been re-run.

### Same library, proven by comparison rather than by the import finishing

`j17_compare.py` compares two libraries table by table, ignoring only timestamp
columns. When a table differs, it re-compares column by column and names the
columns that differ.

```sh
python3 performance/v1.0-j17/j17_make_vault.py /tmp/vault17 300
# import /tmp/vault17 once with the importer before the change and once after,
# each into a new library via TestJ17ImportRealVaultOnDisk, then:
python3 performance/v1.0-j17/j17_compare.py /tmp/before.sqlite /tmp/after.sqlite
```

**The 10,000-note subset**, old importer against new:

```
same  document_blocks (249455 rows)      same  documents_fts (10000 rows)
same  document_links (2 rows)            same  documents_fts_rowid (10000 rows)
same  document_sources (10000 rows)      same  import_checkpoints (1 rows)
same  import_item_states (10113 rows)    same  index_outbox (10000 rows)
same  notebooks (117 rows)               ...
DIFF  database_identity   columns: ['database_id', 'replica_id']
DIFF  document_revisions  columns: ['id']
DIFF  documents           columns: ['current_revision_id']
```

The three differences are the random identifiers each new library and each
revision is given. They could not match between two separate imports.

That corpus has 2 links and no attachments, so on its own it proves little
about the paths that changed most. `j17_make_vault.py` generates a
deterministic 300-note vault with these link forms:
- wikilinks, both forward and backward
- display text and heading anchors
- relative markdown links
- alias targets
- embedded PNG attachments across nested folders
- a missing target

Old importer against new: **1,055 links (865 resolved), 170 attachment
references, 3,129 blocks, 329 item states. Every table is the same except the
same three random-identifier columns.** Resolving forward references depends on
the separate link phase, and it produced the same 865.

The update path had no test that changed a note's body and re-imported it.
`TestImportObsidianVaultFixture` now does: exactly one update, a new revision,
the new body, and a link index rebuilt from the new body.

## J17-C — every other per-item store call in import and export

The survey counts each store call in `internal/importers/*` and
`internal/archivev2`, then checks each call inside a per-item loop against the
batch methods the store interface actually offers. For writes, those are
`PutImportItemStates`, `ApplyImportDocumentBatch`,
`RebuildImportDocumentLinksBatch` and `ApplyRestoreRecords`. For reads, they are
`GetDocuments`, `GetResources`, `GetDocumentTags`, `GetDocumentSources`,
`FindDocumentsBySourceIDs`, `GetImportItemStates` and the `Export*` family.

**Batched, nothing to do:**

| where | calls |
|---|---|
| `internal/archivev2` export | every read is an `Export*` call over a batch of IDs |
| `internal/archivev2` restore | writes go through `ApplyRestoreRecords` per batch |
| Obsidian and Joplin notes and links | `ApplyImportDocumentBatch` and `RebuildImportDocumentLinksBatch` (Obsidian since J17-B) |
| Obsidian and Joplin item states | `PutImportItemStates` per batch |

**Per item, with no batch method to use. Recorded, not invented:**

| importer | per-item call | once per |
|---|---|---|
| Obsidian, Joplin | `PutSourceBundleItem` | preserved source file, only with source preservation on |
| Obsidian, Joplin | `CreateNotebook` / `UpdateNotebook` | folder or notebook |
| Obsidian, Joplin | `CreateResource` / `UpdateResource` | attachment |
| Joplin | `UpsertTag` | tag |

None of these has a batch equivalent in the store, and adding one is a store
change. The plan keeps that for whatever item needs it. On J5's corpora these
are the small counts: 117 notebooks in the 10k subset and 781 folders in the
whole vault, against 382,206 notes. Attachments are the one count that can grow
like notes, and nothing here measured their cost.

**Per item, where a batch method exists but does not fit:**

| importer | per-item calls |
|---|---|
| ChatGPT | `FindDocumentBySource`, `GetDocument`, `CreateDocument` / `UpdateDocument`, `SetDocumentSource` per conversation |
| Claude | the same, per conversation |
| Twitter | the same, plus `CreateResource`, `AttachDocumentResource` and `AddDocumentTag`, per tweet |

`ApplyImportDocumentBatch` requires an import checkpoint and an item state for
each document. These three importers have neither (no `Checkpoint` or
`ImportItemState` anywhere in them). So moving them onto it means giving them
resumable, fingerprinted imports: a design change to each importer, not a change
of call site. They are recorded here. No cost is claimed for them, because none
has been measured on a corpus large enough to matter.

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
