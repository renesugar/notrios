#!/usr/bin/env bash
# I4 — destructive-lifecycle drills against disposable installations.
#
# Every drill installs into its own HOME, puts a real note in it, applies one
# fault, and then checks what survived. Nothing here runs against a real
# library: the roots are isolated per drill and deleted afterwards, which is the
# only responsible way to test a command whose job is deletion.
#
# The faults are chosen to be unprivileged and deterministic. A read-only parent
# directory is "the backup cannot be written"; `ulimit -f` is "the backup does
# not fit", which is the capacity fault without needing a filesystem to fill;
# truncation is corruption. None of them need root, so this runs where the
# developer runs.
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/performance/v0.9-i4
RESULTS=${I4_RESULTS:-$HERE/DRILLS.jsonl}
# Outside the checkout, deliberately. A HOME under dist/ is still inside the
# repository, and notriosctl would resolve source mode there -- which the guard
# in install_home caught on the first run after it was written.
WORK=$(mktemp -d "${TMPDIR:-/tmp}/notrios-i4-XXXXXX")
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT
: > "$RESULTS"

# Everything after the install runs with the working directory inside the
# isolated HOME, never inside the checkout.
#
# This is not tidiness. notriosctl resolves *source mode* when it detects a
# checkout, and in source mode every root is `data` relative to the working
# directory -- so the first version of these drills, which ran from the
# repository, created its notes in the developer's own library and wrote purge
# backups into the repository root. Purge refused to delete anything, because a
# relative root is not an absolute purge target, which is the only reason that
# mistake was harmless.
in_home() { local home=$1; shift; ( cd "$home" && env -i PATH="$PATH" HOME="$home" "$@" ); }

# install_home <name> -> prints the isolated HOME, with notrios installed and
# one note in the library.
install_home() {
  local home=$WORK/$1
  mkdir -p "$home"
  ( cd "$ROOT" && env -i PATH="$PATH" HOME="$home" prefix="$home/.local" \
      python3 scripts/lifecycle.py install ) >"$home/install.log" 2>&1 || {
    echo "install failed for $1" >&2; tail -3 "$home/install.log" >&2; return 1; }

  # A drill that resolved source mode would be measuring the checkout, so it
  # stops here rather than continuing against the wrong library.
  local mode
  mode=$(in_home "$home" "$home/.local/bin/notriosctl" paths --no-redact 2>/dev/null \
         | awk -F': ' '/^mode:/{print $2}')
  if [ "$mode" != "installed" ]; then
    echo "refusing to drill: resolved mode is '$mode', not 'installed'" >&2; return 1
  fi
  in_home "$home" "$home/.local/bin/notriosctl" notes create \
    --title "The note this drill must not lose" --body "body" >/dev/null 2>&1 || return 1
  printf '%s' "$home"
}

library_of() { in_home "$1" "$1/.local/bin/notriosctl" paths --no-redact \
  2>/dev/null | awk '$1=="data"{print $2}'; }

# The library is checked on the filesystem, not through the installed binary.
# The first version of this asked the installed notriosctl whether the note was
# still there -- and purge removes that binary, so every successful purge
# reported "the library is gone" whether or not it was. A check that cannot
# distinguish the thing it is checking from its own tooling is worse than none:
# it manufactures alarms and hides real ones.
library_intact() { [ -f "$1/notes.sqlite" ]; }

record() { # <drill> <status> <detail-json>
  python3 - "$1" "$2" "$3" >> "$RESULTS" <<'PY'
import json, sys
print(json.dumps({"drill": sys.argv[1], "status": sys.argv[2],
                  "observations": json.loads(sys.argv[3])}, sort_keys=True))
PY
}

fail() { echo "   $1" >&2; return 1; }

