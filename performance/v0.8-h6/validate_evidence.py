#!/usr/bin/env python3
"""Validate the v0.8 H6 package evidence.

The record must stay honest about what was actually done. Two things are
checked that a reader cannot see for themselves: that the claim level matches
what the transcript supports, and that the package still declares dependencies
rather than reverting to the underdeclared state H6a measured.
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
    record = json.loads((HERE / "PACKAGE.json").read_text(encoding="utf-8"))
    require(record["schema"] == "notrios.h6.package.v1", "wrong package schema")

    # H6a's ladder: 3 is "natively installed and executed". H6 reaches it for
    # amd64 and must not claim 4, which means a maintainer commitment.
    require(record["claim_level"] == 3,
            f"claim level {record['claim_level']}; H6 installed and executed but nobody "
            "has committed to supporting it, which is what level 4 means")
    require(record["not_claimed"], "no limits are recorded, which is never true")

    # The defect H6a measured must not come back.
    depends = record["depends"]
    require(depends, "the package declares no dependencies; that is the exact gap "
                     "H6a recorded as undeclared-elf-prerequisites")
    require(any(d.startswith("libc6") for d in depends),
            "libc6 is not declared and every binary here links it")

    lintian = (HERE / "LINTIAN.txt").read_text(encoding="utf-8")
    errors = [line for line in lintian.splitlines() if line.startswith("E:")]
    require(not errors, f"the transcript records {len(errors)} lintian errors; H6 shipped none")

    # Every accepted gap needs a reason, or it is just an ignored warning.
    for gap in record["accepted_gaps"]:
        require(gap.get("reason", "").strip(), f"{gap['tag']} is accepted with no reason")

    # The build must not have become a source-tree writer.
    script = (ROOT / "scripts/build_deb.sh").read_text(encoding="utf-8")
    require("DESTDIR=" in script, "the package build no longer stages through DESTDIR")
    require("dpkg-shlibdeps" in script, "dependencies are no longer computed from the binaries")

    print(f"H6 evidence valid: {record['package']['name']} {record['package']['version']} "
          f"{record['package']['architecture']}, {len(depends)} computed dependencies, "
          f"{len(record['verified'])} verified behaviours, 0 lintian errors, "
          f"claim level {record['claim_level']}.")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H6 evidence invalid: {error}", file=sys.stderr)
        sys.exit(1)
