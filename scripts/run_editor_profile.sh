#!/usr/bin/env bash
# v0.5 E6 — editor first-paint and keystroke-latency profile.
#
# Builds the web UI, starts a throwaway notriosd, seeds one large note, drives a
# real headless Chrome against it, and writes a JSON report.
#
# Usage: bash scripts/run_editor_profile.sh [output.json] [note-kilobytes]
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUTPUT=${1:-/tmp/notrios-editor-profile.json}
NOTE_KB=${2:-200}
PORT=${NOTRIOS_EDITOR_PORT:-18099}
CDP_PORT=${NOTRIOS_CDP_PORT:-9333}

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi

cd "$ROOT"
bash scripts/agent_usage_preflight.sh editor-profile
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

echo "starting notriosd on 127.0.0.1:$PORT"
"$WORK/notriosd" -addr "127.0.0.1:$PORT" -db "$WORK/notes.sqlite" >"$WORK/service.log" 2>&1 &
SERVICE_PID=$!
for _ in $(seq 1 60); do
  curl -sf "http://127.0.0.1:$PORT/healthz" >/dev/null && break
  sleep 0.25
done

# A large note with links of every resolution kind, so the measured typing path
# includes the decoration field doing real work rather than sitting empty.
python3 - "$WORK/note.json" "$NOTE_KB" <<'PY'
import json, sys
path, kilobytes = sys.argv[1], int(sys.argv[2])
paragraph = ("Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
             "Café résumé naïve — deliberately not ASCII, so byte offsets and "
             "editor indices disagree. ")
lines = []
target = kilobytes * 1024
while sum(len(line) for line in lines) < target:
    n = len(lines)
    lines.append(f"## Section {n}\n")
    lines.append(paragraph * 3 + "\n")
    lines.append(f"[broken {n}](document://default/documents/doc_missing_{n})\n")
    lines.append(f"[external {n}](https://example.com/{n})\n")
body = "\n".join(lines)
json.dump({"title": "Editor profile note", "body": body}, open(path, "w"))
PY

DOC_ID=$(curl -sf -X POST "http://127.0.0.1:$PORT/api/v1/documents" \
  -H 'Content-Type: application/json' --data-binary "@$WORK/note.json" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
NOTE_CHARACTERS=$(python3 -c "import json;print(len(json.load(open('$WORK/note.json'))['body']))")
echo "seeded note $DOC_ID ($NOTE_CHARACTERS characters)"

echo "starting headless chrome on port $CDP_PORT"
google-chrome --headless=new --disable-gpu --no-first-run --no-default-browser-check \
  --user-data-dir="$WORK/chrome" --remote-debugging-port="$CDP_PORT" \
  --remote-allow-origins='*' about:blank >"$WORK/chrome.log" 2>&1 &
CHROME_PID=$!

NOTRIOS_CDP_PORT="$CDP_PORT" NOTRIOS_NOTE_CHARACTERS="$NOTE_CHARACTERS" node scripts/measure_editor.mjs "http://127.0.0.1:$PORT" "$DOC_ID" "$OUTPUT"
echo "editor profile written to $OUTPUT"
