#!/usr/bin/env bash
# J22-B drill: two Notrios instances on one machine keep their temp files to
# themselves, checked with real processes rather than in-process tests.
#
#   bash performance/v1.0-j22/instance_temp_drill.sh <work-dir> <bin-dir>
#
# <bin-dir> holds notriosd and notriosctl built from the tree under test. The
# drill writes only inside <work-dir>: HOME, the XDG roots and TMPDIR all point
# there, so it never reaches the owner's own instance, keychain or /tmp.
#
# 1. Instance A's service starts and holds a process directory in A's temp
#    directory. Instance B's service starts beside it and does the same.
# 2. A's service is killed with SIGKILL, so its locked pair is left behind,
#    unlocked.
# 3. A command-line export runs against instance A, packed and verified, so it
#    spools an import manifest and a verification spool. Starting, it must
#    remove A's crashed leftover.
# 4. B's live directory must be untouched, B's service must still answer, and
#    nothing named for Notrios may appear in the system temp directory.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

work=${1:?usage: instance_temp_drill.sh <work-dir> <bin-dir>}
bin=${2:?usage: instance_temp_drill.sh <work-dir> <bin-dir>}
bin=$(cd "$bin" && pwd)
rm -rf "$work"
mkdir -p "$work"
work=$(cd "$work" && pwd)

export HOME="$work/home"
export XDG_CONFIG_HOME="$HOME/.config" XDG_DATA_HOME="$HOME/.local/share"
export XDG_STATE_HOME="$HOME/.local/state" XDG_CACHE_HOME="$HOME/.cache"
export XDG_RUNTIME_DIR="$work/runtime"
export TMPDIR="$work/system-tmp"
unset DBUS_SESSION_BUS_ADDRESS
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_STATE_HOME" "$XDG_CACHE_HOME"
mkdir -m 0700 "$XDG_RUNTIME_DIR" "$TMPDIR"

failed=0
pass() { echo "pass: $*"; }
fail() { echo "FAIL: $*"; failed=1; }

write_config() {
  local name=$1 port=$2
  cat >"$work/$name.yaml" <<EOF
server:
  listen_addr: "127.0.0.1:$port"
  public_base_url: "http://127.0.0.1:$port"
data:
  directory: "$work/$name"
EOF
}

# The process directories in an instance's temp directory, without lock files.
process_dirs() {
  find "$work/$1/tmp" -mindepth 1 -maxdepth 1 -type d -name 'p-*' -printf '%f\n' 2>/dev/null | sort
}

wait_for_process_dir() {
  for _ in $(seq 1 150); do
    [[ -n $(process_dirs "$1") ]] && return 0
    sleep 0.1
  done
  return 1
}

pids=()
cleanup() {
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  wait 2>/dev/null || true
}
trap cleanup EXIT

write_config A 18611
write_config B 18612

"$bin/notriosd" -config "$work/A.yaml" >"$work/A-service.log" 2>&1 &
a_pid=$!
pids+=("$a_pid")
"$bin/notriosd" -config "$work/B.yaml" >"$work/B-service.log" 2>&1 &
b_pid=$!
pids+=("$b_pid")

if wait_for_process_dir A && wait_for_process_dir B; then
  a_crashed=$(process_dirs A)
  b_live=$(process_dirs B)
  pass "each service holds its own process directory: A/tmp/$a_crashed, B/tmp/$b_live"
else
  fail "a service did not create its process directory"
  cat "$work/A-service.log" "$work/B-service.log"
  exit 1
fi
[[ $a_crashed != "$b_live" ]] && pass "the two instances' process directories differ" || fail "instances share a process directory"

kill -KILL "$a_pid"
wait "$a_pid" 2>/dev/null || true
if [[ -d "$work/A/tmp/$a_crashed" && -f "$work/A/tmp/$a_crashed.lock" ]]; then
  pass "SIGKILL left A's pair behind, unlocked: A/tmp/$a_crashed"
else
  fail "the killed service's pair was not left behind, so the sweep is not being tested"
fi

# The system temp directory is read-only for the export. Temporary work there
# is created and removed within the command, so an empty directory afterwards
# proves nothing; a directory it cannot write to proves the export never used it.
chmod 0500 "$TMPDIR"
if "$bin/notriosctl" export archive-v2 --config "$work/A.yaml" --pack "$work/A-archive" >"$work/A-export.log" 2>&1; then
  pass "a packed, verified export ran against instance A with the system temp directory read-only"
else
  fail "the export against instance A failed with the system temp directory read-only"
  cat "$work/A-export.log"
fi
chmod 0700 "$TMPDIR"

if [[ ! -e "$work/A/tmp/$a_crashed" && ! -e "$work/A/tmp/$a_crashed.lock" ]]; then
  pass "the export removed A's crashed leftover when it started"
else
  fail "A's crashed leftover is still there"
fi
remaining=$(find "$work/A/tmp" -mindepth 1 -maxdepth 1 -printf '%f\n' | sort)
if [[ -z $remaining ]]; then
  pass "A's temp directory is empty after the export closed its store"
else
  fail "A's temp directory still holds: $remaining"
fi

if [[ $(process_dirs B) == "$b_live" && -f "$work/B/tmp/$b_live.lock" ]]; then
  pass "B's live process directory is untouched"
else
  fail "B's live process directory changed: $(process_dirs B)"
fi
if kill -0 "$b_pid" 2>/dev/null && curl -fsS "http://127.0.0.1:18612/api/v1/status" >/dev/null; then
  pass "B's service is still running and answering"
else
  fail "B's service is not answering"
fi

leaked=$(find "$TMPDIR" -mindepth 1 -maxdepth 1 -printf '%f\n' | sort)
if [[ -z $leaked ]]; then
  pass "nothing was written to the system temp directory"
else
  fail "the system temp directory holds: $leaked"
fi

exit "$failed"
