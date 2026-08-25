# G17a evidence-preservation contract investigation

G17a is investigation code and aggregate evidence, not the G17b production
sealer. It read the external evidence root without modifying it, used ephemeral
fixture keys and a local fixture TSA only under `/tmp`, and created no production
signature, timestamp, manifest, ISO, reserve file, Git push, or optical disc.

## Frozen curated inventory

The exact top-level capture at `2026-08-25T03:09:46Z` contains 78 regular files:
74 ZIPs and four PNGs, totaling 270,506,844 bytes. All ZIP members were fully
decompressed for CRC validation; duplicate, encrypted, symlink, absolute, and
parent-traversal entries were rejected. All PNG chunk CRCs were read. Every file
passed. Seventy-three current-shape release ZIPs each match one distinct Git
commit by all six embedded source anchors. The older `notrios.zip` contains none
of those anchors, so its task/commit association remains unknown rather than
being inferred from its filename.

The committed inventory publishes only aggregate facts and two commitments. The
private detailed capture used to derive them remained under `/tmp` and is not a
G17b input:

- the **content commitment** binds NFC logical name, kind, byte size, and SHA-256
  for every top-level artifact;
- the **capture commitment** additionally binds filesystem modification time,
  structural-validation result, and evidence-derived commit candidates.

G17b must recompute both from original bytes. The expected append after this
capture is the G17a release ZIP; any other drift is a refusal.

## Scope finding

The root also has three top-level directories containing G14 benchmark
workspaces. They include private inputs, caches, repositories, logs, diagnostics,
and large generated artifacts by design. A recursive ISO probe had already seen
at least 47,400 nodes when stopped, and a separate aggregate traversal did not
finish promptly. Exact recursive inclusion would contradict the repository's
private-data rules and the planning claim that this is a small one-disc set.

G17b therefore needs a third blocking user decision. The recommendation is to
seal the curated top-level handoff artifacts plus the expected G17a ZIP and to
exclude all recursive workspace content. A separately privacy-reviewed workspace
preservation project may later select aggregate outputs; it must not be smuggled
into the public/pre-push evidence manifest.

## Generated prototype

Run the deterministic unit fixtures:

```bash
python3 -m unittest discover -s performance/v0.7-g17a -p 'test_*.py' -v
```

Run the end-to-end generated prototype in a new directory outside the repository:

```bash
python3 performance/v0.7-g17a/prototype.py self-test \
  --workdir /tmp/notrios-g17a-self-test \
  --result-output /tmp/notrios-g17a-prototype-results.json
```

`gpg-agent` may require the normal host execution environment. The command
creates a one-day fixture-only OpenPGP key, local fixture root/TSA, manifest,
signature, TSQ/TSR, two staging trees, two ISOs, and two extracted trees entirely
under the supplied new directory. It verifies good and tampered signatures,
RFC 3161 request/response nonce/imprint/policy/chain behavior including wrong CA,
manifest mutation, clean byte-identical ISO rebuild, print-size equality,
Unicode/long names, rationalized Rock Ridge permissions, and full extraction
hashes. Nothing from that workspace is a production trust anchor.

Generate a fresh private detailed inventory and aggregate summary with:

```bash
python3 performance/v0.7-g17a/prototype.py inventory \
  --source /home/renes/evidence/notrios \
  --repo . \
  --captured-at <RFC3339-time> \
  --private-output /tmp/notrios-g17a-private-inventory.json \
  --summary-output /tmp/notrios-g17a-inventory-summary.json
```

Do not commit the private output. `EVIDENCE_PRESERVATION.md` contains the selected
canonical format, verification sequence, ISO contract, and open decisions.
