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

## J7-C: the first run did not complete

```sh
bash performance/v1.0-j7/disaster_recovery.sh <work-dir> <source-library-dir>
```

The first run used J20-C's 382,206-note Obsidian library, from a clean tree at
`8763a46`. Its baseline completed:
- the 7.35 GB library copied into the drill's data root in 48.3 s
- opened as an installed program in 0.9 s
- content digest `8d6f694a…` in 107.6 s
- search probes the=187518, quinoa=385, chicken stock=9703, matching J5 and J20-C

It was then **stopped from outside during `export archive-v2`**, by the task
runner, because the machine was running low on memory. The export had written
1.8 GB. The purge step never ran, so nothing was destroyed: the scratch copy is
intact in the drill's data root, and the source library was only read. No
kernel out-of-memory record exists. **Reported as incomplete, as J7's boundary
requires.**

**What was holding the memory.** J5's export of this corpus peaked at 348 MiB.
The export was not the cause; `/tmp` was. `/tmp` is RAM-backed tmpfs, and it
held 16 GB in 17,078 leftover `notrios-*` entries:

| entries | count | size | cause |
|---|---|---|---|
| `notrios-test-bin-*` | 171 | 6.53 GiB | `cmd/notriosctl/stablelinks_test.go` makes a shared test-binary directory and never removes it |
| `notrios-import-manifest-*` | 6 | 3.33 GiB | J19's Joplin `TestJ19InventoryMemory` never closed the inventory's import manifest (fixed here; the importer itself closes it) |
| `notrios-assets-*` | 16,888 | 1.67 GiB | `defaultAssetRoot` makes a private temp asset directory for a path-less or `:memory:` store, and `SQLiteStore.Close` never removes it |
| G18/G19/G20 Go build caches | 4 | 2.6 GiB | the evidence validators |

With no drill process running, they were removed. `/tmp` went from 16 GB used
to 664 MB, and MemAvailable rose from 38.9 GB to 44.6 GB. The asset-directory and
test-binary leaks are product and test defects outside J7's scope, and are put
to the owner.

**The drill now guards itself.** Each long step runs under a memory guard: below
4 GiB of MemAvailable, the drill stops that step and records it as incomplete
with the reading. Each step also records its peak RSS and the lowest
MemAvailable seen.

### The second run was stopped from outside too, at the purge

The re-run started from a clean tree at `7c6d36a`, with `/tmp` cleared and the
memory guard in place. It got much further:

| step | verdict | seconds | detail |
|---|---|---|---|
| copy library into the data root | pass | 50.5 | 7,350,784,000 bytes |
| open in installed mode | pass | 2.0 | |
| baseline content digest | pass | 82.9 | `8d6f694a…` |
| baseline search probes | pass | | the=187518, quinoa=385, chicken stock=9703 |
| export archive-v2 off-site | pass | 3,310.2 | 1,246,840,047 bytes; peak RSS **340 MiB**; lowest MemAvailable 43,868 MiB |
| verify the archive | pass | 296.8 | |
| purge plan confined to the drill | pass | | 6 steps, backup off-site |
| purge with a verified backup | **stopped from outside** | | while writing `backup.tar` |

**Nothing was destroyed, which shows purge's own safeguard working.** When the
purge was stopped:
- `backup.tar` held 2.66 GB of the 7.35 GB library, and no `MANIFEST.json` had
  been written
- the data root still held every file
- the library's content digest still equals the baseline, `8d6f694a…`

An unverified backup means no deletion, as J3 requires.

**The drill's memory guard did not fire, and it should not have.** MemAvailable
never fell below 43.8 GB during the export, and after the stop it was 44.7 GB,
with 1.7 GB of swap in use. Free memory was 750 MB, because 45 GB was page cache
from the multi-gigabyte reads and writes. Both stops came during such writes:
- the first run's export
- this run's purge backup of the 7.35 GB library

The task runner that supervises background jobs appears to act on free memory,
not on memory the kernel can reclaim. That is inferred from these two runs, not
confirmed. **Reported as incomplete.** How to run J7-C to completion is put to
the owner.

The export at 382,206 notes peaked at 340 MiB, in line with J5's 348 MiB. The
export was never the memory problem.

