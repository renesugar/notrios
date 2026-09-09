#!/usr/bin/env python3
"""G17a investigation helpers; not the production G17b evidence tool."""
from __future__ import annotations

import argparse
import binascii
import datetime as dt
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import stat
import struct
import subprocess
import tempfile
import unicodedata
import zipfile
import zlib


SCHEMA = "notrios.evidence.inventory-summary.v1"
ENTRY_SCHEMA = "notrios.evidence.manifest-entry.v1"
CHECKPOINT_SCHEMA = "notrios.evidence.content-checkpoint.v1"
ZERO_HASH = "0" * 64
CD_BUDGET_BYTES = 650 * 1024 * 1024
ANCHORS = (
    "README.md",
    "PLAN.md",
    "ROADMAP.md",
    "CODING_CLIENT_HANDOFF.md",
    "agent/PLAN_STATUS.md",
    "agent/ATTEMPT_LOG.jsonl",
)
CURRENT_RELEASE_REQUIRED = {
    "README.md", "PLAN.md", "ROADMAP.md", "API_SPEC.md",
    "DATABASE_SCHEMA.md", "PACKAGING.md", "SECURITY_REVIEW.md",
    "plans/mvp/MVP_RELEASE_REPORT.md", "cmd/notriosd/main.go",
    "cmd/notriosctl/main.go", "web/dist/index.html",
}


