#!/usr/bin/env bash
# J7-B: sync over REST between 1.0 and each version it interoperates with.
#
#   bash performance/v1.0-j7/cross_version_rest.sh <work-dir> <bin-dir> <version:commit> [...]
#
# <bin-dir> holds notriosctl-<commit> and notrios-<commit> for each historical
# version. 1.0's notriosctl and notrios are built from this checkout.
#
# Folder sync between pre-1.0 and 1.0 is refused (cross_version_sync.sh): every
# pre-1.0 version caps sync at schema 27, and 1.0 is at 28. J16 also moved the
# REST carrier write path. This drill checks the REST path with real servers,
# so the release documentation can say what happens rather than infer it.
#
# For each historical version:
#   1. a 1.0 library and a copy made from it by export and adopt restore on the
#      historical version pair offline, in the direction that completes (the
#      historical version invites)
#   2. a 1.0 server serves the 1.0 replica; the historical client runs
#      `sync handshake` and `sync exchange` against it
#   3. the historical server serves the historical replica; the 1.0 client does
#      the same
#
# The pass: every REST attempt is refused with an explicit message and a
# non-zero exit, and neither library's content changes. Servers listen only on
# 127.0.0.1. Every command runs behind the keychain guards used by the other
# J7-B drills.
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORK=${1:?work dir}
BIN=${2:?bin dir}
shift 2
[[ $# -ge 1 ]] || { echo "name at least one version:commit" >&2; exit 2; }
[[ ! -e "$WORK" ]] || { echo "$WORK already exists" >&2; exit 2; }
mkdir -p "$WORK/runtime" "$WORK/home"
chmod 700 "$WORK/runtime"
DIGEST="python3 $ROOT/performance/v1.0-j7/content_digest.py"
RESULTS="$WORK/results.tsv"
printf "version\tserver\tclient\tcommand\tverdict\tdetail\n" > "$RESULTS"
record() { printf "%s\t%s\t%s\t%s\t%s\t%s\n" "$@" | tee -a "$RESULTS"; }
guarded() {
  (cd "$WORK" && env -u DBUS_SESSION_BUS_ADDRESS \
    XDG_RUNTIME_DIR="$WORK/runtime" HOME="$WORK/home" \
    XDG_CONFIG_HOME="$WORK/home/.config" XDG_DATA_HOME="$WORK/home/.local/share" \
    XDG_STATE_HOME="$WORK/home/.local/state" XDG_CACHE_HOME="$WORK/home/.cache" \
    "$@")
}
overall() { $DIGEST "$1" 2>/dev/null | head -1 | cut -d' ' -f1; }
firsterr() { grep -m1 -vE '^[[:space:]]*$' "$1"; }

# replica_config <replica-dir> <port>: a config serving this replica on loopback,
# with every data path and the key file inside the replica directory.
replica_config() {
  local dir=$1 port=$2
  cat > "$dir/config.yaml" <<EOF
server:
  listen_addr: "127.0.0.1:$port"
  public_base_url: "http://127.0.0.1:$port"
data:
  directory: "$dir/data"
  database_path: "$dir/notes.sqlite"
  asset_store: "$dir/assets"
  projection_dir: "$dir/data/projections"
search_sidecar:
  enabled: false
  index_dir: "$dir/data/search-index"
remote_media:
  quarantine_dir: "$dir/data/quarantine"
sync:
  rest:
    enabled: true
    require_tls: true
    key_file: "$dir/keys/sync.key"
    credential_store: development-file
EOF
}
flags() { echo "--config $1/config.yaml --db $1/notes.sqlite --asset-store $1/assets --keys $1/keys/sync.key"; }
nokeys() { flags "$1" | sed 's/--keys [^ ]*//'; }
free_port() { python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1', 0)); print(s.getsockname()[1]); s.close()"; }

(cd "$ROOT" && make build-cli >/dev/null && go build -o bin/notrios ./cmd/notrios) || { echo "1.0 build failed" >&2; exit 1; }
cp "$ROOT/bin/notriosctl" "$BIN/notriosctl-1.0-under-test"
cp "$ROOT/bin/notrios" "$BIN/notrios-1.0-under-test"
NEWCTL="$BIN/notriosctl-1.0-under-test"; NEWSRV="$BIN/notrios-1.0-under-test"
echo "1.0 = $(git -C "$ROOT" rev-parse --short HEAD), tracked changes $(git -C "$ROOT" status --porcelain --untracked-files=no | wc -l)" | tee "$WORK/provenance.txt"
python3 "$ROOT/performance/v1.0-j17/j17_make_vault.py" "$WORK/vault-template" 30 > /dev/null

# serve_and_try <version> <server-label> <server-binary> <server-replica> <client-label> <client-binary> <client-replica>
serve_and_try() {
  local version=$1 slabel=$2 sbin=$3 sdir=$4 clabel=$5 cbin=$6 cdir=$7 port pid tries before_s before_c cmd out
  port=$(free_port)
  replica_config "$sdir" "$port"
  before_s=$(overall "$sdir/notes.sqlite"); before_c=$(overall "$cdir/notes.sqlite")
  guarded "$sbin" -config "$sdir/config.yaml" -no-gui > "$sdir/server-$clabel.log" 2>&1 &
  pid=$!
  for tries in $(seq 1 60); do
    curl -fsS "http://127.0.0.1:$port/api/v1/status" > /dev/null 2>&1 && break
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.5
  done
  if ! curl -fsS "http://127.0.0.1:$port/api/v1/status" > /dev/null 2>&1; then
    record "$version" "$slabel" "$clabel" "start server" fail "$(firsterr "$sdir/server-$clabel.log")"
    kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
    return
  fi
  # sync handshake is an authenticated identity check that carries no note
  # content, so it has no pass or fail here: the drill records what each side
  # reports. The first run expected it to refuse, which was wrong.
  out="$cdir/rest-handshake-against-$slabel"
  if guarded "$cbin" sync handshake --url "http://127.0.0.1:$port" $(flags "$cdir") > "$out.out" 2> "$out.err"; then
    record "$version" "$slabel" "$clabel" "sync handshake" info "identity check succeeded; peer reports compatible schema $(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(f\"{d.get('min_compatible_schema')}-{d.get('max_compatible_schema')}\")" "$out.out" 2>/dev/null)"
  else
    record "$version" "$slabel" "$clabel" "sync handshake" info "identity check refused: $(firsterr "$out.err")"
  fi
  # Exchange moves data, so here the owner's rule applies: it must be refused.
  out="$cdir/rest-exchange-against-$slabel"
  if guarded "$cbin" sync exchange --url "http://127.0.0.1:$port" $(flags "$cdir") > "$out.out" 2> "$out.err"; then
    record "$version" "$slabel" "$clabel" "sync exchange" fail "succeeded; data must not move between pre-1.0 and 1.0: $(head -c 200 "$out.out" | tr '\n' ' ')"
  else
    record "$version" "$slabel" "$clabel" "sync exchange" pass "refused: $(firsterr "$out.err")"
  fi
  kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
  if [[ $(overall "$sdir/notes.sqlite") == "$before_s" && $(overall "$cdir/notes.sqlite") == "$before_c" ]]; then
    record "$version" "$slabel" "$clabel" "libraries unchanged" pass ""
  else
    record "$version" "$slabel" "$clabel" "libraries unchanged" fail "content changed during the REST attempts"
  fi
}

for pair in "$@"; do
  version=${pair%%:*}; commit=${pair##*:}
  OLDCTL="$BIN/notriosctl-$commit"; OLDSRV="$BIN/notrios-$commit"
  [[ -x "$OLDCTL" && -x "$OLDSRV" ]] || { record "$version" - - build fail "missing $OLDCTL or $OLDSRV"; continue; }
  D="$WORK/$version"; NEWR="$D/v1.0"; OLDR="$D/$version"
  mkdir -p "$NEWR" "$OLDR"
  replica_config "$NEWR" 0; replica_config "$OLDR" 0

  # 1.0 library; the historical replica is made from it (J7-A: older versions
  # restore 1.0 archives completely).
  guarded "$NEWCTL" import obsidian $(nokeys "$NEWR") "$WORK/vault-template" > "$NEWR/import.out" 2> "$NEWR/import.err" \
    && guarded "$NEWCTL" sync init $(flags "$NEWR") > "$NEWR/init.out" 2> "$NEWR/init.err" \
    && guarded "$NEWCTL" export archive-v2 $(nokeys "$NEWR") "$D/snapshot" > "$NEWR/export.out" 2> "$NEWR/export.err" \
    && guarded "$OLDCTL" restore archive-v2 --intent adopt $(nokeys "$OLDR") "$D/snapshot" > "$OLDR/restore.out" 2> "$OLDR/restore.err" \
    && guarded "$OLDCTL" sync init $(flags "$OLDR") > "$OLDR/init.out" 2> "$OLDR/init.err" \
    || { record "$version" - - setup fail "$(cat "$NEWR"/*.err "$OLDR"/*.err 2>/dev/null | grep -m1 -vE '^[[:space:]]*$')"; continue; }

  # Pair in the direction that completes: the historical version invites.
  guarded "$OLDCTL" sync invite --offline --out "$D/invite.json" --ttl 30m $(flags "$OLDR") > "$OLDR/invite.out" 2> "$OLDR/invite.err" \
    && code=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['code'])" "$OLDR/invite.out") \
    && guarded "$NEWCTL" sync accept --invite "$D/invite.json" --code "$code" --out "$D/acceptance.json" $(flags "$NEWR") > "$NEWR/accept.out" 2> "$NEWR/accept.err" \
    && guarded "$OLDCTL" sync enroll --acceptance "$D/acceptance.json" --code "$code" $(flags "$OLDR") > "$OLDR/enroll.out" 2> "$OLDR/enroll.err" \
    || { record "$version" - - pairing fail "$(cat "$OLDR/invite.err" "$NEWR/accept.err" "$OLDR/enroll.err" 2>/dev/null | grep -m1 -vE '^[[:space:]]*$')"; continue; }
  record "$version" - - pairing pass "$version invited, 1.0 accepted, $version enrolled"

  serve_and_try "$version" 1.0 "$NEWSRV" "$NEWR" "$version" "$OLDCTL" "$OLDR"
  serve_and_try "$version" "$version" "$OLDSRV" "$OLDR" 1.0 "$NEWCTL" "$NEWR"
done

echo "== anything named for Notrios written outside the work dir during the drill:"
find "$HOME/.config" "$HOME/.local/share" -newer "$WORK/provenance.txt" -iname '*notrios*' 2>/dev/null | head
echo "results in $RESULTS"
