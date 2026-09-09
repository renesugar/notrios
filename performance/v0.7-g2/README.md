# v0.7 G2 — envelope, resource, and constrained-device bounds

G2 is an evidence-only investigation. It adds no production sync codec,
schema, cryptography, transport, dependency, or mobile implementation.

The reproducible Go harness compares canonical JSONL and one compact candidate
at 100, 10,000, and 100,000 generated operations. Payload lengths and resource
cases are derived only from committed aggregate profiles for the 1,619,768-body
recipe corpus and the 103,349-body/734-resource Joplin corpus. It also reuses
the archive-v2 loose/pack results. No private path, name, content, content hash,
database, or source resource is committed or read by the harness.

Run:

```bash
GOCACHE=/tmp/notrios-g2-gocache go test ./performance/v0.7-g2/...
GOCACHE=/tmp/notrios-g2-gocache go run ./performance/v0.7-g2/cmd/evidence
python3 performance/v0.7-g2/validate_evidence.py
```

Inputs and outputs:

- `corpus-inputs.json` — aggregate-only G1/P3b/P4 measurements;
- `benchmark-results.json` — environment, tier, resource-policy, pack, hostile,
  and selected-limit evidence;
- `FORMAT.md` — complete investigation format/canonicalization specification;
- `FINDINGS.md` — conclusions and G8/G9 recommendations;
- the emulator and physical-device checklists — explicit limits on the desktop
  proxy claim.

The 100,000 tier is deliberately wider than the proposed receive limit. The
harness then splits it into ten proposed envelopes and records the largest
part, demonstrating that batching rather than a 100,000-record allocation is
the intended scale path.
