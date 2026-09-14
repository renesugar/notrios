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
So a genuinely old library is checked too: J5's real v27 library, migrated by
the fixed binary, against the same library migrated by the old one.

**J5's real v27 Obsidian library, 382,206 notes.** The copy made before
migration digests as `fc6fb625…` (231 objects, v27). Two copies were migrated,
one by each binary:

| migrated by | schema digest | rowid mapping digest |
|---|---|---|
| old binary (`17aa250` tree) | `8b7b6640…9f842` (234 objects, v28) | `a4679b01a821fb3b75fdd5732467aba7ca1c5123bc1f3342993fe984372ae0e1`, 382,206 rows |
| fixed binary (`9256f25`) | `8b7b6640…9f842` (234 objects, v28) | `a4679b01a821fb3b75fdd5732467aba7ca1c5123bc1f3342993fe984372ae0e1`, 382,206 rows |

**The same schema as every synthetic shape, and a byte-identical mapping.**
The fixed binary migrated in 217.09 s, against 210.50 s for the old binary on
the other copy. That cost is paid once. The fix still backed the library up
first, and the verified copy is in `pre-migration-backups/27-to-28-…`.

## J21-C: open and search, re-measured at full size

`open_search_profile.sh` ran with the fixed `bin/notriosctl` on J20-A's two
382,206-note libraries, and on J5's library after the fixed binary migrated it.
Everything ran one process at a time, from a warm page cache (no passwordless
`sudo`). It recorded `dd7d7f8`, the `HEAD` when it started. The only commit made
while it ran changed this README, and the binary was built from the fixed tree.

| Obsidian vault, 382,206 notes | J5 | J20-A, before the fix | J21, fixed |
|---|---|---|---|
| open: `collections list` | not measured | 12.4 s | **0.24 s** (2.33 s on the first, cold open) |
| search "the" (187,518 notes) | 9.1 s | 16.4 s | **4.1 s** warm, 7.2 s cold |
| search "quinoa" (385) | 4.9 s | 12.4 s | **0.07 s** |
| search "chicken stock" (9,703) | 5.2 s | 12.6 s | **0.35 s** |

| Joplin export, 382,206 notes | J5 | J20-A, before the fix | J21, fixed |
|---|---|---|---|
| open: `collections list` | not measured | not measured | **0.21–0.25 s** |
| search "the" | 8.6 s | 14.7 s | **4.2 s** |
| search "quinoa" | 4.3 s | 10.7 s | **0.06 s** |
| search "chicken stock" | 4.8 s | 11.1 s | **0.36 s** |

J5's library, migrated by the fixed binary, opens in 0.24–0.25 s and answers
the three probes in 4.14 s, 0.06 s and 0.35 s.

- **Opening a library now costs about a quarter of a second at 382,206 notes,
  instead of 12.** Every CLI command, server start and profile open gets that
  back.
- **J5's search numbers were never search numbers.** Its 4.9 s for "quinoa" was
  about 4.6 s of V22's re-run backfill and the rest open overhead. A rare-word
  search of the full library takes 0.06 s.
- **The common-word search is the only probe that still takes seconds.**
  Counting 187,518 matches takes about 4 s warm. That is real query work, and
  this item does not change it.
