#!/usr/bin/env python3
"""The J32 rule, held to the cases PLAN.md fixes before any candidate is measured."""
from __future__ import annotations

import pathlib
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import compare  # noqa: E402


def record(baseline: dict[str, list[float]], candidate: dict[str, list[float]]) -> dict:
    def summary(values: dict[str, list[float]]) -> dict:
        return {metric: {"values": v, "median": sorted(v)[len(v) // 2], "min": min(v), "max": max(v),
                         "range": max(v) - min(v)} for metric, v in values.items()}
    return {"case": "t", "results": {"base": {"summary": summary(baseline)},
                                     "cand": {"summary": summary(candidate)}}}


class RuleTest(unittest.TestCase):
    def state(self, metric, baseline, candidate):
        return compare.verdicts(record({metric: baseline}, {metric: candidate}), "base", "cand")[metric]["state"]

    def test_a_win_inside_the_spread_is_not_an_improvement(self):
        # 1% median win inside 5% run-to-run spread.
        self.assertEqual(self.state("wall_seconds", [100, 98, 103, 100, 101], [99, 99, 99, 99, 99]),
                         "within noise")

    def test_a_win_beyond_the_range_is_better(self):
        self.assertEqual(self.state("wall_seconds", [100, 98, 103, 100, 101], [90, 91, 90, 89, 90]), "better")

    def test_a_loss_beyond_the_range_is_worse(self):
        self.assertEqual(self.state("peak_rss_kib", [100, 101, 100, 99, 100], [110, 110, 111, 110, 109]),
                         "worse")

    def test_throughput_is_better_when_higher(self):
        self.assertEqual(self.state("notes_per_second", [100, 101, 100, 99, 100], [120] * 5), "better")
        self.assertEqual(self.state("notes_per_second", [100, 101, 100, 99, 100], [80] * 5), "worse")

    def test_a_count_has_no_noise(self):
        self.assertEqual(self.state("prepared_statements", [500] * 5, [499] * 5), "better")
        self.assertEqual(self.state("prepared_statements", [500] * 5, [500] * 5), "within noise")


if __name__ == "__main__":
    unittest.main()
