#!/usr/bin/env python3
"""Check that the committed G12 evidence says what the documentation claims."""
from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent

# Phases whose absence would mean the milestone did not do what it exists for.
REQUIRED_PHASES = {
    "converge through the provider",
    "peer files are never written by another peer",
    "a conflicting file at a protocol name is refused and repaired",
    "an advertisement seen before its envelopes costs latency only",
    "an unavailable carrier refuses rather than diverging",
    "full carrier loss and reconstruction",
    "rclone copy --immutable moves the carrier without changing it",
    "a drive passed between peers converges",
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g12 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "conformance-results.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g12.conformance.v1", "wrong report schema")
    require(report["database_schema_version"] == 24, "report was not produced against schema v24")
    require(report["notes_exchanged"] > 0, "no notes were exchanged")

    phases = {row["name"]: row for row in report["phases"]}
    missing = REQUIRED_PHASES - set(phases)
    require(not missing, f"phases were not run: {sorted(missing)}")
    for name, row in phases.items():
        require(row["passed"], f"phase failed: {name} ({row['detail']})")

    provider = report["provider"]
    require(provider["remote_probes_ran"], "the provider visibility probes did not run")
    listings = provider["remote_write_to_listing_ms"]
    opens = provider["remote_write_to_open_ms"]
    require(len(listings) >= 3, "fewer than three visibility samples, so the delay is one observation")
    require(min(listings) > 0, "a visibility sample recorded no delay at all")
    # The claim the findings rest on: a name is no fresher than a listing, so a
    # stale directory cannot be worked around by fetching a known object.
    for listing, opened in zip(listings, opens):
        require(abs(listing - opened) < 2_000,
                f"listing and by-name visibility diverged ({listing} vs {opened} ms)")
    require(provider["slowest_listing_ms"] == max(listings), "the slowest listing does not match the samples")
    require(provider["case_sensitive"] is True or provider["case_sensitive"] is False,
            "case sensitivity was not determined")

    for value in (provider["slowest_listing_ms"],):
        require(f"{value:,}" in findings or str(value) in findings,
                f"FINDINGS.md does not mention {value}")
    for sample in listings:
        require(f"{sample:,}" in findings or str(sample) in findings,
                f"FINDINGS.md does not mention the {sample} ms sample")

    require("advertisement" in findings and "latency" in findings,
            "FINDINGS.md does not record the publication-order result")
    require(re.search(r"rclone sync|sync/bisync", findings), "FINDINGS.md does not record the refused verbs")
    require("not two machines" in findings.lower(), "FINDINGS.md does not state the single-host limit")
    require(re.search(r"generated", readme, re.IGNORECASE), "README.md does not state that the corpus is generated")
    require("--immutable" in readme, "README.md does not name the only permitted copy verb")

    print(f"g12 evidence: {len(phases)} phases and {len(listings)} visibility samples validated")


if __name__ == "__main__":
    main()
