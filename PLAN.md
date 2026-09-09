# Plan: v0.8e — Evidence backfill and reserve sealing

Status: **Planned from `ROADMAP.md` on 2026-09-09 after v0.8 closed at product
0.8.0 / schema v27. No item is started. This milestone writes no code and
changes no product behaviour; it produces artifacts and one gate.**

This plan follows `ROADMAP.md`, the preservation contract in `evidence/README.md`
and `evidence/BURN_AND_READBACK.md`, the retroactive-packaging method recorded in
`CODING_CLIENT_HANDOFF.md`, and the item-writing rules in `AGENTS.md`.

## Outcome and boundaries

v0.8 closed with twenty-seven of its thirty-five items holding no handoff ZIP.
Packaging ran per slice through H4 and then stopped, and nothing noticed for the
rest of the milestone. This backfills them and seals them into the reserve
**before the first GitHub push**.

**The ordering is the reason this exists.** The push makes the whole v0.8 body
of work public. Evidence sealed afterwards can establish when it was sealed and
nothing about whether it predates disclosure; evidence sealed before it
establishes both, because the reserve already carries a detached OpenPGP
signature by a pinned subkey and an RFC 3161 timestamp from a pinned authority.
The mechanism exists. What is missing is using it while the repository is still
private.

**What a timestamp here does and does not attest.** A bundle built now has a
file timestamp of now. The RFC 3161 response attests when the *sealing*
happened. It does not attest when the work happened, and no manifest entry,
catalogue row or document may imply that it does. Every archive built by this
milestone is retroactive and is labelled so.

Not in v0.8e:

- any GitHub push, pull request, tag or release — those open v0.9, and this
  milestone finishes first;
- any change to product code, schema, documentation content or gates other than
  the one E4 adds;
- any burn, media write or catalogue sealing without separate authorization;
- any claim that an archive was produced at the time its slice closed.

## Progress

Generated from `docs/docplan/PLAN_SLICES.json` by
`go run ./cmd/docplan --write`, and checked by `internal/docplan`, which fails
the build when the ledger, this document and the repository disagree.

**The rules for keeping it current are in [`AGENTS.md`](AGENTS.md)** — under
"Writing plan items", "Keeping the plan current" and "Plan archival" — because
this section is archived when the plan completes and the rules are not.

<!-- notrios:generated:plan:progress:begin -->
**4 items: 2 complete, 1 in progress, 1 not started, 0 deferred.**

| Item | State | Slices done | Outstanding |
|---|---|---|---|
| E1. Build the twenty-seven missing handoff archives | complete | 3/3 | — |
| E2. Record the backfill honestly, and say what it is | complete | 2/2 | — |
| E3. Seal a reserve volume, and extend the outer catalog | in-progress | 0/2 | 2 |
| E4. Make a missing archive fail rather than pass unnoticed | not-started | 0/1 | 1 |

### Started and not finished

**E3. Seal a reserve volume, and extend the outer catalog**

- `E3-A` A new volume seals the backfilled archives, signed by the production subkey and RFC 3161 timestamped — *blocked* (blocked on: the signing passphrase is unavailable in this session, and seal-content cannot seal a second volume; see performance/v0.8e/VOLUME_PLAN.json)
- `E3-B` The outer catalog is extended and the reserve verifier passes end to end — *blocked* (blocked on: the signing passphrase is unavailable in this session, and seal-content cannot seal a second volume; see performance/v0.8e/VOLUME_PLAN.json)

### Not started

Written and not begun: E4. Their slices are listed under each item.
<!-- notrios:generated:plan:progress:end -->

## E1. Build the twenty-seven missing handoff archives — complete

**Goal.** Every completed or deferred v0.8 item has a source archive taken from
the commit that closed it, in `/home/renes/evidence/notrios`.

**Scope.** The reserve holds bundles for H0, H1, H2a, H2, H2b, H3, H4 and H13.
Missing: **H4a, H4b, H5, H6a, H6, H7, H8, H9, H10, H11, H12, H14, H15, H16, H17,
H18, H19, H20, H21, H22, H23, H24, H25, H26, H27, H28, H29** — twenty-seven.

