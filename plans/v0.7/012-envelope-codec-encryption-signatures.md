# v0.7 G9 — deterministic envelope/container codec, encryption, and signatures

Status: **complete**, 2026-08-13. Product remains 0.6.0; the canonical schema
stays **v23** — G9 adds no schema and no dependency. Implemented by Claude Opus 5
(`claude-opus-5`) under Claude Code, after explicit user approval naming G9.

## Goal

Turn admitted operation ranges and objects into bounded, deterministic,
encrypted, signed protocol artifacts reusable by every transport.

## What landed

**`internal/syncwire`** holds the whole wire contract, transport- and
storage-neutral like the G5–G8 cores:

- **Canonical encoding** — G2's NCB1 promoted, plus an NEV1 envelope. Minimal
  varints, sorted state vectors and dependencies, pinned gzip level and header
  fields. One logical envelope has exactly one byte representation.
- **NAR1 artifacts** — AES-256-GCM under a key derived per artifact by
  HKDF-SHA256 from a fresh 32-byte salt, signed with Ed25519 over
  domain-separated canonical outer bytes.
- **Epochs** — advancing mints a new key for what this replica publishes;
  retiring decides what it will still read. They are separate acts.
- **Blinded routing names** — HMAC-SHA256 under a derived routing key, so a
  shared folder's listing does not tell a reader which content a library holds.
- **Key and signer interfaces** with in-memory test providers, as the item
  specified; the secret store is v0.8's.

## Decisions taken, and why

**The standard library is the whole dependency list.** G0's rule was to use a
maintained implementation rather than invent one. Go 1.24 moved HKDF into
`crypto/hkdf`, and AES-GCM, Ed25519, and HMAC were already there, so G9 needs no
external package — no version pin, no mobile-build check, no supply-chain
review. The license inventory the item asked for is one line: Go standard
library, BSD-3-Clause.

**The nonce is twelve zero bytes, and that is the safe choice here.** Uniqueness
comes from the key, not the nonce: every artifact derives its own key from a
fresh random salt, so no two share a (key, nonce) pair. A random nonce would add
nothing and lengthen the header. A test seals the same plaintext twice and
requires the artifacts, salts, and derived keys all to differ.

**The header is bound twice.** It is the HKDF salt *and* the AEAD associated
data. Either alone would catch an edit, which is the point: there is no single
check whose absence would let a header through. A test re-signs a substituted
header with a genuine key so the signature verifies and only the AEAD refuses.

**Encrypt-then-sign.** A receiver rejects a forged artifact by checking one
signature over bytes it has not decrypted and has not allocated a plaintext for.

**G2's identifier assumption did not survive contact.** The investigation
encoded replica and object identifiers as fixed sixteen-byte hex values, which
its generated corpus satisfied. Production identifiers are bounded strings — a
replica id up to 128 characters, a record id up to 2,048 — so every identifier
is length-prefixed instead. The saving G2 measured came from dropping JSON's
field names and quoting, and that is unaffected. Recorded here because the
promoted codec is not byte-identical to the prototype that justified it.

**Signing is supplied; purge is not switched over.** G6 recorded that local
purge stays gated until G9 supplies signing. `SignDeathCertificate` and
`VerifyDeathCertificate` supply it, binding document, replica, and sequence with
every field tested. Wiring the enrolled-purge path to them is deliberately not
done here: purge propagation is entangled with the retention horizon G17 owns,
and changing what permanently deletes a note is not a change to make as a side
effect of adding a codec.

## Validation

Evidence is `performance/v0.7-g9/` (`FORMAT.md`, `golden-inputs.json`,
`goldens.json`, `generate_goldens.py`, `wire-results.json`, `FINDINGS.md`,
`validate_evidence.py`).

**The goldens are independently generated.** `FORMAT.md` is the specification;
`generate_goldens.py` is a second implementation of it written from that
document rather than from the Go package, and the Go test compares the two byte
for byte across seven cases including Unicode record ids, out-of-order
dependencies, varint boundaries, and empty blocks. A round trip against the
encoder that produced the bytes would pass even if the format were wrong.

**Known-answer tests, not just round trips**: RFC 8032 Ed25519 vector 1, NIST
GCM test case 14, and RFC 5869 HKDF test case 1 — the last through the same
derivation call the artifact key uses.

**Measured**: the canonical encoding is 59.65% and 59.43% smaller than the JSON
the journal stores today at 100 and 10,000 operations, and 28.11% and 32.15%
smaller after deterministic gzip. Crypto overhead is 181–183 bytes regardless of
envelope size. Sealing and opening a 1.4 MB envelope cost about 46 ms each.

**Refused**: every single-bit flip at every offset, truncation, a trailing byte,
non-minimal varints, unsorted vector entries, an operation count claiming four
billion records, an unenrolled or revoked signer, a valid signature over a
substituted header, the wrong group key, a retired epoch, gzip bombs by both the
ratio and the ceiling, two gzip members, and an envelope whose declared sender is
not its signer. Two fuzz targets found nothing.

Repository validation, audit first as `AGENTS.md` requires:

```text
cd web && npm audit / npm ci && npm audit          0 vulnerabilities
npm run typecheck / test -- --run / build          pass, 15 files, 155 tests
bash scripts/run_offline_assets_check.sh           no third-party request
go vet ./... / go test ./...                       pass
python3 scripts/check_required_files.py            70 files
python3 scripts/check_plan_loops.py                pass
bash scripts/validate-scaffold.sh                  pass
bash scripts/build_docs_site.sh                    15 pages
bash scripts/mvp_smoke.sh                          pass
bash scripts/run_performance_smoke.sh              pass
python3 performance/v0.7-g9/validate_evidence.py   2 tiers, 7 goldens
git diff --check                                   clean
scripts/package_release.sh + check_release_zip.py  verified
```

## Out of scope, and still owned elsewhere

No transport publishes or fetches an artifact (G11, G14). No secret store holds
a key (v0.8). No peer is authenticated or enrolled (G13). No snapshot catch-up
(G10). Nothing decrypted is exposed through MCP, and no password appears in any
manifest — both boundaries the item set. The store still journals and admits
JSON operations locally; wiring the canonical codec into the journal's own
storage was never in scope, because the journal is local state and this is a
wire format. G10 remains unapproved.
