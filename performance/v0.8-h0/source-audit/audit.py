#!/usr/bin/env python3
"""Audit the current production seams relevant to H0.

The scanner deliberately uses only the Python standard library and source text.
It is a change detector, not a replacement for Go's type checker: generated
JSON records the exact files and simple structural signals that determine the
facade migration cost.  Run from the repository root with ``--write`` to
refresh the checked-in evidence, or ``--check`` in CI.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
BASELINE = ROOT / "performance/v0.7-g18/SOURCE_AUDIT.json"
OUTPUT = Path(__file__).resolve().parent / "SOURCE_AUDIT.json"


def go_files(directory: Path) -> list[Path]:
    return sorted(p for p in directory.rglob("*.go") if p.is_file())


def production(files: list[Path]) -> list[Path]:
    return [p for p in files if not p.name.endswith("_test.go")]


def text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def count_files(files: list[Path], pattern: str) -> int:
    rx = re.compile(pattern, re.MULTILINE)
    return sum(bool(rx.search(text(p))) for p in files)


def count_matches(files: list[Path], pattern: str) -> int:
    rx = re.compile(pattern, re.MULTILINE)
    return sum(len(rx.findall(text(p))) for p in files)


def import_graph(files: list[Path]) -> dict[str, list[str]]:
    graph: dict[str, list[str]] = {}
    for path in files:
        package = str(path.relative_to(ROOT).parent)
        imports = re.findall(r'"(github\.com/renesugar/notrios/internal/[^"]+)"', text(path))
        graph.setdefault(package, []).extend(imports)
    return {k: sorted(set(v)) for k, v in sorted(graph.items())}


def interface_method_count(content: str, name: str) -> int:
    match = re.search(r"type\s+" + re.escape(name) + r"\s+interface\s*\{", content)
    if not match:
        return 0
    depth = 1
    end = match.end()
    while end < len(content) and depth:
        if content[end] == "{":
            depth += 1
        elif content[end] == "}":
            depth -= 1
        end += 1
    body = content[match.end() : end - 1]
    # Interfaces in this project use one method per line.  Embedded interfaces
    # are excluded because they have no opening parenthesis on the line.
    return sum(bool(re.match(r"\s*[A-Z]\w*\s*\(", line)) for line in body.splitlines())


def command_value(args: list[str], fallback: str = "unknown") -> str:
    try:
        return subprocess.check_output(args, cwd=ROOT, text=True).strip()
    except (OSError, subprocess.CalledProcessError):
        return fallback


def audited_source_sha256(paths: list[Path]) -> str:
    """Hash sorted relative paths and bytes, making evidence commit-independent."""
    digest = hashlib.sha256()
    for path in sorted(paths):
        relative = path.relative_to(ROOT).as_posix().encode("utf-8")
        payload = path.read_bytes()
        digest.update(len(relative).to_bytes(8, "big"))
        digest.update(relative)
        digest.update(len(payload).to_bytes(8, "big"))
        digest.update(payload)
    return digest.hexdigest()


def package_count(import_path: str) -> int | None:
    try:
        output = subprocess.check_output(["go", "list", "-deps", import_path], cwd=ROOT, text=True)
    except (OSError, subprocess.CalledProcessError):
        return None
    return len([line for line in output.splitlines() if line.strip()])


def build() -> dict:
    http_files = production(go_files(ROOT / "internal/httpapi"))
    service_files = production(go_files(ROOT / "internal/service"))
    store_files = production(go_files(ROOT / "internal/store"))
    all_files = production(go_files(ROOT / "internal"))
    cmd_files = production(go_files(ROOT / "cmd"))
    server = ROOT / "internal/httpapi/server.go"
    server_text = text(server)
    routes = re.findall(r'\.HandleFunc\(\s*"(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+([^" ]+)',
                        "\n".join(text(p) for p in http_files))
    openapi_routes = [(method, path) for method, path in routes if method != "HEAD" and path != "/"]
    head = sum(method == "HEAD" for method, _ in routes)
    non_head = len(routes) - head
    store_text = "\n".join(text(p) for p in store_files)
    graph = import_graph(all_files)
    wails_files = sorted(str(p.relative_to(ROOT)) for p in all_files + cmd_files if "wails" in p.name.lower())
    service_imports_httpapi = any("internal/httpapi" in imp for imp in graph.get("internal/service", []))
    httpapi_imports_service = any("internal/service" in imp for imp in graph.get("internal/httpapi", []))
    return {
        "schema": "notrios.h0.source-audit.v1",
        "audited_source_sha256": audited_source_sha256(all_files + cmd_files),
        "go_toolchain": command_value(["go", "version"]),
        "scope": {"production_go_files": len(all_files), "tests_excluded": True},
        "routes": {
            "registered_total_including_head_and_web_root": len(routes),
            "registered_head": head,
            "registered_non_head_including_web_root": non_head,
            "openapi_normalized_non_head": len(openapi_routes),
            "web_root_excluded_from_openapi": any(path == "/" for _, path in routes),
            "route_declaration_files": sorted(str(p.relative_to(ROOT)) for p in http_files if ".HandleFunc(" in text(p)),
        },
        "httpapi": {
            "production_files": len(http_files),
            "files_importing_net_http": count_files(http_files, r'"net/http"'),
            "handler_methods": count_matches(http_files, r"^func\s+\(s \*Server\)\s+handle[A-Z]\w*\s*\("),
            "decode_json_calls": count_matches(http_files, r"\bdecodeJSON\s*\("),
            "direct_store_reference_files": count_files(http_files, r"\bs\.store\s*\."),
            "direct_store_reference_sites": count_matches(http_files, r"\bs\.store\s*\."),
            "sync_concrete_store_reference_sites": count_matches(http_files, r"\bs\.sync\.store\s*\."),
            "concrete_sqlite_type_sites": count_matches(http_files, r"\*store\.SQLiteStore"),
            "raw_json_files": count_files(http_files, r"json\.RawMessage|json\.Marshal|json\.Unmarshal|json\.NewDecoder"),
            "raw_json_sites": count_matches(http_files, r"json\.RawMessage|json\.Marshal|json\.Unmarshal|json\.NewDecoder"),
        },
        "dependencies": {
            "service_packages": package_count("./internal/service"),
            "httpapi_packages": package_count("./internal/httpapi"),
            "service_imports_httpapi": service_imports_httpapi,
            "httpapi_imports_service": httpapi_imports_service,
            "service_or_httpapi_imports_wails": any("wails" in imp.lower() for p in ("internal/service", "internal/httpapi") for imp in graph.get(p, [])),
            "production_httpapi_files_importing_net_http": count_files(http_files, r'"net/http"'),
            "service_files_importing_net_http": count_files(service_files, r'"net/http"'),
            "store_files_importing_c": count_files(store_files, r"^import\s+\"C\"|#cgo|#include\s*[<\"]sqlite3\.h"),
        },
        "store": {
            "production_files": len(store_files),
            "store_interface_methods": interface_method_count(text(ROOT / "internal/store/store.go"), "Store"),
            "files_importing_c": count_files(store_files, r"^import\s+\"C\"|#cgo|#include\s*[<\"]sqlite3\.h"),
            "direct_sqlite_c_files": count_files(store_files, r"\bC\.sqlite3|\bsqlite3_"),
            "current_schema_version": (int(re.search(r"CurrentSchemaVersion\s*=\s*(\d+)", text(ROOT / "internal/store/store.go")).group(1))
                                       if re.search(r"CurrentSchemaVersion\s*=\s*(\d+)", text(ROOT / "internal/store/store.go") ) else None),
        },
        "coupling": {
            "service_production_files": len(service_files),
            "service_imports_httpapi": service_imports_httpapi,
            "httpapi_imports_service": httpapi_imports_service,
            "service_files_importing_net_http": count_files(service_files, r'"net/http"'),
            "service_direct_store_reference_files": count_files(service_files, r"\bstore\s*\."),
            "internal_import_graph": graph,
            "wails_files": wails_files,
            "wails_isolated_from_service_and_httpapi": not any("wails" in imp.lower() for p in ("internal/service", "internal/httpapi") for imp in graph.get(p, [])),
            "wails_build_tagged_files": sorted(str(p.relative_to(ROOT)) for p in all_files + cmd_files if re.search(r"//go:build\s+.*gui", text(p))),
        },
        "sqlite_assertions": {
            "fullmutex": bool(re.search(r"FULLMUTEX|SQLITE_OPEN_FULLMUTEX", store_text)),
            "wal": bool(re.search(r"journal_mode|WAL", store_text, re.IGNORECASE)),
            "busy_timeout": bool(re.search(r"busy_timeout|sqlite3_busy_timeout", store_text, re.IGNORECASE)),
            "fts5": bool(re.search(r"fts5", store_text, re.IGNORECASE)),
            "json": bool(re.search(r"json_extract|json\s*\(", store_text, re.IGNORECASE)),
            "integrity_check": bool(re.search(r"integrity_check", store_text, re.IGNORECASE)),
            "migration_files_or_symbols": count_matches(store_files, r"migrat|schema_v\d+|CurrentSchemaVersion"),
        },
        "facade_migration_inputs": {
            "httpapi_files_with_store_refs": sorted(str(p.relative_to(ROOT)) for p in http_files if re.search(r"\bs\.store\s*\.", text(p))),
            "service_files_with_httpapi_refs": sorted(str(p.relative_to(ROOT)) for p in service_files if "internal/httpapi" in text(p)),
            "httpapi_route_files": sorted(str(p.relative_to(ROOT)) for p in http_files if ".HandleFunc(" in text(p)),
        },
        "production_code_changed": False,
        "private_data_read": False,
    }


def numeric_pairs(current: object, baseline: object, prefix: str = "") -> list[dict]:
    out: list[dict] = []
    if isinstance(current, dict) and isinstance(baseline, dict):
        for key in sorted(set(current) & set(baseline)):
            out.extend(numeric_pairs(current[key], baseline[key], f"{prefix}.{key}".strip(".")))
    elif isinstance(current, (int, float)) and isinstance(baseline, (int, float)) and not isinstance(current, bool) and not isinstance(baseline, bool):
        out.append({"path": prefix, "baseline": baseline, "current": current, "delta": current - baseline})
    return out


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--write", action="store_true")
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    current = build()
    baseline = json.loads(BASELINE.read_text(encoding="utf-8"))
    current["baseline"] = {"path": str(BASELINE.relative_to(ROOT)), "schema": baseline.get("schema")}
    current["drift"] = numeric_pairs(current, baseline)
    rendered = json.dumps(current, indent=2, sort_keys=True) + "\n"
    if args.check:
        if not OUTPUT.exists() or OUTPUT.read_text(encoding="utf-8") != rendered:
            print(f"{OUTPUT} is stale; run audit.py --write")
            return 1
    elif args.write or not OUTPUT.exists():
        OUTPUT.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
