# Plan: v0.6 — MCP and automation expansion

Status: **draft. Written 2026-08-06 from `ROADMAP.md` after v0.5 completed.
No task is approved; F1 requires user approval before any code is written.**

v0.5 is complete and archived under `plans/v0.5/`, including a copy of its own
plan at `plans/v0.5/000-v0.5-plan.md`. Product version is 0.5.0 and the schema
is v16.

## Goal

v0.5 made the library better to work in by hand. v0.6 makes it safe to work in
*by machine*: bounded batch operations that either all happen or report exactly
which ones did, MCP tool sets scoped to what a caller is trusted with, and a
control plane for long-running work that never drags bulk bytes through a
model's context.

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

## Tasks

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
