#!/usr/bin/env python3
"""Fail-closed H0 decision/evidence validator."""
from __future__ import annotations

import hashlib
import json
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"h0 evidence: {message}")


def load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def main() -> None:
    report = load(HERE / "REPORT.json")
    audit = load(HERE / "source-audit/SOURCE_AUDIT.json")
    g18_abi = load(ROOT / "performance/v0.7-g18/ABI_CONTRACT.json")

    require(report["schema"] == "notrios.h0.investigation-report.v1", "report schema differs")
    boundary = report["boundary"]
    require(boundary["investigation_only"] and not boundary["production_code_changed"],
            "investigation crossed the production boundary")
    require(not boundary["production_store_driver_changed"] and
            not boundary["root_dependency_graph_changed"], "driver or root dependency changed")
    require(not boundary["android_or_ios_support_claimed"] and
            not boundary["private_data_read"], "unsupported platform/private-data claim")

    decisions = report["decisions"]
    require(decisions["facade_owner"]["selected"] == "internal/application" and
            decisions["facade_owner"]["blocking_for_h1_resolved"], "facade decision is open")
    sqlite = decisions["sqlite_owner"]
    require(sqlite["selected"] == "checksum-pinned upstream SQLite amalgamation with cgo" and
            sqlite["pin"]["version"] == "3.53.4", "SQLite selection/pin differs")
    require(sqlite["pin"]["archive_sha3_256"] ==
            "628a44cfe82c66aed1ccbbe85a562d2e33ebe64b3288981ed76285612227934e",
            "official archive SHA3 differs")
    required_flags = {"SQLITE_THREADSAFE=1", "SQLITE_ENABLE_FTS5", "SQLITE_DQS=0",
                      "SQLITE_OMIT_LOAD_EXTENSION", "SQLITE_SECURE_DELETE", "SQLITE_USE_URI"}
    require(required_flags.issubset(sqlite["compile_options"]), "selected hardening flags differ")

    abi = decisions["abi"]
    require(abi["major"] == 1 and abi["symbol_count"] == 12, "ABI major/symbol count differs")
    require(abi["symbols"] == g18_abi["symbols"], "H0 symbols drifted from frozen G18 ABI")
    require("c-shared" in abi["primary_build_form"] and "c-archive" in abi["secondary_build_form"],
            "ABI build forms are incomplete")
    require("no Go pointers" in abi["ownership"] and "never invokes a host callback" in abi["threads"],
            "ABI memory/thread policy weakened")

    owner = decisions["canonical_file_owner"]
    require("exactly one" in owner["policy"] and owner["negative_probe"].startswith("passed"),
            "single-owner policy/probe missing")
    require(owner["production_status"].endswith("H1"), "probe misrepresented as production enforcement")

    target = decisions["android_target"]
    require(target["runtime_min_sdk_evidence"] == 35 and target["runtime_abi"] == "x86_64",
            "Android runtime target differs")
    require(target["release_abi_build_only"] == "arm64-v8a" and
            not target["arm64_runtime_claimed"], "arm64 build/runtime boundary differs")

    provenance = report["candidate_provenance"]
    require(provenance["modernc_sqlite"]["version"] == "v1.57.0" and
            provenance["modernc_libc"]["version"] == "v1.74.4", "modernc pins differ")
    require(provenance["modernc_sqlite"]["embedded_sqlite"] == "3.53.3" and
            not provenance["modernc_sqlite"]["upstream_lists_android_support"],
            "modernc version/support boundary differs")
    nested_mod = (HERE / "sqlite-probe/go.mod").read_text(encoding="utf-8")
    nested_sum = (HERE / "sqlite-probe/go.sum").read_text(encoding="utf-8")
    require("modernc.org/sqlite v1.57.0" in nested_mod and
            "modernc.org/libc v1.74.4" in nested_mod, "nested candidate pins differ")
    require(provenance["modernc_sqlite"]["go_sum"] in nested_sum and
            provenance["modernc_libc"]["go_sum"] in nested_sum, "modernc sums differ")
    require("modernc.org/sqlite" not in (ROOT / "go.mod").read_text(encoding="utf-8"),
            "modernc escaped into the product dependency graph")

    require(audit["schema"] == "notrios.h0.source-audit.v1", "source-audit schema differs")
    require(audit["audited_source_sha256"] == report["results"]["source_audit"]["audited_source_sha256"],
            "source hash differs")
    require(audit["routes"]["registered_total_including_head_and_web_root"] == 113 and
            audit["httpapi"]["handler_methods"] == 109, "route/handler audit differs")
    require(audit["store"]["files_importing_c"] == 43 and
            audit["store"]["current_schema_version"] == 27, "store audit differs")
    require(all(item["delta"] == 0 for item in audit["drift"]), "G18 numeric source drift exists")

    results = report["results"]
    for key in ("desktop_real_store", "desktop_sqlite_ab", "android_x86_64",
                "desktop_android_desktop_roundtrip", "android_arm64_build_only"):
        require(results[key]["status"] == "passed", f"{key} did not pass")
    require(results["android_x86_64"]["modernc"]["real_store_suite"].startswith("not run"),
            "modernc real-store limitation hidden")
    require(not results["android_arm64_build_only"]["runtime_executed"],
            "arm64 runtime falsely claimed")
    require(results["desktop_android_desktop_roundtrip"]["final_database_bytes"] == 245760,
            "round-trip size differs")

    hand_header = (HERE / "abi-probe/notrios_abi.h").read_text(encoding="utf-8")
    exported = re.findall(r"\b(notrios_[a-z_]+)\s*\(", hand_header)
    require(exported == abi["symbols"], "hand ABI header symbols differ")
    storelink = (HERE / "storelink-probe/run.sh").read_text(encoding="utf-8")
    require("dynamic SQLite dependency detected" in storelink and "sqlite3_" in storelink,
            "single-engine link checks missing")
    runner = (HERE / "sqlite-probe/run.sh").read_text(encoding="utf-8")
    require(sqlite["pin"]["archive_sha3_256"] in runner and "owner-lock negative test failed" in runner,
            "SQLite provenance/owner runner differs")

    forbidden_suffixes = {".db", ".sqlite", ".so", ".a", ".o", ".zip", ".apk"}
    for path in HERE.rglob("*"):
        if path.is_file():
            require(path.suffix not in forbidden_suffixes, f"generated artifact tracked: {path.name}")
    serialized = json.dumps(report)
    require("/home/" not in serialized and "SEAGATE" not in serialized,
            "portable report contains a host-private path")
    require(len(report["limitations"]) >= 6, "limitations are incomplete")

    subprocess.run(["python3", str(HERE / "source-audit/audit.py"), "--check"],
                   cwd=ROOT, check=True)
    print("h0 evidence: facade, ABI, SQLite pin, desktop/Android matrix, round-trip, and limitations validated")


if __name__ == "__main__":
    main()
