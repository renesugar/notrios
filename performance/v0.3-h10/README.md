# H10 Recoll hardening reference profile

`profile-100000.json` is reproducible native Recoll evidence over 100,000
generated Markdown projection files:

```bash
bash scripts/run_recoll_hardening_profile.sh 100000 \
  performance/v0.3-h10/profile-100000.json
```

The scale tier uses Recoll's internal plain-text extraction for the generated
`.md` files so it measures native index/result behavior without multiplying
Python interpreter startup by 100,000. The normal `TestLiveRecollPipeline`
separately exercises the production Notrios frontmatter handler against the
installed Recoll binaries and verifies field/range queries.

The profile records generation/index/query durations, an exact bounded
1,001-result slice, incremental deletion/addition convergence, runtime memory,
and Recoll/Go environment details. It also damages a canonical 100-note
projection (one missing, one stale, one orphan), records the bounded repair,
and asserts a second reconciliation has zero drift.

This is generated public data, not a private note corpus. Results characterize
the recorded machine and are regression evidence rather than a universal
service-level objective. UI status/accessibility is covered by frontend tests
and the native Wails build gate.
