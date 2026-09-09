#!/usr/bin/env python3
"""Seal an additional immutable reserve volume, and append it to the catalog.

`g17b_evidence.py` sealed volume-0001 and is single-volume by construction: its
checkpoint id, volume id and build paths are constants, its content sealing
requires exactly the 81 approved G17b artifacts, and its catalog step rebuilds
the catalog rather than adding to it. That tool is left exactly as it is -- it
is the record of how the first volume was made, and it still runs -- and this
one does the general case, reusing its helpers rather than copying them.

Three decisions from the repository owner shape it:

  * **A directory per volume.** The evidence is spread across volumes, so each
    volume's checkpoint materials live in `evidence/volumes/<volume>/` rather
    than replacing `evidence/current/`. Volume-0001's materials stay where they
    are; moving them is a separate change and this one does not need it.
  * **The catalog is appended to**, not superseded. A new entry chains onto the
    last, and the catalog checkpoint is re-signed and re-timestamped because it
    commits to the catalog's hash -- which is what appending means, not a
    supersession of anything.
  * **A volume records its predecessor.** `predecessor_checkpoint_sha256` takes
    the hash of the previous checkpoint document, because that is what the field
    is and what makes the link cryptographic; `predecessor_checkpoint_id` records
    the readable name beside it, so the volumes read as one collection.

Rehearsal is a first-class mode. `--signer` and `--gnupghome` let the whole path
run against a throwaway key in a temporary keyring, so the ISO build, the
signing, the timestamping and the catalog append are all exercised before the
production key signs anything.
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import shutil
import sys
import tempfile
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO / "scripts"))
sys.path.insert(0, str(REPO))
import g17b_evidence as g17b  # noqa: E402
from evidence import verify_evidence as ev  # noqa: E402


def sign_with(datum: Path, signature: Path, passphrase, signer: str, gnupghome: str | None) -> None:
    """Detached-sign, with the signer and keyring overridable for a rehearsal."""
    environment = dict(os.environ)
    if gnupghome:
        environment["GNUPGHOME"] = gnupghome
    g17b.run(["gpg", "--batch", "--yes", "--quiet", "--pinentry-mode", "loopback",
              "--passphrase-fd", "0", "--local-user", signer + "!",
              "--detach-sign", "--output", str(signature), str(datum)],
             input_bytes=bytes(passphrase) + b"\n", env=environment)
    if not signature.is_file() or signature.stat().st_size == 0:
        raise ev.EvidenceError(f"no signature produced for {datum.name}")


# The types a volume may carry, and the reason the set can grow.
#
# `g17b_evidence.py` approves .zip and .png, and that tool is the frozen record
# of how volume-0001 was made, so the policy is widened here rather than there.
# A type belongs in this set only when `structural_validation` can prove the
# container is intact -- that is what makes the approval mean something. The git
# bundle and the Debian package were held out of volume-0002 for exactly that
# reason; `validate_git_bundle` and `validate_deb` are what let them in now.
APPROVED_MEDIA_TYPES = {
    ".zip": "application/zip",
    ".png": "image/png",
    ".bundle": "application/x-git-bundle",
    ".deb": "application/vnd.debian.binary-package",
}


def media_type(path: Path) -> str:
    kind = APPROVED_MEDIA_TYPES.get(path.suffix.lower())
    if kind is None:
        raise ev.EvidenceError(f"unapproved curated artifact type: {path.name}")
    return kind


def use_rehearsal_key(gnupghome: str, signer: str) -> None:
    """Point the verifier's pinned fingerprints at the throwaway rehearsal key.

    The verifier pins the production primary and signing fingerprints, and
    refuses any other signer -- in `gpg_validsig` and again in
    `verify_checkpoint`, which is exactly what it should do. So a rehearsal
    would be refused at the first signature unless it says which key it is
    rehearsing with. Rebinding here, and only when `--gnupghome` names a
    throwaway keyring, keeps the production path pinned to the real key while
    letting the rehearsal exercise every other line.

    A rehearsal ISO is therefore not verifiable by the stock verifier it
    carries in TOOLS/. That is correct: a rehearsal volume is not evidence.
    """
    environment = dict(os.environ, GNUPGHOME=gnupghome)
    listing = g17b.run(["gpg", "--batch", "--with-colons", "--fingerprint", "--fingerprint",
                        "--list-keys", signer], env=environment).stdout.decode()
    fingerprints = [line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:")]
    if signer not in fingerprints:
        raise ev.EvidenceError("the rehearsal keyring does not hold the named signing key")
    ev.PRIMARY_FINGERPRINT = fingerprints[0]
    ev.SIGNING_FINGERPRINT = signer


def stage_volume(stage: Path, source: Path, materials: Path, names: list[str]) -> None:
    """Lay out the ISO tree: this volume's payload, signatures and checkpoint."""
    if stage.exists():
        raise ev.EvidenceError("refusing to reuse an ISO staging directory")
    for name in ("PAYLOAD", "SIGNATURES", "CHECKPOINT", "TRUST", "TOOLS", "SCHEMAS", "CUSTODY"):
        (stage / name).mkdir(parents=True, exist_ok=True)
    for name in sorted(names, key=lambda value: value.encode("utf-8")):
        shutil.copyfile(source / name, stage / "PAYLOAD" / name)
    for directory in ("SIGNATURES", "CHECKPOINT", "TRUST"):
        for path in sorted((materials / directory).iterdir()):
            if path.is_file():
                shutil.copyfile(path, stage / directory / path.name)
    shutil.copyfile(REPO / "evidence" / "verify_evidence.py", stage / "TOOLS" / "verify-evidence.py")
    for path in sorted((REPO / "evidence" / "schemas").iterdir()):
        if path.is_file():
            shutil.copyfile(path, stage / "SCHEMAS" / path.name)
    for name in ("CUSTODY_TEMPLATE.json", "BURN_AND_READBACK.md"):
        shutil.copyfile(REPO / "evidence" / name, stage / "CUSTODY" / name)
    shutil.copyfile(REPO / "evidence" / "ISO_README.txt", stage / "README.txt")
    g17b.normalize_tree(stage, g17b.FIXED_EPOCH)


