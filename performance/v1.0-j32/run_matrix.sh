#!/usr/bin/env bash
# Measure every J32 case for one or two harness binaries (v1.0 J32).
#
#   bash performance/v1.0-j32/run_matrix.sh <work-dir> <out-dir> <label>[,<label>] [case ...]
#
# Runs run_bench.py once per case, one after another, writing
# <out-dir>/<shape>-<scenario>.json. With no cases named it runs the whole
# matrix, smallest first. Run it detached, with nothing else on the machine.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

HERE=$(cd "$(dirname "$0")" && pwd)
WORK=${1:?work dir}
OUT=${2:?output dir}
LABELS=${3:?labels}
shift 3
CASES=("$@")
if [[ ${#CASES[@]} -eq 0 ]]; then
  CASES=(collision/fresh collision/reimport
         obsidian-10k/fresh obsidian-10k/reimport
         joplin-10k/fresh joplin-10k/reimport
         link-dense/fresh link-dense/reimport
         near-limit-obsidian/fresh near-limit-obsidian/reimport
         near-limit-joplin/fresh near-limit-joplin/reimport)
fi

mkdir -p "$OUT"
for case in "${CASES[@]}"; do
  name=${case/\//-}
  echo "== $case $(date -u +%FT%TZ)"
  python3 "$HERE/run_bench.py" run "$WORK" "$OUT/$name.json" "$case" "$LABELS"
done
echo "== done $(date -u +%FT%TZ)"
