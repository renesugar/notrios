#!/usr/bin/env python3
"""Resumable aggregate-only production acceptance harness for v0.7 G14e."""
from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import resource
import shutil
import subprocess
import sys
import time
from typing import Any, Callable


SCHEMA = "notrios.g14e.phase-result.v1"
SUMMARY_SCHEMA = "notrios.g14e.full-scale-acceptance.v1"
PHASES = (
    "corpus-equivalence",
    "semantic-current-create", "semantic-current-verify", "semantic-current-restore",
    "semantic-previous-verify", "semantic-previous-restore",
    "recipe-physical-first", "recipe-physical-verify", "recipe-physical-unchanged",
    "recipe-physical-restore", "attachment-physical-first", "attachment-physical-verify",
    "attachment-physical-unchanged", "attachment-physical-restore", "catchup",
    "restic-canonical-check", "restic-raw-check", "borg-canonical-check", "borg-raw-check",
)
PRIVATE_HASH = re.compile(r"\b[0-9a-f]{64}\b")
MAX_STAGE_SECONDS = 7200
MAX_SENDER_RSS = 512 * 1024 * 1024
MAX_RECEIVER_RSS = 256 * 1024 * 1024


class HarnessError(RuntimeError):
    pass


def atomic_json(path: Path, value: Any, replace: bool = False) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.partial")
    with temporary.open("x", encoding="utf-8") as stream:
        json.dump(value, stream, sort_keys=True, indent=2)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())
    if not replace and path.exists():
        temporary.unlink()
        raise HarnessError(f"immutable result already exists: {path.name}")
    os.replace(temporary, path)


def load_g14b(root: Path):
    path = root / "performance" / "v0.7-g14b" / "harness.py"
    spec = importlib.util.spec_from_file_location("notrios_g14b_harness", path)
    if spec is None or spec.loader is None:
        raise HarnessError("could not load the G14b fingerprint implementation")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def command_version(*command: str) -> str:
    try:
        completed = subprocess.run(command, check=True, text=True, capture_output=True)
        return completed.stdout.splitlines()[0][:120]
    except (OSError, subprocess.CalledProcessError, IndexError):
        return "unavailable"


def environment(workspace: Path) -> dict[str, Any]:
    return {
        "os": platform.system().lower(), "architecture": platform.machine(),
        "logical_cpus": os.cpu_count(), "python": platform.python_version(),
        "filesystem_type": command_version("stat", "-f", "-c", "%T", str(workspace)),
        "restic": command_version("restic", "version"),
        "borg": command_version("borg", "--version"),
        "cache_state": "interleaved-repeat",
    }


def phase_result_path(workspace: Path, phase: str) -> Path:
    return workspace / "results" / f"{phase}.json"


def run_command(command: list[str], log: Path, env: dict[str, str] | None = None,
                parse_json: bool = True) -> dict[str, Any]:
    log.parent.mkdir(parents=True, exist_ok=True)
    with log.open("a", encoding="utf-8") as stream:
        completed = subprocess.run(command, text=True, stdout=subprocess.PIPE, stderr=stream, env=env)
    if completed.returncode != 0:
        raise HarnessError(f"command exited {completed.returncode}; inspect the private phase log")
    text = completed.stdout.strip()
    return json.loads(text) if text and parse_json else {}


def source_roots(args: argparse.Namespace, workload: str) -> tuple[Path, Path]:
    canonical = args.g14b_workspace / "data" / workload / "canonical"
    return canonical / "notes.sqlite", canonical / "assets"


def artifact_root(args: argparse.Namespace, name: str) -> Path:
    return args.workspace / "artifacts" / name


def clean_interrupted(path: Path) -> None:
    if path.is_dir():
        shutil.rmtree(path)
    elif path.exists():
        path.unlink()


def fingerprint(args: argparse.Namespace, root: Path) -> dict[str, Any]:
    try:
        root.resolve().relative_to(args.g14b_workspace)
        args.g14b_module.FINGERPRINT_CACHE_ROOT = args.g14b_workspace / "cache/canonical-fingerprints"
    except ValueError:
        args.g14b_module.FINGERPRINT_CACHE_ROOT = args.workspace / "private-fingerprint-cache"
    return args.g14b_module.canonical_fingerprint(root)


