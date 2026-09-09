# v0.7 G17b evidence-handling decision record

**Status:** complete decision record; G17b subsequently completed

**Date:** 2026-08-25

**Model:** GPT-5 (exact serving variant unavailable)

## Purpose

Record the user's answers to G17a's three blocking design questions without
starting G17b or exercising production credentials, timestamp services, the
external ISO reserve, GitHub, or physical media.

## Resolved scope

G17b preserves curated top-level release/plan ZIPs and content-free screenshots
present at its freeze, including the G17a ZIP and any verified evidence-decision
ZIP created before G17b. It excludes all three recursive G14 benchmark
workspaces. An unexplained top-level append or any recursive inclusion is a
refusal pending explicit review.

## Resolved OpenPGP identity

- UID: `Rene Sugar (Evidence Identity) <rene.sugar@gmail.com>`
- Ed25519 certification primary:
  `AEE5F82F2C216D6D15992C8DC96A1C6039BC8098`
- Exact Ed25519 signing subkey:
  `4ABEB98AF99C8321931BCF282C6A8A4568264005`
- Signing-subkey expiry: `2027-08-25T18:32:16Z`

A read-only GnuPG colon listing showed the primary secret offline (`sec#`) and
the signing secret usable (`ssb`). Production commands must force the full
signing-subkey fingerprint with `!`; email and short IDs are labels, not safe key
selectors. GnuPG documents both full-fingerprint selection and `!` semantics:
<https://gnupg.org/documentation/manuals/gnupg/Specify-a-User-ID.html>.

The user attests that separate Secret Service items store the operational secret
subkey export and passphrase and that the passphrase unlocked a test signature.
This planning pass did not retrieve either secret. Before the first production
signature, the owner must attest that a restorable full primary backup and
revocation certificate exist in offline custody; the project must not record
their secret bytes or locations.

## Resolved TSA provider order

1. DigiCert primary: `http://timestamp.digicert.com`
2. Sectigo fallback: `http://timestamp.sectigo.com`

Provider identity alone is not sufficient. An authorized generated pilot must
record and verify the actual policy OID, nonce, SHA-256 imprint, timestamping
EKU, responder and chain fingerprints, certificate validity at `genTime`, and
available revocation evidence using explicit CA/intermediate inputs. A passing
DigiCert result pins its observed policy/chain. A failing DigiCert result permits
one Sectigo pilot under the same gate and published pacing guidance. If neither
passes, stop and reopen the decision. No TSA endpoint was contacted here.

Provider sources:

- DigiCert: <https://knowledge.digicert.com/general-information/rfc3161-compliant-time-stamp-authority-server>
- Sectigo: <https://www.sectigo.com/resource-library/time-stamping-server>
- RFC 3161: <https://www.rfc-editor.org/rfc/rfc3161>

## Supplied implementation references

- The description of Notary Project `tspclient-go` is accurate. It is
  Apache-2.0 and supports RFC 3161/RFC 5816 request, transport, response, and
  verification: <https://github.com/notaryproject/tspclient-go>.
- Trail of Bits `rfc3161-client` is an Apache-2.0 Python API backed by Rust/PyO3
  for constructing and verifying protocol objects; it leaves network transport
  separate. Releases through 1.0.2 were yanked for CVE-2025-52556:
  <https://pypi.org/project/rfc3161-client/>.
- Notary Project Notation signs/verifies OCI and blob artifacts and supports
  RFC 3161, but uses the Notary signature/trust-policy model rather than the
  selected detached-OpenPGP manifest contract:
  <https://github.com/notaryproject/notation>.

None is added as a dependency by this decision. G17b retains the already tested
GnuPG/OpenSSL boundary unless a focused dependency and pinning review proves a
change necessary.

## Remaining blocking approvals

G17b has no unresolved scope, signer-selection, or provider-order design choice.
It remains blocked on an explicit start instruction authorizing:

1. the exact signing-subkey and non-logging Secret Service credential use;
2. generated and production requests to the selected TSA endpoints; and
3. writes below `/media/renes/SEAGATE2TB/notrios-evidence/`.

Offline primary-backup/revocation readiness must be attested before the first
production signature. GitHub push and physical burn remain separate and
unauthorized.

## Validation

- Read-only GnuPG fingerprint/capability/expiry listing; no secret export or use.
- Primary-source review of GnuPG key selection, both provider endpoints,
  `tspclient-go`, `rfc3161-client`, and Notation scope/licensing.
- Plan/status/handoff consistency, JSONL parsing, required-file and plan-loop
  checks, and standard repository tests are recorded in
  `agent/ATTEMPT_LOG.jsonl`. The verified release ZIP is reported after the
  completion commit so its embedded source anchors match that exact tree.

## Post-decision execution

The owner subsequently supplied every blocking operational authorization and
the offline recovery/revocation attestation on 2026-08-25. G17b completed under
those exact boundaries; no GitHub push or physical burn occurred. Its production
outcome and validation are archived in
`plans/v0.7/030-evidence-seals-iso-reserve.md`. This section preserves the
historical pre-authorization state above rather than rewriting the decision
sequence after the fact.
