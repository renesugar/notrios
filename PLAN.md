# Plan: v0.9 — Release-candidate hardening

Status: **Planned from `ROADMAP.md` on 2026-09-09 after v0.8e closed. No item is
started. Product version is 0.8.0 and the canonical database schema is v27.**

This plan follows `ROADMAP.md`, the evidence contract in `evidence/README.md`,
and the item-writing rules in `AGENTS.md`. Every item is independently
approval-gated.

## Outcome and boundaries

v0.9 turns a working local product into a release candidate. It opens by making
v0.8 public, which is the first external write in the project's history, and
then hardens the thing that will actually ship.

**The first two items are ordered, and the order is the point.** The push comes
first because the workflows this milestone must pin and exercise have never run
against any of this work, and a release window is the worst place to discover
that. The Wails v3 migration comes before the matrices, the SBOM, the soak tests
and the freezes, because what gets hardened has to be what ships: everything
after it describes the desktop it produces.

**v0.8e sealed the evidence first, and that is now done.** Three reserve volumes
hold every v0.8 and v0.8e handoff archive, signed by the pinned subkey and RFC
3161 timestamped, before anything becomes public. Nothing in v0.9 may imply that
a timestamp taken after the push says anything about what predates it.

Not in v0.9:

- an end-user GitHub release, or a v1.0 tag;
- Windows or macOS build promotion — deferred to post-v1.0 with the hardware
  they need, and absent from release claims rather than shipped unexecuted;
- hardening a release candidate on a Wails beta -- v3 was `v3.0.0-beta.19` on
  2026-09-09, so I2 is deferred to post-v1.0 and this milestone hardens the
  Wails v2 desktop;
- any secret in a pull-request job, log, artifact, backup or the repository.

## Progress

Generated from `docs/docplan/PLAN_SLICES.json` by
`go run ./cmd/docplan --write`, and checked by `internal/docplan`, which fails
the build when the ledger, this document and the repository disagree.

**The rules for keeping it current are in [`AGENTS.md`](AGENTS.md)** — under
"Writing plan items", "Keeping the plan current" and "Plan archival" — because
this section is archived when the plan completes and the rules are not.

<!-- notrios:generated:plan:progress:begin -->
**10 items: 8 complete, 0 in progress, 1 not started, 1 deferred.**

| Item | State | Slices done | Outstanding |
|---|---|---|---|
| I1. Put v0.8 on GitHub, and reconcile the branches | complete | 3/3 | — |
| I2. Migrate the desktop shell to Wails v3, or record the postponement | deferred | 0/3 | 3 |
| I3. Promote the Ubuntu installer through clean native environments | complete | 3/3 | — |
| I4. Harden the destructive lifecycle, and decide the profile race | complete | 4/4 | — |
| I5. Resolve signing, notarization and timestamping policy | complete | 3/3 | — |
| I6. Generate and verify the release evidence set | complete | 4/4 | — |
| I7. Soak, recover, and freeze the support matrix | complete | 3/3 | — |
| I8. Freeze the 1.0 compatibility surfaces | complete | 3/3 | — |
| I9. Write the release-grade operational documentation | not-started | 0/3 | 3 |
| I10. Serve the documentation site from notrios.com | complete | 2/2 | — |

Nothing is half-finished.
<!-- notrios:generated:plan:progress:end -->

## I1. Put v0.8 on GitHub, and reconcile the branches — complete

**Goal.** The v0.8 body of work is public, on both branches, with no content
difference between them.

**Scope.** Re-audit the remote, run the evidence pre-push gate, push `develop`,
open a `develop`-to-`main` pull request, and after review and explicit
authorization merge it **with a merge commit**. Then bring the merge result back
into `develop`, so `main` is an ancestor of it.

**Why the back-merge, which looks unnecessary.** `main` carries nothing but
merge commits from `develop`, so the pull request has no content to resolve. But
a merge commit created on `main` is a commit `develop` does not have, and
without merging it back `main` goes on showing as ahead. **Zero content
difference is the requirement; identical commit identifiers are not**, and
branches are never forced to reach them.

**Why not squash or rebase.** Either would rewrite `develop`'s history onto
`main` and leave the two branches holding different identifiers for identical
work — which is the condition this item exists to end, not to create.

**Boundaries.** **The push and the merge each need separate explicit
authorization.** No tag, no release, no upload of build artifacts. Nothing is
force-pushed. The evidence is already sealed and is not re-sealed here.

**Dependencies.** v0.8e, complete.

**Working state.** `develop` and `main` on GitHub with no content difference,
`main` an ancestor of `develop`, and the pre-push evidence gate recorded as run.

**`develop` pushed 2026-09-09, on the owner's authorization.**
`26b0925..c63c4f8`, a fast-forward of 314 commits: 1,663 files and about 564,000
insertions covering all of v0.8, v0.8e and the start of v0.9. Nothing was forced.
The repository was already public, so this is the moment the v0.8 body of work
became so -- which is why v0.8e sealed three signed, RFC 3161 timestamped volumes
first, all of them predating this push.

**The gate refused before it passed, and that was the work.** Both faults came
from the same place: it was written when the reserve was one volume and the
evidence directory was exactly what that volume sealed.

