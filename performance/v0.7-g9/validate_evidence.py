#!/usr/bin/env python3
"""Check that the committed G9 evidence says what the documentation claims."""
from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g9 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "wire-results.json").read_text(encoding="utf-8"))
    goldens = json.loads((ROOT / "goldens.json").read_text(encoding="utf-8"))
    inputs = json.loads((ROOT / "golden-inputs.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    fmt = (ROOT / "FORMAT.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g9.wire.v1", "wrong report schema")
    require(report["protocol"] == "1.0", "protocol is not 1.0")

    primitives = report["primitives"]
    require(primitives["external_dependencies"] == 0,
            "G9 claims a dependency; the license inventory says the standard library only")
    for name in ("aead", "kdf", "signature", "routing_blind"):
        require(primitives.get(name), f"the {name} primitive is not recorded")

    require(len(goldens["cases"]) == len(inputs["cases"]) and goldens["cases"],
            "goldens and their inputs are out of step")
    for case, given in zip(goldens["cases"], inputs["cases"]):
        require(case["name"] == given["name"], f"golden {case['name']} does not match its input")
        require(len(case["hex"]) == 2 * case["byte_length"], f"{case['name']}: hex length disagrees with byte_length")
        require(case["byte_length"] > 0, f"{case['name']}: empty golden")

    tiers = report["tiers"]
    require(len(tiers) >= 2, "expected at least two operation tiers")
    for row in tiers:
        label = f"{row['operations']} operations"
        require(row["round_trip_exact"], f"{label}: the artifact did not round trip exactly")
        require(row["canonical_bytes"] < row["json_bytes"], f"{label}: the canonical encoding is not smaller")
        require(row["canonical_gzip_bytes"] < row["json_gzip_bytes"], f"{label}: compressed, it is not smaller")
        # The saving G2 selected on must still hold, in both columns.
        require(row["canonical_smaller_percent"] > 40, f"{label}: raw saving fell to {row['canonical_smaller_percent']}%")
        require(row["canonical_gzip_smaller_percent"] > 10, f"{label}: compressed saving fell to {row['canonical_gzip_smaller_percent']}%")
        # Crypto overhead must stay constant rather than scaling with content.
        require(0 < row["artifact_overhead_bytes"] < 512,
                f"{label}: artifact overhead is {row['artifact_overhead_bytes']} bytes")
        require(row["artifact_bytes"] == row["canonical_bytes"] + row["artifact_overhead_bytes"],
                f"{label}: artifact size does not add up")

    overheads = {row["artifact_overhead_bytes"] for row in tiers}
    require(max(overheads) - min(overheads) <= 8,
            f"crypto overhead varies with envelope size: {sorted(overheads)}")

    for row in tiers:
        for value in (row["json_bytes"], row["canonical_bytes"], row["artifact_overhead_bytes"]):
            require(f"{value:,}" in findings or str(value) in findings,
                    f"FINDINGS.md does not mention {value}")

    require("notrios.artifact-signature.v1" in fmt, "FORMAT.md does not pin the signature domain")
    require("HKDF" in fmt and "AES-256-GCM" in fmt, "FORMAT.md does not name the primitives")
    require(re.search(r"secret store", findings, re.IGNORECASE), "FINDINGS.md does not state the secret-store boundary")
    require("BSD-3-Clause" in readme, "README.md does not carry the license inventory")

    print(f"g9 evidence: {len(tiers)} tiers and {len(goldens['cases'])} goldens validated")


if __name__ == "__main__":
    main()
