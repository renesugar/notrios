# Plan: v0.6 — MCP and automation expansion

Status: **active. Written 2026-08-06 from `ROADMAP.md` after v0.5 completed.
E10, E11, and E12 (the v0.5.0 release-candidate fixes) are complete and the
candidate has no outstanding gates. F0 (notebook targeting) and F1 (batch
organizer transactions) are complete. F2 onward require user approval, and
fourteen of their open decisions were answered on 2026-08-07 across two rounds.
**One blocking decision remains** — where a read-only generated note can live,
which blocks only F5's hubs-report deliverable. F2, F3, F4, F6, and F7 are
unblocked. F5a was **withdrawn** and F5 reframed away from a global graph
canvas.**

v0.5 is complete and archived under `plans/v0.5/`, including a copy of its own
plan at `plans/v0.5/000-v0.5-plan.md`. Product version is 0.5.0 and the schema
is v16.

## Goal

v0.5 made the library better to work in by hand. v0.6 makes it safe to work in
*by machine*: bounded batch operations that either all happen or report exactly
which ones did, MCP tool sets scoped to what a caller is trusted with, and a
control plane for long-running work that never drags bulk bytes through a
model's context. It opens with F0, which is neither: the GUI cannot file a note
into a notebook or move one out, and F1's batch move has no single-note control
to generalize from until it can.

Three constraints shape every task below. The first two carry over from v0.5
unchanged; the third is what this milestone is actually about.

1. **Nothing unbounded.** Every batch, list, and job report pages through a
   keyset or an explicit bound, and every new surface gets a generated-scale
   profile or a written reason it needs none.
2. **Note content is untrusted.** Templates and extracted tasks are structured
   and permission-controlled; none of them becomes a way for note text to
   execute anything or reach arbitrary filesystem paths.
3. **A tool's reach is declared, not inferred.** What an MCP caller may do comes
   from the profile it connected under, checked server-side on every call. A
   tool hidden from `tools/list` must also be refused when called directly, and
   that must be true by construction rather than by remembering to check.

## Working-state rule

Unchanged from v0.5. Complete one task at a time. Every task updates tests and
living docs, runs its relevant validation, records attempt/model status, commits
a working slice, archives it under `plans/v0.6/`, produces a verified ZIP, and
asks for approval before the next task.

## Before v0.6: outstanding v0.5.0 release-candidate fixes

v0.5.0 is built and validated but **not tagged or pushed** — those are owner
steps. It is still a release candidate, so a defect found in it is fixed *in
it*, not carried into v0.6. These land under `plans/v0.5/`, do not change the
version, and must be complete before the owner accepts the candidate.

E10, E11, and E12 are complete. **The v0.5.0 candidate has no outstanding
gates.**

### E12. A read-only title you can still read — complete

Archived as `plans/v0.5/016-readonly-title-control.md`. The note title used
`disabled` rather than `readOnly` when a note could not be edited. Both refuse
edits; only `disabled` removes the control from the tab order, so a title longer
than the box could not be focused, scrolled, selected, or copied. Measured
against a plain probe input: `disabled` gives `focusable: false`, `readOnly`
gives `focusable: true`, with selection and scrolling otherwise identical.

### E11. Finding the GUI's own files, and the docs that explain running it — complete

Archived as `plans/v0.5/015-finding-the-web-interface.md`.

Reported by the user, and every part verified against the source tree.

**The GUI cannot be run from anywhere but the checkout root, and says the wrong
thing when it is.** `internal/httpapi/server.go` resolves
`filepath.Clean("web/dist")` — a path relative to the **process working
directory**. Running `bin/notrios` with `bin/` as the working directory looks
for `bin/web/dist` and finds nothing, so the webview renders raw JSON:

```json
{"error":{"code":"web_ui_not_built","message":"web/dist/index.html not found; run cd web && npm run build or use npm run dev"}}
```

That message is *wrong in exactly this case*: the assets are built, they are
simply not where this process is looking. A diagnostic that sends someone to
rebuild something already built is worse than no diagnostic.

There is **no configuration option** for the directory — `config.Config` has no
UI or web field — and no CLI flag. The only documented answer is one clause in
`docs/gui.md` ("relative to the working directory"), which explains the rule
without making it discoverable or forgiving.

- Resolve the web root from an ordered, documented list: an explicit
  `--web-dir` flag, then a config value, then the working directory, then the
  executable's own directory and its parent — the last two so `bin/notrios`
  works whether it is invoked from the checkout or from `bin/`.
- **Check at startup, not at first paint.** A GUI launch that cannot find its
  interface must fail on the terminal with a message naming every directory it
  looked in, rather than opening a window containing an error object.
- Keep the HTTP `web_ui_not_built` response for the headless service, but make
  its message distinguish "not built" from "not found here".

