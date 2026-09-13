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
**16 items: 7 complete, 0 in progress, 9 not started, 0 deferred.**

| Item | State | Slices done | Outstanding |
|---|---|---|---|
| J1. Build the package in a workflow, and attest what it built | complete | 3/3 | — |
| J2. Create the release signing key, and sign what ships | complete | 3/3 | — |
| J3. Give a packaged installation a supported way to delete its data | complete | 5/5 | — |
| J4. Stabilise the REST and MCP surfaces for 1.0 | complete | 3/3 | — |
| J5. Prove the library at scale | not-started | 0/3 | 3 |
| J6. Ship the versioned no-GUI library and header artifacts | not-started | 0/3 | 3 |
| J7. Validate backup, export, restore, sync compatibility and disaster recovery | not-started | 0/2 | 2 |
| J8. Security review for remote media and MCP | not-started | 0/3 | 3 |
| J9. Publish the release documentation for the supported matrix | not-started | 0/3 | 3 |
| J10. Publish the user-authorized release | not-started | 0/3 | 3 |
| J11. Report the installation's structure and manifest, and verify a purge against it | not-started | 0/3 | 3 |
| J12. Make the README true, and generate what can be generated | complete | 4/4 | — |
| J13. Generate the published command-line examples from executed runs | complete | 4/4 | — |
| J14. Stop leaving bytecode behind, and derive the evidence index | complete | 3/3 | — |
| J15. Migrate the remaining documents to the tracked example set | not-started | 0/3 | 3 |
| J16. Give the carrier write its own path shape | not-started | 0/3 | 3 |

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

## J3. Give a packaged installation a supported way to delete its data — complete

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

**Drilled against the command, with I4's own drills.** Six of them, in
`performance/v1.0-j3`: the unattended refusal, a backup that cannot be written,
a backup that does not fit, the backup that holds the library and excludes sync
keys and **restores** — counted by hits rather than grepped, which is I7's
lesson — the symlink that is not followed out of the profile, and one this item
adds.

**The one it adds came from giving the user a choice.** `--backup-dir` lets
somebody name a destination, and the obvious wrong answer is inside the library
they are about to delete. Every run now asserts the destination is *refused* by
the oracle. H3 proved that property for the default location; this checks it
rather than trusting the layout, and a destination inside the data root is
refused with the library intact.

**The validator had the hole it was written to prevent.** It compared the report
and the run by drill *name* only, and read the restore counts from the run log —
so a report claiming the restore recovered nothing, or that sync keys reached
the backup, passed. Found by probing it, which is the only reason it was found.
It compares observations now.

**`make purge` no longer deletes anything.** It runs the command. 181 lines of
Python — the planning, the measuring, the backup, the verification and the
`rmtree` — are gone, and what is left is the half the command cannot see: what
`make install` recorded writing. It reads the command's plan from
`purge --dry-run --json`, shows both halves together, asks one question about
both, and then runs `purge --confirm --no-plan`, which skips the question the
command would ask and nothing else. The backup destination is passed rather than
recomputed, because it carries a timestamp and the path described has to be the
path written to.

**The ordering inverted, and the tests found what that broke.** lifecycle.py
uninstalled and then deleted; it now deletes and then uninstalls, because the
binary that deletes the data is one of the files uninstall removes. Two tests
failed immediately, and on something I had not predicted: the install manifest
lives *inside the data root*, so the data purge takes it, and the uninstall that
followed found no manifest and refused — notes gone, installed files left
behind. The manifest is now read once, before the deletion, and handed to the
uninstall that runs after it.

**The 27 passing tests could not tell the difference.** Every one of them would
have passed just as happily with the delegation removed and the Python deleting
again — they assert that the data is gone and a backup exists, not who did it.
So two tests replace the installed command with one that plans truthfully and
then does nothing: the data must survive (this script deletes nothing itself)
and, when the command fails, the installed files must survive too (the two
halves fail together, or a user is left with their notes and no program to
delete them with). Both were confirmed by putting an `rmtree` back and watching
them fail.

**The command grew what the Make target already said.** The sync-key warning —
which files are about to be deleted and deliberately not backed up, said before
the question rather than after the deletion — and the full `--no-backup`
warning existed only in the Python. The packaged user, who has no Make target,
was the one person who never saw them. The key-file list is now part of the
command's plan rather than recomputed by the caller, so `make purge` cannot
warn about a different set of files than the command excludes.

**Four of I4's nine drills are still not carried over,** because they exercise
the installed-file half this command deliberately does not touch. The record
names each one rather than running fewer quietly.

### J3-D, added after the item closed: the rule that could not fire

Asked whether an install-time manifest was driving the deletion — it is not; the
data half resolves roots live and walks the filesystem — the answer surfaced
something else. `ExternalProfilePaths` is an oracle rule with two of H3's
fixtures behind it, and **both callers passed an empty list.** The Python did
too, before it delegated, so the Go port lost nothing: the rule had never been
reachable. A profile whose library lives outside the six resolved roots was
never deleted, which is right, and never enumerated or mentioned, which is not
— purge prints what it will remove and asks one question about it, and an
external library was silently missing from that list.

