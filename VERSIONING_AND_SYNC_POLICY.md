# Versioning and Synchronization Policy

The companion service must not depend on Git, Fossil, sist2, or any external projection as the authoritative note store.

## Canonical revision model

SQLite stores durable saved revisions. Restoring an old revision creates a new revision rather than destructively rewriting history.

Use three layers of history:

| Layer | Owner | Purpose |
|---|---|---|
| Editor undo stack | UI | Keystroke-level undo while editing. |
| SQLite revisions | companion service | Durable saved-note history, restore, conflict handling. |
| Git/Fossil checkpoints | optional adapters | Projection backup, external diff/history, sync experiments. |

## go-git

go-git can simplify optional projection checkpointing, reading commit history, importing Git-managed Obsidian vaults, and simple fast-forward push/pull workflows. It is not required for the core REST/MCP service and does not replace SQLite revisions.

Limitations to account for before enabling it as a sync backend:

- Git LFS is not a default path.
- Advanced merging/rebase/conflict workflows need special handling or Git CLI fallback.
- Git commits cannot be part of the same atomic transaction as SQLite writes.

## Fossil

Fossil can be useful for a self-contained versioned projection/export, but it should not replace the application database. Fossil's checkout-operation undo is not a general note undo system. Durable note undo should remain in SQLite revisions.

## External vault sync

Bidirectional sync with existing Obsidian folders must be explicitly configured. Each collection needs an ownership policy:

- managed by companion service;
- external read-only;
- external editable with sync;
- import snapshot only.

Conflicts must be visible to users; do not silently overwrite external edits.
