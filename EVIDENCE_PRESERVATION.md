# Evidence preservation and optical reserve contract

Status: G17a investigation contract with the 2026-08-25 scope, signer, and TSA
design decisions resolved. G17b is not approved and no production evidence has
been sealed.

This document defines an engineering preservation record for Notrios release
artifacts. It is not legal advice and does not declare any artifact admissible,
independently created, clean-room, original, or authored by a particular person.
Those conclusions require facts and authority outside a hash/signature/ISO
workflow.

## Claims and non-claims

Keep these claims separate in every report and verifier:

| Control | Supported claim | Does not establish by itself |
|---|---|---|
| SHA-256 plus structural validation | Current bytes equal the recorded bytes and the container passed named checks. | Author, first creation time, intent, or custody. |
| OpenPGP detached signature | The holder of the selected private key signed the exact verified datum. | The human identity of that holder without a documented trust/attestation process. |
| RFC 3161 response | The timestamped message imprint existed no later than `genTime` under the recorded TSA policy and validated chain. | Task completion time, first-ever existence, truth of signed prose, or requester identity. |
| Canonical chained manifest and signed checkpoint | Entry deletion, insertion, reordering, or alteration is detectable relative to the trusted checkpoint. | WORM storage or an independently published ledger. |
| Deterministic ISO and custody events | Exact staged bytes can be restored, compared, copied, and later burned from one immutable image. | Unbroken custody before the first recorded capture or automatic admissibility. |

RFC 3161 describes time stamping as support for assertions that a datum existed
before a time. RFC 9921 distinguishes evidence about a payload from evidence
about the signature over it. Notrios therefore timestamps a checkpoint signature,
not an unsigned artifact or an unbound prose timestamp.

## Preservation classes and frozen scope

The external root currently has two materially different classes:

1. **Curated handoff artifacts:** top-level release/plan ZIPs and content-free UI
   screenshots. G17a froze 78 such files at `2026-08-25T03:09:46Z`; subsequent
   verified G17a and evidence-decision release ZIPs are explained appends.
2. **Private benchmark workspaces:** three nested directory trees holding private
   inputs, caches, repositories, logs, diagnostics, and generated artifacts from
   archive-scale investigations. They are not release handoffs and are not safe
   for a public/pre-push manifest or recursive ISO copy.

The user selected the curated-top-level scope on 2026-08-25. G17b re-freezes all
curated top-level handoffs then present, explains the known post-G17a appends,
and refuses any other drift. All recursive workspace content is excluded. A
whitelist is mandatory; never run a production ISO builder recursively on the
external evidence root.

The two G17a inventory commitments bind the pre-G17a-ZIP top-level set. The
detailed record used to compute them was private scratch data and is not an
authority for G17b; G17b recomputes every hash and validation from the original
bytes. Modification time is recorded only as weak source metadata.

## Canonical JSON v1

Manifest entries and checkpoints use the following closed canonical JSON
profile. It is deliberately smaller than general JSON so independent verifiers
can agree byte-for-byte:

- UTF-8 without BOM, exactly one LF after each JSON value;
- every string and key is valid Unicode normalized to NFC; no lone surrogate;
- object keys are sorted by the UTF-8 byte sequence of their NFC form;
- no insignificant whitespace;
- only objects, arrays, strings, booleans, null, and signed 64-bit integers;
- no floats, exponent notation, NaN, infinities, or negative zero;
- strings use literal UTF-8 except JSON-required escapes: quotation mark,
  reverse solidus, and U+0000–U+001F; short escapes are used for backspace,
  tab, LF, form feed, and carriage return, otherwise lowercase `\u00xx`;
- duplicate keys, unknown required schemas, and non-canonical encodings are
  refused, not normalized on verification.

The G17a Python prototype is an executable feasibility check, not yet the
production parser. G17b needs independent golden encoders/decoders or cross-
implementation fixtures before this profile becomes a release contract.

## Manifest entry chain

The manifest is append-only JSONL. Every line is a canonical record:

