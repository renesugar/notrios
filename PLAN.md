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
**36 items: 32 complete, 0 in progress, 4 not started, 0 deferred.**

| Item | State | Slices done | Outstanding |
|---|---|---|---|
| J1. Build the package in a workflow, and attest what it built | complete | 3/3 | — |
| J2. Create the release signing key, and sign what ships | complete | 3/3 | — |
| J3. Give a packaged installation a supported way to delete its data | complete | 5/5 | — |
| J4. Stabilise the REST and MCP surfaces for 1.0 | complete | 3/3 | — |
| J5. Prove the library at scale | complete | 3/3 | — |
| J6. Ship the versioned no-GUI library and header artifacts | complete | 3/3 | — |
| J7. Validate backup, export, restore, sync compatibility and disaster recovery | complete | 3/3 | — |
| J8. Security review for remote media and MCP | complete | 3/3 | — |
| J9. Publish the release documentation for the supported matrix | not-started | 0/3 | 3 |
| J10. Publish the user-authorized release | not-started | 0/3 | 3 |
| J11. Report the installation's structure and manifest, and verify a purge against it | complete | 3/3 | — |
| J12. Make the README true, and generate what can be generated | complete | 4/4 | — |
| J13. Generate the published command-line examples from executed runs | complete | 4/4 | — |
| J14. Stop leaving bytecode behind, and derive the evidence index | complete | 3/3 | — |
| J15. Migrate the remaining documents to the tracked example set | complete | 4/4 | — |
| J16. Give the carrier write its own path shape | complete | 3/3 | — |
| J17. Batch the per-item work J5 found in import and export | complete | 3/3 | — |
| J18. Stop scanning the full-text index on every document write | complete | 3/3 | — |
| J19. Test the external performance review, and adopt only what measures better | complete | 3/3 | — |
| J20. Finish the Obsidian inventory memory work, on a fresh J5 baseline | complete | 3/3 | — |
| J21. Stop re-running schema migrations every time a library is opened | complete | 3/3 | — |
| J22. Stop stores and tests leaving directories in the temp root | complete | 3/3 | — |
| J23. Keep existing sync peers syncing after both upgrade in place | complete | 3/3 | — |
| J24. Check the running agent's own usage, not every agent's | complete | 3/3 | — |
| J25. Import a Twitter/X archive as it is downloaded, completely, at its real size | complete | 3/3 | — |
| J26. Import ChatGPT, OpenAI Privacy Portal and Claude archives as downloaded | complete | 6/6 | — |
| J27. Normalise a host before matching it against the domain lists | complete | 1/1 | — |
| J28. Refuse the reserved address ranges remote media must not reach, by a named default the configuration can state | complete | 4/4 | — |
| J29. Decide which HTML reference forms the remote-media scanner is responsible for | complete | 1/1 | — |
| J30. Stop a lying Content-Type header deciding the type of an inconclusive payload | complete | 1/1 | — |
| J31. Bring the vendored Ledger theme up to its Bluge result-URL fix | complete | 3/3 | — |
| J32. Investigate the import performance review's findings, and keep only what measurement shows is faster | not-started | 0/14 | 14 |
| J33. Decide whether Ogg media and comment-led SVG are localizable | complete | 1/1 | — |
| J34. Stop the preview loading remote images through media elements | complete | 1/1 | — |
| J35. Make G18g's browser smoke runnable again | complete | 3/3 | — |
| J36. Resolve Obsidian partial-path links the way Obsidian does | not-started | 0/3 | 3 |

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

## J5. Prove the library at scale — complete

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

## J6. Ship the versioned no-GUI library and header artifacts — complete

**Goal.** A third party can build against the C ABI without this repository.

**Scope.** Versioned library and header artifacts for the supported platform
matrix, with lifecycle, ownership, threading, error, stream and compatibility
examples.

**What exists already.** `cmd/notrioslib` builds the version-1 ABI with
`-buildmode=c-shared`, and `run_host_test.sh` proves the exported set is exactly
the frozen twelve symbols, that no SQLite symbol escapes, that there is no
dynamic SQLite dependency, and that a C host driving only `notrios_abi.h`
works. What is missing is the artifact: a third party has to clone this
repository and run that script.

**Design, 2026-09-19.** Written before the code.

- **The artifact** is one tarball per supported platform,
  `notrios-c-abi-<version>-<os>-<arch>.tar.gz`, holding:
  - `lib/libnotrios.so.1` with `libnotrios.so` beside it as a link;
  - `include/notrios_abi.h`;
  - `LICENSE`, and a `README.md` saying what the ABI promises, what it does
    not, and which platform this artifact was built and tested on;
  - `SHA256SUMS` over the files, so a downloader can check them without this
    repository.
- **Two versions, both stated.** The tarball carries the product version; the
  library carries ABI major 1 in its soname (`libnotrios.so.1`), which is what
  a linker records and what a later incompatible ABI would change. The header
  asserts the pair, so a header and a library from different builds cannot be
  mixed silently.
- **Built by the workflow that attests it** (J1). The release workflow builds
  the tarball beside the `.deb`, attests it with the same action, and uploads
  it as a run artifact. **It publishes nothing**: publication is J10's, under
  the owner's authorization.
- **The examples are the acceptance test.** Six programs under
  `examples/c/`, each one thing a caller must get right: lifecycle, handle
  ownership, threading, errors, streams, and compatibility (an older header
  against a newer library). They compile against the **extracted artifact**,
  not the source tree, because compiling against the repository would prove
  the one thing a third party cannot do.
- **The supported matrix is I7's**, so the artifact is built and tested for
  Ubuntu 24.04 amd64 and nothing else is claimed. A second platform appears
  when a machine runs it, not because a cross-build succeeded.
- **The baseline** (`performance/v0.9-i8`) is re-derived, not edited. A soname
  is a link-time record, not an exported symbol, so the twelve members should
  be unchanged; if the freeze moves, J6-C records what moved and why.

**Boundaries.** The matrix is v0.9 I7's, which claims only what ran: Ubuntu
24.04 amd64. An Android-emulator result is not physical Android support and is
not labelled as such; no iOS artifact is implied. Nothing is published or
uploaded to a release: J6 builds and proves the artifact, J10 publishes it.

**Dependencies.** J1, for the workflow that builds and attests artifacts.

**Working state.** Downloadable library and header artifacts, examples that
compile against them, and the ABI baseline in `performance/v0.9-i8` unchanged or
re-recorded with a reason.

## J7. Validate backup, export, restore, sync compatibility and disaster recovery — complete

**Goal.** Data written by one version comes back through another, and a lost
library is recoverable.

**The versions, by owner decision (2026-09-15).** The repository has no release
tags, so the set is taken from the project's own record:

| version | commit | schema | why it is in the set |
|---|---|---|---|
| 0.7.0 | `c1127f1` (2026-08-31, G20 release acceptance) | 27 | the only product release G20 recorded |
| 0.8.0 | `10e7077` (2026-09-09, H13 closes v0.8 at 0.8.0) | 27 | the most recent labelled release |
| v0.9 close | `d6f7ca2` (2026-09-09, I9) | 27 | still labelled 0.8.0, and the last state before v1.0 work |
| historical archive writer | `5ae93df` (2026-08-04) | — | named by the archive contract's `previous-loose-v2` reader; archives only |
| 1.0 | the tree under test | 28 | |

All four historical commits build with today's toolchain, and each has
`import obsidian` and `export archive-v2`. That lets one generated vault (J17's
`j17_make_vault.py`) be written by every version.

**Two further decisions.**
- **A pre-1.0 version given 1.0's data must refuse it clearly, or restore it
  completely.** Partial or corrupt data is a failure. The rule was first framed
  as "refuse"; the owner revised it (2026-09-15) after J7-A measured all three
  older versions restoring a 1.0 archive with identical content, because schema
  28 adds only a private index an archive does not carry.
- **0.7.0 opening a 1.0 library is a documented 0.7.0 limitation.** 0.7.0
  predates the too-new-database check. It opens a 1.0 library and rewrites
  `user_version` from 28 to 27 with content intact, and 1.0 recovers it on the
  next open. 0.8.0 and v0.9 refuse. 1.0 cannot change 0.7.0, so the release
  documentation (J9) says not to do it, and that reopening with 1.0 recovers.
- **Sync between pre-1.0 and 1.0 does not happen, by any path; replicas upgrade
  first.** J16 moved the REST carrier write path, from
  `PUT|DELETE /api/v1/sync/carrier/{class}/{name}` to
  `/api/v1/sync/carrier/mine/{class}/{name}`, and 1.0 does not bridge it. The
  first framing assumed pre-1.0 peers could still sync with 1.0 through a folder
  carrier. J7-B measured that they cannot:
  - every pre-1.0 version caps sync at schema 27 (range 24–27)
  - J18 moved 1.0 to 28
  - an older version refuses a 1.0 invite
  - when the older version invites, pairing completes but each side skips the
    other's artifacts, so nothing moves

  The owner decided (2026-09-15) that the clean refusal in both directions is
  the pass. The supported path is to upgrade every replica to 1.0, which J7-B
  tests with replicas that were already syncing. The release documentation (J9)
  says so.

**Scope.**

- **J7-A, archives and library files across versions.** Each historical version
  imports the generated vault and exports an archive-v2. 1.0 then:
  - verifies and restores that archive, and compares the content with a 1.0
    import of the same vault
  - opens the historical version's library file directly, which migrates it to
    v28, and compares it the same way

  In the other direction, each historical version is given a 1.0 archive and a
  1.0 library. The pass is a clear refusal, or a complete restore with content
  equal to the 1.0 library.
- **J7-B, sync across versions.** A second replica is made from the first by
  `export archive-v2` and `restore --intent adopt`, as the documentation
  describes. It is paired offline (`invite --offline`, `accept`, `enroll`),
  and `sync once` runs through a shared folder between 1.0 and each historical
  version, in both directions. The pass, by owner decision, is a clean refusal
  both ways: nothing moves and nothing is corrupted.
  `upgrade_in_place.sh` then tests the supported path. Two replicas already
  syncing on each historical version are upgraded to 1.0 one at a time, and
  must:
  - move nothing while their versions differ
  - converge again on their original pairing once both are upgraded

  The REST path is checked with real servers to confirm it refuses too.
- **J7-C, disaster recovery at J5's scale.** A 382,206-note library is
  exported, then destroyed through the J3 purge path, with its verified backup.
  It is restored, then compared with the original: counts, a content digest, and
  J5's search probes. Each step is timed.

**Boundaries.**
- Compatibility claims name the exact commits above and nothing else.
- A drill that does not run to completion is reported as incomplete, with what
  it reached.
- The cross-version runs use the generated vault. Only J7-C runs at full size.
  No subset result is stated as a full-corpus one.

**Dependencies.** J5, for the scale; J3, for the purge path a recovery follows;
J16, for the REST write path; J21, whose open path the v27-to-v28 migration
takes. **J23, by owner decision (2026-09-15):** J7 stays open until J23 fixes
the defect J7-B found. Replicas upgraded in place stop syncing with each other,
and J7-B is not done until `upgrade_in_place.sh` converges for 0.7.0, 0.8.0 and
v0.9. J7-C can complete and be recorded first.

**Working state.**
- A recorded matrix: for each historical version, each direction, and each of
  archive, library file and folder sync, a pass or a clear refusal
- The REST break shown and documented
- A 382,206-note library destroyed and restored with its content proven equal

## J8. Security review for remote media and MCP — complete

**Goal.** The two surfaces that reach outward have been examined by someone
looking for the failure rather than confirming the design.

**Scope.** Remote-media localization — domain policy, quarantine, hashing,
MIME sniffing, size limits, SSRF protection — and the MCP surface, its tool
scopes and what a client can reach through it.

**What this item produces, by owner decision (2026-09-16).** J8 **records
findings and changes no product code.** A finding that needs a code fix becomes
its own plan item, named in the record and left for the owner to approve. This
keeps a security fix from being written by the same pass that found it, in the
same hurry, and keeps the review's output honest: what was tried, what happened,
and what is owed.

- **J8-A, remote media: attempt the failures, record what happens.** Each
  attempt below is run against the shipped pipeline and recorded as refused,
  admitted, or not covered:
  - **Addresses the private-range check may miss.** `ip.IsPrivate()` does not
    cover carrier-grade NAT (`100.64.0.0/10`), IPv6 unique-local (`fc00::/7`),
    or IPv4-mapped IPv6 forms of loopback and private addresses
    (`::ffff:127.0.0.1`), at the static check and at the connect-time check.
  - **DNS rebinding**: a name that resolves to a public address for the policy
    check and a private one at connect time.
  - **Redirect chains**: a hop whose own verdict is block or review, a hop to a
    private address, and more hops than the configured limit.
  - **Host forms**: userinfo (`https://allowed.example@evil.test/`), a trailing
    dot, uppercase, and an IPv6 literal in brackets.
  - **Lying servers**: an `image/*` header over HTML bytes, a sniff-inconclusive
    type, a `Content-Length` smaller than the body, and a body that exceeds the
    class cap only after the first chunk.
  - **Quarantine**: where bytes land, under what permissions, what names them,
    and whether anything reaches the resource store without admission.
  - **Provenance**: whether original URL, final URL, content type, hashes and
    the policy decision are recorded for both refusals and admissions.
- **J8-B, MCP: what a client can actually reach.**
  - Every registered tool is classified, and the scope is enforced on **every**
    dispatch path, not only the ones with a test today.
  - A tool outside the active scope is absent from `tools/list` and from the
    info endpoint, as well as refused when called.
  - The sync scope is orthogonal: a wide MCP scope does not grant sync control.
  - What the **default** scope reaches is written down, tool by tool.
  - The standing constraints hold: no raw SQL, no credential surface, and the
    destructive whole-library operations are unreachable over MCP.
- **J8-C, dispositions.** Every finding carries one:
  - **accepted**, with the reason it is acceptable, or
  - **deferred**, to a named new plan item for the owner to approve.

  No finding is left without one, and none is fixed here.

**Boundaries.**
- **No package under `internal/` or `cmd/` changes in this item.** The review's
  probes live under `performance/v1.0-j8/`.
- Probes reach the network only over loopback, against a local test server.
- A review records what it examined and what it did not. Nothing here becomes a
  claim that the surfaces are secure.
- No finding is written up with an exploit recipe; each names the gap and what
  it would take to close it.

**Dependencies.** J4, so the MCP surface being reviewed is the one 1.0 ships.

**Working state.** Recorded findings, each accepted with a reason or deferred to
a named plan item, and the probes that produced them.

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

## J11. Report the installation's structure and manifest, and verify a purge against it — complete

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

**Design, 2026-09-18.** Written before the code.

- **The command is `notriosctl paths report`.** `paths` is already the answer to
  "where is this instance keeping things?", and this is the same question asked
  exhaustively. It reuses `paths.ForProcess` for the roots, the profile registry
  for every library including the external ones, and the installer's
  `MANIFEST.json` for the program files when it is present.
- **Three output shapes**, because the report has three readers:
  - the default report, for a person, redacted to `~`;
  - `--json`, for a script, with the same content structured;
  - `--paths`, one absolute path per line, which is what the post-purge check
    reads. It is unredacted by definition — a check compares real paths — so it
    refuses to be combined with redaction rather than emitting `~`.
- **What the manifest lists.** Every directory that holds Notrios files or
  data, and every file under those directories, by absolute path. A large
  library makes a large manifest; that is what an oracle for "is it all gone"
  costs, and the report says how many entries it holds.
- **Every entry says whether Notrios owns it.** A registered profile whose
  database or asset store lives outside the owned roots is listed and marked
  external, because a purge deliberately keeps it. The post-purge check must
  therefore require the owned entries to be gone and the external ones to still
  be there — if an external library disappeared, that is a failure too, and the
  check says so.
- **The check is `scripts/check_purged.sh`**, reading a `--paths` manifest
  taken beforehand. It needs nothing but a shell, because everything Notrios
  installed is gone by the time it runs, including anything that could parse
  JSON on its behalf.
- **Adding a command moves the frozen CLI surface** (I8, 92 members), which is
  additive, and `internal/clispec/commands.json`, the generated
  `docs/cli.md` and the help output come with it.

**Boundaries.** The report names paths; it does not print note content, and it
redacts the home directory to `~` by default like `paths`, `config show` and —
since v0.9 I7 — `doctor`, with `--no-redact` for the literal form. A manifest is
a list of where things are, and that is exactly what somebody pastes into an
issue. `--paths` is the exception, and only because its one reader is a script
comparing real paths.

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

