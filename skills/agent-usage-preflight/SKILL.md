---
name: agent-usage-preflight
description: Check local Codex or Claude Code usage before starting a long agent-managed phase, and leave a durable handoff when the reported reserve is too small.
---

# Agent usage preflight

Use the repository probe before a long validation phase, model-backed subtask,
or repeated external harness phase when a usage-limit interruption would leave
cleanup or interpretation unfinished:

```bash
python3 scripts/check_agent_usage.py --agent all --minimum-remaining 20
```

At session start, run `python3 scripts/test_check_agent_usage.py` and compare
the installed client version/output with the parser fixtures. If a client
changed and the probe reports `unknown`, update and retest the parser before
using strict mode. Never replace an unknown reading with 100%.

When the result is `pause`, finish only the current durable checkpoint, update
`CODING_CLIENT_HANDOFF.md` and the agent logs with the exact resume command,
then stop before launching the next phase. Do not terminate already-running
work merely because a later probe is low.

Codex checks use the model-free app-server `account/rateLimits/read` method.
Do not use `codex exec /status`: it is a model prompt, consumes usage, and does
not provide the assumed JSON contract. Claude checks are local-cache-only; do
not spend usage with `claude -p` to estimate remaining usage.

Use `--strict` only where unavailable telemetry must block. Historical reserve
samples are useful only when agent, model, effort, operation, run ID, and reset
window match; do not transfer an estimate to another combination.