```json
{
  "schema": "notrios.evidence.manifest-entry.v1",
  "sequence": 1,
  "previous_entry_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
  "payload": {},
  "entry_sha256": "<64 lowercase hex>"
}
```

Compute `entry_sha256` over the canonical bytes of the same record with the
`entry_sha256` member omitted. Sequence begins at one; the first previous hash
is 64 zeroes; later entries use the previous entry hash. A verifier checks
canonical encoding, sequence, previous hash, entry hash, schema, and payload
before interpreting the next line.

Historical artifact entries are ordered by the UTF-8 bytes of `logical_name`.
Later entries append in sealing order. Paths are logical POSIX names below a
declared volume root; absolute paths, empty components, `.`, `..`, backslashes,
NUL, control characters, and duplicate normalized names are refused.

### Artifact payload

An artifact entry contains at least:

- `record_type: "artifact"`, stable `artifact_id`, logical name and media type;
- byte size and lowercase SHA-256;
- named structural validation result and validator version;
- `captured_at`, `sealed_at`, and `retroactive`;
- `step_completed_at` plus `completion_source` when an attempt/archive record
  proves it, otherwise explicit null;
- exact commit plus `commit_resolution` and matched anchor names when content
  proves it, otherwise explicit null/candidates;
- source modification time labelled weak metadata;
- preservation class and assigned content checkpoint/volume when known.

The filename is never sufficient commit evidence. G17a's rule requires at least
three embedded source anchors to match one Git tree exactly. A tie is ambiguous;
no full match is unknown. The legacy `notrios.zip` remains unknown.

### Signature payload

Each artifact gets a binary detached OpenPGP signature. A signature entry binds:

- referenced artifact ID and artifact-entry hash;
- signature logical name, byte size, and SHA-256;
- exact primary fingerprint and signing-subkey fingerprint;
- OpenPGP implementation/version, digest and public-key algorithms;
- signature creation time as reported by GnuPG, labelled signer-controlled;
- verification status from a clean keyring containing only the committed public
  key, with machine-readable `VALIDSIG` fingerprint equality.

Do not infer trust from a UID string or a local keyring's trust database. The
public key, exact fingerprint, owner attestation/publication method, revocation
certificate handling, rotation, and compromise procedure are separate records.

## Signed and timestamped content checkpoint

After artifact and signature entries are final, write canonical
`content-checkpoint.json` containing:

- schema, checkpoint ID, entry count, first/last entry hashes;
- SHA-256 and byte size of the complete manifest;
- capture/seal window and whether any covered artifact is retroactive;
- canonical-profile version, verifier version, and intended volume IDs;
- predecessor checkpoint hash or null.

Sign those exact checkpoint bytes with a detached OpenPGP signature. Generate an
RFC 3161 request with SHA-256, a nonce, and certificate inclusion over the
checkpoint signature bytes. Preserve TSQ and TSR. Verify twice:

```text
openssl ts -verify -queryfile content-checkpoint.sig.tsq \
  -in content-checkpoint.sig.tsr \
  -CAfile tsa-root.pem -untrusted tsa-intermediates.pem

openssl ts -verify -data content-checkpoint.json.sig \
  -in content-checkpoint.sig.tsr \
  -CAfile tsa-root.pem -untrusted tsa-intermediates.pem
```

Also extract and record nonce, imprint algorithm/value, policy OID, serial,
`genTime`, accuracy/ordering, TSA subject, responder fingerprint, critical
`id-kp-timeStamping` EKU, certificate-validity result at `genTime`, and available
revocation/status evidence. System default roots are never an implicit fallback.

This one token scales across the batch because the timestamped checkpoint
signature commits the manifest and the manifest commits every artifact and
artifact-signature byte. Per-artifact tokens may be added only as an explicit
standalone-extraction profile; they are not stronger evidence that the complete
batch existed by the checkpoint time.

## Selected OpenPGP identity

No secret key was present during the G17a inventory/prototype capture. After
that investigation, the user created and selected this dedicated evidence
identity on 2026-08-25:

