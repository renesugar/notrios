# v1.0 J9: the release documentation, for the supported matrix

## J9-A: the product is 1.0.0

`internal/version` says `1.0.0`. It is the owner's decision (2026-10-21) to bump
it here, one item before the release, so that every instruction J9 writes is
run against the version it describes.

**Nothing compares this string.** Before changing it, every reader of
`version.Version` was found and read:

| reader | what it does with the version |
|---|---|
| `cmd/notriosctl` `version` | prints it |
| `internal/httpapi` status route | reports it |
| `internal/httpapi` MCP handshake | `serverInfo.version` |
| `cmd/notrios` About dialog | shows it |
| `internal/snapshotimage` | records it as `application_version`; admission requires only that it is non-empty and at most 64 bytes |
| `cmd/docplan` | prints it in the README status block |
| `internal/docrules` | refuses documentation that restates it as current |

The scripts read the version from source rather than from a binary:
- `scripts/build_deb.sh` names the package with it.
- `scripts/build_abi_artifact.sh` names the C ABI tarball with it.
- `scripts/lifecycle.py` records it in the install manifest.
- `release.yml` refuses a tag that disagrees with it.

A label in six places is what the version test's comment says. That is now
checked, not assumed.

**Version strings that mean something else were not touched:** database
schema 28, the sync protocol range, the archive format, the C ABI major, and
the profile and publish schema versions.

**What moved, and why each one:**
- `internal/version/version.go` and its test. The test is renamed from
  `TestVersionIsReleaseCandidate` to `TestVersionIsTheRelease`, and the claim
  registry follows it.
- `web/package.json` and `docs-site/package.json` with their lockfiles' root
  entries. The docs-site lockfile still said `0.7.0` while its manifest said
  `0.8.0`, a drift nothing checked. Both now say `1.0.0`. No dependency moved.
  G20's license record carries the two lockfiles' new hashes, and its checker
  accepts them: 39 Go modules, 365 npm packages.
- **The README status block said "the version is bumped when the release is
  cut, which is the last item in the plan"**, and it went false as soon as it
  was generated. The generator in `internal/docplan/progress.go` now says only
  what is true of every milestone: a milestone is the work in the plan, and the
  version is what the binaries built from it say they are.

**Evidence: a 1.0.0 package, and what it says when run.**

```sh
make deb
dpkg-deb -f dist/deb/notrios_1.0.0-1_amd64.deb Package Version Architecture
dpkg-deb -x dist/deb/notrios_1.0.0-1_amd64.deb <dir> && <dir>/usr/bin/notriosctl version
```

```
built dist/deb/notrios_1.0.0-1_amd64.deb
8ee29db5e6a9d34b77ddc4a0b68e7fb659829da1d280a1f538d4e4282ff77c73
Package: notrios
Version: 1.0.0-1
Architecture: amd64
1.0.0
```

**This workstation's package has no desktop binary**, because the host lacks
the webkit2gtk-4.1 headers. `build_deb.sh` prints `no desktop GUI` and goes on,
which is the behaviour J1 found and made the release workflow refuse. So its
`Depends` is `libc6 (>= 2.34)` alone. The owner then installed the headers
(`libwebkit2gtk-4.1-dev`, webkit2gtk-4.1 2.52.6), and `make deb` was run
again. The rebuilt package carries all three binaries (sha256 `83312c90…`),
and its `Depends` is identical to the workflow-built package's. J9-B derives
the runtime rows from it.

## J9-B: the supported rows are exact

`MATRIX.json` is v0.9 I7's matrix made exact. It is checked by
`record_matrix.py`, which runs in validation, and the installation page's
table is generated from it:

```sh
python3 performance/v1.0-j9/record_matrix.py derive --package <deb> --workflow-package <deb> --abi-artifact <tar.gz>
python3 performance/v1.0-j9/record_matrix.py --write    # the installation table
python3 performance/v1.0-j9/record_matrix.py            # the check
```

**The runtime rows come from the binaries.** `derive` runs `dpkg-shlibdeps`
on each binary, the computation `build_deb.sh` makes for the whole package, so
each row says what its own binaries need:

| binaries | needs | newest glibc symbol |
|---|---|---|
| `notriosctl`, `notriosd` | `libc6 (>= 2.34)` | GLIBC_2.34 |
| `libnotrios.so.1` | `libc6 (>= 2.34)` | GLIBC_2.34 |
| `notrios` | `libc6`, `libgdk-pixbuf-2.0-0`, `libglib2.0-0t64`, `libgtk-3-0t64 (>= 3.21.4)`, `libjavascriptcoregtk-4.1-0`, `libsoup-3.0-0`, `libwebkit2gtk-4.1-0 (>= 2.39.91)` | GLIBC_2.34 |

