# RFC 3161 authority assessment

Checked against provider-published material on 2026-08-24. G17a made no
timestamp request, health request, DNS probe, certificate download, or other
call to a TSA endpoint. Published URLs and certificate names can change; G17b
must repeat the review immediately before an approved pilot.

## Candidate comparison

| Candidate | Published facts | G17a disposition |
|---|---|---|
| DigiCert | Publishes an RFC 3161 URL, says timestamp responder certificates rotate at least every 15 months, and publishes the current responder/intermediate/root downloads. | Recommended pilot because the verification materials are most explicit. Selection remains blocked on terms, actual response policy OID, nonce/imprint/EKU/chain verification, and preservation/redistribution of trust material. |
| Sectigo | Publishes RFC 3161 support and a general endpoint; its usage guidance asks scripted callers to wait at least 15 seconds between items. Its legal repository publishes timestamp policies. | Viable fallback. A per-artifact 78-call backfill would take at least 19.5 minutes under the published courtesy interval, strengthening the case for one signed batch checkpoint. Exact generic versus eIDAS policy/chain still needs a pilot. |
| SSL.com | Publishes RFC 3161 C2PA-oriented RSA/ECC endpoints and audited operations, but directs users to request access and confirm use case/volume. | Not the unattended default for this non-C2PA evidence set without an account/use-case agreement. |

Provider marketing descriptions do not replace RFC 3161 verification. G17b
accepts a response only when the query/response nonce matches, SHA-256 message
imprint matches the exact checkpoint-signature bytes, `genTime`, serial, policy
OID, and TSA name are recorded, the leaf has critical `id-kp-timeStamping` EKU,
and OpenSSL verifies through explicitly pinned `-CAfile` and `-untrusted`
material. Preserve revocation/status material available at sealing time or state
plainly when long-term revocation evidence was not obtained.

Primary sources:

- RFC 3161: <https://www.rfc-editor.org/rfc/rfc3161>
- RFC 9921: <https://www.rfc-editor.org/rfc/rfc9921>
- OpenSSL `ts`: <https://docs.openssl.org/3.0/man1/openssl-ts/>
- DigiCert RFC 3161 service and chain:
  <https://knowledge.digicert.com/general-information/rfc3161-compliant-time-stamp-authority-server>
- Sectigo timestamp server guidance:
  <https://www.sectigo.com/faqs/detail/Time-Stamp-Server-Stamping-Protocols-for-Digital-Signatures-Code-Signing>
- Sectigo legal repository: <https://www.sectigo.com/legal>
- SSL.com TSA: <https://www.ssl.com/products/content-authenticity/timestamping/>
- GNU xorriso reproducibility notes:
  <https://www.gnu.org/software/xorriso/man_1_xorriso.html>
