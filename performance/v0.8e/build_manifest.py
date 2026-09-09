#!/usr/bin/env python3
"""Generate the v0.8 archive manifest from the archives themselves.

Generated rather than typed, for the reason this repository keeps rediscovering:
a table somebody maintains by hand is a table that stops being true. Every field
here is read from the file it describes or from git, so the manifest cannot
disagree with the evidence directory unless the directory changed.

What it records that a directory listing cannot: which item each archive belongs
to, which commit it was taken from, and whether it was taken when that slice
closed or retroactively in v0.8e. That last one is the honest part. An archive
built now has a file timestamp of now; only the record can say the work it
carries is older, and only the reserve's signature and RFC 3161 timestamp can
make the record checkable by somebody else.

    python3 performance/v0.8e/build_manifest.py [--check]
"""
import argparse
import collections
import hashlib
import json
import pathlib
import subprocess
import sys
import zipfile

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
EVIDENCE = pathlib.Path.home() / "evidence" / "notrios"
MANIFEST = HERE / "MANIFEST.json"

# The two archives v0.8 took at the time and that survive review: their trees
# match their commits and today's release check accepts them.
TAKEN_AT_CLOSE = {
    "H0": "notrios-v0.8-h0-e164aeb.zip",
    "H4": "notrios-v0.8-h4-693b672.zip",
}


def git(*args: str) -> str:
    return subprocess.run(["git", "-C", str(ROOT), *args],
                          capture_output=True, text=True, check=True).stdout.strip()


def describe(path: pathlib.Path) -> dict:
    data = path.read_bytes()
    with zipfile.ZipFile(path) as archive:
        entries = len(archive.namelist())
    return {"bytes": len(data), "entries": entries,
            "sha256": hashlib.sha256(data).hexdigest()}


def build() -> dict:
    closeouts = {e["item"]: e for e in json.loads((HERE / "CLOSEOUTS.json").read_text())["items"]}
    rebuilt = {}
    for line in (HERE / "RESULTS.jsonl").read_text().splitlines():
        row = json.loads(line)
        rebuilt[row["item"]] = row

    ledger = json.loads(git("show", "88f51d3^:docs/docplan/PLAN_SLICES.json"))
    manifest = collections.OrderedDict()
    manifest["schema"] = "notrios.v08e.archive-manifest.v1"
    manifest["milestone"] = "v0.8"
    manifest["evidence_directory"] = str(EVIDENCE)
    manifest["honest_provenance"] = (
        "Every archive marked retroactive was built in v0.8e, after its slice closed. Its file "
        "timestamp is the date it was built and says nothing about when the work happened. What "
        "attests the sealing is the reserve's OpenPGP signature and RFC 3161 timestamp; nothing "
        "here attests when the work was done.")
    manifest["archives"] = []

    for item in [i["id"] for i in ledger["items"]]:
        entry = collections.OrderedDict(item=item)
        if item in rebuilt:
            row, close = rebuilt[item], closeouts[item]
            path = EVIDENCE / row["archive"]
            entry["archive"] = row["archive"]
            entry["commit"] = row["commit"]
            entry["taken"] = "retroactively in v0.8e"
            if "supersedes" in close:
                entry["supersedes"] = close["supersedes"]
                entry["superseded_because"] = close["resolved_by"]
        else:
            path = EVIDENCE / TAKEN_AT_CLOSE[item]
            entry["archive"] = TAKEN_AT_CLOSE[item]
            entry["commit"] = git("rev-parse", TAKEN_AT_CLOSE[item].rsplit("-", 1)[1][:-4])
            entry["taken"] = "when the slice closed"
        if not path.is_file():
            raise SystemExit(f"{item}: {path} is not there")
        entry.update(describe(path))
        manifest["archives"].append(entry)

    manifest["counts"] = {
        "items": len(manifest["archives"]),
        "retroactive": sum(1 for a in manifest["archives"] if a["taken"] != "when the slice closed"),
        "superseding_an_earlier_archive": sum(1 for a in manifest["archives"] if "supersedes" in a),
        "total_bytes": sum(a["bytes"] for a in manifest["archives"]),
    }
    return manifest


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true",
                        help="fail if the committed manifest is not what the archives say")
    arguments = parser.parse_args()
    manifest = build()
    rendered = json.dumps(manifest, indent=2) + "\n"
    if arguments.check:
        if not MANIFEST.is_file() or MANIFEST.read_text() != rendered:
            print("MANIFEST.json is stale; run python3 performance/v0.8e/build_manifest.py",
                  file=sys.stderr)
            return 1
        counts = manifest["counts"]
        print(f"v0.8 archive manifest current: {counts['items']} items, "
              f"{counts['retroactive']} retroactive, {counts['total_bytes'] / 1e9:.2f} GB")
        return 0
    MANIFEST.write_text(rendered)
    counts = manifest["counts"]
    print(f"wrote {MANIFEST.relative_to(ROOT)}: {counts['items']} items, "
          f"{counts['retroactive']} retroactive, "
          f"{counts['superseding_an_earlier_archive']} superseding an earlier archive")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
