#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP=$(mktemp -d)
PORT=${NOTES_COMPANION_SMOKE_PORT:-18080}
BASE="http://127.0.0.1:${PORT}"
LOG="$TMP/notesd.log"
PID=""

cleanup() {
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

cat > "$TMP/config.yaml" <<YAML
server:
  listen_addr: "127.0.0.1:${PORT}"
  public_base_url: "${BASE}"

data:
  directory: "${TMP}/data"
  database_path: "${TMP}/data/notes.sqlite"
  asset_store: "${TMP}/data/assets"
  projection_dir: "${TMP}/data/projections"

search:
  default_limit: 20
  max_limit: 100
  max_offset: 10000

mcp:
  enabled: true
  default_profile: "read-only"
  max_results: 10
  max_document_bytes: 65536

sist2:
  enabled: false
  binary: "sist2"
  index_dir: "${TMP}/data/sist2"
YAML

cd "$ROOT"
go run ./cmd/notesd -config "$TMP/config.yaml" >"$LOG" 2>&1 &
PID=$!

for _ in $(seq 1 60); do
  if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$PID" 2>/dev/null; then
    cat "$LOG" >&2 || true
    echo "notesd exited before becoming healthy" >&2
    exit 1
  fi
  sleep 0.25
done
curl -fsS "$BASE/healthz" >/dev/null

CREATE_JSON="$TMP/create.json"
cat > "$CREATE_JSON" <<'JSON'
{
  "title": "Smoke Root",
  "body": "# Smoke Root\n\nThis note mentions durable search and links to [itself](document://default/documents/doc_missing_for_smoke)."
}
JSON
CREATE_RESP=$(curl -fsS -H 'Content-Type: application/json' --data-binary "@$CREATE_JSON" "$BASE/api/v1/documents")
DOC_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$CREATE_RESP")
REV_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["current_revision_id"])' <<<"$CREATE_RESP")

UPDATE_JSON="$TMP/update.json"
python3 - <<PY > "$UPDATE_JSON"
import json
doc_id = "${DOC_ID}"
rev_id = "${REV_ID}"
print(json.dumps({
    "title": "Smoke Root Updated",
    "body": f"# Smoke Root Updated\n\nSearch token smokequery. Link: [self](document://default/documents/{doc_id}).",
    "base_revision_id": rev_id,
}))
PY
curl -fsS -H 'Content-Type: application/json' -X PUT --data-binary "@$UPDATE_JSON" "$BASE/api/v1/documents/$DOC_ID" >/dev/null

SEARCH_RESP=$(curl -fsS -H 'Content-Type: application/json' --data-binary '{"query":"smokequery","limit":5}' "$BASE/api/v1/search")
python3 -c 'import json,sys; resp=json.load(sys.stdin); assert any(hit.get("id") == "'"${DOC_ID}"'" for hit in resp.get("hits", [])), resp' <<<"$SEARCH_RESP"

RESOURCE_RESP=$(printf 'smoke resource content' | curl -fsS -X POST --data-binary @- "$BASE/api/v1/resources?filename=smoke.txt&collection_id=default")
RESOURCE_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$RESOURCE_RESP")
curl -fsS -X POST "$BASE/api/v1/documents/$DOC_ID/resources/$RESOURCE_ID" >/dev/null
curl -fsS "$BASE/api/v1/resources/$RESOURCE_ID/content?download=1" | grep -q 'smoke resource content'

MCP_RESP=$(curl -fsS -H 'Content-Type: application/json' --data-binary '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' "$BASE/mcp")
python3 -c 'import json,sys; resp=json.load(sys.stdin); tools=[tool["name"] for tool in resp["result"]["tools"]]; assert "search_documents" in tools, tools' <<<"$MCP_RESP"

echo "MVP smoke test passed using $BASE"
