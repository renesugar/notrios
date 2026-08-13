#!/usr/bin/env python3
"""Check that the committed G10 evidence says what the documentation claims."""
from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g10 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "catchup-results.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g10.catchup.v1", "wrong report schema")
    require(report["database_schema_version"] == 24, "report was not produced against schema v24")

    tiers = report["tiers"]
    require(len(tiers) >= 2, "expected at least two library sizes")
    for row in tiers:
        label = f"{row['notes']} notes"
        require(row["converged"], f"{label}: the catch-up did not converge")
        require(row["total_operations"] == row["snapshot_operations"] + row["tail_operations"],
                f"{label}: operation counts do not add up")
        require(row["tail_operations"] > 0, f"{label}: nothing came after the snapshot, so nothing was measured")
        require(row["tail_replay_ms"] < row["full_replay_ms"],
                f"{label}: replaying only the tail was not faster")
        # The claim the feature exists to support: a snapshot removes most of
        # the work, and the time follows the work rather than merely correlating.
        require(row["work_avoided_percent"] > 50, f"{label}: only {row['work_avoided_percent']}% of work avoided")
        require(row["time_saved_percent"] > 50, f"{label}: only {row['time_saved_percent']}% of time saved")

    for row in tiers:
        for value in (row["total_operations"], row["full_replay_ms"], row["tail_replay_ms"]):
            require(f"{value:,}" in findings or str(value) in findings,
                    f"FINDINGS.md does not mention {value}")

    require("catch-up floor" in findings, "FINDINGS.md does not record the defect this measurement found")
    require("acknowledgement" in findings, "FINDINGS.md does not record the backup/acknowledgement boundary")
    require(re.search(r"argon2", findings, re.IGNORECASE), "FINDINGS.md does not record the new dependency")
    require(re.search(r"desktop", readme, re.IGNORECASE), "README.md does not state the desktop-only scope")

    print(f"g10 evidence: {len(tiers)} library sizes validated")


if __name__ == "__main__":
    main()
