# G17 retention and repair evidence

G17 keeps the resolved 90-day default. The generated full-corpus-scale run
retained 382,206 representative metadata operations—one per document in the
published G14b/G14e 382,206-document workload—in the production schema and
indexes. It added 169,906,176 bytes, or 444.54 bytes per operation. A transparent
5,000-operation/day scenario projects 200,043,377 bytes for 90 days. This does
not invalidate the default beside the independently required retained physical
snapshot, whose full-scale cost G14b/G14e already measured.

The projection is not an observed edit rate. Operation payloads vary, especially
revision bodies, so operators should use the dry-run `eligible_bytes` result for
their own library. The conclusion is narrower: a complete full-corpus churn of
representative metadata in one horizon costs about 162 MiB, while collection
remains bounded by age, the currently re-verified snapshot, and every active
peer acknowledgement.

The harness reads no private corpus data or path. It uses only G14b/G14e's
published aggregate document count and generated identifiers/payloads. Run it
against an empty private workspace:

```bash
go run ./performance/v0.7-g17/cmd/evidence \
  -workspace /tmp/notrios-g17-retention-cost \
  -operations 382206 -daily-operations 5000
python3 performance/v0.7-g17/validate_evidence.py
```

The G17 behavioral matrix lives in `internal/store/sync_retention_test.go` and
covers all-active-peer watermarks, a phone beyond its horizon, credential
revocation versus retirement, signed retirement propagation, stale credential
re-enrollment refusal, snapshot coverage, resource reachability, tombstone
payload collection, permanent death identity, below-floor catch-up, and exact
dry-run digest application.