## J7-B: the REST check, with real servers

```sh
bash performance/v1.0-j7/cross_version_rest.sh <work-dir> <bin-dir> 0.7.0:c1127f1 0.8.0:10e7077 v0.9:d6f7ca2
```

For each older version, a 1.0 library and a replica made from it paired offline,
with the older version inviting. A 1.0 server then took `sync handshake` and
`sync exchange` from the older client, and the older server took both from the
1.0 client. Servers listened only on 127.0.0.1, behind the keychain guards.

| older version | exchange, 1.0 server | exchange, older server | libraries unchanged | handshake, either way |
|---|---|---|---|---|
| 0.7.0 | refused: `peer answered 405: Method Not Allowed` | refused: 405 | yes | succeeds |
| 0.8.0 | refused: 405 | refused: 405 | yes | succeeds |
| v0.9 | refused: 405 | refused: 405 | yes | succeeds |

**No data moves over REST between pre-1.0 and 1.0.** The 405 is J16's moved
carrier write path, seen from both sides.

**The handshake succeeds, and the harness was wrong to expect a refusal.**
`sync handshake` is an authenticated identity check, and it carries no note
content. The 1.0 server answers the older client with its range
(`max_compatible_schema: 28`), and the older server answers with 27. The harness
recorded these as failures because it expected every REST call to refuse. This
record states the correct reading instead of the harness's verdict.

## J7-B: upgrading in place breaks sync between existing peers, a 1.0 defect

```sh
bash performance/v1.0-j7/upgrade_in_place.sh <work-dir> <bin-dir> 0.7.0:c1127f1 0.8.0:10e7077 v0.9:d6f7ca2
```

| step | 0.7.0 | 0.8.0 | v0.9 |
|---|---|---|---|
| both replicas on the older version: sync converges | pass, 2 rounds | pass, 2 rounds | pass, 2 rounds |
| upgrade the first to 1.0 (migrates v27 → v28) | pass | pass | pass |
| mixed versions: nothing moves, nothing corrupted | pass | pass | pass |
| upgrade the second to 1.0 | pass | pass | pass |
| **both on 1.0: sync converges on the original pairing** | **fail** | **fail** | **fail** |

After both upgrades, each replica publishes its new edit: 1 envelope, 2
operations. Neither admits the other's; every round reports `refused_admission`,
and neither edit ever arrives.

**Cause, read from the code and the upgraded libraries:**
1. **Pairing pins the peer's compatibility.** `ConfigureSyncAdmissionPeer`
   stores the peer's handshake in `sync_peer_compatibility`. Pairing is the only
   caller: CLI `accept`/`enroll` and the REST pairing endpoints.
2. **The stored row outlives the upgrade.** Both upgraded libraries are at
   `user_version 28`, and each still holds its peer as `schema_version 27`,
   range 24–27.
3. **Admission compares against that row.** `AdmitSyncOperations` calls
   `validateConfiguredSyncPeerLocked`, which refuses when the peer's current
   handshake differs from the stored one: "peer compatibility differs from
   explicit configuration". The peer now reports 28 with range 24–28, so every
   batch is refused.

The explicit configuration is a deliberate guard, so a peer cannot silently
change what it claims to support. The fix is therefore a design decision, and
it is put to the owner. Until it lands, replicas upgraded in place to 1.0 do not
sync with each other again.

**Owner decision (2026-09-15): fixed in a new plan item, J23, before release.**
The rule J23 must satisfy:
- **Accepted:** a paired peer whose handshake differs from its stored
  compatibility only by a schema rise that both sides' compatible ranges admit.
  Its stored record is updated, with an audit event.
- **Still refused:** a change in protocol, capabilities or identity.

J7's upgrade-in-place drill is J23's acceptance test. Until J23 lands, replicas
upgraded in place from a pre-1.0 version do not sync with each other again.
Re-pairing the same two replicas is not a way back either:
`sync_peer_compatibility` is keyed by `replica_id`, and pairing writes it with a
plain `INSERT`. That is read from the schema and code, not run.

