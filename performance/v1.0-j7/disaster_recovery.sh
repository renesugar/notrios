#!/usr/bin/env bash
# J7-C: destroy a library at J5's scale through the purge path, and recover it.
#
#   bash performance/v1.0-j7/disaster_recovery.sh <work-dir> <source-library-dir>
#
# <source-library-dir> holds notes.sqlite (and assets/) of a real library. It is
# only read: the drill works on a copy.
#
# The drill runs 1.0 as an installed program: a binary copied outside the
# checkout, run from inside <work-dir>, with HOME and every XDG root inside
# <work-dir>. So `purge` resolves this drill's roots and nothing else. Before
# the real purge, `purge --dry-run --json` is read, and the drill refuses to go
# on unless every path it would touch is inside <work-dir>.
#
# Steps, each timed. A step that does not complete is reported as incomplete,
# with what it reached:
#   baseline  copy the library into the data root, digest its content, run J5's
#             search probes
#   export    export archive-v2 to an off-site directory, then verify it
#   destroy   purge with a verified backup written off-site
#   recover   (a) restore the archive with --intent adopt into the empty data root
#             (b) extract purge's own backup
#             each compared with the baseline: content digest and search counts
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORK=${1:?work dir}
SOURCE=${2:?source library dir}
[[ -f "$SOURCE/notes.sqlite" ]] || { echo "no notes.sqlite in $SOURCE" >&2; exit 2; }
[[ ! -e "$WORK" ]] || { echo "$WORK already exists" >&2; exit 2; }
WORK=$(mkdir -p "$WORK" && cd "$WORK" && pwd)
case "$WORK" in "$ROOT"|"$ROOT"/*) echo "the work dir must be outside the checkout" >&2; exit 2;; esac

HOMEDIR="$WORK/home"
DATAROOT="$HOMEDIR/.local/share/notrios"
OFFSITE="$WORK/offsite"
mkdir -p "$HOMEDIR" "$WORK/runtime" "$WORK/bin" "$OFFSITE"
chmod 700 "$WORK/runtime"
RESULTS="$WORK/results.tsv"
printf "step\tverdict\tseconds\tdetail\n" > "$RESULTS"
record() { printf "%s\t%s\t%s\t%s\n" "$@" | tee -a "$RESULTS"; }
DIGEST="python3 $ROOT/performance/v1.0-j7/content_digest.py"

(cd "$ROOT" && make build-cli >/dev/null) || { echo "1.0 build failed" >&2; exit 1; }
cp "$ROOT/bin/notriosctl" "$WORK/bin/notriosctl"
CLI="$WORK/bin/notriosctl"
echo "1.0 = $(git -C "$ROOT" rev-parse --short HEAD), tracked changes $(git -C "$ROOT" status --porcelain --untracked-files=no | wc -l)" | tee "$WORK/provenance.txt"
echo "source $SOURCE" | tee -a "$WORK/provenance.txt"

installed() {
  (cd "$WORK" && env -u DBUS_SESSION_BUS_ADDRESS \
    XDG_RUNTIME_DIR="$WORK/runtime" HOME="$HOMEDIR" \
    XDG_CONFIG_HOME="$HOMEDIR/.config" XDG_DATA_HOME="$HOMEDIR/.local/share" \
    XDG_STATE_HOME="$HOMEDIR/.local/state" XDG_CACHE_HOME="$HOMEDIR/.cache" \
    /usr/bin/time -f '%M' -o "$WORK/.last-rss" "$@")
}
# The first run of this drill was stopped from outside, by the machine running
# low on memory, and could only be reported as killed. Each long step now runs
# under a guard: if MemAvailable falls below the threshold, the drill stops that
# step itself and reports it incomplete, with the reading that stopped it.
MEMORY_GUARD_KIB=${J7_MEMORY_GUARD_KIB:-4194304}
kill_tree() { local p; for p in $(ps -o pid= --ppid "$1" 2>/dev/null); do kill_tree "$p"; done; kill -TERM "$1" 2>/dev/null; }
timed() { # timed <command...>: runs it under the memory guard; elapsed seconds on fd 3
  local start end pid rc avail lowest=""
  rm -f "$WORK/.guard-stop"
  start=$(date +%s.%N)
  "$@" &
  pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    avail=$(awk '/^MemAvailable:/{print $2}' /proc/meminfo)
    if [[ -z "$lowest" ]] || (( avail < lowest )); then lowest=$avail; fi
    if (( avail < MEMORY_GUARD_KIB )); then
      echo "stopped by the drill's memory guard at MemAvailable $((avail / 1024)) MiB; " > "$WORK/.guard-stop"
      kill_tree "$pid"
      break
    fi
    sleep 2
  done
  wait "$pid"; rc=$?
  end=$(date +%s.%N)
  echo "${lowest:-0}" > "$WORK/.lowest-available"
  python3 -c "print(f'{$end - $start:.1f}')" >&3
  [[ -f "$WORK/.guard-stop" ]] && return 137
  return $rc
}
exec 3>"$WORK/.last-seconds"
seconds() { tail -1 "$WORK/.last-seconds"; }
peak() { echo "peak RSS $(( $(cat "$WORK/.last-rss" 2>/dev/null || echo 0) / 1024 )) MiB, lowest MemAvailable $(( $(cat "$WORK/.lowest-available" 2>/dev/null || echo 0) / 1024 )) MiB"; }
guardnote() { cat "$WORK/.guard-stop" 2>/dev/null; }
digest_of() { $DIGEST "$1" > "$2" 2>&1; head -1 "$2" | cut -d' ' -f1; }
probes() { # probes <db> <out-file>: J5's three queries, count only
  local db=$1 out=$2 q
  : > "$out"
  for q in the quinoa "chicken stock"; do
    printf "%s\t%s\n" "$q" "$(installed "$CLI" search --db "$db" --count "$q" 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin).get("count"))' 2>/dev/null)" >> "$out"
  done
}

# baseline
mkdir -p "$DATAROOT"
if timed cp "$SOURCE/notes.sqlite" "$DATAROOT/notes.sqlite"; then
  for extra in notes.sqlite-wal notes.sqlite-shm; do [[ -f "$SOURCE/$extra" ]] && cp "$SOURCE/$extra" "$DATAROOT/$extra"; done
  mkdir -p "$DATAROOT/assets"; [[ -d "$SOURCE/assets" ]] && cp -r "$SOURCE/assets/." "$DATAROOT/assets/"
  record "copy library into the data root" pass "$(seconds)" "$(stat -c %s "$DATAROOT/notes.sqlite") bytes"
else
  record "copy library into the data root" incomplete "$(seconds)" "$(guardnote)copy failed"; exit 1
fi
timed installed "$CLI" collections list --db "$DATAROOT/notes.sqlite" --asset-store "$DATAROOT/assets" > "$WORK/open.out" 2>&1
record "open in installed mode" "$([[ $? == 0 ]] && echo pass || echo fail)" "$(seconds)" "$(grep -m1 -i migrat "$WORK/open.out")"
BASELINE=$(timed digest_of "$DATAROOT/notes.sqlite" "$WORK/baseline-digest.txt")
# A digest stopped by the guard, or one that produced nothing, is not a baseline:
# every later comparison would be against an empty string.
if [[ -f "$WORK/.guard-stop" || -z "$BASELINE" ]]; then
  record "baseline content digest" incomplete "$(seconds)" "$(guardnote)no digest"; exit 1
fi
record "baseline content digest" pass "$(seconds)" "$BASELINE"
probes "$DATAROOT/notes.sqlite" "$WORK/baseline-probes.tsv"
record "baseline search probes" pass "" "$(tr '\t\n' '= ' < "$WORK/baseline-probes.tsv")"

# export
if timed installed "$CLI" export archive-v2 --db "$DATAROOT/notes.sqlite" --asset-store "$DATAROOT/assets" "$OFFSITE/archive" > "$WORK/export.json" 2> "$WORK/export.err"; then
  record "export archive-v2 off-site" pass "$(seconds)" "$(du -sb "$OFFSITE/archive" | cut -f1) bytes; $(peak)"
else
  record "export archive-v2 off-site" incomplete "$(seconds)" "$(guardnote)$(grep -m1 -vE '^[[:space:]]*$' "$WORK/export.err"); $(peak)"; exit 1
fi
if timed installed "$CLI" verify archive-v2 "$OFFSITE/archive" > "$WORK/verify.json" 2> "$WORK/verify.err"; then
  record "verify the archive" pass "$(seconds)" ""
else
  record "verify the archive" fail "$(seconds)" "$(grep -m1 -vE '^[[:space:]]*$' "$WORK/verify.err")"; exit 1
fi

# destroy: the plan first, and refuse unless every path is inside the work dir
installed "$CLI" purge --dry-run --json --backup-dir "$OFFSITE/purge-backup" --no-redact > "$WORK/purge-plan.json" 2> "$WORK/purge-plan.err" \
  || { record "purge plan" fail "" "$(grep -m1 -vE '^[[:space:]]*$' "$WORK/purge-plan.err")"; exit 1; }
outside=$(python3 - "$WORK/purge-plan.json" "$WORK" <<'PY'
import json, sys
plan = json.load(open(sys.argv[1])); work = sys.argv[2].rstrip("/") + "/"
paths = [s.get("path", "") for s in plan.get("steps", [])] + [plan.get("backup", "")]
print("\n".join(p for p in paths if p and not (p + "/").startswith(work)))
PY
)
if [[ -n "$outside" ]]; then
  record "purge plan confined to the drill" fail "" "refusing to purge: $(echo "$outside" | tr '\n' ' ')"; exit 1
fi
record "purge plan confined to the drill" pass "" "$(python3 -c "import json,sys; p=json.load(open(sys.argv[1])); print(len(p.get('steps',[])), 'steps, backup', p.get('backup'))" "$WORK/purge-plan.json")"
if timed installed "$CLI" purge --confirm --backup-dir "$OFFSITE/purge-backup" --no-redact > "$WORK/purge.out" 2> "$WORK/purge.err"; then
  record "purge with a verified backup" pass "$(seconds)" "$(grep -m1 'backup verified' "$WORK/purge.out"); $(peak)"
else
  record "purge with a verified backup" incomplete "$(seconds)" "$(grep -m1 -vE '^[[:space:]]*$' "$WORK/purge.err")"; exit 1
fi
if [[ -e "$DATAROOT/notes.sqlite" ]]; then
  record "library destroyed" fail "" "$DATAROOT/notes.sqlite still exists"; exit 1
fi
record "library destroyed" pass "" "no notes.sqlite in the data root"

# recover (a): from the archive
if timed installed "$CLI" restore archive-v2 --intent adopt --db "$DATAROOT/notes.sqlite" --asset-store "$DATAROOT/assets" "$OFFSITE/archive" > "$WORK/restore.json" 2> "$WORK/restore.err"; then
  record "recover from the archive (adopt)" pass "$(seconds)" "$(peak)"
  got=$(timed digest_of "$DATAROOT/notes.sqlite" "$WORK/restored-digest.txt")
  if [[ -f "$WORK/.guard-stop" ]]; then
    record "archive recovery content" incomplete "$(seconds)" "$(guardnote)"
  elif [[ "$got" == "$BASELINE" ]]; then
    record "archive recovery content" pass "$(seconds)" "equals baseline $BASELINE"
  else
    record "archive recovery content" fail "$(seconds)" "$got differs from baseline $BASELINE"
  fi
  probes "$DATAROOT/notes.sqlite" "$WORK/restored-probes.tsv"
  cmp -s "$WORK/baseline-probes.tsv" "$WORK/restored-probes.tsv" && record "archive recovery search probes" pass "" "$(tr '\t\n' '= ' < "$WORK/restored-probes.tsv")" \
    || record "archive recovery search probes" fail "" "baseline $(tr '\t\n' '= ' < "$WORK/baseline-probes.tsv") restored $(tr '\t\n' '= ' < "$WORK/restored-probes.tsv")"
else
  record "recover from the archive (adopt)" incomplete "$(seconds)" "$(guardnote)$(grep -m1 -vE '^[[:space:]]*$' "$WORK/restore.err"); $(peak)"
fi

# recover (b): from purge's own backup
mkdir -p "$WORK/from-purge-backup"
if timed tar -xf "$OFFSITE/purge-backup/backup.tar" -C "$WORK/from-purge-backup"; then
  extracted=$(find "$WORK/from-purge-backup" -name notes.sqlite | head -1)
  record "extract purge's backup" pass "$(seconds)" "$extracted"
  if [[ -n "$extracted" ]]; then
    got=$(timed digest_of "$extracted" "$WORK/purge-backup-digest.txt")
    if [[ -f "$WORK/.guard-stop" ]]; then
      record "purge-backup recovery content" incomplete "$(seconds)" "$(guardnote)"
    elif [[ "$got" == "$BASELINE" ]]; then
      record "purge-backup recovery content" pass "$(seconds)" "equals baseline"
    else
      record "purge-backup recovery content" fail "$(seconds)" "$got differs from baseline $BASELINE"
    fi
  fi
else
  record "extract purge's backup" incomplete "$(seconds)" "tar failed"
fi

echo "== anything named for Notrios written outside the work dir during the drill:"
find "$HOME/.config" "$HOME/.local/share" "$HOME/.local/state" -newer "$WORK/provenance.txt" -iname '*notrios*' 2>/dev/null | head
echo "results in $RESULTS"
