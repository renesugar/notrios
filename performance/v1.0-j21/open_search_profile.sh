#!/usr/bin/env bash
# J21-C: re-measure what opening a library costs, and J5's search probes, on
# libraries that are already at the current schema.
#
#   bash performance/v1.0-j21/open_search_profile.sh <library.sqlite> [more.sqlite ...]
#
# Each library is timed with `collections list`, which opens it and reads
# almost nothing, and with J5's three search shapes, cold and warm. The
# numbers sit beside J20-A's. Run alone: J19 found shared-machine timings
# 43% apart. The page cache is not dropped (no passwordless sudo).
#
# Every library must already be at the current schema. A library that migrates
# would time the migration instead, so the script refuses one whose
# user_version is lower.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
CLI=$ROOT/bin/notriosctl
[[ -x "$CLI" ]] || { echo "build it first: make build-cli" >&2; exit 2; }
[[ $# -ge 1 ]] || { echo "usage: $0 <library.sqlite> [more.sqlite ...]" >&2; exit 2; }

current=$(grep -oE 'CurrentSchemaVersion = [0-9]+' "$ROOT/internal/store/store.go" | grep -oE '[0-9]+$')
echo "commit $(git -C "$ROOT" rev-parse --short HEAD), tracked changes $(git -C "$ROOT" status --porcelain --untracked-files=no | wc -l), schema v$current"

for db in "$@"; do
  version=$(python3 -c "import sqlite3,sys; c=sqlite3.connect(f'file:{sys.argv[1]}?mode=ro', uri=True); print(c.execute('PRAGMA user_version').fetchone()[0])" "$db")
  if [[ "$version" -lt "$current" ]]; then
    echo "$db is at v$version, below v$current: opening it would time a migration" >&2
    exit 2
  fi
  echo "== $db (v$version)"
  for pass in 1 2; do
    /usr/bin/time -f "open (collections list) pass$pass: %e s wall, %U s user, %S s sys" \
      "$CLI" collections list --db "$db" 2>&1 >/dev/null | tail -1
  done
  for query in the quinoa "chicken stock"; do
    for pass in cold warm; do
      /usr/bin/time -f "search \"$query\" $pass: %e s" \
        "$CLI" search --db "$db" --count "$query" 2>&1 >/dev/null | tail -1
    done
  done
done
