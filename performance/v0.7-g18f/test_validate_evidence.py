import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("g18f", Path(__file__).with_name("validate_evidence.py"))
v = importlib.util.module_from_spec(spec)
spec.loader.exec_module(v)


class EvidenceValidatorTests(unittest.TestCase):
    def setUp(self):
        self.template, self.slots = v.validate_templates()

    def loader_with(self, filename, replacement):
        def load(path):
            if Path(path).name == filename:
                return replacement
            return json.loads(Path(path).read_text(encoding="utf-8"))
        return load

    def test_repository_evidence_is_valid(self):
        v.main()

    def test_freshness_hash_mutation_fails(self):
        report = copy.deepcopy(v.load_json(v.HERE / "REPORT.json"))
        report["documents"][0]["sha256"] = "0" * 64
        with mock.patch.object(v, "load_json", side_effect=self.loader_with("REPORT.json", report)):
            with self.assertRaisesRegex(v.EvidenceError, "stale generated hash"):
                v.validate_report()

    def test_generated_marker_and_audience_mutations_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for path in {item[2] for item in self.slots} | {"docs/docgen/templates.json"}:
                (root / path).parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(v.ROOT / path, root / path)
            for source in v.ROOT.rglob("*.go"):
                if source.name.endswith("_test.go"):
                    continue
                text = source.read_text(encoding="utf-8")
                if "//notrios:doc " in text:
                    target = root / source.relative_to(v.ROOT)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text(text, encoding="utf-8")
            page = root / self.slots[0][2]
            page.write_text(page.read_text(encoding="utf-8").replace("<!-- notrios:generated:", "<!-- mutated:", 1), encoding="utf-8")
            with self.assertRaisesRegex(v.EvidenceError, "marker mismatch"):
                v.validate_generated(self.template, self.slots, root)

        mutated = list(self.slots)
        ident, audience, path, slug = mutated[0]
        mutated[0] = (ident, "api" if audience == "user" else "user", path, slug)
        with self.assertRaisesRegex(v.EvidenceError, "audience mismatch"):
            v.validate_generated(self.template, mutated)

    def test_advisory_matrix_triage_fixture_and_policy_mutations_fail(self):
        original = v.load_json(v.HERE / "ADVISORY_REPORT.json")
        mutations = []

        report = copy.deepcopy(original)
        report["calibration"]["confusion_matrix"]["supported"]["supported"] += 1
        mutations.append(("matrix drift", report))

        report = copy.deepcopy(original)
        contradicted = next(item for item in report["reviews"] if item.get("disposition"))
        contradicted.pop("disposition")
        mutations.append(("human disposition", report))

        report = copy.deepcopy(original)
        action = report["reviews"][0]["actionability_runs"][0]
        action["fixture"] = {"accepted": True, "id": "invented-fixture", "state": "executed", "reason": "mutation"}
        report["summary"]["accepted_fixture_attempts"] = 1
        mutations.append(("closed fixture catalog", report))

        report = copy.deepcopy(original)
        report["policy"]["automatic_edit"] = True
        mutations.append(("policy violation", report))

        for message, report in mutations:
            with self.subTest(message=message):
                with mock.patch.object(v, "load_json", side_effect=self.loader_with("ADVISORY_REPORT.json", report)):
                    with self.assertRaisesRegex(v.EvidenceError, message):
                        v.validate_advisory(self.slots)

    def test_mutation_matrix_names_independent_invariants(self):
        matrix = v.load_json(v.HERE / "MUTATION_MATRIX.json")
        names = {item["name"] for item in matrix["mutations"]}
        self.assertEqual(names, {"freshness-hash", "marker", "audience", "calibration-matrix", "triage", "fixture-id", "policy"})


if __name__ == "__main__":
    unittest.main()
