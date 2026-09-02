# v0.8 H3 — Installed paths, XDG, migration, and destructive lifecycle

**Investigation. Nothing in this slice changes a path, moves data, adds a Make
target, or deletes anything.** It audits where Notrios puts files today,
proposes the contract H4 and H5 implement, and leaves behind evidence that can
be re-run rather than re-read.

Three artifacts here are executable, and the claims below marked **observed**
come from them:

| Artifact | What it does |
| --- | --- |
| `pathprobe/` (Go tests) | Characterizes current behavior against the real packages |
| `resolve_model.py` + `test_resolve_model.py` | Runs the proposed rules, generating `RESOLUTION_TABLE.json` |
| `purge_oracle.py` + `test_purge_oracle.py` | Runs the proposed deletion rules against a real temporary filesystem |
| `test_backup_restore.py` | Builds, verifies, deletes through the oracle, and restores, in a sandbox |

`PATH_CONSUMERS.json` anchors all 25 consumers to exact source substrings, and
`validate_evidence.py` re-checks every anchor. When H4 changes a consumer, the
inventory breaks instead of quietly describing code that no longer exists.

---

## 1. What the audit found

Notrios has no path resolver. It has twenty-five places that each decide
something about location, and the interesting defects are all disagreements
between them rather than mistakes inside any one.

### 1.1 Two answers to "where is the config root" — observed

`internal/synckeys` asks `os.UserConfigDir()`. `internal/profiles` hand-rolls
the same lookup. They agree until the environment is unusual, and then they
disagree about where the user's own files are.

The XDG specification says a relative `XDG_CONFIG_HOME` is invalid and must be
ignored. Go implements that by refusing: `path in $XDG_CONFIG_HOME is relative`.
The hand-rolled version accepts it, so with `XDG_CONFIG_HOME=relative/config`:

- `synckeys.DefaultPath` fails — Notrios cannot find its sync keys;
- `profiles.DefaultPath` returns `relative/config/notrios/profiles.json`,
  resolved against whatever directory the process was started in.

One half of the application fails closed and the other invents a location, in
the same process, for the same user. `TestRelativeXDGConfigHomeIsAcceptedByProfilesAndRefusedByTheStandardLibrary`
(retired by H4 slice B; see §9a).

With no `HOME` at all it degrades further: the registry becomes the bare
relative path `.notrios/profiles.json`, so the registry a `notrios://` link
resolves through depends on the current directory. Two invocations of
`notriosctl` from two directories are two different machines as far as link
routing is concerned. `TestProfileRegistryFallsBackToACurrentDirectoryRelativePath`
(retired by H4 slice B; see §9a).

### 1.2 The library lives in the config root — observed

A generated runtime profile puts its database, asset store, projections, search
index and quarantine under the directory holding the registry. Observed, with a
real profile created by `profiles.Create`:

```
registry  ~/.config/notrios/profiles.json
database  ~/.config/notrios/profiles/profile_<id>/data/notes.sqlite
assets    ~/.config/notrios/profiles/profile_<id>/data/assets
config    ~/.config/notrios/profiles/profile_<id>.yaml
```

The generated `.yaml` is in the right place. Everything else is a user's entire
library inside `~/.config`, which is the one root a user is most likely to copy
between machines, sync with a dotfile manager, or commit to a repository.

This is the largest change H4 has to make and the only one that genuinely needs
migration rather than a new default.
`TestGeneratedProfileDataIsPlacedUnderTheConfigRoot`.

### 1.3 Configuration and interface are read from the working directory — observed

With no `--config`, `notriosd` and `notrios` load `config/config.example.yaml`
relative to the process working directory. That file decides the database path,
the listen address, the public base URL, and the remote-media policy including
`allow_private_networks`.

`WebRootCandidates` similarly puts `web/dist` relative to the working directory
*ahead of* the executable's own directory, and `notriosctl seed-help` defaults
its documentation source to the bare relative path `docs`. The HTML, CSS and JavaScript loaded
into the application window are taken from the current directory in preference
to the assets shipped with the binary.

