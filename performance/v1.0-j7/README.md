# v1.0 J7: backup, export, restore and sync across versions, and disaster recovery

## The versions, by owner decision (2026-09-15)

The repository has no release tags, so the set comes from the project's own
record. Every historical commit below builds with today's toolchain.

| version | commit | schema |
|---|---|---|
| 0.7.0 | `c1127f1`, G20 release acceptance | 27 |
| 0.8.0 | `10e7077`, H13 closes v0.8 at 0.8.0 | 27 |
| v0.9 close | `d6f7ca2`, I9 (still labelled 0.8.0) | 27 |
| historical archive writer | `5ae93df`, named by the archive contract's `previous-loose-v2` reader | 12 |
| 1.0 under test | `525485c` | 28 |

**Rules, by owner decision:**
- A pre-1.0 version given 1.0 data passes if it **refuses clearly, or restores
  completely**. Partial or corrupt data fails. The rule was first framed as
  "refuse". It was revised after J7-A measured complete restores; see below.
- **Sync REST publishing between pre-1.0 and 1.0 is a documented break**, not
  bridged (J16 moved the carrier write path). Folder-carrier sync is the
  supported path, and J7-B tests it.
- **0.7.0 opening a 1.0 library is a documented 0.7.0 limitation** (below).

## J7-A: archives and library files

```sh
python3 performance/v1.0-j7/content_digest.py a.sqlite b.sqlite
bash performance/v1.0-j7/cross_version.sh <work-dir> <bin-dir> 0.7.0:c1127f1 0.8.0:10e7077 v0.9:d6f7ca2
```

Every version imports the same generated vault: 300 notes, 1,055 links, 20
attachments with 170 references, and notebooks in nested folders
(`performance/v1.0-j17/j17_make_vault.py`).

Libraries written by different versions are compared with
`content_digest.py`, which hashes what the notes consist of in columns v27 and
v28 share:
- documents and their current bodies
- notebooks and tags
- links and resources
- provenance (sources)

It leaves out random identifiers and timestamps. It was validated in both
directions before use: two libraries J20 proved identical digest the same, and
a copy with one body changed differs in the `bodies` section alone.

**Every version's import of the vault has the same content as 1.0's:**
`d849eb5e65f2c7a9d10fb4c40d40e2f6347eb9334cf4eb066385b669edc249ab`.

| check | 0.7.0 | 0.8.0 | v0.9 |
|---|---|---|---|
| 1.0 verifies the version's archive | pass | pass | pass |
| 1.0 restores the version's archive (content equals 1.0's import) | pass | pass | pass |
| 1.0 opens the version's library file (migrates v27 → v28, content equal) | pass | pass | pass |
| the version restores a 1.0 archive | pass: restored completely | pass: restored completely | pass: restored completely |
| the version opens a copy of a 1.0 library | **documented failure** | pass: refused, copy unchanged | pass: refused, copy unchanged |

### Older versions restore 1.0 archives completely, and that passes

The first framing of the rule required a refusal, and all three versions
restored instead. Each restored library's content digest is identical to the
1.0 library the archive came from, and 0.7.0's restore reports:
- 300 documents and 300 revisions
- 1,055 links
- 20 resources and 170 references

The archive declares `source_schema_version 28`, but its required capabilities
are the five every one of these readers supports. Schema 28's only change is
J18's full-text rowid mapping, a private index an archive does not carry. So
nothing in the archive is beyond these readers. The owner revised the rule to
"refuse clearly, or restore completely", and the harness checks the content of
any successful restore.

### 0.7.0 opens a 1.0 library, and rewrites its version

0.7.0 predates the "refuse a database from the future" check that 0.8.0 added.
Given a copy of a 1.0 library, it opened it without refusing and ran its own
migration steps. That rewrote `user_version` from 28 to 27. The consequences:
- **Content was not harmed.** The digest is unchanged.
- **1.0 recovers the library.** Opening that copy with 1.0 again migrated it
  back from 27 to 28, with content identical to the original.
- **Search is intact.** A search for "paragraph" finds 300 notes, matching the
  vault and the untouched 1.0 library.

0.8.0 and v0.9 refuse, leaving the copy unchanged. 1.0 cannot change how 0.7.0
behaves, so this is recorded as a 0.7.0 limitation. The release documentation
must say not to open a 1.0 library with 0.7.0, and that opening it with 1.0
afterwards recovers it.

