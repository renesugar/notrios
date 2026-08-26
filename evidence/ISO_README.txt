NOTRIOS EVIDENCE RESERVE V1

This image preserves exact historical release handoffs and their detached
OpenPGP signatures under one signed and RFC 3161 timestamped content checkpoint.
All historical captures are retroactive. The timestamp proves only that the
checkpoint signature existed by the token genTime under the bundled DigiCert
policy and explicitly pinned chain.

Offline verification requires Python 3, GnuPG, OpenSSL, and xorriso. Extract the
image, then run:

  python3 TOOLS/verify-evidence.py payload .

The ISO cannot contain its own final hash. Compare the image against the
checked-in outer ISO catalog and its separately signed/timestamped checkpoint.
No hash, signature, timestamp, Git commit, or ISO alone establishes authorship,
independent creation, unbroken custody, legal admissibility, or WORM storage.