## J15. Migrate the remaining documents to the tracked example set — complete

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

**Measured again, 2026-09-19.** 172 published examples across 16 documents,
and one measurement changes the design this item sketched:

| document | examples |
|---|---|
| `docs/cli.md` | 56 |
| `docs/api/rest.md` | 26 |
| `docs/installation.md` | 26 |
| `docs/operations.md` | 20 |
| `docs/import-export.md` | 12 |
| eleven others | 32 |

**Not one of `docs/api/rest.md`'s 26 examples is a single request.** Every one
is a shell recipe: a variable captured from a previous response, a pipeline
into `jq`, a comment explaining which status to expect, often several requests
that only make sense in order. A "REST request" kind modelling a method, a path
and a body would reproduce none of them, and rewriting the page into that shape
would be migrating a page to raise a count, which the boundaries forbid.

So the second kind is a **recipe**, and what makes it earn its keep is not
re-typing the commands in JSON. It is the declaration beside them: which CLI
commands and which REST routes this recipe uses. Those are checked against the
frozen surfaces — `internal/clispec` and the routes the server registers — so a
documented command that stops existing fails the build, which no fence can do
for itself. The commands themselves are still written once.

**Scope.**

- **J15-A.** A `recipe` kind added to the tracked schema and proven on one
  page, with the same discipline J13 used: the check derived from the
  declaration, and the generator moving the registry hash. A recipe declares
  the CLI commands and REST routes it uses; the generator refuses a command
  that is not in the CLI spec, a route the server does not register, and a
  declaration that names something its own steps never run.
- **J15-B.** Migrate document by document, in the order the measurement above
  suggests, and **record per document why it was migrated or why it was left**,
  in a tracked record rather than in a commit message. A document left alone
  with a reason is a finished decision; a document left alone silently is the
  thing this item is fixing.
  - **Migration must not change a published byte.** A page is migrated by
    declaring what is already there, and the proof is that the generator
    rewrites the page to exactly what it said before. A migration that
    improves the wording at the same time cannot be checked that way, so it is
    not done at the same time.
- **J15-C.** No migrated page's fence can be hand-edited without a failure, and
  no migrated page's hash is moved by a human. `cmd/docexamples`' test already
  does this for one page; it should hold for every page it owns.
- **J15-D.** The troubleshooting page's import rows describe the importers as
  they are. Found 2026-09-16, while checking the documentation site after a
  theme fix: `docs/troubleshooting.md`'s Importing table still quotes two errors
  the importers no longer produce, and gives advice J25 and J26 made wrong.
  - `no tweets.js/tweet.js found under ...` says to point `import twitter` at
    the **extracted** folder, "not the ZIP". Since J25 the ZIP as downloaded is
    the primary input, and the error now reads `no tweets.js, tweets-partN.js
    or tweet.js found under …; expected a Twitter/X archive ZIP or its
    extracted folder`.
  - `read ChatGPT export: ... no such file` says to pass `conversations.json`
    or its directory. Since J26 the ChatGPT, Privacy Portal and Claude export
    ZIPs are read in place, and none of their errors has that form.

  Each row is rewritten against the error text the importer actually returns,
  and the rows gain a check that fails when a quoted error no longer appears in
  the importer's source — the drift here went unnoticed because nothing tied
  the quote to the code. This is prose, not an example fence, so it is not
  counted in the measurement above and is not a migration; it is here because
  it is the same failure J15 exists to end, a published statement nothing
  keeps true.

**Boundaries.** No page is migrated to raise a count. A synopsis stays a
synopsis. Where the declaration would be longer and less clear than the fence,
the fence stays and the record says the comparison was made — the same rule
J13-A used to keep good synopses from being deleted to improve a statistic.

**Dependencies.** J13, for the mechanism and the classification the order comes
from.

**Working state.** Every document either generated from its tracked set or
carrying a recorded reason it is not, and no hand-moved registry hash left on a
generated page.

## J16. Give the carrier write its own path shape — complete

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

### What it took

Writes are `PUT`/`DELETE /api/v1/sync/carrier/mine/{class}/{name}`. The server
routes, the remote-peer allow-list in `internal/service` and the client in
`internal/syncrest` moved together, because moving any one alone leaves a peer
refused on one side. `api/openapi.yaml` now describes the listing and the write
under separate paths, so `{segment1}` and `{segment2}` are gone from the
contract, and `internal/docgen`'s special case for folding the two shapes is
gone with them. The frozen REST surface is re-recorded at 113 members with
exactly the two write routes moved, and J4's review reports every carrier route
described under its own path.

The impersonation test is stronger than before: it tries `PUT` and `DELETE`
against both the removed two-segment shape and the three-segment read shape
with another replica's namespace, and none may succeed. `internal/syncrest`'s
exchange tests round-trip through the real client and server on the new paths.
Record in `performance/v1.0-j16`.

## J17. Batch the per-item work J5 found in import and export — complete

**Goal.** No importer or exporter calls the store once per item where a batch
call exists, and the improvement is measured rather than assumed.

**What J5 found, and how firm it is.** The recipe corpus exists as both an
Obsidian vault and a Joplin RAW export, so the store does identical work either
way and the difference is the importer. The Joplin import of 382,206 notes took
**1.72 h**; the Obsidian import of the same notes took **4.52 h**, while reading
*fewer* files and *fewer* bytes.

The code says where to look, and the batch API already exists:

| importer | link rebuild |
|---|---|
| Joplin | `RebuildImportDocumentLinksBatch(ctx, {DocumentIDs: […]})` — 3,823 calls |
| Obsidian | `RebuildDocumentLinks(ctx, note.TargetID)` — **382,206 calls** |

`internal/importers/obsidian/obsidian.go` batches its *reads* — `GetDocuments`
over a slice — and then rebuilds links one document at a time inside that loop.
Its sibling calls the batch method for the same work.

**J17-A measured it, and the lead was wrong.** `TestJ17LinkRebuildTransactionCost`
runs both paths over the same 2,000 documents in a copy of J5's library:
per-document 9.547 ms, batched 6.320 ms, **ratio 1.51×**. Collapsing 500 commits
into one buys 34%, not 2.6×; a commit is worth ~3.2 ms of the 9.5 ms, where J5's
record had inferred ~27 ms by dividing the gap by the call difference — which is
arithmetic that can only agree with itself. The experiment that could disagree
took 138 seconds, and `performance/v1.0-j5` now carries the correction beside
the claim rather than instead of it.

**The lead that replaces it is structural and deliberately not yet a
conclusion.** The link rebuild is one per-document call among several:

| | Obsidian | Joplin |
|---|---|---|
| document write | `CreateDocument`/`UpdateDocument`, plus `SetDocumentSource` and `MoveDocumentToNotebook` — **per document** | `ApplyImportDocumentBatch` — **per batch** |
| link rebuild | `RebuildDocumentLinks` — per document | `RebuildImportDocumentLinksBatch` — per batch |

`CreateDocument` and `UpdateDocument` each open their own `BEGIN IMMEDIATE`. So
the item continues by measuring that pair the way the link rebuild was measured,
rather than by assuming the second theory because the first one failed.

**Scope.**

