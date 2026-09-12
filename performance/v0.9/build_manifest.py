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
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
sys.path.insert(0, str(ROOT / "performance" / "v0.8e"))
from build_manifest import simple_manifest  # noqa: E402

MANIFEST = HERE / "MANIFEST.json"

# Every archive so far in this milestone was taken from the commit that closed
# its item, minutes after that commit. None is retroactive, and saying otherwise
# would be a lie in the direction that flatters the record.
TAKEN_AT_CLOSE = {"I1": "notrios-v0.9-i1-3799c6f.zip",
                  "I3": "notrios-v0.9-i3-078ffc5.zip",
                  "I4": "notrios-v0.9-i4-d1d8ec0.zip",
                  "I5": "notrios-v0.9-i5-02b15ed.zip",
                  "I6": "notrios-v0.9-i6-e7a535c.zip",
                  "I7": "notrios-v0.9-i7-f67f301.zip",
                  "I8": "notrios-v0.9-i8-6d2a71c.zip",
                  "I9": "notrios-v0.9-i9-eb217c3.zip",
                  # Supersedes notrios-v0.9-i6-79f940f.zip. I6 gained a fourth slice
                  # after it first closed -- the SBOM cross-check and the dated scan --
                  # so the archive that represents the finished item is the one built
                  # from the commit that finished it. The earlier archive is not
                  # deleted; it stays in the evidence directory as what I6 was.
                  "I10": "notrios-v0.9-i10-b19721a.zip",
                  "I2": "notrios-v0.9-i2-e3f6de7.zip"}


NOTE = ("Each archive here was built from the commit that closed its item, by the same worktree "
        "path v0.8e used, and verified byte-identical to that commit. A file timestamp says when "
        "the archive was built and nothing about when the work happened. I2 is deferred rather "
        "than complete: its archive records the commit that made and explained that decision, "
        "which is the work the item produced.")


def build() -> dict:
    # Folded into performance/v0.8e/build_manifest.py by v1.0 J1, which is the
    # fourth milestone this note said should do it.
    return simple_manifest("v0.9", TAKEN_AT_CLOSE, NOTE)


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
