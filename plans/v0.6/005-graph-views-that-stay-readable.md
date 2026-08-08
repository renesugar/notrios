# v0.6 F5 — Graph views that stay readable at scale

Status: complete on 2026-08-08.

Model: Claude Opus 5 (Claude Code).

## What it does

Three deliverables replace the global graph canvas the original plan item
inherited from Obsidian:

1. **A local graph** in the GUI — the notes one or two hops from the open one,
   grouped by distance, with the ceiling stated rather than silently applied.
2. **A hubs report written as a note** — `POST /api/v1/graph/report/note` and
   `notriosctl graph report --write-note` render it into a new builtin
   **Reports** notebook, read-only, stable ID, overwritten in place.
3. **CSV node and edge export** — `notriosctl graph export <dir>`, CLI-only.

It also carries the predicate the three surfaces share, and closes a
pre-existing defect found while working out what that predicate touches.

No schema change and no migration: the `notebooks` table already exists and
bootstrap's `INSERT OR IGNORE` reaches an existing database on the next open.

## Decisions worth recording

**`IsReadOnlyNotebook`, deliberately not `IsBuiltinNotebook`.** Thirteen places
hard-coded `NotebookID == HelpNotebookID` to mean "protected". Adding a second
constant to all thirteen would have been the wrong move, so one predicate
replaces the comparison everywhere — which is why the Reports notebook was
protected on the day it was added rather than the day someone remembered this
line.

The naming is load-bearing, because two different sets exist and the code needs
both:

| Set | Members | Test |
|---|---|---|
| Undeletable | Help, Reports, **Notes** | `nb.Builtin` **or** `id == DefaultNotebookID` |
| Read-only / system-authored | Help, Reports | `store.IsReadOnlyNotebook(id)` |

The default **Notes** notebook is bootstrap-created and undeletable but is
`builtin = 0` in the database, because its content is the *user's*. Treating it
as read-only would have silently excluded most of a library from the graph
report, from publications, and from lint. A test pins that `DefaultNotebookID`
is **not** in the set, so widening it by reaching for the more familiar word
fails there rather than in someone's library.

**The graph report was measuring something other than the live library.** Two
filters were missing and one of them was a defect.

- *Trashed sources — a pre-existing defect.* The row set already excluded
  trashed notes, so one was never *ranked*, but the in-degree subquery placed no
  condition on the link's **source**. Soft delete writes a revision and clears
  the FTS row but deliberately **leaves `document_links` intact** so a restore
  can use them. So a note sitting in the Trash kept propping up the in-degree of
  everything it had linked to, and a note linked only from the Trash was never
  reported as an orphan.
- *System-authored sources.* The report links to every hub it ranks, so without
  this filter writing the report would change the ranking the next generation
  sees — an observer effect built in by construction. Excluding the report from
  its own *ranking* would not have fixed it, because the links would still
  count. The same filter retired a quieter problem: Notrios' own Help notes link
  to each other heavily and had been inflating whatever they referenced all
  along.

Both ends of every counted edge are filtered, not just the source, so
`link_count` finally means what its documentation always claimed: edges between
two live notes that a traversal can actually follow. Rows in read-only
notebooks are not measured either — otherwise every Help note would have arrived
as a fresh orphan the moment the link filter started working.

**Mutation-verified.** Restoring the original in-degree subquery makes both new
tests fail with the exact numbers the defect produced (in-degree 1 for a note
linked only from the Trash, and 1 for a note linked only from Help/Reports).

**The traversal filter has one exemption, and it earns its keep.** `POST
/api/v1/graph` also ignores links originating in a read-only notebook —
otherwise every hub's local graph would show the report sitting one hop away,
noise in exactly the view this slice argues stays useful at scale — **unless
that note is one of the roots asked about**. Without the exemption, opening the
report or any Help page would render an empty graph, which is a regression in a
visible feature dressed up as consistency.

**Publication excludes read-only notebooks; the other two targets do not.** A
publication is the one handoff that leaves the machine, and nothing before this
stopped it from dumping Notrios' own documentation onto someone's site. It is a
rule inside the target's default policy rather than a user-facing field — not
something a user should have to configure, and a field invites getting it wrong
— reported in the dry run as `read_only_notebook:<id>` so it is visible rather
than silent, and not overridable in v0.6, because adding an opt-in later is easy
and removing a leak is not. A **full archive** must stay faithful or restore
becomes lossy; a **subset transfer** moves notes between the user's own
databases, where their own Help and Reports are not a disclosure.

