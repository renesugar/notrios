#!/usr/bin/env python3
"""Migrate a page's published examples into a tracked set, without changing a byte (v1.0 J15).

    python3 performance/v1.0-j15/migrate_page.py docs/api/rest.md docs/docexamples/api-rest.json

A migration is a declaration of what is already published, so this reads the
page, writes the tracked set from what it finds, and wraps each fence in the
markers the generator replaces. `go run ./cmd/docexamples --write` then rewrites
the page, and the only difference from before must be the marker lines: if the
generator would change a published line, the migration is wrong and the diff
says so.

The CLI commands and REST routes each recipe uses are derived from its own
steps, and every derived route is one the server registers -- a route this
cannot match is reported rather than guessed at, because a wrong declaration is
worse than none.
"""
from __future__ import annotations

import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
EXECUTABLE_LANGUAGES = {"bash", "sh", "shell", "console", "yaml", "yml", "toml", "http"}
ROUTE = re.compile(r'mux\.HandleFunc\("([A-Z]+ [^"]+)"')
URL = re.compile(r"https?://[^\s|)\\]+")
CLI = re.compile(r"\bnotriosctl ([a-z][a-z0-9-]*(?: [a-z][a-z0-9-]*)?)")


def slug(value: str) -> str:
    """The section slug, as internal/docaudit computes it."""
    value = value.lower().replace(".", "")
    out, dash = [], False
    for char in value:
        if char.isascii() and (char.isalpha() or char.isdigit()):
            if dash and out:
                out.append("-")
            out.append(char)
            dash = False
        else:
            dash = True
    return "".join(out)


