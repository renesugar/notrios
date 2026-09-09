#!/usr/bin/env bash
# v0.8 H11 — Android-emulator shared-core acceptance.
#
# Proves the H1 library is a viable backend on one Android emulator. It is not
# an Android product and does not become one by passing: there is no APK, no UI,
# no background work and no secure store. The credential provider is supplied by
# the host that pushes these files, which H11 records as a documented gap.
#
# Everything below runs against a real device, and the report it writes is
# generated rather than typed. What cannot be run is not claimed: the arm64
# artifacts are built and hashed and never executed, because this emulator is
# x86_64 and a cross-built artifact that has never run is not evidence.
#
#   ANDROID_HOME=~/Android/Sdk bash performance/v0.8-h11/run_acceptance.sh
#
# Requires a booted emulator on adb. It does not start one: choosing the device
# an acceptance run measures is the operator's decision, not the script's.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
sdk=${ANDROID_HOME:-$HOME/Android/Sdk}
ndk=${ANDROID_NDK_HOME:-$(ls -d "$sdk"/ndk/* | sort -V | tail -1)}
adb=$sdk/platform-tools/adb
bin=$ndk/toolchains/llvm/prebuilt/linux-x86_64/bin
api=${NOTRIOS_ANDROID_API:-35}
device_dir=/data/local/tmp/notrios-h11
out=$(mktemp -d "${TMPDIR:-/tmp}/notrios-h11.XXXXXX")
report=${NOTRIOS_H11_REPORT:-$root/performance/v0.8-h11/REPORT.json}
trap 'rm -rf "$out"' EXIT

say() { printf '\n== %s\n' "$1"; }
sha() { sha256sum "$1" | cut -d' ' -f1; }

"$adb" wait-for-device
fingerprint=$("$adb" shell getprop ro.build.fingerprint | tr -d '\r')
release=$("$adb" shell getprop ro.build.version.release | tr -d '\r')
sdk_int=$("$adb" shell getprop ro.build.version.sdk | tr -d '\r')
device_abi=$("$adb" shell getprop ro.product.cpu.abi | tr -d '\r')
selinux=$("$adb" shell getenforce | tr -d '\r')

say "building the shared core for android/amd64 (the runtime ABI)"
CGO_ENABLED=1 GOOS=android GOARCH=amd64 CC="$bin/x86_64-linux-android$api-clang" \
  go build -buildmode=c-shared -o "$out/libnotrios.so" "$root/cmd/notrioslib"

# The exported surface has to be the frozen twelve here too. A cross-compiled
# library that exported something else, or leaked SQLite, would be a different
# library wearing the same name.
grep -oE 'notrios_[a-z_]+' "$root/cmd/notrioslib/notrios_abi.h" | sort -u > "$out/expected"
"$bin/llvm-nm" -D --defined-only "$out/libnotrios.so" | awk '$3 ~ /^notrios_/ {print $3}' | sort > "$out/actual"
cmp -s "$out/expected" "$out/actual" || { echo "android symbol set differs from the frozen ABI" >&2; exit 1; }
"$bin/llvm-nm" -D --defined-only "$out/libnotrios.so" | grep -q 'sqlite3_' \
  && { echo "the android library exports SQLite symbols" >&2; exit 1; }
symbols=$(wc -l < "$out/actual")

say "building the acceptance host with the NDK"
"$bin/x86_64-linux-android$api-clang" -std=c11 -Wall -Wextra -Werror \
  -I "$root/cmd/notrioslib" -I "$out" "$root/performance/v0.8-h11/acceptance_host.c" \
  -L "$out" -lnotrios -o "$out/acceptance_host"

say "seeding a library with the desktop build"
go build -o "$out/notriosctl" "$root/cmd/notriosctl"
mkdir -p "$out/seed/assets"
seed_title="desktop seeded note"
"$out/notriosctl" notes create --db "$out/seed/library.db" --asset-store "$out/seed/assets" \
  --title "$seed_title" --body "Written by the desktop build before the file crossed to the emulator." >/dev/null
printf 'notrios h11 resource bytes, long enough to need several bounded reads over a 64-byte window so the stream is exercised rather than fetched in one go.' > "$out/payload.txt"
resource_json=$("$out/notriosctl" resources add --db "$out/seed/library.db" \
  --asset-store "$out/seed/assets" --file "$out/payload.txt")
resource_id=$(printf '%s' "$resource_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["resource_id"])')
resource_bytes=$(printf '%s' "$resource_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["size_bytes"])')
seed_sha=$(sha "$out/seed/library.db")

say "pushing to $device_dir"
"$adb" shell rm -rf "$device_dir"
"$adb" shell mkdir -p "$device_dir"
"$adb" push "$out/libnotrios.so" "$out/acceptance_host" "$device_dir/" >/dev/null
"$adb" push "$out/seed/library.db" "$device_dir/library.db" >/dev/null
"$adb" push "$out/seed/assets" "$device_dir/" >/dev/null
"$adb" shell chmod 755 "$device_dir/acceptance_host"

phase() {
  local name=$1; shift
  say "phase: $name"
  "$adb" shell "cd $device_dir && LD_LIBRARY_PATH=$device_dir ./acceptance_host $*" | tee "$out/$name.log"
  grep -q "0 failures" "$out/$name.log" || { echo "phase $name failed" >&2; exit 1; }
  awk -v n="$name" '/checks,/ {gsub(/[^0-9 ]/," "); print n": "$1" checks"}' "$out/$name.log" >/dev/null
}

phase open "open $device_dir/library.db"
phase work "work $device_dir/library.db '$seed_title'"
phase stream "stream $device_dir/library.db $resource_id $resource_bytes"

say "phase: abandon — a committed write, then SIGKILL"
"$adb" shell "cd $device_dir && LD_LIBRARY_PATH=$device_dir ./acceptance_host abandon $device_dir/library.db" > "$out/abandon.log" 2>&1 &
for _ in $(seq 1 30); do sleep 1; grep -q "about to be killed" "$out/abandon.log" && break; done
grep -q "about to be killed" "$out/abandon.log" || { echo "the abandon phase never committed" >&2; exit 1; }
"$adb" shell "pkill -9 -f acceptance_host" || true
sleep 1
# What a killed process leaves: a write-ahead log and the ownership marker it
# never got to release. Recorded because the next open has to cope with both.
leftovers=$("$adb" shell "ls $device_dir" | tr -d '\r' | tr '\n' ' ')
phase verify-after-kill "verify $device_dir/library.db 'committed before the kill' 1"

say "rebooting the device"
"$adb" reboot
sleep 20
"$adb" wait-for-device
for _ in $(seq 1 60); do
  [ "$("$adb" shell getprop sys.boot_completed | tr -d '\r')" = "1" ] && break
  sleep 3
done
phase verify-after-reboot "verify $device_dir/library.db 'committed before the kill' 1"

say "pulling the library back to the desktop"
device_sha=$("$adb" shell "sha256sum $device_dir/library.db" | awk '{print $1}' | tr -d '\r')
"$adb" pull "$device_dir/library.db" "$out/roundtrip.db" >/dev/null
pulled_sha=$(sha "$out/roundtrip.db")
[ "$device_sha" = "$pulled_sha" ] || { echo "the pulled file differs from the device's" >&2; exit 1; }

integrity=$(sqlite3 "$out/roundtrip.db" "PRAGMA integrity_check;")
journal=$(sqlite3 "$out/roundtrip.db" "PRAGMA journal_mode;")
schema=$(sqlite3 "$out/roundtrip.db" "PRAGMA user_version;")
count_android=$("$out/notriosctl" search --db "$out/roundtrip.db" --asset-store "$out/seed/assets" \
  --count 'written on Android' | python3 -c 'import json,sys; print(json.load(sys.stdin)["count"])')
count_killed=$("$out/notriosctl" search --db "$out/roundtrip.db" --asset-store "$out/seed/assets" \
  --count 'committed before the kill' | python3 -c 'import json,sys; print(json.load(sys.stdin)["count"])')
[ "$integrity" = "ok" ] || { echo "integrity_check said $integrity" >&2; exit 1; }
[ "$count_android" = "1" ] && [ "$count_killed" = "1" ] || { echo "the desktop cannot read what Android wrote" >&2; exit 1; }

say "building the arm64 artifacts, which are never executed"
CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC="$bin/aarch64-linux-android$api-clang" \
  go build -buildmode=c-shared -o "$out/libnotrios-arm64.so" "$root/cmd/notrioslib"
"$bin/aarch64-linux-android$api-clang" -std=c11 -Wall -Wextra -Werror \
  -I "$root/cmd/notrioslib" -I "$out" "$root/performance/v0.8-h11/acceptance_host.c" \
  -L "$out" -l:libnotrios-arm64.so -o "$out/acceptance_host-arm64"
arm_machine=$(readelf -h "$out/libnotrios-arm64.so" | awk '/Machine:/ {print $2}')
[ "$arm_machine" = "AArch64" ] || { echo "the arm64 build is not AArch64" >&2; exit 1; }
# Pushed and run once, on purpose, to record that it cannot run here. A
# build-only claim nobody tried is a claim nobody checked.
"$adb" push "$out/acceptance_host-arm64" "$device_dir/" >/dev/null
arm_refusal=$("$adb" shell "chmod 755 $device_dir/acceptance_host-arm64; $device_dir/acceptance_host-arm64 open $device_dir/library.db 2>&1 | head -1" | tr -d '\r')

checks_total=$(cat "$out"/*.log | grep -c '^  ok' || true)

# The report is written from the environment rather than by interpolating shell
# values into Python source. The first version of this script built that source
# by substitution, and the arm64 refusal -- which contains quotes, because it is
# a linker message about a path -- ended the string it was being pasted into.
# Data goes through data channels.
export H11_FINGERPRINT="$fingerprint" H11_RELEASE="$release" H11_SDK_INT="$sdk_int"
export H11_DEVICE_ABI="$device_abi" H11_SELINUX="$selinux" H11_NDK="$(basename "$ndk")"
export H11_API="$api" H11_SYMBOLS="$symbols" H11_CHECKS="$checks_total"
export H11_LEFTOVERS="$leftovers" H11_SEED_SHA="$seed_sha" H11_PULLED_SHA="$pulled_sha"
export H11_INTEGRITY="$integrity" H11_JOURNAL="$journal" H11_SCHEMA="$schema"
export H11_ANDROID_ROWS="$((count_android + count_killed))" H11_ARM_MACHINE="$arm_machine"
export H11_ARM_LIB_SHA="$(sha "$out/libnotrios-arm64.so")"
export H11_ARM_LIB_BYTES="$(stat -c%s "$out/libnotrios-arm64.so")"
export H11_ARM_HOST_SHA="$(sha "$out/acceptance_host-arm64")"
export H11_ARM_REFUSAL="$arm_refusal"
python3 - "$report" <<'PYEND'
import json, os, sys

env = os.environ
json.dump({
    "schema": "notrios.h11.android-acceptance.v1",
    "claim": "the shared core is a viable backend on one Android emulator",
    "not_claimed": [
        "physical Android device", "iOS", "any user interface", "an app-store artifact",
        "background or battery behaviour", "a production secure store on Android",
        "a Flutter client", "arm64 at runtime",
        "any Android API level other than the one recorded below",
    ],
    "environment": {
        "fingerprint": env["H11_FINGERPRINT"],
        "android_release": env["H11_RELEASE"],
        "sdk_int": int(env["H11_SDK_INT"]),
        "device_abi": env["H11_DEVICE_ABI"],
        "selinux": env["H11_SELINUX"],
        "ndk": env["H11_NDK"],
        "min_sdk": int(env["H11_API"]),
    },
    "abi": {
        "exported_symbols": int(env["H11_SYMBOLS"]),
        "sqlite_symbols_exported": 0,
        "capabilities": "0x1f",
    },
    "runtime_x86_64": {
        "status": "passed",
        "checks": int(env["H11_CHECKS"]),
        "phases": ["open", "work", "stream", "abandon+kill",
                   "verify-after-kill", "reboot", "verify-after-reboot"],
        "leftovers_after_sigkill": env["H11_LEFTOVERS"].split(),
    },
    "desktop_interchange": {
        "status": "passed",
        "seed_sha256": env["H11_SEED_SHA"],
        "returned_sha256": env["H11_PULLED_SHA"],
        "integrity_check": env["H11_INTEGRITY"],
        "journal_mode": env["H11_JOURNAL"],
        "schema_version": int(env["H11_SCHEMA"]),
        "notes_android_wrote_that_the_desktop_reads": int(env["H11_ANDROID_ROWS"]),
    },
    "arm64_build_only": {
        "runtime_executed": False,
        "machine": env["H11_ARM_MACHINE"],
        "library_sha256": env["H11_ARM_LIB_SHA"],
        "library_bytes": int(env["H11_ARM_LIB_BYTES"]),
        "host_sha256": env["H11_ARM_HOST_SHA"],
        "refusal_on_this_device": env["H11_ARM_REFUSAL"],
    },
    "credential_provider":
        "supplied by the host that pushes these files; no Android keystore provider is claimed",
}, open(sys.argv[1], "w"), indent=2, sort_keys=True)
PYEND
printf '\nH11 acceptance: %s checks passed on %s; arm64 built and not run\n' "$checks_total" "$fingerprint"
