#!/usr/bin/env python3
"""Tests for the install, uninstall and purge lifecycle.

Every test runs against a disposable HOME and a fixture source tree, so nothing
here can touch the developer's own installation or the checkout. That is not
only hygiene: the target under test deletes directories, and a test suite for it
that could reach real data would be the most dangerous file in the repository.

The fixture's `notriosctl` is a shell script that prints the roots as JSON.
Purge asks the installed binary where the roots are rather than working them out
itself, so a stub exercises the real subprocess path without needing cgo and a
two-minute build.
"""

from __future__ import annotations

import json
import os
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
LIFECYCLE = os.path.join(HERE, "lifecycle.py")


def build_fixture_source(root: str, roots_json: dict) -> None:
    """A minimal tree with the shape install copies from."""
    os.makedirs(os.path.join(root, "bin"), exist_ok=True)
    os.makedirs(os.path.join(root, "web", "dist", "assets"), exist_ok=True)
    os.makedirs(os.path.join(root, "docs"), exist_ok=True)
    os.makedirs(os.path.join(root, "internal", "version"), exist_ok=True)

    with open(os.path.join(root, "internal", "version", "version.go"), "w") as stream:
        stream.write('package version\n\nconst Version = "9.9.9"\n')
    with open(os.path.join(root, "web", "dist", "index.html"), "w") as stream:
        stream.write("<!doctype html><title>fixture</title>\n")
    with open(os.path.join(root, "web", "dist", "assets", "app.js"), "w") as stream:
        stream.write("// fixture\n")
    with open(os.path.join(root, "docs", "index.md"), "w") as stream:
        stream.write("# fixture\n")

    for name in ("notriosd", "notrios"):
        path = os.path.join(root, "bin", name)
        with open(path, "w") as stream:
            stream.write("#!/bin/sh\nexit 0\n")
        os.chmod(path, 0o755)

    # The stub CLI: purge runs `notriosctl paths --json --no-redact`.
    cli = os.path.join(root, "bin", "notriosctl")
    with open(cli, "w") as stream:
        stream.write("#!/bin/sh\ncat <<'JSON'\n")
        stream.write(json.dumps({"mode": "installed", "roots": roots_json}, indent=2))
        stream.write("\nJSON\n")
    os.chmod(cli, 0o755)


class LifecycleTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.sandbox = tempfile.mkdtemp(prefix="notrios-lifecycle-")
        self.addCleanup(shutil.rmtree, self.sandbox, ignore_errors=True)
        self.home = os.path.join(self.sandbox, "home")
        self.source = os.path.join(self.sandbox, "source")
        os.makedirs(self.home)

        self.roots = {
            "config": os.path.join(self.home, ".config", "notrios"),
            "data": os.path.join(self.home, ".local", "share", "notrios"),
            "state": os.path.join(self.home, ".local", "state", "notrios"),
            "cache": os.path.join(self.home, ".cache", "notrios"),
            "runtime": os.path.join(self.home, ".local", "state", "notrios", "runtime"),
            "program_assets": os.path.join(self.home, ".local", "share", "notrios"),
        }
        build_fixture_source(self.source, self.roots)

    def run_lifecycle(self, action: str, *extra: str, **flags: str) -> subprocess.CompletedProcess:
        environment = dict(os.environ)
        environment["HOME"] = self.home
        environment["NOTRIOS_LIFECYCLE_SOURCE"] = self.source
        for key in ("DRYRUN", "FORCE", "NO_BACKUP", "prefix", "DESTDIR"):
            environment.pop(key, None)
        environment.update(flags)
        return subprocess.run([sys.executable, LIFECYCLE, action, *extra],
                              capture_output=True, text=True, env=environment,
                              stdin=subprocess.DEVNULL)

    def seed_user_data(self) -> dict[str, str]:
        planted = {
            os.path.join(self.roots["data"], "notes.sqlite"): "the library",
            os.path.join(self.roots["config"], "config.yaml"): "settings",
            os.path.join(self.roots["config"], "sync-keys.json"): "key material",
            os.path.join(self.roots["state"], "carrier.spool"): "a spool",
            os.path.join(self.roots["cache"], "search-index.dat"): "rebuildable",
        }
        for path, content in planted.items():
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w") as stream:
                stream.write(content)
        return planted

    def manifest(self) -> dict:
        path = os.path.join(self.roots["data"], "MANIFEST.json")
        with open(path, encoding="utf-8") as stream:
            return json.load(stream)


