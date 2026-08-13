# G9 findings

## What the canonical encoding costs

Generated operations of the shape the journal actually stores, against the JSON
representation it stores them as today:

| Operations | JSON | Canonical | Smaller | JSON + gzip | Canonical + gzip | Smaller |
|---:|---:|---:|---:|---:|---:|---:|
| 100 | 34,082 B | 13,751 B | 59.65% | 2,060 B | 1,481 B | 28.11% |
| 10,000 | 3,447,784 B | 1,398,827 B | 59.43% | 179,324 B | 121,678 B | 32.15% |

G2 measured 51.9% smaller raw and 16.3% smaller compressed at ten thousand
operations, and selected the compact codec on that basis. The production numbers
are better in both columns, and the reason is worth stating rather than
celebrating: G2's corpus used fixed sixteen-byte identifiers, while a production
operation carries a longer field-name set — `record_type`, `created_at`,
`operation_id` — and JSON pays for every one of those names in every record.
The two measurements are therefore not directly comparable. What they agree on
is the direction and the rough magnitude, which is what the selection rested on.

The compressed column is the one that matters for a carrier, and it is the
smaller saving. Dropping field names removes exactly the kind of redundancy gzip
was already removing.

## What the crypto costs

| Operations | Artifact | Overhead | Seal | Open |
|---:|---:|---:|---:|---:|
| 100 | 13,932 B | 181 B | <1 ms | 1 ms |
| 10,000 | 1,399,010 B | 183 B | 48 ms | 45 ms |

The overhead is the *whole* cost of confidentiality and authenticity: a header,
a 16-byte AEAD tag, and a 64-byte signature. It is constant — 181 to 183 bytes
regardless of envelope size, the two-byte difference being the varint width of
the length fields — so encryption is free at any scale that matters and
expensive only for an envelope carrying almost nothing.

Sealing and opening a 1.4 MB envelope cost about 46 ms each on this host. Both
are a single pass of AES-GCM plus one Ed25519 operation, so they scale with
bytes rather than with operation count.

## The nonce, and why it is zero

Every artifact derives its own encryption key by HKDF from a fresh 32-byte
random salt carried in the header. Because the key is unique per artifact, the
nonce does not have to be: no two artifacts share a (key, nonce) pair even
though every artifact uses the same twelve zero bytes. Deriving a random nonce
as well would add nothing and would make the header one field longer.

This is asserted rather than asserted-in-prose: a test seals the same plaintext
twice and requires the artifacts, their salts, and their derived keys all to
differ.

## The header is bound twice, on purpose

The canonical header is the HKDF salt *and* the AEAD associated data. Either
alone would catch a modified header — the derivation would produce a different
key, or authentication would fail — so this is deliberate redundancy. There is
no single check whose absence would let a header be edited in transit.

A test proves the associated-data half specifically: it re-signs a *different*
header over the same ciphertext with a genuine key, so the signature verifies
and only the AEAD refuses.

## What a receiver refuses

Every one of these is a fixture:

| Case | Result |
|---|---|
| Any single bit flipped, anywhere in the artifact | refused — every offset tested |
| Truncation, or a trailing byte | refused |
| A non-minimal varint, or unsorted vector entries | `ErrNotCanonical` |
| An operation count claiming four billion records | `ErrLimitExceeded`, before allocating |
| A signature from an unenrolled or revoked replica | `ErrUnknownSigner` |
| A valid signature over a substituted header | `ErrDecrypt` |
| The wrong group key | `ErrDecrypt` |
| An artifact from a retired epoch | `ErrRevokedEpoch` |
| A gzip stream that expands past the ratio or the ceiling | `ErrLimitExceeded` |
| Two gzip members concatenated | `ErrNotCanonical` |
| An envelope whose declared sender is not its signer | `ErrSenderMismatch` |

The signature is checked **before** decryption, so bytes that fail it never
reach the cipher and never require a plaintext-sized allocation.

## Known-answer tests, not just round trips

A round trip against this package would pass even if the key were the plaintext.
Three published vectors prove the primitives are wired correctly: RFC 8032
Ed25519 test vector 1, NIST GCM test case 14 for AES-256, and RFC 5869 HKDF
test case 1 — the last through the same derivation call the artifact key uses.

## Three things this does not cover

**No secret store.** The key ring and signer are interfaces with in-memory test
providers. Where a group key actually lives, and how a device unlocks it, is
v0.8's, exactly as the plan item states. G0's note stands: `go-keyring`
documents macOS, Linux/BSD, and Windows, not Android.

**No transport.** Nothing here publishes, fetches, or names an artifact on a
carrier. G11's shared directory and G14's REST data plane do that, and both will
use these artifacts unchanged.

**Signing is supplied, not yet wired to purge.** G6 recorded that local purge
stays gated until G9 supplies signing. G9 supplies it — `SignDeathCertificate`
and `VerifyDeathCertificate` bind the document, the replica, and the sequence,
and every field is tested — but the enrolled-purge path is not switched over
here. Purge propagation is entangled with the retention horizon G17 owns, and
changing what permanently deletes a note is not a change to make as a side
effect of adding a codec.
