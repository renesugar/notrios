#!/usr/bin/env bash
# J20-A — re-import a J5 corpus under the current code, as a new baseline.
#
#   bash performance/v1.0-j20/baseline_profile.sh <label> <joplin-raw|obsidian> <source-dir> <work-dir>
#
# This is performance/v1.0-j5/scale_profile.sh with one change: it records user
# and system CPU as well as peak RSS and wall time. J5's script is left as J5
# ran it, because it is J5's evidence. The import command, the library on disk
# and the search probes are the same, so the numbers sit beside J5's.
#
# J19 found that timings taken while other work shares the machine differ by
# 43%, so run this with nothing else running, one corpus at a time. The page
# cache is not dropped (no passwordless sudo here), and the profile says so.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
LABEL=${1:-}
KIND=${2:-}
SOURCE=${3:-}
WORK=${4:-}

if [[ ! "$LABEL" =~ ^[a-z0-9-]+$ ]] || [[ -z "$SOURCE" ]] || [[ -z "$WORK" ]]; then
  echo "usage: $0 <lowercase-label> <joplin-raw|obsidian> <source-dir> <work-dir>" >&2
  exit 2
fi
case "$KIND" in joplin-raw|obsidian) ;; *) echo "kind must be joplin-raw or obsidian" >&2; exit 2 ;; esac
[[ -d "$SOURCE" ]] || { echo "no such source: $SOURCE" >&2; exit 2; }

CLI=$ROOT/bin/notriosctl
[[ -x "$CLI" ]] || { echo "build it first: make build-cli" >&2; exit 2; }

# The library goes where the caller says, never in the checkout and never in
# /tmp, and never over a run that already exists.
RUN="$WORK/$LABEL"
[[ ! -e "$RUN" ]] || { echo "$RUN already exists; choose a new label or work dir" >&2; exit 2; }
mkdir -p "$RUN"
DB="$RUN/notes.sqlite"
ASSETS="$RUN/assets"
OUT="$RUN/profile.json"
TIMING="$RUN/time.txt"
IMPORT_LOG="$RUN/import.json"

source_bytes=$(du -sb "$SOURCE" | cut -f1)
source_files=$(find "$SOURCE" -type f -name '*.md' | wc -l)
commit=$(git -C "$ROOT" rev-parse HEAD)
dirty=$(git -C "$ROOT" status --porcelain --untracked-files=no | wc -l)

echo "== importing $LABEL ($KIND) from $SOURCE at $commit" >&2
echo "   $source_files markdown files, $source_bytes bytes" >&2
started=$(date -u +%s)
status=0
/usr/bin/time -f '%M %e %U %S' -o "$TIMING" \
  "$CLI" import "$KIND" --db "$DB" --asset-store "$ASSETS" "$SOURCE" \
  > "$IMPORT_LOG" 2>"$RUN/import.err" || status=$?
finished=$(date -u +%s)

read -r peak_rss_kib wall_seconds user_seconds system_seconds < "$TIMING" || true
db_bytes=$(stat -c %s "$DB" 2>/dev/null || echo 0)
assets_bytes=$(du -sb "$ASSETS" 2>/dev/null | cut -f1 || echo 0)

# The same three search shapes J5 timed, cold and warm.
search_json=$(
  python3 - "$CLI" "$DB" <<'PY'
import json, subprocess, sys, time
cli, db = sys.argv[1], sys.argv[2]
rows = []
for name, query in (("common", "the"), ("rare", "quinoa"), ("two-term", "chicken stock")):
    for pass_name in ("cold", "warm"):
        started = time.monotonic()
        done = subprocess.run([cli, "search", "--db", db, "--count", query],
                              capture_output=True, text=True)
        elapsed = time.monotonic() - started
        count = None
        try:
            count = json.loads(done.stdout).get("count")
        except Exception:
            pass
        rows.append({"query": name, "pass": pass_name, "seconds": round(elapsed, 3),
                     "count": count, "exit": done.returncode})
print(json.dumps(rows))
PY
)

python3 - "$OUT" "$LABEL" "$KIND" "$SOURCE" "$status" "$started" "$finished" \
  "${peak_rss_kib:-0}" "${wall_seconds:-0}" "${user_seconds:-0}" "${system_seconds:-0}" \
  "$db_bytes" "$assets_bytes" "$source_bytes" "$source_files" "$IMPORT_LOG" "$search_json" \
  "$commit" "$dirty" <<'PY'
import json, pathlib, sys
(out, label, kind, source, status, started, finished, peak, wall, user, system, db_bytes,
 assets_bytes, source_bytes, source_files, import_log, search_json, commit, dirty) = sys.argv[1:]
log = pathlib.Path(import_log)
report = json.loads(log.read_text() or "{}") if log.stat().st_size else {}
pathlib.Path(out).write_text(json.dumps({
    "label": label,
    "kind": kind,
    "commit": commit,
    "tracked_changes_at_run": int(dirty),
    "page_cache": "not dropped (no passwordless sudo)",
    "completed": status == "0",
    "exit_status": int(status),
    "elapsed_seconds": int(finished) - int(started),
    "wall_seconds": float(wall),
    "user_seconds": float(user),
    "system_seconds": float(system),
    "peak_rss_kib": int(peak or 0),
    "source": {"path": source, "bytes": int(source_bytes), "markdown_files": int(source_files)},
    "library": {"database_bytes": int(db_bytes), "asset_bytes": int(assets_bytes)},
    "import": report,
    "search": json.loads(search_json),
}, indent=2, sort_keys=True) + "\n")
print(f"wrote {out}")
PY
exit "$status"
