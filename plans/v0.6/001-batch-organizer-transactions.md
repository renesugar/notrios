# v0.6 F1 — Batch organizer transactions

Status: complete on 2026-08-07.

Model: Claude Opus 5 (Claude Code).

## What it does

`POST /api/v1/batch` and `store.RunBatch` apply one bounded organizer
transaction over an explicit list of notes: `move`, `add_tags`, `remove_tags`,
`trash`, `restore`, `duplicate`. Modes are `best_effort` (default) and `atomic`.
Schema v17 adds `batch_operations`, the idempotency ledger.

## The two decisions the plan left open

They were listed as needing an answer before F1 could be designed. The task was
approved without one, so both are taken here and stated plainly rather than
buried.

**Idempotency keys are scoped to the database and persisted.** The alternative —
an in-memory map scoped to the process — fails at exactly the moment the feature
exists for. A batch is retried when something went wrong: a dropped connection,
a client crash, a restart. A ledger that forgets on restart is a ledger that
forgets precisely when it is needed. That cost a schema version, which is the
honest price of the guarantee.

The stored value is the **first run's response, verbatim**, not a recomputation.
Re-deriving outcomes on replay would describe a library that has since moved on
— a different answer to the same question. A `request_sha256` of the arguments
is stored alongside, so a key reused for *different* work is refused rather than
answered with an unrelated result; that is a client bug and hiding it would be
worse than failing.

**A duplicate inherits content, not identity.** It copies the body, the
notebook, the tags, and the resource references — those are content-addressed,
so sharing a blob is correct rather than wasteful — and its title gains
` (copy)`.

It does **not** copy the `document_sources` row. That row asserts "this note *is*
that imported note"; two notes claiming it would make re-import ambiguous and
would surface in lint as `duplicate_source_id`. Nor does it copy revision
history: a duplicate starts fresh rather than claiming edits that never happened
to it.

## Decisions worth recording

**The modes differ in what a failure does, never in what the report says.** Both
report every requested item. A caller told only "failed" cannot tell which half
happened, which is the single most useful thing to know after a partial run.

**Four outcomes, not two.** `applied`, `skipped`, `failed`, `rolled_back`.
`skipped` is a no-op that was not a fault — a note already in the destination, a
tag it already carried — and conflating it with `failed` makes "nothing to do"
look broken while overstating how much a run changed. `rolled_back` is an item
that succeeded and was undone by a later failure in an atomic run; calling it
`failed` would blame it for someone else's problem.

**An atomic run's report is never shorter than its request.** Items after the
failure were never attempted, and they say so with a reason. Silence would leave
the caller counting.

**A request that ran is a `200` even when every item failed.** Per-item failure
is the report's content; `4xx` is reserved for the call itself being wrong. This
is the same rule E7 applied to malformed query blocks.

**Preconditions are per item.** `trash` writes a revision and so needs a base
revision — but per item, because a batch is a set of independent notes and one
of them having moved on is not a reason to refuse the rest in best-effort mode.
The requirement is validated up front for the whole request, so a caller that
forgot it entirely learns before anything runs.

**A repeated document ID is refused.** It would be applied twice, reported
twice, and for `duplicate` would silently make two copies.

**The lock is held for the whole run.** Interleaving another writer between
items would make "atomic" untrue.

**Nothing here reaches a note the single-note surfaces cannot.** Help notes
still refuse to be moved or duplicated, and a test asserts a batch is not a way
around protection.

## Refactoring it required

Atomic mode needs many operations inside one transaction, and the exported
store methods each take the mutex and open their own. Locked cores were
extracted — `moveDocumentToNotebookLocked`, `addDocumentTagLocked`,
`removeDocumentTagLocked`, `deleteDocumentLocked`, `restoreDocumentLocked` — and
the exported methods now wrap them. The tag helpers gained a "did anything
change" return, which is what makes an honest `skipped` possible.

## Two defects caught before they shipped

**The migration ran in the wrong order.** `ensureSchemaV17` was inserted before
`ensureSchemaV16`, so v16's `PRAGMA user_version = 16` ran last and the schema
reported 16 after bootstrapping. Caught by an existing status test.

**`duplicate` used a column that does not exist.** The copy of resource
references named `anchor`; the column is `anchor_json`. Caught by reading the
schema rather than by the compiler, which cannot see inside a SQL string.

**The rollback test was verified to fail on a broken rollback.** Replacing the
atomic `ROLLBACK` with a `COMMIT` makes
`TestBatchAtomicRollsBackAndSaysWhichItemsWereUndone` fail on the assertion that
the library is untouched. A test that cannot fail proves nothing.

## What is deliberately not here

**Stable Markdown-link copy**, listed with batch operations in `ROADMAP.md`. It
produces text for a clipboard rather than changing the library, so it is not a
transaction and does not belong in a surface whose contract is "which items
changed". `notriosctl link` already produces the text.

**A GUI multi-select.** F1 is the API. Wiring it to a selection model in the
search pane is UI work that F5's graph view and the organizer panel will want to
share; doing it here would guess at that shape.

**MCP exposure.** Batch tools belong with the `organizer` profile, which is F2.
Adding them before profiles exist would put bulk mutation on the default
read-only surface.

## Validation

`go vet ./...`, `go test ./...`, required-file, plan-loop, and scaffold checks,
OpenAPI parse, migration-copy equality, web typecheck/tests/build, `make gui`,
docs-site build, Help reseed, REST/MCP smoke (extended with the batch surface:
per-item reporting, keyed replay, and an atomic rollback leaving the library
untouched), performance smoke, and the offline-assets check.

No scale profile. A batch is bounded to 500 items and every operation inside it
is one the single-note surfaces already carry evidence for; the ledger is a
primary-key lookup. The measurement that would matter — many notes through the
write path — is what J1–J3's million-item import evidence already covers.
