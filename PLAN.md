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
**9 items: 0 complete, 0 in progress, 8 not started, 1 deferred.**

| Item | State | Slices done | Outstanding |
|---|---|---|---|
| I1. Put v0.8 on GitHub, and reconcile the branches | not-started | 0/3 | 3 |
| I2. Migrate the desktop shell to Wails v3, or record the postponement | deferred | 0/3 | 3 |
| I3. Promote the Ubuntu installer through clean native environments | not-started | 0/3 | 3 |
| I4. Harden the destructive lifecycle, and decide the profile race | not-started | 0/4 | 4 |
| I5. Resolve signing, notarization and timestamping policy | not-started | 0/3 | 3 |
| I6. Generate and verify the release evidence set | not-started | 0/3 | 3 |
| I7. Soak, recover, and freeze the support matrix | not-started | 0/3 | 3 |
| I8. Freeze the 1.0 compatibility surfaces | not-started | 0/3 | 3 |
| I9. Write the release-grade operational documentation | not-started | 0/3 | 3 |

Nothing is half-finished.
<!-- notrios:generated:plan:progress:end -->

## I1. Put v0.8 on GitHub, and reconcile the branches

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

## I3. Promote the Ubuntu installer through clean native environments

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

## I4. Harden the destructive lifecycle, and decide the profile race

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

## I5. Resolve signing, notarization and timestamping policy

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

## I6. Generate and verify the release evidence set

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

## I7. Soak, recover, and freeze the support matrix

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

## I8. Freeze the 1.0 compatibility surfaces

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

## Decisions register

This is an index only; each decision is owned and explained inside its item.

| Decision | Owner | Status |
|---|---|---|
| Push `develop` to GitHub | I1 | **Blocking.** The owner authorizes the first external write |
| Merge the `develop`-to-`main` pull request | I1 | **Blocking.** The owner authorizes the merge, after review; method is a merge commit, already decided |
| Whether Wails v3 has released | I2 | **Taken 2026-09-09: it has not.** `wails/v3@latest` is `v3.0.0-beta.19`, so the migration is deferred to post-v1.0 and v1.0 ships on Wails v2 |
| The profile-registry validation race | I4 | Non-blocking default: fix it; refusing with a reason is the alternative, leaving it as a test workaround is not |
| Which platforms the frozen matrix claims | I7 | Non-blocking default: only what was executed here |
