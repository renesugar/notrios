#!/usr/bin/env python3
"""Re-derive J4's surface review and check the properties that make it useful.

The report is claims about files that change whenever a route is added, so it is
derived on every run rather than read. These are the properties that would make
the review worthless if they stopped holding.
"""
from __future__ import annotations

import importlib.util
import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class EvidenceError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    spec = importlib.util.spec_from_file_location("j4", HERE / "review_surfaces.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules["j4"] = module
    spec.loader.exec_module(module)

    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v10.j4-surface-review.v1", "wrong schema")

    derived = module.build()
    for field in ("counts", "members", "referred_to_by_nothing"):
        require(derived[field] == report[field],
                f"{field} is stale; run python3 performance/v1.0-j4/review_surfaces.py")

    # The review covers the frozen surface exactly. A member added to the freeze
    # and not to this report is a promise nobody reviewed, which is the failure
    # this item exists to prevent.
    frozen = module.frozen()
    for surface in ("rest", "mcp"):
        reviewed = {row["member"] for row in report["members"] if row["surface"] == surface}
        require(reviewed == set(frozen[surface]["members"]),
                f"the {surface} review and the I8 freeze cover different members")
        require(report["counts"][surface]["members"] == frozen[surface]["count"],
                f"the {surface} count disagrees with the freeze")

    # Every MCP tool the server advertises is tested. D2 found three that were
    # not, and a gate is the only thing that stops a fourth.
    untested = sorted(row["member"] for row in report["members"]
                      if row["surface"] == "mcp" and not row["tested"])
    require(not untested,
            "advertised MCP tools with no test naming them anywhere: " + ", ".join(untested))

    # Nothing in the frozen surface is referred to by nothing at all.
    require(not report["referred_to_by_nothing"],
            "frozen members that no document, example or test refers to: "
            + ", ".join(report["referred_to_by_nothing"]))

    # Every REST route but the web-interface root is in the machine-readable
    # contract. The exemption is named rather than counted, so a second one
    # cannot hide inside the number.
    undescribed = sorted(row["member"] for row in report["members"]
                         if row["surface"] == "rest" and not row["described"])
    require(undescribed == ["GET /"],
            f"routes missing from api/openapi.yaml: {undescribed}; only the web-interface "
            "root is a reviewed exemption (D3)")

    # The decisions are a document a person wrote, so what is checked is that it
    # still addresses each one rather than that it says anything in particular.
    decisions = (HERE / "DECISIONS.md").read_text(encoding="utf-8")
    for marker in ("## D1", "## D2", "## D3", "## D4", "## D5",
                   "## What this review did not settle"):
        require(marker in decisions, f"DECISIONS.md no longer contains {marker}")
    require(re.search(r"D1.*owner decides", decisions),
            "D1 no longer records that the carrier path shape is the owner's decision")

    counts = report["counts"]
    print(f"J4 surface review valid: rest {counts['rest']['members']} routes "
          f"({counts['rest']['described']} described, {counts['rest']['tested']} named by a "
          f"serving test), mcp {counts['mcp']['members']} tools "
          f"({counts['mcp']['tested']} tested), 5 decisions recorded, 1 left to the owner")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"J4 record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