**The notebook dropdown has no border.** `.notebook-picker-trigger` is a bare
span, and `md-editor-rt`'s toolbar item wrapper supplies neither a border nor a
caret, so the notebook name floats over the editor with nothing marking it as a
control. Give it a border, a disclosure caret, and hover/focus affordance, and
keep the disabled state legible.

**Three `make` targets are undocumented:** `serve`, `doctor`, and `seed-help`.
Verified by sweeping every target in the `Makefile` against `README.md`,
`ENVIRONMENT_SETUP.md`, and `docs/`. `make serve` is the one that matters most
here — it is how a developer opens the GUI in a browser, and the user found it
before the documentation mentioned it.

- Document every target, in one table, including which are network-using.
- Say plainly how to run the GUI standalone and how to reach it in a browser,
  and where the web files must be for each.

**`web/README.md` is two milestones stale.** It calls the project "Notes
Companion", tells the reader to run `go run ./cmd/notesd` — a binary renamed in
v0.2 — and describes a v0.1 feature set. Rewrite it as a frontend developer's
entry point, or delete it if `ENVIRONMENT_SETUP.md` already covers the ground.

**Not a defect, but an answer the user asked for:** `make clean` does **not**
remove `web/node_modules`; `make clobber` does. That is deliberate — a clean
should not force a network reinstall — and it is already documented in
`docs/installation.md` and `ENVIRONMENT_SETUP.md`. The target table will state
it in one line so the answer is where the question gets asked.

Working state: `notrios` starts from any working directory or explains exactly
where it looked; the notebook control looks like a control; and every `make`
target is documented alongside instructions for running the GUI both ways.

Both parts came from the same session of Trash testing, both live in
`web/src/components/EditorPane.tsx`, and both are verified by the same browser
sweep across pane widths, so they are one slice rather than two.

### E10. Editor toolbar: layout at narrow pane widths, and which actions belong in it — complete

Archived as `plans/v0.5/014-editor-toolbar-and-notebook-targeting.md`, together
with F0: both landed in one pass because they touch the same component and the
same browser sweep.

#### Part 1 — layout

Found by the user while testing notes in the Trash, and confirmed by measuring
the built UI in a real browser at pane widths from 700 px down to the editor
pane's own minimum of 280 px (`MIN_WIDTHS.editor` in `web/src/panes.ts`).

**What is wrong.** `.editor-toolbar` is one `flex-wrap: wrap` row holding the
title input, the state chip, and every button. For a trashed note that is five
flex children competing for one line, and the result is ragged at every width
the app supports:

| Editor pane width | Toolbar rows | Behaviour |
|---|---|---|
| 700, 600 | 2 | chip shares the title's row, 7 px out of vertical alignment |
| 520, 470 | 3 | "Delete forever" alone drops to a second row |
| 430, 390, 350 | 3 | both buttons drop; chip still crowds the title |
| 320, 280 | 4 | fully ragged |

The chip never gets a row of its own, and because the title input is `flex: 1`
it expands into whatever space a wrapped button vacates — so its width moves
*non-monotonically* as the pane narrows (244 → 144 → 205 → 155 → 206 → 166 →
126 → 202 px). Widening the pane can make the title smaller. That is the part
that reads as broken rather than merely tight.

**What it should do**, for a note in the Trash:

- the title occupies its own row;
- the chip, **Restore**, and **Delete forever** sit on one row beneath it;
- when that row cannot fit, the chip and both buttons stack vertically —
  together, as a unit.

**The last point is the constraint that shapes the implementation.** Plain
`flex-wrap: wrap` moves one item at a time, which is precisely the ragged
behaviour being fixed; the switch to a column has to be all-or-nothing.

It also has to trigger on the **pane's** width, not the window's. These panes
are resized by splitters independently of the window, so a `@media` query would
be measuring the wrong box — a wide window can hold a narrow editor pane and
vice versa. A CSS container query (`container-type: inline-size` on the editor
pane, `@container` on the action row) is the right tool. Its support in
WebKitGTK must be **verified in the Wails webview**, not inferred from Chromium:
the offline-asset and editor harnesses cannot drive that engine, so this is a
manual check, and a `ResizeObserver` fallback is the alternative if it fails.
`App.tsx` already uses `ResizeObserver` for pane measurement, so the fallback
costs no new dependency.

**In scope, decided rather than assumed:** the editable toolbar has the same
defect in milder form — one row at 600 px, "Move to Trash" dropping at ≤470 px,
"New note" joining it at ≤340 px, so it is already two ragged rows across most
of the supported range. The user reported the Trash case; fixing only that would
leave the identical bug next door, so E10 applies the same title-row/action-row
split to both states. If that is not wanted, say so and it narrows to the
trashed state alone.

