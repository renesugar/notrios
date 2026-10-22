#!/usr/bin/env python3
"""Plan, and hold to the plan, the reserve volumes that seal v1.0's evidence.

    python3 performance/v1.0-j10/plan_volumes.py              # check (validation)
    python3 performance/v1.0-j10/plan_volumes.py --write      # rewrite VOLUME_PLAN.json
    python3 performance/v1.0-j10/plan_volumes.py --names NTR-EV-0005 OUT.json

The candidates are v1.0's archive manifest, not a directory listing: the
manifest is committed with every archive's size and hash, so the plan can be
derived and checked on a machine that has no evidence directory, which is every
CI runner. The split is computed rather than chosen. The archives go in item
order into the fewest volumes the budget allows, cut where the larger volume is
smallest, so neither volume is filled to its last few megabytes.

Once a volume is sealed its committed materials say what it holds:
`evidence/volumes/volume-NNNN/SIGNATURES/` has one signature per artifact. The
check holds those against the plan, and the chain of predecessors against the
order, so a volume sealed with a different set, or chained onto the wrong
volume, fails here rather than in a sentence somebody updates by hand.
"""
from __future__ import annotations

import hashlib
import json
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
MANIFEST = ROOT / "performance" / "v1.0" / "MANIFEST.json"
PLAN = HERE / "VOLUME_PLAN.json"
BUDGET = 650 * 1024 * 1024  # evidence/verify_evidence.py CD_BUDGET_BYTES
# What xorriso adds around the payload: volume-0002 printed 680,937 bytes over
# its 567,956,311-byte payload with these same materials. Ten times that is the
# margin a volume must keep, so the plan never cuts closer than the build does.
ISO_MARGIN = 8 * 1024 * 1024
FIRST_VOLUME = 5
PREDECESSOR = "NTR-EV-0004"
CHECKPOINT_DATE = "20261022"


class PlanError(AssertionError):
    pass


def require(condition, message):
    if not condition:
        raise PlanError(message)


def item_number(item: str) -> int:
    return int(re.fullmatch(r"J(\d+)", item).group(1))


def split(archives: list[dict]) -> list[list[dict]]:
    """The fewest volumes in item order, cut where the largest is smallest."""
    room = BUDGET - ISO_MARGIN
    require(all(a["bytes"] <= room for a in archives), "an archive is larger than a volume")
    total = sum(a["bytes"] for a in archives)
    require(total <= 2 * room, "the archives need more than two volumes; the split is two-way")
    if total <= room:
        return [archives]
    best = None
    for cut in range(1, len(archives)):
        first = sum(a["bytes"] for a in archives[:cut])
        larger = max(first, total - first)
        if larger <= room and (best is None or larger < best[0]):
            best = (larger, cut)
    require(best is not None, "no cut in item order fits both volumes")
    return [archives[:best[1]], archives[best[1]:]]


def derive() -> dict:
    manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
    archives = sorted(manifest["archives"], key=lambda a: item_number(a["item"]))
    volumes = []
    predecessor = PREDECESSOR
    for index, group in enumerate(split(archives)):
        number = FIRST_VOLUME + index
        volume_id = f"NTR-EV-{number:04d}"
        size = sum(a["bytes"] for a in group)
        volumes.append({
            "volume_id": volume_id,
            "checkpoint_id": f"v10-close-{CHECKPOINT_DATE}-{number:04d}",
            "materials": f"evidence/volumes/volume-{number:04d}",
            "predecessor_volume_id": predecessor,
            "items": [a["item"] for a in group],
            "to_seal": sorted((a["archive"] for a in group), key=lambda name: name.encode("utf-8")),
            "payload_bytes": size,
            "under_budget_bytes": BUDGET - size,
        })
        predecessor = volume_id
    return {
        "schema": "notrios.v10.j10-volume-plan.v1",
        "what_this_is": ("the volumes that seal v1.0's evidence before the release, derived from "
                         "performance/v1.0/MANIFEST.json. A plan, not a seal: what a sealed volume "
                         "holds is read from its committed signatures."),
        "manifest_sha256": hashlib.sha256(MANIFEST.read_bytes()).hexdigest(),
        "budget_bytes": BUDGET,
        "iso_margin_bytes": ISO_MARGIN,
        "candidates": len(archives),
        "candidate_bytes": sum(a["bytes"] for a in archives),
        "not_a_candidate": {
            "J10": ("this item's own archive is taken when J10 closes, after the seal and the "
                    "release; a volume cannot hold an archive of the commit that records it"),
            "superseded v0.8 archives": ("the six volume-0002 left out stay out, for the reason "
                                         "performance/v0.8e/SEAL_RESULT.json records"),
        },
        "volumes": volumes,
    }


