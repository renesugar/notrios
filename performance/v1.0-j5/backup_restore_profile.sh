#!/usr/bin/env bash
# J5 — back up and restore a library at the size an import produced.
#
#   bash performance/v1.0-j5/backup_restore_profile.sh <label> <work-dir>
#
# Runs against the library `scale_profile.sh` left behind, so the size being
# measured is a size something actually built rather than one this script
# arranged. Four steps, each timed separately because they fail differently:
# export archive-v2, verify it, restore it into a *new* library, and count what
# came back.
#
# The restore never targets the source library. `--intent replace` on the
# library that produced the archive would measure the same thing and risk the
# corpus, and a restore that cannot be compared against an original proves
# nothing about whether it worked.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
LABEL=${1:-}
WORK=${2:-}
[[ "$LABEL" =~ ^[a-z0-9-]+$ ]] && [[ -n "$WORK" ]] || { echo "usage: $0 <label> <work-dir>" >&2; exit 2; }

CLI=$ROOT/bin/notriosctl
RUN="$WORK/$LABEL"
DB="$RUN/notes.sqlite"
ASSETS="$RUN/assets"
[[ -f "$DB" ]] || { echo "no library at $DB; run scale_profile.sh first" >&2; exit 2; }

ARCHIVE="$RUN/archive-v2"
RESTORED="$RUN/restored"
OUT="$RUN/backup-restore.json"
rm -rf "$ARCHIVE" "$RESTORED"
mkdir -p "$RESTORED"

# Each step records its own seconds, peak RSS and exit status. A step that fails
# stops the sequence -- restoring an archive that did not export is not a
# measurement -- and what had completed is still written out.
step() {
  local name=$1; shift
  local timing="$RUN/$name.time"
  local log="$RUN/$name.log"
  local status=0
  /usr/bin/time -f '%M %e' -o "$timing" "$@" > "$log" 2>&1 || status=$?
  local peak elapsed
  peak=$(awk 'END{print $1}' "$timing" 2>/dev/null || echo 0)
  elapsed=$(awk 'END{print $2}' "$timing" 2>/dev/null || echo 0)
  printf '%s %s %s %s\n' "$name" "$status" "${peak:-0}" "${elapsed:-0}" >> "$RUN/steps.txt"
  return "$status"
}

: > "$RUN/steps.txt"
overall=0
step export "$CLI" export archive-v2 --db "$DB" --target full_archive "$ARCHIVE" || overall=$?
if [[ $overall -eq 0 ]]; then
  # Named for what it is. `compatibility archive-v2` is a reader-capability
  # preflight over the manifest, which is why it returns in hundredths of a
  # second on a 3 GB archive; calling it "verify" invited the reading that a
  # multi-gigabyte archive had been checked in 30 ms. The content verification
  # runs inside `export archive-v2` unless --no-verify is passed, so it is
  # already inside the export timing above and is not a separate step.
  step compatibility-preflight "$CLI" compatibility archive-v2 "$ARCHIVE" || overall=$?
fi
if [[ $overall -eq 0 ]]; then
  # --new-database-id is required, and the first run of this script learned that
  # the hard way: `--intent fork` alone is refused with "fork requires a
  # distinct explicit new database ID". The refusal is correct -- forking means
  # becoming a new database, and letting the tool invent the identity would make
  # two libraries that disagree about who they are -- so the harness names one.
  step restore "$CLI" restore archive-v2 --intent fork     --new-database-id "db_j5_${LABEL//-/_}_restored"     --db "$RESTORED/notes.sqlite" "$ARCHIVE" || overall=$?
fi

archive_bytes=$(du -sb "$ARCHIVE" 2>/dev/null | cut -f1 || echo 0)
restored_bytes=$(stat -c %s "$RESTORED/notes.sqlite" 2>/dev/null || echo 0)

python3 - "$OUT" "$LABEL" "$RUN/steps.txt" "$archive_bytes" "$restored_bytes" \
  "$(stat -c %s "$DB")" "$overall" <<'PY'
import json, pathlib, sys
out, label, steps_path, archive_bytes, restored_bytes, source_bytes, overall = sys.argv[1:]
steps = []
for line in pathlib.Path(steps_path).read_text().splitlines():
    name, status, peak, elapsed = line.split()
    steps.append({"step": name, "completed": status == "0", "exit_status": int(status),
                  "peak_rss_kib": int(float(peak)), "seconds": float(elapsed)})
pathlib.Path(out).write_text(json.dumps({
    "label": label,
    "completed": overall == "0",
    "steps": steps,
    "bytes": {"source_database": int(source_bytes), "archive": int(archive_bytes),
              "restored_database": int(restored_bytes)},
    "note": ("Restore targets a new library with --intent fork. Replacing the library that "
             "produced the archive would measure the same thing, risk the corpus, and leave "
             "nothing to compare the restore against."),
}, indent=2, sort_keys=True) + "\n")
print(f"wrote {out}")
PY
exit "$overall"
