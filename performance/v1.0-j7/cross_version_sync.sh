#!/usr/bin/env bash
# J7-B: folder-carrier sync between 1.0 and each version it interoperates with.
#
#   bash performance/v1.0-j7/cross_version_sync.sh <work-dir> <bin-dir> <version:commit> [...]
#
# For each historical version, and in both directions:
#   1. the first replica imports the generated vault and enrols for sync
#   2. the second replica is made from it as the documentation describes:
#      `export archive-v2`, then `restore archive-v2 --intent adopt`, then
#      the second replica enrols
#   3. the two pair offline: `invite --offline`, `accept`, `enroll`
#   4. each replica edits a different note, and `sync once` runs through a
#      shared folder until both have seen both edits, up to a bounded number
#      of rounds
#   5. the pass: both replicas end with identical content
#      (performance/v1.0-j7/content_digest.py), each holding the other's edit
#
# A version with `notes edit` edits through it. A version without it (0.7.0)
# edits the note file in its own copy of the vault and re-imports from that same
# path, which updates the note in place. The record names the mechanism used.
#
# Sync keys must never reach the owner's keychain. Every command runs with an
# explicit development-file credential store, explicit --db and --keys inside
# the work dir, no session bus, and HOME and every XDG root inside the work dir.
# See the J7 README.
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
printf "version\tfirst\tsecond\tcheck\tverdict\tdetail\n" > "$RESULTS"
record() { printf "%s\t%s\t%s\t%s\t%s\t%s\n" "$@" | tee -a "$RESULTS"; }

guarded() {
  (cd "$WORK" && env -u DBUS_SESSION_BUS_ADDRESS \
    XDG_RUNTIME_DIR="$WORK/runtime" HOME="$WORK/home" \
    XDG_CONFIG_HOME="$WORK/home/.config" XDG_DATA_HOME="$WORK/home/.local/share" \
    XDG_STATE_HOME="$WORK/home/.local/state" XDG_CACHE_HOME="$WORK/home/.cache" \
    "$@")
}
overall() { $DIGEST "$1" 2>/dev/null | head -1 | cut -d' ' -f1; }
# The first non-empty line of a command's stderr. A flag error is followed by
# usage text ending in a blank line, so the last line says nothing.
firsterr() { grep -m1 -vE '^[[:space:]]*$' "$1"; }
docid() {
  python3 -c "import sqlite3,sys; r=sqlite3.connect(f'file:{sys.argv[1]}?mode=ro', uri=True).execute('SELECT document_id FROM document_sources WHERE external_id = ?', (sys.argv[2],)).fetchone(); print(r[0] if r else '')" "$1" "$2"
}
body_has() {
  python3 -c "import sqlite3,sys; r=sqlite3.connect(f'file:{sys.argv[1]}?mode=ro', uri=True).execute('SELECT r.body FROM documents d JOIN document_revisions r ON r.id = d.current_revision_id WHERE d.id = ?', (sys.argv[2],)).fetchone(); print('yes' if r and sys.argv[3] in r[0] else 'no')" "$1" "$2" "$3"
}

(cd "$ROOT" && make build-cli >/dev/null) || { echo "1.0 build failed" >&2; exit 1; }
cp "$ROOT/bin/notriosctl" "$BIN/notriosctl-1.0-under-test"
NEW="$BIN/notriosctl-1.0-under-test"
echo "1.0 = $(git -C "$ROOT" rev-parse --short HEAD), tracked changes $(git -C "$ROOT" status --porcelain --untracked-files=no | wc -l)" | tee "$WORK/provenance.txt"
python3 "$ROOT/performance/v1.0-j17/j17_make_vault.py" "$WORK/vault-template" 60 > /dev/null

# replica <dir> <binary>: the paths every command for that replica uses.
flags() { echo "--config $WORK/config.yaml --db $1/notes.sqlite --asset-store $1/assets --keys $1/keys/sync.key"; }

edit_note() { # edit_note <binary> <replica-dir> <external-id> <marker>
  local bin=$1 dir=$2 ext=$3 marker=$4 id
  id=$(docid "$dir/notes.sqlite" "$ext")
  [[ -n "$id" ]] || { echo "no document for $ext"; return 1; }
  if guarded "$bin" notes edit -h >/dev/null 2>&1; then
    # notes edit takes --config, --db and --asset-store, not --keys.
    guarded "$bin" notes edit --document "$id" --body "edited on this replica: $marker" --message "J7-B" $(flags "$dir" | sed 's/--keys [^ ]*//') \
      > "$dir/edit-$marker.out" 2> "$dir/edit-$marker.err" || { firsterr "$dir/edit-$marker.err"; return 1; }
    echo "notes edit"
  else
    printf '# %s\n\nedited on this replica: %s\n' "${ext%.md}" "$marker" > "$dir/vault/$ext"
    guarded "$bin" import obsidian $(flags "$dir" | sed 's/--keys [^ ]*//') "$dir/vault" \
      > "$dir/edit-$marker.out" 2> "$dir/edit-$marker.err" || { firsterr "$dir/edit-$marker.err"; return 1; }
    echo "vault edit and re-import"
  fi
}

