#!/usr/bin/env python3
"""Validate the I4 destructive-lifecycle record, offline.

The drills install, delete and restore; `make validate` must not. So this checks
the record they produced -- that every drill is accounted for, that they all
passed, that each observed something, and that the report still says what it did
not verify.

The limits list is gated for the same reason it exists. These are drills against
deletion, and a record whose caveats quietly disappear reads as a much stronger
claim than anyone made: "the destructive lifecycle is hardened" is not the same
sentence as "eight unprivileged faults were survived, and interruption, process
races and a genuinely full disk were not tried".
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v09.i4-destructive-lifecycle.v1", "wrong report schema")

    drills = [json.loads(line) for line in
              (HERE / "DRILLS.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
    require(drills, "no drills recorded")

    ran = {d["drill"]: d for d in drills}
    reported = {d["id"]: d for d in report["drills"]}
    require(set(ran) == set(reported),
            f"the report and the run disagree about which drills exist: {sorted(set(ran) ^ set(reported))}")
    for name, run in ran.items():
        require(reported[name]["status"] == run["status"], f"{name}: status differs from the run")
        require(reported[name]["observations"] == run["observations"],
                f"{name}: observations differ from the run")
        require(run["observations"], f"{name} passed without recording a single observation")

    failed = sorted(n for n, d in ran.items() if d["status"] != "pass")
    require(not failed, f"drills did not pass: {', '.join(failed)}")
    require(report["counts"]["drills"] == len(drills), "the drill count disagrees")
    require(report["counts"]["passed"] == len(drills), "the passed count disagrees")

    # The four claims this item exists to make, each tied to a drill by name so
    # that removing the drill removes the claim rather than orphaning it.
    for required in ("uninstall-leaves-user-data",
                     "purge-refuses-when-the-backup-cannot-be-written",
                     "purge-backup-restores-and-excludes-sync-keys",
                     "purge-does-not-delete-through-a-symlink"):
        require(required in ran, f"the drill that carries a stated claim is missing: {required}")

    race = report["profile_race"]
    require(race["state"] == "fixed", "the profile race is no longer recorded as fixed")
    for field in ("was", "now", "gave_up", "why_not_a_race_test"):
        require(race.get(field), f"the profile-race record no longer says what it {field}")

    limits = report["not_verified"]
    require(len(limits) >= 6, "the report has stopped saying what it did not verify")
    text = " ".join(limits).lower()
    for subject in ("full filesystem", "running", "interrupt", "privileg"):
        require(subject in text, f"the limits no longer mention {subject}")

    print(f"I4 destructive-lifecycle record valid: {report['counts']['passed']}/"
          f"{report['counts']['drills']} drills passed, profile race fixed, "
          f"{len(limits)} limits recorded")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError, OSError) as error:
        print(f"I4 destructive-lifecycle record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
