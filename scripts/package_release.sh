#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# Default output lives under dist/ (git-ignored) so archives cannot be
# accidentally committed; pass an explicit path to override.
OUT=${1:-"$ROOT/dist/notrios-src.zip"}
cd "$ROOT"
bash scripts/agent_usage_preflight.sh package-release
mkdir -p "$(dirname "$OUT")"

go test ./...
python3 scripts/check_required_files.py
# The workspaces are installed before the scaffold is validated, not after.
#
# validate-scaffold.sh runs `make g18e-validate`, which runs ./cmd/docjourney,
# which resolves the web sources through the TypeScript compiler API -- so it
# needs web/node_modules. This script installed them on the line *after* the
# validation that needs them, which is invisible in a checkout somebody has
# already built in -- the modules are simply there. Run it in a fresh worktree,
# which is exactly what an evidence archive is built from, and it fails:
# "Cannot find module 'typescript'". Found by building the v1.0 J3 archive.
(cd web && npm ci && npm audit && npm run build)
(cd docs-site && npm ci && npm audit)
bash scripts/validate-scaffold.sh
python3 performance/v0.7-g18c/validate_evidence.py
python3 performance/v0.7-g18d/validate_evidence.py
make g18e-validate
make g18f-validate
make g18g-validate
make g19-validate
make g20-validate

rm -f "$OUT"
zip -qr "$OUT" . \
  -x 'node_modules/*' \
  -x 'web/node_modules/*' \
  -x '*/node_modules/*' \
  -x 'data/*' -x 'data/' \
  -x '*/data/*' -x '*/data/' \
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
  -x 'notrios' -x 'notriosd' -x 'notriosctl' -x 'notrioslib' \
  -x '.zvec-grep/*' -x '.zvec-grep' \
  -x 'performance/v0.8-h10/prototype/prototype'
python3 scripts/check_release_zip.py "$OUT"