Each is built from its own close-out commit in a disposable `git worktree`, the
method `CODING_CLIENT_HANDOFF.md` records for the retroactive H2a/H2b/H2
archives, so the live checkout is never touched and each archive reflects the
repository as it stood when that slice finished.

**Verify the tree, not today's gates.** `scripts/package_release.sh` re-runs the
full gate set, and a historical commit will fail gates written after it: pinned
counts that have since moved, an `npm audit` advisory published later, a
document count that grew from fifteen to eighteen. What a retroactive archive
has to prove is that it **is** that commit — every tracked file byte-identical
to the commit's tree, and `scripts/check_release_zip.py` clean. Those gates
passed when the slice closed; re-running today's against yesterday's tree would
measure the wrong thing and would fail for reasons that say nothing about the
archive.

**Use today's packaging script even for old commits.** It excludes the local
`.zvec-grep/` index, which is 95 MB and which every archive would otherwise
carry; that exclusion was added in v0.8 H13 and does not exist in the historical
trees being packaged.

**Boundaries.** No network publication, no reserve write, no ISO. Archives land
in `dist/` and in the evidence directory and nowhere else.

**Dependencies.** None. v0.8 is closed and its commits are immutable.

**Working state.** Twenty-seven archives exist, each verified byte-identical to
its commit's tree and to its copy in the evidence directory, with no archive
containing a binary, a database, `node_modules`, or the local index.

**Open decisions**

- **Which commit closes a slice — Taken as the default, 2026-09-09.** The
  newest commit whose *subject* names the item. A body mention is not a
  close-out: one commit names H6a, H7, H8 and H12 in its body while closing
  none of them. Two items could not be reached by pattern and were resolved by
  reading rather than by widening a regex until it matched something -- H5's
  subject says "(v0.8 H5 slices C and D)", a slice-list form, and H12 was
  deferred rather than implemented, so its close-out is the commit that recorded
  the deferral.

**Outcome (2026-09-09).** Done, and the scope grew twice on evidence.
**Thirty-three archives** were built, not twenty-seven, and every v0.8 item now
has one that passes today's `check_release_zip.py` and is byte-identical to a
real commit. 0.54 GB, all thirty-three distinct, all in the evidence directory.

**The mapping was checked, and the check found three.** Twenty-five of the
twenty-seven resolved from commit subjects; the plan at three of those commits
did not yet carry the item's completion marker. That is not a wrong mapping --
H4a, H4b and H5 were all marked complete later, together, in one bookkeeping
commit -- so the work commit stands as the close-out and `CLOSEOUTS.json`
records why. The check was worth having: it would have caught a genuinely wrong
mapping the same way.

**Five archives already in the reserve carry the notrioslib binary.** H1, H2a,
H2b, H2 and H3 hold the 11 MB ELF that the 2026-09-01 history rewrite removed
from git, and today's `check_release_zip.py` refuses all five. Their trees are
faithful -- each matches its post-rewrite commit through the handoff's mapping
table, which this run tested end to end and found correct -- but sealing a
rejected archive into an immutable reserve would preserve the mistake for ever.
Rebuilt. H1 went from 15.3 MB to 8.0 MB, which is the binary leaving.

**And one archive matched no commit at all.** `notrios-v0.8-h13-030ef2d.zip`,
built during H13 itself, was made from the working tree while `030ef2d` was HEAD
and H13's own changes were uncommitted -- so its filename asserts a commit whose
tree it does not carry, and the verifier said so on the first run against it.
Rebuilt from `10e7077`, H13's real close-out. The lesson is small and general:
name an archive after a commit only when it was taken from one.

**Nothing was deleted.** The six superseded files are still in the evidence
directory, each named by the entry that replaces it. Removing evidence is the
owner's decision and not this run's, and it should be made before the reserve is
sealed rather than after.

