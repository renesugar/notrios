import json
import shutil
import tempfile
import unittest
from pathlib import Path

import validate_evidence as ve


class G18gEvidenceTest(unittest.TestCase):
    def test_source_contract_passes(self):
        frozen = ve.ROOT / "performance/v0.7-g18b/prototype/themes/hugo-theme-ledger"
        with tempfile.TemporaryDirectory() as td:
            production = Path(td) / "hugo-theme-ledger"
            shutil.copytree(frozen, production)
            provenance = {"schema": "notrios.g18g.theme-provenance.v1", "upstream_commit": "f9d28ea297427890ecffa31fa74caa9ee385d9f5", "file_count": 44, "byte_count": 173947, "manifest_sha256": "2897566a3861c1a92f6f83d2606d8605c085d4b63d3f67adbef4954c55880025"}
            ve.validate_theme_pair(production, frozen, provenance)

    def test_production_theme_byte_mutation_is_rejected(self):
        frozen = ve.ROOT / "performance/v0.7-g18b/prototype/themes/hugo-theme-ledger"
        with tempfile.TemporaryDirectory() as td:
            production = Path(td) / "hugo-theme-ledger"
            shutil.copytree(frozen, production)
            target = production / "LICENSE"
            target.write_bytes(target.read_bytes() + b"mutation")
            provenance = {"schema": "notrios.g18g.theme-provenance.v1", "upstream_commit": "f9d28ea297427890ecffa31fa74caa9ee385d9f5", "file_count": 44, "byte_count": 173947, "manifest_sha256": "2897566a3861c1a92f6f83d2606d8605c085d4b63d3f67adbef4954c55880025"}
            with self.assertRaises(ve.EvidenceError): ve.validate_theme_pair(production, frozen, provenance)

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
