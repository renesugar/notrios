# v1.0 J11: what an installation occupies, and whether a purge removed it

`notriosctl paths` reported the roots. `scripts/lifecycle.py` recorded the
program files it installed. Neither listed the user's data, which is the half a
purge deletes — so nothing could answer "did the purge work".

## What was added

**`notriosctl paths --report`** (`internal/installreport`, `cmd/notriosctl`):

- every directory the installation occupies and every file inside them, by
  absolute path;
- every registered profile, with the paths one keeps **outside** the roots
  marked external, because a purge deliberately keeps those;
- the program files, from the installer's `MANIFEST.json`, when it is there.

Three shapes for three readers: the default report for a person, `--json` for a
script, and `--paths` — one `owned`/`external` line per absolute path — for the
check. The first two redact the home directory to `~`, like `paths` and
`config show`; `--paths` does not, because a checker compares real paths, and
`--paths --no-redact` is refused rather than silently accepted.

**It is a flag, not a `paths report` subcommand.** The dispatch model in
`cmd/notriosctl/commands_test.go` treats a subcommand as making its parent a
group, and `notriosctl paths` is a command users run on its own. The frozen CLI
surface is unchanged at 92 members, because flags are not members.

**`scripts/check_purged.sh`** reads a `--paths` manifest taken beforehand and
needs nothing but a shell — by the time it runs, the program that could read a
manifest has been deleted. It reports two different failures:

- an owned path that still exists: the purge was incomplete;
- an external path that is gone: a purge deleted a library it was supposed to
  keep. That is the worse one, and it is reported separately.

## Two defects the work found

- **Relative roots.** A source checkout resolves relative roots (`data`,
  `data/assets`). The first manifest was therefore a list of relative paths,
  which a check running from anywhere else would have tested against whatever
  happened to sit beside it. Every path is made absolute now, with a test.
- **Overlapping roots.** In an installed layout the program-assets root *is*
  the data root, so the first clean-install capture listed all 848 files twice
  and reported 886 paths where there were 446. The first root to claim a
  directory keeps it, the duplication is gone, and the report says "the
  program_assets root is the data root" rather than absorbing it silently.

## Proof

| where | tests |
|---|---|
| `internal/installreport` | 7: every directory and file listed and sorted, a root that does not exist said plainly, external profiles marked and counted, an external path that is missing still listed, program files from the install manifest and the note when there is none, `--paths` ownership marking, JSON round trip, every path absolute even from relative roots, and overlapping roots listed once |
| `cmd/notriosctl` | 3: the report names directories, files and the external library and redacts by default; `--paths` is literal, marks ownership, and refuses the flag combinations that contradict each other while plain `paths` still works; and a full round trip — report, delete every owned path, check passes, put one file back, check fails naming it |
| `scripts/test_check_purged.py` | 8: a complete purge passes; one file left fails and names it; a dangling symlink counts as left behind; deleting a kept library is reported separately; a manifest that is not one, an empty one, and a missing one are each refused with a different message; a path with spaces is handled |

## The clean install, captured by running one

`capture_clean_install.sh` installs into a disposable `HOME` outside the
checkout — inside one, the CLI resolves source mode and would report the
developer's own library — replaces the staged binary with a fresh build, makes a
note, takes the report three ways, purges, and runs the check twice.

`CLEAN_INSTALL.txt` and `CLEAN_INSTALL.json` are that installation, redacted.
`PURGE_CHECK.json` records the run: **446 owned paths before the purge, 0
external**, the check said `purge verified: 446 owned path(s) gone, 0 external
path(s) still present`, and with one file put back it said `the purge left 3 of
446 path(s) behind` — three because restoring a file restores its parent
directories too.

The script's own first run is why the binary is rebuilt: the install staged an
older `notriosctl` from a previous build, which did not have `--report` and
printed a flag error into the record.

## Documentation

`docs/installation.md` gains "Checking that a purge removed everything", after
the removal instructions rather than inside them — putting it inside moved an
existing example into the new section and orphaned its registered id.

Both of its examples report on and then delete this user's own installation, so
they carry the `shared-user-state` unrun reason naming this item's drill.
Pinned evidence moved with them: docaudit (287 sections, 172 executables, 360
unverified, denominator 489), docexec and G18d (172 entries, 102 unrun
reasons, executed unchanged at 70), J13's classification, and the G18a
inventory. `docs/cli.md` is regenerated from the CLI spec.

**Validation.** The whole Go suite passes through `scripts/check_temp_leaks.sh`,
which left no `notrios-*` entry. `validate-scaffold.sh` and
`make g18g-validate` pass.

Scaffold validation caught two things on the way: a `notriosctl` binary left in
the repository root by a `go build` without `-o`, which is the class of mistake
that check exists for, and the G18a baseline and G18f document hash that the new
documentation section moved.

## The archive

Built from `42bf484` with the usage guard on and no override:
- **Usage guard:** it checked only Claude, 28% of the five-hour window
  remaining, above the 20% reserve, identified by process ancestry (J24).
- **Package:** `package_release.sh` passed, including `go test ./...`,
  `validate-scaffold.sh`, `make g18g-validate` and every evidence validator.
- **ZIP:** `notrios-v1.0-j11-42bf484.zip`, 25,718,914 bytes, 2,698 entries.
  `check_release_zip.py` accepts it.