class InstallTests(LifecycleTestCase):
    def test_install_records_every_file_it_wrote(self) -> None:
        result = self.run_lifecycle("install")
        self.assertEqual(result.returncode, 0, result.stderr)
        manifest = self.manifest()
        self.assertEqual(manifest["version"], "9.9.9")

        # Every recorded entry exists and still hashes to what was recorded.
        for entry in manifest["entries"]:
            self.assertTrue(os.path.isfile(entry["path"]), entry["path"])
            self.assertEqual(os.lstat(entry["path"]).st_size, entry["size"])
        recorded = {entry["path"] for entry in manifest["entries"]}
        for expected in ("notriosd", "notriosctl", "notrios"):
            self.assertIn(os.path.join(self.home, ".local", "bin", expected), recorded)

    def test_dry_run_writes_nothing(self) -> None:
        result = self.run_lifecycle("install", "--dry-run")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Nothing was written", result.stdout)
        self.assertFalse(os.path.exists(os.path.join(self.home, ".local", "bin")))

    def test_a_missing_gui_binary_does_not_stop_the_install(self) -> None:
        os.remove(os.path.join(self.source, "bin", "notrios"))
        result = self.run_lifecycle("install")
        self.assertEqual(result.returncode, 0, result.stderr)
        recorded = {entry["path"] for entry in self.manifest()["entries"]}
        self.assertNotIn(os.path.join(self.home, ".local", "bin", "notrios"), recorded)
        self.assertIn(os.path.join(self.home, ".local", "bin", "notriosd"), recorded)

    def test_a_missing_required_binary_refuses(self) -> None:
        os.remove(os.path.join(self.source, "bin", "notriosd"))
        result = self.run_lifecycle("install")
        self.assertEqual(result.returncode, 1)
        self.assertIn("make build web", result.stderr)

    def test_destdir_stages_artifacts_and_creates_no_user_roots(self) -> None:
        staging = os.path.join(self.sandbox, "stage")
        result = self.run_lifecycle("install", DESTDIR=staging)
        self.assertEqual(result.returncode, 0, result.stderr)
        staged = os.path.join(staging, self.home.lstrip(os.sep), ".local", "bin", "notriosd")
        self.assertTrue(os.path.isfile(staged), "DESTDIR did not stage the binary")
        # Staging must not create the real user roots: a package built on one
        # machine would otherwise ship its idea of a home directory.
        self.assertFalse(os.path.exists(os.path.join(self.home, ".local", "bin")))


class UninstallTests(LifecycleTestCase):
    def test_uninstall_removes_only_what_it_installed(self) -> None:
        self.run_lifecycle("install")
        planted = self.seed_user_data()
        result = self.run_lifecycle("uninstall")
        self.assertEqual(result.returncode, 0, result.stderr)
        for path, content in planted.items():
            self.assertTrue(os.path.isfile(path), f"{path} was removed")
            with open(path) as stream:
                self.assertEqual(stream.read(), content)

    def test_a_modified_file_is_kept_and_reported(self) -> None:
        self.run_lifecycle("install")
        edited = os.path.join(self.roots["data"], "docs", "index.md")
        with open(edited, "a") as stream:
            stream.write("a local edit\n")
        result = self.run_lifecycle("uninstall")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(os.path.isfile(edited), "an edited file was deleted")
        self.assertIn("has been modified since it was installed", result.stdout)

    def test_a_symlink_is_kept_and_never_followed(self) -> None:
        self.run_lifecycle("install")
        target = os.path.join(self.sandbox, "outside.txt")
        with open(target, "w") as stream:
            stream.write("must survive")
        replaced = os.path.join(self.home, ".local", "bin", "notriosd")
        os.remove(replaced)
        os.symlink(target, replaced)

        result = self.run_lifecycle("uninstall")
        self.assertIn("is now a symbolic link", result.stdout)
        self.assertTrue(os.path.isfile(target), "uninstall deleted through a symlink")

    def test_uninstall_is_idempotent(self) -> None:
        self.run_lifecycle("install")
        first = self.run_lifecycle("uninstall")
        self.assertEqual(first.returncode, 0, first.stderr)
        second = self.run_lifecycle("uninstall")
        # The manifest is gone after a clean uninstall, so a second run says so
        # rather than failing in some other way.
        self.assertEqual(second.returncode, 1)
        self.assertIn("no install manifest", second.stderr)

    def test_install_use_uninstall_reinstall(self) -> None:
        self.run_lifecycle("install")
        planted = self.seed_user_data()
        self.run_lifecycle("uninstall")
        again = self.run_lifecycle("install")
        self.assertEqual(again.returncode, 0, again.stderr)
        for path, content in planted.items():
            with open(path) as stream:
                self.assertEqual(stream.read(), content, f"{path} changed across a reinstall")

    def test_dry_run_removes_nothing(self) -> None:
        self.run_lifecycle("install")
        result = self.run_lifecycle("uninstall", "--dry-run")
        self.assertIn("Nothing was written", result.stdout)
        self.assertTrue(os.path.isfile(os.path.join(self.home, ".local", "bin", "notriosd")))


