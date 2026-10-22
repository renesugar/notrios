#!/usr/bin/env bash
# Build the versioned C ABI artifact: the library, the header, and enough
# around them that somebody who has never seen this repository can use it
# (v1.0 J6).
#
#   bash scripts/build_abi_artifact.sh [output-directory]
#
# The default output directory is dist/abi. The artifact is one tarball,
# notrios-c-abi-<version>-<os>-<arch>.tar.gz, holding:
#
#   lib/libnotrios.so.1   the shared library, with its ABI major in the soname
#   lib/libnotrios.so     the development link a `-lnotrios` resolves
#   include/notrios_abi.h the header, the only interface a host needs
#   LICENSE               the licence the library ships under
#   README.md             what it promises, what it does not, and where it ran
#   SHA256SUMS            so a downloader can check the files without this repo
#
# Two versions, both stated. The tarball carries the product version; the
# library carries ABI major 1 in its soname, which is what a linker records and
# what an incompatible ABI would change.
#
# It refuses to produce an artifact that fails the boundary checks: the exported
# set must be exactly the frozen symbols, no SQLite symbol may escape, and there
# must be no dynamic SQLite dependency.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT=${1:-$ROOT/dist/abi}
HEADER=$ROOT/cmd/notrioslib/notrios_abi.h

version=$(python3 - "$ROOT/internal/version/version.go" <<'PY'
import re
import sys
source = open(sys.argv[1], encoding="utf-8").read()
match = re.search(r'^\s*(?:const\s+)?Version\s*=\s*"([^"]+)"', source, re.MULTILINE)
if not match:
    raise SystemExit("internal/version/version.go states no Version")
print(match.group(1))
PY
)
abi_major=$(grep -oE '#define[[:space:]]+NOTRIOS_ABI_VERSION[[:space:]]+[0-9]+' "$HEADER" | grep -oE '[0-9]+$')
[[ -n "$abi_major" ]] || { echo "the header states no NOTRIOS_ABI_VERSION" >&2; exit 1; }
goos=$(go env GOOS)
goarch=$(go env GOARCH)
name=notrios-c-abi-$version-$goos-$goarch
soname=libnotrios.so.$abi_major

work=$(mktemp -d "${TMPDIR:-/tmp}/notrios-abi-XXXXXX")
trap 'rm -rf "$work"' EXIT
stage=$work/$name
mkdir -p "$stage/lib" "$stage/include" "$OUT"

echo "building $soname for $goos/$goarch"
( cd "$ROOT" && go build -buildmode=c-shared \
    -ldflags "-extldflags '-Wl,-soname,$soname'" \
    -o "$stage/lib/$soname" ./cmd/notrioslib )
# cgo writes its own generated header beside the library. It is not the
# interface -- notrios_abi.h is, curated and frozen -- and shipping both would
# invite a host to include the wrong one.
rm -f "$stage/lib/libnotrios.so.h"
ln -s "$soname" "$stage/lib/libnotrios.so"
cp "$HEADER" "$stage/include/notrios_abi.h"
cp "$ROOT/LICENSE" "$stage/LICENSE"

# The boundary checks, run against the artifact's own library rather than a
# separate build, because what ships is what must hold.
grep -oE 'notrios_[a-z_]+' "$HEADER" | sort -u > "$work/expected"
nm -D --defined-only "$stage/lib/$soname" | awk '$3 ~ /^notrios_/ {print $3}' | sort > "$work/actual"
if ! cmp -s "$work/expected" "$work/actual"; then
  echo "the artifact's exported symbols differ from the header:" >&2
  diff "$work/expected" "$work/actual" >&2 || true
  exit 1
fi
if nm -D --defined-only "$stage/lib/$soname" | grep -q 'sqlite3_'; then
  echo "the artifact exports SQLite symbols" >&2
  exit 1
fi
if ldd "$stage/lib/$soname" | grep -qi sqlite; then
  echo "the artifact has a dynamic SQLite dependency" >&2
  exit 1
fi
if ! objdump -p "$stage/lib/$soname" | grep -q "SONAME *$soname"; then
  echo "the artifact does not carry the soname $soname" >&2
  exit 1
fi
symbols=$(wc -l < "$work/expected")
needed=$(objdump -p "$stage/lib/$soname" | awk '$1 == "NEEDED" {print $2}' | sort | tr '\n' ' ')

python3 - "$stage/README.md" "$version" "$abi_major" "$goos" "$goarch" "$symbols" "$soname" \
    "$needed" "$work/expected" <<'PY'
import pathlib
import sys

out, version, abi_major, goos, goarch, symbols, soname, needed, expected = sys.argv[1:10]
exported = pathlib.Path(expected).read_text(encoding="utf-8").split()
pathlib.Path(out).write_text(f"""# Notrios C ABI {version} (ABI major {abi_major})

A shared library exposing Notrios without a GUI, a Go toolchain, or this
repository. `include/notrios_abi.h` is the whole interface: {symbols} functions.

## Using it

```sh
cc -std=c11 -Iinclude your_host.c -Llib -lnotrios -Wl,-rpath,'$ORIGIN/lib' -o your_host
```

`lib/{soname}` carries the ABI major in its soname, which is what your linker
records. `lib/libnotrios.so` is the development link `-lnotrios` resolves.

Check the first two calls before anything else:

```c
if (notrios_abi_version() != NOTRIOS_ABI_VERSION) {{ /* refuse: header and library disagree */ }}
uint64_t capabilities = notrios_capabilities();      /* ignore bits you do not know */
```

## What it promises

- **Opaque handles.** No Go pointer crosses the boundary. A closed handle is
  detectably stale rather than merely invalid.
- **One owner per library.** A second open of the same path is refused while
  the first is open.
- **Library-owned buffers.** Every buffer the library returns is released with
  `notrios_buffer_release`. Releasing twice, or releasing a pointer the library
  did not produce, is refused rather than acted on.
- **No callbacks into your threads.** Calls are started and polled; an empty
  poll reports would-block rather than blocking.
- **Bounded requests and reads**, so a malformed size cannot allocate freely.

## What it does not promise

- **No stable ABI across a major.** A different soname means a different ABI.
- **No SQLite.** The engine is linked in statically and exports nothing; a host
  cannot reach the database except through these functions.
- **One platform per artifact.** This one was built and tested on
  {goos}/{goarch}. Nothing here claims any other platform: see the supported
  matrix in the release documentation.

## What is in this artifact

- `lib/{soname}` and `lib/libnotrios.so`
- `include/notrios_abi.h`
- `LICENSE`, this file, and `SHA256SUMS`

Exported symbols: {' '.join(exported)}.

Dynamic dependencies: {needed.strip() or 'none'}.

Verify the files before using them:

```sh
sha256sum -c SHA256SUMS
```
""", encoding="utf-8")
PY

( cd "$stage" && find . -type f ! -name SHA256SUMS -printf '%P\n' | sort | xargs sha256sum > SHA256SUMS )
( cd "$work" && tar --sort=name --owner=0 --group=0 --numeric-owner -czf "$OUT/$name.tar.gz" "$name" )
( cd "$OUT" && sha256sum "$name.tar.gz" > "$name.tar.gz.sha256" )

echo "wrote $OUT/$name.tar.gz"
echo "  $symbols exported symbols, soname $soname, needed: ${needed:-none}"
cat "$OUT/$name.tar.gz.sha256"
