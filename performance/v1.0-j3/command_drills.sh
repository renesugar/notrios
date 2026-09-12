#!/usr/bin/env bash
# J3 — the drills v0.9 I4 ran against `make purge`, run against `notriosctl purge`.
#
#   bash performance/v1.0-j3/command_drills.sh
#
# The point is not that the command works. It is that every safeguard a
# developer gets, a packaged user now gets too -- so these are I4's drills with
# the Make target swapped for the command, and a drill that passes there and
# fails here is the gap this item exists to close.
#
# Each drill installs into its own HOME outside the checkout, for the reason I4
# learned the hard way: inside a checkout the CLI resolves source mode and the
# drill measures the developer's own library.
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/performance/v1.0-j3
RESULTS=${J3_RESULTS:-$HERE/DRILLS.jsonl}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/notrios-j3-XXXXXX")
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT
: > "$RESULTS"

in_home() { local home=$1; shift; ( cd "$home" && env -i PATH="$PATH" HOME="$home" "$@" ); }

install_home() {
  local home=$WORK/$1; mkdir -p "$home"
  ( cd "$ROOT" && env -i PATH="$PATH" HOME="$home" prefix="$home/.local" \
      python3 scripts/lifecycle.py install ) >"$home/install.log" 2>&1 || return 1
  # The command under test is the one just built, not whatever install staged.
  cp "$ROOT/bin/notriosctl" "$home/.local/bin/notriosctl"
  local mode
  mode=$(in_home "$home" "$home/.local/bin/notriosctl" paths 2>/dev/null | awk -F': ' '/^mode:/{print $2}')
  [ "$mode" = installed ] || { echo "resolved mode is '$mode', not installed" >&2; return 1; }
  in_home "$home" "$home/.local/bin/notriosctl" notes create \
    --title "the note this drill must not lose" --body body >/dev/null 2>&1 || return 1
  printf '%s' "$home"
}

library_of() { in_home "$1" "$1/.local/bin/notriosctl" paths --no-redact 2>/dev/null | awk '$1=="data"{print $2}'; }
library_intact() { [ -f "$1/notes.sqlite" ]; }
record() { python3 - "$1" "$2" "$3" >> "$RESULTS" <<'PY'
import json, sys
print(json.dumps({"drill": sys.argv[1], "status": sys.argv[2],
                  "observations": json.loads(sys.argv[3])}, sort_keys=True))
PY
}
fail() { echo "   $1" >&2; return 1; }

# I4: an unattended purge refuses without the flag that says you meant it.
drill_unattended_refusal() {
  local home; home=$(install_home unattended) || return 1
  local library; library=$(library_of "$home")
  local out code
  out=$(in_home "$home" "$home/.local/bin/notriosctl" purge </dev/null 2>&1); code=$?
  library_intact "$library" || fail "the library is gone after a refused purge" || return 1
  [ "$code" -ne 0 ] || fail "an unattended purge did not refuse" || return 1
  grep -qi -- "--confirm" <<<"$out" || fail "the refusal did not say how to automate it deliberately" || return 1
  record unattended-purge-refuses-without-confirm pass \
    "{\"exit\":$code,\"library_intact\":true,\"names_the_flag\":true}"
}

# I4: the backup cannot be written, so nothing is deleted.
drill_backup_unwritable() {
  local home; home=$(install_home unwritable) || return 1
  local library; library=$(library_of "$home")
  local state="$home/.local/state"; mkdir -p "$state"; chmod a-w "$state"
  local code
  in_home "$home" "$home/.local/bin/notriosctl" purge --confirm >/dev/null 2>&1; code=$?
  chmod u+w "$state"
  library_intact "$library" || fail "the library was deleted although the backup could not be written" || return 1
  [ "$code" -ne 0 ] || fail "purge succeeded with nowhere to put the backup" || return 1
  record purge-refuses-when-the-backup-cannot-be-written pass "{\"exit\":$code,\"library_intact\":true}"
}

# I4: the backup does not fit, so nothing is deleted.
drill_backup_too_large() {
  local home; home=$(install_home toolarge) || return 1
  local library; library=$(library_of "$home")
  local code
  ( ulimit -f 8; in_home "$home" "$home/.local/bin/notriosctl" purge --confirm ) >/dev/null 2>&1; code=$?
  library_intact "$library" || fail "the library was deleted although the backup did not fit" || return 1
  [ "$code" -ne 0 ] || fail "purge succeeded although the backup could not be written in full" || return 1
  record purge-refuses-when-the-backup-does-not-fit pass "{\"exit\":$code,\"library_intact\":true}"
}

