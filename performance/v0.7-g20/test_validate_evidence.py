import importlib.util
import json
import shutil
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).parent
spec = importlib.util.spec_from_file_location("g20_evidence", HERE / "validate_evidence.py")
v = importlib.util.module_from_spec(spec)
spec.loader.exec_module(v)

class ReleaseEvidenceTests(unittest.TestCase):
    def test_repository_evidence_passes(self):
        self.assertEqual(v.validate(), 11)

    def test_version_going_backwards_fails(self):
        """A tree older than the one this record describes is not that tree.

        This used to assert that any version other than 0.7.0 failed, with 0.7.1
        as the fixture. That made the record fail the day a later milestone
        shipped -- v0.8 H13 bumped the product to 0.8.0 and a finished
        milestone's evidence started reporting an error about a version it was
        never about. Drift now means going backwards or becoming unparseable,
        which is what would actually mean the tree is not the one described.
        """
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "internal/version").mkdir(parents=True)
            (root / "web").mkdir()
            shutil.copy2(v.ROOT / "internal/version/version.go", root / "internal/version/version.go")
            shutil.copy2(v.ROOT / "web/package.json", root / "web/package.json")
            package = json.loads((root / "web/package.json").read_text())
            package["version"] = "0.6.9"
            (root / "web/package.json").write_text(json.dumps(package))
            with self.assertRaisesRegex(v.EvidenceError, "older than the 0.7.0"):
                v.validate(root, v.REPORT)

    def test_missing_gate_fails(self):
        with tempfile.TemporaryDirectory() as temporary:
            report_path = Path(temporary) / "REPORT.json"
            report = json.loads(v.REPORT.read_text())
            report["gates"].pop()
            report_path.write_text(json.dumps(report))
            with self.assertRaisesRegex(v.EvidenceError, "gate coverage"):
                v.validate(v.ROOT, report_path)

    def hardening_copy(self):
        return json.loads((v.ROOT / "performance/v0.7-g20/hardening/hardening.json").read_text())

    def test_hardening_rejects_traversing_proposal(self):
        analysis = self.hardening_copy()
        analysis["opportunities"][0]["proposalPath"] = "../outside.md"
        with self.assertRaisesRegex(v.EvidenceError, "proposalPath"):
            v.validate_hardening(v.ROOT, analysis)

    def test_hardening_requires_resolved_recommendation_and_coverage(self):
        analysis = self.hardening_copy()
        analysis["opportunities"][0]["recommendedOptionId"] = "missing"
        with self.assertRaisesRegex(v.EvidenceError, "recommended option"):
            v.validate_hardening(v.ROOT, analysis)
        analysis = self.hardening_copy()
        analysis["opportunities"][0]["options"][0]["findingCoverage"] = []
        with self.assertRaisesRegex(v.EvidenceError, "findingCoverage"):
            v.validate_hardening(v.ROOT, analysis)

    def test_hardening_requires_all_tradeoff_dimensions_and_diagrams(self):
        analysis = self.hardening_copy()
        analysis["opportunities"][0]["options"][0]["tradeoffs"].pop()
        with self.assertRaisesRegex(v.EvidenceError, "tradeoff dimensions"):
            v.validate_hardening(v.ROOT, analysis)
        analysis = self.hardening_copy()
        analysis["opportunities"][0]["options"][0]["diagramPaths"]["after"] = "../outside.mmd"
        with self.assertRaisesRegex(v.EvidenceError, "diagram"):
            v.validate_hardening(v.ROOT, analysis)

    def test_hardening_requires_proposal_core_headings(self):
        analysis = self.hardening_copy()
        proposal = v.ROOT / "performance/v0.7-g20/hardening/proposals/network-admission-ownership.md"
        original = proposal.read_text()
        try:
            proposal.write_text(original.replace("## Decision\n", "", 1))
            with self.assertRaisesRegex(v.EvidenceError, "misses headings"):
                v.validate_hardening(v.ROOT, analysis)
        finally:
            proposal.write_text(original)

if __name__ == "__main__":
    unittest.main()