**Out of scope for part 1:** changing pane minimums or the splitter behaviour.
This half is a layout fix, not a redesign.

#### Part 2 — "New note" does not belong in a read-only note's toolbar

Reported by the user: **New note** appears in the toolbar while viewing a note
in the Trash, which is read-only. It appears for Help notes too — the condition
is `selectedDocument && …`, with no editability test at all.

The user is right, and for a sharper reason than it first looks. *"New note" is
not an action on the open note.* Every other control in that toolbar acts on the
note in front of you — save it, trash it, restore it, destroy it. "New note"
discards the editor's contents and starts a blank draft. Putting it among
per-note actions is what makes it read, in a read-only context, as an offer to
create something there.

**Two facts found while checking, both of which change the fix.**

**`onNewNote` has exactly one caller.** `web/src/App.tsx` wires `resetEditor` to
this button and nowhere else; the only other `resetEditor()` calls are internal,
after a delete and after a purge. So *simply hiding the button on read-only
notes strands the user*: open a Help note and there is no way back to a blank
draft except opening an editable note first. On a fresh library — whose only
notes are the fifteen seeded Help notes — a user could open one and then be
unable to create a note at all.

The fix is therefore to **move** the affordance, not to hide it conditionally.
It belongs in the search pane, which is the list/notebook context rather than
the open-note context, and is present whatever is open. The editor toolbar then
holds only actions on the open note, which is the rule that made the button look
wrong in the first place.

**Creating a note is not notebook-scoped today.** `createDocument` in
`web/src/api.ts` sends no `notebook_id` — the client's request type does not
even carry the field — so **every note the GUI creates lands in the default
"Notes" notebook**, regardless of which notebook or search view is open. The
REST API has accepted `notebook_id` on create all along
(`api.DocumentMutationRequest`); only the client never sends it.

This matters because "it should only show up when a user can create a new note
in that notebook" presumes creation targets the notebook being viewed. It does
not. So there is a decision to make, and E10 does not make it silently:

- **(a) Relocate only.** "New note" moves to the search pane and always starts a
  draft that saves into "Notes", as today. A defect fix, nothing more, and the
  right size for a release candidate. **Recommended.**
- **(b) Also scope creation to the selected notebook.** This is what the
  report's wording describes. It became **F0**, the first v0.6 task, once
  checking showed the GUI has no way to move a note between notebooks either —
  scoping creation without a way to correct a mistake would be half a feature.

E10 as written does **(a)** and stays a defect fix; F0 does the rest. Pull F0
into the release candidate instead if you would rather not tag v0.5.0 with a GUI
that cannot file a note.

**Out of scope for part 2:** any other toolbar control, and the contents of the
new-note draft itself.

#### Testing both parts

Tests assert arrangement and behaviour, never CSS. For part 1: at a wide pane
the actions share one row and the title has its own; at a narrow pane the
actions occupy one column; and the title's width never increases as the pane
narrows. For part 2: the toolbar offers "New note" for no note, read-only or
otherwise, and the search pane offers it always — including while a Help note or
a trashed note is open, which is exactly the case that would otherwise strand a
user.

Verification includes measuring the built UI in a real browser across the same
width sweep, because part 1 is a defect every existing unit test passed over and
part 2 shipped through a full test suite that never asked who the button was
for.

Working state: a trashed note's toolbar reads as a title with an action row
under it at every supported pane width, nothing ever half-wraps, and the toolbar
offers only actions that apply to the note in front of you — while starting a
new note stays reachable from anywhere.

## Tasks

### F0. Notebook targeting: create where you are, and move what is misfiled — complete

Archived as `plans/v0.5/014-editor-toolbar-and-notebook-targeting.md`.

Raised by the user after E10 part 2 turned up that the GUI always creates into
the default "Notes" notebook. Checking the rest of the surface made it worse
than a default-value complaint:

| Layer | Move a note to another notebook |
|---|---|
| Store | `MoveDocumentToNotebook` |
| REST | `POST /api/v1/documents/{document_id}/notebook` |
| MCP | `move_note_to_notebook` (editor profile) |
| CLI | **absent** |
| GUI | **absent** — `web/src/api.ts` carries no client function for it at all |

So the GUI creates every note in "Notes" and then offers no way to move it. The
ordinary workflow — write a note, file it — is not merely tedious in the GUI, it
cannot be completed there. `API_SPEC.md`'s full-client parity table has been
claiming this capability on REST's behalf, which is true and beside the point:
the built-in client does not use it.

The shape follows Joplin, which the user cited: **the sidebar selection is the
target, and the sidebar's own highlight is the primary visual cue.** "New note"
lives in the search pane (E10 part 2 already moves it there for a different
reason, and this is the second one). Selecting "All notes", a query-backed
search notebook, or a read-only notebook falls back to "Notes", which is the
right default in each of those cases rather than a compromise.