**Owner decision (2026-09-15): J7 stays open until J23 fixes the
upgrade-in-place defect.** J7-B's statement promises that replicas syncing on an
older version keep syncing once both are upgraded, and that is not true until
J23 lands. So J7-B is not marked done until `upgrade_in_place.sh` converges for
0.7.0, 0.8.0 and v0.9. J7-C can still complete, and be recorded, before then.

### The upgrade path, after J23

J23 (`6bf70d2`) changes admission. It accepts a paired peer whose only change is
a schema rise both sides' ranges admit, and keeps protocol, capabilities and the
range floor pinned. `upgrade_in_place.sh`, re-run against that build, now passes
every row for 0.7.0, 0.8.0 and v0.9:
- two replicas syncing on the older version are upgraded to 1.0 one at a time
- they move nothing while the versions differ
- **they converge again on their original pairing**, with both sides'
  pre-upgrade edits intact

J7-B's upgrade clause is satisfied. The results are in
`performance/v1.0-j23/README.md`.

**The REST and folder-carrier checks were not re-run against the J23 build, by
owner decision (2026-09-15).** J23 changes one thing: admission for a peer that
is already paired. It does not change how 1.0 treats a pre-1.0 peer:
- **REST:** the exchange fails with a 405 on the carrier path J16 moved, before
  any admission runs.
- **Folder carrier:** an older version refuses a 1.0 invite. When the older
  version invites, schema 28 is outside its 24–27 range.

The earlier results stand as recorded above.

### The third run completed: a 382,206-note library destroyed and recovered, twice

The drill was started detached from the task runner (`setsid`/`nohup`), by
owner decision, with its own memory guard in place. It recorded `62c11f1` with
two tracked changes, the plan ledger and this README, neither of them code.
J23's store tests ran while the export ran, so the export and verify timings
include that load. No timing here is claimed as a baseline.

| step | verdict | seconds | detail |
|---|---|---|---|
| copy library into the data root | pass | 42.4 | 7350784000 bytes |
| open in installed mode | pass | 2.0 |  |
| baseline content digest | pass | 72.6 | 8d6f694a89afde71b5d7de6bf186e756c83814795195ace9c5797700d816b47a |
| baseline search probes | pass |  | the=187518 quinoa=385 chicken stock=9703  |
| export archive-v2 off-site | pass | 3279.6 | 1246840047 bytes; peak RSS 324 MiB, lowest MemAvailable 43342 MiB |
| verify the archive | pass | 242.0 |  |
| purge plan confined to the drill | pass |  | 6 steps, backup /media/renes/HD2/notrios-j7/dr-drill-3/offsite/purge-backup |
| purge with a verified backup | pass | 318.6 | backup verified: 1 files verified; peak RSS 11 MiB, lowest MemAvailable 53715 MiB |
| library destroyed | pass |  | no notes.sqlite in the data root |
| recover from the archive (adopt) | pass | 1673.1 | peak RSS 53 MiB, lowest MemAvailable 53205 MiB |
| archive recovery content | pass | 62.5 | equals baseline 8d6f694a89afde71b5d7de6bf186e756c83814795195ace9c5797700d816b47a |
| archive recovery search probes | pass |  | the=187518 quinoa=385 chicken stock=9703  |
| extract purge's backup | pass | 24.2 | /media/renes/HD2/notrios-j7/dr-drill-3/from-purge-backup/data/notes.sqlite |
| purge-backup recovery content | pass | 68.6 | equals baseline |

**J7-C passes at J5's scale.**
- **Destroyed:** the library went through the J3 purge path with a verified
  backup, and afterwards the data root held no `notes.sqlite`.
- **Recovered from the off-site archive:** `restore archive-v2 --intent adopt`
  restored it in 1,673 s. Its content digest equals the baseline
  (`8d6f694a…`), and J5's search probes return the same counts.
- **Recovered from purge's own backup:** extracted in 24 s, with a content
  digest equal to the baseline.
- **Memory:** the export peaked at 324 MiB, the restore at 53 MiB and the
  purge at 11 MiB. MemAvailable never fell below 43 GB, so the guard never
  fired.

The drill's closing check for files named for Notrios outside its work
directory listed two: an Okular document record and a Recent Documents entry
for `NOTRIOS_IMPORT_PLAN.md`. Both came from that document being opened in
Okular. The drill never runs Okular and writes no such files, so they are
unrelated to it.

