#!/usr/bin/env bash
# v0.8 H10 — Wails v3 migration spike.
#
# Builds the production v2 shell and the disposable v3 prototype from the same
# tree, runs both under Xvfb, and writes the measurements the decision report is
# made of. Investigation only: nothing here changes the production shell, and
# the prototype is a nested module so the root go.mod never learns that v3
# exists.
#
#   bash performance/v0.8-h10/run_spike.sh
#
# Needs Xvfb and the GTK3+webkit2gtk-4.1 and GTK4+webkitgtk-6.0 stacks, which
# are what the two shells respectively link.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
out=$(mktemp -d "${TMPDIR:-/tmp}/notrios-h10.XXXXXX")
report=${NOTRIOS_H10_REPORT:-$root/performance/v0.8-h10/MEASUREMENTS.json}
display=${NOTRIOS_H10_DISPLAY:-:91}
trap 'rm -rf "$out"; pkill -f "Xvfb $display" 2>/dev/null || true' EXIT

say() { printf '\n== %s\n' "$1"; }

say "building the frontend once, for both shells"
(cd "$root/web" && npm run build >/dev/null 2>&1)

say "building the production v2 shell"
v2_start=$(date +%s)
(cd "$root" && go build -tags "gui desktop production webkit2_41" -o "$out/notrios-v2" ./cmd/notrios)
v2_seconds=$(( $(date +%s) - v2_start ))

say "building the v3 prototype"
v3_start=$(date +%s)
(cd "$root/performance/v0.8-h10/prototype" && GOFLAGS=-mod=mod go build -o "$out/notrios-v3" .)
v3_seconds=$(( $(date +%s) - v3_start ))

# What each binary actually links, which is the number that matters for
# licensing and for size. The module *graph* grows by more than eighty when v3
# is required, because the v3 module ships its CLI; none of that is compiled in.
go version -m "$out/notrios-v2" | awk '$1=="dep"{print $2}' | sort -u > "$out/v2deps"
go version -m "$out/notrios-v3" | awk '$1=="dep"{print $2}' | sort -u > "$out/v3deps"

native() { ldd "$1" | grep -Eo "lib(webkit[a-z0-9]*gtk|gtk)-[0-9.]+\.so\.[0-9]+" | sort -u | tr '\n' ' '; }

say "running both under Xvfb"
Xvfb "$display" -screen 0 1280x800x24 >/dev/null 2>&1 &
sleep 3
launch() { # binary, data dir, screenshot
  local shell=$1 data=$2 shot=$3
  mkdir -p "$data"
  if [ "$shell" = "$out/notrios-v3" ]; then
    DISPLAY=$display "$shell" -assets "$root/web/dist" -data "$data" >"$data/app.log" 2>&1 &
  else
    DISPLAY=$display XDG_CONFIG_HOME=$data/config XDG_DATA_HOME=$data/data \
      XDG_STATE_HOME=$data/state XDG_CACHE_HOME=$data/cache "$shell" >"$data/app.log" 2>&1 &
  fi
  local pid=$!
  sleep 18
  if ! kill -0 $pid 2>/dev/null; then echo "$(basename "$shell") exited before it could be photographed" >&2; return 1; fi
  DISPLAY=$display scrot -o "$shot" 2>/dev/null || true
  DISPLAY=$display xdotool search --name "Notrios" >/dev/null 2>&1 && echo yes || echo no
  kill $pid 2>/dev/null || true
  sleep 2
}
v2_window=$(launch "$out/notrios-v2" "$out/run-v2" "$out/v2.png")
v3_window=$(launch "$out/notrios-v3" "$out/run-v3" "$out/v3.png")

say "two v3 profiles at once, because SingleInstance is opt-in and unset"
DISPLAY=$display "$out/notrios-v3" -assets "$root/web/dist" -data "$out/pA" >"$out/pA.log" 2>&1 & a=$!
sleep 8
DISPLAY=$display "$out/notrios-v3" -assets "$root/web/dist" -data "$out/pB" >"$out/pB.log" 2>&1 & b=$!
sleep 10
both_running=no
kill -0 $a 2>/dev/null && kill -0 $b 2>/dev/null && both_running=yes
separate_databases=$(ls "$out/pA/notes.sqlite" "$out/pB/notes.sqlite" 2>/dev/null | wc -l)
kill $a $b 2>/dev/null || true

