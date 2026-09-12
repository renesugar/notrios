# Plan: v1.0 — Feature-complete local product

Status: **Planned from `ROADMAP.md` on 2026-09-10 after v0.9 closed. No item is
started. Product version is 0.8.0 and the canonical database schema is v27.**

This plan follows `ROADMAP.md`, the signing policy in
`performance/v0.9-i5/POLICY.json`, the frozen surfaces in
`performance/v0.9-i8/FROZEN.json`, the support matrix in
`performance/v0.9-i7/MATRIX.json`, and the item-writing rules in `AGENTS.md`.
Every item is independently approval-gated.

## Outcome and boundaries

v1.0 publishes. Everything before it was internal: v0.8 built a package nobody
installed, v0.9 installed it in containers and hardened what it found, and this
milestone is the first time a stranger can download Notrios, check that it is
what it claims to be, install it, use it, and remove it — without a checkout, a
compiler, or any part of this repository.

**Publishing is the last act, not the first.** J10 is gated on every other item,
and on the owner's authorization, because a GitHub Release is permanent in a way
a push is not: it is the artifact people cite, mirror and install from.

**Two gaps v0.9 found are the first items here, and neither is optional.** The
package is built on a workstation, so nothing can attest where it came from; and
a packaged installation has no supported way to delete its data, because every
safeguard `make purge` provides lives in a repository the user does not have.
Shipping a 1.0 with either would be shipping a known hole.

**What v0.9 established is not re-litigated.** The Ubuntu package installs and
runs; the destructive lifecycle is drilled; seven surfaces are frozen and
re-derived on every build; the C ABI is frozen by name and by signature. This
milestone extends those, and where it changes one it says so and re-records the
freeze.

Not in v1.0:

- the Wails v3 migration — v0.9 deferred it to post-v1.0 because v3 was still
  `v3.0.0-beta.19`, and a framework swap here would ship a desktop nobody
  measured;
- Windows or macOS installers, unless their build, install, runtime, signing,
  upgrade, removal and data-preservation gates pass; otherwise they are marked
  postponed and their build outputs are **not** presented as supported
  downloads;
- any physical Android claim: the emulator result H11 produced is an emulator
  result, and stays labelled as one;
- any implication that an unsupported iOS artifact exists.

## Progress

Generated from `docs/docplan/PLAN_SLICES.json` by
`go run ./cmd/docplan --write`, and checked by `internal/docplan`, which fails
the build when the ledger, this document and the repository disagree.

**The rules for keeping it current are in [`AGENTS.md`](AGENTS.md)** — under
"Writing plan items", "Keeping the plan current" and "Plan archival" — because
this section is archived when the plan completes and the rules are not.

<!-- notrios:generated:plan:progress:begin -->
**11 items: 2 complete, 0 in progress, 9 not started, 0 deferred.**

| Item | State | Slices done | Outstanding |
|---|---|---|---|
| J1. Build the package in a workflow, and attest what it built | complete | 3/3 | — |
| J2. Create the release signing key, and sign what ships | complete | 3/3 | — |
| J3. Give a packaged installation a supported way to delete its data | not-started | 0/3 | 3 |
| J4. Stabilise the REST and MCP surfaces for 1.0 | not-started | 0/3 | 3 |
| J5. Prove the library at scale | not-started | 0/3 | 3 |
| J6. Ship the versioned no-GUI library and header artifacts | not-started | 0/3 | 3 |
| J7. Validate backup, export, restore, sync compatibility and disaster recovery | not-started | 0/2 | 2 |
| J8. Security review for remote media and MCP | not-started | 0/3 | 3 |
| J9. Publish the release documentation for the supported matrix | not-started | 0/3 | 3 |
| J10. Publish the user-authorized release | not-started | 0/3 | 3 |
| J11. Report the installation's structure and manifest, and verify a purge against it | not-started | 0/3 | 3 |

Nothing is half-finished.
<!-- notrios:generated:plan:progress:end -->

## J1. Build the package in a workflow, and attest what it built — complete

**Goal.** The artifact people download is built by a workflow, and carries proof
of where it came from.

