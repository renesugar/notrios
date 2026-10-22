#!/usr/bin/env python3
"""Derive what is known about every frozen REST route and MCP tool.

J4 asks whether each promise 1.0 is about to make should be made. That is a
judgement, and a judgement is only worth having if it is made against evidence
rather than against a name -- 113 routes read in a list all look equally
reasonable.

So this derives, per member, the facts a reviewer needs and cannot get by
reading the frozen list:

  described   the route appears in api/openapi.yaml, or the tool in the MCP
              tool table the server actually advertises
  documented  a published document mentions it
  exercised   a published example that this repository *executes* uses it
  tested      a Go test in the serving package names it

None of those four is a verdict. A route with all four could still be a promise
1.0 should not make, and a route with none could be load-bearing and quietly
correct. What they give is a defensible order to review in, and a way to notice
the member nothing anywhere refers to -- which is the one a freeze is most
likely to have caught by accident.

    python3 performance/v1.0-j4/review_surfaces.py            # write REPORT.json
    python3 performance/v1.0-j4/review_surfaces.py --check    # compare, do not write
"""
from __future__ import annotations

import argparse
import json
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = pathlib.Path(__file__).resolve().parent
REPORT = HERE / "REPORT.json"
FROZEN = ROOT / "performance/v0.9-i8/FROZEN.json"


def frozen() -> dict:
    return json.loads(FROZEN.read_text(encoding="utf-8"))["surfaces"]


def documents() -> dict[str, str]:
    return {str(path.relative_to(ROOT)): path.read_text(encoding="utf-8")
            for path in sorted((ROOT / "docs").rglob("*.md"))}


def executed_bodies() -> str:
    """Every fence this repository actually runs, concatenated.

    Read from the execution registry's `executed` entries rather than from the
    documents, because "appears in a document" and "appears in something that
    runs" are the two different facts this report keeps apart.
    """
    registry = json.loads((ROOT / "docs/docaudit/registry.json").read_text(encoding="utf-8"))
    wanted = {entry["id"] for entry in registry["executables"] if entry["state"] == "executed"}
    text = []
    for path, body in documents().items():
        stem = path.removeprefix("docs/").removesuffix(".md").replace("/", "-")
        section, ordinal = "", {}
        lines = body.split("\n")
        index = 0
        while index < len(lines):
            heading = re.match(r"^#{1,6}\s+(.*?)\s*$", lines[index])
            if heading:
                section = slug(heading.group(1))
                index += 1
                continue
            if not lines[index].strip().startswith("```"):
                index += 1
                continue
            start = index + 1
            index += 1
            while index < len(lines) and not lines[index].strip().startswith("```"):
                index += 1
            ordinal[section] = ordinal.get(section, 0) + 1
            if f"{stem}-{section}-example-{ordinal[section]}" in wanted:
                text.append("\n".join(lines[start:index]))
            index += 1
    return "\n".join(text)


def slug(value: str) -> str:
    value = value.lower().replace(".", "")
    out, dash = [], False
    for char in value:
        if char.isascii() and char.isalnum():
            if dash and out:
                out.append("-")
            out.append(char)
            dash = False
        else:
            dash = True
    return "".join(out).strip("-")


def serving_tests() -> str:
    parts = []
    for path in sorted((ROOT / "internal/httpapi").glob("*_test.go")):
        parts.append(path.read_text(encoding="utf-8"))
    return "\n".join(parts)


def shape(path: str) -> str:
    """A route's identity: its literal segments, with parameters anonymised.

    `/x/{class}/{name}` and `/x/{segment1}/{segment2}` are the same route
    described by two vocabularies, and a comparison that says otherwise is
    comparing documentation style rather than surface.
    """
    return re.sub(r"\{[^}]*\}", "{}", path.strip())


def route_shapes(member: str) -> tuple[str, str]:
    """Split `POST /api/v1/x/{id}` into its method and path."""
    method, _, path = member.partition(" ")
    return method, path


