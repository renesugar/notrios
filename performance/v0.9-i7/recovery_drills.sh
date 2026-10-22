#!/usr/bin/env bash
# I7-B — what a diagnostic may disclose, what leaves the machine, and whether a
# library actually comes back.
#
#   bash performance/v0.9-i7/recovery_drills.sh
#
# Each drill uses its own installed profile in an isolated HOME outside the
# checkout, for the reason I4 learned the hard way: inside a checkout the CLI
# resolves source mode and the drill measures the developer's own library.
set -uo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/performance/v0.9-i7
RESULTS=${I7_RESULTS:-$HERE/DRILLS.jsonl}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/notrios-i7-XXXXXX")
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT
: > "$RESULTS"

in_home() { local home=$1; shift; ( cd "$home" && env -i PATH="$PATH" HOME="$home" "$@" ); }

# Count hits; never grep the output for the query.
#
# `notriosctl search` echoes the query back -- {"hits": [], "query": "x"} -- so
# grepping the output for the search term matches an *empty* result and reports
# a note that is not there. That is not hypothetical: it is how the first
# version of the recovery drill below concluded that a library it had just
# deleted still held its notes, and how I4's restore assertion passed without
# testing anything.
hits_for() { local home=$1 query=$2
  in_home "$home" "$home/.local/bin/notriosctl" search "$query" 2>/dev/null \
    | python3 -c 'import json,sys; print(len(json.load(sys.stdin).get("hits", [])))' 2>/dev/null \
    || echo 0
}

install_home() {
  local home=$WORK/$1; mkdir -p "$home"
  ( cd "$ROOT" && env -i PATH="$PATH" HOME="$home" prefix="$home/.local" \
      python3 scripts/lifecycle.py install ) >"$home/install.log" 2>&1 || return 1
  local mode
  mode=$(in_home "$home" "$home/.local/bin/notriosctl" paths 2>/dev/null | awk -F': ' '/^mode:/{print $2}')
  [ "$mode" = installed ] || { echo "resolved mode is '$mode', not installed" >&2; return 1; }
  printf '%s' "$home"
}

record() { python3 - "$1" "$2" "$3" >> "$RESULTS" <<'PY'
import json, sys
print(json.dumps({"drill": sys.argv[1], "status": sys.argv[2],
                  "observations": json.loads(sys.argv[3])}, sort_keys=True))
PY
}
fail() { echo "   $1" >&2; return 1; }

drill_diagnostics_redact() {
  local home; home=$(install_home redaction) || return 1
  cat > "$home/notrios.yaml" <<'CFG'
sync:
  target: rest
  credential_ref: DRILLMARKER-credential-reference
  rest:
    enabled: true
    url: https://sync.example.invalid/DRILLMARKER
    credential_store: development-file
CFG
  local leaked=0 redacted=0
  for form in "config show --config $home/notrios.yaml" "paths" "doctor --json" "doctor"; do
    local out
    out=$(in_home "$home" "$home/.local/bin/notriosctl" $form 2>&1)
    grep -q -- "$home" <<<"$out" && { echo "   $form disclosed the home directory" >&2; leaked=$((leaked+1)); }
    grep -q '~/' <<<"$out" && redacted=$((redacted+1))
  done
  [ "$leaked" -eq 0 ] || fail "a diagnostic disclosed the home directory" || return 1
  [ "$redacted" -ge 3 ] || fail "diagnostics printed no redacted paths, so this proves nothing" || return 1

  # --no-redact must still work, or somebody debugging a path cannot see it.
  in_home "$home" "$home/.local/bin/notriosctl" doctor --no-redact 2>&1 | grep -q -- "$home" \
    || fail "--no-redact did not print the real paths" || return 1

  # A credential *reference* names where a credential lives; it is not the
  # credential. It is printed, and that is recorded rather than assumed.
  local ref_shown=false
  in_home "$home" "$home/.local/bin/notriosctl" config show --config "$home/notrios.yaml" 2>&1 \
    | grep -q DRILLMARKER-credential-reference && ref_shown=true
  record diagnostics-redact-the-home-directory pass \
    "{\"surfaces_checked\":4,\"leaked\":0,\"no_redact_still_works\":true,\"credential_reference_printed\":$ref_shown}"
}

drill_nothing_leaves_the_machine() {
  # Crash reporting and telemetry: there is none, and the point is to keep it
  # that way. A dependency is how one arrives.
  local sdks
  sdks=$(grep -icE "sentry|bugsnag|rollbar|datadog|newrelic|posthog|mixpanel|amplitude" "$ROOT/go.mod" || true)
  [ "$sdks" -eq 0 ] || fail "a telemetry or crash-reporting SDK is in go.mod" || return 1
  local webdeps
  webdeps=$(grep -icE "\"(@sentry|bugsnag|posthog|mixpanel|@amplitude)" "$ROOT/web/package.json" || true)
  [ "$webdeps" -eq 0 ] || fail "a telemetry SDK is in the frontend dependencies" || return 1

  # The service listens on loopback and is not started until enabled; a fresh
  # profile must not have a sync target configured.
  local home; home=$(install_home telemetry) || return 1
  local target
  target=$(in_home "$home" "$home/.local/bin/notriosctl" config show 2>/dev/null | awk '$1=="sync.target"{print $2}')
  [ "$target" = none ] || fail "a fresh profile has sync.target=$target, not none" || return 1
  record nothing-reports-anywhere-by-default pass \
    "{\"telemetry_sdks\":0,\"frontend_telemetry\":0,\"fresh_sync_target\":\"none\"}"
}

drill_disaster_recovery() {
  local home; home=$(install_home recovery) || return 1
  in_home "$home" "$home/.local/bin/notriosctl" notes create \
    --title "The only copy of this note" --body "recovered?" >/dev/null 2>&1 || return 1

  in_home "$home" "$home/.local/bin/notriosctl" export archive "$home/rescue" >/dev/null 2>&1 \
    || fail "the export failed" || return 1
  [ -s "$home/rescue/manifest.json" ] || fail "the export produced no manifest" || return 1

  local before; before=$(hits_for "$home" "only copy")
  [ "${before:-0}" -ge 1 ] || fail "the note was never findable, so this drill proves nothing" || return 1

  # Lose the library. Not a tidy uninstall: the file is gone.
  local library
  library=$(in_home "$home" "$home/.local/bin/notriosctl" paths --no-redact 2>/dev/null | awk '$1=="data"{print $2}')
  [ -n "$library" ] || fail "could not resolve the library path" || return 1
  rm -rf "$library"
  local after_loss; after_loss=$(hits_for "$home" "only copy")
  # Fatal, not a warning. A drill that cannot destroy the thing it is about to
  # recover is measuring nothing, and the first version of this let that pass.
  [ "${after_loss:-1}" -eq 0 ] || fail "the library survived being deleted, so this drill proves nothing" || return 1

  in_home "$home" "$home/.local/bin/notriosctl" import archive "$home/rescue" >/dev/null 2>&1 \
    || fail "the import failed" || return 1
  local recovered; recovered=$(hits_for "$home" "only copy")
  [ "${recovered:-0}" -ge 1 ] || fail "the note did not come back" || return 1
  record library-lost-and-recovered-from-an-export pass \
    "{\"hits_before\":$before,\"hits_after_loss\":$after_loss,\"hits_after_recovery\":$recovered,\"library\":\"$library\"}"
}

status=0
for drill in drill_diagnostics_redact drill_nothing_leaves_the_machine drill_disaster_recovery; do
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