- **J17-A, prove the cause before fixing it.** *Five hypotheses tested, five
  rejected; the cause is still unknown and the method has changed.* Link-rebuild
  batching gave 1.51×, document-write batching gave 0.99×, directory
  concentration gave 1.07×, transaction count is falsified by the first two, and
  J18's full-text scan is never executed on the create path an import takes. A
  claim that the importer was superlinear was made on two points and withdrawn
  on four: per-note cost rises from 28.5 ms at 10,000 notes to ~40 ms by 40,000
  and is then flat to 382,206, so the gap is a **constant factor** of roughly
  40 ms against 16 ms, not a scaling defect.

  **The remaining work is a CPU profile of the Obsidian importer, and that is a
  deliberate change of method.** Reading a call site and testing it finds only
  causes somebody thought of, and it has been wrong five times in a row here at
  the cost of a measurement each. A profile answers "where does the per-note
  time go" without guessing first. No sixth call site is proposed in this plan,
  because proposing one would repeat the mistake.

  **The profile has run** (`TestJ17ImportRealVaultOnDisk`, a 10,000-note subset
  on J5's HDD, 28.7 ms/note, 69% CPU and 31% waiting). SQLite accounts for 70% of
  samples. Three facts come out of the call graph:
  - **Nothing is prepared once.** `execPreparedLocked` compiles its SQL on every
    call, and `sqlite3_prepare_v2` takes 30.7% of the CPU.
  - **Links and blocks are rebuilt twice per note**, in `CreateDocument` and
    again in the link phase. The Joplin path also does this, so it is waste but
    not the asymmetry.
  - **The Obsidian path commits at least three times per note.** Those commits
    come from `CreateDocument`, from `SetDocumentSource` and each attachment
    write (both autocommit), and from `RebuildDocumentLinks`. The Joplin path
    commits twice per 100 notes.

  The profile also reopens a rejection. The 0.99× document-write measurement
  ran while J18's 2 s full-text scan dominated every write, so it could not
  detect commit cost, and "transaction count is falsified" overstated it. That
  question is re-measured before anything is claimed about it.

  **Re-measured.** The same 10,000-note import with the library on tmpfs takes
  16.7 ms/note against 28.7 ms/note on the HDD. User CPU is unchanged (158.9 s
  against 161.8 s), and 132 s of waiting on the disk goes away, which is 42% of
  the HDD run. This makes per-note commits a strong lead. It is not proof: the
  run moved the whole library, not just the commits. The next measurement is
  the same import on the HDD with the commits batched, which J17-B must report
  at the size it measures.

  **J17-B measured it.** With the note and link phases on the batch store
  methods, the same 10,000-note import on the HDD takes **19.7 ms/note, against
  28.7 (1.46×)**. Wall time drops from 298.9 s to 198.1 s, and system time from
  43.0 s to 17.3 s. Batching only the commits recovers 101 s of the 132 s that
  moving the whole library to tmpfs recovered. User CPU rose 13 s, which is
  recorded as unexplained. *Corrected by J19-A:* that 19.7 ran while other work
  shared the machine. In clean sequential runs, the batched importer takes 13.1–13.3
  ms/note (three runs) against 27.1–31.1 for the per-note importer (two runs).
  That is **about 2.2×**, with user CPU falling about 25 s, not rising. The
  libraries are compared table by table against the old importer's:
  - on the 10k subset
  - on a generated 300-note vault with 1,055 links, 170 attachment references
    and forward references

  In both, they match everywhere except the random database and revision
  identifiers. The 382,206-note corpus has not been re-run, so no claim is made
  about J5's 4.52 hours. Separately, the 30.7% spent compiling SQL is CPU cost
  that persists on tmpfs, and it is a store-wide question for J19, not an
  importer asymmetry.
- **J17-B, batch what measurement justifies.** The link rebuild is worth
  batching on its own evidence — 1.51×, and the batch path additionally records
  a resumable checkpoint the per-document path does not — but it must be
  reported as a 1.51× improvement to one phase, **not** as the fix for a 2.6×
  gap. Whatever J17-A finds in the document write is batched on the same terms:
  measured first, claimed at the size measured.
- **J17-C, survey the rest of import and export.** The two importers and
  `internal/archivev2` for any other store call made once per item where a batch
  method exists. A per-item call with no batch equivalent is recorded, not
  invented — adding a batch API is a store change and belongs to whatever item
  needs it, not to a survey.

  **Surveyed.**
  - **Archive:** export reads through batched `Export*` calls, and restore writes
    through `ApplyRestoreRecords`. Nothing is per item.
  - **Obsidian and Joplin, notes and links:** batched in both importers.
  - **Obsidian and Joplin, no batch method in the store:** source-bundle files,
    notebooks, resources, and Joplin's tags stay per item. That is recorded,
    along with the counts: notebooks number in the hundreds against 382,206
    notes, and attachment cost is unmeasured.
  - **ChatGPT, Claude and Twitter:** each writes one document at a time.
    `ApplyImportDocumentBatch` requires a checkpoint and an item state per
    document, and these importers have neither, so adopting it would be a
    resumable-import design for each one. It is recorded without a cost claim.
    The table is in `performance/v1.0-j17/README.md`.

**Why this is not "make import faster".** Import time at this size is dominated
by work Notrios chooses to do — J5's comparison against `movenotes-v3` showed a
tool doing 2.83× better while building no full-text index — and nothing here
proposes to stop doing it. The target is the *asymmetry*: two importers, the
same store, the same notes, and one of them four and a half hours slower.

**Boundaries.** Correctness first: the Obsidian importer's link rebuild must
still produce the same link index, proven by comparing the two libraries rather
than by the import finishing. No batch size is raised to win a number — J5
recorded that `movenotes-v3` ran in one 1.4 GB transaction, which is fast and
loses everything on a crash, and Notrios's checkpointed batches are a
deliberate trade this item does not reopen.

**Dependencies.** J5, for the finding and the corpus.

**Working state.** A profile naming where the ~24 ms per note goes, whatever
follows from it measured before it is claimed, and a recorded list of any
remaining per-item calls in import and export.

**What this item has already produced, whether or not the gap is ever closed.**
Five eliminated causes, a corrected characterisation of the gap, and — found
while the second theory was being disproved — J18, which is a larger problem
than the one this item was created for.

## J18. Stop scanning the full-text index on every document write — complete

**Goal.** Writing a document costs what the document costs, not what the library
costs.

**The defect.** `documents_fts` declares `document_id UNINDEXED`, so FTS5 builds
no index on it, and `DELETE FROM documents_fts WHERE document_id = ?` plans as
`SCAN documents_fts VIRTUAL TABLE`. Every document write scans the whole
full-text index. **A write is O(library size).**

**Measured, on three libraries, two of them built by different importers from
different corpora:**

| library | notes | one `UpdateDocument` |
|---|---|---|
| generated | 60 | 156 ms |
| Joplin export | 103,349 | 490 ms |
| Obsidian vault | 382,206 | **2,087 ms** |

The scan is the whole cost: the `document_id` lookup alone on the largest
library takes **2,068 ms** of the 2,087 ms write.

**The fix is confirmed before being proposed.** Deleting by `rowid` on the same
library takes **19 ms** — about 109× — and needs a `document_id → rowid`
mapping, because the rowid is what FTS5 can find without scanning. Note that
`EXPLAIN QUERY PLAN` still prints `SCAN` for the rowid form, with an `INDEX 0:=`
suffix; a reader comparing plans rather than timings would conclude nothing had
improved.

**Ten call sites, so this is a pattern rather than a slow function.** Three in
`sqlite.go`, three in `sqlite_notebooks.go`, and one each in
`sqlite_import_batch.go`, `sync_retention.go`, `sync_ui.go` and
`sync_revision_apply.go`. The cost lands on updating a note, deleting one,
notebook operations, retention, sync apply and the import batch path alike.

**Why it is a 1.0 item.** `UpdateDocument` is what runs when a user saves an
edited note. At 382,206 notes that is two seconds and it grows with the library.
J5 recorded search at ~9 s on the same library; this is the write side of the
same story, and both are interactive.

**Why it is a migration and not a patch — the owner should weigh this.** The fix
changes the full-text schema and touches ten call sites across sync, retention
and notebooks. Existing libraries need the mapping populated, which is a
migration over every document. That is a larger change than anything else
outstanding in this milestone, arriving at freeze time, and the alternative —
shipping 1.0 with a two-second note save that worsens as libraries grow — is
worse. Recorded so the decision is made rather than inherited.

**Boundaries.** Correctness before speed: the full-text index after a write must
contain exactly what it contains today, proven by comparing search results
before and after on the same library rather than by the write being faster. No
other use of `documents_fts` changes shape in this item.

**Dependencies.** None. It is independent of J17, which is what found it.

**Working state.** A document write whose cost does not depend on the size of
the library, the same measurement re-run on the same three libraries beside the
old numbers, and a migration that populates the mapping for an existing library.

## J19. Test the external performance review, and adopt only what measures better — complete

**Goal.** Every suggestion in the performance review is either shown by
measurement to help and implemented, or shown not to and recorded — nothing is
adopted because it reads well.

**Source, and how it is treated.** A code review produced by
`openrouter/google/gemini-3.1-pro-preview` through `opencode`, kept at
`/home/renes/prompts/notrios_gemini_pro_performance_review_report.md`. It is
treated as untrusted input: a list of hypotheses, not findings. That is not a
judgement of its quality — it is the standard J17 had to learn the hard way,
where five plausible call-site theories, each obvious from reading the code,
were each rejected by measurement.

**The review's claims, checked against the source before planning** (2026-09-13):

| claim | in the code? | note |
|---|---|---|
| 32 KiB buffers allocated per file in hot paths | **yes** — `joplinraw/scalable.go:630`, `obsidian/obsidian.go:1264`, `store/sqlite.go:1845` | a `sync.Pool` is a measurable candidate |
| `os.ReadFile` slurps whole items | **yes** — three call sites in each importer | but see below: both importers need the whole body |
| `sha256Hex([]byte(a + "\x00" + b))` string/byte churn | **yes** — `obsidian.go:368`, `:1243` | concatenation then conversion allocates twice |
| `string(bytes.TrimSuffix(…))` in `splitFrontmatterBytes` allocates | **probably not** — `obsidian.go:1406` | the Go compiler elides `string(b)` when it is only compared; a benchmark settles it |
| `seen := map[string]bool{}` without capacity per note | **yes** — `joplinraw.go:545`, `scalable.go:1568` | small; worth measuring, not assuming |
| FTS5 written inside the import transaction | **yes** — `sqlite_import_batch.go:69`, `:103` | a correctness change if deferred; see boundaries |
| full-vault Go maps cost hundreds of MB | **consistent with J5** — import peak RSS was 1,721 MiB (Obsidian) and 2,881 MiB (Joplin) at 382,206 notes | a heap profile says how much is maps |

**Three places the review's proposal cannot be taken as written.**

- **Streaming only the frontmatter does not fit these importers.** The Obsidian
  importer rewrites links across the whole body and both importers write the
  whole body to the full-text index, so the body is read regardless. What is
  testable is streaming the *hash* and avoiding a second copy, not skipping the
  body.
- **Changing how a fingerprint is computed is not a free optimisation.**
  Fingerprints are persisted in import item state (`store.ImportItemState`), and
  the importers use them to recognise unchanged items on re-import. A hash
  computed differently — even over the same bytes in a different composition —
  makes every item look changed once, and a resumed import mid-change would
  misclassify. Any change here must produce byte-identical fingerprints, proven
  by comparison, or carry a migration and say so.
- **Deferring FTS indexing changes what "imported" means.** Today a note is
  searchable the moment its batch commits. An outbox drained by a background
  worker makes notes invisible to search for a while, needs crash recovery for
  the queue, and interacts with J18's rowid mapping. That is a product decision
  as much as a performance one, so it is measured first and decided explicitly.
  The sketch's APIs (`execTx`, `QueryTransientMapping`, `QueueForFTS`) do not
  exist in this repository and are read as intent, not as code.

**Scope.**

- **J19-A, investigate — every surviving claim, measured, with a recorded
  verdict.** Starts from the CPU and heap profile J17-A already names, so that
  each suggestion is tested against where time and memory actually go rather
  than where the review guessed. For each candidate: a benchmark or A/B on J5's
  corpora (a subset first, the 382,206-note vault where the subset shows an
  effect), reporting wall time, peak RSS, allocations and GC, and a verdict of
  **improves**, **no effect**, or **worse**, with the numbers. Candidates:
  pooled I/O buffers; streamed hashing without a second copy; avoiding the
  concatenate-then-convert in fingerprint composition (fingerprints must stay
  byte-identical); preallocated `seen` maps; the `splitFrontmatterBytes`
  conversion (expected to be free — the benchmark confirms or refutes); the
  memory held by inventory maps, from the heap profile, before any move to
  transient tables; and FTS writes inside versus after the import transaction,
  measured including the time until the last note is searchable.
- **J19-B, implement what J19-A showed improves — and only that.** Each change
  lands with its own before/after on the same corpus beside J19-A's numbers, and
  a candidate that measured as no effect or worse is not implemented, however
  reasonable it looks. If deferring FTS measures better, it comes to the owner
  as a decision with the visibility cost stated, not as a merged change.
- **J19-C, prove nothing else moved.** Import reports identical counts on the
  same corpus, fingerprints are byte-identical for unchanged items (a re-import
  of an unchanged vault reports every item unchanged), search results match
  before and after, and J18's mapping validator and tests still pass.

**What J19-A has measured so far** (`performance/v1.0-j19/README.md`):

- **Run noise.** Five clean sequential imports of the same 10,000 notes repeat
  within 1.5%. Shared-machine timings had differed by 43%. That exposed J17-B's
  1.46× as contaminated: clean, it is about 2.2×, and the J17 and J5 records
  carry the correction.
- **Micro-level suggestions.**
  - Pooled `hashFile` buffer: **no effect**, since the buffer never reaches the
    heap.
  - Streamed fingerprint: **worse**, 9 allocations against 6, though
    byte-identical.
  - `splitFrontmatterBytes` conversions: **refuted**, 0 allocations.
  - Preallocated `seen` maps: **no effect**.
  - Frontmatter-only streaming instead of `os.ReadFile`: **not applicable**,
    because the whole body is needed.
- **Whole-vault memory: confirmed, and the cause was not the maps.** At 382,206
  notes the Obsidian inventory and link namespace held 795 MiB live. About
  390 MB of that was titles, aliases and property names pinning per-note
  frontmatter copies. Cloning them cut the inventory from 672 to 438 MiB. The
  library is proven identical, including item states and fingerprints. This
  is implemented under J19-B.

  **The Joplin importer had the same defect, much larger.** Each note-ID map
  key kept its note's entire file alive. Cloning the parsed fields cut the
  inventory from 1,301 to 52 MiB at 382,206 notes, and from 217 to 15 MiB on a
  103,349-note export. A SHA-256 of the whole inventory is identical before and
  after on both exports. J5's 2,881 MiB Joplin import peak is probably mostly
  this; the full import has not been re-run.
- **Full-text writes after the batches: 22% faster on a write path that
  excludes links, blocks and sources.** That is about 4.5% of a real import,
  inferred rather than measured. Both of its forms change when notes become
  searchable, so it is an owner decision and is not merged.
- **Not in the review, and larger than anything it named:**
  - SQL compiled on every call: 35% of import CPU
  - regexp link extraction: 22%
  - block extraction: half of all allocation

**Boundaries.** A suggestion is adopted on measurement, never on plausibility —
J17 is the record of why. No measurement is extrapolated from a subset to the
full corpus; where the full corpus is not run, the record says so. No
fingerprint composition changes without byte-identical proof or an explicit
migration. No asynchronous indexing is merged without the owner's decision.

**Dependencies.** J17-A's profile, which J19-A uses rather than repeats. J5's
corpora and harness. J18, whose rowid mapping any FTS change must keep correct.

**Working state.** A table of every review suggestion with a measured verdict,
the improvements that measured better implemented with their before/after, and
a recorded reason for each suggestion that was not adopted.

**Closing.**
- **Full-text placement: inline, by owner decision** (2026-09-13). Notes stay
  searchable as soon as their batch commits.
- **Adopted:** the clones in both importers' inventories, each with its
  before/after and an equivalence proof. That is `performance/v1.0-j19`.
- **Carried to J20:** the remaining Obsidian inventory memory (structs stored
  twice, hex hashes), and a clean re-run of J5's corpora.

## J20. Finish the Obsidian inventory memory work, on a fresh J5 baseline — complete

**Goal.** The Obsidian importer holds no per-note data twice. Its memory and
time, and the Joplin importer's, are measured again on J5's full corpora under
the current code, so later work compares against numbers that describe this
tree.

**Why a new baseline first.** J5's numbers describe importers that no longer
exist:
- J17-B moved the Obsidian importer onto batch commits, about 2.2× faster on
  10,000 notes.
- J18 removed a full-text scan from every write.
- J19 cut the Joplin inventory from 1,301 to 52 MiB and the Obsidian inventory
  from 672 to 438 MiB.

None of that has been measured on a full 382,206-note import. J5's 4.52 h,
1.72 h, 1,721 MiB and 2,881 MiB are history now, not a baseline. J19 also
showed that timings taken beside other work differ by 43%, so every run here is
one process at a time.

**What J19 left in the Obsidian inventory** (live heap at 382,206 notes, after
the clones; `performance/v1.0-j19`):

| holder | MB |
|---|---|
| `Notes` slice of `vaultFile` | ~80 |
| `Files` slice of `vaultFile`, a second full copy of every note's struct | ~80 |
| link namespace maps | 116 |
| hex hash strings (fingerprints, frontmatter SHA) | 57 |
| property-name slices | 77 |

`Files` is read only by the source-bundle phase and to copy target IDs, so a
second copy of every struct is avoidable in principle.

**Scope.**

- **J20-A, the fresh baseline.** Import J5's Obsidian vault and Joplin RAW
  export, 382,206 notes each, into new libraries on the same HDD with the
  current code. Runs go one at a time, and nothing else runs on the machine.
  For each import, record wall time, user and system CPU, peak RSS and the
  report counts. Put them beside J5's numbers without replacing them. The
  harness says whether the page cache was cold or warm.
- **J20-B, the remaining Obsidian memory, measured before it is changed.**
  Candidates:
  - `Files` holding indices or target IDs rather than a second `vaultFile`
  - hashes held as fixed-size bytes rather than hex strings

  Each is measured with `TestJ19InventoryMemory` on the full vault and adopted
  only if it lowers held memory. It is proven by the J17 library comparison and
  the inventory's item states and fingerprints. A fingerprint composition must
  stay byte-identical: J19's rule.
- **J20-C, the baseline again after J20-B.** The Obsidian import re-runs on the
  same corpus under J20-A's conditions, with the difference stated at the size
  measured.

**Boundaries.** No change is adopted on plausibility. No subset result is
extrapolated to the full corpus. No batch size is raised to win a number. J5's
and J17's records keep their numbers, and new numbers are placed beside them.

**Dependencies.** J17, J18 and J19, whose changes the baseline measures. J5's
corpora, which must still be on the machine. **J21, by owner decision
(2026-09-14):** J20-A found every library open re-running migrations, so J20-B
and J20-C wait until J21 has fixed it, and J20-C measures the fixed tree.

**Working state.** A baseline table for both importers at 382,206 notes under
the current code, and every Obsidian inventory candidate with a measured
verdict.

## J21. Stop re-running schema migrations every time a library is opened — complete

**Goal.** Opening a library that is already at the current schema costs what
opening costs, not what migrating costs, and no step that migrates data runs
again.

**What J20-A found** (`performance/v1.0-j20/README.md`). Every CLI command on
the 382,206-note Obsidian library took about 12 s before doing any work, and
`Bootstrap` was all of it. On every open, `applySchema`:
1. re-runs `0001_initial.sql`, whose statements end with
   `PRAGMA user_version = 17`
2. runs the unguarded `ensureSchemaV4`–`V18`, each ending with
   `PRAGMA user_version = n`
3. reaches the guarded steps (V19–V28) with the version at 18, so every one of
   them runs again

Two of those re-runs are expensive at this size:

| step | per open, 382,206 notes | since |
|---|---|---|
| V28: rebuild the full-text rowid mapping from `documents_fts` | ~7 s | J18 |
| V22: `backfillRevisionObjects` | ~4.6 s | before J5 |

The last step sets the version to 28 again, so the file always reads 28 and
the re-run cannot be seen from outside. The cost reaches every `Bootstrap`
caller: every CLI command (read-only ones write 382,206 mapping rows), every
server start, the ABI registry and every external-profile open. J5's and
J20-A's search timings both include it.

**Scope.**

- **J21-A, a current library runs no migration work on open.** A test opens a
  library at the current schema a second time and fails if any guarded step
  does work again. Two options for the fix:
  - read the version once, before any step runs, and skip the steps a library
    has already passed
  - stop the unguarded steps writing a lower version

  The test decides between them, not the reading of them.
- **J21-B, every library still reaches the same schema.** The migration steps
  are "mostly idempotent" today, and a re-run may silently be supplying objects
  a current library would otherwise lack. So the schema (`sqlite_master`, and
  `user_version`) is compared before and after the change for:
  - a fresh library
  - a library migrated from an old version
  - a library already current

  J18's archive contract, G19 and G20 must still pass.
- **J21-C, re-measured at full size.** Open cost and the J5 search probes are
  re-run on J20-A's two 382,206-note libraries, one run at a time, beside
  J20-A's numbers.

**Boundaries.**
- No library at the current schema has its version or data rewritten on open.
- A library behind the current schema still migrates under the existing lock,
  backup and marker, with nothing about that path weakened.
- No schema object is dropped or renamed.

**Dependencies.** J18, whose V28 made the re-run cost what it costs. J20-A's
libraries and its measurements.

**Working state.** Opening a current library does no migration work. The
schema of every library shape is proven unchanged. Open and search are
re-measured on both full corpora.

## J22. Stop stores and tests leaving directories in the temp root — complete

**Goal.** Nothing Notrios or its test suite creates in the temp directory
outlives the process or test that made it.

**What J7 found** (`performance/v1.0-j7/README.md`, 2026-09-15). J7-C's first
disaster-recovery run was stopped from outside because the machine ran low on
memory. `/tmp` is RAM-backed tmpfs, and it held 16 GB in 17,078 leftover
`notrios-*` entries:

| entries | count | size | cause |
|---|---|---|---|
| `notrios-assets-*` | 16,888 | 1.67 GiB | `defaultAssetRoot` (`internal/store/sqlite.go`) makes a private temp asset directory for a path-less or `:memory:` store, and `SQLiteStore.Close` never removes it |
| `notrios-test-bin-*` | 171 | 6.53 GiB | `cmd/notriosctl/stablelinks_test.go` makes a shared test-binary directory and never removes it |
| `notrios-import-manifest-*` | 6 | 3.33 GiB | a J19 test that did not close its manifest, fixed in J7; the importers close theirs |
| validator Go build caches | 4 | 2.6 GiB | the G18/G19/G20 evidence validators |

The first two are this item. The asset-directory leak is in product code, not
only in tests: any caller that opens a store without an asset store leaks one
directory per open.

**Each instance keeps its temp files to itself, by owner decision
(2026-09-15).** Several Notrios instances can run on one machine. Any code that
creates or removes temp files must keep each instance's temp files in that
instance's own temp directory, and must never touch another instance's.
Product code today puts three kinds of temp entry straight into the shared
system temp root:
- the path-less store's asset root (`defaultAssetRoot`)
- the import manifest spool (`OpenImportManifest`: export, restore, Joplin
  import)
- the archive verification spool (`VerifyDirectory`)

On this machine that root is RAM-backed, and one import manifest reached
3.3 GB. The owner chose:
- **Where an instance's temp directory lives:** a new per-instance path,
  `data.temp_dir`, defaulting to `<cache root>/tmp`. It is covered by the
  profile path-sharing validation that already keeps two profiles' databases,
  assets, projections, index and quarantine apart.
- **Per process:** each process of the instance works in its own subdirectory
  there, holding a lock.
- **With no instance configured** (library use, tests, `:memory:` stores): each
  temp entry is a private `0700` directory in the system temp root, and is
  removed when the object that made it closes.
- **Crash leftovers:** when an instance starts, it removes only subdirectories
  of **its own** temp directory whose lock no live process holds. It never looks
  at another instance's directory, and never scans the system temp root by name.

**Scope.**

- **J22-A, a store removes the temp asset root it created.** `Close` removes an
  asset root the store made for itself, and never one a caller supplied, nor
  the directory beside a database file. A test proves both halves.
- **J22-B, one temp directory per instance.** `data.temp_dir` is resolved,
  created and validated like the other runtime paths. The import manifest, the
  verification spool and the path-less asset root are created in the running
  instance's per-process subdirectory when an instance is configured, and in a
  private system-temp directory otherwise. Tests prove:
  - two instances' temp files never share a directory
  - an instance removes an unlocked leftover in its own directory
  - it leaves a locked one alone, and never touches another instance's
- **J22-C, tests clean up after themselves, and a check keeps it fixed.**
  - The CLI tests' shared binary directory is removed when the package's tests
    finish.
  - The Go test suite runs with `TMPDIR` pointed at a fresh directory. The check
    fails if any `notrios-*` entry is left in it, and any other leftover a run
    reveals is fixed the same way.
  - The validators' Go build caches stop defaulting into the temp root, and the
    check refuses one that does.

**Boundaries.**
- Removal is limited to paths the store, the instance or the test created and
  recorded, or to unlocked subdirectories of the running instance's own temp
  directory. Nothing is removed by name pattern in a shared directory.
- A caller-supplied asset store is never deleted.

**Dependencies.** None.

**Working state.** A full Go test run leaves no `notrios-*` entry in its temp
root, and a check fails if one returns.

## J23. Keep existing sync peers syncing after both upgrade in place — complete

**Goal.** Two replicas that were syncing before an upgrade keep syncing after
both have upgraded, on their original pairing, without weakening what pairing
protects.

**What J7 found** (`performance/v1.0-j7/README.md`, 2026-09-15). J7-B's
`upgrade_in_place.sh` pairs two replicas on each historical version:
- **Before the upgrade** (0.7.0, 0.8.0, v0.9), they converge.
- **While their versions differ,** they move nothing.
- **Once both are on 1.0, they never converge again.** Each publishes, and each
  refuses the other on every round.

The cause:
1. **Pairing pins the peer's exact handshake.** `ConfigureSyncAdmissionPeer`
   stores it in `sync_peer_compatibility`: schema 27, range 24–27, protocol and
   capabilities. Pairing is its only caller.
2. **The upgrade doesn't touch that row.** After both upgrade, each library is
   at schema 28 and still holds its peer as 27, range 24–27.
3. **Admission requires an exact match.** `AdmitSyncOperations` calls
   `validateConfiguredSyncPeerLocked`, whose `sameSyncCompatibility` compares
   protocol, schema version, compatible range and capabilities for exact
   equality. The upgraded peer reports 28 with range 24–28, and every batch is
   refused: "peer compatibility differs from explicit configuration".

Re-pairing the same replicas is no way back: the table is keyed by `replica_id`
and pairing writes it with a plain `INSERT`. So a pre-1.0 user who upgrades both
replicas in place, the supported path under J7's owner decision, loses sync
between them.

**Scope, by owner decision (2026-09-15).**

- **J23-A, accept a schema rise both sides admit.** When a paired peer's
  handshake differs from its stored compatibility only in `schema_version` and
  its compatible range, admission accepts it on two conditions:
  - the new schema version is higher than the stored one
  - the local schema is inside the peer's new range, and the peer's schema is
    inside the local range

  The stored row is then updated, and a `peer.compatibility_upgraded` audit
  event records the old and new values.
- **J23-B, keep everything else pinned.** A change in protocol major or minor
  range, required or optional capabilities, database identity or replica
  identity is still refused, and so is a schema version lower than the stored
  one. Security tests prove each refusal, including a peer that raises its
  schema and changes a capability in the same handshake.
- **J23-C, the drill that found it.** `upgrade_in_place.sh` converges on the
  original pairing, after both replicas upgrade, for 0.7.0, 0.8.0 and v0.9.

**Boundaries.**
- The update happens inside admission's existing transaction, after the peer's
  signature and identity checks. A peer is never accepted on an unauthenticated
  claim.
- No change weakens what an explicit pairing guards against today.
- The sync protocol and the wire formats are unchanged.

**Dependencies.** J7-B's drill, which is J23-C's acceptance test.

**Working state.** Replicas upgraded in place keep syncing, and every other
change to a peer's pinned compatibility is still refused, each proven by a
test.

## J24. Check the running agent's own usage, not every agent's — complete

**Goal.** The usage preflight that guards archives, drills and other long work
checks the quota of the coding agent actually running it. A Claude run is
guarded by Claude's usage, and a Codex run by Codex's. Neither is paused by the
other product's quota.

**What happened** (2026-09-15, while closing J23). The J23 archive build was
paused by `scripts/agent_usage_preflight.sh`. The run was Claude's, and Claude's
quota was above the 20% reserve: five-hour 32% remaining, seven-day 92%
remaining, which the owner's account page confirmed as 91%. The pause came from
**Codex's** weekly bucket (8% remaining). Codex is a separate product with its
own quota, and it has no bearing on a Claude run.

**The cause.** `scripts/check_agent_usage.py` supports
`--agent codex|claude|all`, and its tests exercise `claude` and `codex`
separately. But `agent_usage_preflight.sh` always passes `--agent all`, and the
checker then binds on the lowest bucket across every agent it can read.
`scripts/test_agent_usage_preflight.sh` asserts exactly that argument. Every
caller of the preflight inherits it, including `package_release.sh`,
`validate-scaffold.sh` and the `run_*_profile.sh` scripts.

**A misdiagnosis, recorded.** Before the owner pointed out that the run was
Claude's, the agent treated the Codex reading as the relevant number. It then
suspected that Codex's `usedPercent` field was inverted, because the owner's
91%-remaining figure disagreed with it. That figure was Claude's. The inversion
hypothesis is withdrawn, and Codex's 92%-used reading may well be correct.
The owner overrode the guard for J23's archive and J7's
(`NOTRIOS_AGENT_USAGE_GUARD=off`). Each close commit records the override and
that the running agent's quota was above the reserve.

**Scope.**

**Identifying the running agent, by owner decision (2026-09-15).** More than one
coding agent can run on the machine at once, and a Codex session was running
beside this Claude session when J24 started. So neither of these can say whose
run it is:
- **Environment variables.** An agent's variables are inherited by every shell
  and process started under it, so they leak into other contexts.
- **The checker's existing Claude detector.** It scans every process on the
  machine for a `claude` binary.

The run has to be identified clearly as its own.

- **J24-A, select the running agent by process ancestry.** A new
  `--agent self`, which the preflight passes, resolves in this order:
  1. **The nearest coding agent among this process's parents.** It is found by
     walking the parent chain and classifying each process by its executable
     and `argv[0]`, never by its other arguments:
     - `claude`
     - Codex's native `codex`, a `codex-*` helper, or node running `bin/codex`

     Nested agents resolve to the nearest one. Variables are not consulted.
  2. **Otherwise, an explicit `NOTRIOS_AGENT_USAGE_AGENT`** (`claude`, `codex`
     or `all`). This covers a caller with no agent among its parents, such as a
     detached run whose launching shell has exited. If ancestry does find an
     agent, a differing explicit value is ignored and reported.
  3. **Otherwise `all`,** today's behaviour, so an unidentified caller is never
     less guarded than before.

  The checker reports which agent it checked and why.
- **J24-B, tests.** The checker's tests cover:
  - classification, including processes that only mention an agent in their
    arguments
  - nearest-ancestor resolution with nested agents, orphans and a parent cycle
  - ancestry winning over leaked variables, and each fallback rung
  - a Claude run that a Codex bucket below the reserve does not pause, and that
    its own low bucket does, with the other agent's probe never called
  - the same for a Codex run

  `test_agent_usage_preflight.sh` asserts the preflight passes `--agent self`.
- **J24-C, the callers and the docs.** Every script that calls the preflight
  gets the selection without changes of its own. The documentation for the
  guard says whose usage is checked and how to choose.

**Boundaries.**
- Claude's numbers are correct as they are. The statusLine cache
  (`scripts/claude_statusline_usage.py`) matches the owner's account page, and
  the checker's `--agent claude` path reads it correctly. Only the selection
  changes.
- Other coding agents keep working. Each agent already has its own code path in
  the checker. Claude-specific changes stay on Claude's path. Codex's path, and
  the `all` path an unidentified caller falls back to, behave exactly as before,
  and their existing tests still pass unchanged.
- The reserve rule and the override stay as they are: an explicit owner
  decision per item, recorded in the close commit.
- Nothing from the agent's environment beyond the agent's name is logged. That
  environment carries session identifiers and tokens.

**Dependencies.** None.

**Working state.** A Claude run is guarded only by Claude's usage and a Codex
run only by Codex's, each proven by a test, with `all` kept for callers that
cannot be identified.

## J25. Import a Twitter/X archive as it is downloaded, completely, at its real size — complete

**Goal.** A user can hand Notrios the ZIP they downloaded from Twitter/X. Every
post in it is imported, however many files the archive splits them into, and a
3.3 GB archive imports within bounded memory.

**What was found** (2026-09-15, owner request). The importer from v0.2
(`internal/importers/twitter`) has never been run on a real archive. J5 listed
the owner's archive as a corpus but measured only Joplin and Obsidian. Read
against that archive (`/media/renes/HD2/twitter/twitter-…dd40.zip`, 3.32 GB,
15,088 entries), the importer falls short in three ways:
- **It does not accept the download.** It takes an extracted folder only
  (`notriosctl import twitter <extracted-archive-dir>`). A user is expected to
  unzip 3.3 GB and know which folder to point at.
- **It silently drops most of the posts.** A large archive splits its posts
  across `data/tweets.js` and `data/tweets-part1.js`, `-part2.js`, `-part3.js`.
  The importer reads only `tweets.js`. In this archive that holds 55,339 of
  176,423 posts, so **121,084 (68.6%) would be silently left out**.
  `data/tweet-headers.js` lists all 176,423 and is an independent count.
- **It reads each file whole.** Each part is about 105 MB of JSON, read with
  `os.ReadFile` and decoded into memory in one piece. Every post is then written
  one document at a time (J17-C), with its media resource, attachment and tags
  each a separate store call.

The archive also holds `data/tweets_media/` (8,737 files, 3.31 GB), one post in
`community-tweet.js`, six in `deleted-tweets.js`, and an empty `note-tweet.js`.
The importer reads none of these except the media folder.

**Scope, by owner decision (2026-09-15).**

- **J25-A, the downloaded ZIP and every part.**
  - `notriosctl import twitter` accepts the ZIP as downloaded, and still
    accepts an extracted folder.
  - The ZIP is read in place, with no extraction. Entries are looked up by
    name, never written to disk by their archive path.
  - Posts are read from `tweets.js` and every `tweets-partN.js` in part order,
    or from an older archive's `tweet.js`.
  - **Community posts are imported** like ordinary posts.
  - **Deleted posts are not imported, and the report counts them.** The owner
    decided they must not come back silently.
  - The report compares the posts found with `tweet-headers.js` when it is
    present, and says so if they differ.
  - A test fails first on the unchanged importer: an archive whose posts are
    split across parts.
- **J25-B, bounded memory on untrusted input.**
  - Post files are decoded as a stream rather than read whole.
  - The media index comes from the ZIP's directory.
  - Media are streamed into the asset store.
  - Bounds on entry count, per-file decompressed size and media size are
    enforced and tested, including a decompression bomb and a path escaping the
    archive.
  - Temporary work uses the instance's temp directory (J22).
- **J25-C, the real archive, measured.**
  - A dry run and a full import of the owner's archive, timed by phase with
    peak RSS: every post and every media file accounted for against
    `tweet-headers.js` and the ZIP's directory.
  - A re-import, to check it is idempotent.
  - Whether writing one document at a time needs the batched, resumable path
    Joplin and Obsidian have is decided from these numbers. If it does, that is
    a follow-up item for the owner, as J19 and J20 were.

**Boundaries.**
- The archive is private. Nothing from it (text, names, media, IDs) goes into
  the repository or into evidence: only counts, sizes and timings.
- Runs use scratch space on `/media/renes/HD2`, never RAM-backed `/tmp`, with
  HOME, XDG and TMPDIR isolated.
- No network access. Remote media URLs in posts are not fetched.
- The note format, provenance, thread recovery, and re-import behaviour for
  posts already imported are unchanged.

**Dependencies.** J22, for the instance temp directory.

**Working state.** `notriosctl import twitter <download>.zip` imports every
post of the owner's 3.3 GB archive, and that count equals `tweet-headers.js`
less the deleted posts plus the community post. It does so within bounded
memory, measured and recorded, and fails clearly on a hostile archive.

## J26. Import ChatGPT, OpenAI Privacy Portal and Claude archives as downloaded — complete

**Goal.** A user hands Notrios the ZIP they downloaded from ChatGPT, from the
OpenAI Privacy Portal, or from Claude. Every conversation becomes its own note,
with its code blocks and file attachments carried across, and the real archives
are measured.

**What was found** (2026-09-15, owner request; the same question J25 asked of
the Twitter/X importer). The conversation importers from v0.2 have never run on
a real archive. Both take `conversations.json`, read it whole with
`os.ReadFile`, and import message text only. Measured against the owner's three
archives:

| | ChatGPT export (21.3 MB) | OpenAI Privacy Portal (154.3 MB) | Claude (5.8 MB) |
|---|---|---|---|
| shape | one ZIP: `conversations.json`, `chat.html`, 71 asset files | one ZIP holding **nested ZIPs**: `User Online Activity/Conversations__….zip`, `Files__….zip`, `Ads__….zip` | one ZIP: `conversations.json`, `users.json`, `projects/*.json` |
| conversations | 72 in `conversations.json` | **147 across `conversations-000.json` and `conversations-001.json`** | 75 in `conversations.json` |
| assets | `file-<id>-<name>.<ext>`, `user-<id>/file_<hash>-<name>.<ext>` | **225 `file-<id>.dat`**, extensions stripped | none: attachments carry extracted text, `files` carry a name and UUID only |
| what today's importer does | needs the ZIP extracted; imports text only, no assets | cannot read it at all: the conversations are inside a nested ZIP | needs the ZIP extracted; imports text only |

- **Assets can be matched, under either prefix.** File IDs appear as both
  `file-<id>` and `file_<hash>`, on disk and in the JSON, and the second is the
  common one: 48 of the direct export's 71 assets, 207 of the portal's 225
  `.dat` files, and 248 of 250 `library_files.json` IDs. `file-service://`
  pointers (18) name the first and `sediment://` pointers (40) the second.
  Matching accepts either. In the direct export every asset
  pointer's file ID is present on disk (58 of 58), and 64 of 79 attachment IDs
  are. In the
  Privacy Portal, `conversation_asset_file_names.json` names 132 of the 225
  `.dat` files, and **all 93 of the rest appear in `library_files.json`**, which
  carries `file_extension` and `mime_type`. Between the two, every `.dat` file
  can be given its real name and type.
- **`chat.html` is a reference, not a source.** It renders conversations in both
  export formats, but in the Privacy Portal export it shows the `.dat` names
  rather than the real ones, so the JSON files are what the importer reads.
- **The file library is separate.** `Files__….zip` holds the ChatGPT file
  library (27 files here), most of whose names do not match a
  `library_files.json` record, so they are matched by ID and type rather than
  by name.
- **Messages carry more than text.** ChatGPT: code (181), execution output
  (121), thoughts (105), reasoning recaps (60), browsing displays (26),
  multimodal parts (52 asset pointers); the Privacy Portal export is mostly
  thoughts (1,905) and text (1,638). Claude: `tool_use` (1,318),
  `tool_result` (1,317), `thinking` (631), `text` (1,263), plus 37 attachments
  with extracted text and 109 file references.

**Scope, by owner decision (2026-09-15).**

- **J26-A, one archive reader for every importer.** J25's ZIP-or-folder source
  moves out of `internal/importers/twitter` into a package both importers use,
  keeping its bounds, its refusal of unsafe entries and its streaming decode.
  It gains **nested ZIP** support: a ZIP inside a ZIP is read in place, without
  extracting either. The Twitter/X importer keeps its behaviour and its tests.
- **J26-B, the ChatGPT export ZIP.** `import chatgpt` accepts the downloaded
  ZIP, the extracted folder, or a `conversations.json`, and reads every
  `conversations*.json` shard. Each conversation is one note in the **ChatGPT**
  notebook. Assets referenced by a message are imported as resources and
  embedded, matched by file ID.
- **J26-C, the OpenAI Privacy Portal ZIP.** The same command accepts the portal
  export: the conversations and files are read from the nested ZIPs under
  `User Online Activity/`, in place.
  - `.dat` assets recover their real name and type from
    `conversation_asset_file_names.json`, then `library_files.json`, then by
    sniffing the bytes; each source is recorded in the report.
  - **The file library is imported** (owner decision). A file a conversation
    references is attached to that note; one nothing references gets a stub
    note in a **ChatGPT Files** notebook, so it is searchable rather than
    silently dropped.
- **J26-D, the Claude archive ZIP.** `import claude` accepts the downloaded
  ZIP, and reads sibling `batch-NNNN` ZIPs when the user passes a directory
  holding them. Each conversation is one note in the **Claude** notebook,
  attachment text is carried into the note, and file references are recorded by
  name. **Each project becomes a note** (owner decision): its description,
  prompt template and each doc.
- **J26-E, what a note contains** (owner decision). Code and execution output
  become fenced code blocks, and browsing results become text. Thinking,
  reasoning recaps and tool-call plumbing are **not** imported, and are counted
  in the report. Attachments are named in the note where their message is.
- **J26-F, the three real archives, measured.** Each is dry-run, imported and
  re-imported, timed, with peak RSS, and every conversation and asset
  accounted for against the archive's own counts.

**Boundaries.**
- The archives are private. Nothing from them (text, names, IDs, bytes) goes
  into the repository or into evidence: only counts, sizes and timings. Every
  fixture is synthetic.
- Runs use scratch space on `/media/renes/HD2`, never RAM-backed `/tmp`, with
  HOME, XDG and TMPDIR isolated.
- No network access, and nothing is extracted to disk from any archive.
- Bounds on entry counts, decompressed sizes and nesting depth are enforced and
  tested, including a ZIP nested inside a ZIP.
- Re-import stays idempotent, and a note the user trashed is never resurrected.

**Dependencies.** J25, for the archive reader this generalises; J22, for the
instance temp directory.

**Working state.** `import chatgpt <download>.zip`, `import chatgpt
<privacy-portal>.zip` and `import claude <download>.zip` each import every
conversation in the owner's archives as its own note, with code blocks and
attachments, measured and recorded, and refuse a hostile archive clearly.

## J27. Normalise a host before matching it against the domain lists — complete

**Goal.** A host written in any of the forms DNS treats as the same name
matches the same domain rules.

**What J8 found** (`performance/v1.0-j8/README.md`, finding J8-F1).
`https://blocked.example.org./a.png` is evaluated as `review` while
`https://blocked.example.org/a.png` is blocked: `matchDomain` compares the host
string literally, so the trailing-dot form matches no pattern and falls through
to the policy default. The allowed list fails the safe way round; the blocked
list fails open.

**Measured before starting, 2026-09-16.** How Go's `net/url` reads the forms
this item concerns:

- `https://blocked.example.org./a` has host `blocked.example.org.`, and
  `BLOCKED.Example.ORG.:8443` keeps its case and dot. Both need normalising.
- `127.0.0.1.` and `localhost.` are **not** recognised by the static private
  check either: `net.ParseIP("127.0.0.1.")` is nil and the `localhost`
  comparison is literal. The connect-time check still refuses what they
  resolve to, but the static verdict is `review` instead of `block`. That is
  the same gap in the same function, so it is in this item.
- `blocked.example.org..` parses, with an empty final label. DNS does not treat
  it as the same name, and Go's resolver refuses it (`no such host`).
- `%2E` in a host is a parse error, so a percent-encoded dot cannot disguise a
  host.
- `https://./a` parses with host `.`.
- A full-width dot (`U+FF0E`) survives parsing as a different host. Unicode and
  IDNA mapping are outside this item (boundary below).

**Scope.**

- **J27-A, one normalised form.** One function lowercases the host and removes
  **one** trailing dot, the form DNS treats as the same name. `Evaluate` applies
  it once, before the private-address check and before the blocked, allowed and
  review patterns are matched.
  - Configured patterns get the same normalisation, so `example.org.` in a list
    means `example.org`. The wildcard rule keeps its current meaning.
  - A host that is empty after normalisation, or still has an empty label
    (`..`, a leading dot, a second trailing dot), is **blocked as malformed**,
    not left to the default action. No resolver treats it as a real name, and
    a malformed host must not reach review by accident.
  - Cases cover the dotted form against every list and the dotted forms of
    `localhost` and an address literal. They also cover the malformed forms, a
    dotted pattern, and the unchanged apex-versus-wildcard behaviour.
  - The test fails on today's code.
- **The record.** `performance/v1.0-j27/README.md` states what was measured and
  what changed. `docs/service.md`'s domain-pattern row says how hosts are
  matched. J8's probe is J8's archived evidence and is not edited: its J8-F1
  cases now log "as expected", and the regression guard is J27's test.

**Boundaries.** No change to what the patterns mean, only to the form of the
host they are matched against. Punycode and Unicode host forms are a separate
question and are not in this item.

**Dependencies.** J8, which found it.

**Working state.** A blocked domain stays blocked however the host is spelled,
proven by a test that fails on today's code.

## J28. Refuse the reserved address ranges remote media must not reach, by a named default the configuration can state — complete

**Goal.** The address ranges remote-media localization refuses are one named
set: written in `SECURITY_AND_MEDIA_POLICY.md`, stated explicitly in the
configuration when the owner wants a different set, defaulted to the policy's
set when the configuration says nothing, and applied by one helper at both
places an address is checked.

**What J8 found** (`performance/v1.0-j8/README.md`, finding J8-F2). The written
policy says "private network and link-local". The code refuses what Go's
`IsLoopback`, `IsPrivate`, `IsLinkLocalUnicast`, `IsLinkLocalMulticast` and
`IsUnspecified` cover, plus `localhost` names, and nothing else. The same
expression is written twice — `isPrivateHost` in `internal/media/policy.go`
for URL literals and `checkDialAddress` in `internal/media/fetch.go` at connect
time — sharing neither a list nor a helper.

**Owner direction, 2026-09-16.** Default reserved ranges come from the
references supplied with the direction. The configuration gets an explicit
security block naming the ranges to refuse. Notrios uses the default set when
the configuration does not specify one. The documentation says how to specify
it.

**Measured, 2026-09-16.** Today's expression, run over one address in every
range below and a few beside them: **15 of 44 refused.** Refused: `127.0.0.0/8`, the three RFC 1918
blocks, `169.254.0.0/16`, `0.0.0.0` itself, `::1`, `::`, `fe80::/10`,
`fc00::/7`, IPv4-mapped loopback and private forms, and **link-local
multicast only** (`224.0.0.2`, `ff02::1`). Not refused: the rest of
`0.0.0.0/8`, `100.64.0.0/10`, `192.0.0.0/24`, the three IPv4 documentation
nets, `198.18.0.0/15`, `192.88.99.0/24`, `240.0.0.0/4`, `255.255.255.255`,
admin-scoped and global multicast (`239.1.1.1`, `233.252.0.1`, `ff05::1`,
`ff0e::1`), IPv4-mapped forms of non-private addresses (`::ffff:100.64.0.1`),
`64:ff9b::/96` and `64:ff9b:1::/48` (NAT64; `64:ff9b::7f00:1` is `127.0.0.1`),
the deprecated IPv4-compatible `::/96` (`::7f00:1`), 6to4 `2002::/16`
(`2002:7f00:1::1` embeds `127.0.0.1`), Teredo `2001::/32`, `100::/64`,
`2001:2::/48`, `2001:db8::/32`, `3fff::/20`, `fec0::/10`, `2001:10::/28` and
`5f00::/16`.

**A correction to J8's record.** J8's README lists "multicast" among what
held. The probe used `224.0.0.2`, which is link-local multicast; wider
multicast is not refused. J28-D corrects the README's wording.

### Checking the references

The references were checked against IANA's IPv4 and IPv6 Special-Purpose
Address Registries (both last updated 2025-10-09), RFC 5771 for IPv4
multicast, RFC 4291 for IPv6 multicast and IPv4-compatible addresses, and
RFC 3879 for site-local.

**Accurate.** Every range the references name exists with the purpose they
give it. `169.254.169.254` is the cloud metadata address. The IPv6 metadata
addresses providers use (for example `fd00:ec2::254`) fall inside `fc00::/7`.
Connecting to `0.0.0.0` reaches the local host on Linux and macOS.
`198.18.0.0/15` and `2001:2::/48` are benchmarking; `2001:db8::/32` and
`3fff::/20` (RFC 9637) are documentation.

**Inaccurate or incomplete.**

- **`100.::/64` is a typo** for `100::/64`, the discard-only block (RFC 6666).
- **"Practically exhaustive" is not so.** The references omit, among the
  registries' not-globally-reachable blocks: `64:ff9b:1::/48` (local-use
  NAT64, RFC 8215), `192.88.99.0/24` (deprecated 6to4 relay, RFC 7526),
  `5f00::/16` (SRv6 SIDs, RFC 9602) and `100:0:0:1::/64` (dummy prefix,
  RFC 9780). They omit, of the blocks marked N/A because they embed or
  deprecate: `2002::/16` (6to4), `2001::/32` (Teredo) and `2001:10::/28`
  (deprecated ORCHID). They also omit the deprecated IPv4-compatible `::/96`
  and site-local `fec0::/10`, which J8 found.
