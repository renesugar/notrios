#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
out=$(mktemp -d "${TMPDIR:-/tmp}/notrios-h0-storelink.XXXXXX")
trap 'rm -rf "$out"' EXIT INT TERM
go build -buildmode=c-shared -o "$out/libnotrios_store_probe.so" "$root"
cc -std=c11 -Wall -Wextra -Werror -I"$out" "$root/c-host/host.c" -L"$out" \
  -lnotrios_store_probe -Wl,-rpath,"$out" -o "$out/host"
"$out/host" "$out/store.db"
if ldd "$out/libnotrios_store_probe.so" | grep -q sqlite; then
  echo "storelink-probe: dynamic SQLite dependency detected" >&2
  exit 1
fi
if nm -D --defined-only "$out/libnotrios_store_probe.so" | awk '{print $3}' | grep -q '^sqlite3_'; then
  echo "storelink-probe: embedded SQLite symbols escaped visibility policy" >&2
  exit 1
fi
printf 'storelink-probe: one hidden static SQLite engine PASS (%s bytes)\n' \
  "$(wc -c <"$out/libnotrios_store_probe.so")"
