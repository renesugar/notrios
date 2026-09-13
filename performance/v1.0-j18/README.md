# v1.0 J18 — every document write scans the whole full-text index

```sh
NOTRIOS_J17_LIBRARIES=/small/notes.sqlite:/large/notes.sqlite \
  go test ./internal/store -run TestJ17WriteCostAgainstLibrarySize -count=1 -v
```

## The defect

```sql
CREATE VIRTUAL TABLE documents_fts USING fts5(
    document_id UNINDEXED,
    ...
```

`UNINDEXED` means FTS5 stores the column and builds no index on it. So this,
which runs on every document write:

```sql
DELETE FROM documents_fts WHERE document_id = ?;
```

plans as `SCAN documents_fts VIRTUAL TABLE` — a full scan of every row in the
full-text index to find the one to remove. **A document write is O(library
size).**

## Measured

One `UpdateDocument` against three libraries, two of them built by different
importers from different corpora, so the trend is not a quirk of one file:

| library | size | notes | one write |
|---|---|---|---|
| generated | 1 MB | 60 | **156 ms** |
| Joplin export | 916 MB | 103,349 | **490 ms** |
| Obsidian vault | 7.3 GB | 382,206 | **2,087 ms** |

And the scan is the whole of it. On the 382,206-note library, the
`document_id` lookup alone:

```
SELECT count(*) FROM documents_fts WHERE document_id = '…';
real  0m2.068s
```

against a full write of 2,087 ms. **The scan accounts for essentially 100% of
the cost.**

## The fix, confirmed before proposing it

Deleting by `rowid` instead, on the same library:

```
SELECT count(*) FROM documents_fts WHERE rowid = 1;
real  0m0.019s
```

**2,068 ms → 19 ms, about 109×.** It needs a `document_id → rowid` mapping,
because the rowid is what FTS5 can find without scanning.

**Read the measurement, not the plan.** `EXPLAIN QUERY PLAN` still prints
`SCAN documents_fts VIRTUAL TABLE` for the rowid form — with an `INDEX 0:=`
suffix marking the equality constraint. A reader comparing plans alone would
conclude nothing had improved; only the timing separates them.

## Ten call sites, not one slow function

`DELETE FROM documents_fts WHERE document_id = ?` appears in:

| file | occurrences |
|---|---|
| `internal/store/sqlite.go` | 3 |
| `internal/store/sqlite_notebooks.go` | 3 |
| `internal/store/sqlite_import_batch.go` | 1 |
| `internal/store/sync_retention.go` | 1 |
| `internal/store/sync_ui.go` | 1 |
| `internal/store/sync_revision_apply.go` | 1 |

So the cost lands on updating a note, deleting one, notebook operations,
retention, the sync apply path and the import batch path alike.

## Why this is a product problem and not an import one

`UpdateDocument` is what runs when **a user saves an edited note**. At 382,206
notes that is two seconds, and it grows with the library. J5 already recorded
search at ~9 s on the same library; this is the write side of the same story.

It is *not* what makes an import slow. During a fresh import every document is a
create, so the `DELETE` never executes and no scan is paid — which is why the
Joplin import averaged ~16 ms per note while an edit afterwards costs 2 s. The
importer gap J5 found remains unexplained by this.

## How this was found, and what it cost to find

Two wrong theories first, both about batching, both measured and both rejected:

- **Link rebuild batching**: 1.51×, not the 2.6× it was supposed to explain.
- **Document write batching**: **0.99×** — 400 transactions versus 1 made no
  difference at all.

The second failure is what pointed here. If collapsing 400 commits into one
changes nothing, the cost is not the transaction; it is the work inside it. Both
runs had been reporting ~2 s per document all along, and it was only worth
noticing once batching was ruled out twice.

The lesson recorded in `performance/v1.0-j17` applies again: the first theory's
arithmetic was derived by dividing an observed gap by an observed call
difference, which can only agree with itself. Every number here comes from an
experiment that could have disagreed.