- UID: `Rene Sugar (Evidence Identity) <rene.sugar@gmail.com>`;
- Ed25519 certification primary fingerprint:
  `AEE5F82F2C216D6D15992C8DC96A1C6039BC8098`;
- Ed25519 signing-subkey fingerprint:
  `4ABEB98AF99C8321931BCF282C6A8A4568264005`;
- signing-subkey expiry: `2027-08-25T18:32:16Z`.

A read-only GnuPG colon listing showed `sec#` for the primary, meaning its secret
part is offline/unusable on this workstation, and a usable `ssb` signing subkey.
Every automated signing command must select
`4ABEB98AF99C8321931BCF282C6A8A4568264005!`; GnuPG documents the `!` suffix as
forcing the exact primary or secondary key. Email, short key ID, default-key
selection, or ambient trust must never select a production signer.

The user attests that Secret Service holds separate items with selectors
`service=gpg_evidence,type=subkey_secret` and
`service=gpg_evidence,type=passphrase`, and that the stored passphrase unlocked a
test signature. This review did not retrieve either value. G17b may stream the
passphrase only to GnuPG's standard input in a non-logging process; it must never
put it in arguments, environment, files, output, manifest, ISO, or logs. Do not
export or retrieve the secret-subkey item when the verified local `ssb` is
usable. Before the first production signature, the owner must attest that a
restorable full primary-key backup and revocation certificate exist offline;
the repository records the attestation, not secret locations or bytes.

Export the minimal public key for the ISO and a clean verifier, require exact
primary/subkey `VALIDSIG` fingerprints, and independently publish/attest the
public fingerprint on the first authorized push or release. Rotation and
compromise append records; old signatures are never rewritten.

## Selected RFC 3161 authority order

The user selected DigiCert `http://timestamp.digicert.com` as primary and
Sectigo `http://timestamp.sectigo.com` as fallback. This provider choice does
not trust an arbitrary response from either host. Before production submission,
an explicitly authorized generated pilot must review current terms, download
and hash exact trust material, and verify response policy, nonce, SHA-256
imprint, timestamping EKU, chain, validity at `genTime`, and available revocation
evidence through explicit `-CAfile` and `-untrusted` inputs. A passing DigiCert
pilot pins its observed policy OID and responder chain. If it fails, run one
Sectigo pilot under its published pacing guidance and the same gate.

If neither provider passes or trust material cannot be preserved, record
`timestamp_status` truthfully and stop for a new authority decision. Never
substitute local wall-clock time, an implicit system CA set, or an unsigned
waiver. This decision update contacted no TSA endpoint.

## Deterministic ISO contract

G17a selects xorriso 1.5.6 and an ISO 9660 level-3 image with rationalized Rock
Ridge and Joliet-long views. Volume IDs are ASCII and at most 16 characters;
initial numbering is `NTR-EV-0001`. Staging contains regular files only and is
assembled from a sorted explicit whitelist:

```text
README.txt
PAYLOAD/<original artifacts>
SIGNATURES/<detached artifact signatures>
CHECKPOINT/content-manifest.jsonl
CHECKPOINT/content-checkpoint.json
CHECKPOINT/content-checkpoint.json.sig
CHECKPOINT/content-checkpoint.sig.tsq
CHECKPOINT/content-checkpoint.sig.tsr
TRUST/openpgp-public.asc
TRUST/tsa-*.pem
TOOLS/verify-evidence.py
```

Use fixed UID/GID/read-only modes, fixed mtimes, `TZ=UTC`, a recorded
`SOURCE_DATE_EPOCH`, no startup file, and these logical options:

```text
xorriso -no_rc -as mkisofs -iso-level 3 -r -J -joliet-long \
  -V NTR-EV-0001 -A "NOTRIOS EVIDENCE RESERVE V1" \
  -publisher "NOTRIOS PROJECT" -p "NOTRIOS G17B" ...
```

Record the exact ordered arguments, input manifest, SOURCE_DATE_EPOCH, locale,
OS, xorriso binary SHA-256/version, output block count, and final image hash.
Different xorriso versions are not assumed byte-identical. Build twice from two
clean staging directories and require equal ISO bytes. Extract through xorriso,
verify exact membership and every content hash, and run the offline verifier.