The command now reads the profile registry, resolves each profile's database,
asset store and config path, and passes the ones outside the owned roots. They
appear as `NOT DELETED`, attributed to the profile and the field that named
them. The oracle gets the same list, so a root that *contains* an external
profile is refused rather than deleted with the profile inside it — and a test
asserts the list is what causes that refusal, which is the assertion whose
absence let the gap survive. A registry that cannot be read produces a notice
rather than silence, because "no external profiles" and "I could not tell" are
different sentences and only one is safe to act on.

A seventh drill registers a real second library outside the roots, checks the
plan names it, purges, and checks the library is still *readable* afterwards by
counting hits rather than grepping — I7's lesson. Confirmed by breaking it: with
the enumeration removed, the plan stops naming the library and the drill fails.

**What is deliberately not done, and why it is worth saying:**
`BackupPolicy("external")` is `backup_never_delete`, and H3's fixture reason says
such a path is "enumerated and backed up". The copy is not implemented. Purge
does not delete these paths, so a copy adds no recovery — and because a backup
that cannot be written refuses the whole purge, one large external library would
make purge impossible for exactly the user who has one. So the disagreement is
now between the policy's *name* and the code, and closing it means either
renaming the policy (which touches H3's recorded vocabulary, so it is a decision
rather than an edit) or adding an opt-in flag.

### J3-E, the correction J3-D needed within the hour

Handing the enumerated paths to the oracle looked obviously right. It was
wrong, and only a run showed it. Every enumerated path is outside every owned
root — that is what made it external — so the rule can never fire to *protect* a
root. The one case it fires on is the reverse: a profile naming an **ancestor**
of the roots, which `profile register --asset-store ~/.local/share` produces by
accident. The data root was then `REFUSED`, the library survived a confirmed
purge, and the line claimed it had been "enumerated and backed up" when nothing
had been copied anywhere. A purge that silently keeps the library is the worst
outcome this item has.

**The owner's decision, taken the same day:** an external library is *reported*,
neither deleted nor copied, and it must never block a purge. So the enumerated
paths inform the user and nothing else, and an eighth drill holds it there: a
profile naming a parent of the roots refuses nothing, the library is deleted,
the named parent survives, and the plan says on that same line that the roots
inside it are still deleted — because "not deleted" is true of the directory and
badly misleading about its contents.

That also closes the copy half rather than leaving it owed.
`BackupPolicy("external")` keeps the name `backup_never_delete`: it is H3's word
for H3's intent, it lives in sealed evidence that fixtures gate against, and
evidence is not rewritten to agree with later code. The plan output says
"neither deletes nor copies it" so a reader gets the behaviour rather than the
policy name, and both call sites carry a comment saying why nothing re-wires it.

**J3's archive predates J3-D and J3-E.** `notrios-v1.0-j3-45d097a.zip` was built
when the item closed; both are later work on the same files, and the commits
that carry them are named in the git history rather than sealed in that archive.

**Also fixed while in there:** `scripts/test_lifecycle.py`'s `unittest.main()`
sat above its last class, so `python3 scripts/test_lifecycle.py` ran three fewer
tests than `unittest discover` did and said nothing about it. And its fixture
CLI was a shell stub, which was fine while purge only asked the binary where the
roots were; now that purge asks it to do the deleting, the suite builds and
installs the real one.

## J4. Stabilise the REST and MCP surfaces for 1.0 — complete

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

### What the review found

Derived first, judged second: described, documented, exercised and tested, per
member, because 113 routes read in a list all look equally reasonable and a
judgement is only worth having if it was made against evidence. **None of those
four is a verdict**, and the report says so in its own text.

| | REST | MCP |
|---|---|---|
| members | 113 | 45 |
| described in the machine-readable contract | 112 | 45 |
| mentioned in a published document | 113 | 45 |
| used by an example this repository executes | 39 | 1 |
| named by a serving test | 63 | 45 |

**One decision is the owner's and was not made here.** Carrier reads are
namespaced and carrier writes are not, so both two-segment forms collapse onto
one OpenAPI path where the same parameter means different things depending on
the method — which is why they are called `{segment1}` and `{segment2}`, no
honest name existing for a parameter whose meaning depends on the verb. The
implicit namespace is a real security property: you cannot publish into someone
else's namespace because the URL gives you no way to name one. The cost lands on
anyone generating a client. Changing it breaks the sync wire between peers,
which this milestone still permits and the next does not, so it is recorded with
both options and **nothing was changed**.

**Three advertised MCP tools that nothing tested, now fixed.** `plan_sync`,
`request_resource_fetch` and `retry_sync_job` were advertised by the server,
documented and scope-mapped, with no test anywhere naming them. An advertised
tool promises that calling it does something and that the scope gates say who
may; neither was checked. All three are covered now, gated so a fourth fails,
and MCP is 45 of 45.

**Two that looked wrong and are not,** recorded so the next reviewer finds the
answer rather than rediscovering it: `GET /` is absent from the API contract
because it serves the web interface, and the sync tools sit in the read-only
tier because they pass a second, orthogonal `mcp.sync_scope` gate.

### Two false findings, and what they cost

**The first draft reported two carrier routes as missing from OpenAPI.** They
are not — the contract describes them under different parameter names, and
comparing route *text* rather than route *shape* had made documentation style
look like a missing route. Chasing it is what uncovered the real finding above.

**The `tested` check was satisfiable by a comment.** A probe that removed
`retry_sync_job` from the new test still reported it tested, having matched the
bare name in that test's own doc comment. It requires the name in quotes now,
because a tool is called by name and only mentioned in prose — and the gate for
the MCP finding depends on this check, so one a comment satisfies was worse than
none.

