# v0.8 installation lifecycle and delivery planning amendment

Date: 2026-08-31
Status: complete planning amendment; no v0.8 implementation item started

## Purpose

Expand the unstarted v0.8 plan from a broad installed-layout item into
independently approvable investigation, runtime-path, Make lifecycle, Ubuntu
package, GitHub-native Windows/macOS, installed integration, delayed pull
request, and final synchronization slices. Extend the v0.8-v1.0 roadmap to a
v1.0 GitHub installation that does not require source code or a development
environment.

This amendment changes plans and handoff state only. It does not implement
`make install`, `make uninstall`, `make purge`, an installer, a workflow, a
runtime path, or a migration. H0 remains the next incomplete and unapproved
item.

## Repository facts checked

- The current Makefile has source-tree `clean` and `clobber` targets but no
  `install`, `uninstall`, or `purge`.
- `docs/installation.md` documents manual copies to `$HOME/.local/bin` and
  correctly says the product is source-only.
- Current compiled defaults use checkout-relative `./data` paths and
  `config/config.example.yaml`; web assets resolve through explicit,
  current-working-directory, and executable-relative candidates.
- Existing user-path consumers include the XDG-aware profile registry,
  generated profile configs, sync keys, publication profiles, Recoll
  configuration/indexes, quarantine, catch-up/carrier/backup staging, desktop
  handler metadata, and profile-configured external database/asset paths.
- Existing CI and GUI compilation are Ubuntu-only. There is no Windows/macOS
  build or native installer evidence.
- `gh` 2.98.0 is installed.

## Reference corrections and accepted planning facts

