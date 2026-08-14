#!/usr/bin/env python3
"""Check that the committed G14 evidence says what the documentation claims."""
from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g14 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "rest-data-results.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g14.restdata.v1", "wrong report schema")
    require(report["database_schema_version"] == 25, "report was not produced against schema v25")

    exchanges = report["exchanges"]
    require(len(exchanges) >= 2, "expected at least two exchange sizes")
    for row in exchanges:
        label = f"{row['notes']} notes"
        require(row["converged"], f"{label}: the replicas did not converge over REST")
        # The claim the whole design rests on: one protocol, two couriers.
        require(row["transcript_identical"],
                f"{label}: the REST carrier did not hold what the folder held")
        require(row["rest_exchange_ms"] > 0 and row["folder_exchange_ms"] > 0,
                f"{label}: an exchange was not measured")

    backups = report["backups"]
    require(len(backups) >= 2, "expected at least two backup sizes")
    resumed = 0
    for row in backups:
        label = f"{row['notes']} notes"
        require(row["verified_as_archive_v2"], f"{label}: the snapshot did not verify")
        require(row["restored_records"] > 0, f"{label}: the snapshot verified as empty")
        require(row["sealed_bytes"] > row["archive_bytes"],
                f"{label}: sealing did not add the framing it must")
        require(row["download_requests"] >= 1, f"{label}: no download happened")
        if row["resumed_after_stop"]:
            resumed += 1
    require(resumed >= 1, "no tier exercised a resumed transfer, which is the working state")

    for row in exchanges:
        for value in (row["rest_exchange_ms"], row["folder_exchange_ms"]):
            require(f"{value:,}" in findings or str(value) in findings,
                    f"FINDINGS.md does not mention {value}")
    for row in backups:
        for value in (row["sealed_bytes"], row["archive_bytes"]):
            require(f"{value:,}" in findings or str(value) in findings,
                    f"FINDINGS.md does not mention {value}")

    require("failure budget" in findings.lower(), "FINDINGS.md does not record the defect the run found")
    require("ZIP parsing is never the trust boundary" in findings,
            "FINDINGS.md does not state where correctness comes from")
    require(re.search(r"no network", findings, re.IGNORECASE), "FINDINGS.md does not scope out the network")
    require(re.search(r"generated", readme, re.IGNORECASE), "README.md does not state the corpus is generated")

    print(f"g14 evidence: {len(exchanges)} exchange tiers and {len(backups)} backup tiers validated")


if __name__ == "__main__":
    main()
