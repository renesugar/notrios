#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PROFILE=${1:-100000}
OUTPUT=${2:-/tmp/notrios-recoll-hardening-${PROFILE}.json}

case "$PROFILE" in
  100|100000) ;;
  *)
    echo "usage: $0 [100|100000] [output.json]" >&2
    exit 2
    ;;
esac

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

cd "$ROOT"
bash scripts/agent_usage_preflight.sh recoll-hardening-profile
NOTRIOS_RECOLL_PROFILE="$PROFILE" NOTRIOS_RECOLL_PROFILE_OUTPUT="$OUTPUT" \
  go test -timeout 30m -run '^TestRecollHardeningProfile$' -count=1 -v ./internal/recoll
echo "profile written to $OUTPUT"