Both are correct and convenient in a checkout. Both are wrong the moment the
binary is installed, and the second is the sharper one: it is a content-injection
path into the application's own window, gated only on where the user happened to
be standing when they launched it. The existing comment in `webassets.go`
explains why the working directory comes first — "a developer running from a
checkout means the checkout they are standing in" — and that reasoning is right
for source mode and only for source mode.

`TestDefaultConfigurationIsReadFromTheCurrentWorkingDirectory` (retired by H4
slice B; see §9a), `TestWebRootPrefersTheWorkingDirectoryOverTheExecutable`.

### 1.4 The backups are protected and the originals are not — observed

Every derived artifact is created owner-only: sync carrier spools, sync
backups, backup staging, restore review, the catch-up inbox, the profile
registry, generated profile configs, sync keys, snapshot images — 33 sites:
20 directories at `0700` and 13 files at `0600`.

`config.EnsureDirectories` creates the primary roots at `0755`: the directory
holding the database, the asset store, the projections, the search index, and
the quarantine of untrusted downloads.

So on a shared machine the encrypted backup of a user's notes is owner-only and
the notes themselves are world-readable.

It is sharper than two constants in two packages: **two code paths create the
same directory with different modes.** `config.EnsureDirectories` creates the
data directory `0755` at startup; `publish.File.Save` creates it `0700` on its
way to writing `publish-profiles.json`. Whichever runs first on a given machine
decides, nothing reconciles them afterwards, and neither is wrong on its own
terms. `TestTheDataDirectoryGetsDifferentModesDependingOnWhoCreatesIt`. The test sets the umask to zero so it
observes the mode the program *asked for* rather than the mode this machine's
umask happened to allow — a umask of `077` masks this without fixing it, which
is exactly why the requested mode is the thing worth asserting.
`TestPrimaryDataRootsAreCreatedWorldReadable`.

### 1.5 Two smaller defects, found by source audit

`defaultAssetRoot` for a `:memory:` database returns
`os.TempDir()/notrios-assets` — one fixed name, shared by every user and every
instance on the machine. Two instances collide, and on a multi-user machine the
name is guessable and the directory is created by whoever gets there first.

`parentDir` in `internal/store/sqlite.go` splits on `"/"` with
`strings.LastIndex` rather than using `filepath.Dir`. On Windows a database path
using backslashes yields `"."` as its parent, so the asset store lands in the
working directory. Nothing exercises this today because no Windows build runs;
it is recorded so H4 does not discover it later.

---

### 1.6 Two things in scope that turn out not to exist

The plan lists **logs** among the paths to place. Notrios writes no log file:
every diagnostic goes to stderr through the standard library logger, so the
supervisor decides where it lands — journald under a systemd unit, the terminal
otherwise. The audit's answer is that there is nothing to relocate, and adding a
log root would be inventing a consumer rather than placing one. Recorded in
`LAYOUT.json` under `no_path_consumer` so a later reader does not conclude it
was overlooked.

**Snapshot image output** is fully caller-supplied with no default, and it is
already careful: it refuses a destination overlapping the asset store, a
symlink, a non-directory, or an existing complete snapshot, and creates `0700`.
It is inventoried as an external consumer with an exception rather than given a
root — the user naming the destination is the point of a portable snapshot.

### 1.7 One consumer and one artifact the first pass got wrong

Recorded because the audit's own completeness is a claim like any other.

A sweep for embedded assets found that **SQL migrations are `//go:embed`ded**,
so an earlier draft of `LAYOUT.json` listing `$(datadir)/notrios/migrations/` as
an installed artifact would have shipped files nothing reads — and invited a
stale on-disk copy to diverge from the compiled one. Removed, and recorded under
`embedded_not_installed` so it is not re-added. The Recoll Markdown handler is
embedded the same way and then *written out* into the generated Recoll config
directory at runtime, which is why it is cache rather than an installed artifact.

The same sweep found a consumer the first pass missed: `notriosctl seed-help`
defaults its documentation source to the bare relative path `docs`, so seeding
the Help notebook from an installed binary reads whatever `docs` directory the
user was standing next to. It is consumer 24, owned by H4-A1.

## 2. The proposed layout

