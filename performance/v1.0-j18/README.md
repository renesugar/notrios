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

---

# The fix, measured

Same test, same three libraries, after the change:

| library | notes | before | after | |
|---|---|---|---|---|
| generated | 60 | 156 ms | **7 ms** | 22× |
| Joplin export | 103,349 | 490 ms | **26 ms** | 19× |
| Obsidian vault | 382,206 | **2,087 ms** | **17 ms** | **123×** |

**A note save on a 382,206-note library went from 2.1 seconds to 17
milliseconds**, and the cost stopped depending on the library: 17 ms at 382,206
against 26 ms at 103,349 is noise, not a trend. That was the goal — a write
costing what the document costs rather than what the library costs.

## What the migration costs, once

| library | migration |
|---|---|
| generated | 0.4 s |
| Joplin export | 30 s |
| Obsidian vault | **3 m 49 s** |

Just under four minutes on the largest library, once, when it is first opened
after upgrading. Against two seconds on every note save, forever.

## How it is built

`documents_fts_rowid(document_id TEXT PRIMARY KEY, fts_rowid INTEGER)`,
populated by migration 0028 in a single scan of the index — the same scan a
single write used to do. Three helpers in `internal/store/fts_rowid.go` keep the
pairing in one place: write the index and record the rowid, delete the row and
forget the rowid. All ten call sites go through them or through the rowid form
directly, and the whole-index rebuilds in `sqlite_restore.go` rebuild the
mapping the way the migration does.

## Correctness, which is the boundary that mattered

Speed was never the risk. A wrong mapping deletes *somebody else's* row, and the
symptom is not a crash — it is a note that has quietly stopped being findable,
or one still found after deletion. Neither appears in a timing.

`TestJ18SearchIsUnchangedByTheRowidMapping` and
`TestJ18MappingTracksTheIndexExactly` assert what a reader would notice, and
both were confirmed by breaking the mapping:

- **recording a wrong rowid** → *"after update, kestrel matches [the wrong
  note]"*, and *"1 mapping rows point at nothing"*.
- **deleting nothing from the index** → *"kestrel matches two notes, want
  only one"*, and *"7 rows in the index and 4 in the mapping"*.

A third probe — swapping `INSERT OR REPLACE` for `INSERT OR IGNORE` — did *not*
fail, and that is recorded because it was a bad probe rather than a gap: the
replace path deletes the mapping row first, so both forms behave identically.

## What the schema bump reached, and why each was widened rather than moved

The schema went to v28, and four pinned facts had to change. None was suppressed:

- **`syncstate.MaxCompatibleSchema` 27 → 28.** Peers compare schema versions.
  `documents_fts_rowid` is a private index carrying nothing a peer sends or
  receives and nothing a user wrote, rebuildable from `documents_fts` alone, so
  a v27 and a v28 peer exchange identical bytes. The range **widens**; it does
  not move.
- **`contracts/archive-v2/contract.json` `current_schema_version` 27 → 28**, and
  the physical-refusal golden with it. That is a published contract, so the
  change is deliberate and visible.
- **G19's portability bound.** It pinned `current_schema_version` to 27, which
  asserted the store would never migrate again — never the intent of a
  *portability* bound. The format, version and **minimum** schema stay pinned;
  the writer's schema is now required not to go backwards.
- **G20's canonical-schema check.** It read today's source for exactly 27, which
  was right only while the schema never moved. It now requires the schema this
  0.7.0 record describes to still be reachable, and a migration to exist for
  every step up to wherever the tree is.

Two store tests also moved honestly rather than being relaxed: one asserting the
schema is 27 was renamed, since its point is that v25's security surface
survives *later* migrations; and the "database from the future" fixture has to
stay one ahead of the build.

**I8's freeze caught the archive-v2 change** and refused until it was
re-recorded, which is the gate working on the first thing to touch that surface
since it was frozen.
