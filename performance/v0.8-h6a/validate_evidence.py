#!/usr/bin/env python3
"""Validate the v0.8 H6a investigation evidence.

It checks that the record is internally consistent and, more importantly, that
the claim ladder is still being respected: no finding may assert a level this
investigation could not reach. A report that quietly promoted a cross-compiled
artifact to "supported" would be worse than no report.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]

MAX_LEVEL_REACHED = 2  # structurally inspected; nothing here was run on a target


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    findings = json.loads((HERE / "FINDINGS.json").read_text(encoding="utf-8"))
    toolchain = json.loads((HERE / "TOOLCHAIN.json").read_text(encoding="utf-8"))
    require(findings["schema"] == "notrios.h6a.findings.v1", "wrong findings schema")
    require(toolchain["schema"] == "notrios.h6a.toolchain-probe.v1", "wrong toolchain schema")

    ladder = {entry["level"]: entry["name"] for entry in findings["claim_ladder"]}
    require(ladder == {1: "generated", 2: "structurally inspected",
                       3: "natively installed and executed", 4: "supported"},
            "the claim ladder has changed shape")

    for finding in findings["findings"]:
        require(finding["level"] <= MAX_LEVEL_REACHED,
                f"{finding['id']} claims level {finding['level']}; this investigation "
                f"reached at most {MAX_LEVEL_REACHED}, and a cross-compiled or inspected "
                "artifact is not native runtime evidence")
        require(finding["verdict"] in {"measured", "works", "gap", "blocked"},
                f"{finding['id']} has an unknown verdict {finding['verdict']!r}")
        require(finding["detail"].strip(), f"{finding['id']} records no detail")

    require(findings["not_verified"], "no unverified gaps are recorded, which is never true")

    # The transcript the report quotes has to be here.
    lintian = (HERE / "LINTIAN.txt").read_text(encoding="utf-8")
    errors = [line for line in lintian.splitlines() if line.startswith("E:")]
    require(len(errors) == 3, f"LINTIAN.txt records {len(errors)} errors, the report says 3")
    for tag in ("no-copyright-file", "no-changelog", "extended-description-is-empty"):
        require(tag in lintian, f"the report names {tag} and the transcript does not")

    # The probe config must not have become a production dependency.
    for name in ("Makefile", "scripts/package_release.sh", "scripts/validate-scaffold.sh"):
        body = (ROOT / name).read_text(encoding="utf-8")
        require("goreleaser" not in body,
                f"{name} references goreleaser; H6a is an investigation and selects no "
                "packaging dependency")

    blocked = [f["id"] for f in findings["findings"] if f["verdict"] == "blocked"]
    print(f"H6a evidence valid: {len(findings['findings'])} findings, "
          f"max claim level {MAX_LEVEL_REACHED}, {len(blocked)} blocked platforms, "
          f"{len(findings['not_verified'])} gaps recorded.")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"H6a evidence invalid: {error}", file=sys.stderr)
        sys.exit(1)