def sealed_names(materials: Path) -> set[str]:
    return {path.name[:-len(".sig")] for path in (materials / "SIGNATURES").glob("*.sig")}


def check(plan: dict) -> list[str]:
    require(json.loads(PLAN.read_text(encoding="utf-8")) == plan,
            "VOLUME_PLAN.json is not what the manifest derives; "
            "run python3 performance/v1.0-j10/plan_volumes.py --write")
    catalog = [json.loads(line)["payload"] for line in
               (ROOT / "evidence" / "outer-iso-catalog.jsonl").read_text(encoding="utf-8").splitlines()
               if line.strip()]
    catalogued = {entry["volume_id"]: entry for entry in catalog}
    states = []
    previous_sealed = True
    for volume in plan["volumes"]:
        materials = ROOT / volume["materials"]
        checkpoint_path = materials / "CHECKPOINT" / "content-checkpoint.json"
        if not checkpoint_path.is_file():
            require(volume["volume_id"] not in catalogued,
                    f"{volume['volume_id']} is catalogued but its materials are not committed")
            states.append(f"{volume['volume_id']} planned")
            previous_sealed = False
            continue
        require(previous_sealed, f"{volume['volume_id']} is sealed before the volume it chains onto")
        checkpoint = json.loads(checkpoint_path.read_text(encoding="utf-8"))
        require(checkpoint["checkpoint_id"] == volume["checkpoint_id"],
                f"{volume['volume_id']}: sealed under another checkpoint id")
        require(checkpoint["predecessor_volume_id"] == volume["predecessor_volume_id"],
                f"{volume['volume_id']}: chained onto {checkpoint['predecessor_volume_id']}, "
                f"not {volume['predecessor_volume_id']}")
        sealed = sealed_names(materials)
        require(sealed == set(volume["to_seal"]),
                f"{volume['volume_id']}: sealed set differs from the plan: "
                f"missing {sorted(set(volume['to_seal']) - sealed)[:3]}, "
                f"extra {sorted(sealed - set(volume['to_seal']))[:3]}")
        require(checkpoint["source_artifact_count"] == len(sealed),
                f"{volume['volume_id']}: the checkpoint counts another number of artifacts")
        entry = catalogued.get(volume["volume_id"])
        require(entry is not None, f"{volume['volume_id']} is sealed but not in the catalog")
        require(entry["content_checkpoint_id"] == volume["checkpoint_id"],
                f"{volume['volume_id']}: the catalog names another checkpoint")
        require(entry["size_bytes"] < BUDGET, f"{volume['volume_id']}: the ISO is over budget")
        states.append(f"{volume['volume_id']} sealed ({len(sealed)} artifacts)")
    return states


def main(argv: list[str]) -> int:
    plan = derive()
    if argv[:1] == ["--write"]:
        PLAN.write_text(json.dumps(plan, indent=1, sort_keys=True) + "\n", encoding="utf-8")
        print(f"wrote {PLAN.relative_to(ROOT)}")
        return 0
    if argv[:1] == ["--names"]:
        volume = next((v for v in plan["volumes"] if v["volume_id"] == argv[1]), None)
        require(volume is not None, f"no volume {argv[1]} in the plan")
        Path(argv[2]).write_text(json.dumps({"to_seal": volume["to_seal"]}, indent=1) + "\n",
                                 encoding="utf-8")
        return 0
    states = check(plan)
    print(f"v1.0 volume plan valid: {plan['candidates']} archives, "
          f"{plan['candidate_bytes']:,} bytes, " + "; ".join(states))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except (PlanError, KeyError, ValueError) as error:
        print(f"v1.0 volume plan invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
