#!/usr/bin/env python3
"""Tests for the post-purge check (v1.0 J11).

The script itself is shell, because at the moment it runs nothing of Notrios is
left to read a manifest. These tests are Python only because that is what runs
them here; each one drives the real script over a real directory tree.
"""
from __future__ import annotations

import pathlib
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
CHECK = ROOT / "scripts/check_purged.sh"


def run(manifest: pathlib.Path | str) -> subprocess.CompletedProcess:
    return subprocess.run(["bash", str(CHECK), str(manifest)], capture_output=True, text=True)


class CheckPurgedTest(unittest.TestCase):
    def manifest(self, directory: str, lines: list[str]) -> pathlib.Path:
        path = pathlib.Path(directory) / "manifest.txt"
        path.write_text("".join(line + "\n" for line in lines), encoding="utf-8")
        return path

    def test_a_complete_purge_passes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            kept = root / "nas/notes.sqlite"
            kept.parent.mkdir(parents=True)
            kept.write_text("library", encoding="utf-8")
            manifest = self.manifest(directory, [
                f"owned\t{root}/config/notrios",
                f"owned\t{root}/config/notrios/config.yaml",
                f"external\t{kept}",
            ])
            result = run(manifest)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("2 owned path(s) gone, 1 external path(s) still present", result.stdout)

    def test_one_file_left_behind_fails_and_names_it(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            left = root / "state/notrios/quarantine/leftover"
            left.parent.mkdir(parents=True)
            left.write_text("still here", encoding="utf-8")
            manifest = self.manifest(directory, [
                f"owned\t{root}/config/notrios",
                f"owned\t{left}",
            ])
            result = run(manifest)
            self.assertEqual(result.returncode, 1)
            self.assertIn("left 1 of 2", result.stderr)
            self.assertIn(str(left), result.stderr)

    def test_a_dangling_symlink_counts_as_left_behind(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            link = root / "dangling"
            link.symlink_to(root / "gone")
            result = run(self.manifest(directory, [f"owned\t{link}"]))
            self.assertEqual(result.returncode, 1)
            self.assertIn(str(link), result.stderr)

    def test_deleting_a_library_a_purge_keeps_is_reported_separately(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            result = run(self.manifest(directory, [
                f"owned\t{root}/config/notrios",
                f"external\t{root}/nas/notes.sqlite",
            ]))
            self.assertEqual(result.returncode, 1)
            self.assertIn("a library outside the roots was deleted", result.stderr)
            self.assertNotIn("left", result.stderr.split("keeps are gone")[0])

    def test_a_manifest_that_is_not_one_is_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            result = run(self.manifest(directory, ["/just/a/path", "another"]))
            self.assertEqual(result.returncode, 2)
            self.assertIn("neither owned nor external", result.stderr)

    def test_an_empty_manifest_proves_nothing(self):
        with tempfile.TemporaryDirectory() as directory:
            result = run(self.manifest(directory, []))
            self.assertEqual(result.returncode, 2)
            self.assertIn("proves nothing", result.stderr)

    def test_a_missing_manifest_says_what_to_pass(self):
        result = run("/nonexistent/manifest.txt")
        self.assertEqual(result.returncode, 2)
        self.assertIn("taken before the purge", result.stderr)

    def test_a_path_with_spaces_is_handled(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            spaced = root / "my notes/notrios/config.yaml"
            spaced.parent.mkdir(parents=True)
            spaced.write_text("x", encoding="utf-8")
            result = run(self.manifest(directory, [f"owned\t{spaced}"]))
            self.assertEqual(result.returncode, 1)
            self.assertIn(str(spaced), result.stderr)


if __name__ == "__main__":
    unittest.main()