- **The sharpest J8 case is absent.** `64:ff9b::/96` is in neither list. The
  registry marks it globally reachable, which is exactly why it needs care
  rather than a blanket refusal (decision D4).
- **Refusing `::ffff:0:0/96` is not enough on its own.** It stops the mapped
  form, but IPv4 addresses also arrive embedded in NAT64, 6to4 and
  IPv4-compatible addresses. The check has to reason about the embedded
  address, not only the prefix.
- **The "best-practice" JSON block is not a usable default.** It drops ranges
  its own list names: `192.0.0.0/24`, all three IPv4 documentation nets, and
  the limited broadcast. `255.255.255.255/32` is inside `240.0.0.0/4`, so
  listing both is redundant, though harmless.
- **"Covers all ... scopes defined by RFC 6890" is out of date.** RFC 8190
  updated RFC 6890, and IANA's registries are the current source. The default
  set records the registry date it was checked against.
- **The implementation tips are already met.** "Resolve DNS first and pin the
  address" is met more strictly: `Dialer.Control` checks the concrete address
  about to be dialed, after resolution, on every connection attempt, including
  every redirect hop and each address Happy Eyeballs tries. The transport has
  no `Proxy`, so an environment proxy cannot dial on Notrios's behalf.
  "Validate every redirect hop" is met by `checkRedirect`, which J8 confirmed.
  No change to pinning is recommended.

