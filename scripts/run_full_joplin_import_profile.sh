#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
LABEL=${1:-}
SOURCE=${2:-}
OUTPUT=${3:-}

if [[ ! "$LABEL" =~ ^[a-z0-9-]+$ ]] || [[ ! -d "$SOURCE" ]] || [[ -z "$OUTPUT" ]]; then
  echo "usage: $0 <lowercase-label> <joplin-raw-directory> <output.json>" >&2
  exit 2
fi
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
  NOTRIOS_JOPLIN_FULL_SOURCE="$SOURCE" \
  NOTRIOS_JOPLIN_FULL_LABEL="$LABEL" \
  NOTRIOS_JOPLIN_FULL_OUTPUT="$OUTPUT" \
  go test -timeout 3h -run '^TestJ3RealExportFullImportProfile$' -count=1 -v ./internal/importers/joplinraw

PEAK_RSS_KIB=$(tr -d '[:space:]' < "$PROFILE_TIME")
jq --argjson peak_rss_kib "$PEAK_RSS_KIB" '
  . + {
    peak_rss_kib: $peak_rss_kib,
    resume_notes_per_second: (if .resume_milliseconds > 0 then (.notes_imported * 1000 / .resume_milliseconds) else 0 end),
    total_notes_per_second: (if .total_milliseconds > 0 then (.notes_imported * 1000 / .total_milliseconds) else 0 end)
  }
' "$OUTPUT" > "$PROFILE_JSON"
mv "$PROFILE_JSON" "$OUTPUT"
echo "private-safe aggregate full-import profile written to $OUTPUT"
