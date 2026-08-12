# v0.7 G0 — Threat model, terminology, and reference validation

Status: complete investigation evidence, 2026-08-11.

This directory freezes the security vocabulary and control objectives that the
later v0.7 synchronization slices must satisfy. It contains no production
cryptography, transport, schema migration, or synchronization implementation.
Every control described here is **planned unless the text explicitly identifies
an existing v0.6 control**.

## Conclusions

1. Every v1 sync payload, including REST payloads, must use authenticated
   encryption. Non-loopback REST also requires TLS. The only plaintext mode is
   an explicit, non-interoperable, loopback-only development/test mode.
2. Every enrolled replica has a distinct Ed25519 signing identity. Envelope,
   advertisement, request, acknowledgement, and snapshot-manifest artifacts
   are signed with domain separation. Encryption keys remain separate from
   signing keys.
3. Signatures and hashes do not stop replay by themselves. Receivers also need
   database/protocol binding, replica/key status, sequence and dependency
   validation, state vectors, key epochs, and retained retirement/replay floors.
4. Revoking a replica's signing key is insufficient if it still knows an
   encryption key. Compromise or revocation must create a new recipient/key
   epoch for active replicas. Historical plaintext already seen by the device
   cannot be made secret again.
5. A valid but compromised enrolled replica can create correctly encrypted and
   signed destructive operations. v0.7 is single-user, multi-device sync; it
   has attribution and revocation, not per-operation multi-user authorization.
   Recovery therefore also depends on retained revisions, conflicts,
   tombstones, snapshots, explicit restore intent, and audit records.
6. The carrier is disposable and availability-hostile. It may delete, delay,
   duplicate, reorder, rename, or inject bytes. Peers and verified snapshots
   recover availability; the carrier is never a trust root or canonical store.
7. Visible routing metadata must be the minimum needed to discover and fetch an
   artifact. Plaintext note/resource hashes, names, titles, peer display names,
   and state vectors stay encrypted. A visible content address, where required,
   addresses ciphertext/artifact bytes rather than a raw plaintext object.
8. The useful ideas from Marmot, Cachapa, Subversion, rclone, and Ygo are
   patterns, not adopted wire formats or code. Notrios keeps its own small Go
   replication core and extends native archive v2; SVN dump compatibility is
   explicitly out of scope.

## Evidence map

- [`THREAT_MODEL.md`](THREAT_MODEL.md) — assets, adversaries, boundaries,
  assumptions, key lifecycle, metadata leakage, recovery, audit, and severity.
- [`PROTOCOL_GLOSSARY.md`](PROTOCOL_GLOSSARY.md) — normative meanings for the
  eleven G0 protocol terms and closely related security terms.
- [`MISUSE_CASES.md`](MISUSE_CASES.md) — abuse and failure cases traced to
  planned controls and owning plan slices.
- [`REFERENCE_VALIDATION.md`](REFERENCE_VALIDATION.md) — primary-source status,
  license/platform matrix, and adopt/reference/reject decisions.
- [`validate_evidence.py`](validate_evidence.py) — deterministic structural
  checks for the required G0 evidence.

## Existing versus planned controls

Existing v0.6 controls include stable database/replica identifiers, archive-v2
SHA-256 verification and manifest-last publication, explicit restore intent,
bounded archive validation, loopback default binding, MCP output/scope bounds,
remote-media SSRF/quarantine controls, and content-addressed asset storage.

Authenticated sync encryption, replica signatures, enrollment, key epochs,
replay state, sync-route authentication, sync quotas, carrier handling, and
peer-aware retention are not implemented by G0. Their owners are G3–G17 in
`PLAN.md`.

## Reproduction

Run:

```sh
python3 performance/v0.7-g0/validate_evidence.py
```

Upstream observations were made on 2026-08-11 from the primary sources linked
in `REFERENCE_VALIDATION.md`. Tag observations used `git ls-remote --tags
--refs` against each upstream repository; they are evidence of tags visible on
that date, not a promise that the largest semantic-looking tag is upstream's
recommended production release.
