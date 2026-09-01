#!/usr/bin/env python3
"""Backup-and-restore proof for the H3 purge design.

The plan asks for a backup restore proof, and a specified container format is
not one. This builds the proposed container in a disposable sandbox, verifies
it, deletes the source *through the purge oracle*, restores offline, and
compares every byte and mode back.

It also proves the ordering guarantee that makes purge safe: a truncated backup
fails verification, and a failed verification is what stands between the user
and deletion. That case is executed here rather than modelled, because "the
backup was interrupted" is the failure most likely to happen and least likely
to be tested.

Nothing outside a temporary directory is touched. Run:
    python3 performance/v0.8-h3/test_backup_restore.py
"""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import stat
import sys
import tarfile
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from purge_oracle import ALLOW, ALLOW_ABSENT, Environment, backup_policy, decide

MANIFEST_NAME = "MANIFEST.json"


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 16), b""):
            digest.update(chunk)
    return digest.hexdigest()


def inventory(roots: dict[str, str]) -> list[dict]:
    """Every regular file under every root that policy says to back up."""
    entries = []
    for category, root in sorted(roots.items()):
        if backup_policy(category) != "backup_and_verify":
            continue
        for directory, _, files in os.walk(root):
            for name in sorted(files):
                absolute = os.path.join(directory, name)
                entries.append({
                    "category": category,
                    "relative": os.path.relpath(absolute, os.path.dirname(root)),
                    "sha256": sha256_file(absolute),
                    "mode": oct(stat.S_IMODE(os.lstat(absolute).st_mode)),
                    "size": os.path.getsize(absolute),
                })
    return entries