**Scope.** Move the `.deb` build into GitHub Actions — `dpkg-dev`, the frontend
build, the same `scripts/build_deb.sh` — and attest the result with
`actions/attest-build-provenance`.

**Why this is first.** v0.9 I6 wrote an in-toto provenance statement and
recorded its own limit: a statement this repository wrote about its own build,
on a workstation, with no builder identity anybody else can check — SLSA build
level 1 at most. An attestation closes that half, and it can only attest what a
workflow built. So the move is the prerequisite, not the polish.

**Boundaries.** `id-token: write` and `attestations: write` are declared on the
job that needs them and nowhere else; `performance/v0.9-i6/check_workflow_hardening.py`
already refuses a workflow that declares no permissions, and least privilege
means these two do not spread. No secret becomes reachable from a
`pull_request` trigger — `performance/v0.9-i5/check_secret_exposure.py` enforces
that and must keep passing.

**Dependencies.** None.

**Working state.** A workflow-built `.deb`, an attestation a stranger can check
with `gh attestation verify --repo renesugar/notrios`, and
`performance/v0.9-i5/POLICY.json` recording the prerequisite as met — which its
validator derives from the workflows rather than believing.

**Written 2026-09-11, and not yet run.** `.github/workflows/release.yml` builds
the package with `make deb` — the same entry point a developer uses, because a
workflow and a person running two different things is how they drift — and
attests it with `actions/attest-build-provenance`. It triggers on a `v*` tag or
by hand, uses `environment: production` so only the refs that environment admits
can reach it, and publishes nothing: the package stays a run artifact until J10.

**Two assertions in it are the interesting part, and both were provoked by
reading the build rather than by writing the workflow.**

*The package would have shipped without a GUI, silently.* `build_deb.sh` builds
the desktop binary conditionally: if the GTK or webkit headers are missing it
prints `no desktop GUI` and **carries on**, producing a package one binary
short. That is right for a developer without those headers and wrong for a
release. The workflow installs the headers and then checks the package actually
contains `notrios`, `notriosd` and `notriosctl`, so a missing dependency fails
the build instead of quietly changing what ships.

*A tag says which version is being released; `version.go` says which is being
built.* Nothing connected them, so a `v1.0.0` tag would have produced
`notrios_0.8.0-1_amd64.deb` and published it under the wrong name. The workflow
refuses when they disagree.

Both were exercised locally against the real package: the contents check passes
on the package as built and refuses the same listing with the GUI removed, and
the version check accepts `v0.8.0` against `0.8.0` and refuses `v1.0.0`.

**Proven end to end on 2026-09-11.** The workflow lints clean under `actionlint`
and passes both repository gates — every action pinned by digest, permissions
declared narrowly, no secret reachable from a `pull_request` — and then it was
run. Dispatched from `main` after the merge, which is what keeping `main` in the
`production` deployment policy was for: the pipeline is rehearsed without cutting
a tag that would read as a release of something not being released.

The attestation `gh attestation verify` returns is the claim v0.9 I6 said was
missing:

| | |
|---|---|
| predicate | `https://slsa.dev/provenance/v1` |
| subject | `notrios_0.8.0-1_amd64.deb`, sha256 `ef3aceff5da88b02…` |
| built by | `.github/workflows/release.yml@refs/heads/main` |
| from commit | `3180659fa70e` |
| issuer | `token.actions.githubusercontent.com` |

**A verification that accepts everything proves nothing, so the refusals were
checked too.** One byte appended to the package: refused. Verified against a
different repository: refused. And the *workstation-built* package of the same
version: refused — which is the sharpest of the three, because it shows the
attestation is about this build rather than about this project. Each fails as a
lookup miss, which is the mechanism working rather than a policy check: a changed
byte is a different digest, and no attestation exists for it.

**Exit codes were not taken as evidence.** `gh attestation verify` exits 0 and
prints nothing on success; the claims above come from `--format json`, because
an exit code says a command succeeded and not what it established.

**Two derived gates caught the change before I did**, which is worth recording
because it is the machinery working. `performance/v0.9-i5` records the
attestation prerequisite as `not met` and *derives* that from the workflows, so
adding one made the record false and validation refused it. `performance/v0.9-i6`
derives its pinned-action count the same way and refused `17` once there were
`22`. Neither would have been noticed by reading.

