#!/bin/sh
# Build the version-1 ABI as a shared library and run the C host acceptance
# test against it. This is the boundary check: it proves the exact exported
# symbol set, that no SQLite symbol leaks, and that a C caller using only
# notrios_abi.h can drive the library.
set -eu
cd "$(dirname "$0")"
root=$(cd ../.. && pwd)
out="${TMPDIR:-/tmp}/notrios-abi-host-$$"
trap 'rm -rf "$out"' EXIT
mkdir -p "$out"

go build -buildmode=c-shared -o "$out/libnotrios.so" "$root/cmd/notrioslib"
test -s "$out/libnotrios.h"

# The exported surface is exactly the frozen twelve symbols.
grep -oE 'notrios_[a-z_]+' notrios_abi.h | sort -u > "$out/expected"
nm -D --defined-only "$out/libnotrios.so" | awk '$3 ~ /^notrios_/ {print $3}' | sort > "$out/actual"
if ! cmp -s "$out/expected" "$out/actual"; then
  echo "exported symbol set differs from the frozen ABI contract:" >&2
  diff "$out/expected" "$out/actual" >&2 || true
  exit 1
fi

# No SQLite symbol may escape: a host able to resolve sqlite3_open could open
# the canonical database behind the owner's back.
if nm -D --defined-only "$out/libnotrios.so" | grep -q 'sqlite3_'; then
  echo "the shared library exports SQLite symbols" >&2
  exit 1
fi

# No dynamic SQLite dependency either; the engine is statically linked.
if ldd "$out/libnotrios.so" | grep -qi sqlite; then
  echo "the shared library has a dynamic SQLite dependency" >&2
  exit 1
fi

# The host test lives outside the package directory on purpose: a .c file
# beside main.go is compiled into the library by cgo, and its main() would
# collide with Go's at link time, breaking an ordinary `go build ./...`.
cc -std=c11 -Wall -Wextra -Werror -I. -I"$out" hosttest/host_test.c \
  -L"$out" -lnotrios -Wl,-rpath,"$out" -pthread -o "$out/host_test"
"$out/host_test" "$out/library.db"
echo "notrioslib: 12 exact symbols, no SQLite export, no dynamic SQLite, C host PASS"
