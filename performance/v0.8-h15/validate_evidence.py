#!/usr/bin/env python3
"""Validate the v0.8 H15 GUI control inventory.

The GUI was the one adapter onto Notrios that could not be counted. The command
line, REST and MCP are enumerated from source with pinned counts that fail when
they move; the GUI column of the capability table was filled in from whichever
journeys had been written, which understated it with nothing able to notice.

This holds the measurement to the same standard. Two things are enforced that a
reader cannot check by eye: every state the crawl set out to reach was actually
reached, because a state that silently failed would remove controls from the
inventory rather than fail anything; and the control count matches what was
recorded, so the interface gaining or losing a control is a decision somebody
makes rather than a number that drifts.

The word is "state" rather than "view" for a reason worth keeping. The first
version of this crawl visited six views, found no delete, restore, purge,
remote-media or attachment controls, and that number was then read as evidence
that the GUI lacked those capabilities. It had all of them. They live in states
the crawl never entered -- a note open rather than a notebook listed, a note in
the Trash, the sync centre on a tab other than the one it opens on -- so the
interface was measured in the state that shows the fewest controls.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent

# Pinned by measurement, not by expectation. Identity comes from a test id where
# one exists and from the element's shape otherwise, never from its text: a list
# of search results renders one control per note, and keying on the label would
# make this number grow with the library rather than describe the interface.
#
# 26 -> 29 when the GUI gained tagging: the trigger, the input and the add
# button. The trigger only appeared once it was given role="button" -- the crawl
# could not see a bare span, for the same reason a screen reader could not.
# 29 -> 52 when the crawl grew from six views to sixteen states. Nothing was
# added to the interface to cause that: 23 controls were always there and were
# never looked at, including delete, restore, purge, localize, the attachment
# upload field, and everything on seven of the sync centre's eight tabs.
EXPECTED_CONTROLS = 52
EXPECTED_STATES = 16

# Why each state was added, as something that can fail. A state that is reached
# but shows nothing new is the signature of a step that clicked something other
# than what it named -- which is exactly what happened on the first widened run,
# where a locator written from a `title` attribute missed all eight sync states
# and the run still exited 0. Naming the control each state exists to reveal
# turns that into a failure instead of a quieter inventory.
STATE_EVIDENCE = {
    "note-open": "testid:delete-button",
    "note-trashed": "testid:restore-button",
    "note-remote-media": "testid:localize-button",
}

# Reached, and deliberately expected to contribute nothing of their own. These
# four tabs render an empty state in a library with no conflicts, no missing
# resources, no retention plan and no repairs to make. That is a limit of what
# is seeded rather than of where the crawl goes, and it is written down so the
# emptiness is a known fact rather than a silent one. Seeding a conflict is a
# larger job than seeding a note and belongs with the work that needs it.
KNOWN_EMPTY_STATES = {"sync-retention", "sync-attachments", "sync-conflicts", "sync-repairs"}

# Reached, and showing nothing the opening state does not. These are four of the
# six *views* the earlier crawl was built from, and this is the plainest
# evidence that views were the wrong axis: moving between notebooks changes
# which notes are listed and not which controls exist, so four of the six
# measured the same shell over and over. They are kept because a step that
# stopped reaching them would still be worth knowing about, and named here so
# that "contributes nothing" is a recorded fact rather than an oversight.
SHELL_ONLY_STATES = {"all-notes", "help", "trash", "new-note"}


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "GUI_CONTROLS.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.h15.gui-controls.v2", "wrong control inventory schema")

    states = report["states"]
    require(len(states) == EXPECTED_STATES, f"{len(states)} states crawled, expected {EXPECTED_STATES}")
    missed = [f"{state['id']} ({state.get('error', 'no reason recorded')})"
              for state in states if not state.get("reached")]
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
        require(control.get("states"), f"{control['id']} appears in no state")

    reached = {state["id"] for state in states}
    for state_id, control_id in STATE_EVIDENCE.items():
        require(state_id in reached, f"{state_id} is named in STATE_EVIDENCE but was not crawled")
        holder = next((control for control in controls if control["id"] == control_id), None)
        require(holder is not None, f"{control_id} is gone; {state_id} exists to reveal it")
        require(state_id in holder["states"],
                f"{control_id} was not found in {state_id}, the state added to reveal it")

    # Every state that is not a known empty one must show something the opening
    # state does not. Sixteen states that all measure the same shell would be a
    # more expensive way to learn what six views already said.
    opening = {control["id"] for control in controls if "start" in control["states"]}
    for state in states:
        if state["id"] in KNOWN_EMPTY_STATES or state["id"] in SHELL_ONLY_STATES or state["id"] == "start":
            continue
        own = [control["id"] for control in controls
               if state["id"] in control["states"] and control["id"] not in opening]
        require(own, f"{state['id']} showed nothing the opening state does not; "
                     "either its step did not do what it says, or it should not be a state")

    labelled = sum(1 for control in controls if control.get("examples"))
    print(f"H15 GUI control inventory valid: {len(controls)} distinct controls across "
          f"{len(states)} states, {labelled} carrying a label.")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H15 GUI control inventory invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