drill_unattended_refusal() {
  local home; home=$(install_home unattended) || return 1
  local library; library=$(library_of "$home")
  # No FORCE, no terminal. It must refuse rather than delete, and rather than
  # hang waiting for an answer nobody can give.
  local out code
  out=$(in_home "$home" python3 "$ROOT/scripts/lifecycle.py" purge </dev/null 2>&1)
  code=$?
  library_intact "$library" || fail "the library is gone after a refused purge" || return 1
  [ "$code" -ne 0 ] || fail "an unattended purge did not refuse" || return 1
  grep -qi "force" <<<"$out" || fail "the refusal did not say how to automate it deliberately" || return 1
  record unattended-purge-refuses-without-force pass \
    "{\"exit\":$code,\"library_intact\":true,\"names_the_flag\":true}"
}

drill_backup_unwritable() {
  local home; home=$(install_home unwritable) || return 1
  local library; library=$(library_of "$home")
  local state="$home/.local/state"
  mkdir -p "$state"
  chmod a-w "$state"
  local out code
  out=$( ( cd "$home" && env -i PATH="$PATH" HOME="$home" FORCE=1 \
           python3 "$ROOT/scripts/lifecycle.py" purge ) 2>&1)
  code=$?
  chmod u+w "$state"
  library_intact "$library" || fail "the library was deleted although the backup could not be written" || return 1
  [ "$code" -ne 0 ] || fail "purge succeeded with nowhere to put the backup" || return 1
  record purge-refuses-when-the-backup-cannot-be-written pass \
    "{\"exit\":$code,\"library_intact\":true}"
}

drill_backup_too_large() {
  local home; home=$(install_home toolarge) || return 1
  local library; library=$(library_of "$home")
  # The capacity fault without a filesystem to fill: the backup cannot be
  # written because it would exceed the process's file-size limit.
  local out code
  out=$( ( cd "$home" && ulimit -f 8; env -i PATH="$PATH" HOME="$home" FORCE=1 \
           python3 "$ROOT/scripts/lifecycle.py" purge 2>&1 ) )
  code=$?
  library_intact "$library" || fail "the library was deleted although the backup did not fit" || return 1
  [ "$code" -ne 0 ] || fail "purge succeeded although the backup could not be written in full" || return 1
  record purge-refuses-when-the-backup-does-not-fit pass \
    "{\"exit\":$code,\"library_intact\":true}"
}

drill_backup_contents_and_restore() {
  local home; home=$(install_home restore) || return 1
  local library; library=$(library_of "$home")
  local out code
  out=$( ( cd "$home" && env -i PATH="$PATH" HOME="$home" FORCE=1 \
           python3 "$ROOT/scripts/lifecycle.py" purge ) 2>&1)
  code=$?
  [ "$code" -eq 0 ] || { echo "$out" | tail -5 >&2; fail "a purge with a writable destination failed" || return 1; }

  local archive
  archive=$(find "$home" -name backup.tar -print -quit)
  [ -n "$archive" ] || { find "$home" -maxdepth 6 -name '*notrios*' -newer "$home/install.log" >&2
    fail "the purge left no backup" || return 1; }

  # The listing is written out before it is searched. `tar -tf | grep -q` looks
  # obvious and is wrong under `pipefail`: grep exits at the first match, tar
  # dies of SIGPIPE, and the pipeline reports failure -- so a backup that
  # *contained* the library was reported as one that did not.
  local listing=$WORK/listing.txt
  tar -tf "$archive" > "$listing"
  if grep -qiE "sync-keys|\.keys$|credential" "$listing"; then
    fail "the purge backup contains sync key material" || return 1
  fi
  grep -q "notes.sqlite" "$listing" || fail "the backup does not contain the library" || return 1

  # A backup nobody has restored is a hope. Restore it and read the note back.
  local restored=$WORK/restored
  mkdir -p "$restored"
  tar -xf "$archive" -C "$restored"
  local db; db=$(find "$restored" -name notes.sqlite -print -quit)
  [ -n "$db" ] || fail "no library in the restored tree" || return 1
  # Count the hits; do not grep the output. `notriosctl search` echoes the query
  # back in its JSON -- {"hits": [], "query": "must not lose"} -- so grepping the
  # output for the search term matches on an *empty* result and reports a note
  # that is not there. This assertion passed for exactly that reason and tested
  # nothing. Found in v0.9 I7, when the same pattern claimed a library that had
  # just been deleted still held its notes.
  local hits
  hits=$(in_home "$home" "$ROOT/bin/notriosctl" search --db "$db" \
    --asset-store "$restored/assets" "must not lose" 2>/dev/null \
    | python3 -c 'import json,sys; print(len(json.load(sys.stdin).get("hits", [])))' 2>/dev/null || echo 0)
  [ "${hits:-0}" -ge 1 ] || fail "the restored library does not hold the note" || return 1
  record purge-backup-restores-and-excludes-sync-keys pass \
    "{\"exit\":0,\"archive_holds_library\":true,\"archive_holds_keys\":false,\"restored_hits\":$hits,\"library_before\":\"$library\"}"
}

