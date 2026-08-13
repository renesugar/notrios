#!/usr/bin/env python3
"""Independently generate the canonical NCB1/NEV1 golden bytes.

This is written from the format description in `FORMAT.md`, not from the Go
implementation. That is the whole point of it: a round trip against the encoder
that produced the bytes would pass even if the format were wrong, so the goldens
are produced by a second implementation and the Go tests compare against them.

Run it to regenerate `goldens.json` after a deliberate format change. A change
that was not deliberate shows up as a failing Go test.
"""
from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parent

OPERATIONS_MAGIC = b"NCB1"
ENVELOPE_MAGIC = b"NEV1"


def uvarint(value: int) -> bytes:
    """Minimal unsigned varint, the same shape Go's binary.PutUvarint writes."""
    if value < 0:
        raise ValueError("varints are unsigned")
    out = bytearray()
    while True:
        byte = value & 0x7F
        value >>= 7
        if value:
            out.append(byte | 0x80)
        else:
            out.append(byte)
            return bytes(out)


def string_field(value: str) -> bytes:
    encoded = value.encode("utf-8")
    return uvarint(len(encoded)) + encoded


def encode_operation(operation: dict) -> bytes:
    out = bytearray()
    out += string_field(operation["replica_id"])
    out += uvarint(operation["sequence"])
    out += uvarint(operation["hlc_wall_ms"])
    out += uvarint(operation["hlc_logical"])
    out += string_field(operation["kind"])
    out += string_field(operation["record_type"])
    out += string_field(operation["record_id"])
    out += string_field(operation["created_at"])
    dependencies = operation.get("dependencies", [])
    # Dependencies are sorted by (replica id, sequence) before encoding; the
    # canonical form does not preserve the order a caller supplied.
    dependencies = sorted(dependencies, key=lambda d: (d["replica_id"], d["sequence"]))
    out += uvarint(len(dependencies))
    for dependency in dependencies:
        out += string_field(dependency["replica_id"])
        out += uvarint(dependency["sequence"])
    payload = operation["payload"].encode("utf-8")
    out += uvarint(len(payload))
    out += payload
    return bytes(out)


def encode_operations(operations: list[dict]) -> bytes:
    out = bytearray(OPERATIONS_MAGIC)
    out += uvarint(len(operations))
    for operation in operations:
        out += encode_operation(operation)
    return bytes(out)


def encode_envelope(envelope: dict) -> bytes:
    operations = encode_operations(envelope["operations"])
    out = bytearray(ENVELOPE_MAGIC)
    out += uvarint(envelope["protocol_major"])
    out += uvarint(envelope["protocol_minor"])
    out += string_field(envelope["database_id"])
    out += string_field(envelope["sender_replica_id"])
    vector = envelope["state_vector"]
    out += uvarint(len(vector))
    # Sorted by replica id, because a map has no order and two replicas must
    # produce the same bytes for the same vector.
    for replica_id in sorted(vector):
        out += string_field(replica_id)
        out += uvarint(vector[replica_id])
    out += uvarint(len(operations))
    out += operations
    return bytes(out)


def main() -> None:
    cases = json.loads((ROOT / "golden-inputs.json").read_text(encoding="utf-8"))
    goldens = {"schema": "notrios.g9.goldens.v1", "cases": []}
    for case in cases["cases"]:
        if case["kind"] == "operations":
            encoded = encode_operations(case["operations"])
        elif case["kind"] == "envelope":
            encoded = encode_envelope(case["envelope"])
        else:
            raise SystemExit(f"unknown golden kind {case['kind']!r}")
        goldens["cases"].append({
            "name": case["name"],
            "kind": case["kind"],
            "hex": encoded.hex(),
            "byte_length": len(encoded),
        })
    (ROOT / "goldens.json").write_text(json.dumps(goldens, indent=2) + "\n", encoding="utf-8")
    print(f"g9 goldens: wrote {len(goldens['cases'])} cases")


if __name__ == "__main__":
    main()