The project budget is 650 MiB (681,574,400 bytes), a conservative planning cap
rather than a universal physical-media assertion. The curated current sources
print at 270,962,688 bytes (132,306 sectors), 39.75% of that budget. G17b gates
the complete staging—including signatures, checkpoint material, trust files,
and verifier—using `-print-size` before writing the ISO, and checks it against
the actual selected blank media. Overflow creates the next numbered volume; it
never silently raises the budget.

An issued ISO is immutable. Later evidence starts the next volume even when an
old one has space. Images and adjacent verification material live under
`/media/renes/SEAGATE2TB/notrios-evidence/`; ISO bytes never enter Git.

## Outer ISO catalog and finite closure

An ISO cannot contain its own final hash. After its bytes are final, append an
outer catalog record in Git containing ISO logical name, volume ID, byte size,
SHA-256, content-checkpoint hash, xorriso contract, reserve location ID, and
creation/custody event. Sign and RFC3161-timestamp an outer catalog checkpoint
by the same approved rules.

The catalog-only closure commit is the explicit outer trust boundary and does
not trigger another release ZIP of itself. Otherwise every catalog commit would
create a new ZIP, ISO, and catalog forever. The next ordinary content checkpoint
covers the preceding closure commit. Verifiers report this boundary; they do
not describe the ISO as containing its own catalog.

## Offline verification order

The G17b verifier must work in a clean temporary directory without network,
private key, original repository, mounted ISO, or ambient key/CA stores:

1. hash the ISO and compare the outer catalog;
2. extract it read-only with recorded xorriso-compatible tooling;
3. reject missing, extra, duplicate, unsafe, symlink, device, and non-regular
   entries;
4. verify canonical JSON and the complete entry chain;
5. verify checkpoint manifest size/hash and first/last/entry count;
6. import only the bundled public key into a fresh GnuPG home and require exact
   `VALIDSIG` fingerprints for checkpoint and every artifact;
7. verify TSQ/TSR nonce and imprint, then verify TSR against exact signature data
   with pinned CA/intermediate inputs;
8. re-hash and structurally validate every original artifact;
9. reconcile every artifact/signature to exactly one manifest entry and volume;
10. report time semantics, unknown provenance, retroactive status, trust-chain
    limitations, and the catalog closure boundary.

Any refusal is terminal for the pre-push gate. CI can validate schemas,
canonicalization, generated fixtures, and tracked support files but must not
pretend to see the external ISO reserve.

## Custody and future physical burning

Every copy/move/verify/burn/read-back event records UTC capture time, actor or
opaque custodian ID, action, exact source/destination/volume IDs, tool and
version, before/after hashes, result, notes, and previous-event hash. A manual
entry is marked manual and signed; it is not silently backdated.

Physical CD-R creation is outside G17b and needs explicit owner authorization.
The future runbook inventories the exact drive and blank media, uses a supported
drive/media speed rather than assuming 4×, writes the reserved ISO bytes in a
closed/disc-at-once session where supported, reads the complete written extent
back, compares the image hash, extracts and checks every file, labels the disc
with volume/hash/checkpoint identifiers, and records storage/custody. Multiple
copies are independently verified. “Read-only” media is not called immutable
against loss, substitution, degradation, or malicious remastering.

## G17b blocking operational approvals

The scope, signer, and provider-order design decisions are resolved. G17b cannot begin
from this planning update alone. An explicit start instruction must
authorize all of the following named operations:

1. use of signing subkey `4ABEB98AF99C8321931BCF282C6A8A4568264005` and
   non-logging Secret Service passphrase retrieval;
2. generated and production RFC 3161 requests to the selected DigiCert/Sectigo
   endpoints under the strict acceptance gate above; and
3. evidence staging and immutable reserve writes below
   `/media/renes/SEAGATE2TB/notrios-evidence/`.

The owner must also attest offline primary-backup and revocation-certificate
readiness before the first production signature. None of these approvals
authorizes a GitHub push or physical burn.
