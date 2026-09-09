#!/usr/bin/env python3
from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import corpus_profile
import workload


ROOT = Path(__file__).resolve().parent


class WorkloadTests(unittest.TestCase):
    def test_every_delta_reconstructs_exact_utf8(self) -> None:
        base = "# Café 東京 😀\n\nalpha beta gamma\n".encode()
        result = "# Cafè 京都 🧭\n\nalpha revised gamma\n".encode()
        for method in ("line", "word", "byte-span"):
            with self.subTest(method=method):
                delta = workload.make_delta(base, result, method)
                self.assertIsNotNone(delta)
                self.assertEqual(workload.apply_delta(base, delta), result)

    def test_manifest_merge_expectations_match_probe(self) -> None:
        manifest = json.loads((ROOT / "scenarios.json").read_text())
        for case in manifest["body_cases"]:
            base, left, right = workload.merge_fixture(case["id"])
            for unit, expected in case["expected"].items():
                with self.subTest(case=case["id"], unit=unit):
                    try:
                        _, conflicts = workload.merge3(base, left, right, unit)
                        actual = "conflict" if conflicts else "clean"
                    except ValueError:
                        actual = "limit-rejected"
                    self.assertEqual(actual, expected)

    def test_malformed_and_untrusted_patches_are_all_rejected(self) -> None:
        results = workload.rejection_results()
        self.assertEqual(len(results), 7)
        self.assertTrue(all(item["rejected"] for item in results))

    def test_corpus_profile_is_aggregate_only(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "note.md").write_text(
                "Private title\n\nPrivate body\n\nid: note1\ntype_: 1\n",
                encoding="utf-8",
            )
            (root / "blob.bin").write_bytes(b"private bytes")
            profile = corpus_profile.scan_joplin("fixture", root)
        rendered = json.dumps(profile, sort_keys=True)
        self.assertEqual(profile["body_sizes"]["count"], 1)
        self.assertEqual(profile["resource_sizes"]["count"], 1)
        self.assertNotIn("Private", rendered)
        self.assertNotIn(directory, rendered)
        self.assertNotIn("note.md", rendered)


if __name__ == "__main__":
    unittest.main()
