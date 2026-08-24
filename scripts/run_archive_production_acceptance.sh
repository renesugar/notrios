#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORKSPACE=${1:-/home/renes/evidence/notrios/g14e-full-workspace}
G14B_WORKSPACE=${G14B_WORKSPACE:-/home/renes/evidence/notrios/g14b-full-workspace}
BIN="$WORKSPACE/bin"
HARNESS="$ROOT/performance/v0.7-g14e/harness.py"

mkdir -p "$BIN"
cd "$ROOT"
go build -o "$BIN/notriosctl" ./cmd/notriosctl
go build -o "$BIN/g14e-tool" ./performance/v0.7-g14e/tool

COMMON=(--workspace "$WORKSPACE" --g14b-workspace "$G14B_WORKSPACE" \
  --notriosctl "$BIN/notriosctl" --tool "$BIN/g14e-tool")

PHASES=(
  corpus-equivalence
  semantic-current-create semantic-current-verify semantic-current-restore
  semantic-previous-verify semantic-previous-restore
  recipe-physical-first recipe-physical-verify recipe-physical-unchanged recipe-physical-restore
  attachment-physical-first attachment-physical-verify attachment-physical-unchanged attachment-physical-restore
  catchup
  restic-canonical-check restic-raw-check borg-canonical-check borg-raw-check
)

for phase in "${PHASES[@]}"; do
  python3 "$HARNESS" "${COMMON[@]}" --phase "$phase"
done
python3 "$HARNESS" "${COMMON[@]}" --assemble --output "$WORKSPACE/full-scale-acceptance.json"
