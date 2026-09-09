# Evidence checkpoint and ISO reserve

This directory contains the public, verifiable side of the G17b preservation
checkpoint. Original ZIPs/screenshots and ISO bytes remain outside Git. The
canonical manifest records exact hashes, structural validation, honest
provenance, retroactive capture, detached-signature coverage, and volume
assignment. `current/` is populated only by the authorized sealing command.

The production signer is exact subkey
`4ABEB98AF99C8321931BCF282C6A8A4568264005` under primary
`AEE5F82F2C216D6D15992C8DC96A1C6039BC8098`. Verification imports only the
bundled minimal public key into a clean temporary keyring. DigiCert policy
`2.16.840.1.114412.7.1` and the pilot-observed responder chain are pinned.
Ambient key trust and CA stores are never accepted during verification.

Commands:

```bash
python3 evidence/verify_evidence.py tracked
python3 evidence/verify_evidence.py reserve \
  --reserve-root /media/renes/SEAGATE2TB/notrios-evidence
```

The reserve verifier checks the outer catalog, ISO hash/signature/timestamp,
extracts the ISO without mounting it, walks every file, verifies the canonical
entry chain/checkpoint, validates all 81 original artifacts, and requires exact
OpenPGP and RFC 3161 identities. It reports the expected catalog-only closure
boundary: the current ISO cannot contain its own final hash or the later commit
that records that hash.

No evidence file claims authorship, contemporaneous historical sealing,
independent creation, WORM storage, unbroken custody, or legal admissibility.