# I4: the backup holds the library, excludes sync keys, and restores.
drill_backup_contents_and_restore() {
  local home; home=$(install_home restore) || return 1
  local state; state=$(in_home "$home" "$home/.local/bin/notriosctl" paths --no-redact 2>/dev/null | awk '$1=="state"{print $2}')
  mkdir -p "$state"; printf '{"k":1}' > "$state/sync-keys.json"
  local out; out=$(in_home "$home" "$home/.local/bin/notriosctl" purge --confirm 2>&1) || {
    echo "$out" | tail -3 >&2; fail "a purge with a writable destination failed" || return 1; }
  local archive; archive=$(find "$home" -name backup.tar -print -quit)
  [ -n "$archive" ] || fail "the purge left no backup" || return 1
  local listing=$WORK/listing.txt; tar -tf "$archive" > "$listing"
  grep -qiE "sync-keys" "$listing" && fail "the purge backup contains sync key material" || true
  grep -qiE "sync-keys" "$listing" && return 1
  grep -q "notes.sqlite" "$listing" || fail "the backup does not contain the library" || return 1

  # A backup nobody has restored is a hope.
  local restored=$WORK/restored; mkdir -p "$restored"; tar -xf "$archive" -C "$restored"
  local db; db=$(find "$restored" -name notes.sqlite -print -quit)
  [ -n "$db" ] || fail "no library in the restored tree" || return 1
  local hits
  hits=$(in_home "$home" "$ROOT/bin/notriosctl" search --db "$db" --asset-store "$restored/data/assets" \
         "must not lose" 2>/dev/null | python3 -c 'import json,sys; print(len(json.load(sys.stdin).get("hits",[])))' 2>/dev/null || echo 0)
  [ "${hits:-0}" -ge 1 ] || fail "the restored library does not hold the note" || return 1
  record purge-backup-restores-and-excludes-sync-keys pass \
    "{\"archive_holds_library\":true,\"archive_holds_keys\":false,\"restored_hits\":$hits}"
}

# I4: a symlinked root is not followed out of the profile.
drill_symlinked_data_root() {
  local home; home=$(install_home symlinked) || return 1
  local library; library=$(library_of "$home")
  local outside=$WORK/outside-the-profile; mkdir -p "$outside"
  mv "$library" "$outside/real-library"
  printf 'not part of any notrios profile\n' > "$outside/bystander.txt"
  ln -s "$outside/real-library" "$library"
  local code
  in_home "$home" "$home/.local/bin/notriosctl" purge --confirm >/dev/null 2>&1; code=$?
  [ -f "$outside/bystander.txt" ] || fail "purge deleted a file outside the profile by following a symlink" || return 1
  record purge-does-not-delete-through-a-symlink pass \
    "{\"exit\":$code,\"neighbour_survived\":true,\"target_survived\":$([ -e "$outside/real-library" ] && echo true || echo false)}"
}

# J3's own: a backup destination inside what the run would delete is refused.
drill_backup_destination_guard() {
  local home; home=$(install_home guard) || return 1
  local library; library=$(library_of "$home")
  local out code
  out=$(in_home "$home" "$home/.local/bin/notriosctl" purge --confirm --backup-dir "$library/backup" 2>&1); code=$?
  library_intact "$library" || fail "the library was deleted despite a self-destroying backup path" || return 1
  [ "$code" -ne 0 ] || fail "a backup destination inside the purge target was accepted" || return 1
  grep -qi "delete its own backup" <<<"$out" || fail "the refusal did not say why" || return 1
  record purge-refuses-a-backup-destination-it-would-delete pass "{\"exit\":$code,\"library_intact\":true}"
}

