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

    def _write_cache(self, directory: str, document: dict) -> str:
        path = os.path.join(directory, "usage-cache.json")
        with open(path, "w", encoding="utf-8") as stream:
            json.dump(document, stream)
        return path

    def test_claude_status_line_rate_limits_bind_on_the_tightest_window(self) -> None:
        now = 1_800_000_000.0
        document = {
            "captured_at": now - 60,
            "rate_limits": {
                "5h": {"used_percentage": 18, "resets_at": now + 3600},
                "7d": {"used_percentage": 73, "resets_at": now + 200000},
                "spend_limit": {"used_usd": 4, "limit_usd": 50},
            },
        }
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = self._write_cache(directory, document)
            got = usage.probe_claude(
                None, lambda: True, cache_path=path, now=now
            )
        self.assertEqual(got["state"], "active")
        # The 7-day window is tighter, and the dollar-denominated spend limit
        # carries no percentage so it must never become the binding constraint.
        self.assertEqual(got["binding_remaining_percent"], 27)
        self.assertEqual(got["binding_duration_minutes"], 10080)
        self.assertEqual([item["name"] for item in got["buckets"]], ["5h", "7d"])
        self.assertEqual(got["cache_age_minutes"], 1.0)

    def test_claude_float_noise_is_rounded_for_display(self) -> None:
        now = 1_800_000_000.0
        document = {
            "captured_at": now,
            # The client really does emit this; it computes the percentage.
            "rate_limits": {"5h": {"used_percentage": 28.000000000000004,
                                   "resets_at": now + 3600}},
        }
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = self._write_cache(directory, document)
            got = usage.probe_claude(None, lambda: True, cache_path=path, now=now)
        self.assertEqual(got["buckets"][0]["used_percent"], 28.0)
        self.assertEqual(got["binding_remaining_percent"], 72.0)

    def test_claude_missing_cache_is_unknown_and_names_the_fix(self) -> None:
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            got = usage.probe_claude(
                None,
                lambda: True,
                cache_path=os.path.join(directory, "usage-cache.json"),
            )
        self.assertEqual(got["state"], "unknown")
        self.assertIsNone(got.get("binding_remaining_percent"))
        self.assertIn("claude_statusline_usage.py", got["error"])

    def test_claude_cache_without_rate_limits_is_unknown_not_full(self) -> None:
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = self._write_cache(directory, {"captured_at": 1_800_000_000.0})
            got = usage.probe_claude(
                None, lambda: True, cache_path=path, now=1_800_000_000.0
            )
        self.assertEqual(got["state"], "unknown")
        self.assertIsNone(got.get("binding_remaining_percent"))

    def test_claude_stale_cache_does_not_bind(self) -> None:
        now = 1_800_000_000.0
        document = {
            "captured_at": now - 7200,
            "rate_limits": {"5h": {"used_percentage": 5, "resets_at": now + 600}},
        }
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = self._write_cache(directory, document)
            got = usage.probe_claude(
                None, lambda: True, cache_path=path, max_age_minutes=30, now=now
            )
        self.assertEqual(got["state"], "stale")
        # A two-hour-old 95%-remaining reading must not authorize a long run.
        self.assertIsNone(got.get("binding_remaining_percent"))
        self.assertIn("120.0 minutes old", got["error"])

    def test_claude_window_past_its_reset_is_excluded(self) -> None:
        now = 1_800_000_000.0
        document = {
            "captured_at": now - 60,
            "rate_limits": {
                "5h": {"used_percentage": 99, "resets_at": now - 10},
                "7d": {"used_percentage": 40, "resets_at": now + 200000},
            },
        }
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = self._write_cache(directory, document)
            got = usage.probe_claude(
                None, lambda: True, cache_path=path, now=now
            )
        self.assertEqual(got["state"], "active")
        self.assertEqual(got["binding_remaining_percent"], 60)
        self.assertTrue(got["buckets"][0]["expired"])

    def test_claude_all_windows_expired_is_stale(self) -> None:
        now = 1_800_000_000.0
        document = {
            "captured_at": now - 60,
            "rate_limits": {"5h": {"used_percentage": 99, "resets_at": now - 10}},
        }
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = self._write_cache(directory, document)
            got = usage.probe_claude(None, lambda: True, cache_path=path, now=now)
        self.assertEqual(got["state"], "stale")
        self.assertIn("past its reset", got["error"])

    def test_claude_accepts_iso_resets_and_remaining_percentage(self) -> None:
        document = {
            "captured_at": "2027-01-15T08:00:00Z",
            "rate_limits": [
                {
                    "name": "weekly",
                    "remaining_percentage": 12,
                    "resets_at": "2027-01-20T08:00:00Z",
                }
            ],
        }
        captured = 1_800_000_000.0
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(usage, "_client_version", return_value="claude-test"):
            path = self._write_cache(directory, document)
            got = usage.probe_claude(
                None, lambda: True, cache_path=path, now=captured, max_age_minutes=None
            )
        self.assertEqual(got["state"], "active")
        self.assertEqual(got["binding_remaining_percent"], 12)
        self.assertEqual(got["buckets"][0]["duration_minutes"], 10080)

    def test_claude_stale_pauses_strict_mode_but_not_the_default_gate(self) -> None:
        stale = {
            "agent": "claude",
            "state": "stale",
            "buckets": [],
            "client_version": "claude-test",
            "error": "cache is old",
        }
        with mock.patch.object(usage, "probe_claude", return_value=stale):
            self.assertEqual(usage.main(["--agent", "claude"]), 0)
            self.assertEqual(usage.main(["--agent", "claude", "--strict"]), 3)

    def test_claude_max_age_flag_is_validated(self) -> None:
        self.assertEqual(
            usage.main(["--agent", "claude", "--claude-max-age-minutes", "0"]), 1
        )

    def test_epoch_handles_milliseconds_and_iso(self) -> None:
        self.assertEqual(usage._epoch(1_800_000_000_000), 1_800_000_000.0)
        self.assertEqual(usage._epoch("1800000000"), 1_800_000_000.0)
        self.assertIsNone(usage._epoch("reset"))
        self.assertEqual(usage._reset_utc("reset"), "reset")
        self.assertEqual(
            usage._reset_utc(1_800_000_000_000), usage._reset_utc(1_800_000_000)
        )

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


