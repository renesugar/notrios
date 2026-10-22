#!/usr/bin/env python3
"""Validate the deterministic G18a investigation evidence."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

from build_inventory import ROOT, build


HERE = Path(__file__).resolve().parent
MODULE = "github.com/renesugar/notrios"
ANCHOR_RE = re.compile(r"^(go|ts):([^#]+)#(.+)$")
DIRECTIVE_RE = re.compile(r"^//notrios:(doc|help|enumerates|claim)(?:\s+(.+))?$")


class EvidenceError(ValueError):
    pass


def load(name: str) -> dict:
    with (HERE / name).open(encoding="utf-8") as handle:
        return json.load(handle)


def go_anchor_exists(location: str, symbol: str) -> bool:
    if location == MODULE:
        rel = Path(".")
    elif location.startswith(MODULE + "/"):
        rel = Path(location[len(MODULE) + 1 :])
    else:
        return False
    directory = ROOT / rel
    if not directory.is_dir():
        return False
    source = "\n".join(
        path.read_text(encoding="utf-8")
        for path in sorted(directory.glob("*.go"))
        if not path.name.endswith("_test.go")
    )
    method = re.fullmatch(r"\((\*?)([A-Za-z_]\w*)\)\.([A-Za-z_]\w*)", symbol)
    if method:
        pointer, receiver, name = method.groups()
        receiver_pattern = rf"\(\s*\w+\s+{'\\*' if pointer else ''}{re.escape(receiver)}(?:\[[^]]+\])?\s*\)"
        return re.search(rf"(?m)^func\s+{receiver_pattern}\s+{re.escape(name)}\s*\(", source) is not None
    name = re.escape(symbol)
    patterns = [
        rf"(?m)^func\s+{name}\s*\(",
        rf"(?m)^type\s+{name}\b",
        rf"(?m)^(?:const|var)\s+{name}\b",
        rf"(?ms)^(?:const|var)\s*\(.*?^\s*{name}\b",
    ]
    return any(re.search(pattern, source) for pattern in patterns)


def ts_anchor_exists(location: str, symbol: str) -> bool:
    path = ROOT / location
    if not path.is_file() or not re.fullmatch(r"[A-Za-z_$][\w$]*", symbol):
        return False
    source = path.read_text(encoding="utf-8")
    name = re.escape(symbol)
    patterns = [
        rf"(?m)^(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+{name}\s*\(",
        rf"(?m)^(?:export\s+)?(?:default\s+)?(?:class|interface|type|enum)\s+{name}\b",
        rf"(?m)^(?:export\s+)?(?:const|let|var)\s+{name}\b",
    ]
    return sum(bool(re.search(pattern, source)) for pattern in patterns) == 1


def validate_anchor(anchor: str) -> None:
    match = ANCHOR_RE.fullmatch(anchor)
    if not match:
        raise EvidenceError(f"malformed source-symbol anchor: {anchor}")
    kind, location, symbol = match.groups()
    exists = go_anchor_exists(location, symbol) if kind == "go" else ts_anchor_exists(location, symbol)
    if not exists:
        raise EvidenceError(f"dangling source-symbol anchor: {anchor}")


def parse_directive_groups(source: str) -> list[dict[str, object]]:
    """Parse the fixture subset and enforce group-level G18a invariants."""
    groups: list[dict[str, object]] = []
    pending: list[str] = []
    ids: set[str] = set()
    for raw in source.splitlines() + [""]:
        line = raw.strip()
        if line.startswith("//"):
            pending.append(line)
            continue
        declaration = re.match(r"^(?:func|var|const|type)\s+([A-Za-z_]\w*)", line)
        directives = []
        for comment in pending:
            match = DIRECTIVE_RE.fullmatch(comment)
            if match:
                directives.append((match.group(1), (match.group(2) or "").split()))
        if directives:
            if not declaration:
                if line:
                    raise EvidenceError("directive group is not attached to a declaration")
                # A blank line terminates an unattached comment group.
                raise EvidenceError("dangling directive group")
            docs = [args for kind, args in directives if kind == "doc"]
            audiences = {args[0] for args in docs if args}
            if len(audiences) > 1:
                raise EvidenceError("mixed audiences in one doc group")
            if len(docs) != 1 or len(docs[0]) != 2:
                raise EvidenceError("a directive group requires exactly one well-formed doc directive")
            audience, fragment_id = docs[0]
            if audience not in {"user", "api", "maintainer"}:
                raise EvidenceError(f"unknown audience: {audience}")
            if fragment_id in ids:
                raise EvidenceError(f"duplicate fragment id: {fragment_id}")
            ids.add(fragment_id)
            for kind, args in directives:
                if kind == "help" and audience != "user":
                    raise EvidenceError("help directive requires user audience")
            groups.append({"declaration": declaration.group(1), "audience": audience,
                           "fragment_id": fragment_id, "directives": directives})
        if line and not line.startswith("//"):
            pending = []
        elif not line:
            pending = []
    return groups


def _function_body(source: str, name: str) -> str:
    match = re.search(rf"\bfunction\s+{re.escape(name)}\s*\([^)]*\)\s*\{{", source)
    if not match:
        raise EvidenceError(f"missing TS/TSX function: {name}")
    start = match.end()
    depth = 1
    for index in range(start, len(source)):
        if source[index] == "{":
            depth += 1
        elif source[index] == "}":
            depth -= 1
            if depth == 0:
                return source[start:index]
    raise EvidenceError(f"unterminated TS/TSX function: {name}")


def direct_ts_callees(source: str, name: str) -> set[str]:
    body = _function_body(source, name)
    declared = set(re.findall(r"(?:export\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(", source))
    calls = set(re.findall(r"\b([A-Za-z_$][\w$]*)\s*\(", body))
    return calls & declared


def validate_inventory() -> dict:
    actual = load("INVENTORY.json")
    expected = build()
    if actual != expected:
        raise EvidenceError("INVENTORY.json is stale; run build_inventory.py --write and review the diff")
    # 15 -> 16 in v0.8 H14 slice B: docs/features.md. Pinned by identity rather
    # than as a floor, because a page arriving or leaving without anyone saying
    # so is exactly what this count exists to catch -- helpdocs seeds every
    # Markdown file under docs/, so a stray file becomes a Help note.
    # 18 -> 19 in v1.0 J12: docs/configuration.md. Pinned rather than counted so
    # that a page reaching the published site and the Help notebook is a
    # decision somebody made and recorded here.
    if len(actual["documents"]) != 19:
        raise EvidenceError("published/Help page count is not 19")
    grades = actual["grade_baseline"]
    if not grades["reconciles"] or sum(grades["totals"].values()) != grades["denominator"]:
        raise EvidenceError("grade totals do not reconcile")
    for document in actual["documents"]:
        validate_anchor(document["owner"])
        for section in document["sections"]:
            validate_anchor(section["owner"])
    for surface in actual["surfaces"]:
        validate_anchor(surface["owner"])
        for journey in surface.get("journeys", []):
            validate_anchor(journey["owner"])
    return actual


def validate_calibration() -> None:
    calibration = load("CALIBRATION.json")
    labels = set(calibration["labels"])
    expected_labels = {"supported", "contradicted", "not-determinable"}
    if labels != expected_labels:
        raise EvidenceError("calibration labels changed")
    seen = set()
    for case in calibration["cases"]:
        if case["id"] in seen:
            raise EvidenceError(f"duplicate calibration id: {case['id']}")
        seen.add(case["id"])
        if case["expected"] not in labels:
            raise EvidenceError(f"unknown verdict: {case['expected']}")
        validate_anchor(case["anchor"])
        for callee in case.get("direct_callees", []):
            validate_anchor(callee)
    if not any(case["origin"] == "controlled-negation" for case in calibration["cases"]):
        raise EvidenceError("calibration lacks a negation case")
    if not any(case["expected"] == "not-determinable" and "rationale" in case["id"]
               for case in calibration["cases"]):
        raise EvidenceError("calibration lacks an unreviewable rationale case")


def main() -> int:
    validate_inventory()
    validate_calibration()
    fixture = (HERE / "fixtures/directives.go.fixture").read_text(encoding="utf-8")
    groups = parse_directive_groups(fixture)
    if len(groups) != 2:
        raise EvidenceError(f"directive fixture yielded {len(groups)} groups, want 2")
    tsx = (HERE / "fixtures/anchors.tsx.fixture").read_text(encoding="utf-8")
    if direct_ts_callees(tsx, "RootJourney") != {"directStep"}:
        raise EvidenceError("direct-callee scope included a transitive TSX callee")
    print("G18a evidence valid: 15 pages, 201 sections, source anchors, grades, and 8 calibration cases.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except EvidenceError as error:
        print(f"G18a evidence invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
