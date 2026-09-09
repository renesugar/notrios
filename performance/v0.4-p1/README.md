# v0.4 P1 selection/privacy planner profile

This directory contains aggregate-only evidence for the read-only shared
selection/privacy planner on the generated 100,000-note H7 corpus.

```bash
bash scripts/run_large_library_profile.sh 100000 /tmp/notrios-v0.4-p1-100k.json
```

The full-archive plan asserted all 100,000 selected document identities, 1,000
reachable resources, and 200,000 internal links. It retained only ten detail
examples while the counts and 64-character manifest SHA-256 covered the full
selection. Planning completed in 16.239 seconds. Peak process RSS was 136 MiB
for the entire combined dataset seed, search, cursor, and planner process—not a
planner-only allocation claim.

Planner SQL selects document identity/revision state only. Resources, links,
provenance, and source-bundle metadata are processed in 400-ID batches; note
bodies and resource/source-bundle bytes are never materialized in the Go plan.
The established ordinary-page latency gate also remained satisfied. Existing
nonselective search metrics are recorded for continuity but are not P1 planner
latency targets.
