# v0.7 G4 — replication schema and transactionally complete local journal

Status: complete on 2026-08-12.

Model: GPT-5 (exact variant not exposed).

## Goal and boundaries

Record every sync-relevant canonical mutation exactly once under a monotonic
local `(replica_id, sequence)` in the same SQLite transaction, beginning only
at explicit enrollment. Add the schema needed by later state-vector/admission
slices without implementing their behavior.

This slice adds no transport, peer admission, convergence/merge, HLC, delta,
envelope codec, encryption/signature, REST, MCP, or UI surface. It does not
journal pre-enrollment history under `target: none` and does not make derived
FTS, links/blocks, projections, reports, tasks, jobs, or snippets sync records.

## What shipped

Schema v19 adds replicas, snapshot boundaries, one local sequence allocator,
immutable operations and dependencies, state vectors and explicit gaps, peer
acknowledgements, disk-backed pending admissions, audit events, and a transient
capture seam.

Explicit enrollment records a sequence-zero full-snapshot boundary for the
database's current replica. A configured non-none profile target invokes this
idempotent boundary at service startup but does not start a transport. Before
enrollment the capture triggers are inert. Replica rotation or database
identity adoption retires and disconnects the old allocator; the new identity
must establish another snapshot boundary rather than continue the old replica's
sequence.

Canonical-table triggers feed one `sync_journal_capture` seam. Its trigger
increments the allocator, inserts the immutable operation, advances the local
contiguous state vector, and removes the transient row. It executes inside the
canonical caller's existing transaction, so ordinary failures, atomic batch
rollback, import rollback, and process close with an uncommitted transaction
retain neither canonical nor journal rows.

The importer resource upsert was narrowed from replace semantics to a
conflict-update only when anchor metadata changes. This preserves working
behavior while preventing an unchanged membership from looking like a remove
and re-add to the journal.

## Exact classification table

| Record type | Class | Mutations |
|---|---|---|
| collection | field register | create, update, delete |
| document | field register | create, update, trash, restore, purge |
| revision | immutable record | create |
| notebook | field register/tree node | create, update, delete |
| tag | field register | create, update, delete |
| document-tag | membership element | add, remove |
| search notebook | field register | create, update, delete |
| resource | immutable content reference plus metadata registers | create, update, delete |
| document-resource | membership element | add, metadata update, remove |
| document source/provenance | field register | create, update, delete |
| exact source-bundle item | immutable content reference plus metadata registers | create, update, delete |

Physical blob placement remains local; a resource operation names its canonical
blob hash and G8 owns transfer/materialization. Revision bodies remain in their
immutable canonical revision rows; G7/G9 own content objects and encoding.

## 100k evidence

`scripts/run_sync_journal_profile.sh` runs identical generated 100,000-note
imports with journaling disabled and enabled. Each enrolled note produces
exactly three operations: document, immutable revision, and provenance. Import
checkpoints/item state, FTS, parsed links, and projection outbox rows remain
excluded.

| Run | Documents/s | Elapsed | SQLite bytes | Operations |
|---|---:|---:|---:|---:|
| disabled | 316.9 | 315.5 s | 182,845,440 | 0 |
| enabled | 255.8 | 390.9 s | 352,526,336 | 300,000 |

The observed desktop-proxy cost is 23.9% elapsed overhead and 92.8% database-
byte overhead. This is a recorded bound, not a mobile claim or a promise that
later encrypted/wire encoding has the same cost. The exact JSON evidence is
`performance/v0.7-g4/evidence-100k.json`.

## Validation evidence

Focused tests cover schema-v18 upgrade/fresh-v19 parity, explicit and
idempotent enrollment, sequence and operation-ID monotonicity, local-vector
advancement, canonical vocabulary, derived-row exclusion, transient-seam
cleanup, atomic batch and importer rollback, uncommitted-close rollback,
trash/restore/purge and structural lifecycle operations, identity retirement,
re-enrollment, and non-none service startup.

Repository-wide validation passed with:

- baseline and post-`npm ci` `npm audit --json` (zero vulnerabilities; no fix
  was needed), frontend typecheck, 155 tests, production build, and offline-
  asset/CSP check;
- `go test ./...`, `go vet ./...`, scaffold validation, all 66 required-file
  checks, and plan-loop checks;
- the 15-page documentation build, MVP smoke, performance smoke, and
  `git diff --check`;
- the dedicated 100,000-note journal profile, including an exact 300,000-
  operation assertion for the enrolled run.

`scripts/package_release.sh` also rebuilt and verified the handoff ZIP with the
offline `web/dist/` bundle present and runtime/dependency/database artifacts
excluded.

G5 is next and remains unapproved.