Both were found by probing checks I had just written and believed.

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

**One decision arrives here from J2, and the owner has acted on it.** The CI
secret held the release key's primary, which can certify as well as sign; it was
replaced on 2026-09-12 with an export narrowed to the signing subkey.

**What confirms that, and what cannot.** A GitHub secret is write-only, so the
value cannot be inspected — and the workflow's existing fingerprint check cannot
see the difference either: a `--export-secret-subkeys` export still imports a
stub for the primary, so the first `fpr` line is the published primary
fingerprint either way. What separates them is field 15 of the `sec` record:
`+` when the primary's secret material is present, `#` when it is only a stub.
Measured on a throwaway key rather than taken from a manual page, and the check
was run against both export shapes before being installed — the full export
refused, the subkeys-only export accepted. The workflow now asserts `#`, and
asserts the signing subkey's own secret is present so that a narrowing which
went too far fails at the gate rather than at the signing step.

**The narrowed secret signs a release.** Dispatch
[34697727452](https://github.com/renesugar/notrios/actions/runs/34697727452)
on `main`, created 2026-09-12 13:54:27Z against a secret replaced at 13:14:14Z,
so it is the new value and not the old one — the two earlier dispatches that day
(01:55Z, 02:32Z) predate the replacement and say nothing about it. Every step
green: `signing key: 1234C691AC0776A18524D55687027B1DD464695E`, the `.deb` and
`SHA256SUMS` detached-signed, the RFC 3161 timestamp verified against the pinned
TSA roots, and `release set verified: 6 artifacts, 2 signatures checked,
provenance bound to the bytes, state signed` from the verifier a downloader
runs. A subkey-only export is sufficient to sign, which was the open question.

**What that run does not establish, and what is left.** It proves the secret
*signs*; it cannot prove the secret is *narrow*, because `main` does not yet
carry the field-15 check — the run would have passed identically with the old
full-primary value. Carrying the gate to `main` is what turns the narrowing from
reported into proven, and it is the one thing still owed here.

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

**It must also cover every registered profile, including the external ones.**
Asked whether such a report already named external libraries, the answer was no,
and not by a small margin: the closest thing that exists is `notriosctl doctor`,
which reports the *default* database and asset store and does not mention
profiles at all. A user with several profiles gets a structure report about one
of them. J3-D taught the shape of the answer — read the registry, resolve each
profile's database, asset store and config path, and mark the ones outside the
owned roots — and this item is where it becomes a report rather than a line in a
purge plan. Two consequences follow: the report is the thing that tells somebody
where their libraries are *before* they purge, and the post-purge check must not
expect an external library to be gone, because purge deliberately keeps it.

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

## J12. Make the README true, and generate what can be generated — complete

**Goal.** The repository's front page answers what it claims to answer, and the
parts of it that go stale are derived from tracked data rather than edited by
hand.

**Scope.** Six defects, all reported by the owner reading it:

- **The project status is wrong.** It says `v0.8 (current, 0.8.0)` and does not
  mention v0.9 at all. The plan and roadmap already carry generated blocks —
  `go run ./cmd/docplan --write` writes both — and the README has none, which is
  why it is the document that drifted. It gets one.
- **The opening paragraphs are long enough to hide their own structure.** They
  are read first and by the reader with the least context.
- **Quick start omits the lifecycle.** `make install`, `make uninstall` and
  `make purge` are not there, and `make clean` and `make clobber` are named
  nowhere with an explanation of what they remove. A reader cannot tell which
  targets touch their data — which, given that one of them deletes a library, is
  the least acceptable omission on the page.
- **Configuration points nowhere.** The README has a brief section and one
  example file, and there is no `docs/configuration.md` for it to refer to. The
  configuration surface is 63 JSON-tagged keys with a recorded owner
  (`internal/config#Config`), and one example is not documentation of 63 keys.
- **Serving the documentation site locally is undocumented.** `make docs`
  builds `_site/`, and nothing tells a reader that, or that
  `python3 -m http.server 8000 --directory _site` is enough to read it.
- **`_site` goes stale silently.** Nothing in `make validate` rebuilds it;
  `make g18g-validate` does and is only reached by `scripts/package_release.sh`.
  A checkout can carry months-old rendered documentation while every gate is
  green, and the owner found exactly that.

**Why the generated block matters more than the correction.** Editing the
version row fixes today and guarantees the same bug next milestone. Every other
generated surface in this repository — the plan's progress table, the roadmap's
status line, the docs inventory, the licence inventory — is derived and gated,
and each of those was made derived *after* a hand-maintained copy went wrong.
This is the same lesson arriving at the README.

**A trap worth naming before somebody hits it.**
`scripts/build_docs_site.sh` pins the document count at **exactly 18**
`docs/**/*.md` files, deliberately, so that a page appearing on the published
site is a decision somebody made. Adding `docs/configuration.md` makes it 19 and
the docs build fails — and it fails at packaging time, not in `make validate`,
which is the same late-failure the script's own comment complains about. The
pin is bumped as part of adding the page, and this is where the note lives so it
is read before the build breaks.

**Boundaries.** No instruction is written that has not been run. The
configuration document is checked against `internal/config` rather than
described from memory, and a key documented but absent — or present but
undocumented — fails rather than reads well.

**Dependencies.** None on the other items; the configuration document overlaps
J9's documentation work and is deliberately separate, because J9 is about the
supported matrix and this is about the front door.

**Working state.** A README whose status block is generated and gated, whose
quick start covers every target that touches a user's files, and which points at
a configuration document that exists; a documentation site a reader can serve
locally, with instructions that were followed to write them.

### What it took, and what the gates caught

**The generated block is gated three ways,** each confirmed by breaking it: a
stale block fails, a milestone row that marks itself *current* fails — that is
the shape of the mistake, not one instance of it — and removing the markers
fails. The milestone and the product version print as two facts because they
disagree: the plan is v1.0 and the binaries report 0.8.0, since the version is
bumped when the release is cut. One number would have hidden that.

**The opening was worse than long.** It was a fifty-line changelog that stopped
at "v0.8 H1 is next and unapproved" and still said GitHub push was
unauthorized — two milestones and one authorization out of date. Project status
gained the v0.8e, v0.9 and v1.0 rows it never had.

**`docs/configuration.md` is task-shaped with a generated table,** and the
generator reflects over `config.Config` and `config.Default()`, so a documented
default is the default the service applies. It reports **53 settable keys** and
reconciles that against the recorded surface's 63 from the same walk: the
difference is the 10 section names. A key with no doc comment gets an empty
cell rather than an invented description — visible, and therefore fixable.

**`make docs-stale`** reports rather than fails. A stale git-ignored build
directory is not a reason to refuse a commit, and a gate on it would be switched
off inside a week; `--fail-stale` exists for a caller that disagrees. Its first
output was "0.0 day(s)", which is true, useless, and the kind of number a reader
stops believing — it picks the largest unit that does not round to zero now.

**Eight pinned counts refused the new page, which is the system working.** The
docs-site build (18→19), the Help-seeding test and its two seed reports, the
G18a page count and grade baseline, the G18b staging prototype and its test, the
docaudit sections and denominator, the G18g section inventory and Pagefind index,
and the G18g public route list. The last one matters most: those are addresses
other people may have linked to, and nothing has ever been removed from it.

**The `make docs-stale` target also tripped I8's frozen surface** — 34 lifecycle
targets to 35 — and a docrules gate caught the rewritten opening dropping its
pointer to AGENTS.md's "Keeping the reference documents current", which is
precisely the section a rewriter needs.

## J13. Generate the published command-line examples from executed runs — complete

**Goal.** Every command line a reader is shown either ran, or says plainly that
it did not and why — and the ones that ran are inserted from the run rather than
typed beside it.

**The question the documentation does not answer.** For a configuration option:
*what do I use this for, and what else must be set for it to work on a command
line?* `docs/configuration.md` gained the key table and the prose in J12 and has
**zero** command-line examples, so a reader learns that
`search_sidecar.index_dir` exists and not what to run to use it, nor that it is
useless without the rest of the `search_sidecar` section and a `recollindex` on
the machine. That coupling — which options must be set *together* — is the part
a key table structurally cannot express.

**What exists already, and why this is an extension rather than a new idea.**
`internal/docexec` executes hash-pinned fenced blocks out of the documentation
against real fixtures, through adapters (`cli-shell`, `loopback-shell`,
`config-fragment`, `cli-edge`), and `docs/docaudit/registry.json` records each
one as executed or unverified with a reviewed reason. `cmd/docgen` already
*generates* source-anchored fragments into the pages. So the machinery for
running examples and the machinery for generating prose both exist; what does
not exist is a tracked source of *use-case* examples whose text is generated
from the run.

**Measured, not estimated** — from the registry, 2026-09-12:

| | count |
|---|---|
| examples published | 164 |
| executed against fixtures | 65 |
| unverified with a reviewed reason | 99 |
| of those, `illustrative-placeholder` | 41 |

By document, worst first: `docs/cli.md` 3 executed / 53 unverified,
`docs/installation.md` 1 / 23, `docs/operations.md` 11 / 9,
`docs/configuration.md` 0 / 0 — it has no examples to be either.

**The 41 placeholders are not one problem.** 35 are in `docs/cli.md` and are
command *synopses* — `notriosctl search [--limit N] … "<query>"` — where the
brackets are optional-argument notation. A synopsis is not a broken example; it
is a different thing, correctly unexecutable, and replacing it with a worked
example would make the reference list worse. The ones that deserve the owner's
description — hand-written and never verified — are the recipes that *look*
runnable: the composite operator recipes in `docs/operations.md` that mix a safe
default with an illustrative `/safe/snapshot` path, the `SHA256SUMS` check in
`docs/installation.md` that names a file from a release set this repository does
not contain, and the publishing recipe. This item separates those two
populations before changing either, because a plan that treats them alike would
delete good synopses to improve a number.

**Scope, in order.**

- **J13-A, the examination.** Classify every fenced block in `docs/` as
  synopsis, use-case recipe, or transcript, and report per document: which
  commands have a synopsis and no worked example anywhere, which recipes are
  unverified, and which sections offer an option with no example of using it.
  A report, gated for freshness, not a score.
- **J13-B, the tracked source and the generator.** A per-document example set
  (`docs/docexamples/<document>.json`, matching how `docexec` already addresses
  examples per document and section), each entry naming the use case, the
  commands, the fixture it needs, and what must be set for it to work. A
  generator inserts the command — and where it is short and stable, the real
  output — into a marked block in the page, and the gate refuses a block that
  disagrees with the recorded run. Per document rather than one global file
  because a single file that every page depends on is a merge conflict with a
  schedule.
- **J13-C, `docs/configuration.md`.** A use-case example for each section that
  has one: moving a library to another disk, putting the service behind a proxy,
  turning the search sidecar on, allowing a remote-media domain. Each says which
  other keys must be set with it, and each runs.
- **J13-D, replace what was never verified.** The operations, installation and
  publishing recipes become generated executed examples, or keep a reason that
  names where they *are* executed. Synopses stay synopses and are exempt by
  classification rather than by being quietly excused.

**Boundaries.** No example is published that has not run, unless the registry
carries a reviewed reason — the rule G18d already enforces, extended to the
generated ones. Generation must not rewrite prose: the generator owns marked
blocks and nothing else, as `cmd/docconfig` and `cmd/docplan` do. Nothing
probabilistic decides whether an example is correct.

**An open decision, stated rather than assumed.** Whether this extends
`cmd/docgen` or becomes `cmd/docexamples`. `docgen` already generates into these
pages and would keep one generator; an example set needs fixtures and a
sandboxed run, which is `docexec`'s machinery and not `docgen`'s. The plan
assumes a separate command that reuses `docexec`'s adapters, because the thing
that runs examples and the thing that renders doc comments have nothing in
common but a destination — but this is worth deciding before J13-B rather than
during it.

**Dependencies.** J12, for the configuration page the examples go in.

**Working state.** A per-document example set, a generator whose output is
gated against the recorded run, a configuration page whose sections each show
what to run and what else to set, and a published example count where "executed"
is the default and every exception names its reason.

### J13-A: the examination, and how it corrected the plan above

The plan's own estimate was wrong, and the classification is why that is now
known. It said the population deserving replacement was "the six that look
runnable", inferred from the `illustrative-placeholder` reason code. Classifying
the blocks themselves gives **116 recipes and 45 synopses**, and crossing that
with the registry gives **54 unverified recipes**, not six. The reason code was
the wrong instrument: a literal recipe can be unverified for `shared-user-state`
or `host-installation` just as easily.

The part of the estimate that held is the part that mattered: **no synopsis is
executed, and none should be.** 45 synopses, zero executed. That is the
classification's own consistency check rather than a happy result — a block
with a metavariable cannot have run — and the validator asserts it.

**The 54 are still not one problem.** By recorded reason: 19
`shared-user-state`, 17 `host-installation`, 9 `external-network`, 4
`interactive-or-long-running`, 3 `illustrative-placeholder`, 2
`privileged-host-change`. A recipe that installs a package as root or deletes a
library cannot run in a test and runs in the container matrix or the drills
instead. J13-D's target is narrower than "the 54".

**Ten recorded reasons contradict their own block,** which only a cross-
reference finds. Seven synopses are excused as `interactive-or-long-running`,
`shared-user-state` or `external-network` — a synopsis cannot be run at all, so
none of those is why it was not run. Three literal recipes are excused as
`illustrative-placeholder`: a runnable block filed as decoration, one each in
`docs/api/rest.md`, `docs/installation.md` and `docs/operations.md`.

**Two gaps, in the words the evidence supports.** `docs/configuration.md` has
53 settable keys and 0 examples, which is J13-C. And **61 of 92 commands have no
executed *published* example** — the wording narrowed after the first draft
called them "never shown working", which was false: most are exercised by
`cmd/notriosctl`'s tests and by the I4, I7 and J3 drills. The claim is about
documentation only.

### J13-D, the ten contradictions and the gate that keeps them at zero

All ten are fixed, and the count that found them is now a gate: **no recorded
reason may contradict its block.**

**Seven were synopses** excused as `interactive-or-long-running`,
`shared-user-state` or `external-network` — none of which can be why a synopsis
was not run. Two details were describing a different block entirely:
`cli-jobs-example-1` was recorded as "literal body starts or controls a daemon",
and those commands report on jobs and start nothing. They are
`illustrative-placeholder` now, and the secondary fact moved into the detail
rather than being discarded: for five of the seven both were true, so the code
names the reason that applies first and the detail names the one a reader wants,
with where the concrete form does run.

**Two literal recipes had nowhere honest to go,** so the closed reason set in
`internal/docaudit` gained two codes rather than stretching one.
`sha256sum -c SHA256SUMS` has nothing placeholder in it — it is exactly what a
downloader runs, and what it lacks is a published release set this repository
does not contain and deliberately does not fabricate:
`artifact-not-in-repository`. The REST block is literal after substitution and
unrunnable as one block because the same `$DOC` must be both active and in
Trash, which one fixture cannot be: `incompatible-prerequisites`. The set stays
closed, because a free-text reason is a place to put "later".

**One was the document's fault, not the record's.**
`operations-garbage-collection-example-1` was excused for mixing in
"illustrative `/safe/snapshot` paths", and `/safe/snapshot` reads like a path
somebody could type and is not one. The page changed instead of the label — it
is `<snapshot-dir>` now, in four places in `docs/operations.md` and two in
`docs/cli.md` — so the block is honestly a synopsis and its code is true of it.

**The unverified-recipe count went 54 to 53, and that is the honest figure.**
J13-A's point was that the number was never the target: 19 are
`shared-user-state`, 17 `host-installation`, 9 `external-network` — blocks that
install as root, delete a library or need a remote peer, each running in the
container matrix or the drills and saying so. Converting them would mean faking
the environment or moving the drills into the documentation.

**And it confirmed what J13-B is worth.** Changing three hand-written fences
meant three registry hashes to move by hand — the work `cmd/docexamples` removes
for the one page it owns. They came from `cmd/docaudit --list-executables`, the
owning tool's own scan rather than a second opinion. Migrating the remaining
pages to the tracked set is the natural follow-on, and this is the evidence for
it.

### J13-C, done before the generator so the format has a real customer

Four executed examples and one that says why it cannot be. Each writes a small
file with the keys that must be set *together* and runs the command that proves
they took effect, which is the half a key table structurally cannot express:
`data.directory` alone moves where new roots default and leaves an existing
library where it was; `server.listen_addr` and `public_base_url` have to
disagree for a proxy to work; `search_sidecar.enabled` alone has nothing to run
and nowhere to index; an `allowed_domains` list needs a `default_action` to be
an exception to.

**The check is derived from the example rather than restated beside it.**
`config-show-origin-file` reads the fence's own heredoc, collects the keys it
sets, and requires each back from `config show` with **origin `file`** and the
written value — so an example cannot claim a key it does not set. `origin` is
the column that carries the weight, because a value alone could be the compiled
default agreeing by accident, and in the proxy example it *is* the default.
Proved by dropping `--config` from one example and watching it name the key.

**It refuses an example that checks nothing,** which matters because `config
show` summarises 20 of the 53 keys: a fence setting only unreported keys would
otherwise pass with zero assertions. Proved by making one do exactly that.

**The fifth example is the honest one.** Search page limits get a fragment and
no command, because there is no command that would check it — `config show` does
not report those two keys and `notriosctl search` takes `--db` and not
`--config`. The first draft of this slice published
`config show --config bigger-pages.yaml` there, which would have been an example
that appears to verify something it does not: the exact defect J13 exists to
remove, caught only by reading the output instead of assuming it.

**Three requirements for J13-B that an imagined design would have missed:** the
fixture must support the idiom rather than the example being contorted to suit
the fixture (these need `cat` and a heredoc, so one coreutil joined the fixture
PATH); the postcondition should be read from the example body, because
restating it in Go is a second copy that can drift; and a verification command
has to be run against real output before publication — two of five candidates
were unverifiable and one was silently so.

### J13-B, the tracked set and the generator

`docs/docexamples/configuration.json` is the source of the page's five command
blocks, and `cmd/docexamples` publishes them. What it tracks is the **intent** —
the use case, the keys that must be set together, the command that checks them —
and the generator renders the heredoc *and* the command from that. So the
example sets exactly the keys the source names by construction, J13-C's
postcondition reads the rendered YAML back and requires each key from
`config show` with origin `file`, and there are two independent derivations from
one declaration with a real check of the product between them. Storing a body
would have made the tracked set a second copy of the fence.

**It moves the registry hash, because I kept doing that by hand.** Twice in
J13-C a fence changed and `registry.json` had to be re-hashed manually — right
both times, one keystroke from wrong both times, and a stale hash fails
`internal/docaudit` as a mystery rather than an instruction. The edit is
textual, which was a correction: the first version decoded and re-encoded the
registry, and since that sorts every object's keys, five hashes would have
rewritten all 169 entries into a diff nobody could review.

**Confirmed by breaking it in both directions** — a fence edited in the page, and
the tracked set changed with nothing regenerated (page *and* registry both
reported stale). The renderer refuses six shapes it cannot render honestly, and
a test asserts the markers wrap exactly one fence, because a marker inside the
fence would be published as part of the command.

**Two things checked rather than assumed.** The renderer reproduced the committed
fences byte-for-byte on its first complete run and its hashes matched the
registry with no changes — it was not adjusted to match, it matched. And the
markers are HTML comments that do not reach the published page: zero
occurrences in `_site/configuration.html`, with `g18g-validate` passing.

Only `docs/configuration.md` is generated. Migrating the rest is J13-D, and
doing it now would mean designing the migration against fifteen pages before the
format had survived one.

**One self-inflicted find worth keeping:** the remote-media example's closing
fence ended up with prose glued to it, so Hugo swallowed the last three sections
of the page into a code block and `retention`, `profiles` and `every-key` got no
anchors. `make g18g-validate` refused with `broken fragment
configuration.html -> #every-key`, which is the only gate that would have caught
it — `make validate` does not build the site.

**The extraction is trusted because it is cross-checked, not because it looks
right.** Blocks come from `docs/docaudit/registry.json` and their bodies are
matched to it by sha256; one mismatch refuses the whole report. That fired twice
— once because the slug function was not `docaudit`'s (it removes `.` rather
than replacing it, so `upgrading-from-before-08`, and the obvious regex lost an
id), and once deliberately as a probe. The metavariable rule took three drafts,
each false positive a real pattern here: a Markdown link inside a JSON payload,
a jq filter, and a JSON array whose `[` sits outside the quoted key.

