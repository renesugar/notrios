#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP=$(mktemp -d)
PORT=${NOTES_COMPANION_SMOKE_PORT:-18080}
BASE="http://127.0.0.1:${PORT}"
LOG="$TMP/notriosd.log"
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

mcp:
  enabled: true
  default_profile: "read-only"
  max_results: 10
  max_document_bytes: 65536

search_sidecar:
  enabled: false
  binary: "recollindex"
  index_dir: "${TMP}/data/search-index"
YAML

cd "$ROOT"

# A stale server on the port would silently serve this test its old data.
if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then
  echo "port ${PORT} is already in use (stale notriosd from an earlier run?); refusing to run" >&2
  exit 1
fi

# Build and run the binary directly: killing a `go run` wrapper can orphan
# the real server process and leak it across smoke runs.
go build -o "$TMP/notriosd" ./cmd/notriosd
"$TMP/notriosd" -config "$TMP/config.yaml" >"$LOG" 2>&1 &
PID=$!

for _ in $(seq 1 60); do
  if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$PID" 2>/dev/null; then
    cat "$LOG" >&2 || true
    echo "notriosd exited before becoming healthy" >&2
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

# --- Organizer surface (v0.5 E8) --------------------------------------------
# Tag rename with its dry run, and the trash-first delete/restore/purge cycle,
# against a real server rather than only in unit tests.

curl -fsS -X POST "$BASE/api/v1/documents/$DOC_ID/tags/project" >/dev/null
curl -fsS -X POST "$BASE/api/v1/documents/$DOC_ID/tags/project%2Falpha" >/dev/null

# An omitted dry_run must default to true: it reports and changes nothing.
RENAME_RESP=$(curl -fsS -H 'Content-Type: application/json' \
  --data-binary '{"from":"project","to":"work","include_children":true}' "$BASE/api/v1/tags/rename")
python3 -c 'import json,sys; r=json.load(sys.stdin); assert r["dry_run"] is True, r; assert len(r["changes"]) == 2, r' <<<"$RENAME_RESP"
curl -fsS "$BASE/api/v1/tags" | grep -q '"name":"project"'

curl -fsS -H 'Content-Type: application/json' \
  --data-binary '{"from":"project","to":"work","include_children":true,"dry_run":false}' "$BASE/api/v1/tags/rename" >/dev/null
TAGS_RESP=$(curl -fsS "$BASE/api/v1/tags")
python3 -c 'import json,sys; names=sorted(t["name"] for t in json.load(sys.stdin)["tags"]); assert names == ["work","work/alpha"], names' <<<"$TAGS_RESP"

# A notebook deletion preview is read-only and reports the re-homing rule.
NB_RESP=$(curl -fsS -H 'Content-Type: application/json' --data-binary '{"name":"Smoke Notebook"}' "$BASE/api/v1/notebooks")
NB_ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$NB_RESP")
PREVIEW_RESP=$(curl -fsS "$BASE/api/v1/notebooks/$NB_ID/deletion-preview")
python3 -c 'import json,sys; p=json.load(sys.stdin); assert p["deletable"] is True, p; assert p["rehome_notebook_id"] == "nb_notes", p' <<<"$PREVIEW_RESP"
curl -fsS "$BASE/api/v1/notebooks/$NB_ID" >/dev/null   # the preview deleted nothing
curl -fsS -X DELETE "$BASE/api/v1/notebooks/$NB_ID" >/dev/null

# Delete is trash-first, and restore brings the note back.
CURRENT_REV=$(curl -fsS "$BASE/api/v1/documents/$DOC_ID" | python3 -c 'import json,sys; print(json.load(sys.stdin)["current_revision_id"])')
curl -fsS -X DELETE "$BASE/api/v1/documents/$DOC_ID?base_revision_id=$CURRENT_REV" >/dev/null
curl -fsS "$BASE/api/v1/trash" | grep -q "$DOC_ID"
# A trashed note reads back as trashed rather than 404-ing; that is what makes
# it reviewable before a restore.
python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["deleted_at"], d; assert d["editable"] is False, d' \
  <<<"$(curl -fsS "$BASE/api/v1/documents/$DOC_ID")"
curl -fsS -X POST "$BASE/api/v1/trash/$DOC_ID/restore" >/dev/null
python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["editable"] is True, d; assert not d.get("deleted_at"), d' \
  <<<"$(curl -fsS "$BASE/api/v1/documents/$DOC_ID")"

