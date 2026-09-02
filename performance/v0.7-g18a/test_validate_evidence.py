import json
import sys
import unittest
from pathlib import Path


HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import validate_evidence as evidence


class G18aEvidenceTests(unittest.TestCase):
    def test_checked_evidence(self):
        inventory = evidence.validate_inventory()
        evidence.validate_calibration()
        # 203 since v0.8 H2b added "Links that leave Notrios" and H2 added
        # "Diagrams" to docs/gui.md.
        # The baseline tracks the documentation surface, so a new section moves
        # it by one; the number is asserted rather than computed so that an
        # unnoticed section appearing or vanishing still fails here.
        # 203 -> 205 in v0.8 H4 slice D: the `paths` and `config show`
        # sections in docs/cli.md.
        # 205 -> 210 in v0.8 H4 slice E: `migrate` and its two subsections in
        # docs/cli.md, "Upgrading from before 0.8" in docs/installation.md, and
        # "Finding your notes" in docs/troubleshooting.md.
        # 210 -> 211 in v0.8 H4a: "The two default addresses" in
        # docs/service.md.
        # 211 -> 212 in v0.8 H4b: "Schema migrations and their backup" in
        # docs/service.md.
        self.assertEqual(inventory["grade_baseline"]["denominator"], 212)

    def test_duplicate_fragment_is_rejected(self):
        source = """// A.\n//notrios:doc user same\nfunc A() {}\n// B.\n//notrios:doc user same\nfunc B() {}\n"""
        with self.assertRaisesRegex(evidence.EvidenceError, "duplicate fragment"):
            evidence.parse_directive_groups(source)

    def test_dangling_directive_is_rejected(self):
        source = "// Text.\n//notrios:doc user dangling\n\nfunc Later() {}\n"
        with self.assertRaisesRegex(evidence.EvidenceError, "dangling"):
            evidence.parse_directive_groups(source)

    def test_mixed_audience_and_non_user_help_are_rejected(self):
        mixed = "// Text.\n//notrios:doc user endpoint\n//notrios:doc api endpoint-api\nfunc Endpoint() {}\n"
        with self.assertRaisesRegex(evidence.EvidenceError, "mixed audiences"):
            evidence.parse_directive_groups(mixed)
        source = "// Text.\n//notrios:doc api endpoint\n//notrios:help api section\nfunc Endpoint() {}\n"
        with self.assertRaisesRegex(evidence.EvidenceError, "requires user"):
            evidence.parse_directive_groups(source)

    def test_dangling_go_and_ts_anchors_are_rejected(self):
        for anchor in (
            "go:github.com/renesugar/notrios/internal/store#DoesNotExist",
            "ts:web/src/App.tsx#DoesNotExist",
            "web/src/App.tsx:93",
        ):
            with self.subTest(anchor=anchor), self.assertRaisesRegex(evidence.EvidenceError, "anchor"):
                evidence.validate_anchor(anchor)

    def test_direct_callee_scope_stops_at_one_hop(self):
        source = (HERE / "fixtures/anchors.tsx.fixture").read_text(encoding="utf-8")
        self.assertEqual(evidence.direct_ts_callees(source, "RootJourney"), {"directStep"})
        self.assertEqual(evidence.direct_ts_callees(source, "directStep"), {"transitiveStep"})
        self.assertNotIn("transitiveStep", evidence.direct_ts_callees(source, "RootJourney"))

    def test_negation_pair_is_lexically_near_but_opposite(self):
        cases = {case["id"]: case for case in json.loads((HERE / "CALIBRATION.json").read_text())["cases"]}
        supported = cases["recoll-negated-supported"]
        contradicted = cases["recoll-negation-mutation"]
        self.assertEqual(supported["claim"].replace(" not", ""), contradicted["claim"])
        self.assertNotEqual(supported["expected"], contradicted["expected"])


if __name__ == "__main__":
    unittest.main()
