# v0.7 G6 convergence evidence

This directory indexes the deterministic evidence implemented as executable
tests in `internal/syncmerge/merge_test.go` and
`internal/store/sync_metadata_test.go`. It contains no private note content or
paths and adds no production dependency.

`results.json` records the fixed schedules and expected counts. Reproduce the
focused evidence with:

```bash
go test ./internal/syncmerge ./internal/store -run 'TestConverge|TestPurgeCertificate|TestOrderTie|TestSyncMetadata|TestLocalJournalAssigns|TestEnrolledLocalPurge|TestSchemaV21'
```

The pure model permutes eight concurrent metadata/lifecycle/membership/tree
operations exhaustively and then runs 250 deterministic randomized
shuffle/duplicate schedules. The SQLite test applies two four-operation peer
streams in opposite orders, compares ordered canonical snapshots and repair
reports, and checks foreign keys plus ordinary Store reads after each admitted
transaction.

These are correctness fixtures, not throughput or mobile performance claims.
G7/G8 add body/resource dependency graphs, and G9 adds authenticated framing.
