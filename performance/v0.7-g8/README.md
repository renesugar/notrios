# v0.7 G8 — resource metadata, lazy materialization, chunks, and integrity

G8 is production code. This directory holds the evidence that a replica can use
a synchronized note before its attachment has arrived, that the default policy
fetches only what is small enough to be worth fetching unasked, and that a
chunked transfer resumes, verifies, and reconstructs exactly.

The harness builds **real replicas** — the production `internal/store` on real
SQLite files and real asset trees — rather than a model. Every attachment is
generated; no private corpus content, filename, or path is read or recorded,
and the temporary replicas are deleted when the run ends.

## What is measured

Four object sizes, chosen around the thresholds G2 selected: 64 KiB (an
attachment small enough to view inline), exactly 1 MiB (the whole-versus-chunked
boundary), and 4 MiB and 16 MiB (chunked). Eight attachments at each size are
created on one replica and their operations shipped to another.

Per size:

- `metadata_only_ms` — admitting the operations with no attachment byte moved;
- `note_readable_before_bytes` — whether the note is usable at that point;
- `auto_requested` / `auto_reason` — what the default policy decided on its own;
- `fetched_bytes` and `materialize_ms` — the pinned transfer;
- `exact_reconstruction` — every materialized object compared byte for byte;
- `resume_chunks_refetched` / `resume_chunks_total` — a transfer interrupted
  after one chunk, then resumed.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g8-gocache \
  go test ./internal/syncassets/... ./internal/store/...

env GOCACHE=/tmp/notrios-g8-gocache \
  go run ./performance/v0.7-g8/cmd/evidence \
  -out performance/v0.7-g8/materialization-results.json

PYTHONDONTWRITEBYTECODE=1 \
  python3 performance/v0.7-g8/validate_evidence.py
```

## Evidence

- `materialization-results.json` — the four sizes with their timings, policy
  decisions, transfer totals, resume counts, and exactness;
- `FINDINGS.md` — what the numbers say and the three limits they do not remove.

## Scope

Desktop measurements on one host, over the in-process fixture provider. G11's
shared directory and G14's REST data plane own the real carriers, and G2's
Android emulator and physical-device checklists still gate any mobile claim.
Nothing here makes one.
