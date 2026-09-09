#!/usr/bin/env python3
"""Validate the I3 installer matrix record, offline and without Docker.

`make validate` runs everywhere, including where there is no container runtime,
so this checks the *record* rather than re-running the matrix: that every
scenario the runner produced is in the report, that they all passed, that the
fresh install was measured on a pristine image, and that the report still says
what it did not verify.

That last one is the point of having a validator at all. A record whose limits
quietly disappear is worse than no record: it reads as a stronger claim than
anyone made. H6a's own honesty is the reason this item existed to be done, so
the same honesty is gated here rather than trusted.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PRISTINE = "ubuntu:24.04"
FRESH_INSTALL = "01-fresh-install"


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v09.i3-installer-matrix.v1", "wrong report schema")

    results = [json.loads(line) for line in
               (HERE / "RESULTS.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
    require(results, "no scenario results recorded")

    # The report is a rendering of the run, not a second account of it.
    recorded = {r["scenario"]: r for r in results}
    reported = {s["id"]: s for s in report["scenarios"]}
    require(set(recorded) == set(reported),
            f"the report and the run disagree about which scenarios exist: "
            f"{sorted(set(recorded) ^ set(reported))}")
    for name, run in recorded.items():
        require(reported[name]["status"] == run["status"], f"{name}: status differs from the run")
        require(reported[name]["image"] == run["image"], f"{name}: image differs from the run")
        require(reported[name]["observations"] == run["observations"],
                f"{name}: observations differ from the run")

    failed = sorted(name for name, run in recorded.items() if run["status"] != "pass")
    require(not failed, f"scenarios did not pass: {', '.join(failed)}")
    require(report["counts"]["scenarios"] == len(results), "the scenario count disagrees")
    require(report["counts"]["passed"] == len(results), "the passed count disagrees")

    # The fresh install is the one that measures whether the declared
    # dependencies are sufficient, which it cannot do on an image where they are
    # already installed.
    require(FRESH_INSTALL in recorded, f"{FRESH_INSTALL} is missing from the run")
    require(recorded[FRESH_INSTALL]["image"] == PRISTINE,
            f"{FRESH_INSTALL} must run on a pristine {PRISTINE}, not {recorded[FRESH_INSTALL]['image']}")
    for other, run in recorded.items():
        if other != FRESH_INSTALL:
            require(run["image"] != PRISTINE,
                    f"{other} claims the pristine image; only the fresh install should")

    # Each scenario must have reported something it saw. A scenario that passes
    # while observing nothing is a scenario that asserted nothing.
    for name, run in recorded.items():
        require(run["observations"], f"{name} passed without recording a single observation")

    limits = report["not_verified"]
    require(len(limits) >= 5, "the report has stopped saying what it did not verify")
    text = " ".join(limits).lower()
    for subject in ("arm64", "gui", "kernel"):
        require(subject in text, f"the limits no longer mention {subject}")

    require(report["level_reached"]["level"] == 3, "the claimed ladder level moved")
    require(report["packages"], "no packages recorded")

    print(f"I3 installer matrix valid: {report['counts']['passed']}/{report['counts']['scenarios']} "
          f"scenarios passed, fresh install on {PRISTINE}, "
          f"{len(limits)} limits recorded")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError, OSError) as error:
        print(f"I3 installer matrix invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