## J14. Stop leaving bytecode behind, and derive the evidence index — complete

**Goal.** Running the repository's own Python leaves nothing behind, and the
README's Validation section stops being a third hand-maintained list.

### The bytecode caches, measured before deciding

19 `__pycache__` directories and 48 `.pyc` files were in the tree. Deleting all
of them broke nothing — `make validate` passed from cold and regenerated 13
directories and 35 files in the process — so nothing depends on them and they
are pure byproduct of every validate run.

**Two claims about them are worth separating, because only one holds.** They are
noise, and `make clean` already removes them, so they accumulate between cleans
and turn up in `ls`. That is real. The sharper worry — *out-of-date bytecode
causing build issues* — does not apply to `__pycache__` in Python 3, and I
measured it rather than agreeing with it: an orphaned `__pycache__/mod.pyc`
whose source has been deleted is **not importable** (`No module named 'mod'`,
per PEP 3147), and a cache whose source has changed is invalidated and
recompiled. The hazard that shape describes is a legacy sibling `.pyc` next to
its source, which this repository does not have and `.gitignore` would not
track anyway.

So the fix is worth doing for the reason that survives inspection: nothing here
benefits from a bytecode cache, and not writing one is simpler than cleaning one
up. `PYTHONDONTWRITEBYTECODE=1` stops it, and `PYTHONUNBUFFERED=1` belongs
beside it for a different reason — every one of these scripts prints progress a
person reads while waiting, and buffered output through a pipe arrives in a
block at the end, which is why a long validate can look hung.