def review_rest(members: list[str], docs: dict[str, str], executed: str, tests: str) -> list[dict]:
    openapi = (ROOT / "api/openapi.yaml").read_text(encoding="utf-8")
    # Compared by *shape*, not by spelling. The first draft matched the path
    # text and reported two carrier routes as undescribed; OpenAPI does describe
    # them, as `/{segment1}/{segment2}` where the router says `{class}/{name}`.
    # A parameter's name is not part of the route's identity, and treating it as
    # one produced a false finding -- though chasing it found a real one, which
    # is recorded in DECISIONS.md.
    described_shapes = {shape(line.strip().rstrip(":"))
                        for line in openapi.split("\n")
                        if line.startswith("  /")}
    rows = []
    for member in members:
        method, path = route_shapes(member)
        described = shape(path) in described_shapes
        # A document mentions the route if it names the path, with or without
        # the braces a reader would substitute.
        literal = re.sub(r"\{[^}]+\}", "", path)
        mentioned = [name for name, body in docs.items()
                     if path in body or (literal and literal in body)]
        rows.append({
            "surface": "rest",
            "member": member,
            "method": method,
            "path": path,
            "described": described,
            "documented": sorted(mentioned),
            "exercised": path in executed or (literal and literal in executed),
            "tested": path in tests or (literal and literal in tests),
        })
    return rows


def review_mcp(members: list[str], docs: dict[str, str], executed: str, tests: str) -> list[dict]:
    advertised = (ROOT / "internal/httpapi/mcp.go").read_text(encoding="utf-8")
    rows = []
    for member in members:
        quoted = f'"{member}"'
        mentioned = [name for name, body in docs.items() if member in body]
        rows.append({
            "surface": "mcp",
            "member": member,
            "described": quoted in advertised,
            "documented": sorted(mentioned),
            "exercised": member in executed,
            # Quoted, because a tool is *called* by name and only mentioned in
            # prose. The first version searched the test text for the bare
            # name, and a probe that removed retry_sync_job from the test still
            # reported it tested -- it was named in the new test's own doc
            # comment. A gate satisfied by a comment is worse than no gate,
            # because D2 relies on this one.
            "tested": quoted in tests,
        })
    return rows


def build() -> dict:
    surfaces = frozen()
    docs = documents()
    executed = executed_bodies()
    tests = serving_tests()

    rows = (review_rest(surfaces["rest"]["members"], docs, executed, tests)
            + review_mcp(surfaces["mcp"]["members"], docs, executed, tests))

    def counted(surface: str, field: str) -> int:
        return sum(1 for row in rows if row["surface"] == surface and
                   (row[field] if field != "documented" else bool(row[field])))

    unreferenced = sorted(row["member"] for row in rows
                          if not row["documented"] and not row["exercised"]
                          and not row["tested"])

    return {
        "schema": "notrios.v10.j4-surface-review.v1",
        "item": "J4",
        "milestone": "v1.0",
        "what_this_is": (
            "What is known about every frozen REST route and MCP tool, derived rather than "
            "recalled, so the review of what 1.0 should promise happens against evidence and in "
            "a defensible order."),
        "these_are_not_verdicts": (
            "described/documented/exercised/tested are facts about the repository, not judgements "
            "about the surface. A route with all four may still be a promise 1.0 should not make; "
            "a route with none may be load-bearing and correct. The decisions are recorded "
            "separately, by a person, and say so."),
        "content_commit": subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT,
                                         capture_output=True, text=True,
                                         check=True).stdout.strip(),
        "counts": {
            "rest": {
                "members": surfaces["rest"]["count"],
                "described": counted("rest", "described"),
                "documented": counted("rest", "documented"),
                "exercised": counted("rest", "exercised"),
                "tested": counted("rest", "tested"),
            },
            "mcp": {
                "members": surfaces["mcp"]["count"],
                "described": counted("mcp", "described"),
                "documented": counted("mcp", "documented"),
                "exercised": counted("mcp", "exercised"),
                "tested": counted("mcp", "tested"),
            },
        },
        "referred_to_by_nothing": unreferenced,
        "members": rows,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true",
                        help="fail if the committed report is not what the repository says")
    arguments = parser.parse_args()
    rendered = json.dumps(build(), indent=2) + "\n"
    if arguments.check:
        if not REPORT.is_file() or REPORT.read_text(encoding="utf-8") != rendered:
            print("J4 surface review is stale; run "
                  "python3 performance/v1.0-j4/review_surfaces.py", file=sys.stderr)
            return 1
        print("J4 surface review current")
        return 0
    REPORT.write_text(rendered, encoding="utf-8")
    counts = json.loads(rendered)["counts"]
    print(f"wrote {REPORT.relative_to(ROOT)}: rest {counts['rest']}, mcp {counts['mcp']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
