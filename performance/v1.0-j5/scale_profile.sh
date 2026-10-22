#!/usr/bin/env bash
# J5 — import a real corpus into a real library, and measure what it cost.
#
#   bash performance/v1.0-j5/scale_profile.sh <label> <joplin-raw|obsidian> <source-dir> <work-dir>
#
# This runs the command a user runs, against a library on disk, and records what
# happened. It is not the existing `run_real_joplin_profile.sh`, which measures
# a *dry run* -- a scan of the export that never writes a note -- and puts its
# library in `t.TempDir()`. On this machine /tmp is tmpfs, so that profile's
# library is in RAM, which is fine for a scan and would be a hazard for a
# 382,000-note import.
#
# Nothing here is extrapolated. A run that does not finish is recorded as not
# finished, with whatever it had reached, which is v0.9 I7's rule.
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
# /tmp. A profile that fills the volume holding the evidence reserve would be a
# performance run that cost a signed archive.
RUN="$WORK/$LABEL"
rm -rf "$RUN"
mkdir -p "$RUN"
DB="$RUN/notes.sqlite"
ASSETS="$RUN/assets"
OUT="$RUN/profile.json"
TIMING="$RUN/time.txt"
IMPORT_LOG="$RUN/import.json"

source_bytes=$(du -sb "$SOURCE" | cut -f1)
source_files=$(find "$SOURCE" -type f -name '*.md' | wc -l)

echo "== importing $LABEL ($KIND) from $SOURCE" >&2
echo "   $source_files markdown files, $source_bytes bytes" >&2
started=$(date -u +%s)
status=0
/usr/bin/time -f '%M %e' -o "$TIMING" \
  "$CLI" import "$KIND" --db "$DB" --asset-store "$ASSETS" "$SOURCE" \
  > "$IMPORT_LOG" 2>"$RUN/import.err" || status=$?
finished=$(date -u +%s)

peak_rss_kib=$(awk '{print $1}' "$TIMING" 2>/dev/null || echo 0)
db_bytes=$(stat -c %s "$DB" 2>/dev/null || echo 0)
assets_bytes=$(du -sb "$ASSETS" 2>/dev/null | cut -f1 || echo 0)

# Search at the size the import produced, timed from the command line because
# that is where a user feels it. Three shapes: a common word, a rare one, and a
# two-term query, each run twice so a cold and a warm number are both recorded
# rather than averaged into something neither.
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
  "$peak_rss_kib" "$db_bytes" "$assets_bytes" "$source_bytes" "$source_files" \
  "$IMPORT_LOG" "$search_json" <<'PY'
import json, pathlib, sys
(out, label, kind, source, status, started, finished, peak, db_bytes, assets_bytes,
 source_bytes, source_files, import_log, search_json) = sys.argv[1:]
report = json.loads(pathlib.Path(import_log).read_text() or "{}") if pathlib.Path(import_log).stat().st_size else {}
pathlib.Path(out).write_text(json.dumps({
    "label": label,
    "kind": kind,
    "source": source,
    "completed": status == "0",
    "exit_status": int(status),
    "elapsed_seconds": int(finished) - int(started),
    "peak_rss_kib": int(peak or 0),
    "source": {"path": source, "bytes": int(source_bytes), "markdown_files": int(source_files)},
    "library": {"database_bytes": int(db_bytes), "asset_bytes": int(assets_bytes)},
    "import": report,
    "search": json.loads(search_json),
}, indent=2, sort_keys=True) + "\n")
print(f"wrote {out}")
PY
exit "$status"