def seal_content(source: Path, names: list[str], materials: Path, checkpoint_id: str,
                 volume_id: str, predecessor: dict[str, str] | None, sealed_at: str,
                 passphrase, signer: str, gnupghome: str | None, public_key: Path) -> dict:
    """Write this volume's signed, timestamped checkpoint over its own payload."""
    for name in ("CHECKPOINT", "SIGNATURES", "TRUST"):
        (materials / name).mkdir(parents=True, exist_ok=True)
    for path in (REPO / "evidence" / "current" / "TRUST").iterdir():
        if path.is_file():
            shutil.copyfile(path, materials / "TRUST" / path.name)
    trust_key = materials / "TRUST" / "openpgp-public.asc"
    if gnupghome:  # a rehearsal verifies against its own throwaway key
        shutil.copyfile(public_key, trust_key)

    history = g17b.git_anchor_history()
    records: list[dict] = []
    previous = ev.ZERO_HASH
    for index, name in enumerate(sorted(names, key=lambda value: value.encode("utf-8")), start=1):
        path = source / name
        kind = media_type(path)
        validation = ev.structural_validation(path, kind)
        if not validation["valid"]:
            raise ev.EvidenceError(f"structural validation refused: {name}")
        stat = path.stat()
        artifact_id = f"artifact-{index:04d}"
        artifact_payload = {
            "record_type": "artifact", "artifact_id": artifact_id,
            "logical_name": "PAYLOAD/" + name, "media_type": kind,
            "size_bytes": stat.st_size, "sha256": ev.sha256_file(path),
            "structural_validation": validation,
            "captured_at": sealed_at, "sealed_at": sealed_at, "retroactive": True,
            "step_completed_at": None, "completion_source": None,
            "source_mtime_weak": g17b.utc(dt.datetime.fromtimestamp(stat.st_mtime, dt.timezone.utc)),
            "preservation_class": "curated-top-level-handoff",
            "assigned_checkpoint_id": checkpoint_id, "assigned_volume_id": volume_id,
            **g17b.provenance(path, history),
        }
        record = g17b.make_record(len(records) + 1, previous, artifact_payload)
        records.append(record)
        previous = str(record["entry_sha256"])

        signature_path = materials / "SIGNATURES" / (name + ".sig")
        sign_with(path, signature_path, passphrase, signer, gnupghome)
        gpg = ev.gpg_validsig(signature_path, path, trust_key)
        signature_payload = {
            "record_type": "signature", "artifact_id": artifact_id,
            "artifact_entry_sha256": record["entry_sha256"],
            "logical_name": "SIGNATURES/" + signature_path.name,
            "size_bytes": signature_path.stat().st_size,
            "sha256": ev.sha256_file(signature_path),
            "primary_fingerprint": ev.PRIMARY_FINGERPRINT,
            "signing_fingerprint": ev.SIGNING_FINGERPRINT,
            "implementation": g17b.run(["gpg", "--version"]).stdout.decode().splitlines()[0],
            "public_key_algorithm": gpg["public_key_algorithm"],
            "digest_algorithm": gpg["digest_algorithm"],
            "signature_created_at_signer_controlled": g17b.utc(
                dt.datetime.fromtimestamp(int(gpg["created_epoch"]), dt.timezone.utc)),
            "clean_keyring_validsig_verified": True,
        }
        signature_record = g17b.make_record(len(records) + 1, previous, signature_payload)
        records.append(signature_record)
        previous = str(signature_record["entry_sha256"])

    manifest = materials / "CHECKPOINT" / "content-manifest.jsonl"
    manifest.write_bytes(b"".join(ev.canonical_bytes(record) for record in records))
    checkpoint = {
        "schema": ev.CHECKPOINT_SCHEMA, "checkpoint_id": checkpoint_id,
        "entry_count": len(records), "first_entry_sha256": records[0]["entry_sha256"],
        "last_entry_sha256": records[-1]["entry_sha256"],
        "manifest_sha256": ev.sha256_file(manifest),
        "manifest_size_bytes": manifest.stat().st_size,
        "capture_window_start": sealed_at, "capture_window_end": sealed_at,
        "seal_window_start": sealed_at, "seal_window_end": sealed_at,
        "contains_retroactive_artifacts": True,
        "canonical_profile": "notrios-canonical-json-v1",
        "verifier_version": "notrios-evidence-verifier-v1",
        "intended_volume_ids": [volume_id],
        "predecessor_checkpoint_sha256": (predecessor or {}).get("sha256"),
        "source_artifact_count": len(names),
    }
    if predecessor:
        # The readable half of the link, beside the cryptographic one. The
        # volumes are one collection of evidence and should read as one.
        checkpoint["predecessor_checkpoint_id"] = predecessor["checkpoint_id"]
        checkpoint["predecessor_volume_id"] = predecessor["volume_id"]
    checkpoint_path = materials / "CHECKPOINT" / "content-checkpoint.json"
    g17b.write_canonical(checkpoint_path, checkpoint)
    signature_path = materials / "CHECKPOINT" / "content-checkpoint.json.sig"
    sign_with(checkpoint_path, signature_path, passphrase, signer, gnupghome)
    ev.gpg_validsig(signature_path, checkpoint_path, trust_key)

    query = materials / "CHECKPOINT" / "content-checkpoint.sig.tsq"
    response = materials / "CHECKPOINT" / "content-checkpoint.sig.tsr"
    g17b.request_timestamp(signature_path, query, response, "http://timestamp.digicert.com")
    timestamp = ev.verify_timestamp(signature_path, query, response,
                                    materials / "TRUST" / "tsa-root.pem",
                                    materials / "TRUST" / "tsa-untrusted.pem",
                                    materials / "TRUST" / "tsa-responder.pem")
    g17b.write_canonical(materials / "CHECKPOINT" / "timestamp-verification.json", {
        "schema": "notrios.evidence.timestamp-verification.v1",
        "provider": "DigiCert", "endpoint": "http://timestamp.digicert.com",
        **{key: value for key, value in timestamp.items() if key != "openssl_chain_result"},
        "nonce_matched": True, "sha256_imprint_matched": True,
        "revocation_evidence": "not fetched; responder OCSP/CRL URLs are preserved in its certificate",
    })
    material_paths = [manifest, checkpoint_path, signature_path, query, response,
                      *sorted((materials / "TRUST").iterdir())]
    g17b.write_canonical(materials / "CHECKPOINT" / "seal-materials.json", {
        "schema": "notrios.evidence.seal-materials.v1", "checkpoint_id": checkpoint_id,
        "materials": [{"logical_name": path.relative_to(materials).as_posix(),
                       "size_bytes": path.stat().st_size, "sha256": ev.sha256_file(path)}
                      for path in material_paths],
        "finite_closure_note": "The timestamp response cannot hash itself; verification binds it to the query and exact checkpoint signature.",
    })
    ev.verify_checkpoint(materials, verify_payload=False)
    return checkpoint


