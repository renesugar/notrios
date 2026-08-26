#!/usr/bin/env python3
"""Strict offline verifier for Notrios evidence checkpoints and ISO reserves.

The verifier intentionally uses only the Python standard library plus the
recorded GnuPG, OpenSSL, and xorriso subprocess boundaries. It never consults
an ambient OpenPGP trust database or an implicit TLS/CA store.
"""
from __future__ import annotations

import argparse
import binascii
import datetime as dt
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import struct
import subprocess
import tempfile
import unicodedata
import zipfile


ENTRY_SCHEMA = "notrios.evidence.manifest-entry.v1"
CHECKPOINT_SCHEMA = "notrios.evidence.content-checkpoint.v1"
CATALOG_ENTRY_SCHEMA = "notrios.evidence.iso-catalog-entry.v1"
CATALOG_CHECKPOINT_SCHEMA = "notrios.evidence.iso-catalog-checkpoint.v1"
ZERO_HASH = "0" * 64
PRIMARY_FINGERPRINT = "AEE5F82F2C216D6D15992C8DC96A1C6039BC8098"
SIGNING_FINGERPRINT = "4ABEB98AF99C8321931BCF282C6A8A4568264005"
TSA_POLICY_OID = "2.16.840.1.114412.7.1"
TSA_ROOT_SHA256 = "552f7bdcf1a7af9e6ce672017f4f12abf77240c78e761ac203d1d9d20ac89988"
TSA_RESPONDER_SHA256 = "4aa03fa22cd75c84c55c938f828e676b9caecab33fe36d269aa334f146110a33"
CD_BUDGET_BYTES = 650 * 1024 * 1024


class EvidenceError(RuntimeError):
    """A fail-closed evidence validation error."""


