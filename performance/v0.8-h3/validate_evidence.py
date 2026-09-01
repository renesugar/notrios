#!/usr/bin/env python3
"""Validate the v0.8 H3 installed-path investigation evidence.

Offline and self-contained. It re-checks three things a document cannot check
about itself:

1. every consumer anchor still occurs exactly once in the source it names, so
   the inventory describes code that exists;
2. every consumer's target root, owning change and defect id is declared
   somewhere else in the evidence, so the inventory and the layout contract
   cannot drift apart;
3. the generated fixture and resolution reports record no failures.

It does not re-run the Go probe: `go test ./performance/v0.8-h3/pathprobe/`
does that, and duplicating it here would only produce a second place to
maintain the same claim.
"""

from __future__ import annotations

import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))

# Owning changes named in REPORT.md section 8. A consumer may also carry an
# "exception:<reason>" owner, which must be argued in the report rather than
# listed here.
KNOWN_CHANGES = {"H4-C1", "H4-C2", "H4-D1", "H4-D2", "H4-S1", "H4-K1", "H4-R1", "H4-A1", "H4-X1"}

KNOWN_MIGRATIONS = {"move", "rebuild", "regenerate", "none"}

# Every defect id must be explained in the report. The map says where.
DEFECT_EXPLANATIONS = {
    "relative-xdg-accepted": "1.1",
    "cwd-relative-fallback": "1.1",
    "hardcoded-dot-config-on-all-platforms": "1.1",
    "second-config-root-resolver": "1.1",
    "cwd-relative-config-load": "1.3",
    "cwd-relative-default": "1.3",
    "cwd-precedes-executable": "1.3",
    "world-readable-root": "1.4",
    "shared-predictable-temp-path": "1.5",
    "multi-instance-collision": "1.5",
    "posix-only-separator": "1.5",
    "ignores-xdg-data-home": "2",
    "regenerated-not-user-authored": "4",
}


