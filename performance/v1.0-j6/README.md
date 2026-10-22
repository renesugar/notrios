# v1.0 J6: the C ABI as something a third party can use

`cmd/notrioslib` already built the version-1 ABI, and `run_host_test.sh` already
proved the exported set is exactly twelve symbols, that no SQLite symbol
escapes, that nothing links SQLite dynamically, and that a C host driving only
`notrios_abi.h` works. What was missing was the artifact: to use any of it, you
had to clone this repository and run that script.

## The artifact (J6-A)

`scripts/build_abi_artifact.sh` produces one tarball per platform,
`notrios-c-abi-<version>-<os>-<arch>.tar.gz`:

| file | what it is |
|---|---|
| `lib/libnotrios.so.1` | the library, with ABI major 1 in its soname |
| `lib/libnotrios.so` | the development link `-lnotrios` resolves |
| `include/notrios_abi.h` | the whole interface: twelve functions and `NOTRIOS_ABI_VERSION` |
| `LICENSE` | the licence it ships under |
| `README.md` | what it promises, what it does not, and the platform it was built and tested on |
| `SHA256SUMS` | so a downloader can check the files without this repository |

**Two versions, both stated.** The product version names the tarball; the ABI
major is in the soname, which is what a linker records and what an incompatible
ABI would change. The header now defines `NOTRIOS_ABI_VERSION`, so a host can
refuse a library of another major instead of discovering it call by call.

**The build refuses to ship a broken artifact.** It checks the artifact's own
library, not a separate build: exported symbols exactly the header's, no SQLite
symbol exported, no dynamic SQLite dependency, and the soname present. It also
drops the header cgo generates beside the library, which is not the interface
and would invite a host to include the wrong one.

`ARTIFACT.json` records one build: 7,453,911 bytes, six files, the twelve
symbols, soname `libnotrios.so.1`, dynamic dependencies `libc.so.6` and
`libm.so.6`, and `sha256sum -c SHA256SUMS` verifying. The tarball's own hash is
deliberately absent: a Go build is not bit-reproducible here, so a number that
changed every run would be evidence of nothing.

**Built where it can be attested.** `.github/workflows/release.yml` builds the
tarball beside the `.deb`, runs the examples against it, attests it with the
same action, and uploads it as a **run artifact**. It publishes nothing:
publication is J10's, under the owner's authorization.

## The examples (J6-B)

`examples/c/` holds six programs, one per thing a caller must get right:

| example | what it shows |
|---|---|
| `01_lifecycle` | check the versions agree, open, start a call, poll to completion, release, close |
| `02_ownership` | the library's buffer released once; releasing twice refused; releasing a foreign pointer refused; a closed handle stale rather than invalid |
| `03_threading` | four threads × eight calls on one instance, each releasing what its own call produced |
| `04_errors` | unknown operation, malformed request, second owner, zero handle, call after close — each a code to act on |
| `05_streams` | a stream over a missing resource refused with no handle; an empty event queue reporting would-block; the read-and-release loop |
| `06_compatibility` | the major must match exactly; unknown capability bits are ignored, not refused, which is how an old host keeps working against a newer library |

`run_examples.sh` verifies the artifact's own `SHA256SUMS`, then compiles each
example with `-I` and `-L` pointing **only inside the extracted tarball**, and
runs each against its own library directory — one owner per library is part of
what the ABI promises, so sharing one would fail on it. Compiling against the
repository would prove the one thing a third party cannot do.

All six compiled and ran clean.

## The baseline (J6-C)

`performance/v0.9-i8` re-derives the ABI surface from the header and reports
**c_abi 12, unchanged**. A soname is a link-time record and
`NOTRIOS_ABI_VERSION` is an uppercase macro, so neither is an exported symbol
and neither moved the frozen set.

**One frozen surface did move:** `make_lifecycle` 37 → 39, for `abi-artifact`
and `abi-examples`. Additive, nothing removed or redefined, and the freeze was
rebuilt with `build_freeze.py` rather than edited.

`performance/v0.9-i6`'s derived count of digest-pinned actions moved 22 → 24
with the workflow's two new `uses:`, and is recorded with that reason.

## What is not claimed

The supported matrix is v0.9 I7's, so this artifact is built and tested for
**Ubuntu 24.04 amd64 and nothing else**. A second platform appears when a
machine runs it, not because a cross-build succeeded. No Windows, macOS, iOS or
Android artifact is implied, and an Android-emulator result would not be
physical Android support.

## Validation

`make abi` (the existing host test) passes. The whole Go suite passes through
`scripts/check_temp_leaks.sh`, which left no `notrios-*` entry, and
`validate-scaffold.sh` passes.

## The archive

Built from `7a7d688` with the usage guard on and no override:
- **Usage guard:** it checked only Claude, 59% of the five-hour window
  remaining, identified by process ancestry (J24).
- **Package:** `package_release.sh` passed, including `go test ./...`,
  `validate-scaffold.sh`, `make g18g-validate` and every evidence validator.
- **ZIP:** `notrios-v1.0-j6-7a7d688.zip`, 25,738,953 bytes, 2,712 entries.
  `check_release_zip.py` accepts it.
