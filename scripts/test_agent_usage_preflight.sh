#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

FAKE="$TMP/fake-checker.sh"
cat >"$FAKE" <<'EOF'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >"${FAKE_ARGS:?}"
printf '%s\n' "${FAKE_OUTPUT:-}"
exit "${FAKE_STATUS:-0}"
EOF
chmod +x "$FAKE"
preflight=(bash "$ROOT/scripts/agent_usage_preflight.sh")

run() {
  FAKE_ARGS="$TMP/args" FAKE_STATUS="$1" NOTRIOS_AGENT_USAGE_COMMAND="$FAKE" \
    NOTRIOS_AGENT_MODEL=test-model NOTRIOS_AGENT_EFFORT=high \
    NOTRIOS_AGENT_USAGE_MINIMUM=33 "${preflight[@]}" test-op "$TMP/history.json"
}

run 0
grep -F -- '--operation test-op --agent all --minimum-remaining 33 --model test-model --effort high --json' "$TMP/args" >/dev/null
grep -F -- "--history $TMP/history.json" "$TMP/args" >/dev/null
if run 2; then exit 1; fi
if NOTRIOS_AGENT_USAGE_GUARD=strict FAKE_ARGS="$TMP/args" FAKE_STATUS=7 \
  NOTRIOS_AGENT_USAGE_COMMAND="$FAKE" "${preflight[@]}" strict-op; then exit 1; fi
FAKE_ARGS="$TMP/args" FAKE_STATUS=7 FAKE_OUTPUT='[{"state": "unknown"}]' NOTRIOS_AGENT_USAGE_COMMAND="$FAKE" \
  "${preflight[@]}" advisory-op
FAKE_ARGS="$TMP/args" FAKE_STATUS=0 FAKE_OUTPUT='[{"state": "unknown"}]' NOTRIOS_AGENT_USAGE_COMMAND="$FAKE" \
  "${preflight[@]}" unknown-op 2>"$TMP/warning"
grep -F 'telemetry unavailable' "$TMP/warning" >/dev/null
FAKE_ARGS="$TMP/args" FAKE_STATUS=0 NOTRIOS_AGENT_USAGE_MINIMUM=100.5 \
  NOTRIOS_AGENT_USAGE_COMMAND="$FAKE" "${preflight[@]}" invalid-advisory 2>"$TMP/invalid-warning"
grep -F 'using 20' "$TMP/invalid-warning" >/dev/null
grep -F -- '--minimum-remaining 20' "$TMP/args" >/dev/null
rm -f "$TMP/strict-args"
if FAKE_ARGS="$TMP/strict-args" FAKE_STATUS=0 NOTRIOS_AGENT_USAGE_MINIMUM=100.5 \
  NOTRIOS_AGENT_USAGE_GUARD=strict NOTRIOS_AGENT_USAGE_COMMAND="$FAKE" \
  "${preflight[@]}" invalid-strict; then exit 1; fi
[[ ! -e "$TMP/strict-args" ]]
NOTRIOS_AGENT_USAGE_GUARD=off NOTRIOS_AGENT_USAGE_COMMAND="$TMP/does-not-exist" \
  "${preflight[@]}" off-op
echo "agent usage preflight tests passed"
