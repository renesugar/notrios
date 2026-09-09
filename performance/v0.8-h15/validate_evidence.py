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

from interface_signature import interface_signature

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
# 52 -> 53 when the GUI gained the Import/Export control.
# 53 -> 58 when jobs and a kept search were seeded: Retry on a cancelled sync
# job, Cancel on one still queued, the keep-search control, and three controls
# belonging to the notebook a seeded import created. Only one of those five is
# a new feature; the rest were always there and had nothing to render for.
# 58 -> 59: the library-health button in the header. It was added in the same
# item as this count and the crawl was never re-run, so the pinned inventory
# went on passing at 58 while the interface had 59 -- found the first time the
# interface signature below was checked, which is the whole argument for having
# it. A number nobody re-measures describes the interface it was taken from.
# 59 -> 79 in v0.8 H28, which named controls rather than adding any. Naming
# both splits and merges: the sync centre's eight tabs were one shape and are
# now eight names, while the two shapes a search result had -- selected and not
# -- became one control, because a test id is identity and a class is not.
# 79 -> 89 in v0.8 H29, which added a capability rather than a name: ticking a
# result starts a selection, and the editor and preview are replaced by the
# operations that apply to a set. Nine of the ten are that panel; the tenth is
# the tick box itself, which appears on every result in every state that lists
# one.
EXPECTED_CONTROLS = 89
# 16 -> 17 in v0.8 H29: `selection`. The panel does not exist until something is
# ticked, so without a state that ticks one the crawl would have measured an
# interface that cannot act on a set -- the same mistake the six-view crawl made
# about attachments and remote media, in a new place.
EXPECTED_STATES = 17

# How much of the interface can be pointed at by name.
#
# This is the number H15-G was blocked on. A journey against an unnamed control
# has to locate it by shape -- the nth button inside the third div -- which is
# the form the catalogue avoids, because it breaks on a layout change that broke
# nothing. So "can this be documented?" is a measurement, and this is it.
#
# 22 of 59 in v0.8 H15, and the 22 were largely the furniture that appears in
# every state; the controls that make a state that state were mostly in the
# other 37. 72 of 79 in v0.8 H28. 82 of 89 in v0.8 H29, whose ten new controls
# all arrived with names -- which is what the gate is for: the cost of naming a
# control is lowest while it is being written.
#
# Pinned exactly rather than as a floor. A floor would let a control be added
# without a name as long as something else gained one, which is the drift this
# exists to catch: the number has to be re-measured, and a re-measurement that
# cannot fail is not one.
EXPECTED_ADDRESSABLE = 82

# The seven that carry no name, and why each is not an omission. Six are
# rendered by md-editor-rt -- the wrapper it puts around the notebook and tag
# triggers, and four of its own menu items -- so the outer element is not ours
# to name; both triggers carry a test id on the element inside it, which is what
# a journey clicks. The seventh is an `<a>` in a note's own body: rendered note
# content is not a control of the interface, and naming it would mean naming
# whatever somebody wrote.
#
# Written down so that "seven remain" is a decision with reasons rather than a
# remainder nobody looked at. Anything else appearing here is a control that
# needs a name.
UNADDRESSABLE_BY_ORIGIN = {
    "shape:a|||",
    "shape:button||md-editor-disabled.md-editor-toolbar-item|md-editor-toolbar-left",
    "shape:button||md-editor-toolbar-item|md-editor-toolbar-left",
    "shape:li|menuitem|md-editor-menu-item.md-editor-menu-item-image|md-editor-menu",
    "shape:li|menuitem|md-editor-menu-item.md-editor-menu-item-katex|md-editor-menu",
    "shape:li|menuitem|md-editor-menu-item.md-editor-menu-item-mermaid|md-editor-menu",
    "shape:li|menuitem|md-editor-menu-item.md-editor-menu-item-title|md-editor-menu",
}

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
    # Retry renders only on a failed or cancelled job, so this entry is really
    # a guard on the seeding: if nothing arranges a cancelled sync job, the
    # control vanishes and the jobs row goes back to looking absent. It was
    # recorded as unmeasured for exactly that reason until one was seeded.
    "sync-overview": "testid:sync-job-retry",
    # The panel replaces the editor and the preview, so this state is also the
    # only one where those two are absent. Naming a control it alone reveals
    # keeps a step that ticked nothing from passing quietly. Not the panel
    # itself: the crawl collects interactive elements, and a <section> is not
    # one however well named -- so the control that leaves the selection stands
    # for it, because it is the one thing the panel always offers.
    "selection": "testid:selection-clear",
}

