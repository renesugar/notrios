#!/usr/bin/env python3
"""Fail when a finished plan item has no archive in the record.

Packaging stopped after H4 and nobody noticed for thirty-one items and two
milestones. The answer this repository gives to that class of failure is a test,
so this is one, and it runs in `make validate` rather than at release time --
because a gate nobody runs until release is a gate that reports the damage
instead of preventing it.

It reads the *record*, not the evidence directory. `/home/renes/evidence/notrios`
is not in the repository and is not on every machine, so a gate that looked there
would pass vacuously wherever the archives are absent. What it checks is that
every item a plan's own progress table records as complete or deferred appears in
that milestone's archive manifest. Coverage, not contents: it cannot verify a
hash it has no file for and it does not pretend to. Verifying bytes is the
reserve verifier's job, with a device attached.

The registry below is deliberately small, and the gate refuses a milestone that
is missing from it. Registering v0.8e was not decoration -- adding the check
immediately found that E1 and E2 had already repeated the failure the milestone
exists to end.
"""
import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]

# The first milestone the evidence record covers. Earlier plans closed before
# there was a record to be missing from, and are not retroactively in scope.
COVERAGE_BEGINS = (0, 8)

# milestone -> (the plan document whose table is the ledger, its archive manifest)
REGISTRY = {
    "v0.8": ("plans/v0.8/000-v0.8-plan.md", "performance/v0.8e/MANIFEST.json"),
    "v0.8e": ("plans/v0.8e/000-v0.8e-plan.md", "performance/v0.8e/MANIFEST_V08E.json"),
    "v0.9": ("PLAN.md", "performance/v0.9/MANIFEST.json"),
}

FINISHED = {"complete", "deferred"}
ROW = re.compile(r"^\|\s*([A-Za-z]+[0-9]+[a-z]?)\.\s[^|]*\|\s*([a-z-]+)\s*\|")


class CoverageError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise CoverageError(message)


def milestone_order(name: str) -> tuple:
    """Order v0.8 before v0.8e before v0.9, so a newer plan cannot slip past."""
    match = re.fullmatch(r"v(\d+)\.(\d+)([a-z]*)", name)
    require(match is not None, f"unrecognised milestone name: {name}")
    return (int(match.group(1)), int(match.group(2)), match.group(3))


def finished_items(document: pathlib.Path) -> dict[str, str]:
    """Read a plan's generated progress table: item id -> state."""
    require(document.is_file(), f"no plan document at {document}")
    text = document.read_text(encoding="utf-8")
    begin = text.find("<!-- notrios:generated:plan:progress:begin -->")
    end = text.find("<!-- notrios:generated:plan:progress:end -->")
    require(begin != -1 and end > begin,
            f"{document} has no generated progress block to read")
    items = {}
    for line in text[begin:end].splitlines():
        match = ROW.match(line)
        if match:
            items[match.group(1)] = match.group(2)
    require(items, f"{document} has a progress block with no item rows")
    return {item: state for item, state in items.items() if state in FINISHED}


def check() -> tuple[int, int]:
    # Every milestone at or after the coverage start must be registered. A new
    # plan directory that nobody registered is exactly how the last omission
    # survived, so it fails here rather than being skipped silently.
    known = {"v0.8e"} | {path.name for path in (ROOT / "plans").iterdir() if path.is_dir()}
    for name in sorted(known):
        if re.fullmatch(r"v\d+\.\d+[a-z]*", name) and milestone_order(name) >= COVERAGE_BEGINS:
            require(name in REGISTRY,
                    f"milestone {name} closed without an archive manifest registered in "
                    f"{pathlib.Path(__file__).name}; register it or the next milestone loses "
                    f"its archives the way v0.8 lost twenty-seven")

    milestones = covered = 0
    for milestone, (plan_path, manifest_path) in sorted(REGISTRY.items()):
        items = finished_items(ROOT / plan_path)
        manifest = ROOT / manifest_path
        if not items and not manifest.is_file():
            # A milestone that has finished nothing yet has nothing to record.
            # Requiring the manifest before the first item closes would make the
            # gate demand a file describing an empty set, and a gate that asks
            # for something meaningless is one people learn to work around.
            milestones += 1
            continue
        require(manifest.is_file(), f"{milestone}: no archive manifest at {manifest_path}")
        archives = json.loads(manifest.read_text(encoding="utf-8"))["archives"]
        recorded = {entry["item"] for entry in archives}
        missing = sorted(set(items) - recorded)
        require(not missing,
                f"{milestone}: finished with no archive in {manifest_path}: {', '.join(missing)}")
        # The other direction matters too: an archive for an item the plan does
        # not record finished means the record and the plan have drifted apart,
        # and there is no way to tell which one is wrong without looking.
        stray = sorted(recorded - set(items))
        require(not stray,
                f"{milestone}: {manifest_path} records archives for items the plan does not "
                f"record as finished: {', '.join(stray)}")
        milestones += 1
        covered += len(items)
    return milestones, covered


def main() -> int:
    milestones, covered = check()
    print(f"archive coverage valid: {covered} finished plan items across {milestones} milestones "
          f"each have an archive in the record")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except CoverageError as error:
        print(f"archive coverage invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
