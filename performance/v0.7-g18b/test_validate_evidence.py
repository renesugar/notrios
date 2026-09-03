from pathlib import Path
import tempfile
import unittest

import build_prototype
import validate_evidence


class G18bEvidenceTest(unittest.TestCase):
    def test_theme_manifest_is_exact(self) -> None:
        provenance = validate_evidence.load_json("THEME_PROVENANCE.json")
        snapshot = validate_evidence.ROOT / provenance["snapshot_root"]
        self.assertEqual(
            validate_evidence.theme_manifest(snapshot),
            (
                provenance["file_count"],
                provenance["byte_count"],
                provenance["manifest_sha256"],
            ),
        )

    def test_staging_preserves_all_canonical_doc_bytes(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            pairs = build_prototype.stage_source(Path(temporary) / "site")
            # 15 -> 16 in v0.8 H14 slice B: docs/features.md.
            self.assertEqual(len(pairs), 17)
            for source, target in pairs:
                self.assertEqual(source.read_bytes(), target.read_bytes())

    def test_route_resolution_is_base_scoped(self) -> None:
        self.assertEqual(
            validate_evidence.local_target("api/rest.html", "../service.html#listen"),
            ("api/../service.html", "listen"),
        )
        self.assertEqual(
            validate_evidence.local_target("index.html", "/outside.css"),
            ("OUTSIDE_BASE:/outside.css", ""),
        )
        self.assertIsNone(validate_evidence.local_target("index.html", "https://example.com/"))

    def test_static_evidence_passes(self) -> None:
        self.assertEqual(validate_evidence.validate_source(), [])


if __name__ == "__main__":
    unittest.main()