def run(args: list[str], *, cwd: Path | None = None,
        env: dict[str, str] | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        args, cwd=cwd, env=env, check=check, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def _check_canonical_value(value: object) -> None:
    if isinstance(value, float):
        raise ValueError("floating-point values are outside the evidence schema")
    if isinstance(value, str):
        if unicodedata.normalize("NFC", value) != value:
            raise ValueError("strings must already be NFC-normalized")
        return
    if value is None or isinstance(value, bool):
        return
    if isinstance(value, int):
        if value < -(2**63) or value > 2**63 - 1:
            raise ValueError("integers must fit signed 64-bit")
        return
    if isinstance(value, list):
        for item in value:
            _check_canonical_value(item)
        return
    if isinstance(value, dict):
        for key, item in value.items():
            if not isinstance(key, str):
                raise ValueError("JSON object keys must be strings")
            _check_canonical_value(key)
            _check_canonical_value(item)
        return
    raise ValueError(f"unsupported canonical JSON type: {type(value).__name__}")


def canonical_bytes(value: object) -> bytes:
    _check_canonical_value(value)
    return (json.dumps(
        value, ensure_ascii=False, allow_nan=False, sort_keys=True,
        separators=(",", ":"),
    ) + "\n").encode("utf-8")


def make_chain(payloads: list[dict[str, object]]) -> list[dict[str, object]]:
    previous = ZERO_HASH
    records: list[dict[str, object]] = []
    for sequence, payload in enumerate(payloads, start=1):
        core = {
            "schema": ENTRY_SCHEMA,
            "sequence": sequence,
            "previous_entry_sha256": previous,
            "payload": payload,
        }
        entry_hash = hashlib.sha256(canonical_bytes(core)).hexdigest()
        record = dict(core)
        record["entry_sha256"] = entry_hash
        records.append(record)
        previous = entry_hash
    return records


def manifest_bytes(records: list[dict[str, object]]) -> bytes:
    return b"".join(canonical_bytes(record) for record in records)


def verify_chain(records: list[dict[str, object]]) -> None:
    previous = ZERO_HASH
    for expected_sequence, record in enumerate(records, start=1):
        if record.get("schema") != ENTRY_SCHEMA:
            raise ValueError("wrong manifest entry schema")
        if record.get("sequence") != expected_sequence:
            raise ValueError("manifest sequence is not contiguous")
        if record.get("previous_entry_sha256") != previous:
            raise ValueError("manifest previous-entry hash mismatch")
        core = {key: value for key, value in record.items() if key != "entry_sha256"}
        actual = hashlib.sha256(canonical_bytes(core)).hexdigest()
        if record.get("entry_sha256") != actual:
            raise ValueError("manifest entry hash mismatch")
        previous = actual


def checkpoint_for(records: list[dict[str, object]]) -> dict[str, object]:
    verify_chain(records)
    encoded = manifest_bytes(records)
    return {
        "schema": CHECKPOINT_SCHEMA,
        "entry_count": len(records),
        "first_entry_sha256": records[0]["entry_sha256"] if records else ZERO_HASH,
        "last_entry_sha256": records[-1]["entry_sha256"] if records else ZERO_HASH,
        "manifest_sha256": hashlib.sha256(encoded).hexdigest(),
    }


def verify_payload_tree(records: list[dict[str, object]], root: Path) -> None:
    expected: dict[str, tuple[int, str]] = {}
    for record in records:
        payload = record.get("payload")
        if not isinstance(payload, dict) or payload.get("record_type") != "artifact":
            continue
        name = payload.get("logical_name")
        size = payload.get("size_bytes")
        digest = payload.get("sha256")
        if not isinstance(name, str) or not _safe_zip_name(name):
            raise ValueError("unsafe artifact logical name")
        if not isinstance(size, int) or size < 0 or not isinstance(digest, str) or len(digest) != 64:
            raise ValueError("invalid artifact size or digest")
        if name in expected:
            raise ValueError("duplicate artifact logical name")
        expected[name] = (size, digest)
    actual: dict[str, Path] = {}
    for path in root.rglob("*"):
        if path.is_symlink():
            raise ValueError("symlink in payload tree")
        if path.is_file():
            actual[path.relative_to(root).as_posix()] = path
    missing = sorted(set(expected) - set(actual))
    extra = sorted(set(actual) - set(expected))
    if missing or extra:
        raise ValueError(f"payload tree membership mismatch: missing={len(missing)} extra={len(extra)}")
    for name, (size, digest) in expected.items():
        path = actual[name]
        if path.stat().st_size != size or sha256_file(path) != digest:
            raise ValueError(f"payload bytes differ: {name}")


def _safe_zip_name(name: str) -> bool:
    if not name or "\x00" in name or "\\" in name or name.startswith("/"):
        return False
    logical_name = name[:-1] if name.endswith("/") else name
    if not logical_name:
        return False
    path = PurePosixPath(logical_name)
    return (
        not path.is_absolute()
        and all(part not in ("", ".", "..") for part in logical_name.split("/"))
    )


def validate_zip(path: Path) -> dict[str, object]:
    with zipfile.ZipFile(path) as archive:
        infos = archive.infolist()
        names = [item.filename for item in infos]
        duplicate_count = len(names) - len(set(names))
        unsafe_count = sum(not _safe_zip_name(name) for name in names)
        encrypted_count = sum(bool(item.flag_bits & 0x1) for item in infos)
        symlink_count = sum(
            stat.S_ISLNK((item.external_attr >> 16) & 0xFFFF) for item in infos
        )
        corrupt_member = archive.testzip()
        name_set = set(names)
        current_shape = CURRENT_RELEASE_REQUIRED <= name_set and any(
            name.startswith("web/dist/assets/") for name in names
        )
        anchor_oids = {
            anchor: git_blob_oid(archive.read(anchor), "sha1")
            for anchor in ANCHORS if anchor in name_set
        }
    return {
        "valid": not any((duplicate_count, unsafe_count, encrypted_count, symlink_count))
        and corrupt_member is None,
        "entries": len(infos),
        "duplicate_names": duplicate_count,
        "unsafe_names": unsafe_count,
        "encrypted_entries": encrypted_count,
        "symlink_entries": symlink_count,
        "corrupt_member": corrupt_member,
        "current_release_shape": current_shape,
        "anchor_oids": anchor_oids,
    }


def validate_png(path: Path) -> dict[str, object]:
    data = path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        return {"valid": False, "chunks": 0, "reason": "bad signature"}
    offset = 8
    chunks = 0
    seen_ihdr = False
    seen_iend = False
    reason = ""
    while offset < len(data):
        if len(data) - offset < 12:
            reason = "truncated chunk header"
            break
        length = struct.unpack(">I", data[offset:offset + 4])[0]
        kind = data[offset + 4:offset + 8]
        end = offset + 12 + length
        if end > len(data):
            reason = "truncated chunk"
            break
        payload = data[offset + 8:offset + 8 + length]
        expected = struct.unpack(">I", data[offset + 8 + length:end])[0]
        actual = binascii.crc32(kind + payload) & 0xFFFFFFFF
        if expected != actual:
            reason = "chunk CRC mismatch"
            break
        chunks += 1
        if chunks == 1:
            seen_ihdr = kind == b"IHDR" and length == 13
        if kind == b"IEND":
            seen_iend = True
            offset = end
            break
        offset = end
    if not reason and offset != len(data):
        reason = "trailing bytes"
    valid = not reason and seen_ihdr and seen_iend
    return {"valid": valid, "chunks": chunks, "reason": reason or None}


def git_blob_oid(data: bytes, algorithm: str) -> str:
    digest = hashlib.new(algorithm)
    digest.update(f"blob {len(data)}\0".encode("ascii"))
    digest.update(data)
    return digest.hexdigest()


def git_anchor_history(repo: Path) -> tuple[str, list[tuple[str, dict[str, str]]]]:
    object_format = run(["git", "rev-parse", "--show-object-format"], cwd=repo).stdout.strip()
    commits = run(["git", "rev-list", "--all", "--reverse"], cwd=repo).stdout.splitlines()
    history: list[tuple[str, dict[str, str]]] = []
    for commit in commits:
        result = run(["git", "ls-tree", "-r", commit, "--", *ANCHORS], cwd=repo)
        anchors: dict[str, str] = {}
        for line in result.stdout.splitlines():
            metadata, name = line.split("\t", 1)
            oid = metadata.split()[2]
            anchors[name] = oid
        history.append((commit, anchors))
    return object_format, history


def map_provenance(anchor_oids: dict[str, str], history: list[tuple[str, dict[str, str]]]) -> dict[str, object]:
    if len(anchor_oids) < 3:
        return {"status": "insufficient", "matched_anchors": len(anchor_oids), "candidates": []}
    scores: list[tuple[int, str]] = []
    for commit, known in history:
        score = sum(known.get(name) == oid for name, oid in anchor_oids.items())
        scores.append((score, commit))
    best = max((score for score, _ in scores), default=0)
    candidates = [commit for score, commit in scores if score == best]
    if best == len(anchor_oids):
        status = "unique_exact" if len(candidates) == 1 else "ambiguous_exact"
    else:
        status = "partial_only"
    return {"status": status, "matched_anchors": best, "candidates": candidates}


def iso_options(volume_id: str) -> list[str]:
    if len(volume_id) > 16 or not volume_id.replace("-", "").replace("_", "").isalnum():
        raise ValueError("volume ID must be portable Joliet-safe ASCII and at most 16 characters")
    return [
        "-iso-level", "3", "-r", "-J", "-joliet-long",
        "-V", volume_id,
        "-A", "NOTRIOS EVIDENCE RESERVE V1",
        "-publisher", "NOTRIOS PROJECT",
        "-p", "NOTRIOS G17B",
    ]


def iso_print_size(source: Path, volume_id: str, epoch: int) -> int:
    env = os.environ.copy()
    env.update({"SOURCE_DATE_EPOCH": str(epoch), "TZ": "UTC"})
    result = run(
        ["xorriso", "-no_rc", "-as", "mkisofs", *iso_options(volume_id),
         "-print-size", str(source)], env=env,
    )
    numbers = [line.strip() for line in result.stdout.splitlines() if line.strip().isdigit()]
    if not numbers:
        numbers = [line.strip() for line in result.stderr.splitlines() if line.strip().isdigit()]
    if not numbers:
        raise RuntimeError("xorriso did not report an ISO block count")
    return int(numbers[-1])


def iso_print_size_files(paths: list[Path], volume_id: str, epoch: int) -> int:
    env = os.environ.copy()
    env.update({"SOURCE_DATE_EPOCH": str(epoch), "TZ": "UTC"})
    grafts: list[str] = []
    for path in paths:
        name = path.name.replace("\\", "\\\\").replace("=", "\\=")
        source = str(path.resolve()).replace("\\", "\\\\").replace("=", "\\=")
        grafts.append(f"{name}={source}")
    result = run(
        ["xorriso", "-no_rc", "-as", "mkisofs", *iso_options(volume_id),
         "-graft-points", "-print-size", *grafts], env=env,
    )
    numbers = [line.strip() for line in result.stdout.splitlines() if line.strip().isdigit()]
    if not numbers:
        numbers = [line.strip() for line in result.stderr.splitlines() if line.strip().isdigit()]
    if not numbers:
        raise RuntimeError("xorriso did not report an ISO block count")
    return int(numbers[-1])


def build_iso(source: Path, output: Path, volume_id: str, epoch: int) -> None:
    env = os.environ.copy()
    env.update({"SOURCE_DATE_EPOCH": str(epoch), "TZ": "UTC"})
    run(
        ["xorriso", "-no_rc", "-as", "mkisofs", *iso_options(volume_id),
         "-o", str(output), str(source)], env=env,
    )


def inventory(args: argparse.Namespace) -> None:
    source = args.source.resolve()
    repo = args.repo.resolve()
    if not source.is_dir() or source == repo or repo in source.parents:
        raise SystemExit("source must be an existing external directory")
    captured = dt.datetime.fromisoformat(args.captured_at.replace("Z", "+00:00"))
    if captured.tzinfo is None:
        raise SystemExit("captured-at must include a timezone")
    object_format, history = git_anchor_history(repo)
    if object_format != "sha1":
        raise SystemExit(f"prototype currently expects a SHA-1 Git object store, got {object_format}")
    records: list[dict[str, object]] = []
    provenance_counts: dict[str, int] = {}
    kind_counts: dict[str, int] = {}
    validation_counts = {"valid": 0, "invalid": 0, "current_release_shape": 0}
    sidecars = {suffix: 0 for suffix in (".sig", ".asc", ".tsq", ".tsr")}
    top_level_paths = sorted((item for item in source.iterdir() if item.is_file()), key=lambda item: item.name.encode("utf-8"))
    top_level_directories = sum(item.is_dir() for item in source.iterdir())
    for path in top_level_paths:
        normalized_name = unicodedata.normalize("NFC", path.name)
        if normalized_name != path.name:
            raise SystemExit("inventory contains a non-NFC filename")
        suffix = path.suffix.lower()
        for sidecar in sidecars:
            if suffix == sidecar:
                sidecars[sidecar] += 1
        if suffix == ".zip":
            kind = "zip"
            validation = validate_zip(path)
            provenance = map_provenance(validation.pop("anchor_oids"), history)
            validation_counts["current_release_shape"] += int(validation["current_release_shape"])
        elif suffix == ".png":
            kind = "png"
            validation = validate_png(path)
            provenance = {"status": "not_applicable", "matched_anchors": 0, "candidates": []}
        else:
            kind = "other"
            validation = {"valid": True, "reason": "hash-and-size only"}
            provenance = {"status": "not_applicable", "matched_anchors": 0, "candidates": []}
        kind_counts[kind] = kind_counts.get(kind, 0) + 1
        validation_counts["valid" if validation["valid"] else "invalid"] += 1
        status = str(provenance["status"])
        provenance_counts[status] = provenance_counts.get(status, 0) + 1
        file_stat = path.stat()
        records.append({
            "logical_name": normalized_name,
            "kind": kind,
            "size_bytes": file_stat.st_size,
            "sha256": sha256_file(path),
            "mtime_ns": file_stat.st_mtime_ns,
            "validation": validation,
            "provenance": provenance,
        })
    content_projection = [
        {key: record[key] for key in ("logical_name", "kind", "size_bytes", "sha256")}
        for record in records
    ]
    capture_projection = records
    epoch = int(captured.timestamp())
    blocks = iso_print_size_files(top_level_paths, "NTR-G17A-PROBE", epoch)
    summary = {
        "schema": SCHEMA,
        "scope": "top-level regular files only; recursive workspaces measured separately",
        "captured_at": captured.astimezone(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
        "source_regular_files": len(records),
        "source_top_level_directories": top_level_directories,
        "source_total_bytes": sum(int(record["size_bytes"]) for record in records),
        "largest_artifact_bytes": max((int(record["size_bytes"]) for record in records), default=0),
        "kind_counts": kind_counts,
        "sidecar_counts": sidecars,
        "validation_counts": validation_counts,
        "provenance_counts": provenance_counts,
        "git_object_format": object_format,
        "git_commits_considered": len(history),
        "content_inventory_commitment_sha256": hashlib.sha256(canonical_bytes(content_projection)).hexdigest(),
        "capture_inventory_commitment_sha256": hashlib.sha256(canonical_bytes(capture_projection)).hexdigest(),
        "source_iso_print_size_blocks": blocks,
        "source_iso_print_size_bytes": blocks * 2048,
        "conservative_cd_budget_bytes": CD_BUDGET_BYTES,
        "source_iso_budget_basis_points": (blocks * 2048 * 10_000) // CD_BUDGET_BYTES,
        "source_directory_modified": False,
        "production_iso_created": False,
        "expected_next_append": "the verified G17a release ZIP created after this frozen capture",
    }
    args.private_output.parent.mkdir(parents=True, exist_ok=True)
    args.private_output.write_bytes(canonical_bytes({"summary": summary, "records": records}))
    args.summary_output.parent.mkdir(parents=True, exist_ok=True)
    args.summary_output.write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(summary, sort_keys=True))


def tree_summary(args: argparse.Namespace) -> None:
    source = args.source.resolve()
    if not source.is_dir():
        raise SystemExit("source must be an existing directory")
    regular_files = 0
    regular_bytes = 0
    directories = 0
    symlinks = 0
    other = 0
    largest_file_bytes = 0
    for root_text, dir_names, file_names in os.walk(source, followlinks=False):
        root = Path(root_text)
        directories += len(dir_names)
        for name in file_names:
            path = root / name
            try:
                file_stat = path.lstat()
            except FileNotFoundError:
                raise SystemExit("recursive inventory changed during traversal")
            if stat.S_ISREG(file_stat.st_mode):
                regular_files += 1
                regular_bytes += file_stat.st_size
                largest_file_bytes = max(largest_file_bytes, file_stat.st_size)
            elif stat.S_ISLNK(file_stat.st_mode):
                symlinks += 1
            else:
                other += 1
    top_files = [item for item in source.iterdir() if item.is_file()]
    top_bytes = sum(item.stat().st_size for item in top_files)
    result = {
        "schema": "notrios.g17a.recursive-scope-summary.v1",
        "scope": "aggregate recursive metadata only; no content hashes or names",
        "regular_files": regular_files,
        "regular_bytes": regular_bytes,
        "directories": directories,
        "symlinks": symlinks,
        "other_nodes": other,
        "largest_file_bytes": largest_file_bytes,
        "top_level_regular_files": len(top_files),
        "top_level_regular_bytes": top_bytes,
        "nested_regular_files": regular_files - len(top_files),
        "nested_regular_bytes": regular_bytes - top_bytes,
        "source_directory_modified": False,
    }
    args.summary_output.parent.mkdir(parents=True, exist_ok=True)
    args.summary_output.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(result, sort_keys=True))


