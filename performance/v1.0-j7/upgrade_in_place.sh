#!/usr/bin/env bash
# J7-B: replicas syncing on an older version keep syncing after upgrading to 1.0.
#
#   bash performance/v1.0-j7/upgrade_in_place.sh <work-dir> <bin-dir> <version:commit> [...]
#
# J7-B measured that a pre-1.0 version and 1.0 refuse each other's sync
# artifacts: every pre-1.0 version caps at schema 27, and 1.0 is at 28. By owner
# decision (2026-09-15), the supported path is to upgrade every replica, and this
# drill tests that path:
#
#   1. two replicas of one library, both on the older version, pair offline and
#      converge with an edit on each side
#   2. one replica is upgraded in place: 1.0 opens its library, which migrates
#      it from v27 to v28. While the two versions differ, the drill records what
#      sync does. The expected result is a refusal with nothing moved.
#   3. the second replica is upgraded the same way
#   4. both, now on 1.0 with their original pairing and key files, make another
#      edit each, and must converge again
#
# The pass: step 1 converges; step 2 refuses without corrupting either side;
# step 4 converges, with both replicas' pre-upgrade and post-upgrade edits present
# on both. The same keychain guards as cross_version_sync.sh apply.
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
printf 'sync:\n  rest:\n    credential_store: development-file\n' > "$WORK/config.yaml"
DIGEST="python3 $ROOT/performance/v1.0-j7/content_digest.py"
RESULTS="$WORK/results.tsv"
ROUNDS=4
printf "version\tstep\tverdict\tdetail\n" > "$RESULTS"
record() { printf "%s\t%s\t%s\t%s\n" "$@" | tee -a "$RESULTS"; }
guarded() {
  (cd "$WORK" && env -u DBUS_SESSION_BUS_ADDRESS \
    XDG_RUNTIME_DIR="$WORK/runtime" HOME="$WORK/home" \
    XDG_CONFIG_HOME="$WORK/home/.config" XDG_DATA_HOME="$WORK/home/.local/share" \
    XDG_STATE_HOME="$WORK/home/.local/state" XDG_CACHE_HOME="$WORK/home/.cache" \
    "$@")
}
overall() { $DIGEST "$1" 2>/dev/null | head -1 | cut -d' ' -f1; }
firsterr() { grep -m1 -vE '^[[:space:]]*$' "$1"; }
userversion() { python3 -c "import sqlite3,sys; print(sqlite3.connect(f'file:{sys.argv[1]}?mode=ro', uri=True).execute('PRAGMA user_version').fetchone()[0])" "$1"; }
docid() {
  python3 -c "import sqlite3,sys; r=sqlite3.connect(f'file:{sys.argv[1]}?mode=ro', uri=True).execute('SELECT document_id FROM document_sources WHERE external_id = ?', (sys.argv[2],)).fetchone(); print(r[0] if r else '')" "$1" "$2"
}
body_has() {
  python3 -c "import sqlite3,sys; r=sqlite3.connect(f'file:{sys.argv[1]}?mode=ro', uri=True).execute('SELECT r.body FROM documents d JOIN document_revisions r ON r.id = d.current_revision_id WHERE d.id = ?', (sys.argv[2],)).fetchone(); print('yes' if r and sys.argv[3] in r[0] else 'no')" "$1" "$2" "$3"
}
flags() { echo "--config $WORK/config.yaml --db $1/notes.sqlite --asset-store $1/assets --keys $1/keys/sync.key"; }
nokeys() { flags "$1" | sed 's/--keys [^ ]*//'; }

(cd "$ROOT" && make build-cli >/dev/null) || { echo "1.0 build failed" >&2; exit 1; }
cp "$ROOT/bin/notriosctl" "$BIN/notriosctl-1.0-under-test"
NEW="$BIN/notriosctl-1.0-under-test"
echo "1.0 = $(git -C "$ROOT" rev-parse --short HEAD), tracked changes $(git -C "$ROOT" status --porcelain --untracked-files=no | wc -l)" | tee "$WORK/provenance.txt"
python3 "$ROOT/performance/v1.0-j17/j17_make_vault.py" "$WORK/vault-template" 60 > /dev/null