### The Validation section, the same failure as the project status

It runs three unrelated things into one paragraph: which arguments the profile
scripts accept, where committed reference evidence lives, and which documents to
read before tagging. The middle one has drifted exactly as the status paragraph
had: **77 `performance/` directories exist and 13 are named**, and the list
covers v0.3, v0.4 and v0.5 while v0.7, v0.8, v0.8e, v0.9 and v1.0 — the large
majority of the evidence — go unmentioned. It is a list of directories on disk,
which makes it derivable, which makes it J12's lesson arriving a third time.

**Scope.**

- **J14-A.** Every place the repository invokes Python exports
  `PYTHONDONTWRITEBYTECODE=1` and `PYTHONUNBUFFERED=1` — one `export` in the
  Makefile reaches every recipe, and the entry shell scripts
  (`validate-scaffold.sh`, `package_release.sh`, the drill runners) need their
  own.
- **J14-B.** A check that a validate run leaves no bytecode behind, so J14-A
  stays true. Reporting, not gating, for the same reason `docs-stale` reports:
  a byproduct in an ignored path is not a reason to refuse a commit.
- **J14-C.** The Validation section is split into how to validate, what the
  profile arguments accept, where the evidence is, and what to read before a
  release — with the evidence index generated from `performance/*/` and gated,
  like the project status block.

