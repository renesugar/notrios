#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PROFILE=${1:-10000}
OUTPUT=${2:-/tmp/notrios-large-library-${PROFILE}.json}

case "$PROFILE" in
  10000|100000|500000) ;;
  *)
    echo "usage: $0 [10000|100000|500000] [output.json]" >&2
    exit 2
    ;;
esac

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

cd "$ROOT"
bash scripts/agent_usage_preflight.sh large-library-profile
NOTRIOS_SCALE_PROFILE="$PROFILE" NOTRIOS_PROFILE_OUTPUT="$OUTPUT" \
  go test -timeout 30m -run '^TestLargeLibraryProfile$' -count=1 -v ./internal/store
echo "profile written to $OUTPUT"