def build_volume(reserve: Path, relative: str, volume_id: str, source: Path, names: list[str],
                 materials: Path, passphrase, signer: str, gnupghome: str | None,
                 public_key: Path) -> dict:
    """Build the ISO twice, prove the builds identical, then sign and stamp it."""
    volume_path = reserve / relative
    if volume_path.exists():
        raise ev.EvidenceError(f"{relative} already exists; a completed volume is never regenerated")
    volume_path.parent.mkdir(parents=True, exist_ok=True)
    build_root = reserve / f".build-{volume_id}"
    if build_root.exists():
        raise ev.EvidenceError("an existing build workspace requires manual review")
    build_root.mkdir()
    try:
        stage_a, stage_b = build_root / "stage-a", build_root / "stage-b"
        stage_volume(stage_a, source, materials, names)
        stage_volume(stage_b, source, materials, names)
        hashes = g17b.tree_hashes(stage_a)
        if hashes != g17b.tree_hashes(stage_b):
            raise ev.EvidenceError("clean ISO staging trees differ")
        blocks = _print_size(stage_a, volume_id)
        if blocks != _print_size(stage_b, volume_id):
            raise ev.EvidenceError("ISO print-size is inconsistent")
        if blocks * 2048 >= ev.CD_BUDGET_BYTES:
            raise ev.EvidenceError(f"ISO would be {blocks * 2048} bytes, over the {ev.CD_BUDGET_BYTES} budget")
        first, second = build_root / "build-a.iso", build_root / "build-b.iso"
        _build(stage_a, first, volume_id)
        _build(stage_b, second, volume_id)
        if ev.sha256_file(first) != ev.sha256_file(second):
            raise ev.EvidenceError("two clean ISO builds are not byte-identical")
        shutil.move(str(first), volume_path)

        signature = volume_path.with_suffix(volume_path.suffix + ".sig")
        sign_with(volume_path, signature, passphrase, signer, gnupghome)
        ev.gpg_validsig(signature, volume_path, materials / "TRUST" / "openpgp-public.asc")
        query = volume_path.with_suffix(volume_path.suffix + ".sig.tsq")
        response = volume_path.with_suffix(volume_path.suffix + ".sig.tsr")
        g17b.request_timestamp(signature, query, response, "http://timestamp.digicert.com")
        timestamp = ev.verify_timestamp(signature, query, response,
                                        materials / "TRUST" / "tsa-root.pem",
                                        materials / "TRUST" / "tsa-untrusted.pem",
                                        materials / "TRUST" / "tsa-responder.pem")
        volume_path.with_suffix(volume_path.suffix + ".sha256").write_text(
            f"{ev.sha256_file(volume_path)}  {volume_path.name}\n", encoding="ascii")
        verification = volume_path.with_suffix(volume_path.suffix + ".verification.json")
        g17b.write_canonical(verification, {
            "schema": "notrios.evidence.iso-verification.v1", "volume_id": volume_id,
            "iso_sha256": ev.sha256_file(volume_path), "iso_size_bytes": volume_path.stat().st_size,
            "printed_blocks": blocks, "budget_bytes": ev.CD_BUDGET_BYTES,
            "clean_builds": 2, "byte_identical": True, "stage_file_count": len(hashes),
            "timestamp": {k: v for k, v in timestamp.items() if k != "openssl_chain_result"},
        })
        # Read it back the way a stranger would: extract without mounting and
        # walk it. A volume that cannot be read back is not evidence.
        with tempfile.TemporaryDirectory(prefix="notrios-readback-") as directory:
            extracted = Path(directory)
            ev.extract_iso(volume_path, extracted)
            if g17b.tree_hashes(extracted) != hashes:
                raise ev.EvidenceError("ISO extraction hash walk differs from staging")
            ev.verify_checkpoint(extracted, verify_payload=True)
        return {"volume_id": volume_id, "relative_path": relative, "blocks": blocks,
                "size_bytes": volume_path.stat().st_size, "sha256": ev.sha256_file(volume_path)}
    except Exception:
        for path in (volume_path, *(volume_path.with_suffix(volume_path.suffix + suffix)
                                    for suffix in (".sig", ".sig.tsq", ".sig.tsr", ".sha256",
                                                   ".verification.json"))):
            if path.exists():
                path.unlink()
        raise
    finally:
        if build_root.exists():
            g17b.remove_readonly_tree(build_root)