**Not refused by default, deliberately.** Blocks the registries mark globally
reachable are not reserved against the internet, and refusing them would be
arbitrary:

- `192.0.0.9/32` and `192.0.0.10/32` (PCP and TURN anycast), which fall inside
  the refused `192.0.0.0/24` anyway
- `2001:1::1`, `2001:1::2` and `2001:1::3`
- `2001:3::/32` and `192.52.193.0/24` (AMT)
- the AS112 blocks
- `2001:20::/28` (ORCHIDv2) and `2001:30::/28` (drone remote ID)

An owner who wants them refused lists them.

### The default refusal set

Decided by D4's consequence below: 15 IPv4 ranges and 16 IPv6 ranges, plus the
embedded-IPv4 rule, which is not a list entry.

IPv4:

| range | purpose | source |
|---|---|---|
| `0.0.0.0/8` | "this network"; `0.0.0.0` reaches the local host | RFC 791, RFC 1122 |
| `10.0.0.0/8` | private use | RFC 1918 |
| `100.64.0.0/10` | shared address space, carrier-grade NAT | RFC 6598 |
| `127.0.0.0/8` | loopback | RFC 1122 |
| `169.254.0.0/16` | link-local, including cloud metadata | RFC 3927 |
| `172.16.0.0/12` | private use | RFC 1918 |
| `192.0.0.0/24` | IETF protocol assignments | RFC 6890 |
| `192.0.2.0/24` | documentation, TEST-NET-1 | RFC 5737 |
| `192.88.99.0/24` | deprecated 6to4 relay anycast | RFC 7526 |
| `192.168.0.0/16` | private use | RFC 1918 |
| `198.18.0.0/15` | benchmarking | RFC 2544 |
| `198.51.100.0/24` | documentation, TEST-NET-2 | RFC 5737 |
| `203.0.113.0/24` | documentation, TEST-NET-3 | RFC 5737 |
| `224.0.0.0/4` | multicast | RFC 5771 |
| `240.0.0.0/4` | reserved; contains `255.255.255.255` | RFC 1112, RFC 919 |

IPv6:

| range | purpose | source |
|---|---|---|
| `::/128` | unspecified | RFC 4291 |
| `::1/128` | loopback | RFC 4291 |
| `64:ff9b:1::/48` | local-use NAT64 | RFC 8215 |
| `100::/64` | discard-only | RFC 6666 |
| `100:0:0:1::/64` | dummy prefix | RFC 9780 |
| `2001::/32` | Teredo | RFC 4380 |
| `2001:2::/48` | benchmarking | RFC 5180 |
| `2001:10::/28` | deprecated ORCHID | RFC 4843 |
| `2001:db8::/32` | documentation | RFC 3849 |
| `2002::/16` | 6to4 | RFC 3056 |
| `3fff::/20` | documentation | RFC 9637 |
| `5f00::/16` | SRv6 SIDs | RFC 9602 |
| `fc00::/7` | unique local, including cloud metadata | RFC 4193 |
| `fe80::/10` | link-local | RFC 4291 |
| `fec0::/10` | deprecated site-local | RFC 3879 |
| `ff00::/8` | multicast | RFC 4291 |

**The embedded-IPv4 rule (D4).** An address in `::ffff:0:0/96` (IPv4-mapped,
RFC 4291), `::/96` (deprecated IPv4-compatible, RFC 4291) or `64:ff9b::/96`
(NAT64 well-known prefix, RFC 6052) is refused when its IPv6 form matches an
IPv6 entry **or** its embedded IPv4 address matches an IPv4 entry of the
effective set. The IPv6 form is checked first, so `::` and `::1`, which lie
inside `::/96`, stay refused as themselves.

**Why these three prefixes are not in the table.** A table entry refuses its
whole range, and D4 decided against refusing these prefixes outright. They are
therefore not default entries. The rule is fixed behaviour, applied against
whichever IPv4 set is in effect, stated or default. An owner who lists one of
the prefixes in a stated set refuses that prefix outright, as an explicit
choice. The D2 warning does not name them, because they are not default ranges.

`localhost` and `*.localhost` names stay refused at the static check, as they
are today (RFC 6761).

### The configuration block (D1)

```yaml
security:
  remote_media:
    # Absent: the default set above. Present, even as []: exactly this set.
    refused_address_ranges:
      - "127.0.0.0/8"
      - "10.0.0.0/8"
      # ...
    # Exceptions to the refused set; empty by default.
    permitted_address_ranges: []
```

`notriosctl config show` reports the effective set and whether it came from
the file or the default. The block governs remote-media fetches only (D6).
Lists are read at `security.remote_media.<key>`, one level deeper than the
loader reads lists today, so the loader is extended for this section only.

### Owner decisions, 2026-09-16

Every recommendation was accepted as written.

- **D1 — Where the block lives.** A new top-level `security:` section, with a
  `remote_media:` subsection whose name states the surface it governs.
- **D2 — Replace or extend.** A stated list **replaces** the default. Both
  service start and `config show` warn, naming every default range the
  configured list omits.
- **D3 — Malformed entries.** An entry that is not a valid CIDR or address
  fails configuration loading, and the error names the entry.
- **D4 — IPv4 carried inside IPv6.** For IPv4-mapped, IPv4-compatible and
  `64:ff9b::/96` addresses, the embedded IPv4 address is extracted and checked
  against the IPv4 set, instead of refusing the prefix outright. On IPv6-only
  networks with DNS64, every IPv4-only media host resolves into `64:ff9b::`, so
  refusing that prefix would break localization there; checking the embedded
  address keeps `64:ff9b::7f00:1` refused. 6to4 (`2002::/16`) and Teredo
  (`2001::/32`) stay refused whole.
- **D5 — `allow_private_networks`.** Kept, a frozen configuration key (I8), with
  its meaning unchanged: `true` switches off the refused set at both checks,
  as it switches off today's checks.
- **D6 — What the block governs.** Remote-media fetches only. Sync's dialing is
  untouched.
- **D7 — Exceptions.** `permitted_address_ranges` is included, empty by
  default.

### Consequences applied without a further decision

These follow from the decisions above, and the plan now says them:

- **From D4:** the three embedded-IPv4 prefixes left the default table and
  became the rule stated beside it (above). Otherwise a default entry would
  refuse the NAT64 prefix outright, which is what D4 rejected.
- **From D2:** a stated empty list, `refused_address_ranges: []`, refuses
  nothing, and the warning names every default range.
- **From D3:** the rule covers both lists. A malformed
  `permitted_address_ranges` entry fails loading too, since silently dropping
  one would change what is fetchable without saying so.

### Further recommendations

None of these changes a decision; each makes a decided behaviour visible.

- **Where warnings appear.** `config show --json` carries a `warnings` array;
  plain `config show` prints them to standard error; the REST configuration
  view reports the effective set, its origin and the same warnings; service
  start logs them once.
- **A stated set that does nothing.** When `allow_private_networks: true`,
  `config show` and service start say that the refused set, including the
  `localhost` names, is inactive. A stated set switched off by D5 should not
  look as if it applies.
- **Exceptions are visible.** A non-empty `permitted_address_ranges` is logged
  at service start and shown by `config show`. An exception is still subject
  to domain policy.
- **Older binaries.** The loader ignores unknown sections by design, so a
  binary from before J28 ignores `security:` and applies today's narrower
  checks without warning. The configuration guide says which version first
  reads the block. Old binaries cannot be changed; this is a documentation
  duty, not a code one.

