#!/usr/bin/env python3
"""Validate the I6 record offline, and enforce the workflow hardening.

The release set itself lives in `dist/` and is not committed -- it contains a
29 MB package. So this checks the record of it, and runs the two checks that
apply to the repository rather than to a built set: workflow hardening, and
(through I5's validator, which `make validate` also runs) secret exposure.

The pinned-action count is *derived* from the workflows rather than read from
the report, so the report cannot claim a hardening the repository does not have.
"""
import json
import pathlib
import re
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
USES = re.compile(r"uses:\s*[^@\s]+@([0-9a-f]{40})")


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v09.i6-release-evidence.v1", "wrong report schema")

    released = report["release_set"]
    require(released["artifacts"], "the record names no artifacts")
    require(set(released["sha256"]) == set(released["artifacts"]),
            "the recorded hashes and the artifact list disagree")
    require("SHA256SUMS" in released["artifacts"],
            "the set does not carry the file a downloader checks the others against")

    sbom = released["sbom"]
    inventory = json.loads((ROOT / "performance/v0.7-g20/DEPENDENCY_LICENSES.json")
                           .read_text(encoding="utf-8"))
    require(sbom["go_modules"] == len(inventory["go"]["modules"]),
            f"the record says {sbom['go_modules']} Go modules, the licence inventory has "
            f"{len(inventory['go']['modules'])}")
    require(sbom["npm_packages"] == sum(v["packages"] for v in inventory["npm"]["lockfiles"].values()),
            "the record's npm package count disagrees with the licence inventory")
    require(sbom["components"] == sbom["go_modules"] + sbom["npm_packages"],
            "the component total does not add up")

    # Derived: what the workflows actually are, not what the report says.
    pinned = 0
    workflows = sorted((ROOT / ".github" / "workflows").glob("*.y*ml"))
    for path in workflows:
        pinned += len(USES.findall(path.read_text(encoding="utf-8")))
    require(report["workflow_hardening"]["actions_pinned_by_digest"] == pinned,
            f"the record claims {report['workflow_hardening']['actions_pinned_by_digest']} pinned "
            f"actions, the workflows have {pinned}")

    hardening = subprocess.run([sys.executable, str(HERE / "check_workflow_hardening.py")],
                               capture_output=True, text=True)
    if hardening.returncode != 0:
        raise EvidenceError(hardening.stderr.strip() or "workflow hardening failed")

    draft = report["draft_flow"]
    require(draft["state"].startswith("exercised dry"),
            "the draft flow record no longer says whether anything was uploaded")
    require(draft.get("why_not_executed"), "the record no longer says why no draft was created")

    require(released["signing_state"] == "unsigned",
            "the record claims a signed set; I5 created no key")

    limits = report["not_verified"]
    require(len(limits) >= 5, "the report has stopped saying what it did not verify")
    text = " ".join(limits).lower()
    for subject in ("signature", "scan", "attest"):
        require(subject in text, f"the limits no longer mention {subject}")

    print(f"I6 release evidence valid: {len(released['artifacts'])} artifacts, "
          f"{sbom['components']} SBOM components matching the licence inventory, "
          f"{pinned} actions pinned, {len(limits)} limits recorded")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError, OSError) as error:
        print(f"I6 release evidence invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