The 1.0.0 package built here declares exactly the `Depends` of the
workflow-built 0.8.0 package from run 34697727452, all seven entries, and the
check refuses a matrix in which they differ.

**This machine is not the machine the v0.9 rows were measured on.** The
workstation was upgraded from Ubuntu 24.04 to 26.04 on 2026-09-29: the upgrade
log reads `Upgrading from noble to resolute`. Workstation evidence from before
that date is 24.04 evidence, and evidence from after it is 26.04 evidence. The
matrix states the host of every row.

**Built on 26.04, run on 24.04.** Every binary's newest glibc symbol is
GLIBC_2.34, and 24.04 ships glibc 2.39, so a package built here installs and
runs there. That was measured, not inferred:

- **The command line and service, at 1.0.0, in clean `ubuntu:24.04`
  containers.** I3's harness is unchanged except that its versions are now
  parameters, defaulting to the 0.8.0 pair I3 recorded. All seven of its
  scenarios pass: fresh install, operation with no toolchain, upgrade from
  1.0.0~rc1, downgrade refused and then taken, remove and reinstall, profile
  discovery, and source-layout migration. So do J9's two, below.
  `I3-RESULTS.jsonl` holds what each one observed.
- **The C library and header, at 1.0.0.** J6's six examples were compiled with
  gcc 13.3 against the extracted `notrios-c-abi-1.0.0-linux-amd64.tar.gz` and
  run in a clean `ubuntu:24.04` (glibc 2.39), all passing. The log is
  `ABI-EXAMPLES-ubuntu-24.04.log`.
- **The web interface, in Google Chrome 156.0.8078.47.** H15's control crawl
  ran against the 1.0.0 service through Playwright 1.61.1 and drove all 89
  controls it inventories. `GUI_CONTROLS-chrome-156.json` is that run's
  inventory. H15's own record is left as it was: the crawl rewrites it, and
  the only differences were the Help notebook gaining one link from current
  documentation and an ordering shuffle.

**Rows not claimed:**
- **WebKit 26.5.** It launches here again since the host libraries were
  installed on 2026-10-21, but the crawl drives Chromium only.
- **The desktop shell.** It is compiled and packaged, and nothing has opened its
  window.
- **The command line on 26.04.1.** It is built and tested there from source, and
  the package has not been installed there.
- **arm64**, never run.
- **Windows and macOS**, absent from release claims.

**A finding for J49, recorded rather than fixed (J8 rule).** `release.yml`
runs on `ubuntu-latest`. Run 34697727452 got the `ubuntu-24.04` image, but the
label moves when GitHub moves it. Because `dpkg-shlibdeps` computes `Depends`
from the build system, a release built after the label moves would target a
row nobody ran. This is J49-D.

## J9-C: the instructions a stranger needs, executed

**Verifying a download.** The page gives four checks: checksums, signatures
against the key from the repository, the timestamp, and the attestation.
`run_verification.py` reads the six blocks out of the page and runs them as
published against the rehearsal set from run 34697727452, changing only the
file name. Each block runs in a GnuPG home of its own, so no keyring of the
owner's is read or written. Then each check is made to fail once. All 11
results are as the page says (`VERIFY.json`):

| check | result |
|---|---|
| `sha256sum -c SHA256SUMS` | six lines OK |
| fetch the key from `main`, show its fingerprint | primary `1234 C691 … 695E` |
| both signatures | two good signatures, by subkey `348B 6B87 … EA2C` |
| `openssl ts -verify` | `Verification: OK` |
| the attested time | `Sep 12 13:58:13 2026 GMT` |
| `gh attestation verify … --signer-workflow …/release.yml`, logged in | exit 0 |
| the same, logged out | stops at `gh auth login`, exit 4 |
| one byte added to the package | checksum FAILED, BAD signature, attestation refused |
| one byte added to the signature | `message imprint mismatch`, `Verification: FAILED` |

**Running the blocks found a fault in one of them.** The timestamp block ended
with `openssl ts -reply … | grep 'Time stamp'`. That line succeeds whatever the
verification said, so the block exited 0 after printing `Verification: FAILED`.
A reader would see the failure. A script would not. The time display is now a
block of its own, and the runner stops at the first failing line, so a block
cannot hide an earlier failure behind its last line's exit status.

`--signer-workflow` was added and measured. It narrows the claim from "built in
this repository" to "built by `release.yml`", and naming `ci.yml` instead is
refused.

**Two findings, for J49 (owner decision, 2026-10-21):**
- **The attestation needs a GitHub login unless the reader has the bundle.**
  `gh attestation verify --bundle` verifies logged out. It still refuses an
  altered byte and the wrong repository. The release set carries no bundle.
