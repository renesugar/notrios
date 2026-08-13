# v0.7 G11 — ephemeral shared-directory protocol and peer discovery

G11 is production code. This directory holds the measurement of the carrier
itself: what one exchange through a shared folder costs, what the folder ends up
holding, how a bounded scan behaves as the folder fills, and what deleting the
whole carrier costs to recover from.

The harness builds **real replicas** on real SQLite files and drives the shipped
round through a real directory — the same `internal/synccarrier` code the CLI
runs. All notes are generated; no private corpus content, path, or title is read
or recorded.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g11-gocache go test ./internal/synccarrier/... ./internal/synckeys/... ./cmd/notriosctl/...

env GOCACHE=/tmp/notrios-g11-gocache \
  go run ./performance/v0.7-g11/cmd/evidence -out performance/v0.7-g11/carrier-results.json

PYTHONDONTWRITEBYTECODE=1 python3 performance/v0.7-g11/validate_evidence.py
```

## Evidence

- `carrier-results.json` — exchange time, carrier size, quiet-round cost,
  isolated carrier-layer cost, scan bounds, and delete-and-recover behavior at
  two library sizes;
- `FINDINGS.md` — what the numbers say, the two defects the work found, and what
  this does not cover.

## Scope

Desktop measurements on one host, one local filesystem, two replicas, no
network. A cloud provider, a removable device, delayed listings, and provider
filename behavior are **G12's** evidence, deliberately not claimed here. Timings
are one run each and are dominated by SQLite admission, which is why the carrier
layer is also measured on its own rather than inferred from a difference between
two whole-system runs.
