#!/usr/bin/env python3
"""Apply the J32 rule to a measurement: did the candidate improve, beyond noise?

    python3 performance/v1.0-j32/compare.py <bench.json> <baseline-label> <candidate-label> \
        <declared-metric> [<bench.json> ...]

The rule is PLAN.md's, fixed before any candidate was measured:

- improved: the candidate's median beats the baseline's median, on the declared
  metric, by more than the baseline's own range (maximum minus minimum);
- no regression: on every other metric the candidate's median is not worse
  than the baseline's by more than the baseline's range.

Lower is better for every metric except notes_per_second. A metric with zero
range (a count, such as prepared_statements) improves on any decrease, because
a count has no noise.

Extra bench files are further cases of the same comparison (the ordinary
corpus, where a scaling change must be neutral): the declared metric must not
regress there either. The verdict is printed and the exit status is 0 only
when the candidate improved and regressed nowhere.
"""
from __future__ import annotations

import json
import pathlib
import sys

HIGHER_IS_BETTER = {"notes_per_second"}


def verdicts(record: dict, baseline: str, candidate: str) -> dict[str, dict]:
    base = record["results"][baseline]["summary"]
    cand = record["results"][candidate]["summary"]
    out = {}
    for metric in base:
        if metric not in cand:
            continue
        b, c = base[metric], cand[metric]
        noise = b["range"]
        delta = (b["median"] - c["median"]) if metric not in HIGHER_IS_BETTER else (c["median"] - b["median"])
        # delta > 0 means the candidate is better.
        if delta > noise:
            state = "better"
        elif -delta > noise:
            state = "worse"
        else:
            state = "within noise"
        out[metric] = {"baseline_median": b["median"], "candidate_median": c["median"],
                       "baseline_range": noise,
                       "change_percent": round(100 * (c["median"] - b["median"]) / b["median"], 2)
                       if b["median"] else None,
                       "state": state}
    return out


def main() -> int:
    if len(sys.argv) < 5:
        print(__doc__, file=sys.stderr)
        return 2
    first, baseline, candidate, declared, *others = sys.argv[1:]
    ok = True
    for index, path in enumerate([first, *others]):
        record = json.loads(pathlib.Path(path).read_text(encoding="utf-8"))
        rows = verdicts(record, baseline, candidate)
        if declared not in rows:
            print(f"{path}: no metric {declared!r}", file=sys.stderr)
            return 2
        print(f"== {record['case']} ({path})")
        for metric, row in rows.items():
            marker = "*" if metric == declared else " "
            print(f" {marker} {metric:22} {row['baseline_median']:>16.6g} -> {row['candidate_median']:<16.6g}"
                  f" range {row['baseline_range']:<12.6g} {row['change_percent']!s:>8}%  {row['state']}")
        if index == 0 and rows[declared]["state"] != "better":
            ok = False
            print(f"   the declared metric {declared} did not improve beyond noise")
        worse = [metric for metric, row in rows.items() if row["state"] == "worse"]
        if worse:
            ok = False
            print(f"   worse beyond noise: {', '.join(worse)}")
    print("verdict: improved, no regression" if ok else "verdict: not shown to improve")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
