# v0.5 E2 — workspace lint scale profile

Date: 2026-08-05. Generated data only; no private corpus, no note content, no
local paths. Produced by `scripts/run_large_library_profile.sh`, which now runs
a full lint at each tier and records per-check timings.

## Results

| Tier | Notes | Links | Full lint | Peak RSS | Ordinary-page gate |
|---|---:|---:|---:|---:|---|
| 10k | 10,000 | 20,000 | 0.34 s | 26 MB | met |
| 100k | 100,000 | 200,000 | 3.20 s | 87 MB | met |
| 500k | 500,000 | 1,000,000 | 18.9 s | 376 MB | met |

Cost is linear in links and memory is flat against the same tier without lint:
every check streams its rows, folding each into the digest before discarding it,
so only the capped examples are ever held.

## The measurement that changed the implementation

The first implementation ran each check as its own SQL statement. It measured
**61.3 s at 500k** — bad enough to be worth explaining rather than documenting.
Per-check timings at 100k said why immediately:

```
broken_document_link        386 ms
broken_resource_link       1951 ms
ambiguous_link             1775 ms
unresolved_block_anchor    1812 ms
missing_title               873 ms
unlocalized_remote_media   1732 ms
missing_alt_text           1829 ms
duplicate_source_id          52 ms   (indexed)
unreferenced_resource         3 ms   (indexed)
projection_backlog            0 ms   (indexed)
```

Six checks, six separate scans of the same `document_links` table — about 10 of
the 12 seconds. The indexed checks were already free. Nothing was
algorithmically wrong; the pass simply read the largest table six times.

The six link checks now share **one ordered scan** that classifies each row into
whichever checks it violates, preserving each check's original semantics
including rows that violate more than one:

| Tier | Six scans | One scan | Speedup |
|---|---:|---:|---:|
| 100k | 10.9 s | 3.2 s | 3.4× |
| 500k | 61.3 s | 18.9 s | 3.2× |

Every lint test passed unchanged across the rewrite, which is the useful signal:
the behaviour is identical and only the number of scans changed.

Per-check timings stayed in the report afterwards. They are what made this
visible in minutes instead of guesswork, and "which check is expensive" is
something an operator wants to know on their own library.

## What remains linear

18.9 s at half a million notes and a million links is a maintenance command's
cost, not an interactive one, and the report says so: `notriosctl lint` is
something to run deliberately, not on every save. The remaining time is the
single scan plus the temp B-tree that orders it — the ordering is what makes the
digest and the examples deterministic, which is worth paying for. If a future
library makes that unacceptable, the next step is a covering index on
`(source_document_id, source_line, source_column)`, not another restructure.

Committed JSON: `lint-100k.json`, `lint-500k.json`, and
`lint-10k-before-single-scan.json` from the earlier six-scan implementation for
comparison.
