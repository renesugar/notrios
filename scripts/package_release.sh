#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# Default output lives under dist/ (git-ignored) so archives cannot be
# accidentally committed; pass an explicit path to override.
OUT=${1:-"$ROOT/dist/notrios-src.zip"}
cd "$ROOT"
mkdir -p "$(dirname "$OUT")"

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
  -x '.DS_Store' \
  -x 'dist/*' \
  -x 'bin/*' \
  -x '_site/*' \
  -x '.playwright-mcp/*' \
  -x '*/__pycache__/*' \
  -x '*.pyc' \
  -x '*~' \
  -x '.claude/*' \
  -x 'notrios-*.zip' \
  -x 'coverage.*' \
  -x 'notrios' -x 'notriosd' -x 'notriosctl'
python3 scripts/check_release_zip.py "$OUT"
