#!/usr/bin/env bash
# J11 — what a clean installation occupies, captured by running one.
#
#   bash performance/v1.0-j11/capture_clean_install.sh
#
# It installs into its own HOME outside the checkout, for the reason J3's drills
# learned: inside a checkout the CLI resolves source mode and would report the
# developer's own library. Then it runs `notriosctl paths --report` three ways,
# purges, and runs scripts/check_purged.sh against the manifest taken
# beforehand.
#
# Everything committed is redacted to `~`, except the path manifest, which is
# the shape a checker reads and is literal by definition. That one stays in the
# scratch directory and is recorded here as counts and verdicts only.
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/performance/v1.0-j11
OUT=${J11_OUT:-$HERE}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/notrios-j11-XXXXXX")
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

home=$WORK/home
mkdir -p "$home"
run_in_home() { ( cd "$home" && env -i PATH="$PATH" HOME="$home" "$@" ); }

echo "# installing into $home/.local"
( cd "$ROOT" && env -i PATH="$PATH" HOME="$home" prefix="$home/.local" \
    python3 scripts/lifecycle.py install ) > "$WORK/install.log" 2>&1 || {
  echo "install failed:" >&2; tail -20 "$WORK/install.log" >&2; exit 1; }

cli=$home/.local/bin/notriosctl
[[ -x "$cli" ]] || { echo "the install did not produce $cli" >&2; exit 1; }

# The command under test is the one just built, not whatever the install staged
# from a previous build -- the same correction J3's drills make.
( cd "$ROOT" && go build -o "$WORK/notriosctl" ./cmd/notriosctl ) || exit 1
cp "$WORK/notriosctl" "$cli"

mode=$(run_in_home "$cli" paths 2>/dev/null | awk -F': ' '/^mode:/{print $2}')
[[ "$mode" == installed ]] || { echo "resolved mode is '$mode', not installed" >&2; exit 1; }

# A library with something in it, so the manifest has files to account for.
run_in_home "$cli" notes create --title "the note this capture purges" --body body >/dev/null 2>&1 || true

echo "# the report, as a person reads it"
run_in_home "$cli" paths --report > "$OUT/CLEAN_INSTALL.txt" 2>&1
echo "# the report, as a script reads it"
run_in_home "$cli" paths --report --json > "$OUT/CLEAN_INSTALL.json" 2>&1
echo "# the manifest the post-purge check reads, kept outside the library"
run_in_home "$cli" paths --report --paths > "$WORK/before-purge.txt" 2>&1

owned=$(grep -c '^owned' "$WORK/before-purge.txt" || true)
external=$(grep -c '^external' "$WORK/before-purge.txt" || true)
echo "# $owned owned path(s), $external external path(s) before the purge"

echo "# purging"
run_in_home "$cli" purge --confirm --no-backup --no-plan > "$WORK/purge.log" 2>&1 || {
  echo "purge failed:" >&2; tail -20 "$WORK/purge.log" >&2; exit 1; }

echo "# checking the purge against the manifest"
if bash "$ROOT/scripts/check_purged.sh" "$WORK/before-purge.txt" > "$WORK/check.log" 2>&1; then
  status=verified
else
  status=incomplete
fi
cat "$WORK/check.log"

# And the failure the check exists for: put one file back and run it again.
left=$(grep '^owned' "$WORK/before-purge.txt" | tail -1 | cut -f2-)
mkdir -p "$(dirname "$left")" && echo "left behind" > "$left"
if bash "$ROOT/scripts/check_purged.sh" "$WORK/before-purge.txt" > "$WORK/check-left.log" 2>&1; then
  left_status=missed
else
  left_status=caught
fi

python3 - "$OUT/PURGE_CHECK.json" "$owned" "$external" "$status" "$left_status" \
    "$WORK/check.log" "$WORK/check-left.log" <<'PY'
import json
import pathlib
import sys

out, owned, external, status, left_status, check, check_left = sys.argv[1:8]
left_said = pathlib.Path(check_left).read_text(encoding="utf-8").strip().splitlines()
pathlib.Path(out).write_text(json.dumps({
    "schema": "notrios.j11.purge-check/1",
    "what_this_is": (
        "One run of capture_clean_install.sh: a clean install into a disposable HOME, the "
        "structure-and-manifest report taken before a purge, the purge, and the check that "
        "reads that manifest afterwards. The manifest's paths are this run's scratch "
        "directory, so only counts and verdicts are recorded; the reports beside this file "
        "are redacted."),
    "owned_paths_before_purge": int(owned),
    "external_paths_before_purge": int(external),
    "after_purge": {
        "status": status,
        "said": pathlib.Path(check).read_text(encoding="utf-8").strip(),
    },
    "with_one_file_left_behind": {
        "status": left_status,
        "said": left_said[0] if left_said else "",
    },
}, indent=2) + "\n", encoding="utf-8")
print(f"wrote {out}")
PY
[[ "$status" == verified && "$left_status" == caught ]]
