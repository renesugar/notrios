#!/usr/bin/env python3
"""Print a Markdown table of J32 bench results for one binary.

    python3 performance/v1.0-j32/summarize.py <label> <bench.json> [...]

One row per case: the median of each headline metric with its range (maximum
minus minimum across the measured runs), which is the noise a candidate has to
beat under PLAN.md's rule.
"""
from __future__ import annotations

import json
import pathlib
import sys

COLUMNS = (
    ("wall_seconds", "wall s", 1, 1),
    ("notes_per_second", "notes/s", 1, 1),
    ("peak_rss_kib", "peak RSS MiB", 1 / 1024, 1),
    ("total_alloc_bytes", "allocated MiB", 1 / (1 << 20), 1),
    ("mallocs", "allocations k", 1 / 1000, 1),
    ("prepared_statements", "statements", 1, 0),
)


def cell(summary: dict, metric: str, scale: float, digits: int) -> str:
    if metric not in summary:
        return "—"
    row = summary[metric]
    if row["median"] * scale < 10 and digits:
        digits = 3  # a rate below ten a second would otherwise round to nothing
    return f"{row['median'] * scale:,.{digits}f} ± {row['range'] * scale:,.{digits}f}"


def main() -> int:
    if len(sys.argv) < 3:
        print(__doc__, file=sys.stderr)
        return 2
    label, paths = sys.argv[1], sys.argv[2:]
    print("| case | notes | " + " | ".join(title for _, title, _, _ in COLUMNS) + " |")
    print("|---|---:|" + "---:|" * len(COLUMNS))
    for path in paths:
        record = json.loads(pathlib.Path(path).read_text(encoding="utf-8"))
        result = record["results"][label]
        notes = result["runs"][0]["notes_seen"]
        print(f"| {record['case']} | {notes:,} | "
              + " | ".join(cell(result["summary"], metric, scale, digits)
                           for metric, _, scale, digits in COLUMNS) + " |")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
