# v0.8 H6a — desktop installer and GitHub-native build investigation

Investigation only. No packaging dependency was added to the build, no workflow
was pushed, no artifact was uploaded, and nothing here is a support claim.

## The claim ladder

Everything below is placed on it explicitly, because the distance between
"an installer was produced" and "this platform is supported" is where installer
projects usually go wrong.

| Level | Name | What it means |
|---|---|---|
| 1 | generated | a tool produced an artifact |
| 2 | structurally inspected | its contents, metadata and dependencies were read and checked |
| 3 | natively installed and executed | installed and run **on the target platform** |
| 4 | supported | level 3, plus someone committing to keep it working |

**Nothing in this investigation reaches level 3 except `linux/amd64`.** That is
the finding, not a shortfall in the work: cross-compiling and reading an archive
cannot tell you whether software runs.

## The constraint everything else follows from

`CGO_ENABLED=0` does not build. Not "builds without SQLite" — `internal/store`
declares its exported types inside the cgo file, so the package fails to
typecheck. Any packaging option therefore needs a C toolchain **for every target
it claims**, and no amount of Go tooling removes that.

Measured consequences:

- With only the host compiler, GoReleaser built 1 of 6 targets. The other five
  died in `runtime/cgo`: `gcc_arm64.S:30: Error: no such instruction`.
- With `gcc-aarch64-linux-gnu` installed and `CC` set, `linux/arm64` built a
  genuine aarch64 ELF and an `arm64` `.deb`. The vendored SQLite amalgamation
  needed no extra flags — a better outcome than expected.
- That arm64 binary **cannot be executed here**: `Exec format error`. It is
  level 2 and stays there until an arm64 machine runs it.

So one Ubuntu runner can *produce* both architectures for two extra apt packages
and one environment line. Only an arm64 machine can say whether the result
works. Those are different claims and the ladder keeps them apart.

## What the prototype `.deb` is, and is not

It builds, it is 13.4 MB, and it installs `/usr/bin/{notriosd,notriosctl}` plus
`/usr/share/notrios/{web,docs}`. That layout is exactly the relationship H5's
program-assets resolution derives, so a distro package finds its interface and
help with no extra configuration — the one thing that worked for free.

It is not a policy-compliant Debian package. `lintian` reports **3 errors and
297 warnings** (`LINTIAN.txt`):

- `no-copyright-file` — for an Apache-2.0 project, this is the one to fix first;
- `no-changelog`, `extended-description-is-empty`, `unknown-section`;
- `undeclared-elf-prerequisites` — the package declares **no `Depends` at all**
  while the binary links `libc` and `libm`. nFPM does no shared-library
  dependency resolution; `dh_shlibdeps` does. An underdeclared package installs
  happily and fails on a machine missing something it never mentioned.
- `hardening-no-pie`, `no-manual-page`, and `non-standard-file-perm` on every
  documentation file, because nFPM copied the checkout's `0664` modes.

The control archive holds only `control` and `md5sums`: **no maintainer
scripts**. Nothing registers the `notrios://` scheme handler or updates the
desktop database, so the package installs a desktop entry that does not take
effect. H5's `make install` has the same gap; the difference is that a package
is expected to close it.

## Two operational findings

**Builds are not reproducible by default.** Two runs produced different `.deb`s
*and* different binaries. The cause is GoReleaser's default ldflags injecting
`-X main.date=<timestamp>`, which is recorded in Go build info even though
`main.date` does not exist in these binaries. Overriding ldflags fixes it; the
point is that it is not free.

**`goreleaser --clean` empties `dist/`,** which this repository already uses for
source release ZIPs — it removed them during this investigation. Nothing was
lost, because each ZIP had already been copied to the evidence directory, which
is exactly why that practice exists. One of the two uses of `dist/` has to move.

## Windows and macOS

`makensis` 3.09 and the Wails v2.13.0 CLI are installed, so a Windows installer
could be *generated* here. It would sit at level 1: a cross-built Windows
artifact is not native evidence and nothing here can execute it.

macOS cannot be assessed on this machine at all, and emulating Apple hardware
locally is outside this item's boundaries. It needs a native runner or it needs
postponing — those are the only two honest options.

## Recommendation

**Ubuntu: GoReleaser plus nFPM is a reasonable choice, but the `.deb` it emits
today is a starting point rather than a package.** The gaps above are all
fixable in configuration; none of them are discovered by looking at the tool's
documentation, which is why they are written down here with a `lintian`
transcript beside them.

Worth considering for H6: the package could be generated from the same layout
`scripts/lifecycle.py` already installs and records in its manifest, so the
install contract has one definition rather than two that drift.

**Windows and macOS: no toolchain can be selected from this machine.** The
decision is between provisioning native runners and postponing both platforms,
and this investigation deliberately does not pretend to settle it.

## What was not verified

Recorded so the gaps are visible rather than assumed:

- GitHub-hosted runner availability, per-minute cost and artifact retention were
  not checked against upstream documentation in this pass, so no projection is
  offered.
- Whether the arm64 `.deb` installs or runs on an arm64 machine.
- Wails v2 native packaging output on Windows or macOS.