def _write_png(path: Path) -> None:
    def chunk(kind: bytes, payload: bytes) -> bytes:
        return struct.pack(">I", len(payload)) + kind + payload + struct.pack(">I", binascii.crc32(kind + payload) & 0xFFFFFFFF)
    raw = b"\x00\x12\x34\x56\xff"
    data = b"\x89PNG\r\n\x1a\n"
    data += chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 6, 0, 0, 0))
    data += chunk(b"IDAT", zlib.compress(raw))
    data += chunk(b"IEND", b"")
    path.write_bytes(data)


def _normalize_tree(root: Path, epoch: int) -> None:
    for path in sorted(root.rglob("*"), reverse=True):
        if path.is_file():
            path.chmod(0o444)
        elif path.is_dir():
            path.chmod(0o555)
        os.utime(path, (epoch, epoch), follow_symlinks=False)
    root.chmod(0o555)
    os.utime(root, (epoch, epoch), follow_symlinks=False)


def _tree_hashes(root: Path) -> dict[str, str]:
    return {
        path.relative_to(root).as_posix(): sha256_file(path)
        for path in sorted(root.rglob("*")) if path.is_file()
    }


def _generate_gpg_fixture(work: Path, datum: Path) -> tuple[Path, Path]:
    home = work / "gnupg-sign"
    verify_home = work / "gnupg-verify"
    home.mkdir(mode=0o700)
    verify_home.mkdir(mode=0o700)
    env = os.environ.copy()
    env["GNUPGHOME"] = str(home)
    identity = "Notrios G17a Fixture <fixture@example.invalid>"
    run(["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
         "--quick-generate-key", identity, "ed25519", "sign", "1d"], env=env)
    fingerprint = run(["gpg", "--batch", "--with-colons", "--list-secret-keys"], env=env)
    fingerprints = [line.split(":")[9] for line in fingerprint.stdout.splitlines() if line.startswith("fpr:")]
    if not fingerprints:
        raise RuntimeError("fixture key fingerprint was not produced")
    signature = work / "content-checkpoint.json.sig"
    public_key = work / "fixture-public-key.asc"
    run(["gpg", "--batch", "--yes", "--pinentry-mode", "loopback", "--passphrase", "",
         "--local-user", fingerprints[0], "--output", str(signature), "--detach-sign", str(datum)], env=env)
    exported = run(["gpg", "--batch", "--armor", "--export", fingerprints[0]], env=env)
    public_key.write_text(exported.stdout, encoding="utf-8")
    verify_env = os.environ.copy()
    verify_env["GNUPGHOME"] = str(verify_home)
    run(["gpg", "--batch", "--import", str(public_key)], env=verify_env)
    run(["gpg", "--batch", "--verify", str(signature), str(datum)], env=verify_env)
    tampered = work / "tampered-checkpoint.json"
    tampered.write_bytes(datum.read_bytes() + b" ")
    refused = run(["gpg", "--batch", "--verify", str(signature), str(tampered)], env=verify_env, check=False)
    if refused.returncode == 0:
        raise RuntimeError("GnuPG accepted a signature over tampered bytes")
    return signature, public_key