- Track the **selected notebook** in the app shell and create new notes into it,
  with the sidebar highlight as the cue.
- Put a **notebook dropdown in the editor's own toolbar**, pre-selected to the
  sidebar's notebook. It is a second cue *and* the correction: change it and the
  open note moves. Joplin's failure mode is creating several notes in the wrong
  notebook before noticing, and a control that shows the answer where the typing
  happens is what shortens that.
- Add `notriosctl` coverage for the same single-note move, since the CLI lacks
  it too.

**The user's condition on the dropdown — only if the editor toolbar can take it,
because a separate control above the toolbar would wrap awkwardly and "may not
be worth doing" — checks out in our favour, twice.** Verified against the
installed `md-editor-rt` 6.5.3 rather than assumed:

- It accepts custom toolbar items. `defToolbars?: Array<ReactElement>` takes the
  elements and `ToolbarNames = keyof ToolbarTips | number` positions them by
  index in the `toolbars` array. It exports **`DropdownToolbar`** — controlled
  `visible`/`onChange` with an arbitrary `overlay` — which is a dropdown
  already, so the notebook picker is the overlay's content and nothing is
  reimplemented.
- Its toolbar **cannot** reintroduce the E10 wrapping problem, because it does
  not wrap: `.md-editor-toolbar-wrapper` is `overflow-x: auto; overflow-y:
  hidden; scrollbar-width: none`. It scrolls horizontally at narrow widths.

So the fallback the user described — a separate control above the toolbar — is
not needed, and the "may not be worth doing" branch does not apply.

One cost, recorded rather than discovered later: positioning a custom item means
passing an explicit `toolbars` array instead of relying on the default. That is
cheap here because `md-editor-rt` exports **`allToolbar`**, so the list stays
`[...allToolbar, 0]` with the existing `toolbarsExclude` still filtering it —
a new built-in tool in a future release still appears.

**Traps found while checking, which constrain the implementation.**

**The selected notebook must be tracked by ID, not inferred from the query.**
The sidebar builds a notebook row's query as `notebook:"<name>"`
(`web/src/sidebar.ts`), and notebook names are unique only **among siblings**
(`notebooks_sibling_name_idx`). `Contacts/Work` and `Personal/Work` can both
exist and both produce `notebook:"Work"`, so deriving the creation target from
`activeQuery` would silently pick the wrong notebook. The app shell currently
keeps only `activeQuery`; it needs the selected row.

**"Create here" must not become "create in a place that refuses it."** The Help
notebook is protected and notes cannot be moved into or out of it; the store
returns `403`. A creation target that lands on Help has to be refused in the
client the same way the delete affordance already is — by mirroring the server
rule rather than discovering it through an error. The same rule governs the
dropdown: Help is not an available destination, and for a Help note or a trashed
note the dropdown is shown disabled rather than hidden, so the note's notebook
stays visible even where it cannot be changed.

**The dropdown must not silently move a note the user is only reading.** It is
pre-selected from the *sidebar* when starting a new note, but from the *open
note's own notebook* once one is open — otherwise browsing to a note from "All
notes" would show it belonging somewhere it does not. Changing it is a move and
takes effect immediately, as `MoveDocumentToNotebook` is not revision-scoped;
the message says which notebook the note landed in.

**Explicitly not here:** moving *several* notes at once. Multi-select move is a
batch operation and belongs to F1, which is the next task and already covers
bounded move with per-item outcomes. F0 is the single-note path F1 builds on;
doing them in this order means F1 extends a working control rather than
inventing one.

Working state: a note created while a notebook is selected lands in that
notebook, the target is visible before saving, and a note in the wrong notebook
can be moved from the GUI and from the CLI.

### F1. Batch organizer transactions — complete

Archived as `plans/v0.6/001-batch-organizer-transactions.md`. Both open
decisions were taken rather than deferred: **idempotency keys are
database-scoped and persisted** (schema v17 `batch_operations`), because a batch
is retried exactly when a process may have restarted; and **a duplicate
inherits content, not identity** — body, notebook, tags, and resource
references, but never the `document_sources` row or the revision history.

- Add bounded batch operations over an explicit list of note IDs: move to
  notebook, add/remove tags, trash, restore, and duplicate.
- Support both **all-or-nothing** and **best-effort** modes, chosen per request,
  and return per-item outcomes either way. A best-effort run that half-succeeded
  must say which half.
- Make requests idempotent: the same request key applied twice does the work
  once and returns the first run's outcomes.
- Keep every item revision-preconditioned where the operation writes a revision,
  as `notriosctl fix` and the v0.5 organizer already do.

