# v0.6 F7 — Documentation and release wrap-up

Status: complete on 2026-08-08.

Model: Claude Opus 5 (Claude Code).

## What it does

Reconciles every v0.6 `ROADMAP.md` bullet against the code, audits the
documentation against the implementation, bumps the product version to
**0.6.0** (schema v18), writes the v0.6.0 release checklist, archives the v0.6
plan, and drafts the v0.7 plan.

Following E9's precedent, the reconciliation was the point rather than a
formality — and, as in E9, it found things.

## What the reconciliation found

**Three defects, all fixed here.**

1. **`POST /api/v1/batch` applied `trash`, `add_tags`, and `remove_tags` to
   notes in a read-only notebook**, while the equivalent single-note routes
   answered `403` for the same notes. `move` and `duplicate` were guarded and
   the other three were not — which is exactly what a per-operation check
   produces. Demonstrated live: a seeded Help note was tagged *and sent to the
   Trash* through the batch route.

   The guard now runs **once, before the dispatch**, so an operation added later
   inherits it. `restore` is the deliberate exemption: it can only apply to a
   note already in the Trash, and refusing it would strand one there. The test
   asserts all five operations rather than the three that were broken.

2. **`POST/DELETE /api/v1/documents/{id}/tags/{tag}` had no read-only guard at
   all.** It was the one mutation route that never called `guardReadOnlyNote`.
   This matters more than it looks: `note_tags` is keyed by a stable document
   ID, so a tag applied to a Help note **survives `seed-help` reseeding it**,
   and the GUI offers no way to remove it from a note it treats as read-only.

3. **Six REST surfaces had neither an MCP tool nor a recorded reason**, despite
   F3's guide claiming every surface had been decided: restore-from-trash,
   revision restore, resource attach/detach, notebook create/rename,
   search-notebook create/delete, and collection create/patch — plus buffer link
   checking and stable-link resolution. Each is now decided in
   `docs/api/mcp.md`, with the reason recorded per surface.

**One scope-design inconsistency, fixed.** Tagging a single note was reachable
over MCP only through `run_batch` under `organizer` — so labelling one note you
had just created required granting the ability to trash five hundred. Tagging
one note is a single-note write, which is what `editor` is for, so `tag_note`
and `untag_note` were added there. That is what makes the roadmap's "complete
MCP write-tool coverage gated by explicit scopes" bullet true rather than
approximately true.

**One roadmap bullet shipped half, and is recorded that way.** "MCP
starts/statuses bulk export/import/sync jobs" became watching only in F6, on the
grounds that every job kind names a filesystem path and no REST or MCP surface
accepts one. The bullet now says so instead of being struck through. It does
**not** move to v0.7: it is a decision, not an omission.

## Checks that are now mechanical rather than read

Two reconciliation checks that E9 did by reading are now scripts anyone can
rerun:

- every registered MCP tool is named in `docs/api/mcp.md` (39 of 39);
- every REST route in `server.go` appears in `api/openapi.yaml`, and every
  documented path has a route (63 paths, no drift either way).

Both passed after the fixes above, and both would have caught the F3 gap if they
had existed then — but only the second kind; the six missed surfaces had no
*tool* to check, which is why that one stayed a reading exercise. Worth noting
rather than papering over.

## Version and documents

Product version **0.6.0** in `internal/version`, `web/package.json`, and the CLI
guide. Schema **v18**. `CODING_CLIENT_HANDOFF.md`, `README.md`,
`RELEASE_CHECKLIST.md`, `ROADMAP.md`, `SYSTEM_ARCHITECTURE.md`, and
`docs/api/mcp.md` reconciled; `CODING_CLIENT_HANDOFF.md` had been claiming
schema v16 since v0.5.

The v0.6 plan is archived at `plans/v0.6/000-v0.6-plan.md`, and `PLAN.md` now
holds the **v0.7 draft** — versioning and synchronization.

## The v0.7 draft, and its honest count

v0.7 carries **eight blocking decisions**, six of them open since v0.4 and
deferred to "separate v0.7 plans". This is that plan, and each now sits in the
item that needs it rather than in a milestone-level list — the rule F1 taught.

It also opens with **G0, an investigation slice**: two of the eight decisions
(per-field versus whole-record LWW, and add-wins versus remove-wins) are
unanswerable in the abstract, because they depend on what really diverges in a
real library. That is the case `AGENTS.md` says gets a spike rather than a
guess, and v0.6 learned the lesson twice — F5a was withdrawn for measuring a
foregone conclusion, and F6 was narrowed once the evidence was in front of it.

v0.7 is also the first milestone that can lose a user's data by being subtly
wrong, since it merges divergent copies. The plan says so at the top.

## Validation

`go vet ./...`, `go test ./...`, `make validate`,
`python3 scripts/check_required_files.py`, `python3 scripts/check_plan_loops.py`,
OpenAPI parse and route-drift check, MCP tool-coverage check, `npx tsc --noEmit`,
web tests, `npm run build`, `make gui`, `bash scripts/build_docs_site.sh`,
`notriosctl seed-help` twice (15 created, then 15 kept),
`bash scripts/mvp_smoke.sh`, `bash scripts/run_performance_smoke.sh`,
`bash scripts/run_offline_assets_check.sh`, and a verified source ZIP.