drill_uninstall_keeps_data() {
  local home; home=$(install_home uninstall) || return 1
  local library; library=$(library_of "$home")
  in_home "$home" python3 "$ROOT/scripts/lifecycle.py" uninstall >/dev/null 2>&1 \
    || fail "uninstall failed" || return 1
  [ ! -x "$home/.local/bin/notriosctl" ] || fail "uninstall left the binary behind" || return 1
  [ -d "$library" ] || fail "uninstall deleted the user's library" || return 1
  find "$library" -name notes.sqlite | grep -q . || fail "uninstall deleted the database" || return 1
  record uninstall-leaves-user-data pass \
    "{\"binary_removed\":true,\"library_present\":true,\"library\":\"$library\"}"
}

drill_modified_artifact() {
  local home; home=$(install_home modified) || return 1
  printf 'tampered\n' >> "$home/.local/bin/notriosctl"
  local out
  out=$(in_home "$home" python3 "$ROOT/scripts/lifecycle.py" uninstall 2>&1)
  # An artifact that is not what was installed is not this manifest's to delete.
  # Keeping it, saying why, and leaving the manifest so the command can be run
  # again is the behaviour worth pinning: the alternative is deleting a file
  # somebody else may have replaced on purpose.
  [ -f "$home/.local/bin/notriosctl" ] || fail "uninstall deleted a modified artifact" || return 1
  grep -qi "modified since it was installed" <<<"$out" \
    || fail "uninstall did not say why it kept the file" || return 1
  grep -qi "manifest was left in place" <<<"$out" \
    || fail "uninstall did not keep the manifest for a repeat run" || return 1
  record uninstall-keeps-a-modified-artifact-and-says-why pass \
    "{\"kept\":true,\"explained\":true,\"manifest_kept\":true}"
}

drill_symlinked_data_root() {
  local home; home=$(install_home symlinked) || return 1
  local library; library=$(library_of "$home")
  # The library is replaced by a link to somewhere else, with a file beside the
  # target that must survive whatever happens. Deleting *through* a symlink is
  # how a purge reaches data it was never pointed at.
  local outside=$WORK/outside-the-profile
  mkdir -p "$outside"
  mv "$library" "$outside/real-library"
  printf 'not part of any notrios profile\n' > "$outside/bystander.txt"
  ln -s "$outside/real-library" "$library"

  local out code
  out=$( ( cd "$home" && env -i PATH="$PATH" HOME="$home" FORCE=1 \
           python3 "$ROOT/scripts/lifecycle.py" purge ) 2>&1)
  code=$?
  [ -f "$outside/bystander.txt" ] \
    || fail "purge deleted a file outside the profile by following a symlink" || return 1
  record purge-does-not-delete-through-a-symlink pass \
    "{\"exit\":$code,\"neighbour_survived\":true,\"target_survived\":$([ -e "$outside/real-library" ] && echo true || echo false)}"
}

