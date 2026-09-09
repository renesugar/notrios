# v0.7 G17b — evidence seals and immutable ISO reserve

**Status:** complete

**Date:** 2026-08-25

**Model:** GPT-5 (exact serving variant unavailable)

## Goal and authority

Apply the G17a preservation contract to the curated external handoffs, commit
public verification material, issue a deterministic CD-sized ISO on the
designated SEAGATE reserve, and fail closed before any future GitHub push.

The owner explicitly authorized the exact Ed25519 signing subkey, non-logging
Secret Service passphrase retrieval, generated and production DigiCert/Sectigo
RFC 3161 requests, and writes below the evidence reserve. The owner attested
that the full primary-key backup and revocation certificate are restorable
offline. Their secret bytes and locations were not inspected or recorded.
GitHub push and physical optical burning remained unauthorized and did not
occur.

## Frozen source and provenance

The production freeze contains 81 top-level files and 284,012,518 original
bytes. Recomputing the G17a projection after removing the three named appends
reproduced the 78-file commitment exactly. The accepted appends are the G17a
release ZIP, the evidence-handling decision ZIP, and the verified G17b release
ZIP. All three recursive private benchmark workspaces remained excluded.

Every ZIP passed complete CRC, duplicate/path, encryption, and symlink checks;
every PNG passed signature/chunk CRC checks. The originals were hashed and
signed without rewriting their metadata. Six embedded source anchors resolve
76 ZIPs uniquely to 76 commits. The four screenshots are not applicable to
commit mapping. Legacy `notrios.zip` has fewer than three anchors and remains
explicitly `insufficient`; its filename was not used as provenance.

The verified G17b release ZIP is
`notrios-v0.7-g17b-20260825.zip`, 4,529,419 bytes, 1,234 entries, SHA-256
`3e7b81c1cc20963f016a46eb59ebac66084df3f732696d9070738afb52b3ea20`.
It was created from implementation commit `d433a61`, checked with the repository
release verifier, then included as the 81st source artifact. The later
catalog-only closure intentionally does not create another ZIP.

## OpenPGP and RFC 3161 seals

The exact signing selector was
`4ABEB98AF99C8321931BCF282C6A8A4568264005!` under primary
`AEE5F82F2C216D6D15992C8DC96A1C6039BC8098`. The passphrase moved only from
Secret Service to GnuPG standard input. It never appeared in arguments,
environment variables, files, output, manifests, the ISO, or logs. The locally
usable signing subkey made secret-subkey export retrieval unnecessary.

The generated DigiCert pilot passed nonce and SHA-256 imprint matching,
critical timestamping EKU, certificate validity at `genTime`, and explicit
chain verification. It pinned:

- policy OID `2.16.840.1.114412.7.1`;
- responder SHA-256
  `4aa03fa22cd75c84c55c938f828e676b9caecab33fe36d269aa334f146110a33`;
- DigiCert Trusted Root G4 SHA-256
  `552f7bdcf1a7af9e6ce672017f4f12abf77240c78e761ac203d1d9d20ac89988`.

Sectigo was not contacted. OCSP/CRL URLs are preserved in the responder
certificate, but revocation material was not fetched because the authorization
named the selected TSA endpoints rather than their separate status services.

The 162-entry manifest contains one artifact and one detached-signature entry
per source. Content commit `538b74d9976522f01dea54dfe9dd5d1b38055ca0`
precedes imaging and anchors:

- manifest SHA-256
  `683c10aaf2ee114306c799d431d3f33e202d77356d47ee0e237f56a5ab505bac`;
- checkpoint SHA-256
  `ed88bbdf9a5875d0a86547f5560101e99cbc9c8256764f399fa785286895a298`;
- content-checkpoint token `genTime` `2026-08-26T00:21:58Z`.

All historical artifacts are marked `retroactive: true`. The token establishes
that the exact checkpoint signature existed by its `genTime` under the pinned
policy and chain; it does not convert old task dates into contemporaneous seals.

## ISO and finite catalog closure

Two separately assembled, normalized staging trees on the external reserve
contained 179 files and produced byte-identical xorriso 1.5.6 images. The final
print size was 139,127 blocks / 284,932,096 bytes, 41.80% of the conservative
650 MiB project budget. The issued immutable volume is:

```text
volume ID: NTR-EV-0001
reserve relative path: volume-0001/notrios-evidence-0001.iso
SHA-256: f4df1e047e3f372efdf5ab3d3a89089f2413c1e91243afaff01012054161258f
signature token genTime: 2026-08-26T00:25:40Z
```

Its adjacent `.sha256`, `.sig`, `.sig.tsq`, `.sig.tsr`, and verification JSON
exist on the reserve. A clean temporary extraction reproduced every staging
hash and passed all artifact structure, exact OpenPGP identity, canonical chain,
checkpoint, nonce/imprint, policy, EKU, time-validity, and explicit-chain checks.

The final outer catalog SHA-256 is
`b47f9a7d1879459ee7b0c269aafe852c577e514fadf3369ba6137f5b94a0b1c0`.
Its checkpoint SHA-256 is
`dde374e88f1548bcbd4fa0cd6498f306918d4c79e7adfac60950b4d666c7603e`
and its final token `genTime` is `2026-08-26T00:31:18Z`. The first catalog seal
cryptographically verified, but review found that it mislabeled the ISO
signature token time as ISO creation time. That full seal is retained under
`evidence/superseded/catalog-attempt-0001/`; the corrected catalog uses an
explicit local-clock custody event, weak ISO file mtime, and separately labelled
ISO-signature `genTime`.

The commit containing this archive and final catalog is the catalog-only
closure commit. It is deliberately outside `NTR-EV-0001`; requiring a new ZIP
and ISO for that commit would recurse forever. Resolve it with
`git log -1 --format=%H -- evidence/outer-iso-catalog.jsonl`. The next ordinary
content checkpoint covers this closure.

## Verification and refusal evidence

The offline verifier needs no network, private key, ambient keyring/CA store,
mounted ISO, or original repository. The host pre-push gate additionally checks
the current curated source, external reserve, volume ID/hash, and Git ancestry.
The committed refusal harness passed 14 cases:

- manifest, checkpoint signature, timestamp response, catalog, and ISO tamper;
- wrong OpenPGP key, TSA CA, timestamp datum, and stale query;
- missing, extra, and swapped members;
- unavailable reserve; and
- catalog/HEAD ancestry mutation.

Standard validation passed: npm audit before/after `npm ci` with zero
vulnerabilities, frontend typecheck/163 tests/build, `go test ./... -count=1`,
`go vet ./...`, G17a unit/aggregate validation, evidence unit/refusal tests,
scaffold and required-file checks, docs build, plan-loop/JSONL/privacy checks,
release ZIP verification, tracked/source/reserve verification, and the final
pre-push command. The first frontend/Go timing pass was starved by a stale G17a
recursive xorriso probe left running since August 24; the exact excluded probe
was terminated, after which the unchanged suites passed. No application schema,
runtime endpoint, dependency, original evidence byte, remote, or physical
medium changed.