### Owner decisions D8 and D9, 2026-09-16

D7's first answer did not restate two parts of its recommendation; the owner
decided both as recommended.

- **D8 — An exception may not re-permit loopback or "this host".**
  `permitted_address_ranges` cannot permit anything in `127.0.0.0/8`,
  `::1/128`, `0.0.0.0/8` or `::/128`. That includes the embedded-IPv4 forms
  (`::ffff:127.0.0.1`, `64:ff9b::7f00:1`). An entry overlapping one of them
  fails loading, and the error names the entry. Reaching those addresses needs
  `allow_private_networks: true`, the coarse switch D5 kept. The reason: a
  loopback exception would make the service fetch from its own REST, MCP and
  sync endpoints on a URL a note supplied.
- **D9 — An exception reaches embedded forms.** Exceptions are matched with the
  same embedded-IPv4 rule as the refused set, so permitting `192.168.1.0/24`
  also permits `::ffff:192.168.1.5` and `64:ff9b::c0a8:105`, which name the
  same IPv4 host. D8's guard applies to those forms too. Without this, a
  permitted NAS would be unreachable on a NAT64 network, the case D4 exists
  for.

**No decisions remain open.**

### Implementation design, 2026-09-16

Read before starting; where it departs from the scope's wording, it says so.

- **A leaf package, not a function in `internal/media`.** The configuration
  loader must reject malformed entries (D3) and loopback exceptions (D8), and
  `internal/media` imports `internal/config`. The default set, entry parsing,
  the embedded-IPv4 rule and matching therefore live in a new
  `internal/addressrange`, imported by both. `internal/media` holds no range
  logic of its own, and both of its checks call the same `Rules.Check`. The
  "one helper" requirement is met, one package down.
- **Not stated means default.** `security.remote_media.refused_address_ranges`
  is nil unless the file states it, and nil resolves to the default set. That
  is also how `config show` and the REST view know the origin. `[]` is stated
  and refuses nothing (D2).
- **What counts as malformed** (D3). Each of these fails loading, naming the
  key and the entry:
  - an entry that is neither an address nor a CIDR prefix;
  - a prefix with host bits set (`10.0.0.1/8`). A widened *exception* must not
    pass silently, and the rule is the same for both lists;
  - an IPv6 zone;
  - a list key with no value and no items, which asks the owner to write `[]`
    or remove the key;
  - a list value that is not a bracketed flow list.

  A bare address means its /32 or /128. Unknown keys under `security:` are
  ignored, as everywhere else in the loader.
- **D8 at load and at match.** An exception overlapping `127.0.0.0/8`,
  `0.0.0.0/8`, `::1/128` or `::/128` fails loading. So does an IPv6 exception
  overlapping those addresses' mapped, compatible or NAT64 forms. As defence
  in depth, `Check` never lets an exception permit those addresses in any form.
- **Wiring.** `media.NewPolicy`, `media.NewFetcher` and `localize.New` take an
  optional `media.WithAddressRanges`, and every production call site passes
  the loaded block. A caller that passes nothing gets the default set, so an
  omission errs strict, and a test proves that the service path carries a
  stated set. A policy built from ranges that fail to parse blocks every URL,
  giving the error as the reason.
- **Reasons.** A refusal names the range: `address 100.64.0.1 is in refused
  range 100.64.0.0/10`. The J8 address probe classifies refusals by today's
  wording (`private, loopback, or link-local`), so its classifier is updated to
  recognise the new reason. That edit is recorded in J28's README, since the
  probe is J8's evidence.
- **The REST view.** `media_policy` in status and `GET` of the media policy
  gain `refused_address_ranges`, `permitted_address_ranges`,
  `address_ranges_origin` and `warnings`, in `internal/api` and
  `api/openapi.yaml`.

**Scope.**

- **J28-A, the named default.** `SECURITY_AND_MEDIA_POLICY.md` replaces
  "private network and link-local" with the default refusal set above:
  - each range with its purpose and source;
  - the registry date it was checked against;
  - the ranges deliberately left out and why;
  - how embedded IPv4 is treated.

  Owner decisions D1–D9 are recorded there.
- **J28-B, one helper at both checks.** One function in `internal/media`
  applies the effective set, the exceptions and the embedded-IPv4 rule. Both
  the static literal check and `checkDialAddress` call it, and neither keeps
  its own expression.
  - A table-driven test takes one address from every default range and checks
    it at both checks. It fails on today's code for the 27 measured
    addresses that belong to the default set; the other 2 of the 29 not refused
    today (`::ffff:8.8.8.8`, ORCHIDv2) should stay fetchable.
  - Cases prove that a registry block left out stays fetchable, that an
    embedded public IPv4 behind `64:ff9b::` is not refused, and that
    `64:ff9b::7f00:1` is.
  - Cases also cover: `::1` and `::` still refused as themselves; an exception
    permitting a private range and its embedded forms (D9); an exception never
    permitting loopback or "this host" (D8); and `allow_private_networks: true`
    switching off both checks (D5).
- **J28-C, the configuration block.** The loader reads the block chosen in D1.
  - An absent block yields the default set; a stated block replaces it (D2).
  - A malformed entry in either list fails loading and names the entry (D3),
    as does an exception overlapping loopback or "this host" (D8).
  - `config show`, and the REST configuration view that already exposes
    `allow_private_networks`, report the effective set and its origin.
  - Service start and `config show` warn about default ranges a stated set
    omits (D2), report exceptions, and say when `allow_private_networks: true`
    leaves the set inactive.
  - The frozen configuration surface is rebuilt with `build_freeze.py`, and the
    commit says what moved and that it is additive.
- **J28-D, the documentation.**
  - `docs/configuration.md` gains a tracked example, through
    `docs/docexamples/configuration.json`, showing a stated set and an
    exception, checked by `config show`.
  - The configuration guide says which version first reads `security:`, since
    older binaries ignore it.
  - `docs/service.md`'s key list and table, `config/config.example.yaml` (the
    block commented out, with the default named), `SECURITY_REVIEW.md`'s media
    claims, and `docs/troubleshooting.md` (a refusal message and what to
    change) are updated.
  - J8's README corrects "multicast" to "link-local multicast".
  - Pinned documentation evidence that moves (the G18a inventory, G18f hashes,
    the docaudit registry) is regenerated, not hand-edited.

**Boundaries.**

- No change to connection pinning, redirect handling or the proxy refusal,
  which already meet the references' tips.
- No hostname-based refusals beyond `localhost`; the connect-time check sees
  the resolved address.
- No change to sync's dialing (D6).
- The default set is checked against the IANA registries once, when J28 is
  implemented, and dated. Notrios does not fetch the registries at run time.

**Dependencies.** J8, which found it. All owner decisions, D1–D9, are made;
every slice can start.

**Working state.**
- The policy names a dated default set, which the configuration can replace
  and extend with exceptions.
- One helper applies the set at both checks.
- Every default range has a case at both checks.
- The documentation shows how to state the set.

## J29. Decide which HTML reference forms the remote-media scanner covers — complete

**Goal.** What the scanner is responsible for finding is stated, and it finds
it.

**What J8 found** (`performance/v1.0-j8/README.md`, finding J8-F3). `ScanBody`
finds Markdown embeds and links and quoted HTML `<img src="…">`, including
across newlines and in either case. It does not find an unquoted
`<img src=https://…>` or a `<video src="…">`. A reference the scan never
returns is never evaluated by any later stage.

**Owner decisions, 2026-09-17.**
- `<video>` and the other media elements are in the scanner's scope.
- Inline images are not remote. A `data:` URI such as
  `data:image/png;base64,…`, which Joplin renders, and inline SVG are fine.

  The owner then named the form:
  `!\[.*?\]\(data:image\/(?:png|jpeg|jpg|gif|webp|svg\+xml);base64,[A-Za-z0-9+/=]+\)`.
  So the scanner does not report a base64 `data:` URI of a PNG, JPEG, GIF, WebP
  or SVG image, whether in Markdown or in an HTML attribute. Today it
  deliberately reports such a Markdown embed as blocked, and that changes.
  - Any other `data:` URI, such as `data:text/html` or a non-base64
    `data:image/svg+xml,`, is still reported and blocked, the conservative
    reading of the named form.
  - `file:` references are still reported and blocked.
  - The explicit URL check (`EvaluateURLs`) still refuses a `data:` URL, since
    nothing fetches one. The lint check for unlocalized remote media already counts only
  `http:` and `https:`, so it does not change.

**Design, 2026-09-17.** Written before the code.

- **Covered forms.** An HTML element in a note body, any case, whose attribute
  names a URL a renderer loads as media:

  | element | attributes |
  |---|---|
  | `img` | `src`, `srcset` |
  | `source` (in `picture`, `video` or `audio`) | `src`, `srcset` |
  | `video` | `src`, `poster` |
  | `audio` | `src` |
  | `track` | `src` |
  | `embed` | `src` |
  | `object` | `data` |
  | `image` (SVG) | `href`, `xlink:href` |

- **How it reads them.** Not an HTML parser, and no new dependency.
  - A pattern finds where each covered element's start tag begins.
  - A small attribute reader then follows HTML's rules for that one tag:
    double-quoted, single-quoted and unquoted values; values spanning lines;
    a `>` inside quotes; entities decoded with the standard `html` package.
  - `srcset` is split into its candidate URLs and descriptors.
  - Each URL is reported as embedded media, at the line its tag starts, and
    deduplicated as today.
- **What it does not promise**, written into `SECURITY_AND_MEDIA_POLICY.md`:
  - CSS: `style` attributes, `<style>` and `url()`;
  - documents: `iframe`, `frame`, `srcdoc`;
  - `link` preloads and icons, `meta` refresh, `input type=image`;
  - sources a script or a renderer extension builds;
  - HTML the renderer would treat differently from the text, such as a tag
    split by a comment.
  - HTML inside a code span or fence is reported although it does not render,
    which errs towards reporting.
  - A URL written with entities is evaluated decoded, but localization rewrites
    only literal occurrences of it, so such a reference is reported and not
    rewritten.
- **Proof.** Tests fail on today's code for an unquoted `img`, `srcset`, every
  element and attribute above, and a Markdown `data:` embed that is no longer
  reported. They also cover quoting, case, line
  numbers, entities, `srcset` descriptors and a `>` inside a quoted value, and
  the existing Markdown and quoted-`img` results stay unchanged.

**Found while designing, and deferred to J34 rather than fixed here.** The GUI
preview's sanitizer (`web/src/preview-utils.tsx`) neutralises remote `img`
sources but keeps remote `video` and `audio` `src`, `video` `poster`, `track`
`src` and SVG `image` `href`, as a probe of the function itself showed. The
served UI's Content-Security-Policy has no `media-src`, so remote audio and
video fall back to `default-src 'self'` and are blocked. `poster` and SVG
`image` are image loads, though, and `img-src` allows remote `http:` and
`https:`.

**Scope.**

- **J29-A, the covered forms.** The policy states the covered forms and what
  the scan does not promise, the scanner finds every covered form, and the
  tests above pass.

**Boundaries.** The scan is not replaced with an HTML parser; the reason is
recorded above. The preview sanitizer is J34's, not this item's.

**Dependencies.** J8, which found it.

**Working state.** A stated list of covered forms, a scanner that finds them,
and a policy that says what is not promised.

## J30. Stop a lying header deciding the type of an inconclusive payload — complete

**Goal.** A resource's recorded type describes its bytes.

**What J8 found** (`performance/v1.0-j8/README.md`, finding J8-F4). Bytes that
sniff as `text/plain` and are served with `Content-Type: image/jpeg` are
quarantined as an image, with that type recorded and a `.jpg` extension on the
quarantine file. The fallback exists for a real case — SVG sniffs as
`text/xml` — and applies whenever the sniff is inconclusive, so any text
payload can be labelled an image. A positive sniff still beats the header.

The blast radius is bounded: bytes are stored content-addressed under their
hash, nothing executes them, and admission still applies exact-hash rules.

**Measured before starting, 2026-09-17.** Go 1.27's `http.DetectContentType`
was run on a real header of every extension the policy treats as media.

- **Sniffed positively, so they need no fallback:** PNG, JPEG, GIF, WebP, BMP,
  ICO, MP4, M4V, M4A, WebM, MKV, AVI, WAV, MP3 with an ID3 tag, and PDF.
- **Sniffed as inconclusive, so today they localize only through the header:**
  - SVG with an XML declaration (`text/xml`) or without one (`text/plain`);
  - TIFF, AVIF, HEIC, QuickTime MOV, FLAC, and an MP3 that starts with a frame
    rather than an ID3 tag (all `application/octet-stream`).

  Narrowing the fallback must keep every one of these working, or it changes
  what counts as localizable media, which the boundary rules out.

**Scope.**

- **J30-A, a fallback that checks the bytes.** When the sniff is inconclusive,
  the header is consulted only for the named types in the table below, and
  only when the payload carries that format's own signature. Any other claimed
  media type, or a payload without the signature, is refused with a reason
  naming both the sniffed and the claimed type. A positive sniff still wins, as
  today.

  | claimed type | sniff it may follow | signature required in the first 512 bytes |
  |---|---|---|
  | `image/svg+xml` | `text/xml`, `text/plain` | after an optional BOM, whitespace, XML declaration, processing instructions, comments and doctype, the first element is `<svg` |
  | `image/tiff` | `application/octet-stream` | `II*\0` or `MM\0*` |
  | `image/avif` | `application/octet-stream` | an `ftyp` box whose major or compatible brand is `avif` or `avis` |
  | `image/heic`, `image/heif` | `application/octet-stream` | an `ftyp` box with a brand in `heic heix heim heis hevc hevx mif1 msf1` |
  | `video/quicktime` | `application/octet-stream` | an `ftyp` box with brand `qt  `, or a `moov`, `mdat` or `wide` atom at offset 4 |
  | `audio/mpeg` | `application/octet-stream` | an MPEG audio frame header: sync bits, a valid version and layer, and a bitrate index that is not 15 |
  | `audio/flac`, `audio/x-flac` | `application/octet-stream` | `fLaC` |

  Tests prove each of these still localizes with a real signature. They also
  prove that text claiming `image/jpeg`, text claiming SVG without an `<svg`
  root, and arbitrary bytes claiming each named type are all refused, and that
  nothing is left in quarantine. The refusal tests fail on today's code.

  J8's probe (`TestJ8AServerThatLiesAboutItsContent`) needs no edit: its J8-F4
  case then passes as expected.

**Found while measuring, and deferred to J33 rather than fixed here:**
- `.ogg` sniffs as `application/ogg`, which is positive and not a media type
  the policy accepts, so Ogg files are refused today although the policy lists
  `.ogg` as media;
- an SVG that opens with a comment sniffs as `text/html`, which is positive
  and wins, so it is refused.

**Boundaries.** No change to the sniffing library or to what counts as
localizable media beyond this rule.

**Dependencies.** J8, which found it.

**Working state.** SVG still works, a text payload claiming to be an image is
refused, and both are proven by tests that fail on today's code.

## J31. Bring the vendored Ledger theme up to its Bluge result-URL fix — complete

**Goal.** The documentation site's copy of `hugo-theme-ledger` is the upstream
commit that carries the Bluge result-URL fix, and the checks that pin the copy
say which commit it is instead of refusing any change to it.

**What changed upstream** (2026-09-16). `hugo-theme-ledger` commit `85ac1b0`,
merged to `main` as `cf68886`, resolves Bluge search results under a site
subpath: stored site-root-relative URLs are prefixed with the configured
`siteRoot`, Hugo's already-prefixed `.RelPermalink` values are left alone, and
absolute destinations pass through. From `f9d28ea` — the commit Notrios vendors
— to `cf68886` it changes two files: `assets/js/search/backends/bluge.js` and
its test. Only `bluge.js` is in the vendored 44-file runtime snapshot; the
snapshot carries no tests.

