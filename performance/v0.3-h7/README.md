# H7 large-library reference profiles

These JSON files are generated evidence for the v0.3 H7 keyset-pagination and
performance baseline. Reproduce a profile from the repository root with:

```bash
bash scripts/run_large_library_profile.sh 10000 performance/v0.3-h7/profile-10000.json
bash scripts/run_large_library_profile.sh 100000 performance/v0.3-h7/profile-100000.json
bash scripts/run_large_library_profile.sh 500000 performance/v0.3-h7/profile-500000.json
```

The driver converts relative output paths to repository-root paths. Each run
creates a temporary SQLite database and synthetic notes, revisions, FTS rows,
links, tags, notebooks, logical resources, and deduplicated blobs. It records
the environment, SQLite query plans, database size, peak process RSS, seed and
cursor-stream time, and latency distributions for first/next/deep pages and
representative filters.

The ordinary first, next, and 90%-deep local page p95 values must remain below
the recorded 100 ms reference target. This is a regression gate for the
recorded environment, not a machine-independent service-level promise.
