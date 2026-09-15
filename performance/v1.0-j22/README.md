# v1.0 J22: stores, tests and instances keep their temp files to themselves

## The defect (found by J7)

J7-C's first disaster-recovery run was stopped from outside because the machine
ran low on memory. `/tmp` is RAM-backed tmpfs, and it held 16 GB in 17,078
leftover `notrios-*` entries:
- **`notrios-assets-*` (16,888):** a store opened without a database file never
  removed the private asset root it made.
- **`notrios-test-bin-*` (171, 6.5 GiB):** the CLI tests built a shared binary
  directory and never removed it.
- **Validator Go build caches (2.6 GiB):** four validators parked them there.

When J22 began, `/tmp` again held 1,452 `notrios-assets-*` entries, 9
`notrios-test-bin-*` entries and four validator caches, 1.4 GB in all.

## Each instance keeps its temp files to itself (owner decision, 2026-09-15)

The owner added the constraint that decides the design. Several Notrios
instances can run on one machine, so each instance's temp files belong in its
own temp directory, and cleanup must never reach another instance's.
- **Placement (owner's choice):** the new per-instance path `data.temp_dir`,
  defaulting to `<cache_dir>/tmp`. Spools go to disk, not RAM-backed `/tmp`.
- **Leftovers (owner's choice):** an instance removes only its own crashed
  processes' leftovers.

Three product paths used the shared system temp root before J22:
- the path-less store's asset root
- the import manifest (packed export, restore, Joplin import)
- the archive verification spool (verify, restore, and export's own
  post-publish verification)

## J22-A: a store removes the temp asset root it created

`internal/store/j22_temp_asset_root_test.go` was written first.
- **Before the fix:** on the unchanged store, all three ways to open a
  path-less store failed ("Close left the temp asset root it created"):
  `OpenSQLite(":memory:")`, `OpenSQLite("")` and
  `OpenSQLiteWithAssetStore(":memory:", "")`.
- **The fix:** a store now records the asset root it created (`ownedAssetRoot`)
  and removes it on `Close`, and on an open that fails after making it.
- **Never removed:** the `assets` directory beside a database file, a
  caller-supplied root, or a caller-supplied root named like a temp root. All
  three cases pass.
- **Fallback gone:** when the temp root was unwritable, the store used to fall
  back to one shared `notrios-assets` directory. It now fails to open instead,
  because a directory shared by every instance is what the private one replaced.

## J22-B: one temp directory per instance

### `internal/tempspace`

A `Space` is one process's share of an instance temp directory.
- **Per process:** `Open(root)` claims a private `p-<id>/` directory beside a
  lock file `p-<id>.lock`. The process holds that `flock` for as long as the
  Space is open, and the kernel releases it when the process exits, however it
  exits.
- **Leftover removal:** before claiming, `Open` removes the pairs in **the same
  directory** whose lock it can take itself.
  - Only a regular `p-<16 hex>.lock` counts, and a `p-<id>` symlink is never
    followed.
  - Nothing else in the directory is touched.
  - No other directory is examined.
- **Race guard:** a process that locks a file a concurrent removal has just
  unlinked detects it by comparing inodes, and claims a new name.
- **Close:** removes the directory, then the lock file, then releases the lock.
  A crash at any point leaves nothing, or an unlocked pair the next `Open`
  removes.
- **No instance:** a nil Space puts each piece of temporary work in its own
  private `0700` directory in the system temp root, which its owner removes.

Its tests:
- a nil Space is private and inert
- two processes of one instance get different locked directories, and closing
  one leaves the other
- two instances' work never crosses
- a crashed process's pair, and an orphan lock file, are removed and reported
- a live process's directory is left alone
- another instance's crashed pair is left alone, as are unrelated files, a
  lockless `p-` directory, a non-hex lock name, a symlinked `p-<id>` (its target
  survives) and a stray `notrios-*` directory
- a file is refused as an instance temp directory

### Wiring

- **Config.** `data.temp_dir` is parsed, shown by `config show` (origin
  `resolved` when derived), created by `EnsureDirectories`, and moved by
  `UseDataDirectory`.
  - Unstated, it is derived from **the instance's own `data.cache_dir`**, not
    the cache root. A profile config written before the key existed therefore
    gets a directory of its own, not one shared with every profile.
  - `profile create` writes it, and the profile collision check compares it
    across profiles.
- **Store.** `store.OpenInstanceSQLite(db, assets, tempDir)` opens the store and
  its Space, and closes the Space with the store. The service (`notriosd`, and
  the GUI, which embeds it) and every `notriosctl` command that opens a store
  from config use it.
- **Temp users.** Each takes the Space from its store, through
  `store.TempSpaceOf`:
  - `OpenImportManifestIn` and `VerifyDirectoryIn` spool in it
  - archive export (the dedup index, and the post-publish verification), archive
    restore (the verification, the admitted and object indexes), and the Joplin
    importer's inventory
  - the GUI's archive verify
- **No instance.** `notriosctl verify archive-v2 <dir>` opens no instance, and
  keeps using a private system temp directory.

### Instance tests

`internal/store/j22_instance_temp_test.go` and
`internal/config/j22_temp_dir_test.go`:
- two instance stores each put an import manifest in their own process
  directory, closing one store removes its Space and not the other's, and a
  store without an instance keeps a private system temp directory
- `temp_dir` is derived from `cache_dir`, a stated value is kept, one data
  directory holds it, `EnsureDirectories` creates it `0700`, and
  `UseDataDirectory` moves it

### The drill: two real instances

`performance/v1.0-j22/instance_temp_drill.sh <work> <bin>` points HOME, the XDG
roots and `TMPDIR` into its work directory. Then:
1. It starts two `notriosd` instances, A and B, and kills A with SIGKILL.
2. It runs a packed, verified `notriosctl export archive-v2` against A **with
   the system temp directory read-only**. Temporary work there is created and
   removed within one command, so an empty directory afterwards would prove
   nothing. A directory the command cannot write to proves it never used it.
3. It checks A's temp directory, B's live directory and service, and the system
   temp directory.

**Before the export fix**, with binaries built from the tree while export still
verified outside its Space, the drill failed:

```
FAIL: the export against instance A failed with the system temp directory read-only
published archive failed verification: mkdir .../system-tmp/notrios-archive-verify-1309208877: permission denied
```

That was a gap the unit tests had not reached. `archivev2.Export` verified
through `VerifyDirectory`, with no Space. The same failed run showed a second
behaviour: a command that ends in `os.Exit` never closes its store, so its
process directory stays behind, unlocked, like a crash's. The instance's next
start removes it.

**After the fix,** all nine checks pass:

| check | result |
|---|---|
| each service holds its own process directory (`A/tmp/p-6e009a5e6d0a0aa0`, `B/tmp/p-621ac709a42b1161`) | pass |
| the two instances' process directories differ | pass |
| SIGKILL left A's pair behind, unlocked | pass |
| a packed, verified export ran against A with the system temp directory read-only | pass |
| the export removed A's crashed leftover when it started | pass |
| A's temp directory is empty after the export closed its store | pass |
| B's live process directory is untouched | pass |
| B's service is still running and answering | pass |
| nothing was written to the system temp directory | pass |

## J22-C: tests clean up, and a check keeps it that way

- **CLI tests.** `cmd/notriosctl` gains a `TestMain` that removes the shared
  binary directory after the package's tests.
- **Validator Go caches.** The G18e, G19 and G20 Makefile targets and the G18d
  validator no longer default `GOCACHE` into `/tmp`. They use the caller's
  `GOCACHE`, or Go's default cache, as `go test ./...` in `validate-scaffold.sh`
  already did. No record said why the temp caches were introduced.
- **The full-corpus benchmark.** `scripts/run_archive_full_corpus_benchmark.sh`
  built its binaries to fixed `/tmp/notrios-g14b-*` names, so two runs overwrote
  each other's. It now builds into a private directory it removes, and passes
  those paths to the harness.

`scripts/check_temp_leaks.sh` runs in CI in place of `go test ./...`, and as
`make temp-leak-check`:
1. It refuses any Makefile, shell or Python source that defaults `GOCACHE` into
   the temp root. Recorded evidence and attempt logs that quote old commands are
   not rewritten.
2. It runs `go test -count=1 ./...` with `TMPDIR` set to a fresh directory, and
   fails on any `notrios-*` entry left in it. Anything else left is listed.

**Against the unchanged tree** (`42a3a69`) the check failed. All Go tests
passed, and afterwards:
- the source check found seven lines: three Makefile targets, the G18d
  validator, and the benchmark script's cache and two fixed binary paths
- the fresh temp directory held **245 `notrios-assets-*`** and **1
  `notrios-test-bin-*`**, and nothing else

**Against the changed tree** the check passes:
- **Go caches:** none defaulted into the temp root.
- **Leftovers:** after the full `go test -count=1 ./...`, **no `notrios-*`
  entry** and nothing else was left in the fresh temp directory.

The first full run failed four documentation gates, none of them about temp
files:
- the configuration key table was stale (`cmd/docconfig`)
- docgen's enumeration counts expected 63 and 53 keys, and `data.temp_dir` made
  64 and 54 (`internal/docgen`)
- the plan block was stale (`internal/docplan`)
- `internal/tempspace` was missing from `CONTEXT_MAP.md` (`internal/docrules`)

Each was fixed: regenerated, the counts updated with a note, the atlas entry
written. Those four packages were then rerun through the same check, which
passes: all four `ok`, and no `notrios-*` entry left.

Earlier runs of the full check were stopped twice before completing, each time
because the tree changed under them. The first started before a wrong assertion
in the J22 store test was corrected: `t.TempDir()` itself lives under the
system temp root. The second started before the export verification fix. Only
the run described above is a result.

## The archive's first attempt

The first archive build, from `d0cbba6` with the usage guard on, passed the
guard: it checked only Claude, 64% remaining. It then stopped in the G18a
evidence validator, which reported `INVENTORY.json is stale`. That inventory
records a hash of each documentation page and the count of configuration keys.
J22 changed two pages (`docs/configuration.md`, `docs/service.md`) and added a
key (63 to 64). It was regenerated with `build_inventory.py --write`, and the
diff held exactly those three changes. No ZIP was written by that attempt.

Packaging stops at the first failure, so the validators after that one were run
locally before the archive was rebuilt, each recording its own exit. One more
failed: G18f, `stale generated hash docs/service.md`. Its `REPORT.json` pins
the hash of five generated pages and has no regenerator. The `docs/service.md`
hash was set to the page's new SHA-256 by hand, as J13-D did for the same
report. G18c, G18d, G18e, G18g, G19 and G20 passed as they were.

The scaffold validation then stopped in H3: `asset-store-memory: anchor occurs
0 times in internal/store/sqlite.go; the consumer has changed and the inventory
must be revisited`. H3's path-consumer inventory pins each consumer to an exact
line, and J22 replaced that one. The entry was revisited, as H4 slice C
(`771326d`) did when it last changed this anchor:
- **Anchor:** now J22's line, `root, err := noInstance.MkdirTemp("notrios-assets-")`.
- **"current":** now also records that the store owns the directory and removes
  it on Close.

Its category, defects, owner, status and H3's own target text are unchanged,
and `REPORT.md`'s description of what H3 originally found stays as it was.

Next, I8's freeze of the public surface reported that `temp_dir` had been added
to the configuration surface, and nothing removed. The addition is deliberate
and compatible: the key is optional, and a configuration that does not state it
derives it from its own `data.cache_dir`. So every existing configuration, and
every profile config written before the key existed, loads unchanged and gets a
temp directory of its own. The freeze was rebuilt with
`performance/v0.9-i8/build_freeze.py`. It moved two surfaces, and nothing else:
- **configuration:** 60 to 61 members, adding `temp_dir`
- **make lifecycle:** 36 to 37, adding `temp-leak-check`, which is additive: no
  existing target changed

I8 validates against the rebuilt freeze.

## What J22 does not change

- **`purge`.** Its plan and categories are unchanged. By default `data.temp_dir`
  sits inside `data.cache_dir`, and J22 did not check whether `purge` removes it
  there. A `temp_dir` stated elsewhere is not added to the purge plan either;
  that is recorded here, not decided.
- **Other scripts.** The drill and profile scripts that make their own
  `mktemp -d` work directories are unchanged. They remove what they make, and
  none is part of the Go test suite.
