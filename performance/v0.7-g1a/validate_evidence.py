#!/usr/bin/env python3
from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parent
REPOSITORY = ROOT.parents[1]


def load(name: str):
    return json.loads((ROOT / name).read_text(encoding="utf-8"))


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(message)


def main() -> None:
    benchmark = load("benchmark-results.json")
    interop = load("interoperability.json")
    vectors = load("vectors.json")
    documentation = "\n".join(
        (ROOT / name).read_text(encoding="utf-8")
        for name in ("README.md", "FINDINGS.md", "PROVENANCE.md")
    )

    require(benchmark["schema"] == "notrios.g1a.benchmark.v1", "wrong benchmark schema")
    require(len(benchmark["records"]) == 21, "expected 21 benchmark fixtures")
    require(benchmark["summary"]["text_fixtures"] == 14, "expected 14 text fixtures")
    require(benchmark["summary"]["binary_fixtures"] == 7, "expected seven binary fixtures")
    require(benchmark["summary"]["all_exact"], "a benchmark round trip was inexact")
    require(benchmark["summary"]["all_deterministic"], "an encoding was nondeterministic")
    require(benchmark["summary"]["beneficial_vcdiff"] == 19, "benefit count drifted")

    records = benchmark["records"]
    require(len({item["id"] for item in records}) == len(records), "duplicate fixture id")
    for item in records:
        require(item["exact_vcdiff_round_trip"], f"VCDIFF mismatch: {item['id']}")
        require(item["exact_private_round_trip"], f"private mismatch: {item['id']}")
        require(item["deterministic"], f"nondeterministic fixture: {item['id']}")
        require(len(item["source_sha256"]) == 64 and len(item["target_sha256"]) == 64, "hash missing")
        for metric_name in ("vcdiff_encode", "vcdiff_decode", "private_encode", "private_decode"):
            metric = item[metric_name]
            require(metric["cpu_nanos_p50"] >= 0, f"CPU missing: {item['id']}")
            require(metric["allocated_bytes_p50"] >= 0, f"allocation missing: {item['id']}")
            require(metric["peak_rss_kib"] > 0, f"RSS missing: {item['id']}")
    text_records = [item for item in records if item["class"] == "text"]
    require(all(item["g1_line_json_bytes"] > item["vcdiff_bytes"] for item in text_records), "G1 comparison drifted")
    nonbeneficial = {item["id"] for item in records if item["vcdiff_bytes"] >= item["complete_bytes"]}
    require(nonbeneficial == {"binary-empty", "binary-unrelated-262144"}, "fallback cases drifted")

    require(interop["schema"] == "notrios.g1a.interoperability.v1", "wrong interop schema")
    require(len(interop["fixtures"]) == 3, "expected three interop fixtures")
    require(interop["summary"]["outbound_external_decodes_exact"], "external decoder mismatch")
    require(interop["summary"]["external_encode_attempts"] == 6, "interop matrix incomplete")
    outbound = [
        tool["go_to_tool"]
        for fixture in interop["fixtures"]
        for tool in fixture["results"].values()
    ]
    inbound = [
        tool["tool_to_go"]
        for fixture in interop["fixtures"]
        for tool in fixture["results"].values()
    ]
    require(all(item["accepted"] and item["exact"] for item in outbound), "outbound interop failed")
    require(all(item["exact"] for item in inbound if item["accepted"]), "accepted inbound stream was inexact")

    require(vectors["schema"] == "notrios.g1a.vectors.v1", "wrong vector schema")
    require(len(vectors["golden_vectors"]) == 2, "golden vector matrix incomplete")
    require(len(vectors["hostile_cases"]) >= 10, "hostile vector matrix incomplete")
    require(len(vectors["declared_limits"]) >= 12, "decoder limit matrix incomplete")
    require(all(item["result"] == "passed" for item in vectors["fuzz_runs"]), "fuzz failure recorded")

    for phrase in (
        "constrained RFC 3284",
        "not Subversion svndiff",
        "complete-object fallback",
        "immutable named parent",
        "not a general VCDIFF decoder",
        "No production sync codec",
        "No GPL source",
    ):
        require(phrase in documentation, f"required conclusion missing: {phrase}")
    rendered = json.dumps({"benchmark": benchmark, "interop": interop, "vectors": vectors}, sort_keys=True)
    for forbidden in ("/home/", "JoplinExport_", "Private title", "Private body"):
        require(forbidden not in rendered, f"private/local value leaked: {forbidden}")
    go_mod = (REPOSITORY / "go.mod").read_text(encoding="utf-8")
    require("xdelta" not in go_mod and "vcdiff" not in go_mod, "runtime delta dependency added")

    print(
        "G1a evidence valid: "
        f"{len(records)} fixtures, {benchmark['summary']['beneficial_vcdiff']} beneficial VCDIFFs, "
        f"{len(outbound)} exact external decodes, {sum(item['accepted'] for item in inbound)}/{len(inbound)} constrained inbound accepts"
    )


if __name__ == "__main__":
    main()
