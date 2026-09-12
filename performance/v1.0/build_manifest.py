#!/usr/bin/env python3
"""Generate v1.0's archive manifest from the archives themselves.

Fourth milestone, fourth builder. v0.9's said that if a fourth needed one, the
three should be folded into a single parameterised builder — so this one is that
fold: it takes the milestone and the per-item archives as data, and v0.9's file
now delegates to it rather than keeping a second copy of the same logic.

    python3 performance/v1.0/build_manifest.py [--check]
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

# Every archive so far was taken from the commit that closed its item, minutes
# after that commit. None is retroactive, and saying otherwise would be a lie in
# the direction that flatters the record.
TAKEN_AT_CLOSE = {
    "J1": "notrios-v1.0-j1-ab4186c.zip",
    "J2": "notrios-v1.0-j2-442a968.zip",
    "J3": "notrios-v1.0-j3-45d097a.zip",
}

NOTE = ("Each archive here was built from the commit that closed its item, by the same worktree "
        "path v0.8e used, and verified byte-identical to that commit. A file timestamp says when "
        "the archive was built and nothing about when the work happened.")


def build() -> dict:
    return simple_manifest("v1.0", TAKEN_AT_CLOSE, NOTE)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true",
                        help="fail if the committed manifest is not what the archives say")
    arguments = parser.parse_args()
    rendered = json.dumps(build(), indent=2) + "\n"
    if arguments.check:
        if not MANIFEST.is_file() or MANIFEST.read_text() != rendered:
            print("MANIFEST.json is stale; run python3 performance/v1.0/build_manifest.py",
                  file=sys.stderr)
            return 1
        print("v1.0 archive manifest current")
        return 0
    MANIFEST.write_text(rendered)
    counts = json.loads(rendered)["counts"]
    print(f"wrote {MANIFEST.relative_to(ROOT)}: {counts['items']} items, "
          f"{counts['total_bytes']:,} bytes")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
