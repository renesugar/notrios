#!/usr/bin/env python3
from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parent


def load(name: str):
    return json.loads((ROOT / name).read_text(encoding="utf-8"))


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(message)


def main() -> None:
    corpus = load("corpus-profile.json")
    scenarios = load("scenarios.json")
    workload = load("workload-results.json")
    upstream = load("upstream-probe.json")
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(corpus["schema"] == "notrios.g1.corpus-profile.v1", "wrong corpus schema")
    require(len(corpus["corpora"]) == 3, "expected three corpus profiles")
    rendered_corpus = json.dumps(corpus, sort_keys=True)
    for forbidden in ("/home/", "JoplinExport_", "Private title", "Private body"):
        require(forbidden not in rendered_corpus, f"private value leaked: {forbidden}")
    for item in corpus["corpora"]:
        require(item["privacy"] == "aggregate-only", "corpus privacy marker missing")
        require(item["body_sizes"]["count"] > 0, "empty body distribution")
        require("inference" in item["revision_observation"], "revision limitation missing")

    intervals = scenarios["offline_intervals"]
    require(intervals == ["1-hour", "1-day", "1-week", "30-days"], "offline intervals drifted")
    require(workload["offline_interval_edit_counts"] == {
        "1-hour": 1, "1-day": 4, "1-week": 12, "30-days": 32
    }, "interval edit counts drifted")
    require(len(workload["delta_records"]) == 4 * 7 * 4, "delta matrix incomplete")
    require(len(workload["merge_records"]) == 6 * 3, "merge matrix incomplete")
    require(len(workload["scale_records"]) == 4, "large generated matrix incomplete")
    require(len(workload["patch_rejections"]) == 7, "patch rejection matrix incomplete")
    require(all(item["rejected"] for item in workload["patch_rejections"]), "a malicious patch was accepted")

    for record in workload["delta_records"]:
        require(record["result_sha256"] == record["reconstructed_sha256"], "delta reconstruction hash mismatch")
        require(record["cpu_ns_p50"] >= 0 and record["peak_rss_kib"] > 0, "delta resource evidence missing")
    for record in workload["scale_records"]:
        if record["classification"] != "limit-rejected":
            require(record["result_sha256"] == record["reconstructed_sha256"], "scale reconstruction hash mismatch")

    observed = {(item["case"], item["granularity"]): item["classification"] for item in workload["merge_records"]}
    for case in scenarios["body_cases"]:
        for unit, expected in case["expected"].items():
            require(observed[(case["id"], unit)] == expected, f"merge classification drift: {case['id']} {unit}")
    require(all(item.get("valid_utf8") for item in workload["merge_records"] if item["classification"] != "limit-rejected"), "invalid merge UTF-8")

    require(upstream["candidate_license"] == "MIT", "candidate license drifted")
    require(upstream["candidate_commit"] == "3b1669897fb1aa7c1fb2699a3c6a45bbb46e9ec1", "candidate commit drifted")
    require(all(value == "passed" for key, value in upstream["probe"].items() if key != "command"), "upstream probe failed")

    for phrase in (
        "complete UTF-8 body object",
        "named-parent",
        "bounded line-first",
        "word-token refinement",
        "Byte merge is rejected",
        "No production merge engine",
    ):
        require(phrase in findings + readme, f"selection text missing: {phrase}")

    print(
        "G1 evidence valid: "
        f"{sum(item['body_sizes']['count'] for item in corpus['corpora']):,} bodies, "
        f"{len(workload['delta_records'])} delta records, "
        f"{len(workload['merge_records'])} merge records, "
        f"{len(workload['patch_rejections'])} rejected patches"
    )


if __name__ == "__main__":
    main()