Full data in `LAYOUT.json`. Six roots, separated by *what the user loses if it
disappears* rather than by what is convenient to co-locate.

| Root | Holds | Linux | Windows | macOS |
| --- | --- | --- | --- | --- |
| config | service config, profile registry, generated profile configs, sync keys | `$XDG_CONFIG_HOME/notrios` | `%APPDATA%\Notrios\Config` | `~/Library/Application Support/Notrios/Config` |
| data | database, asset store, publication profiles | `$XDG_DATA_HOME/notrios` | `%LOCALAPPDATA%\Notrios\Data` | `…/Notrios/Data` |
| state | carrier spools, sync backups, catch-up inbox, quarantine | `$XDG_STATE_HOME/notrios` | `%LOCALAPPDATA%\Notrios\State` | `…/Notrios/State` |
| cache | projections, Recoll index, generated Recoll config | `$XDG_CACHE_HOME/notrios` | `%LOCALAPPDATA%\Notrios\Cache` | `~/Library/Caches/Notrios` |
| runtime | backup staging, restore review, per-process temp asset roots | `$XDG_RUNTIME_DIR/notrios` | `%LOCALAPPDATA%\Notrios\Runtime` | `$TMPDIR/Notrios/Runtime` |
| program assets | web interface, migrations, seed docs, desktop metadata | `$datadir/notrios` | beside the executable | `Notrios.app/Contents/Resources` |

Every mutable root is created `0700`.

**Quarantine is state, not cache.** It holds untrusted downloaded bytes, which
sounds disposable, but it is also the record of what a note tried to fetch and
the only copy of media a user may have approved without localizing. Classifying
it as cache would make `purge` delete it without backup.

**`XDG_RUNTIME_DIR` has no specified fallback**, and inventing one in `/tmp`
would put staged plaintext backups in a world-traversable directory. Notrios
falls back to `<state>/runtime` at `0700` and says so. If the variable is set
but the directory is not owner-only, it is refused and the same fallback used —
observed in the resolution table.

**XDG variables are honoured on Linux only.** A user who exports
`XDG_CONFIG_HOME` for another tool has not asked Notrios to leave `~/Library`.

---

## 3. Precedence, and the mode state machine

```
explicit  ──▶ a path given on the command line or in an explicitly named config file
portable  ──▶ a marker file beside the executable
installed ──▶ the native per-OS roots
source    ──▶ a checkout detected beside the executable or working directory
```

Explicit wins over everything, including portable and source mode. An explicit
*relative* path is refused rather than resolved — observed.

**Portable mode is never inferred.** Not from the current directory, not from
that directory being writable, not from the absence of installed roots.
Guessing portability from writability makes a USB stick and a home directory
the same decision, and the wrong guess writes a user's library somewhere they
will not find it.

Three behaviors become source-mode-only:

- reading `config/config.example.yaml` from the working directory;
- resolving `web/dist` from the working directory;
- the `./data` default layout.

`RESOLUTION_TABLE.json` is generated from `resolve_model.py`, so the table and
the rules cannot disagree. 14 scenarios, all matching, covering the XDG
absolute-path requirement, empty-is-unset, the runtime-dir ownership check, the
no-`HOME` refusal, Windows and macOS ignoring XDG, portable selection, and
explicit-path precedence.

---

## 4. Migration

Migration is **explicit, atomic per root, resumable, and reversible.** It is
never implicit: an installed binary that finds a checkout-relative layout
reports it and exits rather than moving a user's library because it felt able
to.

```
detect ──▶ plan ──▶ backup ──▶ verify ──▶ copy ──▶ verify ──▶ commit ──▶ retire
   │         │         │          │         │         │          │
   └─ nothing to do    └──────────┴─────────┴─────────┴──────────┴─▶ rollback
```

- **detect** — an old layout exists and the new roots are empty or absent.
- **plan** — write `<state>/migration/plan.json`: every source, every
  destination, sizes, and free space at the destination. Refuse if free space is
  under twice the source size.
- **backup** — copy, never move. The source is untouched until commit.
- **verify** — SHA-256 every copied file against the source.
- **copy/commit** — write a journal entry per root before and after. A restart
  reads the journal and resumes at the last incomplete root.
