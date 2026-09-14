# v1.0 J21: stop re-running schema migrations on every open

## The defect (found by J20-A)

`Bootstrap` → `applySchema` ran every schema step on every open. Two things
lowered the version on the way:
- `0001_initial.sql` ends at `PRAGMA user_version = 17`
- the unguarded `ensureSchemaV4`–`V18` end at 18

So every guarded step from V19 on found the version below its own and ran
again. The last step restored 28, so the file on disk always read 28. On the
382,206-note Obsidian library, `Bootstrap` took 12.1 s per open:
- ~7 s rebuilding V28's full-text rowid mapping (J18)
- ~4.6 s in V22's revision backfill, present before J5

Every CLI command paid this, read-only ones included, as did every server
start, the ABI registry and every external-profile open.
`performance/v1.0-j20/README.md` has the profile.

## J21-A: the fix, and the test that failed first

`applySchema` now reads `user_version` once. At the current version it skips
`0001_initial.sql` and V4–V28, and runs only the open invariants:
- the database identity
- the default collection
- the built-in notebooks
- the sync metadata baseline

Each of those is an insert-or-ignore, or a read that writes nothing when the
row exists. A fresh library reads 0 and a library being migrated reads below
28, so both still run every step, the latter under the existing lock, backup
and marker.

`TestJ21CurrentLibraryRunsNoMigrationOnOpen` (`internal/store/j21_open_test.go`)
creates a current library with one note, then plants a value each expensive
step would repair: a full-text mapping row shifted by 1,000,000 (V28) and an
empty `content_sha256` (V22). Then it reopens. **On the old code it failed on
both counts:**

```
schema v28's mapping rebuild ran again on open: the planted rowid (+1000000) was repaired
schema v22's revision backfill ran again on open: the planted empty content_sha256 was filled
```

With the fix it passes. The store package's full suite passes, including the
migration-backup, lock and marker tests in `premigration_test.go`.

## J21-B: every library shape reaches the same schema

`TestJ21SchemaShapes` writes a library for each way to reach the current
schema:
- fresh
- current, reopened
- current with `user_version` lowered to 4, 18, 19, 21, 22 or 27, then
  migrated

`schema_digest.py` hashes each library's `sqlite_master` rows (type, name,
table, SQL) and `user_version`.

| code | digest of all eight shapes |
|---|---|
| before the fix | `8b7b6640fb0be8589c738454af6a71d3ed2cffe0a4f8545019746d9848f9d842` (234 objects, v28) |
| after the fix | `8b7b6640fb0be8589c738454af6a71d3ed2cffe0a4f8545019746d9848f9d842` (234 objects, v28) |

All 16 libraries have one digest.

**Why skipping cannot lose an object.** A current library could lack an object
only if an early step gained one after later versions existed. Git shows
`0001_initial.sql` last changed on 2026-08-07, and V4–V18 have not changed
since V19 was added on 2026-08-12 (`86aa149`). Every object those steps create
existed before any library could reach v19.

**Every caller of `Bootstrap` still passes its tests:**
- `internal/service` (server start)
- `internal/profiles` (external-profile open)
- `internal/abi` (registry)
- `internal/syncstate`
- `internal/archivev2`, which includes the archive contract's schema range and
  restore paths
- `cmd/notriosctl`

**Schema gates.** G19's evidence validates against the fixed tree (5 schemas,
3 complete goldens, 12 matrix cases, physical refusal) and so does G20's
(0.7.0/schema-v27, 11 closed gates).

The "migrated" shapes above hold the current schema with the version lowered.
A genuinely old library is checked below: J5's real v27 library, migrated by the
fixed binary, compared with the same library migrated by the old one.
