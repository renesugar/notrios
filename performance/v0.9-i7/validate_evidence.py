#!/usr/bin/env python3
"""Validate the I7 record offline.

The soak runs a service for a quarter of an hour and the drills install and
delete; `make validate` does neither. This checks what they recorded, and
enforces the one rule a support matrix exists to enforce: a row may not claim
more than the evidence it names.

The claim levels are *derived* from the evidence that produced them -- I3's
report for the platform this project ships -- so a row cannot be promoted by
editing this file.
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
    require(report["schema"] == "notrios.v09.i7-soak-recover-freeze.v1", "wrong report schema")

    soak = json.loads((HERE / "SOAK.json").read_text(encoding="utf-8"))
    require(soak["complete"], f"the soak did not run to completion: {soak['incomplete_because']}")
    require(soak["errors"] == 0, f"the soak recorded {soak['errors']} failed requests")
    require(soak["ran_for_seconds"] >= 600,
            f"the soak ran {soak['ran_for_seconds']}s, too short to separate warm-up from a trend")
    require(soak["growth"]["open_fds"] <= 0,
            f"descriptors grew by {soak['growth']['open_fds']} during the soak")
    analysis = soak["analysis"]
    require(analysis["slow_leak_ruled_out"] is False,
            "the record claims a slow leak is ruled out; fifteen minutes cannot establish that")
    require(report["soak"]["ran_for_seconds"] == soak["ran_for_seconds"],
            "the report and the soak disagree about how long it ran")

    drills = [json.loads(line) for line in
              (HERE / "DRILLS.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
    ran = {d["drill"]: d for d in drills}
    reported = {d["id"]: d for d in report["drills"]}
    require(set(ran) == set(reported),
            f"the report and the run disagree about the drills: {sorted(set(ran) ^ set(reported))}")
    failed = sorted(n for n, d in ran.items() if d["status"] != "pass")
    require(not failed, f"drills did not pass: {', '.join(failed)}")
    for name, drill in ran.items():
        require(drill["observations"], f"{name} passed without recording an observation")

    recovery = ran.get("library-lost-and-recovered-from-an-export")
    require(recovery is not None, "the recovery drill is missing")
    observed = recovery["observations"]
    # The drill must have destroyed something before recovering it. A recovery
    # that never lost anything is the shape the first version of this had.
    require(observed["hits_before"] >= 1 and observed["hits_after_loss"] == 0
            and observed["hits_after_recovery"] >= 1,
            f"the recovery drill did not lose and regain the library: {observed}")

    matrix = json.loads((HERE / "MATRIX.json").read_text(encoding="utf-8"))
    require(matrix["schema"] == "notrios.v09.i7-support-matrix.v1", "wrong matrix schema")
    i3 = json.loads((ROOT / "performance/v0.9-i3/REPORT.json").read_text(encoding="utf-8"))
    for row in matrix["surfaces"]:
        require("claim_level" in row and "executed" in row and "supported" in row,
                f"{row.get('platform')}: a row must say its level, whether it ran, and whether it "
                f"is supported")
        require(row.get("evidence"), f"{row['platform']}: a row must name its evidence")
        # The rule the matrix exists for.
        if row["supported"]:
            require(row["executed"], f"{row['platform']}: supported without having been executed")
            require(row["claim_level"] >= 3,
                    f"{row['platform']}: supported at level {row['claim_level']}")
        if not row["executed"]:
            require(row["claim_level"] < 3,
                    f"{row['platform']}: claims level {row['claim_level']} without being executed")
            require(row.get("blocked_on") or row.get("evidence"),
                    f"{row['platform']}: not executed and nothing says why")
        if row["platform"] in ("Windows", "macOS"):
            require(row.get("absent_from_release_claims") is True,
                    f"{row['platform']}: a postponed platform must be absent from release claims")

    shipped = next(r for r in matrix["surfaces"]
                   if r["platform"] == "Ubuntu 24.04, amd64" and r["surface"].startswith("command"))
    require(shipped["claim_level"] == i3["level_reached"]["level"],
            f"the matrix claims level {shipped['claim_level']} for the shipped platform; I3's "
            f"evidence establishes {i3['level_reached']['level']}")

    desktop = next(r for r in matrix["surfaces"] if r["surface"].startswith("desktop shell"))
    require(desktop["executed"] is False and desktop["supported"] is False,
            "the desktop shell is claimed as executed or supported; CI compiles it and nothing "
            "in v0.9 has run it")

    limits = report["not_verified"]
    require(len(limits) >= 6, "the report has stopped saying what it did not verify")
    text = " ".join(limits).lower()
    for subject in ("emulator", "crash", "support bundle", "slow"):
        require(subject in text, f"the limits no longer mention {subject}")

    print(f"I7 record valid: soak {soak['ran_for_seconds']}s / {soak['requests']} requests / "
          f"{soak['errors']} errors, {len(ran)} drills, matrix {report['supported_rows']} of "
          f"{report['matrix_rows']} rows supported, {len(limits)} limits recorded")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError, OSError, StopIteration) as error:
        print(f"I7 record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
