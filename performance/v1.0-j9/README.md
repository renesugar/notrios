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
`Depends` is `libc6 (>= 2.34)` alone. The workflow-built package carries all
three binaries, and J9-B takes the runtime rows from that package, not from this
one.
