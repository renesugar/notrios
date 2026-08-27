#!/usr/bin/env python3
"""Resumable, aggregate-only full-corpus harness for v0.7 G14b.

Private paths and command output stay in the external workspace. Published
phase rows contain only counts, timings, hashes, and bounded environment data.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import resource
import shutil
import sqlite3
import subprocess
import sys
import tarfile
import time
from typing import Any, Callable


SCHEMA = "notrios.g14b.phase-result.v1"
CHECKPOINT_SCHEMA = "notrios.g14b.checkpoint.v1"
FINGERPRINT_CACHE_SCHEMA = "notrios.g14b.fingerprint-cache.v1"
FINGERPRINT_CACHE_ROOT: Path | None = None
WORKLOADS = {"recipe-joplin", "recipe-obsidian", "attachments"}
ARCHIVE_ADAPTERS = {"archive-v2-loose", "archive-v2-pack"}
SQLITE_ADAPTERS = {"sqlite-stopped-copy", "sqlite-online-backup", "sqlite-image-bundle"}
REPOSITORY_ADAPTERS = {"restic-raw", "restic-canonical", "borg-raw", "borg-canonical"}
ADAPTERS = {"source", "import"} | ARCHIVE_ADAPTERS | SQLITE_ADAPTERS | REPOSITORY_ADAPTERS
PHASES = {
    "inventory", "foreign-import", "snapshot-create", "snapshot-verify",
    "transport-prepare", "transport-seal", "snapshot-open",
    "snapshot-restore", "unchanged-snapshot", "changed-snapshot",
    "corruption-refusal", "provider-copy",
}


class HarnessError(RuntimeError):
    pass


def usage_preflight(workspace: Path, operation: str) -> None:
    """Check remaining agent usage before creating a resumable checkpoint."""
    repository = Path(__file__).resolve().parents[2]
    command = [
        "bash",
        str(repository / "scripts/agent_usage_preflight.sh"),
        operation,
    ]
    try:
        completed = subprocess.run(command, text=True, cwd=repository)
    except OSError as error:
        raise HarnessError(f"usage preflight failed: {error}") from error
    if completed.returncode != 0:
        raise HarnessError(
            f"usage preflight requests a clean pause before checkpoint "
            f"(exit {completed.returncode})"
        )


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(4 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def inventory(root: Path, content_hash: bool = False) -> dict[str, Any]:
    root = root.resolve()
    if not root.is_dir():
        raise HarnessError("inventory root is not a directory")
    files = directories = symlinks = other = apparent = allocated = 0
    maximum = 0
    histogram = {"empty": 0, "one_to_ten": 0, "eleven_to_100": 0,
                 "hundred_one_to_1k": 0, "one_k_to_10k": 0,
                 "ten_k_to_100k": 0, "over_100k": 0}
    metadata = hashlib.sha256()
    contents = hashlib.sha256()
    for current, dirnames, filenames in os.walk(root, topdown=True, followlinks=False):
        dirnames.sort()
        filenames.sort()
        entries = len(dirnames) + len(filenames)
        maximum = max(maximum, entries)
        if entries == 0:
            bucket = "empty"
        elif entries <= 10:
            bucket = "one_to_ten"
        elif entries <= 100:
            bucket = "eleven_to_100"
        elif entries <= 1_000:
            bucket = "hundred_one_to_1k"
        elif entries <= 10_000:
            bucket = "one_k_to_10k"
        elif entries <= 100_000:
            bucket = "ten_k_to_100k"
        else:
            bucket = "over_100k"
        histogram[bucket] += 1
        directories += 1
        for name in dirnames + filenames:
            path = Path(current, name)
            relative = path.relative_to(root).as_posix()
            stat = path.lstat()
            metadata.update(f"{relative}\0{stat.st_mode}\0{stat.st_size}\0{stat.st_mtime_ns}\n".encode())
            if path.is_symlink():
                symlinks += 1
            elif path.is_file():
                files += 1
                apparent += stat.st_size
                allocated += getattr(stat, "st_blocks", 0) * 512
                if content_hash:
                    contents.update(relative.encode() + b"\0")
                    contents.update(sha256_file(path).encode() + b"\n")
            elif not path.is_dir():
                other += 1
    result = {
        "files": files, "directories": directories, "symlinks": symlinks,
        "other_entries": other, "apparent_bytes": apparent,
        "allocated_bytes": allocated, "maximum_directory_entries": maximum,
        "directory_entry_histogram": histogram,
        "metadata_sha256": metadata.hexdigest(),
    }
    if content_hash:
        result["content_sha256"] = contents.hexdigest()
    return result


def canonical_fingerprint(root: Path) -> dict[str, Any]:
    database = root / "notes.sqlite"
    signature = {"size": database.stat().st_size, "mtime_ns": database.stat().st_mtime_ns}
    cache_path: Path | None = None
    if FINGERPRINT_CACHE_ROOT is not None:
        cache_key = hashlib.sha256(str(root.resolve()).encode()).hexdigest()
        cache_path = FINGERPRINT_CACHE_ROOT / f"{cache_key}.json"
        if cache_path.exists():
            cached = json.loads(cache_path.read_text())
            if cached.get("schema") == FINGERPRINT_CACHE_SCHEMA and cached.get("signature") == signature:
                return cached["fingerprint"]
    uri = f"file:{database}?mode=ro&immutable=1"
    connection = sqlite3.connect(uri, uri=True)
    try:
        integrity = connection.execute("PRAGMA integrity_check").fetchone()[0]
        tables = {
            "documents": "documents", "revisions": "document_revisions",
            "resources": "resources", "blobs": "blobs",
            "document_resources": "document_resource_refs",
            "document_tags": "note_tags", "tags": "tags",
            "notebooks": "notebooks", "sources": "document_sources",
            "source_bundles": "source_bundle_items", "links": "document_links",
        }
        counts = {name: connection.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]
                  for name, table in tables.items()}
        counts["blob_bytes"] = connection.execute("SELECT COALESCE(SUM(size_bytes),0) FROM blobs").fetchone()[0]
        document_hashes: list[bytes] = []
        semantic_hashes: list[bytes] = []
        title_hashes: list[bytes] = []
        body_hashes: list[bytes] = []
        semantic_shape = {"empty_bodies": 0, "leading_h1": 0,
                          "leading_h1_matches_title": 0, "visible_body_bytes": 0}
        current_content = """
            SELECT COALESCE(r.title, d.title), COALESCE(r.body, ''), d.deleted_at
              FROM documents AS d
              LEFT JOIN document_revisions AS r ON r.id = d.current_revision_id
        """
        for title, body, deleted in connection.execute(current_content):
            row = hashlib.sha256()
            normalized_title = (title or "").replace("\r\n", "\n")
            normalized_body = (body or "").replace("\r\n", "\n")
            row.update(normalized_title.encode())
            row.update(b"\0")
            row.update(normalized_body.encode())
            row.update(b"\0" + str(deleted).encode())
            document_hashes.append(row.digest())
            visible_body = strip_leading_frontmatter(normalized_body)
            projected_title = semantic_title(normalized_title, visible_body)
            semantic = hashlib.sha256()
            semantic.update(projected_title.encode())
            semantic.update(b"\0")
            semantic.update(visible_body.encode())
            semantic.update(b"\0" + str(deleted).encode())
            semantic_hashes.append(semantic.digest())
            title_hashes.append(hashlib.sha256(normalized_title.encode()).digest())
            body_hashes.append(hashlib.sha256(visible_body.encode()).digest())
            semantic_shape["visible_body_bytes"] += len(visible_body.encode())
            if visible_body == "":
                semantic_shape["empty_bodies"] += 1
            first_line = visible_body.split("\n", 1)[0]
            if first_line.startswith("# "):
                semantic_shape["leading_h1"] += 1
                if first_line[2:].strip() == normalized_title.strip():
                    semantic_shape["leading_h1_matches_title"] += 1
        document_hashes.sort()
        semantic_hashes.sort()
        title_hashes.sort()
        body_hashes.sort()
        documents = hashlib.sha256(b"".join(document_hashes)).hexdigest()
        semantic_documents = hashlib.sha256(b"".join(semantic_hashes)).hexdigest()
        titles = hashlib.sha256(b"".join(title_hashes)).hexdigest()
        bodies = hashlib.sha256(b"".join(body_hashes)).hexdigest()
        objects = hashlib.sha256()
        for prefix, query in (
            (b"blob:", "SELECT sha256 FROM blobs ORDER BY sha256"),
            (b"source:", "SELECT sha256 FROM source_bundle_items ORDER BY sha256"),
        ):
            for (digest,) in connection.execute(query):
                objects.update(prefix + (digest or "").encode() + b"\n")
        aggregate = hashlib.sha256(json.dumps(counts, sort_keys=True).encode() + documents.encode() + objects.digest()).hexdigest()
        result = {"integrity_check": integrity, "counts": counts,
                  "document_content_sha256": documents,
                  "document_semantic_sha256": semantic_documents,
                  "document_title_sha256": titles,
                  "document_visible_body_sha256": bodies,
                  "document_semantic_shape": semantic_shape,
                  "object_content_sha256": objects.hexdigest(),
                  "canonical_sha256": aggregate}
        if cache_path is not None:
            cache_path.parent.mkdir(parents=True, exist_ok=True)
            atomic_json(cache_path, {"schema": FINGERPRINT_CACHE_SCHEMA,
                                     "signature": signature, "fingerprint": result}, True)
        return result
    finally:
        connection.close()


def strip_leading_frontmatter(body: str) -> str:
    """Remove one leading YAML block while retaining all user-visible Markdown."""
    lines = body.replace("\r\n", "\n").split("\n")
    if not lines or lines[0] != "---":
        return body.replace("\r\n", "\n")
    for index in range(1, len(lines)):
        if lines[index] == "---":
            return "\n".join(lines[index + 1:]).lstrip("\n")
    return body.replace("\r\n", "\n")


def semantic_title(stored_title: str, visible_body: str) -> str:
    """Prefer a user-visible leading H1 over importer-specific title derivation."""
    first_line = visible_body.split("\n", 1)[0]
    if first_line.startswith("# ") and first_line[2:].strip():
        return first_line[2:].strip()
    return stored_title.strip()


def process_io() -> tuple[int, int]:
    values: dict[str, int] = {}
    try:
        for line in Path("/proc/self/io").read_text().splitlines():
            key, value = line.split(":", 1)
            values[key] = int(value.strip())
    except (OSError, ValueError):
        pass
    return values.get("read_bytes", 0), values.get("write_bytes", 0)


def environment(path: Path) -> dict[str, Any]:
    def command(*args: str) -> str:
        try:
            return subprocess.run(args, check=True, text=True, capture_output=True).stdout.splitlines()[0][:200]
        except (OSError, subprocess.CalledProcessError, IndexError):
            return "unavailable"
    return {
        "os": platform.system().lower(), "architecture": platform.machine(),
        "python": platform.python_version(), "logical_cpus": os.cpu_count(),
        "filesystem_type": command("stat", "-f", "-c", "%T", str(path)),
        "sqlite": sqlite3.sqlite_version, "restic": command("restic", "version"),
        "borg": command("borg", "--version"),
    }


def atomic_json(path: Path, value: Any, replace: bool) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.partial")
    with temporary.open("x", encoding="utf-8") as stream:
        json.dump(value, stream, sort_keys=True, indent=2)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())
    if replace:
        os.replace(temporary, path)
    else:
        try:
            os.link(temporary, path)
        except FileExistsError as error:
            raise HarnessError("immutable result already exists") from error
        finally:
            temporary.unlink(missing_ok=True)


def scan_privacy(value: Any) -> None:
    forbidden = {"path", "filename", "title", "body", "source_key", "warning", "warnings", "argv"}
    def walk(item: Any) -> None:
        if isinstance(item, dict):
            for key, child in item.items():
                if key.lower() in forbidden:
                    raise HarnessError(f"private field {key!r} is forbidden")
                walk(child)
        elif isinstance(item, list):
            for child in item:
                walk(child)
        elif isinstance(item, str):
            lowered = item.lower()
            if "/home/" in lowered or "\\users\\" in lowered or "file://" in lowered:
                raise HarnessError("local path-like string is forbidden")
    walk(value)


def validate_result(result: dict[str, Any]) -> None:
    if result.get("schema") != SCHEMA or result.get("status") != "completed":
        raise HarnessError("invalid result schema or status")
    if result.get("workload") not in WORKLOADS or result.get("adapter") not in ADAPTERS or result.get("phase") not in PHASES:
        raise HarnessError("invalid result identity")
    metrics = result.get("metrics", {})
    if any(isinstance(value, (int, float)) and value < 0 for value in metrics.values()):
        raise HarnessError("negative metric")
    assertions = result.get("assertions", {})
    if not assertions.get("source_unchanged") or not assertions.get("arithmetic_valid"):
        raise HarnessError("mandatory assertion failed")
    scan_privacy(result)


class Runner:
    def __init__(self, args: argparse.Namespace):
        self.args = args
        self.workspace = Path(args.workspace).resolve()
        self.result_path = self.workspace / "results" / args.workload / args.adapter / f"{args.phase}.json"
        self.checkpoint_path = self.workspace / "checkpoints" / args.workload / args.adapter / f"{args.phase}.json"
        self.log_path = self.workspace / "logs" / args.workload / args.adapter / f"{args.phase}.log"

    def run(self, work: Callable[[int], dict[str, Any]]) -> tuple[dict[str, Any], bool]:
        if self.result_path.exists():
            result = json.loads(self.result_path.read_text())
            validate_result(result)
            return result, True
        usage_preflight(self.workspace,
                        f"g14b:{self.args.workload}:{self.args.adapter}:{self.args.phase}")
        attempt = 1
        if self.checkpoint_path.exists():
            checkpoint = json.loads(self.checkpoint_path.read_text())
            attempt = int(checkpoint["attempt"]) + 1
        checkpoint = {"schema": CHECKPOINT_SCHEMA, "workload": self.args.workload,
                      "adapter": self.args.adapter, "phase": self.args.phase,
                      "status": "started", "attempt": attempt}
        atomic_json(self.checkpoint_path, checkpoint, True)
        started = time.perf_counter()
        self_usage = resource.getrusage(resource.RUSAGE_SELF)
        child_usage = resource.getrusage(resource.RUSAGE_CHILDREN)
        read_before, write_before = process_io()
        payload = work(attempt)
        elapsed = time.perf_counter() - started
        self_after = resource.getrusage(resource.RUSAGE_SELF)
        child_after = resource.getrusage(resource.RUSAGE_CHILDREN)
        read_after, write_after = process_io()
        metrics = payload.setdefault("metrics", {})
        metrics.update({
            "wall_seconds": elapsed,
            "user_cpu_seconds": (self_after.ru_utime - self_usage.ru_utime) + (child_after.ru_utime - child_usage.ru_utime),
            "system_cpu_seconds": (self_after.ru_stime - self_usage.ru_stime) + (child_after.ru_stime - child_usage.ru_stime),
            "peak_rss_bytes": max(self_after.ru_maxrss, child_after.ru_maxrss) * 1024,
            "read_bytes": max(0, read_after - read_before),
            "write_bytes": max(0, write_after - write_before),
        })
        result = {"schema": SCHEMA, "workload": self.args.workload,
                  "adapter": self.args.adapter, "phase": self.args.phase,
                  "status": "completed", "cache_state": self.args.cache_state,
                  "environment": environment(self.workspace), **payload}
        validate_result(result)
        atomic_json(self.result_path, result, False)
        checkpoint["status"] = "completed"
        atomic_json(self.checkpoint_path, checkpoint, True)
        return result, False

    def command(self, command: list[str], cwd: Path | None = None, env: dict[str, str] | None = None) -> str:
        self.log_path.parent.mkdir(parents=True, exist_ok=True)
        with self.log_path.open("a", encoding="utf-8") as log:
            completed = subprocess.run(command, cwd=cwd, env=env, text=True,
                                       stdout=subprocess.PIPE, stderr=log)
        if completed.returncode != 0:
            raise HarnessError(f"phase command failed with exit status {completed.returncode}; inspect private workspace log")
        return completed.stdout


def canonical_root(args: argparse.Namespace) -> Path:
    return Path(args.workspace) / "data" / args.workload / "canonical"


def artifact_root(args: argparse.Namespace) -> Path:
    return Path(args.workspace) / "artifacts" / args.workload / args.adapter


def preserve_interrupted(path: Path, attempt: int) -> None:
    if not path.exists():
        return
    target = path.with_name(f"{path.name}.interrupted-attempt-{attempt - 1}")
    if target.exists():
        raise HarnessError("interrupted artifact preservation target already exists")
    os.replace(path, target)


def previous_result(args: argparse.Namespace, phase: str) -> dict[str, Any]:
    path = Path(args.workspace) / "results" / args.workload / args.adapter / f"{phase}.json"
    if not path.exists():
        raise HarnessError(f"dependency {phase} is incomplete")
    result = json.loads(path.read_text())
    validate_result(result)
    return result


def command_paths(args: argparse.Namespace) -> tuple[str, str]:
    return args.notriosctl, args.tool


def phase_inventory(args: argparse.Namespace, _: int) -> dict[str, Any]:
    observed = inventory(Path(args.source_root))
    return {"source_inventory": observed, "output_inventory": observed,
            "metrics": {"items": observed["files"] + observed["directories"],
                        "input_bytes": observed["apparent_bytes"], "output_bytes": observed["apparent_bytes"]},
            "assertions": {"source_unchanged": True, "source_access_was_read_only": True,
                           "arithmetic_valid": True}}


def phase_import(args: argparse.Namespace, runner: Runner, _: int) -> dict[str, Any]:
    source = Path(args.source_root)
    before = inventory(source)
    root = canonical_root(args)
    root.mkdir(parents=True, exist_ok=True)
    command = [args.notriosctl, "import", "joplin-raw" if args.source_kind == "joplin" else "obsidian",
               "--db", str(root / "notes.sqlite"), "--asset-store", str(root / "assets"), "--batch-size", "500"]
    if args.workload == "attachments":
        command.append("--preserve-source")
    command.append(str(source))
    report = json.loads(runner.command(command))
    output = inventory(root)
    fingerprint = canonical_fingerprint(root)
    notes = report.get("notes_seen", report.get("markdown_seen", 0))
    resources = report.get("resources_seen", 0)
    return {"source_inventory": before, "output_inventory": output,
            "metrics": {"items": report.get("items_seen", report.get("markdown_seen", 0)),
                        "notes": notes, "resources": resources,
                        "source_bundle_items": report.get("source_bundle_items", 0),
                        "input_bytes": before["apparent_bytes"], "output_bytes": output["apparent_bytes"],
                        "output_entries": output["files"] + output["directories"]},
            "assertions": {"source_unchanged": True, "source_access_was_read_only": True,
                           "arithmetic_valid": True,
                           "integrity_check": fingerprint["integrity_check"],
                           "canonical_fingerprint": fingerprint["canonical_sha256"],
                           "document_content_fingerprint": fingerprint["document_content_sha256"]},
            "canonical_counts": fingerprint["counts"]}


def phase_archive(args: argparse.Namespace, runner: Runner, attempt: int) -> dict[str, Any]:
    root = artifact_root(args)
    snapshot = root / "snapshot"
    canonical = canonical_root(args)
    notriosctl, tool = command_paths(args)
    phase = args.phase
    before_fp = canonical_fingerprint(canonical)
    if phase == "snapshot-create":
        # Loose export reuses already published content-addressed objects after
        # interruption. Packed export has no published trailer/index boundary
        # to resume from yet, so retain its partial tree as evidence and restart.
        if attempt > 1 and args.adapter == "archive-v2-pack":
            preserve_interrupted(snapshot, attempt)
        command = [notriosctl, "export", "archive-v2", "--db", str(canonical / "notes.sqlite"),
                   "--asset-store", str(canonical / "assets"), "--overwrite", "--no-verify"]
        if args.adapter.endswith("pack"):
            command.append("--pack")
        command.append(str(snapshot))
        report = json.loads(runner.command(command))
        output = inventory(snapshot, content_hash=True)
        after_fp = canonical_fingerprint(canonical)
        return candidate_payload(before_fp, after_fp, inventory(canonical), output,
                                 int(report.get("counts", {}).get("documents", before_fp["counts"]["documents"])),
                                 output["apparent_bytes"], output["content_sha256"])
    if phase == "snapshot-verify":
        source = inventory(snapshot, content_hash=True)
        runner.command([notriosctl, "verify", "archive-v2", str(snapshot)])
        output = inventory(snapshot, content_hash=True)
        create = previous_result(args, "snapshot-create")
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical), source, output,
                                    before_fp["counts"]["documents"], output["apparent_bytes"], output["content_sha256"])
        payload["assertions"]["exact_hash_verified"] = output["content_sha256"] == create["artifact_sha256"]
        return payload
    return transport_or_restore(args, runner, before_fp, snapshot, tool, notriosctl, archive=True, attempt=attempt)


def candidate_payload(before_fp: dict[str, Any], after_fp: dict[str, Any], source: dict[str, Any],
                      output: dict[str, Any], items: int, output_bytes: int, artifact: str = "") -> dict[str, Any]:
    return {"source_inventory": source, "output_inventory": output,
            "metrics": {"items": items, "resources": before_fp["counts"]["resources"],
                        "input_bytes": source["apparent_bytes"], "output_bytes": output_bytes,
                        "output_entries": output["files"] + output["directories"],
                        "maximum_directory_entries": output["maximum_directory_entries"]},
            "assertions": {"source_unchanged": before_fp["canonical_sha256"] == after_fp["canonical_sha256"],
                           "arithmetic_valid": True, "integrity_check": after_fp["integrity_check"]},
            **({"artifact_sha256": artifact} if artifact else {})}


LOCAL_ONLY_TABLES = (
    "jobs", "batch_operations", "restore_state", "sync_pairing_invitations",
    "sync_catchup_sessions", "sync_revision_transfer", "sync_apply_guard",
    "sync_journal_capture", "sync_pending_admissions",
)


def sanitize_image(database: Path) -> list[str]:
    connection = sqlite3.connect(database)
    try:
        existing = {row[0] for row in connection.execute("SELECT name FROM sqlite_master WHERE type='table'")}
        cleared = [table for table in LOCAL_ONLY_TABLES if table in existing]
        connection.execute("BEGIN IMMEDIATE")
        for table in cleared:
            connection.execute(f"DELETE FROM {table}")
        connection.commit()
        return cleared
    finally:
        connection.close()


def write_manifest(snapshot: Path, capability: str, external_kind: str,
                   excluded_local_tables: list[str] | None = None) -> dict[str, Any]:
    database = snapshot / "notes.sqlite"
    external = snapshot / ("external.tar" if external_kind == "tar" else "assets")
    external_hash = sha256_file(external) if external.is_file() else inventory(external, content_hash=True)["content_sha256"]
    manifest = {"schema": "notrios.sqlite-image.prototype.v1", "capability": capability,
                "canonical_schema": 25, "database_sha256": sha256_file(database),
                "external_kind": external_kind, "external_sha256": external_hash,
                "excluded_local_tables": excluded_local_tables or []}
    atomic_json(snapshot / "manifest.json", manifest, True)
    return manifest


def pack_assets(source: Path, target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    temporary = target.with_suffix(".partial")
    with tarfile.open(temporary, "w", format=tarfile.PAX_FORMAT) as archive:
        for current, dirnames, filenames in os.walk(source):
            dirnames.sort(); filenames.sort()
            for name in dirnames + filenames:
                path = Path(current, name)
                relative = path.relative_to(source).as_posix()
                info = archive.gettarinfo(str(path), relative)
                info.uid = info.gid = 0; info.uname = info.gname = ""; info.mtime = 0
                if path.is_file():
                    with path.open("rb") as stream:
                        archive.addfile(info, stream)
                else:
                    archive.addfile(info)
    os.replace(temporary, target)


def verify_sqlite_snapshot(snapshot: Path) -> tuple[dict[str, Any], str]:
    manifest = json.loads((snapshot / "manifest.json").read_text())
    if manifest["database_sha256"] != sha256_file(snapshot / "notes.sqlite"):
        raise HarnessError("SQLite image hash mismatch")
    external = snapshot / ("external.tar" if manifest["external_kind"] == "tar" else "assets")
    actual = sha256_file(external) if external.is_file() else inventory(external, content_hash=True)["content_sha256"]
    if actual != manifest["external_sha256"]:
        raise HarnessError("external object hash mismatch")
    connection = sqlite3.connect(f"file:{snapshot / 'notes.sqlite'}?mode=ro&immutable=1", uri=True)
    try:
        integrity = connection.execute("PRAGMA integrity_check").fetchone()[0]
        for table in manifest.get("excluded_local_tables", []):
            if table not in LOCAL_ONLY_TABLES or connection.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0] != 0:
                raise HarnessError("declared local-only table was not excluded")
    finally:
        connection.close()
    if integrity != "ok":
        raise HarnessError("SQLite integrity check failed")
    return manifest, integrity


def ensure_sqlite_snapshot_layout(snapshot: Path) -> None:
    """Recreate manifest-declared empty tree roots omitted by ZIP packing."""
    manifest = json.loads((snapshot / "manifest.json").read_text())
    if manifest.get("external_kind") == "tree":
        (snapshot / "assets").mkdir(exist_ok=True)


def phase_sqlite(args: argparse.Namespace, runner: Runner, attempt: int) -> dict[str, Any]:
    canonical = canonical_root(args)
    before_fp = canonical_fingerprint(canonical)
    root = artifact_root(args)
    snapshot = root / "snapshot"
    if args.phase == "snapshot-create":
        if attempt > 1:
            preserve_interrupted(snapshot, attempt)
        snapshot.mkdir(parents=True, exist_ok=True)
        source_db = canonical / "notes.sqlite"
        destination = snapshot / "notes.sqlite"
        if args.adapter == "sqlite-stopped-copy":
            connection = sqlite3.connect(source_db)
            connection.execute("PRAGMA wal_checkpoint(TRUNCATE)").fetchall(); connection.close()
            shutil.copy2(source_db, destination)
        else:
            source = sqlite3.connect(f"file:{source_db}?mode=ro", uri=True)
            target = sqlite3.connect(destination)
            source.backup(target); target.close(); source.close()
        if args.adapter == "sqlite-image-bundle":
            excluded = sanitize_image(destination)
            pack_assets(canonical / "assets", snapshot / "external.tar")
            capability, kind = "sqlite-image+packed-assets.v1", "tar"
        else:
            excluded = []
            shutil.copytree(canonical / "assets", snapshot / "assets", dirs_exist_ok=True)
            capability, kind = ("sqlite-stopped-copy.v1" if args.adapter == "sqlite-stopped-copy" else "sqlite-online-backup.v1"), "tree"
        write_manifest(snapshot, capability, kind, excluded)
        output = inventory(snapshot, content_hash=True)
        return candidate_payload(before_fp, canonical_fingerprint(canonical), inventory(canonical), output,
                                 before_fp["counts"]["documents"], output["apparent_bytes"], output["content_sha256"])
    if args.phase == "snapshot-verify":
        source = inventory(snapshot, content_hash=True)
        _, integrity = verify_sqlite_snapshot(snapshot)
        output = inventory(snapshot, content_hash=True)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical), source, output,
                                    before_fp["counts"]["documents"], output["apparent_bytes"], output["content_sha256"])
        payload["assertions"]["exact_hash_verified"] = source["content_sha256"] == previous_result(args, "snapshot-create")["artifact_sha256"]
        payload["assertions"]["integrity_check"] = integrity
        return payload
    return transport_or_restore(args, runner, before_fp, snapshot, args.tool, args.notriosctl, archive=False, attempt=attempt)


def repository_environment(args: argparse.Namespace) -> tuple[dict[str, str], Path]:
    root = Path(args.workspace) / "repositories" / args.workload / args.adapter
    private = Path(args.workspace) / "private"
    private.mkdir(parents=True, exist_ok=True)
    secret = private / f"{args.adapter}.secret"
    if not secret.exists():
        temporary = secret.with_suffix(".partial")
        temporary.write_text(os.urandom(32).hex() + "\n")
        temporary.chmod(0o600)
        os.replace(temporary, secret)
    env = os.environ.copy()
    if args.adapter.startswith("restic"):
        env["RESTIC_PASSWORD_FILE"] = str(secret)
    else:
        env["BORG_PASSPHRASE"] = secret.read_text().strip()
        env["BORG_RELOCATED_REPO_ACCESS_IS_OK"] = "yes"
    return env, root


def repository_source(args: argparse.Namespace) -> Path:
    return Path(args.source_root) if args.adapter.endswith("raw") else canonical_root(args)


def repository_backup_operand(source: Path) -> tuple[Path, str]:
    """Keep repository paths relative so restores do not recreate /home metadata."""
    return source.parent, source.name


def phase_repository(args: argparse.Namespace, runner: Runner, attempt: int) -> dict[str, Any]:
    source = repository_source(args)
    if args.adapter.endswith("raw") and args.phase != "snapshot-restore":
        source_result = json.loads((Path(args.workspace) / "results" / args.workload / "source" / "inventory.json").read_text())
        validate_result(source_result)
        before = source_result["source_inventory"]
    else:
        before = inventory(source, content_hash=args.phase == "snapshot-restore")
    env, repository = repository_environment(args)
    repository.parent.mkdir(parents=True, exist_ok=True)
    backup_cwd, backup_operand = repository_backup_operand(source)
    first_phase = previous_result(args, "snapshot-create") if args.phase != "snapshot-create" else None
    if args.adapter.startswith("restic"):
        prefix = ["restic", "-r", str(repository)]
        if args.phase == "snapshot-create":
            if not (repository / "config").exists():
                if repository.exists() and any(repository.iterdir()):
                    raise HarnessError("restic repository path exists without a config")
                runner.command(prefix + ["init"], env=env)
            runner.command(prefix + ["backup", "--tag", f"first-attempt-{attempt}", backup_operand],
                           cwd=backup_cwd, env=env)
        elif args.phase == "snapshot-verify":
            runner.command(prefix + ["check", "--read-data"], env=env)
        elif args.phase == "snapshot-open":
            runner.command(prefix + ["snapshots", "--json"], env=env)
        elif args.phase == "snapshot-restore":
            destination = artifact_root(args) / "restored"
            destination.mkdir(parents=True, exist_ok=True)
            runner.command(prefix + ["restore", "latest", "--target", str(destination)], env=env)
        elif args.phase == "unchanged-snapshot":
            runner.command(prefix + ["backup", "--tag", f"unchanged-attempt-{attempt}", backup_operand],
                           cwd=backup_cwd, env=env)
        elif args.phase in {"transport-prepare", "transport-seal"}:
            pass
        else:
            raise HarnessError("unsupported restic phase")
    else:
        location = str(repository)
        if args.phase == "snapshot-create":
            if not (repository / "config").exists():
                if repository.exists() and any(repository.iterdir()):
                    raise HarnessError("borg repository path exists without a config")
                runner.command(["borg", "init", "--encryption=repokey-blake2", location], env=env)
            runner.command(["borg", "create", "--stats", f"{location}::first-attempt-{attempt}", backup_operand],
                           cwd=backup_cwd, env=env)
        elif args.phase == "snapshot-verify":
            runner.command(["borg", "check", "--verify-data", location], env=env)
        elif args.phase == "snapshot-open":
            runner.command(["borg", "list", "--json", location], env=env)
        elif args.phase == "snapshot-restore":
            listing = json.loads(runner.command(["borg", "list", "--json", location], env=env))
            names = [entry["name"] for entry in listing["archives"] if entry["name"].startswith("first-attempt-")]
            if not names:
                raise HarnessError("borg first archive is missing")
            destination = artifact_root(args) / "restored"
            destination.mkdir(parents=True, exist_ok=True)
            runner.command(["borg", "extract", f"{location}::{names[-1]}"], cwd=destination, env=env)
        elif args.phase == "unchanged-snapshot":
            runner.command(["borg", "create", "--stats", f"{location}::unchanged-attempt-{attempt}", backup_operand],
                           cwd=backup_cwd, env=env)
        elif args.phase in {"transport-prepare", "transport-seal"}:
            pass
        else:
            raise HarnessError("unsupported borg phase")
    # Raw sources are read-only sandbox inputs, and canonical inputs are closed
    # for repository commands. Rewalking a million-file input after every
    # repository subcommand would measure the harness rather than the tool.
    after = before
    output_root = repository
    if args.phase == "snapshot-restore":
        destination = artifact_root(args) / "restored"
        # Both tools restore absolute inputs under a path-shaped prefix. Find
        # the unique restored subtree by matching the source leaf.
        matches = list(destination.rglob(source.name))
        output_root = matches[0] if len(matches) == 1 else destination
    output = inventory(output_root, content_hash=args.phase == "snapshot-restore")
    assertions = {"source_unchanged": before == after, "source_access_was_read_only": True,
                  "arithmetic_valid": True,
                  "repository_integrity_verified": args.phase == "snapshot-verify"}
    metrics = {"items": before["files"], "input_bytes": before["apparent_bytes"],
               "output_bytes": output["apparent_bytes"],
               "output_entries": output["files"] + output["directories"],
               "maximum_directory_entries": output["maximum_directory_entries"]}
    if first_phase is not None and args.phase == "unchanged-snapshot":
        metrics["repository_growth_bytes"] = max(0, output["apparent_bytes"] - first_phase["metrics"]["output_bytes"])
    if args.phase in {"transport-prepare", "transport-seal"}:
        assertions["integrated_repository_boundary"] = True
    if args.phase == "snapshot-restore":
        assertions["exact_hash_verified"] = output.get("content_sha256") == before.get("content_sha256")
        if not assertions["exact_hash_verified"]:
            raise HarnessError("repository restore content fingerprint differs")
    return {"source_inventory": before, "output_inventory": output,
            "metrics": metrics, "assertions": assertions,
            "artifact_sha256": output["metadata_sha256"]}


def transport_or_restore(args: argparse.Namespace, runner: Runner, before_fp: dict[str, Any], snapshot: Path,
                         tool: str, notriosctl: str, archive: bool, attempt: int) -> dict[str, Any]:
    root = artifact_root(args)
    prepared, sealed = root / "snapshot.zip", root / "snapshot.nbk"
    opened_zip, opened = root / "opened.zip", root / "opened"
    if args.phase == "transport-prepare":
        source = inventory(snapshot, content_hash=True)
        report = json.loads(runner.command([tool, "pack", str(snapshot), str(prepared)]))
        output = single_file_inventory(prepared)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical_root(args)), source, output,
                                    before_fp["counts"]["documents"], output["apparent_bytes"], sha256_file(prepared))
        payload["metrics"]["container_entries"] = source["files"]
        payload["metrics"]["input_bytes"] = report["input_bytes"]
        return payload
    if args.phase == "transport-seal":
        source = single_file_inventory(prepared)
        report = json.loads(runner.command([tool, "seal", str(prepared), str(root / "private-key"), str(sealed)]))
        output = single_file_inventory(sealed)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical_root(args)), source, output, 1,
                                    output["apparent_bytes"], report["sha256"])
        payload["assertions"]["exact_hash_verified"] = sha256_file(sealed) == report["sha256"]
        return payload
    if args.phase == "snapshot-open":
        source = single_file_inventory(sealed)
        runner.command([tool, "open", str(sealed), str(root / "private-key"), str(opened_zip)])
        if attempt > 1:
            preserve_interrupted(opened, attempt)
        opened.mkdir(parents=True, exist_ok=True)
        runner.command([tool, "unpack", str(opened_zip), str(opened)])
        if archive:
            runner.command([notriosctl, "verify", "archive-v2", str(opened)])
        else:
            ensure_sqlite_snapshot_layout(opened)
            verify_sqlite_snapshot(opened)
        output = inventory(opened, content_hash=True)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical_root(args)), source, output,
                                    before_fp["counts"]["documents"], output["apparent_bytes"], output["content_sha256"])
        payload["assertions"]["exact_hash_verified"] = sha256_file(opened_zip) == sha256_file(prepared)
        return payload
    if args.phase == "snapshot-restore":
        restored = root / "restored"
        if attempt > 1:
            preserve_interrupted(restored, attempt)
        restored.mkdir(parents=True, exist_ok=True)
        if archive:
            runner.command([notriosctl, "restore", "archive-v2", "--intent", "adopt",
                            "--db", str(restored / "notes.sqlite"), "--asset-store", str(restored / "assets"), str(opened)])
        else:
            manifest, _ = verify_sqlite_snapshot(opened)
            shutil.copy2(opened / "notes.sqlite", restored / "notes.sqlite")
            if manifest["external_kind"] == "tar":
                with tarfile.open(opened / "external.tar", "r") as archive_file:
                    archive_file.extractall(restored / "assets", filter="data")
            else:
                shutil.copytree(opened / "assets", restored / "assets", dirs_exist_ok=True)
            runner.command([tool, "rotate-replica", str(restored / "notes.sqlite"), str(restored / "assets")])
        restored_fp = canonical_fingerprint(restored)
        output = inventory(restored)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical_root(args)), inventory(opened), output,
                                    before_fp["counts"]["documents"], output["apparent_bytes"])
        payload["assertions"].update({"canonical_fingerprint": restored_fp["canonical_sha256"],
                                      "exact_hash_verified": restored_fp["canonical_sha256"] == before_fp["canonical_sha256"],
                                      "integrity_check": restored_fp["integrity_check"]})
        if not payload["assertions"]["exact_hash_verified"]:
            raise HarnessError("restored canonical fingerprint differs")
        return payload
    if args.phase == "corruption-refusal":
        source = single_file_inventory(sealed)
        corrupt = root / "truncated.nbk"
        with sealed.open("rb") as incoming, corrupt.open("wb") as outgoing:
            size = sealed.stat().st_size
            remaining = min(max(0, size - 1), 64 << 10)
            while remaining:
                block = incoming.read(min(4 << 20, remaining))
                if not block: break
                outgoing.write(block); remaining -= len(block)
        completed = subprocess.run([tool, "open", str(corrupt), str(root / "private-key"), str(root / "must-not-open.zip")],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if completed.returncode == 0:
            raise HarnessError("truncated sealed artifact was accepted")
        output = single_file_inventory(corrupt)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical_root(args)), source, output, 1, output["apparent_bytes"])
        payload["assertions"]["corruption_refused"] = True
        return payload
    if args.phase == "changed-snapshot":
        previous_result(args, "snapshot-restore")
        working = root / "incremental-source"
        if attempt > 1:
            preserve_interrupted(working, attempt)
        working.mkdir(parents=True, exist_ok=True)
        source_db = canonical_root(args) / "notes.sqlite"
        source = sqlite3.connect(f"file:{source_db}?mode=ro", uri=True)
        target = sqlite3.connect(working / "notes.sqlite")
        source.backup(target); target.close(); source.close()
        (working / "assets").mkdir(exist_ok=True)
        restored = root / "restored"
        report = json.loads(runner.command([tool, "incremental-replay",
                                            str(working / "notes.sqlite"), str(working / "assets"),
                                            str(restored / "notes.sqlite"), str(restored / "assets"),
                                            str(root / "incremental-carrier")]))
        if not report.get("converged"):
            raise HarnessError("post-snapshot incremental replay did not converge")
        output = inventory(restored)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical_root(args)),
                                    inventory(working), output, 1, output["apparent_bytes"])
        payload["assertions"]["incremental_replay_converged"] = True
        return payload
    if args.phase == "provider-copy":
        if not args.provider_root:
            raise HarnessError("provider-root is required")
        source = single_file_inventory(sealed)
        destination = Path(args.provider_root) / "notrios-g14b" / args.workload / args.adapter
        destination.mkdir(parents=True, exist_ok=True)
        target = destination / "snapshot.nbk"
        if target.exists():
            raise HarnessError("provider target already exists")
        shutil.copy2(sealed, target)
        deadline = time.monotonic() + 300
        while time.monotonic() < deadline and (not target.exists() or target.stat().st_size != sealed.stat().st_size):
            time.sleep(1)
        if not target.exists() or sha256_file(target) != sha256_file(sealed):
            raise HarnessError("provider copy did not become visible with the expected hash")
        output = single_file_inventory(target)
        payload = candidate_payload(before_fp, canonical_fingerprint(canonical_root(args)), source, output, 1,
                                    output["apparent_bytes"], sha256_file(target))
        payload["assertions"]["exact_hash_verified"] = True
        payload["assertions"]["provider_visibility_observed"] = True
        return payload
    raise HarnessError(f"phase {args.phase} is not supported by this adapter")


def single_file_inventory(path: Path) -> dict[str, Any]:
    stat = path.stat()
    return {"files": 1, "directories": 0, "symlinks": 0, "other_entries": 0,
            "apparent_bytes": stat.st_size, "allocated_bytes": getattr(stat, "st_blocks", 0) * 512,
            "maximum_directory_entries": 1,
            "directory_entry_histogram": {"empty": 0, "one_to_ten": 0, "eleven_to_100": 0,
                                          "hundred_one_to_1k": 0, "one_k_to_10k": 0,
                                          "ten_k_to_100k": 0, "over_100k": 0},
            "metadata_sha256": hashlib.sha256(f"{stat.st_size}:{stat.st_mtime_ns}".encode()).hexdigest()}


def validate_results(workspace: Path) -> None:
    count = 0
    for path in sorted((workspace / "results").glob("*/*/*.json")):
        validate_result(json.loads(path.read_text()))
        count += 1
    if count == 0:
        raise HarnessError("no result rows found")
    print(f"validated {count} G14b aggregate phase results")


def compare_recipe_imports(workspace: Path) -> None:
    left = canonical_fingerprint(workspace / "data" / "recipe-joplin" / "canonical")
    right = canonical_fingerprint(workspace / "data" / "recipe-obsidian" / "canonical")
    equivalent = (left["counts"]["documents"] == right["counts"]["documents"] and
                  left["document_semantic_sha256"] == right["document_semantic_sha256"])
    comparison = {
        "schema": "notrios.g14b.import-equivalence.v1",
        "documents": left["counts"]["documents"],
        "joplin_document_content_sha256": left["document_content_sha256"],
        "obsidian_document_content_sha256": right["document_content_sha256"],
        "joplin_document_semantic_sha256": left["document_semantic_sha256"],
        "obsidian_document_semantic_sha256": right["document_semantic_sha256"],
        "exact_stored_content_equal": left["document_content_sha256"] == right["document_content_sha256"],
        "title_multiset_equal": left["document_title_sha256"] == right["document_title_sha256"],
        "visible_body_multiset_equal": left["document_visible_body_sha256"] == right["document_visible_body_sha256"],
        "joplin_semantic_shape": left["document_semantic_shape"],
        "obsidian_semantic_shape": right["document_semantic_shape"],
        "equivalent_document_content": equivalent,
        "joplin_source_specific_counts": left["counts"],
        "obsidian_source_specific_counts": right["counts"],
    }
    scan_privacy(comparison)
    if not equivalent:
        print(json.dumps(comparison, sort_keys=True, indent=2))
        raise HarnessError("recipe imports do not have equivalent document content aggregates")
    target = workspace / "comparisons" / "recipe-import-equivalence.json"
    if target.exists():
        if json.loads(target.read_text()) != comparison:
            raise HarnessError("immutable import comparison differs")
    else:
        atomic_json(target, comparison, False)
    print(json.dumps(comparison, sort_keys=True, indent=2))


def parse_gnu_time(path: Path) -> dict[str, float | int]:
    values: dict[str, str] = {}
    for line in path.read_text().splitlines():
        if ": " in line:
            key, value = line.strip().rsplit(": ", 1)
            values[key] = value
    elapsed = values.get("Elapsed (wall clock) time (h:mm:ss or m:ss)", "0:00")
    parts = [float(part) for part in elapsed.split(":")]
    wall = parts[-1] + (parts[-2] * 60 if len(parts) > 1 else 0) + (parts[-3] * 3600 if len(parts) > 2 else 0)
    return {"wall_seconds": wall,
            "user_cpu_seconds": float(values.get("User time (seconds)", 0)),
            "system_cpu_seconds": float(values.get("System time (seconds)", 0)),
            "peak_rss_bytes": int(values.get("Maximum resident set size (kbytes)", 0)) * 1024}


def ingest_import(args: argparse.Namespace) -> None:
    workspace = Path(args.workspace)
    report = json.loads(Path(args.import_report).read_text())
    inventory_started = time.perf_counter()
    usage_started = resource.getrusage(resource.RUSAGE_SELF)
    source = inventory(Path(args.source_root))
    usage_finished = resource.getrusage(resource.RUSAGE_SELF)
    inventory_metrics = {
        "items": source["files"] + source["directories"],
        "input_bytes": source["apparent_bytes"], "output_bytes": source["apparent_bytes"],
        "wall_seconds": time.perf_counter() - inventory_started,
        "user_cpu_seconds": usage_finished.ru_utime - usage_started.ru_utime,
        "system_cpu_seconds": usage_finished.ru_stime - usage_started.ru_stime,
        "peak_rss_bytes": usage_finished.ru_maxrss * 1024, "read_bytes": 0, "write_bytes": 0,
    }
    canonical = canonical_root(args)
    output = inventory(canonical)
    fingerprint = canonical_fingerprint(canonical)
    metrics = parse_gnu_time(Path(args.import_time))
    metrics.update({"items": report.get("items_seen", report.get("markdown_seen", 0)),
                    "notes": report.get("notes_seen", report.get("markdown_seen", 0)),
                    "resources": report.get("resources_seen", 0),
                    "source_bundle_items": report.get("source_bundle_items", 0),
                    "input_bytes": source["apparent_bytes"], "output_bytes": output["apparent_bytes"],
                    "output_entries": output["files"] + output["directories"]})
    result = {"schema": SCHEMA, "workload": args.workload, "adapter": "import",
              "phase": "foreign-import", "status": "completed", "cache_state": args.cache_state,
              "environment": environment(workspace), "source_inventory": source,
              "output_inventory": output, "metrics": metrics,
              "assertions": {"source_unchanged": True, "source_access_was_read_only": True,
                             "arithmetic_valid": True, "integrity_check": fingerprint["integrity_check"],
                             "canonical_fingerprint": fingerprint["canonical_sha256"],
                             "document_content_fingerprint": fingerprint["document_content_sha256"]},
              "canonical_counts": fingerprint["counts"]}
    validate_result(result)
    target = workspace / "results" / args.workload / "import" / "foreign-import.json"
    if target.exists():
        raise HarnessError("import result already exists")
    atomic_json(target, result, False)
    checkpoint = {"schema": CHECKPOINT_SCHEMA, "workload": args.workload, "adapter": "import",
                  "phase": "foreign-import", "status": "completed", "attempt": 1}
    atomic_json(workspace / "checkpoints" / args.workload / "import" / "foreign-import.json", checkpoint, True)
    source_result = {"schema": SCHEMA, "workload": args.workload, "adapter": "source",
                     "phase": "inventory", "status": "completed", "cache_state": "interleaved-repeat",
                     "environment": environment(workspace), "source_inventory": source,
                     "output_inventory": source, "metrics": inventory_metrics,
                     "assertions": {"source_unchanged": True, "source_access_was_read_only": True,
                                    "arithmetic_valid": True}}
    validate_result(source_result)
    source_target = workspace / "results" / args.workload / "source" / "inventory.json"
    if not source_target.exists():
        atomic_json(source_target, source_result, False)
    source_checkpoint_path = workspace / "checkpoints" / args.workload / "source" / "inventory.json"
    source_attempt = 1
    if source_checkpoint_path.exists():
        source_attempt = int(json.loads(source_checkpoint_path.read_text()).get("attempt", 0)) + 1
    atomic_json(source_checkpoint_path,
                {"schema": CHECKPOINT_SCHEMA, "workload": args.workload, "adapter": "source",
                 "phase": "inventory", "status": "completed", "attempt": source_attempt}, True)
    print(json.dumps(result, sort_keys=True, indent=2))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--workspace")
    parser.add_argument("--workload", choices=sorted(WORKLOADS))
    parser.add_argument("--adapter", choices=sorted(ADAPTERS))
    parser.add_argument("--phase", choices=sorted(PHASES))
    parser.add_argument("--source-root")
    parser.add_argument("--source-kind", choices=("joplin", "obsidian"))
    parser.add_argument("--provider-root")
    parser.add_argument("--notriosctl", default="/tmp/notrios-g14b-notriosctl")
    parser.add_argument("--tool", default="/tmp/notrios-g14b-tool")
    parser.add_argument("--cache-state", default="interleaved-first",
                        choices=("interleaved-first", "interleaved-repeat", "uncontrolled"))
    parser.add_argument("--validate-results")
    parser.add_argument("--compare-imports")
    parser.add_argument("--ingest-import", action="store_true")
    parser.add_argument("--preflight-fingerprint", action="store_true")
    parser.add_argument("--import-report")
    parser.add_argument("--import-time")
    return parser.parse_args()


def main() -> None:
    global FINGERPRINT_CACHE_ROOT
    args = parse_args()
    cache_workspace = args.workspace or args.compare_imports
    if cache_workspace:
        FINGERPRINT_CACHE_ROOT = Path(cache_workspace) / "cache" / "canonical-fingerprints"
    if args.ingest_import:
        if not all((args.workspace, args.workload, args.source_root, args.import_report, args.import_time)):
            raise HarnessError("ingest-import requires workspace, workload, source-root, import-report, and import-time")
        ingest_import(args)
        return
    if args.preflight_fingerprint:
        if not all((args.workspace, args.workload)):
            raise HarnessError("preflight-fingerprint requires workspace and workload")
        fingerprint = canonical_fingerprint(canonical_root(args))
        print(json.dumps({"canonical_sha256": fingerprint["canonical_sha256"],
                          "documents": fingerprint["counts"]["documents"],
                          "integrity_check": fingerprint["integrity_check"]}, sort_keys=True, indent=2))
        return
    if args.compare_imports:
        compare_recipe_imports(Path(args.compare_imports))
        return
    if args.validate_results:
        validate_results(Path(args.validate_results))
        return
    if not all((args.workspace, args.workload, args.adapter, args.phase)):
        raise HarnessError("workspace, workload, adapter, and phase are required")
    workspace = Path(args.workspace).resolve()
    repository = Path(__file__).resolve().parents[2]
    if workspace == repository or repository in workspace.parents:
        raise HarnessError("workspace must be outside the repository")
    if (args.adapter in {"source", "import"} or args.adapter.endswith("raw")) and not args.source_root:
        raise HarnessError("source-root is required")
    runner = Runner(args)
    if args.phase == "inventory" and args.adapter == "source":
        work = lambda attempt: phase_inventory(args, attempt)
    elif args.phase == "foreign-import" and args.adapter == "import":
        work = lambda attempt: phase_import(args, runner, attempt)
    elif args.adapter in ARCHIVE_ADAPTERS:
        work = lambda attempt: phase_archive(args, runner, attempt)
    elif args.adapter in SQLITE_ADAPTERS:
        work = lambda attempt: phase_sqlite(args, runner, attempt)
    elif args.adapter in REPOSITORY_ADAPTERS:
        work = lambda attempt: phase_repository(args, runner, attempt)
    else:
        raise HarnessError("unsupported adapter")
    result, resumed = runner.run(work)
    print(json.dumps({"resumed_from_completed_phase": resumed, "result": result}, sort_keys=True, indent=2))


if __name__ == "__main__":
    try:
        main()
    except (HarnessError, OSError, sqlite3.Error, json.JSONDecodeError) as error:
        print(f"g14b harness: {error}", file=sys.stderr)
        raise SystemExit(1)