## J2. Create the release signing key, and sign what ships — complete

**Goal.** Somebody who downloads the package can establish who stands behind it,
not only which workflow produced it.

**Scope.** Create the release signing key under the custody v0.9 I5 decided,
sign the release artifacts and `SHA256SUMS`, and RFC 3161 timestamp the
signature.

**Why a key as well as an attestation.** They are different claims and the
policy already says so: an attestation says *this artifact came out of that
build*; a signature says *the holder of this key approved it*. A reproducible
build with impeccable provenance can still be a build of something nobody meant
to ship. The trust roots differ too — `gpg --verify` needs a public key and
nothing else, which is the property this project's evidence reserve is
deliberately built on.

**Boundaries.** **The key is not the evidence key**, for the reason I5 recorded:
the reserve's key exists so the archive's integrity is independent of
everything else, and a release key is used by automation on shipping's
schedule. It lives in an environment secret with required reviewers, never a
repository secret, and never reaches a `pull_request` job. Creating it needs the
owner: it is their identity.

**Dependencies.** J1, for the workflow that would use it.

**Working state.** A published public key, a signed release set, and
`verify_release_set.py` accepting a set that records itself as signed because it
carries signatures.

**Done 2026-09-11, and verified the way a downloader would verify it.** The
workflow signs the package and `SHA256SUMS`, timestamps the package's signature
against the authority this project already pins, and verifies the whole set
before it leaves the runner. The set was then downloaded and checked here:

```
gpg --verify   GOODSIG Rene Sugar, VALIDSIG primary 1234C691AC0776A18524D55687027B1DD464695E
sha256sum -c   all OK
openssl ts     Verification: OK, policy 2.16.840.1.114412.7.1, Sep 12 02:35:40 2026 GMT
```

**The verifier never verified, and that was the real work.** It noticed that
signatures *existed* and never checked one — a set could have carried a `.asc`
of anything and passed, which is a check that reads as protection and is not. It
now verifies each signature in a keyring of its own, holds the signing
fingerprint against the published key, and requires signatures over the artifact
and `SHA256SUMS` by name rather than inferring them from what happens to be
present. Proven with a throwaway key *before* the real one signed anything, and
refused in five ways: checked against the real key, a signature that is not one,
one artifact's signature swapped for another's, the checksum file unsigned, and
signed artifacts recorded as unsigned.

**Signing is inline rather than through an action**, because a third-party
action would be the component handling the most sensitive material in the
pipeline and importing a key is one command. The step also checks the imported
key *is* the published key, so a wrong secret fails the build instead of
producing a set signed by something nobody can verify against.

**Two ordering facts the rehearsal forced into the open.** `SHA256SUMS` cannot
cover its own signature — the signature is made over the finished file — which
is the reserve's closure boundary again, and the coverage rule says so rather
than failing on it. And signatures have to be derived from the directory rather
than from `SHA256SUMS`, because `SHA256SUMS.asc` is deliberately absent from the
file it signs; deriving from that list missed exactly the signature that matters
most.

**A finding for J10's deferred decision.** `VALIDSIG` shows the signature was
made by subkey `348B6B87…EA2C`, not by the primary — gpg prefers a
signing-capable subkey and the release key has one. So exporting only that
subkey into CI would produce byte-identical signatures, and the narrowing
deferred to J10 costs nothing in output. It is a smaller change than it sounded.

**The key exists already, created by the owner on 2026-09-11.** RSA-4096,
fingerprint `1234C691AC0776A18524D55687027B1DD464695E`,
`Rene Sugar <rene.sugar@gmail.com>`. Its public half is at
`keys/release-public.asc` and the private half is a `production` environment
secret named `GPG_RELEASE_PRIVATE_KEY`. Three facts were checked rather than
taken on trust:

- **It is not the evidence key**, which I5 requires. The evidence primary is
  `AEE5F82F…8098` and its signing subkey `4ABEB98A…4005`; this is a different
  key entirely, so a release-key rotation cannot disturb the reserve's chain of
  custody.
