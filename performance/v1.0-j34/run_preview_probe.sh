#!/usr/bin/env bash
# J34: build the UI, start notriosd on an in-memory library with isolated
# roots, serve a stand-in "remote" origin that logs every request, create a note
# using every J29-covered media form plus inline images, and run
# preview_probe.mjs against it. Prints the probe's JSON report.
#
#   PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs bash performance/v1.0-j34/run_preview_probe.sh
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/notrios-j34.XXXXXX")
cleanup() { kill "${app_pid:-}" "${remote_pid:-}" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT
app_port=${J34_APP_PORT:-18740}
remote_port=${J34_REMOTE_PORT:-18741}

if [[ "${J34_SKIP_WEB_BUILD:-}" != 1 ]]; then (cd "$ROOT/web" && npm run build >/dev/null); fi
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

r="http://127.0.0.1:$remote_port"
inline_png="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
body=$(cat <<MD
# J34 preview probe

![markdown image]($r/markdown.png)

<img src="$r/img-src.png" srcset="$r/img-srcset.png 2x">

<picture><source srcset="$r/picture-source.png"><img src="$r/picture-img.png"></picture>

<video src="$r/video-src.mp4" poster="$r/video-poster.png"><source src="$r/video-source.mp4"><track src="$r/track.vtt"></video>

<audio src="$r/audio-src.mp3"></audio>

<embed src="$r/embed.png">

<object data="$r/object.png"></object>

<svg width="10" height="10"><rect width="10" height="10"/><image href="$r/svg-image.png" width="10" height="10"/></svg>

![inline png]($inline_png)

<img src="$inline_png" alt="inline html png">
MD
)
payload=$(python3 -c 'import json,sys; print(json.dumps({"title": "J34 preview probe", "body": sys.stdin.read()}))' <<<"$body")
document_id=$(curl -fsS -X POST -H 'Content-Type: application/json' -d "$payload" "http://127.0.0.1:$app_port/api/v1/documents" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

node "$ROOT/performance/v1.0-j34/preview_probe.mjs" "http://127.0.0.1:$app_port" "$r" "$document_id"
echo "requests the stand-in remote server received:" >&2
if [[ -s "$work/remote.log" ]]; then sort -u "$work/remote.log" >&2; else echo "(none)" >&2; fi
