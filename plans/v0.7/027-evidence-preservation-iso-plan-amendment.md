# v0.7 planning amendment — sealed evidence and optical ISO reserve — complete

Status: complete on 2026-08-24.

Model: GPT-5 (Codex; exact model variant not exposed).

## Goal and boundary

Insert an evidence-preservation sequence before G18 and before any GitHub push.
This is a documentation/planning slice. It does not hash, sign, timestamp,
rewrite, move, or package the external evidence; create an ISO; use or create a
secret key; contact a timestamp authority; push Git; or burn optical media. It
does not approve G17a or G17b.

## Why the existing process is insufficient

Every completed plan item produced a release ZIP and copied it to
`/home/renes/evidence/notrios`, but the repository had no canonical inventory,
detached artifact signatures, independently verified timestamp responses,
volume catalog, offline verifier, or pre-push coverage gate. Filenames and
filesystem modification times are useful leads, not cryptographic completion
records. Git history corroborates the project sequence, but the local and
hosted repositories are not assumed to be immutable third-party ledgers.

The supplied proposal correctly identified the value of hashing, signatures,
independent time evidence, manifests, and offline copies. Several legal and
technical claims needed narrowing before implementation:

- A hash records equality to known bytes; it does not identify an author or
  establish when those bytes first existed.
- An OpenPGP detached signature establishes that a selected key signed exact
  bytes. Attribution depends on the key's identity/trust process, and signing a
  historical file now does not make the signature historical.
- RFC 3161 provides evidence that a datum existed no later than the token time
  under a TSA policy. It does not establish a software task's completion time.
  Timestamping only the artifact would not prove that its detached signature
  already existed. The plan therefore recommends timestamping the detached
  signature, which binds the original artifact.
- OpenSSL timestamp verification needs explicit trust anchors and, where
  required, untrusted intermediates. A verifier that accepts whichever system
  roots happen to exist is not the reproducible offline check required here.
- A hash chain detects changes relative to a trusted checkpoint; neither a Git
  commit nor a chain alone makes storage immutable or WORM.
- ISO 9660 is a portable preservation container and a useful exact burn master.
  It is not automatic proof of authorship, independent creation, clean-room
  process, custody, or legal admissibility. Those are case- and jurisdiction-
  dependent conclusions outside this engineering plan.
- A fixed 4× burn is not universally safer. A later burn procedure must use a
  speed supported by the specific media and drive and verify every written copy.

## Read-only local facts

The 2026-08-24 inventory found 77 regular files in the external evidence root:
73 ZIPs and four PNGs, approximately 266 MB in total. The largest file is the
legacy `notrios.zip`, approximately 58.5 MB. No `.sig`, `.asc`, `.tsq`, or
`.tsr` sidecar was present. The scope must therefore include every frozen
regular file, not only ZIPs; excluding the four UI screenshots or legacy bundle
would create an unexplained gap.

The host has GnuPG 2.4.4, OpenSSL 3.0.13, and xorriso 1.5.6 available. The
designated `/media/renes/SEAGATE2TB` reserve has ample free space. Tool presence
is not provider selection, key authorization, capacity proof, or a completed
evidence operation.

At observation time, local `develop` had no configured upstream. It was 104
commits ahead and five commits behind `origin/main`; `origin/develop` also
existed. These are volatile read-only observations, not a push instruction or a
statement that either remote reference is current. The new host gate must check
the actual final HEAD and catalog commits immediately before any separately
authorized push.

## Plan change

`PLAN.md` and `ROADMAP.md` now insert two approval-gated slices after completed
G17 and before G18:

1. **G17a — evidence provenance, sealing, and optical reserve investigation.**
   Freeze the all-file inventory; define exact hash/signature/time/custody claim
   vocabulary; map artifacts to tasks/commits without guessing; select a
   canonical chained JSONL manifest, signed checkpoints, and a two-level ISO
   catalog; evaluate an RFC 3161 provider and explicit trust chain; and prove a
   deterministic CD-sized ISO/offline-verifier contract using generated files
   only. It owns two blocking user decisions: the exact OpenPGP signing identity
   and the RFC 3161 authority/policy.