**Two things the build does that are worth keeping.** It installs web
dependencies once per *distinct lockfile* rather than once per commit -- the
thirty-three carry three between them -- and it installs from the commit's own
lockfile rather than today's, because the archive has to carry the frontend that
commit would have built. And `verify_archive.py` knows the packager's exclusion
list, so a tracked file the packager is supposed to drop is reported as excluded
rather than as missing: two v0.8 commits carry a tracked vitest cache file under
`node_modules`, and calling that a defect would have been the tool misreading
correct behaviour.

## E2. Record the backfill honestly, and say what it is — complete

**Goal.** A manifest that a reader can check, and a handoff document that covers
every slice rather than the first four.

**Scope.** One committed manifest naming, for each archive: the item, the
close-out commit, byte count, entry count, SHA-256, and whether it was built at
the time the slice closed or retroactively in this milestone. Extend the
snapshot table in `CODING_CLIENT_HANDOFF.md` from four rows to all thirty-five.

**Boundaries.** The manifest records what was built. It does not claim the
archives were produced when their slices closed, and it does not stand in for
the reserve's signature and timestamp, which are what make a claim checkable by
somebody else.

**Dependencies.** E1.

**Working state.** Every v0.8 item appears in the manifest with its archive's
hash, and every retroactive archive is labelled retroactive.

**Outcome (2026-09-09).** Done. `performance/v0.8e/MANIFEST.json` covers all
thirty-five items -- archive, full commit, bytes, entries, SHA-256, and when it
was taken -- and the handoff's snapshot table went from four rows to
thirty-five.

**Both are generated, and the generation is checked.** `build_manifest.py` reads
every field from the archive it describes or from git, so the manifest cannot
disagree with the evidence directory unless the directory changed. The handoff
table is rendered from the manifest, and `validate_evidence.py` re-renders it in
`make validate` and fails when the document and the record diverge. That gate is
the point rather than a nicety: the four rows it replaces stood unchanged while
thirty-one slices closed without an archive, because nothing compared them to
anything.

**The check runs without the archives, deliberately.** The evidence directory is
not in the repository and not on every machine, so what `make validate` can
check is the record: internal coherence, no item twice, no two items claiming
the same bytes, every count agreeing with the rows beneath it, and every
superseding entry saying why. Verifying the bytes needs the archives and the
reserve verifier, and the validator does not pretend otherwise.

**One check is about a sentence rather than a number.** The manifest has to keep
saying what a retroactive archive's timestamp does *not* attest, and the
validator fails if that sentence goes. Every other field could stay correct
while the record quietly began reading as though the archives were
contemporary, and the whole reason this milestone runs before the push is that
the distinction matters.

**Two rows were verified to be honest in the other direction too.** Only H0 and
H4 are marked "at close"; every other row says v0.8e, and six say they replace
an earlier archive and name it. Nothing in the table claims an archive is older
than it is.

## E3. Seal a reserve volume, and extend the outer catalog

**Goal.** The backfilled archives are inside the immutable reserve, signed and
timestamped, before anything is pushed.

**Scope.** A new volume at `/media/renes/SEAGATE2TB/notrios-evidence`, extending
the outer catalog chain from `volume-0001`: ISO built, hashed, signed by the
exact production subkey, timestamped against the pinned authority, catalogue
appended, and the whole thing verified with
`python3 evidence/verify_evidence.py reserve --reserve-root …`, which extracts
without mounting, walks every file, and requires exact OpenPGP and RFC 3161
identities rather than ambient trust.

**Boundaries.** **This item is blocked on separate authorization and does not
begin without it.** No burn, media write, signing or catalogue sealing happens
otherwise. The reserve is append-only: `volume-0001` is not rewritten.

**Dependencies.** E1, E2, and the owner's authorization.

**Working state.** A sealed volume covering the backfilled archives, the outer
catalog extended, the reserve verifier passing end to end, and the closure
boundary the reserve already documents recorded rather than papered over — a
volume cannot contain its own final hash or the commit that records it.

