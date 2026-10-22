#!/usr/bin/env python3
"""Check J18's record, and stop the defect it removed from coming back.

The measurements cannot be re-derived in a validate run: they need three
libraries that live outside the repository, one of them 7.3 GB, and the largest
takes four minutes just to migrate. So what is checked here is the record's
internal consistency and -- the part that matters for the future -- the state of
the source it describes.

The defect was one SQL statement repeated at ten call sites. Nothing stops an
eleventh being written except a check that looks.
"""
from __future__ import annotations

import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class EvidenceError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise EvidenceError(message)


def main() -> None:
    report = json.loads((HERE / "REPORT.json").read_text(encoding="utf-8"))
    require(report["schema"] == "notrios.v10.j18-fts-rowid.v1", "wrong schema")

    # The defect must not exist anywhere in the store. This is the whole reason
    # this validator is in `make validate`: an eleventh call site written next
    # year would reintroduce an O(library size) write silently, and nothing else
    # in the repository would notice.
    offenders = []
    for path in sorted((ROOT / "internal/store").glob("*.go")):
        if path.name.endswith("_test.go") or path.name == "fts_rowid.go":
            continue
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if re.search(r"documents_fts\s+WHERE\s+document_id", line):
                offenders.append(f"{path.relative_to(ROOT)}:{number}")
    require(not offenders,
            "the full-text index is addressed by document_id again, which FTS5 cannot index, so "
            "these writes scan the whole index: " + ", ".join(offenders))

    # The mapping has to be created for a fresh database as well as migrated
    # into an existing one, or a new library has no table to record into.
    migration = ROOT / "internal/store/migrations/0028_fts_rowid_map.sql"
    require(migration.is_file(), "migration 0028 is missing")
    text = migration.read_text(encoding="utf-8")
    require("CREATE TABLE IF NOT EXISTS documents_fts_rowid" in text,
            "migration 0028 no longer creates the mapping")
    require("SELECT document_id, rowid FROM documents_fts" in text,
            "migration 0028 no longer backfills the mapping for an existing library")
    require("PRAGMA user_version = 28" in text, "migration 0028 does not record its version")
    require("ensureSchemaV28" in (ROOT / "internal/store/sqlite.go").read_text(encoding="utf-8"),
            "applySchema no longer runs the v28 step, so a fresh database has no mapping")

    # A peer at the previous schema must still be admissible: the mapping is a
    # local index, so widening the range was the claim, and a range that moved
    # instead would have broken sync for a reason that carries no wire change.
    state = (ROOT / "internal/syncstate/state.go").read_text(encoding="utf-8")
    require(re.search(r"MinCompatibleSchema\s*=\s*24", state),
            "the minimum compatible schema moved; J18 widened the range rather than moving it")
    require(re.search(r"MaxCompatibleSchema\s*=\s*28", state),
            "the maximum compatible schema no longer admits v28")

    # The record's own numbers must say what the item claims: the write stopped
    # scaling with the library.
    measurements = {m["notes"]: m for m in report["measurements"]}
    require(set(measurements) == {60, 103349, 382206}, "the three measured libraries changed")
    largest, middle = measurements[382206], measurements[103349]
    require(largest["write_ms_after"] < middle["write_ms_before"] / 10,
            "the largest library's write is no longer an order of magnitude below what the "
            "middle one used to cost, so the record no longer shows what it claims")
    require(largest["write_ms_after"] <= middle["write_ms_after"] * 2,
            "the write cost still scales with the library, which was the defect")

    for name in report["correctness"]["tests"]:
        found = any(f"func {name}(" in p.read_text(encoding="utf-8")
                    for p in (ROOT / "internal/store").glob("*_test.go"))
        require(found, f"{name} is named as the correctness evidence and does not exist")

    print(f"J18 record valid: {largest['write_ms_before']} ms -> {largest['write_ms_after']} ms "
          f"at {382206:,} notes, no call site addresses the index by document_id, "
          f"schema range admits v{28}")


if __name__ == "__main__":
    try:
        main()
    except EvidenceError as error:
        print(f"J18 record invalid: {error}", file=sys.stderr)
        raise SystemExit(1)
