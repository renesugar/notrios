#!/usr/bin/env python3
"""Destructive-in-temporary-space refusal matrix for G17b evidence."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

REPO = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO))
from evidence import verify_evidence as ev


def expect_refusal(results: dict[str, bool], name: str, operation) -> None:
    try:
        operation()
    except (ev.EvidenceError, OSError, ValueError):
        results[name] = True
        return
    raise SystemExit(f"refusal matrix unexpectedly accepted: {name}")


def flip(path: Path) -> None:
    data = bytearray(path.read_bytes())
    if not data:
        raise SystemExit(f"empty refusal fixture: {path.name}")
    data[len(data) // 2] ^= 1
    path.write_bytes(data)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--reserve-root", type=Path, required=True)
    args = parser.parse_args()
    repo = args.repo.resolve()
    current = repo / "evidence" / "current"
    catalog = repo / "evidence" / "outer-iso-catalog.jsonl"
    iso = args.reserve_root.resolve() / "volume-0001" / "notrios-evidence-0001.iso"
    results: dict[str, bool] = {}

    with tempfile.TemporaryDirectory(prefix="notrios-g17b-refusal-") as directory:
        work = Path(directory)

        metadata = work / "metadata"
        shutil.copytree(current, metadata)
        manifest = metadata / "CHECKPOINT" / "content-manifest.jsonl"
        original_manifest = manifest.read_bytes()
        flip(manifest)
        expect_refusal(results, "manifest_tamper", lambda: ev.verify_checkpoint(metadata, verify_payload=False))
        manifest.write_bytes(original_manifest)

        checkpoint_signature = metadata / "CHECKPOINT" / "content-checkpoint.json.sig"
        original_signature = checkpoint_signature.read_bytes()
        flip(checkpoint_signature)
        expect_refusal(results, "checkpoint_signature_tamper",
                       lambda: ev.verify_checkpoint(metadata, verify_payload=False))
        checkpoint_signature.write_bytes(original_signature)

        public_key = metadata / "TRUST" / "openpgp-public.asc"
        original_key = public_key.read_bytes()
        public_key.write_text("not a public key\n", encoding="utf-8")
        expect_refusal(results, "bad_openpgp_key",
                       lambda: ev.verify_checkpoint(metadata, verify_payload=False))
        public_key.write_bytes(original_key)

        checkpoint = metadata / "CHECKPOINT" / "content-checkpoint.json"
        query = metadata / "CHECKPOINT" / "content-checkpoint.sig.tsq"
        response = metadata / "CHECKPOINT" / "content-checkpoint.sig.tsr"
        root = metadata / "TRUST" / "tsa-root.pem"
        chain = metadata / "TRUST" / "tsa-untrusted.pem"
        responder = metadata / "TRUST" / "tsa-responder.pem"
        wrong = work / "wrong-data"
        wrong.write_bytes(checkpoint_signature.read_bytes() + b"x")
        expect_refusal(results, "wrong_timestamp_data",
                       lambda: ev.verify_timestamp(wrong, query, response, root, chain, responder))
        expect_refusal(results, "stale_timestamp_query",
                       lambda: ev.verify_timestamp(checkpoint_signature,
                                                   repo / "evidence" / "pilot" / "generated-query.tsq",
                                                   response, root, chain, responder))
        expect_refusal(results, "bad_tsa_ca",
                       lambda: ev.verify_timestamp(checkpoint_signature, query, response,
                                                   responder, chain, responder))
        tampered_response = work / "tampered.tsr"
        tampered_response.write_bytes(response.read_bytes()[:-1])
        expect_refusal(results, "timestamp_response_tamper",
                       lambda: ev.verify_timestamp(checkpoint_signature, query,
                                                   tampered_response, root, chain, responder))

        extracted = work / "extracted"
        extracted.mkdir()
        ev.extract_iso(iso, extracted)
        signatures = sorted((extracted / "SIGNATURES").iterdir())
        signatures[0].chmod(0o644)
        signatures[1].chmod(0o644)
        first_bytes = signatures[0].read_bytes()
        second_bytes = signatures[1].read_bytes()
        signatures[0].unlink()
        expect_refusal(results, "missing_member",
                       lambda: ev.verify_checkpoint(extracted, verify_payload=True))
        signatures[0].write_bytes(first_bytes)
        extra = extracted / "SIGNATURES" / "unexpected.sig"
        extra.write_bytes(b"unexpected")
        expect_refusal(results, "extra_member",
                       lambda: ev.verify_checkpoint(extracted, verify_payload=True))
        extra.unlink()
        signatures[0].write_bytes(second_bytes)
        signatures[1].write_bytes(first_bytes)
        expect_refusal(results, "swapped_members",
                       lambda: ev.verify_checkpoint(extracted, verify_payload=True))
        signatures[0].write_bytes(first_bytes)
        signatures[1].write_bytes(second_bytes)

        bad_catalog = work / "bad-catalog.jsonl"
        shutil.copyfile(catalog, bad_catalog)
        flip(bad_catalog)
        expect_refusal(results, "catalog_tamper", lambda: ev.load_catalog(bad_catalog))

        missing_reserve = work / "missing-reserve"
        missing_reserve.mkdir()
        expect_refusal(results, "unavailable_reserve", lambda: ev.verify_reserve(
            missing_reserve, catalog, repo / "evidence" / "outer-iso-catalog-checkpoint.json",
            repo / "evidence" / "outer-iso-catalog-checkpoint.json.sig",
            repo / "evidence" / "outer-iso-catalog-checkpoint.sig.tsq",
            repo / "evidence" / "outer-iso-catalog-checkpoint.sig.tsr",
            root, chain, responder, public_key))

        tampered_reserve = work / "tampered-reserve" / "volume-0001"
        tampered_reserve.mkdir(parents=True)
        tampered_iso = tampered_reserve / iso.name
        shutil.copyfile(iso, tampered_iso)
        flip(tampered_iso)
        expect_refusal(results, "iso_hash_tamper", lambda: ev.verify_reserve(
            tampered_reserve.parent, catalog,
            repo / "evidence" / "outer-iso-catalog-checkpoint.json",
            repo / "evidence" / "outer-iso-catalog-checkpoint.json.sig",
            repo / "evidence" / "outer-iso-catalog-checkpoint.sig.tsq",
            repo / "evidence" / "outer-iso-catalog-checkpoint.sig.tsr",
            root, chain, responder, public_key))

    root_commit = subprocess.run(
        ["git", "rev-list", "--max-parents=0", "HEAD"], cwd=repo, check=True,
        text=True, stdout=subprocess.PIPE).stdout.splitlines()[0]
    reverse = subprocess.run(
        ["git", "merge-base", "--is-ancestor", "HEAD", root_commit], cwd=repo,
        check=False).returncode
    if reverse == 0:
        raise SystemExit("refusal matrix unexpectedly accepted: catalog/head mutation")
    results["catalog_head_mutation"] = True

    print(json.dumps({"schema": "notrios.evidence.refusal-results.v1",
                      "refusals": results, "passed": len(results)}, sort_keys=True))


if __name__ == "__main__":
    main()
