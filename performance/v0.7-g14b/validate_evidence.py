#!/usr/bin/env python3
"""Validate the committed aggregate-only G14b decision evidence."""
from __future__ import annotations

import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parent
COMPLETE_ARCHIVE_PHASES = {
    "snapshot-create", "snapshot-verify", "transport-prepare",
    "transport-seal", "snapshot-open", "snapshot-restore",
    "corruption-refusal",
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g14b evidence: {message}")


def indexed_rows(report: dict) -> dict[tuple[str, str, str], dict]:
    rows: dict[tuple[str, str, str], dict] = {}
    for row in report["rows"]:
        key = (row["workload"], row["adapter"], row["phase"])
        require(key not in rows, f"duplicate row {key}")
        require(row["status"] in {"completed", "interrupted"}, f"bad status for {key}")
        if row["status"] == "completed":
            require(row["wall_seconds"] >= 0, f"negative wall time for {key}")
            require(row["peak_rss_bytes"] > 0, f"non-positive RSS for {key}")
            require(row["source_unchanged"], f"source mutation for {key}")
        rows[key] = row
    return rows


def main() -> None:
    report_path = ROOT / "full-corpus-results.json"
    report = json.loads(report_path.read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    serialized = json.dumps(report, sort_keys=True)

    require(report["schema"] == "notrios.g14b.full-corpus-summary.v1", "wrong schema")
    require(report["selection"]["option"] in {"A", "B", "C"}, "invalid A/B/C selection")
    policy = report["acceptance_policy"]
    require(policy == {
        "maximum_stage_seconds": 7200,
        "maximum_sender_rss_bytes": 512 * 1024 * 1024,
        "maximum_receiver_rss_bytes": 256 * 1024 * 1024,
        "transport_entries_must_not_scale_per_object": True,
    }, "acceptance policy drifted")

    equivalence = report["recipe_import_equivalence"]
    require(equivalence["documents_each"] == 382_206, "recipe document count differs")
    require(equivalence["semantic_fingerprint_equal"], "semantic recipe fingerprints differ")
    require(equivalence["visible_body_multiset_equal"], "visible recipe bodies differ")
    require(equivalence["visible_body_bytes_each"] == 451_964_499, "visible byte count differs")
    require(equivalence["stored_title_differences"] == 3_682, "stored-title distinction lost")
    require(not equivalence["raw_fingerprint_equal"], "raw source-specific equality was overstated")

    attachment = report["attachment_workload"]
    require(attachment["documents"] == 103_349, "attachment document count differs")
    require(attachment["resources_imported"] == 758, "attachment resource count differs")
    require(attachment["source_bundle_items"] == 111_330, "source-bundle count differs")
    require(attachment["missing_resource_content"] == 5, "missing-resource evidence differs")

    rows = indexed_rows(report)
    for adapter in ("archive-v2-pack", "sqlite-image-bundle"):
        phases = {phase for workload, candidate, phase in rows
                  if workload == "recipe-joplin" and candidate == adapter
                  and rows[(workload, candidate, phase)]["status"] == "completed"}
        require(COMPLETE_ARCHIVE_PHASES <= phases, f"incomplete recipe phase set for {adapter}")
        for phase in COMPLETE_ARCHIVE_PHASES:
            row = rows[("recipe-joplin", adapter, phase)]
            require(row["wall_seconds"] < policy["maximum_stage_seconds"],
                    f"{adapter} {phase} exceeded two hours")
            limit = (policy["maximum_receiver_rss_bytes"] if phase in {
                "snapshot-verify", "snapshot-open", "snapshot-restore"
            } else policy["maximum_sender_rss_bytes"])
            require(row["peak_rss_bytes"] <= limit, f"{adapter} {phase} exceeded RSS gate")

    require(rows[("recipe-joplin", "archive-v2-pack", "snapshot-create")]["output_files"] < 100,
            "packed archive did not bound transport entries")
    require(rows[("recipe-joplin", "sqlite-image-bundle", "snapshot-create")]["output_files"] < 10,
            "SQLite image bundle did not bound transport entries")
    replay = rows[("recipe-joplin", "sqlite-stopped-copy", "changed-snapshot")]
    require(replay["peak_rss_bytes"] > policy["maximum_sender_rss_bytes"],
            "incremental replay RSS failure is missing")

    for adapter in ("restic-raw", "restic-canonical", "borg-raw", "borg-canonical"):
        for phase in ("snapshot-create", "snapshot-verify", "unchanged-snapshot", "snapshot-restore"):
            require(("recipe-joplin", adapter, phase) in rows,
                    f"missing repository reference row {adapter}/{phase}")
        require(rows[("recipe-joplin", adapter, "snapshot-verify")]
                ["repository_integrity_verified"], f"repository verification missing for {adapter}")
        require(rows[("recipe-joplin", adapter, "snapshot-restore")]["exact_hash_verified"],
                f"repository restore mismatch for {adapter}")

    for adapter in ("archive-v2-pack", "sqlite-image-bundle"):
        provider = rows[("recipe-joplin", adapter, "provider-copy")]
        require(provider["provider_visibility_observed"] and provider["exact_hash_verified"],
                f"provider visibility/hash evidence missing for {adapter}")

    forbidden = ("/home/", "JoplinExport_", "recipe_joplin", "recipe_vault", "recipedb_repo")
    require(not any(value in serialized for value in forbidden), "private path entered committed evidence")
    require(re.search(r"\b[0-9a-f]{64}\b", serialized) is None,
            "private full-corpus hash entered committed evidence")
    require("no measured exFAT claim" in findings, "filesystem scope is not explicit")
    require(f"Option {report['selection']['option']}" in findings, "findings omit the selected option")
    require("prototype" in readme and "no production" in readme,
            "investigation boundary is not explicit")
    print(f"g14b evidence: {len(rows)} aggregate rows and option "
          f"{report['selection']['option']} validated")


if __name__ == "__main__":
    main()