`verify_source` required the live source to *equal* volume-0001's checkpoint. A
growing reserve cannot satisfy that -- volumes 0002 and 0003 sealed forty-four
more artifacts, and a milestone always holds archives built after its last
volume, because an archive cannot be inside the volume whose sealing commit
produced it. It now checks the property that survives: across every sealed
checkpoint, each artifact present, byte-identical, structurally valid and
covered by the signature its own checkpoint recorded -- 125 artifacts rather
than 81. Unsealed files are counted and named rather than refused, because
refusing them would demand the reserve be sealed before the work that produces
the next thing to seal. Altering a sealed artifact, deleting one and replacing
one with a symlink were each tried against a hardlinked copy of the real source,
and each refused.

The second fault was quieter. The content-commit check read the catalog with
`json.load`, which worked on one entry and raised `Extra data` the moment
volume-0002 appended a second line -- **a gate that stopped running rather than
started failing**, which is the worse of the two. It reads every entry now and
requires each volume's content commit to be an ancestor of `HEAD`.

**Pre-push audit, recorded because a public push is not reversible.** No private
key material in the tree -- the `.asc` and `.pem` files are the public key and
the TSA certificates the verifier needs. No credential-shaped strings outside
test fixtures and variable names. `agent/ATTEMPT_LOG.jsonl` was already on the
remote and holds task and status rows, not transcripts.

