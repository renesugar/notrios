#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUTPUT=${1:-/tmp/notrios-snapshot-restore-100000.json}

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

cd "$ROOT"
bash scripts/agent_usage_preflight.sh snapshot-restore-profile
NOTRIOS_RESTORE_PROFILE=100000 NOTRIOS_RESTORE_PROFILE_OUTPUT="$OUTPUT" \
  go test -timeout 30m -run '^TestGenerated100KRestoreProfile$' -count=1 -v ./internal/snapshotimage
echo "profile written to $OUTPUT"
