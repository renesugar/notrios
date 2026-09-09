# H9 Obsidian importer reference profiles

These generated JSON files exercise the bounded, checkpointed v0.3 H9
Obsidian importer. Reproduce them from the repository root:

```bash
bash scripts/run_obsidian_import_profile.sh 100 performance/v0.3-h9/profile-100.json
bash scripts/run_obsidian_import_profile.sh 10000 performance/v0.3-h9/profile-10000.json
bash scripts/run_obsidian_import_profile.sh 100000 performance/v0.3-h9/profile-100000.json
bash scripts/run_obsidian_import_profile.sh 500000 performance/v0.3-h9/profile-500000.json
```

Every tier generates nested vault folders, aliases, relative note links,
heading and block references, embeds, unknown frontmatter, and local assets.
The production dry-run planner runs with exact source-bundle accounting. The
100 and 10k tiers then interrupt a real import after a durable note batch,
resume it, and assert that dry-run create totals match the completed import.

The 100k and 500k tiers are complete inventory/dry-run batching profiles, not
canonical-write benchmarks. Per-document canonical transactions make those
writes dependent on hardware and dataset shape; the 10k tier is the large
interrupted/resumed write gate. Reports record durations, batching, resume
state, graph rewrites, source-bundle counts, and Go runtime memory reservation.
They are reproducible evidence for the recorded environment, not universal
service-level objectives. No private vault is used or committed.
