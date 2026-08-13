#!/usr/bin/env python3
"""Check that the committed G11 evidence says what the documentation claims."""
from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g11 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "carrier-results.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g11.carrier.v1", "wrong report schema")
    require(report["database_schema_version"] == 24, "report was not produced against schema v24")

    tiers = report["tiers"]
    require(len(tiers) >= 2, "expected at least two library sizes")
    for row in tiers:
        label = f"{row['notes']} notes"
        require(row["converged"], f"{label}: the replicas did not converge")
        require(row["operations"] > 0, f"{label}: no operations were exchanged")
        require(row["carrier_artifacts"] > 0, f"{label}: the carrier holds nothing")
        # The property the naming scheme exists for: an idle library must not
        # fill a shared drive.
        require(row["artifacts_added_by_ten_quiet_rounds"] == 0,
                f"{label}: ten quiet rounds added {row['artifacts_added_by_ten_quiet_rounds']} artifacts")
        require(row["notes_pending_when_carrier_deleted"] > 0,
                f"{label}: the carrier was deleted with nothing in flight, which tests nothing")
        require(row["rounds_to_reconverge_after_carrier_deleted"] > 0,
                f"{label}: recovery after deletion was not measured")
        # The claim the findings make about where the cost is.
        require(row["carrier_layer_operations"] > 0, f"{label}: the carrier layer was not measured")
        require(row["seal_and_publish_ms"] + row["read_and_open_ms"] < row["first_exchange_ms"] // 4,
                f"{label}: the carrier layer is not a small share of the exchange")

    for row in tiers:
        for value in (row["first_exchange_ms"], row["quiet_round_ms"], row["carrier_bytes"],
                      row["seal_and_publish_ms"], row["read_and_open_ms"]):
            require(f"{value:,}" in findings or str(value) in findings,
                    f"FINDINGS.md does not mention {value}")

    bounds = report["scan_bounds"]
    require(len(bounds) >= 2, "expected at least two scan tiers")
    capped = [row for row in bounds if row["listed"] < row["artifacts_in_one_class"]]
    require(capped, "no scan tier exceeded the listing cap, so the bound was never exercised")

    require("torn artifact" in findings, "FINDINGS.md does not record the repair defect")
    require("standoff" in findings, "FINDINGS.md does not record the empty-carrier defect")
    require(re.search(r"G12", findings), "FINDINGS.md does not defer cloud and removable media to G12")
    require(re.search(r"desktop", readme, re.IGNORECASE), "README.md does not state the desktop-only scope")

    print(f"g11 evidence: {len(tiers)} library sizes and {len(bounds)} scan tiers validated")


if __name__ == "__main__":
    main()