The v0.5 organizer deliberately stopped short of this (see
`plans/v0.5/012-organizer-ux.md`), so F1 is where it lands.

Working state: a client can move fifty notes into a notebook and know exactly
what happened to each one, and re-sending the request after a dropped connection
does not do it twice.

### F2. Tool visibility profiles

- Replace the two-profile (`read-only` / `editor`) split with the roadmap's
  five: search-only, read-only, editor, organizer, administrator.
- Enforce the profile at the call site, not only in `tools/list`. A hidden tool
  that answers when called is not hidden.
- Add a test that walks every registered tool against every profile and asserts
  the allowed set exactly, so a new tool cannot be added without deciding which
  profiles get it.

**Open decisions**

- **Is `administrator` reachable over MCP at all?** **Resolved 2026-08-07: no.**
  Notrios is single-user, so administrator and author are the same person; a
  profile here restricts what that person's *agent* may do, not who they are.
  Destructive whole-library operations stay a deliberate act on the command
  line. Multi-user roles are a future milestone, not this one.

  *Consequence, and a revision to the roadmap wording:* **there are four MCP
  profiles, not five.** `search-only`, `read-only`, `editor`, `organizer`. A
  profile that cannot be selected is not a profile — the CLI is not gated by
  MCP profiles, so "administrator" would name an empty set. `ROADMAP.md` says
  five; F2 corrects it rather than shipping a fifth that does nothing.

- **The word "profile" already means something else in Notrios.** **Resolved
  2026-08-07: rename the MCP concept.** The database registry is user-facing,
  documented, and on disk; the MCP key is one line in a config file.

  *Checking to implement it found the collision is **three**-way, not two.*
  `notriosctl profile register` (a named local database), `notriosctl publish
  profile save` (a saved publication selection), and `mcp.default_profile` (a
  permission tier). That strengthens the answer rather than complicating it: the
  first two are both "a saved named configuration", which is a coherent use of
  the word, and the MCP one is a different kind of thing wearing the same name.
  It is the odd one out, and it is the one that moves.

  **The rename, concretely:** `mcp.default_profile` becomes `mcp.default_scope`,
  and the concept is a **tool scope** throughout the documentation. Values are
  unchanged — `search-only`, `read-only`, `editor`, `organizer`.
  `mcp.default_profile` stays accepted as a deprecated alias, because a config
  file that silently stops applying a *restriction* is the worst possible
  failure mode for this particular key: it would widen what an agent may do
  without saying so. If both keys appear, the narrower of the two wins and the
  service logs it.

- **Does a profile change take effect mid-session?** **Resolved: no, restart.**
  Making the choice explicit is the point — the same reasoning that makes
  switching database profiles a deliberate act rather than a hidden one.

*Recorded for the future, not built here:* multi-user roles (administrator,
author, reviewer) over a shared remote service. Joplin offers only read-only
versus read-write per shared notebook; Obsidian offers no granular permissions
at all. If Notrios adds roles, it would be going further than either, which is
a milestone of its own — see `agent/OPEN_QUESTIONS.md`.

**When roles do arrive, profiles will not be the mechanism.** A tool-visibility
profile is a guardrail the single user puts on their own agent; it is not
authorization, because there is no second principal to authorize against. F2
must say so in `SECURITY_REVIEW.md`, so a later reader does not mistake a
seatbelt for a lock.

Working state: connecting under `search-only` can search and read nothing else,
and adding a tool without classifying it fails the test rather than shipping
open.

### F3. MCP coverage and resource reads

- Close the gap between what REST exposes and what MCP does, deliberately: for
  each REST surface, either add the tool or record why it stays off the model
  path. `docs/api/mcp.md` already lists what is currently withheld and why; F3
  turns that list into decisions.
- Add MCP resource reads (attachment metadata and bounded content) under the
  profiles from F2.
- Keep the existing rule that bodies and payloads are truncated at
  `mcp.max_document_bytes` and never streamed whole into context.

**Open decisions**

- **Which REST surfaces become MCP tools.** **Resolved: the proposed default.**
  Read-shaped surfaces become tools under `read-only` (blocks, graph traversal
  and report, query-block evaluation, the lint report); write-shaped and
  whole-library ones (fix, tag rename, notebook deletion, GC, archive
  export/verify/restore, publication) stay off, with the reason recorded per
  surface. Batch operations land under `organizer`.
- **Do resource reads return bytes or metadata plus a URI?** **Resolved: the
  proposed default** — metadata plus a `resource://` URI, bytes only for
  text-like MIME types under `mcp.max_document_bytes`.