- **retire** — the old layout is *renamed*, not deleted, to
  `<old>.migrated-<timestamp>`, and the user is told where it is. Deletion is a
  separate, later, explicit act.

Per category: config and data **move**; state **moves**; cache is **rebuilt**,
never migrated — copying a Recoll index to a new path would carry stale absolute
paths inside it; runtime is **discarded**.

**Collision** — a destination that already holds a database is never merged.
Migration refuses and names both paths. Merging two libraries is a decision, not
a copy.

**Interruption** — the journal makes a partial migration resumable, and because
the source is intact until commit, the worst case is wasted disk rather than a
lost library.

---

## 5. Failure models

Modelled unless marked executed — H4 and H5 own the rest. Recorded here so they
are designed rather than discovered.

| Failure | Behavior |
| --- | --- |
| Destination not writable | Refuse before copying anything; name the path and the required mode |
| Disk full mid-copy | Journal marks the root incomplete; source intact; resume or roll back |
| Root is a symlink | Resolve and report; refuse if it leaves the user's own tree |
| Root crosses a mount boundary | Permitted for a *configured* root; refused as a purge target (§7) |
| Path missing | Not an error for purge (`ALLOW_ABSENT`); an error for migration source |
| Backup interrupted | The backup is incomplete, so verification fails, so nothing is deleted — **executed**, `test_backup_restore.py` |
| Concurrent process | The existing `flock` owner lock refuses; lifecycle operations refuse rather than wait |
| External profile path | Enumerated and backed up, never deleted automatically |

---

## 6. Install, uninstall, and the manifest

Recommended default prefix `$HOME/.local`: the dogfooding case, no `sudo`, and
where the desktop entry already goes. GNU directory variables (`prefix`,
`exec_prefix`, `bindir`, `datarootdir`, `datadir`) and `DESTDIR` staging remain
overridable.

**`DESTDIR` stages immutable installed artifacts only.** It is never prepended
to a user's config, data, state, cache, or runtime roots, and package staging
creates none of them. A package that created `~/.local/share/notrios` at build
time would ship one build machine's idea of a user directory to every user.

`install` writes `MANIFEST.json` recording every installed path with its
SHA-256, mode and size. `uninstall` removes only paths listed there, under a
validated install root, whose hash still matches — a modified or foreign-owned
artifact is preserved and reported rather than overwritten or deleted. It leaves
every user root intact.

---

## 7. Purge

`purge` is `uninstall` plus bounded removal of mutable roots. The decision
procedure is implemented in `purge_oracle.py` and exercised by 30 fixtures
against a real temporary filesystem, so symlink and containment rules are
decided by the kernel rather than by string comparison.

**Closed by default.** A target is refused unless it is positively identified as
a path under a root Notrios owns. Unknown, ambiguous, outside, unreadable,
crossing a mount, reached through a symlink, or named by a profile pointing
elsewhere — all refuse. A wrongly refused deletion costs the user a manual `rm`;
a wrongly allowed one costs them their notes.

Refusals, each with a fixture: empty, whitespace-only, relative, `/`, any system
root, the home directory, any ancestor of home, anything containing `..`, any
path outside every owned root, any external profile path *or a directory
containing one*, a symlink (even one staying inside), a symlink resolving onto
the owned root, a target on a different filesystem, and the case where no owned
roots were declared at all.

Allowed: a directory inside an owned root, an owned root itself, and a path that
is already absent — `ALLOW_ABSENT`, so a repeated purge is a no-op rather than
an error.

Containment is decided on the **resolved** path, so a symlink cannot look
contained while pointing out. That rule and the symlink rule deliberately
overlap: replacing `realpath` with textual normalization was tried, and the
fixtures caught it — two of them by the *other* rule, which is what defence in
depth looks like when it works.

The mount-boundary rule is **modelled**: the device lookup is injectable because
a test cannot mount a filesystem.

### The backup restore proof

`test_backup_restore.py` executes the sequence rather than specifying it: build
the proposed container in a sandbox, verify it, delete the source roots
**through the purge oracle**, restore offline, and compare every byte and mode
back. Six cases.