drill() { # drill <version> <commit> <first-binary> <first-label> <second-binary> <second-label>
  local version=$1 commit=$2 firstbin=$3 first=$4 secondbin=$5 second=$6
  local D="$WORK/$version/$first-then-$second" A B code mechanism
  A="$D/first"; B="$D/second"
  mkdir -p "$A" "$B" "$D/carrier"
  cp -r "$WORK/vault-template" "$A/vault"

  guarded "$firstbin" import obsidian $(flags "$A" | sed 's/--keys [^ ]*//') "$A/vault" > "$A/import.out" 2> "$A/import.err" \
    || { record "$version" "$first" "$second" setup fail "first import: $(tail -1 "$A/import.err")"; return; }
  guarded "$firstbin" sync init $(flags "$A") > "$A/init.out" 2> "$A/init.err" \
    || { record "$version" "$first" "$second" setup fail "first sync init: $(tail -1 "$A/init.err")"; return; }
  guarded "$firstbin" export archive-v2 $(flags "$A" | sed 's/--keys [^ ]*//') "$D/snapshot" > "$A/export.out" 2> "$A/export.err" \
    || { record "$version" "$first" "$second" setup fail "first export: $(tail -1 "$A/export.err")"; return; }
  guarded "$secondbin" restore archive-v2 --intent adopt $(flags "$B" | sed 's/--keys [^ ]*//') "$D/snapshot" > "$B/restore.out" 2> "$B/restore.err" \
    || { record "$version" "$first" "$second" "second replica by adopt" fail "$(tail -1 "$B/restore.err")"; return; }
  cp -r "$A/vault" "$B/vault"
  guarded "$secondbin" sync init $(flags "$B") > "$B/init.out" 2> "$B/init.err" \
    || { record "$version" "$first" "$second" setup fail "second sync init: $(tail -1 "$B/init.err")"; return; }
  record "$version" "$first" "$second" "second replica by adopt" pass "restored by $second from $first's archive"

  guarded "$firstbin" sync invite --offline --out "$D/invite.json" --ttl 30m $(flags "$A") > "$A/invite.out" 2> "$A/invite.err" \
    || { record "$version" "$first" "$second" pairing fail "invite: $(tail -1 "$A/invite.err")"; return; }
  code=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['code'])" "$A/invite.out")
  guarded "$secondbin" sync accept --invite "$D/invite.json" --code "$code" --out "$D/acceptance.json" $(flags "$B") > "$B/accept.out" 2> "$B/accept.err" \
    || { record "$version" "$first" "$second" pairing fail "accept by $second: $(tail -1 "$B/accept.err")"; return; }
  guarded "$firstbin" sync enroll --acceptance "$D/acceptance.json" --code "$code" $(flags "$A") > "$A/enroll.out" 2> "$A/enroll.err" \
    || { record "$version" "$first" "$second" pairing fail "enroll by $first: $(tail -1 "$A/enroll.err")"; return; }
  record "$version" "$first" "$second" pairing pass "offline invite by $first, accepted by $second, enrolled by $first"

  mechanism=$(edit_note "$firstbin" "$A" "note-0000.md" "J7B-FIRST") \
    || { record "$version" "$first" "$second" "edit on first" fail "$mechanism"; return; }
  local second_mechanism
  # The generated vault puts note i in folder i % 6, so notes 0 and 6 are both
  # at the vault root and their source id is the bare file name.
  second_mechanism=$(edit_note "$secondbin" "$B" "note-0006.md" "J7B-SECOND") \
    || { record "$version" "$first" "$second" "edit on second" fail "$second_mechanism"; return; }

  local round converged=no id0 id1 digest_a digest_b
  id0=$(docid "$A/notes.sqlite" "note-0000.md"); id1=$(docid "$A/notes.sqlite" "note-0006.md")
  for round in $(seq 1 "$ROUNDS"); do
    guarded "$firstbin" sync once --carrier "$D/carrier" $(flags "$A") > "$A/once-$round.out" 2> "$A/once-$round.err" \
      || { record "$version" "$first" "$second" "sync round $round" fail "$first: $(tail -1 "$A/once-$round.err")"; return; }
    guarded "$secondbin" sync once --carrier "$D/carrier" $(flags "$B") > "$B/once-$round.out" 2> "$B/once-$round.err" \
      || { record "$version" "$first" "$second" "sync round $round" fail "$second: $(tail -1 "$B/once-$round.err")"; return; }
    digest_a=$(overall "$A/notes.sqlite"); digest_b=$(overall "$B/notes.sqlite")
    if [[ "$digest_a" == "$digest_b" && $(body_has "$B/notes.sqlite" "$id0" J7B-FIRST) == yes && $(body_has "$A/notes.sqlite" "$id1" J7B-SECOND) == yes ]]; then
      converged=yes; break
    fi
  done
  if [[ "$converged" == yes ]]; then
    record "$version" "$first" "$second" "folder sync converges" pass "after $round round(s); edits by $mechanism on $first and $second_mechanism on $second; content $digest_a"
  else
    record "$version" "$first" "$second" "folder sync converges" fail "not after $ROUNDS rounds: first $digest_a, second $digest_b, second has first's edit: $(body_has "$B/notes.sqlite" "$id0" J7B-FIRST), first has second's edit: $(body_has "$A/notes.sqlite" "$id1" J7B-SECOND)"
  fi
}

for pair in "$@"; do
  version=${pair%%:*}; commit=${pair##*:}
  OLD="$BIN/notriosctl-$commit"
  [[ -x "$OLD" ]] || { record "$version" - - build fail "no binary at $OLD"; continue; }
  drill "$version" "$commit" "$NEW" 1.0 "$OLD" "$version"
  drill "$version" "$commit" "$OLD" "$version" "$NEW" 1.0
done

echo "== anything named for Notrios written outside the work dir during the drill:"
find "$HOME/.config" "$HOME/.local/share" -newer "$WORK/config.yaml" -iname '*notrios*' 2>/dev/null | head
echo "results in $RESULTS"