- **The C ABI tarball is attested and not signed**, and it is outside the
  signed `SHA256SUMS`.

The page says both, as they are today. J49 changes them and re-runs these
instructions.

**Upgrade, rollback and recovery, in clean containers.** J9 adds two scenarios
to I3's harness. The older version in both is the workflow-built 0.8.0, the
last package anyone could have installed:

- **08, the upgrade 0.8.0 → 1.0.0.**
  - The note written by 0.8.0 survives.
  - The upgraded library, now schema 28, takes a new note.
  - `doctor` passes.
- **09, the rollback 1.0.0 → 0.8.0.**
  - 0.8.0 refuses the 1.0 library, saying `database schema is newer than this
    build supports: the database records schema version 28 and this build
    supports 27.`, and exits 1.
  - The archive-v2 that 1.0 exported and verified is restored by 0.8.0 with
    `restore archive-v2 --intent adopt` into a new library, and the note comes
    back.
  - The 1.0 library that was set aside is still there.

**The backup the page recommended was not a backup.** It said `export
archive`, which writes v1, and the CLI reference says v1 "is not a lossless
backup — use `export archive-v2` for that". J7's disaster-recovery drill used
v2, and so does J7-A's cross-version proof that 0.8.0 restores a 1.0 archive.
The page now gives v2 for backing up, verifying, restoring and rolling back.
It quotes J7-C's numbers for the 382,206-note library: export in 55 minutes,
restore in 28, 324 MiB of memory at most, and contents and search counts
matching the original.

**What other items handed J9:**
- the 0.7.0 caution (J7-A);
- the rule that every replica upgrades, since pre-1.0 and 1.0 do not sync by any
  path (J7-B).

The page says both. Each block it publishes is registered in
`docs/docaudit/registry.json` with where it was run.

## J9-D: no sentence the milestone made untrue

`internal/docrules/release_claims_test.go` lists by phrase the nine sentences
that J1, J2 and J3 made false. It refuses any of them in `docs/` or the README,
with whitespace folded so a rewrapped paragraph still matches. Each phrase also
carries the sentence it was found in, and a second test checks the phrase still
matches it, so a pattern that has drifted cannot pass for ever. Against the
documents as they stood before J9, the gate reports 10 violations. Now it
reports none.

**The phrases, and what removed each:**
- installation page:
  - "Nothing is signed yet" and "no key has been created" (J2), replaced by the
    four checks;
  - "There is no packaged way to delete your data" and "the package does not
    ship it" (J3), replaced by `notriosctl purge`;
  - "There is no published download yet", "has not been released anywhere",
    "not published anywhere … not a supported release" and "There are no
    official prebuilt binaries" (J1), replaced by the release set;
- README: "There are no official prebuilt binaries" (J1), and "deleting those
  is `make purge` and nothing else" (J3).

**The Reports section** is J40-F's, and its own tests
(`internal/docfeatures/reports_test.go`) find it complete: every surface whose
name says report is listed, and everything listed exists.

**The page describes the 1.0 release before it exists.** It reaches readers
only with the release: the site deploys from `main`, and the package's
built-in help is built from `main` at J10. The gap between J10-A's merge and
J10-B's publication is recorded in J10.

## Archive

`notrios-v1.0-j9-978c67c.zip` was built from commit `978c67c`, with the ledger
not yet flipped: 26,783,850 bytes, 3,039 entries, accepted by
`check_release_zip.py`. No usage override was needed. The seven-day bucket had
78% remaining and the five-hour bucket 72%.

The archive also carries J50-A, vitest 4.1.12, and two J9 fixes that came after
J9's record commit `7dbede2`. **Three builds stopped first, each refused by a
gate:**
- **From `7dbede2`:** `npm audit` in `web/`, on two `@vitest/mocker` advisories
  published after the record was written. That is J50, and J50-A (`456a6fb`)
  cleared it.
- **The same commit also failed validation**, which was not seen until J50's
  plan commit ran it. Once `VERIFY.json` was tracked, the site-address check
  read gh's recorded 404 URL on `api.github.com/repos/renesugar/notrios/…` as a
  project-path base. `0ed193d` exempts the GitHub API's repository URLs, as J9
  had exempted github.com and raw.githubusercontent.com, with a test on both
  sides.
- **From `456a6fb`:** the site check in the packager, `broken link
  installation.html -> ../PACKAGING.html`. The installation page linked a
  repository file the site does not publish, and `validate-scaffold.sh` does
  not build the site, so only the packager could see it. `978c67c` links the
  file on GitHub instead.
