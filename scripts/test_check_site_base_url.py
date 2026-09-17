#!/usr/bin/env python3
"""Tests for the documentation address check (v1.0 J35).

Each test builds a small git repository with its own hugo.toml, because the
check reads the served address from the build's configuration and the repository
name from the origin remote. Nothing here reaches the network.
"""
from __future__ import annotations

import pathlib
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import check_site_base_url as check  # noqa: E402


def build_repository(directory: str, base_url: str, files: dict[str, str]) -> pathlib.Path:
    root = pathlib.Path(directory)
    (root / "docs-site/static").mkdir(parents=True, exist_ok=True)
    (root / "docs-site/hugo.toml").write_text(f'baseURL = "{base_url}"\n', encoding="utf-8")
    (root / "docs-site/static/CNAME").write_text("notrios.com\n", encoding="utf-8")
    for name, text in files.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
    for command in (["init", "-q"], ["remote", "add", "origin", "https://github.com/renesugar/notrios.git"],
                    ["add", "-A"]):
        subprocess.run(["git", "-C", str(root)] + command, check=True, capture_output=True)
    return root


class SiteBaseURLTest(unittest.TestCase):
    def findings(self, base_url: str, files: dict[str, str]) -> list[tuple]:
        with tempfile.TemporaryDirectory() as directory:
            root = build_repository(directory, base_url, files)
            _, findings = check.scan(root)
            return findings

    def test_a_document_naming_the_project_path_as_current_fails(self):
        findings = self.findings("https://notrios.com/", {
            "DOCS.md": "The build uses the GitHub Pages project base path (`/notrios/`).\n",
        })
        self.assertEqual([(name, number) for name, number, _, _ in findings], [("DOCS.md", 1)])
        self.assertIn("root-absolute", findings[0][3])

    def test_the_same_document_listed_as_history_passes(self):
        original = list(check.HISTORY)
        check.HISTORY.append(("DOCS.md", "/notrios/", "a record of the address before the move"))
        try:
            self.assertEqual(self.findings("https://notrios.com/", {
                "DOCS.md": "The build used the GitHub Pages project base path (`/notrios/`).\n",
            }), [])
        finally:
            check.HISTORY[:] = original

    def test_every_address_shaped_mention_is_found(self):
        findings = self.findings("https://notrios.com/", {
            "a.md": "Serve it at https://renesugar.github.io/notrios/ instead.\n",
            "b.mjs": "const base = 'http://127.0.0.1:18618/notrios/';\n",
            "c.md": 'A result href starts with "/notrios/".\n',
        })
        self.assertEqual(sorted(name for name, _, _, _ in findings), ["a.md", "b.mjs", "c.md"])

    def test_a_repository_name_in_a_filesystem_path_is_not_an_address(self):
        self.assertEqual(self.findings("https://notrios.com/", {
            "a.md": "The library lives in `~/.config/notrios/profiles.json`.\n",
            "b.md": "See `cmd/notrios/gui_wails.go` and /home/renes/evidence/notrios/x.zip.\n",
            "c.md": "Open http://127.0.0.1:8080/api/v1/status for the service.\n",
        }), [])

    def test_a_project_path_build_is_checked_the_other_way(self):
        """The mirror image: with a project-path baseURL, the custom domain is
        the error, and the project path is correct."""
        findings = self.findings("https://renesugar.github.io/notrios/", {
            "a.md": "The site is served at https://notrios.com/ today.\n",
            "b.md": "Assets resolve under `/notrios/`, which is correct here.\n",
        })
        self.assertEqual([name for name, _, _, _ in findings], ["a.md"])
        self.assertIn("project path", findings[0][3])

    def test_the_repository_itself_agrees_with_its_built_address(self):
        """The check passes on this repository, which is what it is for."""
        root = pathlib.Path(__file__).resolve().parents[1]
        base, findings = check.scan(root)
        self.assertTrue(base.startswith("https://"), base)
        self.assertEqual(findings, [], f"{len(findings)} mention(s) contradict {base}")


if __name__ == "__main__":
    unittest.main()
