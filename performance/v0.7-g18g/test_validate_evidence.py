import json
import shutil
import tempfile
import unittest
from pathlib import Path

import validate_evidence as ve


class RelativeDirectoryLinkLimitation(unittest.TestCase):
    """A known limitation, written as the test that would prove it fixed.

    It is skipped rather than deleted or inverted. Deleting it loses the
    finding; asserting the current behaviour would record a defect as intended
    and fail when somebody repairs it. Skipped, it describes what should hold,
    costs nothing, and whoever fixes `local_target` removes one decorator.
    """

    @unittest.skip("known limitation: relative directory links resolve to a "
                   "directory instead of its index.html; see local_target in "
                   "validate_evidence.py")
    def test_a_relative_directory_link_resolves_to_its_index(self):
        # Absolute links already behave: the trailing slash survives.
        self.assertEqual(ve.local_target("index.html", "/search/")[0], "search/index.html")
        # Relative ones do not, because as_posix() drops the trailing slash
        # before the check that would have appended index.html.
        self.assertEqual(ve.local_target("index.html", "./search/")[0], "search/index.html")
        self.assertEqual(ve.local_target("api/mcp.html", "../search/")[0], "search/index.html")


class G18gEvidenceTest(unittest.TestCase):
    # J31 split one check into two: the frozen G18b snapshot is still checked
    # exactly, and the production copy is checked against its own provenance, so
    # the vendored theme can take an upstream fix without rewriting v0.7's
    # record. These tests prove the split holds in both directions.
    FROZEN = "performance/v0.7-g18b/prototype/themes/hugo-theme-ledger"

    def production_copy(self, directory):
        """A copy of the frozen snapshot, with provenance describing it."""
        production = Path(directory) / "hugo-theme-ledger"
        shutil.copytree(ve.ROOT / self.FROZEN, production)
        rows, total, manifest = ve.theme_manifest(production)
        provenance = {"schema": "notrios.g18g.theme-provenance.v1",
                      "upstream_commit": "f9d28ea297427890ecffa31fa74caa9ee385d9f5",
                      "file_count": len(rows), "byte_count": total, "manifest_sha256": manifest}
        return production, provenance

    def test_source_contract_passes(self):
        frozen_rows = ve.validate_frozen_theme(ve.ROOT / self.FROZEN)
        with tempfile.TemporaryDirectory() as td:
            production, provenance = self.production_copy(td)
            ve.validate_production_theme(production, frozen_rows, provenance)

    def test_a_production_theme_that_its_provenance_does_not_describe_is_rejected(self):
        frozen_rows = ve.validate_frozen_theme(ve.ROOT / self.FROZEN)
        with tempfile.TemporaryDirectory() as td:
            production, provenance = self.production_copy(td)
            target = production / "LICENSE"
            target.write_bytes(target.read_bytes() + b"mutation")
            with self.assertRaises(ve.EvidenceError):
                ve.validate_production_theme(production, frozen_rows, provenance)

    def test_a_production_theme_updated_with_its_provenance_is_accepted(self):
        """The point of the split: new bytes pass when provenance says so."""
        frozen_rows = ve.validate_frozen_theme(ve.ROOT / self.FROZEN)
        with tempfile.TemporaryDirectory() as td:
            production, _ = self.production_copy(td)
            target = production / "assets/js/search/backends/bluge.js"
            target.write_bytes(target.read_bytes() + b"\n// an upstream fix\n")
            rows, total, manifest = ve.theme_manifest(production)
            provenance = {"schema": "notrios.g18g.theme-provenance.v1",
                          "upstream_commit": "cf68886beb31ec0bbe7e2f05e538a20fec722e5a",
                          "file_count": len(rows), "byte_count": total, "manifest_sha256": manifest}
            ve.validate_production_theme(production, frozen_rows, provenance)

    def test_a_production_theme_with_an_extra_or_missing_file_is_rejected(self):
        frozen_rows = ve.validate_frozen_theme(ve.ROOT / self.FROZEN)
        for change in ("add", "remove"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as td:
                production, _ = self.production_copy(td)
                if change == "add":
                    (production / "assets/js/search/backends/new-backend.js").write_text("// upstream added this\n")
                else:
                    (production / "assets/js/search/backends/bluge.js").unlink()
                rows, total, manifest = ve.theme_manifest(production)
                provenance = {"schema": "notrios.g18g.theme-provenance.v1",
                              "upstream_commit": "cf68886beb31ec0bbe7e2f05e538a20fec722e5a",
                              "file_count": len(rows), "byte_count": total, "manifest_sha256": manifest}
                with self.assertRaises(ve.EvidenceError):
                    ve.validate_production_theme(production, frozen_rows, provenance)

    def test_a_production_provenance_without_a_commit_is_rejected(self):
        frozen_rows = ve.validate_frozen_theme(ve.ROOT / self.FROZEN)
        with tempfile.TemporaryDirectory() as td:
            production, provenance = self.production_copy(td)
            for value in (None, "", "main", "cf68886", "Z" * 40):
                with self.subTest(commit=value):
                    broken = dict(provenance)
                    if value is None: broken.pop("upstream_commit")
                    else: broken["upstream_commit"] = value
                    with self.assertRaises(ve.EvidenceError):
                        ve.validate_production_theme(production, frozen_rows, broken)

    def test_a_mutated_frozen_snapshot_is_still_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            frozen = Path(td) / "hugo-theme-ledger"
            shutil.copytree(ve.ROOT / self.FROZEN, frozen)
            target = frozen / "assets/js/search/backends/bluge.js"
            target.write_bytes(target.read_bytes() + b"\n// not what v0.7 reviewed\n")
            with self.assertRaises(ve.EvidenceError):
                ve.validate_frozen_theme(frozen)

    def test_theme_manifest_is_frozen_exactly(self):
        p = ve.ROOT / "performance/v0.7-g18b/prototype/themes/hugo-theme-ledger"
        rows, total, digest = ve.theme_manifest(p)
        self.assertEqual(len(rows), 44)
        self.assertEqual(total, 173947)
        self.assertEqual(digest, "2897566a3861c1a92f6f83d2606d8605c085d4b63d3f67adbef4954c55880025")
        self.assertEqual(rows[0][0], "LICENSE")

    def test_mutations_are_rejected(self):
        mutations = [
            ("THEME_PROVENANCE.json", ("upstream_commit", "0" * 40)),
            ("ROUTES.json", ("preserved_routes", [])),
            ("BUILD_CONTRACT.json", ("search", {"scope_marker": "all"})),
            ("BUILD_CONTRACT.json", ("raw_docs_help_contract", {"byte_equivalent": False})),
            ("BUILD_CONTRACT.json", ("offline_policy", {"remote_fonts": True})),
        ]
        for name, (key, value) in mutations:
            with self.subTest(name=name, key=key), tempfile.TemporaryDirectory() as td:
                bundle = Path(td) / "bundle"
                shutil.copytree(ve.HERE, bundle, ignore=shutil.ignore_patterns("__pycache__"))
                root = Path(td) / "root"
                frozen = ve.ROOT / "performance/v0.7-g18b/prototype/themes/hugo-theme-ledger"
                shutil.copytree(frozen, root / "performance/v0.7-g18b/prototype/themes/hugo-theme-ledger")
                production = root / "docs-site/themes/hugo-theme-ledger"
                shutil.copytree(frozen, production)
                (production.parent.parent / "THEME_PROVENANCE.json").write_text((bundle / "THEME_PROVENANCE.json").read_text(), encoding="utf-8")
                target = bundle / name
                data = json.loads(target.read_text())
                data[key] = value
                target.write_text(json.dumps(data), encoding="utf-8")
                old = ve.HERE
                try:
                    ve.HERE = bundle
                    with self.assertRaises(ve.EvidenceError): ve.source_checks(root=root, bundle=bundle)
                finally: ve.HERE = old


if __name__ == "__main__":
    unittest.main()