It proves the ordering guarantee that makes purge safe. A truncated archive
fails verification, and a failed verification is the only thing standing between
the user and deletion — so the test asserts that nothing was deleted while
verification failed. Omitting a whole root from the backup was tried, and the
manifest caught it (`data/data.bin is recorded but not in the archive`); that is
precisely the failure that would otherwise let purge delete data it had not
backed up.

It also checks that the backup destination is itself **refused** by the oracle,
so the container cannot land somewhere purge would later remove, and that cache
does not reappear on restore — it is disposed of, not preserved.

Backup policy by category, also fixtured: config, data and state are backed up
and the backup verified before anything is removed; cache and runtime are
disposed of; program assets go by manifest; external paths are backed up and
never deleted. An **unclassified** category defaults to `backup_and_verify` — a
new root added without a policy must not be silently disposed of.

---

## 8. Consumer to owning change

25 consumers, each mapped in `PATH_CONSUMERS.json` to one H4 change or an
explicit exception.

| Change | Consumers | Work |
| --- | --- | --- |
| H4-C1 | profile registry, generated profile configs, sync keys | One config-root resolver; delete the hand-rolled lookup |
| H4-C2 | service configuration file | Working-directory config only in source mode |
| H4-D1 | data directory, database, asset store, publication profiles | Move to `<data>`; refuse the working-directory fallback |
| H4-D2 | derived asset root, `:memory:` asset root | `filepath.Dir`; per-process temp root under `<runtime>` |
| H4-S1 | carrier, backups, catch-up inbox, quarantine, media fetch | Move to `<state>`, create `0700` |
| H4-K1 | projections, search index, Recoll config | Move to `<cache>`; rebuild, never migrate |
| H4-R1 | backup staging, restore review | Move to `<runtime>` with a `<state>` fallback |
| H4-A1 | built web interface, Help seed docs | Installed datadir ahead of the working directory |
| H4-X1 | desktop entry | Honour `XDG_DATA_HOME` |

**Two exceptions.** The ABI database owner lock stays beside the database it
guards. It is not a user-facing path; moving it to a standard root would break
the thing it exists to do, which is let a second process on the *same database*
find the same lock. And the snapshot image output root is named by the user,
which is the point of a portable snapshot.

---

## 9. Open decisions

All three were non-blocking for H3, and all three are now answered with the
recommended defaults. They become blocking for H4 and H5, which must restate
them.

1. **Installed versus portable precedence** (blocking for H4) — explicit first,
   an explicit portable marker second, native installed locations otherwise.
   Portable mode is never inferred from the working directory or its
   writability. **Modelled and observed** in `RESOLUTION_TABLE.json`.

2. **Default `make install` destination** (blocking for H5) — an unprivileged
   user-local layout under `$HOME/.local`, retaining GNU directory variables and
   `DESTDIR`. Immutable assets follow install variables; mutable data follows
   runtime resolution and is not created by package staging.

3. **Purge treatment of external profile paths** (blocking for H5) — enumerate
   and back up, refuse to delete. Fixtured in both directions: a target that
   *is* an external path and a target that *contains* one. Approval of H3
   authorizes no deletion outside Notrios's standard roots.

H5 additionally needs a **backup container and destination** decision.
Recommended: a single owner-only (`0600`) tar archive with a `MANIFEST.json` of
per-file SHA-256, written to `$XDG_STATE_HOME/notrios-purge-backups/<timestamp>/`
— outside every purge root by construction. If that destination is unsafe, lacks
capacity, or resolves through a symlink, purge refuses rather than choosing
somewhere else.

---

## 9a. Amended by H4 (2026-09-01)

`RESOLUTION_TABLE.json` was regenerated during H4 slice A. Three corrections,
recorded here rather than applied quietly, because this file is the contract H4
is bound to reproduce.

**The resolver emitted paths the purge oracle refuses.** `resolve_model.py`
joined without cleaning, so portable mode produced roots like
`/media/stick/notrios/bin/../notrios-data/cache` and the macOS bundle produced
`…/Contents/MacOS/../Resources`. `purge_oracle.decide` rejects exactly that
shape under its `normal-form` rule. Two artifacts of this same investigation
disagreed, and had H4 implemented the table literally, a portable installation
could never have been purged — `make purge` would have refused its own roots.
That is the defect class H3 was written about, reproduced inside H3's own
evidence.

