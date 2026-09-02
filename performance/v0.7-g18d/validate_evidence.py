#!/usr/bin/env python3
"""Validate and rerun the checked G18d documentation example evidence."""

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / "performance/v0.7-g18d"
REASONS = {
    "external-network",
    "host-installation",
    "privileged-host-change",
    "shared-user-state",
    "interactive-or-long-running",
    "illustrative-placeholder",
}


def load(path: Path):
    return json.loads(path.read_text(encoding="utf-8"))


def without_runtime(report: dict) -> dict:
    value = json.loads(json.dumps(report))
    value["runtime_ms"] = 0
    for topic in value["topics"]:
        topic["runtime_ms"] = 0
    return value


def validate_registry(registry: dict, report: dict) -> None:
    assert registry["schema"] == "notrios.docaudit.registry.v2"
    examples = registry["executables"]
    # 131 -> 133 in v0.8 H4 slice D: the `paths` and `config show` synopses.
    # 133 -> 137 in v0.8 H4 slice E: the `migrate` synopsis and its dry-run
    # example, the upgrade recipe in docs/installation.md, and the
    # asset-installation block added to the same page.
    # 137 -> 139 in v0.8 H5: the make install/uninstall/purge examples in
    # docs/installation.md, less the three hand-rolled ones they replace.
    assert len(examples) == report["entries"] == 141
    assert len({item["id"] for item in examples}) == 141
    executed = [item for item in examples if item["state"] == "executed"]
    unverified = [item for item in examples if item["state"] == "unverified"]
    assert len(executed) == report["executed"] == 63
    # 68 -> 70: both new synopses are bracketed-optional forms, registered as
    # illustrative placeholders like every other synopsis in that document.
    # 70 -> 74 in slice E. Executed stays 63: one is a bracketed synopsis, two
    # relocate this user's library, and one writes to a system directory as
    # root, so all four are reviewed unrun reasons. The migrate command itself
    # is covered by executed tests in cmd/notriosctl.
    # 74 -> 76 in v0.8 H5. Executed stays 63: every new example installs into
    # this user's home or deletes their library.
    assert len(unverified) == report["unverified"] == 78
    assert {item["execution"]["surface"] for item in executed} == {
        "cli", "config", "rest", "mcp"
    }
    assert all(item.get("check_anchor") and item.get("execution") and not item.get("unrun_reason") for item in executed)
    assert all(not item.get("check_anchor") and not item.get("execution") and item.get("unrun_reason") for item in unverified)
    assert all(item["unrun_reason"]["code"] in REASONS for item in unverified)
    assert all("PENDING" not in item["unrun_reason"]["detail"] for item in unverified)
    counts = {reason: 0 for reason in REASONS}
    for item in unverified:
        counts[item["unrun_reason"]["code"]] += 1
    assert {key: value for key, value in counts.items() if value} == report["unrun_reasons"]
    assert sum(1 for topic in report["topics"] if topic["counts"].get("executed", 0)) == 12


def main() -> None:
    expected = load(EVIDENCE / "REPORT.json")
    mutations = load(EVIDENCE / "MUTATION_MATRIX.json")
    registry = load(ROOT / "docs/docaudit/registry.json")
    assert expected["schema"] == "notrios.docexec.report.v1"
    assert expected["runtime_ms"] > 0
    assert len(mutations["manifest_cases"]) == 7
    assert len(mutations["schema_cases"]) == 4
    validate_registry(registry, expected)

    with tempfile.TemporaryDirectory(prefix="notrios-g18d-") as directory:
        actual_path = Path(directory) / "REPORT.json"
        environment = dict(os.environ)
        environment["GOCACHE"] = str(Path(tempfile.gettempdir()) / "notrios-g18d-gocache")
        environment["NOTRIOS_DOCEXEC_REPORT"] = str(actual_path)
        subprocess.run(
            ["go", "test", "./internal/docexec", "-run", "^TestRepositoryExamples$", "-count=1", "-timeout", "240s"],
            cwd=ROOT,
            env=environment,
            check=True,
        )
        actual = load(actual_path)
    assert actual["runtime_ms"] > 0
    assert all(topic["runtime_ms"] > 0 for topic in actual["topics"] if topic["counts"].get("executed", 0))
    assert without_runtime(actual) == without_runtime(expected), "REPORT.json deterministic fields are stale"
    print(
        "G18d evidence valid: "
        f"{actual['executed']}/{actual['entries']} examples executed across 12 topics "
        f"in {actual['runtime_ms']} ms; {actual['unverified']} reviewed unrun reasons."
    )


if __name__ == "__main__":
    main()