class EvidenceError(ValueError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise EvidenceError(message)


def load(name: str):
    with open(os.path.join(HERE, name), encoding="utf-8") as stream:
        return json.load(stream)


def validate() -> dict:
    consumers_doc = load("PATH_CONSUMERS.json")
    layout = load("LAYOUT.json")
    fixtures = load("PURGE_ORACLE_FIXTURES.json")
    resolution = load("RESOLUTION_TABLE.json")
    report = open(os.path.join(HERE, "REPORT.md"), encoding="utf-8").read()

    require(consumers_doc["schema"] == "notrios.path-consumers/1", "unexpected consumer schema")
    require(layout["schema"] == "notrios.installed-layout/1", "unexpected layout schema")
    require(layout["status"] == "proposed",
            "H3 is an investigation; the layout must remain proposed, not applied")

    consumers = consumers_doc["consumers"]
    require(len(consumers) >= 20, f"only {len(consumers)} consumers inventoried")

    # 1. Anchors still resolve, exactly once each.
    seen_ids = set()
    for consumer in consumers:
        cid = consumer["id"]
        require(cid not in seen_ids, f"duplicate consumer id {cid}")
        seen_ids.add(cid)

        path = os.path.join(REPO, consumer["file"])
        require(os.path.exists(path), f"{cid}: {consumer['file']} does not exist")
        text = open(path, encoding="utf-8").read()
        count = text.count(consumer["anchor"])
        require(count == 1,
                f"{cid}: anchor occurs {count} times in {consumer['file']}; "
                "the consumer has changed and the inventory must be revisited")

    # 2. Categories, owners, migrations and defects are all declared.
    # "external" is a category but deliberately not a root: it names territory
    # shared with other applications. It is declared separately so the
    # owner-only check below cannot be weakened to accommodate it.
    declared_roots = set(layout["roots"])
    declared_categories = declared_roots | set(layout["external_categories"])
    for consumer in consumers:
        cid = consumer["id"]
        require(consumer["category"] in declared_categories,
                f"{cid}: category {consumer['category']!r} is not a declared root or external category")
        require(consumer["migration"] in KNOWN_MIGRATIONS,
                f"{cid}: unknown migration {consumer['migration']!r}")

        owner = consumer["owner"]
        if owner.startswith("exception:"):
            require("exception" in report.lower(),
                    f"{cid}: claims an exception the report does not argue")
        else:
            require(owner in KNOWN_CHANGES, f"{cid}: unknown owning change {owner!r}")
            require(owner in report, f"{cid}: owning change {owner} is not named in REPORT.md")

        for defect in consumer["defects"]:
            require(defect in DEFECT_EXPLANATIONS,
                    f"{cid}: defect {defect!r} is not explained anywhere in the report")

    # Every declared change must actually own something. A change nobody needs
    # is a change H4 would implement for no reason.
    owned = {c["owner"] for c in consumers}
    for change in KNOWN_CHANGES:
        require(change in owned, f"{change} is declared but owns no consumer")

    # 3. Every mutable root is owner-only, and the backup policy is complete.
    for name, root in layout["roots"].items():
        for platform in ("linux", "windows", "macos"):
            require(platform in root, f"root {name} has no {platform} entry")
        if name != "program_assets":
            require(root["mode"] == "0700",
                    f"root {name} is created {root['mode']}, not owner-only")
        require("backup_policy" in root, f"root {name} declares no backup policy")

    # The purge oracle's policy table must cover every declared root, or purge
    # would meet a category it has no rule for.
    sys.path.insert(0, HERE)
    import purge_oracle  # noqa: E402

    for name, root in {**layout["roots"], **layout["external_categories"]}.items():
        policy = purge_oracle.backup_policy(name)
        require(policy == root["backup_policy"],
                f"root {name}: layout says {root['backup_policy']}, oracle says {policy}")
    require(purge_oracle.backup_policy("an-unclassified-category") == "backup_and_verify",
            "an unclassified category must default to being backed up")

    # 4. The generated reports record no failures.
    require(fixtures["failed"] == 0, f"{fixtures['failed']} purge-oracle fixtures failed")
    require(fixtures["passed"] == fixtures["total"],
            "purge-oracle pass count does not match the case count")
    require(fixtures["total"] >= 25, f"only {fixtures['total']} purge-oracle cases")
    require(resolution["failed"] == 0, f"{resolution['failed']} resolution scenarios failed")
    require(resolution["total"] >= 12, f"only {resolution['total']} resolution scenarios")

    # 5. The backup restore proof must exist and be cited. The plan asks for a
    # restore proof, and a specified container format is not one.
    proof = os.path.join(HERE, "test_backup_restore.py")
    require(os.path.exists(proof), "the backup restore proof is missing")
    require("test_backup_restore.py" in report,
            "REPORT.md does not reference the backup restore proof")

    # 6. The investigation boundary: H3 must not have changed a default.
    defaults = open(os.path.join(REPO, "internal/config/config.go"), encoding="utf-8").read()
    require('Directory:     "./data",' in defaults,
            "the ./data default has changed; H3 is investigation-only and H4 owns that change")

    # 7. Claims marked observed must correspond to a probe test that exists.
    probe_dir = os.path.join(HERE, "pathprobe")
    probe_source = "".join(
        open(os.path.join(probe_dir, name), encoding="utf-8").read()
        for name in sorted(os.listdir(probe_dir)) if name.endswith("_test.go")
    )
    for named in re.findall(r"`(Test[A-Za-z0-9_]+)`", report):
        require(f"func {named}(" in probe_source,
                f"REPORT.md cites {named}, which does not exist in pathprobe/")

    return {
        "consumers": len(consumers),
        "roots": len(layout["roots"]),
        "purge_fixtures": fixtures["total"],
        "resolution_scenarios": resolution["total"],
        "owning_changes": len(KNOWN_CHANGES),
    }


def main() -> int:
    try:
        summary = validate()
    except EvidenceError as err:
        print(f"H3 evidence invalid: {err}", file=sys.stderr)
        return 1
    print("H3 evidence valid: " + ", ".join(f"{k}={v}" for k, v in summary.items()))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
