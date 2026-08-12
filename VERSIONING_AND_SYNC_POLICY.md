# Versioning and Synchronization Policy

The service must not depend on Git, Fossil, Recoll, or any external projection/index as the authoritative note store.

## Canonical revision model

SQLite stores durable saved revisions. Restoring an old revision creates a new revision rather than destructively rewriting history.

Use three layers of history:

| Layer | Owner | Purpose |
|---|---|---|
| Editor undo stack | UI | Keystroke-level undo while editing. |
| SQLite revisions | Notrios service | Durable saved-note history, restore, conflict handling. |
| Git/Fossil checkpoints | optional adapters | Projection backup, external diff/history, sync experiments. |

Restoring an old revision is a normal new write and therefore produces a new
replication operation once synchronization exists. Character-level editor
history is not the database sync protocol.

## Database synchronization

The v0.7 design is specified in `SYNCHRONIZATION.md`. Its invariants are:

- SQLite plus the asset store remain canonical on each replica.
- Stable logical database IDs are distinct from per-copy replica/device IDs.
- Operations have per-replica sequence IDs; HLCs order conflicts but do not
  replace delivery/acknowledgement vectors.
- Note bodies remain immutable saved revisions. A revision always binds its
  complete result hash/object and may carry a transfer delta from a named
  parent. G1 selected optional beneficial line-token deltas and exact
  base/result verification. G1a found a Notrios-owned pure-Go Subversion-style
  matcher and constrained RFC 3284 VCDIFF default-table profile feasible for
  optional binary-safe transfer deltas from a real named parent. It explicitly
  does not select Subversion svndiff or a private container; production use is
  gated on G2 bounds and a later G7/G8 approval. Conflict resolution separately
  uses a Notrios-owned bounded line-first three-way merge with Unicode-aware
  word-token refinement only for bounded conflict regions. Same-token,
  delete/edit, malformed, or over-limit overlap remains a typed conflict.
- Resource metadata may converge before bytes; content hashes and permanent
  URIs support bounded lazy fetch, resume, verification, and deduplication.
- G2 recommends compact canonical NCB1 operation records inside a readable
  canonical-JSON outer envelope, deterministic gzip, simultaneous
  10,000-operation/16 MiB canonical/4 MiB compressed envelope ceilings, and
  1 MiB fixed resource chunks above a 1 MiB whole-object threshold. These are
  G9/G8 implementation inputs, not current protocol code; mobile limits remain
  provisional.
- Tombstone/resource collection waits for retention plus acknowledgements from
  every active peer.
- REST and an ephemeral shared directory are transport adapters over one
  protocol. Removable/cloud-mapped directories and rclone may carry the latter
  during tests, but rclone is not a dependency and `rclone sync` is never the
  conflict/deletion algorithm.
- A blank, reset, or history-expired peer catches up through a verified,
  encrypted archive-v2 snapshot bound to a state vector, then applies later
  operations. The directory is disposable and peers can reconstruct it.
- `target: none` is a first-class configuration and performs no peer transfer.
- MCP may plan/start ordinary incremental sync, request bounded resource fetch,
  and inspect status/conflicts at an explicit sync scope. Enrollment, keys,
  backup export/restore, retirement, purge, and bulk bytes remain outside MCP;
  REST/object files carry bulk artifacts.

## go-git

go-git can simplify optional projection checkpointing, reading commit history,
and importing Git-managed Obsidian vaults. It is not required for the core
REST/MCP service and does not replace SQLite revisions or the Notrios
replication protocol.

Limitations to account for before enabling it as a sync backend:

- Git LFS is not a default path.
- Advanced merging/rebase/conflict workflows need special handling or Git CLI fallback.
- Git commits cannot be part of the same atomic transaction as SQLite writes.

## Fossil

Fossil can be useful for a self-contained versioned projection/export, but it
should not replace the application database or sync protocol. Fossil's
checkout-operation undo is not a general note undo system.

## External vault sync

Bidirectional sync with existing Obsidian folders must be explicitly configured. Each collection needs an ownership policy:

- managed by companion service;
- external read-only;
- external editable with sync;
- import snapshot only.

Conflicts must be visible to users; do not silently overwrite external edits.
External-vault sync is a format adapter with an ownership policy, not a second
canonical database or an implicit participant in native sync.