2. **G17b — historical backfill, manifest, ISO reserve, and pre-push gate.**
   After those decisions and explicit operational approvals, hash and validate
   every original, create detached signatures and timestamps over the
   signatures, check the canonical manifest/checkpoint and outer ISO catalog
   into Git, place immutable numbered ISO images and adjacent verification sets
   under `/media/renes/SEAGATE2TB/notrios-evidence/`, verify clean offline
   restore, and fail closed before a GitHub push if any artifact is uncovered.

The content manifest included inside an ISO cannot contain that ISO's own final
hash. G17b therefore uses two levels: a signed content checkpoint inside the
image and a checked-in outer catalog created after the final ISO bytes exist.
An issued volume is immutable; later artifacts receive a new volume number even
if old media has spare capacity. The final catalog-only closure commit is an
explicit outer trust boundary and does not recursively require a new release ZIP
of itself; the next content checkpoint covers the preceding closure. Without
this exception, every catalog commit would create a new ZIP, ISO hash, and
catalog commit forever.

Backfilled items carry `retroactive: true`, the current `captured_at` and
`sealed_at`, and only a separately supported `step_completed_at`. A present-day
TSA response never backdates the seal. Future per-item ZIPs become
contemporaneously sealable only after G17b changes the execution rule. ISO
checkpoints are required before future pushes and at milestone close; physical
burning remains a later owner-authorized operation from the exact reserved ISO
bytes.

## Primary references checked

- RFC 3161, *Internet X.509 Public Key Infrastructure Time-Stamp Protocol
  (TSP)*: <https://www.rfc-editor.org/rfc/rfc3161>
- RFC 9921, *Evidence about the Signing Time*: distinguishes timestamping a
  payload from proving a signature existed by a time:
  <https://www.rfc-editor.org/rfc/rfc9921>
- OpenSSL `ts` documentation: `-data`/`-queryfile`, explicit `-CAfile`, and
  `-untrusted` verification inputs:
  <https://docs.openssl.org/3.0/man1/openssl-ts/>
- GnuPG manual, detached signature creation and verification:
  <https://www.gnupg.org/documentation/manuals/gnupg/GPG-Input-and-Output.html>
- GNU xorriso manual, ISO 9660 creation, media operations, and verification:
  <https://www.gnu.org/software/xorriso/man_1_xorriso.html>
- NIST IR 8387, *Digital Evidence Preservation: Considerations for Evidence
  Handlers*: preservation, hashing/signatures, and chain-of-custody concerns:
  <https://doi.org/10.6028/NIST.IR.8387>
- U.S. Copyright Office, *Copyright Basics*: independent creation is a legal
  concept distinct from a technical hash/signature record:
  <https://www.copyright.gov/circs/circ01.pdf>

No universal “clean-room evidence bundle” standard or automatic admissibility
rule was found in these sources. The plan uses narrower preservation and
verification language and is not legal advice.

## Documents reconciled

- `PLAN.md`, `ROADMAP.md`, `README.md`, `CODING_CLIENT_HANDOFF.md`;
- `agent/PLAN_STATUS.md`, `CONTEXT_MAP.md`, `FEATURE_MATRIX.md`;
- `SYNCHRONIZATION.md`, `TESTING_POLICY.md`, `ENVIRONMENT_SETUP.md`.

## Validation

Passed on 2026-08-24:

- frontend audit before and after `npm ci`: zero vulnerabilities (the sandboxed
  post-install audit lost DNS; the approved network retry passed);
- `npm run typecheck`, 16 Vitest files / 163 tests, and `npm run build`;
- `go test ./...` and `go vet ./...`;
- `bash scripts/validate-scaffold.sh` (70 required files);
- `bash scripts/build_docs_site.sh` (15 pages, 3,064 words);
- `python3 scripts/check_required_files.py` and
  `python3 scripts/check_plan_loops.py`;
- complete JSONL parsing, cross-document next-item/status search, and
  `git diff --check`.

Release packaging is produced after the coherent planning commit and verified
with the repository checker before handoff.

## Outcome

**Outcome (2026-08-24).** Evidence preservation is now a finite, approval-gated
engineering sequence rather than an implied property of filenames and Git
history. G17a is next and unapproved; it blocks G17b, G18, and any GitHub push.
The exact signing fingerprint and RFC 3161 authority/policy are visible blocking
decisions. No artifact, external directory, ISO, key, TSA, remote repository, or
physical medium was changed by this amendment.
