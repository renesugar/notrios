#!/usr/bin/env python3
"""Inspect local coding-agent usage without making a model request."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import os
import re
import selectors
import signal
import subprocess
import sys
import time
from typing import Any, Callable, Sequence


WINDOW_NAMES = {"primary", "secondary", "weekly", "fiveHour", "five_hour"}
HISTORY_MATCH_FIELDS = ("agent", "model", "effort", "operation")

# Claude Code keeps rate limits in memory and hands them to the configured
# statusLine command; it writes no usage file of its own. scripts/
# claude_statusline_usage.py records that payload here so this probe stays
# local-cache-only and never spends quota to measure quota.
CLAUDE_USAGE_CACHE = "~/.claude/usage-cache.json"
CLAUDE_MAX_AGE_MINUTES = 30.0

CLAUDE_WINDOW_MINUTES = {
    "5h": 300,
    "5hour": 300,
    "5_hour": 300,
    "five_hour": 300,
    "fivehour": 300,
    "session": 300,
    "primary": 300,
    "7d": 10080,
    "7day": 10080,
    "7_day": 10080,
    "seven_day": 10080,
    "week": 10080,
    "weekly": 10080,
    "secondary": 10080,
    "30d": 43200,
    "month": 43200,
    "monthly": 43200,
    "opus_weekly": 10080,
}


def _percentage(value: Any) -> float | None:
    try:
        number = float(value)
    except (TypeError, ValueError):
        return None
    return number if 0 <= number <= 100 else None


def _epoch(value: Any) -> float | None:
    """Return POSIX seconds for an epoch number, epoch string, or ISO timestamp."""
    if value is None or isinstance(value, bool):
        return None
    number: float | None = None
    if isinstance(value, (int, float)):
        number = float(value)
    elif isinstance(value, str):
        text = value.strip()
        if not text:
            return None
        try:
            number = float(text)
        except ValueError:
            try:
                parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
            except ValueError:
                return None
            if parsed.tzinfo is None:
                parsed = parsed.replace(tzinfo=timezone.utc)
            return parsed.timestamp()
    if number is None:
        return None
    # Clients differ on units; anything past the year 5138 in seconds is
    # milliseconds, so treat it as such rather than emitting an absurd date.
    if abs(number) > 1e11:
        number /= 1000.0
    return number


def _reset_utc(value: Any) -> str | None:
    if value is None:
        return None
    if isinstance(value, (int, float)) or (
        isinstance(value, str) and value.strip().isdigit()
    ):
        seconds = _epoch(value)
        if seconds is None:
            return str(value)
        return (
            datetime.fromtimestamp(seconds, timezone.utc)
            .isoformat()
            .replace("+00:00", "Z")
        )
    return str(value)


def _bucket(
    name: str,
    raw: Any,
    *,
    version: str | None = None,
    limit_id: str | None = None,
) -> dict[str, Any] | None:
    if not isinstance(raw, dict):
        return None

    used = _percentage(
        raw.get("usedPercent", raw.get("used_percent", raw.get("used")))
    )
    remaining = _percentage(
        raw.get(
            "remainingPercent",
            raw.get("remaining_percent", raw.get("remaining")),
        )
    )
    if remaining is None and used is not None:
        remaining = 100 - used
    if used is None and remaining is not None:
        used = 100 - remaining
    if used is None or remaining is None:
        return None

    duration = raw.get("windowDurationMins", raw.get("window_duration_mins"))
    reset = _reset_utc(
        raw.get(
            "resetsAt",
            raw.get("resetAt", raw.get("reset_at", raw.get("reset"))),
        )
    )
    if raw.get("resetAfterSeconds") is not None:
        try:
            reset_after = float(raw["resetAfterSeconds"])
            duration = duration if duration is not None else reset_after / 60
            if reset is None:
                reset = _reset_utc(time.time() + reset_after)
        except (TypeError, ValueError):
            pass

    bucket: dict[str, Any] = {
        "name": name,
        "used_percent": used,
        "remaining_percent": remaining,
        "reset_utc": reset,
        "duration_minutes": duration,
        "exhausted": bool(
            raw.get("exhausted")
            or raw.get("rateLimitReachedType")
            or used >= 100
        ),
    }
    if version is not None:
        bucket["version"] = version
    if isinstance(limit_id, str) and len(limit_id) < 80:
        bucket["limit_id"] = limit_id
    return bucket


def parse_codex_response(
    message: Any, version: str | None = None
) -> list[dict[str, Any]]:
    """Normalize current multi-limit and legacy Codex rate-limit payloads."""
    if not isinstance(message, dict):
        return []
    data = message.get("result", message)
    if not isinstance(data, dict):
        return []
    limits = data.get("rateLimits", data.get("rate_limits", {}))
    if not isinstance(limits, dict):
        return []

    snapshots = (
        limits.get("rateLimitsByLimitId")
        or limits.get("rate_limits_by_limit_id")
        or data.get("rateLimitsByLimitId")
        or data.get("rate_limits_by_limit_id")
    )
    if not isinstance(snapshots, dict):
        snapshots = {str(limits.get("limitId") or "default"): limits}

    buckets: list[dict[str, Any]] = []
    seen: set[str] = set()
    outer_reached = limits.get(
        "rateLimitReachedType", data.get("rateLimitReachedType")
    )
    for limit_id, snapshot in snapshots.items():
        if not isinstance(snapshot, dict):
            continue
        windows = {
            name: value
            for name, value in snapshot.items()
            if name in WINDOW_NAMES and isinstance(value, dict)
        }
        if not windows and any(
            key in snapshot for key in ("usedPercent", "remainingPercent")
        ):
            windows = {"primary": snapshot}

        reached = snapshot.get("rateLimitReachedType") or outer_reached
        for name, raw in windows.items():
            identity = json.dumps([name, raw], sort_keys=True, default=str)
            if identity in seen:
                continue
            bucket = _bucket(
                name,
                raw,
                version=version,
                limit_id=str(limit_id),
            )
            if bucket is None:
                continue
            if reached:
                bucket["exhausted"] = True
                bucket["rate_limit_reached_type"] = reached
            buckets.append(bucket)
            seen.add(identity)

    if not buckets:
        legacy = _bucket("primary", limits, version=version)
        if legacy is not None:
            buckets.append(legacy)
    return buckets


def summarize(
    agent: str,
    buckets: list[dict[str, Any]],
    version: str | None = None,
    error: str | None = None,
) -> dict[str, Any]:
    result: dict[str, Any] = {
        "agent": agent,
        "state": "unknown",
        "buckets": buckets,
        "client_version": version,
    }
    if error:
        result["error"] = error
    if not buckets:
        return result

    binding = min(buckets, key=lambda item: item["remaining_percent"])
    result.update(
        binding_remaining_percent=binding["remaining_percent"],
        binding_reset_utc=binding.get("reset_utc"),
        binding_duration_minutes=binding.get("duration_minutes"),
        state=(
            "pause"
            if binding["exhausted"] or binding["remaining_percent"] <= 0
            else "active"
        ),
    )
    return result


def _client_version(command: Sequence[str]) -> str | None:
    try:
        completed = subprocess.run(
            [command[0], "--version"],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            timeout=2,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    lines = completed.stdout.strip().splitlines()
    return lines[0] if lines else None


def _stop_process(process: subprocess.Popen[str] | None) -> None:
    if process is None:
        return
    if process.stdin is not None:
        try:
            process.stdin.close()
        except (OSError, ValueError):
            pass
    try:
        if os.name == "posix" and getattr(process, "pid", None) is not None:
            os.killpg(process.pid, signal.SIGTERM)
        else:
            process.terminate()
        process.wait(timeout=1)
    except (OSError, subprocess.TimeoutExpired):
        try:
            if os.name == "posix" and getattr(process, "pid", None) is not None:
                os.killpg(process.pid, signal.SIGKILL)
            else:
                process.kill()
            process.wait(timeout=1)
        except (OSError, subprocess.TimeoutExpired):
            pass


def probe_codex(
    timeout: float = 10,
    command: Sequence[str] | None = None,
    version: str | None = None,
) -> dict[str, Any]:
    command = list(command or ("codex", "app-server", "--stdio"))
    payloads = (
        {
            "id": 1,
            "method": "initialize",
            "params": {
                "clientInfo": {"name": "notrios-usage-probe", "version": "0"}
            },
        },
        {"method": "initialized"},
        {"id": 2, "method": "account/rateLimits/read"},
    )
    process: subprocess.Popen[str] | None = None
    selector: selectors.BaseSelector | None = None
    resolved_version = version
    try:
        process = subprocess.Popen(
            command,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            start_new_session=(os.name == "posix"),
        )
        assert process.stdin is not None and process.stdout is not None
        process.stdin.write("\n".join(json.dumps(item) for item in payloads) + "\n")
        process.stdin.flush()

        selector = selectors.DefaultSelector()
        selector.register(process.stdout, selectors.EVENT_READ)
        response: dict[str, Any] | None = None
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if not selector.select(max(0, deadline - time.monotonic())):
                break
            line = process.stdout.readline()
            if not line:
                break
            try:
                message = json.loads(line)
            except json.JSONDecodeError:
                continue
            if isinstance(message, dict) and message.get("id") == 2:
                response = message
                break

        resolved_version = resolved_version or _client_version(command)
        if response is None:
            return summarize(
                "codex",
                [],
                resolved_version,
                "timeout or invalid response",
            )
        return summarize(
            "codex",
            parse_codex_response(response, resolved_version),
            resolved_version,
        )
    except (OSError, ValueError) as error:
        resolved_version = resolved_version or _client_version(command)
        return summarize("codex", [], resolved_version, str(error))
    finally:
        if selector is not None:
            selector.close()
        _stop_process(process)


def _claude_running() -> bool:
    try:
        process_ids = os.listdir("/proc")
    except OSError:
        process_ids = []
    for process_id in process_ids:
        if not process_id.isdigit():
            continue
        try:
            with open(f"/proc/{process_id}/cmdline", "rb") as stream:
                arguments = [
                    os.path.basename(item.decode(errors="ignore")).lower()
                    for item in stream.read().split(b"\0")
                    if item
                ]
        except OSError:
            continue
        if any(item in ("claude", "claude-code") for item in arguments[:2]):
            return True
    try:
        return (
            subprocess.run(
                ["pgrep", "-f", r"(^|/)claude([ .]|$)"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            ).returncode
            == 0
        )
    except OSError:
        return False


def _text_percentage(value: Any) -> tuple[float, bool] | None:
    """Return (percentage, is_used); bare numeric cache values mean used."""
    if isinstance(value, (int, float)):
        number = _percentage(value)
        return (number, True) if number is not None else None
    if isinstance(value, str):
        match = re.search(
            r"(\d+(?:\.\d+)?)\s*%?\s*(remaining|used)?",
            value,
            re.IGNORECASE,
        )
        if not match:
            return None
        number = _percentage(match.group(1))
        if number is None:
            return None
        return number, (match.group(2) or "used").lower() == "used"
    if isinstance(value, dict):
        for key in (
            "remaining_percentage",
            "remainingPercentage",
            "remainingPercent",
            "remaining_percent",
            "used_percentage",
            "usedPercentage",
            "usedPercent",
            "used_percent",
            "percent",
        ):
            if key not in value:
                continue
            parsed = _text_percentage(value[key])
            if parsed is not None:
                return parsed[0], key.lower().startswith("used")
    return None


def _claude_window_minutes(name: Any) -> float | None:
    key = re.sub(r"[^a-z0-9_]", "", str(name).lower())
    if key in CLAUDE_WINDOW_MINUTES:
        return CLAUDE_WINDOW_MINUTES[key]
    match = re.fullmatch(r"(\d+)(m|h|d|w)", key)
    if match:
        scale = {"m": 1, "h": 60, "d": 1440, "w": 10080}[match.group(2)]
        return float(match.group(1)) * scale
    return None


def _claude_bucket(
    name: Any,
    value: Any,
    *,
    version: str | None,
    now: float,
) -> dict[str, Any] | None:
    """Normalize one status-line rate-limit window.

    Entries that carry no percentage (`spend_limit` expressed in dollars, for
    example) return None so they never bind the reserve check.
    """
    parsed = _text_percentage(value)
    if parsed is None:
        return None
    percentage, is_used = parsed
    used = percentage if is_used else 100 - percentage
    reset_raw: Any = None
    duration: Any = None
    if isinstance(value, dict):
        for key in ("resets_at", "resetsAt", "reset_at", "resetAt", "reset"):
            if value.get(key) is not None:
                reset_raw = value[key]
                break
        for key in (
            "window_duration_mins",
            "windowDurationMins",
            "window_minutes",
            "duration_minutes",
        ):
            if value.get(key) is not None:
                duration = value[key]
                break
    if duration is None:
        duration = _claude_window_minutes(name)
    reset_epoch = _epoch(reset_raw)
    bucket: dict[str, Any] = {
        "name": str(name),
        "used_percent": used,
        "remaining_percent": 100 - used,
        "reset_utc": _reset_utc(reset_raw),
        "duration_minutes": duration,
        "exhausted": used >= 100,
        # A window past its reset has already refilled, and the client is known
        # to keep reporting the pre-reset percentage while a session sits idle
        # (fixed upstream, but the cache on disk can predate the fix). Such a
        # window is excluded from binding instead of being trusted.
        "expired": reset_epoch is not None and reset_epoch <= now,
    }
    if version is not None:
        bucket["version"] = version
    return bucket


def parse_claude_rate_limits(
    rate_limits: Any,
    version: str | None = None,
    now: float | None = None,
) -> list[dict[str, Any]]:
    """Normalize a Claude Code status-line `rate_limits` payload."""
    now = time.time() if now is None else now
    entries: list[tuple[Any, Any]] = []
    if isinstance(rate_limits, dict):
        entries = list(rate_limits.items())
    elif isinstance(rate_limits, list):
        for index, item in enumerate(rate_limits):
            if isinstance(item, dict):
                label = item.get("name") or item.get("window") or item.get("id")
                entries.append((label if label is not None else index, item))
    buckets: list[dict[str, Any]] = []
    for name, value in entries:
        bucket = _claude_bucket(name, value, version=version, now=now)
        if bucket is not None:
            buckets.append(bucket)
    return buckets


def _claude_cache_path(
    cache_path: str | None, settings_path: str | None
) -> str:
    """Resolve the usage cache: explicit, then settings sibling, then env, then default."""
    if cache_path:
        return os.path.expanduser(cache_path)
    if settings_path:
        directory = os.path.dirname(os.path.abspath(os.path.expanduser(settings_path)))
        return os.path.join(directory, "usage-cache.json")
    return os.path.expanduser(
        os.environ.get("NOTRIOS_CLAUDE_USAGE_CACHE") or CLAUDE_USAGE_CACHE
    )


def _read_claude_cache(path: str) -> tuple[dict[str, Any] | None, str | None]:
    try:
        with open(path, encoding="utf-8") as stream:
            data = json.load(stream)
    except FileNotFoundError:
        return None, (
            f"no status-line usage cache at {path}; install "
            "scripts/claude_statusline_usage.py as the statusLine command"
        )
    except OSError as error:
        return None, f"cannot read {path}: {error}"
    except ValueError:
        return None, f"{path} is not valid JSON"
    if not isinstance(data, dict):
        return None, f"{path} does not contain a JSON object"
    return data, None


def _claude_legacy_buckets(
    settings_path: str | None, version: str | None
) -> list[dict[str, Any]]:
    """Read the older `statusline_cache` block kept inside settings.json."""
    path = os.path.expanduser(settings_path or "~/.claude/settings.json")
    try:
        with open(path, encoding="utf-8") as stream:
            settings = json.load(stream)
    except (OSError, ValueError):
        return []
    cache = settings.get("statusline_cache") if isinstance(settings, dict) else None
    if not isinstance(cache, dict):
        return []
    buckets: list[dict[str, Any]] = []
    for name, value in cache.items():
        lowered = name.lower()
        if "usage" not in lowered and "limit" not in lowered:
            continue
        parsed = _text_percentage(value)
        if parsed is None:
            continue
        percentage, is_used = parsed
        raw = (
            value
            if isinstance(value, dict)
            else {"usedPercent": percentage if is_used else 100 - percentage}
        )
        bucket = _bucket(name, raw, version=version)
        if bucket is not None:
            buckets.append(bucket)
    return buckets


def probe_claude(
    settings_path: str | None = None,
    process_detector: Callable[[], bool] | None = None,
    version: str | None = None,
    cache_path: str | None = None,
    max_age_minutes: float | None = CLAUDE_MAX_AGE_MINUTES,
    now: float | None = None,
) -> dict[str, Any]:
    """Report Claude usage from local files only; never spend quota to measure it."""
    if not (process_detector or _claude_running)():
        return {
            "agent": "claude",
            "state": "not-running",
            "buckets": [],
            "client_version": None,
            "source": None,
        }

    now = time.time() if now is None else now
    resolved_version = version or _client_version(("claude",))
    path = _claude_cache_path(cache_path, settings_path)
    cache, error = _read_claude_cache(path)

    buckets: list[dict[str, Any]] = []
    source: str | None = None
    age_minutes: float | None = None
    if cache is not None:
        source = path
        buckets = parse_claude_rate_limits(
            cache.get("rate_limits"), resolved_version, now
        )
        captured = _epoch(cache.get("captured_at"))
        if captured is not None:
            age_minutes = max(0.0, (now - captured) / 60.0)
        if not buckets:
            error = (
                f"{path} carries no usable rate-limit window; the client may "
                "predate the status-line rate_limits field or may not be on a "
                "plan that reports limits"
            )

    if not buckets:
        legacy = _claude_legacy_buckets(settings_path, resolved_version)
        if legacy:
            buckets = legacy
            source = f"{os.path.expanduser(settings_path or '~/.claude/settings.json')}"
            source += " (legacy statusline_cache)"
            error = None
            age_minutes = None

    live = [item for item in buckets if not item.get("expired")]
    too_old = (
        age_minutes is not None
        and max_age_minutes is not None
        and age_minutes > max_age_minutes
    )
    stale = bool(buckets) and (not live or too_old)

    result = summarize(
        "claude", [] if stale else live, resolved_version, error
    )
    result["buckets"] = buckets
    result["source"] = source
    if age_minutes is not None:
        result["cache_age_minutes"] = round(age_minutes, 3)
    if stale:
        result["state"] = "stale"
        result["error"] = (
            f"usage cache is {age_minutes:.1f} minutes old, over the "
            f"{max_age_minutes:g}-minute limit"
            if too_old
            else "every reported rate-limit window is past its reset time"
        )
    return result


def _history(path: str | None) -> list[dict[str, Any]]:
    if not path:
        return []
    records: list[dict[str, Any]] = []
    try:
        with open(path, encoding="utf-8") as stream:
            for line in stream:
                try:
                    record = json.loads(line)
                except ValueError:
                    print(
                        "warning: ignoring malformed usage history line",
                        file=sys.stderr,
                    )
                    continue
                if isinstance(record, dict):
                    records.append(record)
    except OSError:
        pass
    return records


def required_reserve(
    explicit: float,
    _result: dict[str, Any],
    history: list[dict[str, Any]] | None = None,
    fields: dict[str, Any] | None = None,
    margin: float = 5,
) -> float:
    """Return the explicit floor or largest comparable observed drop + margin."""
    required = float(explicit)
    fields = fields or {}
    matching = {
        key: fields.get(key)
        for key in HISTORY_MATCH_FIELDS
        if key in fields
    }
    records = history or []
    for before in records:
        if before.get("phase") != "before":
            continue
        if any(before.get(key) != value for key, value in matching.items()):
            continue
        for after in records:
            if after.get("phase") != "after":
                continue
            if any(after.get(key) != value for key, value in matching.items()):
                continue
            if before.get("run_id") != after.get("run_id"):
                continue
            if before.get("reset_utc") != after.get("reset_utc"):
                continue
            if before.get("duration_minutes") != after.get("duration_minutes"):
                continue
            start = _percentage(before.get("remaining_percent"))
            finish = _percentage(after.get("remaining_percent"))
            if start is None or finish is None or finish > start:
                continue
            required = max(required, start - finish + margin)
    return required


def _append_history(path: str, record: dict[str, Any]) -> None:
    directory = os.path.dirname(os.path.abspath(path)) or "."
    os.makedirs(directory, exist_ok=True)
    lock_path = f"{path}.lock"
    with open(lock_path, "a+", encoding="utf-8") as lock:
        try:
            import fcntl

            fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        except ImportError:
            pass
        encoded = json.dumps(record, sort_keys=True) + "\n"
        with open(path, "a", encoding="utf-8") as stream:
            stream.write(encoded)
            stream.flush()
            os.fsync(stream.fileno())


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent", choices=("codex", "claude", "all"), default="all")
    parser.add_argument("--minimum-remaining", type=float, default=20)
    parser.add_argument("--strict", action="store_true")
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--timeout", type=float, default=10)
    parser.add_argument("--settings")
    parser.add_argument("--claude-usage-cache")
    parser.add_argument(
        "--claude-max-age-minutes", type=float, default=CLAUDE_MAX_AGE_MINUTES
    )
    parser.add_argument("--history")
    parser.add_argument("--operation", default="")
    parser.add_argument("--model", default="")
    parser.add_argument("--effort", default="")
    parser.add_argument("--sample", choices=("before", "after"))
    parser.add_argument("--run-id", default="")
    parser.add_argument("--margin", type=float, default=5)
    return parser


def _print_human(results: list[dict[str, Any]]) -> None:
    for result in results:
        print(
            f"{result['agent']}: state={result['state']} "
            f"binding_remaining={result.get('binding_remaining_percent', 'unknown')} "
            f"required_reserve={result['required_reserve_percent']} "
            f"version={result.get('client_version') or 'unknown'} "
            f"error={result.get('error', '')}"
        )
        if result.get("source"):
            age = result.get("cache_age_minutes")
            suffix = f" age_minutes={age}" if age is not None else ""
            print(f"  source={result['source']}{suffix}")
        for bucket in result.get("buckets", []):
            print(
                f"  {bucket['name']}: remaining={bucket['remaining_percent']} "
                f"used={bucket['used_percent']} "
                f"duration_minutes={bucket.get('duration_minutes')} "
                f"reset={bucket.get('reset_utc')}"
                + (" expired=true" if bucket.get("expired") else "")
            )


def main(argv: Sequence[str] | None = None) -> int:
    arguments = _parser().parse_args(argv)
    if not 0 <= arguments.minimum_remaining <= 100:
        print("error: --minimum-remaining must be between 0 and 100", file=sys.stderr)
        return 1
    if arguments.margin < 0:
        print("error: --margin must be non-negative", file=sys.stderr)
        return 1
    if arguments.timeout <= 0:
        print("error: --timeout must be positive", file=sys.stderr)
        return 1
    if arguments.claude_max_age_minutes <= 0:
        print(
            "error: --claude-max-age-minutes must be positive",
            file=sys.stderr,
        )
        return 1
    if arguments.sample and not all(
        (
            arguments.history,
            arguments.operation,
            arguments.model,
            arguments.effort,
            arguments.run_id,
        )
    ):
        print(
            "error: --sample requires --history, --operation, --model, "
            "--effort, and --run-id",
            file=sys.stderr,
        )
        return 1

    def claude() -> dict[str, Any]:
        return probe_claude(
            arguments.settings,
            cache_path=arguments.claude_usage_cache,
            max_age_minutes=arguments.claude_max_age_minutes,
        )

    if arguments.agent == "codex":
        results = [probe_codex(arguments.timeout)]
    elif arguments.agent == "claude":
        results = [claude()]
    else:
        results = [probe_codex(arguments.timeout), claude()]

    records = _history(arguments.history)
    base_fields = {
        "operation": arguments.operation,
        "model": arguments.model,
        "effort": arguments.effort,
        "run_id": arguments.run_id,
    }
    for result in results:
        fields = {**base_fields, "agent": result["agent"]}
        if (
            arguments.sample
            and result.get("binding_remaining_percent") is not None
            and result.get("state") in ("active", "pause")
        ):
            binding = min(
                result.get("buckets", []),
                key=lambda item: item["remaining_percent"],
            )
            assert arguments.history is not None
            _append_history(
                arguments.history,
                {
                    **fields,
                    "phase": arguments.sample,
                    "remaining_percent": result["binding_remaining_percent"],
                    "reset_utc": binding.get("reset_utc"),
                    "duration_minutes": binding.get("duration_minutes"),
                },
            )
            records = _history(arguments.history)
        result["required_reserve_percent"] = required_reserve(
            arguments.minimum_remaining,
            result,
            records,
            fields,
            arguments.margin,
        )

    unknown = any(result["state"] in ("unknown", "stale") for result in results)
    pause = any(
        result["state"] == "pause"
        or (
            result.get("binding_remaining_percent") is not None
            and result["binding_remaining_percent"]
            <= result["required_reserve_percent"]
        )
        for result in results
    )

    if arguments.as_json:
        output: Any = results if arguments.agent == "all" else results[0]
        print(json.dumps(output, sort_keys=True))
    else:
        _print_human(results)
    if pause:
        return 2
    if unknown and arguments.strict:
        return 3
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