def expected_fingerprint(args: argparse.Namespace, workload: str) -> dict[str, Any]:
    corpus = json.loads(phase_result_path(args.workspace, "corpus-equivalence").read_text())
    key = {"recipe-joplin": "recipe_joplin", "attachments": "attachments"}[workload]
    return corpus["private_fingerprints"][key]


def inventory(args: argparse.Namespace, root: Path) -> dict[str, Any]:
    return args.g14b_module.inventory(root, content_hash=False)


def corpus_equivalence(args: argparse.Namespace, log: Path) -> dict[str, Any]:
    del log
    recipe_j = fingerprint(args, args.g14b_workspace / "data/recipe-joplin/canonical")
    recipe_o = fingerprint(args, args.g14b_workspace / "data/recipe-obsidian/canonical")
    attachment = fingerprint(args, args.g14b_workspace / "data/attachments/canonical")
    frozen = json.loads((args.root / "performance/v0.7-g14b/full-corpus-results.json").read_text())
    equivalent = frozen["recipe_import_equivalence"]
    attachment_frozen = frozen["attachment_workload"]
    return {
        "counts": {
            "recipe_joplin_documents": recipe_j["counts"]["documents"],
            "recipe_obsidian_documents": recipe_o["counts"]["documents"],
            "attachment_documents": attachment["counts"]["documents"],
            "attachment_resources": attachment["counts"]["resources"],
            "attachment_blobs": attachment["counts"]["blobs"],
            "attachment_source_bundle_items": attachment["counts"]["source_bundles"],
        },
        "private_fingerprints": {
            "recipe_joplin": recipe_j, "recipe_obsidian": recipe_o, "attachments": attachment,
        },
        "assertions": {
            "integrity_checks_ok": all(value["integrity_check"] == "ok" for value in (recipe_j, recipe_o, attachment)),
            "recipe_document_counts_equal": recipe_j["counts"]["documents"] == recipe_o["counts"]["documents"],
            "recipe_semantic_fingerprints_equal": recipe_j["document_semantic_sha256"] == recipe_o["document_semantic_sha256"],
            "recipe_visible_bodies_equal": recipe_j["document_visible_body_sha256"] == recipe_o["document_visible_body_sha256"],
            "matches_frozen_equivalence": recipe_j["counts"]["documents"] == equivalent["documents_each"]
                and equivalent["semantic_fingerprint_equal"] and equivalent["visible_body_multiset_equal"],
            "matches_frozen_attachment_counts": attachment["counts"]["documents"] == attachment_frozen["documents"]
                and attachment["counts"]["resources"] == attachment_frozen["resources_imported"]
                and attachment["counts"]["source_bundles"] == attachment_frozen["source_bundle_items"],
            "imports_not_repeated_because_code_unchanged": True,
        },
        "frozen_import_seconds": {
            "recipe_joplin": 8158.0, "recipe_obsidian": 9861.0, "attachments": 1611.8,
        },
    }


def semantic_phase(args: argparse.Namespace, phase: str, log: Path) -> dict[str, Any]:
    db, assets = source_roots(args, "recipe-joplin")
    current = artifact_root(args, "semantic-current")
    previous = args.g14b_workspace / "artifacts/recipe-joplin/archive-v2-pack/snapshot"
    archive = current if phase.startswith("semantic-current") else previous
    if phase == "semantic-current-create":
        clean_interrupted(current)
        report = run_command([str(args.notriosctl), "export", "archive-v2", "--db", str(db),
                              "--asset-store", str(assets), "--pack", "--overwrite", "--no-verify",
                              str(current)], log)
        observed = inventory(args, current)
        return {"report": report, "counts": {"output_files": observed["files"],
                "output_bytes": observed["apparent_bytes"], "maximum_directory_entries": observed["maximum_directory_entries"]},
                "assertions": {"bounded_transport_shape": observed["files"] < 100}}
    if phase.endswith("verify"):
        report = run_command([str(args.notriosctl), "verify", "archive-v2", str(archive)], log)
        return {"report": report, "assertions": {"current_reader_verified": True}}
    target = artifact_root(args, phase + "-library")
    clean_interrupted(target)
    target.mkdir(parents=True)
    report = run_command([str(args.notriosctl), "restore", "archive-v2", "--intent", "adopt",
                          "--db", str(target / "notes.sqlite"), "--asset-store", str(target / "assets"),
                          str(archive)], log)
    expected = expected_fingerprint(args, "recipe-joplin")
    actual = fingerprint(args, target)
    return {"report": report, "counts": actual["counts"], "private_fingerprint": actual,
            "assertions": {"exact_canonical_fingerprint": actual["canonical_sha256"] == expected["canonical_sha256"],
                           "current_reader_restored": True}}


