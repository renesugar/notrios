#!/usr/bin/env bash
# v0.5 E6a — verify the built-in UI loads nothing from a third party.
#
# Builds the web UI, starts a throwaway notriosd, seeds a note containing math
# and a fenced code block, and drives a real headless Chrome with the cache
# disabled and every known CDN blocked. Fails on any cross-origin request, any
# injected remote script or stylesheet, any CSP violation, or math that did not
# render.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
PORT=${NOTRIOS_OFFLINE_PORT:-18092}
CDP_PORT=${NOTRIOS_CDP_PORT:-9341}

cd "$ROOT"
command -v google-chrome >/dev/null || { echo "google-chrome is required" >&2; exit 2; }

WORK=$(mktemp -d)
cleanup() {
  [[ -n "${SERVICE_PID:-}" ]] && kill "$SERVICE_PID" 2>/dev/null || true
  [[ -n "${CHROME_PID:-}" ]] && kill "$CHROME_PID" 2>/dev/null || true
  sleep 1
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT

echo "building the web UI"
(cd web && npm run build >/dev/null)
go build -o "$WORK/notriosd" ./cmd/notriosd

"$WORK/notriosd" -addr "127.0.0.1:$PORT" -db "$WORK/notes.sqlite" >"$WORK/service.log" 2>&1 &
SERVICE_PID=$!
for _ in $(seq 1 60); do
  curl -sf "http://127.0.0.1:$PORT/healthz" >/dev/null && break
  sleep 0.25
done

python3 - "$WORK/note.json" <<'PY'
import json, sys
body = "\n\n".join([
    "# Offline asset check",
    "Inline math: $E = mc^2$ and $\\alpha + \\beta$.",
    "Block math:",
    "$$\\int_{0}^{\\infty} e^{-x^2}\\,dx = \\frac{\\sqrt{\\pi}}{2}$$",
    "A fenced code block, the other formerly CDN-loaded feature:",
    "```python\ndef f(x):\n    return x ** 2\n```",
])
json.dump({"title": "Offline asset check", "body": body}, open(sys.argv[1], "w"))
PY

DOC_ID=$(curl -sf -X POST "http://127.0.0.1:$PORT/api/v1/documents" \
  -H 'Content-Type: application/json' --data-binary "@$WORK/note.json" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

google-chrome --headless=new --disable-gpu --no-first-run --no-default-browser-check \
  --user-data-dir="$WORK/chrome" --remote-debugging-port="$CDP_PORT" \
  --remote-allow-origins='*' about:blank >"$WORK/chrome.log" 2>&1 &
CHROME_PID=$!

NOTRIOS_CDP_PORT="$CDP_PORT" node scripts/check_offline_assets.mjs "http://127.0.0.1:$PORT" "$DOC_ID"