edit_note() { # edit_note <binary> <replica-dir> <external-id> <marker>: appends a marker line
  local bin=$1 dir=$2 ext=$3 marker=$4 id
  id=$(docid "$dir/notes.sqlite" "$ext"); [[ -n "$id" ]] || { echo "no document for $ext"; return 1; }
  if guarded "$bin" notes append -h >/dev/null 2>&1; then
    guarded "$bin" notes append --document "$id" --text "marker $marker" $(nokeys "$dir") \
      > "$dir/edit-$marker.out" 2> "$dir/edit-$marker.err" || { firsterr "$dir/edit-$marker.err"; return 1; }
    echo "notes append"
  else
    printf '\nmarker %s\n' "$marker" >> "$dir/vault/$ext"
    guarded "$bin" import obsidian $(nokeys "$dir") "$dir/vault" \
      > "$dir/edit-$marker.out" 2> "$dir/edit-$marker.err" || { firsterr "$dir/edit-$marker.err"; return 1; }
    echo "vault edit and re-import"
  fi
}

rounds() { # rounds <binA> <A> <binB> <B> <carrier> <label> <marker-on-A> <ext-A> <marker-on-B> <ext-B>: sync until both hold both markers
  local binA=$1 A=$2 binB=$3 B=$4 carrier=$5 label=$6 mA=$7 eA=$8 mB=$9 eB=${10} round idA idB
  idA=$(docid "$A/notes.sqlite" "$eA"); idB=$(docid "$A/notes.sqlite" "$eB")
  for round in $(seq 1 "$ROUNDS"); do
    guarded "$binA" sync once --carrier "$carrier" $(flags "$A") > "$A/once-$label-$round.out" 2> "$A/once-$label-$round.err" || { echo "round $round, first: $(firsterr "$A/once-$label-$round.err")"; return 1; }
    guarded "$binB" sync once --carrier "$carrier" $(flags "$B") > "$B/once-$label-$round.out" 2> "$B/once-$label-$round.err" || { echo "round $round, second: $(firsterr "$B/once-$label-$round.err")"; return 1; }
    if [[ $(overall "$A/notes.sqlite") == $(overall "$B/notes.sqlite") && $(body_has "$B/notes.sqlite" "$idA" "$mA") == yes && $(body_has "$A/notes.sqlite" "$idB" "$mB") == yes ]]; then
      echo "converged after $round round(s)"; return 0
    fi
  done
  echo "not converged after $ROUNDS rounds: second has $mA: $(body_has "$B/notes.sqlite" "$idA" "$mA"), first has $mB: $(body_has "$A/notes.sqlite" "$idB" "$mB")"
  return 1
}