- **`keys/release-public.asc` is present in the working tree and is not tracked
  by git**, and nothing ignores it. Committing it is part of this item: the
  public key has to travel with the source, because a verifier who fetches the
  key from the same release page as the artifact has verified very little.
- **The `production` environment has no protection rules at all.** `gh api
  repos/renesugar/notrios/environments` reports `protection_rules: []`. The
  secret is scoped to the environment, which is the important half, but I5's
  policy says "an environment with required reviewers", and required reviewers
  are what make each use of the key a deliberate act rather than a consequence
  of a push. That is a settings change, not a repository change, so it is an
  open decision below rather than something this item can do.

**Scope this adds.** Commit the public key; write the release workflow that
imports the private key and signs; and have `verify_release_set.py` check a
signature against the committed public key rather than merely noticing one
exists.

**Open decisions**

- **Restricting which refs can deploy to `production` — Answered 2026-09-11 by
  the owner, and this one is closed.** Required reviewers were the wrong
  instrument: on a solo project they either wait for somebody who does not
  exist or reduce to self-approval, which is a click rather than a review.

  The environment now carries a custom deployment policy admitting exactly two
  refs — `branch: main` and `tag: v*` — where it previously admitted everything
  (`deployment_branch_policy: null`). That closes the exposure the reviewer
  requirement was reaching for: an arbitrary branch carrying a workflow that
  names the environment can no longer reach the key at all, whatever that
  workflow says.

  **And `main` is protected**, which is the half that makes admitting it safe:
  `protected=true`, required pull-request reviews, force pushes disabled. So the
  two admitted paths are a tag, and a branch that cannot be changed without a
  pull request and cannot be rewritten. That is better than the tags-only
  restriction recommended here, because it leaves room for a
  `workflow_dispatch` rehearsal from `main` without reopening anything.

  Verified rather than taken on trust:
  `gh api repos/renesugar/notrios/environments/production` and its
  `deployment-branch-policies`, plus `branches/main/protection`.

- **The passphrase-less design — Taken as the default, and the reasoning holds
  with one caveat.** A passphrase stored beside the key it unlocks adds no
  layer, GitHub's secret store is the vault, and `gpg --batch` cannot answer a
  prompt: all three are correct, and this is ordinary practice for a key that
  exists only for automation. The caveat is that "no passphrase" removes the
  last obstacle *after* exfiltration, so everything now rests on who can cause
  that environment to run — which is the decision above, and why the two belong
  together.
- **What goes in the secret: the whole key, or a signing subkey — Deferred to
  J10 on 2026-09-11, deliberately, and the reason is not the one offered.** The
  owner notes that hardware security keys are required to sign in to the GitHub
  account. That is a real and substantial reduction — it makes account takeover
  by credential theft very hard, and account takeover is the likeliest route to
  abusing this environment on a solo project. Together with actions pinned by
  digest (v0.9 I6), the environment restricted to `main` and `v*`, and `main`
  protected against direct pushes, the plausible paths are now narrow.

  **But hardware keys reduce likelihood, and this decision is about blast
  radius.** The secret is decrypted into the runner at job time, and anything
  executing in that job can read it — a compromised build dependency, or a
  mistake in the workflow itself. Neither involves signing in, so neither is
  affected by how the account is protected. If exfiltration happens anyway, a
  primary key means the attacker holds the identity: they can certify other keys
  and issue new subkeys, and recovery is reissuing the identity rather than
  rotating a subkey.

  **What actually makes deferring safe is that the key has signed nothing.** No
  release exists, nobody has fetched this public key, and no published artifact
  depends on it. Reissuing the identity today costs one `gpg --generate-key`.
  That cost starts growing the moment a signed release exists that somebody has
  verified — because then the key is a thing other people hold, and replacing it
  means telling them.

  So the deadline is **J10, not J2**: narrow the secret to a signing subkey with
  `gpg --export-secret-subkeys` before the first release anyone relies on, or
  record at that point that it was decided otherwise. Until then the exposure is
  real and its consequence is near zero, which is a defensible place to stand
  and a bad place to forget about.

- **Who holds the release key — Answered 2026-09-11: the owner, under their own
  identity.** It is an identity claim about a person, and the key's user ID says
  so.

