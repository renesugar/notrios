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

## Claude requires a status-line writer

Claude Code holds rate limits in memory and never writes them to disk. It hands
them to the configured `statusLine` command on stdin as `rate_limits` (client
2.1.80 and later: 5-hour and 7-day windows with `used_percentage` and
`resets_at`). Without a status line that records them the probe has nothing to
read and reports `unknown`, which is correct but useless as a gate.

Install `scripts/claude_statusline_usage.py` once, in `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "python3 /home/renes/projects/notrios/scripts/claude_statusline_usage.py",
    "refreshInterval": 30000
  }
}
```

It writes `~/.claude/usage-cache.json` (override with
`NOTRIOS_CLAUDE_USAGE_CACHE` or `--claude-usage-cache`) and prints a one-line
status. It makes no model request and records no prompt or transcript content.
The setting takes effect in sessions started after the edit.

The Claude probe reports `stale` rather than a number when the cache is older
than `--claude-max-age-minutes` (default 30) or when every reported window is
past its `resets_at`. `stale` never authorizes a long run: it is treated like
`unknown`, so `--strict` exits 3. This matters because the client is known to
report a window's pre-reset percentage while a session sits idle.

Use `--strict` only where unavailable telemetry must block. Historical reserve
samples are useful only when agent, model, effort, operation, run ID, and reset
window match; do not transfer an estimate to another combination.
