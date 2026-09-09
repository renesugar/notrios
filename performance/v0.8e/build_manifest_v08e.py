#!/usr/bin/env python3
"""Generate v0.8e's own archive manifest from the archives themselves.

E2 built the v0.8 manifest and E4 added the gate that reads it -- and the gate's
first act was to report that v0.8e had already repeated v0.8's omission: E1 and
E2 closed without a handoff archive. So this milestone needs the same record it
demanded of the previous one, which is the point of a gate that includes the
milestone that wrote it.

Separate from `build_manifest.py` rather than folded into it: that script is the
record of how the v0.8 manifest was produced, it pins a git revision for the
v0.8 ledger, and its `TAKEN_AT_CLOSE` set is a statement about v0.8. Reusing its
`describe` keeps the one thing that must not diverge -- how an archive is
measured -- in a single place.

    python3 performance/v0.8e/build_manifest_v08e.py [--check]
"""
import argparse
import collections
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
sys.path.insert(0, str(HERE))
from build_manifest import EVIDENCE, describe, git  # noqa: E402

MANIFEST = HERE / "MANIFEST_V08E.json"
RESULTS = ROOT / "dist" / "backfill-v08e" / "RESULTS.jsonl"

# E3's archive was built from its own close-out commit by the full release gate,
# minutes after that commit, in the session that closed it. That is an archive
# taken when the slice closed, and saying otherwise would be a lie in the
# direction that flatters the record.
TAKEN_AT_CLOSE = {"E3": "notrios-v0.8e-e3-d24c02e.zip",
                  "E4": "notrios-v0.8e-e4-3ea7e3f.zip",
                  "E5": "notrios-v0.8e-e5-68a0eac.zip"}


def build() -> dict:
    rebuilt = {}
    if RESULTS.is_file():
        for line in RESULTS.read_text().splitlines():
            row = json.loads(line)
            rebuilt[row["item"]] = row

    manifest = collections.OrderedDict()
    manifest["schema"] = "notrios.v08e.archive-manifest.v1"
    manifest["milestone"] = "v0.8e"
    manifest["evidence_directory"] = str(EVIDENCE)
    manifest["honest_provenance"] = (
        "E1 and E2 closed without an archive and theirs were built in E4, from their own "
        "close-out commits in a disposable worktree. Their file timestamps are the date they "
        "were built and say nothing about when the work happened. E3's was taken when the "
        "slice closed, and so was E4's -- from the commit that implements it, because the "
        "entry recording an archive cannot live inside the commit that archive is built "
        "from. That is the closure boundary the reserve already documents for a volume "
        "that cannot contain its own final hash, not a gap in the record. Nothing here "
        "attests when any of the work was done; what attests a sealing is the reserve's "
        "OpenPGP signature and RFC 3161 timestamp.")
    manifest["archives"] = []

    for item in ("E1", "E2", "E3", "E4", "E5"):
        entry = collections.OrderedDict(item=item)
        if item in TAKEN_AT_CLOSE:
            name = TAKEN_AT_CLOSE[item]
            entry["archive"] = name
            entry["commit"] = git("rev-parse", name.rsplit("-", 1)[1][:-4])
            entry["taken"] = "when the slice closed"
        else:
            row = rebuilt.get(item)
            if row is None:
                raise SystemExit(f"{item}: no archive in {RESULTS.relative_to(ROOT)}; "
                                 f"run performance/v0.8e/backfill.sh with the v0.8e closeouts")
            entry["archive"] = row["archive"]
            entry["commit"] = git("rev-parse", row["commit"])
            entry["taken"] = "retroactively in v0.8e E4"
        path = EVIDENCE / entry["archive"]
        if not path.is_file():
            raise SystemExit(f"{item}: {path} is not there")
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
            print("MANIFEST_V08E.json is stale; run "
                  "python3 performance/v0.8e/build_manifest_v08e.py", file=sys.stderr)
            return 1
        print("v0.8e archive manifest current")
        return 0
    MANIFEST.write_text(rendered)
    counts = json.loads(rendered)["counts"]
    print(f"wrote {MANIFEST.relative_to(ROOT)}: {counts['items']} items, "
          f"{counts['retroactive']} retroactive, {counts['total_bytes']:,} bytes")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
