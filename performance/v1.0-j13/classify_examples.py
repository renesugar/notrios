#!/usr/bin/env python3
"""Classify every published fenced block, and say where a worked example is missing.

J13-A. The examination before the work, because "99 of 164 examples are
unverified" is one number covering two different situations and acting on it
without separating them would delete good documentation to improve a statistic.

A command *synopsis* -- `notriosctl search [--limit N] ... "<query>"` -- is not a
broken example. The brackets are optional-argument notation and the angle
brackets are metavariables; it is a reference form, correctly unexecutable, and
replacing it with one worked invocation would make a reference list worse. A
*recipe* is different: it looks like something a reader can paste, so if it has
never run, it is a promise nobody checked.

# Where the blocks come from

`docs/docaudit/registry.json`, not a second extractor. The registry is derived
from internal/docaudit's own fence scanner, and re-implementing that scanner
here would give this tool its own opinion about what a fenced block is. Instead
the bodies are extracted from the documents and matched to the registry **by
sha256**: if a single hash fails to match, this tool is reading something other
than what the audit reads, and it refuses rather than reporting.

    python3 performance/v1.0-j13/classify_examples.py            # write REPORT.json
    python3 performance/v1.0-j13/classify_examples.py --check    # compare, do not write
"""
from __future__ import annotations

import argparse
import collections
import hashlib
import json
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = pathlib.Path(__file__).resolve().parent
REPORT = HERE / "REPORT.json"

HEADING = re.compile(r"^(#{1,6})\s+(.*?)\s*$")
# The language set and the prefix list are internal/docaudit/markdown.go's
# isExecutableFence, mirrored so the extraction can be checked against it. The
# sha256 match is what proves the mirror is faithful; this list is only how the
# candidates are found.
FENCE_LANGUAGES = {"bash", "sh", "shell", "console", "yaml", "yml", "toml", "http"}
COMMAND_PREFIXES = ("notriosctl ", "notriosd ", "curl ", "go run ", "npm ", "make ",
                    "$ notrios", "$ curl", "GET /api/", "POST /api/", "PUT /api/",
                    "PATCH /api/", "DELETE /api/")


def slug(text: str) -> str:
    """internal/docaudit/markdown.go's slug, including its dot rule.

    The dot is *removed* rather than turned into a separator, so
    "Upgrading from before 0.8" is `upgrading-from-before-08`. Writing the
    obvious regex instead produced `...-0-8` and the hash cross-check refused
    the whole report over one id -- which is the cross-check doing its job.
    """
    text = text.lower().replace(".", "")
    out: list[str] = []
    dash = False
    for char in text:
        if char.isascii() and char.isalnum():
            if dash and out:
                out.append("-")
            out.append(char)
            dash = False
        else:
            dash = True
    return "".join(out).strip("-")


def executable_fence(language: str, body: str) -> bool:
    if language in FENCE_LANGUAGES:
        return True
    if language == "json":
        return any(token in body for token in ('"jsonrpc"', '"method"', '"server"'))
    return body.strip().startswith(COMMAND_PREFIXES)


def fences(path: pathlib.Path) -> list[dict]:
    """Every executable fence in one document, with its section and ordinal."""
    lines = path.read_text(encoding="utf-8").split("\n")
    section = ""
    ordinal: dict[str, int] = collections.Counter()
    found = []
    index = 0
    while index < len(lines):
        heading = HEADING.match(lines[index])
        if heading:
            section = slug(heading.group(2))
            index += 1
            continue
        stripped = lines[index].strip()
        if not stripped.startswith("```") and not stripped.startswith("~~~"):
            index += 1
            continue
        marker = stripped[:3]
        rest = stripped[3:].strip()
        language = rest.split()[0].lower() if rest else ""
        start = index + 1
        index += 1
        while index < len(lines) and not lines[index].strip().startswith(marker):
            index += 1
        body = "\n".join(lines[start:index])
        if executable_fence(language, body):
            ordinal[section] += 1
            found.append({
                "section": section,
                "language": language,
                "ordinal": ordinal[section],
                "sha256": hashlib.sha256(body.encode("utf-8")).hexdigest(),
                "body": body,
            })
        index += 1
    return found


# Metavariable notation: `<name>` and `[--flag]`, the two forms this repository
# uses, both meaning "substitute something here" rather than "type this".
#
# The two forms need different rules, and the first draft of this used one
# regex for both and mislabelled seven executed examples as synopses. Square
# brackets appear constantly in literal shell -- `[a](Kitchen)` is a Markdown
# link inside a JSON payload, and `.result.tools[].name` is a jq filter -- and
# both of those live inside quotes. Angle brackets are not valid shell, JSON or
# jq syntax in any of these documents, so they count wherever they appear,
# which keeps `notriosctl search "<query>"` correctly a synopsis.
ANGLE = re.compile(r"<[a-zA-Z][\w .|/-]*>")
BRACKET = re.compile(r"\[([^\]\n]+)\]")
QUOTED = re.compile(r"'[^']*'|\"(?:\\.|[^\"\\])*\"")


