# v0.5 E1 — block scale profile

Date: 2026-08-05. Generated data only; no private corpus, no note content, no
local paths. Produced by `scripts/run_large_library_profile.sh` at the
10,000 / 100,000 / 500,000-note tiers, which now records block metrics.

## What is measured, and what is not

The generated seeder writes documents, revisions, links, and tags with raw SQL
for speed, so it produces **no blocks** — blocks are derived by the save path.
The profile therefore measures two different things, and they are labelled
differently on purpose:

- **the real path over a bounded sample**: 200 genuine `CreateDocument` calls
  with a six-block body, on top of the seeded library. This is what a save
  actually costs now that blocks are rebuilt inside the same transaction.
- **index behaviour at real row counts**: block rows for every seeded note
  (six per note) inserted with raw SQL, then anchor and listing lookups
  re-measured. Those rows are shaped like the parser's output but are not
  produced by it; they exist only to answer whether the index still holds at
  a library's worth of rows.

Nothing here claims the parser was run at 500,000 notes.

## Results

| Tier | Notes | Block rows | DB before / after blocks | Save p50 | Anchor p95 | Anchor p95 at scale | List p95 at scale |
|---|---:|---:|---|---:|---:|---:|---:|
| 10k | 10,000 | 61,200 | 12 MB → 25 MB | 3.064 ms | 0.352 ms | 0.423 ms | 0.200 ms |
| 100k | 100,000 | 601,200 | 114 MB → 245 MB | 5.466 ms | 0.340 ms | 0.544 ms | 0.245 ms |
| 500k | 500,000 | 3,001,200 | 570 MB → 1,229 MB | 5.028 ms | 0.271 ms | 0.307 ms | 0.203 ms |

Query plan for the listing path:

```text
SEARCH document_blocks USING INDEX document_blocks_document_idx (document_id=?)
```

The ordinary-page p95 gate (100 ms) still passes at every tier with blocks
present, and the existing All-notes/search metrics are unchanged within noise.

## What the numbers say

**Anchor resolution does not care how large the library is.** It stays between
0.2 ms and 0.55 ms from 61,000 to **3,001,200** block rows, because every lookup
is scoped to one document and served by `(document_id, ordinal)` or the partial
marker index. That was the risk the plan named — block rows outnumber notes by
design — and the index answers it: at 3 million rows the anchor lookup is
0.307 ms p95, indistinguishable from the same lookup at 61,000.

**Blocks roughly double the database.** 12 MB → 25 MB at 10k, 114 MB → 245 MB at
100k, 570 MB → 1,229 MB at 500k, at six blocks per note. That is the real cost of this feature and it is
worth stating plainly: a note's blocks are a second index over its text, and
they are sized like one. Row width was kept to what an anchor needs — no block
text is stored, only a hash and a byte range — which is why the growth tracks
row count rather than note length.

**Save cost does not track library size.** 3.1 ms at 10k, 5.5 ms at 100k, 5.0 ms
at 500k for a six-block note — the 100k and 500k numbers are within run-to-run
noise of each other. The block rebuild itself measures 0.8–1.5 ms in isolation
(`blocks_rebuild_one_document`) at every tier; the rest is the existing FTS,
link, and revision work.

**Peak RSS is unchanged.** 379–381 MB at 500k with and without three million
block rows present: nothing about blocks is held in memory, since every
operation is scoped to one note.

## Reproducing

```bash
bash scripts/run_large_library_profile.sh 10000  /tmp/blocks-10k.json
bash scripts/run_large_library_profile.sh 100000 /tmp/blocks-100k.json
bash scripts/run_large_library_profile.sh 500000 /tmp/blocks-500k.json
```

The 500,000-note tier takes appreciably longer than it did before this slice:
seeding three million synthetic block rows is a 149-second write on its own, and
the whole tier now runs about 14 minutes on the reference machine.

Committed JSON: `blocks-10k.json`, `blocks-100k.json`, `blocks-500k.json`.
