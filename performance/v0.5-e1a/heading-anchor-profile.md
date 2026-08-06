# v0.5 E1a — heading anchor cost

Date: 2026-08-06. Generated data only; no private corpus, no note content, no
local paths. Same 100,000-note tier as E1/E2, re-run after schema v15 and the
heading-anchor check.

## What adding heading anchors cost

| Metric | E2 (schema v14) | E1a (schema v15) |
|---|---:|---:|
| full workspace lint | 3.20 s | 3.05 s |
| anchor resolution p95 | 0.276 ms | 0.404 ms |
| note save p50 (six-block note) | 4.17 ms | 4.16 ms |
| database | 114 MB | 114 MB |
| peak RSS | 87 MB | 89 MB |

Nothing measurable. The slug is one nullable column on a table that already
existed, `NULL` for every non-heading block, so the database does not grow.
Saving a note derives the slug from text the parser has already split, so the
save path is unchanged.

Two numbers deserve honesty rather than a headline:

- **Anchor resolution moved from 0.276 ms to 0.404 ms p95.** The resolution
  query gained a third `OR` branch and a partial index to consider. At this
  magnitude the difference is close to run-to-run noise on this machine, and it
  is measured on a lookup that is already sub-millisecond and scoped to one
  note, so it is reported rather than optimized.
- **Lint came out 5% faster**, which it should not have — the heading check adds
  work. It is noise in the opposite direction, and it is left as measured rather
  than re-run until it tells a tidier story.

## Why the heading check is nearly free

Heading anchors cannot be verified in SQL: matching one means slugifying the
written anchor with the same Unicode rules the parser uses, which lives in Go.
The check therefore rides the single link scan E2 introduced, and looks up each
target note's heading slugs through a bounded cache (512 documents, cleared
rather than grown). A note linked from a hundred places costs one query, and the
pass still holds nothing proportional to the library.

Committed JSON: `blocks-headings-100k.json`.
