#!/usr/bin/env python3
"""Resolution fixtures for the H3 path resolver model.

Produces RESOLUTION_TABLE.json, which is the "exact path-resolution table" H3
owes. It is generated from the model rather than typed, so the table and the
rules cannot disagree.

Run: python3 performance/v0.8-h3/test_resolve_model.py
"""

from __future__ import annotations

import json
import os
import sys
import inspect
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from purge_oracle import Environment, decide
from resolve_model import LINUX, MACOS, WINDOWS, ResolutionError, resolve

SCENARIOS = []
FAILURES = []


def scenario(name, *, expect_roots=None, expect_code=None, expect_error=None, **kwargs):
    # A callable's repr carries its memory address, which would make this file
    # differ on every run. Injected callables are recorded as what they are.
    def describe(value):
        if callable(value):
            return "<injected>"
        return value if isinstance(value, (str, bool, dict, type(None))) else str(value)

    # Record the *effective* inputs, defaults included. A table that omits an
    # argument the model defaulted is not reproducible by another
    # implementation: it would have to guess the default, and a guess that
    # happens to match is not the same as a contract.
    bound = inspect.signature(resolve).bind(**kwargs)
    bound.apply_defaults()
    entry = {"scenario": name, "inputs": {k: describe(v) for k, v in bound.arguments.items()}}
    try:
        result = resolve(**kwargs)
    except ResolutionError as err:
        entry["error"] = str(err)
        entry["roots"] = {}
        entry["notices"] = []
        if expect_error is None:
            FAILURES.append(f"{name}: unexpected refusal {err}")
        elif expect_error not in str(err):
            FAILURES.append(f"{name}: refusal {str(err)!r} does not mention {expect_error!r}")
        SCENARIOS.append(entry)
        return
    entry["mode"] = result.mode
    entry["roots"] = result.roots
    entry["notice_codes"] = result.codes()
    entry["notices"] = result.notices
    if expect_error is not None:
        FAILURES.append(f"{name}: expected a refusal mentioning {expect_error!r}, got {result.roots}")
    for root, expected in (expect_roots or {}).items():
        if result.roots.get(root) != expected:
            FAILURES.append(f"{name}: {root} = {result.roots.get(root)!r}, expected {expected!r}")
    if expect_code and expect_code not in result.codes():
        FAILURES.append(f"{name}: expected notice code {expect_code!r}, got {result.codes()}")

    # Every root the resolver produces must be a path the purge oracle will
    # accept as well-formed. These are two artifacts of one investigation, and
    # H4 found them disagreeing: the resolver emitted ".." segments that the
    # oracle refuses under its normal-form rule, so a portable installation
    # could not have been purged. Checking it here means they cannot drift
    # apart again.
    # Scoped to what the oracle actually governs: it is a POSIX tool, and
    # source-mode roots are working-directory-relative by design because a
    # checkout is where that is the correct answer. Purge operates on installed
    # and portable layouts.
    posix_target = kwargs.get("os_name", LINUX) in (LINUX, MACOS)
    if posix_target and result.mode in ("installed", "portable"):
        for root_name, root in result.roots.items():
            verdict = decide(root, Environment(owned_roots=[root], home="/home/u"))
            if verdict.rule in ("normal-form", "absolute", "non-empty"):
                FAILURES.append(
                    f"{name}: resolved {root_name}={root!r} is malformed by the purge "
                    f"oracle's own rules ({verdict.rule}): {verdict.reason}"
                )
    SCENARIOS.append(entry)