## J3. Give a packaged installation a supported way to delete its data

**Goal.** A user who installed the package can remove their notes with the same
safeguards a developer gets.

**Scope.** A `notriosctl purge` carrying what `make purge` carries: a verified
backup written before anything is deleted, a refusal without an explicit flag
when nothing can answer a prompt, no deletion through a symlink out of the
profile, and sync key material excluded from the backup.

**Why it is not merely a port.** The safeguards are the feature. v0.9 I4 drilled
each one and they hold; what is missing is that a packaged user cannot reach
any of them, because they live in `scripts/lifecycle.py`. `internal/store`
already implements the backup and verification `lifecycle.py` drives, so the
work is moving the safeguards behind the command line rather than inventing
them.

**Boundaries.** The two Make targets keep working: a developer's muscle memory
is not the thing to break. Nothing gains a shorter path to deletion than it has
today — if anything the packaged form should be harder, because the user has no
checkout to fall back on. The CLI surface changes, so
`performance/v0.9-i8/FROZEN.json` is re-recorded and the commit says what moved.

**Dependencies.** None.

**Where the backup goes, which the owner specified.** `make purge` writes its
backup under the state root — a place chosen so the backup cannot land somewhere
the same run would delete — and then tells the user where it is. The command
must do the same and must write it to **a location the user knows about**,
because a backup somebody cannot find is not a backup. Opting out stays
possible and stays loud: `make purge` requires `NO_BACKUP=1` and prints a
warning that nothing will be copied anywhere before it is deleted, and the
command keeps that shape rather than inventing a gentler one.

**Working state.** `notriosctl purge` drilled by the v0.9 I4 harness against a
packaged installation, with every drill that passes for the Make target passing
for the command, and the backup location printed where the user will read it.

**Built 2026-09-11.** `notriosctl purge` carries what `make purge` carries: a
verified backup before anything is deleted, a refusal when nothing can answer,
no deletion through a symlink out of a profile, and sync key material kept out
of the backup. It deletes *data*, not program files — removing the program stays
`apt remove` or `make uninstall`, and both leave the notes alone by design.

**The safety-critical half is a port, and the two are held together.** H3's
oracle decides "may Notrios delete this path", and it was written before any
code could act on it precisely so the rules could be argued about first. Two
implementations of a deletion rule is the worst possible duplication, so
`internal/purge/oracle_test.go` drives H3's own thirty fixtures by name and
reads the expected verdict and rule out of `PURGE_ORACLE_FIXTURES.json` rather
than restating them — and fails if a fixture case exists that the port does not
exercise, so the coverage cannot rot either. Following a symlink instead of
refusing it, reclassifying `cache` as irreplaceable, and dropping `/usr` from
the forbidden roots were each introduced on purpose and each caught.

**Three things were found by running it rather than by writing it.**

*The port's own test tested the wrong rule.* Go's `filepath.Join` cleans its
result, so the dot-dot traversal case presented an already-normalised path and
reached the `home` rule instead of `normal-form`. Python's `os.path.join` does
not clean; the fixture comparison caught the difference.

*The prompt called `/dev/null` a terminal.* The obvious check —
`os.Stdin.Stat()` and `ModeCharDevice` — is true of `/dev/null`, because
`/dev/null` **is** a character device. A purge with stdin redirected therefore
printed a question nobody could see and reported an answer nobody gave. It
failed closed, which is the direction to be wrong in, but it told the user they
had declined rather than that nothing could ask them. Reading and treating an
immediate EOF as "nobody is there" is correct and needs no new dependency.

*A test proved a different rule than the one it named.* The exclusion check is
deliberately made against the archive rather than trusting the filter ran — so
the test plants key material in an archive. The first version replaced the
archive wholesale, and the "recorded but not in the archive" rule caught it
first. It now rebuilds every recorded member and re-records the hash, so the
only rule left to catch it is the one under test.

**The freeze fired, which is the machinery working.** Adding a command changed
the CLI surface I8 froze, and validation refused until the freeze was
re-recorded. The change is an addition — 91 commands to 92, nothing renamed or
removed — so it is compatible.

