# v0.7 G6 — Deterministic metadata, membership, deletion, and tree convergence

Status: completed 2026-08-12  
Model: GPT-5 (exact variant not exposed)  
Product version: 0.6.0  
Schema: v21

## Goal and boundaries

Make G6-owned non-body records converge after G5 admission. This slice owns
durable HLC order, sparse scalar registers, document-tag membership,
trash/restore/death-certificate state, notebook tree/name repair, and atomic
canonical application. It does not merge note bodies, apply revision graphs,
move resource bytes, authenticate/sign peers, collect retained payloads, add a
carrier/background worker, or expose synchronization through REST, MCP, or UI.

## Defaults recorded before implementation

- Mutable update payloads contain only changed fields; create payloads contain
  the complete initial scalar state.
- A case-insensitive uniqueness collision preserves the protocol-order winner's
  requested name and gives each loser a stable short-ID suffix plus a repair.
- Death certificates require non-empty structural signer/signature fields.
  Cryptographic verification remains G9 work, so enrolled ordinary local purge
  is refused and only the internal trusted fixture seam can exercise a
  structurally signed certificate.

## Implemented working state

- Schema v21 adds `hlc_wall_ms`/`hlc_logical`, `sync_hlc_clock`, explicit
  enrollment baselines/floors, field/lifecycle/membership projections,
  persistent death certificates, deterministic repair rows, and an apply guard.
- Local allocation never moves its HLC backward or overflows the bounded
  logical component. Admission also refuses a source whose consecutive HLC
  regresses, rolling the operation/vector transaction back.
- `internal/syncmerge` sorts by wall, logical, replica ID, then sequence. Scalar
  fields, lifecycle, and membership elements compare independently. Duplicate
  operations are inert.
- The notebook repair pass considers requested parent edges from highest
  protocol order downward. A cycle-forming edge loses to accepted higher-order
  edges; missing homes use the protected Recovered notebook or root. Name and
  document-home repairs have deterministic content-derived IDs.
- Store admission rebuilds the post-boundary projection and updates canonical
  collections, documents, notebooks, tags, search notebooks, provenance, and
  note-tag rows before the same transaction advances/returns its vector. The
  apply guard prevents remote changes from producing local echo operations.
- Trash and restore remain LWW lifecycle values rather than absence. A death
  certificate permanently blocks same-ID restoration, while physical payload
  deletion waits for G17 acknowledgement/retention policy.

## Validation evidence

- Exhaustive 8-operation small state: 40,320 delivery orders produce identical
  projection bytes and repair rows.
- 250 seeded shuffled/duplicate schedules reproduce the same result, including
  independent title/notebook fields, membership add/remove, trash/restore,
  notebook cycles, missing homes, and clock skew.
- Two real SQLite receivers admit the same two peer streams in opposite order
  and finish with identical selected canonical rows and repair reports.
  `pragma_foreign_key_check` and ordinary Store reads pass after every admitted
  peer transaction.
- Focused tests cover sparse local payloads, monotonic HLC allocation, backward
  remote-HLC rollback, structural/unsigned purge certificates, same-ID restore
  refusal, enrolled local-purge gating, schema-v18/v20 restart upgrade, replay,
  and interrupted transactions.
- Regular validation began with `npm audit`, used no forced dependency update,
  then reinstalled and re-audited the bundled frontend. Full Go/vet, frontend,
  offline-assets, scaffold, docs, smoke, performance-smoke, diff, package, and
  ZIP verification gates passed; the verified commands are also recorded in
  `agent/ATTEMPT_LOG.jsonl`.

## Handoff

G7 is next and remains unapproved. It owns immutable note revision objects,
bounded named-parent transfer deltas, exact hash verification, three-way merge,
and durable typed conflicts.