**Also owed, found while reading:** the `make clean` line J12 added to the
README understates what it removes. It lists `bin/ dist/ _site/ web/dist/
.playwright-mcp/` and omits that the same target already deletes every
`__pycache__` and `.pyc` in the tree — which is the fact a reader asking "how do
I get rid of these" most needs.

**Boundaries, with one changed deliberately rather than quietly.** The plan
said *nothing is added to `make validate`*. That is right for the byproduct
report and wrong for the source check, and the two are different in kind: a
leftover `__pycache__` is a fact about this machine right now, while "every
entry point exports the variable" is a fact about tracked files — the same kind
`check_required_files.py` already asserts in that script. So `--sources` is a
gate in `make validate` and `--byproducts` is `make bytecode`, and this sentence
is the record of changing my own boundary instead of stretching it.

The generated evidence index lists directories and does not describe them; a
directory's contents are its README's business.

**Dependencies.** J12, whose generated-block machinery this reuses.

**Working state.** A validate run that leaves no bytecode, a Validation section
whose evidence index cannot drift, and a `make clean` description that is true.

### What it took

**The exports go in 20 shell scripts, the Makefile and three workflows**, and
the Makefile's `export` reaches every recipe and every script a recipe calls.
Proved rather than assumed: every cache was deleted, a complete `make validate`
run, and `make bytecode` reports **no bytecode in the tree** afterwards.