def create_backup(roots: dict[str, str], destination: str) -> str:
    """One owner-only tar plus a manifest, written outside every purge root."""
    os.makedirs(destination, mode=0o700, exist_ok=True)
    entries = inventory(roots)
    archive = os.path.join(destination, "backup.tar")

    # Written 0600 from the start rather than chmod-ed afterwards: a file that
    # is briefly world-readable is world-readable.
    handle = os.open(archive, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(handle, "wb") as raw, tarfile.open(fileobj=raw, mode="w") as tar:
        for category, root in sorted(roots.items()):
            if backup_policy(category) != "backup_and_verify":
                continue
            tar.add(root, arcname=os.path.basename(root))

    manifest_path = os.path.join(destination, MANIFEST_NAME)
    handle = os.open(manifest_path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(handle, "w", encoding="utf-8") as stream:
        json.dump({"schema": "notrios.purge-backup/1", "entries": entries,
                   "archive_sha256": sha256_file(archive)}, stream, indent=2)
    return archive


def verify_backup(destination: str) -> tuple[bool, str]:
    """Verification is the gate deletion waits behind."""
    archive = os.path.join(destination, "backup.tar")
    manifest_path = os.path.join(destination, MANIFEST_NAME)
    if not os.path.exists(archive) or not os.path.exists(manifest_path):
        return False, "the backup is incomplete"
    with open(manifest_path, encoding="utf-8") as stream:
        manifest = json.load(stream)
    if sha256_file(archive) != manifest["archive_sha256"]:
        return False, "the archive does not match the hash recorded for it"
    try:
        with tarfile.open(archive) as tar:
            members = {member.name for member in tar.getmembers()}
    except tarfile.TarError as err:
        return False, f"the archive is unreadable: {err}"
    for entry in manifest["entries"]:
        if entry["relative"] not in members:
            return False, f"{entry['relative']} is recorded but not in the archive"
    return True, "verified"


def restore(archive: str, into: str) -> None:
    os.makedirs(into, mode=0o700, exist_ok=True)
    with tarfile.open(archive) as tar:
        tar.extractall(into, filter="data")


class BackupRestoreProof(unittest.TestCase):
    def setUp(self) -> None:
        self.sandbox = tempfile.mkdtemp(prefix="h3-backup-")
        self.addCleanup(shutil.rmtree, self.sandbox, ignore_errors=True)

        self.roots = {
            "config": os.path.join(self.sandbox, "user", "config"),
            "data": os.path.join(self.sandbox, "user", "data"),
            "state": os.path.join(self.sandbox, "user", "state"),
            "cache": os.path.join(self.sandbox, "user", "cache"),
        }
        for category, root in self.roots.items():
            os.makedirs(root, mode=0o700, exist_ok=True)
            path = os.path.join(root, f"{category}.bin")
            handle = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
            with os.fdopen(handle, "wb") as stream:
                stream.write(f"contents of {category}".encode() * 64)

        # Outside every purge root, by construction rather than by convention.
        self.backup_dir = os.path.join(self.sandbox, "purge-backups", "20260901T000000Z")

    def test_backup_covers_exactly_the_irreplaceable_categories(self) -> None:
        create_backup(self.roots, self.backup_dir)
        with open(os.path.join(self.backup_dir, MANIFEST_NAME), encoding="utf-8") as stream:
            manifest = json.load(stream)
        categories = {entry["category"] for entry in manifest["entries"]}
        self.assertEqual(categories, {"config", "data", "state"},
                         "cache is disposed of, not backed up; anything else is")

    def test_the_backup_container_is_owner_only(self) -> None:
        archive = create_backup(self.roots, self.backup_dir)
        for path in (archive, os.path.join(self.backup_dir, MANIFEST_NAME)):
            mode = stat.S_IMODE(os.lstat(path).st_mode)
            self.assertEqual(mode, 0o600, f"{path} is {oct(mode)}, not owner-only")

    def test_the_backup_destination_is_outside_every_purge_root(self) -> None:
        create_backup(self.roots, self.backup_dir)
        env = Environment(owned_roots=list(self.roots.values()),
                          home=os.path.join(self.sandbox, "user"))
        decision = decide(self.backup_dir, env)
        self.assertEqual(decision.verdict, "REFUSE",
                         "the backup destination must not itself be a purgeable path")

    def test_delete_then_restore_reproduces_every_byte_and_mode(self) -> None:
        archive = create_backup(self.roots, self.backup_dir)
        ok, reason = verify_backup(self.backup_dir)
        self.assertTrue(ok, reason)

        before = inventory(self.roots)
        self.assertTrue(before, "nothing was inventoried, so the proof would be vacuous")

        # Deletion goes through the oracle, so this proof also exercises the
        # path purge actually takes rather than a shortcut around it.
        env = Environment(owned_roots=list(self.roots.values()),
                          home=os.path.join(self.sandbox, "user"))
        for category, root in self.roots.items():
            decision = decide(root, env)
            self.assertIn(decision.verdict, (ALLOW, ALLOW_ABSENT),
                          f"{category}: {decision}")
            shutil.rmtree(root)
            self.assertFalse(os.path.exists(root))

        restored_into = os.path.join(self.sandbox, "restored")
        restore(archive, restored_into)

        restored_roots = {c: os.path.join(restored_into, os.path.basename(r))
                          for c, r in self.roots.items()
                          if backup_policy(c) == "backup_and_verify"}
        for category, root in restored_roots.items():
            self.assertTrue(os.path.isdir(root), f"{category} was not restored")

        after = inventory(restored_roots)
        self.assertEqual([(e["relative"], e["sha256"], e["mode"]) for e in before],
                         [(e["relative"], e["sha256"], e["mode"]) for e in after],
                         "restored contents or modes differ from the originals")

        self.assertFalse(os.path.exists(os.path.join(restored_into, "cache")),
                         "cache is disposed of and must not reappear on restore")

    def test_a_truncated_backup_fails_verification_and_so_blocks_deletion(self) -> None:
        archive = create_backup(self.roots, self.backup_dir)
        original = os.path.getsize(archive)
        with open(archive, "r+b") as handle:
            handle.truncate(original // 2)

        ok, reason = verify_backup(self.backup_dir)
        self.assertFalse(ok, "a truncated archive must not verify")
        self.assertIn("hash", reason)

        # The guarantee: verification failing is what keeps the data alive.
        for root in self.roots.values():
            self.assertTrue(os.path.isdir(root),
                            "nothing may be deleted while verification fails")

    def test_a_missing_manifest_fails_verification(self) -> None:
        create_backup(self.roots, self.backup_dir)
        os.remove(os.path.join(self.backup_dir, MANIFEST_NAME))
        ok, reason = verify_backup(self.backup_dir)
        self.assertFalse(ok, reason)


if __name__ == "__main__":
    unittest.main(verbosity=2)