# Purge requires the object-specific confirmation header.
CURRENT_REV=$(curl -fsS "$BASE/api/v1/documents/$DOC_ID" | python3 -c 'import json,sys; print(json.load(sys.stdin)["current_revision_id"])')
curl -fsS -X DELETE "$BASE/api/v1/documents/$DOC_ID?base_revision_id=$CURRENT_REV" >/dev/null
if curl -fsS -X DELETE "$BASE/api/v1/trash/$DOC_ID" >/dev/null 2>&1; then
  echo "purge succeeded without the confirmation header" >&2
  exit 1
fi
curl -fsS -X DELETE -H "X-Notrios-Confirmation: purge-document:$DOC_ID" "$BASE/api/v1/trash/$DOC_ID" >/dev/null
if curl -fsS "$BASE/api/v1/documents/$DOC_ID" >/dev/null 2>&1; then
  echo "purged note is still readable" >&2
  exit 1
fi

# --- Batch organizer transactions (v0.6 F1) ---------------------------------
BATCH_NB=$(curl -fsS -H 'Content-Type: application/json' --data-binary '{"name":"Batch Notebook"}' "$BASE/api/v1/notebooks" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
BATCH_A=$(curl -fsS -H 'Content-Type: application/json' --data-binary '{"title":"Batch A","body":"a"}' "$BASE/api/v1/documents" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
BATCH_B=$(curl -fsS -H 'Content-Type: application/json' --data-binary '{"title":"Batch B","body":"b"}' "$BASE/api/v1/documents" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

# Best effort reports every item, including one that cannot be moved.
BATCH_BODY=$(python3 -c "
import json
print(json.dumps({'operation':'move','notebook_id':'${BATCH_NB}',
  'items':[{'document_id':'${BATCH_A}'},{'document_id':'${BATCH_B}'},{'document_id':'doc_absent'}]}))")
BATCH_RESP=$(curl -fsS -H 'Content-Type: application/json' --data-binary "$BATCH_BODY" "$BASE/api/v1/batch")
python3 -c 'import json,sys; r=json.load(sys.stdin); assert r["applied"]==2 and r["failed"]==1, r; assert len(r["items"])==3, r' <<<"$BATCH_RESP"

# A keyed request replays instead of doing the work twice.
KEYED=$(python3 -c "
import json
print(json.dumps({'request_key':'smoke-1','operation':'add_tags','tags':['smoked'],
  'items':[{'document_id':'${BATCH_A}'}]}))")
curl -fsS -H 'Content-Type: application/json' --data-binary "$KEYED" "$BASE/api/v1/batch" >/dev/null
REPLAY=$(curl -fsS -H 'Content-Type: application/json' --data-binary "$KEYED" "$BASE/api/v1/batch")
python3 -c 'import json,sys; r=json.load(sys.stdin); assert r["replayed"] is True, r' <<<"$REPLAY"
python3 -c 'import json,sys; tags=[t["name"] for t in json.load(sys.stdin)["tags"]]; assert tags.count("smoked")==1, tags' \
  <<<"$(curl -fsS "$BASE/api/v1/documents/$BATCH_A/tags")"

# An atomic run that hits a failure leaves the library untouched.
ATOMIC=$(python3 -c "
import json
print(json.dumps({'operation':'add_tags','mode':'atomic','tags':['never'],
  'items':[{'document_id':'${BATCH_B}'},{'document_id':'doc_absent'}]}))")
ATOMIC_RESP=$(curl -fsS -H 'Content-Type: application/json' --data-binary "$ATOMIC" "$BASE/api/v1/batch")
python3 -c 'import json,sys; r=json.load(sys.stdin); assert r["rolled_back"]==1 and r["failed"]==1, r' <<<"$ATOMIC_RESP"
python3 -c 'import json,sys; tags=[t["name"] for t in json.load(sys.stdin)["tags"]]; assert "never" not in tags, tags' \
  <<<"$(curl -fsS "$BASE/api/v1/documents/$BATCH_B/tags")"

MCP_RESP=$(curl -fsS -H 'Content-Type: application/json' --data-binary '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' "$BASE/mcp")
python3 -c 'import json,sys; resp=json.load(sys.stdin); tools=[tool["name"] for tool in resp["result"]["tools"]]; assert "search_documents" in tools, tools' <<<"$MCP_RESP"

echo "MVP smoke test passed using $BASE"
