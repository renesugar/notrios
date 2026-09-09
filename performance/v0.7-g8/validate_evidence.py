#!/usr/bin/env python3
"""Check that the committed G8 evidence says what the documentation claims."""
from __future__ import annotations

import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parent
MEBIBYTE = 1 << 20


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"g8 evidence: {message}")


def main() -> None:
    report = json.loads((ROOT / "materialization-results.json").read_text(encoding="utf-8"))
    findings = (ROOT / "FINDINGS.md").read_text(encoding="utf-8")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")

    require(report["schema"] == "notrios.g8.materialization.v1", "wrong report schema")
    require(report["database_schema_version"] == 23, "report was not produced against schema v23")

    bounds = report["bounds"]
    require(bounds["whole_object_threshold"] == MEBIBYTE, "the whole-object threshold is not G2's 1 MiB")
    require(bounds["chunk_bytes"] == MEBIBYTE, "the chunk size is not G2's 1 MiB")
    require(bounds["max_chunks"] * bounds["chunk_bytes"] == bounds["max_object_bytes"],
            "the chunk ceiling and the object ceiling disagree")

    sizes = report["sizes"]
    require(len(sizes) == 4, "expected four object sizes")
    for row in sizes:
        label = row["label"]
        require(row["note_readable_before_bytes"],
                f"{label}: the note was not readable before its attachment arrived")
        require(row["exact_reconstruction"], f"{label}: an object did not reconstruct exactly")
        require(row["materialized"] == row["attachments"],
                f"{label}: {row['materialized']} of {row['attachments']} attachments materialized")
        require(row["fetched_bytes"] == row["object_bytes"] * row["attachments"],
                f"{label}: fetched bytes do not equal the objects fetched")

        expected_chunks = 1 if row["object_bytes"] <= MEBIBYTE else -(-row["object_bytes"] // MEBIBYTE)
        require(row["chunks"] == expected_chunks,
                f"{label}: {row['chunks']} chunks for {row['object_bytes']} bytes")

        # The policy split must land exactly on the threshold, in both
        # directions: this is the claim, not an observation about this host.
        if row["object_bytes"] <= MEBIBYTE:
            require(row["auto_requested"] == row["attachments"] and row["auto_reason"] == "eager_small_object",
                    f"{label}: a small attachment was not fetched automatically ({row['auto_reason']})")
        else:
            require(row["auto_requested"] == 0 and row["auto_reason"] == "above_eager_threshold",
                    f"{label}: a large attachment was fetched unasked ({row['auto_reason']})")

        # A resumed transfer must ask for strictly less than the whole object
        # whenever the object has more than one segment to resume from.
        require(row["resume_chunks_total"] == expected_chunks, f"{label}: resume total disagrees with the plan")
        if expected_chunks > 1:
            require(0 < row["resume_chunks_refetched"] < expected_chunks,
                    f"{label}: resume refetched {row['resume_chunks_refetched']} of {expected_chunks}")
        else:
            require(row["resume_chunks_refetched"] == 0,
                    f"{label}: a whole object completed in the interrupted pass should refetch nothing")

    for row in sizes:
        for value in (row["fetched_bytes"], row["metadata_only_ms"], row["materialize_ms"]):
            require(f"{value:,}" in findings or str(value) in findings,
                    f"{row['label']}: FINDINGS.md does not mention {value}")

    require(re.search(r"desktop", readme, re.IGNORECASE) and re.search(r"in-process", findings, re.IGNORECASE),
            "the desktop and in-process-provider scope is not stated in both documents")
    require("archive" in findings.lower(), "FINDINGS.md does not record the export limitation")

    print(f"g8 evidence: {len(sizes)} object sizes validated")


if __name__ == "__main__":
    main()