CLAUDE_EXE = "/home/user/.local/share/claude/versions/2.1.270"
CODEX_VENDOR = (
    "/home/user/.nvm/versions/node/v26.3.0/lib/node_modules/@openai/codex/"
    "node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/"
)


def process_tree(entries: dict[int, tuple[int, str, list[str]]]):
    """A fake process table: pid -> (parent pid, executable, argv)."""
    return lambda pid: entries.get(pid)


class RunningAgentTests(unittest.TestCase):
    """J24: a run is guarded by the agent that launched it, found by ancestry.

    Several agents can run on one machine at once, and an agent's environment
    variables leak into shells started from it, so neither a machine-wide
    process scan nor an inherited variable says which agent this run belongs to.
    """

    def test_agent_processes_are_classified_from_executable_and_argv0(self) -> None:
        classify = usage.classify_agent_process
        self.assertEqual(classify(CLAUDE_EXE, ["claude", "--resume", "id"]), "claude")
        self.assertEqual(classify("/usr/local/bin/claude", ["claude"]), "claude")
        self.assertEqual(classify(CODEX_VENDOR + "codex", [CODEX_VENDOR + "codex"]), "codex")
        self.assertEqual(
            classify(CODEX_VENDOR + "codex-code-mode-host", [CODEX_VENDOR + "codex-code-mode-host"]),
            "codex",
        )
        self.assertEqual(
            classify("/usr/bin/node", ["node", "/home/user/.nvm/bin/codex", "resume"]), "codex"
        )
        self.assertIsNone(classify("/usr/bin/bash", ["/bin/bash", "-c", "claude"]))
        self.assertIsNone(classify("/usr/bin/vim", ["vim", "codex", "claude"]))
        self.assertIsNone(classify("/usr/bin/node", ["node", "/srv/claude-notes/app.js"]))

    def test_nearest_agent_ancestor_is_the_running_agent(self) -> None:
        tree = {
            50: (40, "/usr/bin/python3", ["python3", "check_agent_usage.py"]),
            40: (30, "/usr/bin/bash", ["bash", "scripts/agent_usage_preflight.sh"]),
            30: (20, CLAUDE_EXE, ["claude"]),
            20: (10, "/usr/bin/bash", ["/bin/bash"]),
            10: (1, CODEX_VENDOR + "codex", [CODEX_VENDOR + "codex"]),
        }
        agent, reason = usage.identify_running_agent(50, process_tree(tree))
        self.assertEqual(agent, "claude")
        self.assertIn("pid 30", reason)
        agent, _ = usage.identify_running_agent(20, process_tree(tree))
        self.assertEqual(agent, "codex")

    def test_no_agent_ancestor_and_a_parent_cycle_identify_nothing(self) -> None:
        orphan = {50: (1, "/usr/bin/python3", ["python3"]), 1: (0, "/sbin/init", ["init"])}
        self.assertIsNone(usage.identify_running_agent(50, process_tree(orphan))[0])
        cycle = {50: (40, "/usr/bin/bash", ["bash"]), 40: (50, "/usr/bin/bash", ["bash"])}
        self.assertIsNone(usage.identify_running_agent(50, process_tree(cycle))[0])
        self.assertIsNone(usage.identify_running_agent(50, process_tree({}))[0])

    def test_ancestry_wins_over_inherited_environment(self) -> None:
        claude = lambda: ("claude", "process ancestry: pid 30 claude")
        leaked = {"CODEX_THREAD_ID": "x", "NOTRIOS_AGENT_USAGE_AGENT": "codex"}
        agent, reason = usage.resolve_self_agent(leaked, claude)
        self.assertEqual(agent, "claude")
        self.assertIn("ignored NOTRIOS_AGENT_USAGE_AGENT=codex", reason)
        codex = lambda: ("codex", "process ancestry: pid 10 codex")
        self.assertEqual(usage.resolve_self_agent({"CLAUDECODE": "1"}, codex)[0], "codex")

    def test_explicit_agent_applies_only_when_ancestry_finds_none(self) -> None:
        nobody = lambda: (None, "no coding agent among this process's ancestors")
        self.assertEqual(
            usage.resolve_self_agent({"NOTRIOS_AGENT_USAGE_AGENT": "codex"}, nobody)[0], "codex"
        )
        self.assertEqual(usage.resolve_self_agent({"CLAUDECODE": "1"}, nobody)[0], "all")
        agent, reason = usage.resolve_self_agent({"NOTRIOS_AGENT_USAGE_AGENT": "gpt"}, nobody)
        self.assertEqual(agent, "all")
        self.assertIn("gpt", reason)

    def _self_run(self, running: str, claude_remaining: float, codex_remaining: float):
        other = "codex" if running == "claude" else "claude"
        probes = {
            "probe_claude": mock.Mock(return_value=result("claude", claude_remaining)),
            "probe_codex": mock.Mock(return_value=result("codex", codex_remaining)),
        }
        with mock.patch.object(
            usage, "identify_running_agent", return_value=(running, f"pid 30 {running}")
        ), mock.patch.object(usage, "probe_claude", probes["probe_claude"]), \
                mock.patch.object(usage, "probe_codex", probes["probe_codex"]), \
                mock.patch.dict(os.environ, {"NOTRIOS_AGENT_USAGE_AGENT": ""}), \
                redirect_stdout(io.StringIO()):
            status = usage.main(["--agent", "self", "--minimum-remaining", "20", "--json"])
        probes[f"probe_{other}"].assert_not_called()
        return status

    def test_a_claude_run_is_not_paused_by_codex_quota_but_is_by_its_own(self) -> None:
        self.assertEqual(self._self_run("claude", claude_remaining=91, codex_remaining=8), 0)
        self.assertEqual(self._self_run("claude", claude_remaining=10, codex_remaining=90), 2)

    def test_a_codex_run_is_not_paused_by_claude_quota_but_is_by_its_own(self) -> None:
        self.assertEqual(self._self_run("codex", claude_remaining=8, codex_remaining=91), 0)
        self.assertEqual(self._self_run("codex", claude_remaining=90, codex_remaining=10), 2)

    def test_an_unidentified_run_is_guarded_by_every_agent_as_before(self) -> None:
        with mock.patch.object(
            usage, "identify_running_agent", return_value=(None, "none")
        ), mock.patch.object(usage, "probe_claude", return_value=result("claude", 91)), \
                mock.patch.object(usage, "probe_codex", return_value=result("codex", 8)), \
                mock.patch.dict(os.environ, {"NOTRIOS_AGENT_USAGE_AGENT": ""}), \
                redirect_stdout(io.StringIO()) as output:
            self.assertEqual(usage.main(["--agent", "self", "--json"]), 2)
        self.assertEqual(len(json.loads(output.getvalue())), 2)

    def test_this_process_ancestry_reads_the_real_process_table(self) -> None:
        if not os.path.isdir("/proc/self"):
            self.skipTest("no /proc")
        entry = usage.read_process(os.getpid())
        self.assertIsNotNone(entry)
        self.assertEqual(entry[0], os.getppid())


if __name__ == "__main__":
    unittest.main()
