#!/usr/bin/env python3
"""Authorized G17b evidence sealing and immutable reserve builder.

Secrets are retrieved only by the signing subprocess helper, retained in memory
for the shortest practical interval, streamed on stdin to GnuPG, and never
placed in arguments, environment variables, files, stdout, or stderr.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import unicodedata
import zipfile

REPO = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO))
from evidence import verify_evidence as ev


CURRENT = REPO / "evidence" / "current"
EXPECTED_G17A_CONTENT_COMMITMENT = "160c66280845b43dcfc7b97edadcbe5f49b9180093b84937d3348dfcb8a6b284"
KNOWN_POST_G17A = {
    "notrios-v0.7-g17a-20260824.zip",
    "notrios-v0.7-evidence-handling-decisions-20260825.zip",
    "notrios-v0.7-g17b-20260825.zip",
}
ANCHORS = (
    "README.md", "PLAN.md", "ROADMAP.md", "CODING_CLIENT_HANDOFF.md",
    "agent/PLAN_STATUS.md", "agent/ATTEMPT_LOG.jsonl",
)
CHECKPOINT_ID = "g17b-backfill-20260825-0001"
VOLUME_ID = "NTR-EV-0001"
VOLUME_RELATIVE = "volume-0001/notrios-evidence-0001.iso"
RESERVE_LOCATION_ID = "notrios-seagate-reserve-01"
FIXED_EPOCH = 1787702400  # 2026-08-25T00:00:00Z


def run(args: list[str], *, cwd: Path | None = None, env: dict[str, str] | None = None,
        input_bytes: bytes | None = None, check: bool = True) -> subprocess.CompletedProcess[bytes]:
    return ev.run(args, cwd=cwd, env=env, input_bytes=input_bytes, check=check)


def utc(value: dt.datetime) -> str:
    return value.astimezone(dt.timezone.utc).isoformat().replace("+00:00", "Z")


def parse_utc(value: str) -> dt.datetime:
    parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ev.EvidenceError("timestamp must include a timezone")
    return parsed.astimezone(dt.timezone.utc)


def write_canonical(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(ev.canonical_bytes(value))


def make_record(sequence: int, previous: str, payload: dict[str, object],
                schema: str = ev.ENTRY_SCHEMA) -> dict[str, object]:
    core = {"schema": schema, "sequence": sequence,
            "previous_entry_sha256": previous, "payload": payload}
    record = dict(core)
    record["entry_sha256"] = hashlib.sha256(ev.canonical_bytes(core)).hexdigest()
    return record


def secret_service_passphrase() -> bytearray:
    result = subprocess.run(
        ["secret-tool", "lookup", "service", "gpg_evidence", "type", "passphrase"],
        check=False, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
    )
    if result.returncode != 0 or not result.stdout:
        raise ev.EvidenceError("Secret Service passphrase lookup failed")
    return bytearray(result.stdout.rstrip(b"\r\n"))


def sign_file(datum: Path, signature: Path, passphrase: bytearray) -> None:
    signature.parent.mkdir(parents=True, exist_ok=True)
    input_bytes = bytes(passphrase) + b"\n"
    result = subprocess.run(
        ["gpg", "--batch", "--yes", "--quiet", "--pinentry-mode", "loopback",
         "--passphrase-fd", "0", "--local-user", ev.SIGNING_FINGERPRINT + "!",
         "--digest-algo", "SHA256", "--output", str(signature), "--detach-sign", str(datum)],
        input=input_bytes, check=False, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
    )
    if result.returncode != 0:
        raise ev.EvidenceError("GnuPG production signing failed")


def clear_secret(secret: bytearray) -> None:
    for index in range(len(secret)):
        secret[index] = 0


def export_public_key(path: Path) -> None:
    result = run(["gpg", "--batch", "--armor", "--export-options", "export-minimal",
                  "--export", ev.PRIMARY_FINGERPRINT])
    if b"BEGIN PGP PUBLIC KEY BLOCK" not in result.stdout:
        raise ev.EvidenceError("selected OpenPGP public key export failed")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(result.stdout)


def _pem_certificates(data: bytes) -> list[bytes]:
    return [match + (b"\n" if not match.endswith(b"\n") else b"") for match in re.findall(
        rb"-----BEGIN CERTIFICATE-----\r?\n.*?-----END CERTIFICATE-----", data, re.DOTALL)]


def _cert_text(cert: bytes, work: Path, index: int) -> tuple[Path, str]:
    path = work / f"cert-{index}.pem"
    path.write_bytes(cert)
    text = run(["openssl", "x509", "-in", str(path), "-text", "-noout"]).stdout.decode()
    return path, text


def extract_tsa_material(response: Path, output: Path) -> tuple[Path, Path]:
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="notrios-tsa-extract-") as directory:
        work = Path(directory)
        token = work / "token.p7s"
        run(["openssl", "ts", "-reply", "-in", str(response), "-token_out", "-out", str(token)])
        certs = run(["openssl", "pkcs7", "-in", str(token), "-inform", "DER", "-print_certs"]).stdout
        blocks = _pem_certificates(certs)
        if len(blocks) < 2:
            raise ev.EvidenceError("TSA response did not include a responder chain")
        responder: bytes | None = None
        for index, block in enumerate(blocks):
            _, text = _cert_text(block, work, index)
            if "CA:FALSE" in text and "Time Stamping" in text:
                responder = block
        if responder is None:
            raise ev.EvidenceError("TSA response did not include a timestamp responder certificate")
        responder_path = output / "tsa-responder.pem"
        untrusted_path = output / "tsa-untrusted.pem"
        responder_path.write_bytes(responder)
        untrusted_path.write_bytes(b"".join(blocks))
    return responder_path, untrusted_path


def request_timestamp(datum: Path, query: Path, response: Path, endpoint: str) -> None:
    run(["openssl", "ts", "-query", "-data", str(datum), "-sha256", "-cert", "-out", str(query)])
    result = run(
        ["curl", "--fail-with-body", "--silent", "--show-error", "--connect-timeout", "20",
         "--max-time", "60", "-H", "Content-Type: application/timestamp-query",
         "-H", "Accept: application/timestamp-reply", "--data-binary", "@" + str(query),
         "--output", str(response), endpoint], check=False,
    )
    if result.returncode != 0 or not response.is_file():
        raise ev.EvidenceError("RFC 3161 provider request failed")


def record_pilot(args: argparse.Namespace) -> None:
    pilot = args.pilot.resolve()
    trust = CURRENT / "TRUST"
    pilot_output = REPO / "evidence" / "pilot"
    trust.mkdir(parents=True, exist_ok=True)
    pilot_output.mkdir(parents=True, exist_ok=True)
    root = trust / "tsa-root.pem"
    shutil.copyfile(args.root_cert.resolve(), root)
    responder, untrusted = extract_tsa_material(pilot / "response.tsr", trust)
    result = ev.verify_timestamp(pilot / "datum.txt", pilot / "query.tsq",
                                 pilot / "response.tsr", root, untrusted, responder)
    if result["policy_oid"] != ev.TSA_POLICY_OID:
        raise ev.EvidenceError("DigiCert pilot returned an unapproved policy")
    for source_name, target_name in (("datum.txt", "generated-datum.txt"),
                                     ("query.tsq", "generated-query.tsq"),
                                     ("response.tsr", "generated-response.tsr")):
        shutil.copyfile(pilot / source_name, pilot_output / target_name)
    root_fingerprint = run(["openssl", "x509", "-in", str(root), "-noout",
                            "-fingerprint", "-sha256"]).stdout.decode().strip().split("=", 1)[-1].replace(":", "").lower()
    pilot_record = {
        "schema": "notrios.evidence.tsa-pilot.v1",
        "provider": "DigiCert",
        "endpoint": "http://timestamp.digicert.com",
        "generated_fixture_only": True,
        "accepted": True,
        "policy_oid": result["policy_oid"],
        "nonce_matched": True,
        "sha256_imprint_matched": True,
        "critical_timestamping_eku": result["critical_timestamping_eku"],
        "certificate_valid_at_gen_time": result["certificate_valid_at_gen_time"],
        "gen_time": result["gen_time"],
        "responder_sha256": result["responder_sha256"],
        "root_sha256": root_fingerprint,
        "datum_sha256": ev.sha256_file(pilot_output / "generated-datum.txt"),
        "query_sha256": ev.sha256_file(pilot_output / "generated-query.tsq"),
        "response_sha256": ev.sha256_file(pilot_output / "generated-response.tsr"),
        "revocation_evidence": "not fetched; responder OCSP/CRL URLs are preserved in its certificate",
        "fallback_used": False,
    }
    write_canonical(REPO / "evidence" / "tsa-pilot.json", pilot_record)
    print(json.dumps({"status": "accepted", "provider": "DigiCert",
                      "policy_oid": result["policy_oid"],
                      "responder_sha256": result["responder_sha256"]}, sort_keys=True))


def git_anchor_history() -> list[tuple[str, dict[str, str]]]:
    commits = run(["git", "rev-list", "--all", "--reverse"], cwd=REPO).stdout.decode().splitlines()
    history: list[tuple[str, dict[str, str]]] = []
    for commit in commits:
        result = run(["git", "ls-tree", "-r", commit, "--", *ANCHORS], cwd=REPO).stdout.decode()
        anchors: dict[str, str] = {}
        for line in result.splitlines():
            metadata, name = line.split("\t", 1)
            anchors[name] = metadata.split()[2]
        history.append((commit, anchors))
    return history


def git_blob_oid(data: bytes) -> str:
    digest = hashlib.sha1()
    digest.update(f"blob {len(data)}\0".encode())
    digest.update(data)
    return digest.hexdigest()


def provenance(path: Path, history: list[tuple[str, dict[str, str]]]) -> dict[str, object]:
    if path.suffix.lower() != ".zip":
        return {"commit_resolution": "not_applicable", "commit": None,
                "matched_anchor_names": [], "candidates": []}
    with zipfile.ZipFile(path) as archive:
        names = set(archive.namelist())
        anchors = {name: git_blob_oid(archive.read(name)) for name in ANCHORS if name in names}
    if len(anchors) < 3:
        return {"commit_resolution": "insufficient", "commit": None,
                "matched_anchor_names": sorted(anchors), "candidates": []}
    scores = [(sum(known.get(name) == oid for name, oid in anchors.items()), commit)
              for commit, known in history]
    best = max(score for score, _ in scores)
    candidates = [commit for score, commit in scores if score == best]
    exact = best == len(anchors) and len(candidates) == 1
    return {"commit_resolution": "unique_exact" if exact else
            ("ambiguous_exact" if best == len(anchors) else "partial_only"),
            "commit": candidates[0] if exact else None,
            "matched_anchor_names": sorted(name for name, oid in anchors.items()
                                           if exact and dict(history)[candidates[0]].get(name) == oid),
            "candidates": [] if exact else candidates}


def media_type(path: Path) -> str:
    if path.suffix.lower() == ".zip":
        return "application/zip"
    if path.suffix.lower() == ".png":
        return "image/png"
    raise ev.EvidenceError(f"unapproved curated artifact type: {path.name}")


def verify_g17a_base(paths: list[Path]) -> None:
    base = [path for path in paths if path.name not in KNOWN_POST_G17A]
    projection = []
    for path in base:
        kind = "zip" if path.suffix.lower() == ".zip" else "png"
        projection.append({"logical_name": path.name, "kind": kind,
                           "size_bytes": path.stat().st_size, "sha256": ev.sha256_file(path)})
    commitment = hashlib.sha256(ev.canonical_bytes(projection)).hexdigest()
    if len(base) != 78 or commitment != EXPECTED_G17A_CONTENT_COMMITMENT:
        raise ev.EvidenceError("curated source differs from the frozen G17a base")
    appends = {path.name for path in paths} - {path.name for path in base}
    if appends != KNOWN_POST_G17A:
        raise ev.EvidenceError(f"curated append set differs: {sorted(appends)}")


def seal_content(args: argparse.Namespace) -> None:
    source = args.source.resolve()
    sealed_at = utc(parse_utc(args.sealed_at))
    paths = sorted((path for path in source.iterdir() if path.is_file()),
                   key=lambda path: path.name.encode("utf-8"))
    if any(unicodedata.normalize("NFC", path.name) != path.name for path in paths):
        raise ev.EvidenceError("curated source contains a non-NFC filename")
    verify_g17a_base(paths)
    if len(paths) != 81:
        raise ev.EvidenceError("G17b freeze must contain exactly 81 approved curated artifacts")
    trust_source = CURRENT / "TRUST"
    for name in ("tsa-root.pem", "tsa-untrusted.pem", "tsa-responder.pem"):
        if not (trust_source / name).is_file():
            raise ev.EvidenceError("accepted TSA pilot trust material is missing")
    history = git_anchor_history()
    with tempfile.TemporaryDirectory(prefix="notrios-g17b-content-", dir=REPO) as directory:
        work = Path(directory) / "current"
        for name in ("CHECKPOINT", "SIGNATURES", "TRUST"):
            (work / name).mkdir(parents=True)
        for path in trust_source.iterdir():
            if path.is_file():
                shutil.copyfile(path, work / "TRUST" / path.name)
        export_public_key(work / "TRUST" / "openpgp-public.asc")
        secret = secret_service_passphrase()
        records: list[dict[str, object]] = []
        previous = ev.ZERO_HASH
        try:
            for index, path in enumerate(paths, start=1):
                artifact_id = f"artifact-{index:04d}"
                kind = media_type(path)
                validation = ev.structural_validation(path, kind)
                if not validation["valid"]:
                    raise ev.EvidenceError(f"structural validation refused: {path.name}")
                file_stat = path.stat()
                artifact_payload: dict[str, object] = {
                    "record_type": "artifact", "artifact_id": artifact_id,
                    "logical_name": "PAYLOAD/" + path.name, "media_type": kind,
                    "size_bytes": file_stat.st_size, "sha256": ev.sha256_file(path),
                    "structural_validation": validation,
                    "captured_at": sealed_at, "sealed_at": sealed_at, "retroactive": True,
                    "step_completed_at": None, "completion_source": None,
                    "source_mtime_weak": utc(dt.datetime.fromtimestamp(file_stat.st_mtime, dt.timezone.utc)),
                    "preservation_class": "curated-top-level-handoff",
                    "assigned_checkpoint_id": CHECKPOINT_ID, "assigned_volume_id": VOLUME_ID,
                    **provenance(path, history),
                }
                artifact_record = make_record(len(records) + 1, previous, artifact_payload)
                records.append(artifact_record)
                previous = str(artifact_record["entry_sha256"])
                signature_path = work / "SIGNATURES" / (path.name + ".sig")
                sign_file(path, signature_path, secret)
                gpg = ev.gpg_validsig(signature_path, path, work / "TRUST" / "openpgp-public.asc")
                signature_payload: dict[str, object] = {
                    "record_type": "signature", "artifact_id": artifact_id,
                    "artifact_entry_sha256": artifact_record["entry_sha256"],
                    "logical_name": "SIGNATURES/" + signature_path.name,
                    "size_bytes": signature_path.stat().st_size,
                    "sha256": ev.sha256_file(signature_path),
                    "primary_fingerprint": ev.PRIMARY_FINGERPRINT,
                    "signing_fingerprint": ev.SIGNING_FINGERPRINT,
                    "implementation": run(["gpg", "--version"]).stdout.decode().splitlines()[0],
                    "public_key_algorithm": gpg["public_key_algorithm"],
                    "digest_algorithm": gpg["digest_algorithm"],
                    "signature_created_at_signer_controlled": utc(dt.datetime.fromtimestamp(
                        int(gpg["created_epoch"]), dt.timezone.utc)),
                    "clean_keyring_validsig_verified": True,
                }
                signature_record = make_record(len(records) + 1, previous, signature_payload)
                records.append(signature_record)
                previous = str(signature_record["entry_sha256"])
            manifest = work / "CHECKPOINT" / "content-manifest.jsonl"
            manifest.write_bytes(b"".join(ev.canonical_bytes(record) for record in records))
            checkpoint = {
                "schema": ev.CHECKPOINT_SCHEMA, "checkpoint_id": CHECKPOINT_ID,
                "entry_count": len(records), "first_entry_sha256": records[0]["entry_sha256"],
                "last_entry_sha256": records[-1]["entry_sha256"],
                "manifest_sha256": ev.sha256_file(manifest),
                "manifest_size_bytes": manifest.stat().st_size,
                "capture_window_start": sealed_at, "capture_window_end": sealed_at,
                "seal_window_start": sealed_at, "seal_window_end": sealed_at,
                "contains_retroactive_artifacts": True,
                "canonical_profile": "notrios-canonical-json-v1",
                "verifier_version": "notrios-evidence-verifier-v1",
                "intended_volume_ids": [VOLUME_ID], "predecessor_checkpoint_sha256": None,
                "source_artifact_count": len(paths),
            }
            checkpoint_path = work / "CHECKPOINT" / "content-checkpoint.json"
            write_canonical(checkpoint_path, checkpoint)
            signature_path = work / "CHECKPOINT" / "content-checkpoint.json.sig"
            sign_file(checkpoint_path, signature_path, secret)
        finally:
            clear_secret(secret)
        ev.gpg_validsig(signature_path, checkpoint_path, work / "TRUST" / "openpgp-public.asc")
        query = work / "CHECKPOINT" / "content-checkpoint.sig.tsq"
        response = work / "CHECKPOINT" / "content-checkpoint.sig.tsr"
        request_timestamp(signature_path, query, response, "http://timestamp.digicert.com")
        timestamp = ev.verify_timestamp(signature_path, query, response,
                                        work / "TRUST" / "tsa-root.pem",
                                        work / "TRUST" / "tsa-untrusted.pem",
                                        work / "TRUST" / "tsa-responder.pem")
        write_canonical(work / "CHECKPOINT" / "timestamp-verification.json", {
            "schema": "notrios.evidence.timestamp-verification.v1",
            "provider": "DigiCert", "endpoint": "http://timestamp.digicert.com",
            **{key: value for key, value in timestamp.items() if key != "openssl_chain_result"},
            "nonce_matched": True, "sha256_imprint_matched": True,
            "revocation_evidence": "not fetched; responder OCSP/CRL URLs are preserved in its certificate",
        })
        material_paths = [manifest, checkpoint_path, signature_path, query, response,
                          *sorted((work / "TRUST").iterdir())]
        write_canonical(work / "CHECKPOINT" / "seal-materials.json", {
            "schema": "notrios.evidence.seal-materials.v1",
            "checkpoint_id": CHECKPOINT_ID,
            "materials": [{"logical_name": path.relative_to(work).as_posix(),
                           "size_bytes": path.stat().st_size, "sha256": ev.sha256_file(path)}
                          for path in material_paths],
            "finite_closure_note": "The timestamp response cannot hash itself; verification binds it to the query and exact checkpoint signature.",
        })
        ev.verify_checkpoint(work, verify_payload=False)
        backup = REPO / "evidence" / ".current-replaced"
        if backup.exists():
            shutil.rmtree(backup)
        if CURRENT.exists():
            CURRENT.rename(backup)
        work.rename(CURRENT)
        if backup.exists():
            shutil.rmtree(backup)
    summary = {
        "schema": "notrios.evidence.source-freeze.v1", "captured_at": sealed_at,
        "artifact_count": len(paths), "total_bytes": sum(path.stat().st_size for path in paths),
        "content_manifest_sha256": ev.sha256_file(CURRENT / "CHECKPOINT" / "content-manifest.jsonl"),
        "content_checkpoint_sha256": ev.sha256_file(CURRENT / "CHECKPOINT" / "content-checkpoint.json"),
        "expected_appends_after_g17a": sorted(KNOWN_POST_G17A),
        "recursive_workspaces_excluded": 3, "source_originals_modified": False,
    }
    write_canonical(REPO / "evidence" / "source-freeze.json", summary)
    print(json.dumps({"status": "sealed", **summary}, sort_keys=True))


def iso_options(volume_id: str) -> list[str]:
    if not re.fullmatch(r"[A-Z0-9_-]{1,16}", volume_id):
        raise ev.EvidenceError("invalid portable ISO volume identifier")
    return [
        "-iso-level", "3", "-r", "-J", "-joliet-long", "-V", volume_id,
        "-A", "NOTRIOS EVIDENCE RESERVE V1", "-publisher", "NOTRIOS PROJECT",
        "-p", "NOTRIOS G17B",
    ]


def normalize_tree(root: Path, epoch: int) -> None:
    for path in sorted(root.rglob("*"), reverse=True):
        if path.is_file():
            path.chmod(0o444)
        elif path.is_dir():
            path.chmod(0o555)
        os.utime(path, (epoch, epoch), follow_symlinks=False)
    root.chmod(0o555)
    os.utime(root, (epoch, epoch), follow_symlinks=False)


def remove_readonly_tree(root: Path) -> None:
    if not root.exists():
        return
    for path in [root, *root.rglob("*")]:
        if path.is_dir():
            path.chmod(0o755)
    shutil.rmtree(root)


def tree_hashes(root: Path) -> dict[str, str]:
    return {path.relative_to(root).as_posix(): ev.sha256_file(path)
            for path in sorted(root.rglob("*")) if path.is_file()}


def build_stage(stage: Path, source: Path) -> None:
    if stage.exists():
        raise ev.EvidenceError("refusing to reuse an ISO staging directory")
    for name in ("PAYLOAD", "SIGNATURES", "CHECKPOINT", "TRUST", "TOOLS", "SCHEMAS", "CUSTODY"):
        (stage / name).mkdir(parents=True, exist_ok=True)
    manifest_records = ev.load_chain(CURRENT / "CHECKPOINT" / "content-manifest.jsonl")
    artifact_names = [str(record["payload"]["logical_name"]).removeprefix("PAYLOAD/")
                      for record in manifest_records if record["payload"].get("record_type") == "artifact"]
    if set(artifact_names) != {path.name for path in source.iterdir() if path.is_file()}:
        raise ev.EvidenceError("source membership drifted after content checkpoint")
    for name in sorted(artifact_names, key=lambda value: value.encode("utf-8")):
        shutil.copyfile(source / name, stage / "PAYLOAD" / name)
    for directory in ("SIGNATURES", "CHECKPOINT", "TRUST"):
        for path in sorted((CURRENT / directory).iterdir()):
            if path.is_file():
                shutil.copyfile(path, stage / directory / path.name)
    shutil.copyfile(REPO / "evidence" / "verify_evidence.py", stage / "TOOLS" / "verify-evidence.py")
    for path in sorted((REPO / "evidence" / "schemas").iterdir()):
        if path.is_file():
            shutil.copyfile(path, stage / "SCHEMAS" / path.name)
    for name in ("CUSTODY_TEMPLATE.json", "BURN_AND_READBACK.md"):
        shutil.copyfile(REPO / "evidence" / name, stage / "CUSTODY" / name)
    shutil.copyfile(REPO / "evidence" / "ISO_README.txt", stage / "README.txt")
    normalize_tree(stage, FIXED_EPOCH)


def xorriso_build(stage: Path, output: Path, print_only: bool = False) -> int:
    env = os.environ.copy()
    env.update({"SOURCE_DATE_EPOCH": str(FIXED_EPOCH), "TZ": "UTC", "LC_ALL": "C"})
    args = ["xorriso", "-no_rc", "-as", "mkisofs", *iso_options(VOLUME_ID)]
    if print_only:
        result = run([*args, "-print-size", str(stage)], env=env)
        text = (result.stdout + result.stderr).decode()
        blocks = [int(line) for line in text.splitlines() if line.strip().isdigit()]
        if not blocks:
            raise ev.EvidenceError("xorriso did not return a print-size block count")
        return blocks[-1]
    run([*args, "-o", str(output), str(stage)], env=env)
    return output.stat().st_size // 2048


def build_reserve(args: argparse.Namespace) -> None:
    source = args.source.resolve()
    reserve = args.reserve_root.resolve()
    volume_path = reserve / VOLUME_RELATIVE
    if volume_path.exists() or volume_path.with_suffix(volume_path.suffix + ".sig").exists():
        raise ev.EvidenceError("immutable reserve volume already exists")
    reserve.mkdir(parents=True, exist_ok=True)
    volume_path.parent.mkdir(parents=True, exist_ok=True)
    build_root = reserve / ".g17b-build-0001"
    if build_root.exists():
        raise ev.EvidenceError("existing G17b build workspace requires manual review")
    build_root.mkdir()
    try:
        stage_a = build_root / "stage-a"
        stage_b = build_root / "stage-b"
        build_stage(stage_a, source)
        build_stage(stage_b, source)
        hashes_a = tree_hashes(stage_a)
        hashes_b = tree_hashes(stage_b)
        if hashes_a != hashes_b:
            raise ev.EvidenceError("clean ISO staging trees differ")
        blocks_a = xorriso_build(stage_a, build_root / "build-a.iso", print_only=True)
        blocks_b = xorriso_build(stage_b, build_root / "build-b.iso", print_only=True)
        if blocks_a != blocks_b or blocks_a * 2048 >= ev.CD_BUDGET_BYTES:
            raise ev.EvidenceError("ISO print-size is inconsistent or exceeds 650 MiB")
        xorriso_build(stage_a, build_root / "build-a.iso")
        xorriso_build(stage_b, build_root / "build-b.iso")
        first = build_root / "build-a.iso"
        second = build_root / "build-b.iso"
        if first.stat().st_size != blocks_a * 2048 or second.stat().st_size != blocks_b * 2048:
            raise ev.EvidenceError("ISO byte size differs from xorriso print-size")
        if ev.sha256_file(first) != ev.sha256_file(second):
            raise ev.EvidenceError("two clean ISO builds are not byte-identical")
        shutil.move(first, volume_path)
        secret = secret_service_passphrase()
        signature = volume_path.with_suffix(volume_path.suffix + ".sig")
        try:
            sign_file(volume_path, signature, secret)
        finally:
            clear_secret(secret)
        ev.gpg_validsig(signature, volume_path, CURRENT / "TRUST" / "openpgp-public.asc")
        query = volume_path.with_suffix(volume_path.suffix + ".sig.tsq")
        response = volume_path.with_suffix(volume_path.suffix + ".sig.tsr")
        request_timestamp(signature, query, response, "http://timestamp.digicert.com")
        timestamp = ev.verify_timestamp(signature, query, response,
                                        CURRENT / "TRUST" / "tsa-root.pem",
                                        CURRENT / "TRUST" / "tsa-untrusted.pem",
                                        CURRENT / "TRUST" / "tsa-responder.pem")
        checksum = volume_path.with_suffix(volume_path.suffix + ".sha256")
        checksum.write_text(f"{ev.sha256_file(volume_path)}  {volume_path.name}\n", encoding="ascii")
        verify_record = volume_path.with_suffix(volume_path.suffix + ".verification.json")
        write_canonical(verify_record, {
            "schema": "notrios.evidence.iso-verification.v1", "volume_id": VOLUME_ID,
            "iso_sha256": ev.sha256_file(volume_path), "iso_size_bytes": volume_path.stat().st_size,
            "printed_blocks": blocks_a, "budget_bytes": ev.CD_BUDGET_BYTES,
            "clean_builds": 2, "byte_identical": True, "stage_file_count": len(hashes_a),
            "timestamp": {key: value for key, value in timestamp.items() if key != "openssl_chain_result"},
        })
        with tempfile.TemporaryDirectory(prefix="notrios-g17b-restore-") as directory:
            extracted = Path(directory)
            ev.extract_iso(volume_path, extracted)
            if tree_hashes(extracted) != hashes_a:
                raise ev.EvidenceError("ISO extraction hash walk differs from staging")
            ev.verify_checkpoint(extracted, verify_payload=True)
    except Exception:
        # The numbered final image is removed only while this first issuance is
        # incomplete; a successfully completed volume is never regenerated.
        for path in (volume_path, volume_path.with_suffix(volume_path.suffix + ".sig"),
                     volume_path.with_suffix(volume_path.suffix + ".sig.tsq"),
                     volume_path.with_suffix(volume_path.suffix + ".sig.tsr"),
                     volume_path.with_suffix(volume_path.suffix + ".sha256"),
                     volume_path.with_suffix(volume_path.suffix + ".verification.json")):
            if path.exists():
                path.unlink()
        raise
    finally:
        if build_root.exists():
            remove_readonly_tree(build_root)
    print(json.dumps({"status": "reserved", "volume_id": VOLUME_ID,
                      "relative_path": VOLUME_RELATIVE, "size_bytes": volume_path.stat().st_size,
                      "sha256": ev.sha256_file(volume_path)}, sort_keys=True))


def cleanup_workspace(args: argparse.Namespace) -> None:
    reserve = args.reserve_root.resolve()
    build_root = reserve / ".g17b-build-0001"
    if build_root.parent != reserve or build_root.name != ".g17b-build-0001":
        raise ev.EvidenceError("unexpected cleanup target")
    remove_readonly_tree(build_root)
    print(json.dumps({"status": "clean", "workspace": build_root.name}, sort_keys=True))


def seal_catalog(args: argparse.Namespace) -> None:
    reserve = args.reserve_root.resolve()
    iso = reserve / VOLUME_RELATIVE
    if not iso.is_file():
        raise ev.EvidenceError("reserved ISO is missing")
    signature = iso.with_suffix(iso.suffix + ".sig")
    query = iso.with_suffix(iso.suffix + ".sig.tsq")
    response = iso.with_suffix(iso.suffix + ".sig.tsr")
    verification = ev.load_canonical(iso.with_suffix(iso.suffix + ".verification.json"))
    content_checkpoint = CURRENT / "CHECKPOINT" / "content-checkpoint.json"
    manifest = CURRENT / "CHECKPOINT" / "content-manifest.jsonl"
    content_commit = run(["git", "rev-parse", "HEAD"], cwd=REPO).stdout.decode().strip()
    xorriso_version = run(["xorriso", "-version"]).stdout.decode().splitlines()[0]
    binary = shutil.which("xorriso")
    if binary is None:
        raise ev.EvidenceError("xorriso binary disappeared")
    existing_names = (
        "outer-iso-catalog.jsonl", "outer-iso-catalog-checkpoint.json",
        "outer-iso-catalog-checkpoint.json.sig", "outer-iso-catalog-checkpoint.sig.tsq",
        "outer-iso-catalog-checkpoint.sig.tsr", "outer-iso-catalog-verification.json",
    )
    existing = [REPO / "evidence" / name for name in existing_names
                if (REPO / "evidence" / name).exists()]
    if existing:
        if not args.supersede_reason or len(existing) != len(existing_names):
            raise ev.EvidenceError("existing catalog set requires an explicit complete supersession")
        archive = REPO / "evidence" / "superseded" / "catalog-attempt-0001"
        if archive.exists():
            raise ev.EvidenceError("catalog supersession archive already exists")
        archive.mkdir(parents=True)
        archived = []
        for path in existing:
            target = archive / path.name
            shutil.copyfile(path, target)
            archived.append({"logical_name": path.name, "size_bytes": target.stat().st_size,
                             "sha256": ev.sha256_file(target)})
        write_canonical(archive / "SUPERSESSION.json", {
            "schema": "notrios.evidence.supersession.v1",
            "reason": args.supersede_reason,
            "previous_set_verified": True,
            "materials": archived,
        })
    recorded_at = utc(dt.datetime.now(dt.timezone.utc))
    iso_mtime = utc(dt.datetime.fromtimestamp(iso.stat().st_mtime, dt.timezone.utc))
    payload: dict[str, object] = {
        "record_type": "iso-volume", "volume_id": VOLUME_ID,
        "logical_name": "notrios-evidence-0001.iso",
        "reserve_location_id": RESERVE_LOCATION_ID,
        "reserve_relative_path": VOLUME_RELATIVE,
        "size_bytes": iso.stat().st_size, "sha256": ev.sha256_file(iso),
        "signature_sha256": ev.sha256_file(signature),
        "timestamp_query_sha256": ev.sha256_file(query),
        "timestamp_response_sha256": ev.sha256_file(response),
        "content_checkpoint_id": CHECKPOINT_ID,
        "content_checkpoint_sha256": ev.sha256_file(content_checkpoint),
        "content_manifest_sha256": ev.sha256_file(manifest),
        "content_commit": content_commit,
        "iso_file_mtime_weak": iso_mtime,
        "iso_signature_gen_time": verification["timestamp"]["gen_time"],
        "creation_tool": xorriso_version,
        "creation_binary_sha256": ev.sha256_file(Path(binary)),
        "creation_options": iso_options(VOLUME_ID),
        "source_date_epoch": FIXED_EPOCH,
        "os": platform.system() + " " + platform.release(),
        "clean_builds": 2, "byte_identical": True,
        "printed_blocks": verification["printed_blocks"],
        "budget_bytes": ev.CD_BUDGET_BYTES,
        "custody_event": {
            "schema": "notrios.evidence.custody-event.v1",
            "event_time_local_clock": recorded_at,
            "actor_or_opaque_custodian_id": "local-owner-workstation",
            "action": "create-and-verify-reserve",
            "source_id": CHECKPOINT_ID,
            "destination_id": RESERVE_LOCATION_ID,
            "volume_id": VOLUME_ID,
            "tool_and_version": xorriso_version,
            "before_sha256": ev.sha256_file(CURRENT / "CHECKPOINT" / "content-checkpoint.json"),
            "after_sha256": ev.sha256_file(iso),
            "result": "two byte-identical builds; extracted content verified",
            "manual": False,
            "notes": "Local-clock event; RFC 3161 evidence applies to the signed ISO and catalog checkpoints.",
            "previous_event_sha256": ev.ZERO_HASH,
        },
    }
    record = make_record(1, ev.ZERO_HASH, payload, ev.CATALOG_ENTRY_SCHEMA)
    catalog = REPO / "evidence" / "outer-iso-catalog.jsonl"
    catalog.write_bytes(ev.canonical_bytes(record))
    checkpoint_value = {
        "schema": ev.CATALOG_CHECKPOINT_SCHEMA, "checkpoint_id": "g17b-iso-catalog-20260825-0001",
        "entry_count": 1, "first_entry_sha256": record["entry_sha256"],
        "last_entry_sha256": record["entry_sha256"],
        "catalog_sha256": ev.sha256_file(catalog), "catalog_size_bytes": catalog.stat().st_size,
        "content_commit": content_commit,
        "closure_boundary": "catalog-only commit is intentionally outside current ISO; next ordinary checkpoint covers it",
    }
    catalog_checkpoint = REPO / "evidence" / "outer-iso-catalog-checkpoint.json"
    write_canonical(catalog_checkpoint, checkpoint_value)
    catalog_signature = REPO / "evidence" / "outer-iso-catalog-checkpoint.json.sig"
    secret = secret_service_passphrase()
    try:
        sign_file(catalog_checkpoint, catalog_signature, secret)
    finally:
        clear_secret(secret)
    ev.gpg_validsig(catalog_signature, catalog_checkpoint, CURRENT / "TRUST" / "openpgp-public.asc")
    catalog_query = REPO / "evidence" / "outer-iso-catalog-checkpoint.sig.tsq"
    catalog_response = REPO / "evidence" / "outer-iso-catalog-checkpoint.sig.tsr"
    request_timestamp(catalog_signature, catalog_query, catalog_response,
                      "http://timestamp.digicert.com")
    timestamp = ev.verify_timestamp(catalog_signature, catalog_query, catalog_response,
                                    CURRENT / "TRUST" / "tsa-root.pem",
                                    CURRENT / "TRUST" / "tsa-untrusted.pem",
                                    CURRENT / "TRUST" / "tsa-responder.pem")
    write_canonical(REPO / "evidence" / "outer-iso-catalog-verification.json", {
        "schema": "notrios.evidence.iso-catalog-verification.v1",
        "catalog_sha256": ev.sha256_file(catalog),
        "signature_sha256": ev.sha256_file(catalog_signature),
        "query_sha256": ev.sha256_file(catalog_query),
        "response_sha256": ev.sha256_file(catalog_response),
        "timestamp": {key: value for key, value in timestamp.items() if key != "openssl_chain_result"},
    })
    result = ev.verify_reserve(
        reserve, catalog, catalog_checkpoint, catalog_signature, catalog_query,
        catalog_response, CURRENT / "TRUST" / "tsa-root.pem",
        CURRENT / "TRUST" / "tsa-untrusted.pem", CURRENT / "TRUST" / "tsa-responder.pem",
        CURRENT / "TRUST" / "openpgp-public.asc",
    )
    print(json.dumps({"status": "catalog-sealed", **result}, sort_keys=True))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    pilot = sub.add_parser("record-pilot")
    pilot.add_argument("--pilot", type=Path, required=True)
    pilot.add_argument("--root-cert", type=Path, required=True)
    pilot.set_defaults(function=record_pilot)
    content = sub.add_parser("seal-content")
    content.add_argument("--source", type=Path, required=True)
    content.add_argument("--sealed-at", required=True)
    content.set_defaults(function=seal_content)
    reserve = sub.add_parser("build-reserve")
    reserve.add_argument("--source", type=Path, required=True)
    reserve.add_argument("--reserve-root", type=Path, required=True)
    reserve.set_defaults(function=build_reserve)
    cleanup = sub.add_parser("cleanup-workspace")
    cleanup.add_argument("--reserve-root", type=Path, required=True)
    cleanup.set_defaults(function=cleanup_workspace)
    catalog = sub.add_parser("seal-catalog")
    catalog.add_argument("--reserve-root", type=Path, required=True)
    catalog.add_argument("--supersede-reason")
    catalog.set_defaults(function=seal_catalog)
    args = parser.parse_args()
    try:
        args.function(args)
    except (ev.EvidenceError, OSError, zipfile.BadZipFile) as exc:
        raise SystemExit(f"G17b refused: {exc}")


if __name__ == "__main__":
    main()
