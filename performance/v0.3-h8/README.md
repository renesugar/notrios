# H8 Joplin RAW importer reference profiles

These JSON files are generated evidence for the v0.3 H8 bounded,
checkpointed Joplin RAW importer. Reproduce them from the repository root:

```bash
bash scripts/run_joplin_import_profile.sh 100 performance/v0.3-h8/profile-100.json
bash scripts/run_joplin_import_profile.sh 10000 performance/v0.3-h8/profile-10000.json
bash scripts/run_joplin_import_profile.sh 100000 performance/v0.3-h8/profile-100000.json
```

Each tier generates source item files for nested folders, real tags and
note-tag joins, resources, unknown properties, and the selected note count. It
always runs the production dry-run planner. The 100 and 10k tiers then:

1. runs the production dry-run planner;
2. starts the real bounded import;
3. injects an interruption at a durable note-batch boundary;
4. resumes from the persisted checkpoint;
5. assert the dry-run create totals match the completed real import.

The 100k tier is deliberately a complete inventory/dry-run batching profile,
not a 100k canonical-write benchmark. The current Store API preserves
per-document canonical transactions, so a full 100k write is hardware- and
dataset-dependent and is not a routine regression gate. The 10k tier is the
large interrupted/resumed real-import gate; focused tests cover idempotent
reruns and final-checkpoint crash recovery.

The report records the environment, batch size/count, durations (Go
`time.Duration` nanoseconds), imported totals, resume state, and Go runtime
memory reservation. It is reproducible evidence for the recorded environment,
not a machine-independent service-level objective. Private Joplin exports are
never used or committed.
