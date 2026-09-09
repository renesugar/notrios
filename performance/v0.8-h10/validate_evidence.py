#!/usr/bin/env python3
"""Validate the v0.8 H10 Wails v3 spike record.

The spike needs Xvfb, two webview stacks and a frontend build, so it is not in
`make validate`. What is checked here is the committed record: that it reaches a
decision, that the decision is one of the three the item allows, that it says
what it did not measure, and — the part worth automating — that the boundary it
was given still holds.

That last one is not a claim about a file. The production module must not have
learned that v3 exists, and this reads go.mod rather than the report's opinion
of it: a spike that quietly added a dependency would otherwise pass on its own
say-so.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]

ALLOWED_RECOMMENDATIONS = {"migrate", "defer", "reject"}

# The capabilities the production shell uses. A spike that compared fewer than
# these compared something smaller than Notrios.
REQUIRED_CAPABILITIES = {
    "asset server", "bound objects", "menu", "accelerators", "window operations",
    "close veto", "question dialog", "directory picker", "JS calling convention",
}


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.h10.wails3-decision.v1", "wrong spike schema")

    require(report["recommendation"] in ALLOWED_RECOMMENDATIONS,
            f"{report['recommendation']!r} is not one of {sorted(ALLOWED_RECOMMENDATIONS)}")
    require(report.get("recommended_timing"), "a recommendation with no timing is half an answer")

    compared = {item["capability"] for item in report["capabilities"]}
    missing = REQUIRED_CAPABILITIES - compared
    require(not missing, f"the spike did not compare {sorted(missing)}")
    for item in report["capabilities"]:
        require(item.get("v2") and item.get("v3") and item.get("cost"),
                f"{item['capability']} is compared without saying what it costs")

    require(report.get("what_this_did_not_measure"),
            "a spike that lists no gaps is a spike nobody looked hard at")
    require(report.get("risks"), "no risks recorded")

    measurements = report["measurements"]
    require(measurements["windows_opened"]["v3"] == "yes",
            "the prototype never opened a window, so nothing below it was observed")
    require(measurements["windows_opened"]["v2"] == "yes",
            "the v2 baseline never opened a window, so there was nothing to compare against")

    # The boundary, read from the repository rather than from the report.
    gomod = (ROOT / "go.mod").read_text(encoding="utf-8")
    require("wailsapp/wails/v3" not in gomod,
            "the production module requires wails/v3; the spike was supposed to stay isolated")
    require("wailsapp/wails/v2" in gomod,
            "the production module no longer requires wails/v2, so the spike removed what it was told not to")
    prototype_mod = HERE / "prototype" / "go.mod"
    require(prototype_mod.is_file(),
            "the prototype has no module of its own, so it is not isolated from the production build")

    print(f"H10 spike valid: recommendation={report['recommendation']}, "
          f"{len(compared)} capabilities compared, production still on "
          f"{measurements['wails']['production']}")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError) as error:
        print(f"H10 spike invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
