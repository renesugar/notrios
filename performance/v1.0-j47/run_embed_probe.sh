#!/usr/bin/env bash
# J47: build the UI, start notriosd on an in-memory library with isolated
# roots, serve a stand-in "remote" origin that logs every request, upload one
# PNG attachment, create a note that embeds it as Markdown and as a wiki embed
# beside a remote image and a note embed, and run embed_probe.mjs against it.
# Prints the probe's JSON report. Adapted from performance/v1.0-j34.
#
#   PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs bash performance/v1.0-j47/run_embed_probe.sh
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/notrios-j47.XXXXXX")
cleanup() { kill "${app_pid:-}" "${remote_pid:-}" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT
app_port=${J47_APP_PORT:-18750}
remote_port=${J47_REMOTE_PORT:-18751}

if [[ "${J47_SKIP_WEB_BUILD:-}" != 1 ]]; then (cd "$ROOT/web" && npm run build >/dev/null); fi
go build -o "$work/notriosd" "$ROOT/cmd/notriosd"

cat > "$work/remote.py" <<'PY'
import http.server, sys
port, log = int(sys.argv[1]), sys.argv[2]
PNG = bytes.fromhex("89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000154a24f5d0000000049454e44ae426082")
class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        with open(log, "a") as f: f.write(self.path + "\n")
        self.send_response(200); self.send_header("Content-Type", "image/png"); self.end_headers(); self.wfile.write(PNG)
    def log_message(self, *a): pass
http.server.HTTPServer(("127.0.0.1", port), Handler).serve_forever()
PY
python3 "$work/remote.py" "$remote_port" "$work/remote.log" &
remote_pid=$!

mkdir -p "$work/home" "$work/xdg"
env -u DBUS_SESSION_BUS_ADDRESS HOME="$work/home" XDG_CONFIG_HOME="$work/xdg/config" XDG_DATA_HOME="$work/xdg/data" \
  XDG_STATE_HOME="$work/xdg/state" XDG_CACHE_HOME="$work/xdg/cache" XDG_RUNTIME_DIR="$work/xdg/run" TMPDIR="$work" \
  "$work/notriosd" -config /dev/null -addr "127.0.0.1:$app_port" -db :memory: -web-dir "$ROOT/web/dist" >"$work/app.log" 2>&1 &
app_pid=$!
for _ in $(seq 100); do curl -fsS "http://127.0.0.1:$app_port/api/v1/status" >/dev/null 2>&1 && break; sleep 0.2; done

python3 -c 'import sys; sys.stdout.buffer.write(bytes.fromhex("89504e470d0a1a0a0000000d4948445200000004000000040806000000a9f19e7e0000001249444154789c63306838f01f1933902e0000170726f1d1e6a9550000000049454e44ae426082"))' > "$work/heron.png"
resource_uri=$(curl -fsS -X POST -H 'Content-Type: image/png' --data-binary "@$work/heron.png" \
  "http://127.0.0.1:$app_port/api/v1/resources?filename=heron.png" | python3 -c 'import json,sys; print(json.load(sys.stdin)["uri"])')
r="http://127.0.0.1:$remote_port"
body=$(cat <<MD
# J47 embed probe

![markdown attachment]($resource_uri)

![[$resource_uri|wiki attachment]]

![[$r/wiki-remote.png]]

![markdown remote]($r/markdown-remote.png)

![[document://default/documents/doc_elsewhere]]
MD
)
payload=$(python3 -c 'import json,sys; print(json.dumps({"title": "J47 embed probe", "body": sys.stdin.read()}))' <<<"$body")
document_id=$(curl -fsS -X POST -H 'Content-Type: application/json' -d "$payload" "http://127.0.0.1:$app_port/api/v1/documents" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

node "$ROOT/performance/v1.0-j47/embed_probe.mjs" "http://127.0.0.1:$app_port" "$r" "$document_id"
echo "requests the stand-in remote server received:" >&2
if [[ -s "$work/remote.log" ]]; then sort -u "$work/remote.log" >&2; else echo "(none)" >&2; fi