def registered_routes() -> list[str]:
    routes = []
    for path in sorted((ROOT / "internal/httpapi").glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        routes += ROUTE.findall(path.read_text(encoding="utf-8"))
    return sorted(set(routes))


def classification() -> dict[str, str]:
    """J13's class for every published example: recipe, synopsis, fragment.

    A synopsis shows the *form* of a command, so generating one from a
    declaration of the form is a tautology; J15's boundaries say it stays a
    synopsis. This is how they are told apart, rather than by guessing from the
    text a second time.
    """
    data = json.loads((ROOT / "performance/v1.0-j13/REPORT.json").read_text(encoding="utf-8"))
    rows = data if isinstance(data, list) else next(
        value for value in data.values()
        if isinstance(value, list) and value and isinstance(value[0], dict))
    return {row["id"]: row["class"] for row in rows}


def cli_commands() -> set[str]:
    spec = json.loads((ROOT / "internal/clispec/commands.json").read_text(encoding="utf-8"))
    return {" ".join(command["path"]) for command in spec["commands"]}


def route_for(method: str, path: str, routes: list[str]) -> str | None:
    """The registered route a published path calls, or None."""
    wanted = [segment for segment in path.split("/") if segment]
    for route in routes:
        route_method, route_path = route.split(" ", 1)
        if route_method != method:
            continue
        segments = [segment for segment in route_path.split("/") if segment]
        if len(segments) != len(wanted):
            continue
        if all(segment.startswith("{") or segment == value or "$" in value
               for segment, value in zip(segments, wanted)):
            return route
    return None


def uses_of(lines: list[str], routes: list[str], commands: set[str]) -> tuple[dict, list[str]]:
    """What a recipe drives, derived from its own steps."""
    found_rest, found_cli, unmatched = [], [], []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("#"):
            continue
        for command in CLI.findall(line):
            # The longest known command this line names: `notes create` before `notes`.
            for candidate in (command, command.split(" ")[0]):
                if candidate in commands:
                    if candidate not in found_cli:
                        found_cli.append(candidate)
                    break
        if "curl" not in line:
            continue
        method = "GET"
        if match := re.search(r"-X\s+([A-Z]+)", line):
            method = match.group(1)
        # A published example splices shell variables into a URL by closing and
        # reopening quotes -- .../resources/'$RES'/content -- so the quoting is
        # removed before the path is read, and a $variable segment is treated
        # as the value a {placeholder} stands for.
        for url in URL.findall(line.replace("'", "").replace('"', "")):
            path = re.sub(r"\?.*$", "", url.split("/", 3)[3] if url.count("/") >= 3 else "")
            if not path:
                continue
            route = route_for(method, "/" + path, routes)
            if route is None:
                unmatched.append(f"{method} /{path}")
            elif route not in found_rest:
                found_rest.append(route)
    uses = {}
    if found_cli:
        uses["cli"] = found_cli
    if found_rest:
        uses["rest"] = found_rest
    return uses, unmatched


def main() -> int:
    if len(sys.argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    document, destination = sys.argv[1], sys.argv[2]
    page = ROOT / document
    lines = page.read_text(encoding="utf-8").split("\n")
    routes, commands = registered_routes(), cli_commands()
    classes = classification()

    examples, out_lines = [], []
    section, ordinals = "", {}
    unmatched_all = []
    index = 0
    while index < len(lines):
        line = lines[index]
        if heading := re.match(r"^(#{1,6})\s+(.*)$", line):
            section = slug(heading.group(2))
            out_lines.append(line)
            index += 1
            continue
        stripped = line.strip()
        if not stripped.startswith("```"):
            out_lines.append(line)
            index += 1
            continue
        language = stripped[3:].split()[0].lower() if stripped[3:].split() else ""
        start = index + 1
        index += 1
        while index < len(lines) and not lines[index].strip().startswith("```"):
            index += 1
        body = lines[start:index]
        closing = lines[index] if index < len(lines) else "```"
        index += 1
        if language not in EXECUTABLE_LANGUAGES:
            out_lines.extend([line] + body + [closing])
            continue

        ordinals[section] = ordinals.get(section, 0) + 1
        ordinal = ordinals[section]
        stem = document.removeprefix("docs/").removesuffix(".md").replace("/", "-")
        example_id = f"{stem}-{section}-example-{ordinal}"
        kind = classes.get(example_id, "recipe")
        if kind != "recipe":
            out_lines.extend([line] + body + [closing])
            print(f"  left  {example_id}: a {kind}, which stays as it is")
            continue

        uses, unmatched = uses_of(body, routes, commands)
        unmatched_all += [f"{example_id}: {item}" for item in unmatched]
        if not uses:
            # Nothing to declare means tracking it would assert nothing, so it
            # stays a hand-written fence and the record says why.
            out_lines.extend([line] + body + [closing])
            print(f"  left  {example_id}: declares no command or route")
            continue

        steps = []
        for body_line in body:
            if not body_line.strip():
                steps.append({})
            elif body_line.strip().startswith("#"):
                steps.append({"comment": body_line})
            else:
                steps.append({"run": body_line})
        examples.append({
            "section": section,
            "ordinal": ordinal,
            "kind": "recipe",
            "language": language,
            "uses": uses,
            "steps": steps,
        })
        out_lines.append(f"<!-- notrios:generated:example:{example_id}:begin -->")
        out_lines.extend([line] + body + [closing])
        out_lines.append(f"<!-- notrios:generated:example:{example_id}:end -->")

    if unmatched_all:
        print("paths that match no registered route:", file=sys.stderr)
        for item in unmatched_all:
            print(f"  {item}", file=sys.stderr)
        return 1

    page.write_text("\n".join(out_lines), encoding="utf-8")
    (ROOT / destination).write_text(json.dumps({
        "schema": "notrios.docexamples.v1",
        "document": document,
        "note": ("The published examples on this page are generated from here by "
                 "`go run ./cmd/docexamples --write`. Each recipe's steps are the published "
                 "text; what this file adds is the declaration beside them -- the CLI commands "
                 "and REST routes it drives, checked against the CLI spec and the routes the "
                 "server registers, so a documented command that stops existing fails the build."),
        "examples": examples,
    }, indent=2) + "\n", encoding="utf-8")
    print(f"tracked {len(examples)} example(s) from {document} in {destination}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
