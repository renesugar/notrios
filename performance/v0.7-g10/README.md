# v0.7 G10 — snapshot catch-up and reset state machine

G10 is production code. This directory holds the measurement of what catch-up is
for: a blank, far-behind, repaired, or deliberately reset replica reaching a
known state without replaying a library's whole operation history.

The harness builds **real replicas** on real SQLite files and walks the shipped
state machine — permission, request, offer, transfer, verify, restore, intent,
cutover — rather than a shortcut. All notes are generated; no private corpus
content, path, or title is read or recorded.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g10-gocache go test ./internal/synccatchup/... ./internal/store/...

env GOCACHE=/tmp/notrios-g10-gocache \
  go run ./performance/v0.7-g10/cmd/evidence -out performance/v0.7-g10/catchup-results.json

PYTHONDONTWRITEBYTECODE=1 python3 performance/v0.7-g10/validate_evidence.py
```

## Evidence

- `catchup-results.json` — operations avoided and time saved at two library
  sizes, with the cutover walked through the real state machine;
- `FINDINGS.md` — what the numbers say, the defect this measurement found, and
  what it does not cover.

## Scope

Desktop measurements on one host, with no carrier and no network. The snapshot
*restore* itself is archive-v2's, measured under P3 and P4 at 382,206 notes and
not re-measured here; what this measures is the incremental work a recorded
state vector removes afterwards.
