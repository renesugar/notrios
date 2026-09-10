#!/usr/bin/env bash
# I8 — the C ABI at its edges, under instrumentation that understands Go.
#
#   bash performance/v0.9-i8/run_edges.sh          # race + asan
#   I8_VALGRIND=1 bash performance/v0.9-i8/run_edges.sh   # and the leak pass
#
# The first version of this used valgrind, and valgrind is the wrong instrument
# for a Go c-shared library. Go grows a goroutine stack by allocating a larger
# one and copying the frames into it, rewriting pointers as it goes; to memcheck
# every one of those writes lands outside a known block. The run produced ten
# million invalid-access reports, every frame in runtime.* -- newstack,
# copystack, adjustframe -- and a host that only calls notrios_abi_version()
# produces them too. Targeted suppressions removed 9.5 million and it still hit
# memcheck's cap.
#
# Go ships instruments that do understand its runtime, so those are what run
# here: -race for the concurrency and shutdown edges, -asan for memory errors.
# valgrind is kept behind a flag for leak accounting only, which is the one
# number it still reports usefully.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/performance/v0.9-i8
OUT=$(mktemp -d "${TMPDIR:-/tmp}/notrios-i8-XXXXXX")
trap 'rm -rf "$OUT"' EXIT

echo "== the exported surface is still exactly the frozen twelve"
go build -buildmode=c-shared -o "$OUT/libnotrios.so" "$ROOT/cmd/notrioslib"
grep -oE 'notrios_[a-z_]+' "$ROOT/cmd/notrioslib/notrios_abi.h" | sort -u > "$OUT/expected"
nm -D --defined-only "$OUT/libnotrios.so" | awk '$3 ~ /^notrios_/ {print $3}' | sort > "$OUT/actual"
cmp -s "$OUT/expected" "$OUT/actual" || { echo "exported symbols drifted:" >&2
  diff "$OUT/expected" "$OUT/actual" >&2 || true; exit 1; }
echo "   $(wc -l < "$OUT/actual") symbols"
if nm -D --defined-only "$OUT/libnotrios.so" | grep -q 'sqlite3_'; then
  echo "the shared library exports SQLite symbols" >&2; exit 1
fi

echo "== the typed surface still matches the recorded ABI"
# nm proves the twelve names are there. abidiff proves their *signatures* are:
# a parameter that changes from size_t to int keeps every name and breaks every
# caller. The baseline is regenerated with abidw and compared, not read.
abidw --no-show-locs --drop-undefined-syms "$OUT/libnotrios.so" \
  | sed "s|path='[^']*'|path='libnotrios.so'|" > "$OUT/abi.xml"
if ! abidiff "$HERE/ABI_BASELINE.xml" "$OUT/abi.xml" > "$OUT/abidiff.txt" 2>&1; then
  echo "the typed ABI differs from the recorded baseline:" >&2
  head -20 "$OUT/abidiff.txt" >&2
  echo "if the change is intended, regenerate ABI_BASELINE.xml and say why in the commit" >&2
  exit 1
fi
echo "   typed ABI unchanged"

go build -o "$OUT/notriosctl" "$ROOT/cmd/notriosctl"
seed() { mkdir -p "$2/assets"
  "$OUT/notriosctl" notes create --db "$2/library.db" --asset-store "$2/assets" \
    --title "seeded for the edges host" --body body >/dev/null; }

# run <name> <extra-go-flags> <extra-cc-flags> <env-prefix...>
run_variant() {
  local name=$1 goflags=$2 ccflags=$3; shift 3
  echo "== $name"
  local dir=$OUT/$name; mkdir -p "$dir"
  # shellcheck disable=SC2086
  go build $goflags -buildmode=c-shared -o "$dir/libnotrios.so" "$ROOT/cmd/notrioslib"
  # shellcheck disable=SC2086
  cc -std=c11 -D_POSIX_C_SOURCE=200809L -Wall -Wextra -O1 -g $ccflags \
     -I"$ROOT/cmd/notrioslib" \
     "$HERE/edges_host.c" -o "$dir/edges" -L"$dir" -lnotrios -lpthread
  seed "$name" "$dir"
  local status=0
  ( cd "$dir" && LD_LIBRARY_PATH="$dir" "$@" ./edges "$dir/library.db" ) \
    > "$OUT/$name.log" 2>&1 || status=$?
  echo "   exit $status"
  grep -E "^(  ok|  FAIL)" "$OUT/$name.log" | tail -3 || true
  printf '   instrumentation reports: %s\n' "$(grep -ciE 'DATA RACE|ERROR: AddressSanitizer' "$OUT/$name.log" || true)"
  if [[ $status -ne 0 ]]; then echo "   --- output ---"; tail -25 "$OUT/$name.log"; fi
  echo "$status" > "$OUT/$name.status"
}

run_variant asan "-asan" "-fsanitize=address" env ASAN_OPTIONS=detect_leaks=0

# The race detector runs on the Go side, not through the C boundary.
#
# Two attempts to put it there failed: TSan maps a large shadow region and must
# do it as the process starts, so loaded via a c-shared library it dies with
# "failed to allocate ... bytes at address"; instrumenting the host with
# -fsanitize=thread as well changes the message to "unexpected memory mapping",
# and disabling ASLR with setarch does not help either. AddressSanitizer crosses
# that boundary happily, which is why the asan run above is the C host.
#
# internal/abi is the same dispatch, session and handle code the twelve entry
# points call straight into, and its concurrency tests are written for exactly
# these edges: many callers on one session, close while calls are in flight,
# cancellation racing completion.
echo "== race (Go side; TSan cannot be loaded through c-shared here)"
race_status=0
( cd "$ROOT" && go test -race -count=1 ./internal/abi/ ) > "$OUT/race.log" 2>&1 || race_status=$?
echo "   exit $race_status"
printf '   data races: %s\n' "$(grep -c "WARNING: DATA RACE" "$OUT/race.log" || true)"
[[ $race_status -eq 0 ]] || tail -20 "$OUT/race.log"
echo "$race_status" > "$OUT/race.status"

if [[ ${I8_VALGRIND:-0} == 1 ]]; then
  echo "== valgrind (leak accounting only)"
  dir=$OUT/plain; mkdir -p "$dir"
  cp "$OUT/libnotrios.so" "$dir/"
  cc -std=c11 -D_POSIX_C_SOURCE=200809L -O1 -g -I"$ROOT/cmd/notrioslib" \
     "$HERE/edges_host.c" -o "$dir/edges" \
     -L"$dir" -lnotrios -lpthread
  seed plain "$dir"
  ( cd "$dir" && LD_LIBRARY_PATH="$dir" valgrind --leak-check=full \
      --errors-for-leak-kinds=definite --suppressions="$HERE/go-runtime.supp" \
      --log-file="$OUT/valgrind.log" ./edges "$dir/library.db" ) > "$OUT/plain.log" 2>&1 || true
  grep -E "definitely lost|Invalid free" "$OUT/valgrind.log" | sed 's/^==[0-9]*== /   /' | head -3
fi

python3 "$HERE/record_edges.py" --out "$OUT" --symbols "$OUT/actual"