say "the production module never learned that v3 exists"
production_clean=no
(cd "$root" && git diff --quiet go.mod go.sum) && production_clean=yes

# How much of the frontend speaks the v2 binding convention, which v3 removes.
frontend_refs=$(grep -rn "window\.go\|\.go?\.main" "$root/web/src" --include='*.ts' --include='*.tsx' \
  | grep -v __tests__ | wc -l)
frontend_files=$(grep -rln "window\.go\|\.go?\.main" "$root/web/src" --include='*.ts' --include='*.tsx' \
  | grep -v __tests__ | wc -l)

export H10_V2_BYTES=$(stat -c%s "$out/notrios-v2") H10_V3_BYTES=$(stat -c%s "$out/notrios-v3")
export H10_V2_SECONDS=$v2_seconds H10_V3_SECONDS=$v3_seconds
export H10_V2_DEPS=$(wc -l < "$out/v2deps") H10_V3_DEPS=$(wc -l < "$out/v3deps")
export H10_ADDED=$(comm -13 "$out/v2deps" "$out/v3deps" | tr '\n' ' ')
export H10_DROPPED=$(comm -23 "$out/v2deps" "$out/v3deps" | tr '\n' ' ')
export H10_V2_NATIVE="$(native "$out/notrios-v2")" H10_V3_NATIVE="$(native "$out/notrios-v3")"
export H10_V2_WINDOW=$v2_window H10_V3_WINDOW=$v3_window
export H10_BOTH_PROFILES=$both_running H10_SEPARATE_DBS=$separate_databases
export H10_PRODUCTION_CLEAN=$production_clean
export H10_FRONTEND_REFS=$frontend_refs H10_FRONTEND_FILES=$frontend_files
export H10_WAILS3=$(cd "$root/performance/v0.8-h10/prototype" && GOFLAGS=-mod=mod go list -m github.com/wailsapp/wails/v3 | awk '{print $2}')
export H10_WAILS2=$(cd "$root" && go list -m github.com/wailsapp/wails/v2 | awk '{print $2}')

python3 - "$report" <<'PYEND'
import json, os, sys

env = os.environ
json.dump({
    "schema": "notrios.h10.wails3-spike.v1",
    "wails": {"production": env["H10_WAILS2"], "prototype": env["H10_WAILS3"]},
    "builds": {
        "v2_bytes": int(env["H10_V2_BYTES"]), "v3_bytes": int(env["H10_V3_BYTES"]),
        "v2_seconds": int(env["H10_V2_SECONDS"]), "v3_seconds": int(env["H10_V3_SECONDS"]),
        "note": "the two binaries are not the same program: the v2 one is the whole notrios "
                "command and the prototype is a shell plus the service, so the sizes are not "
                "a comparison of the frameworks",
    },
    "linked_modules": {
        "v2": int(env["H10_V2_DEPS"]), "v3": int(env["H10_V3_DEPS"]),
        "added_by_v3": env["H10_ADDED"].split(),
        "dropped_by_v3": env["H10_DROPPED"].split(),
    },
    "native_stack": {"v2": env["H10_V2_NATIVE"].split(), "v3": env["H10_V3_NATIVE"].split()},
    "windows_opened": {"v2": env["H10_V2_WINDOW"], "v3": env["H10_V3_WINDOW"]},
    "multiple_profiles": {
        "both_running": env["H10_BOTH_PROFILES"],
        "separate_databases": int(env["H10_SEPARATE_DBS"]),
    },
    "frontend_binding_convention": {
        "references": int(env["H10_FRONTEND_REFS"]),
        "files": int(env["H10_FRONTEND_FILES"]),
        "note": "window.go.main.X.Y does not exist in v3; these are what a migration rewrites",
    },
    "production_untouched": env["H10_PRODUCTION_CLEAN"],
}, open(sys.argv[1], "w"), indent=2, sort_keys=True)
PYEND
printf '\nH10 spike measured: v2 window=%s, v3 window=%s, production untouched=%s\n' \
  "$v2_window" "$v3_window" "$production_clean"