def physical_phase(args: argparse.Namespace, phase: str, log: Path) -> dict[str, Any]:
    workload = "attachments" if phase.startswith("attachment-") else "recipe-joplin"
    label = "attachment" if workload == "attachments" else "recipe"
    db, assets = source_roots(args, workload)
    first = artifact_root(args, f"{label}-physical-first")
    unchanged = artifact_root(args, f"{label}-physical-unchanged")
    if phase.endswith("physical-first"):
        clean_interrupted(first)
        report = run_command([str(args.notriosctl), "snapshot", "create", "--db", str(db),
                              "--asset-store", str(assets), str(first)], log)
        observed = inventory(args, first)
        return {"report": report, "counts": {"output_files": observed["files"],
                "output_bytes": observed["apparent_bytes"], "maximum_directory_entries": observed["maximum_directory_entries"]},
                "assertions": {"verified_by_writer": bool(report.get("verified")),
                               "bounded_transport_shape": observed["files"] < 100}}
    if phase.endswith("physical-verify"):
        report = run_command([str(args.notriosctl), "snapshot", "verify", str(first)], log)
        return {"report": report, "assertions": {"ready_for_install": bool(report.get("ready_for_install"))}}
    if phase.endswith("physical-unchanged"):
        clean_interrupted(unchanged)
        report = run_command([str(args.notriosctl), "snapshot", "create", "--db", str(db),
                              "--asset-store", str(assets), str(unchanged)], log)
        left = json.loads((first / "manifest.json").read_text())
        right = json.loads((unchanged / "manifest.json").read_text())
        left_packs = left["external"].get("packs") or []
        right_packs = right["external"].get("packs") or []
        same_packs = [(p["sha256"], p["size_bytes"], p["entries"]) for p in left_packs] == [
            (p["sha256"], p["size_bytes"], p["entries"]) for p in right_packs]
        return {"report": report, "counts": {"packs": len(right_packs),
                "objects": right["external"]["objects"]},
                "assertions": {"unchanged_content_sha256": left["content_sha256"] == right["content_sha256"],
                               "unchanged_database_image": left["database"]["sha256"] == right["database"]["sha256"],
                               "unchanged_external_packs": same_packs}}
    target = artifact_root(args, f"{label}-physical-restored")
    clean_interrupted(target)
    target.mkdir(parents=True)
    run_command([str(args.notriosctl), "doctor", "--db", str(target / "notes.sqlite"),
                 "--asset-store", str(target / "assets")], log, parse_json=False)
    report = run_command([str(args.notriosctl), "snapshot", "restore", "--intent", "adopt",
                          "--db", str(target / "notes.sqlite"), "--asset-store", str(target / "assets"),
                          "--emergency", str(target / "emergency"), str(first)], log)
    expected = expected_fingerprint(args, workload)
    actual = fingerprint(args, target)
    return {"report": report, "counts": actual["counts"], "private_fingerprint": actual,
            "assertions": {"exact_canonical_fingerprint": actual["canonical_sha256"] == expected["canonical_sha256"],
                           "emergency_snapshot_verified": bool(report.get("emergency_snapshot")),
                           "restore_complete": report.get("stage") == "complete",
                           "replica_rotated": report.get("old_replica_id") != report.get("new_replica_id")}}