Joining now cleans, in the model and in `internal/paths`. `test_resolve_model.py`
additionally asserts that **every root it resolves is accepted by the purge
oracle**, for POSIX targets in installed and portable modes, so the two cannot
drift apart again. Adding that check immediately caught a second instance: the
macOS branch bypassed the join helper entirely and kept its `..` after the first
fix. All fourteen constructions now go through one helper, so cleaning is not
something a branch can forget.

**Notices carry codes.** Comparing Python and Go notice prose fails on quoting
convention alone — `repr` uses `'`, `%q` uses `"` — while saying nothing about
behaviour. Each notice is now `{code, message}`, and the cross-implementation
test compares codes.

**Scenarios record their effective inputs.** The table omitted arguments the
model had defaulted, so another implementation would have had to guess
`executable_dir` to reproduce a row. Defaults are now bound and recorded. One
scenario was also incoherent — a POSIX `executable_dir` on a Windows target —
and now uses a Windows path.

Sixteen scenarios, up from fourteen; source mode gained coverage.

**Source mode no longer overrides the config root.** The first version did, and
wiring the profile registry through the resolver promptly relocated it into the
checkout's `config/` directory. A checkout is not a different user: the registry
and sync keys are a developer's identity across every build they run, they have
always lived in the user config root, and writing them into the source tree
would leave a file naming every local database path one `git add -A` from being
committed. Source mode now moves data, state, cache, runtime and program assets
into the checkout, and leaves config alone. `NOTRIOS_PROFILE_REGISTRY` already
exists for anyone who wants an isolated registry.

**Source mode collapses its four mutable roots onto `./data`** (H4 slice C).
The four-way split exists for user installations: XDG separates them, purge
treats them differently, and only some are backed up. A checkout has one scratch
directory. As first modelled, source mode would have moved every existing
developer's database from `./data/notes.sqlite` to `./data/data/notes.sqlite` to
satisfy a distinction that does not apply there.

**Probes retired by H4 slice D**, the web-root order having been inverted:
`TestWebRootPrefersTheWorkingDirectoryOverTheExecutable` and
`TestExplicitWebRootIsTheOnlyCandidate`. Their inverses live in
`internal/httpapi`.

**Source mode became a full instance boundary** (H4 slice D). Slice B kept the
config root native in a checkout, reasoning that a checkout is not a different
user. True, and the wrong question: a checkout is a different *instance*. A
developer who also had Notrios installed shared one config root with it, so
their real installed configuration was found first and running the checkout
build with no flags opened their production library. Every root is now
checkout-local, under `./data` rather than `./config` because the former is
gitignored and a registry names every local database path.

**Probes retired by H4 slice B**, their subjects being fixed:
`TestRelativeXDGConfigHomeIsAcceptedByProfilesAndRefusedByTheStandardLibrary`,
`TestProfileRegistryFallsBackToACurrentDirectoryRelativePath`, and
`TestDefaultConfigurationIsReadFromTheCurrentWorkingDirectory`. Each is listed
in `pathprobe/`'s retirement note naming the inverted regression test that
replaced it — in `internal/profiles` and `internal/config` respectively, beside
the code rather than in the evidence directory.

## 10. Not verified

- **No Windows or macOS execution.** The Windows and macOS rows of the matrix
  and the `parentDir` separator defect are reasoned from Go's documented
  behavior and read from source. `os.UserConfigDir` was executed on Linux only.
- **The mount-boundary rule is modelled**, not executed against a real mount.
- **Most failure models in §5 are designs**, not tests: permission,
  disk-full, mount, collision and concurrency. H4 and H5 own them. The
  interrupted-backup row is executed in `test_backup_restore.py`.
- **No migration was performed.** The state machine is specified and unexecuted;
  there is no migration code to test yet.
- **The resolver model is Python.** H4 implements it in Go and must reproduce
  `RESOLUTION_TABLE.json`; agreement between the two is not yet demonstrated,
  because one side does not exist.
