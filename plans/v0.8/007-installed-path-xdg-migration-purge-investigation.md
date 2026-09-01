# v0.8 H3 — Installed-path, XDG, migration, and destructive-lifecycle investigation

Date: 2026-09-01
Status: complete investigation; contract proposed, no runtime change
Model: Claude Opus 5 (`claude-opus-5`) via Claude Code 2.1.252, parent-owned
throughout; no subagents were used.

## Outcome

**A per-OS path contract, a migration state machine, and a fail-closed
install/uninstall/purge design are specified and evidenced.** No path default
changed, no data moved, no Make lifecycle target was added, and nothing was
deleted. `internal/config`'s `./data` defaults are untouched, and
`validate_evidence.py` asserts that they are — an investigation that quietly
implemented itself would be the failure mode worth guarding against.

Evidence: `performance/v0.8-h3/`.

## What the audit actually found

Notrios has no path resolver. It has 25 places that each decide something about
location, and every defect worth reporting is a **disagreement between two of
them** rather than a mistake inside one.

**Two answers to "where is the config root."** `internal/synckeys` asks
`os.UserConfigDir()`, which refuses a relative `XDG_CONFIG_HOME` — the XDG
specification says such a value is invalid and must be ignored. `internal/profiles`
hand-rolls the same lookup and accepts it, resolving the profile registry
against the process working directory. In that environment one half of the
application fails closed and the other invents a location, for the same user, in
the same process. With no `HOME` at all the registry degrades to the bare
relative path `.notrios/profiles.json`, so which database a `notrios://` link
resolves to depends on the directory the process was started in.

**The library lives in the config root.** A generated runtime profile places its
database, asset store, projections, search index and quarantine under the
directory holding the registry — `~/.config/notrios/profiles/<id>/data/notes.sqlite`.
Observed by creating a real profile, not inferred from source. The generated
`.yaml` beside it is correctly placed; everything else is a user's entire
library inside the one root they are most likely to sync with a dotfile manager
or commit to a repository. This is the only consumer that genuinely needs
migration rather than a new default.

**Configuration and interface are read from the working directory.** With no
`--config`, `config/config.example.yaml` is loaded relative to the current
directory, and it decides the database path, listen address, public base URL and
remote-media policy. `WebRootCandidates` likewise puts `web/dist` ahead of the
executable's own directory, so the HTML and JavaScript loaded into the
application window come from wherever the user was standing. Both are right in a
checkout and wrong once installed; the second is the sharper one, because it is
a content-injection path into the application's own window.

**The backups are protected and the originals are not.** Thirty-three sites
create derived artifacts — carrier spools, sync backups, staging, the catch-up
inbox, the registry, sync keys, snapshot images — 20 directories at `0700` and
13 files at `0600`.
`config.EnsureDirectories` creates the primary roots at `0755`. On a shared
machine the encrypted backup of a user's notes is owner-only and the notes are
world-readable. Sharper still: `config.EnsureDirectories` creates the data
directory `0755` and `publish.File.Save` creates the same directory `0700`, so
its permissions depend on which code path ran first.

Two smaller ones by source audit: the `:memory:` asset root is a single fixed
`os.TempDir()/notrios-assets` shared by every user and instance, and `parentDir`
splits on `"/"` rather than using `filepath.Dir`, so a Windows database path
would put its asset store in the working directory.

## Two things in scope that turn out not to exist

The plan lists **logs** among the paths to place. Notrios writes no log file:
everything goes to stderr through the standard library logger, so the
supervisor decides where it lands. There is nothing to relocate, and adding a
log root would be inventing a consumer rather than placing one. Recorded in
`LAYOUT.json` under `no_path_consumer` so a later reader does not conclude it
was overlooked.

**Snapshot image output** has no default at all — the caller names it, which is
the point of a portable snapshot — and is already careful, refusing a
destination overlapping the asset store, a symlink, a non-directory, or an
existing complete snapshot. Inventoried as an external consumer with an
exception rather than given a root.

## Two things the first pass got wrong

Recorded because an audit's completeness is a claim like any other.

**SQL migrations are `//go:embed`ded.** An earlier draft of `LAYOUT.json` listed
`$(datadir)/notrios/migrations/` as an installed artifact, which would have
shipped files nothing reads and invited a stale on-disk copy to diverge from the
compiled one. Removed, and recorded under `embedded_not_installed` so it is not
re-added. The Recoll Markdown handler is embedded the same way and then written
*out* at runtime, which is why it is cache rather than an installed artifact.

**One consumer was missed.** `notriosctl seed-help` defaults its documentation
source to the bare relative path `docs`, so seeding the Help notebook from an
installed binary reads whatever `docs` directory the user is standing next to.
It is consumer 24, owned by H4-A1. Found by sweeping for `//go:embed` rather
than by the original grep for path literals — the two sweeps disagree, and the
disagreement is what surfaced it.

## What was built to prove it

Three executable artifacts, each checked against a deliberate regression before
being trusted:

- **`pathprobe/`** — Go tests characterizing today's behavior against the real
  packages. Every claim marked *observed* in `REPORT.md` comes from one, and
  `validate_evidence.py` refuses a report that cites a test which does not
  exist. Flipping `EnsureDirectories` to `0700` fails the permission test with a
  message naming the H4 change that caused it.
- **`resolve_model.py`** — an executable model of the proposed rules, generating
  the 14-scenario `RESOLUTION_TABLE.json`. A table typed into a document can be
  internally inconsistent; a generated one cannot.