def run_cases() -> None:
    home = "/home/u"

    scenario("linux, no XDG variables set",
             env={"HOME": home}, os_name=LINUX,
             expect_roots={"config": "/home/u/.config/notrios",
                           "data": "/home/u/.local/share/notrios",
                           "state": "/home/u/.local/state/notrios",
                           "cache": "/home/u/.cache/notrios",
                           "runtime": "/home/u/.local/state/notrios/runtime"},
             expect_code="runtime_dir_unset")

    scenario("linux, all XDG variables absolute",
             env={"HOME": home, "XDG_CONFIG_HOME": "/x/cfg", "XDG_DATA_HOME": "/x/dat",
                  "XDG_STATE_HOME": "/x/st", "XDG_CACHE_HOME": "/x/ca",
                  "XDG_RUNTIME_DIR": "/run/user/1000"},
             os_name=LINUX,
             expect_roots={"config": "/x/cfg/notrios", "data": "/x/dat/notrios",
                           "state": "/x/st/notrios", "cache": "/x/ca/notrios",
                           "runtime": "/run/user/1000/notrios"})

    scenario("linux, XDG_CONFIG_HOME is relative",
             env={"HOME": home, "XDG_CONFIG_HOME": "relative/config"}, os_name=LINUX,
             expect_roots={"config": "/home/u/.config/notrios"},
             expect_code="xdg_relative_ignored")

    scenario("linux, XDG_DATA_HOME is whitespace only",
             env={"HOME": home, "XDG_DATA_HOME": "   "}, os_name=LINUX,
             expect_roots={"data": "/home/u/.local/share/notrios"})

    scenario("linux, XDG_RUNTIME_DIR exists but is world-readable",
             env={"HOME": home, "XDG_RUNTIME_DIR": "/run/shared"}, os_name=LINUX,
             runtime_dir_is_private=lambda path: False,
             expect_roots={"runtime": "/home/u/.local/state/notrios/runtime"},
             expect_code="runtime_dir_not_private")

    scenario("linux, no HOME at all",
             env={}, os_name=LINUX,
             expect_error="no home directory is set")

    scenario("windows, native locations",
             env={"USERPROFILE": "C:\\Users\\u", "APPDATA": "C:\\Users\\u\\AppData\\Roaming",
                  "LOCALAPPDATA": "C:\\Users\\u\\AppData\\Local"},
             os_name=WINDOWS, executable_dir="C:\\Program Files\\Notrios",
             expect_roots={"config": "C:\\Users\\u\\AppData\\Roaming\\Notrios\\Config",
                           "data": "C:\\Users\\u\\AppData\\Local\\Notrios\\Data",
                           "program_assets": "C:\\Program Files\\Notrios"})

    scenario("windows ignores XDG_CONFIG_HOME",
             env={"USERPROFILE": "C:\\Users\\u", "APPDATA": "C:\\Users\\u\\AppData\\Roaming",
                  "LOCALAPPDATA": "C:\\Users\\u\\AppData\\Local", "XDG_CONFIG_HOME": "C:\\xdg"},
             os_name=WINDOWS, executable_dir="C:\\Program Files\\Notrios",
             expect_roots={"config": "C:\\Users\\u\\AppData\\Roaming\\Notrios\\Config"},
             expect_code="xdg_ignored_on_platform")

    scenario("macos, native locations",
             env={"HOME": "/Users/u"}, os_name=MACOS,
             executable_dir="/Applications/Notrios.app/Contents/MacOS",
             expect_roots={"config": "/Users/u/Library/Application Support/Notrios/Config",
                           "cache": "/Users/u/Library/Caches/Notrios",
                           "program_assets": "/Applications/Notrios.app/Contents/Resources"})

    scenario("macos ignores XDG_CACHE_HOME",
             env={"HOME": "/Users/u", "XDG_CACHE_HOME": "/Users/u/xdgcache"}, os_name=MACOS,
             expect_roots={"cache": "/Users/u/Library/Caches/Notrios"},
             expect_code="xdg_ignored_on_platform")

    scenario("portable mode, selected by the marker file",
             env={"HOME": home}, os_name=LINUX, portable_marker=True,
             executable_dir="/media/stick/notrios/bin",
             expect_roots={"data": "/media/stick/notrios/notrios-data/data",
                           "program_assets": "/media/stick/notrios/share/notrios"},
             expect_code="portable_selected")

    scenario("portable mode is not selected by a writable working directory",
             env={"HOME": home}, os_name=LINUX, portable_marker=False,
             expect_roots={"data": "/home/u/.local/share/notrios"})

    scenario("source mode, a checkout beside the executable",
             env={"HOME": home}, os_name=LINUX, source_checkout=True,
             expect_roots={"config": "config", "data": "data/data",
                           "program_assets": "web/dist"},
             expect_code="source_selected")

    scenario("explicit paths win over the installed layout",
             env={"HOME": home}, os_name=LINUX,
             explicit={"data": "/srv/notrios/library"},
             expect_roots={"data": "/srv/notrios/library",
                           "config": "/home/u/.config/notrios"},
             expect_code="explicit_override")

    scenario("an explicit relative path is refused rather than resolved",
             env={"HOME": home}, os_name=LINUX,
             explicit={"data": "library"},
             expect_error="is relative")

    _write_table()


def _write_table() -> None:
    report = {
        "schema": "notrios.path-resolution-table/1",
        "milestone": "v0.8",
        "item": "H3",
        "note": "Generated by test_resolve_model.py from resolve_model.py. H4 must reproduce this table.",
        "total": len(SCENARIOS),
        "failed": len(FAILURES),
        "scenarios": SCENARIOS,
    }
    destination = os.path.join(os.path.dirname(os.path.abspath(__file__)), "RESOLUTION_TABLE.json")
    with open(destination, "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2)
        handle.write("\n")


class ResolutionScenarios(unittest.TestCase):
    """Wrapped as unittest so `unittest discover` runs these rather than
    importing the module, finding no TestCase, and reporting success."""

    @classmethod
    def setUpClass(cls) -> None:
        SCENARIOS.clear()
        FAILURES.clear()
        run_cases()

    def test_every_scenario_resolved_as_specified(self) -> None:
        self.assertTrue(SCENARIOS, "no resolution scenarios ran")
        self.assertEqual(FAILURES, [], "\n".join(FAILURES))

    def test_enough_scenarios_ran_to_be_worth_trusting(self) -> None:
        self.assertGreaterEqual(len(SCENARIOS), 15)


def main() -> int:
    run_cases()
    for failure in FAILURES:
        print("FAIL:", failure)
    print(f"{len(SCENARIOS) - len(FAILURES)}/{len(SCENARIOS)} resolution scenarios matched")
    return 1 if FAILURES else 0


if __name__ == "__main__":
    raise SystemExit(main())
