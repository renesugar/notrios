import copy
import json
import unittest
from pathlib import Path

from validate_evidence import EvidenceError, validate


ROOT = Path(__file__).parent


class EvidenceValidationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.manifest = json.loads((ROOT / "JOURNEYS.json").read_text())
        cls.report = json.loads((ROOT / "REPORT.json").read_text())

    def test_baseline(self):
        validate(self.manifest, self.report)

    def assert_mutation_rejected(self, mutate):
        report = copy.deepcopy(self.report)
        mutate(report)
        with self.assertRaises(EvidenceError):
            validate(self.manifest, report)

    def test_wrong_label_rejected(self):
        manifest = copy.deepcopy(self.manifest)
        manifest["journeys"][0]["label"] = ""
        with self.assertRaises(EvidenceError):
            validate(manifest, self.report)

    def test_missing_result_rejected(self):
        self.assert_mutation_rejected(lambda r: r["results"].pop(0))

    def test_omitted_action_metric_rejected(self):
        self.assert_mutation_rejected(lambda r: r["results"][0]["actions"].pop("clicks"))

    def test_zero_assertions_rejected(self):
        self.assert_mutation_rejected(lambda r: r["results"][0]["assertions"].update({"visible": 0}))

    def test_wrong_viewport_rejected(self):
        self.assert_mutation_rejected(lambda r: r["results"][0].update({"viewport": "mobile"}))

    def test_external_request_rejected(self):
        self.assert_mutation_rejected(lambda r: r["health"][0]["external_requests"].append("https://example.invalid"))

    def test_duplicate_health_identity_rejected(self):
        self.assert_mutation_rejected(lambda r: r["health"][1].update({"viewport": "desktop", "replica": "host"}))

    def test_corrupted_unrun_reason_rejected(self):
        self.assert_mutation_rejected(lambda r: r["results"][-1]["reason"].update({"code": "executed"}))


if __name__ == "__main__":
    unittest.main()
