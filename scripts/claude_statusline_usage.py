#!/usr/bin/env python3
"""Claude Code status-line command that caches rate-limit telemetry locally.

Claude Code never persists rate-limit percentages to disk. It passes them to the
configured `statusLine` command on stdin as a `rate_limits` object (client 2.1.80
and later: 5-hour and 7-day windows carrying `used_percentage` and `resets_at`).
Without a status line that records it, `scripts/check_agent_usage.py` has nothing
local to read and correctly reports `state=unknown` rather than assuming quota.

This script records that payload and prints a one-line status. Install it by
adding to `~/.claude/settings.json`:

    {
      "statusLine": {
        "type": "command",
        "command": "python3 /home/renes/projects/notrios/scripts/claude_statusline_usage.py",
        "refreshInterval": 30000
      }
    }

`refreshInterval` keeps the cache fresh while a session sits idle; without it the
status line only reruns when the transcript renders.

The cache path defaults to `~/.claude/usage-cache.json` and is overridden by
`NOTRIOS_CLAUDE_USAGE_CACHE`. Nothing here makes a model request, and no prompt,
transcript, or credential content is written.
"""

from __future__ import annotations

from datetime import datetime, timezone
import json
import os
import sys
import tempfile
from typing import Any


DEFAULT_CACHE = "~/.claude/usage-cache.json"


def cache_path() -> str:
    return os.path.expanduser(
        os.environ.get("NOTRIOS_CLAUDE_USAGE_CACHE") or DEFAULT_CACHE
    )


def _now_utc() -> str:
    return (
        datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")
    )


def record(payload: dict[str, Any]) -> dict[str, Any]:
    """Return the cache document for one status-line invocation."""
    model = payload.get("model")
    workspace = payload.get("workspace")
    return {
        "captured_at": _now_utc(),
        "rate_limits": payload.get("rate_limits"),
        "client_version": payload.get("version"),
        "session_id": payload.get("session_id"),
        "model_id": model.get("id") if isinstance(model, dict) else None,
        "current_dir": (
            workspace.get("current_dir") if isinstance(workspace, dict) else None
        ),
        "writer": "scripts/claude_statusline_usage.py",
    }


def write_cache(document: dict[str, Any], path: str | None = None) -> str:
    """Atomically replace the cache so a concurrent reader never sees a partial file."""
    target = path or cache_path()
    directory = os.path.dirname(os.path.abspath(target)) or "."
    os.makedirs(directory, exist_ok=True)
    handle, temporary = tempfile.mkstemp(dir=directory, prefix=".usage-cache-")
    try:
        with os.fdopen(handle, "w", encoding="utf-8") as stream:
            json.dump(document, stream, sort_keys=True)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(temporary, 0o600)
        os.replace(temporary, target)
    except BaseException:
        try:
            os.unlink(temporary)
        except OSError:
            pass
        raise
    return target


def _percent(window: Any) -> float | None:
    if isinstance(window, (int, float)):
        return float(window)
    if not isinstance(window, dict):
        return None
    for key in ("used_percentage", "usedPercent", "used_percent", "percent"):
        value = window.get(key)
        if isinstance(value, (int, float)):
            return float(value)
    for key in ("remaining_percentage", "remainingPercent", "remaining_percent"):
        value = window.get(key)
        if isinstance(value, (int, float)):
            return 100.0 - float(value)
    return None


def status_line(payload: dict[str, Any]) -> str:
    """Render the visible status line; usage percentages are shown as used."""
    parts: list[str] = []
    model = payload.get("model")
    if isinstance(model, dict) and model.get("display_name"):
        parts.append(str(model["display_name"]))
    workspace = payload.get("workspace")
    if isinstance(workspace, dict) and workspace.get("current_dir"):
        parts.append(os.path.basename(str(workspace["current_dir"])))
    limits = payload.get("rate_limits")
    windows = []
    if isinstance(limits, dict):
        for name, window in limits.items():
            used = _percent(window)
            if used is not None:
                windows.append(f"{name} {used:.0f}%")
    if windows:
        parts.append(" ".join(windows))
    else:
        parts.append("usage n/a")
    return " | ".join(parts) if parts else "notrios usage cache"


def main(argv: list[str] | None = None) -> int:
    argv = sys.argv[1:] if argv is None else argv
    if argv and argv[0] in ("-h", "--help"):
        print(__doc__)
        return 0
    try:
        raw = sys.stdin.read()
    except (OSError, ValueError):
        raw = ""
    payload: Any = None
    if raw.strip():
        try:
            payload = json.loads(raw)
        except ValueError:
            payload = None
    if not isinstance(payload, dict):
        # A status line must never fail loudly; it would replace the user's footer
        # with a traceback and still leave the cache untouched.
        print("notrios usage cache: no status-line JSON on stdin")
        return 0
    try:
        write_cache(record(payload))
    except OSError as error:
        print(f"{status_line(payload)} | cache error: {error}")
        return 0
    print(status_line(payload))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