# Reached, and deliberately expected to contribute nothing of their own. These
# four tabs render an empty state in a library with no conflicts, no missing
# resources, no retention plan and no repairs to make. That is a limit of what
# is seeded rather than of where the crawl goes, and it is written down so the
# emptiness is a known fact rather than a silent one. Seeding a conflict is a
# larger job than seeding a note and belongs with the work that needs it.
KNOWN_EMPTY_STATES = {"sync-retention", "sync-attachments", "sync-conflicts", "sync-repairs"}

# Controls that must never come back enabled, because this crawl is a browser.
# Importing, exporting and taking snapshots name a folder on the machine running
# the library, and they reach the core through the Wails bridge, which is bound
# only when the window's own process owns the store. The decision was to render
# the control and disable it with the reason rather than hide it, so the crawl
# should find it every time and find it disabled every time. An enabled one here
# would mean the gate had broken open and a browser was being offered an
# operation it cannot perform.
MUST_BE_DISABLED_IN_A_BROWSER = {"testid:transfer-header-button"}

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
    require(report["schema"] == "notrios.h15.gui-controls.v3", "wrong control inventory schema")

    # The crawl is a report rather than a build gate, and this is what keeps
    # that from being a hole. The inventory is a committed file describing an
    # interface that can move without it: a control added and never crawled
    # would leave every check below passing against a measurement of something
    # that no longer exists. The signature is of the things that decide what the
    # crawl would find, so prose and styling do not trip it and a new control
    # does. It cannot say what changed -- only the crawl can -- so it says to
    # run the crawl.
    recorded = report.get("interface_signature", "")
    require(recorded, "the inventory records no interface signature; re-run the crawl to add one")
    current = interface_signature(HERE.parents[1] / "web" / "src")
    require(recorded == current,
            "the interface has changed since its controls were counted "
            f"(recorded {recorded[:12]}, now {current[:12]}). Re-run the crawl with "
            "NOTRIOS_GUI_CONTROLS_RUN=1 go test ./cmd/notriosctl -run TestGUIControlInventory, "
            "and update EXPECTED_CONTROLS with what was added or removed.")

    states = report["states"]
    require(len(states) == EXPECTED_STATES, f"{len(states)} states crawled, expected {EXPECTED_STATES}")
    missed = [f"{state['id']} ({state.get('error', 'no reason recorded')})"
              for state in states if not state.get("reached")]
    require(not missed, f"the crawl never reached {missed}; those controls are missing from the inventory")

    controls = report["controls"]
    require(len(controls) == EXPECTED_CONTROLS,
            f"{len(controls)} controls found, expected {EXPECTED_CONTROLS}. If the interface really "
            "changed, update EXPECTED_CONTROLS and say what was added or removed.")

    # Recomputed here rather than believed. The crawl writes the summary and
    # this reads the controls, so a summary that disagreed with the list it
    # summarises would be caught instead of pinned.
    summary = report.get("summary", {})
    addressable = [control for control in controls if control.get("testid")]
    unaddressable = [control["id"] for control in controls if not control.get("testid")]
    require(summary.get("controls") == len(controls)
            and summary.get("addressable") == len(addressable)
            and summary.get("unaddressable") == len(unaddressable),
            f"the recorded summary {summary} does not describe the {len(controls)} controls beneath it")
    require(len(addressable) == EXPECTED_ADDRESSABLE,
            f"{len(addressable)} of {len(controls)} controls carry a name, expected "
            f"{EXPECTED_ADDRESSABLE}. A control added without a data-testid cannot be named by a "
            "journey; give it one, or update EXPECTED_ADDRESSABLE and say why it cannot have one.")
    unnamed = set(unaddressable) - UNADDRESSABLE_BY_ORIGIN
    require(not unnamed,
            f"these controls carry no name and no recorded reason: {sorted(unnamed)}. Add a "
            "data-testid named for what the control does, or record it in UNADDRESSABLE_BY_ORIGIN "
            "with the reason it cannot have one.")

    identities = set()
    for control in controls:
        require(control["id"] not in identities, f"duplicate control {control['id']}")
        identities.add(control["id"])
        require(control.get("instances", 0) > 0, f"{control['id']} was recorded with no instances")
        require(control.get("states"), f"{control['id']} appears in no state")

    for control_id in MUST_BE_DISABLED_IN_A_BROWSER:
        control = next((item for item in controls if item["id"] == control_id), None)
        require(control is not None,
                f"{control_id} is gone; it should be present and disabled, not missing")
        require(control.get("always_disabled"),
                f"{control_id} was found enabled in a browser, where the native bridge it needs "
                "is not bound")

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
