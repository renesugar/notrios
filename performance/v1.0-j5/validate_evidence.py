#!/usr/bin/env python3
"""Check J5's record against itself and against the code it describes.

The measurements cannot be re-derived on demand -- they took eleven hours and
the libraries live outside the repository -- so this checks the properties that
would make the record misleading if they stopped holding, and the one claim
about the source tree that a later edit could silently falsify.
"""
from __future__ import annotations

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
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v10.j5-scale.v1", "wrong schema")

    runs = report["runs"]
    require(set(runs) == {"joplin-export-with-resources", "recipe-vault-obsidian",
                          "recipe-joplin-raw"},
            "the three measured corpora are not the three recorded")

    # Every recorded run completed. J5's boundary is that an unfinished
    # measurement is reported as unfinished, so a run marked complete must say
    # so in its own profile rather than in prose around it.
    for label, run in runs.items():
        profile = run["import"]
        require(profile and profile.get("completed"),
                f"{label}: recorded without a completed import profile")
        require(profile["import"].get("checkpoint_status") == "completed",
                f"{label}: the importer did not report a completed checkpoint")
        for step, detail in run["backup_restore"].items():
            require(detail["completed"], f"{label}: the {step} step did not complete")

    # The pair is only a controlled comparison if both imported the same notes.
    vault = runs["recipe-vault-obsidian"]["import"]["import"]["notes_imported"]
    joplin = runs["recipe-joplin-raw"]["import"]["import"]["notes_imported"]
    require(vault == joplin == 382206,
            f"the recipe pair no longer imported the same note count: {vault} and {joplin}")

    # The comparison names what it measured. A number without the version and
    # without what the other tool builds is the kind of ratio this record exists
    # to avoid publishing.
    comparison = report["comparison"]
    for field in ("tool", "commit", "seconds", "peak_rss_kib", "what_it_builds",
                  "what_notrios_builds_instead", "so_the_ratio_is_not_like_for_like"):
        require(comparison.get(field), f"the comparison does not record {field}")
    require(len(comparison["commit"]) == 40, "the compared tool's commit is not a full sha")

    # The finding that becomes J17, checked against the source rather than the
    # counters. If somebody fixes it, this fails and the record gets corrected;
    # if somebody removes the batch API, this fails too.
    obsidian = (ROOT / "internal/importers/obsidian/obsidian.go").read_text(encoding="utf-8")
    joplin_src = (ROOT / "internal/importers/joplinraw/scalable.go").read_text(encoding="utf-8")
    require("RebuildImportDocumentLinksBatch" in joplin_src,
            "the Joplin importer no longer uses the batch link rebuild, so J5's comparison "
            "no longer describes this tree")
    # J5 measured the tree as it was. Once J17-B batched the importer, the
    # record stays valid only if it says so beside the claim, pointing at the
    # re-measurement rather than silently describing code that is gone.
    if "RebuildDocumentLinks(run.ctx, note.TargetID)" not in obsidian:
        require("RebuildImportDocumentLinksBatch" in obsidian
                and "ApplyImportDocumentBatch" in obsidian,
                "the Obsidian importer no longer rebuilds links per document, and does not "
                "use the batch methods either -- this record no longer describes the tree")
        readme = (ROOT / "performance/v1.0-j5/README.md").read_text(encoding="utf-8")
        require("J17-B" in readme and "performance/v1.0-j17" in readme,
                "the Obsidian importer was batched, but J5's record does not say so -- "
                "correct the record beside the claim and point at J17's re-measurement")

    print(f"J5 record valid: 3 corpora, {vault:,} notes in the controlled pair, "
          f"round trip measured, comparison against {comparison['tool']} "
          f"at {comparison['commit'][:7]} recorded with what each tool builds")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"J5 record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
