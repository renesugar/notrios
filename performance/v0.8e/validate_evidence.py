#!/usr/bin/env python3
"""Validate the v0.8 archive record without needing the archives.

The evidence directory is not in the repository and is not on every machine, so
this checks the committed record: that the manifest is internally coherent, that
it accounts for every v0.8 item, and that the table in `CODING_CLIENT_HANDOFF.md`
is the one the manifest renders. Verifying the *bytes* needs the archives and the
reserve verifier; this is the half that can run in `make validate`, and running
it there is the point — the four-row table it replaces survived thirty-one slices
closing without an archive because nothing compared it to anything.
"""
import json
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class EvidenceError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def render_table(manifest) -> str:
    rows = []
    for archive in manifest["archives"]:
        taken = "at close" if archive["taken"] == "when the slice closed" else "v0.8e"
        if "supersedes" in archive:
            taken = "v0.8e, replacing an earlier one"
        rows.append(f"| {archive['item']} | `{archive['commit'][:7]}` | {archive['bytes']:,} | "
                    f"{archive['entries']:,} | `{archive['sha256'][:16]}` | {taken} |")
    return ("| Slice | Commit | Bytes | Entries | SHA-256 (first 16) | Taken |\n"
            "| --- | --- | --- | --- | --- | --- |\n" + "\n".join(rows))


def main() -> None:
    manifest = json.loads((HERE / "MANIFEST.json").read_text(encoding="utf-8"))
    require(manifest["schema"] == "notrios.v08e.archive-manifest.v1", "wrong manifest schema")

    archives = manifest["archives"]
    items = [a["item"] for a in archives]
    require(len(items) == len(set(items)), "an item appears twice in the manifest")

    # Every archive says which commit it came from and when it was taken. A row
    # that omits "taken" is a row that has stopped distinguishing an archive
    # made at the time from one made afterwards, which is the whole point.
    for archive in archives:
        for field in ("item", "archive", "commit", "taken", "bytes", "entries", "sha256"):
            require(archive.get(field) not in (None, ""), f"{archive.get('item')}: no {field}")
        require(len(archive["commit"]) == 40, f"{archive['item']}: commit is not a full sha")
        require(len(archive["sha256"]) == 64, f"{archive['item']}: archive hash is not a sha256")
        if "supersedes" in archive:
            require(archive.get("superseded_because"),
                    f"{archive['item']} replaces {archive['supersedes']} and does not say why")

    hashes = [a["sha256"] for a in archives]
    require(len(hashes) == len(set(hashes)), "two items claim the same archive bytes")

    counts = manifest["counts"]
    require(counts["items"] == len(archives), "the count disagrees with the list beneath it")
    require(counts["retroactive"] == sum(1 for a in archives if a["taken"] != "when the slice closed"),
            "the retroactive count disagrees with the rows")
    require(counts["total_bytes"] == sum(a["bytes"] for a in archives), "the byte total disagrees")

    # An archive built after its slice closed must be labelled so, and the
    # manifest must say what the label does not mean. Dropping that sentence
    # would leave a record that reads as though the archives were contemporary.
    require("retroactive" in manifest["honest_provenance"]
            and "says nothing about when the work happened" in manifest["honest_provenance"],
            "the manifest no longer says what a retroactive archive's timestamp does not attest")

    handoff = (ROOT / "CODING_CLIENT_HANDOFF.md").read_text(encoding="utf-8")
    require(render_table(manifest) in handoff,
            "the snapshot table in CODING_CLIENT_HANDOFF.md is not what MANIFEST.json renders; "
            "run python3 performance/v0.8e/build_manifest.py and update the table")

    print(f"v0.8 archive record valid: {counts['items']} items, {counts['retroactive']} retroactive, "
          f"{counts['superseding_an_earlier_archive']} superseding, "
          f"{counts['total_bytes'] / 1e9:.2f} GB; handoff table matches")


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError) as error:
        print(f"v0.8 archive record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
