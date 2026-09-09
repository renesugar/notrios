#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import subprocess
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
    def test_usage_preflight_uses_shared_wrapper_and_propagates_pause(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            completed = subprocess.CompletedProcess([], 0)
            with patch.object(H.subprocess, "run", return_value=completed) as probe:
                H.usage_preflight(Path(temporary), "g14e:p")
            command = probe.call_args.args[0]
            self.assertEqual(command[0], "bash")
            self.assertTrue(command[1].endswith("scripts/agent_usage_preflight.sh"))
            self.assertEqual(command[2], "g14e:p")
            with patch.object(
                H.subprocess,
                "run",
                return_value=subprocess.CompletedProcess([], 2),
            ):
                with self.assertRaises(H.HarnessError):
                    H.usage_preflight(Path(temporary), "g14e:q")

    def _args(self, workspace: Path) -> SimpleNamespace:
        return SimpleNamespace(workspace=workspace, root=Path("/repo"))

    def test_completed_result_skips_usage_guard(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            workspace = Path(temporary)
            H.phase_result_path(workspace, "corpus-equivalence").parent.mkdir(parents=True)
            H.phase_result_path(workspace, "corpus-equivalence").write_text(json.dumps({"status": "completed"}))
            with patch.object(H, "usage_preflight") as guard:
                result = H.execute_phase(self._args(workspace), "corpus-equivalence")
            self.assertEqual(result["status"], "completed")
            guard.assert_not_called()

    def test_usage_pause_prevents_checkpoint_and_work(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            workspace = Path(temporary)
            args = self._args(workspace)
            with patch.object(H, "usage_preflight", side_effect=H.HarnessError("pause")), \
                    patch.object(H, "corpus_equivalence", side_effect=AssertionError("work ran")):
                with self.assertRaises(H.HarnessError):
                    H.execute_phase(args, "corpus-equivalence")
            self.assertFalse((workspace / "checkpoints/corpus-equivalence.json").exists())

    def test_usage_active_allows_checkpoint_and_work(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            workspace = Path(temporary)
            args = self._args(workspace)
            payload = {"assertions": {"completed": True}}
            with patch.object(H, "usage_preflight") as guard, patch.object(H, "corpus_equivalence", return_value=payload):
                result = H.execute_phase(args, "corpus-equivalence")
            self.assertEqual(result["status"], "completed")
            self.assertEqual(json.loads((workspace / "checkpoints/corpus-equivalence.json").read_text())["status"], "completed")
            guard.assert_called_once()

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
