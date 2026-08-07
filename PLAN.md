# Plan: v0.6 — MCP and automation expansion

Status: **draft. Written 2026-08-06 from `ROADMAP.md` after v0.5 completed.
One v0.5.0 release-candidate fix (E10, the editor toolbar: layout at narrow pane
widths plus which actions belong in it) is outstanding and comes first — see the
section before the v0.6 tasks. No task is approved; both E10 and F1 require user
approval before any code is written.**

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
it*, not carried into v0.6. E10 below lands under `plans/v0.5/`, does not change
the version, and must be complete before the owner accepts the candidate.

Both parts came from the same session of Trash testing, both live in
`web/src/components/EditorPane.tsx`, and both are verified by the same browser
sweep across pane widths, so they are one slice rather than two.

### E10. Editor toolbar: layout at narrow pane widths, and which actions belong in it

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

### F0. Notebook targeting: create where you are, and move what is misfiled

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

- Track the **selected notebook** in the app shell and create new notes into it.
  Selecting a search notebook (All notes, Trash, a saved search) or having no
  selection falls back to "Notes".
- Show the target before it is used, so a note's destination is never a surprise
  at save time.
- Add a single-note **move to notebook** control — a destination picker over the
  notebook tree — so a note created in the wrong place can be filed.
- Add `notriosctl` coverage for the same single-note move, since the CLI lacks
  it too.

**Two traps found while checking, both of which constrain the implementation.**

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
rule rather than discovering it through an error.

**Explicitly not here:** moving *several* notes at once. Multi-select move is a
batch operation and belongs to F1, which is the next task and already covers
bounded move with per-item outcomes. F0 is the single-note path F1 builds on;
doing them in this order means F1 extends a working control rather than
inventing one.

Working state: a note created while a notebook is selected lands in that
notebook, the target is visible before saving, and a note in the wrong notebook
can be moved from the GUI and from the CLI.

### F1. Batch organizer transactions

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

Working state: every REST capability is either an MCP tool or has a recorded
reason it is not, and no tool can return an unbounded payload.

### F4. Note templates and task extraction

Moved from v0.5, where it was on the roadmap but never entered the plan.

- Templates are stored notes with declared, typed placeholders — never
  executable text and never a filesystem path.
- Task extraction reads checkbox list items into a queryable form, addressable
  by the v0.5 block model rather than by line number.

Working state: creating a note from a template cannot execute anything or read
a file, and a task's identity survives the note being edited around it.

### F5. A graph view in the GUI

Also moved from v0.5, where E4 shipped the data and named itself "visualization
data" for exactly this reason.

- Render the bounded traversal from `POST /api/v1/graph` around the open note,
  with the depth ceiling visible rather than silently applied.
- Show what `GET /api/v1/graph/report` already computes — orphans, isolates,
  hubs — without recomputing it in the client.
- The view must degrade to a message when the service is unreachable, as every
  other assist in the editor does.

Working state: a node ceiling is a stated limit in the UI, not a frozen window.

### F6. Job control plane for bulk work

- Give import, export, and (later) sync a job API: start, status, cancel, and a
  bounded result summary.
- MCP may start and watch a job; it may never carry archive or blob bytes.
  REST and object transfer stay the data plane.
- `GET /api/v1/jobs/{job_id}` exists today as a placeholder; F6 is where it
  becomes real.

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

## Decisions required before or during v0.6

- **Whether F0 belongs in the v0.5.0 release candidate.** The GUI cannot move a
  note between notebooks at all, which is a gap rather than a regression — v0.5
  never promised it — but it is the kind of gap a first user meets immediately.
  Shipping the candidate first and fixing it in v0.6 is the conservative call;
  pulling F0 forward is defensible.
- **Idempotency key scope.** Whether a batch request key is scoped to a session,
  a client, or the database decides what "the same request twice" means across a
  restart. F1 cannot be designed without answering it.
- **What duplicate means.** A duplicated note must not inherit the original's
  external identity (`document_sources`), but whether it inherits resource
  references, tags, and its notebook is a product decision rather than an
  implementation detail.
- **Whether `administrator` is reachable over MCP at all.** Garbage collection,
  restore, and purge are administrator-shaped, and the v0.5 position was that
  whole-library operations stay on surfaces a person drives. F2 should either
  keep that or overturn it explicitly.
- Carried forward from v0.5 and still open: the long-term SQLite driver choice,
  and adoption of the official MCP Go SDK. Neither blocks F1.

## Already implemented, deliberately not re-listed

`ROADMAP.md` lists two v0.6 bullets that shipped earlier and are **not** tasks
here: LLM-safe surgical edits with dry-run and revision preconditions, and
LLM-safe SEARCH/REPLACE edits — both are the existing `edit_note` tool and
`PATCH /api/v1/documents/{id}`, complete since the v0.2 redesign (task R8).
Restating them as work would make the milestone look larger than it is.

## Scope control

Record-level sync and transports (v0.7), the deferred `movenotes-v3`
compatibility bridge (v0.7 slice 3), authentication and multi-user deployment,
Wails v3/mobile migration, HTTP range downloads, semantic/vector search, and
additional importers all remain outside v0.6 unless the roadmap is deliberately
revised.
