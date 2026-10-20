# v1.0 J44: the search sidecar indexes the projection, and nothing else

After Recoll was reinstalled, `TestLiveSidecarStartupRepairsDamageAndReportsStatus`
ran for more than five minutes instead of a few seconds. The process doing the
work was `rcljoplin.py`, Recoll's Joplin indexer, and it was reading the owner's
own Joplin library at `~/.config/joplin-desktop/database.sqlite` into the test's
temporary index. It ignored SIGTERM and was killed with SIGKILL. The temporary
index, about 170 MB by then, was deleted. Nothing it held could have reached an
archive: the test's temporary directories are outside the worktree an archive is
built from.

## Why it could happen

Three facts about Recoll, which none of Notrios' configuration accounted for:

- **External indexers are declared in a `backends` file** in the configuration
  directory. Unlike Recoll's other configuration files, it takes nothing from the
  shared defaults.
- **recollindex copies its sample `backends` into any configuration directory
  that has none.** The sample shipped with Recoll 1.44 declares `[JOPLIN]` with
  the indexer enabled.
- **`topdirs` does not bound an external indexer.** The Joplin indexer finds its
  library under `HOME`, or wherever `joplindbpath` says, whatever directories the
  configuration names.

So Notrios' sidecar, whose configuration named only the projection directory,
took in another application's notes on any machine with that Recoll and a Joplin
library. Its tests isolated `XDG_CONFIG_HOME`, `XDG_RUNTIME_DIR` and
`RECOLL_CONFDIR`, but not `HOME`, so a test run reached the owner's real library.
The Recoll build in use is dated 2026-07-14, so this was possible from then on,
not only after the reinstall.

## J44-A: the sidecar declares no external indexer

`EnsureConfig` writes a `backends` file that declares no section, with a comment
saying why. It rewrites the file on every start, so a file someone edited, or
one Recoll installed under an older build, is replaced.

- `TestEnsureConfigDeclaresNoExternalIndexers` pins this without a Recoll
  binary. It plants a `[JOPLIN]` file and checks that the next `EnsureConfig`
  replaces it.
- Both live tests call `recolltest.AssertNoExternalIndexers` after a real index
  pass, because that is when recollindex would have installed its sample.
  Removing the write makes `TestLiveRecollPipeline` fail, and the failure message
  quotes Recoll's own `[JOPLIN]` sample. So the check is shown to fire, not
  assumed to.

## J44-B: the tests do not reach the owner

`internal/recoll/recolltest` is a test-support package. Its `Isolate` moves
`HOME` and every XDG directory under the test's own temporary directory: config,
data, state, cache, runtime, and the shared `XDG_CONFIG_DIRS` and
`XDG_DATA_DIRS`. All three tests that run a real Recoll binary use it.
`TestEveryLiveRecollTestIsolates` scans the repository's test files and fails the
build if one that runs Recoll does not call it.

Seeing which files a test actually opens needs syscall tracing, which the test
suite cannot assume. `scripts/check_recoll_isolation.sh` does it on demand. It
runs the two live tests under `strace` and fails on either of two things: a path
under the real `HOME` opened outside the repository and Go's caches, or any
Recoll external indexer started. It skips when `strace`, `recollindex` or
`recollq` is missing.

| run | traced calls | owner's paths opened | external indexer started |
|---|---|---|---|
| real `HOME`, with both protections | 1,962 | none | none |
| synthetic home, both protections removed | — | the synthetic `joplin-desktop/database.sqlite` | `rcljoplin.py` |

The second row shows that both detectors fire. An earlier attempt watched each
process's open files through `/proc`, and saw nothing during a run. That result
proved nothing: one sweep took more than two minutes, while the tests finished in
about four seconds. It is not evidence and is not counted here.

With the Joplin indexer gone, the sidecar startup test went from more than five
minutes to under two seconds. The scripts that start a service, `mvp_smoke.sh`
and the J7 cross-version drill, keep the sidecar disabled, and the HTTP tests use
a fake status, so no other path runs Recoll.

## J44-C: what an existing instance's index holds

An instance that ran an older build keeps whatever the Joplin indexer added
until its index is rebuilt. Restarting does not clean it. This was shown with a
drill using Recoll 1.44 and a synthetic library of 30 Joplin notes, under an
isolated `HOME`:

| step | projection documents | Joplin documents |
|---|---|---|
| configuration as the old build wrote it, first index pass | 3 | 30 |
| J44-A's `backends` written, one projection change, the next incremental pass | 4 | 30 |
| `recollindex -z` | 4 | 0 |
| index directory deleted, configuration regenerated, first pass | 4 | 0 |

Recoll tags an external indexer's documents with `rclbes`, so that its own purge
leaves them to that indexer. Once the indexer is gone, nothing purges them.
Projection reconciliation compares files with the store, not the index with the
store, so it cannot reach them either.

**They never reach a search page.** `documentIDFromURL` accepts only `file:`
URLs under the projection's `notes` directory, and
`TestParseResultsRejectsHostileAndDuplicateRows` now has a `joplin://` row to pin
that. **They do take places in recollq's window of 1,001 results** before they
are dropped, so a query matching many Joplin notes can lose matches that only
Recoll would have found. FTS5 results are not affected.

`docs/troubleshooting.md` has a row saying who is affected, a check, and the
fix. The check is `recollq -c <index_dir> -Q rclbes:JOPLIN`, which prints a count
and no note. The fix is to stop the service, delete the index directory, and
start the service again. A `recollindex -z` run by hand is not the documented
fix: run before the upgraded build has rewritten `backends`, it would start the
Joplin indexer again.

## What this does not prove

- **The trace is opt-in.** It needs `strace` and permission to trace a child, so
  it is not part of the scaffold validation or the archive build. It should be
  run after any Recoll upgrade.
- **It recognises an external indexer by name:** an `execve` of an `rcl*.py`
  other than Notrios' own handler. An indexer named differently would be caught
  only if it opened a path under `HOME`. A future Recoll that declared indexers
  somewhere other than `backends` would need J44-A revisited.
- **No automatic rebuild.** The only sign an instance was affected is the old
  build's `backends` file, which J44-A's write replaces. Rebuilding on its own
  would mean reading that file before `EnsureConfig` overwrites it. That is a
  code change, and under J8 it would be an item of its own.
- **The owner's own installed instance was not checked.** The check above is the
  way to do it.

There are no measurement directories here. This item's evidence is its tests,
`internal/recoll/recolltest/recolltest_test.go`,
`internal/recoll/recoll_test.go` and `internal/service/service_test.go`, the
opt-in `scripts/check_recoll_isolation.sh`, and the runs recorded above.