- GNU defines conventional `install` and `uninstall` targets, lowercase
  directory variables, and strongly encourages `DESTDIR` staging. It does not
  define a standard `purge` target or a semantic `DRYRUN` variable. Therefore
  Notrios must specify and test its custom purge and preview contract rather
  than calling the supplied example a convention:
  [GNU standard targets](https://www.gnu.org/s/make/manual/html_node/Standard-Targets.html)
  and [GNU `DESTDIR`](https://www.gnu.org/software/autoconf/manual/standards/DESTDIR.html).
- XDG 0.8 defines distinct config, data, state, cache, and runtime bases,
  requires environment-provided paths to be absolute, and defines fallbacks
  when config/data/state/cache values are unset. The plan includes all five
  categories and refuses to silently treat `DESTDIR` as a user runtime root:
  [XDG Base Directory Specification 0.8](https://specifications.freedesktop.org/basedir/0.8/).
- Wails v2 can build production binaries and documents NSIS generation for
  Windows. Its Linux guidance states that end users need GTK3 and WebKitGTK
  runtime libraries and describes nFPM packaging. These are candidates, not
  proof that the present Notrios build packages or runs on another OS:
  [Wails v2 build](https://wails.io/docs/gettingstarted/building/),
  [Wails v2 NSIS](https://wails.io/docs/guides/windows-installer/), and
  [Wails Linux distribution support](https://wails.io/docs/guides/linux-distro-support/).
- nFPM currently supports native Linux formats including `deb` and also
  exposes experimental MSIX support. The plan therefore compares it for Linux
  but does not preselect it for all platforms:
  [GoReleaser nFPM configuration](https://www.goreleaser.com/customization/package/nfpm/).
- GitHub-hosted jobs can run on Ubuntu, Windows, and macOS VMs. A native runner
  makes the missing hardware testable, but a build artifact becomes a support
  claim only after native install/launch/upgrade/remove/reinstall evidence:
  [GitHub-hosted runners](https://docs.github.com/en/actions/how-tos/manage-runners/github-hosted-runners/use-github-hosted-runners).
- Pull requests are the review boundary for merging a source branch into a
  base branch. The plan keeps changes on `develop`, delays its push until native
  runners are needed, and uses a `develop`-to-`main` PR rather than direct
  `main` commits:
  [GitHub pull requests](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request).
- As of the planning check, Wails v3 is beta for desktop, Wails v2 remains the
  stable release, and Android/iOS are experimental. The existing separate v3
  spike remains appropriate:
  [Wails v3 status](https://v3.wails.io/status/).

## Safety contract added to the plan

- `uninstall` removes only validated installed-artifact manifest entries and
  leaves all mutable user state.
- `purge` backs up config/data/state by default, verifies the backup and an
  offline restore before deletion, disposes cache/runtime separately, and
  refuses unsafe or broad targets.
- `DRYRUN=1` lists exact installed files, mutable roots, exclusions, external
  refusals, backup destination, and action order without writes or prompts.
- `FORCE=1` only suppresses confirmation; it never disables validation or the
  default backup.
- `NO_BACKUP=1` skips backup and emits the deep irreversible-loss warning. It
  still prompts unless combined with `FORCE=1`.
- Only exact unset/`1` flag forms are accepted. Non-interactive purge without
  `FORCE=1` fails closed.
- External paths named by profiles are listed and may be backed up when safely
  attributable, but the default purge refuses to delete them automatically.
- `clean` and `clobber` remain development-tree operations and never become
  aliases for installed lifecycle cleanup.

## Delivery sequence added to the plan

1. H3 investigates and freezes paths, ownership, migration, and destructive
   lifecycle semantics.
2. H4 implements runtime path/assets/migration behavior.
3. H5 implements and fault-tests Make install/uninstall/purge.
4. H6a selects installer formats and tooling from exact evidence.
5. H6 produces and natively executes the Ubuntu-priority internal package.
6. H7 implements Windows/macOS native GitHub workflows and candidates; H8
   supplies the shared installed-integration harness and Ubuntu baseline.
7. H9-H11 retain credential-store, Wails v3 spike, and Android-emulator work.
8. H12 delays the first GitHub push until native runners are necessary, then
   pushes `develop`, opens the PR, and records native results.
9. H13 reconciles v0.8, packages internal evidence, and—only after explicit
   merge authorization—merges through the PR and back-synchronizes `develop`.

## Planning-time branch audit

Read-only remote/local checks on 2026-08-31 found:

| Ref | Commit |
|---|---|
| remote `main` | `265ef4ef84ea90f0e325522a3a4308a5804f122c` |
| remote `develop` | `26b0925c21b3ecc264c370936e42d4b973548b8d` |
| local `develop` before this amendment | `fd2192d1e830833fcf74b191bfc01851a61ed8bd` |

Remote `main` is an ancestor of local `develop`; local `develop` is zero behind
and 133 commits ahead of remote `develop`. No merge is needed at this observed
state, but H12 must fetch and repeat the audit because the remote can change.
No push, PR, merge, tag, release, installer upload, reserve/ISO write, or
physical burn occurred during this amendment.

## Roadmap outcome

- v0.8 creates internal, native-tested prerelease installers and the complete
  safe local lifecycle; Ubuntu is the priority, while Windows/macOS are
  evidence-dependent and may be explicitly postponed.
- v0.9 hardens clean-environment upgrades, purge backup/restore and races,
  signing/notarization, SBOM/provenance, pinned workflows, support claims, and
  release-grade end-user documentation.
- v1.0 publishes an owner-authorized GitHub Release with at least a supported
  signed Ubuntu installer. Windows/macOS appear only if their native and trust
  gates pass. A recipient can install and operate the supported product without
  the source repository or developer toolchain.

## Validation

- Read the complete installation-planning prompt and checked its unstable
  reference claims against current primary sources.
- Reviewed current Makefile, installation documentation, path consumers,
  workflow matrix, branch ancestry, and remote branch heads.
- Confirmed every new plan item contains goal, scope, boundaries, dependencies,
  working state, validation/evidence, and item-local open decisions where
  applicable.
- Confirmed H0 remains next and no implementation item was marked started or
  complete.
