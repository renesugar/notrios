#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
keep=${SQLITE_PROBE_KEEP:-0}
tmp=$(mktemp -d "${TMPDIR:-/tmp}/notrios-h0-sqlite.XXXXXX")
cleanup() { [ "$keep" = 1 ] || rm -rf "$tmp"; }
trap cleanup EXIT INT TERM
archive=${SQLITE_PROBE_ARCHIVE:-${SQLLITE_PROBE_ARCHIVE:-$tmp/sqlite-amalgamation-3530400.zip}}
if [ ! -f "$archive" ]; then
  curl --fail --location --output "$archive" https://www.sqlite.org/2026/sqlite-amalgamation-3530400.zip
fi
expected=628a44cfe82c66aed1ccbbe85a562d2e33ebe64b3288981ed76285612227934e
if command -v sha3sum >/dev/null 2>&1; then actual=$(sha3sum -a 256 "$archive" | awk '{print $1}'); elif command -v openssl >/dev/null 2>&1; then actual=$(openssl dgst -sha3-256 "$archive" | awk '{print $NF}'); else echo 'sha3sum or openssl is required' >&2; exit 1; fi
[ "$actual" = "$expected" ] || { echo "official archive SHA3 mismatch" >&2; exit 1; }
unzip -q "$archive" -d "$tmp/amalgamation"
src=$(find "$tmp/amalgamation" -name sqlite3.c -print -quit)
inc=$(dirname "$src")
flags='-DSQLITE_ENABLE_FTS5 -DSQLITE_ENABLE_JSON1 -DSQLITE_THREADSAFE=1 -DSQLITE_DQS=0 -DSQLITE_OMIT_LOAD_EXTENSION -DSQLITE_SECURE_DELETE -DSQLITE_USE_URI'
build_start=$(date +%s%N)
cc ${CFLAGS:-} $flags -O2 -I"$inc" "$root/workload.c" "$src" -lpthread -ldl -lm -o "$tmp/c-workload"
build_end=$(date +%s%N); c_build_ms=$(( (build_end-build_start)/1000000 )); c_size=$(wc -c <"$tmp/c-workload")
cc -O2 "$root/owner_lock.c" -o "$tmp/owner-lock"
("$tmp/owner-lock" "$tmp/owner.lock" >"$tmp/owner-1.out") & owner=$!
sleep 0.2
if "$tmp/owner-lock" "$tmp/owner.lock" >"$tmp/owner-2.out" 2>/dev/null; then echo "owner-lock negative test failed" >&2; kill "$owner" 2>/dev/null || true; exit 1; fi
wait "$owner"
c_start=$(date +%s%N); /usr/bin/time -f '%M' -o "$tmp/c-rss" "$tmp/c-workload" run "$tmp/roundtrip.db" | tee "$tmp/c-result.json"; c_end=$(date +%s%N); c_workload_ms=$(( (c_end-c_start)/1000000 )); c_rss=$(cat "$tmp/c-rss")
mkdir -p "$tmp/gomod"
cp "$root/workload.go" "$root/go.mod" "$tmp/gomod/"
if [ -f "$root/go.sum" ]; then cp "$root/go.sum" "$tmp/gomod/"; fi
m_build_start=$(date +%s%N)
(cd "$tmp/gomod" && GOPROXY=${SQLITE_PROBE_GOPROXY:-https://proxy.golang.org} GOCACHE="$tmp/gocache" go build -buildvcs=false -tags modernc -o "$tmp/modernc-workload" .)
m_build_end=$(date +%s%N); m_build_ms=$(( (m_build_end-m_build_start)/1000000 ))
modernc_size=$(wc -c <"$tmp/modernc-workload"); m_start=$(date +%s%N); /usr/bin/time -f '%M' -o "$tmp/m-rss" "$tmp/modernc-workload" run "$tmp/modernc.db" | tee "$tmp/modernc-result.json"; m_end=$(date +%s%N); m_workload_ms=$(( (m_end-m_start)/1000000 )); m_rss=$(cat "$tmp/m-rss")
/usr/bin/time -f '%M' -o "$tmp/m-roundtrip-rss" "$tmp/modernc-workload" roundtrip "$tmp/roundtrip.db" | tee "$tmp/modernc-roundtrip-result.json"
"$tmp/c-workload" verify "$tmp/roundtrip.db" | tee "$tmp/c-reopen-result.json"
echo "sqlite archive sha3-256=$actual"
echo "modernc versions: modernc.org/sqlite v1.57.0; modernc.org/libc v1.74.4"
printf '{"archive_sha3":"%s","c":{"build_ms":%s,"workload_ms":%s,"max_rss_kb":%s,"artifact_bytes":%s},"modernc":{"build_ms":%s,"workload_ms":%s,"max_rss_kb":%s,"artifact_bytes":%s},"roundtrip":true}\n' "$actual" "$c_build_ms" "$c_workload_ms" "$c_rss" "$c_size" "$m_build_ms" "$m_workload_ms" "$m_rss" "$modernc_size" | tee "$tmp/result.json"
echo "evidence directory: $tmp"