**Measured before planning.** The site as committed builds and passes
`make g18g-validate`. With the new `bluge.js` swapped into a scratch worktree
it also builds, the G18g unit tests pass, Pagefind search returns results in a
browser with no console errors or warnings, and the theme's own 16 JavaScript
tests pass. The only refusal is the evidence validator's: `production theme
differs from frozen snapshot file or hash`.

**What this does and does not change for Notrios.** Notrios searches with
Pagefind (`backend = "pagefind"`), so no reader of today's site sees a
different result. The bundled `ledger-search` script does change, because every
backend is bundled. The reason to take it is that the vendored copy should be
an upstream commit that carries its known fixes, not a commit that predates
one, and a site that later enables Bluge under a subpath would otherwise ship
the bug.

**Why this is its own item.** The pin is not one number. `validate_evidence.py`
for G18g requires the production copy to be byte-identical to the frozen G18b
prototype snapshot and requires the upstream commit `f9d28ea` both in
`docs-site/THEME_PROVENANCE.json` and in G18g's evidence bundle. G18b's
snapshot and G18g's bundle are v0.7 evidence of what was reviewed then; editing
them to admit a new commit would rewrite that record. So the check has to be
split before the copy can move.

**Scope.**

- **J31-A, the pin split.** The frozen G18b snapshot and G18g's evidence bundle
  keep validating exactly as they do today, unchanged. The production copy
  under `docs-site/` is validated against its own
  `docs-site/THEME_PROVENANCE.json` — upstream commit, file list, byte count
  and manifest hash — rather than against the frozen snapshot. A test proves the
  split both ways: a production file that differs from its provenance fails,
  and a frozen-snapshot file that differs still fails.
- **J31-B, the refresh.** The copy is taken from a fresh clone of
  `https://github.com/renesugar/hugo-theme-ledger` at `cf68886` (or whatever
  `main` holds when the item starts, recorded exactly), not from a local
  working copy. Only the snapshot's existing file list is copied; a file
  upstream added to the runtime tree is reported, not silently vendored.
  `docs-site/THEME_PROVENANCE.json` records the new commit, counts and
  manifest, and names the upstream commits between the two pins.
  `DOCS_SITE.md` and `docs-site/README.md` name the new commit; records that
  describe v0.7 as it was (`CODING_CLIENT_HANDOFF.md`'s G18b account,
  `agent/PLAN_STATUS.md`) are left as history.
- **J31-C, the site still works.** `make g18g-validate` passes on the refreshed
  copy, G18g's browser smoke passes, search returns results with no console
  errors, and the theme's own tests are run from the pinned clone — they are
  not vendored — with the count recorded in the item's README.

**Boundaries.** No other theme change, no Notrios layout override added or
removed, Hugo, Node and Pagefind pins unchanged, and no search backend change:
Notrios stays on Pagefind. Nothing is pushed to the theme repository. v0.7
evidence stays byte-identical.

**Dependencies.** G18b and G18g, whose pins this splits.

**Working state.** `docs-site/themes/hugo-theme-ledger` is the recorded upstream
commit with the Bluge fix, its provenance says so, the frozen v0.7 snapshot is
untouched, and every docs-site check passes.

## J35. Make G18g's browser smoke runnable again — complete

**Goal.** The documentation site's browser evidence can be produced by running
the documented command.

**What J31 found, 2026-09-17.** `performance/v0.7-g18g/browser_smoke.mjs` does
not finish in this environment, on the vendored theme **before or after** J31's
refresh, so the failure is not the refresh's:

- It stops at the theme menu, clicking `[data-theme-value="light"]`, which
  Playwright reports as "element is not visible" until it times out.
- Its documented base URL is a `/notrios/` subpath, but the site is built with
  `baseURL = "https://notrios.com/"`, so the built pages reference absolute
  root paths. Served under a subpath, Pagefind's assets 404 and the search page
  returns nothing. A probe that loads the search page under `/notrios/` finds
  no results for that reason, and the same probe at the root finds them.

**Owner explanation, 2026-09-17.** The `/notrios/` subpath is a leftover from
GitHub Pages. The site is served at `notrios.com` once changes reach `main`, so
the apex build served at the root is the real thing, and the smoke's subpath
default is simply out of date. The comments lag because a local build cannot
test the `notrios.com` deployment. `ROADMAP.md` already records the apex move
(v0.9 I10) and that a build is correct only for the `baseURL` it was built
with.

**Measured, 2026-09-17.** `https://renesugar.github.io/notrios/` answers
**301 Moved Permanently** to `https://notrios.com/`. Nothing is served at the
subpath at all, so a subpath build has no address today. The Pages address
would serve pages again only if the custom domain were given up, and that
would need its own `baseURL` build — which is why no document should promise
one build serves both.

So J35-B is not a choice between two options: the smoke is corrected to the
root, and the leftovers say so. They are:
- `performance/v0.7-g18g/browser_smoke.mjs`'s default
  `http://127.0.0.1:18618/notrios/`;
- `performance/v0.7-g18g/README.md`'s instructions;
- `DOCS_SITE.md` (two mentions of preserving `/notrios/` URLs);
- `FEATURE_MATRIX.md`'s "preserve `/notrios/` links".

A Pages-shaped fallback build needs its own `baseURL`, which `ROADMAP.md`
already explains, so nothing here promises one build serves both addresses.

J31 proved search on the refreshed theme with its own probe
(`performance/v1.0-j31/search_probe.mjs`), which is narrower than the smoke.

**Scope.**

- **J35-A, a smoke that runs.** Establish why the theme control is not visible
  to a click (a menu that must be opened first, a viewport or animation
  assumption, or a genuine regression), and fix the script or the theme so the
  documented command completes. Record which it was.
- **J35-B, the base URL it is run at.** The smoke, G18g's README,
  `DOCS_SITE.md` and `FEATURE_MATRIX.md` describe the address the site is
  actually served at: the apex, at the root. The Pages subpath is named as the
  historical address rather than as what a run expects: it 301-redirects to the
  apex, so no document should claim a single build serves both, and none should
  imply the subpath is live.

- **J35-C, a check that catches the next drift.** The owner's point is worth
  more than the four corrections: a local build cannot exercise the
  `notrios.com` deployment, so a statement about the served address drifts with
  nothing to catch it. A check reads the served address from the one place that
  decides it — `docs-site/hugo.toml`'s `baseURL` — and fails when a tracked
  file names a base address that contradicts it.
  - It covers repository documents, the G18g evidence scripts and their README,
    and the build and smoke scripts' own defaults.
  - A historical mention is allowed, but only when it is listed with its
    reason, the way other pinned evidence is listed: `ROADMAP.md`'s record of
    the I10 move and the Pages redirect are history, not claims about today.
    An unlisted mention fails, and the failure names the file, the line and the
    address the build actually serves.
  - It runs with the checks that need no browser, so it fails in `make
    validate` rather than only at packaging time.
  - A test proves it both ways: a document naming the subpath as current
    fails, and the same document naming it as history, listed, passes.

**Boundaries.** No change to what the site publishes, and no relaxing of the
smoke's checks to make it pass. If a check cannot run here, it says so rather
than being deleted. The check reads the address from the build's own
configuration; it never reaches the network, so it cannot confirm what
`notrios.com` serves — only that the repository agrees with itself.

**Dependencies.** J31, which found it.

**Working state.** The documented command runs to completion and reports its
measurements, or G18g's README says exactly what cannot run and why; every
statement about the served address agrees with the build's `baseURL`, and a
check fails when one stops agreeing.

## J32. Investigate the import performance review's findings, and keep only what measurement shows is faster

**Goal.** Every finding in `performance-code-review.md` is investigated. A change
ships only if a benchmark, written before the change, shows it improves import
performance and every correctness gate still passes. Every finding ends with a
recorded disposition, and a finding whose measurement shows no gain is
reverted, not kept.

**Why.** The review (2026-09-17, a static snapshot by another agent) found
candidates in the Joplin RAW and Obsidian importers, their shared SQLite write
path, and the Markdown parsers. It ran no benchmark: its numbers come from J17,
J19 and J20, and it says so. Its key citations were checked against the code
before this item was written:
- `execPreparedLocked` prepares and finalizes on every call;
- `PropertyOrder` is built whether or not source is preserved;
- `appendUnique` scans linearly;
- `lineColumn` rescans the body from the start for every link;
- Obsidian splices replacements into the body one at a time.

J17's history is the reason for the rule. Its first batching result (1.46×) was
contaminated by concurrent work, and the corrected figure (2.2×) differed. J19
measured three plausible ideas as no-ops.

**What "shows it improved" means, fixed before any candidate is measured.**
- **Runs.** Baseline and candidate alternate on the same machine, the same
  disk, the same binary build flags and the same generated corpus. There are at
  least five measured runs of each after a warm-up, and nothing else runs:
  no tests, builds, reviews or other imports.
- **The metric** is declared in the slice before the change: wall time,
  notes/s, peak RSS, live heap after GC, allocations, or prepared-statement
  count.
- **Improved** means the candidate's median beats the baseline's median by
  more than the baseline's own range (maximum minus minimum) on the declared
  metric. A 1% median win inside 5% run-to-run spread is not an improvement.
- **No regression elsewhere.** The other recorded metrics of the matrix cases
  do not get worse by more than the baseline range. A memory gain that costs
  wall time beyond noise is a trade-off for the owner, not a ship.
- **Scaling findings** may be proven on the input shape they target, such as a
  link-dense note or a collision-heavy namespace, but must be neutral within
  noise on the ordinary corpus.
- **Correctness first.** A candidate that fails a correctness gate is not
  measured further.

**Correctness gates** (the review's §6, applied to every candidate):
- **Content:** identical canonical bodies, titles, notebook placement,
  provenance, tags, attachment references, source-bundle bytes and hashes, and
  property order.
- **Links:** identical byte spans, rune columns, contexts, anchors and
  resolution.
- **Blocks:** identical IDs, occurrence numbers, hashes, slugs and offsets.
- **Resume:** identical inventory fingerprints and checkpoint positions, and
  resume works after an interruption at each phase.
- **No-op reimport** creates no revisions or projection jobs.
- **Search:** results are visible immediately after each committed batch.
- `performance/v1.0-j17/j17_compare.py` shows no unexplained difference between
  independently imported libraries.
- The full `go test ./...` passes through `scripts/check_temp_leaks.sh`.

**Scope.** Each candidate slice records its hypothesis, declared metric,
baseline, candidate result and disposition. It is independent of the others,
and a candidate is measured against the baseline in force when it is tested,
so an earlier kept change is part of that baseline.

- **J32-A, the harness and the baseline.** A reproducible synthetic matrix,
  generated and never private:
  - Joplin RAW and Obsidian corpora at 10k notes from the existing profile
    scripts, fresh import and no-op reimport;
  - a link-dense corpus (notes of about 1 MiB with many links, including long
    single-line notes);
  - a collision-heavy namespace (many files named `index.md` in different
    folders, and repeated aliases);
  - notes near the 64 MiB limit.

  A prepared-statement counter is added, test-only or behind the existing
  `ImportMetrics`, so F1 can be measured by count as well as by time. The
  baseline is recorded with dispersion before any candidate. The larger
  scales (100k, 382,206-equivalent) are run only for candidates that pass at
  10k, because each full run takes over an hour.

  How the harness is built, decided 2026-09-22 before any code:
  - **One run, one process.** Peak RSS is a per-process maximum, so a run that
    shares a process with another inherits its peak. Each measured import is a
    fresh process of a test binary compiled once per build under measurement
    (`go test -c ./cmd/notriosctl`). It calls the CLI's own `import` command
    in-process, so what is timed is the command a user runs, flag parsing and
    all. `/usr/bin/time` supplies peak RSS and CPU; the process itself
    supplies wall time around the command, `runtime.MemStats` (bytes and
    objects allocated, and live heap after a forced GC), and the
    prepared-statement count.
  - **The counter** is a process-wide atomic incremented in `prepareLocked`,
    the store's only `sqlite3_prepare_v2` call site, and read through an
    exported function. One atomic add beside a prepare costs nothing a prepare
    does not already dwarf. No new flag or environment variable reaches the
    shipped CLI: the test binary reads its arguments from its own environment
    and is skipped in an ordinary `go test`.
  - **Corpora** are generated by a script in `performance/v1.0-j32/` from a
    fixed seed, into a work directory outside the checkout (HD2 here), and each
    records a manifest hash so a baseline and a candidate are shown to have
    read the same bytes. The Joplin and Obsidian shapes are the existing
    profile tests' shapes at 10k. The link-dense, collision and near-limit
    shapes are new, and small enough that a run takes seconds to minutes.
  - **Scenarios:** a fresh import into an empty library, and a no-op reimport
    of the same corpus into the library that fresh import produced (copied
    first, so each measured reimport starts from the same bytes).
  - **The runner** does one unmeasured warm-up and then at least five
    measured runs per case. When comparing, it alternates baseline and
    candidate binaries run by run. For each metric it records every value and
    the median, minimum, maximum and range, plus the load average before each
    run, and it refuses to start while the one-minute load is above 1.
    It writes one JSON file per case; `compare.py` applies the rule above and
    prints the verdict per metric.
  - **The baseline is `develop` at the commit that adds the harness**, so it
    measures exactly the code every candidate starts from.
- **J32-B, F1: reuse prepared statements** for the repeated block and link
  inserts, batch-scoped, reset and rebound as `import_manifest.go` already
  does, and finalized on every exit. Metrics: prepared-statement count and
  import wall time.

  Decided before the change, 2026-09-22:
  - **The two loops, and only those.** `rebuildDocumentBlocksLocked` and
    `rebuildDocumentLinksLocked` call `execPreparedLocked` once per block and
    once per link, and `execPreparedLocked` prepares and finalizes on every
    call. Each gains one statement prepared before its loop, reset and
    rebound per row, finalized on every exit including an error. The other
    236 `execPreparedLocked` call sites are not touched: they are not in a
    loop, and a change that helps nothing is a change to revert.
  - **Not a statement cache.** A cache living across transactions would
    outlive a schema migration and a restore, which is a correctness risk for
    a gain this slice has not yet shown. If the measurement makes the case for
    one, it is proposed as its own item with that evidence.
  - **The declared metrics** are the prepared-statement count and wall time on
    `obsidian-10k/fresh`, the ordinary corpus. `link-dense/fresh`, where a
    note holds thousands of links, is measured as the scaling case and must
    not regress; the baseline's own range is the bar in both.
  - **Correctness before measurement:** the full `go test` through
    `scripts/check_temp_leaks.sh`, and `j17_compare.py` between a library the
    baseline binary imported and one the candidate imported, which must differ
    in nothing but volatile columns.
- **J32-C, §3.1B: build `PropertyOrder` only when source is preserved.**
  Metrics: live heap and peak RSS on a property-rich vault. Preserve-source
  output must stay byte-identical.

  Checked before the change, 2026-09-22:
  - **Only Obsidian is unconditional.** Joplin's scan already drops
    `PropertyOrder` unless `retainBundleInventory` is set
    (`internal/importers/joplinraw/scalable.go`). Obsidian's `readInventory`
    parses and clones it for every note, and the only reader is
    `processSourceBundle`, which runs under `options.PreserveSource`. So the
    gate is the `retainFiles` parameter `readInventory` already takes, which
    J20 added for the same reason.
  - **Resume is not affected.** The inventory fingerprint is built from each
    file's item key and content hash, not from its parsed properties, so a
    gated field cannot change a fingerprint or a checkpoint position.
  - **A property-rich corpus** is added to the matrix: 10,000 notes with about
    25 frontmatter properties each, which is what makes the retained slices
    worth measuring. The declared metrics are live heap and peak RSS on
    `property-rich/fresh` with preserve-source off, which is the CLI default
    and the case the gate is meant to help.
  - **Preserve-source is the correctness gate,** not a metric: the same vault
    imported with `--preserve-source` by the baseline binary and by the
    candidate must produce libraries that `j17_compare.py` finds identical
    apart from volatile identifier columns, so the recorded property order is
    unchanged where it is kept.
- **J32-D, §3.1A: assemble rewritten Obsidian bodies in one pass.** First pin
  the current behaviour for overlapping and mixed Markdown and wiki matches in
  a test, then change only the assembly. Metrics: allocations and wall time on
  the link-dense corpus.

  Checked before the change, 2026-09-22:
  - **What it does now.** `rewriteObsidianLinks` collects replacements, sorts
    them by start descending, and splices each one into the body with
    `body[:start] + text + body[end:]`. Every splice copies the whole note, so
    a note with thousands of links copies itself thousands of times. J32-A
    measured 11 GB allocated to import four 1 MiB notes.
  - **Overlaps exist and must be pinned first.** `markdownlinks.Extract`
    returns Markdown matches and then wiki matches, each ascending, so the
    list is not in position order, and a wiki link inside a Markdown link
    overlaps it: in `See [label]([[Target]]) here.` the Markdown match covers
    bytes 4-23 and the wiki match 12-22. Whether both become replacements
    depends on resolution, so the test pins what today's code writes for
    nested, adjacent, repeated and multi-byte cases before anything changes.
  - **The change is the assembly only.** What is recognised, what resolves and
    what each replacement's text is all stay as they are. One pass over the
    body in ascending order, writing into a builder sized from the body and
    the replacements.
  - **The declared metrics** are allocations and wall time on
    `link-dense/fresh`. `obsidian-10k/fresh` is the ordinary corpus that must
    not regress, and the canonical bodies it produces must be identical, which
    `j17_compare.py` shows by the `documents` and `document_links` tables.
- **J32-E, F8: one-pass link coordinates** in `markdownlinks`, keeping byte
  offsets, rune columns and result order. Metric: parse time on the
  link-dense corpus, including long single-line notes.

  Checked before the change, 2026-09-22:
  - **What it does now.** `decorateCandidate` calls `lineColumn`, which
    decodes the body from byte zero for every candidate. A note with
    thousands of links decodes itself thousands of times, and a note that is
    one long line is the worst case, because a column is counted in runes
    since the last newline.
  - **The change.** `Extract` finds Markdown matches and then wiki matches,
    each ascending by offset, so each pass can carry a cursor forward instead
    of restarting: line, column and offset advance from the previous
    candidate to the next. A cursor asked for an offset behind it recomputes
    from the start, so the result cannot depend on the order it is asked.
  - **Unchanged:** what is recognised, the order `Extract` returns, byte
    offsets, rune columns, line numbers and contexts. A test compares the
    cursor with today's `lineColumn` at every candidate offset and at random
    offsets, on bodies with multi-byte text, CRLF, no trailing newline and no
    newline at all.
  - **The declared metric** is wall time on `link-dense/fresh`, whose corpus
    holds both multi-line and single-line notes of about 1 MiB.
    `obsidian-10k/fresh` must not regress, and both corpora must produce
    identical `document_links` rows, which is where offsets, columns and
    contexts are stored.
- **J32-F, F9: collision-heavy namespace construction.** Metrics: namespace
  build time on the collision corpus, and memory on the ordinary corpus, where
  sets can cost more.

  Measured before changing anything, 2026-09-22, and **not a cost**:
  - A CPU profile of a fresh collision import shows `buildLinkNamespace` and
    `appendUnique` below the profiler's 0.38 s reporting threshold, under 0.5%
    of 77 s of samples.
  - A benchmark at the corpus's shape (5,000 notes named `index.md`, 50
    aliases shared by 100 notes each) builds the namespace in 213 ms, against
    223 ms for the same 5,000 notes with no shared alias at all. Colliding
    names make it slightly *cheaper*, because fewer distinct map keys offset
    the linear scans. So the premise — that duplicate scans cost on a
    collision-heavy vault — does not hold at this size, and a set would be
    replacing 0.2% of an import with the memory the slice was written to
    avoid. No change ships.
  - What the same profile does show is in J32-N.