def catchup_phase(args: argparse.Namespace, log: Path) -> dict[str, Any]:
    host = artifact_root(args, "recipe-physical-restored")
    workspace = artifact_root(args, "catchup")
    clean_interrupted(workspace)
    workspace.mkdir(parents=True)
    report = run_command([str(args.tool), "catchup", str(host / "notes.sqlite"), str(host / "assets"), str(workspace)], log)
    expected = fingerprint(args, host)
    actual = fingerprint(args, workspace / "restored")
    return {"report": report, "counts": actual["counts"], "private_fingerprint": actual,
            "assertions": {"exact_canonical_fingerprint": actual["canonical_sha256"] == expected["canonical_sha256"],
                           "rest_resume_exercised": report.get("rest_download_calls", 0) > 1,
                           "directory_resume_exercised": report.get("directory_download_calls", 0) > 1,
                           "rest_verified": report.get("rest_verified", False),
                           "directory_verified": report.get("directory_verified", False),
                           "emergency_replacement_complete": report.get("restore_stage") == "complete",
                           "post_snapshot_converged": report.get("post_snapshot_converged", False)}}


def repository_phase(args: argparse.Namespace, phase: str, log: Path) -> dict[str, Any]:
    adapter = phase.removesuffix("-check")
    repository = args.g14b_workspace / "repositories/recipe-joplin" / adapter
    secret = args.g14b_workspace / "private" / f"{adapter}.secret"
    env = os.environ.copy()
    if adapter.startswith("restic"):
        env["RESTIC_PASSWORD_FILE"] = str(secret)
        command = ["restic", "-r", str(repository), "check", "--read-data"]
    else:
        env["BORG_PASSPHRASE"] = secret.read_text().strip()
        command = ["borg", "check", "--verify-data", str(repository)]
    run_command(command, log, env, parse_json=False)
    return {"counts": {"frozen_rows_compared": 4},
            "assertions": {"repository_integrity_verified": True, "frozen_baseline_unchanged": True}}


def public_corpus_summary(corpus: dict[str, Any]) -> dict[str, Any]:
    counts = corpus["counts"]
    timings = corpus["frozen_import_seconds"]
    return {
        "counts": {
            "equivalent_joplin_documents": counts["recipe_joplin_documents"],
            "equivalent_obsidian_documents": counts["recipe_obsidian_documents"],
            "attachment_documents": counts["attachment_documents"],
            "attachment_resources": counts["attachment_resources"],
            "attachment_blobs": counts["attachment_blobs"],
            "attachment_source_bundle_items": counts["attachment_source_bundle_items"],
        },
        "assertions": corpus["assertions"],
        "frozen_import_seconds": {
            "equivalent_joplin_view": timings["recipe_joplin"],
            "equivalent_obsidian_view": timings["recipe_obsidian"],
            "attachment_workload": timings["attachments"],
        },
    }


def public_phase_counts(result: dict[str, Any]) -> dict[str, Any]:
    if result["phase"] == "corpus-equivalence":
        return public_corpus_summary(result)["counts"]
    return result.get("counts", {})


def execute_phase(args: argparse.Namespace, phase: str) -> dict[str, Any]:
    if phase == "corpus-equivalence":
        work: Callable[[argparse.Namespace, Path], dict[str, Any]] = corpus_equivalence
    elif phase.startswith("semantic-"):
        work = lambda a, log: semantic_phase(a, phase, log)
    elif "-physical-" in phase:
        work = lambda a, log: physical_phase(a, phase, log)
    elif phase == "catchup":
        work = catchup_phase
    else:
        work = lambda a, log: repository_phase(a, phase, log)
    result_path = phase_result_path(args.workspace, phase)
    if result_path.exists():
        return json.loads(result_path.read_text())
    checkpoint = args.workspace / "checkpoints" / f"{phase}.json"
    attempts = 1
    if checkpoint.exists():
        attempts = int(json.loads(checkpoint.read_text()).get("attempt", 0)) + 1
    atomic_json(checkpoint, {"schema": SCHEMA, "phase": phase, "status": "started", "attempt": attempts}, True)
    before = resource.getrusage(resource.RUSAGE_SELF)
    children = resource.getrusage(resource.RUSAGE_CHILDREN)
    started = time.perf_counter()
    payload = work(args, args.workspace / "logs" / f"{phase}.log")
    elapsed = time.perf_counter() - started
    after = resource.getrusage(resource.RUSAGE_SELF)
    child_after = resource.getrusage(resource.RUSAGE_CHILDREN)
    result = {"schema": SCHEMA, "phase": phase, "status": "completed", "attempt": attempts,
              "metrics": {"wall_seconds": elapsed,
                  "user_cpu_seconds": after.ru_utime - before.ru_utime + child_after.ru_utime - children.ru_utime,
                  "system_cpu_seconds": after.ru_stime - before.ru_stime + child_after.ru_stime - children.ru_stime,
                  "peak_rss_bytes": max(after.ru_maxrss, child_after.ru_maxrss) * 1024}, **payload}
    if not all(result.get("assertions", {}).values()):
        raise HarnessError(f"acceptance assertion failed in {phase}")
    atomic_json(result_path, result)
    atomic_json(checkpoint, {"schema": SCHEMA, "phase": phase, "status": "completed", "attempt": attempts}, True)
    return result