def run(args: list[str], *, cwd: Path | None = None,
        env: dict[str, str] | None = None, input_bytes: bytes | None = None,
        check: bool = True) -> subprocess.CompletedProcess[bytes]:
    result = subprocess.run(
        args, cwd=cwd, env=env, input=input_bytes, check=False,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    if check and result.returncode != 0:
        raise EvidenceError(f"command failed ({Path(args[0]).name}, exit {result.returncode})")
    return result


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def _check_canonical_value(value: object) -> None:
    if isinstance(value, float):
        raise EvidenceError("floating-point values are outside canonical JSON v1")
    if isinstance(value, str):
        if unicodedata.normalize("NFC", value) != value:
            raise EvidenceError("canonical JSON strings must be NFC")
        return
    if value is None or isinstance(value, bool):
        return
    if isinstance(value, int):
        if value < -(2**63) or value > 2**63 - 1:
            raise EvidenceError("canonical JSON integers must fit signed 64-bit")
        return
    if isinstance(value, list):
        for item in value:
            _check_canonical_value(item)
        return
    if isinstance(value, dict):
        for key, item in value.items():
            if not isinstance(key, str):
                raise EvidenceError("canonical JSON keys must be strings")
            _check_canonical_value(key)
            _check_canonical_value(item)
        return
    raise EvidenceError(f"unsupported canonical JSON type: {type(value).__name__}")


def canonical_bytes(value: object) -> bytes:
    _check_canonical_value(value)
    return (json.dumps(value, ensure_ascii=False, allow_nan=False, sort_keys=True,
                       separators=(",", ":")) + "\n").encode("utf-8")


def _pairs_no_duplicates(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result: dict[str, object] = {}
    for key, value in pairs:
        if key in result:
            raise EvidenceError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_canonical(path: Path) -> object:
    raw = path.read_bytes()
    try:
        value = json.loads(raw.decode("utf-8"), object_pairs_hook=_pairs_no_duplicates,
                           parse_float=lambda _: (_ for _ in ()).throw(
                               EvidenceError("floats are outside canonical JSON v1")))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise EvidenceError(f"invalid JSON: {path.name}") from exc
    if canonical_bytes(value) != raw:
        raise EvidenceError(f"non-canonical JSON: {path.name}")
    return value


def safe_logical_name(name: str) -> bool:
    if not name or name.startswith("/") or "\\" in name or "\0" in name:
        return False
    if unicodedata.normalize("NFC", name) != name:
        return False
    path = PurePosixPath(name)
    return not path.is_absolute() and all(
        part not in ("", ".", "..") and not any(ord(char) < 32 for char in part)
        for part in name.split("/")
    )


def validate_zip(path: Path) -> dict[str, object]:
    with zipfile.ZipFile(path) as archive:
        infos = archive.infolist()
        names = [item.filename for item in infos]
        unsafe = sum(not safe_logical_name(name[:-1] if name.endswith("/") else name)
                     for name in names)
        duplicates = len(names) - len(set(names))
        encrypted = sum(bool(item.flag_bits & 1) for item in infos)
        symlinks = sum(stat.S_ISLNK((item.external_attr >> 16) & 0xFFFF)
                       for item in infos)
        corrupt = archive.testzip()
    return {
        "validator": "python.zipfile-crc-path-v1",
        "valid": not any((unsafe, duplicates, encrypted, symlinks)) and corrupt is None,
        "entries": len(infos),
        "unsafe_names": unsafe,
        "duplicate_names": duplicates,
        "encrypted_entries": encrypted,
        "symlink_entries": symlinks,
        "corrupt_member": corrupt,
    }


def validate_png(path: Path) -> dict[str, object]:
    data = path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        return {"validator": "png-chunk-crc-v1", "valid": False,
                "chunks": 0, "reason": "bad signature"}
    offset = 8
    chunks = 0
    ihdr = False
    iend = False
    reason: str | None = None
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
        if (binascii.crc32(kind + payload) & 0xFFFFFFFF) != expected:
            reason = "chunk CRC mismatch"
            break
        chunks += 1
        ihdr = ihdr or (chunks == 1 and kind == b"IHDR" and length == 13)
        if kind == b"IEND":
            iend = True
            offset = end
            break
        offset = end
    if reason is None and offset != len(data):
        reason = "trailing bytes"
    return {"validator": "png-chunk-crc-v1", "valid": reason is None and ihdr and iend,
            "chunks": chunks, "reason": reason}


def structural_validation(path: Path, media_type: str) -> dict[str, object]:
    if media_type == "application/zip":
        return validate_zip(path)
    if media_type == "image/png":
        return validate_png(path)
    return {"validator": "sha256-size-v1", "valid": True}


def load_chain(path: Path, expected_schema: str = ENTRY_SCHEMA) -> list[dict[str, object]]:
    raw = path.read_bytes()
    if not raw or not raw.endswith(b"\n"):
        raise EvidenceError("manifest must be non-empty and LF-terminated")
    records: list[dict[str, object]] = []
    previous = ZERO_HASH
    for expected_sequence, line in enumerate(raw.splitlines(keepends=True), start=1):
        try:
            record = json.loads(line.decode("utf-8"), object_pairs_hook=_pairs_no_duplicates,
                                parse_float=lambda _: (_ for _ in ()).throw(EvidenceError("float")))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise EvidenceError("invalid manifest JSONL") from exc
        if canonical_bytes(record) != line:
            raise EvidenceError("non-canonical manifest entry")
        if not isinstance(record, dict) or record.get("schema") != expected_schema:
            raise EvidenceError("wrong manifest entry schema")
        if record.get("sequence") != expected_sequence:
            raise EvidenceError("manifest sequence is not contiguous")
        if record.get("previous_entry_sha256") != previous:
            raise EvidenceError("manifest previous-entry hash mismatch")
        core = {key: value for key, value in record.items() if key != "entry_sha256"}
        actual = hashlib.sha256(canonical_bytes(core)).hexdigest()
        if record.get("entry_sha256") != actual:
            raise EvidenceError("manifest entry hash mismatch")
        payload = record.get("payload")
        if not isinstance(payload, dict):
            raise EvidenceError("manifest payload is not an object")
        records.append(record)
        previous = actual
    return records


def gpg_validsig(signature: Path, datum: Path, public_key: Path) -> dict[str, object]:
    with tempfile.TemporaryDirectory(prefix="notrios-evidence-gpg-") as directory:
        home = Path(directory)
        home.chmod(0o700)
        env = os.environ.copy()
        env["GNUPGHOME"] = str(home)
        run(["gpg", "--batch", "--no-autostart", "--quiet", "--import", str(public_key)], env=env)
        result = run(["gpg", "--batch", "--no-autostart", "--status-fd", "1", "--trust-model", "always",
                      "--verify", str(signature), str(datum)], env=env)
    lines = result.stdout.decode("utf-8", "replace").splitlines()
    fields = next((line.split() for line in lines if line.startswith("[GNUPG:] VALIDSIG ")), None)
    if fields is None or len(fields) < 12:
        raise EvidenceError("GnuPG did not emit a complete VALIDSIG record")
    if fields[2] != SIGNING_FINGERPRINT or fields[-1] != PRIMARY_FINGERPRINT:
        raise EvidenceError("OpenPGP signature fingerprint mismatch")
    return {"signing_fingerprint": fields[2], "primary_fingerprint": fields[-1],
            "created_date": fields[3], "created_epoch": int(fields[4]),
            "public_key_algorithm": int(fields[8]), "digest_algorithm": int(fields[9])}


def verify_timestamp(datum: Path, query: Path, response: Path, trust_root: Path,
                     untrusted_chain: Path, responder: Path,
                     expected_policy: str = TSA_POLICY_OID) -> dict[str, object]:
    run(["openssl", "ts", "-verify", "-queryfile", str(query), "-in", str(response),
         "-CAfile", str(trust_root), "-untrusted", str(untrusted_chain)])
    run(["openssl", "ts", "-verify", "-data", str(datum), "-in", str(response),
         "-CAfile", str(trust_root), "-untrusted", str(untrusted_chain)])
    reply = run(["openssl", "ts", "-reply", "-in", str(response), "-text"]).stdout.decode()
    query_text = run(["openssl", "ts", "-query", "-in", str(query), "-text"]).stdout.decode()
    policy = re.search(r"^Policy OID: (\S+)$", reply, re.MULTILINE)
    timestamp = re.search(r"^Time stamp: (.+ GMT)$", reply, re.MULTILINE)
    nonce_reply = re.search(r"^Nonce: (\S+)$", reply, re.MULTILINE)
    nonce_query = re.search(r"^Nonce: (\S+)$", query_text, re.MULTILINE)
    if not policy or policy.group(1) != expected_policy:
        raise EvidenceError("timestamp policy OID mismatch")
    if "Hash Algorithm: sha256" not in reply or "Hash Algorithm: sha256" not in query_text:
        raise EvidenceError("timestamp message imprint is not SHA-256")
    if not nonce_reply or not nonce_query or nonce_reply.group(1).lower() != nonce_query.group(1).lower():
        raise EvidenceError("timestamp nonce is absent or mismatched")
    if not timestamp:
        raise EvidenceError("timestamp response has no genTime")
    generated = dt.datetime.strptime(timestamp.group(1), "%b %d %H:%M:%S %Y GMT").replace(tzinfo=dt.timezone.utc)
    epoch = int(generated.timestamp())
    verify = run(["openssl", "verify", "-attime", str(epoch), "-purpose", "timestampsign",
                  "-CAfile", str(trust_root), "-untrusted", str(untrusted_chain),
                  str(responder)])
    cert_text = run(["openssl", "x509", "-in", str(responder), "-text", "-noout"]).stdout.decode()
    if not re.search(r"X509v3 Extended Key Usage: critical\s+Time Stamping", cert_text):
        raise EvidenceError("TSA responder lacks critical timeStamping EKU")
    fingerprint = run(["openssl", "x509", "-in", str(responder), "-noout",
                       "-fingerprint", "-sha256"]).stdout.decode().strip().split("=", 1)[-1].replace(":", "").lower()
    root_fingerprint = run(["openssl", "x509", "-in", str(trust_root), "-noout",
                            "-fingerprint", "-sha256"]).stdout.decode().strip().split("=", 1)[-1].replace(":", "").lower()
    if fingerprint != TSA_RESPONDER_SHA256 or root_fingerprint != TSA_ROOT_SHA256:
        raise EvidenceError("TSA responder or root fingerprint differs from accepted pilot")
    return {"policy_oid": policy.group(1), "nonce": nonce_reply.group(1).lower(),
            "gen_time": generated.isoformat().replace("+00:00", "Z"),
            "responder_sha256": fingerprint, "certificate_valid_at_gen_time": True,
            "critical_timestamping_eku": True, "openssl_chain_result": verify.stdout.decode().strip()}


def verify_checkpoint(root: Path, *, verify_payload: bool) -> dict[str, object]:
    manifest_path = root / "CHECKPOINT" / "content-manifest.jsonl"
    checkpoint_path = root / "CHECKPOINT" / "content-checkpoint.json"
    signature_path = root / "CHECKPOINT" / "content-checkpoint.json.sig"
    records = load_chain(manifest_path)
    checkpoint = load_canonical(checkpoint_path)
    if not isinstance(checkpoint, dict) or checkpoint.get("schema") != CHECKPOINT_SCHEMA:
        raise EvidenceError("wrong content checkpoint schema")
    raw_manifest = manifest_path.read_bytes()
    expected = {
        "entry_count": len(records),
        "first_entry_sha256": records[0]["entry_sha256"],
        "last_entry_sha256": records[-1]["entry_sha256"],
        "manifest_sha256": hashlib.sha256(raw_manifest).hexdigest(),
        "manifest_size_bytes": len(raw_manifest),
    }
    for key, value in expected.items():
        if checkpoint.get(key) != value:
            raise EvidenceError(f"content checkpoint mismatch: {key}")
    gpg_validsig(signature_path, checkpoint_path, root / "TRUST" / "openpgp-public.asc")
    timestamp = verify_timestamp(
        signature_path,
        root / "CHECKPOINT" / "content-checkpoint.sig.tsq",
        root / "CHECKPOINT" / "content-checkpoint.sig.tsr",
        root / "TRUST" / "tsa-root.pem",
        root / "TRUST" / "tsa-untrusted.pem",
        root / "TRUST" / "tsa-responder.pem",
    )
    artifacts: dict[str, tuple[dict[str, object], str]] = {}
    signatures: dict[str, dict[str, object]] = {}
    for record in records:
        payload = record["payload"]
        kind = payload.get("record_type")
        if kind == "artifact":
            artifact_id = payload.get("artifact_id")
            logical = payload.get("logical_name")
            if not isinstance(artifact_id, str) or not isinstance(logical, str) or not safe_logical_name(logical):
                raise EvidenceError("invalid artifact identity or logical name")
            if payload.get("assigned_checkpoint_id") != "g17b-backfill-20260825-0001" or payload.get("assigned_volume_id") != "NTR-EV-0001":
                raise EvidenceError("artifact is not assigned to the current checkpoint and volume")
            if artifact_id in artifacts:
                raise EvidenceError("duplicate artifact identity")
            artifacts[artifact_id] = (payload, str(record["entry_sha256"]))
        elif kind == "signature":
            artifact_id = payload.get("artifact_id")
            logical = payload.get("logical_name")
            if not isinstance(artifact_id, str) or not isinstance(logical, str) or not safe_logical_name(logical):
                raise EvidenceError("invalid signature identity or logical name")
            if artifact_id in signatures:
                raise EvidenceError("duplicate artifact signature")
            if payload.get("primary_fingerprint") != PRIMARY_FINGERPRINT or payload.get("signing_fingerprint") != SIGNING_FINGERPRINT:
                raise EvidenceError("signature metadata fingerprint mismatch")
            signatures[artifact_id] = payload
        else:
            raise EvidenceError("unknown manifest record type")
    if set(artifacts) != set(signatures):
        raise EvidenceError("artifact/signature coverage mismatch")
    expected_signature_paths = {str(payload["logical_name"]) for payload in signatures.values()}
    actual_signature_paths = {
        path.relative_to(root).as_posix() for path in (root / "SIGNATURES").rglob("*") if path.is_file()
    }
    if actual_signature_paths != expected_signature_paths:
        raise EvidenceError("checked signature tree membership mismatch")
    for artifact_id, signature in signatures.items():
        signature_path = root / str(signature["logical_name"])
        if signature_path.stat().st_size != signature.get("size_bytes") or sha256_file(signature_path) != signature.get("sha256"):
            raise EvidenceError("checked signature size/hash mismatch")
        if signature.get("artifact_entry_sha256") != artifacts[artifact_id][1]:
            raise EvidenceError("signature references the wrong artifact entry")
    if verify_payload:
        expected_paths = {str(payload[0]["logical_name"]) for payload in artifacts.values()}
        expected_paths.update(str(payload["logical_name"]) for payload in signatures.values())
        actual_paths = {
            path.relative_to(root).as_posix() for prefix in ("PAYLOAD", "SIGNATURES")
            for path in (root / prefix).rglob("*") if path.is_file()
        }
        if actual_paths != expected_paths:
            raise EvidenceError("payload/signature tree membership mismatch")
        for artifact_id, artifact_pair in artifacts.items():
            artifact = artifact_pair[0]
            artifact_path = root / str(artifact["logical_name"])
            signature = signatures[artifact_id]
            signature_path = root / str(signature["logical_name"])
            for payload, path in ((artifact, artifact_path), (signature, signature_path)):
                if path.stat().st_size != payload.get("size_bytes") or sha256_file(path) != payload.get("sha256"):
                    raise EvidenceError(f"size/hash mismatch: {path.name}")
            validation = structural_validation(artifact_path, str(artifact["media_type"]))
            if not validation["valid"] or validation != artifact.get("structural_validation"):
                raise EvidenceError(f"structural validation mismatch: {artifact_path.name}")
            result = gpg_validsig(signature_path, artifact_path, root / "TRUST" / "openpgp-public.asc")
            if result["signing_fingerprint"] != signature.get("signing_fingerprint"):
                raise EvidenceError("artifact signature record fingerprint mismatch")
        support = {
            "README.txt", "TOOLS/verify-evidence.py",
            "SCHEMAS/manifest-entry-v1.schema.json", "SCHEMAS/content-checkpoint-v1.schema.json",
            "CUSTODY/CUSTODY_TEMPLATE.json", "CUSTODY/BURN_AND_READBACK.md",
            "CHECKPOINT/content-manifest.jsonl", "CHECKPOINT/content-checkpoint.json",
            "CHECKPOINT/content-checkpoint.json.sig", "CHECKPOINT/content-checkpoint.sig.tsq",
            "CHECKPOINT/content-checkpoint.sig.tsr", "CHECKPOINT/timestamp-verification.json",
            "CHECKPOINT/seal-materials.json", "TRUST/openpgp-public.asc",
            "TRUST/tsa-root.pem", "TRUST/tsa-untrusted.pem", "TRUST/tsa-responder.pem",
        }
        all_files = {path.relative_to(root).as_posix() for path in root.rglob("*") if path.is_file()}
        if all_files != expected_paths | support:
            raise EvidenceError("complete evidence tree has missing or extra files")
    timestamp_record = load_canonical(root / "CHECKPOINT" / "timestamp-verification.json")
    if not isinstance(timestamp_record, dict):
        raise EvidenceError("timestamp verification record is not an object")
    for key in ("policy_oid", "nonce", "gen_time", "responder_sha256",
                "certificate_valid_at_gen_time", "critical_timestamping_eku"):
        if timestamp_record.get(key) != timestamp.get(key):
            raise EvidenceError(f"timestamp verification record mismatch: {key}")
    materials = load_canonical(root / "CHECKPOINT" / "seal-materials.json")
    if not isinstance(materials, dict) or materials.get("schema") != "notrios.evidence.seal-materials.v1":
        raise EvidenceError("seal-materials record is invalid")
    for item in materials.get("materials", []):
        if not isinstance(item, dict) or not isinstance(item.get("logical_name"), str) or not safe_logical_name(item["logical_name"]):
            raise EvidenceError("invalid seal-materials member")
        path = root / item["logical_name"]
        if not path.is_file() or path.stat().st_size != item.get("size_bytes") or sha256_file(path) != item.get("sha256"):
            raise EvidenceError("seal support material differs")
    return {"entries": len(records), "artifacts": len(artifacts),
            "manifest_sha256": expected["manifest_sha256"], "timestamp": timestamp}


def extract_iso(iso: Path, destination: Path) -> None:
    if any(destination.iterdir()):
        raise EvidenceError("ISO extraction destination must be empty")
    run(["xorriso", "-no_rc", "-osirrox", "on", "-indev", str(iso),
         "-extract", "/", str(destination)])
    for path in destination.rglob("*"):
        mode = path.lstat().st_mode
        if stat.S_ISLNK(mode) or (not stat.S_ISDIR(mode) and not stat.S_ISREG(mode)):
            raise EvidenceError("ISO contains a symlink or special file")


def load_catalog(path: Path) -> list[dict[str, object]]:
    return load_chain(path, CATALOG_ENTRY_SCHEMA)


def verify_reserve(reserve_root: Path, catalog_path: Path, catalog_checkpoint: Path,
                   catalog_signature: Path, catalog_query: Path, catalog_response: Path,
                   trust_root: Path, untrusted: Path, responder: Path,
                   public_key: Path) -> dict[str, object]:
    records = load_catalog(catalog_path)
    checkpoint = load_canonical(catalog_checkpoint)
    if not isinstance(checkpoint, dict) or checkpoint.get("schema") != CATALOG_CHECKPOINT_SCHEMA:
        raise EvidenceError("wrong ISO catalog checkpoint schema")
    raw = catalog_path.read_bytes()
    for key, value in {
        "entry_count": len(records), "catalog_sha256": hashlib.sha256(raw).hexdigest(),
        "catalog_size_bytes": len(raw), "last_entry_sha256": records[-1]["entry_sha256"],
    }.items():
        if checkpoint.get(key) != value:
            raise EvidenceError(f"ISO catalog checkpoint mismatch: {key}")
    gpg_validsig(catalog_signature, catalog_checkpoint, public_key)
    verify_timestamp(catalog_signature, catalog_query, catalog_response,
                     trust_root, untrusted, responder)
    verified: list[str] = []
    for record in records:
        payload = record["payload"]
        relative = payload.get("reserve_relative_path")
        if not isinstance(relative, str) or not safe_logical_name(relative):
            raise EvidenceError("unsafe reserve-relative ISO path")
        iso = reserve_root / relative
        if not iso.is_file() or iso.stat().st_size != payload.get("size_bytes") or sha256_file(iso) != payload.get("sha256"):
            raise EvidenceError("reserved ISO is absent or differs")
        pvd_result = run(["xorriso", "-no_rc", "-indev", str(iso), "-pvd_info"])
        pvd = (pvd_result.stdout + pvd_result.stderr).decode()
        volume = re.search(r"^Volume Id\s+: '([^']+)'", pvd, re.MULTILINE)
        if not volume or volume.group(1) != payload.get("volume_id"):
            raise EvidenceError("reserved ISO volume identifier differs")
        signature = iso.with_suffix(iso.suffix + ".sig")
        if sha256_file(signature) != payload.get("signature_sha256"):
            raise EvidenceError("reserved ISO signature differs")
        if sha256_file(iso.with_suffix(iso.suffix + ".sig.tsq")) != payload.get("timestamp_query_sha256") or sha256_file(iso.with_suffix(iso.suffix + ".sig.tsr")) != payload.get("timestamp_response_sha256"):
            raise EvidenceError("reserved ISO timestamp material differs")
        gpg_validsig(signature, iso, public_key)
        verify_timestamp(signature, iso.with_suffix(iso.suffix + ".sig.tsq"),
                         iso.with_suffix(iso.suffix + ".sig.tsr"), trust_root,
                         untrusted, responder)
        with tempfile.TemporaryDirectory(prefix="notrios-evidence-iso-") as directory:
            extracted = Path(directory)
            extract_iso(iso, extracted)
            result = verify_checkpoint(extracted, verify_payload=True)
        if result["manifest_sha256"] != payload.get("content_manifest_sha256"):
            raise EvidenceError("ISO content manifest differs from catalog")
        verified.append(str(payload["volume_id"]))
    return {"volumes": verified, "catalog_sha256": hashlib.sha256(raw).hexdigest(),
            "closure_boundary": "catalog-only commit is outside the current ISO"}


def _default_tracked_root(repo: Path) -> Path:
    return repo / "evidence" / "current"


def verify_source(repo: Path, source_root: Path) -> dict[str, object]:
    root = _default_tracked_root(repo)
    records = load_chain(root / "CHECKPOINT" / "content-manifest.jsonl")
    artifacts = [record["payload"] for record in records if record["payload"].get("record_type") == "artifact"]
    signatures = {record["payload"]["artifact_id"]: record["payload"] for record in records
                  if record["payload"].get("record_type") == "signature"}
    expected = {str(item["logical_name"]).removeprefix("PAYLOAD/") for item in artifacts}
    actual = {path.name for path in source_root.iterdir() if path.is_file()}
    if actual != expected or any(path.is_symlink() for path in source_root.iterdir()):
        raise EvidenceError("curated source membership drift")
    for artifact in artifacts:
        path = source_root / str(artifact["logical_name"]).removeprefix("PAYLOAD/")
        if path.stat().st_size != artifact.get("size_bytes") or sha256_file(path) != artifact.get("sha256"):
            raise EvidenceError("curated source byte drift")
        validation = structural_validation(path, str(artifact["media_type"]))
        if validation != artifact.get("structural_validation") or not validation["valid"]:
            raise EvidenceError("curated source structural drift")
        signature = signatures[str(artifact["artifact_id"])]
        gpg_validsig(root / str(signature["logical_name"]), path, root / "TRUST" / "openpgp-public.asc")
    return {"artifacts": len(artifacts), "source_drift": False}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    tracked = sub.add_parser("tracked", help="verify checked-in checkpoint material")
    tracked.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[1])
    payload = sub.add_parser("payload", help="verify a staged or extracted complete checkpoint")
    payload.add_argument("root", type=Path)
    source = sub.add_parser("source", help="verify the live curated source against the checkpoint")
    source.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[1])
    source.add_argument("--source-root", type=Path, required=True)
    reserve = sub.add_parser("reserve", help="verify external ISO reserve and checked-in catalog")
    reserve.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[1])
    reserve.add_argument("--reserve-root", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "tracked":
        root = _default_tracked_root(args.repo)
        result = verify_checkpoint(root, verify_payload=False)
    elif args.command == "payload":
        result = verify_checkpoint(args.root, verify_payload=True)
    elif args.command == "source":
        result = verify_source(args.repo, args.source_root)
    else:
        repo = args.repo
        current = _default_tracked_root(repo)
        result = verify_reserve(
            args.reserve_root, repo / "evidence" / "outer-iso-catalog.jsonl",
            repo / "evidence" / "outer-iso-catalog-checkpoint.json",
            repo / "evidence" / "outer-iso-catalog-checkpoint.json.sig",
            repo / "evidence" / "outer-iso-catalog-checkpoint.sig.tsq",
            repo / "evidence" / "outer-iso-catalog-checkpoint.sig.tsr",
            current / "TRUST" / "tsa-root.pem", current / "TRUST" / "tsa-untrusted.pem",
            current / "TRUST" / "tsa-responder.pem", current / "TRUST" / "openpgp-public.asc",
        )
    print(json.dumps({"status": "verified", **result}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (EvidenceError, OSError, zipfile.BadZipFile) as exc:
        raise SystemExit(f"evidence verification refused: {exc}")