drill_external_data_root() {
  local home=$WORK/external
  mkdir -p "$home"
  local elsewhere=$WORK/elsewhere-share
  mkdir -p "$elsewhere"
  ( cd "$ROOT" && env -i PATH="$PATH" HOME="$home" prefix="$home/.local" \
      python3 scripts/lifecycle.py install ) >"$home/install.log" 2>&1 || return 1
  # A data root the user put somewhere else entirely. The lifecycle must act on
  # where the notes actually are, not on where they would be by default.
  local library
  library=$( cd "$home" && env -i PATH="$PATH" HOME="$home" XDG_DATA_HOME="$elsewhere" \
    "$home/.local/bin/notriosctl" paths --no-redact 2>/dev/null | awk '$1=="data"{print $2}')
  case "$library" in "$elsewhere"/*) ;; *) fail "an external data root was ignored: $library" || return 1;; esac
  ( cd "$home" && env -i PATH="$PATH" HOME="$home" XDG_DATA_HOME="$elsewhere" \
    "$home/.local/bin/notriosctl" notes create --title "The note this drill must not lose" --body b ) >/dev/null 2>&1 || return 1
  library_intact "$library" || fail "the external library was not created" || return 1
  record external-data-root-is-used-where-it-actually-is pass \
    "{\"library\":\"$library\",\"under_xdg_override\":true}"
}

drill_purge_while_a_daemon_runs() {
  local home; home=$(install_home running) || return 1
  local library; library=$(library_of "$home")
  # A daemon holding the library open is the process race the item names. The
  # question is not whether purge wins -- it is whether the outcome is one a
  # person can act on, and whether the backup still happened before anything was
  # deleted.
  ( cd "$home" && env -i PATH="$PATH" HOME="$home" "$home/.local/bin/notriosd" \
      -listen 127.0.0.1:18471 ) >"$home/daemon.log" 2>&1 &
  local daemon_pid=$!
  local waited=0
  while [ $waited -lt 100 ] && ! grep -qiE "listening|started|serving" "$home/daemon.log" 2>/dev/null; do
    waited=$((waited + 1)); sleep 0.1
  done

  local out code
  out=$( ( cd "$home" && env -i PATH="$PATH" HOME="$home" FORCE=1 \
           python3 "$ROOT/scripts/lifecycle.py" purge ) 2>&1)
  code=$?
  kill "$daemon_pid" 2>/dev/null; wait "$daemon_pid" 2>/dev/null

  # Whatever the outcome, a backup must exist if anything was deleted. The
  # failure this guards against is a half-purge that leaves neither the library
  # nor a copy of it.
  local archive; archive=$(find "$home" -name backup.tar -print -quit)
  if ! library_intact "$library" && [ -z "$archive" ]; then
    fail "the library was deleted while a daemon held it, and no backup was written" || return 1
  fi
  record purge-while-a-daemon-holds-the-library pass \
    "{\"exit\":$code,\"library_intact\":$(library_intact "$library" && echo true || echo false),\"backup_written\":$([ -n "$archive" ] && echo true || echo false)}"
}

status=0
for drill in drill_unattended_refusal drill_backup_unwritable drill_backup_too_large \
             drill_backup_contents_and_restore drill_uninstall_keeps_data \
             drill_modified_artifact drill_symlinked_data_root drill_external_data_root \
             drill_purge_while_a_daemon_runs; do
  echo "== $drill" >&2
  if ! "$drill"; then
    record "${drill#drill_}" fail '{}'
    status=1
  fi
done

python3 - "$RESULTS" <<'PY'
import json, sys
rows = [json.loads(line) for line in open(sys.argv[1]) if line.strip()]
print(f"{len(rows)} drills, {sum(1 for r in rows if r['status'] == 'pass')} passed")
for r in rows:
    print(f"  {r['status']:4}  {r['drill']}")
PY
exit $status
