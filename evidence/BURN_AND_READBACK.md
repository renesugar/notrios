# Future optical burn and read-back runbook

Physical burning is not authorized by G17b. When separately authorized, use the
exact reserved ISO bytes; never regenerate an issued volume ID.

1. Record the ISO SHA-256, volume ID, drive model/firmware, media manufacturer
   information, supported write speeds, operator/custodian, and UTC start time.
2. Choose a speed supported by both drive and medium. Do not assume that 4× is
   universally safer. Use a closed disc-at-once session where supported.
3. Read the complete written extent back to a new image and compare its SHA-256
   with the reserved ISO.
4. Extract the read-back image and run `TOOLS/verify-evidence.py payload` over
   it. A byte, signature, timestamp, or membership mismatch refuses the copy.
5. Label the disc with volume ID, abbreviated image hash, checkpoint ID, and
   copy number. Record a signed custody event from `CUSTODY_TEMPLATE.json`.

Repeat the complete read-back for every copy. Read-only optical media is not
described as immune to loss, substitution, degradation, or remastering.