**The gate exists because 20 hand-edited files is where the 21st is forgotten,**
and it was confirmed by breaking it three ways — a script missing the export, the
Makefile missing one of the two variables, and a workflow missing one — each
named precisely. Scripts that only *mention* python in a comment are not
required to export anything, so the check strips comments before deciding.

**The workflows get it for a different reason, said in each file:** a runner is
discarded after the job, so bytecode on it harms nothing. Unbuffered output
means a failing step's log ends where the failure happened rather than wherever
the buffer flushed.

**The evidence index generates 77 directories grouped by milestone** — counts
and one example each, not 77 names, because a reader wants to know the evidence
exists and roughly how much there is; `ls performance/` does the rest better.
Gated, and confirmed by editing both a milestone count and the total.

**The new `make bytecode` target tripped I8's frozen surface** (35 lifecycle
targets to 36), re-recorded deliberately: a reporting target is compatible,
nothing renamed or removed.

## J15. Migrate the remaining documents to the tracked example set

**Goal.** A published command line is generated from a tracked declaration
wherever generating it earns its keep, and where it does not, the plan says so
in writing instead of leaving a reader to wonder why one page is different.

**Why this is a separate item.** J13 built the mechanism and used it on one
page. It never promised the other fifteen, and saying "the natural follow-on"
in a summary is not the same as planning it — this item is the correction of
that.

