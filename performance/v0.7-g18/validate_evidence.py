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
    editor_search = load("EDITOR_SEARCH_QA.json")
    android_sqlite = load("ANDROID_SQLITE_FOLLOWUP.json")
    modernc_sqlite = load("MODERNC_SQLITE_EVALUATION.json")
    android_emulator = load("ANDROID_EMULATOR_FOLLOWUP.json")

    require(audit["schema"] == "notrios.g18.source-audit.v1", "source-audit schema differs")
    require(platform["schema"] == "notrios.g18.platform-matrix.v1", "platform schema differs")
    require(abi["schema"] == "notrios.g18.abi-contract.v1", "ABI schema differs")
    require(mermaid["schema"] == "notrios.g18.mermaid-contract.v1", "Mermaid schema differs")
    require(android["schema"] == "notrios.g18.android-feasibility.v1", "Android schema differs")
    require(editor_search["schema"] == "notrios.g18.editor-search-qa.v1",
            "editor-search QA schema differs")
    require(android_sqlite["schema"] == "notrios.g18.android-sqlite-followup.v1",
            "Android SQLite follow-up schema differs")
    require(modernc_sqlite["schema"] == "notrios.g18.modernc-sqlite-evaluation.v1",
            "modernc SQLite evaluation schema differs")
    require(android_emulator["schema"] == "notrios.g18.android-emulator-followup.v1",
            "Android emulator follow-up schema differs")

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

    # One file in the production module may import Wails, and the pinned list
    # below is that file. A nested module is not in the production module: its
    # directory carries its own go.mod, so `go build ./...` at the root cannot
    # reach it and nothing it imports can reach the shipped binary. Skipping
    # those keeps this check saying what it means -- v0.8 H10's Wails v3 spike
    # is exactly such a module, and adding its files to the list instead would
    # have turned "only the adapter imports Wails" into "these three files do".
    nested_modules = [module.parent for module in REPO.rglob("go.mod")
                      if module.parent != REPO]
    wails_files = []
    for path in REPO.rglob("*.go"):
        if any(part in {".git", "node_modules"} for part in path.parts):
            continue
        if any(nested in path.parents for nested in nested_modules):
            continue
        if "github.com/wailsapp/wails" in path.read_text(encoding="utf-8"):
            wails_files.append(path.relative_to(REPO).as_posix())
    require(sorted(wails_files) == audit["dependencies"]["wails_import_files"],
            "Wails escaped its build-tagged adapter")
    wails_source = (REPO / wails_files[0]).read_text(encoding="utf-8")
    require(wails_source.startswith("//go:build gui\n"), "Wails adapter lost its gui build tag")

    # G18 recorded 43 store files that directly use the SQLite C API. v0.8 H1
    # slice C added internal/store/sqlite_cgo.go, which imports "C" solely to
    # carry the #cgo build directives for the vendored amalgamation and calls
    # no SQLite function. The recorded finding is unchanged, so the file is
    # excluded here rather than the frozen evidence being rewritten. Any other
    # new cgo file still fails this check.
    build_owner = "sqlite_cgo.go"
    cgo_files = [path for path in (REPO / "internal" / "store").glob("*.go")
                 if re.search(r'^import "C"', path.read_text(encoding="utf-8"), re.MULTILINE)]
    api_users = [path for path in cgo_files if path.name != build_owner]
    require(len(api_users) == audit["dependencies"]["store_files_importing_c"],
            "store cgo file count differs")
    owner = REPO / "internal" / "store" / build_owner
    require(owner.exists() and "C.sqlite3_" not in owner.read_text(encoding="utf-8"),
            "the cgo build owner started calling the SQLite API; it must only carry directives")
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
    require(android["configured_avds"] == 2 and android["emulator_execution_attempted"] is True and
            android["cross_compile"]["passed"] is False,
            "Android environment/runtime evidence differs")
    emulator_execution = android["emulator_execution"]
    require(emulator_execution["api_level"] == 35 and
            emulator_execution["abi"] == "x86_64" and
            emulator_execution["runtime_probe_passed"] is True and
            emulator_execution["reboot_persistence_probe_passed"] is True and
            emulator_execution["c_shared_dlopen_passed"] is True and
            emulator_execution["stopped_after_probe"] is True,
            "Android API-35 runtime probe differs")
    host_sqlite = android["cross_compile"]["host_debian_sqlite_development"]
    require(host_sqlite["header_found"] is True and host_sqlite["android_target_usable"] is False,
            "host SQLite development files are confused with an Android-target library")
    require(android["cross_compile"]["host_header_followup"]["passed"] is False,
            "host-header follow-up result differs")
    doctor = android["flutter_doctor"]
    require(doctor["android_toolchain_passed"] is True and doctor["connected_android_devices"] == 0,
            "Flutter Doctor Android/device evidence differs")
    require(doctor["available_android_avds"] == 2,
            "Flutter AVD inventory differs")
    require(android["flutter_build_claimed"] is False, "G18 falsely claims a Flutter build")
    require(doctor["linux_desktop_toolchain_passed"] is True and
            doctor["linux_desktop_blocker"] is None and doctor["no_issues_found"] is True,
            "Flutter Doctor follow-up is not the all-passing result")
    host_tools = android["host_toolchain_followup"]
    require(host_tools["clang_path"] == "/usr/bin/clang" and
            host_tools["clangxx_path"] == "/usr/bin/clang++",
            "Clang PATH follow-up differs")
    require(host_tools["swiftly_on_path"] is True and
            host_tools["swift_toolchain_selected"] is False,
            "Swiftly PATH/selection finding differs")

    package_lock = json.loads((REPO / "web" / "package-lock.json").read_text(encoding="utf-8"))
    for package, version in editor_search["versions"].items():
        require(package_lock["packages"][f"node_modules/{package}"]["version"] == version,
                f"editor-search dependency version drifted: {package}")
    md_editor_path = (REPO / "web" / "node_modules" / "md-editor-rt" / "lib" / "es" /
                      "chunks" / "Editor.mjs")
    cm_search_path = (REPO / "web" / "node_modules" / "@codemirror" / "search" / "dist" /
                      "index.js")
    editor_pane_source = (REPO / "web" / "src" / "components" / "EditorPane.tsx").read_text(
        encoding="utf-8")
    require("@codemirror/search" in
            package_lock["packages"]["node_modules/md-editor-rt"]["dependencies"],
            "md-editor-rt lock entry no longer depends on CodeMirror search")
    # The Go-only CI job deliberately has no node_modules. Recheck installed
    # source when available, while the exact lock versions and browser result
    # remain portable evidence in a clean checkout.
    if md_editor_path.exists() and cm_search_path.exists():
        md_editor_source = md_editor_path.read_text(encoding="utf-8")
        cm_search_source = cm_search_path.read_text(encoding="utf-8")
        require('from "@codemirror/search"' in md_editor_source,
                "md-editor-rt no longer imports CodeMirror search")
        require('{ key: "Mod-f", run: openSearchPanel' in cm_search_source,
                "CodeMirror default search-open key drifted")
        require('key: "Mod-h"' not in cm_search_source and 'key: "Mod-H"' not in cm_search_source,
                "CodeMirror now has a Mod-H binding; refresh the shortcut conclusion")
        require('button("replaceAll"' in cm_search_source and 'name: "word"' in cm_search_source and
                'name: "case"' in cm_search_source and 'name: "re"' in cm_search_source,
                "CodeMirror search-panel controls drifted")
    require("<MdEditor" in editor_pane_source and "readOnly={!editable}" in editor_pane_source,
            "Notrios editor integration drifted")
    qa = editor_search["rendered_browser_qa"]
    require(qa["ctrl_f_opened_panel"] is True and qa["single_replace_passed"] is True and
            qa["whole_word_replace_all_passed"] is True and qa["console_errors_or_warnings"] == 0,
            "rendered editor search/replace QA did not pass")

    sqlite_source = (REPO / "internal" / "store" / "sqlite.go").read_text(encoding="utf-8")
    # Same 43 SQLite-API files as above; the build owner is excluded for the
    # reason given there. This record's link_contract field still reads
    # "pkg-config: sqlite3" because it is a frozen G18 snapshot of the
    # pre-vendoring state, which v0.8 H1 slice C deliberately replaced.
    require(android_sqlite["current_notrios_store"]["store_files_importing_c"] == len(api_users),
            "Android SQLite follow-up cgo inventory drifted")
    require("SQLITE_OPEN_FULLMUTEX" in sqlite_source and "PRAGMA journal_mode = WAL" in sqlite_source,
            "current store connection contract drifted")
    require("USING fts5" in (REPO / "internal" / "store" / "migrations" /
                             "0001_initial.sql").read_text(encoding="utf-8"),
            "FTS5 is no longer a required store feature")
    require(any("json_valid" in path.read_text(encoding="utf-8")
                for path in (REPO / "internal" / "store" / "migrations").glob("*.sql")),
            "JSON SQL functions are no longer a required store feature")
    approach_fits = {item["id"]: item["fit"] for item in android_sqlite["approaches"]}
    require(approach_fits["pinned_upstream_amalgamation_in_go_core"] ==
            "required C baseline for H0 comparison", "Android SQLite C baseline drifted")
    require(approach_fits["pinned_modernc_sqlite_in_go_core"] ==
            "promising H0 candidate; not selected", "modernc Android candidate drifted")
    require(approach_fits["jetpack_bundled_sqlite_driver"] ==
            "not a drop-in dependency for the selected Go core",
            "Jetpack driver is misrepresented as satisfying cgo")
    require(android_sqlite["android_facts"]["ndk_public_sqlite_c_api"] is False,
            "evidence falsely claims SQLite is a public NDK C API")
    require(len(android_sqlite["blocking_v0_8_decisions"]) >= 5 and
            len(android_sqlite["required_investigation_gates"]) >= 8,
            "Android SQLite investigation is underspecified")

    package = modernc_sqlite["package"]
    require(package["module"] == "modernc.org/sqlite" and package["version"] == "v1.57.0",
            "modernc probe pin differs")
    require(package["exact_modernc_libc_version"] == "v1.74.4",
            "modernc libc probe pin differs")
    require(package["license_compatible_with_notrios"] is True,
            "modernc license conclusion differs")
    probe = modernc_sqlite["disposable_probe"]
    require(probe["repository_dependency_changed"] is False,
            "modernc evaluation crossed the dependency boundary")
    require(probe["native_linux"]["runtime"] == "passed" and
            probe["native_linux"]["fts5_create_insert_match"] == "passed" and
            probe["native_linux"]["json_extract"] == "passed",
            "modernc native feature probe differs")
    require(probe["android_arm64"]["ordinary_executable_build"].startswith("passed") and
            probe["android_arm64"]["c_shared_build"].startswith("passed") and
            probe["android_arm64"]["upstream_documented_support"] is False and
            probe["android_arm64"]["runtime_executed"] is False,
            "modernc Android build-only boundary differs")
    android_x86 = probe["android_x86_64_api35"]
    require(android_x86["sqlite_version"] == "3.53.3" and
            android_x86["fts5_create_insert_match"] == "passed" and
            android_x86["json_extract"] == "passed" and
            android_x86["wal_file_mode"] == "passed" and
            android_x86["post_reboot_fts5_count"] == 3 and
            android_x86["post_reboot_integrity_check"] == "ok" and
            android_x86["c_shared_dlopen"] == "passed" and
            android_x86["upstream_documented_support"] is False,
            "modernc Android x86_64 runtime boundary differs")
    require(probe["browser_js_wasm"]["build"] == "failed" and
            probe["browser_js_wasm"]["upstream_documented_support"] is False,
            "modernc Web conclusion differs")
    require(modernc_sqlite["linux_file_interoperability_probe"]["result"] == "passed",
            "modernc C file round-trip differs")
    require(len(modernc_sqlite["blocking_h0_gates"]) >= 10,
            "modernc H0 comparison is underspecified")
    require(len(modernc_sqlite["satisfied_h0_pregates"]) == 4,
            "modernc completed pre-gate inventory differs")
    root_go_mod = (REPO / "go.mod").read_text(encoding="utf-8")
    require("modernc.org/sqlite" not in root_go_mod,
            "modernc was added to the product before H0 selection")

    require(android_emulator["host"]["java_home_overridden"] is False and
            android_emulator["runtime"]["stopped_after_probe"] is True and
            android_emulator["modernc_runtime"]["emulator_reboot_persistence"] == "passed" and
            android_emulator["user_configuration_needed"] == [],
            "Android emulator follow-up crossed its stated boundary")

    serialized = json.dumps([audit, platform, abi, mermaid, android, editor_search,
                             android_sqlite, modernc_sqlite, android_emulator])
    require("/home/" not in serialized and "SEAGATE" not in serialized,
            "portable evidence contains a host-private path")
    require(audit["private_data_read"] is False and audit["production_code_changed"] is False,
            "G18 crossed its investigation/handoff boundary")
    print("g18 evidence: 109 API operations, 19 platform capabilities, ABI/Mermaid/Android emulator/editor/modernc follow-up validated")


if __name__ == "__main__":
    main()
