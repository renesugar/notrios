#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch


PATH = Path(__file__).with_name("harness.py")
SPEC = importlib.util.spec_from_file_location("g14e_harness", PATH)
assert SPEC and SPEC.loader
H = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(H)


class HarnessTests(unittest.TestCase):
    def test_phase_matrix_has_no_duplicates(self) -> None:
        self.assertEqual(len(H.PHASES), len(set(H.PHASES)))
        self.assertIn("catchup", H.PHASES)
        self.assertEqual(sum(name.endswith("-check") for name in H.PHASES), 4)

    def test_atomic_json_refuses_immutable_replacement(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "result.json"
            H.atomic_json(path, {"one": 1})
            with self.assertRaises(H.HarnessError):
                H.atomic_json(path, {"two": 2})
            self.assertEqual(json.loads(path.read_text()), {"one": 1})

    def test_private_hash_pattern(self) -> None:
        self.assertIsNotNone(H.PRIVATE_HASH.search("a" * 64))
        self.assertIsNone(H.PRIVATE_HASH.search("a" * 63))

    def test_receiver_limits_are_stricter(self) -> None:
        self.assertLess(H.MAX_RECEIVER_RSS, H.MAX_SENDER_RSS)
        self.assertEqual(H.MAX_STAGE_SECONDS, 7200)

    def test_empty_external_pack_set_normalizes_to_a_list(self) -> None:
        manifest = {"external": {"packs": None}}
        self.assertEqual(manifest["external"].get("packs") or [], [])

    def test_repository_tools_are_not_json_producers(self) -> None:
        args = SimpleNamespace(g14b_workspace=Path("/private/g14b"))
        with patch.object(H, "run_command") as run:
            got = H.repository_phase(args, "restic-canonical-check", Path("phase.log"))
        self.assertTrue(got["assertions"]["repository_integrity_verified"])
        self.assertFalse(run.call_args.kwargs["parse_json"])

    def test_public_corpus_summary_removes_private_workload_labels(self) -> None:
        corpus = {
            "counts": {
                "recipe_joplin_documents": 2, "recipe_obsidian_documents": 2,
                "attachment_documents": 1, "attachment_resources": 1,
                "attachment_blobs": 1, "attachment_source_bundle_items": 1,
            },
            "assertions": {"equal": True},
            "frozen_import_seconds": {"recipe_joplin": 1, "recipe_obsidian": 2, "attachments": 3},
        }
        serialized = json.dumps(H.public_corpus_summary(corpus), sort_keys=True)
        self.assertNotIn("recipe_joplin", serialized)
        self.assertNotIn("recipe_obsidian", serialized)
        row_counts = H.public_phase_counts({"phase": "corpus-equivalence", **corpus})
        self.assertNotIn("recipe_joplin", json.dumps(row_counts, sort_keys=True))


if __name__ == "__main__":
    unittest.main()
