#!/usr/bin/env python3
"""Validate the I8 record, and re-derive the frozen surfaces.

Two different jobs. The surfaces are *re-derived from source* on every run and
compared with what was frozen, so this is a live gate rather than a record
check: adding a REST route or renaming a CLI command fails until somebody
re-records the freeze and says what moved. The edges record is checked the way
the other milestone records are, because running sanitizers inside `make
validate` would put minutes on every build.
"""
import hashlib
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
sys.path.insert(0, str(HERE))
import surfaces  # noqa: E402


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    frozen = json.loads((HERE / "FROZEN.json").read_text(encoding="utf-8"))
    require(frozen["schema"] == "notrios.v09.i8-frozen-surfaces.v1", "wrong freeze schema")
    recorded = frozen["surfaces"]
    live = surfaces.inventory()

    require(set(recorded) == set(live),
            f"a surface appeared or vanished: {sorted(set(recorded) ^ set(live))}")
    for name in sorted(live):
        was, now = recorded[name], live[name]
        # The recorded hash must match the recorded members, before it is
        # compared with anything. Trusting the stored hash let a truncated
        # member list pass: the summary still agreed with live source while the
        # list it summarised did not, which is a freeze that records the wrong
        # thing and says so in a field nobody checks.
        restated = hashlib.sha256("\n".join(was["members"]).encode("utf-8")).hexdigest()
        require(restated == was["sha256"],
                f"the frozen {name} surface's hash does not match its own member list; "
                f"the record has been edited by hand")
        require(len(was["members"]) == was["count"],
                f"the frozen {name} surface's count does not match its member list")
        if was["sha256"] == now["sha256"]:
            continue
        added = sorted(set(now["members"]) - set(was["members"]))
        removed = sorted(set(was["members"]) - set(now["members"]))
        raise EvidenceError(
            f"the {name} surface changed and the freeze does not record it: "
            f"{len(added)} added {added[:4]}, {len(removed)} removed {removed[:4]}. "
            f"If it is deliberate, run performance/v0.9-i8/build_freeze.py and say in the commit "
            f"what moved and whether it is compatible.")

    require(frozen["not_frozen_here"], "the freeze no longer says what it does not cover")

    edges = json.loads((HERE / "EDGES.json").read_text(encoding="utf-8"))
    require(edges["schema"] == "notrios.v09.i8-abi-edges.v2", "wrong edges schema")
    asan = edges["variants"]["asan"]
    require(asan["ran"] and not asan["failed"],
            f"the asan run did not pass every check: {asan.get('failed')}")
    require(asan["address_sanitizer_reports"] == 0,
            f"AddressSanitizer reported {asan['address_sanitizer_reports']} errors")
    require(asan["exit_status"] == 0, f"the asan host exited {asan['exit_status']}")
    race = edges["variants"]["race"]
    require(race["ran"] and race["data_races"] == 0 and race["exit_status"] == 0,
            f"the race run reported {race.get('data_races')} races, exit {race.get('exit_status')}")

    require(edges["symbols"] == live["c_abi"]["members"],
            "the symbols the edges run observed are not the frozen ABI symbols")

    instrumentation = edges["instrumentation"]
    require("valgrind" in instrumentation["why"].lower(),
            "the record no longer explains why valgrind is not the instrument here")
    limits = edges["not_verified"]
    require(len(limits) >= 4, "the edges record has stopped saying what it did not verify")
    text = " ".join(limits).lower()
    for subject in ("fuzz", "msan", "dlclose"):
        require(subject in text, f"the limits no longer mention {subject}")

    counts = ", ".join(f"{name} {data['count']}" for name, data in sorted(live.items()))
    print(f"I8 record valid: surfaces re-derived and unchanged ({counts}); "
          f"asan {asan['passed']}/{asan['checks']} checks with 0 reports, 0 data races")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError, OSError) as error:
        print(f"I8 record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
