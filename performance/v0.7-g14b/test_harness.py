#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest


SPEC = importlib.util.spec_from_file_location("g14b_harness", Path(__file__).with_name("harness.py"))
assert SPEC and SPEC.loader
harness = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(harness)


class HarnessTests(unittest.TestCase):
    def test_privacy_rejects_detail_fields_and_paths(self) -> None:
        for value in ({"title": "private"}, {"detail": "/home/person/private"}, {"argv": ["tool"]}):
            with self.assertRaises(harness.HarnessError):
                harness.scan_privacy(value)
        harness.scan_privacy({"counts": {"documents": 10}, "digest": "a" * 64})

    def test_inventory_detects_mutation_and_histogram(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "empty").mkdir()
            (root / "filled").mkdir()
            (root / "filled" / "one").write_bytes(b"one")
            before = harness.inventory(root, content_hash=True)
            self.assertEqual(before["files"], 1)
            self.assertEqual(before["directories"], 3)
            self.assertEqual(sum(before["directory_entry_histogram"].values()), 3)
            (root / "filled" / "two").write_bytes(b"two")
            self.assertNotEqual(before, harness.inventory(root, content_hash=True))

    def test_sqlite_snapshot_rejects_truncation(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            snapshot = Path(temporary)
            database = sqlite3.connect(snapshot / "notes.sqlite")
            database.execute("CREATE TABLE sample(value TEXT)")
            database.commit(); database.close()
            (snapshot / "assets").mkdir()
            harness.write_manifest(snapshot, "test.v1", "tree")
            _, integrity = harness.verify_sqlite_snapshot(snapshot)
            self.assertEqual(integrity, "ok")
            self.assertFalse((snapshot / "notes.sqlite-wal").exists())
            self.assertFalse((snapshot / "notes.sqlite-shm").exists())
            with (snapshot / "notes.sqlite").open("r+b") as stream:
                stream.truncate(64)
            with self.assertRaises(harness.HarnessError):
                harness.verify_sqlite_snapshot(snapshot)

    def test_sqlite_tree_layout_recreates_zip_omitted_empty_assets(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            snapshot = Path(temporary)
            (snapshot / "manifest.json").write_text(json.dumps({"external_kind": "tree"}))
            harness.ensure_sqlite_snapshot_layout(snapshot)
            self.assertTrue((snapshot / "assets").is_dir())

    def test_immutable_atomic_result(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary) / "result.json"
            harness.atomic_json(target, {"one": 1}, False)
            with self.assertRaises(harness.HarnessError):
                harness.atomic_json(target, {"two": 2}, False)
            self.assertEqual(json.loads(target.read_text()), {"one": 1})

    def test_fingerprint_reads_body_from_current_revision(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            database = sqlite3.connect(root / "notes.sqlite")
            database.executescript("""
                CREATE TABLE documents(
                    id TEXT PRIMARY KEY, title TEXT NOT NULL,
                    current_revision_id TEXT, deleted_at TEXT
                );
                CREATE TABLE document_revisions(
                    id TEXT PRIMARY KEY, title TEXT NOT NULL, body TEXT NOT NULL
                );
                CREATE TABLE resources(id TEXT); CREATE TABLE blobs(sha256 TEXT, size_bytes INTEGER);
                CREATE TABLE document_resource_refs(id TEXT); CREATE TABLE note_tags(id TEXT);
                CREATE TABLE tags(id TEXT); CREATE TABLE notebooks(id TEXT);
                CREATE TABLE document_sources(id TEXT); CREATE TABLE source_bundle_items(sha256 TEXT);
                CREATE TABLE document_links(id TEXT);
                INSERT INTO documents VALUES ('document', 'stale title', 'revision', NULL);
                INSERT INTO document_revisions VALUES ('revision', 'current title', 'current body');
            """)
            database.commit(); database.close()
            first = harness.canonical_fingerprint(root)
            database = sqlite3.connect(root / "notes.sqlite")
            database.execute("UPDATE document_revisions SET body = 'changed body'")
            database.commit(); database.close()
            second = harness.canonical_fingerprint(root)
            self.assertNotEqual(first["document_content_sha256"], second["document_content_sha256"])
            self.assertEqual(first["counts"]["documents"], 1)

    def test_semantic_projection_ignores_only_leading_frontmatter(self) -> None:
        left = "---\nsource_system: joplin_raw\njoplin_id: private\n---\nvisible\n"
        right = "---\nsource_system: obsidian\nobsidian_path: private\n---\nvisible\n"
        self.assertEqual(harness.strip_leading_frontmatter(left), "visible\n")
        self.assertEqual(harness.strip_leading_frontmatter(left), harness.strip_leading_frontmatter(right))
        malformed = "---\nnot closed\nvisible"
        self.assertEqual(harness.strip_leading_frontmatter(malformed), malformed)
        self.assertEqual(harness.semantic_title("filename title", "# Visible title\nbody\n"), "Visible title")
        self.assertEqual(harness.semantic_title("stored title", "body\n"), "stored title")

    def test_fingerprint_cache_invalidates_when_database_changes(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "canonical"
            root.mkdir()
            database = sqlite3.connect(root / "notes.sqlite")
            database.executescript("""
                CREATE TABLE documents(id TEXT, title TEXT, current_revision_id TEXT, deleted_at TEXT);
                CREATE TABLE document_revisions(id TEXT, title TEXT, body TEXT);
                CREATE TABLE resources(id TEXT); CREATE TABLE blobs(sha256 TEXT, size_bytes INTEGER);
                CREATE TABLE document_resource_refs(id TEXT); CREATE TABLE note_tags(id TEXT);
                CREATE TABLE tags(id TEXT); CREATE TABLE notebooks(id TEXT);
                CREATE TABLE document_sources(id TEXT); CREATE TABLE source_bundle_items(sha256 TEXT);
                CREATE TABLE document_links(id TEXT);
                INSERT INTO documents VALUES ('d', 'title', 'r', NULL);
                INSERT INTO document_revisions VALUES ('r', 'title', 'body');
            """)
            database.commit(); database.close()
            old_cache = harness.FINGERPRINT_CACHE_ROOT
            harness.FINGERPRINT_CACHE_ROOT = Path(temporary) / "cache"
            try:
                first = harness.canonical_fingerprint(root)
                database = sqlite3.connect(root / "notes.sqlite")
                database.execute("UPDATE document_revisions SET body = 'a changed body that changes size'")
                database.commit(); database.close()
                second = harness.canonical_fingerprint(root)
            finally:
                harness.FINGERPRINT_CACHE_ROOT = old_cache
            self.assertNotEqual(first["document_content_sha256"], second["document_content_sha256"])

    def test_image_sanitizer_clears_only_declared_local_tables(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            database_path = Path(temporary) / "notes.sqlite"
            database = sqlite3.connect(database_path)
            database.execute("CREATE TABLE documents(id TEXT)")
            database.execute("CREATE TABLE jobs(id TEXT)")
            database.execute("INSERT INTO documents VALUES ('canonical')")
            database.execute("INSERT INTO jobs VALUES ('local')")
            database.commit(); database.close()
            cleared = harness.sanitize_image(database_path)
            self.assertEqual(cleared, ["jobs"])
            database = sqlite3.connect(database_path)
            self.assertEqual(database.execute("SELECT COUNT(*) FROM documents").fetchone()[0], 1)
            self.assertEqual(database.execute("SELECT COUNT(*) FROM jobs").fetchone()[0], 0)
            database.close()

    def test_interrupted_artifact_is_preserved(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            artifact = Path(temporary) / "snapshot"
            artifact.mkdir()
            (artifact / "partial").write_bytes(b"partial")
            harness.preserve_interrupted(artifact, 2)
            preserved = Path(temporary) / "snapshot.interrupted-attempt-1"
            self.assertFalse(artifact.exists())
            self.assertEqual((preserved / "partial").read_bytes(), b"partial")

    def test_repository_backup_operand_is_relative(self) -> None:
        parent, operand = harness.repository_backup_operand(Path("/private/source/tree"))
        self.assertEqual(parent, Path("/private/source"))
        self.assertEqual(operand, "tree")
        self.assertFalse(Path(operand).is_absolute())


if __name__ == "__main__":
    unittest.main()
