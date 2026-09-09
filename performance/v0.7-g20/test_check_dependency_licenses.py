import importlib.util, json, unittest
from pathlib import Path
from unittest import mock

HERE = Path(__file__).parent
spec = importlib.util.spec_from_file_location("licenses", HERE / "check_dependency_licenses.py")
v = importlib.util.module_from_spec(spec); spec.loader.exec_module(v)

class DependencyLicenseTests(unittest.TestCase):
    def test_inventory_passes(self):
        # 361 npm packages since v0.8 H2 added mermaid@11.17.2 and its 110
        # transitive dependencies. 39 Go modules since v0.8 H9 adopted
        # zalando/go-keyring, which brought danieljoos/wincred and bumped
        # godbus/dbus/v5 from v5.1.0 to v5.2.2. The count is asserted rather
        # than computed so that an unexplained dependency arriving still fails
        # here.
        self.assertEqual(v.validate(), (39, 361))

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

    def test_uninventoried_lockfile_fails_closed(self):
        with mock.patch.object(v, "npm_lockfiles", return_value=v.npm_lockfiles() | {"new/package-lock.json"}):
            with self.assertRaisesRegex(v.LicenseError, "npm lockfile coverage mismatch"):
                v.validate()

if __name__ == "__main__": unittest.main()