**Open decisions**

- **One volume or one per slice — Taken as the default, 2026-09-09.** One
  volume for the backfill. The reserve's unit is a sealing event rather than a
  slice, and twenty-seven volumes would multiply the signing and timestamping
  ceremony without making any archive more verifiable.
- **Whether the six superseded archives are sealed — Blocking, unanswered.**
  They are still in the evidence directory and the plan below counts them.
  Sealing them preserves what was found; removing them first leaves a reserve in
  which every archive passes today's gate. Recommended: remove them, because an
  immutable copy of an archive that today's release check refuses is a thing
  somebody will later have to explain. Either way the decision belongs before
  the seal.

**Progress (2026-09-09). Prepared, not sealed. Two things stop it, and neither
is a matter of effort.**

*The reserve was verified first, and that part is done.* `volume-0001` extracts,
walks, and checks against its OpenPGP signature and RFC 3161 timestamp with the
exact pinned identities: `{"status": "verified", "volumes": ["NTR-EV-0001"]}`,
catalog `b47f9a7d1879459e`. The chain this milestone would append to is sound,
which is a precondition and is now established rather than assumed.

*The plan for what a second volume carries is computed and recorded.*
`VOLUME_PLAN.json` names the **46 files, 0.71 GB** in the evidence directory that
`volume-0001` does not hold -- 35 v0.8 archives, the 6 they supersede, and 5
others -- each with its size and SHA-256. It is a plan and not a seal: nothing
in it is signed, timestamped, or written to the reserve.

**Stopped at the signing passphrase.** `secret-tool lookup service gpg_evidence
type passphrase` exits 1 in this session, so no signature and no timestamp can
be produced. The signing subkey `2C6A8A4568264005` is present and the primary is
offline, which is the arrangement the reserve documents; what is missing is the
passphrase the sealing tool reads from the Secret Service.

**And `seal-content` cannot seal a second volume.** It requires *exactly* the 81
approved G17b artifacts and the G17a base commitment, and its volume id,
checkpoint id and build paths are `0001` constants. Sealing `volume-0002` means
generalising a tool that handles the production key and an append-only signed
chain -- and that should not be written blind. Without the passphrase its
signing path cannot be exercised even once, and untested signing code committed
against a production key is worse than no code.

So this item stays open deliberately. What it needs is one session with the
keyring unlocked, the superseded-archive decision made, and the generalisation
written where its signing path can be run.

## E4. Make a missing archive fail rather than pass unnoticed

**Goal.** The next milestone cannot lose twenty-seven archives quietly.

**Scope.** A check that runs offline in `make validate`: every plan item the
ledger records as complete or deferred has an entry in E2's manifest. It reads
the committed manifest rather than the evidence directory, because the evidence
directory is not in the repository and not on every machine — so the gate has to
be about the record, and the record is what the reserve seals.

**Why a gate rather than a note.** Packaging stopped after H4 and the omission
survived thirty-one items and two milestones' worth of attention. A convention
nobody checks is a convention that lapses; this repository's answer to that is a
test, and this is the same failure the documentation machinery exists to prevent
appearing in the evidence machinery.

**Boundaries.** The gate checks coverage, not contents. It cannot verify a hash
it has no file for, and it must not pretend to: what verifies bytes is the
reserve verifier, with a device attached.

**Dependencies.** E2.

**Working state.** Removing a manifest entry for a completed item fails
`make validate`, checked by removing one on purpose.

## Decisions register

This is an index only; each decision is owned and explained inside its item.

| Decision | Owner | Status |
|---|---|---|
| Which commit closes a slice | E1 | Non-blocking default: the last commit naming the item; an ambiguous one is reported rather than guessed |
| One reserve volume or one per slice | E3 | Non-blocking default: one volume for the backfill |
| Reserve sealing authorization | E3 | **Blocking.** The owner authorizes the burn, signature and catalogue append; this plan schedules it and does not grant it |