**Still owed, and recorded rather than glossed:** `scripts/lifecycle.py` still
has its own purge, so the dangerous logic now exists twice. The oracle halves
are gated against each other; the backup and deletion halves are not. Making
`make purge` delegate to `notriosctl purge` is the remaining work, and its
ordering is the interesting part — the data must be purged while the binary that
does it still exists, so delegation means purging before uninstalling rather
than after.

## J4. Stabilise the REST and MCP surfaces for 1.0

**Goal.** The REST API and the MCP tool and resource schemas are what 1.0
promises, and a change to either is a deliberate, visible act.

**Scope.** Review the 113 REST routes and 45 MCP tools I8 froze against what 1.0
should promise; remove or rename what should not be promised *before* the
freeze becomes a commitment; and record the result.

**Why now and not later.** I8 froze the surfaces so that change is visible.
Freezing is not the same as endorsing: this is the last milestone in which a
route can be withdrawn without breaking somebody, and the review has to happen
before the release rather than after it.

**Boundaries.** Behaviour is not frozen by a name freeze — two releases can
agree on every route and disagree about what one does — so this item is about
what is promised, and says which parts of behaviour it did not settle.

**Dependencies.** None.

**Working state.** A recorded review of every route and tool, the frozen
surfaces re-recorded if anything moved, and the deprecation of anything 1.0
should not carry.

## J5. Prove the library at scale

**Goal.** Notrios works on a library far larger than any it has been measured
on.

**Scope.** Large-scale performance tests with hundreds of thousands of documents
and resources: import, search, sync, backup and restore at that size, with the
numbers recorded.

**Boundaries.** A measurement not observed to completion is reported as
incomplete rather than extrapolated — v0.9 I7's rule, and the same reason.
Generated corpora are labelled as generated; nothing here implies a real
library of that size was used.

**Dependencies.** None.

**Where the corpus comes from — the owner supplied the sources, sizes measured
2026-09-11.** Three kinds, and they answer different questions:

| Source | Size | What it is good for |
|---|---|---|
| `/home/renes/projects/recipedb/recipe_joplin` | 5.1 GB | a very large Joplin RAW import, notes only |
| `/home/renes/projects/recipedb/recipe_vault` | 1.6 GB | the **same data** as an Obsidian vault |
| `/home/renes/Documents/Joplin Archive/JoplinExport_2026_07_18` | 1.5 GB | a large Joplin RAW export **with resources** |
| `/media/renes/HD2/twitter/twitter-…dd40.zip` | 3.1 GB | a large single-archive Twitter/X import |
| `github.com/renesugar/movenotes-v3` | — | generates Joplin and Obsidian corpora to order |

Scratch space for generated corpora: `/media/renes/HD2` (13 GB in use) and
`/media/renes/SEAGATE2TB` — **but not the second one for anything large**: the
evidence reserve lives there, and a performance run is not a reason to fill the
volume that holds the signed archive.

**The recipe pair is the most useful thing here and the least obvious.** The same
corpus in two import formats makes it possible to separate *importer* cost from
*store* cost: if Joplin and Obsidian imports of identical data diverge, the
difference is the importer, and nothing else in the measurement has to be held
constant to see it.

**Open decisions**

- **How the bulk corpus is generated — Non-blocking default: import the real
  ones first, generate only to fill gaps.** The supplied directories are real
  libraries with real shapes, and a generator produces whatever distribution its
  author imagined. The default is therefore to measure the real corpora, and to
  generate only for sizes and resource mixes they do not reach. `movenotes-v3`
  is the generator for that, cloned fresh rather than used in place, because the
  owner is modifying the local copy.
- **Whether a generator becomes a dependency of this module — Non-blocking
  default: no.** A corpus generator is a tool, not part of the product, and
  every module in `go.mod` is something the licence gate governs, the SBOM
  carries and a release inherits. If `gofakeit` or similar is used, it belongs
  behind its own module under `performance/`, the way the H10 Wails v3 prototype
  did — Go's internal-package rule is path-based, so a nested module can still
  import `internal/`. A `go get` at the repository root adds a requirement that
  nothing imports, which `go mod tidy` then removes and the G20 hash gate
  notices in between.
