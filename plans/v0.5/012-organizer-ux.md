# v0.5 E8 — Organizer UX: trash-first delete, restore, and hierarchical tag rename

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## What it does

**In the GUI.** The editor toolbar offers *Move to Trash* on an editable note.
A trashed note opens with an "In the Trash" badge and two offers, *Restore* and
*Delete forever*. Sidebar notebook rows carry a delete affordance that confirms
with the service's own deletion preview.

**Through Store/REST/CLI.** `store.RenameTag`, `POST /api/v1/tags/rename`, and
`notriosctl tags rename` rename a tag and, with `include_children`, everything
under it — dry run by default, with per-tag counts.

**New read-only endpoint.** `GET /api/v1/notebooks/{id}/deletion-preview`.

## Decisions worth recording

**A dry run is a rolled-back apply, not a prediction.** `RenameTag` opens one
transaction, runs the real statements, and rolls back when `dry_run` is set. A
hierarchical rename can cascade — renaming `a/x` up to `a` makes a deeper tag's
new name a shallower tag's old one, and renaming onto an occupied name merges —
and cascading is exactly where a separate predictor and applier drift apart. It
is also the case where being wrong is expensive: a tag that disappeared into the
wrong parent is tedious to undo by hand. One code path removes the class of bug.

**`dry_run` defaults to true on REST and CLI.** The asymmetry is the point: a
caller who forgets the field gets a report; changing the library requires saying
so. The report costs one round trip.

**The CLI exits 1 when a dry run's plan contains a merge.** A script that meant
to rename and would instead have combined two hierarchies stops rather than
continuing to the next step.

**Hierarchy is matched by path segment, not by string prefix.** `projects` is
not a child of `project`. Matching uses ASCII case folding rather than Go's
Unicode `EqualFold`, because SQLite's `NOCASE` — which owns the unique index on
tag names — folds only A–Z. A Unicode-aware fold here would let a rename sweep
up a tag the database considers distinct.

**Renaming a tag into its own subtree is refused.** `project` → `project/old`
would make a tag its own ancestor, and the result would depend on the order tags
happened to come out in. Refusing keeps old-name-to-new-name injective and
order-independent; the remaining order-sensitive case (renaming a child *up*
onto its parent's name) is handled by processing shallowest first, which frees
the shallower name before the deeper tag needs it.

**A rename never rewrites note bodies, and never rewrites saved searches.** Tags
in Notrios are relational; a `#project` in someone's prose is prose. A saved
search that mentions the old name is *reported* in `warnings`, because deciding
which occurrences of a word are the tag is the kind of guess that silently
changes what a search means.

**A rename enqueues projection jobs for every affected note.** The derived
projection carries a note's tags, and nothing else in the transaction would
re-project them. A dry run's rollback takes the outbox rows with it, which a
test asserts directly.

**The ceiling (500 tags) refuses rather than truncates.** A half-renamed
hierarchy is worse than no rename.

**A notebook deletion preview answers even when the answer is "nothing".** A
protected notebook returns `200` with `deletable: false` and a reason rather
than a `403`. "What would happen" has an answer; the sidebar uses it to omit an
affordance that would only ever fail.

**A trashed note does not share the Help note's badge.** Both are uneditable,
but only one can be brought back, and calling a trashed note "read-only" hides
the one thing its reader can act on.

**The confirmation text is the feature.** "Delete this notebook?" tells the
reader nothing they did not already know. What is worth confirming is that the
notes are not deleted, that they move to the Trash, and that they are re-homed
to the default notebook so a later restore has a destination — and those numbers
come from the service, not from a guess in the client. The wording lives in
`web/src/organizer.ts` so it can be tested.

**Deleting or restoring updates the results list in place.** Re-running the
search would lose the pane's contents and scroll position. `removeHit`
deliberately does not backfill from the cursor: the list would then hold a row
from a page the user never scrolled to, and the next `loadMore` would read from
a cursor that no longer lines up with the screen.

## A defect the browser found

Driving the flow in a real browser turned up something unit tests could not:
`GET /api/v1/documents/{id}` returned **404 for a trashed note**, so clicking a
row in the Trash opened nothing.

It was a pre-existing defect, not one E8 introduced, and the codebase already
disagreed with itself about it. `api.Document.Editable` is documented as "false
for … trashed notes"; `toAPIDocument` computes `deleted_at` and clears
`editable` from it; `web/src/api.ts` declares `deleted_at` on a document; and
`App.openStableLink` has a branch for a link that resolves to `trashed` which
calls straight into the document read. All of that was dead code, because the
read stopped at the Trash.

The fix is narrow on purpose. `store.GetDocumentIncludingTrashed` is a second,
explicitly-named read used by exactly one caller — the REST document GET. The
ordinary `GetDocument` is unchanged, so every write path and every agent-facing
read still stops at the Trash and `is:trashed` stays a deliberate opt-in rather
than a scope a caller falls into. The same read also had to start populating
`DeletedAt`, which it selected and threw away; that is what makes a trashed
note read-only above the store.

One existing test asserted the old behaviour (`deleted document get should be
404`). It now asserts the new contract *and* the parts that must not change: a
trashed note is still unwritable and still absent from ordinary search.

A second, smaller thing the browser showed: opening a trashed note fired a
remote-media scan that 404s, because that route reads through the ordinary
`GetDocument`. The client now skips the scan for a trashed note — localizing
media writes a revision, which a trashed note cannot take, so the scan could
only ever produce an offer that must be refused.

## What is deliberately not here

**A GUI tag rename.** E8 scoped rename to Store/REST/CLI and the GUI to
deletion and restore. Adding a rename panel would be scope the plan did not ask
for.

**Bulk anything.** No multi-note trash, no batch organizer transaction. Those
are v0.6 and E8 must not pre-empt them. A hierarchical rename touches many rows
because a hierarchy is one thing, not because it is a batch: there is no way
here to submit a list of unrelated organizer edits.

**A projection enqueue on `AddDocumentTag`/`RemoveDocumentTag`.** They have
never enqueued one, and adding it would put an outbox row per tag into every
import that adds tags after creating a note. That is a pre-existing gap, noted
rather than widened. A rename is different: no other event covers it.

## Validation

`go vet ./...`, `go test ./...`, required files, plan-loop, scaffold validation,
OpenAPI parse, web typecheck/tests (124)/build, `make gui`, docs-site build,
Help reseed, REST/MCP smoke (extended to cover tag rename, the deletion
preview, and the trash cycle end to end), performance smoke, and the E6a
offline-assets check.

The whole GUI flow was also driven in a real browser against a running service:
delete to Trash, read the trashed note, restore, delete the notebook holding the
open note, and purge. That is what found the defect above; the unit tests all
passed while the feature did not work.

No scale profile. Nothing here is a new unbounded surface: the deletion preview
counts with two indexed `COUNT(1)` queries over one notebook subtree, and the
rename is bounded to 500 tags. The paths that do scale — search, the trash
keyset, notebook deletion itself — carry the 10k/100k/500k evidence already.
