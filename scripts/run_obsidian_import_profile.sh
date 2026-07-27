#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PROFILE=${1:-100}
OUTPUT=${2:-/tmp/notrios-obsidian-import-${PROFILE}.json}

case "$PROFILE" in
  100|10000|100000|500000) ;;
  *)
    echo "usage: $0 [100|10000|100000|500000] [output.json]" >&2
    exit 2
    ;;
esac

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

cd "$ROOT"
NOTRIOS_OBSIDIAN_PROFILE="$PROFILE" NOTRIOS_OBSIDIAN_PROFILE_OUTPUT="$OUTPUT" \
  go test -timeout 30m -run '^TestObsidianImporterProfile$' -count=1 -v ./internal/importers/obsidian
echo "profile written to $OUTPUT"
