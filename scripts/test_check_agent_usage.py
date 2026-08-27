#!/usr/bin/env python3
from __future__ import annotations

from contextlib import redirect_stdout
import io
import json
import os
import tempfile
import unittest
from unittest import mock

import check_agent_usage as usage


def bucket(remaining: float, reset: str = "reset", duration: int = 60):
    return {
        "name": "primary",
        "used_percent": 100 - remaining,
        "remaining_percent": remaining,
        "reset_utc": reset,
        "duration_minutes": duration,
        "exhausted": remaining <= 0,
    }


def result(agent: str, remaining: float) -> dict:
    return {
        "agent": agent,
        "state": "active" if remaining > 0 else "pause",
        "buckets": [bucket(remaining)],
        "binding_remaining_percent": remaining,
        "binding_reset_utc": "reset",
        "binding_duration_minutes": 60,
        "client_version": "test-version",
    }


class UsageTests(unittest.TestCase):
    def test_current_primary_secondary_payload(self) -> None:
        buckets = usage.parse_codex_response(
            {
                "result": {
                    "rateLimits": {
                        "primary": {"usedPercent": 10, "resetAt": "x"},
                        "secondary": {"usedPercent": 90},
                    }
                }
            }
        )
        self.assertEqual([item["remaining_percent"] for item in buckets], [90, 10])

    def test_legacy_single_bucket(self) -> None:
        buckets = usage.parse_codex_response({"rateLimits": {"usedPercent": 25}})
        self.assertEqual(buckets[0]["remaining_percent"], 75)

    def test_exact_live_payload_and_duplicate_map(self) -> None:
        snapshot = {
            "limitId": "codex",
            "primary": {
                "usedPercent": 24,
                "resetsAt": 1787885451,
                "windowDurationMins": 300,
            },
            "secondary": {
                "usedPercent": 19,
                "resetsAt": 1788454031,
                "windowDurationMins": 10080,
            },
            "rateLimitReachedType": None,
        }
        payload = {
            "result": {
                "rateLimits": snapshot,
                "rateLimitsByLimitId": {"codex": snapshot},
            }
        }
        buckets = usage.parse_codex_response(payload, "codex-cli 0.150.1")
        self.assertEqual([item["remaining_percent"] for item in buckets], [76, 81])
        self.assertEqual([item["duration_minutes"] for item in buckets], [300, 10080])
        self.assertEqual({item["limit_id"] for item in buckets}, {"codex"})
        self.assertEqual(buckets[0]["version"], "codex-cli 0.150.1")

    def test_duplicate_snapshots_are_deduplicated(self) -> None:
        window = {
            "usedPercent": 18,
            "resetsAt": "2026-08-27T12:00:00Z",
            "windowDurationMins": 300,
        }
        payload = {
            "result": {
                "rateLimits": {},
                "rateLimitsByLimitId": {
                    "first": {"primary": window},
                    "duplicate": {"primary": window},
                },
            }
        }
        self.assertEqual(len(usage.parse_codex_response(payload)), 1)

    def test_exhaustion_comes_from_percentage_or_snapshot(self) -> None:
        used = usage.parse_codex_response(
            {"result": {"rateLimits": {"primary": {"usedPercent": 100}}}}
        )
        reached = usage.parse_codex_response(
            {
                "result": {
                    "rateLimits": {
                        "primary": {"usedPercent": 10},
                        "rateLimitReachedType": "rate_limit_reached",
                    }
                }
            }
        )
        self.assertTrue(used[0]["exhausted"])
        self.assertTrue(reached[0]["exhausted"])
        self.assertEqual(usage.summarize("codex", reached)["state"], "pause")

    def test_codex_timeout_reaps_process(self) -> None:
        class SlowProcess:
            stdin = mock.Mock()
            stdout = mock.Mock()
            pid = 12345

            def terminate(self):
                return None

            def wait(self, timeout=None):
                return None

        process = SlowProcess()
        with mock.patch.object(usage.subprocess, "Popen", return_value=process), \
                mock.patch.object(usage, "_client_version", return_value="v"), \
                mock.patch.object(usage.os, "killpg") as kill_group, \
                mock.patch.object(
                    usage.selectors.DefaultSelector, "select", return_value=[]
                ):
            got = usage.probe_codex(0.01, ["fake"], "v")
        self.assertEqual(got["state"], "unknown")
        process.stdin.close.assert_called_once()
        kill_group.assert_called_once_with(12345, usage.signal.SIGTERM)

    def test_claude_missing_malformed_and_numeric_used(self) -> None:
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = os.path.join(directory, "settings.json")
            self.assertEqual(usage.probe_claude(path, lambda: True)["state"], "unknown")
            with open(path, "w", encoding="utf-8") as stream:
                stream.write("{")
            self.assertEqual(usage.probe_claude(path, lambda: True)["state"], "unknown")
            with open(path, "w", encoding="utf-8") as stream:
                json.dump({"statusline_cache": {"five_hour_usage": 30}}, stream)
            self.assertEqual(
                usage.probe_claude(path, lambda: True)["binding_remaining_percent"],
                70,
            )

    def test_claude_used_and_remaining_strings_agree(self) -> None:
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = os.path.join(directory, "settings.json")
            with open(path, "w", encoding="utf-8") as stream:
                json.dump(
                    {
                        "statusline_cache": {
                            "five_hour_usage": "18% used",
                            "weekly_usage": "82% remaining",
                        }
                    },
                    stream,
                )
            got = usage.probe_claude(path, lambda: True)
        self.assertEqual(got["binding_remaining_percent"], 82)

    def test_claude_not_running_is_neutral_in_strict_mode(self) -> None:
        not_running = {
            "agent": "claude",
            "state": "not-running",
            "buckets": [],
            "client_version": None,
        }
        with mock.patch.object(usage, "probe_claude", return_value=not_running):
            self.assertEqual(usage.main(["--agent", "claude", "--strict"]), 0)

    def test_history_learns_prior_run_and_isolates_tuple(self) -> None:
        records = [
            {
                "phase": phase,
                "agent": "codex",
                "operation": "op",
                "model": "model",
                "effort": "high",
                "run_id": "old-run",
                "remaining_percent": remaining,
                "reset_utc": "same-reset",
                "duration_minutes": 300,
            }
            for phase, remaining in (("before", 90), ("after", 60))
        ]
        fields = {
            "agent": "codex",
            "operation": "op",
            "model": "model",
            "effort": "high",
            "run_id": "current-run",
        }
        self.assertEqual(usage.required_reserve(20, {}, records, fields, 5), 35)
        self.assertEqual(
            usage.required_reserve(
                20, {}, records, {**fields, "model": "different"}, 5
            ),
            20,
        )

    def test_history_ignores_reset_crossing(self) -> None:
        base = {
            "agent": "codex",
            "operation": "op",
            "model": "model",
            "effort": "high",
            "run_id": "run",
            "duration_minutes": 300,
        }
        crossed = [
            {**base, "phase": "before", "remaining_percent": 10, "reset_utc": "a"},
            {**base, "phase": "after", "remaining_percent": 90, "reset_utc": "b"},
        ]
        self.assertEqual(usage.required_reserve(20, {}, crossed, base, 5), 20)

    def test_sampling_appends_only_known_binding(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            history = os.path.join(directory, "usage.jsonl")
            with mock.patch.object(usage, "probe_codex", return_value=result("codex", 90)):
                status = usage.main(
                    ["--agent", "codex", "--history", history, "--operation", "op",
                     "--model", "model", "--effort", "high", "--run-id", "run",
                     "--sample", "before", "--json"]
                )
            self.assertEqual(status, 0)
            self.assertEqual(len(usage._history(history)), 1)

            unknown = {"agent": "codex", "state": "unknown", "buckets": []}
            with mock.patch.object(usage, "probe_codex", return_value=unknown):
                usage.main(
                    ["--agent", "codex", "--history", history, "--operation", "op",
                     "--model", "model", "--effort", "high", "--run-id", "unknown",
                     "--sample", "after"]
                )
            self.assertEqual(len(usage._history(history)), 1)

    def test_adaptive_reserve_can_pause_later_run(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            history = os.path.join(directory, "usage.jsonl")
            for phase, remaining in (("before", 90), ("after", 60)):
                usage._append_history(
                    history,
                    {
                        "phase": phase, "agent": "codex", "operation": "op",
                        "model": "model", "effort": "high", "run_id": "old",
                        "remaining_percent": remaining, "reset_utc": "reset",
                        "duration_minutes": 60,
                    },
                )
            with mock.patch.object(usage, "probe_codex", return_value=result("codex", 35)):
                status = usage.main(
                    ["--agent", "codex", "--history", history, "--operation", "op",
                     "--model", "model", "--effort", "high", "--run-id", "new"]
                )
            self.assertEqual(status, 2)

    def test_threshold_equality_pauses(self) -> None:
        with mock.patch.object(usage, "probe_codex", return_value=result("codex", 20)):
            self.assertEqual(
                usage.main(["--agent", "codex", "--minimum-remaining", "20"]), 2
            )

    def test_unknown_is_advisory_or_strict(self) -> None:
        unknown = {"agent": "codex", "state": "unknown", "buckets": []}
        with mock.patch.object(usage, "probe_codex", return_value=unknown):
            self.assertEqual(usage.main(["--agent", "codex"]), 0)
            self.assertEqual(usage.main(["--agent", "codex", "--strict"]), 3)

    def test_argument_validation_returns_one(self) -> None:
        self.assertEqual(usage.main(["--timeout", "0"]), 1)
        self.assertEqual(usage.main(["--margin", "-1"]), 1)
        self.assertEqual(usage.main(["--minimum-remaining", "101"]), 1)
        self.assertEqual(usage.main(["--sample", "before"]), 1)

    def test_json_and_human_output_include_contract_fields(self) -> None:
        with mock.patch.object(usage, "probe_codex", return_value=result("codex", 75)):
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertEqual(usage.main(["--agent", "codex", "--json"]), 0)
            payload = json.loads(output.getvalue())
            self.assertEqual(payload["binding_remaining_percent"], 75)
            self.assertEqual(payload["required_reserve_percent"], 20)

            output = io.StringIO()
            with redirect_stdout(output):
                self.assertEqual(usage.main(["--agent", "codex"]), 0)
            self.assertIn("primary: remaining=75", output.getvalue())


if __name__ == "__main__":
    unittest.main()
