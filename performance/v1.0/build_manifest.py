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
    "J12": "notrios-v1.0-j12-ba7e431.zip",
    "J4": "notrios-v1.0-j4-9d6e38a.zip",
    "J5": "notrios-v1.0-j5-fde1330.zip",
    "J16": "notrios-v1.0-j16-792f406.zip",
    "J18": "notrios-v1.0-j18-12cc1c9.zip",
    "J13": "notrios-v1.0-j13-088b488.zip",
    "J14": "notrios-v1.0-j14-2cfaeed.zip",
    "J17": "notrios-v1.0-j17-e0ecc47.zip",
    "J19": "notrios-v1.0-j19-ae0eb90.zip",
    "J21": "notrios-v1.0-j21-a6de139.zip",
    "J20": "notrios-v1.0-j20-f5b7102.zip",
    "J23": "notrios-v1.0-j23-bb68681.zip",
    "J7": "notrios-v1.0-j7-d331551.zip",
    "J24": "notrios-v1.0-j24-b92f715.zip",
    "J22": "notrios-v1.0-j22-07f3acc.zip",
    "J25": "notrios-v1.0-j25-2fd67ec.zip",
    "J26": "notrios-v1.0-j26-fe024ae.zip",
    "J8": "notrios-v1.0-j8-0f0f956.zip",
    "J27": "notrios-v1.0-j27-ab6fd32.zip",
    "J28": "notrios-v1.0-j28-2253560.zip",
    "J30": "notrios-v1.0-j30-0b47dd5.zip",
    "J29": "notrios-v1.0-j29-dc866c1.zip",
    "J34": "notrios-v1.0-j34-dbae64d.zip",
    "J33": "notrios-v1.0-j33-f8749e6.zip",
    "J31": "notrios-v1.0-j31-d8c67d4.zip",
    "J35": "notrios-v1.0-j35-f3d0cdf.zip",
    "J11": "notrios-v1.0-j11-42bf484.zip",
    "J6": "notrios-v1.0-j6-7a7d688.zip",
    "J15": "notrios-v1.0-j15-5a2efdb.zip",
    "J32": "notrios-v1.0-j32-2937c76.zip",
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
