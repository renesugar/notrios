#!/usr/bin/env python3
"""Re-derive J13-A's classification and refuse a report that no longer matches.

The report is a set of claims about files that change every time somebody edits
documentation, so it is derived on every run rather than read. `--check` in the
classifier does the comparison; this adds the properties that must hold for the
classification to be worth anything, each of which was a real mistake first.
"""
from __future__ import annotations

import importlib.util
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class EvidenceError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    spec = importlib.util.spec_from_file_location("j13a", HERE / "classify_examples.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules["j13a"] = module
    spec.loader.exec_module(module)

    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v10.j13a-example-classification.v1", "wrong schema")

    # Derived, not read. build() re-extracts every fence and re-matches it to
    # the registry by hash, so this fails if a document changed and the report
    # did not -- and fails inside build() if the extraction stops agreeing with
    # internal/docaudit at all.
    derived = module.build()
    for field in ("counts", "documents", "recipes_never_run", "reason_contradicts_class",
                  "commands_with_no_executed_published_example"):
        require(derived[field] == report[field],
                f"{field} is stale; run python3 performance/v1.0-j13/classify_examples.py")

    rows = report["examples"]
    require(len(rows) == report["counts"]["examples"], "the example list and the count disagree")

    # No synopsis is executed. This is the classification's own consistency
    # check and the reason it can be trusted: a block with a metavariable in it
    # cannot have been run, so a single synopsis marked executed means the
    # classifier is wrong. Three drafts of the metavariable rule were caught
    # exactly here -- Markdown link syntax in a JSON payload, a jq filter, and
    # a quoted JSON array whose brackets read as optional-argument notation.
    executed_synopses = [r["id"] for r in rows if r["class"] == "synopsis"
                         and r["state"] == "executed"]
    require(not executed_synopses,
            "classified as a synopsis and recorded as executed, so the classifier is wrong "
            f"about: {executed_synopses}")

    # Every block is classified, and every class is one this report defines.
    known = {"synopsis", "recipe", "transcript", "fragment", "request", "empty"}
    unknown = sorted({r["class"] for r in rows} - known)
    require(not unknown, f"unknown classification: {unknown}")
    require(all(r["rule"] for r in rows),
            "a block was classified without recording the rule that decided it")

    # The classification covers the registry exactly. A block the registry
    # publishes and this does not classify is a block nobody looked at.
    registry_ids = {entry["id"] for entry in module.registry_examples()}
    require({r["id"] for r in rows} == registry_ids,
            "the classification and docs/docaudit/registry.json cover different blocks")

    # No recorded reason may contradict its own block. This was a count in
    # J13-A and is a gate since J13-D closed the ten it found: a synopsis
    # excused as "interactive" names a cause that could never have applied, and
    # a literal recipe filed as "illustrative" is a runnable command called
    # decoration. Both are how a documented command stops being checked without
    # anybody deciding to stop checking it.
    #
    # Fixing the ten and leaving the number in a report would have meant the
    # eleventh arrives silently, which is the failure this repository keeps
    # finding in its own gates.
    contradictions = report["reason_contradicts_class"]
    require(not contradictions,
            "a recorded reason contradicts its block: "
            + "; ".join(f"{c['id']} is a {c['class']} excused as {c['reason']}"
                        for c in contradictions))

    print(f"J13-A classification valid: {report['counts']['examples']} blocks "
          f"({report['counts']['by_class']}), "
          f"{len(report['recipes_never_run'])} recipes never run, "
          "no recorded reason contradicts its block")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"J13-A record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
