#!/usr/bin/env bash
# J22-C: nothing the Go test suite creates in the temp directory outlives it.
#
# J7's disaster-recovery drill was stopped because /tmp, which is RAM-backed
# here, held 16 GB in 17,078 leftover notrios-* entries: stores that never
# removed the asset root they made, a test binary directory built per run and
# never removed, and validators that parked Go build caches there.
#
# Two halves:
#   1. No validator defaults a Go build cache into the temp root. This is a
#      fact about tracked files, so it is checked from them.
#   2. The Go test suite runs with TMPDIR pointed at a fresh directory, and
#      every notrios-* entry left in it afterwards fails the check. Anything
#      else left there is listed, so a new leak under another name is visible.
#
# The check removes only the directory it created for the run.
#
#   bash scripts/check_temp_leaks.sh [go-package ...]   # default ./...
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

status=0

echo "== Go build caches in the temp root"
# Executable sources only: attempt logs and recorded evidence quote the commands
# that were run, and rewriting history is not the fix.
if caches=$(git grep -nE 'GOCACHE.*(/tmp/|gettempdir\(\))' -- Makefile '*.sh' '*.py' ':!scripts/check_temp_leaks.sh'); then
  echo "a validator defaults its Go build cache into the temp root:"
  echo "$caches"
  echo "use the caller's GOCACHE, or Go's own default cache, instead"
  status=1
else
  echo "none: validators use the caller's GOCACHE or Go's default cache"
fi

packages=("$@")
[[ ${#packages[@]} -eq 0 ]] && packages=(./...)

work=$(mktemp -d "${TMPDIR:-/tmp}/j22-temp-check.XXXXXX")
trap 'rm -rf "$work"' EXIT
fresh="$work/tmp"
mkdir -m 0700 "$fresh"

echo "== go test ${packages[*]} with TMPDIR=$fresh"
set +e
TMPDIR="$fresh" go test -count=1 "${packages[@]}"
tests=$?
set -e
if [[ $tests -ne 0 ]]; then
  echo "go test failed (exit $tests)"
  status=1
fi

echo "== left in the fresh temp directory"
leaked=$(find "$fresh" -mindepth 1 -maxdepth 1 -name 'notrios-*' -printf '%f\n' | sort)
other=$(find "$fresh" -mindepth 1 -maxdepth 1 ! -name 'notrios-*' -printf '%f\n' | sort)
if [[ -n $leaked ]]; then
  echo "notrios-* entries left behind, by kind:"
  sed -E 's/[-.][A-Za-z0-9]+$//' <<<"$leaked" | sort | uniq -c
  status=1
else
  echo "no notrios-* entry left"
fi
if [[ -n $other ]]; then
  echo "other entries left (listed, not failed):"
  sed -E 's/[0-9]+$//' <<<"$other" | sort | uniq -c
fi

exit "$status"
