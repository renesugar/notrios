#!/usr/bin/env python3
"""Build the deterministic G18a claim-surface inventory.

This is investigation evidence, not a documentation generator.  It records the
present manual corpus at section granularity so G18c has a finite migration
denominator and fails loudly when that baseline changes.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
OUT = Path(__file__).with_name("INVENTORY.json")

DOC_OWNERS = {
    "docs/api/mcp.md": "go:github.com/renesugar/notrios/internal/httpapi#(*Server).mcpTools",
    "docs/api/rest.md": "go:github.com/renesugar/notrios/internal/httpapi#NewServerWithOptions",
    "docs/archive-v2.md": "go:github.com/renesugar/notrios/internal/archivev2#Export",
    "docs/cli.md": "go:github.com/renesugar/notrios/cmd/notriosctl#printHelp",
    "docs/gui.md": "ts:web/src/App.tsx#App",
    "docs/import-export.md": "go:github.com/renesugar/notrios/cmd/notriosctl#runImport",
    "docs/index.md": "go:github.com/renesugar/notrios/internal/helpdocs#Seed",
    "docs/installation.md": "go:github.com/renesugar/notrios/cmd/notriosd#main",
    "docs/operations.md": "go:github.com/renesugar/notrios/internal/store#(*SQLiteStore).LintWorkspace",
    "docs/publishing.md": "go:github.com/renesugar/notrios/internal/publish#(Profile).PlanRequest",
    "docs/query-language.md": "go:github.com/renesugar/notrios/internal/query#Parse",
    "docs/selection-planning.md": "go:github.com/renesugar/notrios/internal/store#(*SQLiteStore).PlanSelection",
    "docs/service.md": "go:github.com/renesugar/notrios/internal/config#Config",
    "docs/stable-links.md": "go:github.com/renesugar/notrios/internal/store#(*SQLiteStore).ResolveStableLink",
    "docs/troubleshooting.md": "go:github.com/renesugar/notrios/internal/config#Load",
}

GUI_JOURNEYS = [
    ("search-open", 3, "ts:web/src/App.tsx#App"),
    ("new-note-in-notebook", 4, "ts:web/src/components/SearchPane.tsx#SearchPane"),
    ("edit-save", 3, "ts:web/src/components/EditorPane.tsx#EditorPane"),
    ("trash-restore", 5, "ts:web/src/components/EditorPane.tsx#EditorPane"),
    ("delete-notebook-review", 4, "ts:web/src/organizer.ts#notebookDeletionPrompt"),
    ("insert-and-check-link", 4, "ts:web/src/components/LinkIntelligence.tsx#LinkPicker"),
    ("inspect-local-graph", 3, "ts:web/src/components/LocalGraph.tsx#LocalGraph"),
    ("open-sync-center", 3, "ts:web/src/components/SyncCenter.tsx#SyncCenter"),
    ("editor-find-replace", 4, "ts:web/src/components/EditorPane.tsx#EditorPane"),
]


def slug(value: str) -> str:
    value = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return value or "section"


def headings(path: Path) -> list[dict[str, object]]:
    result = []
    seen: dict[str, int] = {}
    fence: str | None = None
    for line in path.read_text(encoding="utf-8").splitlines():
        marker = re.match(r"^\s*(`{3,}|~{3,})", line)
        if marker:
            token = marker.group(1)[0]
            fence = None if fence == token else token
            continue
        if fence is not None:
            continue
        match = re.match(r"^(#{1,3})\s+(.+?)\s*$", line)
        if not match:
            continue
        title = match.group(2).strip()
        base = slug(title)
        seen[base] = seen.get(base, 0) + 1
        suffix = "" if seen[base] == 1 else f"-{seen[base]}"
        result.append({"id": base + suffix, "level": len(match.group(1)), "title": title})
    return result


def count_cli_usage() -> int:
    text = (ROOT / "cmd/notriosctl/main.go").read_text(encoding="utf-8")
    body = re.search(r"func printHelp\(\) \{\s*fmt\.Print\(`(.*?)`\)\s*\}", text, re.S)
    if not body:
        raise ValueError("printHelp raw string was not found")
    return sum(line.startswith("  notriosctl ") for line in body.group(1).splitlines())


def count_config_keys() -> int:
    text = (ROOT / "internal/config/config.go").read_text(encoding="utf-8")
    return len(re.findall(r'`json:"([^",]+)', text))


def count_openapi_operations() -> tuple[int, int]:
    text = (ROOT / "api/openapi.yaml").read_text(encoding="utf-8")
    operations = len(re.findall(r"^    (?:get|post|put|patch|delete):\s*$", text, re.M))
    operation_ids = len(re.findall(r"^\s+operationId:\s*\S+", text, re.M))
    return operations, operation_ids


def count_mcp_tools() -> int:
    text = (ROOT / "internal/httpapi/mcp.go").read_text(encoding="utf-8")
    return len(re.findall(r'\bName:\s*"[a-z][a-z0-9_]*"', text))


def build() -> dict[str, object]:
    documents = []
    grade_totals = {"executed": 0, "generated": 0, "claimed": 0, "unverified": 0}
    section_total = 0
    for rel, owner in sorted(DOC_OWNERS.items()):
        path = ROOT / rel
        units = headings(path)
        for unit in units:
            unit.update({
                "audience": "user",
                "kind": "actionable_or_reference",
                "owner": owner,
                "owner_role": "review_root_candidate",
                "grade": "unverified",
            })
            grade_totals["unverified"] += 1
        section_total += len(units)
        documents.append({
            "path": rel,
            "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            "help_note": "doc_help_" + re.sub(r"[^a-z0-9]", "_", rel[5:-3].lower()).strip("_"),
            "owner": owner,
            "sections": units,
        })

    rest_operations, operation_ids = count_openapi_operations()
    surfaces = [
        {"id": "published_markdown", "count": len(documents), "unit": "pages", "sections": section_total,
         "owner": "go:github.com/renesugar/notrios/internal/helpdocs#Seed"},
        {"id": "help_notebook", "count": len(documents), "unit": "mirrored notes", "sections": section_total,
         "owner": "go:github.com/renesugar/notrios/internal/helpdocs#Seed"},
        {"id": "cli_help", "count": count_cli_usage(), "unit": "usage forms",
         "owner": "go:github.com/renesugar/notrios/cmd/notriosctl#printHelp"},
        {"id": "configuration", "count": count_config_keys(), "unit": "JSON-tagged keys",
         "owner": "go:github.com/renesugar/notrios/internal/config#Config"},
        {"id": "openapi", "count": rest_operations, "unit": "operations", "operation_ids": operation_ids,
         "owner": "go:github.com/renesugar/notrios/internal/httpapi#NewServerWithOptions",
         "finding": "No operationId values exist; source-symbol anchors must not depend on them."},
        {"id": "mcp_tools", "count": count_mcp_tools(), "unit": "tools",
         "owner": "go:github.com/renesugar/notrios/internal/httpapi#(*Server).mcpTools"},
        {"id": "mcp_resources", "count": 0, "unit": "protocol resources",
         "owner": "go:github.com/renesugar/notrios/internal/httpapi#(*Server).handleMCP",
         "finding": "The server advertises tools only; read_resource is a tool, not an MCP resources/read surface."},
        {"id": "gui_journeys", "count": len(GUI_JOURNEYS), "unit": "proposed journeys",
         "owner": "ts:web/src/App.tsx#App",
         "measurement_state": "proposed_not_executed",
         "journeys": [{"id": item, "proposed_actions": actions, "owner": owner}
                      for item, actions, owner in GUI_JOURNEYS]},
    ]
    return {
        "schema": "notrios.g18a.claim-surface-inventory.v1",
        "as_of": "2026-08-26",
        "claim_unit": "Each non-fenced H1-H3 section is one migration denominator unit; G18c splits it into source-adjacent fragments and replaces its review-root candidate with claim-local declaration anchors.",
        "documents": documents,
        "surfaces": surfaces,
        "grade_baseline": {
            "rule": "One strongest honest grade per section; rationale is excluded before grading.",
            "totals": grade_totals,
            "denominator": section_total,
            "reconciles": sum(grade_totals.values()) == section_total,
            "finding": "The manual does not yet name directive fragments or executable checks, so all current section units are unverified even where independent tests exist.",
        },
        "preserved_drift": {
            "document": "docs/service.md",
            "claim": "schema v20 migration",
            "canonical_owner": "go:github.com/renesugar/notrios/internal/store#CurrentSchemaVersion",
            "canonical_value": 27,
            "verdict": "contradicted",
        },
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    encoded = json.dumps(build(), indent=2, ensure_ascii=False) + "\n"
    if args.write:
        OUT.write_text(encoded, encoding="utf-8")
    else:
        print(encoded, end="")


if __name__ == "__main__":
    main()