**Lint skips notes nobody can fix — but only the content checks.** The link scan
filtered on collection and `deleted_at` and nothing else, while
`documentIsWritableLocked` refuses to touch a read-only note: lint reported
broken links inside Notrios' own documentation, `notriosctl fix` structurally
could not repair them, and the user could not edit the note either.
`projection_backlog` is deliberately **not** filtered — it says the search index
is behind rather than that a note needs editing, and hiding it would hide a real
operational problem for nothing. The distinction is pinned by a test so it reads
as deliberate rather than as a place the filter was forgotten.

*This is a visible change to lint output.* On the repository's own seeded Help
notebook it removes **71 unresolved-link findings** that were never actionable,
and `report_sha256` changes with them.

**A generated note cannot use Markdown escaping to survive a hostile title.**
The first implementation escaped `[` and `]` in the link text. Notrios' own link
parser forbids `]` inside link text *even backslash-escaped* — the pattern is
`\[([^\]\n]*)\]\(` — so the escaped form produced a link the store could not
resolve: a broken link in a generated note, in the user's lint report, blamed on
a note they cannot edit. A title carrying `]` is now written verbatim beside a
short `([open](…))` link instead of being altered to fit; a title with no
bracket still labels its own link. Both branches are asserted, because a
fallback that never fires and one that always fires are both wrong.

**No graph library, and none needed.** `gonum/graph`, `dominikbraun/graph`, and
`yourbasic/graph` are all capable and appropriately licensed, but the traversal,
the shortest path, and the in-degree ranking are already bounded SQL, and a
local graph at depth 1–2 is tens of nodes. Adding a dependency to recompute what
the store already answers would be pure cost.

**Export is CSV and CLI-only.** CSV streams at any library size without holding
a document tree in memory, and Gephi, Cytoscape, NetworkX, and igraph all import
it; column names follow Gephi's convention because it is the fussiest of the
four. It writes files to a path the user named — not a choice a REST caller or
an MCP client should make — and refuses to replace an existing file without
`--overwrite`. Notrios emits the graph rather than reimplementing centrality and
community detection inside a notes app.

**Nothing new on the MCP path.** `get_graph_report` reads; regenerating the note
scans the whole collection and overwrites a note, and export writes files. Both
join the withheld list with the reason recorded, and
`TestNoScopeReachesWholeLibraryOperations` now names them so a future tool
cannot arrive quietly.

## Verified in a browser, which found a defect the tests did not

Against a database with the 15 seeded Help notes plus four ordinary ones:

- the sidebar renders All notes, Notes, **Reports**, Help, Trash;
- `graph report` reports `document_count: 0` before any ordinary note exists —
  the 15 Help notes are correctly not measured;
- Kitchen Plan's local graph shows Alpha and Beta and **not** the report, while
  the database confirms the report *does* carry a resolved link to it, so the
  absence is the filter working rather than a missing link;
- the report's own local graph shows all four notes it names, which is the root
  exemption;
- regenerating twice leaves every count identical and Reports holding one note;
- `PUT` and `DELETE` on the report answer `403 notes in the Reports notebook are
  read-only`.

**The defect:** the editor's read-only badge was hard-coded to
`Read-only Help note` and rendered that on a note in **Reports** — wrong in the
one place a reader looks to find out why they cannot type. It now names the
note's actual notebook, with a test for each.

## Validation

`go vet ./...`, `go test ./...`, `make validate`,
`python3 scripts/check_required_files.py`, `python3 scripts/check_plan_loops.py`,
OpenAPI parse (61 paths), `npx tsc --noEmit`, 155 web tests, `npm run build`,
`make gui`, `bash scripts/build_docs_site.sh`, `notriosctl seed-help`,
`bash scripts/mvp_smoke.sh`, `bash scripts/run_performance_smoke.sh`,
`bash scripts/run_offline_assets_check.sh`, plus the browser session above and
end-to-end CLI runs of `graph report`, `graph report --write-note`, and
`graph export`.
