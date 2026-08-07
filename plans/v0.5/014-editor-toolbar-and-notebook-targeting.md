# v0.5 E10 + v0.6 F0 — The editor toolbar, and where a note gets filed

Status: complete on 2026-08-07.

Model: Claude Opus 5 (Claude Code).

Two tasks, one pass. E10 is a v0.5.0 release-candidate fix and F0 is the first
v0.6 task, but they touch the same component, the same tests, and the same
browser sweep — doing them separately would have meant restructuring the toolbar
twice.

## What it does

**E10 part 1 — layout.** The editor toolbar is a title row plus an action row
instead of one wrapping line. The action row stacks vertically as a unit when
the pane is too narrow for it.

**E10 part 2 — affordance.** "New note" left the editor toolbar for the search
pane, where it names its destination.

**F0 — notebook targeting.** The sidebar selection is the creation target; a
notebook control in the editor's own toolbar shows where the open note lives and
files it elsewhere on selection; `notriosctl notes move` covers the CLI.

## Measured, not asserted

The defect was found by measurement and the fix is verified the same way, in a
real browser across pane widths from 700 px down to the editor pane's own 280 px
minimum.

| | Before | After |
|---|---|---|
| Chip shares the title's row | at every width | never |
| Action row wraps raggedly | from 520 px down | never |
| Rows at 280 px | 4 | 2 |
| Title width as the pane narrows | 244 → 144 → 205 → 155 → 206 → 166 → 126 → 202 px | 684 → 584 → 504 → 454 → 414 → 374 → 364 → 334 → 304 → 264 px |

The title width is the tell. It used to move *non-monotonically* because the
input is `flex: 1` and expanded into whatever space a wrapped button vacated —
widening the pane could make the title smaller. It now decreases monotonically,
because nothing else shares its row.

The action row switches state exactly once, at the breakpoint, with no
intermediate ragged step: 36 px tall (one row) down to 385 px, 111 px (stacked)
at 380 px and below.

## Decisions worth recording

**The breakpoints sit above the measured content width on purpose.** A trashed
action row needs 326 px and would still fit at a 340 px pane; it stacks at 380.
That slack is the mechanism, not sloppiness. `flex-wrap: wrap` is the base rule
— the floor for an engine without container-query support — and if the
breakpoint sat at the true fit-limit there would be a band of widths where
`flex-wrap` raggedly wrapped one item before the all-or-nothing switch fired.
Stacking slightly early is what guarantees the ragged state is unreachable.

**The query is on the pane, not the viewport.** `container-type: inline-size` on
`.editor-pane` with `@container editor-pane (max-width: …)`. These panes are
splitter-resized independently of the window, so a media query would measure the
wrong box: a wide window can hold a narrow editor pane. Container-query support
was confirmed present in the test engine (`CSS.supports('container-type:
inline-size')`), and the `flex-wrap` base rule means an engine without it still
gets a row contained to itself rather than one crowding the title.

**Two breakpoints, not one shared worst case.** A trashed row (chip plus two
buttons, 326 px) stacks at 380 px; an editable row (263 px) at 300 px. One
shared breakpoint would have stacked the editable row at widths where it fits
comfortably.

**"New note" moved rather than being hidden.** It is not an action on the open
note — it discards the editor and starts a draft — which is why it read, on a
read-only Help or trashed note, as an offer to create something there. But
`onNewNote` had exactly one caller, so hiding it on read-only notes would have
stranded a reader: on a library holding only the fifteen seeded Help notes there
would have been no way to create a note at all. The search pane is the list
context and is present whatever is open.

**The notebook control lives in `md-editor-rt`'s own toolbar.** The user gated
this on whether that toolbar could take it, since a separate control above it
would wrap. Both halves were verified against the installed 6.5.3 rather than
assumed: `defToolbars` plus a numeric entry in `toolbars` accept custom items and
`DropdownToolbar` is exported with a controlled `visible`/`overlay` contract; and
`.md-editor-toolbar-wrapper` is `overflow-x: auto` with `flex-wrap: nowrap`, so
it scrolls at narrow widths. Confirmed in the browser: the toolbar is 35 px tall
at both a 300 px and a 900 px pane, so it never grows a second row.

The one cost — a custom item needs an explicit `toolbars` array — stays cheap
because `allToolbar` is exported, so `[...allToolbar, 0]` keeps future built-in
tools appearing and `toolbarsExclude` still filters.

**Selection is tracked by notebook ID, never by the sidebar row's query.** A
notebook row's query is `notebook:"<name>"` and names are unique only among
siblings (`notebooks_sibling_name_idx`), so `Contacts/Work` and `Personal/Work`
produce the same query. Deriving the creation target from `activeQuery` would
have filed into whichever came first, and highlighting by query would have lit
up both rows.

**The picker shows the open note's notebook, not the sidebar's selection.**
Otherwise reaching a note from "All notes" would claim it lives wherever the
sidebar happens to point. With no note open it shows the pending creation
target, so the sidebar and the toolbar agree about where a draft will land.

**"All notes", saved searches, and Help fall back to the default notebook, and
that is the right answer rather than a consolation.** They name a view, not a
place. Help is additionally never offered as a *destination*, because the
service refuses notes moved into or out of it — the picker mirrors the rule
instead of discovering a 403.

**A move takes effect immediately and writes no revision.** It changes where a
note lives, not what it says, so `MoveDocumentToNotebook` is not
revision-scoped. The message names the destination rather than leaving the
toolbar to imply it.

**`notriosctl notes move` has no dry run.** Unlike `gc`, `fix`, and `tags
rename`, a move is neither destructive nor lossy — moving it back is the same
command with the other notebook — so a confirmation step would be ceremony. It
does refuse an ambiguous notebook *name* and list the matching IDs, since
picking one silently is exactly the misfiling the command exists to correct.

## A test that was passing over the feature

The `MdEditor` stub in every web fixture rendered only a textarea, so it dropped
`defToolbars` — the notebook picker never reached the DOM and four new
assertions failed against a component that was working. The stub now renders
custom toolbar items, because a stub that silently discards half the contract it
stands in for will pass whatever is built against it.

## What is deliberately not here

**Multi-note move.** Selecting several notes and filing them together is a
bounded batch operation with per-item outcomes, which is F1. F0 deliberately
builds the single-note control that F1 generalizes rather than inventing one
there.

**Making creation notebook-scoped in the CLI or MCP.** `notriosctl notes move`
covers correction; `POST /api/v1/documents` has always accepted `notebook_id`.
Nothing else needed changing.

## Validation

`go vet ./...`, `go test ./...`, required-file, plan-loop, and scaffold checks,
web typecheck, **141** tests, and production build, `make gui`, REST/MCP smoke,
and the offline-assets check.

The browser sweep is the part unit tests cannot do, and it covered both halves:
the width sweep above, the picker rendering inside the real `md-editor-rt`
toolbar, Help absent from the destination list, a note created directly into
`Work` from the sidebar selection, and that note re-filed to `Notes` from the
toolbar with the search index agreeing. No console errors.