**Measured, 2026-09-12.** 169 published examples across 16 documents. **5 are
generated from a tracked set; 164 are hand-written fences** whose registry
hashes are moved by hand when they change.

| Document | Examples | recipes / synopses |
|---|---|---|
| `docs/cli.md` | 56 | 17 / 38 |
| `docs/api/rest.md` | 26 | 26 / 0 |
| `docs/installation.md` | 24 | 24 / 0 |
| `docs/operations.md` | 20 | 12 / 7 |
| `docs/import-export.md` | 12 | 12 / 0 |
| ten others | 31 | — |

**The format only models one kind of example, and that is the real work.**
`cmd/docexamples` renders a configuration file and the command that checks it:
its tracked entry is a list of `key`/`value` settings. It cannot express a
`curl` call with a JSON body, a multi-step CLI recipe, or a command synopsis. So
"migrate the remaining pages" is not a mechanical conversion — it is adding
example *kinds*, and each kind has to earn the indirection.

**Where generation earns it, and where it does not.** This is the judgement the
item exists to make rather than assume:

- **It earns it when the example encodes an agreement** — "these keys must be
  set together", "this flag requires that one" — because the declaration is
  then the claim, and the rendered text cannot disagree with it. That is why
  `docs/configuration.md` went first.
- **It earns it when fences churn.** J13-D changed three fences and had to move
  three registry hashes by hand, right each time and one keystroke from wrong
  each time.
- **It does not obviously earn it for a one-line literal** like
  `sha256sum -c SHA256SUMS`, where the fence is already the clearest possible
  statement and a JSON description of it would add a second file to read for no
  gain. A page of those should stay as it is, with the reason recorded.
- **It does not apply to synopses at all.** 38 of `docs/cli.md`'s 56 examples
  are command synopses, which exist to show the *form*; generating a form from
  a declaration of the form is a tautology with extra steps.

So the honest target is not 164 of 164.

**Scope.**

- **J15-A.** The example kinds the other documents need — at least a shell
  recipe and a REST request — added to the tracked schema and each proven on one
  page, with the same discipline J13 used: the check derived from the
  declaration, and the generator moving the registry hash.
- **J15-B.** Migrate document by document, in the order the measurement above
  suggests, and **record per document why it was migrated or why it was left**.
  A document left alone with a reason is a finished decision; a document left
  alone silently is the thing this item is fixing.
- **J15-C.** No migrated page's fence can be hand-edited without a failure, and
  no migrated page's hash is moved by a human. `cmd/docexamples`' test already
  does this for one page; it should hold for every page it owns.

**Boundaries.** No page is migrated to raise a count. A synopsis stays a
synopsis. Where the declaration would be longer and less clear than the fence,
the fence stays and the record says the comparison was made — the same rule
J13-A used to keep good synopses from being deleted to improve a statistic.

**Dependencies.** J13, for the mechanism and the classification the order comes
from.

**Working state.** Every document either generated from its tracked set or
carrying a recorded reason it is not, and no hand-moved registry hash left on a
generated page.

## J16. Give the carrier write its own path shape

**Goal.** Every carrier path means one thing, and the API description can name
its parameters honestly.

**Why.** J4's D1. `GET /api/v1/sync/carrier/{namespace}/{class}` and
`PUT`/`DELETE /api/v1/sync/carrier/{class}/{name}` are both two segments after
`carrier`, so they collapse onto a single OpenAPI path whose parameters are
called `{segment1}` and `{segment2}` — because no honest name exists for a
parameter whose meaning depends on the verb. A client generated from that
contract has methods taking `segment1` and `segment2`.

**What is not changing, and why the shape is the only thing to fix.** A carrier
namespace is `HMAC(routing key derived from the group key, "replica" ‖
replica_id)`. Nobody chooses it: only an enrolled group member can name one at
all, and the carrier host cannot tell whose it is. Writes derive it from the
authenticated principal because **a replica must not publish as another one** —
a caller-supplied namespace on a write would forge segments attributed to a
peer. That property stays exactly as it is. This item changes the URL, not the
authorization.

**Scope.** Give the write an unambiguous path whose extra segment is a
**literal** rather than a parameter — `PUT`/`DELETE
/api/v1/sync/carrier/mine/{class}/{name}` — so the list and the write no longer
share a shape. Then `api/openapi.yaml` describes two paths with real parameter
names instead of one with `segment1`/`segment2`. Update the handler, the client
in `notriosctl sync exchange`, `docs/api/rest.md`, I8's frozen REST surface, and
J4's review.

**Why now.** It is a breaking change to the sync wire, and **there is nothing to
break**: J10 has not published a release, so no peers exist outside this
repository. After 1.0 the same change costs a migration; today it costs a
rename. That asymmetry is the entire argument for doing it in this milestone,
and it expires when J10 runs.

**Boundaries.** The old two-segment write path is removed rather than kept
alongside. Keeping both would mean shipping the ambiguity permanently in order
to be compatible with peers that do not exist. No authorization behaviour
changes: the namespace is still derived from the caller and never read from the
URL.

**Dependencies.** J4, which found it. Must land before J10.

**Working state.** Two carrier paths with one meaning each, an OpenAPI
description with honest parameter names, a re-recorded frozen surface, and a
round trip through `notriosctl sync exchange` proving peers still talk.
