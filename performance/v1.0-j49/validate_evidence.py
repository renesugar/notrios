#!/usr/bin/env python3
"""v1.0 J49: the release is built on a row the matrix supports.

    python3 performance/v1.0-j49/validate_evidence.py

Every `runs-on` in `.github/workflows/release.yml` must name an exact runner
image, and that image must be a supported row of J9's matrix. `ubuntu-latest`
is refused: dpkg-shlibdeps computes the package's Depends from the system it
builds on, so a label GitHub moves would retarget the release at a row nobody
ran, and nothing here would notice until a stranger's install failed.
"""
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/release.yml"
MATRIX = ROOT / "performance/v1.0-j9/MATRIX.json"
LABEL = re.compile(r"^\s*runs-on:\s*(\S+)\s*$", re.M)
IMAGE = re.compile(r"^ubuntu-(\d{2}\.\d{2})$")


def runner_problems(workflow_text: str, matrix: dict) -> tuple[list[str], list[str]]:
    """(labels found, problems). A problem names the label and what is wrong."""
    supported = {row["platform"] for row in matrix["surfaces"]
                 if row["supported"] and row["surface"].startswith("command line")}
    labels = LABEL.findall(workflow_text)
    problems = [] if labels else ["the workflow names no runs-on"]
    for label in labels:
        image = IMAGE.match(label)
        if not image:
            problems.append(f"runs-on {label!r} is not an exact Ubuntu image; it can move")
            continue
        row = f"Ubuntu {image.group(1)} amd64"
        if row not in supported:
            problems.append(f"runs-on {label!r} builds on {row}, which the matrix does not support")
    return labels, problems


def main() -> int:
    labels, problems = runner_problems(WORKFLOW.read_text(encoding="utf-8"),
                                       json.loads(MATRIX.read_text(encoding="utf-8")))
    if problems:
        for problem in problems:
            print(f"J49 release runner invalid: {problem}", file=sys.stderr)
        return 1
    print(f"J49 release runner valid: {', '.join(sorted(set(labels)))}, a supported row of the matrix")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
