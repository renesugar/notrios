#!/usr/bin/env bash
# Prove that the tests which run a real Recoll binary reach nothing of the owner's.
#
# Recoll can run external indexers that find their own data; the Joplin one in
# Recoll 1.44 looks under HOME. A sidecar test that left HOME alone once read the
# owner's real Joplin library (v1.0 J44). The tests now isolate HOME and the XDG
# directories (internal/recoll/recolltest) and the sidecar declares no external
# indexer, but those are claims about the code. This traces every file the live
# tests open and every program they start, and fails if any path under the real
# HOME is opened outside the repository and Go's own caches, or if a Recoll
# external indexer is started at all.
#
# Opt-in, not part of the scaffold validation: it needs strace and permission to
# trace a child process, which a CI runner may not grant.
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

for tool in strace recollindex recollq; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "recoll isolation: SKIPPED, $tool is not installed"
    exit 0
  fi
done

work=$(mktemp -d -t notrios-recoll-isolation.XXXXXX)
trap 'rm -rf "$work"' EXIT
allowed=("$ROOT" "$(go env GOMODCACHE)" "$(go env GOCACHE)" "$(go env GOPATH)")

run_traced() { # package, test pattern
  local package=$1 pattern=$2 name
  name=$(basename "$package")
  go test -c -o "$work/$name.test" "./$package"
  (cd "$package" && strace -f -qq -e trace=open,openat,openat2,execve \
    -o "$work/$name.trace" "$work/$name.test" -test.run "$pattern" -test.count=1 >"$work/$name.out" 2>&1) || {
    cat "$work/$name.out"
    echo "recoll isolation: FAILED, $package $pattern did not pass under trace"
    exit 1
  }
  if ! grep -q '^PASS' "$work/$name.out"; then
    cat "$work/$name.out"; echo "recoll isolation: FAILED, no PASS from $package"; exit 1
  fi
}

run_traced internal/service 'TestLiveSidecarStartupRepairsDamageAndReportsStatus'
run_traced internal/recoll 'TestLiveRecollPipeline'

status=0
# An external indexer started at all is a failure, whatever it would have read.
if grep -hoE 'execve\("[^"]*(rcl[a-z]+\.py)"' "$work"/*.trace | grep -v 'notrios_md_handler' | grep -q .; then
  echo "recoll isolation: FAILED, a Recoll external indexer was started:"
  grep -hoE 'execve\("[^"]*rcl[a-z]+\.py"' "$work"/*.trace | sort -u
  status=1
fi
# Any path under the real HOME outside the repository and Go's caches.
home_paths=$(grep -hoE "\"$HOME/[^\"]*\"" "$work"/*.trace | tr -d '"' | sort -u || true)
outside=""
while IFS= read -r path; do
  [ -n "$path" ] || continue
  inside=0
  for root in "${allowed[@]}"; do
    case "$path" in "$root"|"$root"/*) inside=1; break;; esac
  done
  [ "$inside" = 1 ] || outside+="$path"$'\n'
done <<<"$home_paths"
if [ -n "$outside" ]; then
  echo "recoll isolation: FAILED, live Recoll tests opened paths in the owner's home directory:"
  printf '%s' "$outside" | head -20
  status=1
fi
if [ "$status" = 0 ]; then
  echo "recoll isolation: OK, $(cat "$work"/*.trace | wc -l) traced calls, nothing of the owner's opened and no external indexer started"
fi
exit "$status"