- **Whether to write SQLite libraries directly — Recommended against, and this
  is worth stating.** Generating text is easy — `gofakeit` is MIT and would pass
  the licence gate — and writing rows straight into a library would be the
  fastest way to a large database. It would also measure a
  library no importer ever produced: schema invariants, revision chains, asset
  references and search projections all get established *by* the write path, and
  a corpus that skipped it would flatter every later measurement. Generate
  import files and import them. If direct writes are ever needed for size, they
  are labelled as synthetic and never used for correctness claims.
- **What resource files the corpus uses — Non-blocking default: the archive with
  resources, plus a small generated set for formats it lacks.** The Joplin
  export carries real attachments; `OpenPrinting/sample-files`,
  `xeor/test_files` and `TestingFilesGenerator` cover PDF, PNG, ZIP and DOCX at
  chosen sizes if a gap appears. Anything downloaded is treated as untrusted
  input, which is the standing rule for imported material.

**Working state.** Recorded timings and resource use at the target size, which
corpus produced each number, and an honest statement of what degraded.

## J6. Ship the versioned no-GUI library and header artifacts

**Goal.** A third party can build against the C ABI without this repository.

**Scope.** Versioned library and header artifacts for the supported platform
matrix, with lifecycle, ownership, threading, error, stream and compatibility
examples.

**Boundaries.** The matrix is v0.9 I7's, which claims only what ran: Ubuntu
24.04 amd64. An Android-emulator result is not physical Android support and is
not labelled as such; no iOS artifact is implied.

**Dependencies.** J1, for the workflow that builds and attests artifacts.

**Working state.** Downloadable library and header artifacts, examples that
compile against them, and the ABI baseline in `performance/v0.9-i8` unchanged or
re-recorded with a reason.

## J7. Validate backup, export, restore, sync compatibility and disaster recovery

**Goal.** Data written by one version comes back through another, and a lost
library is recoverable.

**Scope.** Backup, export, restore and sync compatibility across the versions
1.0 will interoperate with, and disaster-recovery drills at the scale J5
establishes.

**Boundaries.** Compatibility claims name the exact versions tested. A drill
that was not run to completion is reported as incomplete.

**Dependencies.** J5, for the scale; J3, for the purge path a recovery follows.

**Working state.** Recorded round trips across versions, and a recovery drill
that destroys and restores a library of the size J5 measured.

## J8. Security review for remote media and MCP

**Goal.** The two surfaces that reach outward have been examined by someone
looking for the failure rather than confirming the design.

**Scope.** Remote-media localization — domain policy, quarantine, hashing,
MIME sniffing, size limits, SSRF protection — and the MCP surface, its tool
scopes and what a client can reach through it.

**Boundaries.** A review records what it examined and what it did not. Nothing
here becomes a claim that the surfaces are secure; the output is findings and
their disposition.

**Dependencies.** J4, so the MCP surface being reviewed is the one 1.0 ships.

**Working state.** Recorded findings, each fixed or explicitly accepted with a
reason.

## J9. Publish the release documentation for the supported matrix

**Goal.** The documentation site and the release carry what a stranger needs,
with exact rows rather than reassuring prose.

**Scope.** Installation and support documentation with exact supported
OS/architecture/runtime rows, migration, rollback, disaster-recovery and
artifact-authenticity instructions — including how to verify an attestation and
a signature.

**Why it is nearly last.** v0.9 I9 rewrote the installation page for a reader
with only the package, and it documents two things that J1, J2 and J3 change:
that nothing is signed, and that a packaged install cannot delete its data. Both
sentences must become false before the release, and this item is where they are
corrected.

**Boundaries.** No instruction is written that has not been executed. Every
command is generated from or checked against the real command line, and the
example registry records where each one runs.

**Dependencies.** J1, J2, J3.

**Working state.** Documentation whose supported rows match
`performance/v0.9-i7/MATRIX.json`, and no sentence left that the milestone made
untrue.

## J10. Publish the user-authorized release

**Goal.** Notrios 1.0 exists, and what was published is what was reviewed.

**Scope.** Synchronize `develop` and `main`, pass required CI, the installer
readback and the pre-push evidence gates, then create the release from reviewed
`main`. Afterwards verify the version tag, release notes, installers,
checksums, signatures, SBOM and provenance, upgrade and rollback instructions,
and the downloaded bytes.