# What optional-argument notation never contains. Every bracketed form in this
# documentation holds flags or bare words -- `[--limit N]`, `[shared flags]`,
# `[docs-dir]`, `[--target publication_handoff|subset_transfer]`. A JSON array
# holds braces, quotes or colons, and that is the whole difference.
JSON_ISH = set('{}":')


def has_metavariable(line: str) -> bool:
    if ANGLE.search(line):
        return True
    # Two filters, each for a pattern that actually appears. Quoted spans go
    # first, which removes `[a](Kitchen)` in a payload and `.tools[].name` in a
    # jq filter; then a bracket whose contents look like JSON is not notation,
    # which removes `"items": [{...}]` and `"tags":["publish"]` where the
    # bracket sits outside the quoted key.
    # The placeholder is a quote character, not an empty pair. Blanking a
    # quoted span to `''` turned `"tags":["publish"]` into `['']`, which looks
    # exactly like a bare-word option; a surviving quote keeps it JSON-ish.
    for match in BRACKET.finditer(QUOTED.sub('"', line)):
        if not (JSON_ISH & set(match.group(1))):
            return True
    return False


def command_lines(body: str) -> list[str]:
    kept = []
    for raw in body.split("\n"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        kept.append(line)
    return kept


def classify(entry: dict) -> tuple[str, str]:
    """Return (class, the rule that decided it).

    Reported with the rule rather than as a bare label, because a
    classification nobody can audit is a number somebody will argue with.
    """
    language = entry["language"]
    if language in {"yaml", "yml", "toml", "json"}:
        return "fragment", f"{language} body: configuration, not a command line"
    if language == "http":
        return "request", "http body: a REST request, not a shell command"

    lines = command_lines(entry["body"])
    if not lines:
        return "empty", "no command lines"

    if any(line.startswith("$ ") for line in lines):
        return "transcript", "a `$ ` prompt, so the block shows a session rather than a command"

    metavariables = [line for line in lines if has_metavariable(line)]
    if metavariables:
        return "synopsis", ("optional-argument or metavariable notation in "
                            f"{len(metavariables)} of {len(lines)} line(s)")
    return "recipe", f"{len(lines)} literal command line(s), nothing to substitute"


def registry_examples() -> list[dict]:
    data = json.loads((ROOT / "docs/docaudit/registry.json").read_text(encoding="utf-8"))

    def walk(node):
        if isinstance(node, dict):
            for value in node.values():
                found = walk(value)
                if found is not None:
                    return found
        elif isinstance(node, list):
            if node and isinstance(node[0], dict) and "state" in node[0] and "path" in node[0]:
                return node
            for item in node:
                found = walk(item)
                if found is not None:
                    return found
        return None

    examples = walk(data)
    if examples is None:
        raise SystemExit("docs/docaudit/registry.json: no example list found")
    return examples


def clispec_commands() -> list[str]:
    data = json.loads((ROOT / "internal/clispec/commands.json").read_text(encoding="utf-8"))
    commands = []
    for command in data.get("commands", []):
        path = command.get("path") or []
        if path:
            commands.append(" ".join(path))
    return sorted(set(commands))


def configuration_keys() -> list[str]:
    """Every settable key, read from the generated table rather than re-derived.

    cmd/docconfig owns that table and its own test gates it against
    internal/config, so reading it here adds no third opinion about what a key
    is.
    """
    text = (ROOT / "docs/configuration.md").read_text(encoding="utf-8")
    return sorted(set(re.findall(r"^\| `([a-z0-9_.]+)` \|", text, re.M)))


def build() -> dict:
    examples = registry_examples()
    by_id = {entry["id"]: entry for entry in examples}

    # Extraction is matched to the registry by hash. A mismatch means this tool
    # and internal/docaudit disagree about what the documents contain, and a
    # report built on that would be measuring the wrong thing.
    bodies: dict[str, dict] = {}
    for path in sorted({entry["path"] for entry in examples}):
        document = ROOT / path
        identifier_base = path.removeprefix("docs/").removesuffix(".md").replace("/", "-")
        for fence in fences(document):
            bodies[f"{identifier_base}-{fence['section']}-example-{fence['ordinal']}"] = fence

    missing = sorted(set(by_id) - set(bodies))
    mismatched = sorted(name for name in set(by_id) & set(bodies)
                        if by_id[name]["sha256"] != bodies[name]["sha256"])
    if missing or mismatched:
        raise SystemExit(
            "extraction does not agree with docs/docaudit/registry.json; refusing to report.\n"
            f"  not found by this tool: {missing}\n"
            f"  hash mismatch: {mismatched}")

    rows = []
    for name, entry in sorted(by_id.items()):
        kind, rule = classify(bodies[name])
        rows.append({
            "id": name,
            "path": entry["path"],
            "section": entry["section"],
            "state": entry["state"],
            "class": kind,
            "rule": rule,
            "unrun_reason": (entry.get("unrun_reason") or {}).get("code", ""),
        })

    # The population that matters: a block a reader would paste, that nothing
    # has ever run.
    unrun_recipes = [r for r in rows if r["class"] in {"recipe", "transcript"}
                     and r["state"] != "executed"]

    # Commands with no executed *published* example.
    #
    # The wording matters and the first draft got it wrong by calling these
    # "never shown working". Most of them are exercised heavily -- by
    # cmd/notriosctl's own tests, by the I4, I7 and J3 drills, by the container
    # matrix -- so the claim is narrowly about the documentation: a reader of
    # docs/ sees a synopsis and never sees the command run. That is a
    # documentation gap, not an untested command, and conflating the two would
    # be the sort of number that makes a report untrustworthy.
    executed_bodies = "\n".join(bodies[r["id"]]["body"] for r in rows if r["state"] == "executed")
    commands = clispec_commands()
    undocumented_working = [command for command in commands
                            if f"notriosctl {command}" not in executed_bodies]

    keys = configuration_keys()
    configuration_examples = [r for r in rows if r["path"] == "docs/configuration.md"]

    per_document: dict[str, dict] = {}
    for row in rows:
        bucket = per_document.setdefault(row["path"], collections.Counter())
        bucket[f"{row['class']}/{row['state']}"] += 1
    documents = {path: dict(sorted(counts.items())) for path, counts in sorted(per_document.items())}

    return {
        "schema": "notrios.v10.j13a-example-classification.v1",
        "item": "J13-A",
        "milestone": "v1.0",
        "what_this_is": (
            "Every published fenced block, classified and cross-referenced against the "
            "execution registry, so the work that follows can tell a correct synopsis from a "
            "recipe nobody ran."),
        "content_commit": subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT,
                                         capture_output=True, text=True,
                                         check=True).stdout.strip(),
        "counts": {
            "examples": len(rows),
            "executed": sum(1 for r in rows if r["state"] == "executed"),
            "by_class": dict(sorted(collections.Counter(r["class"] for r in rows).items())),
            "by_class_and_state": dict(sorted(collections.Counter(
                f"{r['class']}/{r['state']}" for r in rows).items())),
        },
        "recipes_never_run": sorted(r["id"] for r in unrun_recipes),
        "commands_with_no_executed_published_example": {
            "of": len(commands),
            "count": len(undocumented_working),
            "means": ("No fenced block that this repository executes contains this command. It "
                      "says nothing about whether the command is tested -- most are, in "
                      "cmd/notriosctl's tests and in the I4, I7 and J3 drills. It says a reader "
                      "of docs/ is shown the form and never shown it run."),
            "commands": undocumented_working,
        },
        # Where the recorded reason and the block's content disagree. Found only
        # by crossing the two, which is the whole argument for this examination:
        # a synopsis cannot be run at all, so excusing one as "interactive" or
        # "shared user state" names a cause that could never have applied; and a
        # literal recipe excused as "illustrative" is a runnable block filed as
        # decoration.
        "reason_contradicts_class": sorted(
            [{"id": r["id"], "class": r["class"], "reason": r["unrun_reason"],
              "why": "a synopsis cannot be run, so this is not why it was not run"}
             for r in rows if r["class"] == "synopsis"
             and r["unrun_reason"] not in ("illustrative-placeholder", "")] +
            [{"id": r["id"], "class": r["class"], "reason": r["unrun_reason"],
              "why": "a literal recipe filed as illustrative; it is runnable or it is wrong"}
             for r in rows if r["class"] in ("recipe", "transcript")
             and r["unrun_reason"] == "illustrative-placeholder"],
            key=lambda entry: entry["id"]),
        "configuration_page": {
            "settable_keys": len(keys),
            "examples": len(configuration_examples),
            "note": ("J12 gave the page its prose and its generated key table and no examples "
                     "at all, which is the gap J13-C closes."),
        },
        "documents": documents,
        "examples": rows,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true",
                        help="fail if the committed report is not what the documents say")
    arguments = parser.parse_args()
    rendered = json.dumps(build(), indent=2) + "\n"
    if arguments.check:
        if not REPORT.is_file() or REPORT.read_text(encoding="utf-8") != rendered:
            print("J13-A report is stale; run "
                  "python3 performance/v1.0-j13/classify_examples.py", file=sys.stderr)
            return 1
        counts = json.loads(rendered)["counts"]
        print(f"J13-A classification current: {counts['examples']} examples, "
              f"{counts['executed']} executed")
        return 0
    REPORT.write_text(rendered, encoding="utf-8")
    counts = json.loads(rendered)["counts"]
    print(f"wrote {REPORT.relative_to(ROOT)}: {counts['examples']} examples, "
          f"{counts['executed']} executed, classes {counts['by_class']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
