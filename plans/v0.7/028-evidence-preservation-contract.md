# v0.7 G17a — evidence-preservation contract investigation

**Status:** complete

**Date:** 2026-08-24

**Model:** GPT-5 (exact serving variant unavailable)

## Goal and boundary

Define and test an honest, scalable preservation contract for the accumulated
Notrios release ZIPs and screenshots before any GitHub push. The investigation
was read-only against `/home/renes/evidence/notrios` and used only aggregate
committed inventory evidence. Generated cryptographic and ISO fixtures lived in
a disposable `/tmp` workspace.

This slice did not create or use a production signing identity, contact a TSA,
write the evidence or reserve directories, create a production ISO, push Git,
burn media, or assert WORM status, authorship, legal custody, or admissibility.

## Inventory and provenance result

The frozen curated top-level capture contains 78 files: 74 ZIPs and four PNGs,
totaling 270,506,844 bytes. Full ZIP decompression/CRC and path-safety checks and
PNG chunk-CRC checks passed for every file. No `.sig`, `.asc`, `.tsq`, or `.tsr`
sidecar was present.

Six embedded Git-tree anchors uniquely map each of the 73 current release-shaped
ZIPs to a distinct commit. The legacy `notrios.zip` contains none of the anchors
and remains unknown. Filenames and filesystem mtimes are never promoted into
commit or completion proof. The committed evidence publishes aggregate results
and two inventory commitments; the detailed filename/hash/candidate capture
remained private scratch data and is not an authority for G17b.

The root also contains three recursive G14 benchmark workspaces with private
inputs, caches, repositories, logs, diagnostics, and generated artifacts. A
read-only ISO probe had observed at least 47,400 nodes when stopped. Recursive
inclusion is therefore neither a privacy-safe default nor consistent with the
one-disc premise. G17b needs an explicit scope approval; the recommendation is
the curated top-level handoffs plus the expected G17a release ZIP.

## Selected contract

- Each original artifact receives a binary detached OpenPGP signature.
- A versioned canonical JSONL chain hashes the complete canonical entry core and
  records exact hashes and sizes for every artifact and signature.
- A canonical content checkpoint commits the complete manifest; its detached
  OpenPGP signature is the datum submitted once to RFC 3161.
- The timestamp token therefore binds all covered artifact/signature bytes to a
  complete-batch time claim. Per-artifact tokens are optional standalone-
  extraction material, not the scalable default.
- Backfilled material is always `retroactive: true`; capture, independently
  evidenced task completion, signing, TSA time, and custody events remain
  separate fields.
- The deterministic ISO contains payload, signatures, checkpoint, trust
  material, verifier, and instructions. A later signed/timestamped outer catalog
  records the byte-final ISO hash because an ISO cannot contain its own hash.
- Issued volumes are immutable and numbered. The catalog-only closure commit is
  an explicit finite boundary rather than creating an infinite release-ZIP loop.

The selected ISO profile is xorriso 1.5.6, ISO level 3, rationalized Rock Ridge,
Joliet-long, an ASCII volume ID of at most 16 characters, fixed time/ownership/
permissions, and a sorted explicit whitelist. The conservative project budget
is 650 MiB (681,574,400 bytes), not a universal CD capacity assertion. The
curated source set prints as 270,962,688 bytes (132,306 sectors), or 39.75% of
that budget. G17b must rerun `-print-size` against its complete final staging and
the actual selected medium.

## Generated prototype evidence

The end-to-end disposable self-test created a one-day fixture-only OpenPGP key,
a local fixture root/intermediate/TSA, two staging trees, and two ISO images. It
passed:

- canonical entry-chain validation and mutation refusal;
- correct detached-signature verification and tampered-data refusal;
- RFC 3161 query/response nonce, SHA-256 imprint, policy, explicit root and
  intermediate chain verification;
- wrong-data and wrong-CA refusal;
- two clean byte-identical 401,408-byte ISO builds;
- print-size equality, Unicode/long-name and rationalized-permission checks;
- extraction membership and complete hash-walk equality.

Tool versions were GnuPG 2.4.4, OpenSSL 3.0.13, and xorriso 1.5.6. These fixture
keys and certificates are not production trust anchors.

## RFC 3161 assessment

Published materials support a one-checkpoint DigiCert pilot as the recommended
next investigation because DigiCert documents a generic RFC 3161 service and
downloadable responder chain. Sectigo is a fallback and documents a 15-second
interval for scripted calls. SSL.com's published service is C2PA/access-
coordinated. No endpoint was contacted. G17b must repeat the terms, policy,
certificate, EKU, chain, nonce/imprint, and availability review immediately
before an approved generated pilot and preserve explicitly pinned trust input.

Primary references:

- RFC 3161: <https://www.rfc-editor.org/rfc/rfc3161>
- RFC 9921: <https://www.rfc-editor.org/rfc/rfc9921>
- OpenSSL `ts`: <https://docs.openssl.org/3.0/man1/openssl-ts/>
- DigiCert RFC 3161 documentation: <https://knowledge.digicert.com/general-information/rfc3161-compliant-time-stamp-authority-server>
- Sectigo timestamp guidance: <https://www.sectigo.com/faqs/detail/Time-Stamp-Server-Stamping-Protocols-for-Digital-Signatures-Code-Signing>
- SSL.com TSA: <https://www.ssl.com/products/content-authenticity/timestamping/>
- GNU xorriso manual: <https://www.gnu.org/software/xorriso/man_1_xorriso.html>

## G17b blockers

1. Approve the curated top-level handoffs plus the G17a ZIP, or separately scope
   a privacy-reviewed recursive workspace selection.
2. Approve an exact existing OpenPGP fingerprint or creation of the recommended
   dedicated offline-primary evidence identity and replaceable signing subkey,
   including UID, algorithm, expiry, backups, revocation, and attestation.
3. Approve the recommended DigiCert generated pilot and, only after successful
   review, the exact authority/policy/chain for production checkpoint sealing;
   otherwise explicitly omit or waive the third-party time claim.

G17b, G18, all later work, and every GitHub push remain blocked until these
decisions are resolved.

## Files

- `EVIDENCE_PRESERVATION.md`
- `performance/v0.7-g17a/README.md`
- `performance/v0.7-g17a/FINDINGS.md`
- `performance/v0.7-g17a/TSA_ASSESSMENT.md`
- `performance/v0.7-g17a/INVENTORY_SUMMARY.json`
- `performance/v0.7-g17a/RECURSIVE_SCOPE_FINDING.json`
- `performance/v0.7-g17a/PROTOTYPE_RESULTS.json`
- `performance/v0.7-g17a/prototype.py`
- `performance/v0.7-g17a/test_prototype.py`
- `performance/v0.7-g17a/validate_evidence.py`

## Validation

- `python3 -m unittest discover -s performance/v0.7-g17a -p 'test_*.py' -v`
  — six tests pass.
- `python3 performance/v0.7-g17a/validate_evidence.py` — aggregate inventory,
  exact commit mappings, generated cryptographic/ISO contract, and document
  statements agree.
- Fresh host self-test under `/tmp` — generated-only OpenPGP/RFC 3161/ISO chain
  passes all positive and negative checks described above.
- Standard repository validation is recorded in `agent/ATTEMPT_LOG.jsonl`; the
  verified release ZIP is reported in the task handoff after the completion
  commit so its embedded source anchors match that exact tree.
