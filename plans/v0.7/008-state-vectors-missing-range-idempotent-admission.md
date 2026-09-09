# v0.7 G5 — State vectors, missing-range planning, and idempotent admission

Status: completed 2026-08-12

Model: GPT-5 (exact variant unavailable)

## Goal and boundaries

Exchange immutable operations repeatedly, out of order, and with gaps while
proving the highest contiguous sequence each local replica has durably admitted
and what it still needs. G5 is restricted to in-memory/local fixtures. It adds
no canonical record conflict/application semantics, transport, cryptographic
enrollment or framing, REST/MCP/UI surface, or background synchronization.

## Recorded defaults

The item had no blocking open decision. Before code, these non-blocking defaults
were recorded in `PLAN.md` and are now implemented:

- protocol 1.0, schema compatibility 19-20, with any widening requiring an
  explicit compatibility-table change;
- a handshake validates an already configured peer and never enrolls it;
- 1,024 state-vector/range entries, 10,000 planned sequences, and a 10,000-
  position sparse-sequence window;
- G2's per-peer pending ceiling of 10,000 operations/64 MiB, plus 16 MiB per
  admission call, 1 MiB per operation, 512 KiB payload, and 64 dependencies.

## What shipped

`internal/syncstate` is a transport/storage-neutral package with a closed
protocol capability set, bounded vector validation/comparison, deterministic
missing contiguous range planning, and strict internal operation normalization.
It rejects a database/protocol/schema/capability mismatch, malformed or
overflowing sequence, ambiguous operation identity, invalid JSON/time/record
identifier, duplicate dependency, and non-canonical pending bytes.

Schema v20 adds `sync_peer_compatibility` and a local sequence-exhaustion
trigger. Store admission requires the peer to have been explicitly configured
through the internal local-fixture seam. It normalizes the complete batch before
locking, refuses unknown record/kind pairs, and then transactionally:

1. treats identical pending/admitted bytes as an inert duplicate and refuses
   changed bytes for the same sequence/operation ID;
2. writes new bytes to the disk-backed pending queue;
3. enforces aggregate pending bounds without arbitrary eviction;
4. drains only each replica's next sequence whose named dependencies already
   exist in the immutable operation set;
5. inserts operation/dependency rows, advances the local contiguous vector,
   rebuilds explicit gap rows, and returns an acknowledgement only after commit.

The `sync_noop/sync.noop` pair is a closed G5 test record. It gives convergence
fixtures an operation with no G6 canonical meaning; no canonical-table trigger
emits it and every other unknown record/kind pair is refused.

## Evidence

`performance/v0.7-g5/README.md` maps every claim to a focused test. The pure
model runs 100 deterministic random seeds over three replicas and 25 operations
per source with shuffle, duplicate, first-wave drop, and eventual delivery. A
real three-SQLite-replica fixture repeats out-of-order/duplicate/delayed
delivery and finishes with identical vectors and operation-ID sets.

Focused durable fixtures additionally cover dependency blocking and cross-
replica drain, exact/conflicting replay, missing plans that omit already pending
positions, explicit peer configuration, every compatibility mismatch, unknown
required records, count/byte/dependency/vector/plan/skew limits, old/future
clock values, acknowledgement monotonicity, v19-v20 migration, process restart,
injected pre-commit rollback, and local sequence exhaustion with canonical-write
rollback.

Repository-wide validation passed with:

- baseline and post-`npm ci` `npm audit --json` (zero vulnerabilities; no fix
  needed), frontend typecheck, 155 tests, production build, and the offline
  browser/CSP/KaTeX check;
- focused and full `go test ./...`, `go vet ./...`, 70 required-file checks,
  plan-loop validation, and scaffold validation;
- the 15-page documentation build, MVP service smoke, performance smoke, and
  `git diff --check`.

Release packaging and explicit ZIP verification are recorded in the completion
report after the coherent commit.

G6 is next and remains unapproved.
