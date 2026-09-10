#!/usr/bin/env python3
"""Generate v0.9's archive manifest from the archives themselves.

Third milestone, third builder, and the reason each one exists separately is
that the milestone-specific part is real: which items the milestone has, and
which of their archives were taken when the slice closed rather than
retroactively. What must not diverge -- how an archive is measured -- is
imported from v0.8e's builder rather than copied again.

If a fourth milestone needs one of these, fold the three into a single
parameterised builder. Three is where a pattern becomes visible; it is not yet
where a shared abstraction is better than a short file that says exactly what
this milestone did.

    python3 performance/v0.9/build_manifest.py [--check]
"""
import argparse
import collections
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
sys.path.insert(0, str(ROOT / "performance" / "v0.8e"))
from build_manifest import EVIDENCE, describe, git  # noqa: E402

MANIFEST = HERE / "MANIFEST.json"

# Every archive so far in this milestone was taken from the commit that closed
# its item, minutes after that commit. None is retroactive, and saying otherwise
# would be a lie in the direction that flatters the record.
TAKEN_AT_CLOSE = {"I1": "notrios-v0.9-i1-3799c6f.zip",
                  "I3": "notrios-v0.9-i3-078ffc5.zip",
                  "I4": "notrios-v0.9-i4-d1d8ec0.zip",
                  "I5": "notrios-v0.9-i5-02b15ed.zip",
                  "I6": "notrios-v0.9-i6-79f940f.zip",
                  "I10": "notrios-v0.9-i10-b19721a.zip",
                  "I2": "notrios-v0.9-i2-e3f6de7.zip"}


def build() -> dict:
    manifest = collections.OrderedDict()
    manifest["schema"] = "notrios.v08e.archive-manifest.v1"
    manifest["milestone"] = "v0.9"
    manifest["evidence_directory"] = str(EVIDENCE)
    manifest["honest_provenance"] = (
        "Each archive here was built from the commit that closed its item, by the same "
        "worktree path v0.8e used, and verified byte-identical to that commit. A file "
        "timestamp says when the archive was built and nothing about when the work happened. "
        "I2 is deferred rather than complete: its archive records the commit that made and "
        "explained that decision, which is the work the item produced.")
    manifest["archives"] = []
    for item, name in sorted(TAKEN_AT_CLOSE.items()):
        path = EVIDENCE / name
        if not path.is_file():
            raise SystemExit(f"{item}: {path} is not there")
        entry = collections.OrderedDict(item=item, archive=name)
        entry["commit"] = git("rev-parse", name.rsplit("-", 1)[1][:-4])
        entry["taken"] = "when the slice closed"
        entry.update(describe(path))
        manifest["archives"].append(entry)
    manifest["counts"] = {
        "items": len(manifest["archives"]),
        "retroactive": sum(1 for a in manifest["archives"] if a["taken"] != "when the slice closed"),
        "total_bytes": sum(a["bytes"] for a in manifest["archives"]),
    }
    return manifest


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true",
                        help="fail if the committed manifest is not what the archives say")
    arguments = parser.parse_args()
    rendered = json.dumps(build(), indent=2) + "\n"
    if arguments.check:
        if not MANIFEST.is_file() or MANIFEST.read_text() != rendered:
            print("MANIFEST.json is stale; run python3 performance/v0.9/build_manifest.py",
                  file=sys.stderr)
            return 1
        print("v0.9 archive manifest current")
        return 0
    MANIFEST.write_text(rendered)
    counts = json.loads(rendered)["counts"]
    print(f"wrote {MANIFEST.relative_to(ROOT)}: {counts['items']} items, "
          f"{counts['total_bytes']:,} bytes")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
