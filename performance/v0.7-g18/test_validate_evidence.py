import json
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parent


class G18ContractTests(unittest.TestCase):
    def load(self, name: str) -> dict:
        return json.loads((ROOT / name).read_text(encoding="utf-8"))

    def test_abi_has_closed_status_and_bounded_stream_contract(self) -> None:
        abi = self.load("ABI_CONTRACT.json")
        self.assertEqual(len(abi["status_codes"]), len(set(abi["status_codes"])))
        self.assertIn("end_of_stream", abi["status_codes"])
        self.assertEqual(abi["bounds"]["stream_read_bytes"], 1024 * 1024)
        self.assertFalse(abi["callbacks_from_go_threads"])

    def test_platform_matrix_is_rectangular(self) -> None:
        matrix = self.load("PLATFORM_MATRIX.json")
        allowed = set(matrix["status_values"])
        for capability in matrix["capabilities"]:
            self.assertTrue(set(matrix["platforms"]).issubset(capability))
            self.assertTrue(all(capability[target] in allowed for target in matrix["platforms"]))

    def test_mermaid_limits_have_negative_fixtures(self) -> None:
        mermaid = self.load("MERMAID_CONTRACT.json")
        fixture_ids = {item["id"] for item in mermaid["fixtures"]}
        self.assertTrue({"malformed_syntax", "node_limit_plus_one", "edge_limit_plus_one",
                         "source_byte_limit_plus_one", "offline_cache_disabled", "csp_probe"}
                        .issubset(fixture_ids))
        self.assertFalse(mermaid["current"]["enabled"])


if __name__ == "__main__":
    unittest.main()