def _generate_tsa_fixture(work: Path, datum: Path) -> tuple[Path, Path, Path, str]:
    root_key = work / "fixture-root.key"
    root_cert = work / "fixture-root.pem"
    tsa_key = work / "fixture-tsa.key"
    tsa_csr = work / "fixture-tsa.csr"
    tsa_cert = work / "fixture-tsa.pem"
    ext = work / "tsa-ext.cnf"
    serial = work / "tsa-serial"
    config = work / "tsa.cnf"
    query = work / "content-checkpoint.sig.tsq"
    response = work / "content-checkpoint.sig.tsr"
    policy = "1.3.6.1.4.1.55555.17.1"
    run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-sha256",
         "-days", "2", "-subj", "/CN=Notrios G17a Fixture Root",
         "-keyout", str(root_key), "-out", str(root_cert)])
    run(["openssl", "req", "-newkey", "rsa:2048", "-nodes", "-sha256",
         "-subj", "/CN=Notrios G17a Fixture TSA",
         "-keyout", str(tsa_key), "-out", str(tsa_csr)])
    ext.write_text(
        "basicConstraints=critical,CA:FALSE\n"
        "keyUsage=critical,digitalSignature,nonRepudiation\n"
        "extendedKeyUsage=critical,timeStamping\n"
        "subjectKeyIdentifier=hash\n"
        "authorityKeyIdentifier=keyid,issuer\n", encoding="utf-8",
    )
    run(["openssl", "x509", "-req", "-in", str(tsa_csr), "-CA", str(root_cert),
         "-CAkey", str(root_key), "-CAcreateserial", "-days", "1", "-sha256",
         "-extfile", str(ext), "-out", str(tsa_cert)])
    serial.write_text("01\n", encoding="ascii")
    config.write_text(
        "[ tsa ]\n"
        "default_tsa = tsa_config1\n"
        "[ tsa_config1 ]\n"
        f"serial = {serial}\n"
        "crypto_device = builtin\n"
        f"signer_cert = {tsa_cert}\n"
        f"certs = {root_cert}\n"
        f"signer_key = {tsa_key}\n"
        "signer_digest = sha256\n"
        f"default_policy = {policy}\n"
        "digests = sha256\n"
        "accuracy = secs:1\n"
        "ordering = yes\n"
        "tsa_name = yes\n"
        "ess_cert_id_chain = yes\n"
        "ess_cert_id_alg = sha256\n", encoding="utf-8",
    )
    run(["openssl", "ts", "-query", "-data", str(datum), "-sha256", "-cert", "-out", str(query)])
    run(["openssl", "ts", "-reply", "-config", str(config), "-section", "tsa_config1",
         "-queryfile", str(query), "-out", str(response)])
    run(["openssl", "ts", "-verify", "-queryfile", str(query), "-in", str(response),
         "-CAfile", str(root_cert), "-untrusted", str(tsa_cert)])
    run(["openssl", "ts", "-verify", "-data", str(datum), "-in", str(response),
         "-CAfile", str(root_cert), "-untrusted", str(tsa_cert)])
    tampered = work / "tampered-signature"
    tampered.write_bytes(datum.read_bytes() + b"x")
    refused = run(["openssl", "ts", "-verify", "-data", str(tampered), "-in", str(response),
                   "-CAfile", str(root_cert), "-untrusted", str(tsa_cert)], check=False)
    if refused.returncode == 0:
        raise RuntimeError("OpenSSL accepted a timestamp over tampered bytes")
    wrong_key = work / "wrong-root.key"
    wrong_cert = work / "wrong-root.pem"
    run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-sha256",
         "-days", "1", "-subj", "/CN=Wrong Fixture Root",
         "-keyout", str(wrong_key), "-out", str(wrong_cert)])
    refused = run(["openssl", "ts", "-verify", "-queryfile", str(query), "-in", str(response),
                   "-CAfile", str(wrong_cert), "-untrusted", str(tsa_cert)], check=False)
    if refused.returncode == 0:
        raise RuntimeError("OpenSSL accepted a timestamp under the wrong trust root")
    text_result = run(["openssl", "ts", "-reply", "-in", str(response), "-text"])
    if policy not in text_result.stdout or "Hash Algorithm: sha256" not in text_result.stdout or "Nonce:" not in text_result.stdout:
        raise RuntimeError("timestamp response omitted policy, SHA-256, or nonce")
    return query, response, root_cert, policy


