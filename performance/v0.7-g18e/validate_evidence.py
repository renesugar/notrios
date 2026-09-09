#!/usr/bin/env python3
"""Strict, dependency-free validator for the G18e browser evidence pair."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


EXPECTED_VIEWPORTS = [
    {"id": "desktop", "width": 1440, "height": 960},
    {"id": "narrow-sync", "width": 390, "height": 844},
]
EXPECTED_SCREENSHOTS = ["/tmp/notrios-g18e-desktop.png", "/tmp/notrios-g18e-narrow-sync.png"]
ALLOWED_401 = "Failed to load resource: the server responded with a status of 401 (Unauthorized)"
REQUIRED_ACTION_KEYS = {"clicks", "keypresses", "typed_fields", "branches", "maximum_modal_depth", "recovery_steps"}
REQUIRED_ASSERTION_KEYS = {"visible", "canonical"}


class EvidenceError(ValueError):
    pass


def _fail(message: str) -> None:
    raise EvidenceError(message)


def _obj(value: Any, name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        _fail(f"{name} must be an object")
    return value


def _nonnegative_int(value: Any, name: str) -> None:
    if not isinstance(value, int) or isinstance(value, bool) or value < 0:
        _fail(f"{name} must be a non-negative integer")


def validate(manifest: dict[str, Any], report: dict[str, Any]) -> None:
    if manifest.get("schema") != "notrios.docjourney.manifest.v1":
        _fail("wrong manifest schema")
    if report.get("schema") != "notrios.docjourney.report.v1":
        _fail("wrong report schema")
    if manifest.get("viewports") != EXPECTED_VIEWPORTS or report.get("viewports") != EXPECTED_VIEWPORTS:
        _fail("viewports must be exactly desktop 1440x960 and narrow-sync 390x844")

    journeys = manifest.get("journeys")
    if not isinstance(journeys, list) or len(journeys) != 37:
        _fail("manifest must contain exactly 37 journeys")
    by_id: dict[str, dict[str, Any]] = {}
    for journey in journeys:
        item = _obj(journey, "journey")
        jid = item.get("id")
        if not isinstance(jid, str) or not jid or jid in by_id:
            _fail("journey IDs must be unique nonempty strings")
        by_id[jid] = item
        if not isinstance(item.get("label"), str) or not item["label"].strip():
            _fail(f"{jid}: label is required")
        state = item.get("state")
        if state not in {"executed", "unverified"}:
            _fail(f"{jid}: invalid state")
        if state == "executed":
            views = item.get("viewports")
            if not isinstance(views, list) or not views or any(v not in {"desktop", "narrow-sync"} for v in views) or len(set(views)) != len(views):
                _fail(f"{jid}: invalid declared viewports")
            if not isinstance(item.get("postconditions"), list) or not item["postconditions"]:
                _fail(f"{jid}: postconditions required")
        elif not isinstance(item.get("unrun_reason"), dict) or set(item["unrun_reason"]) != {"code", "detail", "owner"}:
            _fail(f"{jid}: unverified journey must have exact unrun_reason")

    executed = [j for j in journeys if j.get("state") == "executed"]
    unverified = [j for j in journeys if j.get("state") == "unverified"]
    if len(executed) != 32 or len(unverified) != 5:
        _fail("manifest counts must be 32 executed and 5 unverified")
    counts = _obj(report.get("manifest_counts"), "manifest_counts")
    if counts != {"total": 37, "executed": 32, "unverified": 5}:
        _fail("report manifest_counts mismatch")
    runtime = _obj(report.get("runtime"), "runtime")
    if runtime.get("excluded_from_metrics") is not True or runtime.get("sleep_based_success") is not False:
        _fail("runtime must exclude mechanics and forbid sleep-based success")

    results = report.get("results")
    if not isinstance(results, list) or len(results) != 49:
        _fail("report must contain exactly 49 result rows")
    passed = [r for r in results if isinstance(r, dict) and r.get("state") == "passed"]
    unrun = [r for r in results if isinstance(r, dict) and r.get("state") == "unrun"]
    if len(passed) != 44 or len(unrun) != 5:
        _fail("report must contain 44 passed viewport rows and 5 unrun rows")

    seen: set[tuple[str, str]] = set()
    for result in passed:
        jid, viewport = result.get("id"), result.get("viewport")
        if jid not in by_id or by_id[jid].get("state") != "executed":
            _fail(f"passed result references non-executed journey: {jid}")
        if viewport not in by_id[jid].get("viewports", []):
            _fail(f"{jid}: result has undeclared viewport {viewport}")
        key = (jid, viewport)
        if key in seen:
            _fail(f"duplicate result {jid}/{viewport}")
        seen.add(key)
        if "label" in result and result["label"] != by_id[jid]["label"]:
            _fail(f"{jid}: result label does not match manifest")
        actions = _obj(result.get("actions"), f"{jid}: actions")
        if set(actions) != REQUIRED_ACTION_KEYS:
            _fail(f"{jid}: action metrics keys mismatch")
        total = 0
        for key_name in REQUIRED_ACTION_KEYS - {"typed_fields"}:
            _nonnegative_int(actions[key_name], f"{jid}: {key_name}")
            total += actions[key_name]
        if total == 0:
            _fail(f"{jid}: vacuous action metrics")
        if not isinstance(actions["typed_fields"], list) or any(not isinstance(v, str) or not v for v in actions["typed_fields"]):
            _fail(f"{jid}: typed_fields must be a string list")
        assertions = _obj(result.get("assertions"), f"{jid}: assertions")
        if set(assertions) != REQUIRED_ASSERTION_KEYS:
            _fail(f"{jid}: assertion keys mismatch")
        for key_name in REQUIRED_ASSERTION_KEYS:
            if not isinstance(assertions[key_name], int) or isinstance(assertions[key_name], bool) or assertions[key_name] <= 0:
                _fail(f"{jid}: visible/canonical assertions must be positive")
        if "duration_ms" in result:
            _nonnegative_int(result["duration_ms"], f"{jid}: duration_ms")

    for journey in executed:
        for viewport in journey["viewports"]:
            if (journey["id"], viewport) not in seen:
                _fail(f"missing result {journey['id']}/{viewport}")
    if any(key[0] not in {j["id"] for j in executed} for key in seen):
        _fail("unexpected passed result")

    unrun_ids: set[str] = set()
    for result in unrun:
        jid = result.get("id")
        if jid in unrun_ids or jid not in by_id or by_id[jid].get("state") != "unverified":
            _fail(f"invalid unrun result {jid}")
        unrun_ids.add(jid)
        if set(result) != {"id", "state", "reason"} or result.get("reason") != by_id[jid].get("unrun_reason"):
            _fail(f"{jid}: unrun reason mismatch")
    if unrun_ids != {j["id"] for j in unverified}:
        _fail("unverified journeys do not have exactly one matching unrun result")

    screenshots = report.get("screenshots")
    if screenshots != EXPECTED_SCREENSHOTS:
        _fail("screenshots must be exactly the two required paths")
    health = report.get("health")
    if not isinstance(health, list) or len(health) != 4:
        _fail("health must contain host and joiner for both viewports")
    expected_health = {
        ("desktop", "host"),
        ("desktop", "joiner"),
        ("narrow-sync", "host"),
        ("narrow-sync", "joiner"),
    }
    seen_health: set[tuple[str, str]] = set()
    for entry in health:
        item = _obj(entry, "health entry")
        identity = (item.get("viewport"), item.get("replica"))
        if identity not in expected_health or identity in seen_health:
            _fail("invalid health identity")
        seen_health.add(identity)
        for field in ("console_errors", "console_warnings", "page_errors", "csp_violations", "external_requests"):
            values = item.get(field)
            if not isinstance(values, list) or any(not isinstance(v, str) for v in values):
                _fail(f"health {field} must be a string list")
        if any(v != ALLOWED_401 for v in item["console_errors"]):
            _fail("unexpected console error")
        if item["console_warnings"] or item["page_errors"] or item["csp_violations"] or item["external_requests"]:
            _fail("browser health contains a failure or external request")
    if seen_health != expected_health:
        _fail("health must cover each viewport/replica pair exactly once")
    if report.get("health_failures") != []:
        _fail("health_failures must be empty")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", nargs="?", default="performance/v0.7-g18e/JOURNEYS.json")
    parser.add_argument("report", nargs="?", default="performance/v0.7-g18e/REPORT.json")
    args = parser.parse_args()
    try:
        validate(json.loads(Path(args.manifest).read_text()), json.loads(Path(args.report).read_text()))
    except (OSError, json.JSONDecodeError, EvidenceError) as exc:
        print(f"G18e evidence invalid: {exc}")
        return 1
    print("G18e evidence valid: 37 journeys, 44 viewport executions, 5 unrun")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
