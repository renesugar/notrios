#!/usr/bin/env python3
from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parent
REPOSITORY = ROOT.parents[1]


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(message)


def main() -> None:
    evidence = json.loads((ROOT / "benchmark-results.json").read_text(encoding="utf-8"))
    inputs = json.loads((ROOT / "corpus-inputs.json").read_text(encoding="utf-8"))
    docs = "\n".join(
        (ROOT / name).read_text(encoding="utf-8")
        for name in (
            "README.md",
            "FINDINGS.md",
            "FORMAT.md",
            "ANDROID_EMULATOR_CHECKLIST.md",
            "PHYSICAL_DEVICE_CHECKLIST.md",
        )
    )

    require(evidence["schema"] == "notrios.g2.bounds-evidence.v1", "wrong evidence schema")
    require(inputs["schema"] == "notrios.g2.aggregate-inputs.v1", "wrong input schema")
    tiers = evidence["operation_tiers"]
    require(len(tiers) == 6, "expected two formats at three tiers")
    require({row["operations"] for row in tiers} == {100, 10_000, 100_000}, "tier coverage drifted")
    require({row["format"] for row in tiers} == {"canonical-jsonl", "compact-ncb1"}, "format matrix drifted")
    require(all(row["exact"] and row["deterministic"] for row in tiers), "codec exactness/determinism failed")
    require(all(row["encode"]["peak_rss_kib"] > 0 for row in tiers), "RSS evidence missing")

    by_key = {(row["format"], row["operations"]): row for row in tiers}
    json10 = by_key[("canonical-jsonl", 10_000)]
    compact10 = by_key[("compact-ncb1", 10_000)]
    require(compact10["compressed_bytes"] <= json10["compressed_bytes"] * 0.85, "compact compressed benefit is no longer material")
    require(compact10["decode"]["wall_nanos_p50"] <= json10["decode"]["wall_nanos_p50"] * 0.20, "compact decode benefit drifted")
    require(compact10["decode"]["allocated_bytes_p50"] <= json10["decode"]["allocated_bytes_p50"] * 0.25, "compact allocation benefit drifted")
    require(by_key[("compact-ncb1", 100_000)]["proposed_envelope_count"] == 10, "100k batching drifted")

    limits = evidence["selected_limits"]
    require(limits["envelope_operations"] == 10_000, "operation limit drifted")
    require(limits["envelope_encoded_bytes"] == 16 << 20, "encoded limit drifted")
    require(limits["envelope_compressed_bytes"] == 4 << 20, "compressed limit drifted")
    require(limits["whole_resource_below_bytes"] == 1 << 20, "whole threshold drifted")
    require(limits["fixed_chunk_bytes"] == 1 << 20, "chunk size drifted")
    require(limits["maximum_resource_chunks"] == 16_384, "chunk count drifted")

    require(len(evidence["resource_policies"]) == 9, "resource policy matrix incomplete")
    selected = [row for row in evidence["resource_policies"] if row["whole_below"] == 1 << 20 and row["chunk_bytes"] == 1 << 20]
    require(len(selected) == 1, "selected resource policy missing")
    require(selected[0]["cases"]["attachment-heavy"]["objects"] == 103, "attachment chunk estimate drifted")
    require(len(evidence["pack_reuse"]) == 2, "archive pack evidence missing")
    require(all(row["file_reduction"] > 7_000 for row in evidence["pack_reuse"]), "pack file reduction drifted")
    require(len(evidence["hostile_cases"]) >= 5, "hostile matrix incomplete")
    require(all(row["rejected"] for row in evidence["hostile_cases"]), "hostile case was accepted")

    for phrase in (
        "Select the compact NCB1",
        "10,000 operations",
        "16 MiB",
        "4 MiB",
        "64:1",
        "1 MiB fixed chunks",
        "FastCDC remains deferred",
        "no desktop-to-mobile claim",
        "No production sync codec",
    ):
        require(phrase in docs, f"required conclusion missing: {phrase}")

    rendered = json.dumps({"evidence": evidence, "inputs": inputs}, sort_keys=True)
    for forbidden in ("/home/", "JoplinExport_", "source_path", "content_sha256", "Private body"):
        require(forbidden not in rendered, f"private/local value leaked: {forbidden}")
    go_mod = (REPOSITORY / "go.mod").read_text(encoding="utf-8")
    require("vcdiff" not in go_mod and "xdelta" not in go_mod, "external delta dependency added")

    print(
        "G2 evidence valid: 6 operation tiers, 9 resource policies, "
        f"{len(evidence['hostile_cases'])} hostile rejections, compact NCB1 selected"
    )


if __name__ == "__main__":
    main()
