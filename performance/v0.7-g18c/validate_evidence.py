#!/usr/bin/env python3
"""Validate the checked G18c report and mutation evidence."""

from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / "performance/v0.7-g18c"


def load(name: str):
    return json.loads((EVIDENCE / name).read_text(encoding="utf-8"))


def main() -> None:
    expected = load("REPORT.json")
    mutations = load("MUTATION_MATRIX.json")
    inventory = json.loads(
        (ROOT / "performance/v0.7-g18a/INVENTORY.json").read_text(encoding="utf-8")
    )
    registry = json.loads(
        (ROOT / "docs/docaudit/registry.json").read_text(encoding="utf-8")
    )

    manual_sections = sum(len(document["sections"]) for document in inventory["documents"])
    assert expected["schema"] == "notrios.docaudit.report.v1"
    assert expected["manual_sections"] == 199
    assert manual_sections >= expected["manual_sections"]
    assert expected["claims"] == len(registry["claims"]) == 4
    # The frozen report records 131 executable-shaped fences, which is what
    # G18c found. The live registry only has to still contain them: pinning it
    # to exactly 131 made this assertion unsurvivable, because every documented
    # example added afterwards moves the live count and none of them invalidate
    # the evidence. `manual_sections` above already takes that view, and this
    # now matches it -- a shrinking registry is still caught.
    #
    # It went stale for two commits without anyone noticing because
    # validate-scaffold.sh only syntax-checks this file; package_release.sh was
    # the only thing that ran it, so the failure surfaced at release time. It is
    # run by `make validate` now.
    assert expected["executables"] == 131
    assert len(registry["executables"]) >= expected["executables"]
    assert expected["journeys"] == len(registry["journeys"]) == 9
    assert expected["fragments"] == 12
    assert sum(expected["counts"].values()) == expected["denominator"] == 351
    assert len(expected["topics"]) == len(inventory["documents"]) == 15
    surfaces = {surface["id"]: surface for surface in expected["surfaces"]}
    assert len(surfaces) == len(inventory["surfaces"]) == 8
    assert surfaces["openapi"]["count"] == 109
    assert surfaces["openapi"]["operation_ids"] == 0
    assert surfaces["mcp_tools"]["count"] == 46
    assert surfaces["mcp_resources"]["count"] == 0
    assert len(mutations["go_audit_cases"]) == 20
    assert len(mutations["typescript_cases"]) == 8

    # REPORT.json is the frozen G18c baseline. Later slices may add documented
    # sections, and G18d deliberately changes executable grades while
    # preserving all 131 identities. Current-state freshness belongs to the
    # inventory plus later validators; the baseline must remain a covered
    # subset rather than being rewritten as if G18c measured future sections.
    print(
        "G18c evidence valid: "
        f"{expected['denominator']} units; "
        f"{expected['counts']['generated']} generated, "
        f"{expected['counts']['claimed']} claimed, "
        f"{expected['counts']['executed']} executed, and "
        f"{expected['counts']['unverified']} unverified."
    )


if __name__ == "__main__":
    main()