def _deterministic_env() -> dict[str, str]:
    """The environment xorriso must see, or the volume is not reproducible.

    `xorriso_build` sets these for volume-0001 and this omitted them, which the
    rehearsal caught: without `SOURCE_DATE_EPOCH` xorriso stamps the wall clock
    into the descriptor, so two builds are identical only when they happen to
    land in the same second. Small rehearsal volumes straddled a second
    boundary about one run in four; the real 550 MB volume takes far longer
    than a second to build, so it would have differed every time -- and a
    single-build path would have produced a volume nobody could rebuild.
    """
    return dict(os.environ, SOURCE_DATE_EPOCH=str(g17b.FIXED_EPOCH), TZ="UTC", LC_ALL="C")


def _print_size(stage: Path, volume_id: str) -> int:
    result = g17b.run(["xorriso", "-no_rc", "-as", "mkisofs", *g17b.iso_options(volume_id),
                       "-print-size", str(stage)], env=_deterministic_env())
    for line in (result.stdout + result.stderr).decode().splitlines():
        if line.strip().isdigit():
            return int(line.strip())
    raise ev.EvidenceError("xorriso did not return a print-size block count")


def _build(stage: Path, output: Path, volume_id: str) -> None:
    g17b.run(["xorriso", "-no_rc", "-as", "mkisofs", *g17b.iso_options(volume_id),
              "-o", str(output), str(stage)], env=_deterministic_env())


