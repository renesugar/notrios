#!/usr/bin/env bash
# J7-A: archives and library files across the versions 1.0 interoperates with.
#
#   bash performance/v1.0-j7/cross_version.sh <work-dir> <bin-dir> <version:commit> [...]
#
# <bin-dir> holds notriosctl-<commit> for each historical version. The 1.0
# binary is built from this checkout. Each version writes the same generated
# vault (performance/v1.0-j17/j17_make_vault.py).
#
# For each historical version:
#   forward    its archive is verified and restored by 1.0, and its library
#              file is opened by 1.0 (migrating to v28). Both must hold the
#              same notes as a 1.0 import of the same vault.
#   backward   it is given a 1.0 archive and a copy of a 1.0 library. By owner
#              decision (2026-09-15):
#              - the archive passes if it is refused clearly or restored
#                completely, with content equal to the 1.0 library. Any other
#                successful restore is partial or corrupt, and fails.
#              - the library copy passes only if the open is refused and the
#                copy is left unchanged (user_version and content). 0.7.0
#                fails this one cell; it is documented, not fixed.
#
# Nothing here touches a library the drill did not create. Every open of a
# library by a version other than the one that wrote it runs on a copy.
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORK=${1:?work dir}
BIN=${2:?bin dir}
shift 2
[[ $# -ge 1 ]] || { echo "name at least one version:commit" >&2; exit 2; }
[[ ! -e "$WORK" ]] || { echo "$WORK already exists" >&2; exit 2; }
mkdir -p "$WORK"
DIGEST="python3 $ROOT/performance/v1.0-j7/content_digest.py"
RESULTS="$WORK/results.tsv"
printf "version\tcommit\tcheck\tverdict\tdetail\n" > "$RESULTS"
record() { printf "%s\t%s\t%s\t%s\t%s\n" "$1" "$2" "$3" "$4" "$5" | tee -a "$RESULTS"; }
userversion() { python3 -c "import sqlite3,sys; print(sqlite3.connect(f'file:{sys.argv[1]}?mode=ro', uri=True).execute('PRAGMA user_version').fetchone()[0])" "$1"; }
overall() { $DIGEST "$1" 2>/dev/null | head -1 | cut -d' ' -f1; }

(cd "$ROOT" && make build-cli >/dev/null) || { echo "1.0 build failed" >&2; exit 1; }
NEW="$ROOT/bin/notriosctl"
echo "1.0 = $(git -C "$ROOT" rev-parse --short HEAD), tracked changes $(git -C "$ROOT" status --porcelain --untracked-files=no | wc -l)" | tee "$WORK/provenance.txt"

python3 "$ROOT/performance/v1.0-j17/j17_make_vault.py" "$WORK/vault" 300 > /dev/null

# The 1.0 reference: this vault imported and archived by 1.0.
mkdir -p "$WORK/v1.0"
"$NEW" import obsidian --db "$WORK/v1.0/notes.sqlite" --asset-store "$WORK/v1.0/assets" "$WORK/vault" > "$WORK/v1.0/import.json" 2> "$WORK/v1.0/import.err" \
  || { echo "1.0 import failed" >&2; exit 1; }
"$NEW" export archive-v2 --db "$WORK/v1.0/notes.sqlite" --asset-store "$WORK/v1.0/assets" "$WORK/v1.0/archive" > "$WORK/v1.0/export.json" 2> "$WORK/v1.0/export.err" \
  || { echo "1.0 export failed" >&2; exit 1; }
REFERENCE=$(overall "$WORK/v1.0/notes.sqlite")
REFERENCE_VERSION=$(userversion "$WORK/v1.0/notes.sqlite")
echo "1.0 reference content $REFERENCE (user_version $REFERENCE_VERSION)" | tee -a "$WORK/provenance.txt"

for pair in "$@"; do
  version=${pair%%:*}
  commit=${pair##*:}
  OLD="$BIN/notriosctl-$commit"
  D="$WORK/$version"
  mkdir -p "$D"
  [[ -x "$OLD" ]] || { record "$version" "$commit" build fail "no binary at $OLD"; continue; }

  # The historical version writes the vault.
  if ! "$OLD" import obsidian --db "$D/notes.sqlite" --asset-store "$D/assets" "$WORK/vault" > "$D/import.json" 2> "$D/import.err"; then
    record "$version" "$commit" old-import fail "$(tail -1 "$D/import.err")"; continue
  fi
  "$OLD" export archive-v2 --db "$D/notes.sqlite" --asset-store "$D/assets" "$D/archive" > "$D/export.json" 2> "$D/export.err" \
    || { record "$version" "$commit" old-export fail "$(tail -1 "$D/export.err")"; continue; }
  OLD_CONTENT=$(overall "$D/notes.sqlite")
  record "$version" "$commit" old-library info "user_version $(userversion "$D/notes.sqlite"), content $OLD_CONTENT"

  # Forward: 1.0 verifies and restores the old archive.
  if "$NEW" verify archive-v2 "$D/archive" > "$D/new-verify.json" 2> "$D/new-verify.err"; then
    record "$version" "$commit" "forward archive verify" pass ""
  else
    record "$version" "$commit" "forward archive verify" fail "$(tail -1 "$D/new-verify.err")"
  fi
  if "$NEW" restore archive-v2 --intent fork --new-database-id "db_j7_${version//[^a-z0-9]/_}" \
       --db "$D/restored/notes.sqlite" --asset-store "$D/restored/assets" "$D/archive" > "$D/new-restore.json" 2> "$D/new-restore.err"; then
    got=$(overall "$D/restored/notes.sqlite")
    if [[ "$got" == "$REFERENCE" ]]; then
      record "$version" "$commit" "forward archive restore" pass "content equals the 1.0 import"
    else
      record "$version" "$commit" "forward archive restore" fail "content $got differs from 1.0 import $REFERENCE"
    fi
  else
    record "$version" "$commit" "forward archive restore" fail "$(tail -1 "$D/new-restore.err")"
  fi

  # Forward: 1.0 opens a copy of the old library file, migrating it.
  mkdir -p "$D/opened"
  cp "$D/notes.sqlite" "$D/opened/notes.sqlite"
  [[ -f "$D/notes.sqlite-wal" ]] && cp "$D/notes.sqlite-wal" "$D/opened/notes.sqlite-wal"
  cp -r "$D/assets" "$D/opened/assets"
  if "$NEW" export archive-v2 --db "$D/opened/notes.sqlite" --asset-store "$D/opened/assets" "$D/opened/archive" > "$D/new-open.json" 2> "$D/new-open.err"; then
    got=$(overall "$D/opened/notes.sqlite")
    uv=$(userversion "$D/opened/notes.sqlite")
    if [[ "$got" == "$REFERENCE" && "$uv" == "$REFERENCE_VERSION" ]]; then
      record "$version" "$commit" "forward library open" pass "migrated to v$uv, content equals the 1.0 import"
    else
      record "$version" "$commit" "forward library open" fail "v$uv, content $got against 1.0 import $REFERENCE"
    fi
  else
    record "$version" "$commit" "forward library open" fail "$(tail -1 "$D/new-open.err")"
  fi

  # Backward: the old version is given a 1.0 archive. By owner decision
  # (2026-09-15) it passes by refusing clearly, or by restoring completely:
  # content equal to the 1.0 library the archive came from. A restore that
  # succeeds with any other content is partial or corrupt, and fails.
  if "$OLD" restore archive-v2 --intent fork --new-database-id "db_j7_back_${version//[^a-z0-9]/_}" \
       --db "$D/back-restore/notes.sqlite" --asset-store "$D/back-restore/assets" "$WORK/v1.0/archive" > "$D/old-restore.json" 2> "$D/old-restore.err"; then
    got=$(overall "$D/back-restore/notes.sqlite")
    if [[ "$got" == "$REFERENCE" ]]; then
      record "$version" "$commit" "backward archive restore" pass "restored completely: content equals the 1.0 library (user_version $(userversion "$D/back-restore/notes.sqlite"))"
    else
      record "$version" "$commit" "backward archive restore" fail "restored with different content $got; partial or corrupt"
    fi
  else
    record "$version" "$commit" "backward archive restore" pass "refused: $(tail -1 "$D/old-restore.err")"
  fi

  # Backward: the old version opens a copy of a 1.0 library.
  mkdir -p "$D/back-open"
  cp "$WORK/v1.0/notes.sqlite" "$D/back-open/notes.sqlite"
  [[ -f "$WORK/v1.0/notes.sqlite-wal" ]] && cp "$WORK/v1.0/notes.sqlite-wal" "$D/back-open/notes.sqlite-wal"
  cp -r "$WORK/v1.0/assets" "$D/back-open/assets"
  before_uv=$(userversion "$D/back-open/notes.sqlite"); before=$(overall "$D/back-open/notes.sqlite")
  if "$OLD" export archive-v2 --db "$D/back-open/notes.sqlite" --asset-store "$D/back-open/assets" "$D/back-open/archive" > "$D/old-open.json" 2> "$D/old-open.err"; then
    opened=ok
  else
    opened=refused
  fi
  after_uv=$(userversion "$D/back-open/notes.sqlite"); after=$(overall "$D/back-open/notes.sqlite")
  if [[ "$opened" == refused && "$after_uv" == "$before_uv" && "$after" == "$before" ]]; then
    record "$version" "$commit" "backward library open" pass "refused, library unchanged: $(tail -1 "$D/old-open.err")"
  elif [[ "$opened" == refused ]]; then
    record "$version" "$commit" "backward library open" fail "refused, but the library changed: user_version $before_uv -> $after_uv"
  else
    record "$version" "$commit" "backward library open" fail "opened without refusing: user_version $before_uv -> $after_uv, content $([[ "$after" == "$before" ]] && echo unchanged || echo changed)"
  fi
done

echo "results in $RESULTS"