- **Byte-range resource reads.** *New, requested, non-blocking.* The caller
  should be able to read a *range* of a resource, deciding from the metadata and
  URI how much to pull — the same shape as `get_note_line_range`, which already
  lets a model read part of a note instead of all of it. This is what makes the
  metadata-plus-URI default usable rather than merely safe.

  *It lands on a known gap:* `API_SPEC.md` lists HTTP range requests for
  resource content among the deferred items, so
  `GET /api/v1/resources/{id}/content` does not honour `Range` today.
  **Resolved 2026-08-07: add REST range support first**, and have the MCP tool
  use it. One implementation, and it closes a deferred gap that browsers and
  media playback want anyway.

  Three details that follow, all defaults rather than open questions unless you
  disagree. Range applies to **resource content only**, not to
  `/documents/{id}/body` — a note body is already bounded and readable whole,
  and `/lines` and `/search-in` already serve partial reads with better
  semantics than byte offsets. A range request must remain compatible with
  `?download=1`. And an unsatisfiable range gets `416` with a `Content-Range`
  header naming the actual size, rather than silently returning the whole
  resource, so a caller that guessed wrong finds out.

  `API_SPEC.md`'s deferred list and `PLAN.md`'s scope-control section both
  needed correcting, since both said range requests were out of scope.

Working state: every REST capability is either an MCP tool or has a recorded
reason it is not, and no tool can return an unbounded payload.

### F4. Note templates and task extraction

Moved from v0.5, where it was on the roadmap but never entered the plan.

- Templates are stored notes with declared, typed placeholders — never
  executable text and never a filesystem path.
- Task extraction reads checkbox list items into a queryable form, addressable
  by the v0.5 block model rather than by line number.

**Open decisions**

- **What a placeholder is.** **Resolved: both**, as recommended — prompted
  values supplied at creation, plus automatic substitutions drawn from a
  **closed vocabulary** of names. Closed is the operative word: an open
  vocabulary is an expression language, and an expression language in note
  content is the thing E7 spent a slice refusing.
- **Is an extracted task stored or computed on read?** **Resolved: computed on
  read**, from the schema-v14 block rows that already carry content-derived
  identity. No new table until a query needs one a scan cannot serve.

Working state: creating a note from a template cannot execute anything or read
a file, and a task's identity survives the note being edited around it.

### F5a. Investigation: how to render a bounded graph — **withdrawn**

F5a would have measured a global force-directed canvas against cheaper
alternatives at the 5,000-node ceiling. It is withdrawn because the user made
the decision on better grounds than a measurement would have given, and the
finding it was most likely to reach is already established: Obsidian's global
graph degrades into an unreadable hairball past a few thousand notes, while its
*local* graph stays useful at any scale.

Measuring a thing in order to reject it is only worth doing when the rejection
is in doubt. Here it is not, and a spike that confirms an accepted answer is
ceremony. Recorded rather than deleted, because "we chose not to investigate,
and why" is the part a later reader needs.

What remains genuinely unknown is small enough to settle inside F5: a local
graph at depth 1–2 is tens of nodes, which any rendering approach handles, so
there is no library-versus-hand-rolled question worth a slice.

### F5. Graph views that stay readable at scale

Reframed 2026-08-07 from the user's counter-proposal. The original — "a graph
view in the GUI" — inherited Obsidian's global canvas without asking whether it
earns its place at Notrios' target scale. It does not. Three deliverables
replace it, each aimed at a question the global canvas answers badly.

**1. A local graph, around the open note.** Bounded traversal from
`POST /api/v1/graph` at depth 1–2, with the depth ceiling visible rather than
silently applied. This is the half of Obsidian's feature that stays useful in a
large vault: a map of contextual relevance for *this* note, not a picture of
everything.

**2. A "Top N hubs" report, written as a note.** The most-linked notes as an
ordinary Markdown note in the default notebook, with links to each. A ranked
list is readable at any library size, which is exactly what the global canvas
stops being.

*Most of this already exists.* `GET /api/v1/graph/report` has shipped since v0.5
E4 and already ranks hubs by **in-degree** — the same metric the analysis
recommends — alongside orphans and isolates, from one ordered scan. F5 is
presentation, not computation, and needs no graph library: the traversal is
already SQL and already bounded.

**3. Export for tools built for large graphs.** Notrios should not become Gephi.
Emit the link graph in an interchange format so people who want centrality,
modularity, or community detection can use software designed for it — Gephi
handles hundreds of thousands of nodes, Cytoscape adds typed edges and semantic
filtering that a notes app has no business reimplementing.

**Open decisions**

