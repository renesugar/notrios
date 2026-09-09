#!/usr/bin/env python3
"""Unit tests for deterministic G18d evidence validation helpers."""

import importlib.util
from pathlib import Path
import unittest


PATH = Path(__file__).with_name("validate_evidence.py")
SPEC = importlib.util.spec_from_file_location("g18d_validate", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class EvidenceHelpersTest(unittest.TestCase):
    def test_without_runtime_preserves_input(self):
        report = {"runtime_ms": 9, "topics": [{"runtime_ms": 3, "counts": {}}]}
        normalized = MODULE.without_runtime(report)
        self.assertEqual(normalized["runtime_ms"], 0)
        self.assertEqual(normalized["topics"][0]["runtime_ms"], 0)
        self.assertEqual(report["runtime_ms"], 9)


if __name__ == "__main__":
    unittest.main()
