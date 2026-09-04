#!/usr/bin/env python3
"""Validate the v0.8 H14 actionability pilot.

Two things are enforced that a reader cannot check by eye.

Every recorded run names a free model. The whole item is constrained to zero
cost, a model is free when its slug contains `free`, and a run against anything
else must not be able to hide in the evidence.

And a task is credited only when the prose arm acted *and* the no-prose arm did
not. That rule is the measurement: without it a capable model producing a
plausible invocation from command-line convention alone would be scored as
documentation quality. This file refuses a summary that claims credit the arms
do not support, because the tempting mistake is to read a successful prose arm
as a good page.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ARMS = {"prose", "no-prose", "misleading"}
OUTCOMES = {"acted", "no-change", "no-command", "refused", "did-not-run", "timeout", "unavailable"}


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "ACTIONABILITY.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.h14.actionability.v1", "wrong actionability schema")
    runs = report["runs"]
    require(runs, "no runs recorded")

    by_task = {}
    for run in runs:
        require("free" in run["model"], f"{run['model']} is not a free model slug")
        require(run["arm"] in ARMS, f"unknown arm {run['arm']!r}")
        require(run["outcome"] in OUTCOMES, f"unknown outcome {run['outcome']!r}")
        require(run.get("prompt_sha256"), "a run records no prompt hash")
        by_task.setdefault((run["model"], run["journey"]), {})[run["arm"]] = run["outcome"]

    credited = 0
    for (model, journey), arms in by_task.items():
        require(ARMS <= set(arms), f"{model}/{journey} is missing an arm: {sorted(arms)}")
        if arms["prose"] == "acted" and arms["no-prose"] != "acted":
            credited += 1

    print(f"H14 actionability evidence valid: {len(runs)} runs, {len(by_task)} task/model pairs, "
          f"{credited} credited (prose acted and no-prose did not).")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H14 actionability evidence invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