- **How the hubs report regenerates.** **Resolved 2026-08-07: a stable note ID,
  overwritten, read-only like a Help note** — a report a reader can edit is a
  report that silently stops being true. It excludes itself from its own
  ranking, or the report becomes a hub.

  *Implementing that turned up a mechanism problem.* "Read-only" is not a
  property a note can carry: `toAPIDocument` computes
  `Editable: doc.NotebookID != store.HelpNotebookID`, hard-coded to one
  notebook ID. A generated report in the default notebook cannot be read-only
  without changing that, and the Help notebook is not a home for it —
  `notriosctl seed-help` removes notes whose source file disappeared, so the
  next reseed would delete the report.

  **New, blocking for this deliverable.** Options:
  - **(a) A builtin "Reports" notebook**, and generalize the rule from "is the
    Help notebook" to "is a builtin notebook". Symmetric with Help, gives future
    generated reports (lint, GC, resource usage) a home, and the protection
    mechanism already exists — it just needs to stop naming one notebook.
    Costs a sidebar row and a bootstrap row. **Recommended.**
  - **(b) Keep it in the default notebook and add a per-note protection
    reason** — a `generated_by` column or metadata key that `Editable` also
    consults. More precise, but it makes note-level protection a second concept
    alongside notebook-level protection, and every surface that decides
    editability has to learn both.
  - **(c) Accept that the report is editable**, and let regeneration overwrite
    whatever was typed. Cheapest, and exactly the failure the answer above
    rejects.

  Note that (a) changes an invariant `sidebar.ts` and `UI_DESIGN.md` both
  state — "Help is always immediately above Trash" — so a second builtin
  notebook needs its position in that ordering decided too. *Recommended:*
  Reports sits with Help in the last-anchored group, above it.

- **What triggers regeneration.** *New, non-blocking.* The report is stale the
  moment a link changes. *Default if unanswered:* explicit only —
  `notriosctl graph report --write-note` and a REST equivalent — never on a
  schedule and never on write. A whole-collection scan on every save would be
  the one unbounded thing in an otherwise bounded design, and the report carries
  its generation timestamp so a reader can see how old it is.

- **Which export format.** **Resolved: CSV node and edge lists.** They stream at
  any library size without holding a document tree in memory, and Gephi,
  Cytoscape, NetworkX, and igraph all import them.
- **Whether export is CLI-only.** **Resolved: CLI-only** —
  `notriosctl graph export`, matching archive export. It writes files to a path
  the user names, which is not a choice a REST caller or an MCP client should
  make.

**Deliberately not here:** a global graph canvas. No Go graph library is needed
either — `gonum/graph`, `dominikbraun/graph`, and `yourbasic/graph` are all
capable and appropriately licensed, but the traversal, the shortest path, and
the in-degree ranking are already implemented in bounded SQL. Adding a
dependency to recompute what the store already answers would be pure cost.

Working state: a reader can see what surrounds the note in front of them, read
which notes the library actually hangs off, and take the whole graph elsewhere
if they want analysis Notrios does not do.

### F6. Job control plane for bulk work

- Give import, export, and (later) sync a job API: start, status, cancel, and a
  bounded result summary.
- MCP may start and watch a job; it may never carry archive or blob bytes.
  REST and object transfer stay the data plane.
- `GET /api/v1/jobs/{job_id}` exists today as a placeholder; F6 is where it
  becomes real.

**Open decisions**

- **Do jobs survive a restart?** **Resolved: persist job records, do not resume
  the work.** A client can ask what happened to a job it started; an interrupted
  import already resumes through its own checkpoints, and a second resume
  mechanism on top would give two answers to one question. This needs a schema
  version for the job table.
- **Is cancellation cooperative or immediate?** **Resolved: cooperative** — the
  flag is checked at the next bounded batch boundary, matching how the importers
  already commit.
- **Reproducing a job.** **Resolved: store parameters, render the command.**
  `notriosctl jobs show --command` renders stored *parameters* back into a
  runnable command line rather than storing raw argv — argv would capture local
  paths and any secrets that happened to be on the command line into the
  database, and a rendering can improve as flags change while a stored string
  cannot.
- **Job dependencies, and the scope boundary.** **Resolved: no scheduler.**
  Status is queryable by ID and legible to a shell — `notriosctl jobs status
  <id>` exits non-zero while running or on failure, `--wait` blocks until the
  job settles — which is enough for `job-a && job-b`. If `--wait` proves
  insufficient in practice, the next step is a **documented exit-code
  contract**, not a DAG. Sequencing stays in the caller's script.

  *Recorded so the contract is designed rather than accreted:* exit codes will
  need to distinguish at least *succeeded*, *failed*, *still running*, and *no
  such job*, because a script that cannot tell "not finished" from "failed" will
  either poll forever or give up early.

Working state: a long import can be started, watched, and cancelled without a
client holding the connection open, and no job payload reaches a model.

### F7. v0.6 documentation and release wrap-up