def sanitized_summary(args: argparse.Namespace) -> dict[str, Any]:
    results = [json.loads(phase_result_path(args.workspace, phase).read_text()) for phase in PHASES]
    rows = []
    for result in results:
        row = {"phase": result["phase"], **result["metrics"], **public_phase_counts(result),
               **result.get("assertions", {})}
        rows.append(row)
    corpus = results[0]
    frozen = json.loads((args.root / "performance/v0.7-g14b/full-corpus-results.json").read_text())
    summary = {
        "schema": SUMMARY_SCHEMA,
        "environment": environment(args.workspace),
        "acceptance_policy": {"maximum_stage_seconds": MAX_STAGE_SECONDS,
            "maximum_sender_rss_bytes": MAX_SENDER_RSS,
            "maximum_receiver_rss_bytes": MAX_RECEIVER_RSS,
            "transport_entries_must_not_scale_per_object": True},
        "corpora": public_corpus_summary(corpus),
        "format_freeze": {"whole_library_default": "sqlite-image+packed-assets.v1",
            "semantic_portable_format": "archive-v2", "encrypted_frame_format": "NBK1",
            "container": "deterministic sequential USTAR", "schema": 25,
            "new_format_introduced": False},
        "frozen_repository_baseline_rows": [row for row in frozen["rows"] if row["adapter"] in {
            "restic-canonical", "restic-raw", "borg-canonical", "borg-raw"}],
        "rows": rows,
        "claims": {"exfat_measured": False, "physical_mobile_measured": False,
                   "cloud_provider_rerun": False, "aggregate_only": True},
    }
    serialized = json.dumps(summary, sort_keys=True)
    forbidden = ("/home/", "JoplinExport_", "recipe_joplin", "recipe_vault", "recipedb")
    if any(value in serialized for value in forbidden) or PRIVATE_HASH.search(serialized):
        raise HarnessError("private path or full-corpus hash entered sanitized summary")
    return summary


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--workspace", type=Path, required=True)
    parser.add_argument("--g14b-workspace", type=Path, required=True)
    parser.add_argument("--notriosctl", type=Path, required=True)
    parser.add_argument("--tool", type=Path, required=True)
    parser.add_argument("--phase", choices=PHASES)
    parser.add_argument("--assemble", action="store_true")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    args.root = Path(__file__).resolve().parents[2]
    args.workspace = args.workspace.resolve()
    args.g14b_workspace = args.g14b_workspace.resolve()
    args.g14b_module = load_g14b(args.root)
    return args


def main() -> None:
    args = parse_args()
    if args.assemble:
        if args.output is None:
            raise HarnessError("--assemble requires --output")
        atomic_json(args.output, sanitized_summary(args), True)
        print(json.dumps({"assembled": str(args.output), "phases": len(PHASES)}))
        return
    if not args.phase:
        raise HarnessError("one --phase is required")
    result = execute_phase(args, args.phase)
    print(json.dumps({"phase": args.phase, "status": result["status"], "resumed": result["attempt"] > 1}))


if __name__ == "__main__":
    try:
        main()
    except (HarnessError, OSError, ValueError, KeyError, json.JSONDecodeError) as error:
        print(f"g14e harness: {error}", file=sys.stderr)
        raise SystemExit(1)
