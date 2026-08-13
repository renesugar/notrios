# G9 wire format

This is the written specification the independent golden generator implements.
It exists so the format is a specification rather than "whatever the Go encoder
produces": `generate_goldens.py` is written from this file, and the Go tests
compare against its output.

All integers are **minimal** unsigned varints, as `binary.PutUvarint` writes
them. A decoder re-encodes every varint and refuses anything longer, because one
logical value must have exactly one spelling — otherwise one envelope would have
two hashes and two signatures.

A `string` is a varint byte length followed by that many UTF-8 bytes.

## Operation block — `NCB1`

```
"NCB1"
uvarint operation_count
operation_count × {
  string   replica_id
  uvarint  sequence
  uvarint  hlc_wall_ms
  uvarint  hlc_logical
  string   kind
  string   record_type
  string   record_id
  string   created_at            (RFC 3339, normalized to UTC)
  uvarint  dependency_count
  dependency_count × { string replica_id ; uvarint sequence }
  uvarint  payload_length
  bytes    payload               (compact JSON, as normalization produced it)
}
```

Dependencies are sorted by `(replica_id, sequence)` before encoding. A caller's
ordering is not preserved, because it is not part of the logical operation.

## Envelope — `NEV1`

```
"NEV1"
uvarint protocol_major
uvarint protocol_minor
string  database_id
string  sender_replica_id
uvarint vector_entry_count
vector_entry_count × { string replica_id ; uvarint contiguous_sequence }
uvarint operation_block_length
bytes   operation_block          (an NCB1 block)
```

Vector entries are sorted by replica id and must be unique. A decoder refuses an
unsorted or duplicated list: a map has no order, so without this rule two
replicas would encode the same vector differently.

## Artifact — `NAR1`

```
"NAR1"
uvarint header_length
bytes   header                   (see below)
uvarint ciphertext_length        (must equal the header's own field)
bytes   ciphertext               (AES-256-GCM output: plaintext + 16-byte tag)
bytes   signature                (64 bytes, Ed25519)
```

### Header

The only part a carrier can read.

```
uvarint protocol_major
uvarint protocol_minor
string  artifact_kind            ("envelope" | "object" | "manifest" | "snapshot")
string  key_id
uvarint epoch
string  signer_key_id
string  routing_name             (may be empty; a keyed blind, never a content hash)
uvarint salt_length              (always 32)
bytes   salt
uvarint ciphertext_length
```

### Cryptographic binding

- **Per-artifact key**: `HKDF-SHA256(group_key, salt = canonical_header, info =
  "notrios.artifact-key.v1", 32)`. The header is both the HKDF salt and the AEAD
  associated data, so editing it changes the key *and* fails authentication.
- **Nonce**: twelve zero bytes. This is safe because uniqueness comes from the
  key, not the nonce: every artifact derives its own key from a fresh 32-byte
  random salt, so no two artifacts share a (key, nonce) pair.
- **Associated data**: the canonical header bytes.
- **Signature**: Ed25519 over
  `"notrios.artifact-signature.v1" || uvarint(len(header)) || header ||
  uvarint(len(ciphertext)) || ciphertext`.
  Encrypt-then-sign, so a receiver rejects a forgery without decrypting.
- **Routing name**: `HMAC-SHA256(HKDF(group_key, "notrios.routing-key.v1"),
  "notrios.routing-name.v1" || 0 || kind || 0 || content_address)`, truncated to
  16 bytes and hex-encoded.

### Bounds

Operations 10,000 · payload 512 KiB · dependencies 64 · vector entries 1,024 ·
encoded 16 MiB · compressed 4 MiB · expansion ratio 64:1. Every one is checked
before the allocation it guards, because the sender is not yet trusted.

### Compression

gzip level 6, with the modification time set to the Unix epoch and the OS byte
set to 255. Both are pinned because gzip records them by default, and an
artifact whose bytes depended on when or where it was produced could not be
content-addressed.