**Why the verification comes after publication too.** What is on a release page
is not necessarily what was uploaded, and the only way to know is to download it
back and check it. That is the same reason the reserve reads an ISO back after
burning it.

**Boundaries.** **The release needs the owner's explicit authorization, and this
plan schedules it rather than granting it.** Nothing is tagged, published or
uploaded before that. The evidence is sealed into the reserve before the release
is published, for the reason v0.8e exists: a timestamp taken afterwards cannot
establish that the evidence predates disclosure.

**One decision arrives here from J2.** The CI secret holds the release key's
primary, which can certify as well as sign. That was deferred because the key
had signed nothing and reissuing the identity cost a keygen — a calculation that
stops being true at exactly this point, because a published release is a key
other people hold. Narrow the secret to a signing subkey before publishing, or
record here that it was decided otherwise and why.

**Dependencies.** Every other item.

**Working state.** A published, verified release, and a reserve volume sealed
before it.

## J11. Report the installation's structure and manifest, and verify a purge against it

**Goal.** A user, or a script, can ask where every Notrios file and directory
is — and after a purge, confirm that they are gone.

**Scope.** A report with two parts, from `notriosctl` or an adjacent tool:

- **structure** — every directory that holds Notrios files or data, and
- **manifest** — every Notrios file and data file, by absolute path.

Plus a `bash` check that reads a manifest taken before a purge and confirms each
path is absent afterwards.

**Why the existing manifest is not enough.** `scripts/lifecycle.py` already
writes `MANIFEST.json` into the data root, recording what the installer put
where — prefix, bindir, datadir and a sorted list of installed entries. It
covers **program files only**. The user's data is precisely what it does not
list, and the user's data is what a purge deletes, so the manifest cannot
answer "did the purge work". This item extends the report to both.

**Why the check is a shell script and not a Go test.** Everything Notrios
installed is gone after a purge, including anything that could read a manifest
and including the manifest itself. The verification has to survive the thing it
verifies, which means it runs outside the installation, from a copy of the
manifest taken beforehand, in a language the machine already has.

**Two uses, both of them concrete.** After `make install` on a clean machine,
the report *is* the record of what a clean install looks like — which is the
thing v0.9 I3's container matrix asserted piecemeal and never captured whole.
After a purge, the same report taken beforehand becomes the oracle for whether
the deletion was complete.

**Boundaries.** The report names paths; it does not print note content, and it
redacts the home directory to `~` by default like `paths`, `config show` and —
since v0.9 I7 — `doctor`, with `--no-redact` for the literal form. A manifest is
a list of where things are, and that is exactly what somebody pastes into an
issue.

**Dependencies.** J3, so the purge it verifies is the one a packaged user can
run.

**Working state.** A structure-and-manifest report for a clean install, a shell
check that passes on a completed purge, and — proved by running it — fails when
a single file is left behind.

## Decisions register

This is an index only; each decision is owned and explained inside its item.

| Decision | Owner | Status |
|---|---|---|
| Who holds the release signing key | J2 | **Blocking.** An identity claim about a person or project; it cannot be delegated to a repository |
| Whether Windows or macOS ship | J6, J9 | Non-blocking default: postponed and absent from release claims unless their gates pass |
| What the REST and MCP surfaces promise at 1.0 | J4 | Non-blocking default: what I8 froze, minus anything the review withdraws |
| Publishing the release | J10 | **Blocking.** The owner authorizes the tag and the publication |
| Which refs may deploy to `production` | J2 | **Answered 2026-09-11.** Restricted to `main` (protected, PR-required, no force push) and tags `v*`; required reviewers correctly rejected as a team instrument |
| Whether the CI secret holds the primary key or only a signing subkey | J2 → J10 | **Deferred 2026-09-11.** Safe for now because the key has signed nothing, so reissuing costs a keygen; revisit before the first release anyone relies on |
| How the large-scale corpus is generated | J5 | Non-blocking default: import the real supplied libraries first, generate only to fill gaps, never write SQLite directly for correctness claims |
