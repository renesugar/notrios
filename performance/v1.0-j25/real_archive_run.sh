#!/usr/bin/env bash
# J25-C: import a real Twitter/X archive, as downloaded, and measure it.
#
#   bash performance/v1.0-j25/real_archive_run.sh <archive.zip> <work-dir> <bin-dir>
#
# <bin-dir> holds notriosctl built from the tree under test. The archive is
# private: this script records only counts, sizes and timings, and writes only
# inside <work-dir>, with HOME, the XDG roots and TMPDIR all pointed there.
# <work-dir> belongs on a disk, not RAM-backed /tmp: the library it builds holds
# the archive's media.
#
# Phases, each timed with /usr/bin/time -v:
#   1. dry run of the ZIP
#   2. import of the ZIP into an empty library, with peak RSS sampled every
#      5 s and MemAvailable watched (the run is stopped, and says so, if it
#      falls under NOTRIOS_J25_MIN_AVAILABLE_MIB, default 8192)
#   3. re-import of the same ZIP into the same library, which must change
#      nothing
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

archive=${1:?usage: real_archive_run.sh <archive.zip> <work-dir> <bin-dir>}
work=${2:?usage: real_archive_run.sh <archive.zip> <work-dir> <bin-dir>}
bin=${3:?usage: real_archive_run.sh <archive.zip> <work-dir> <bin-dir>}
minimum_available_mib=${NOTRIOS_J25_MIN_AVAILABLE_MIB:-8192}

bin=$(cd "$bin" && pwd)
mkdir -p "$work"
work=$(cd "$work" && pwd)
library="$work/library/notes.sqlite"

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

# run_phase <name> <args...>: runs notriosctl under /usr/bin/time, sampling the
# process's RSS and the machine's MemAvailable every 5 s.
run_phase() {
  local name=$1
  shift
  local samples="$work/$name-samples.csv"
  echo "elapsed_s,rss_mib,mem_available_mib" >"$samples"
  local started=$SECONDS
  /usr/bin/time -v "$bin/notriosctl" "$@" >"$work/$name.json" 2>"$work/$name-time.txt" &
  local timer=$!
  local stopped=""
  while kill -0 "$timer" 2>/dev/null; do
    local child rss available
    child=$(pgrep -P "$timer" -x notriosctl | head -1 || true)
    rss=0
    [[ -n $child ]] && rss=$(awk '/^VmRSS:/ {print int($2 / 1024)}' "/proc/$child/status" 2>/dev/null || echo 0)
    available=$(available_mib)
    echo "$((SECONDS - started)),$rss,$available" >>"$samples"
    if (( available < minimum_available_mib )) && [[ -n $child ]]; then
      stopped="MemAvailable fell to ${available} MiB, under ${minimum_available_mib} MiB"
      kill "$child" 2>/dev/null || true
    fi
    sleep 5
  done
  local status=0
  wait "$timer" || status=$?
  note "== $name: exit $status, $((SECONDS - started)) s"
  [[ -n $stopped ]] && note "   stopped by the memory guard: $stopped"
  grep -E 'Elapsed|Maximum resident' "$work/$name-time.txt" | sed 's/^\s*/   /' | tee -a "$summary"
  python3 - "$work/$name.json" "$samples" <<'EOF' | tee -a "$summary"
import csv, json, sys
try:
    report = json.load(open(sys.argv[1]))
except Exception as error:
    print(f"   no report: {error}")
    report = None
if report is not None:
    keys = ["source_format", "tweet_files", "posts_in_tweet_files", "tweet_headers",
            "community_posts_seen", "deleted_posts_skipped", "duplicate_posts_skipped",
            "archive_entries_rejected", "tweets_seen", "notes_imported", "notes_updated",
            "notes_unchanged", "threads_recovered", "media_files_in_archive", "media_unmatched",
            "media_imported", "media_missing", "tags_applied", "attachments_linked"]
    for key in keys:
        print(f"   {key}: {report.get(key)}")
    print(f"   warnings: {len(report.get('warnings') or [])}")
rows = list(csv.DictReader(open(sys.argv[2])))
if rows:
    print(f"   sampled peak RSS: {max(int(r['rss_mib']) for r in rows)} MiB; "
          f"lowest MemAvailable: {min(int(r['mem_available_mib']) for r in rows)} MiB")
EOF
  return "$status"
}

note "J25-C real archive run"
note "archive: $(stat -c %s "$archive") bytes, $(unzip -Z1 "$archive" | wc -l) entries"
note "notriosctl: $("$bin/notriosctl" version 2>/dev/null | head -1 || echo unknown)"
note "started: $(date -u +%Y-%m-%dT%H:%M:%SZ), MemAvailable $(available_mib) MiB"

run_phase dry-run import twitter --dry-run --db "$library" "$archive"
rm -rf "$work/library"
run_phase import import twitter --db "$library" "$archive"
note "library after import: $(du -sb "$work/library" | cut -f1) bytes"
run_phase reimport import twitter --db "$library" "$archive"
note "library after re-import: $(du -sb "$work/library" | cut -f1) bytes"
leftover=$(find "$TMPDIR" -mindepth 1 -maxdepth 1 | wc -l)
note "entries left in TMPDIR: $leftover"
note "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