status=0
# J3-D: a profile whose library lives outside the roots is listed and survives.
#
# I4's external-data-root drill, which J3 first recorded as not carried over.
# The safeguard being drilled is not the deletion -- it is the *listing*: the
# oracle always refused to delete such a path, but nothing told the oracle one
# existed, so the user approved a plan their external library was missing from.
drill_external_profile() {
  local home; home=$(install_home external) || return 1
  local library; library=$(library_of "$home")
  local outside="$home/elsewhere"
  mkdir -p "$outside"

  # A real library, created by the command. Writing bytes into a file named
  # .sqlite is not enough -- `profile register` opens the database to read the
  # database id out of it, which is how this drill first failed -- and it is
  # also the more honest fixture: what must survive is a library with a note in
  # it, not an empty file.
  in_home "$home" "$home/.local/bin/notriosctl" notes create \
    --db "$outside/recipes.sqlite" --title "the recipe this drill must not lose" \
    --body body >/dev/null 2>&1 || fail "the external library could not be created" || return 1

  # Registered through the command rather than by writing the registry by hand,
  # so the drill exercises the format the product actually writes.
  in_home "$home" "$home/.local/bin/notriosctl" profile register \
    --name recipes --db "$outside/recipes.sqlite" >/dev/null 2>&1 || \
    fail "the external profile could not be registered" || return 1

  local plan
  plan=$(in_home "$home" "$home/.local/bin/notriosctl" purge --dry-run --no-redact 2>&1)
  grep -q "NOT DELETED" <<<"$plan" || fail "the plan does not list the external path" || return 1
  grep -q "$outside/recipes.sqlite" <<<"$plan" || fail "the plan does not name the external library" || return 1
  grep -q "recipes" <<<"$plan" || fail "the plan does not say which profile names it" || return 1

  in_home "$home" "$home/.local/bin/notriosctl" purge --confirm >/dev/null 2>&1 || \
    fail "the purge itself failed" || return 1

  [ -f "$outside/recipes.sqlite" ] || fail "the external library was deleted" || return 1
  [ ! -d "$library" ] || fail "the data root survived a confirmed purge" || return 1

  # Still readable, not merely still present: a file that survived as bytes but
  # cannot be opened is not a library anybody kept. Counted by hits rather than
  # grepped for the query, which is I7's lesson and nearly cost I4 a drill.
  local hits
  hits=$(in_home "$home" "$home/.local/bin/notriosctl" search --db "$outside/recipes.sqlite" \
           --count recipe 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin).get("count",0))' 2>/dev/null)
  [ "${hits:-0}" -ge 1 ] || fail "the external library survived but holds no notes (hits=${hits:-0})" || return 1

  # Not copied either, which is a decision rather than an oversight: purge does
  # not delete this path, so a copy adds no recovery and a large external
  # library would make the backup -- and therefore the whole purge -- fail.
  local copied=false
  if tar -tf "$home/.local/state/notrios-purge-backups"/*/backup.tar 2>/dev/null \
       | grep -q recipes.sqlite; then copied=true; fi
  [ "$copied" = false ] || fail "the external library was copied into the backup" || return 1

  record purge-lists-an-external-profile-and-keeps-it pass \
    "{\"listed\":true,\"attributed\":true,\"survived\":true,\"notes_readable\":$hits,\"data_root_deleted\":true,\"copied_into_backup\":false}"
}

# J3-E: a profile that names a parent of the roots must not block the purge.
#
# The bug this drill exists for shipped for an hour. Handing the enumerated
# paths to the oracle meant `profile register --asset-store ~/.local/share` --
# a parent of the data root, and an easy thing to type -- made purge REFUSE the
# data root. The notes survived a confirmed purge and the message said they had
# been "enumerated and backed up" when nothing had been copied anywhere. A
# purge that silently keeps the library is the worst outcome this item has.
drill_external_ancestor_does_not_block() {
  local home; home=$(install_home ancestor) || return 1
  local library; library=$(library_of "$home")

  in_home "$home" "$home/.local/bin/notriosctl" profile register --name wide \
    --db "$library/notes.sqlite" --asset-store "$(dirname "$library")" >/dev/null 2>&1 || \
    fail "the ancestor profile could not be registered" || return 1

  local plan
  plan=$(in_home "$home" "$home/.local/bin/notriosctl" purge --dry-run --no-redact 2>&1)
  grep -q "REFUSED" <<<"$plan" && fail "a profile naming a parent directory refused a root" && return 1
  grep -q "still deleted" <<<"$plan" || \
    fail "the report does not say the roots inside the named parent are still deleted" || return 1

  in_home "$home" "$home/.local/bin/notriosctl" purge --confirm >/dev/null 2>&1 || \
    fail "the purge itself failed" || return 1
  library_intact "$library" && fail "the library survived a confirmed purge" && return 1
  [ -d "$(dirname "$library")" ] || fail "the named parent directory was deleted" || return 1

  record purge-is-not-blocked-by-a-profile-naming-a-parent pass \
    "{\"refused_any_root\":false,\"library_deleted\":true,\"named_parent_kept\":true,\"says_roots_still_deleted\":true}"
}

for drill in drill_unattended_refusal drill_backup_unwritable drill_backup_too_large \
             drill_backup_contents_and_restore drill_symlinked_data_root \
             drill_backup_destination_guard drill_external_profile \
             drill_external_ancestor_does_not_block; do
  echo "== $drill" >&2
  "$drill" || { record "${drill#drill_}" fail '{}'; status=1; }
done
python3 - "$RESULTS" <<'PY'
import json, sys
rows = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
print(f"{len(rows)} drills, {sum(1 for r in rows if r['status']=='pass')} passed")
for r in rows: print(f"  {r['status']:4}  {r['drill']}")
PY
exit $status
