#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:-"$ROOT/notes-companion-mvp.zip"}
cd "$ROOT"

go test ./...
python3 scripts/check_required_files.py
bash scripts/validate-scaffold.sh
(cd web && npm ci && npm run build)

rm -f "$OUT"
zip -qr "$OUT" . \
  -x 'web/node_modules/*' \
  -x 'data/*' \
  -x '.git/*' \
  -x '*.sqlite' \
  -x '*.sqlite-*' \
  -x 'tmp/*' \
  -x '.DS_Store'
python3 scripts/check_release_zip.py "$OUT"