### The historical archive writer

`5ae93df` (schema 12) imported the same vault and exported an archive-v2.
1.0's `compatibility` command admits it under `current-v2`, `verify` passes,
and `restore` succeeds. Every note survives, identical to 1.0's own import of
the vault:
- documents and bodies
- links and resources
- sources

The only difference from `5ae93df`'s own library is two notebooks,
`nb_recovered` ("Recovered") and `nb_reports` ("Reports"). Both are 1.0's
built-ins, which 1.0 creates in every library. They are not content.

### The matrix, re-run from the committed harness

`cross_version.sh` was corrected to the revised rule, committed (`a551b6d`), and
re-run into a new work directory. It recorded one tracked change: the edit to
J7's decision text in `PLAN.md`, which is documentation. The verdicts are
identical to the table above:
- every forward check passes
- every backward archive restore passes by restoring completely
- 0.8.0 and v0.9 refuse a 1.0 library
- 0.7.0 fails only the documented library-open cell

## J7-B preparation: sync keys never reach the user's keychain

Binaries run from outside a source checkout count as installed. An installed
0.8.0, v0.9 or 1.0 with no key material defaults its sync keys to the operating
system's keychain, reached through the D-Bus Secret Service. A cross-version
drill on the owner's machine must not put test keys there. So every sync command
in J7-B runs with two independent guards:

1. **A scratch config** with `sync.rest.credential_store: development-file`,
   plus explicit `--db` and `--keys` inside the drill's tree. An explicit
   setting always wins in `config.ResolveCredentialStore`. The resolver is
   byte-identical at 0.8.0, v0.9 and 1.0, and 0.7.0 has no native store.
2. **No session bus.** `DBUS_SESSION_BUS_ADDRESS` is unset, and
   `XDG_RUNTIME_DIR`, `HOME` and every `XDG_*` root point into the drill's
   tree. A mistaken native selection would fail loudly, not write.

A probe ran `sync init` that way on copies of J7-A's libraries, with 1.0, 0.8.0,
v0.9 and 0.7.0. Every binary reported
`"credential_store": "locked-file-development"` and wrote its key only to the
scratch `keys/sync.key`. Nothing named for Notrios appeared under the user's
`~/.config` or `~/.local/share`.

## J7-B: sync across versions is refused in both directions

```sh
bash performance/v1.0-j7/cross_version_sync.sh <work-dir> <bin-dir> 0.7.0:c1127f1 0.8.0:10e7077 v0.9:d6f7ca2
```

For each older version, and in both directions:
- the first replica imports a generated 60-note vault
- the second is made from it by `export archive-v2` and
  `restore --intent adopt`
- they pair offline
- each edits a different note
- `sync once` runs through a shared folder

Every command runs behind the keychain guards above. The first run stopped
before any sync round because of two harness bugs, both fixed in `9eda287`:
- `notes edit` was passed `--keys`
- the second note's path was wrong

The results below are from the re-run.

| older version | 1.0 invites, older version accepts | older version invites, 1.0 accepts | then sync through the folder |
|---|---|---|---|
| 0.7.0 | refused | pass | nothing moves |
| 0.8.0 | refused | pass | nothing moves |
| v0.9 | refused | pass | nothing moves |

**Every pre-1.0 version caps sync at schema 27.** The range is 24–27 at
`c1127f1`, `10e7077` and `d6f7ca2`. J18 (`01367b5`) raised 1.0's range to
24–28, so the versions refuse each other:
- **An older version refuses a 1.0 invite:** `sync schema mismatch: local
  schema 27 remote schema 28 range 24-27`.
- **When the older version invites, pairing completes, and then sync moves
  nothing.** Through four rounds each side publishes and admits nothing, and
  reports `Skipped: {"incompatible_peer": 1, "refused_admission": 1}`. Neither
  edit reaches the other replica, and neither library is changed by the other.

**This withdraws the premise that pre-1.0 peers sync with 1.0 through a folder
carrier.** No sync path exists between a pre-1.0 version and 1.0, over a folder
or REST. The refusal is clean: no command fails, and no partial data is written.

**Owner decision (2026-09-15):**
- J7-B's pass is this clean refusal in both directions.
- The supported path is to upgrade every replica to 1.0. The release
  documentation (J9) says so.
- `upgrade_in_place.sh` tests that path. Two replicas syncing on an older
  version are upgraded one at a time, and must keep syncing on their original
  pairing.
