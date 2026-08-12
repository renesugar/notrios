# G5 state-vector and admission evidence

This directory records the reproducible, content-free evidence for v0.7 G5.
G5 adds no network/directory carrier and reads no private corpus.

## Reproduce

```bash
go test ./internal/syncstate -run 'TestCompareVectors|TestVectorAndPlan|TestHandshakeCompatibility|TestNormalizeOperation|TestThreeReplicaModel' -count=1
go test ./internal/store -run 'TestSyncAdmission|TestSchemaV19Upgrade' -count=1
```

The first command exercises a pure model; the second uses real temporary
SQLite databases and synthetic no-content operations/documents.

## Evidence map

| Property | Evidence |
|---|---|
| Eventual three-replica convergence | 100 deterministic random seeds × 3 receivers × 3 sources × 25 sequences; shuffled, duplicated, first-wave drops, then complete delivery |
| A gap is never inferred complete | model deletes sequence 1 while retaining later receipt; SQLite delivers sequence 2 first and persists explicit gap 1 while vector remains 0 |
| Exact replay / equivocation | exact pending and admitted bytes count as duplicates; changed normalized bytes for the same operation identity refuse atomically |
| Dependency queue | a source-B sequence blocked on source-A sequence stays pending/vector 0 and drains both only after A commits |
| Compatibility | database ID, protocol-major/minor intersection, schema 19-20 range, sorted required capabilities, own-vector entry, and explicit configured-peer state are validated |
| Bounds | 1,024 vector entries/ranges, 10,000 planned sequences/sparse skew/pending operations, 16 MiB call, 64 MiB per-peer pending, 1 MiB operation, 512 KiB payload, and 64 dependencies |
| Crash/restart | a sequence-2 gap survives close/reopen; injected failure after drain but before commit restores the original pending/vector state |
| Acknowledgement | cannot exceed the local durable vector, move backward, or name an unknown database replica |
| Exhaustion/skew | old timestamps do not reorder sequences; sparse sequence excess refuses; local sequence exhaustion aborts and rolls back its canonical write |

## Boundary

The normalized JSON stored in `sync_pending_admissions` is an internal G5
representation, not the selected production wire codec. G9 still owns NCB1
promotion/replacement, canonical golden bytes, authenticated encryption,
Ed25519 signatures, and untrusted-input framing. G6 owns canonical metadata,
membership, deletion, and tree application. The explicit fixture-peer method
does not establish a trusted peer and is unreachable from REST or MCP.
