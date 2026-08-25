#!/usr/bin/env python3
"""Validate committed aggregate G17a evidence without reading external artifacts."""
from __future__ import annotations

import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[1]


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g17a evidence: {message}")


def load(name: str) -> dict:
    return json.loads((ROOT / name).read_text(encoding="utf-8"))


def main() -> None:
    inventory = load("INVENTORY_SUMMARY.json")
    prototype = load("PROTOTYPE_RESULTS.json")
    recursive = load("RECURSIVE_SCOPE_FINDING.json")
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    assessment = (ROOT / "TSA_ASSESSMENT.md").read_text(encoding="utf-8")
    contract = (REPO / "EVIDENCE_PRESERVATION.md").read_text(encoding="utf-8")

    require(inventory["schema"] == "notrios.evidence.inventory-summary.v1", "inventory schema differs")
    require(inventory["source_regular_files"] == 78, "frozen top-level count differs")
    require(inventory["kind_counts"] == {"png": 4, "zip": 74}, "artifact kinds differ")
    require(inventory["source_total_bytes"] == 270_506_844, "source byte total differs")
    require(inventory["validation_counts"]["valid"] == 78, "not every artifact validated")
    require(inventory["validation_counts"]["invalid"] == 0, "invalid artifact committed")
    require(inventory["validation_counts"]["current_release_shape"] == 73, "release-shape count differs")
    require(inventory["provenance_counts"] == {
        "insufficient": 1, "not_applicable": 4, "unique_exact": 73,
    }, "provenance resolution totals differ")
    require(all(value == 0 for value in inventory["sidecar_counts"].values()), "historical sidecar unexpectedly present")
    require(inventory["source_top_level_directories"] == 3, "recursive-scope trigger differs")
    require(inventory["source_iso_print_size_bytes"] == inventory["source_iso_print_size_blocks"] * 2048,
            "ISO print-size arithmetic differs")
    require(inventory["source_iso_print_size_bytes"] < inventory["conservative_cd_budget_bytes"],
            "curated source no longer fits the investigation budget")
    require(inventory["source_iso_budget_basis_points"] == 3975, "budget percentage differs")
    for field in ("content_inventory_commitment_sha256", "capture_inventory_commitment_sha256"):
        require(re.fullmatch(r"[0-9a-f]{64}", inventory[field]) is not None, f"invalid {field}")
    require(not inventory["source_directory_modified"] and not inventory["production_iso_created"],
            "investigation crossed a production boundary")

    require(prototype["schema"] == "notrios.g17a.prototype-results.v1", "prototype schema differs")
    for field in (
        "generated_only", "canonical_chain_verified", "gpg_detached_signature_verified",
        "gpg_tamper_refused", "rfc3161_query_and_response_verified",
        "rfc3161_tamper_refused", "rfc3161_wrong_ca_refused",
        "iso_sha256_equal", "iso_extract_hash_walk_equal", "joliet_long_unicode_fixture",
    ):
        require(prototype[field] is True, f"prototype assertion failed: {field}")
    require(prototype["iso_rebuilds"] == 2, "clean ISO was not built twice")
    require(prototype["iso_size_bytes"] == prototype["iso_print_size_blocks"] * 2048,
            "generated ISO print-size differs")
    require(prototype["timestamped_datum"] == "detached content-checkpoint signature",
            "wrong timestamp topology")
    require(not prototype["production_key_used"] and not prototype["production_tsa_contacted"]
            and not prototype["production_iso_created"], "prototype claims production activity")

    require(recursive["schema"] == "notrios.g17a.recursive-scope-finding.v1", "scope schema differs")
    require(recursive["contains_known_private_benchmark_workspaces"] is True, "private workspace finding lost")
    require(recursive["exact_recursive_inventory_completed"] is False, "unmeasured recursive scope claimed complete")
    require(recursive["full_recursive_inventory_commitment_sha256"] is None, "invented recursive commitment")
    require(recursive["observed_recursive_iso_nodes_at_least"] >= 47_400, "recursive lower bound lost")
    require(recursive["user_scope_decision_required"] is True, "scope decision is no longer blocking")

    require("signed checkpoint" in findings and "Remaining blocking decisions" in findings,
            "findings omit selected topology/decisions")
    require("DigiCert" in assessment and "Sectigo" in assessment and "SSL.com" in assessment,
            "provider comparison incomplete")
    for phrase in (
        "not legal advice", "No secret key was present", "650 MiB",
        "catalog-only closure commit", "G17b cannot begin", "System default roots",
    ):
        require(phrase in contract, f"contract omits: {phrase}")
    serialized = json.dumps([inventory, prototype, recursive])
    require("/home/" not in serialized and "SEAGATE" not in serialized,
            "aggregate JSON contains a host path")
    print("g17a evidence: 78 curated artifacts, 73 exact commit mappings, generated crypto/ISO contract validated")


if __name__ == "__main__":
    main()
