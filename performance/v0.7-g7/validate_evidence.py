#!/usr/bin/env python3
"""Check that the committed G7 evidence says what the documentation claims.

This runs in CI-style validation rather than at generation time, so a report
that was regenerated with different numbers cannot silently disagree with
FINDINGS.md.
"""
from __future__ import annotations

import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parent
EXPECTED_INTERVALS = ["1 hour", "1 day", "1 week", "30 days"]


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g7 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "transfer-results.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g7.transfer.v1", "wrong report schema")
    require(report["protocol"] == "1.0", "protocol is not 1.0")
    require(report["database_schema_version"] == 22, "report was not produced against schema v22")

    intervals = report["intervals"]
    require([row["interval"] for row in intervals] == EXPECTED_INTERVALS,
            "the four G1 offline intervals are not all present, in order")

    previous_share = None
    for row in intervals:
        label = row["interval"]
        require(row["documents"] > 0, f"{label}: no documents")
        require(row["converged_documents"] == row["documents"],
                f"{label}: {row['converged_documents']} of {row['documents']} documents converged")
        require(row["revisions"] == row["delta_revisions"] + row["inline_revisions"],
                f"{label}: revision counts do not add up")
        require(row["transfer_bytes"] < row["complete_body_bytes"],
                f"{label}: transferring deltas was not smaller than transferring complete bodies")
        require(row["pending_bodies"] == 0, f"{label}: revisions were left without bytes")
        require(not row["pending_reasons"], f"{label}: unexpected pending reasons {row['pending_reasons']}")
        require(set(row["conflict_kinds"]) <= {"same_token"},
                f"{label}: unexpected conflict kinds {row['conflict_kinds']}")
        require(sum(row["conflict_kinds"].values()) == row["conflicts"],
                f"{label}: conflict kinds do not sum to the conflict count")

        share = row["transfer_percent_of_complete"]
        require(0 < share < 100, f"{label}: implausible transfer share {share}")
        # Longer divergence must not cost proportionally more: this is the
        # claim the feature exists to support, so it is checked rather than
        # described.
        if previous_share is not None:
            require(share < previous_share,
                    f"{label}: transfer share {share} did not improve on {previous_share}")
        previous_share = share

    # Every number FINDINGS.md prints in its transfer table must come from the
    # report. A table that drifted from the data it summarizes is worse than no
    # table.
    for row in intervals:
        for value in (row["revisions"], row["delta_revisions"], row["transfer_bytes"], row["complete_body_bytes"]):
            require(f"{value:,}" in findings or str(value) in findings,
                    f"{row['interval']}: FINDINGS.md does not mention {value}")
        require(f"{row['transfer_percent_of_complete']:.2f}%" in findings,
                f"{row['interval']}: FINDINGS.md does not mention the {row['transfer_percent_of_complete']:.2f}% share")

    require("performance/v0.7-g1a/PROVENANCE.md" in readme,
            "README.md does not carry the delta codec's provenance forward")
    require(re.search(r"desktop", readme, re.IGNORECASE) and re.search(r"desktop", findings, re.IGNORECASE),
            "the desktop-only scope is not stated in both documents")

    print(f"g7 evidence: {len(intervals)} intervals validated")


if __name__ == "__main__":
    main()
