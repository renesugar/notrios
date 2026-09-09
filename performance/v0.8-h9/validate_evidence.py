#!/usr/bin/env python3
"""Validate the v0.8 H9 credential-store evidence.

Three things are enforced that a reader cannot check by eye.

Every behaviour is either executed or inspected with a reason, and no
behaviour on a platform this machine does not have claims execution -- the same
rule H8 applies to its matrix, for the same reason: an item that ships a
Windows provider nobody ran must say so.

Every guard the item leans on has a recorded mutation that catches it. A guard
whose mutation is missing is a guard nobody has shown to bite.

And the evidence file itself holds no secret material. That is this item's own
boundary, and a file asserting it is the obvious place for it to be broken.
"""
import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
STATES = {"executed", "inspected"}
# Platforms with no host here. H7 governs when that changes.
UNRUNNABLE = ("windows", "macos")


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def check_no_secret_material(report) -> None:
    """Refuse anything shaped like key material.

    Base64 of a 32-byte key is 44 characters, and of an Ed25519 private key 88.
    Any long unbroken base64-looking run in this file is either a key or a
    mistake, and neither belongs here.
    """
    text = json.dumps(report)
    for candidate in re.findall(r"[A-Za-z0-9+/]{40,}={0,2}", text):
        raise EvidenceError(f"evidence holds a base64-shaped run of {len(candidate)} characters")
    for word in ("BEGIN PRIVATE KEY", "signing_key", "data_key", "group_key"):
        require(word not in text, f"evidence names {word!r}, which suggests it carries material")


def main() -> None:
    report = json.loads((HERE / "RESULTS.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.h9.credential-store.v1", "wrong evidence schema")

    behaviours = report["behaviours"]
    require(behaviours, "no behaviours recorded")
    seen = set()
    executed = inspected = 0
    for behaviour in behaviours:
        identifier = behaviour["id"]
        require(identifier not in seen, f"duplicate behaviour {identifier}")
        seen.add(identifier)
        state = behaviour["state"]
        require(state in STATES, f"{identifier} has unknown state {state!r}")
        require(behaviour.get("detail"), f"{identifier} records no detail")
        if state == "executed":
            executed += 1
            lowered = identifier.lower()
            for platform in UNRUNNABLE:
                require(platform not in lowered,
                        f"{identifier} claims execution on a platform this host does not have")
        else:
            inspected += 1
            require("H7" in behaviour["detail"] or "no " in behaviour["detail"],
                    f"{identifier} is inspected but does not say why")

    mutations = report["mutations"]
    require(mutations, "no mutations recorded")
    for mutation in mutations:
        require(mutation.get("guard") and mutation.get("mutation") and mutation.get("caught_by"),
                f"incomplete mutation entry: {mutation}")

    require(report["boundaries"], "no boundaries recorded")
    for boundary in report["boundaries"]:
        require(boundary.get("held_by"), f"{boundary['id']} names no mechanism")

    require(report["deferred"], "nothing deferred is recorded")
    for item in report["deferred"]:
        require(item.get("reason"), f"{item['id']} is deferred without a reason")

    check_no_secret_material(report)

    # The adopted module must be the one the licence inventory carries, so this
    # file cannot drift from what actually ships.
    inventory = json.loads((ROOT / "performance/v0.7-g20/DEPENDENCY_LICENSES.json")
                           .read_text(encoding="utf-8"))
    modules = inventory["go"]["modules"]
    adopted = report["adopted"]
    key = f"{adopted['module']}@{adopted['version']}"
    require(key in modules, f"{key} is not in the G20 licence inventory")
    require(modules[key] == adopted["license"],
            f"{key} is {modules[key]} in G20 but {adopted['license']} here")
    for brought in adopted["brought_in"]:
        sub = f"{brought['module']}@{brought['version']}"
        require(sub in modules, f"{sub} is not in the G20 licence inventory")
        require(modules[sub] == brought["license"],
                f"{sub} is {modules[sub]} in G20 but {brought['license']} here")

    print(f"H9 evidence valid: {executed} executed, {inspected} inspected, "
          f"{len(mutations)} mutations, {len(report['defects_found'])} defects found, "
          f"{len(report['deferred'])} deferrals.")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H9 evidence invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