- **`purge_oracle.py`** — the deletion decision procedure, 30 fixtures against a
  real temporary filesystem so symlink and containment rules are decided by the
  kernel rather than by string comparison.
- **`test_backup_restore.py`** — the restore proof, executed rather than
  specified: build the container, verify it, delete the source roots *through
  the oracle*, restore offline, compare every byte and mode back. It asserts the
  ordering guarantee — a truncated archive fails verification, and nothing is
  deleted while verification fails. Omitting a whole root from the backup was
  tried and the manifest caught it, which is exactly the failure that would let
  purge delete data it had not backed up.

`PATH_CONSUMERS.json` anchors all 25 consumers to exact source substrings that
must occur **exactly once**. When H4 changes a consumer the inventory breaks,
rather than quietly describing code that no longer exists.

## The contract

Six roots separated by what the user loses if each disappears: config, data,
state, cache, runtime, and immutable program assets. Every mutable root `0700`.
Full matrix in `LAYOUT.json`.

Two classifications that are judgements rather than conventions. **Quarantine is
state, not cache** — it sounds disposable, but it is the record of what a note
tried to fetch and the only copy of media a user may have approved without
localizing, and calling it cache would let `purge` delete it without backup.
**`XDG_RUNTIME_DIR` has no specified fallback**, and inventing one in `/tmp`
would put staged plaintext backups somewhere world-traversable, so Notrios falls
back to `<state>/runtime` at `0700` and says so — and refuses a runtime
directory that exists but is not owner-only.

Precedence: explicit, then an explicit portable marker, then installed, then
source. **Portable mode is never inferred** from the current directory or from
that directory being writable; guessing portability from writability makes a USB
stick and a home directory the same decision.

Migration copies rather than moves, verifies by SHA-256, journals per root so an
interruption resumes, and *renames* the old layout rather than deleting it.
Cache is rebuilt, never migrated — a copied Recoll index carries stale absolute
paths. A destination that already holds a database is refused, not merged:
merging two libraries is a decision, not a copy.

Purge is closed by default. A target is refused unless positively identified as
a path under a root Notrios owns; unknown, outside, unreadable, crossing a
mount, reached through a symlink, or named by an external profile all refuse.
The asymmetry is the design: a wrongly refused deletion costs a manual `rm`, a
wrongly allowed one costs a library.

## Two things worth keeping

Containment is decided on the **resolved** path, so a symlink cannot look
contained while pointing outside. Replacing `realpath` with textual
normalization was tried: the fixtures caught it, two of them via the *separate*
symlink rule. The overlap is deliberate.

An **unclassified** backup category defaults to `backup_and_verify`. A new root
added later without a policy must not be silently disposed of.

## Open decisions, now answered

All three were non-blocking for H3 and are answered with the recommended
defaults; H4 and H5 must restate them.

1. **Installed versus portable precedence** (blocking for H4) — explicit first,
   explicit marker second, native locations otherwise; never inferred. Modelled
   and observed in `RESOLUTION_TABLE.json`.
2. **Default `make install` destination** (blocking for H5) — user-local
   `$HOME/.local`, GNU directory variables and `DESTDIR` retained. `DESTDIR`
   stages immutable artifacts only and creates no user root.
3. **Purge treatment of external profile paths** (blocking for H5) — enumerate
   and back up, refuse to delete. Fixtured in both directions: a target that
   *is* an external path, and a target that *contains* one.

H5 additionally needs a backup container decision. Recommended: one owner-only
tar with a per-file SHA-256 manifest, written under `$XDG_STATE_HOME` outside
every purge root by construction, refusing rather than relocating if that
destination is unsafe.

## Not verified

- **No Windows or macOS execution.** Those matrix rows and the `parentDir`
  separator defect are read from source and Go's documented behavior;
  `os.UserConfigDir` was executed on Linux only.
- **The mount-boundary rule is modelled**, not run against a real mount; the
  device lookup is injected.
- **Most failure models are designs**, not tests: permission, disk-full,
  mount, collision and concurrency. H4 and H5 own them. The
  interrupted-backup case is the exception — it is executed, because it is the
  failure most likely to happen and the one deletion waits behind.
- **No migration was performed** — there is no migration code to test yet.
- **The resolver model is Python.** H4 implements it in Go and must reproduce
  the table; the two agreeing is not yet demonstrated, because one side does not
  exist.

## Validation

```
go test ./...                                     all packages pass
make validate                                     scaffold validation passes
go test ./performance/v0.8-h3/pathprobe/          8 characterization tests
python3 -m unittest discover -s performance/v0.8-h3 -p 'test_*.py'
                                                  10 tests: 30 purge fixtures,
                                                  14 resolution scenarios,
                                                  6 backup/restore cases
python3 performance/v0.8-h3/validate_evidence.py  25 consumers, 6 roots,
                                                  9 owning changes
```

The two Python fixture files are `unittest`-based rather than plain scripts:
the repository's other `test_*.py` files are discovered with
`unittest discover`, and a module with no `TestCase` would have been discovered,
found empty, and reported as passing.

`gofmt -l` reports only the pre-existing `internal/markdownblocks/parser_test.go`
deviation, left alone. `go build ./...` still fails on the pre-existing
`performance/v0.8-h0/abi-probe` host program, unchanged by this slice.

No push, PR, merge, tag, release/upload, evidence-reserve write, ISO, or
physical burn was performed.

## Commits

- `6e0f617` investigate installed paths, XDG, migration, and purge (v0.8 H3)
