#!/usr/bin/env bash
# J26-F: import the three real conversation archives, as downloaded, and measure
# them.
#
#   bash performance/v1.0-j26/real_archives_run.sh <work-dir> <bin-dir> \
#       chatgpt=<chatgpt-export.zip> portal=<OpenAI-export.zip> claude=<claude-data-….zip>
#
# <bin-dir> holds notriosctl built from the tree under test. The archives are
# private: this script records only counts, sizes and timings, and writes only
# inside <work-dir>, with HOME, the XDG roots and TMPDIR pointed there. Put
# <work-dir> on a disk rather than RAM-backed /tmp.
#
# Each archive gets its own library and three phases, timed with
# /usr/bin/time -v: a dry run, an import, and a re-import that must change
# nothing.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

work=${1:?usage: real_archives_run.sh <work-dir> <bin-dir> name=<archive.zip> ...}
bin=${2:?usage: real_archives_run.sh <work-dir> <bin-dir> name=<archive.zip> ...}
shift 2
[[ $# -gt 0 ]] || { echo "name=<archive.zip> arguments are required" >&2; exit 2; }

bin=$(cd "$bin" && pwd)
mkdir -p "$work"
work=$(cd "$work" && pwd)

export HOME="$work/home"
export XDG_CONFIG_HOME="$HOME/.config" XDG_DATA_HOME="$HOME/.local/share"
export XDG_STATE_HOME="$HOME/.local/state" XDG_CACHE_HOME="$HOME/.cache"
export XDG_RUNTIME_DIR="$work/runtime"
export TMPDIR="$work/tmp"
unset DBUS_SESSION_BUS_ADDRESS
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_STATE_HOME" "$XDG_CACHE_HOME"
mkdir -m 0700 -p "$XDG_RUNTIME_DIR" "$TMPDIR"

summary="$work/summary.txt"
: >"$summary"
note() { echo "$*" | tee -a "$summary"; }
available_mib() { awk '/^MemAvailable:/ {print int($2 / 1024)}' /proc/meminfo; }

# importer <archive> names which command reads it: the portal export is a
# ChatGPT export in its other shape.
importer() {
  case $1 in
    claude) echo claude ;;
    *) echo chatgpt ;;
  esac
}

run_phase() {
  local name=$1 kind=$2 library=$3 archive=$4
  shift 4
  local samples="$work/$name-samples.csv"
  echo "elapsed_s,rss_mib,mem_available_mib" >"$samples"
  local started=$SECONDS
  # Go's flag package stops at the first non-flag argument, so every flag goes
  # before the archive path.
  /usr/bin/time -v "$bin/notriosctl" import "$(importer "$kind")" --db "$library" "$@" "$archive" \
    >"$work/$name.json" 2>"$work/$name-time.txt" &
  local timer=$!
  while kill -0 "$timer" 2>/dev/null; do
    local child rss
    child=$(pgrep -P "$timer" -x notriosctl | head -1 || true)
    rss=0
    [[ -n $child ]] && rss=$(awk '/^VmRSS:/ {print int($2 / 1024)}' "/proc/$child/status" 2>/dev/null || echo 0)
    echo "$((SECONDS - started)),$rss,$(available_mib)" >>"$samples"
    sleep 5
  done
  local status=0
  wait "$timer" || status=$?
  note "== $name: exit $status, $((SECONDS - started)) s"
  grep -E 'Elapsed|Maximum resident' "$work/$name-time.txt" | sed 's/^\s*/   /' | tee -a "$summary"
  python3 - "$work/$name.json" "$samples" <<'EOF' | tee -a "$summary"
import csv, json, sys
try:
    report = json.load(open(sys.argv[1]))
except Exception as error:
    print(f"   no report: {error}")
    report = None
if report is not None:
    for key, value in sorted(report.items()):
        if key == "warnings":
            print(f"   warnings: {len(value or [])}")
        elif isinstance(value, list):
            print(f"   {key}: {len(value)} ({', '.join(str(v) for v in value[:4])}{' …' if len(value) > 4 else ''})")
        else:
            print(f"   {key}: {value}")
rows = list(csv.DictReader(open(sys.argv[2])))
if rows:
    print(f"   sampled peak RSS: {max(int(r['rss_mib']) for r in rows)} MiB; "
          f"lowest MemAvailable: {min(int(r['mem_available_mib']) for r in rows)} MiB")
EOF
  return "$status"
}

note "J26-F real archive runs"
note "notriosctl: $("$bin/notriosctl" version 2>/dev/null | head -1 || echo unknown)"
note "started: $(date -u +%Y-%m-%dT%H:%M:%SZ), MemAvailable $(available_mib) MiB"

failed=0
for pair in "$@"; do
  kind=${pair%%=*}
  archive=${pair#*=}
  [[ -f $archive ]] || { note "!! $kind: no archive at $archive"; failed=1; continue; }
  note ""
  note "### $kind: $(stat -c %s "$archive") bytes"
  library="$work/$kind/library/notes.sqlite"
  rm -rf "$work/$kind"
  mkdir -p "$(dirname "$library")"
  run_phase "$kind-dry-run" "$kind" "$library" "$archive" --dry-run || failed=1
  rm -rf "$work/$kind/library"
  mkdir -p "$(dirname "$library")"
  run_phase "$kind-import" "$kind" "$library" "$archive" || failed=1
  note "   library after import: $(du -sb "$work/$kind/library" | cut -f1) bytes"
  run_phase "$kind-reimport" "$kind" "$library" "$archive" || failed=1
  note "   library after re-import: $(du -sb "$work/$kind/library" | cut -f1) bytes"
done

note ""
note "entries left in TMPDIR: $(find "$TMPDIR" -mindepth 1 -maxdepth 1 | wc -l)"
note "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
exit "$failed"
