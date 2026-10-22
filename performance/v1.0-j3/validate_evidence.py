#!/usr/bin/env python3
"""Validate the J3 record offline.

The drills install and delete; `make validate` does neither. This checks what
they recorded, and enforces the one property the item exists for: a safeguard
the Make target has and the command does not is the gap, so the record must name
every drill it did not carry over rather than quietly running fewer.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v10.j3-packaged-purge.v1", "wrong report schema")

    drills = [json.loads(line) for line in
              (HERE / "DRILLS.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
    require(drills, "no drills recorded")
    ran = {d["drill"]: d for d in drills}
    reported = {d["id"]: d for d in report["drills"]}
    require(set(ran) == set(reported),
            f"the report and the run disagree: {sorted(set(ran) ^ set(reported))}")
    # Compared by observation, not only by name. Checking the run log for a
    # property while comparing the report by name alone lets a doctored report
    # through: a probe that zeroed the restore count in REPORT.json passed,
    # because the property was read from the run and the report was only checked
    # for having the same drill names.
    for name, run in ran.items():
        require(reported[name]["status"] == run["status"], f"{name}: status differs from the run")
        require(reported[name]["observations"] == run["observations"],
                f"{name}: observations differ from the run")

    failed = sorted(n for n, d in ran.items() if d["status"] != "pass")
    require(not failed, f"drills did not pass: {', '.join(failed)}")
    for name, drill in ran.items():
        require(drill["observations"], f"{name} passed without recording an observation")

    # The restore drill must have actually found the note, not merely run.
    restore = ran.get("purge-backup-restores-and-excludes-sync-keys")
    require(restore is not None, "the restore drill is missing")
    observed = restore["observations"]
    require(observed.get("restored_hits", 0) >= 1,
            f"the restore drill recovered nothing: {observed}")
    require(observed.get("archive_holds_keys") is False,
            "the restore drill did not establish that sync keys stay out of the backup")

    # Every I4 drill is either carried over or named as not carried over. A
    # safeguard that quietly stopped being checked is the failure this item is
    # about.
    i4 = json.loads((ROOT / "performance/v0.9-i4/REPORT.json").read_text(encoding="utf-8"))
    i4_names = {d["id"] for d in i4["drills"]}
    carried = set(ran)
    limits = " ".join(report["not_verified"]).lower()
    for name in sorted(i4_names):
        if name in carried:
            continue
        subject = name.replace("-", " ").split()[0]
        require(subject in limits or name.replace("-", " ") in limits or
                any(word in limits for word in name.split("-") if len(word) > 5),
                f"I4 drills {name} and this record neither carries it over nor says why not")

    equivalence = report["oracle_equivalence"]
    require(equivalence.get("held_by") and equivalence.get("proved_by_breaking"),
            "the record no longer says how the two oracles are held together")

    owed = report["still_owed"]
    require(owed.get("what") and owed.get("the_fix"),
            "the record no longer says what is still owed or how to close it")

    print(f"J3 record valid: {report['counts']['passed']}/{report['counts']['drills']} drills "
          f"passed against the command, oracle equivalence gated, "
          f"{len(report['not_verified'])} limits recorded")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError, OSError) as error:
        print(f"J3 record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
