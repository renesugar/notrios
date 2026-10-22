import importlib.util, json, unittest
from pathlib import Path
from unittest import mock

HERE = Path(__file__).parent
spec = importlib.util.spec_from_file_location("licenses", HERE / "check_dependency_licenses.py")
v = importlib.util.module_from_spec(spec); spec.loader.exec_module(v)

class DependencyLicenseTests(unittest.TestCase):
    def test_inventory_passes(self):
        # 365 npm packages since v1.0 J45 moved to mermaid@12.2.0: its new
        # parser added chevrotain and four @chevrotain packages, katex@0.18.11
        # dropped fastdom and strictdom, and elkjs became the local
        # web/elk-refused stub, reached through a link that is not counted. It was
        # 361 from v0.8 H2, which added mermaid@11.17.2 and its 110 transitive
        # dependencies. 39 Go modules since v0.8 H9 adopted zalando/go-keyring,
        # which brought danieljoos/wincred and bumped godbus/dbus/v5 from v5.1.0
        # to v5.2.2. The count is asserted rather than computed so that an
        # unexplained dependency arriving still fails here.
        self.assertEqual(v.validate(), (39, 365))

    def test_missing_go_dependency_fails_closed(self):
        declared = v.go_modules()
        with mock.patch.object(v, "go_modules", return_value=declared | {"example.invalid/missing@v1.0.0"}):
            with self.assertRaisesRegex(v.LicenseError, "Go coverage mismatch"):
                v.validate()

    def test_forbidden_npm_license_fails_closed(self):
        original = v.load
        def poisoned(path):
            value = original(path)
            if str(path).endswith("web/package-lock.json"):
                value = json.loads(json.dumps(value))
                next(p for k, p in value["packages"].items() if k)["license"] = "GPL-3.0-only"
            return value
        with mock.patch.object(v, "load", side_effect=poisoned):
            with self.assertRaisesRegex(v.LicenseError, "unapproved or unknown"):
                v.validate()

    def _poison_web_lockfile(self, change):
        original = v.load
        def poisoned(path):
            value = original(path)
            if str(path).endswith("web/package-lock.json"):
                value = json.loads(json.dumps(value))
                change(value["packages"])
            return value
        return mock.patch.object(v, "load", side_effect=poisoned)

    def test_link_is_checked_by_its_target_license(self):
        # v1.0 J45: elkjs is a link to web/elk-refused, so the stub's licence is
        # the one that counts. A link has no licence field of its own, and
        # before the rule it was refused for that; test_inventory_passes shows
        # it is now accepted. Were the stub GPL, it would be refused.
        def change(packages):
            self.assertTrue(packages["node_modules/elkjs"].get("link"))
            self.assertIsNone(packages["node_modules/elkjs"].get("license"))
            packages[packages["node_modules/elkjs"]["resolved"]]["license"] = "GPL-3.0-only"
        with self._poison_web_lockfile(change):
            with self.assertRaisesRegex(v.LicenseError, "unapproved or unknown license 'GPL-3.0-only'"):
                v.validate()

    def test_link_target_alone_decides(self):
        # The link is checked itself, through its target and under its own
        # name, rather than passing because the target's row happened to be.
        original = v.check_license
        seen = []
        def record(name, lic):
            seen.append(name)
            return original(name, lic)
        with mock.patch.object(v, "check_license", side_effect=record):
            v.validate()
        self.assertIn("web/package-lock.json:node_modules/elkjs -> elk-refused@0.0.0-notrios", seen)

    def test_link_to_an_unlisted_target_fails_closed(self):
        def change(packages):
            packages["node_modules/elkjs"]["resolved"] = "not-in-the-lockfile"
        with self._poison_web_lockfile(change):
            with self.assertRaisesRegex(v.LicenseError, "which the lockfile does not list"):
                v.validate()

    def test_uninventoried_lockfile_fails_closed(self):
        with mock.patch.object(v, "npm_lockfiles", return_value=v.npm_lockfiles() | {"new/package-lock.json"}):
            with self.assertRaisesRegex(v.LicenseError, "npm lockfile coverage mismatch"):
                v.validate()

if __name__ == "__main__": unittest.main()