**Pull request [#6](https://github.com/renesugar/notrios/pull/6) opened
2026-09-09, and CI went red on three independent faults.** Every one predated
this milestone, and every one was invisible on a developer workstation — which
is the entire argument for putting the push before the hardening rather than
inside it.

*`doctor` required a keyring it was not using.* An unreachable native credential
store was a required failure whenever the store resolved to native, regardless
of whether anything was in it, so a temporary library holding no credentials at
all reported FAILED. That made every headless machine — server, container, CI
runner — unable to pass notrios's own health check, and it passed on every
desktop because a desktop session has a Secret Service. On the owner's decision,
an empty store is now informational and says a keyring will be needed to store
keys here; **sealed key material plus an unreachable store stays a required
failure**, because those keys exist and cannot be read, and nothing is ever
substituted for the store. `sync migrate-credentials --dry-run` had the same
shape: it refused to describe a migration whose destination was unreachable,
though its documented job — which store holds the keys now, which would hold
them afterwards — needs nothing opened. It reports the plan and the obstacle
now, and still exits non-zero.

*A frontend test had never tested anything.* `draft.test.ts` asserted that
`saveDraft` reports failure when storage refuses, by spying on the storage
object. Under Node 22 `localStorage` is jsdom's `Storage`, a Proxy whose
defineProperty trap *stores items*: assigning `setItem` writes an entry called
"setItem" and leaves the real method in place, so the spy was never called and
the write succeeded. Under Node 26 it is Node's own `MemoryStorage`, not a jsdom
`Storage` at all, so a prototype spy patches a prototype nothing inherits from.
An instance spy passes on 26 and no-ops on 22; a prototype spy does the reverse.
Swapping the global binding works on both, because `saveDraft` reads the global
at call time.

*The `go` job ran `validate-scaffold.sh` without the web workspace*, so the
frontend audit died on `MODULE_NOT_FOUND`. It only became visible once the
credential-store failure ahead of it was fixed, which is what a pipeline nobody
has run looks like: one fault at a time, each hidden behind the last.

**Node is pinned once, in `.nvmrc`, read by every workflow** — the owner's
suggestion. CI was on 22 while this machine and `docs.yml` were both on 26.3.0;
the workflows did not agree with each other, and that skew is what let a test
that never tested anything survive four milestones.

All four jobs pass: `go`, `web`, `gui-build`, `smoke`.

**Merged 2026-09-09 on the owner's authorization, with a merge commit**:
`3799c6f Merge pull request #6 from renesugar/develop`. Squash and rebase were
both ruled out in advance, because either would rewrite `develop`'s history onto
`main` and leave the two branches holding different identifiers for identical
work — the condition this item exists to end.

**The back-merge turned out to need no merge at all.** `main` now contained
every commit `develop` had plus the merge commit, so bringing it back was a
fast-forward: `develop` moved to `3799c6f` without rewriting anything. Both
branches are that commit. `git rev-list --left-right --count` reports `0 0`,
`git diff` between them is empty, and `main` is an ancestor of `develop` because
they are the same commit. Zero content difference was the requirement; identical
identifiers were not required and were reached anyway, without forcing either
branch.

**A flake surfaced between the green run and the merge, and was fixed rather
than merged past.** The same commit passed one CI run and failed the next with
`window is not defined` thrown from a timer callback. `reportUnsavedChanges`
polls for several seconds waiting for Wails to inject `window.go`; in a browser
the binding never appears, so a document torn down inside that window leaves a
tick with no `window` to read, and the throw comes from a timer where nothing is
waiting to catch it. Both halves needed fixing, because the tick has to stop as
well as survive: stopping used to call `window.clearInterval`, the one call that
cannot work when the window is what went away. Two tests cover it and both were
confirmed to fail with the exact `ReferenceError` before the guards went in.

## I2. Migrate the desktop shell to Wails v3, or record the postponement — deferred

**Goal.** The desktop that gets hardened is the desktop that ships.

**Deferred to post-v1.0 on 2026-09-09, by the condition this item set for
itself.** The gate was a released Wails v3.
`go list -m github.com/wailsapp/wails/v3@latest` answers `v3.0.0-beta.19`, so
there is no release to migrate to and a release candidate is not hardened on a
beta. v0.9 hardens the Wails v2 desktop; v1.0 ships it; the migration is the
post-v1.0 entry in `ROADMAP.md`.

**This is the condition being met, not the plan failing.** H10's spike passed on
beta.18 and found the port small -- eight linked Go modules against the v2
shell's sixteen, `window.go` absent in v3 so eleven frontend call sites move onto
`@wailsio/runtime` or generated bindings, and one real hazard in the v3 question
dialog answering on a callback rather than returning the button, so the
unsaved-work veto must wait and fail closed. None of those findings expires while
a release is waited for, which is why waiting costs nothing.

**What the postponement costs, stated so it is not lost.** The migration no
longer rides on v0.9's hardening and must bring its own: the installer's
dependency inventory, the SBOM and licence reports, the soak tests, the desktop
support matrix and the installation documentation all get repeated for the
migrated desktop. A migrated desktop inheriting a v2 desktop's evidence would be
claiming something nobody measured. `ROADMAP.md` records this under the
post-v1.0 entry.

**What it changes for the rest of this milestone.** I3 through I8 no longer wait
on a framework swap and no longer describe a desktop that might be replaced
underneath them. The native stack stays GTK3 and webkit2gtk-4.1, which is what
ships.

**Scope, unchanged and carried forward.** Move the shell from Wails v2 to v3:
the eleven `window.go` call sites onto `@wailsio/runtime` or generated bindings,
the veto proven to fail closed on the callback answer, and the native stack from
GTK3/webkit2gtk-4.1 to GTK4/webkitgtk-6.0.

**Dependencies.** A released Wails v3, which does not exist as of 2026-09-09.

**Working state.** The decision recorded here and in `ROADMAP.md`, v2 untouched,
and nothing downstream in this milestone waiting on it.

## I3. Promote the Ubuntu installer through clean native environments — complete

**Goal.** The installer is exercised where nothing of ours has run before.

**Scope.** Clean native environment matrices rehearsing fresh install,
source-layout migration, upgrade across prereleases, downgrade refusal and
rollback, remove and reinstall, profile discovery, and operation with neither
source nor a development toolchain present.

**Boundaries.** Ubuntu only. Windows and macOS have no feasible candidate to
promote here and stay absent from release claims rather than shipping
unexecuted build output.

**Dependencies.** I1. I2 is deferred, so this hardens the Wails v2 desktop --
which is what ships.

**Working state.** Each rehearsal executed in a clean environment with its
result recorded, including the ones that must refuse.

**Done, 2026-09-09. The package is installed and run for the first time.**
H6a built it and read it — level 2 on its own claim ladder, "structurally
inspected" — and listed whether it installs or runs among the things it had not
verified. Eight items went by without anyone installing it. Seven scenarios now
do, each in its own clean Ubuntu 24.04 container with no source tree, no Go, no
Node and no compiler, running the application as an unprivileged user. That
combination is what makes the answer mean anything: the machine that builds a
package hides every missing dependency, and running as root hides every
permission mistake.

**The pair of packages is built honestly.** The prerelease comes from a
disposable worktree with `version.go` patched, so the binary reports the version
its control file declares. Overriding only the packaged version would have been
one line and a lie — the rehearsal is about upgrading *between versions*, and
the interesting assertion is that the installed binary changed. `0.8.0~rc1`
rather than `0.8.0-rc1`, because in Debian ordering `~` sorts before the
release; getting that backwards would have rehearsed the opposite of the claim.

**What the runs establish.** Dependencies resolve on a pristine image. `doctor`
exits 0 with no keyring — the behaviour I1 changed, now asserted somewhere
genuinely headless rather than on a workstation that has a Secret Service.
Notes can be created, found and **exported** where nothing can be compiled.
An upgrade replaces the binary and keeps the library. A downgrade is refused
unasked (apt exits 100) and performed when asked, intact either way. Removing
the package leaves the user's notes alone and reinstalling finds them — a claim
the documentation made loudly and nothing had tested against the packaged form.
A named profile is found again. A pre-0.8 `./data` library migrates, and its
dry run moves nothing.

**Two of my own mistakes are kept in the record because they are the
instructive part.** The first draft of the profile check forbade `/usr`
outright, which is wrong in one direction and right in the other: the packaged
frontend belongs in `/usr/share/notrios`, read-only and replaced on upgrade,
while every root the user writes to must stay out of it. It asserts both halves
now. And the first draft of the no-toolchain scenario ran the export with
invented flags, swallowed the failure with `|| true`, and recorded
`export_written=no` as though that were a result — which is exactly how a real
fault would have hidden. A scenario that reports its own failure as an
observation is worse than no scenario, because it looks like coverage.

**What it does not establish, gated so it cannot quietly shrink.** arm64 is
untouched and stays at level 2 exactly as H6a left it. Only 24.04 was tested;
the t64 library transition makes 22.04 a separate question. The GUI was never
launched — these are headless containers with no display, so the desktop stays
at level 2. And **a container is not a machine**: it shares the host kernel and
has no init, no systemd user session, no D-Bus and no keyring, which is why the
credential path these runs exercise is the headless one. Fault injection belongs
to I4. `validate_evidence.py` fails if that list shrinks, and dropping the arm64
limit, dropping a scenario, claiming a warmed image for the fresh install, and
passing a scenario that observed nothing were each tried and each refused.

## I4. Harden the destructive lifecycle, and decide the profile race — complete

**Goal.** `install`, `uninstall` and `purge` behave under fault and contention,
and uninstall never deletes user data.

**Scope.** Backup capacity and corruption faults, restore drills,
process/mount/symlink races, external profile roots, modified installed
artifacts, unattended execution, and package-manager interoperability.

**One race is already identified and waiting.** A starting daemon validates
*every* profile in the registry rather than only the one it is starting, so two
profiles launched at the same moment read each other's databases and one aborts
with `stale_database: sqlite exec: database is locked`. It is a transient lock
reported as a stale library, and a retry would succeed. Found in v0.8 H13 when a
flaky test was made to say why it failed rather than only that it had; the test
now starts its daemons one at a time, which is what a person does — and which
deliberately left the product behaviour for this item to decide about.

**Boundaries.** Destructive drills run against disposable profiles. No drill
touches the evidence reserve or a real library.

**Dependencies.** I3.

**Working state.** Each fault injected and its behaviour recorded; the profile
race either fixed or refused with a reason, not left as a test workaround.

**Done, 2026-09-09.** Nine faults drilled against `scripts/lifecycle.py`, each
in its own disposable installation outside the checkout: an unattended purge
refuses without `FORCE=1` and says how to automate it deliberately; a purge
whose backup cannot be written, or does not fit, refuses with the library
intact; a purge that runs writes a backup that holds the library, excludes sync
key material, and **restores** — the note is read back out of it, because a
backup nobody has restored is a hope; uninstall leaves the user's data and keeps
a modified artifact while saying why; purge does not delete through a symlink
out of the profile; an external data root is used where it actually is.

**The profile race is fixed rather than worked around.** Startup opens only the
database it is starting. What that gave up is recorded and pinned by a test: a
database swapped underneath a stale registry entry is now invisible to a startup
that is not starting it, and visible to an audit. Racing two daemons is *not*
how it is asserted — the window is milliseconds and the unfixed code passed five
consecutive runs, and a test that only sometimes fails on a defect is not
evidence of a fix.

**One behaviour was found and recorded rather than blessed.** A purge run while
a daemon holds the library succeeds: it writes and verifies the backup, then
deletes, and the running daemon is never consulted. Nothing is unrecoverable —
the backup precedes the deletion, and the daemon keeps serving its open file —
but the user is not told a process is still running against what they deleted.
The drill asserts the property that matters, that nothing is deleted without a
backup, and leaves whether purge should notice a live daemon to I7, where
runtime state is already the subject.

**Three harness mistakes are kept in the record, because each gave a confident
wrong answer rather than an error.** The first version ran inside the checkout,
so `notriosctl` resolved source mode and the drills wrote notes into this
repository's own library and purge backups into the repository root — harmless
only because purge refuses a relative path as a deletion target. The first
library check asked the *installed* binary whether the note survived, and purge
removes that binary, so every successful purge reported data loss that had not
happened. And `tar -tf | grep -q` under `pipefail` reports failure when grep
exits early and tar dies of SIGPIPE, so a backup that contained the library was
reported as one that did not. A harness for destructive operations can least
afford exactly that failure mode, so `install_home` now refuses to drill unless
the resolved mode is `installed`.

**What was not exercised is gated.** A genuinely full filesystem (`ulimit -f`
refuses for a different reason than ENOSPC), mount races (they need privileges
these drills deliberately do not take, so that half of the slice is recorded
rather than exercised), interruption mid-purge, multi-user or root-owned
prefixes, and the packaged `apt` lifecycle, which I3 covers separately. Dropping
the symlink drill, dropping the interruption limit, removing what the race
record gave up, and claiming a drill observed nothing were each tried against
the validator and each refused.

## I5. Resolve signing, notarization and timestamping policy — complete

**Goal.** Every supported platform has a signing story that someone else can
follow, and what is blocked is written down as blocked.

**Scope.** Production signing, notarization and timestamping policy per
supported platform, and the handling rules for the material it needs.

**Boundaries.** **Certificates, tokens and passphrases stay out of
pull-request jobs, logs, artifacts, backups and the repository.** Where an owner
account or a native trust service is unavailable, the item records what remains
blocked rather than substituting something weaker and calling it done.

**Dependencies.** I1.

**Working state.** A policy per platform, and an explicit list of what cannot be
completed without which account or service.

**Done, 2026-09-09. No key was created, deliberately.** Deciding what a key is
for, whose it is, where it lives and how it rotates is this item; creating one
first would invert that. I6 is where a signing step first appears, under the
constraints recorded here. Nothing signs a release today — `build_deb.sh` and
`package_release.sh` produce unsigned artifacts, which is what v0.8 allowed for
internal evidence.

**The boundary is enforced rather than promised.**
`check_secret_exposure.py` refuses a workflow that could hand signing material
to a pull request, and the reason is concrete here: `ci.yml` runs on both `push`
and `pull_request`, and **a same-repository pull request receives repository
secrets** — so a repository-level signing key would be readable from any branch
anyone pushes. The policy is therefore environment secrets with required
reviewers, never repository secrets. Three rules, each tried against a
deliberately bad workflow and each refused: a secret in a workflow with a
`pull_request` trigger, `pull_request_target` at all, and a secret used with no
`environment` to gate it.

**What the gate cannot enforce is written down beside it.** Whether a step
echoes a secret to a log or writes it into an artifact is a runtime property of
that step, not of the workflow file, and no static check settles it. The backup
half of the boundary is enforced elsewhere: the purge backup excludes key
material, drilled in I4.

**The release key is not the evidence key**, and the reason is recorded rather
than assumed: the reserve's key exists so the archive's integrity is independent
of everything else, while a release key is used by automation on shipping's
schedule and exposed to a build pipeline. Sharing one would make the archive
only as trustworthy as the release pipeline's worst day.

**Two values are derived rather than restated**, because a fact written twice
becomes two facts. The RFC 3161 policy OID comes from
`evidence/verify_evidence.py` — the code that will actually reject a mismatched
timestamp — and the Ubuntu amd64 claim level comes from I3's report, which is
what installed and ran the package. Both are checked; moving either alone fails.

**Three platforms are blocked, and on machines and accounts rather than on
effort.** arm64 has no machine and stays at H6a's level 2 — signing an artifact
nobody has run would attest its origin and say nothing about whether it works.
Windows needs an OV or EV certificate, which is an organisational identity check
and, for EV, a hardware token or cloud HSM a hosted runner cannot hold. macOS
needs Apple Developer Program membership, which is an owner account and cannot
be delegated to a repository. Dropping a blocker's reason, drifting the claim
level away from I3's evidence, changing the timestamp OID away from the
verifier's, merging the release key with the evidence key, and removing
signature verification from the steps were each tried and each refused.

## I6. Generate and verify the release evidence set — complete

**Goal.** A candidate carries checksums, an SBOM and provenance that verify
offline.

**Scope.** Checksums, SBOM, dependency/license/security reports, provenance and
attestations, reproducible metadata, installer inventories, and pinned
least-privilege GitHub workflows. Exercise a non-public release-candidate or
draft flow.

**Boundaries.** v0.9 is not an end-user GitHub release. Workflows are pinned by
digest and least-privilege; a workflow that needs a secret to run is not added
to a pull-request trigger.

**Dependencies.** I5. I2 is deferred.

**Working state.** Each artifact generated and independently verified, and the
draft flow exercised without publishing.

**Done, 2026-09-09.** The set is four artifacts — the package, `SHA256SUMS`, a
CycloneDX 1.5 SBOM of 400 components, and an in-toto provenance statement — and
it verifies offline by recomputation rather than read-back.

**No SBOM tool was added.** None is installed, and the inputs were already here:
G20's licence inventory, derived from `go.mod` for its own reasons, and the two
npm lockfiles. That follows H6a's smallest-maintainable-toolchain rule and buys
something better than convenience — **the SBOM is cross-checked against an
inventory produced by different code for a different purpose**, and matches
exactly: 39 Go modules, 353 + 8 npm packages. A generator and a verifier written
from the same assumption fail together and look like agreement.

Licence expressions are parsed rather than string-matched, because
`(MPL-2.0 OR Apache-2.0)` is a choice between two acceptable licences and
refusing it as "not a bare SPDX identifier" would reject a dependency whose
terms are fine twice over.

**All 17 actions are pinned to commit digests**, with the tag kept as a trailing
comment so a reader can still tell which version a digest is. A tag is a pointer
somebody else can move, and whoever controls it chooses what runs here with this
repository's token. `ci.yml` declared no `permissions:` at all and inherited the
repository default — read/write on every scope — for a workflow that only builds
and tests; it declares `contents: read` now. The validator derives the pinned
count from the workflows rather than reading it from the report, so the record
cannot claim a hardening the repository does not have.

**The same mistake twice, caught two different ways.** The draft script builds
its upload list from `SHA256SUMS`, and that file does not list itself — so the
first version would have uploaded every artifact **except the file a downloader
checks the others against**. Running the dry flow showed it. Then the report
generator made the identical mistake, and the validator refused the report. A
dry run that only printed a command nobody read would have caught neither, which
is the argument for running the flow rather than describing it.

**The draft was not created.** Everything up to the API call is exercised: the
set verifies, `gh` is authenticated, the candidate tag is free, and the exact
command is printed for a person to read first. Creating a draft uploads
artifacts to GitHub, and an upload is an external write the owner authorises
separately — a draft is not public and creates no tag until published, but it is
still an upload.

**Follow-up slice I6-D, 2026-09-10: cross-checked, and scanned.** The tools were
not installed when I6 was written, and the report carried "no security scanner
ran" as a limit. The owner installed syft, cdxgen, grype and govulncheck, so the
limit was closed rather than carried.

**The cross-check found a defect no count check could have caught.** A scoped
npm package's namespace is percent-encoded in a purl —
`pkg:npm/%40antfu/install-pkg@1.1.0` — and this generator emitted a raw `@`.
Agreement with syft was **124 of 361 before the fix and 268 of 270 after it**.
The counts had been right all along and every scoped identifier was wrong, which
is precisely the failure a second opinion exists to find: the verifier and the
generator were written by one author from one assumption, and agreed with each
other perfectly.

The other two disagreements are explained rather than repaired. syft's 66 Go
modules are a superset of the 39 this project ships — the extra ones are test
and tooling dependencies the licence gate deliberately does not govern, and
**every module we ship appears in syft's set**, which the validator now
requires. cdxgen's 12 are the direct dependencies from `go.mod`, a subset of
ours.

**syft sees six GitHub Actions that this SBOM does not model at all.** I6 pinned
those actions by digest, so what runs in CI is controlled — but the release
evidence does not describe it. That gap is recorded, not closed.

**And a warning worth acting on rather than reading.** cdxgen reports that SBOM
generation invokes build tooling which inherits the environment, and named the
API keys exported on this workstation. Its output carried none of them — checked
— but generating an SBOM is itself a supply-chain surface, so the generators run
with a scrubbed environment.

**The scan is recorded and never gated, and the two tools show why.** grype
reports 26 advisories against the dependency graph, 7 of them Critical.
govulncheck, which walks the call graph, reports **0 reachable from this code**
and 18 in modules merely required. Both are true and they answer different
questions. Gating on the first would have failed the build on criticals the
second shows this code never calls — and a vulnerability database changes daily,
so a gate would turn a passing build red because somebody else published an
advisory, carrying no information about this commit. What is gated is the
record: that a scan ran, when, with what, and that its reachability half is
present. Turning it into a gate, undating it, dropping reachability, hiding an
undiscoverable module and dropping the CI-actions gap were each tried and each
refused.

**Reachability is Go-only.** Nothing equivalent ran for the 361 npm packages, so
grype's findings there stay module-level and unreduced. Recorded as a limit.

**What this does not establish, gated so it cannot shrink.** Nothing is signed
or timestamped — I5 created no key deliberately, and the verifier refuses a set
that records itself as signed while carrying no signature. The upload path,
asset limits and notes rendering are unexercised. The SBOM covers dependencies
rather than the package's contents. No vulnerability scanner ran; a CycloneDX
document is not a scan result. And the provenance is a statement this repository
wrote about its own build on a workstation, with no builder identity anybody
else can check — SLSA build level 1 at most.

## I7. Soak, recover, and freeze the support matrix — complete

**Goal.** The candidate survives being left running, and its claims are limited
to what was executed.

**Scope.** Long-lived installed directory, REST and emulator soak tests; native
integration and cleanup matrices; support-bundle redaction; crash-reporting
policy; disaster-recovery drills. Freeze the exact desktop support matrix.

**Boundaries.** A postponed platform remains absent from release claims. A soak
result that was not observed to completion is reported as incomplete rather than
extrapolated.

**Dependencies.** I3, I4. I2 is deferred.

**Working state.** Soak runs completed with their durations recorded, drills
executed, and a frozen matrix naming only what ran.

**Done, 2026-09-10.** 900 seconds, 347,092 requests, 0 errors, and the
measurement is the slope rather than the peak. Resident memory climbs about 6 MB
in the first thirty seconds and then oscillates inside a 1.16 MB band for the
remaining fourteen and a half minutes; descriptors do not grow at all.

**That rules out a fast leak and not a slow one, and the record says so.** The
fitted slope over the settled window is +794 kB/hour against a 1.16 MB
oscillation band, which fifteen minutes cannot distinguish from the sawtooth of
a garbage-collected runtime. My first draft called it noise; that was a claim
the run does not support, and the validator now refuses any version of the
record that says a slow leak is ruled out.

**`doctor` did not redact, and it is the command people paste into issues.**
`paths` and `config show` replace the home directory with `~` by default and
both offer `--no-redact`; doctor printed absolute paths, username and all.
`paths.Redact`'s own comment says resolved paths are printed *"in `notriosctl
doctor`"* and redacted there — documented intent nobody had wired up. Fixed with
`--no-redact` for parity, and the repository's own gates then caught the rest of
it: `TestNoCommandHidesAFlagItAccepts` refused a flag the usage did not describe,
which cascaded into the generated CLI documentation, G18a's section inventory
and G18f's pinned hash. Four gates for one flag, each one correct.

**An assertion that tested nothing, in two places.** `notriosctl search` echoes
the query back in its JSON — `{"hits": [], "query": "x"}` — so grepping the
output for the search term matches an *empty* result. I4's restore check passed
for exactly that reason and had never tested anything; the first version of the
recovery drill here concluded that a library it had just deleted still held its
notes, and I nearly recorded that as a pass because a `|| true` swallowed the
failure. Both count hits now. I4's drill was re-run and its record refreshed:
`restored_hits: 1`, so the claim it always made is now the claim it checks.

**The matrix claims only what ran: six rows, two supported.** The row most at
risk was the desktop shell. CI *compiles* it and I3's containers are headless, so
it sits at level 1 while the command line and service earned level 3 **in those
same containers** — compiling a GUI proves it links, not that it runs. Windows
and macOS are absent from release claims rather than listed as forthcoming, and
the validator derives the shipped platform's level from I3's report so a row
cannot be promoted by editing the matrix. Promoting the desktop shell, letting a
postponed platform back into release claims, marking arm64 supported without
running it, claiming a slow leak was ruled out, hiding descriptor growth and
dropping the emulator limit were each tried and each refused.

**What was not established.** Fifteen minutes is not a long-lived soak; the load
is one endpoint on loopback with no writes or concurrency; no emulator soak ran;
no crash was induced, so the crash-reporting position is checked by absence
rather than by observing where a panic goes; there is no support bundle to
redact, so what was tested is the diagnostics that exist; and `config show`
prints a credential *reference*, which names where a credential lives rather
than being one.

## I8. Freeze the 1.0 compatibility surfaces — complete

**Goal.** The interfaces 1.0 will promise are fixed and tested at their edges.

**Scope.** Freeze REST, MCP, archive, sync, configuration, installer, Make
lifecycle and shared C ABI compatibility candidates. Run ABI ownership, leak,
double-free, wrong-handle, concurrent-shutdown, cancellation and stream-limit
tests.

**Why the ABI tests are named individually.** v0.8 H11 found three host bugs and
one wrong constant that *passed* — `CANCELLED` was 9 where 9 is `unavailable`.
An ABI is only frozen at the edges somebody actually pushed on.

**Boundaries.** A freeze is a candidate until 1.0 accepts it. Breaking changes
after this item are recorded as breaking, not folded in quietly.

**Dependencies.** I6. I2 is deferred.

**Working state.** Each surface frozen with its compatibility test suite green,
and each named ABI failure mode exercised.

**Done, 2026-09-10. The freeze is live rather than a record.** Seven surfaces —
91 CLI commands, 113 REST routes, 45 MCP tools, 34 Make targets, 12 ABI symbols,
27 archive-contract files, 60 configuration keys — are re-derived from the source
that defines them on every `make validate` and compared with what was frozen,
naming what was added and removed. Adding a route is not forbidden; adding one
silently is. The typed ABI is frozen too: `nm` proves the twelve names are there
and `abidiff` proves their *signatures* are, because a parameter that changes
from `size_t` to `int` keeps every name and breaks every caller.

**Three derivations were wrong before they were right, and reading the counts
caught all three.** `make_lifecycle` found 5 targets of 34 — the pattern excluded
any target whose prerequisites contain `=`. `configuration` found **zero** keys,
looking for `yaml:` tags in a package that uses `json:`; a freeze of an empty
surface passes for ever. And the validator trusted the recorded hash rather than
recomputing it, so a **truncated member list passed**: the summary agreed with
live source while the list it summarised did not. That one was found by breaking
the gate on purpose, which is the only reason it was found at all.

**valgrind is the wrong instrument for a Go library, and wrong loudly.** Go grows
a goroutine stack by allocating a larger one and copying the frames into it,
rewriting pointers as it goes; to memcheck every one of those writes lands
outside a known block. The first run produced **ten million** invalid-access
reports, every frame in `runtime.*`, and a host that does nothing but call
`notrios_abi_version()` produces them too — the control that settles it.
Suppressions removed 9.5 million and it still hit memcheck's cap.

**So each instrument runs where it works**, which the owner's note named
exactly. AddressSanitizer crosses the c-shared boundary: 15 of 15 edge checks,
zero reports. ThreadSanitizer cannot — it maps a large shadow region at process
start, so through a c-shared library it fails with *"failed to allocate … bytes"*
and, with the host instrumented too, *"unexpected memory mapping"*, with ASLR
disabled as well. The race detector therefore runs on the Go side against the
same dispatch, session and handle code the twelve entry points call into, with
concurrency tests written for these edges: many callers on one session, close
while calls are in flight, cancellation racing completion. It was **proved live
before it was trusted** — a deliberately racy probe made it fire, then the probe
was removed. valgrind stays behind a flag for leak accounting, the one number it
still reports usefully.

**Every named failure mode is refused cleanly.** Handles never issued
(`INVALID_HANDLE`), a buffer released twice, a pointer the library never issued,
an instance closed with a call outstanding (`STALE_HANDLE` afterwards), two
threads on one instance, and a cancelled call polled to a verdict. One check was
my own bug first: it polled ten thousand times in a tight loop and reported that
a cancelled call never answered. It answers — the loop never let the runtime
schedule the goroutine that would produce the verdict. Counting iterations
measures the host's scheduling luck, so it uses a wall-clock deadline that
yields.

**What is not frozen is written down.** Behaviour: two releases can agree on
every name here and disagree about what a call does. The sync wire protocol and
the installer's on-disk layout are owned elsewhere. There is no fuzzing, `-msan`
did not run, and nothing tests `dlclose`, a second `dlopen`, or two processes
opening one profile.

## I9. Write the release-grade operational documentation

**Goal.** Somebody with neither the repository nor a development environment can
install, upgrade, roll back, back up, restore, uninstall, purge, troubleshoot
and verify artifacts.

**Scope.** Documentation for each of those, written for that reader.

**Boundaries.** No instruction is written that has not been executed in this
milestone. Every command shown is generated from or checked against the real
command line, the way the existing documentation gates require.

**Dependencies.** I3, I4, I5, I6, I7, I8.

**Working state.** Each document present, checked by the documentation gates,
and naming no step that was never run.

## I10. Serve the documentation site from notrios.com — complete

**Goal.** The documentation is published at `https://notrios.com/`, and the
configuration says so rather than carrying a placeholder.

**Scope.** Hugo's `baseURL`, a tracked `CNAME` in the site's static files, and
the G18b site contract that records what the site is configured to serve.

**Why the `baseURL` matters more than it looks.** It has been
`https://example.github.io/notrios/` since G18b — a placeholder, never the real
address. Two things follow. The host is wrong, and the *path* is wrong too: a
custom apex domain serves the site at `/`, not under `/notrios/`, which is
already what the Pagefind `bundlePath` (`/pagefind/pagefind.js`) assumes. So
this is a correction, not only a rename.

**Why a tracked CNAME, given the domain is already set.** GitHub reports
`build_type: workflow` and `cname: notrios.com`, so the domain lives in the
repository's Pages settings and `actions/deploy-pages` does not rewrite them.
The "every deployment wipes the custom domain" behaviour belongs to
branch-based publishing, where the deployed branch *is* the configuration and a
tool like `peaceiris/actions-gh-pages` overwrites it unless told otherwise —
this repository does not publish that way. The file is added regardless: it
costs nothing, `build_docs_site.sh` already copies `docs-site/static/` into the
build, and it keeps the domain with the site if the publishing source ever
changes. It is belt and braces, and recorded as such rather than as the load-
bearing mechanism.

**Boundaries.** No DNS change: CloudFlare and GitHub are already configured, and
this item does not touch either. No change to what the site contains. The push
and merge follow I1's route and authorization.

**Dependencies.** None.

**Working state.** The site builds with the real base URL, the built output
carries `CNAME`, and the contract check compares the recorded base URL against
the one Hugo is actually configured with rather than a literal in a validator.

**Done, 2026-09-09.** `baseURL` is `https://notrios.com/`,
`docs-site/static/CNAME` holds `notrios.com`, and the built site carries it —
`build_docs_site.sh` already copies `docs-site/static/` into the build, so the
file needed no build change to arrive.

**Changing the base URL broke a gate, which is how the placeholder had
survived.** `performance/v0.7-g18g` hardcoded `/notrios/` in three places: the
Pagefind result base, the prefix stripped from absolute links, and the prefix
that marked an asset as site-local. All three were correct while the base URL
was `https://example.github.io/notrios/` and wrong the moment it stopped being
— and the asset check failed in the worst direction, reading a root-served
`/css/x.css` as a *filesystem* path and reporting that the site did not carry a
file it plainly carried. The old build passed and the new one failed, which is
how I knew I had caused it rather than found it.

All three now derive the base path from `docs-site/hugo.toml`, so it can only be
wrong in the file Hugo actually reads. `performance/v0.7-g18b` had the same
shape — a literal base URL inside the validator, which is a second place to
edit, and a check that must be edited to keep passing is one that gets edited
without being read. It compares the contract against Hugo's configuration now,
and fails whichever side moves alone; both directions were tried.

The looser asset rule was checked for permissiveness rather than assumed: with a
referenced stylesheet removed the site reports 18 errors, and with a linked page
removed it reports the broken link.

**Two things about the reference, checked rather than repeated.** GitHub reports
`build_type: workflow` and `cname: notrios.com`, so the domain is held in the
repository's Pages settings and `actions/deploy-pages` does not rewrite them;
the "every deployment wipes the custom domain" behaviour belongs to branch-based
publishing, which this repository does not use. The CNAME is added anyway, for
the reason given above rather than as the load-bearing mechanism. And the base
URL needed the *path* dropped as well as the host changed: an apex domain serves
at `/`, which is what the Pagefind `bundlePath` had assumed all along.

**Open decisions**

- **HTTPS enforcement — Resolved by the owner, 2026-09-09.** It was off, with an
  approved certificate for `notrios.com` and `www.notrios.com`, so the site
  answered on plain HTTP. The owner enabled it: GitHub now reports
  `https_enforced: true`, and `http://notrios.com/` returns a 301 to
  `https://notrios.com/`. It was a repository settings change rather than a
  change in this repository, which is why it was the owner's to make.

## Decisions register

This is an index only; each decision is owned and explained inside its item.

| Decision | Owner | Status |
|---|---|---|
| Push `develop` to GitHub | I1 | **Blocking.** The owner authorizes the first external write |
| Merge the `develop`-to-`main` pull request | I1 | **Blocking.** The owner authorizes the merge, after review; method is a merge commit, already decided |
| Whether Wails v3 has released | I2 | **Taken 2026-09-09: it has not.** `wails/v3@latest` is `v3.0.0-beta.19`, so the migration is deferred to post-v1.0 and v1.0 ships on Wails v2 |
| The profile-registry validation race | I4 | Non-blocking default: fix it; refusing with a reason is the alternative, leaving it as a test workaround is not |
| Which platforms the frozen matrix claims | I7 | Non-blocking default: only what was executed here |
