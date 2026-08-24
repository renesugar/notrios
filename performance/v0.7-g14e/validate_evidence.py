#!/usr/bin/env python3
"""Validate the aggregate-only production acceptance and format freeze."""
from __future__ import annotations

import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parent
REPORT = ROOT / "FULL_SCALE_ACCEPTANCE.json"
EXPECTED_PHASES = {
    "corpus-equivalence",
    "semantic-current-create", "semantic-current-verify", "semantic-current-restore",
    "semantic-previous-verify", "semantic-previous-restore",
    "recipe-physical-first", "recipe-physical-verify", "recipe-physical-unchanged",
    "recipe-physical-restore", "attachment-physical-first", "attachment-physical-verify",
    "attachment-physical-unchanged", "attachment-physical-restore", "catchup",
    "restic-canonical-check", "restic-raw-check", "borg-canonical-check", "borg-raw-check",
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g14e evidence: {message}")


def main() -> None:
    report = json.loads(REPORT.read_text(encoding="utf-8"))
    serialized = json.dumps(report, sort_keys=True)
    require(report["schema"] == "notrios.g14e.full-scale-acceptance.v1", "wrong schema")
    require(report["acceptance_policy"] == {
        "maximum_stage_seconds": 7200,
        "maximum_sender_rss_bytes": 512 * 1024 * 1024,
        "maximum_receiver_rss_bytes": 256 * 1024 * 1024,
        "transport_entries_must_not_scale_per_object": True,
    }, "acceptance policy drifted")
    rows = {row["phase"]: row for row in report["rows"]}
    require(set(rows) == EXPECTED_PHASES and len(rows) == len(report["rows"]), "phase matrix is incomplete or duplicated")
    for phase, row in rows.items():
        require(row["wall_seconds"] >= 0 and row["peak_rss_bytes"] > 0, f"invalid metrics for {phase}")
        require(row["wall_seconds"] < 7200, f"stage-time gate failed for {phase}")
        for key, value in row.items():
            if key not in {"phase", "wall_seconds", "user_cpu_seconds", "system_cpu_seconds", "peak_rss_bytes"} and isinstance(value, bool):
                require(value, f"assertion {key} failed for {phase}")

    for phase in {
        "semantic-current-verify", "semantic-current-restore", "semantic-previous-verify",
        "semantic-previous-restore", "recipe-physical-verify", "recipe-physical-restore",
        "attachment-physical-verify", "attachment-physical-restore",
    }:
        require(rows[phase]["peak_rss_bytes"] <= 256 * 1024 * 1024, f"receiver RSS gate failed for {phase}")
    for phase in {
        "semantic-current-create", "recipe-physical-first", "recipe-physical-unchanged",
        "attachment-physical-first", "attachment-physical-unchanged", "catchup",
    }:
        require(rows[phase]["peak_rss_bytes"] <= 512 * 1024 * 1024, f"sender RSS gate failed for {phase}")
    require(rows["semantic-current-create"]["output_files"] < 100, "semantic packed shape is unbounded")
    require(rows["recipe-physical-first"]["output_files"] < 100, "recipe physical shape is unbounded")
    require(rows["attachment-physical-first"]["output_files"] < 100, "attachment physical shape is unbounded")

    corpora = report["corpora"]
    require(corpora["counts"] == {
        "equivalent_joplin_documents": 382_206,
        "equivalent_obsidian_documents": 382_206,
        "attachment_documents": 103_349,
        "attachment_resources": 758,
        "attachment_blobs": 731,
        "attachment_source_bundle_items": 111_330,
    }, "corpus counts changed")
    require(set(corpora["frozen_import_seconds"]) == {
        "equivalent_joplin_view", "equivalent_obsidian_view", "attachment_workload",
    }, "public import timing labels changed")
    require(all(corpora["assertions"].values()), "corpus equivalence assertion failed")
    require(len(report["frozen_repository_baseline_rows"]) == 16, "frozen Restic/Borg rows changed")
    freeze = report["format_freeze"]
    require(freeze["whole_library_default"] == "sqlite-image+packed-assets.v1", "whole-library format changed")
    require(freeze["semantic_portable_format"] == "archive-v2" and not freeze["new_format_introduced"], "semantic compatibility changed")
    require(report["claims"] == {"aggregate_only": True, "cloud_provider_rerun": False,
            "exfat_measured": False, "physical_mobile_measured": False}, "scope claims drifted")

    forbidden = ("/home/", "JoplinExport_", "recipe_joplin", "recipe_vault", "recipedb")
    require(not any(value in serialized for value in forbidden), "private path entered evidence")
    require(re.search(r"\b[0-9a-f]{64}\b", serialized) is None, "private full-corpus hash entered evidence")
    print(f"g14e evidence: {len(rows)} aggregate phases and frozen production format validated")


if __name__ == "__main__":
    main()
