#!/usr/bin/env python3
"""Validate the committed aggregate G14a calibration summary."""
from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parent
PHASES = [
    "inventory", "foreign-import", "snapshot-create", "snapshot-verify",
    "transport-prepare", "transport-seal", "snapshot-open",
    "snapshot-restore", "incremental-replay",
]


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g14a evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "calibration-results.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    require(report["schema"] == "notrios.g14a.calibration-summary.v1", "wrong schema")
    require(report["adapter"] == "archive-v2-loose", "wrong calibration adapter")
    require([tier["tier"] for tier in report["tiers"]] == [10_000, 100_000], "tiers differ")
    rows_by_tier: dict[int, dict[str, dict]] = {}
    for tier in report["tiers"]:
        rows = {row["phase"]: row for row in tier["rows"]}
        require(list(rows) == PHASES, f"tier {tier['tier']} phase order differs")
        for row in rows.values():
            require(row["wall_seconds"] > 0, "non-positive wall time")
            require(row["peak_rss_bytes"] > 0, "non-positive RSS")
            require(row["source_unchanged"] and row["arithmetic_valid"], "mandatory assertion failed")
        rows_by_tier[tier["tier"]] = rows

    small, large = rows_by_tier[10_000], rows_by_tier[100_000]
    for phase in PHASES:
        require(large[phase]["wall_seconds"] / small[phase]["wall_seconds"] < 10,
                f"{phase} is superlinear across generated tiers")
        require(large[phase]["wall_seconds"] < 2 * 60 * 60, f"{phase} exceeded two hours")
    require(large["snapshot-create"]["output_files"] > 100_000,
            "loose one-file-per-object failure is not represented")
    require(large["foreign-import"]["resources"] == 20 and large["snapshot-restore"]["resources"] == 20,
            "generated resource count did not survive the round trip")
    require(large["transport-prepare"]["container_entries"] == large["snapshot-create"]["output_files"],
            "ZIP entry arithmetic differs")
    zip_overhead = large["transport-prepare"]["output_bytes"] - large["transport-prepare"]["input_bytes"]
    seal_overhead = large["transport-seal"]["output_bytes"] - large["transport-seal"]["input_bytes"]
    require(zip_overhead == 26_224_320 and seal_overhead == 6_004, "100k overhead arithmetic differs")
    require(large["snapshot-open"]["peak_rss_bytes"] < 256 * 1024 * 1024, "open exceeds proxy")
    require(large["snapshot-restore"]["peak_rss_bytes"] < 256 * 1024 * 1024, "restore exceeds proxy")
    require(large["incremental-replay"]["peak_rss_bytes"] > 512 * 1024 * 1024,
            "desktop RSS failure is not represented")
    require("797,937,664" in findings and "100,093" in findings, "findings omit failures")
    require("never point" in readme and "recipedb_repo" in readme, "repository safety rule missing")
    forbidden = ("/home/", "JoplinExport_", "recipe_joplin", "recipe_vault")
    serialized = json.dumps(report)
    require(not any(value in serialized for value in forbidden), "committed evidence contains a private path")
    print("g14a evidence: 18 aggregate phase rows, two expected baseline failures validated")


if __name__ == "__main__":
    main()
