#!/usr/bin/env bash
# Check the local agent's remaining usage before a long, non-checkpointed run.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1  # no .pyc litter; progress arrives as it happens

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <operation> [history-path]" >&2
  exit 2
fi

guard=${NOTRIOS_AGENT_USAGE_GUARD:-advisory}
if [[ $guard == off ]]; then
  exit 0
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
operation=$1
history=${2:-${NOTRIOS_AGENT_USAGE_HISTORY:-}}
minimum=${NOTRIOS_AGENT_USAGE_MINIMUM:-20}
model=${NOTRIOS_AGENT_MODEL:-unknown}
effort=${NOTRIOS_AGENT_EFFORT:-unknown}
if [[ ! $minimum =~ ^[0-9]+([.][0-9]+)?$ ]] || ! awk -v value="$minimum" 'BEGIN { exit !(value >= 0 && value <= 100) }'; then
  if [[ $guard == strict ]]; then
    echo "agent usage preflight rejected invalid minimum: $minimum" >&2
    exit 3
  fi
  echo "warning: invalid usage minimum '$minimum'; using 20" >&2
  minimum=20
fi
strict_args=()
[[ $guard == strict ]] && strict_args+=(--strict)

# Tests and local wrappers can replace the checker with a command and arguments.
if [[ -n ${NOTRIOS_AGENT_USAGE_COMMAND:-} ]]; then
  read -r -a checker <<< "$NOTRIOS_AGENT_USAGE_COMMAND"
else
  checker=(python3 "$ROOT/scripts/check_agent_usage.py")
fi
args=(--operation "$operation" --agent all --minimum-remaining "$minimum" --model "$model" --effort "$effort" --json)
[[ -n $history ]] && args+=(--history "$history")

set +e
output=$("${checker[@]}" "${args[@]}" "${strict_args[@]}" 2>&1)
status=$?
set -e
printf '%s\n' "$output"

case $status in
  0)
    if [[ $output == *'"state": "unknown"'* || $output == *'"state":"unknown"'* ]]; then
      echo "warning: agent usage telemetry unavailable; continuing operation: $operation" >&2
    fi
    if [[ $output == *'"state": "stale"'* || $output == *'"state":"stale"'* ]]; then
      echo "warning: agent usage telemetry is stale and was not trusted; continuing operation: $operation" >&2
    fi
    exit 0
    ;;
  2)
    echo "agent usage preflight paused operation: $operation" >&2
    exit 2
    ;;
  *)
    if [[ $guard == strict ]]; then
      echo "agent usage preflight failed in strict mode (status $status): $operation" >&2
      exit "$status"
    fi
    echo "warning: agent usage telemetry unavailable; continuing operation: $operation (status $status)" >&2
    exit 0
    ;;
esac
