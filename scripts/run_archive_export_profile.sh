#!/usr/bin/env bash
# Reproducible v0.4 P3 native archive-v2 streaming export profile.
#
# Each tier seeds a generated library with nested notebooks, tags, shared
# resources, and cross-note links, then measures a full-database export, its
# verification, a resumed export over already-published objects, and a bounded
# notebook subset export. Evidence is aggregate only: no note titles, bodies,
# resources, or local paths are recorded.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PROFILE=${1:-1000}
OUTPUT=${2:-/tmp/notrios-archive-export-${PROFILE}.json}

case "$PROFILE" in
  100|1000|5000|100000) ;;
  *)
    echo "usage: $0 [100|1000|5000|100000] [output.json]" >&2
    exit 2
    ;;
esac

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

cd "$ROOT"
NOTRIOS_ARCHIVE_PROFILE="$PROFILE" NOTRIOS_PROFILE_OUTPUT="$OUTPUT" \
  go test -timeout 30m -run '^TestArchiveExportProfile$' -count=1 -v ./internal/archivev2
echo "profile written to $OUTPUT"
