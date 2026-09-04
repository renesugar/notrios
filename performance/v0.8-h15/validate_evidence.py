#!/usr/bin/env python3
"""Validate the v0.8 H15 GUI control inventory.

The GUI was the one adapter onto Notrios that could not be counted. The command
line, REST and MCP are enumerated from source with pinned counts that fail when
they move; the GUI column of the capability table was filled in from whichever
journeys had been written, which understated it with nothing able to notice.

This holds the measurement to the same standard. Two things are enforced that a
reader cannot check by eye: every view the crawl set out to visit was actually
reached, because a view that silently failed would remove controls from the
inventory rather than fail anything; and the control count matches what was
recorded, so the interface gaining or losing a control is a decision somebody
makes rather than a number that drifts.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent

# 26 distinct controls across six views. Identity comes from a test id where one
# exists and from the element's shape otherwise, never from its text: a list of
# search results renders one control per note, and keying on the label would
# make this number grow with the library rather than describe the interface.
# 26 -> 29 when the GUI gained tagging: the trigger, the input and the add
# button. The trigger only appeared once it was given role="button" -- the crawl
# could not see a bare span, for the same reason a screen reader could not.
EXPECTED_CONTROLS = 29
EXPECTED_VIEWS = 6


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "GUI_CONTROLS.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.h15.gui-controls.v1", "wrong control inventory schema")

    views = report["views"]
    require(len(views) == EXPECTED_VIEWS, f"{len(views)} views crawled, expected {EXPECTED_VIEWS}")
    missed = [view["id"] for view in views if not view.get("reached")]
    require(not missed, f"the crawl never reached {missed}; those controls are missing from the inventory")

    controls = report["controls"]
    require(len(controls) == EXPECTED_CONTROLS,
            f"{len(controls)} controls found, expected {EXPECTED_CONTROLS}. If the interface really "
            "changed, update EXPECTED_CONTROLS and say what was added or removed.")

    identities = set()
    for control in controls:
        require(control["id"] not in identities, f"duplicate control {control['id']}")
        identities.add(control["id"])
        require(control.get("instances", 0) > 0, f"{control['id']} was recorded with no instances")
        require(control.get("views"), f"{control['id']} appears in no view")

    labelled = sum(1 for control in controls if control.get("examples"))
    print(f"H15 GUI control inventory valid: {len(controls)} distinct controls across "
          f"{len(views)} views, {labelled} carrying a label.")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H15 GUI control inventory invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
