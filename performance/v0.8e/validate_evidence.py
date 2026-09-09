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

    sealed = check_seal_record()

    print(f"v0.8 archive record valid: {counts['items']} items, {counts['retroactive']} retroactive, "
          f"{counts['superseding_an_earlier_archive']} superseding, "
          f"{counts['total_bytes'] / 1e9:.2f} GB; handoff table matches; "
          f"volume-0002 seals {sealed} and accounts for every candidate it did not")


def check_seal_record() -> int:
    """Hold the seal record against the plan it claims to fulfil.

    E3 sealed 38 of the 40 non-superseded candidates the plan named, so the
    difference has to be accounted for by name and not by a sentence somebody
    updates by hand. Every candidate must end up in exactly one of three
    places -- sealed, left out as superseded, or refused by the media-type
    policy -- and the third group must be exactly the files that policy
    actually refuses, computed here rather than transcribed. A record that
    merely asserts "38 of 40" would go stale the moment either number moved.
    """
    plan = json.loads((HERE / "VOLUME_PLAN.json").read_text(encoding="utf-8"))
    result = json.loads((HERE / "SEAL_RESULT.json").read_text(encoding="utf-8"))
    require(result["schema"] == "notrios.v08e.seal-result.v1", "wrong seal-result schema")

    candidates = {c["name"]: c for c in plan["candidates"]}
    sealed = set(result["sealed"])
    superseded = set(result["excluded_superseded"])
    unapproved = set(result["excluded_unapproved_media_type"])

    groups = [sealed, superseded, unapproved]
    for index, group in enumerate(groups):
        for other in groups[index + 1:]:
            require(not (group & other), "a candidate is in two groups of the seal record")
    require(sealed | superseded | unapproved == set(candidates),
            "the seal record does not account for every candidate the volume plan named")
    require(result["sealed_count"] == len(sealed), "the sealed count disagrees with the list")
    require(result["sealed_bytes"] == sum(candidates[name]["bytes"] for name in sealed),
            "the sealed byte total disagrees with the plan's sizes")
    require(result["sealed_bytes"] < result["budget_bytes"],
            "the sealed volume claims to exceed its own budget")

    # The approved media types live in the sealing tool; recomputing the refused
    # set here means the record cannot claim a file was refused when it was not.
    approved = (".zip", ".png")
    require(superseded == {name for name, c in candidates.items()
                           if c["role"] == "superseded by a v0.8e rebuild"},
            "the superseded group is not the set the plan marks superseded")
    require(unapproved == {name for name in candidates
                           if name not in superseded and not name.lower().endswith(approved)},
            "the media-type exclusions are not the files the policy actually refuses")
    require(all(name.lower().endswith(approved) for name in sealed),
            "a sealed artifact is not an approved media type")
    require(result["clean_builds"] == 2 and result["byte_identical"] is True,
            "the volume does not record two byte-identical builds")
    require(result["predecessor_volume_id"] == "NTR-EV-0001",
            "volume-0002 does not chain onto volume-0001")
    return len(sealed)


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, KeyError, ValueError) as error:
        print(f"v0.8 archive record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
