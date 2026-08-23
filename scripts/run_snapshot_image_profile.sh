#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUTPUT=${1:-/tmp/notrios-snapshot-image-100000.json}

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

cd "$ROOT"
NOTRIOS_SNAPSHOT_PROFILE=100000 NOTRIOS_SNAPSHOT_PROFILE_OUTPUT="$OUTPUT" \
  go test -timeout 30m -run '^TestGenerated100KSnapshotProfile$' -count=1 -v ./internal/snapshotimage
echo "profile written to $OUTPUT"
