#!/usr/bin/env python3
"""Validate the v0.8 H8 installed-integration matrix.

The matrix is only worth having if it cannot quietly become optimistic. Two
things are enforced that a reader cannot check by eye: every row is either
executed with a result or postponed with a reason, and no platform is described
as covered when its rows were skipped. H8's own boundary says a skipped row
blocks a support claim rather than closing it, and this is where that stops
being a sentence and starts being a check.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
STATES = {"passed", "failed", "postponed"}


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "RESULTS.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.h8.integration-matrix.v1", "wrong results schema")
    rows = report["rows"]
    require(rows, "the matrix has no rows")

    identifiers = set()
    for row in rows:
        require(row["id"] not in identifiers, f"duplicate row {row['id']}")
        identifiers.add(row["id"])
        require(row["status"] in STATES, f"{row['id']} has unknown status {row['status']!r}")

        # The rule: a result or a reason, never neither and never both implied.
        if row["status"] == "postponed":
            require(row.get("reason", "").strip(),
                    f"{row['id']} is postponed with no reason, which is indistinguishable "
                    "from being forgotten")
            require("detail" not in row,
                    f"{row['id']} is postponed and carries a result; a row that did not run "
                    "must not look like one that did")
        else:
            require(row.get("detail", "").strip(),
                    f"{row['id']} is {row['status']} and records nothing about what happened")

    # No platform is covered by rows that did not run there.
    for platform in ("windows", "macos"):
        executed = [r for r in rows
                    if platform in r["platforms"] and r["status"] in ("passed", "failed")]
        require(not executed,
                f"{platform} rows report execution, but this harness runs on "
                f"{report['host_platform']} and cannot execute them: {[r['id'] for r in executed]}")

    linux_ran = [r for r in rows if "linux" in r["platforms"] and r["status"] == "passed"]
    require(linux_ran, "no Linux row passed, so the Ubuntu baseline claims nothing")

    totals = report["totals"]
    for state in STATES:
        counted = sum(1 for r in rows if r["status"] == state)
        require(totals[state] == counted,
                f"totals say {totals[state]} {state}, the rows say {counted}")
    require(totals["failed"] == 0,
            f"{totals['failed']} row(s) failed; the matrix records a baseline, not a wish")

    print(f"H8 evidence valid: {totals['passed']} passed, {totals['postponed']} postponed "
          f"with reasons, 0 failed; no non-{report['host_platform']} row claims execution.")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H8 evidence invalid: {error}", file=sys.stderr)
        sys.exit(1)
