#!/usr/bin/env python3
"""Validate G18's finite handoff against the checked-in source tree."""
from __future__ import annotations

import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[1]


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g18 evidence: {message}")


def load(name: str) -> dict:
    return json.loads((ROOT / name).read_text(encoding="utf-8"))


def production_go_files(root: Path) -> list[Path]:
    return sorted(path for path in root.glob("*.go") if not path.name.endswith("_test.go"))


def main() -> None:
    audit = load("SOURCE_AUDIT.json")
    platform = load("PLATFORM_MATRIX.json")
    abi = load("ABI_CONTRACT.json")
    mermaid = load("MERMAID_CONTRACT.json")
    android = load("ANDROID_FEASIBILITY.json")

    require(audit["schema"] == "notrios.g18.source-audit.v1", "source-audit schema differs")
    require(platform["schema"] == "notrios.g18.platform-matrix.v1", "platform schema differs")
    require(abi["schema"] == "notrios.g18.abi-contract.v1", "ABI schema differs")
    require(mermaid["schema"] == "notrios.g18.mermaid-contract.v1", "Mermaid schema differs")
    require(android["schema"] == "notrios.g18.android-feasibility.v1", "Android schema differs")

    routes: list[tuple[str, str]] = []
    for path in production_go_files(REPO / "internal" / "httpapi"):
        routes.extend(re.findall(r'HandleFunc\("(GET|POST|PUT|PATCH|DELETE|HEAD) ([^"]+)"',
                                 path.read_text(encoding="utf-8")))
    require(len(routes) == audit["routes"]["registered_total_including_head_and_web_root"],
            "registered route total differs")
    require(sum(method == "HEAD" for method, _ in routes) == audit["routes"]["registered_head"],
            "HEAD route total differs")
    require(sum(method != "HEAD" and route != "/" for method, route in routes) ==
            audit["routes"]["openapi_normalized_non_head"], "normalized API route total differs")

    wails_files = []
    for path in REPO.rglob("*.go"):
        if any(part in {".git", "node_modules"} for part in path.parts):
            continue
        if "github.com/wailsapp/wails" in path.read_text(encoding="utf-8"):
            wails_files.append(path.relative_to(REPO).as_posix())
    require(sorted(wails_files) == audit["dependencies"]["wails_import_files"],
            "Wails escaped its build-tagged adapter")
    wails_source = (REPO / wails_files[0]).read_text(encoding="utf-8")
    require(wails_source.startswith("//go:build gui\n"), "Wails adapter lost its gui build tag")

    cgo_files = [path for path in (REPO / "internal" / "store").glob("*.go")
                 if re.search(r'^import "C"', path.read_text(encoding="utf-8"), re.MULTILINE)]
    require(len(cgo_files) == audit["dependencies"]["store_files_importing_c"],
            "store cgo file count differs")
    service = (REPO / "internal" / "service" / "service.go").read_text(encoding="utf-8")
    require("*httpapi.Server" in service and "*http.Server" in service,
            "service/HTTP coupling finding no longer holds; rerun the facade audit")
    require("internal/service" not in "\n".join(
        path.read_text(encoding="utf-8") for path in production_go_files(REPO / "internal" / "httpapi")),
        "HTTP adapter acquired a service import cycle")

    statuses = set(platform["status_values"])
    require(len(platform["capabilities"]) == 19, "platform matrix is no longer finite at 19 capabilities")
    require(len({item["id"] for item in platform["capabilities"]}) == 19,
            "platform capability IDs are not unique")
    for item in platform["capabilities"]:
        for target in platform["platforms"]:
            require(item[target] in statuses, f"invalid platform status for {item['id']}:{target}")

    required_symbols = {"notrios_instance_open", "notrios_instance_close", "notrios_call_cancel",
                        "notrios_event_poll", "notrios_stream_read", "notrios_buffer_release"}
    require(required_symbols.issubset(abi["symbols"]), "ABI lifecycle/ownership surface is incomplete")
    require(abi["callbacks_from_go_threads"] is False, "ABI permits arbitrary Go-thread callbacks")
    require(all(value <= 1048576 for key, value in abi["bounds"].items() if key.endswith("bytes")),
            "ABI byte bound exceeds the one-MiB dispatch/stream ceiling")
    require(len(abi["mandatory_tests"]) >= 10, "ABI negative test contract is incomplete")

    editor_assets = (REPO / mermaid["current"]["source"]).read_text(encoding="utf-8")
    require(mermaid["current"]["required_literal"] in editor_assets,
            "current GUI no longer has the audited Mermaid-disabled baseline")
    require(mermaid["current"]["enabled"] is False, "evidence falsely claims Mermaid enabled")
    require(len(mermaid["fixtures"]) >= 14, "Mermaid fixture contract is incomplete")
    require(android["configured_avds"] == 0 and android["cross_compile"]["passed"] is False,
            "Android evidence overclaims the available environment")
    require(android["flutter_build_claimed"] is False, "G18 falsely claims a Flutter build")

    serialized = json.dumps([audit, platform, abi, mermaid, android])
    require("/home/" not in serialized and "SEAGATE" not in serialized,
            "portable evidence contains a host-private path")
    require(audit["private_data_read"] is False and audit["production_code_changed"] is False,
            "G18 crossed its investigation/handoff boundary")
    print("g18 evidence: 109 API operations, 19 platform capabilities, ABI/Mermaid/Android handoff validated")


if __name__ == "__main__":
    main()
