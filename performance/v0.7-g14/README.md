# v0.7 G14 — REST sync data plane and resumable encrypted backup download

G14 is production code. This directory measures two things: what an exchange
over an authenticated peer costs against the same exchange through a folder, and
what a resumable encrypted snapshot costs to produce, move, and verify.

The harness drives the shipped code — the same round, the same `Carrier`
interface, the same backup path `notriosctl sync fetch-backup` uses. Everything
is generated; no private corpus content, path, or title is read or recorded.

## Reproduce

```bash
env GOCACHE=/tmp/notrios-g14-gocache go test ./internal/syncrest/... ./internal/syncbackup/... \
  ./internal/httpapi/... ./internal/syncauth/...

env GOCACHE=/tmp/notrios-g14-gocache \
  go run ./performance/v0.7-g14/cmd/evidence -out performance/v0.7-g14/rest-data-results.json

PYTHONDONTWRITEBYTECODE=1 python3 performance/v0.7-g14/validate_evidence.py
```

## Evidence

- `rest-data-results.json` — exchange time over both carriers with the
  transcript comparison, and backup production, transfer, and verification at
  two library sizes;
- `FINDINGS.md` — what the numbers say, the defect the first data-plane run
  found, and what this does not cover.

## Scope

Loopback HTTP on one host, no network, one run each. These are protocol costs
rather than a network measurement: a real link's latency and bandwidth are not
modelled, and G12 already showed that a provider's own cadence dominates
anything the protocol does. Two library sizes, because this measures a transport
and not a library.
