#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PROFILE=${1:-100}
OUTPUT=${2:-/tmp/notrios-joplin-import-${PROFILE}.json}

case "$PROFILE" in
  100|10000|100000) ;;
  *)
    echo "usage: $0 [100|10000|100000] [output.json]" >&2
    exit 2
    ;;
esac

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

mkdir -p "$(dirname "$OUTPUT")"
PROFILE_TIME=$(mktemp)
PROFILE_JSON=$(mktemp)
PROFILE_CACHE=${NOTRIOS_GO_CACHE:-/tmp/notrios-j3-go-cache}
trap 'rm -f "$PROFILE_TIME" "$PROFILE_JSON"' EXIT

cd "$ROOT"
/usr/bin/time -f '%M' -o "$PROFILE_TIME" \
  env GOCACHE="$PROFILE_CACHE" \
  NOTRIOS_JOPLIN_PROFILE="$PROFILE" NOTRIOS_JOPLIN_PROFILE_OUTPUT="$OUTPUT" \
  go test -timeout 90m -run '^TestJoplinImporterProfile$' -count=1 -v ./internal/importers/joplinraw

PEAK_RSS_KIB=$(tr -d '[:space:]' < "$PROFILE_TIME")
jq --argjson peak_rss_kib "$PEAK_RSS_KIB" '. + {peak_rss_kib: $peak_rss_kib}' "$OUTPUT" > "$PROFILE_JSON"
mv "$PROFILE_JSON" "$OUTPUT"
echo "profile written to $OUTPUT"
