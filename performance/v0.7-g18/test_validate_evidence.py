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

    def test_host_sqlite_is_not_android_target_evidence(self) -> None:
        android = self.load("ANDROID_FEASIBILITY.json")
        host = android["cross_compile"]["host_debian_sqlite_development"]
        self.assertTrue(host["header_found"])
        self.assertFalse(host["android_target_usable"])
        self.assertFalse(android["cross_compile"]["host_header_followup"]["passed"])
        self.assertTrue(android["flutter_doctor"]["android_toolchain_passed"])
        self.assertTrue(android["flutter_doctor"]["linux_desktop_toolchain_passed"])
        self.assertTrue(android["flutter_doctor"]["no_issues_found"])
        self.assertIsNone(android["flutter_doctor"]["linux_desktop_blocker"])
        self.assertEqual(android["flutter_doctor"]["connected_android_devices"], 0)

    def test_editor_search_rendered_controls_and_shortcut_are_honest(self) -> None:
        editor = self.load("EDITOR_SEARCH_QA.json")
        controls = set(editor["source_findings"]["editable_panel_controls"])
        self.assertTrue({"Find", "Replace", "match case", "regexp", "by word",
                         "replace", "replace all"}.issubset(controls))
        self.assertEqual(editor["source_findings"]["default_open_shortcut"], "Mod-f")
        self.assertFalse(editor["source_findings"]["ctrl_h_or_mod_h_default_binding"])
        rendered = editor["rendered_browser_qa"]
        self.assertTrue(rendered["single_replace_passed"])
        self.assertTrue(rendered["whole_word_replace_all_passed"])
        self.assertEqual(rendered["console_errors_or_warnings"], 0)

    def test_android_sqlite_options_preserve_go_database_ownership(self) -> None:
        followup = self.load("ANDROID_SQLITE_FOLLOWUP.json")
        approaches = {item["id"]: item for item in followup["approaches"]}
        self.assertEqual(approaches["pinned_upstream_amalgamation_in_go_core"]["fit"],
                         "recommended investigation default")
        self.assertEqual(approaches["jetpack_bundled_sqlite_driver"]["fit"],
                         "not a drop-in dependency for the selected Go core")
        self.assertFalse(followup["android_facts"]["ndk_public_sqlite_c_api"])
        self.assertIn("FTS5", followup["current_notrios_store"]["required_features"])
        self.assertIn("JSON SQL functions", followup["current_notrios_store"]["required_features"])

    def test_mermaid_limits_have_negative_fixtures(self) -> None:
        mermaid = self.load("MERMAID_CONTRACT.json")
        fixture_ids = {item["id"] for item in mermaid["fixtures"]}
        self.assertTrue({"malformed_syntax", "node_limit_plus_one", "edge_limit_plus_one",
                         "source_byte_limit_plus_one", "offline_cache_disabled", "csp_probe"}
                        .issubset(fixture_ids))
        self.assertFalse(mermaid["current"]["enabled"])


if __name__ == "__main__":
    unittest.main()