def custody_event_sha256(event: dict) -> str:
    """Hash a custody event the way every other record here is hashed.

    Nothing writes an `event_sha256` field -- volume-0001's event does not have
    one, and `CUSTODY_TEMPLATE.json` does not either. So a custody chain has to
    be built from the event document itself, canonicalised and hashed, exactly
    as `make_record` hashes an entry. Reading a stored field instead would have
    produced a `previous_event_sha256` of all zeroes on every volume: a chain
    that is present, well-formed, and links nothing.
    """
    return g17b.hashlib.sha256(ev.canonical_bytes(event)).hexdigest()


def append_catalog(reserve: Path, volume: dict, materials: Path, checkpoint_id: str,
                   evidence_dir: Path, passphrase, signer: str, gnupghome: str | None) -> dict:
    """Add this volume to the outer catalog, chained onto the last entry."""
    catalog = evidence_dir / "outer-iso-catalog.jsonl"
    existing = ev.load_catalog(catalog) if catalog.is_file() else []
    if any(record["payload"].get("volume_id") == volume["volume_id"] for record in existing):
        raise ev.EvidenceError("this volume is already in the catalog")
    previous = str(existing[-1]["entry_sha256"]) if existing else ev.ZERO_HASH
    previous_event = (custody_event_sha256(existing[-1]["payload"]["custody_event"])
                      if existing else ev.ZERO_HASH)

    iso = reserve / volume["relative_path"]
    verification = ev.load_canonical(iso.with_suffix(iso.suffix + ".verification.json"))
    binary = shutil.which("xorriso")
    if binary is None:
        raise ev.EvidenceError("xorriso binary disappeared")
    xorriso_version = g17b.run(["xorriso", "-version"]).stdout.decode().splitlines()[0]
    recorded_at = g17b.utc(dt.datetime.now(dt.timezone.utc))
    content_checkpoint = materials / "CHECKPOINT" / "content-checkpoint.json"
    payload = {
        "record_type": "iso-volume", "volume_id": volume["volume_id"],
        "logical_name": Path(volume["relative_path"]).name,
        "reserve_location_id": g17b.RESERVE_LOCATION_ID,
        "reserve_relative_path": volume["relative_path"],
        "size_bytes": iso.stat().st_size, "sha256": ev.sha256_file(iso),
        "signature_sha256": ev.sha256_file(iso.with_suffix(iso.suffix + ".sig")),
        "timestamp_query_sha256": ev.sha256_file(iso.with_suffix(iso.suffix + ".sig.tsq")),
        "timestamp_response_sha256": ev.sha256_file(iso.with_suffix(iso.suffix + ".sig.tsr")),
        "content_checkpoint_id": checkpoint_id,
        "content_checkpoint_sha256": ev.sha256_file(content_checkpoint),
        "content_manifest_sha256": ev.sha256_file(materials / "CHECKPOINT" / "content-manifest.jsonl"),
        "content_commit": g17b.run(["git", "rev-parse", "HEAD"], cwd=REPO).stdout.decode().strip(),
        "iso_file_mtime_weak": g17b.utc(dt.datetime.fromtimestamp(iso.stat().st_mtime, dt.timezone.utc)),
        "iso_signature_gen_time": verification["timestamp"]["gen_time"],
        "creation_tool": xorriso_version,
        "creation_binary_sha256": ev.sha256_file(Path(binary)),
        "creation_options": g17b.iso_options(volume["volume_id"]),
        "source_date_epoch": g17b.FIXED_EPOCH,
        "os": g17b.platform.system() + " " + g17b.platform.release(),
        "clean_builds": 2, "byte_identical": True,
        "printed_blocks": verification["printed_blocks"],
        "budget_bytes": ev.CD_BUDGET_BYTES,
        "custody_event": {
            "schema": "notrios.evidence.custody-event.v1",
            "event_time_local_clock": recorded_at,
            "actor_or_opaque_custodian_id": "local-owner-workstation",
            "action": "create-and-verify-reserve",
            "source_id": checkpoint_id,
            "destination_id": g17b.RESERVE_LOCATION_ID,
            "volume_id": volume["volume_id"],
            "tool_and_version": xorriso_version,
            "before_sha256": ev.sha256_file(content_checkpoint),
            "after_sha256": ev.sha256_file(iso),
            "result": "two byte-identical builds; extracted content verified",
            "manual": False,
            "notes": "Local-clock event; RFC 3161 evidence applies to the signed ISO and catalog checkpoints.",
            "previous_event_sha256": previous_event,
        },
    }
    record = g17b.make_record(len(existing) + 1, previous, payload, ev.CATALOG_ENTRY_SCHEMA)
    catalog.write_bytes(b"".join(ev.canonical_bytes(entry) for entry in [*existing, record]))

    # The catalog checkpoint commits to the catalog's hash, so appending means
    # re-signing and re-stamping it. That is not a supersession of anything: the
    # earlier entries are unchanged and still chained.
    checkpoint_value = {
        "schema": ev.CATALOG_CHECKPOINT_SCHEMA,
        "checkpoint_id": f"notrios-iso-catalog-{volume['volume_id'].lower()}",
        "entry_count": len(existing) + 1,
        "first_entry_sha256": (existing[0] if existing else record)["entry_sha256"],
        "last_entry_sha256": record["entry_sha256"],
        "catalog_sha256": ev.sha256_file(catalog), "catalog_size_bytes": catalog.stat().st_size,
        "content_commit": payload["content_commit"],
        "closure_boundary": "catalog-only commit is intentionally outside current ISO; next ordinary checkpoint covers it",
    }
    checkpoint_path = evidence_dir / "outer-iso-catalog-checkpoint.json"
    g17b.write_canonical(checkpoint_path, checkpoint_value)
    signature = evidence_dir / "outer-iso-catalog-checkpoint.json.sig"
    sign_with(checkpoint_path, signature, passphrase, signer, gnupghome)
    ev.gpg_validsig(signature, checkpoint_path, materials / "TRUST" / "openpgp-public.asc")
    query = evidence_dir / "outer-iso-catalog-checkpoint.sig.tsq"
    response = evidence_dir / "outer-iso-catalog-checkpoint.sig.tsr"
    g17b.request_timestamp(signature, query, response, "http://timestamp.digicert.com")
    timestamp = ev.verify_timestamp(signature, query, response,
                                    materials / "TRUST" / "tsa-root.pem",
                                    materials / "TRUST" / "tsa-untrusted.pem",
                                    materials / "TRUST" / "tsa-responder.pem")
    g17b.write_canonical(evidence_dir / "outer-iso-catalog-verification.json", {
        "schema": "notrios.evidence.iso-catalog-verification.v1",
        "catalog_sha256": ev.sha256_file(catalog),
        "entry_count": len(existing) + 1,
        **{k: v for k, v in timestamp.items() if k != "openssl_chain_result"},
    })
    return checkpoint_value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--source", type=Path, required=True,
                        help="directory holding the artifacts to seal")
    parser.add_argument("--names", type=Path, required=True,
                        help="JSON array of file names, or a JSON object with a to_seal array")
    parser.add_argument("--reserve-root", type=Path, required=True)
    parser.add_argument("--evidence-dir", type=Path, default=REPO / "evidence",
                        help="where the catalog lives; a rehearsal points this at a scratch copy")
    parser.add_argument("--volume-id", required=True, help="portable ISO volume id, e.g. NTR-EV-0002")
    parser.add_argument("--checkpoint-id", required=True)
    parser.add_argument("--materials", type=Path, required=True,
                        help="this volume's checkpoint directory, e.g. evidence/volumes/volume-0002")
    parser.add_argument("--predecessor-materials", type=Path,
                        help="the previous volume's checkpoint directory, to chain onto")
    parser.add_argument("--predecessor-volume-id")
    parser.add_argument("--signer", default=ev.SIGNING_FINGERPRINT)
    parser.add_argument("--gnupghome", help="rehearsal only: a temporary keyring holding a throwaway key")
    parser.add_argument("--rehearsal-public-key", type=Path,
                        help="rehearsal only: the throwaway key's exported public half")
    parser.add_argument("--passphrase-file", type=Path,
                        help="rehearsal only; production reads the Secret Service")
    arguments = parser.parse_args()

    loaded = json.loads(arguments.names.read_text())
    names = loaded if isinstance(loaded, list) else loaded["to_seal"]
    source = arguments.source.resolve()
    missing = [name for name in names if not (source / name).is_file()]
    if missing:
        raise SystemExit(f"not in {source}: {missing[:3]}")

    predecessor = None
    if arguments.predecessor_materials:
        previous_checkpoint = arguments.predecessor_materials / "CHECKPOINT" / "content-checkpoint.json"
        document = ev.load_canonical(previous_checkpoint)
        predecessor = {"sha256": ev.sha256_file(previous_checkpoint),
                       "checkpoint_id": str(document["checkpoint_id"]),
                       "volume_id": arguments.predecessor_volume_id or ""}

    if arguments.passphrase_file:
        passphrase = bytearray(arguments.passphrase_file.read_bytes().strip())
    else:
        passphrase = g17b.secret_service_passphrase()
    if arguments.gnupghome:
        use_rehearsal_key(arguments.gnupghome, arguments.signer)
    elif arguments.signer != ev.SIGNING_FINGERPRINT:
        raise SystemExit("a signer other than the pinned key needs --gnupghome; production signs with the pinned key")
    sealed_at = g17b.utc(dt.datetime.now(dt.timezone.utc))
    public_key = arguments.rehearsal_public_key or (REPO / "evidence" / "current" / "TRUST" / "openpgp-public.asc")
    try:
        checkpoint = seal_content(source, names, arguments.materials.resolve(),
                                  arguments.checkpoint_id, arguments.volume_id, predecessor,
                                  sealed_at, passphrase, arguments.signer, arguments.gnupghome,
                                  public_key)
        volume = build_volume(arguments.reserve_root.resolve(),
                              f"volume-{arguments.volume_id.rsplit('-', 1)[-1]}/"
                              f"notrios-evidence-{arguments.volume_id.rsplit('-', 1)[-1]}.iso",
                              arguments.volume_id, source, names, arguments.materials.resolve(),
                              passphrase, arguments.signer, arguments.gnupghome, public_key)
        catalog = append_catalog(arguments.reserve_root.resolve(), volume,
                                 arguments.materials.resolve(), arguments.checkpoint_id,
                                 arguments.evidence_dir.resolve(), passphrase,
                                 arguments.signer, arguments.gnupghome)
    finally:
        g17b.clear_secret(passphrase)
    print(json.dumps({"status": "sealed", "artifacts": len(names),
                      "checkpoint": checkpoint["checkpoint_id"], "volume": volume,
                      "catalog_entries": catalog["entry_count"]}, sort_keys=True, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