def self_test(args: argparse.Namespace) -> None:
    base = args.workdir.resolve()
    if base.exists():
        raise SystemExit("self-test workdir must not already exist")
    base.mkdir(parents=True)
    epoch = 1_788_134_400  # 2026-08-29T00:00:00Z, fixed generated-fixture time.
    work = base / "work"
    work.mkdir()
    artifact = work / "artifact-one.txt"
    artifact.write_text("generated G17a fixture\n", encoding="utf-8")
    screen = work / "generated-screen.png"
    _write_png(screen)
    payloads = []
    for path in (artifact, screen):
        payloads.append({
            "record_type": "artifact",
            "logical_name": f"payload/{path.name}",
            "size_bytes": path.stat().st_size,
            "sha256": sha256_file(path),
            "retroactive": True,
        })
    records = make_chain(payloads)
    verify_chain(records)
    manifest = work / "content-manifest.jsonl"
    manifest.write_bytes(manifest_bytes(records))
    checkpoint = work / "content-checkpoint.json"
    checkpoint.write_bytes(canonical_bytes(checkpoint_for(records)))
    signature, public_key = _generate_gpg_fixture(work, checkpoint)
    query, response, root_cert, policy = _generate_tsa_fixture(work, signature)
    tsa_cert = work / "fixture-tsa.pem"
    stage_hashes: dict[str, str] | None = None
    iso_hashes: list[str] = []
    iso_sizes: list[int] = []
    printed_blocks: list[int] = []
    long_name = "unicode-å-資料-" + "x" * 60 + ".txt"
    for build_number in (1, 2):
        stage = base / f"stage-{build_number}"
        (stage / "payload").mkdir(parents=True)
        (stage / "verification").mkdir()
        shutil.copyfile(artifact, stage / "payload" / artifact.name)
        shutil.copyfile(screen, stage / "payload" / screen.name)
        (stage / "payload" / long_name).write_text("long Unicode path fixture\n", encoding="utf-8")
        for source in (manifest, checkpoint, signature, public_key, query, response, root_cert, tsa_cert):
            shutil.copyfile(source, stage / "verification" / source.name)
        (stage / "README.txt").write_text("Generated G17a fixture only.\n", encoding="utf-8")
        _normalize_tree(stage, epoch)
        hashes = _tree_hashes(stage)
        if stage_hashes is None:
            stage_hashes = hashes
        elif hashes != stage_hashes:
            raise RuntimeError("clean staging trees differ")
        blocks = iso_print_size(stage, "NTR-G17A-TEST", epoch)
        printed_blocks.append(blocks)
        image = base / f"fixture-{build_number}.iso"
        build_iso(stage, image, "NTR-G17A-TEST", epoch)
        iso_hashes.append(sha256_file(image))
        iso_sizes.append(image.stat().st_size)
        extracted = base / f"extracted-{build_number}"
        extracted.mkdir()
        run(["xorriso", "-no_rc", "-osirrox", "on", "-indev", str(image),
             "-extract", "/", str(extracted)])
        if _tree_hashes(extracted) != hashes:
            raise RuntimeError("ISO extraction differs from staging")
    if len(set(iso_hashes)) != 1 or len(set(iso_sizes)) != 1:
        raise RuntimeError("clean ISO rebuild is not byte-identical")
    if any(size != blocks * 2048 for size, blocks in zip(iso_sizes, printed_blocks)):
        raise RuntimeError("xorriso print-size differs from final image byte size")
    if iso_sizes[0] >= CD_BUDGET_BYTES:
        raise RuntimeError("generated ISO exceeds the conservative CD budget")
    result = {
        "schema": "notrios.g17a.prototype-results.v1",
        "generated_only": True,
        "workdir_outside_repository": True,
        "manifest_entries": len(records),
        "canonical_chain_verified": True,
        "gpg_detached_signature_verified": True,
        "gpg_tamper_refused": True,
        "rfc3161_query_and_response_verified": True,
        "rfc3161_tamper_refused": True,
        "rfc3161_wrong_ca_refused": True,
        "rfc3161_policy_oid": policy,
        "timestamped_datum": "detached content-checkpoint signature",
        "iso_rebuilds": 2,
        "iso_sha256_equal": True,
        "iso_size_bytes": iso_sizes[0],
        "iso_print_size_blocks": printed_blocks[0],
        "iso_extract_hash_walk_equal": True,
        "rock_ridge_policy": "rationalized uid/gid zero and read-only modes",
        "joliet_long_unicode_fixture": True,
        "conservative_cd_budget_bytes": CD_BUDGET_BYTES,
        "production_key_used": False,
        "production_tsa_contacted": False,
        "production_iso_created": False,
        "gpg_version": run(["gpg", "--version"]).stdout.splitlines()[0],
        "openssl_version": run(["openssl", "version"]).stdout.strip(),
        "xorriso_version": run(["xorriso", "-version"]).stdout.splitlines()[0],
    }
    args.result_output.parent.mkdir(parents=True, exist_ok=True)
    args.result_output.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(result, sort_keys=True))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)
    inventory_parser = subparsers.add_parser("inventory")
    inventory_parser.add_argument("--source", type=Path, required=True)
    inventory_parser.add_argument("--repo", type=Path, required=True)
    inventory_parser.add_argument("--captured-at", required=True)
    inventory_parser.add_argument("--private-output", type=Path, required=True)
    inventory_parser.add_argument("--summary-output", type=Path, required=True)
    inventory_parser.set_defaults(function=inventory)
    tree_parser = subparsers.add_parser("tree-summary")
    tree_parser.add_argument("--source", type=Path, required=True)
    tree_parser.add_argument("--summary-output", type=Path, required=True)
    tree_parser.set_defaults(function=tree_summary)
    test_parser = subparsers.add_parser("self-test")
    test_parser.add_argument("--workdir", type=Path, required=True)
    test_parser.add_argument("--result-output", type=Path, required=True)
    test_parser.set_defaults(function=self_test)
    return parser.parse_args()


if __name__ == "__main__":
    parsed = parse_args()
    parsed.function(parsed)