- **J32-N, the collection scope defeats the title index.** Found while
  measuring J32-F, 2026-09-22. `findDocumentByTitleLocked` resolves every link
  by title with `collection_id = COALESCE(NULLIF(?, ''), collection_id)`,
  which is not a constant the index can seek, so the query plan is
  `SCAN documents USING COVERING INDEX documents_title_idx` plus a temp B-tree
  for the ordering — a full index scan for every link. It is 46% of a fresh
  collision import, 35 s of 77 s, at 5,500 notes, and it grows with the
  library, not with the import.
  - **The change:** when a collection is given, the SQL says
    `collection_id = ?`; the "every collection" form keeps today's predicate.
    The scoping rule itself does not change, and the same helper stays the one
    place that writes it.
  - **Declared metric:** wall time on `collision/fresh`, with
    `obsidian-10k/fresh` as the ordinary case that must not regress.
  - **Correctness:** the resolution a link gets, including which links are
    ambiguous, must not change: `j17_compare.py` on both corpora, and the
    existing collection-scoping tests, which are what say the predicate still
    means the same thing.
- **J32-G, §3.1C: hash Markdown from the bytes already read** in the Obsidian
  inventory, with a bounded read. Metric: inventory wall time on warm and on
  explicitly dropped caches.
- **J32-H, F3: existence-only reads** where only presence is needed, through a
  typed store method that keeps deleted-note filtering. The notes-phase body
  comparison is not touched. Metrics: allocations and wall time.
- **J32-I, F2: reuse blocks in the final link pass** when they describe the
  current revision. This is an investigation first: the invariant that blocks
  and links describe the same body under cancellation and resume is proven
  before anything is measured. Metric: wall time on fresh imports.
- **J32-J, §3.1D: Obsidian checkpoint consolidation and stable-note skips**,
  with failures injected between commit and progress publication, and a
  provenance-repair case. Metrics: commit count and wall time on no-op and 1%
  changed reimports.
- **J32-K, F5: byte-bounded batches** and bounded rereads. Metric: peak RSS on
  the near-limit corpus. A single legitimate large note still imports.
- **J32-L, the smaller candidates.** Each is measured and dispositioned
  separately:
  - F7, Joplin's duplicate ID map;
  - F10, title splitting and the list-prefix match;
  - §3.2B, cached Joplin notebook paths, on a deep hierarchy;
  - §3.2A, compact Joplin resource representations, on a resource-heavy
    corpus;
  - benchmark-only experiments on disposable databases: `cache_size`,
    `temp_store`, and building the disposable manifest's indexes after
    inserts. `synchronous` is never relaxed on a canonical library.
- **J32-M, the record.** `performance/v1.0-j32/README.md` holds one row per
  finding: implemented with its measurement, rejected with its measurement and
  the revert, or not investigated with the reason. It also records any
  correctness defect found along the way. Such a defect is not fixed here; it
  becomes its own plan item, as J8 established. `performance-code-review.md`
  stays a dated snapshot; the record supersedes it by reference and does not
  edit it.

**Not in this item, unless an earlier slice's measurement makes the case
first:**
- the resource and source-bundle staging API (F4, F6), because it is a new
  store API;
- a disk-backed Obsidian namespace (§3.1E);
- a parallel parse pipeline (§4.4).

The review ranks each of these after the smaller changes. If a slice's results
make the case for one, it is proposed as its own item with that evidence.

**Boundaries.**
- No change ships on a projected, estimated or single-run result.
- No durability is relaxed on a user's library: WAL, `foreign_keys`, and
  inline FTS stay, the last by J19's owner decision.
- J19's measured no-ops (hash-buffer pooling, streamed fingerprint
  concatenation, comparison-conversion changes) are not re-proposed without new
  evidence.
- No parser grammar changes: a performance change keeps what is recognised.
- No private archive or vault content enters the repository or evidence;
  profiles come from synthetic corpora.
- Long timed runs check agent usage first and run detached with isolated
  scratch data, as J25 and J26 did.

**Dependencies.** J17, J19 and J20 for the measured history, and J22 for
per-instance temp isolation in the harness. Nothing depends on this item: it
does not block J9 or J10.

**Working state.**
- Every finding in the review has a recorded disposition backed by a
  measurement or a stated reason.
- Only changes whose benchmark showed an improvement beyond noise, with every
  correctness gate passing, remain in the code.

## J33. Decide whether Ogg media and comment-led SVG are localizable — complete

**Goal.** Two kinds of file the remote-media policy lists as media either
localize, or the policy says plainly why they do not.

**What J30 found while measuring, 2026-09-17.**
- **Ogg.** Go's `http.DetectContentType` sniffs an
  Ogg file (`OggS`) as `application/ogg`. That is a positive sniff, so the
  header is never consulted, and `classFromMIME` does not accept
  `application/*` other than PDF. `.ogg` and `.oga` are in the policy's media
  extensions, so a note linking an Ogg file is scanned as media and then
  refused at fetch with "content type application/ogg is not localizable
  media".
- **SVG that opens with a comment.** An SVG whose first bytes are `<!-- … -->`
  sniffs as `text/html`, which is also positive. The rule that a positive sniff
  beats the header is what refuses it, and that rule exists to stop HTML
  claiming to be an image.

**Owner decisions, 2026-09-17.** Both as recommended.
- **Ogg localizes**, as the audio and video class. The sniff positively
  identifies a media container, and Ogg carries only audio and video.
- **An SVG that opens with a comment stays refused.** The rule that a positive
  sniff beats the header is what stops HTML claiming to be an image, and it
  stays absolute.

**Measured, 2026-09-17.**
- `application/ogg` has the extension `.ogx` by default, so the quarantine
  file would be named `.ogx`. `.ogg` is preferred instead, as J30 already
  prefers `.jpg` over the alphabetical first.
- An Opus file (`OggS` … `OpusHead`) also sniffs as `application/ogg`.
- `.ogv` and `.opus` are **not** in the media extension list, although both are
  Ogg media, so a plain link to one is not scanned as media today.

**Scope.**

- **J33-A, Ogg localizes and comment-led SVG does not.**
  - `application/ogg` is classified as the audio and video class, with `.ogg`
    as its quarantine extension.
  - `.ogv` and `.opus` join the media extensions, so a link to either is
    scanned like `.ogg`, with the same size cap.
  - A payload whose bytes sniff as `text/html` is refused whatever it claims,
    including an SVG that opens with a comment. A test states that plainly, so
    a later reader sees it was decided rather than missed.
  - `SECURITY_AND_MEDIA_POLICY.md` records both decisions.
  - The Ogg tests fail on today's code.

**Boundaries.** No change to the positive-sniff rule for HTML, and no new media
class.

**Dependencies.** J30, which measured both.

**Working state.** Each case either localizes with a test proving it, or is
refused with the policy saying why.

## J34. Stop the preview loading remote images through media elements — complete

**Goal.** A note's remote media never loads in the preview until it is
localized, whichever element names it.

**What J29 found while designing, 2026-09-17.** Run on the real
`normalizePreviewHTML`, these inputs come back unchanged:
- `<video src="https://…" poster="https://…">`;
- `<audio src="https://…">`;
- `<video><track src="https://…"></video>`;
- `<svg><image href="https://…"/></svg>`.

An unquoted remote `<img src=…>` comes back as the inert placeholder, as
intended.

The UI's Content-Security-Policy has no `media-src`, so remote audio, video and
track loads fall back to `default-src 'self'` and are blocked. `img-src`
allows remote `http:` and `https:`, because a remote `img` is neutralised
before it reaches the page, and neither `poster` nor SVG `image` is
neutralised. So a `poster` or an SVG `image` in a note can make the preview
fetch a remote image, bypassing the media policy the placeholder exists to
enforce.

**Not examined yet:**
- whether the Markdown renderer passes such HTML through to the sanitizer
  unescaped;
- whether the desktop webview enforces the policy header it is served.

The first step is to establish both, with a rendered note.

**Measured before starting, 2026-09-17.** The two open questions were settled
on the real UI by `performance/v1.0-j34/run_preview_probe.sh`.

- **How it ran.** The script built the web UI and started `notriosd` on an
  in-memory library with isolated roots. A second local server, on another
  port, stood in for "remote" and logged what it received. The script created
  a note using every J29-covered form plus two inline images, and opened it in
  headless Chromium and WebKit (the engine of the desktop webview) through
  Playwright.
- **Findings** (`performance/v1.0-j34/BEFORE.json`):
  - **The renderer passes raw HTML to the sanitizer**, which renders it.
  - **In both engines the stand-in server received `/video-poster.png` and
    `/svg-image.png`.**
  - CSP blocked `video` and `audio` `src` as `media-src` violations, through
    the `default-src` fallback.
  - `track` was not requested (no track was enabled).
  - `embed`, `object` and `source` are already removed by the sanitizer.
  - Markdown and HTML `img` became inert placeholders.
  - Both inline `data:image/png;base64` images rendered.
- **Not driven:** the desktop app's own WebKitGTK webview. WebKit enforcing
  the CSP is the closest evidence available.

**Design, 2026-09-17.**

- **One classification in the sanitizer**, applied to every URL attribute it
  keeps, the one `img` already uses:
  - a `resource://` source is resolved to its content URL;
  - an inline `data:image/…` source is kept, by owner direction;
  - an `http:` or `https:` source is moved to a `data-remote-…` attribute and
    removed, leaving the element inert;
  - anything else is removed.
- **What it covers:**
  - `video` and `audio` `src`, and `video` `poster`;
  - `track` `src`;
  - `href` and `xlink:href` on every SVG element except `a`: J29's `image`,
    and also `feImage` and `use`, which load the same way. Link navigation
    keeps its own rule.
  - `embed`, `object` and `source` stay removed.
- **Inline SVG markup keeps rendering**, by owner direction.
- **The CSP gains an explicit `media-src 'self'`**, so the refusal no longer
  depends on the `default-src` fallback.
- **Proof, in two parts:**
  - Vitest unit tests of `normalizePreviewHTML` for every form, which fail on
    today's code.
  - The rendered-note probe rerun after the change, which must show no request
    reaching the stand-in server, no CSP violation left to rely on, and inline
    images still rendering.

  The probe is not part of `go test` or the web test run: like G18g's browser
  smoke, it needs a Playwright module and a browser this repository does not
  depend on. Its script and before/after reports are committed as evidence.

**Scope.**

- **J34-A, one rule for every media reference.** The sanitizer treats every
  covered element and attribute in J29's table as it treats `img`: remote
  sources become inert metadata, `resource://` sources are resolved, and
  `srcset` is dropped. Inline images keep rendering, by owner direction
  (2026-09-17): `data:image/…` sources, which Joplin renders, and inline
  `<svg>`. Only a remote reference inside them, such as an SVG `<image
  href="https://…">`, is neutralised. The CSP states `media-src` explicitly rather than
  relying on the fallback. A rendered-note test proves that no remote request
  is made for any form, and it fails on today's code.

**Boundaries.** No change to what localization accepts, and no relaxing of any
CSP directive.

**Dependencies.** J29, whose table names the forms.

**Working state.** No covered form loads a remote resource in the preview, and
the CSP says so explicitly.

## J36. Resolve Obsidian partial-path links the way Obsidian does

**Goal.** A vault link that names a note or attachment by a partial path, such
as `[[topic-00001/index]]`, resolves to the file it names when exactly one file
ends with that path, instead of being reported as ambiguous and left
unresolved.

**What J32-A found, 2026-09-22.** J32's collision corpus holds 5,000 notes
named `index.md`, each in its own folder under one of 50 area folders
(`area-01/topic-00001/index.md`), and links them as `[[topic-00001/index]]`.
The import reports every one of those links as
`ambiguous Obsidian link "topic-00001/index"` although only one file ends with
`topic-00001/index.md`.

The cause is `resolveNoteID` in `internal/importers/obsidian/obsidian.go`. It
tries two exact paths, one relative to the linking note's folder and one from
the vault root. If neither matches, it falls back to the base name alone
(`index`), which 5,000 notes share. No step matches a path by suffix, so a
partial path is either an exact relative or root path, or it is treated as a
bare name. `resolveAssetID` has the same shape for attachments
(`[[attachments/img.png]]` from another folder). No existing test covers a
partial path, which is why every gate passed.

Found while building a benchmark, so under the J8 rule it is recorded here and
not fixed in J32.

**Scope.**

- **J36-A, pin the rule and Obsidian's behaviour.** Confirm against Obsidian's
  own documented behaviour how a partial path resolves, including when several
  files end with it. Then write the rule down as tests that fail on today's
  code:
  - a partial path matching exactly one note resolves to it;
  - one matching several notes stays ambiguous, with the warning;
  - the match falls on folder boundaries, so `pic-00001/index` matches nothing;
  - the existing relative, root, base-name and alias resolution is unchanged.
- **J36-B, notes and attachments.** `resolveNoteID` and `resolveAssetID` gain
  the suffix step between the exact paths and the base-name fallback. The
  namespace gains whatever index makes that step cheap: a suffix scan per link
  over 5,000 same-named notes is the kind of cost J32-F measures.
- **J36-C, libraries imported before the fix.** Record what a reimport of an
  already-imported vault does:
  - which notes it revises, which should be only those whose rewritten body
    changes because a link now resolves;
  - whether a no-op reimport of an unaffected vault still creates no revisions.

**Boundaries.**
- No parser grammar change: `markdownlinks` recognises the same links.
- An ambiguous link stays unresolved with a warning. The importer never guesses
  between candidates.

**Dependencies.** None blocking. J32's collision corpus exercises this code.
Whichever lands second is measured against a baseline that includes the
other, as J32's scope already requires.

**Working state.** A partial-path link that names one file resolves to it, and
the tests say so.