- Document batch operations, profiles, templates, the graph view, and the job
  API in both the site and the Help notebook.
- Reconcile `FEATURE_MATRIX.md`, architecture/schema/API/security documents, and
  the release checklist.
- Run the full release validation, archive the plan, draft the next plan from
  `ROADMAP.md`, and produce a verified source ZIP.

Working state: documentation matches implementation and v0.6 release checks
pass.

## Decisions register

An index, not a home. Each decision lives in the item it affects, under that
item's **Open decisions** subsection (see `AGENTS.md`, "Writing plan items"); if
it is only listed here, the person approving the item will not see it. That is
what happened to F1, whose two decisions sat here and nowhere else.

| Decision | Item | Status |
|---|---|---|
| Idempotency key scope | F1 | **Resolved:** database-scoped and persisted (schema v17) |
| What a duplicate inherits | F1 | **Resolved:** content, never external identity or revision history |
| Whether F0 belonged in the v0.5.0 candidate | F0 | **Resolved by shipping:** F0 landed with E10 in `076526f` |
| Is `administrator` reachable over MCP | F2 | **Resolved 2026-08-07: no** — and so there are four profiles, not five |
| Does a profile change need a restart | F2 | **Resolved: no, restart** |
| The word "profile" means three things | F2 | **Resolved: rename the MCP one** to `mcp.default_scope`, old key a deprecated alias |
| Which REST surfaces become MCP tools | F3 | **Resolved:** read-shaped become tools; the rest stay off with a recorded reason |
| Do MCP resource reads return bytes | F3 | **Resolved:** metadata plus URI, bytes only for text-like MIME |
| How byte-range resource reads reach REST's deferred range support | F3 | **Resolved: add REST `Range` first**, MCP uses it |
| What a template placeholder is | F4 | **Resolved: both**, with a closed vocabulary of automatic names |
| Are extracted tasks stored or computed | F4 | **Resolved: computed on read** |
| How to render a bounded graph | F5 | **Resolved by reframing** — no global canvas; F5a withdrawn |
| How the hubs report regenerates | F5 | **Resolved:** stable ID, overwritten, read-only |
| Where a read-only generated note can live | F5 | **Open, blocking** — new; `Editable` is hard-coded to the Help notebook |
| What triggers report regeneration | F5 | Open, non-blocking — new |
| Which graph export format | F5 | **Resolved: CSV node and edge lists** |
| Whether graph export is CLI-only | F5 | **Resolved: CLI-only** |
| Do jobs survive a restart | F6 | **Resolved:** persist records, do not resume work |
| Is cancellation cooperative | F6 | **Resolved: cooperative** |
| How a job's command line is reproduced | F6 | **Resolved:** stored parameters rendered back, never raw argv |
| Whether Notrios sequences dependent jobs | F6 | **Resolved: no** — status plus `--wait`; an exit-code contract before a DAG |
| Multi-user roles and an `author` concept | — | Deferred to a future milestone; `agent/OPEN_QUESTIONS.md` |
| Long-term SQLite driver | — | Open; `agent/OPEN_QUESTIONS.md` 1 |
| Official MCP Go SDK adoption | — | Open; `agent/OPEN_QUESTIONS.md` 2 |

Fourteen decisions have been answered across two rounds, and each round threw
off a few new ones — which is the normal shape of this rather than a failure of
the round before.

**One blocking decision remains**, and like the others it came out of
implementing an answer rather than out of nowhere: *where a read-only generated
note can live*, since `Editable` is hard-coded to the Help notebook ID and the
Help notebook itself would delete the report on the next reseed. It blocks only
F5's second deliverable.

**F2, F3, F4, F6, and F7 are unblocked.** F5's local graph and export halves are
unblocked too; only the hubs-report note waits.

## Already implemented, deliberately not re-listed

`ROADMAP.md` lists two v0.6 bullets that shipped earlier and are **not** tasks
here: LLM-safe surgical edits with dry-run and revision preconditions, and
LLM-safe SEARCH/REPLACE edits — both are the existing `edit_note` tool and
`PATCH /api/v1/documents/{id}`, complete since the v0.2 redesign (task R8).
Restating them as work would make the milestone look larger than it is.

## Scope control

Record-level sync and transports (v0.7), the deferred `movenotes-v3`
compatibility bridge (v0.7 slice 3), authentication, multi-user deployment and
user roles, Wails v3/mobile migration, semantic/vector search, and additional
importers all remain outside v0.6 unless the roadmap is deliberately revised.

**HTTP range requests moved *in*.** They were on this list; F3's byte-range
resource read needs them, and the resolved answer implements range support in
REST once rather than giving MCP a private mechanism. `API_SPEC.md`'s deferred
list was corrected to match.

A global graph canvas is now explicitly out: see F5.