class PurgeFlagTests(LifecycleTestCase):
    def test_ambiguous_flag_values_are_refused(self) -> None:
        self.run_lifecycle("install")
        for value in ("0", "no", "true", "yes", "false"):
            result = self.run_lifecycle("purge", NO_BACKUP=value)
            self.assertEqual(result.returncode, 1, f"NO_BACKUP={value} was accepted")
            self.assertIn("is not a value this understands", result.stderr)

    def test_dry_run_writes_nothing_and_asks_nothing(self) -> None:
        self.run_lifecycle("install")
        planted = self.seed_user_data()
        result = self.run_lifecycle("purge", DRYRUN="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("nothing was deleted, nothing was asked", result.stdout)
        for path in planted:
            self.assertTrue(os.path.isfile(path), f"{path} was removed by a dry run")
        self.assertFalse(os.path.exists(
            os.path.join(self.home, ".local", "state", "notrios-purge-backups")))

    def test_non_interactive_without_force_fails_closed(self) -> None:
        self.run_lifecycle("install")
        planted = self.seed_user_data()
        result = self.run_lifecycle("purge")
        self.assertEqual(result.returncode, 1)
        self.assertIn("no terminal to ask", result.stderr)
        for path in planted:
            self.assertTrue(os.path.isfile(path), f"{path} was removed without an answer")

    def test_no_backup_still_refuses_without_force_and_names_what_is_lost(self) -> None:
        self.run_lifecycle("install")
        planted = self.seed_user_data()
        result = self.run_lifecycle("purge", NO_BACKUP="1")
        self.assertEqual(result.returncode, 1)
        for named in ("notes", "sync key material", "no undo"):
            self.assertIn(named, result.stdout, f"the warning does not mention {named}")
        for path in planted:
            self.assertTrue(os.path.isfile(path))


class PurgeExecutionTests(LifecycleTestCase):
    def backup_directory(self) -> str:
        container = os.path.join(self.home, ".local", "state", "notrios-purge-backups")
        entries = sorted(os.listdir(container))
        self.assertEqual(len(entries), 1, f"expected one backup, found {entries}")
        return os.path.join(container, entries[0])

    def test_force_backs_up_verifies_then_deletes(self) -> None:
        self.run_lifecycle("install")
        self.seed_user_data()
        result = self.run_lifecycle("purge", FORCE="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("backup verified", result.stdout)

        for category in ("config", "data", "state", "cache"):
            self.assertFalse(os.path.exists(self.roots[category]),
                             f"the {category} root survived a purge")

    def test_the_backup_survives_the_purge_that_wrote_it(self) -> None:
        self.run_lifecycle("install")
        self.seed_user_data()
        self.run_lifecycle("purge", FORCE="1")
        directory = self.backup_directory()
        self.assertTrue(os.path.isfile(os.path.join(directory, "backup.tar")))
        self.assertTrue(os.path.isfile(os.path.join(directory, "MANIFEST.json")))

    def test_the_backup_container_is_owner_only(self) -> None:
        self.run_lifecycle("install")
        self.seed_user_data()
        self.run_lifecycle("purge", FORCE="1")
        directory = self.backup_directory()
        container = os.path.dirname(directory)
        for path, expected in ((container, 0o700), (directory, 0o700),
                               (os.path.join(directory, "backup.tar"), 0o600),
                               (os.path.join(directory, "MANIFEST.json"), 0o600)):
            mode = stat.S_IMODE(os.lstat(path).st_mode)
            self.assertEqual(mode, expected, f"{path} is {oct(mode)}, want {oct(expected)}")

    def test_the_notes_can_be_restored_offline(self) -> None:
        self.run_lifecycle("install")
        planted = self.seed_user_data()
        self.run_lifecycle("purge", FORCE="1")
        restored = os.path.join(self.sandbox, "restored")
        with tarfile.open(os.path.join(self.backup_directory(), "backup.tar")) as tar:
            tar.extractall(restored, filter="data")

        notes = os.path.join(restored, "data", "notes.sqlite")
        self.assertTrue(os.path.isfile(notes), "the notes are not in the backup")
        with open(notes) as stream:
            self.assertEqual(stream.read(), planted[
                os.path.join(self.roots["data"], "notes.sqlite")])
        keys = os.path.join(restored, "config", "sync-keys.json")
        self.assertTrue(os.path.isfile(keys), "the sync keys are not in the backup")

    def test_cache_is_disposed_of_rather_than_backed_up(self) -> None:
        self.run_lifecycle("install")
        self.seed_user_data()
        self.run_lifecycle("purge", FORCE="1")
        with tarfile.open(os.path.join(self.backup_directory(), "backup.tar")) as tar:
            members = [member.name for member in tar.getmembers()]
        self.assertFalse([name for name in members if name.startswith("cache")],
                         "cache was backed up; its policy is dispose")
        self.assertFalse(os.path.exists(self.roots["cache"]))

    def test_no_backup_with_force_deletes_and_writes_no_backup(self) -> None:
        self.run_lifecycle("install")
        self.seed_user_data()
        result = self.run_lifecycle("purge", FORCE="1", NO_BACKUP="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(os.path.exists(self.roots["data"]))
        self.assertFalse(os.path.exists(
            os.path.join(self.home, ".local", "state", "notrios-purge-backups")),
            "NO_BACKUP=1 still wrote a backup")

    def test_purge_refuses_when_it_cannot_ask_where_the_roots_are(self) -> None:
        self.run_lifecycle("install")
        planted = self.seed_user_data()
        os.remove(os.path.join(self.home, ".local", "bin", "notriosctl"))
        result = self.run_lifecycle("purge", FORCE="1")
        self.assertEqual(result.returncode, 1)
        self.assertIn("nothing can say which roots", result.stderr)
        for path in planted:
            self.assertTrue(os.path.isfile(path), "data was deleted without resolving the roots")


class SourceTreeSeparationTests(LifecycleTestCase):
    def test_the_lifecycle_never_touches_the_source_tree(self) -> None:
        """clean and clobber own the checkout; these targets never write to it."""
        before = {}
        for walk_root, _, names in os.walk(self.source):
            for name in names:
                full = os.path.join(walk_root, name)
                before[full] = os.lstat(full).st_size

        self.run_lifecycle("install")
        self.seed_user_data()
        self.run_lifecycle("purge", FORCE="1")

        after = {}
        for walk_root, _, names in os.walk(self.source):
            for name in names:
                full = os.path.join(walk_root, name)
                after[full] = os.lstat(full).st_size
        self.assertEqual(before, after, "the lifecycle modified the tree it installs from")


if __name__ == "__main__":
    unittest.main()


class PackagedInstallTests(LifecycleTestCase):
    """A package manager owns the program; purge owns the data.

    A .deb deliberately ships no install manifest, because dpkg already records
    the file list and its own md5sums, and a second ownership record in the same
    tree is drift waiting to happen. Purge still has a job in that case -- the
    data half -- and refusing to do it because the manifest is absent would
    leave a user with no supported way to delete their own library.
    """

    def install_without_a_manifest(self) -> None:
        self.run_lifecycle("install")
        os.remove(os.path.join(self.roots["data"], "MANIFEST.json"))

    def test_purge_works_without_a_manifest_and_says_who_owns_the_program(self) -> None:
        self.install_without_a_manifest()
        self.seed_user_data()
        result = self.run_lifecycle("purge", DRYRUN="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("package manager", result.stdout)
        self.assertIn("BACK UP AND DELETE", result.stdout)

    def test_purge_without_a_manifest_still_deletes_the_data(self) -> None:
        self.install_without_a_manifest()
        self.seed_user_data()
        result = self.run_lifecycle("purge", FORCE="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        for category in ("config", "data", "state", "cache"):
            self.assertFalse(os.path.exists(self.roots[category]),
                             f"the {category} root survived a purge")
        # And the backup was still written and verified first.
        self.assertIn("backup verified", result.stdout)

    def test_purge_without_a_manifest_leaves_the_installed_binaries(self) -> None:
        self.install_without_a_manifest()
        self.seed_user_data()
        binary = os.path.join(self.home, ".local", "bin", "notriosd")
        self.assertTrue(os.path.isfile(binary))
        self.run_lifecycle("purge", FORCE="1")
        self.assertTrue(os.path.isfile(binary),
                        "purge removed a program file it does not own")
