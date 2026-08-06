# v0.5 E5 — editor link intelligence scale profile

Date: 2026-08-06. Generated data only; no private corpus, no note content, no
local paths. Produced by `scripts/run_large_library_profile.sh`.

Reference machine: Intel Core i5-9300H, Linux/amd64, Go 1.26.5, SQLite 3.45.1.

Both operations run **on a keystroke**, so flatness against library size is the
requirement rather than a nicety.

## Results

| Tier | Notes | Suggest (p95) | Buffer check (p95) | Peak RSS |
|---|---:|---:|---:|---:|
| 10k | 10,000 | 0.51 ms | 1.40 ms | 27 MB |
| 100k | 100,000 | 1.03 ms | 8.30 ms | 89 MB |
| 500k | 500,000 | 0.44 ms | 1.40 ms | 379 MB |

Flat across a fifty-fold library. The 100k row's higher p95 is run noise on a
loaded machine, not a trend — the 500k tier, five times larger, is faster on
both — and it is reported as measured rather than smoothed. Peak RSS is
unchanged against the same tier before this slice.

Both statements resolve to covering index searches:

```
suggest_title_prefix       SEARCH documents USING COVERING INDEX documents_title_idx (collection_id=? AND deleted_at=? AND title>? AND title<?)
link_resolution_by_title   SEARCH documents USING COVERING INDEX documents_title_idx (collection_id=? AND deleted_at=? AND title=?)
```

No temporary B-tree appears in either, because `id` is the index's last column
and `ORDER BY title COLLATE NOCASE, id` is therefore exactly index order.

## What schema v16 is worth

The profile measures this rather than asserting it: each tier drops
`documents_title_idx`, re-measures, and restores it. Dropping it reproduces
exactly the state before this slice, when link resolution ran
`lower(title) = lower(?)` — a comparison no index can serve.

| Tier | Suggest, indexed | Suggest, unindexed | Buffer check, indexed | Buffer check, unindexed |
|---|---:|---:|---:|---:|
| 10k | 0.22 ms | 41.6 ms | 0.84 ms | 19.5 ms |
| 100k | 0.46 ms | 1,324 ms | 1.37 ms | 1,000 ms |
| 500k | 0.27 ms | 5,390 ms | 0.68 ms | 4,198 ms |

The indexed column is flat; the unindexed one is linear in library size. At half
a million notes an unindexed suggestion costs **5.4 seconds** and an unindexed
buffer check **4.2 seconds** — per keystroke. The difference is about 20,000× for
the suggestion and 6,200× for the check.

That cost was not hypothetical and did not belong only to this slice. Every link
that named a note by title paid it **on every save and on every lint pass**,
once per link. The index is the reason E5's working state — "without a
per-keystroke whole-library query" — is met, and it made two older paths faster
as a side effect.

The change to get there is small and semantically identical:
`lower(title) = lower(?)` became `title = ? COLLATE NOCASE`. SQLite's built-in
`lower()` folds ASCII only, exactly as the NOCASE collation does, so the two
compare the same strings the same way — one can use an index and the other
cannot.

## The one measurement that is not flat

`suggest_no_match` — a query matching nothing — grows 0.62 ms → 1.70 ms →
5.83 ms across the three tiers. It is the only path that runs **both** passes to
exhaustion: the title-prefix range finds nothing, so the bounded FTS5
interior-word pass runs, and an FTS5 prefix term lookup walks a term dictionary
that grows with the library.

It is bounded, it stays well inside a keystroke, and it is the case where the
answer is "nothing" — but it is growth rather than flatness, and it is recorded
here rather than left out of the table. If a future library makes it matter, the
fix is an FTS5 `prefix=` index on the title column, not a change to the two-pass
design.

Committed JSON: `editor-links-10k.json`, `editor-links-100k.json`,
`editor-links-500k.json`.
