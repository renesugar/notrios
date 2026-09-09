# v0.7 G0 — Threat model, terminology, and reference validation

Status: complete on 2026-08-11.

Model: GPT-5 (Codex; exact model variant not exposed).

## Goal and boundaries

Freeze the synchronization security objectives, adversaries, trust boundaries,
terminology, misuse controls, and upstream reference/dependency facts before
any schema or protocol format is implemented.

This was an investigation-only slice. It added no production cryptography,
transport, sync service, schema migration, secret, or dependency. Reference
implementations were not copied or ported, and hashes were not treated as
authentication.

## Working state

The complete evidence is committed under `performance/v0.7-g0/`:

- `THREAT_MODEL.md` inventories security objectives, assets, adversaries,
  trust boundaries, assumptions, trust roots, enrollment, key lifecycle,
  admission order, metadata leakage, recovery, audit, attacker stories, and
  severity calibration;
- `PROTOCOL_GLOSSARY.md` normatively defines profile, database, replica, peer,
  carrier, snapshot, envelope, object, state vector, advertisement, and request,
  plus related security terms;
- `MISUSE_CASES.md` traces thirty hostile/failure scenarios to required behavior
  and their owning G slices;
- `REFERENCE_VALIDATION.md` records primary-source status, license, platform,
  and adopt/reference/reject decisions for the proposed crypto/secret-store
  candidates and supplied synchronization references;
- `validate_evidence.py` mechanically checks the evidence structure and warns
  about secret-shaped assignments.

`SECURITY_REVIEW.md` now records the conclusions while explicitly saying none
is live. `SYNCHRONIZATION.md` carries the key-epoch, signature/AEAD coverage,
and metadata-budget consequences. `PLAN.md`, `ROADMAP.md`, the status, and the
handoff identify G1 as next but unapproved.

## Decisions and consequences

The three user-resolved decisions remain unchanged: authenticated encryption
for every v1 payload (including REST), per-replica Ed25519 signatures on the
five named artifact classes, and no SVN dump compatibility.

G0 makes their consequences explicit:

1. Revoking a compromised replica must advance the encryption epoch for the
   remaining active peers. Signature revocation alone cannot exclude a device
   that retained the old encryption key. Previously disclosed history cannot be
   made secret retroactively.
2. The signature covers domain-separated canonical outer artifact bytes,
   including visible routing and a ciphertext commitment; AEAD binds the same
   header as associated data. G9 owns exact bytes and algorithm selection.
3. Signature, AEAD, hash, enrollment, and replay state are distinct controls.
   Exact hashes do not authenticate a writer or artifact role, and signatures
   do not make an old artifact fresh.
4. Carrier-visible content addresses identify ciphertext/artifact bytes where
   routing needs them. Plaintext body/resource hashes and semantic metadata stay
   encrypted.
5. A correctly keyed compromised active replica can produce valid destructive
   operations. v0.7 therefore relies on attribution, revocation, retained
   revisions/conflicts/tombstones, acknowledgement-gated GC, snapshots, and
   explicit restore intent rather than pretending to infer human intent.
6. Plaintext development mode must be explicit, loopback-only,
   non-interoperable, visibly audited, and unavailable as downgrade/fallback.

## Reference findings

- The Go standard library is the preferred Ed25519 implementation candidate.
  `golang.org/x/crypto` provides plausible XChaCha20-Poly1305 and Argon2id
  candidates under BSD-3-Clause, but G9/G2 still own the suite, nonce, KDF
  parameters, direct pin, test vectors, and constrained-device evidence.
- `zalando/go-keyring` is an MIT desktop candidate only: its documented list is
  macOS, Linux/BSD, and Windows, with Linux depending on Secret Service D-Bus.
  `flutter_secure_storage` is a BSD-3-Clause post-1.0 candidate with material
  per-platform prerequisites and migration/backup behavior to test.
- Marmot (MIT), Cachapa CRDT packages (Apache-2.0), and Go Ygo implementations
  (MIT) are maintained/useful references but do not match Notrios's complete
  offline database replication problem. None was adopted.
- rclone (MIT) remains an optional external conformance carrier. Official
  `copy --immutable` semantics fit append-only testing; `sync`, `bisync`,
  `move`, delete, and purge are excluded because they can delete protocol data.
- Apache Subversion remains a state-vector/change-log/base-delta conceptual
  reference. Its dump grammar is not a Notrios format.
- Minisign (ISC) is rejected as a runtime dependency: detached folder/file
  signatures and an external executable do not provide typed in-process
  per-replica protocol attribution.

## Validation evidence

- `python3 performance/v0.7-g0/validate_evidence.py`
- `go vet ./...`
- `go test ./...`
- `python3 scripts/check_required_files.py`
- `python3 scripts/check_plan_loops.py`
- `bash scripts/validate-scaffold.sh`
- frontend typecheck, tests, and production build
- docs-site build
- `git diff --check`
- release ZIP created with `scripts/package_release.sh` and verified with
  `scripts/check_release_zip.py`

The terminal attempt record in `agent/ATTEMPT_LOG.jsonl` lists the completed
validation set. The final handoff reports the post-commit ZIP path and hash.
