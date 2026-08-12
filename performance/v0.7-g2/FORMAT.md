# G2 compact-record and compression candidate

This is an investigation specification, not an implemented sync protocol. G9
must either promote it deliberately with golden vectors or replace it.

## NCB1 canonical record stream

All integers use the shortest unsigned LEB128/Go `Uvarint` spelling. Decoders
reject overflow, truncation, and non-minimal spellings. Byte strings have no
normalization or alternate representation.

```text
"NCB1"
operation_count uvarint
repeat operation_count times:
  replica_id       16 raw bytes
  sequence         uvarint
  hlc_wall_ms      uvarint
  hlc_logical      uvarint (must fit uint32)
  kind             one byte
  object_id        16 raw bytes
  dependency_count uvarint
  dependencies     dependency_count × 32 raw SHA-256 bytes
  payload_length   uvarint
  payload           payload_length bytes
```

There are no maps, floats, optional fields, alternate integer widths, custom
tables, or extension bytes. The payload is exactly one valid canonical JSON
value produced by the operation-kind encoder; G9 must specify each payload
schema, field order, Unicode normalization rule, and integer domain. Unknown
operation kinds reject unless a future outer capability says otherwise.

The G2 prototype uses fixed hexadecimal identifiers in its logical model only
so JSONL remains inspectable. NCB1 stores their decoded bytes. It validates
every identifier, dependency count, payload, input length, record count, and
trailing byte before accepting a stream.

NCB1 is recommended for the operation-record part because the measured compact
benefit was material. The outer envelope manifest remains a canonical JSON
candidate so routing/version/part descriptors remain inspectable and can reuse
archive-v2 conventions.

## Deterministic compression profile

The candidate is gzip carrying DEFLATE at level 6 with one member and:

- `FLG=0` (no name, comment, extra fields, or header CRC);
- `MTIME=0`;
- `OS=255`;
- the ordinary CRC32 and uncompressed-size footer.

The prototype produces identical bytes on repeat runs. A Go compressor may
change output across toolchain versions while remaining a valid DEFLATE
encoder. G9 must pin cross-version golden bytes and an encoder compatibility
rule before a compressed artifact is used as a signed canonical byte string.
Admission is bounded independently by compressed bytes, expanded bytes, and a
64:1 ratio; none substitutes for record/count/dependency limits.

## Proposed G9 admission limits

| Bound | G2 recommendation |
|---|---:|
| Operations per envelope | 10,000 |
| Canonical record bytes | 16 MiB |
| Compressed record bytes | 4 MiB |
| One record | 1 MiB |
| One operation payload | 512 KiB |
| Dependencies per operation | 64 |
| Expanded/compressed ratio | 64:1 |
| Pending operations per peer | 10,000 |
| Pending encoded bytes per peer | 64 MiB, disk-backed |

These are simultaneous ceilings: a sender closes an envelope on the first one
reached. A receiver refuses wider declared limits. Pending overflow applies
backpressure and requests repair/snapshot catch-up; it never evicts an
arbitrary dependency and then applies a broken reference.
