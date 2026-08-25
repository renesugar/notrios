# G17a findings and recommendation

## Decisions reached

1. **Use a signed checkpoint as the timestamp datum.** Create one detached
   signature for every artifact, hash both artifact and signature into the
   canonical chained manifest, sign the content checkpoint, and submit the
   checkpoint signature to RFC 3161. A valid token over that signature plus a
   verifying checkpoint proves that all exact artifact/signature bytes named by
   the checkpoint existed by token time. Per-artifact tokens add standalone
   convenience but no stronger batch-time claim; they multiply provider calls,
   rate-limit exposure, and long-term certificate material. They remain an
   optional separately approved profile, not the G17b default.
2. **Use canonical JSONL entries plus canonical JSON checkpoints.** The chain is
   over complete canonical entry cores, not merely ZIP hashes. Unknown values
   are explicit. No absolute path enters a public record.
3. **Use a two-level catalog.** The ISO contains the signed/timestamped content
   checkpoint. A later signed/timestamped outer checkpoint binds the final ISO
   hash. The catalog-only closure commit is an explicit trust boundary; creating
   a new ZIP for it would recurse forever.
4. **Use immutable numbered volumes.** `NTR-EV-0001` is never regenerated after
   issue. New evidence receives a new volume. The exact content staging is a
   whitelist, not a recursive copy of the evidence root.
5. **Use a 650 MiB project budget, not a universal media-capacity claim.** The
   current curated sources print as 270,962,688 ISO bytes, 39.75% of the budget,
   before G17b support material. G17b must gate the actual final staging with the
   same xorriso version/options and the actual selected media capacity.

## Premises rejected

- The evidence root is not only the top-level ZIP/PNG set. Recursive workspaces
  are private, large, and not approved for publication or ISO inclusion.
- A current signature or timestamp cannot be represented as historical.
- Git, a hash chain, or ISO bytes are not WORM storage or automatic legal proof.
- Timestamping the payload alone does not establish that its signature existed.
- An OpenSSL verification without explicit CA/intermediate inputs is not a
  reproducible trust decision.
- “Always burn at 4×” is not a reliable media rule; later burning uses a speed
  supported by the actual drive/media and verifies the complete read-back.

## Remaining blocking decisions for G17b

1. **Scope:** approve curated top-level handoffs plus the G17a ZIP, excluding all
   recursive benchmark workspaces (recommended), or commission a separate
   privacy-reviewed recursive preservation plan.
2. **OpenPGP identity:** no secret key is currently present in the local GnuPG
   keyring. Approve creation of a dedicated offline-primary/evidence-signing
   identity and exact UID/fingerprint (recommended), nominate an external
   existing key, or omit the signer claim.
3. **RFC 3161 authority:** approve a live DigiCert pilot and pin the returned
   policy/responder chain if its terms and verification pass (recommended), use
   another reviewed provider, or omit/waive third-party time evidence. No
   provider endpoint was contacted in G17a.