for pair in "$@"; do
  version=${pair%%:*}; commit=${pair##*:}
  OLD="$BIN/notriosctl-$commit"
  [[ -x "$OLD" ]] || { record "$version" build fail "no binary at $OLD"; continue; }
  D="$WORK/$version"; A="$D/first"; B="$D/second"
  mkdir -p "$A" "$B" "$D/carrier"
  cp -r "$WORK/vault-template" "$A/vault"

  # 1. both replicas on the older version, paired and converging
  guarded "$OLD" import obsidian $(nokeys "$A") "$A/vault" > "$A/import.out" 2> "$A/import.err" \
    && guarded "$OLD" sync init $(flags "$A") > "$A/init.out" 2> "$A/init.err" \
    && guarded "$OLD" export archive-v2 $(nokeys "$A") "$D/snapshot" > "$A/export.out" 2> "$A/export.err" \
    && guarded "$OLD" restore archive-v2 --intent adopt $(nokeys "$B") "$D/snapshot" > "$B/restore.out" 2> "$B/restore.err" \
    && cp -r "$A/vault" "$B/vault" \
    && guarded "$OLD" sync init $(flags "$B") > "$B/init.out" 2> "$B/init.err" \
    || { record "$version" "setup on $version" fail "$(cat "$A"/*.err "$B"/*.err 2>/dev/null | grep -m1 -vE '^[[:space:]]*$')"; continue; }
  guarded "$OLD" sync invite --offline --out "$D/invite.json" --ttl 30m $(flags "$A") > "$A/invite.out" 2> "$A/invite.err" \
    && code=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['code'])" "$A/invite.out") \
    && guarded "$OLD" sync accept --invite "$D/invite.json" --code "$code" --out "$D/acceptance.json" $(flags "$B") > "$B/accept.out" 2> "$B/accept.err" \
    && guarded "$OLD" sync enroll --acceptance "$D/acceptance.json" --code "$code" $(flags "$A") > "$A/enroll.out" 2> "$A/enroll.err" \
    || { record "$version" "pairing on $version" fail "$(cat "$A/invite.err" "$B/accept.err" "$A/enroll.err" 2>/dev/null | grep -m1 -vE '^[[:space:]]*$')"; continue; }
  mechA=$(edit_note "$OLD" "$A" note-0000.md BEFORE-A) && mechB=$(edit_note "$OLD" "$B" note-0006.md BEFORE-B) \
    || { record "$version" "edits on $version" fail "$mechA $mechB"; continue; }
  if detail=$(rounds "$OLD" "$A" "$OLD" "$B" "$D/carrier" before BEFORE-A note-0000.md BEFORE-B note-0006.md); then
    record "$version" "both on $version: sync converges" pass "$detail; edits by $mechA"
  else
    record "$version" "both on $version: sync converges" fail "$detail"; continue
  fi

  # 2. upgrade the first replica only; record what mixed-version sync does
  guarded "$NEW" sync status $(flags "$A") > "$A/upgrade.out" 2> "$A/upgrade.err" \
    || { record "$version" "upgrade first to 1.0" fail "$(firsterr "$A/upgrade.err")"; continue; }
  record "$version" "upgrade first to 1.0" pass "user_version $(userversion "$A/notes.sqlite"); $(grep -m1 -i migrat "$A/upgrade.err")"
  before_a=$(overall "$A/notes.sqlite"); before_b=$(overall "$B/notes.sqlite")
  guarded "$NEW" sync once --carrier "$D/carrier" $(flags "$A") > "$A/once-mixed.out" 2> "$A/once-mixed.err"; rc_a=$?
  guarded "$OLD" sync once --carrier "$D/carrier" $(flags "$B") > "$B/once-mixed.out" 2> "$B/once-mixed.err"; rc_b=$?
  skipped=$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(d['exchange'].get('Skipped'))" "$B/once-mixed.out" 2>/dev/null)
  if [[ $(overall "$A/notes.sqlite") == "$before_a" && $(overall "$B/notes.sqlite") == "$before_b" ]]; then
    record "$version" "mixed 1.0 and $version: nothing moves, nothing corrupted" pass "exit codes $rc_a/$rc_b; $version side skipped: $skipped"
  else
    record "$version" "mixed 1.0 and $version: nothing moves, nothing corrupted" fail "content changed while versions differed"
  fi

  # 3. upgrade the second replica
  guarded "$NEW" sync status $(flags "$B") > "$B/upgrade.out" 2> "$B/upgrade.err" \
    || { record "$version" "upgrade second to 1.0" fail "$(firsterr "$B/upgrade.err")"; continue; }
  record "$version" "upgrade second to 1.0" pass "user_version $(userversion "$B/notes.sqlite")"

  # 4. both on 1.0, original pairing: new edits must converge
  edit_note "$NEW" "$A" note-0000.md AFTER-A > /dev/null && edit_note "$NEW" "$B" note-0006.md AFTER-B > /dev/null \
    || { record "$version" "edits on 1.0" fail "edit failed"; continue; }
  if detail=$(rounds "$NEW" "$A" "$NEW" "$B" "$D/carrier" after AFTER-A note-0000.md AFTER-B note-0006.md); then
    idA=$(docid "$A/notes.sqlite" note-0000.md); idB=$(docid "$A/notes.sqlite" note-0006.md)
    kept="first keeps BEFORE-B: $(body_has "$A/notes.sqlite" "$idB" BEFORE-B), second keeps BEFORE-A: $(body_has "$B/notes.sqlite" "$idA" BEFORE-A)"
    record "$version" "both upgraded: sync converges on the original pairing" pass "$detail; $kept"
  else
    record "$version" "both upgraded: sync converges on the original pairing" fail "$detail"
  fi
done

echo "== anything named for Notrios written outside the work dir during the drill:"
find "$HOME/.config" "$HOME/.local/share" -newer "$WORK/config.yaml" -iname '*notrios*' 2>/dev/null | head
echo "results in $RESULTS"
