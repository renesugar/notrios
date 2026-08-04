# v0.4 P3 native archive-v2 streaming export profile

Aggregate-only evidence for the manifest-last archive-v2 writer. Each tier
seeds a generated library (eight notebooks, one shared resource per 100 notes,
one cross-note link per note, one tag per 25 notes), then measures a full
export, its verification, a resumed export over already-published objects, and
a bounded single-notebook subset export.

```bash
bash scripts/run_archive_export_profile.sh 100  performance/v0.4-p3/export-100.json
bash scripts/run_archive_export_profile.sh 1000 performance/v0.4-p3/export-1000.json
bash scripts/run_archive_export_profile.sh 5000 performance/v0.4-p3/export-5000.json
```

No note titles, bodies, resources, local paths, or databases are recorded.

## Results (Intel i5-9300H, Go 1.26.5, linux/amd64)

| Tier | Objects | Archive bytes | Export | Verify | Resume | Subset | Peak RSS |
|---|---|---|---|---|---|---|---|
| 100 | 102 | 160 KB | 0.083 s | 0.103 s | 0.077 s | 0.015 s | 16 MiB |
| 1,000 | 1,011 | 1.6 MB | 0.643 s | 1.333 s | 0.783 s | 0.130 s | 24 MiB |
| 5,000 | 5,052 | 7.9 MB | 3.210 s | 5.130 s | 2.956 s | 0.461 s | 46 MiB |

Peak RSS is whole-process (seed plus export plus verify plus subset), not an
exporter-only allocation claim. It grows with the retained document-ID and
notebook/tag identity sets, not with note bodies: bodies, resource bytes, and
source-bundle bytes stream through a 64 KiB buffer.

Every tier reported `resume_bytes_rewritten: 0` after its manifest was removed:
a resumed export reuses each already-published content-addressed object and
republishes only the manifest. `commit_stable_across_runs` confirms the same
canonical state produced the same record counts on both runs.

## Object budget

The dominant object cost is one immutable body object per revision.
`object_budget_used_fraction` reports the share of `Limits.MaxObjects`
(10,000) an archive consumed:

| Tier | Objects | Budget used |
|---|---|---|
| 100 | 102 | 1.0 % |
| 1,000 | 1,011 | 10.1 % |
| 5,000 | 5,052 | 50.5 % |

The relationship is linear, so the current format bounds one archive to
roughly 9,900 revisions plus resources and source bundles. That ceiling —
and the 4 MiB manifest bound that lists every object inline — is a
**format-level limitation recorded for review**, not an exporter defect: a
million-note full backup needs an archive-v2 revision that moves the object
inventory out of the manifest. The exporter refuses to exceed the documented
limits and fails before publishing a manifest, so no over-budget archive can
appear complete. See `agent/OPEN_QUESTIONS.md`.
